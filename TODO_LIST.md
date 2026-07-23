# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.

## Status legend

| Status        | Meaning                                                     |
| ------------- | ----------------------------------------------------------- |
| `TODO`        | Not started. Needs doing.                                   |
| `IN_PROGRESS` | Actively being worked on.                                   |
| `BLOCKED`     | Cannot proceed, external dependency or decision needed.     |
| `DONE`        | Completed. Remove from this list and log in `CHANGELOG.md`. |

## High Impact

| Task                                | Status | Impact | Effort | Evidence                                                          |
| ----------------------------------- | ------ | ------ | ------ | ----------------------------------------------------------------- |
| Add golangci-lint config            | `TODO` | High   | 30min  | No `.golangci.yml` exists; CONTRIBUTING.md references the command |
| Set up CI pipeline (GitHub Actions) | `TODO` | High   | 1h     | No `.github/workflows/` directory exists                          |

## Medium Impact

| Task                                | Status | Impact | Effort | Evidence                                                              |
| ----------------------------------- | ------ | ------ | ------ | --------------------------------------------------------------------- |
| Add benchmark tests for Hub.OnEvent | `TODO` | Med    | 30min  | Non-blocking fan-out is performance-critical; no benchmarks exist     |
| Add `.gitignore` for Go project     | `TODO` | Med    | 10min  | `.gitignore` exists but content not verified for Go-specific patterns |
| Verify `go vet ./...` passes        | `TODO` | Med    | 10min  | No explicit vet step documented or verified                           |

## Low Impact

| Task                                    | Status | Impact | Effort | Evidence                                                              |
| --------------------------------------- | ------ | ------ | ------ | --------------------------------------------------------------------- |
| Add `docs/DOMAIN_LANGUAGE.md`           | `TODO` | Low    | 30min  | Optional for library; would define "subscriber", "hub", "snapshot"    |
| Add `go generate` for `go.sum` lockfile | `TODO` | Low    | 10min  | Module has zero deps; go.sum may not be needed but should be verified |
| Tag first release as `v0.1.0`           | `TODO` | Low    | 5min   | No git tags exist; initial commit is tagged in changelog but not git  |

---

<!-- Guidance for the builder:
  - Source of truth is the CODE. Verify each item before adding, many
    documented TODOs are already done.
  - One task per row. If it takes more than ~2 hours, split it into smaller
    tasks.
  - Cite evidence (file:line) so the next person can verify without re-deriving.
  - DONE items should be REMOVED, not kept. Use CHANGELOG.md for history.
  - If a task is vague ("improve X"), refine it into concrete steps or move
    it to ROADMAP.md.
  - Deduplicate by semantic intent, not by text match.
-->
