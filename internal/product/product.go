package product

import "time"

// Category represents a target scraping category for a site.
type Category struct {
	// Slug is the site-specific URL identifier or path fragment (e.g., "decor-objects" or "furniture-sofas").
	Slug string `json:"slug"`

	// Name is our internal normalized taxonomy name (e.g., "decor_objects", "sofas").
	Name string `json:"name"`
}

// ProductRef is a lightweight reference to a product discovered during category listing.
// It gives the worker pool the minimal unit of work to fetch product details.
type ProductRef struct {
	// URL is the canonical product page URL.
	URL string `json:"url"`

	// Category is the normalized category name.
	Category string `json:"category"`
}

// Product represents a normalized catalog item matching our canonical schema.
type Product struct {
	Source     string    `json:"source"`
	ExternalID string    `json:"external_id"`
	URL        string    `json:"url"`
	Name       string    `json:"name"`
	PriceMinor int64     `json:"price_minor"` // paise (1 INR = 100 paise) to prevent float rounding errors
	Currency   string    `json:"currency"`    // "INR"
	Category   string    `json:"category"`
	Styles     []string  `json:"styles"` // populated for sites with style facets (Nestasia); empty for Pepperfry
	ImageURL   string    `json:"image_url"`
	InStock    bool      `json:"in_stock"`
	ScrapedAt  time.Time `json:"scraped_at"`
}

// StableKey returns the unique composite key used for deduplication and idempotent upserts.
func (p Product) StableKey() string {
	return p.Source + ":" + p.ExternalID
}
