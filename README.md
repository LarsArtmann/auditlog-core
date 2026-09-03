# auditlog-core — ARCHIVED

> **This repository is archived and read-only.** It has zero consumers and is
> fully superseded by smaller, actively maintained libraries. Do not depend on it.

## Status

`auditlog-core` was a 2026-07-23 experiment to extract shared infrastructure
(SSE dashboard, NDJSON reading, atomic file writes) from
[`go-workflow-auditlog`](https://github.com/LarsArtmann/go-workflow-auditlog)
and [`samber-do-auditlog`](https://github.com/LarsArtmann/samber-do-auditlog)
into one module. Both consumers adopted it the same day and dropped it hours
later in favor of finer-grained libraries. It was never tagged or published,
so `go get` never worked.

## Where the code lives now

| Was here                      | Successor                                        |
| ----------------------------- | ------------------------------------------------ |
| `live/` (SSE hub + server)    | [`go-sse`](https://github.com/LarsArtmann/go-sse) |
| `ndjson/` (NDJSON reader)     | [`go-ndjson`](https://github.com/LarsArtmann/go-ndjson) |
| `loader/` (format detection)  | [`go-ndjson`](https://github.com/LarsArtmann/go-ndjson) |
| `WriteToFile` / `CheckNoClobber` | [`go-atomic-write`](https://github.com/LarsArtmann/go-atomic-write) |

If you need audit log live dashboards, NDJSON event streams, or atomic file
writes in Go, use those libraries directly.

## License

See [LICENSE](LICENSE) file for details.
