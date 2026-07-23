// Package live provides a generic real-time HTTP dashboard server using
// Server-Sent Events (SSE). It is the shared infrastructure behind the
// live dashboards in go-workflow-auditlog and samber-do-auditlog.
//
// The server does not depend on any specific auditlog implementation.
// Instead, it accepts callbacks for event ingestion and report generation,
// making it reusable across different audit log domains.
//
// # Architecture
//
// The live server uses SSE for real-time communication:
//
//   - GET /            - Interactive dashboard HTML (static, cached)
//   - GET /api/report  - Current report as JSON (point-in-time snapshot)
//   - GET /api/events  - SSE stream (snapshot + live events + completion)
//   - GET /api/health  - Health check
//
// # Quick Start
//
//	hub := live.NewHub()
//	server := live.New(hub, reportProvider, live.Config{Addr: ":8080"})
//	go server.ListenAndServe()
package live
