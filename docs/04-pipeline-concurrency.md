# Milestone 4: Concurrency Pipeline & Rate Limiting

## Overview
This milestone implements the central concurrency engine in [`internal/pipeline`](file:///C:/Programming/Projects/decor-scrapper/internal/pipeline/pipeline.go). All concurrency primitives (goroutines, worker pools, channels, context propagation, and per-domain token bucket rate limiting) are unified into this single package per [decisions.md](file:///C:/Programming/Projects/decor-scrapper/decisions.md).

---

## 1. Concurrency Architecture

```
[Sites: Pepperfry & Nestasia]
       │
       ▼
 [Phase 1: Discovery] ────► [jobs channel]
                                │
   ┌────────────────────────────┼────────────────────────────┐
   │                            │                            │
   ▼                            ▼                            ▼
[Worker 1]                  [Worker 2]                  [Worker N]
(Rate Limiter Wait)         (Rate Limiter Wait)         (Rate Limiter Wait)
(FetchProduct)              (FetchProduct)              (FetchProduct)
   │                            │                            │
   └────────────────────────────┼────────────────────────────┘
                                │
                                ▼
                       [results channel]
                                │
                                ▼
                     [Phase 3: Collector]
                                │
                                ▼
                   [store.JSONFileStore] (Save / Merge)
                                │
                                ▼
                         [catalog.json]
```

---

## 2. Implementation Components (`internal/pipeline`)

- **Worker Pool**: Configurable number of concurrent workers consuming discrete `workItem` references.
- **Bounded Channels**: Non-blocking channel buffering matching work queue sizes to eliminate deadlock risks.
- **Per-Domain Rate Limiting (`golang.org/x/time/rate`)**: Independent token bucket rate limiters per target domain, throttling requests without starving other sites.
- **Context & Graceful Cancellation**: Clean teardown on `context.Canceled` or `context.DeadlineExceeded` with zero goroutine leaks.
- **Partial Failure Resilience**: Product fetch errors trigger an optional `OnFetchError` callback without halting other worker goroutines.
- **Atomic Store Persistence**: Invokes [`store.Store.Save`](file:///C:/Programming/Projects/decor-scrapper/internal/store/store.go) to merge and write catalog updates.

---

## 3. Verification & Testing
Tested in [`internal/pipeline/pipeline_test.go`](file:///C:/Programming/Projects/decor-scrapper/internal/pipeline/pipeline_test.go):
- **`TestPipeline_EndToEndWithStore`**: Multi-site discovery and ingestion with real `JSONFileStore` atomic writes.
- **`TestPipeline_Concurrency`**: Proves parallel worker pool speedup over sequential execution.
- **`TestPipeline_ContextCancellation`**: Verifies prompt and clean termination when context is cancelled mid-run.
- **`TestPipeline_ErrorResilience`**: Verifies individual item failures do not crash the pipeline and healthy items are saved.
- **`TestPipeline_EmptyListing`**: Tests graceful handling of empty category listings.
