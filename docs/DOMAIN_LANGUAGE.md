# Domain Language — auditlog-core

> Ubiquitous language for the auditlog-core shared infrastructure module.

## Core Concepts

### Hub

Fan-out broadcaster. Maintains a list of subscribers and broadcasts events to all of them.
Thread-safe. Events are delivered non-blocking: if a subscriber's buffer is full, the event
is silently dropped for that subscriber.

### Subscriber

Represents one connected SSE client. Owns a buffered event channel (128 events) and a
`Done` channel that signals lifecycle completion. Subscribers recover missed events by
disconnecting and reconnecting, which triggers a fresh snapshot.

### Server

HTTP server wrapping a Hub. Serves the dashboard HTML, report JSON, SSE event stream,
and health endpoint. Created via `live.New(hub, config, ...options)`. Has zero domain
knowledge — all domain behavior is injected via functional options (providers).

### Snapshot

The initial SSE payload sent to a client on connect. Contains the current report, past
events, metadata, and completion status. Enables clients to recover state after
reconnection without missing events.

### Complete

A lifecycle signal. Once `SignalComplete()` is called, the Hub marks the lifecycle as
finished. All connected SSE clients receive the complete payload. New subscribers after
completion still receive the snapshot (with `complete: true`) but no live events.

## Provider Pattern

The Server has no domain knowledge. All domain-specific behavior is injected:

| Provider          | Responsibility                                             |
| ----------------- | ---------------------------------------------------------- |
| ReportProvider    | Returns current report as JSON bytes                       |
| SnapshotProvider  | Returns initial SSE snapshot (report + events + metadata)  |
| CompleteProvider  | Returns final SSE complete payload                         |
| DashboardProvider | Returns the dashboard HTML string (cached at construction) |
| HealthProvider    | Returns additional health info (events count, dropped)     |

## SSE Lifecycle

1. Client connects to `/api/events`
2. Server sends `snapshot` event (current state)
3. Server streams `event` events as they arrive (via `Hub.OnEvent`)
4. On `Hub.SignalComplete()`, server sends `complete` event and closes connection
5. Slow clients: events dropped when buffer overflows; recover via snapshot on reconnect

## Atomic File Writes

`WriteToFile` provides crash-safe file writes: data is written to a temporary file, then
atomically renamed to the final path. A crash during write leaves the previous file intact.
`CheckNoClobber` guards against accidental overwrites.

## Consumers

- **samber-do-auditlog**: DI audit log plugin for `samber/do`
- **go-workflow-auditlog**: Workflow audit log for `Azure/go-workflow`
