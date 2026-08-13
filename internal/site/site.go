package site

import (
	"context"

	"devaraja-anu/decor-scrapper/internal/product"
)

// Site represents the contract that all retailer scrapers must implement.
// The orchestration pipeline interacts strictly through this interface.
type Site interface {
	// Name returns the canonical identifier for the retailer (e.g., "nestasia", "pepperfry").
	Name() string

	// Categories returns the configured categories to scrape for this site.
	Categories() []product.Category

	// ListProducts discovers product references within a specific category.
	ListProducts(ctx context.Context, cat product.Category) ([]product.ProductRef, error)

	// FetchProduct retrieves and normalizes the full product details for a given reference.
	FetchProduct(ctx context.Context, ref product.ProductRef) (product.Product, error)
}
