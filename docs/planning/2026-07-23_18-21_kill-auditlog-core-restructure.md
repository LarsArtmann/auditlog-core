# Kill auditlog-core: Restructure to Focused Modules

**Date:** 2026-07-23 18:21
**Status:** PLAN — Phase 1-4 executable now, Phase 5 (SSE) deferred

---

## The Problem

`auditlog-core` is a false abstraction — a god-module holding four unrelated packages together under a meaningless name. Three of four packages duplicate existing, better infrastructure elsewhere in the ecosystem:

| Package | What it does | Already exists at | Quality |
|---------|-------------|-------------------|---------|
| root (`WriteToFile`, `CheckNoClobber`) | Atomic file writes | `go-atomic-write` | **Inferior** — no fsync, no fingerprinting, no concurrent modification detection |
| `live/` (Hub, Server, SSE) | SSE fan-out + HTTP dashboard server | `cqrs-htmx` (`fanOut[T]`, `SSEStream`, `Broadcaster`) | **Inferior** — no reconnect, no generic type parameter, no proper SSE protocol |
| `ndjson/` (generic reader) | NDJSON line-delimited JSON reader | Nowhere | **Original** — created this session |
| `loader/` (format detection) | JSON report vs NDJSON detection | Nowhere | **Original** — created this session |

---

## The Decision

**Delete `auditlog-core`.** Replace with focused, honest modules:

| New module | Contains | json/v2? | Consumers |
|-----------|----------|----------|-----------|
| `go-ndjson` | NDJSON reader + format detection | Yes | samber-do, go-workflow |
| `go-sse` (Phase 5) | `fanOut[T]` + `SSEStream` extracted from cqrs-htmx | No | samber-do, go-workflow, cqrs-htmx |
| `go-atomic-write` (already exists) | Atomic file I/O | No | go-workflow |

Consumers end up with precise dependencies instead of one god-module.

---

## Pareto Breakdown

### 1% → 51%: The Architectural Decision

The decision to kill auditlog-core and what replaces it. Once made, the rest is execution.

- `go-ndjson` = ndjson reader + loader (the only original code)
- `go-atomic-write` = file I/O (already exists, better API)
- `go-sse` = SSE infrastructure (Phase 5, extracted from cqrs-htmx)

### 4% → 64%: Extract go-ndjson + Kill Duplicate File I/O

1. Create `go-ndjson` module (ndjson + loader)
2. Add streaming `WriteFunc` to go-atomic-write
3. Switch go-workflow from duplicate `WriteToFile` to go-atomic-write
4. Delete auditlog-core root (dead code after steps 1-3)

### 20% → 80%: SSE Extraction (Phase 5 — Future)

5. Extract `go-sse` from cqrs-htmx (`fanOut[T]`, `SSEStream`, SSE event types)
6. Migrate auditlog-core/live to go-sse
7. Migrate cqrs-htmx to go-sse
8. Delete auditlog-core/live
9. Delete auditlog-core entirely

### Remaining 20% → 100%

10. Documentation (AGENTS.md, FEATURES.md, ROADMAP.md across all repos)
11. Update go.work workspace
12. Full test + lint verification
13. Git commit + push

---

## API Gap Analysis

### go-atomic-write vs auditlog-core WriteToFile

| Feature | go-atomic-write `Write` | auditlog-core `WriteToFile` |
|---------|------------------------|-----------------------------|
| Input | `[]byte` (full content in memory) | `func(io.Writer) error` (streaming callback) |
| fsync | Yes | No |
| Concurrent modification detection | Yes (xxhash64 fingerprint) | No |
| Cross-platform atomic rename | Yes (retry + backoff on Windows) | No (plain os.Rename) |
| File locking | Yes (gofrs/flock) | No |
| Context cancellation | No | Yes |

**Gap:** go-atomic-write lacks the streaming callback API. Consumers need it for large reports/diagrams.

**Solution:** Add `WriteFunc(path string, fn func(io.Writer) error, fingerprint Fingerprint) error` to go-atomic-write. Combines streaming callback with fingerprint verification + fsync.

### cqrs-htmx SSE vs auditlog-core/live

| Feature | cqrs-htmx | auditlog-core/live |
|---------|-----------|-------------------|
| Hub type | `fanOut[T any]` (generic) | `Hub` (concrete, `json.RawMessage` only) |
| SSE protocol | `SSEStream` (headers, heartbeat, LastEventID, OnDisconnect) | `writeSSE` (basic fprintf) |
| Reconnect | Yes (SSEEventID, SSEEventStore, ReplayEvents) | No |
| Pattern | Dispatch hooks (BroadcastOnSuccess/OnError) | Provider injection (WithReportProvider, etc.) |
| Coupling to parent | 3 symbols (AfterDispatchHook, ContentTypeSSE, NewStructuredError) | N/A |

**Phase 5 plan:** Extract `fanOut[T]` + `SSEStream` + `Event` into `go-sse`. Both cqrs-htmx's Broadcaster (dispatch hooks) and auditlog's Server (provider injection) build on top.

---

## Coupling Analysis

### WriteToFile call sites

| Consumer | WriteToFile calls | CheckNoClobber calls | Notes |
|----------|-------------------|---------------------|-------|
| samber-do-auditlog | **0** | **0** | Doesn't use it at all |
| go-workflow-auditlog | **15** | **2** | Has its OWN local duplicate in helpers.go (old signature, no context) |

go-workflow's `helpers.go` WriteToFile is a local copy — does NOT import auditlog-core's version. Switching to go-atomic-write removes the duplicate.

### live/ coupling

Both consumers' live/ packages are **tightly coupled** to auditlog-core/live:
- Hold `*corelive.Server` and `*corelive.Hub` as concrete struct fields
- Use all 5 provider function types (`ReportProvider`, `SnapshotProvider`, etc.)
- 7 wrapper methods are pure delegation
- **Zero coupling to HTTP/SSE internals** — all transport logic is in core

This means Phase 5 (SSE extraction) can swap the underlying implementation without touching consumer provider code — as long as the API surface stays compatible.

---

## Phase Plan

### Phase 1: Extract go-ndjson (30 min) — EXECUTE NOW

Create `/home/lars/projects/go-ndjson/` as independent module.

| Step | Task | Risk | Time |
|------|------|------|------|
| 1.1 | Create go-ndjson module structure | None | 5 min |
| 1.2 | Move ndjson/ files from auditlog-core | None | 5 min |
| 1.3 | Move loader/ as sub-package | None | 5 min |
| 1.4 | Update consumer imports | Low | 5 min |
| 1.5 | Update go.work | None | 2 min |
| 1.6 | Test + lint all projects | Low | 8 min |

### Phase 2: Extend go-atomic-write (20 min) — EXECUTE NOW

| Step | Task | Risk | Time |
|------|------|------|------|
| 2.1 | Add `WriteFunc` streaming variant | Low | 8 min |
| 2.2 | Add tests for WriteFunc | Low | 8 min |
| 2.3 | Test + lint | None | 4 min |

### Phase 3: Kill duplicate WriteToFile in go-workflow (45 min) — EXECUTE NOW

| Step | Task | Risk | Time |
|------|------|------|------|
| 3.1 | Add go-atomic-write dependency to go-workflow | Low | 3 min |
| 3.2 | Switch 15 call sites to `atomicwrite.WriteFunc` | Medium | 15 min |
| 3.3 | Remove helpers.go WriteToFile + CheckNoClobber | Low | 5 min |
| 3.4 | Update viz/viz.go re-export | Low | 5 min |
| 3.5 | Handle CheckNoClobber (inline or remove) | Low | 5 min |
| 3.6 | Test + lint | Medium | 12 min |

### Phase 4: Delete auditlog-core root (15 min) — EXECUTE NOW

| Step | Task | Risk | Time |
|------|------|------|------|
| 4.1 | Remove helpers.go, helpers_test.go | Low | 3 min |
| 4.2 | Remove ErrExportWriteFailed, ErrFileExists sentinels | Low | 2 min |
| 4.3 | Verify nothing breaks | Low | 5 min |
| 4.4 | Test + lint | Low | 5 min |

### Phase 5: SSE Extraction (FUTURE — Plan Only)

| Step | Task | Risk | Time |
|------|------|------|------|
| 5.1 | Create go-sse module | Medium | 30 min |
| 5.2 | Port fanOut[T] from cqrs-htmx | Medium | 20 min |
| 5.3 | Port SSEStream from cqrs-htmx | Medium | 30 min |
| 5.4 | Define Event type (break link to cqrs-lite/transport) | Medium | 15 min |
| 5.5 | Migrate auditlog-core/live on go-sse | High | 45 min |
| 5.6 | Migrate cqrs-htmx on go-sse | High | 60 min |
| 5.7 | Migrate consumer live/ packages | Medium | 45 min |
| 5.8 | Delete auditlog-core/live | Low | 10 min |
| 5.9 | Delete auditlog-core entirely | Low | 10 min |
| 5.10 | Full verification | Medium | 30 min |

### Phase 6: Documentation + Commit (30 min) — EXECUTE NOW

| Step | Task | Risk | Time |
|------|------|------|------|
| 6.1 | Update auditlog-core AGENTS.md | None | 5 min |
| 6.2 | Update consumer AGENTS.md files | None | 5 min |
| 6.3 | Update go.work | None | 2 min |
| 6.4 | Final test + lint sweep | None | 8 min |
| 6.5 | Git commit + push all repos | Low | 10 min |

---

## Mermaid Execution Graph

```mermaid
graph TD
    subgraph "Phase 1: go-ndjson (NOW)"
        A[Create go-ndjson module] --> B[Move ndjson/ + loader/]
        B --> C[Update consumer imports]
        C --> D[Update go.work]
        D --> D1[Test + Lint]
    end

    subgraph "Phase 2: go-atomic-write (NOW)"
        E[Add WriteFunc streaming API] --> F[Add tests]
        F --> F1[Test + Lint]
    end

    subgraph "Phase 3: Kill dup WriteToFile (NOW)"
        G[Add go-atomic-write dep] --> H[Switch 15 call sites]
        H --> I[Remove helpers.go dup]
        I --> J[Handle CheckNoClobber]
        J --> J1[Test + Lint]
    end

    subgraph "Phase 4: Delete dead code (NOW)"
        K[Remove auditlog-core root] --> K1[Test + Lint]
    end

    subgraph "Phase 5: SSE Extraction (FUTURE)"
        L[Create go-sse module] --> M[Port fanOut[T] from cqrs-htmx]
        M --> N[Port SSEStream from cqrs-htmx]
        N --> O[Migrate auditlog-core/live]
        O --> P[Migrate cqrs-htmx]
        P --> Q[Migrate consumer live/ packages]
        Q --> R[Delete auditlog-core/live]
        R --> S[Delete auditlog-core entirely]
        S --> S1[Full verification]
    end

    subgraph "Phase 6: Docs + Commit (NOW)"
        T[Update AGENTS.md files] --> U[Update go.work]
        U --> V[Final test + lint]
        V --> W[Git commit + push]
    end

    D1 --> E
    F1 --> G
    J1 --> K
    K1 --> T

    style A fill:#4a4,color:#fff
    style E fill:#4a4,color:#fff
    style G fill:#4a4,color:#fff
    style K fill:#4a4,color:#fff
    style T fill:#4a4,color:#fff
    style L fill:#a44,color:#fff
    style M fill:#a44,color:#fff
    style N fill:#a44,color:#fff
    style O fill:#a44,color:#fff
    style P fill:#a44,color:#fff
    style Q fill:#a44,color:#fff
    style R fill:#a44,color:#fff
    style S fill:#a44,color:#fff
```

---

## Verschlimmbessern Guardrails

1. **Never break cqrs-htmx** — SSE extraction (Phase 5) is deferred specifically because cqrs-htmx is a large, working codebase
2. **Never break working tests** — every phase ends with full test + lint verification
3. **go-atomic-write.WriteFunc is additive** — existing Write API stays unchanged
4. **go-ndjson is new code** — zero existing consumers to break (only created this session)
5. **Consumer live/ packages untouched** — Phase 5 only; provider injection pattern stays as-is
6. **Each phase is independently revertable** — if any phase breaks, revert just that phase

---

## End State After Phase 1-4

```
/home/lars/projects/
├── go-atomic-write/         # Extended with WriteFunc streaming API
├── go-ndjson/               # NEW — ndjson reader + loader detection
├── auditlog-core/           # Slimmed — only live/ remains (Phase 5 target)
│   ├── live/                # SSE hub + HTTP server (to be replaced by go-sse)
│   ├── go.mod               # Still module root, but root package is empty
│   └── ...
├── cqrs-htmx/               # Unchanged (Phase 5 target)
├── samber-do-auditlog/      # Updated imports (go-ndjson instead of auditlog-core/ndjson)
├── go-workflow-auditlog/    # Updated imports + uses go-atomic-write for file I/O
└── go.work                  # Updated with new modules
```

## End State After Phase 5 (Future)

```
/home/lars/projects/
├── go-atomic-write/         # Atomic file I/O
├── go-ndjson/               # NDJSON reader + format detection
├── go-sse/                  # NEW — SSE infrastructure (fanOut[T], SSEStream)
├── cqrs-htmx/               # Updated — depends on go-sse
├── samber-do-auditlog/      # Depends on go-ndjson + go-sse + go-atomic-write
├── go-workflow-auditlog/    # Same
└── go.work                  # auditlog-core REMOVED
```
