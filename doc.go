// Package auditlogcore provides shared infrastructure for audit log libraries.
//
// It contains the generic SSE (Server-Sent Events) hub and HTTP server that
// power the live dashboards in both go-workflow-auditlog and samber-do-auditlog.
// The core module has zero domain dependencies — it communicates via callbacks
// and json.RawMessage, letting each auditlog implementation provide its own
// Event type and report serialization.
package auditlogcore
