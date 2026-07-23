# AGENTS.md — auditlog-core

Zero-dependency shared infrastructure for audit log live dashboards. Used by
`go-workflow-auditlog` and `saber-do-auditlog` as a library dependency.

---

## Commands

```bash
go test ./... -race          # run all tests (root + live/)
go test ./... -count=1        # bypass test cache
golangci-lint run ./...       # lint (now uses .golangci.yml matching sibling standards)
```

No Makefile, no justfile, no flake.nix — pure `go` toolchain.

A `go.work` workspace at the parent directory links this module with both consumer
projects (`samber-do-auditlog`, `go-workflow-auditlog`) for local development.

---

## Architecture

### Packages

| Package               | Purpose                                                                  |
| --------------------- | ------------------------------------------------------------------------ |
| `auditlogcore` (root) | File-write helpers (`WriteToFile`, `CheckNoClobber`) and sentinel errors |
| `live`                | Generic SSE hub + HTTP dashboard server                                  |

### live/ — Real-time Dashboard

The `live` package is the core value of this module. It provides a domain-agnostic
SSE (Server-Sent Events) server that consumer projects wire up with their own
providers.

**Key types:**

- `Hub` — fan-out broadcaster. Maintains subscriber list, broadcasts events,
  signals lifecycle completion. Safe for concurrent use.
- `Subscriber` — represents one SSE client. Owns an event channel (128-buffer)
  and a `Done` channel.
- `Server` — HTTP server wrapping the Hub. Created via `live.New(hub, cfg, ...opts)`.

**Provider pattern (critical):**

The Server has zero domain knowledge. All domain-specific behavior is injected
via functional options:

| Provider            | Purpose                                                                   |
| ------------------- | ------------------------------------------------------------------------- |
| `ReportProvider`    | Returns current report as JSON bytes                                      |
| `SnapshotProvider`  | Returns initial SSE snapshot (report + events + metadata + complete flag) |
| `CompleteProvider`  | Returns final SSE complete payload                                        |
| `DashboardProvider` | Returns the dashboard HTML string                                         |
| `HealthProvider`    | Returns additional health info (events count, dropped count)              |

**HTTP routes** (all under configurable `Config.Prefix`, default `"/"`):

| Route                 | Method | Behavior                                                 |
| --------------------- | ------ | -------------------------------------------------------- |
| `{prefix}/`           | GET    | Dashboard HTML (cached at server creation)               |
| `{prefix}/api/report` | GET    | Point-in-time report JSON                                |
| `{prefix}/api/events` | GET    | SSE stream: snapshot → live events → complete            |
| `{prefix}/api/health` | GET    | Health JSON (uptime, clients, events, dropped, complete) |

**SSE lifecycle:**

1. Client connects to `/api/events`
2. Server sends `snapshot` event (current state)
3. Server streams `event` events as they arrive (via `Hub.OnEvent`)
4. On `Hub.SignalComplete()`, server sends `complete` event and closes connection
5. Slow clients: events are dropped when buffer (128) overflows — clients
   recover via snapshot on reconnect

**Default config values:**

- `Addr: ":0"` (random port)
- `Prefix: "/"`
- `ReadHeaderTimeout: 5s`
- `HeartbeatInterval: 15s`

### root — File Write Helpers

- `WriteToFile(ctx, path, fn)` — atomic write via temp file + rename. Uses 64KB
  `bufio.Writer` buffer. Temp files are cleaned up on error. Context is checked before
  write and before rename for cancellation support.
- `CheckNoClobber(path)` — returns `ErrFileExists` if file exists.
- `ErrExportWriteFailed` — sentinel for all write failures (matchable via `errors.Is`)
- `ErrFileExists` — wraps `ErrExportWriteFailed`

---

## Code Conventions

- **Error handling:** All errors wrapped with `fmt.Errorf("%w: ...")` for
  `errors.Is` compatibility. Sentinel errors defined as package-level vars.
- **nolint directives:** `//nolint:exhaustruct` used where struct initialization
  is intentionally partial (hub, http.Server). Follow this pattern.
- **Test style:** Standard library `testing` only — no external test frameworks.
  All tests call `t.Parallel()`. Test helper functions use `t.Helper()`.
- **Package naming:** `live` sub-package, imported as `live` by consumers.
  Root package imported as `auditlogcore`.
- **No external dependencies** — only stdlib. This is intentional and must be
  preserved.

---

## Gotchas

1. **`Server` implements `http.Handler`** — you can use it directly with
   `httptest.NewServer(server)` without calling `ListenAndServe`. Tests use
   this pattern.

2. **`DashboardHTML` is cached at construction** — `WithDashboardProvider` calls
   `fn()` immediately and stores the result. Changing the provider after
   construction has no effect on served HTML.

3. **`SignalComplete` is one-way** — once called, `IsComplete()` returns true
   forever. There is no reset. New subscribers after completion still get the
   snapshot (with `complete: true`) but no live events.

4. **Non-blocking event delivery** — `Hub.OnEvent` uses `select`/`default` so
   it never blocks the caller. Overflowing events are silently dropped.

5. **`Addr()` returns actual listen address** — important when using `:0`.
   After `ListenAndServe` binds, `Addr()` returns the real port. In tests
   using `httptest.NewServer`, use `ts.URL` instead.

6. **Prefix normalization** — trailing slashes are stripped from `Config.Prefix`.
   Routes are registered as `{prefix}/`, `{prefix}/api/report`, etc.

---

## Consumer Usage Pattern

```go
hub := live.NewHub()
server := live.New(hub, live.Config{Addr: ":8080"},
    live.WithDashboardProvider(func() string { return dashboardHTML }),
    live.WithReportProvider(func() ([]byte, error) { return json.Marshal(report) }),
    live.WithSnapshotProvider(func(isComplete bool) (json.RawMessage, error) {
        // build snapshot JSON with report, events, metadata, complete
    }),
    live.WithCompleteProvider(func() (json.RawMessage, error) {
        // build final report JSON
    }),
)

go server.ListenAndServe()

// In your audit recorder callback:
server.OnEvent(eventJSON)       // broadcast to all SSE clients
server.SignalComplete()          // finalize lifecycle
server.Shutdown(ctx)             // graceful HTTP shutdown
```
