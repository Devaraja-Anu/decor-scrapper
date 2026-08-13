package store

import (
	"context"

	"devaraja-anu/decor-scrapper/internal/product"
)

// Store defines the contract for persisting and retrieving catalog products.
type Store interface {
	// Save persists products by merging them idempotently with existing records.
	Save(ctx context.Context, products []product.Product) error

	// Load retrieves all currently persisted products.
	Load(ctx context.Context) ([]product.Product, error)
}
