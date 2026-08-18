# decisions.md

Running log of architectural and product decisions for the furniture-scraper project, with rationale. Update this file whenever a non-trivial decision is made or revisited — the "why" matters more than the "what."

---

## Project purpose & scope

**Decision:** Standalone Go scraper project, fully decoupled from the eventual "AI interior design" app.

**Rationale:** Two goals are being served at once — (1) a portfolio artifact demonstrating real engineering judgment on scraping/ETL, and (2) a first hands-on project for learning Go concurrency (goroutines, channels, worker pools, context, rate limiting), coming from a REST-API background where this is new ground. Keeping it decoupled from the design app means neither project can break the other, and the scraper can be evaluated/rerun/tested entirely on its own.

**Contract with the downstream app:** the scraper's only obligation to the (not-yet-started) design app is the data it writes to storage — a stable schema, nothing more. The design app doesn't know or care how the data got there.

---

## Site selection

**Decision:** Scrape **Pepperfry** (furniture) and **Nestasia** (decor/homeware), both India-based.

**Rejected:** Article.com, Lulu and Georgia (US-based) — switched to India-based sites because the end product targets Indian users, and US retailers largely don't ship large furniture internationally. Using US sites would have made the "budget + deliverable to your spot" constraint fake.

**Rejected: Amazon.** Explicitly disallowed — `robots.txt` disallows general crawlers site-wide except whitelisted bots (Googlebot etc.), and Amazon's Conditions of Use explicitly prohibit data mining/scraping tools. Also has aggressive automated anti-bot enforcement (IP bans, AWS WAF CAPTCHA challenges). Not worth the engineering time fighting anti-bot infrastructure for data that isn't differentiated (everyone's product data is on Amazon). Amazon has an official Product Advertising API as the sanctioned path if ever needed — not pursued for this project.

**Why Pepperfry + Nestasia specifically, and why they pair well:**
- Pepperfry: India's largest online furniture retailer. Custom platform, server-rendered HTML with clean/consistent URL structure (`/category/{slug}.html`). Skews toward big furniture (budget-defining items).
- Nestasia: India home decor/lifestyle brand, built on **Shopify** (confirmed via `cdn.shopify.com` assets and `/collections/`, `/products/` URL conventions). Skews toward decor/accent pieces (cheaper "finishing" layer). Explicitly markets across styles matching our taxonomy (Contemporary, Scandinavian, Bohemian, Minimalist, Eclectic), and — critically — exposes this as **structured per-product facet data** (Style, Colour, Material, Shape, Pattern) on collection pages. Likely exposes a `/collections/{slug}/products.json` endpoint (standard Shopify feature, not independently verified live — confirm via `curl` before building against it) which would require zero HTML parsing for that site.
- The furniture/decor split maps naturally onto how a real room budget actually splits — not just picked for site diversity.

**Category scope for v1:** 3 categories per site. Deliberately small — get the full pipeline working end-to-end first; expanding categories later is just more config, not new code.

---

## Scraping approach

**Decision:** Scrape once (or on an occasional manual rerun), cache into our own DB. Never live-scrape per user request.

**Rationale:** Lower risk profile than continuous/live scraping (not hammering the target site repeatedly), and the app stays fast since it queries our own DB, not the retailer's site, at request time.

**Acknowledged, not resolved by tooling:** this is still technically against most retailers' ToS regardless of frequency — scrape-once is a materially lower-risk shortcut for a small, non-commercial portfolio project, not a ToS-compliant approach. If this ever goes beyond portfolio scope (public-facing, monetized), swap to a real API/affiliate feed (Etsy Open API, affiliate networks) — noted as a "next steps" item, not built now.

---

## Language: Go only

**Decision:** Entire scraper (fetching, parsing, orchestration, storage) written in Go. No Python, no hybrid split.

**Considered and rejected: Python (Scrapy).** Honestly, if the sole objective were "most robust scraper, least effort," Python/Scrapy wins — it provides retry logic, per-domain throttling, robots.txt compliance, and item pipelines out of the box, all of which have to be hand-built in Go. Rejected anyway because:
- The objective here isn't minimum effort — it's demonstrating engineering judgment (portfolio) and learning concurrency fundamentals hands-on (explicit personal goal), and hand-building these primitives is the point, not overhead to avoid.
- Only 2 known sites, not arbitrary/unknown ones — Go's weaker general-purpose scraping ecosystem (vs. Python's) matters far less at this scale.
- One site (Nestasia, if Shopify JSON confirmed) needs no HTML parsing at all — just `net/http` + `encoding/json`. The other (Pepperfry) needs real parsing, and `goquery` is sufficient for one well-understood site.

**Considered and rejected: hybrid (Python parses, Go orchestrates).** Two runtimes, cross-language boundary, added deployable — real integration cost not justified by a parsing problem small enough for Go to handle alone.

**Noted for later reference, not chosen:** `Colly` (github.com/gocolly/colly) exists as a more batteries-included Go scraping framework (built-in parallelism/rate limiting). Deliberately not used — hand-building the worker pool and rate limiter is the explicit learning goal for this project.

---

## Architecture: `Site` interface

**Decision:** Orchestration (`pipeline`) depends only on a `site.Site` interface, never on concrete site packages directly. Concrete implementations live in `internal/site/pepperfry` and `internal/site/nestasia`; only `main.go` knows they exist.

```go
type Site interface {
    Name() string
    Categories() []Category
    ListProducts(ctx context.Context, cat Category) ([]ProductRef, error)
    FetchProduct(ctx context.Context, ref ProductRef) (product.Product, error)
}
```

**Rationale:** This is the seam that lets a third site be added later as an isolated change (new subpackage + one line in `main.go`), without touching the pipeline.

**Why `ListProducts`/`FetchProduct` are split into two methods** rather than one `FetchProducts(category) ([]Product, error)`: gives the pipeline a uniform, product-level unit of work to parallelize across a worker pool. A single combined method would only allow parallelism at the category level, which is a less interesting (and less applicable, given only 3 categories/site) concurrency problem.

**Asymmetry resolved (v2 update):** In earlier iterations, `FetchProduct` made a redundant second request since `products.json` had basic details. However, Shopify's public `/products/{handle}.json` endpoint omits the variant `available` boolean entirely. By directing `FetchProduct` to the Storefront AJAX endpoint `/products/{handle}.js`, this second call now serves a critical purpose: fetching live, accurate stock availability per variant. The asymmetry resolved itself naturally.

---

## `Product` schema

```go
type Product struct {
    Source     string    // "pepperfry", "nestasia"
    ExternalID string
    URL        string
    Name       string
    PriceMinor int64     // paise, not rupees — avoids float rounding bugs
    Currency   string    // "INR" — explicit even though currently always INR
    Category   string    // normalized category, not the site's raw label
    Styles     []string  // see style-tagging decision below
    ImageURL   string
    InStock    bool
    ScrapedAt  time.Time
}

func (p Product) StableKey() string {
    return p.Source + ":" + p.ExternalID
}
```

**`PriceMinor` as `int64` (paise) rather than `float64` (rupees):** avoids float rounding corruption accumulating over repeated writes. Cheap to get right from the start, painful to retrofit.

**`StableKey()` (Source + ExternalID):** storage does upsert-by-key, not blind insert, so reruns don't duplicate rows. Idempotency built in from day one rather than retrofitted.

---

## Pepperfry image extraction (resolved)

**Decision:** Image extraction prioritizes `<script type="application/ld+json">` Schema.org `Product` image first. If missing, it falls back to DOM gallery nodes (`img.v-product-gallery__image`, `img.vipImage`, `data-src`, `data-img`, `data-zoom-image`).

**Strict exclusion:** Any URL matching static site assets, logos, placeholders, or icons (e.g. `w22-pf-logo.svg`, `/assets/`, `.svg`) is explicitly rejected. If no valid product image URL is found, the parser returns `""` (empty string) rather than returning a site logo. A missing image is far better than an incorrect image rendered in the downstream design app.

---

## Style tagging & Facet Extraction (resolved)

**Decision:** 
- **Nestasia:** Extracted strictly from structured `tags` (supporting `style_<StyleName>` prefixes like `style_Contemporary`, `style_Modern`, `style_Bohemian` plus direct synonyms like `scandi`, `nordic`, `boho`, `luxe`, `minimal`) and structured `options`.
- **Pepperfry:** Ships with `Styles: []string{}` — style inference for Pepperfry items is explicitly pushed to the downstream design app's LLM layer, not solved in the scraper.

**Rejected: Free-text matching on `title` or `body_html` for Nestasia:** Product descriptions frequently contain compatibility marketing claims (e.g., "pairs well with modern living rooms"). Matching free text would introduce false positives, violating the core principle that a wrong signal is worse than an empty one. Extraction is strictly confined to structured merchant-set fields.

**Rejected: hand-mapped category→style static table for Pepperfry** (e.g. `"sofas" → ["scandinavian"]`). Initially considered viable, but rejected on reflection: category is orthogonal to style — a "sofas" category contains scandi, industrial, and boho sofas mixed together. A blanket category→style mapping wouldn't be coarse-but-honest data, it would be **actively wrong data** — tagging an industrial sofa as Scandinavian is worse than tagging nothing, since downstream filtering logic would trust and act on a false signal. A false signal is worse than an absent one.

**Rejected: scraping Pepperfry's brand/collection pages for style signal** (e.g. the "Bohemiana" brand, which is boho-styled throughout). This is real, honest, site-sourced signal — but coverage is inconsistent (only products belonging to a style-coherent branded collection would get tagged; most of the catalog wouldn't), and pursuing it means crawling a 4th page-type with undocumented/unverified coverage across Pepperfry's other brands (Woodsworth, Casacraft, Amberville, Mintwud, etc.). Real scope creep against a weekend-sprint timeline. Out of scope for v1; could be revisited later as an explicit enhancement, not a default.

**Note left in code:** the `Styles: []string{}` on Pepperfry products is commented in-source explaining this is a deliberate decision, not an oversight, so it doesn't read as a bug later.

---

## Storage — SUPERSEDED, see "Storage format (v2)" below

**Original decision (kept for history, no longer in effect):** `store.Store` interface, with `store/sqlite` as the first (and for v1, only) concrete implementation.

**Original rationale:** Small upfront cost, real payoff if storage ever needs to change (e.g. Postgres, or a different access pattern for the design app) — `pipeline` never needs to change, only the wiring in `main.go`.

**Why superseded:** once the actual consumer (a Next.js design app) was known, a flat JSON file read natively by Next.js (no DB client, no connection string, no external service account) turned out to be a better fit than any DB option, including SQLite. This also retroactively makes the entire "free hosting + SQLite persistence" investigation (Render/Fly.io/Railway free-tier limits, Turso, Neon, Supabase) moot for this project — kept below for the record, since it was real reasoning done in good faith, not deleted.

---

## Storage format (v2 — current)

**Decision:** Scraper writes a single `catalog.json` file (array of `Product` objects) instead of a database. See `schema.md` for the exact structure.

**Rationale:** The design app is Next.js, which can read a JSON file natively (`fetch` or `fs.readFileSync`/`import`) with zero DB client, connection string, or external service account. For a project already carrying real external dependencies (LLM API calls, image handling), removing an entire dependency category on the storage side is a genuine simplification, not a downgrade — this became clear once the actual consumer of the data was known, whereas the original SQLite decision was made without that context.

**What does NOT change from the original storage reasoning, just because the format did:**
- **Idempotency is still the scraper's job, not free anymore.** SQLite would have given upsert-by-key (`UNIQUE(source, external_id)` + `ON CONFLICT`) for free. With JSON, the scraper must load the existing `catalog.json` (if present) before a rerun, merge new results in keyed by `Product.StableKey()`, and write the merged set back out. This logic lives in `store/jsonfile`, replacing `store/sqlite` as the concrete implementation behind `store.Store`. Method shape leans toward "load full set, merge, write full set" rather than SQLite's natural per-row upsert.
- **Atomic writes are still required.** A crash mid-write must not corrupt or truncate `catalog.json`. Pattern: write to a temp file, then atomic rename over the real file. This was a known risk of the JSON approach even back when SQLite was being compared against it — it doesn't go away just because JSON won out for a different reason (consumer fit, not because this risk was resolved).

## Hand-off mechanism: scraper → design app

**Decision:** Scraper repo is **public on GitHub**. Next.js app fetches `catalog.json` via its raw GitHub URL (`raw.githubusercontent.com/...`) at **build time, with periodic revalidation** (Next.js `revalidate` / ISR-style refetch), not on every request.

**Assumption flagged, not yet independently confirmed:** this requires the scraper repo to stay public — `raw.githubusercontent.com` serves public repos without auth; a private repo would need a token passed with every fetch (secret management in the Next.js app, more moving parts for no real benefit here). Confirm the repo is intended to stay public before relying on this.

**Why build-time + periodic revalidation over request-time fetch:** freshness isn't actually valuable here — the catalog only changes when the scraper is manually rerun, not continuously — so there's no upside to fetching on every request, only downside: a live dependency on GitHub responding at the moment a user/interviewer loads the page. Build-time (with revalidation so a full redeploy isn't needed to pick up a new scrape) keeps a GitHub outage from being able to break a page load for someone viewing the deployed app.

**Failure mode, decided deliberately:** if the fetch fails (build time or on a scheduled revalidation), **fail loudly** — let the build/revalidation fail and be noticed, rather than silently falling back to a last-known-good JSON committed into the Next.js repo. A visible broken build is more debuggable than a silently stale or wrong catalog; a stale-fallback mechanism is more engineering than this project's failure modes justify.

**Decoupling preserved:** the scraper doesn't know or care who reads `catalog.json`; the Next.js app doesn't know or care how it was produced, only that it's a stable, versioned URL. This keeps both projects independently buildable/testable, consistent with the standing "one project shouldn't interfere with another" principle.

---

## Testing strategy

**Decision:** Golden-file / fixture-based tests for site parsers. Real HTML/JSON responses saved once to `testdata/{site}/`, parser tests run against those static files — no live network calls in tests.

**Known limitation, accepted:** fixtures go stale. If a target site changes its markup, tests will keep passing against the old fixture while the live scraper silently breaks. This is a known tradeoff of golden-file testing, not a flaw specific to this project — mitigated by periodically refreshing fixtures, not by avoiding the approach.

---

## Concurrency

**Decision:** All concurrency logic (worker pool, channels, rate limiting, context cancellation) confined to a single `internal/pipeline` package. Rate limiting (`golang.org/x/time/rate`, per-domain token bucket) and `context.Context`-based timeouts/cancellation are **built in from the start**, not added incrementally — given the weekend-sprint timeline and infrequent check-ins, retrofitting these later under time pressure is riskier than baking them in up front.

**Rationale for confining concurrency to one package:** given concurrency is new ground (coming from a REST-API background), keeping it in one place makes it easier to reason about correctness, rather than scattering partial-concurrency logic across the codebase.

---

## Explicitly deferred / out of scope for v1

- **Distributed architecture** (job queue + multiple independent worker processes/nodes). Considered as a learning exercise for distributed systems, but deliberately deferred: concurrency fundamentals (single-process goroutines/channels) should be solid before adding distributed coordination on top, or debugging becomes ambiguous (goroutine bug vs. distributed coordination bug). For 2 sites' data volume, a distributed architecture is also genuine over-engineering relative to actual need — if pursued later, should be a consciously separate, labeled exercise (e.g. a new branch), not a v1 requirement. May also be better served by a purpose-built toy problem (distributed queue, Raft, etc.) rather than bolted onto this scraper.
- **Generic/config-driven scraping framework** (plugin architecture, auto-discovery of new sites). Two concrete site implementations behind one interface is sufficient for v1; abstracting further now would be guessing at what varies, not knowing it. Revisit only if/when a third site is added and a real pattern emerges.
- **Bounding-box / room-placement metadata per product** (e.g. floor-standing vs. wall-mounted vs. tabletop) — relevant to the eventual design app's "swap item" / hotspot UI, but neither Pepperfry nor Nestasia will ever provide this; it would have to be inferred or hand-tagged downstream. Noted so it's a conscious future gap, not a surprise later.
