# Milestone 2: Nestasia Site Scraper

## Overview
This milestone implements the [`site.Site`](file:///C:/Programming/Projects/decor-scrapper/internal/site/site.go) interface for **Nestasia** (`internal/site/nestasia`), scraping high-impact decor pieces directly from Shopify JSON endpoints and normalizing them into canonical [`product.Product`](file:///C:/Programming/Projects/decor-scrapper/internal/product/product.go) entities.

---

## 1. High-Impact Target Categories
Nestasia is configured for three high-impact categories that materially transform room aesthetics:

| URL Slug | Normalized Name | Visual & Spatial Room Impact |
|---|---|---|
| `rugs` | `rugs` | Anchors furniture arrangements, introduces texture/palette |
| `wall-decor` | `wall_decor` | Fills vertical space with mirrors, wall art, and wall hangings |
| `lamps-lighting` | `lighting` | Establishes ambient mood, warm lighting, and accent fixtures |

---

## 2. Scraper Implementation (`internal/site/nestasia`)

### Discovery (`ListProducts`)
- Queries Shopify's collection API: `/collections/{slug}/products.json?limit=250`
- Maps each item into a lightweight [`product.ProductRef`](file:///C:/Programming/Projects/decor-scrapper/internal/product/product.go) with canonical URL and normalized category name.
- Sends standard browser headers (`User-Agent`, `Accept`, `Sec-Ch-Ua`) for WAF compatibility.

### Ingestion & Normalization (`FetchProduct`)
- Fetches single item details via `/products/{handle}.json`.
- **`PriceMinor`**: Converts rupees string (`"4999.00"`) into integer paise (`499900`) with rounding safeguards.
- **`InStock`**: Evaluates variant inventory availability.
- **`Styles`**: Extracts and normalizes style facets against our canonical taxonomy:
  `["bohemian", "contemporary", "glam", "minimalist", "modern", "scandinavian"]`
- **`TagsList`**: Flexible JSON unmarshaler handling both array `["tag1", "tag2"]` and comma-delimited `"tag1, tag2"` Shopify formats.

---

## 3. Verification & Testing

Tested offline against static JSON golden fixtures in [`internal/site/nestasia/testdata/`](file:///C:/Programming/Projects/decor-scrapper/internal/site/nestasia/testdata/) using `net/http/httptest`:
- **Metadata & Category Verification**: Validates correct slugs, normalized names, and site identifier.
- **Collection Listing**: Tests extraction of product references from collection payloads.
- **Product Normalization**: Tests field mapping, price paise conversion, stock status, and style tagging across all 3 categories.
- **Error Handling**: Verifies resilience against HTTP 404/500 errors and malformed JSON payloads.
