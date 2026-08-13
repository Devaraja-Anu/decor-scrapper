# schema.md

Data model for `catalog.json`, the artifact the scraper produces and the Next.js design app consumes. Pure structure/format here — see `decisions.md` for the reasoning behind choices (JSON over DB, style-tagging asymmetry, hand-off mechanism, etc).

---

## File: `catalog.json`

Top-level shape: a single JSON array of Product objects. No wrapper object, no pagination — the whole catalog in one file, matching the project's actual scale (2 sites × 3 categories, expected low hundreds of products).

```json
[
  {
    "source": "pepperfry",
    "external_id": "FR2004SH4XNTIN-2545615",
    "url": "https://www.pepperfry.com/product/some-sofa-2545615.html",
    "name": "Example 3-Seater Sofa",
    "price_minor": 2499900,
    "currency": "INR",
    "category": "sofas",
    "styles": [],
    "image_url": "https://cdn.pepperfry.com/...",
    "in_stock": true,
    "scraped_at": "2026-08-13T10:15:00Z"
  },
  {
    "source": "nestasia",
    "external_id": "8127456789012",
    "url": "https://nestasia.in/products/example-vase",
    "name": "Example Ceramic Vase",
    "price_minor": 149900,
    "currency": "INR",
    "category": "vases",
    "styles": ["bohemian", "minimalist"],
    "image_url": "https://cdn.shopify.com/...",
    "in_stock": true,
    "scraped_at": "2026-08-13T10:16:32Z"
  }
]
```

## Field reference

| Field         | Type       | Notes |
|---------------|-----------|-------|
| `source`      | string    | `"pepperfry"` \| `"nestasia"` — matches `Site.Name()` in the scraper. New sites append new values here, nothing else changes shape. |
| `external_id` | string    | Site's own product ID where available; derived deterministically from the URL if the site doesn't expose one. Combined with `source`, forms the stable identity used for upsert-by-key on rerun (`StableKey()` = `source + ":" + external_id`). |
| `url`         | string    | Canonical product page URL. Also the crawl target (`ProductRef.URL` in the scraper). |
| `name`        | string    | Product display name, as scraped. |
| `price_minor` | integer   | **Paise, not rupees.** Integer to avoid float rounding drift across repeated scrape/merge cycles. Divide by 100 for display. |
| `currency`    | string    | Always `"INR"` currently — kept explicit rather than assumed, in case a future site prices differently. |
| `category`    | string    | Normalized category name (the scraper's own taxonomy — e.g. `"sofas"`, `"vases"` — not the site's raw label). One of the 3 curated categories per site for v1. |
| `styles`      | string[]  | **Populated for Nestasia** (sourced from the site's own structured style facets: Bohemian, Contemporary, Glam, Minimalist, Modern, Scandinavian — lowercased/normalized here). **Empty array for Pepperfry** — Pepperfry has no reliable per-product style signal (see `decisions.md` for why category→style mapping was rejected as actively misleading, not just coarse). Style inference for Pepperfry products is deferred to the design app's LLM layer, not solved here. Consumers must not assume every product has non-empty `styles`. |
| `image_url`   | string    | Primary product image, direct URL. |
| `in_stock`    | boolean   | As reported by the source at scrape time. Not re-verified between scrapes. |
| `scraped_at`  | string    | ISO 8601 UTC timestamp of when this specific product was last successfully fetched/merged. Lets consumers (or you, debugging) see catalog staleness per-item, not just file-level. |

## Invariants the scraper must uphold

- **Uniqueness:** no two entries share the same `source` + `external_id` pair. Enforced by the scraper's load-merge-write cycle in `store/jsonfile` (see `decisions.md` → Storage format v2), not by the file format itself — the JSON schema has no way to enforce this on its own, so it's entirely on the write path to get right.
- **Reruns are merges, not replacements.** A product missing from a given scrape run (e.g. a transient fetch failure) should *not* be dropped from `catalog.json` — it should retain its last-known data until a future successful fetch updates it. Only genuinely delisted products (explicit handling TBD — not yet decided) would be removed.
- **Atomic file writes.** The full array is written to a temp file and atomically renamed over `catalog.json` — partial/corrupt writes must not be observable by a reader (including the Next.js build fetching mid-write, however unlikely given the file is fetched from a committed GitHub state, not live during a write).
- **Consumers must treat `styles` as optionally empty**, not as a guaranteed non-empty field — this is a real, permanent asymmetry between sources, not a temporary gap to be back-filled later by the scraper itself.

## Deliberately not in this schema (v1)

- Bounding-box / room-placement metadata (e.g. floor-standing vs. wall-mounted vs. tabletop) — flagged in `decisions.md` as relevant to the design app's later "swap item" / hotspot UI, but out of scope; neither source site provides it, and it would need separate inference/tagging downstream.
- Any per-product "delisted" or soft-delete marker — open question noted above, not yet resolved. Revisit once a real rerun surfaces an actual delisted product and forces the decision.
