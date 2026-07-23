# auditlog-core

> Zero-dependency shared infrastructure for audit log live dashboards in Go.

## Why

Audit log libraries (`go-workflow-auditlog`, `saber-do-auditlog`) each need
real-time SSE dashboards with snapshot recovery, heartbeat keepalive, and
lifecycle management. Without a shared core, this logic is duplicated and
diverges. `auditlog-core` extracts the transport layer (SSE hub, HTTP server,
atomic file writes) into a domain-agnostic library with zero external
dependencies.

## Installation

```bash
go get github.com/larsartmann/auditlog-core
```

Requires Go 1.26+. No external dependencies.

## Quick start

```go
package main

import (
    "log"

    "github.com/larsartmann/auditlog-core/live"
)

func main() {
    hub := live.NewHub()

    server := live.New(hub, live.Config{Addr: ":8080"},
        live.WithDashboardProvider(func() string {
            return `<html><body><h1>Live Dashboard</h1></body></html>`
        }),
        live.WithReportProvider(func() ([]byte, error) {
            return []byte(`{"status":"running"}`), nil
        }),
        live.WithSnapshotProvider(func(isComplete bool) (json.RawMessage, error) {
            return json.Marshal(map[string]any{
                "report":   json.RawMessage(`{"status":"running"}`),
                "events":   []any{},
                "complete": isComplete,
            })
        }),
    )

    log.Fatal(server.ListenAndServe())
}
```

## Usage

### Broadcasting events

```go
// Push an event to all connected SSE clients
server.OnEvent(json.RawMessage(`{"type":"step_completed","step":"validate"}`))

// Signal lifecycle completion (sends final report, closes connections)
server.SignalComplete()
```

### Atomic file exports

```go
import auditlogcore "github.com/larsartmann/auditlog-core"

// Guard against accidental overwrites
if err := auditlogcore.CheckNoClobber(path); err != nil {
    return err
}

// Atomic write: temp file + rename (crash-safe)
err := auditlogcore.WriteToFile(path, func(w io.Writer) error {
    _, err := w.Write(reportJSON)
    return err
})
```

### Server endpoints

| Endpoint              | Method | Description                                    |
| --------------------- | ------ | ---------------------------------------------- |
| `{prefix}/`           | GET    | Dashboard HTML (cached at server creation)     |
| `{prefix}/api/report` | GET    | Current report as JSON                         |
| `{prefix}/api/events` | GET    | SSE stream: snapshot, live events, completion  |
| `{prefix}/api/health` | GET    | Health JSON (uptime, clients, events, dropped) |

## License

See [LICENSE](LICENSE) file for details.
