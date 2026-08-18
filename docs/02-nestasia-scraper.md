# Milestone 2: Nestasia Site Scraper

## Overview
This milestone implements the [`site.Site`](file:///C:/Programming/Projects/decor-scrapper/internal/site/site.go) interface for **Nestasia** (`internal/site/nestasia`), scraping high-impact decor pieces directly from Shopify JSON endpoints and normalizing them into canonical [`product.Product`](file:///C:/Programming/Projects/decor-scrapper/internal/product/product.go) entities.

---

## 1. High-Impact Target Categories
Nestasia is configured for six high-impact categories that materially transform room aesthetics:

| URL Slug | Normalized Name | Visual & Spatial Room Impact |
|---|---|---|
| `rugs` | `rugs` | Anchors furniture arrangements, introduces texture/palette |
| `lamps-lighting` | `lighting` | Establishes ambient mood, warm lighting, and accent fixtures |
| `wall-decor` | `artwork_paintings` | Fills vertical space with mirrors, wall art, and wall hangings |
| `curtains` | `curtains` | Frames windows, controls daylight, soft textile layering |
| `planters` | `plants_planters` | Biophilic accents, greenery, and tabletop/floor pots |
| `storage-organizers` | `storage_shelving` | Functional storage, wall shelves, and aesthetic baskets |

---

## 2. Scraper Implementation (`internal/site/nestasia`)

### Discovery (`ListProducts`)
- Queries Shopify's collection API: `/collections/{slug}/products.json?limit=250`
- Maps each item into a lightweight [`product.ProductRef`](file:///C:/Programming/Projects/decor-scrapper/internal/product/product.go) with canonical URL and normalized category name.
- Sends standard browser headers (`User-Agent`, `Accept`, `Sec-Ch-Ua`) for WAF compatibility.

### Ingestion & Normalization (`FetchProduct`)
- Fetches single item details via Storefront AJAX endpoint `/products/{handle}.js` (with automatic fallback to `/products/{handle}.json`).
- **`InStock`**: Retrieves live, accurate variant `available` boolean status emitted by the Storefront endpoint (avoiding silent omission in public JSON endpoints).
- **`PriceMinor`**: Converts price into integer paise (`499900`) with float/int rounding safeguards.
- **`Styles`**: Extracts and normalizes style facets strictly from structured `tags` (supporting `style_<Name>` prefixes like `style_Contemporary`, `style_Modern` and direct synonyms `boho`, `scandi`, `nordic`, `luxe`, `minimal`) and structured `options`, avoiding false-positive free-text guesses.
- **`TagsList`**: Flexible JSON unmarshaler handling both array `["tag1", "tag2"]` and comma-delimited `"tag1, tag2"` Shopify formats.

---

## 3. Verification & Testing

Tested offline against static JSON golden fixtures in [`internal/site/nestasia/testdata/`](file:///C:/Programming/Projects/decor-scrapper/internal/site/nestasia/testdata/) using `net/http/httptest`:
- **Metadata & Category Verification**: Validates correct slugs, normalized names, and site identifier.
- **Collection Listing**: Tests extraction of product references from collection payloads.
- **Product Normalization**: Tests field mapping, price paise conversion, stock status, and style tagging across all 3 categories.
- **Error Handling**: Verifies resilience against HTTP 404/500 errors and malformed JSON payloads.
