# Status Report: NDJSON/Loader Extraction to auditlog-core

**Date:** 2026-07-23 16:37
**Session scope:** Extracting NDJSON reader and format-detection loader into `auditlog-core` shared packages using `encoding/json/v2`, migrating both consumers off their duplicated copies.

---

## a) FULLY DONE

### New shared packages in auditlog-core

| Package                | Files                                                   | Status                                                  |
| ---------------------- | ------------------------------------------------------- | ------------------------------------------------------- |
| `auditlog-core/ndjson` | `doc.go`, `reader.go`, `reader_test.go`, `fuzz_test.go` | Done — 12 unit tests + fuzz test, all pass with `-race` |
| `auditlog-core/loader` | `doc.go`, `format.go`, `format_test.go`                 | Done — 7 unit tests, all pass with `-race`              |

**`ndjson.Read[T any]`** — Generic reader using `encoding/json/v2`. Takes a type parameter and optional per-line validation callback. Sentinel errors (`ErrEmpty`, `ErrNoEvents`, `ErrOversizedLine`) are exported for `errors.Is` matching.

**`loader.Detect`** — Format detection by inspecting first non-blank line for `"version"` (JSON report) vs `"event_type"` (NDJSON). Returns `Format` enum (`FormatAuto`, `FormatJSON`, `FormatNDJSON`).

### Consumer migration

| Project              | File        | Change                                                                                                                                                    | Status |
| -------------------- | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| go-workflow-auditlog | `ndjson.go` | 87 lines → 47 lines. Delegates to `ndjson.Read[Event]` with `validateEvent` callback. Re-exports sentinels.                                               | Done   |
| samber-do-auditlog   | `ndjson.go` | 80 lines → 47 lines. Same delegation pattern.                                                                                                             | Done   |
| samber-do-auditlog   | `loader.go` | 175 lines → 118 lines. `Format` is now a type alias for `loader.Format`. `detectFormatFromBytes`/`detectLineFormat` removed, replaced by `loader.Detect`. | Done   |

### Lint config updates

| Project                              | Change                                                                                  |
| ------------------------------------ | --------------------------------------------------------------------------------------- |
| go-workflow-auditlog `.golangci.yml` | Added `github.com/larsartmann/auditlog-core/ndjson` to depguard allow-list              |
| samber-do-auditlog `.golangci.yml`   | Added `github.com/larsartmann/auditlog-core/loader` and `ndjson` to depguard allow-list |

### Documentation

| File                      | Change                                                                                                                                                  |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `auditlog-core/AGENTS.md` | Updated commands to note `GOEXPERIMENT=jsonv2` requirement. Added `ndjson` and `loader` to packages table. Added package description sections for both. |

### Verification

All three projects pass build + test (`-race`) + lint with **0 issues**:

- `auditlog-core` — 4 packages (root, live, ndjson, loader)
- `samber-do-auditlog` — 4 packages (root, cmd/auditlog, live, cmd/genschema/example)
- `go-workflow-auditlog` — root + live sub-module

---

## b) PARTIALLY DONE

### Error message semantics — CHANGED but not fully verified

The shared `ndjson.Read` returns `fmt.Errorf("ndjson line %d: %w", ...)` for unmarshal errors. This is a subtle behavior change:

| Project     | Original prefix    | New prefix         | Breaking?                                                                                      |
| ----------- | ------------------ | ------------------ | ---------------------------------------------------------------------------------------------- |
| go-workflow | `"ndjson line %d"` | `"ndjson line %d"` | No change                                                                                      |
| samber-do   | `"line %d"`        | `"ndjson line %d"` | **Changed** — tests pass because they use `strings.Contains(err, "line 2")` which matches both |

No test failures, but external users parsing error strings would see a different prefix.

### ErrEmpty vs ErrNoEvents semantics — CHANGED for go-workflow

| Input        | go-workflow original       | Shared package                           |
| ------------ | -------------------------- | ---------------------------------------- |
| `""`         | `ErrEmpty` (sawData=false) | `ErrEmpty` (lineNum=0)                   |
| `"\n\n\n"`   | `ErrEmpty` (sawData=false) | **`ErrNoEvents`** (lineNum=3, all blank) |
| valid events | events                     | events                                   |

This is a **behavior change** for go-workflow. Tests pass because no go-workflow test specifically tests `"\n\n\n"` → `ErrEmpty`. samber-do is unaffected because its original logic already matched (`lineNum == 0` → `ErrEmpty`).

### LSP diagnostics — stale throughout session

The LSP showed typecheck errors in `helpers_test.go` (WriteToFile signature) and `live/benchmarks_test.go` (missing http/httptest imports) throughout the entire session. These are stale — `go test` and `golangci-lint run` both pass. The LSP likely lacks `GOEXPERIMENT=jsonv2` configuration. Not investigated or resolved.

---

## c) NOT STARTED

1. **go-workflow loader.go not migrated** — Only has JSON loading (no format detection), so it doesn't need `loader.Detect`. Left as-is intentionally.
2. **Fuzz tests not actually run** — Wrote `FuzzRead` in auditlog-core/ndjson but never ran `go test -fuzz=FuzzRead` to generate corpus entries beyond seeds.
3. **No fuzz test for loader.Detect** — Only unit tests, no fuzz coverage.
4. **DOMAIN_LANGUAGE.md not updated** — Still reflects pre-extraction state. Missing `ndjson.Read`, `loader.Detect`, `Format` enum, `ErrEmpty`/`ErrNoEvents`/`ErrOversizedLine` vocabulary.
5. **ADR-001 not updated** — The extraction decision ADR doesn't mention the ndjson/loader extraction or the json/v2 decision.
6. **go-workflow classify.go** — Still references sentinel errors directly. Works fine (they're re-exported aliases) but could now reference `ndjson.ErrEmpty` directly from the shared package.
7. **NDJSON writer extraction** — Both consumers have duplicate `writeEventsNDJSON` functions (samber-do uses stdlib json, go-workflow uses json/v2 + jsontext). Not extracted. This is a separate concern from the reader.
8. **go-workflow stream.go** — Has `NDJSONStreamer` (streaming writer using jsontext.Encoder). Not considered for extraction.

---

## d) TOTALLY FUCKED UP

### The `sawData` bug — first attempt was wrong

Initial `ndjson.Read` implementation used a `sawData` bool that was set inside the loop body AFTER the blank-line check. For input `"\n\n\n"`:

- `sawData` never set to `true` (all lines blank)
- Returned `ErrEmpty` instead of `ErrNoEvents`
- My own test `TestRead_OnlyBlankLines` caught this immediately

**Fix:** Changed to `lineNum == 0` check (same as samber-do's original logic). This is actually a **better** heuristic than go-workflow's original `sawData` — `lineNum == 0` means truly empty input (scanner never saw any bytes), while `sawData` conflated "no non-blank lines" with "empty input".

**But this introduced the go-workflow behavior change described in section b.**

### Messy edit sequence on reader.go

My first edit removed `sawData` from the variable declarations. My second edit tried to inline `sawData := len(bytes.TrimSpace(line)) > 0` inside the loop — which was both wrong (recomputed every iteration after the blank check already passed) and created a declared-but-read variable. Had to rewrite the entire loop body a third time. Should have rewritten the whole function with `write` instead of incremental `edit` calls.

### Stale LSP diagnostics never investigated

18 LSP warnings persisted throughout the session showing typecheck errors in files that actually compile and test fine. I dismissed them as "stale" without verifying whether the LSP was misconfigured, whether `GOEXPERIMENT=jsonv2` needs to be set for the LSP, or whether there's a real issue being masked. This is lazy — the LSP is a tool, and if it's showing errors, either fix the errors or fix the tool configuration.

---

## e) WHAT WE SHOULD IMPROVE

### Architecture

1. **NDJSON writer duplication remains** — Both consumers still have their own `writeEventsNDJSON`. The reader is shared but the writer is not. This is the next obvious extraction target.
2. **Event type unification** — Each consumer has its own `Event` struct with project-specific fields. The shared `ndjson.Read[T]` handles this via generics, but there's no shared base Event type or interface. If one were defined, more logic could be shared.
3. **json/v2 inconsistency** — auditlog-core's root + live packages use `encoding/json`. The new ndjson + loader packages use `encoding/json/v2`. This means the entire module requires `GOEXPERIMENT=jsonv2` even though only 2 of 4 packages need it. Once json/v2 is stabilized (Go 1.27+?), the whole module should migrate.
4. **go-workflow's loader.go is JSON-only** — It can't detect NDJSON. If a user feeds an NDJSON file to go-workflow's `LoadReport`, it silently fails with a decode error. Using the shared `loader.Detect` would give auto-detection for free.
5. **Format type alias vs re-export** — samber-do uses `type Format = loader.Format` (type alias). This means consumers see `loader.Format` in error messages and godoc, not `auditlog.Format`. Could cause confusion.

### Testing

6. **Fuzz tests are cosmetic** — Wrote fuzz tests but never ran them in fuzzing mode. They only execute with seed corpus during normal `go test`. Need CI integration or manual fuzzing sessions.
7. **No integration test for the shared ndjson → consumer round-trip** — The consumer tests test their own `ReadEvents`, but there's no test that verifies the shared package's error sentinels are truly the same variables (not copies) across the module boundary.
8. **Error message assertions are fragile** — Tests use `strings.Contains` which masks the `"line %d"` → `"ndjson line %d"` prefix change. Should assert exact error format or use a typed error.

### Process

9. **Should have used `write` not `edit`** for the reader.go bugfix — three failed edits vs one clean rewrite.
10. **Should have checked error message compatibility before migrating** — The `"line %d"` vs `"ndjson line %d"` difference should have been caught during design, not discovered during self-review.
11. **Should have run `go test -fuzz` at least briefly** — Not running the fuzz test I just wrote is like buying a fire extinguisher and never pulling the pin.

---

## f) NEXT 50 THINGS TO DO

### High priority — correctness

1. **Decide ErrEmpty vs ErrNoEvents semantics for blank-only input** — go-workflow behavior changed. Either accept the new behavior (document it) or add a special case in the consumer wrapper.
2. **Run `go test -fuzz=FuzzRead -fuzztime=30s` on auditlog-core/ndjson** — Actually exercise the fuzzer.
3. **Add fuzz test for loader.Detect** — Feed arbitrary bytes, verify no panic.
4. **Audit go-workflow tests for ErrEmpty assertions on blank-only input** — The behavior changed; verify no test relies on the old behavior.
5. **Verify error message compatibility in samber-do** — Search for any code parsing error strings that expects `"line %d"` not `"ndjson line %d"`.

### High priority — architecture

6. **Extract NDJSON writer into auditlog-core/ndjson** — Both consumers have `writeEventsNDJSON`. Extract as `ndjson.Write[T](writer, items)`.
7. **Migrate go-workflow loader.go to use loader.Detect** — Give it NDJSON auto-detection capability.
8. **Evaluate NDJSONStreamer extraction** — go-workflow's `stream.go` has a streaming NDJSON writer. Could be shared.
9. **Define shared Event interface** — Allow `EventType`, `Phase`, `IsKnown()` to be shared across consumers without coupling to either's concrete type.

### Medium priority — testing

10. **Add cross-module sentinel identity test** — Verify `errors.Is(consumer.ErrEmpty, ndjson.ErrEmpty)` returns true.
11. **Add exact error format assertions** — Replace `strings.Contains` with exact format checks in consumer tests.
12. **Add edge case tests for unicode in NDJSON lines** — Multi-byte UTF-8 in event fields.
13. **Add test for 1MB boundary** — Line exactly at `MaxLineBytes` should pass; one byte over should fail.
14. **Benchmark ndjson.Read** — No benchmarks yet for the shared reader.
15. **Add property test: Read → Write → Read round-trip** — Verify NDJSON can be written and re-read losslessly.

### Medium priority — documentation

16. **Update DOMAIN_LANGUAGE.md** — Add ndjson.Read, loader.Detect, Format, sentinel errors.
17. **Update ADR-001** — Document the ndjson/loader extraction decision and json/v2 choice.
18. **Write ADR-002 for json/v2 adoption** — Why the shared packages use json/v2, what happens when it stabilizes.
19. **Update FEATURES.md** — Document ndjson and loader as shared infrastructure features.
20. **Update consumer AGENTS.md files** — Document the new ndjson/loader dependency in both consumer projects.
21. **Add package-level examples** — `ExampleRead`, `ExampleDetect` in test files.
22. **Update WORKSPACE.md** — Note GOEXPERIMENT=jsonv2 is now needed for auditlog-core too (not just consumers).

### Medium priority — lint/tooling

23. **Fix LSP GOEXPERIMENT configuration** — LSP shows stale errors. Configure gopls with `GOEXPERIMENT=jsonv2`.
24. **Add `go test -fuzz` to CI** — Run fuzz tests for 30s on each PR.
25. **Add cross-module lint check** — Verify consumers don't define their own NDJSON readers.
26. **Consider `go vet` rules for NDJSON** — Static analysis for common NDJSON mistakes.
27. **Add pre-commit hook for ndjson round-trip test** — Verify Write/Read symmetry.

### Lower priority — polish

28. **Rename `HealthInfo` to `HealthResponse`** — Noted in previous session, still pending.
29. **Make `Subscriber` an interface** — Noted in previous session, still pending.
30. **Add `ndjson.Write[T]` with buffering** — Match WriteToFile's 64KB buffer pattern.
31. **Add `ndjson.ValidateLine[T]`** — Single-line validation helper for CLI input.
32. **Add `loader.DetectFromReader`** — Avoid full buffer read for large files.
33. **Add `loader.DetectFromPath`** — File-path convenience wrapper.
34. **Consider `ndjson.MaxLineBytes` as configurable option** — Some users may want larger lines.
35. **Add `ndjson.NewReader[T]`** — Streaming reader that yields events one at a time.
36. **Add `ndjson.Scan[T](scanner, validate)`** — Work with existing `bufio.Scanner`.
37. **Document sentinel error classification** — Map each sentinel to errorfamily category.

### Consolidation

38. **Consolidate `WriteToFile` in go-workflow** — Still has duplicate `WriteToFile` with old signature (no context.Context). Should delegate to `auditlogcore.WriteToFile`.
39. **Remove go-workflow's `helpers.go` `WriteToFile`** — Replace with `auditlogcore.WriteToFile` call.
40. **Consolidate `CheckNoClobber` in both consumers** — Both have their own; should use auditlogcore.
41. **Unify `ErrExportWriteFailed` / `ErrRenderFailed`** — Consumers have overlapping write-error sentinels.
42. **Consolidate export.go** — Both consumers have export functions that could share more infrastructure.

### go-output relationship

43. **Evaluate go-output JSONL writer for NDJSON export** — go-output supports JSONL output. Could consumers use it instead of their own `writeEventsNDJSON`?
44. **Check if go-output has an input reader** — If not, confirm auditlog-core/ndjson fills the gap.
45. **Document the boundary** — go-output = rendering (output only), auditlog-core/ndjson = parsing (input only).

### Publishing

46. **Resolve sum.golang.org 500 error** — Blocks `go get github.com/larsartmann/auditlog-core`. Needed before consumers can reference published version.
47. **Tag auditlog-core v0.1.0** — After ndjson/loader are stable.
48. **Update consumer go.mod to reference published auditlog-core** — Remove `replace` directives once published.
49. **Add GONOSUMDB workaround documentation** — For local development before publishing.

### Cleanup

50. **Remove go.work from parent directory if it causes issues** — Currently affects all projects under `/home/lars/projects/`, not just the three intended ones.

---

## g) QUESTIONS

### Q1: Should the blank-only-input behavior change stand?

The shared `ndjson.Read` returns `ErrNoEvents` for `"\n\n\n"` (all blank lines). go-workflow's original code returned `ErrEmpty` for this case. Tests pass either way. Should we:

- **(A)** Keep the new behavior (ErrNoEvents for blank-only — arguably more correct since the input wasn't truly "empty")
- **(B)** Add a special case so blank-only input returns ErrEmpty (preserving go-workflow's original behavior)
- **(C)** Merge ErrEmpty and ErrNoEvents into a single sentinel (they're semantically nearly identical)

### Q2: Should go-workflow's loader.go get NDJSON auto-detection?

go-workflow's `LoadReport` is JSON-only — it can't detect or load NDJSON files. samber-do's `LoadReport` has full auto-detection. The shared `loader.Detect` makes this trivial to add. Should I migrate go-workflow's loader to use `loader.Detect` and add NDJSON support, or is JSON-only intentional for go-workflow?

### Q3: Should the NDJSON writer be extracted now?

Both consumers have duplicate `writeEventsNDJSON` functions. The reader extraction is done. Should the writer follow immediately into `auditlog-core/ndjson.Write[T]`, or should we stabilize/test the reader extraction first before adding more surface area?
