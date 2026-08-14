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

	reqURL := fmt.Sprintf("%s/products/%s.json", n.baseURL, url.PathEscape(handle))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return product.Product{}, fmt.Errorf("creating product request for %q: %w", handle, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return product.Product{}, fmt.Errorf("fetching product %q: %w", handle, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return product.Product{}, fmt.Errorf("fetching product %q returned status %d", handle, resp.StatusCode)
	}

	var payload shopifyProductResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return product.Product{}, fmt.Errorf("decoding product %q response: %w", handle, err)
	}

	return n.normalizeProduct(payload.Product, ref)
}

func (n *Nestasia) normalizeProduct(sp shopifyProduct, ref product.ProductRef) (product.Product, error) {
	var priceMinor int64
	var inStock bool

	if len(sp.Variants) > 0 {
		var err error
		priceMinor, err = parsePriceMinor(sp.Variants[0].Price)
		if err != nil {
			return product.Product{}, fmt.Errorf("parsing price for product %d: %w", sp.ID, err)
		}
		for _, v := range sp.Variants {
			if v.Available {
				inStock = true
				break
			}
		}
	}

	var imageURL string
	if len(sp.Images) > 0 {
		imageURL = sp.Images[0].Src
	}

	styles := extractStyles(sp.Tags)

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
	Products []shopifyProduct `json:"products"`
}

type shopifyProductResponse struct {
	Product shopifyProduct `json:"product"`
}

type shopifyProduct struct {
	ID       int64            `json:"id"`
	Title    string           `json:"title"`
	Handle   string           `json:"handle"`
	Tags     TagsList         `json:"tags"`
	Variants []shopifyVariant `json:"variants"`
	Images   []shopifyImage   `json:"images"`
}

type shopifyVariant struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Price     string `json:"price"`
	Available bool   `json:"available"`
}

type shopifyImage struct {
	ID  int64  `json:"id"`
	Src string `json:"src"`
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

func parsePriceMinor(priceStr string) (int64, error) {
	priceStr = strings.TrimSpace(priceStr)
	if priceStr == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price string %q: %w", priceStr, err)
	}
	return int64(math.Round(f * 100)), nil
}

func extractStyles(tags []string) []string {
	matched := make(map[string]struct{})
	for _, rawTag := range tags {
		tag := strings.ToLower(strings.TrimSpace(rawTag))
		switch {
		case strings.Contains(tag, "bohemian") || tag == "boho":
			matched["bohemian"] = struct{}{}
		case strings.Contains(tag, "contemporary"):
			matched["contemporary"] = struct{}{}
		case strings.Contains(tag, "glam") || strings.Contains(tag, "glamour"):
			matched["glam"] = struct{}{}
		case strings.Contains(tag, "minimalist") || strings.Contains(tag, "minimal"):
			matched["minimalist"] = struct{}{}
		case strings.Contains(tag, "modern"):
			matched["modern"] = struct{}{}
		case strings.Contains(tag, "scandinavian") || tag == "scandi":
			matched["scandinavian"] = struct{}{}
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
