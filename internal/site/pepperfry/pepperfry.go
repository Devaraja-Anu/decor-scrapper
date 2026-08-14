package pepperfry

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"devaraja-anu/decor-scrapper/internal/product"
)

const (
	defaultBaseURL = "https://www.pepperfry.com"
	siteName       = "pepperfry"
)

var idFromURLRegex = regexp.MustCompile(`[-_](\d+)\.html`)
var numericOnlyRegex = regexp.MustCompile(`[^\d.]`)

// Option configures the Pepperfry scraper.
type Option func(*Pepperfry)

// Pepperfry implements the site.Site interface for Pepperfry's HTML pages.
type Pepperfry struct {
	baseURL    string
	httpClient *http.Client
}

// WithBaseURL overrides the base URL (useful for testing).
func WithBaseURL(baseURL string) Option {
	return func(p *Pepperfry) {
		p.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(p *Pepperfry) {
		p.httpClient = client
	}
}

// New creates a new Pepperfry scraper instance.
func New(opts ...Option) *Pepperfry {
	p := &Pepperfry{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the canonical site identifier.
func (p *Pepperfry) Name() string {
	return siteName
}

// Categories returns the core furniture categories for Pepperfry.
func (p *Pepperfry) Categories() []product.Category {
	return []product.Category{
		{Slug: "furniture-sofas", Name: "sofas"},
		{Slug: "furniture-coffee-tables", Name: "coffee_tables"},
		{Slug: "furniture-dining-chairs", Name: "dining_chairs"},
	}
}

// ListProducts discovers product references from a category listing page.
func (p *Pepperfry) ListProducts(ctx context.Context, cat product.Category) ([]product.ProductRef, error) {
	reqURL := fmt.Sprintf("%s/category/%s.html", p.baseURL, cat.Slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for category %q: %w", cat.Slug, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DecorScraper/1.0)")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching category %q: %w", cat.Slug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching category %q returned status %d", cat.Slug, resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML for category %q: %w", cat.Slug, err)
	}

	seenURLs := make(map[string]struct{})
	var refs []product.ProductRef

	// Query product links from listing cards
	doc.Find("a.product-link, .product-card a, a[href*='/product/']").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		fullURL := p.resolveURL(href)
		if !strings.Contains(fullURL, "/product/") {
			return
		}

		if _, seen := seenURLs[fullURL]; !seen {
			seenURLs[fullURL] = struct{}{}
			refs = append(refs, product.ProductRef{
				URL:      fullURL,
				Category: cat.Name,
			})
		}
	})

	return refs, nil
}

// FetchProduct parses a Pepperfry product detail page into a canonical Product entity.
func (p *Pepperfry) FetchProduct(ctx context.Context, ref product.ProductRef) (product.Product, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref.URL, nil)
	if err != nil {
		return product.Product{}, fmt.Errorf("creating request for product %q: %w", ref.URL, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; DecorScraper/1.0)")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return product.Product{}, fmt.Errorf("fetching product %q: %w", ref.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return product.Product{}, fmt.Errorf("fetching product %q returned status %d", ref.URL, resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return product.Product{}, fmt.Errorf("parsing HTML for product %q: %w", ref.URL, err)
	}

	// 1. Extract External ID
	externalID := p.extractExternalID(doc, ref.URL)
	if externalID == "" {
		return product.Product{}, fmt.Errorf("could not extract external ID for %q", ref.URL)
	}

	// 2. Extract Name
	name := strings.TrimSpace(doc.Find("h1.v-product__title, h1.product-title, h1").First().Text())
	if name == "" {
		name = strings.TrimSpace(doc.Find("meta[property='og:title']").AttrOr("content", ""))
	}
	if name == "" {
		return product.Product{}, fmt.Errorf("could not extract product title for %q", ref.URL)
	}

	// 3. Extract Price
	var priceStr string
	priceSelectors := []string{
		".v-product__price--current",
		".product-price",
		"meta[property='product:price:amount']",
		".v-product__price",
	}
	for _, sel := range priceSelectors {
		if strings.HasPrefix(sel, "meta") {
			if content := doc.Find(sel).AttrOr("content", ""); content != "" {
				priceStr = content
				break
			}
		} else {
			if node := doc.Find(sel); node.Length() > 0 {
				txt := strings.TrimSpace(node.First().Text())
				if txt != "" {
					priceStr = txt
					break
				}
			}
		}
	}

	priceMinor, err := parsePriceMinorINR(priceStr)
	if err != nil {
		return product.Product{}, fmt.Errorf("parsing price for product %q: %w", ref.URL, err)
	}

	// 4. Extract Image URL
	imageURL := doc.Find("img.v-product-gallery__image, img.product-image").First().AttrOr("src", "")
	if imageURL == "" {
		imageURL = doc.Find("meta[property='og:image']").AttrOr("content", "")
	}
	if imageURL != "" {
		imageURL = p.resolveURL(imageURL)
	}

	// 5. Extract InStock status
	inStock := true
	if doc.Find(".v-product__out-of-stock-msg, .out-of-stock, button[disabled].v-product__btn-notify").Length() > 0 {
		inStock = false
	}

	// 6. Styles: Deliberately empty for Pepperfry products per decisions.md.
	// Pepperfry lacks structured style facets; coarse category mapping produces false signals.
	// Style inference is intentionally deferred downstream to the design app's LLM layer.
	styles := []string{}

	return product.Product{
		Source:     siteName,
		ExternalID: externalID,
		URL:        ref.URL,
		Name:       name,
		PriceMinor: priceMinor,
		Currency:   "INR",
		Category:   ref.Category,
		Styles:     styles,
		ImageURL:   imageURL,
		InStock:    inStock,
		ScrapedAt:  time.Now().UTC(),
	}, nil
}

func (p *Pepperfry) extractExternalID(doc *goquery.Document, rawURL string) string {
	// Strategy A: meta retailer item id
	if id := doc.Find("meta[property='product:retailer_item_id']").AttrOr("content", ""); id != "" {
		return strings.TrimSpace(id)
	}

	// Strategy B: data-product-id attribute on container
	if id := doc.Find("[data-product-id]").First().AttrOr("data-product-id", ""); id != "" {
		return strings.TrimSpace(id)
	}

	// Strategy C: regex from URL (e.g. /product/sofa-name-2545615.html)
	if match := idFromURLRegex.FindStringSubmatch(rawURL); len(match) > 1 {
		return match[1]
	}

	return ""
}

func (p *Pepperfry) resolveURL(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		return p.baseURL + href
	}
	return p.baseURL + "/" + href
}

func parsePriceMinorINR(priceStr string) (int64, error) {
	tokens := strings.Fields(priceStr)
	if len(tokens) == 0 {
		return 0, fmt.Errorf("empty price string")
	}
	target := tokens[0]

	cleaned := numericOnlyRegex.ReplaceAllString(target, "")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return 0, fmt.Errorf("empty price string from %q", priceStr)
	}

	// If contains decimal point
	if strings.Contains(cleaned, ".") {
		val, err := strconv.ParseFloat(cleaned, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing float price %q: %w", cleaned, err)
		}
		return int64(math.Round(val * 100)), nil
	}

	// Pure integer string
	val, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing int price %q: %w", cleaned, err)
	}
	return val * 100, nil
}
