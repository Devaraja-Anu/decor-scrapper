# Decor Scraper 🛋️🖼️

A standalone, concurrent Go scraping pipeline designed to extract structured furniture and decor catalog data from Indian retailers (**Pepperfry** and **Nestasia**). The resulting canonical `catalog.json` dataset feeds an AI-powered room interior redesign web application.

---

## 🎯 Architecture & Design Highlights

- **Complete Decoupling**: The scraper's only contract with the downstream Next.js application is a stable, versioned [`catalog.json`](schema.md) file.
- **Heterogeneous Extraction**:
  - **Nestasia (Decor)**: Shopify JSON endpoints (`/collections/{slug}/products.json`) with automated style facet extraction (`bohemian`, `contemporary`, `glam`, `minimalist`, `modern`, `scandinavian`).
  - **Pepperfry (Furniture)**: Server-rendered HTML parsing via `goquery` with deliberate separation of style inference to downstream LLMs.
- **Idiomatic Concurrency (`internal/pipeline`)**:
  - Worker pool pattern with goroutines and bounded channels.
  - Per-domain token bucket rate limiting (`golang.org/x/time/rate`).
  - Context cancellation and timeout propagation (`context.Context`).
  - Graceful OS interrupt handling (`SIGINT`/`SIGTERM`).
- **Atomic & Idempotent Persistence (`internal/store`)**:
  - Stable composite keying (`source:external_id`) ensures rerun idempotency.
  - Temporary file write + atomic rename prevents file corruption during mid-write interruptions.
  - Deterministic sorting by `StableKey` ensures clean git diffs.

---

## 📂 Project Structure

```
decor-scrapper/
├── cmd/
│   └── scraper/
│       └── main.go               # CLI entry point
├── docs/                         # Milestone documentation
│   ├── 01-domain-and-storage.md
│   ├── 02-nestasia-scraper.md
│   ├── 03-pepperfry-scraper.md
│   ├── 04-pipeline-concurrency.md
│   └── 05-cli-and-e2e.md
├── internal/
│   ├── pipeline/                 # Worker pool, rate limiting, orchestration
│   ├── product/                  # Canonical Product domain model & schema
│   ├── site/                     # Site interface & retailer implementations
│   │   ├── nestasia/             # Shopify JSON scraper + offline fixtures
│   │   └── pepperfry/            # HTML scraper (goquery) + offline fixtures
│   └── store/                    # Atomic JSON persistence & merge engine
├── decisions.md                  # Architectural decisions & rationale log
├── schema.md                     # Catalog JSON specification
├── main.go                       # Root executable wrapper
├── go.mod
└── README.md
```

---

## 🚀 Quick Start

### Prerequisites
- Go 1.22+ installed.

### Run the Scraper
```bash
# Run full scraper for all configured sites
go run .

# Or run via cmd/scraper
go run ./cmd/scraper

# Scrape a specific retailer
go run . -site nestasia
go run . -site pepperfry

# Custom concurrency, rate limiting, and output path
go run . -workers 8 -rate 5.0 -out ./catalog.json -timeout 15m
```

### CLI Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-out` | string | `"catalog.json"` | Destination path for output catalog JSON file |
| `-workers` | int | `4` | Number of concurrent fetch workers |
| `-rate` | float64 | `3.0` | Max requests per second per target site |
| `-site` | string | `""` | Filter to specific site (`"nestasia"`, `"pepperfry"`, or empty for all) |
| `-timeout` | duration | `10m` | Overall execution timeout before context cancellation |

---

## 🧪 Testing

All tests run completely offline using golden-file fixtures and `net/http/httptest.Server`:

```bash
# Run all test suites across the repository
go test -v ./...
```

---

## 📖 Documentation & Architecture

1. [Milestone 1: Domain Model & Storage Layer](docs/01-domain-and-storage.md)
2. [Milestone 2: Nestasia Site Scraper](docs/02-nestasia-scraper.md)
3. [Milestone 3: Pepperfry Site Scraper](docs/03-pepperfry-scraper.md)
4. [Milestone 4: Concurrency Pipeline & Rate Limiting](docs/04-pipeline-concurrency.md)
5. [Milestone 5: CLI Entry Point & End-to-End Orchestration](docs/05-cli-and-e2e.md)
6. [Programming Concepts & Architecture Guide](docs/concepts.md) 🧠

