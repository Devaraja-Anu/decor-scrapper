> [!NOTE]
> **Context Handoff / Active Milestone Specification**
> This file specifies the immediate next milestone to be implemented in the project. Any AI agent or developer reading this repository in a fresh context window should use this specification, along with [decisions.md](file:///C:/Programming/Projects/decor-scrapper/decisions.md) and [schema.md](file:///C:/Programming/Projects/decor-scrapper/schema.md), as the exact blueprint and verification checklist for Milestone 5.

---

# Milestone 5: CLI Entry Point & End-to-End Orchestration (`cmd/scraper`)

## 1. Overview & Objectives
Implement the standalone CLI entry point (`cmd/scraper/main.go` and/or root `main.go`) to wire all components together:
- `internal/site/nestasia`
- `internal/site/pepperfry`
- `internal/pipeline`
- `internal/store`

Provides a clean command-line interface with flags, graceful OS signal handling (`Ctrl+C`), progress logging, and catalog persistence.

---

## 2. CLI Flags & Configuration

| Flag | Type | Default | Description |
|---|---|---|---|
| `-out` | string | `"catalog.json"` | Path to the output catalog JSON file |
| `-workers` | int | `4` | Number of concurrent fetch workers |
| `-rate` | float64 | `3.0` | Max requests per second per target site |
| `-site` | string | `""` | Optional site filter (`"nestasia"`, `"pepperfry"`, or empty for all) |
| `-timeout` | duration | `10m` | Overall execution timeout before context cancellation |

---

## 3. Implementation Requirements (`cmd/scraper/main.go`)

1. **Signal Handling**:
   - Uses `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` to ensure graceful cancellation on user interrupt.
2. **Site Wiring**:
   - Instantiates `nestasia.New()` and `pepperfry.New()`.
   - Filters sites if `-site` flag is specified.
3. **Pipeline Execution**:
   - Instantiates `store.NewJSONFileStore(outputPath)`.
   - Configures `pipeline.Config` with workers, rate limits, and an `OnFetchError` logging callback.
   - Executes `pipeline.Run(ctx)`.
4. **Progress & Output Reporting**:
   - Logs start of category listing phase.
   - Logs item discovery and fetch progress.
   - Reports total products merged, file size, and execution duration.

---

## 4. Verification & Testing Strategy

1. **CLI Compilation**:
   - `go build -o scraper.exe ./cmd/scraper` (or `go build ./...`)
2. **Flag Verification**:
   - Test `-help` flag output.
   - Test invalid flag handling.
3. **End-to-End Test Execution**:
   - Verify all tests in `go test ./...` continue to pass.
   - Verify `catalog.json` output structure matches `schema.md`.
