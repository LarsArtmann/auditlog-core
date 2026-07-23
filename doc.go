// Package auditlogcore provides shared infrastructure for audit log libraries.
//
// The root package is intentionally empty — file I/O helpers have been moved to
// [github.com/larsartmann/go-atomic-write] and NDJSON reading/format detection
// to [github.com/larsartmann/go-ndjson]. The live/ sub-package remains for SSE
// infrastructure and is targeted for extraction to a future go-sse module.
package auditlogcore
