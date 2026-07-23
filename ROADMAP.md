# Roadmap

> Long-term direction and raw ideas. Items here are NOT actionable tasks.
> When an idea is refined into bounded work, it moves to TODO_LIST.md.

## Themes

### 1. Dashboard Completeness

The `live` package provides the SSE transport and HTTP routing, but the actual
dashboard HTML is injected via `DashboardProvider`. Consumer projects
(`go-workflow-auditlog`, `saber-do-auditlog`) each build their own. A shared,
composable dashboard template could reduce duplication across consumers.

Raw ideas:

- Built-in default dashboard HTML with minimal styling (Tailwind or similar)
- Dashboard customisation API (theme, layout, component slots)
- SSE reconnection logic in the browser-side JS (currently consumer-provided)
- Dark mode support in the default dashboard

### 2. Observability and Metrics

The health endpoint reports uptime, client count, events, and dropped count.
Production deployments may need deeper observability.

Raw ideas:

- Prometheus metrics endpoint (`/metrics`) for events/sec, dropped, latency
- Structured logging hook for event lifecycle (subscribe, event, unsubscribe, complete)
- OpenTelemetry tracing integration for SSE connections
- Configurable event buffering strategies (ring buffer, replay log)

### 3. Reliability and Recovery

Events that overflow the 128-slot subscriber buffer are silently dropped.
The snapshot-on-reconnect mechanism recovers state, but there is no event
replay for clients that briefly disconnect.

Raw ideas:

- Event replay from a bounded buffer on reconnect (last N events)
- Configurable per-subscriber buffer size
- Circuit breaker for slow consumers (auto-unsubscribe after N drops)
- Backpressure signaling to event producers

### 4. Developer Experience

The library is easy to wire up, but could be easier to test against and extend.

Raw ideas:

- `httptest` helpers for testing consumer SSE integrations
- Middleware chain for the HTTP server (auth, logging, CORS)
- `go generate` helper for consumer dashboard templates
- Example directory with working consumer integration demos

### 5. Ecosystem Integration

This module is shared infrastructure for audit log libraries. Tighter
integration with the Go ecosystem would increase its value.

Raw ideas:

- `context.Context` propagation through the hub (for request-scoped event metadata)
- Event type registry (typed events via generics, not just `json.RawMessage`)
- Plugin system for event transformers (enrichment, filtering, aggregation)
- WebSocket upgrade path alongside SSE (for bidirectional dashboards)

## Non-goals

Things we are deliberately NOT pursuing and why:

- **Built-in storage/persistence:** This module is transport and UI infrastructure. Event storage belongs in the consumer (`go-workflow-auditlog`, `saber-do-auditlog`).
- **Database dependencies:** Zero external dependencies is a core design constraint. All state is in-memory.
- **Authentication/authorization:** Security is the consumer's responsibility. The server is a library, not a service.
- **Frontend framework coupling:** The dashboard HTML is a string provided by the consumer. No React, Vue, or other framework coupling.
