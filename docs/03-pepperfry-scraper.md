# Milestone 3: Pepperfry Site Scraper

## Overview
This milestone implements the [`site.Site`](file:///C:/Programming/Projects/decor-scrapper/internal/site/site.go) interface for **Pepperfry** ([`internal/site/pepperfry`](file:///C:/Programming/Projects/decor-scrapper/internal/site/pepperfry/pepperfry.go)), India's premier online furniture retailer, parsing server-rendered HTML pages with `goquery` and normalizing them into canonical [`product.Product`](file:///C:/Programming/Projects/decor-scrapper/internal/product/product.go) entities.

---

## 1. 3 Core Furniture Categories
Pepperfry supplies the foundational furniture layer for the room:

| Category Slug | Normalized Name | Spatial Role |
|---|---|---|
| `furniture-sofas` | `sofas` | Primary living room seating anchor |
| `furniture-coffee-tables` | `coffee_tables` | Central living room surface & accent |
| `furniture-dining-chairs` | `dining_chairs` | Dining seating and accent chairs |

---

## 2. Implementation Details (`internal/site/pepperfry`)

### Discovery (`ListProducts`)
- Queries category URL: `/category/{slug}.html`
- Uses `goquery` to parse product card anchors (`a.product-link`, `.product-card a`, `a[href*="/product/"]`).
- Emits deduplicated, absolute-resolved [`product.ProductRef`](file:///C:/Programming/Projects/decor-scrapper/internal/product/product.go) items.

### Parsing & Normalization (`FetchProduct`)
- Fetches and parses product detail pages with `goquery`.
- **`ExternalID`**: Multi-tiered extraction strategy (Retailer item ID meta tag -> Container `data-product-id` attribute -> Regex fallback from URL path).
- **`PriceMinor`**: Extracts leaf price text, strips currency symbols and commas, parsing into integer paise (`2499900` for ₹24,999) with float-rounding safety.
- **`InStock`**: Detects out-of-stock badges and disabled notify buttons vs. active cart buttons.
- **`Styles`**: Explicitly set to empty `[]string{}` per architectural decision in [decisions.md](file:///C:/Programming/Projects/decor-scrapper/decisions.md), preserving data integrity and leaving style classification to the downstream LLM layer.

---

## 3. Verification & Testing
Verified offline against static HTML golden fixtures in [`internal/site/pepperfry/testdata/`](file:///C:/Programming/Projects/decor-scrapper/internal/site/pepperfry/testdata/) using `net/http/httptest`:
- `TestPepperfry_Metadata`: Validates site name and category definitions.
- `TestPepperfry_ListProducts`: Verifies card extraction and URL resolution.
- `TestPepperfry_FetchProduct_Sofa`: Checks detailed field parsing, price in paise, and stock status on a live fixture.
- `TestPepperfry_FetchProduct_CoffeeTable`: Checks teak coffee table attributes.
- `TestPepperfry_FetchProduct_DiningChair_OutOfStock`: Checks sold-out status detection.
- `TestPepperfry_Errors`: Verifies proper error bubbling on 500 status and 404 targets.
