package nestasia

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"devaraja-anu/decor-scrapper/internal/product"
)

const (
	defaultBaseURL = "https://nestasia.in"
	siteName       = "nestasia"
)

// canonical style order for deterministic output
var knownStyles = []string{
	"bohemian",
	"contemporary",
	"glam",
	"minimalist",
	"modern",
	"scandinavian",
}

// Option configures the Nestasia scraper.
type Option func(*Nestasia)

// Nestasia implements the site.Site interface for the Nestasia Shopify store.
type Nestasia struct {
	baseURL    string
	httpClient *http.Client
}

// WithBaseURL overrides the base URL (useful for testing).
func WithBaseURL(baseURL string) Option {
	return func(n *Nestasia) {
		n.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(n *Nestasia) {
		n.httpClient = client
	}
}

// New creates a new Nestasia scraper instance.
func New(opts ...Option) *Nestasia {
	n := &Nestasia{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(n)
	}
	return n
}

// Name returns the canonical site identifier.
func (n *Nestasia) Name() string {
	return siteName
}

// Categories returns the high-impact target decor categories for Nestasia.
func (n *Nestasia) Categories() []product.Category {
	return []product.Category{
		{Slug: "rugs", Name: "rugs"},
		{Slug: "wall-decor", Name: "wall_decor"},
		{Slug: "lamps-lighting", Name: "lighting"},
	}
}

// ListProducts discovers product references within a collection.
func (n *Nestasia) ListProducts(ctx context.Context, cat product.Category) ([]product.ProductRef, error) {
	reqURL := fmt.Sprintf("%s/collections/%s/products.json?limit=250", n.baseURL, url.PathEscape(cat.Slug))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for category %q: %w", cat.Slug, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching collection %q: %w", cat.Slug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching collection %q returned status %d", cat.Slug, resp.StatusCode)
	}

	var payload shopifyCollectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding collection %q response: %w", cat.Slug, err)
	}

	refs := make([]product.ProductRef, 0, len(payload.Products))
	for _, p := range payload.Products {
		prodURL := fmt.Sprintf("%s/products/%s", n.baseURL, p.Handle)
		refs = append(refs, product.ProductRef{
			URL:      prodURL,
			Category: cat.Name,
		})
	}

	return refs, nil
}

// FetchProduct retrieves and normalizes product details for a given reference.
func (n *Nestasia) FetchProduct(ctx context.Context, ref product.ProductRef) (product.Product, error) {
	handle := extractHandle(ref.URL)
	if handle == "" {
		return product.Product{}, fmt.Errorf("unable to extract product handle from URL %q", ref.URL)
	}

	// Primary: Hit Storefront AJAX endpoint /products/{handle}.js for live stock availability
	reqURL := fmt.Sprintf("%s/products/%s.js", n.baseURL, url.PathEscape(handle))
	resp, err := n.doGetWithRetry(ctx, reqURL)
	if err != nil {
		// Fallback to .json endpoint if .js encountered an error
		return n.fetchProductJSONFallback(ctx, handle, ref)
	}
	defer resp.Body.Close()

	// If .js endpoint returns 404, fallback to .json endpoint
	if resp.StatusCode == http.StatusNotFound {
		return n.fetchProductJSONFallback(ctx, handle, ref)
	}

	if resp.StatusCode != http.StatusOK {
		return product.Product{}, fmt.Errorf("fetching product %q returned status %d", handle, resp.StatusCode)
	}

	var payload rawShopifyProduct
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return product.Product{}, fmt.Errorf("decoding product %q response: %w", handle, err)
	}

	return n.normalizeProduct(payload, ref)
}

func (n *Nestasia) fetchProductJSONFallback(ctx context.Context, handle string, ref product.ProductRef) (product.Product, error) {
	reqURL := fmt.Sprintf("%s/products/%s.json", n.baseURL, url.PathEscape(handle))
	resp, err := n.doGetWithRetry(ctx, reqURL)
	if err != nil {
		return product.Product{}, fmt.Errorf("fetching fallback product %q: %w", handle, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return product.Product{}, fmt.Errorf("fetching fallback product %q returned status %d", handle, resp.StatusCode)
	}

	var wrapper struct {
		Product rawShopifyProduct `json:"product"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return product.Product{}, fmt.Errorf("decoding fallback product %q response: %w", handle, err)
	}

	return n.normalizeProduct(wrapper.Product, ref)
}

func (n *Nestasia) doGetWithRetry(ctx context.Context, reqURL string) (*http.Response, error) {
	const maxRetries = 3
	backoff := 1500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		resp, err := n.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			retryAfter := resp.Header.Get("Retry-After")
			waitDuration := backoff
			if s, err := strconv.Atoi(retryAfter); err == nil && s > 0 {
				waitDuration = time.Duration(s) * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitDuration):
				backoff *= 2
				continue
			}
		}

		return resp, nil
	}

	return nil, fmt.Errorf("exceeded max retries for %q due to rate limiting (429)", reqURL)
}

func (n *Nestasia) normalizeProduct(sp rawShopifyProduct, ref product.ProductRef) (product.Product, error) {
	var priceMinor int64
	var inStock bool

	// 1. Stock Status: Prefer product-level availability or any available variant
	if sp.Available != nil && *sp.Available {
		inStock = true
	}
	for _, v := range sp.Variants {
		if v.Available != nil && *v.Available {
			inStock = true
			break
		}
	}

	// 2. Price Parsing
	if len(sp.Variants) > 0 {
		var err error
		priceMinor, err = parseFlexiblePriceMinor(sp.Variants[0].Price)
		if err != nil {
			return product.Product{}, fmt.Errorf("parsing variant price for product %d: %w", sp.ID, err)
		}
	}
	if priceMinor == 0 && sp.Price != nil {
		var err error
		priceMinor, err = parseFlexiblePriceMinor(sp.Price)
		if err != nil {
			return product.Product{}, fmt.Errorf("parsing product price for product %d: %w", sp.ID, err)
		}
	}

	// 3. Image URL
	imageURL := extractImageURL(sp)

	// 4. Styles: Extracted strictly from structured Tags and Options (no free-text guessing)
	styles := extractStyles(sp.Tags, sp.Options)

	canonicalURL := ref.URL
	if canonicalURL == "" {
		canonicalURL = fmt.Sprintf("%s/products/%s", n.baseURL, sp.Handle)
	}

	return product.Product{
		Source:     siteName,
		ExternalID: strconv.FormatInt(sp.ID, 10),
		URL:        canonicalURL,
		Name:       sp.Title,
		PriceMinor: priceMinor,
		Currency:   "INR",
		Category:   ref.Category,
		Styles:     styles,
		ImageURL:   imageURL,
		InStock:    inStock,
		ScrapedAt:  time.Now().UTC(),
	}, nil
}

func extractImageURL(sp rawShopifyProduct) string {
	if len(sp.Images) > 0 {
		first := sp.Images[0]
		switch v := first.(type) {
		case string:
			return normalizeImageURL(v)
		case map[string]any:
			if src, ok := v["src"].(string); ok {
				return normalizeImageURL(src)
			}
		}
	}
	if sp.FeaturedImage != "" {
		return normalizeImageURL(sp.FeaturedImage)
	}
	return ""
}

func normalizeImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	return raw
}

// TagsList supports unmarshaling both JSON array of strings and comma-delimited strings.
type TagsList []string

// UnmarshalJSON unmarshals a JSON array or a comma-separated string into a string slice.
func (t *TagsList) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		parts := strings.Split(str, ",")
		res := make([]string, 0, len(parts))
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				res = append(res, trimmed)
			}
		}
		*t = res
		return nil
	}

	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	*t = list
	return nil
}

type shopifyCollectionResponse struct {
	Products []rawShopifyProduct `json:"products"`
}

type rawShopifyProduct struct {
	ID            int64               `json:"id"`
	Title         string              `json:"title"`
	Handle        string              `json:"handle"`
	Tags          TagsList            `json:"tags"`
	Available     *bool               `json:"available,omitempty"`
	Price         any                 `json:"price,omitempty"`
	Variants      []rawShopifyVariant `json:"variants"`
	Images        []any               `json:"images"`
	FeaturedImage string              `json:"featured_image,omitempty"`
	Options       []shopifyOption     `json:"options"`
}

type rawShopifyVariant struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Price     any    `json:"price"`
	Available *bool  `json:"available,omitempty"`
}

type shopifyOption struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

func extractHandle(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		trimmed := strings.TrimRight(rawURL, "/")
		idx := strings.LastIndex(trimmed, "/")
		if idx != -1 {
			return trimmed[idx+1:]
		}
		return rawURL
	}
	path := strings.TrimRight(u.Path, "/")
	idx := strings.LastIndex(path, "/")
	if idx != -1 {
		return path[idx+1:]
	}
	return path
}

func parseFlexiblePriceMinor(val any) (int64, error) {
	if val == nil {
		return 0, nil
	}
	switch v := val.(type) {
	case float64:
		return int64(math.Round(v)), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, nil
		}
		if strings.Contains(v, ".") {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid float price string %q: %w", v, err)
			}
			return int64(math.Round(f * 100)), nil
		}
		num, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid int price string %q: %w", v, err)
		}
		return num * 100, nil
	default:
		return 0, fmt.Errorf("unsupported price type %T", val)
	}
}

func extractStyles(tags []string, options []shopifyOption) []string {
	matched := make(map[string]struct{})

	// 1. Structured Tags (including style_<StyleName> prefixes and synonyms)
	for _, rawTag := range tags {
		tag := strings.ToLower(strings.TrimSpace(rawTag))
		tag = strings.TrimPrefix(tag, "style_")
		tag = strings.TrimPrefix(tag, "style-")
		matchStyleString(tag, matched)
	}

	// 2. Structured Options (e.g. Option name "Style", "Theme", "Aesthetic")
	for _, opt := range options {
		optName := strings.ToLower(strings.TrimSpace(opt.Name))
		if strings.Contains(optName, "style") || strings.Contains(optName, "theme") || strings.Contains(optName, "aesthetic") {
			for _, val := range opt.Values {
				v := strings.ToLower(strings.TrimSpace(val))
				v = strings.TrimPrefix(v, "style_")
				v = strings.TrimPrefix(v, "style-")
				matchStyleString(v, matched)
			}
		}
	}

	if len(matched) == 0 {
		return []string{}
	}

	styles := make([]string, 0, len(matched))
	for _, s := range knownStyles {
		if _, ok := matched[s]; ok {
			styles = append(styles, s)
		}
	}
	return styles
}

func matchStyleString(val string, matched map[string]struct{}) {
	switch {
	case strings.Contains(val, "bohemian") || val == "boho":
		matched["bohemian"] = struct{}{}
	case strings.Contains(val, "contemporary"):
		matched["contemporary"] = struct{}{}
	case strings.Contains(val, "glam") || strings.Contains(val, "glamour") || strings.Contains(val, "luxe"):
		matched["glam"] = struct{}{}
	case strings.Contains(val, "minimalist") || val == "minimal" || val == "minimalism":
		matched["minimalist"] = struct{}{}
	case strings.Contains(val, "modern") || strings.Contains(val, "mid-century"):
		matched["modern"] = struct{}{}
	case strings.Contains(val, "scandinavian") || val == "scandi" || strings.Contains(val, "nordic"):
		matched["scandinavian"] = struct{}{}
	}
}
