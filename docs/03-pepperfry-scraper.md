# Milestone 3: Pepperfry Site Scraper

## Overview
This milestone implements the [`site.Site`](file:///C:/Programming/Projects/decor-scrapper/internal/site/site.go) interface for **Pepperfry** ([`internal/site/pepperfry`](file:///C:/Programming/Projects/decor-scrapper/internal/site/pepperfry/pepperfry.go)), India's premier online furniture retailer, parsing server-rendered HTML pages with `goquery` and normalizing them into canonical [`product.Product`](file:///C:/Programming/Projects/decor-scrapper/internal/product/product.go) entities.

---

## 1. Core Furniture & Fixture Categories
Pepperfry supplies the foundational furniture and large fixture layer for the room:

| Category Slug | Normalized Name | Spatial Role |
|---|---|---|
| `category/3-seater-sofas` | `sofas` | Primary living room seating anchor |
| `category/coffee-tables` | `coffee_tables` | Central living room surface & accent |
| `category/dining-chairs` | `dining_chairs` | Dining seating and accent chairs |
| `category/queen-size-beds` | `beds` | Bedroom anchor and focal piece |
| `category/bedside-tables` | `side_tables` | Bedside and sofa accent tables |
| `category/book-shelves` | `storage_shelving` | Open shelving and room organization |
| `category/lamps-lighting` | `lighting` | Floor and table lamps for ambient mood |

---

## 2. Implementation Details (`internal/site/pepperfry`)

### Discovery (`ListProducts`)
- Queries active leaf category URLs: `https://www.pepperfry.com/{slug}.html` (e.g. `category/3-seater-sofas.html`).
- Uses `goquery` to parse product card anchors (`a.product-link`, `.product-card a`, `a[href*="/product/"]`).
- Emits deduplicated, absolute-resolved [`product.ProductRef`](file:///C:/Programming/Projects/decor-scrapper/internal/product/product.go) items.
- Sends full desktop browser request headers (`User-Agent`, `Sec-Ch-Ua`, `Accept`, `Accept-Language`) for WAF compatibility.

### Parsing & Normalization (`FetchProduct`)
- Fetches and parses product detail pages with `goquery`.
- **`ExternalID`**: Multi-tiered extraction strategy (Retailer item ID meta tag -> Container `data-product-id` attribute -> Regex fallback from URL path).
- **`PriceMinor`**: Multi-strategy price extraction:
  1. *Schema.org JSON-LD*: Parses `<script type="application/ld+json">` for `Product` entity `offers.price`.
  2. *Meta & Microdata*: Checks `meta[property='product:price:amount']`, `meta[itemprop='price']`, `meta[property='og:price:amount']`, and `meta[name='twitter:data1']`.
  3. *DOM Selectors*: Matches `.v-product__price--current`, `.product-price`, `.v-product__price`, `.offer-price`, and `[data-price]`.
  4. *Embedded Script JSON*: RegEx fallback across inline script tags for `"offer_price"`, `"selling_price"`, etc.
  5. *Paise Conversion*: Converts numeric string into integer paise (`5049900` for ₹50,499.00) with float-rounding safety.
- **`ImageURL`**: Sequenced image resolution:
  1. *Schema.org JSON-LD*: Prioritizes `<script type="application/ld+json">` `Product` image first (stable SEO contract).
  2. *DOM Gallery*: Inspects lazy-loading and gallery tags (`img.v-product-gallery__image`, `img.vipImage`, `data-src`, `data-img`, `data-zoom-image`).
  3. *Strict Filtering*: Explicitly rejects site assets, SVG, icons, placeholders, and logos (`w22-pf-logo.svg`, `/assets/`, `.svg`). Returns `""` (empty string) if no genuine photo is present rather than displaying a false logo.
- **`InStock`**: Evaluates Schema.org JSON-LD `offers.availability` (`InStock` vs. `OutOfStock`) and DOM out-of-stock badges (`.v-product__out-of-stock-msg`, `.out-of-stock`, `.sold-out`).
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
- `TestPepperfry_ImageExtraction_ExcludesLogo`: Verifies that site logos and assets are never returned.
- `TestPepperfry_ImageExtraction_JSONLDAndLazyLoad`: Verifies JSON-LD graph extraction and gallery image fallback.

