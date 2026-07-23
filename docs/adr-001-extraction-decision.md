# ADR-001: Extract auditlog-core as shared infrastructure

Date: 2026-07-23

## Status

Accepted

## Context

Both `samber-do-auditlog` and `go-workflow-auditlog` independently implement the same
real-time dashboard infrastructure:

- SSE hub (fan-out broadcaster with subscriber management)
- HTTP server (dashboard, report, events, health endpoints)
- Atomic file-write helpers (temp file + rename for crash safety)
- SSE lifecycle management (snapshot, live events, completion)

This duplication caused:

1. **Divergence** — bug fixes and improvements in one project didn't propagate to the other
2. **Maintenance overhead** — ~600 LOC duplicated across two repos
3. **Inconsistency** — different test patterns, error handling, and API ergonomics
4. **Stalled innovation** — improvements like `context.Context` support, benchmarks, and
   race-detector coverage required changes in two places

## Decision

Extract the shared infrastructure into `github.com/larsartmann/auditlog-core`, a
zero-dependency library with two packages:

- **Root package** (`auditlogcore`): atomic file-write helpers and sentinel errors
- **`live` sub-package**: domain-agnostic SSE hub + HTTP server with provider injection

### Key design choices

1. **Zero external dependencies** — only Go standard library. This is intentional and
   must be preserved. The library is infrastructure, not application code.

2. **Provider pattern over domain coupling** — The Server has zero domain knowledge.
   All domain-specific behavior (report structure, event types, dashboard HTML) is
   injected via functional options (`WithReportProvider`, `WithSnapshotProvider`, etc.).
   This allows any consumer to use the same transport layer.

3. **`go.work` workspace over `replace` directives** — All three repos are linked via
   a `go.work` file at the parent directory. This eliminates fragile `replace` directives
   in `go.mod` files and enables lockstep development.

4. **Non-blocking event delivery** — `Hub.OnEvent` uses `select`/`default` so it never
   blocks the caller. Overflowing events are silently dropped. Subscribers recover via
   snapshot on reconnect. This trades reliability for latency, which is the right
   tradeoff for real-time dashboards.

## Consequences

- Consumers must wrap the core types with domain-specific logic (thin wrappers)
- The `replace` directives remain in `go.mod` until `auditlog-core` is published with
  a stable tag (`v0.1.0`)
- Adding a third consumer requires only wrapping the core types, not reimplementing
  the transport layer
- The `WriteToFile` function signature now takes `context.Context` as its first parameter
  for cancellation support
