# Milestone 5: CLI Entry Point & End-to-End Orchestration

## Overview
This milestone integrates all scraping components into a unified command-line application located at [`main.go`](file:///C:/Programming/Projects/decor-scrapper/main.go) and [`cmd/scraper/main.go`](file:///C:/Programming/Projects/decor-scrapper/cmd/scraper/main.go).

---

## 1. CLI Usage & Flags

```bash
# Run full scraper for all configured sites (Nestasia & Pepperfry)
go run .

# Run with custom concurrency and output location
go run . -workers 8 -rate 5.0 -out ./data/catalog.json

# Scrape only a single target site
go run . -site nestasia
go run . -site pepperfry

# Inspect options
go run . -help
```

### Supported Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-out` | string | `"catalog.json"` | Destination path for output catalog JSON file |
| `-workers` | int | `4` | Number of concurrent fetch workers |
| `-rate` | float64 | `3.0` | Max requests per second per target site |
| `-site` | string | `""` | Filter to specific site (`"nestasia"`, `"pepperfry"`, or empty for all) |
| `-timeout` | duration | `10m` | Overall scrape execution timeout before cancellation |

---

## 2. Integrated Architecture

```
                 [CLI Flag Parsing & Configuration]
                                │
                                ▼
                       [Site Initialization]
                 ├── Nestasia (Shopify JSON)
                 └── Pepperfry (HTML with goquery)
                                │
                                ▼
                   [Pipeline Concurrency Engine]
                 ├── Discovery (Category listings)
                 ├── Worker Pool (Parallel ingestion)
                 └── Per-domain Rate Limiting
                                │
                                ▼
                      [Store Persistence]
                 └── Atomic merge to catalog.json
```

---

## 3. End-to-End Verification

- **Graceful Shutdown**: Listens for `os.Interrupt` and `syscall.SIGTERM` signals, ensuring workers and atomic writes terminate cleanly without corrupting the catalog file.
- **Test Suite Verification**: Complete unit test coverage across all domain, site, store, and pipeline packages:
  ```bash
  go test -v ./...
  ```
- **Schema Compliance**: Emits `catalog.json` adhering strictly to [`schema.md`](file:///C:/Programming/Projects/decor-scrapper/schema.md).
