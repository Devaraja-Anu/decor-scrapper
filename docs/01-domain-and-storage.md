# Milestone 1: Domain Model & Storage Layer

## Overview
This milestone establishes the foundational types and storage engine for the scraper, ensuring complete decoupling between site-specific scraping logic and the data persistence format.

---

## 1. Domain Types (`internal/product`)
- **`Product`**: Canonical representation of a scraped item adhering to `schema.md`.
  - `PriceMinor int64`: Stores price in paise (integer) to eliminate floating-point drift.
  - `Currency string`: Explicitly set to `"INR"`.
  - `Styles []string`: Populated for facet-rich sources (Nestasia) and left empty for un-faceted sources (Pepperfry).
- **`Category`**: Encapsulates `Slug` (site URL path) and `Name` (normalized taxonomy name).
- **`ProductRef`**: Lightweight URL and category reference discovered during listing, representing the discrete unit of work for the worker pool.
- **`StableKey()`**: Returns `Source + ":" + ExternalID` to guarantee idempotent merges across repeated runs.

---

## 2. Site Interface Contract (`internal/site`)
Defines the uniform interface that the orchestration pipeline consumes:
```go
type Site interface {
    Name() string
    Categories() []product.Category
    ListProducts(ctx context.Context, cat product.Category) ([]product.ProductRef, error)
    FetchProduct(ctx context.Context, ref product.ProductRef) (product.Product, error)
}
```

---

## 3. Storage Layer (`internal/store`)
Implemented in `internal/store/jsonfile.go`:
- **Safe Merges**: Merges new items with existing items using `StableKey()`. If an existing product is omitted from a transient scrape run, its record is retained.
- **Atomic Writes**: Writes encoded JSON to a temporary file (`catalog-*.tmp`) in the destination directory, syncs to disk, and executes an atomic rename over `catalog.json` to prevent file corruption during mid-write crashes.
- **Deterministic Ordering**: Sorts entries alphabetically by `StableKey()` before serialization, keeping git diffs clean.

---

## 4. Verification
All units are verified with Go unit tests:
- `internal/product`: Tests `StableKey()` generation and schema field integrity.
- `internal/store`: Tests empty catalog initialization, multi-batch merge logic, record updates, and atomic temporary file cleanup.
