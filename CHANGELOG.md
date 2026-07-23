# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `AGENTS.md` with architecture context for AI sessions
- `FEATURES.md` feature inventory
- `TODO_LIST.md` actionable work items
- `ROADMAP.md` long-term direction
- `CONTRIBUTING.md` with development setup instructions

### Fixed

- Unchecked `resp.Body.Close()` return values in `live/server_test.go` (errcheck warnings)

## [0.1.0] - 2026-07-23

### Added

- `WriteToFile` — atomic file writes via temp file + rename with 64KB bufio buffer (`helpers.go:52`)
- `CheckNoClobber` — no-clobber guard returning `ErrFileExists` (`helpers.go:32`)
- `ErrExportWriteFailed` and `ErrFileExists` sentinel errors (`helpers.go:16-22`)
- `Hub` — concurrent-safe SSE fan-out broadcaster with non-blocking event delivery (`live/hub.go:42`)
- `Subscriber` — per-client SSE connection with buffered event channel (`live/hub.go:14`)
- `Hub.SignalComplete` — lifecycle completion signaling to all subscribers (`live/hub.go:115`)
- `Server` — HTTP server implementing `http.Handler` with configurable prefix, heartbeat, and read timeout (`live/server.go:69`)
- Dashboard HTML endpoint (`GET {prefix}/`)
- Report JSON endpoint (`GET {prefix}/api/report`)
- SSE events endpoint with snapshot, live events, and completion (`GET {prefix}/api/events`)
- Health check endpoint with uptime, client count, and completion status (`GET {prefix}/api/health`)
- Functional options pattern for server configuration (`WithReportProvider`, `WithSnapshotProvider`, `WithCompleteProvider`, `WithDashboardProvider`, `WithHealthProvider`)
- Graceful shutdown via `Server.Shutdown(ctx)`
- Full test suite: 23 tests across root and `live/` packages
