package pepperfry

import (
	"context"
	"encoding/json"
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
		{Slug: "category/3-seater-sofas", Name: "sofas"},
		{Slug: "category/coffee-tables", Name: "coffee_tables"},
		{Slug: "category/dining-chairs", Name: "dining_chairs"},
	}
}

// ListProducts discovers product references from a category listing page.
func (p *Pepperfry) ListProducts(ctx context.Context, cat product.Category) ([]product.ProductRef, error) {
	slug := strings.TrimPrefix(cat.Slug, "/")
	reqURL := fmt.Sprintf("%s/%s.html", p.baseURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for category %q: %w", cat.Slug, err)
	}
	setBrowserHeaders(req)

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
	setBrowserHeaders(req)

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
		name = strings.TrimSpace(doc.Find("meta[name='twitter:title']").AttrOr("content", ""))
	}
	if name == "" {
		return product.Product{}, fmt.Errorf("could not extract product title for %q", ref.URL)
	}

	// 3. Extract Price via multi-strategy (JSON-LD, meta tags, DOM, or embedded script JSON)
	priceMinor, err := p.extractPrice(doc)
	if err != nil {
		return product.Product{}, fmt.Errorf("parsing price for product %q: %w", ref.URL, err)
	}

	// 4. Extract Image URL (prefer JSON-LD or gallery image, ignore site logo)
	imageURL := p.extractImage(doc)

	// 5. Extract InStock status
	inStock := p.extractInStock(doc)

	// 6. Styles: Deliberately empty for Pepperfry products per decisions.md.
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

func setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Ch-Ua", `"Not)A;Brand";v="99", "Google Chrome";v="127", "Chromium";v="127"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
}

var jsonPriceRegex = regexp.MustCompile(`(?i)(?:offer_price|offerPrice|selling_price|sellingPrice|final_price|finalPrice|special_price|"price")\s*[:=]\s*["']?([0-9.,]+)`)

func (p *Pepperfry) extractPrice(doc *goquery.Document) (int64, error) {
	// Strategy A: JSON-LD Schema.org Product
	var jsonLdPrice string
	doc.Find("script[type='application/ld+json']").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		txt := strings.TrimSpace(s.Text())
		if txt == "" {
			return true
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(txt), &parsed); err == nil {
			if parsed["@type"] == "Product" {
				if offers, ok := parsed["offers"].(map[string]any); ok {
					if price, exists := offers["price"]; exists {
						jsonLdPrice = fmt.Sprintf("%v", price)
						return false
					}
				}
			}
		}
		return true
	})
	if jsonLdPrice != "" {
		if minor, err := parsePriceMinorINR(jsonLdPrice); err == nil && minor > 0 {
			return minor, nil
		}
	}

	// Strategy B: Standard Meta tags & Itemprops
	metaSelectors := []string{
		"meta[property='product:price:amount']",
		"meta[property='og:price:amount']",
		"meta[itemprop='price']",
		"[itemprop='price']",
		"meta[name='twitter:data1']",
	}
	for _, sel := range metaSelectors {
		var val string
		if strings.HasPrefix(sel, "meta") {
			val = doc.Find(sel).AttrOr("content", "")
		} else {
			val = doc.Find(sel).First().Text()
		}
		if strings.TrimSpace(val) != "" {
			if minor, err := parsePriceMinorINR(val); err == nil && minor > 0 {
				return minor, nil
			}
		}
	}

	// Strategy C: DOM CSS Classes
	domSelectors := []string{
		".v-product__price--current",
		".product-price",
		".v-product__price",
		".offer-price",
		".final-price",
		".selling-price",
	}
	for _, sel := range domSelectors {
		if node := doc.Find(sel); node.Length() > 0 {
			val := node.First().Text()
			if dataAttr := node.First().AttrOr("data-price", ""); dataAttr != "" {
				val = dataAttr
			}
			if strings.TrimSpace(val) != "" {
				if minor, err := parsePriceMinorINR(val); err == nil && minor > 0 {
					return minor, nil
				}
			}
		}
	}

	// Strategy D: Embedded Script JSON Regex search
	var scriptPrice string
	doc.Find("script").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		txt := s.Text()
		if match := jsonPriceRegex.FindStringSubmatch(txt); len(match) > 1 {
			scriptPrice = match[1]
			return false
		}
		return true
	})
	if scriptPrice != "" {
		if minor, err := parsePriceMinorINR(scriptPrice); err == nil && minor > 0 {
			return minor, nil
		}
	}

	return 0, fmt.Errorf("empty price string")
}

func (p *Pepperfry) extractInStock(doc *goquery.Document) bool {
	// Check out-of-stock DOM indicators
	if doc.Find(".v-product__out-of-stock-msg, .out-of-stock, button[disabled].v-product__btn-notify, .sold-out").Length() > 0 {
		return false
	}

	// Check JSON-LD
	var ldStock *bool
	doc.Find("script[type='application/ld+json']").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(s.Text()), &parsed); err == nil {
			if offers, ok := parsed["offers"].(map[string]any); ok {
				if avail, ok := offers["availability"].(string); ok {
					b := strings.Contains(strings.ToLower(avail), "instock")
					ldStock = &b
					return false
				}
			}
		}
		return true
	})
	if ldStock != nil {
		return *ldStock
	}

	return true
}

func (p *Pepperfry) extractImage(doc *goquery.Document) string {
	// Strategy A: JSON-LD Schema.org Product image
	var ldImg string
	doc.Find("script[type='application/ld+json']").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(s.Text()), &parsed); err == nil {
			if parsed["@type"] == "Product" {
				if img, ok := parsed["image"].(string); ok && isProductImage(img) {
					ldImg = img
					return false
				}
				if imgs, ok := parsed["image"].([]any); ok && len(imgs) > 0 {
					if imgStr, ok := imgs[0].(string); ok && isProductImage(imgStr) {
						ldImg = imgStr
						return false
					}
				}
			}
		}
		return true
	})
	if ldImg != "" {
		return p.resolveURL(ldImg)
	}

	// Strategy B: DOM gallery and product image tags
	var domImg string
	doc.Find("img[src*='/media/catalog/product/'], img[data-src*='/media/catalog/product/'], img.v-product-gallery__image, img.product-image").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src := s.AttrOr("src", "")
		if !isProductImage(src) {
			src = s.AttrOr("data-src", "")
		}
		if isProductImage(src) {
			domImg = src
			return false
		}
		return true
	})
	if domImg != "" {
		return p.resolveURL(domImg)
	}

	// Strategy C: Meta tags if valid product photo
	if ogImg := doc.Find("meta[property='og:image']").AttrOr("content", ""); isProductImage(ogImg) {
		return p.resolveURL(ogImg)
	}
	if twImg := doc.Find("meta[name='twitter:image']").AttrOr("content", ""); isProductImage(twImg) {
		return p.resolveURL(twImg)
	}

	return ""
}

func isProductImage(urlStr string) bool {
	if urlStr == "" {
		return false
	}
	lower := strings.ToLower(urlStr)
	if strings.Contains(lower, "logo") || strings.HasSuffix(lower, ".svg") {
		return false
	}
	return true
}
