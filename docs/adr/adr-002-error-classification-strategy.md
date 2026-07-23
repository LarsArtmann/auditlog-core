# ADR-002: Error classification strategy — consumer-layer mapping, not library import

Date: 2026-07-23

## Status

Accepted

## Context

[`github.com/larsartmann/go-error-family`](https://github.com/larsartmann/go-error-family)
provides behavioral error classification for Go: every error gets a `Family`
(Rejection, Conflict, Transient, Corruption, Infrastructure) that drives retry
decisions, exit codes, HTTP status codes, and user-facing tone.

auditlog-core defines three sentinel errors:

- `ErrExportWriteFailed` — base for all file/write failures (infrastructure)
- `ErrFileExists` — wraps `ErrExportWriteFailed`; caller requested a no-clobber
  write to an existing path (rejection)
- `live.ErrServerAlreadyRunning` — `ListenAndServe` called on a running server
  (state conflict)

The question: should auditlog-core import go-error-family and attach `Family`
classifications to these sentinels directly?

### Why not import it here

| Factor                             | Detail                                                                                                                                                                                                                                                                                                            |
| ---------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Zero-dep invariant**             | ADR-001 defines zero external dependencies as a load-bearing design constraint for this library. Importing go-error-family breaks it.                                                                                                                                                                             |
| **`GOEXPERIMENT=jsonv2` is viral** | go-error-family's root imports `encoding/json/v2`. Importing it would force every consumer of auditlog-core — `go-workflow-auditlog`, `samber-do-auditlog`, and their transitive dependents — to set `GOEXPERIMENT=jsonv2` at build time. auditlog-core's value is being invisible plumbing; this makes it noisy. |
| **Wrong architectural layer**      | go-error-family's own philosophy is "libraries classify, applications enrich." auditlog-core is a leaf library whose direct consumers are themselves libraries (a samber/do plugin and a workflow engine). Classification compounds at the application boundary, not in shared transport plumbing.                |
| **Over-engineering**               | auditlog-core owns three errors and makes zero behavioral decisions from them: no retry logic, no exit codes, no classified HTTP responses (handler errors are operational and handled inline via `http.Error`). Attaching five Families to three sentinels solves a problem that does not exist at this layer.   |

## Decision

auditlog-core does **not** import go-error-family. The sentinel errors remain
plain stdlib values, matchable with `errors.Is`, wrapped with
`fmt.Errorf("%w: ...")` for context.

Instead, the canonical sentinel → `Family` mapping is published here as a
recipe that any consumer adopting go-error-family can register at its
application boundary.

### Sentinel → Family mapping

| Sentinel                            | Family           | Rationale                                                                                            |
| ----------------------------------- | ---------------- | ---------------------------------------------------------------------------------------------------- |
| `auditlogcore.ErrExportWriteFailed` | `Infrastructure` | System-level failure (disk full, permission, rename); not retryable by the caller.                   |
| `auditlogcore.ErrFileExists`        | `Rejection`      | Caller requested a no-clobber write to an existing path — an impossible operation they must resolve. |
| `live.ErrServerAlreadyRunning`      | `Conflict`       | Conflicting state: `ListenAndServe` invoked twice. The caller must resolve the existing server.      |

### Consumer recipe

go-error-family supports registering third-party sentinels without importing
them from the library that owns them. A consumer does:

```go
import (
    "github.com/larsartmann/auditlog-core"
    "github.com/larsartmann/auditlog-core/live"
    "github.com/larsartmann/go-error-family"
)

func init() {
    errorfamily.RegisterClassification(auditlogcore.ErrExportWriteFailed, errorfamily.Infrastructure)
    errorfamily.RegisterClassification(auditlogcore.ErrFileExists, errorfamily.Rejection)
    errorfamily.RegisterClassification(live.ErrServerAlreadyRunning, errorfamily.Conflict)
}
```

After registration, `errorfamily.Classify(err)`, `errorfamily.IsRetryable(err)`,
and `errorfamily.ExitCode(err)` work transparently on errors returned from
auditlog-core — no wrapping required at the call site.

## Consequences

- The zero-dependency invariant from ADR-001 is preserved.
- `GOEXPERIMENT=jsonv2` is not forced onto auditlog-core consumers.
- Consumers that want behavioral classification opt in at their own boundary,
  using the mapping above; consumers that do not are unaffected.
- This ADR is the single source of truth for the mapping. If auditlog-core adds
  a new sentinel, this table must be updated and consumers should re-register.
- If a future consumer needs richer error context (stack traces, trace IDs), it
  composes go-error-family with `samber/oops` at the application layer, per the
  "libraries classify, applications enrich" split — still no import here.
