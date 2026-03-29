# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Building

## Backend (Go code)

Run `go build -tags sqlite_fts5` from within `cmd/aurelius`.

## Frontend (TypeScript/JavaScript)

- `npm install` - Install dependencies
- `npm run build` - Build production assets (compiled to `cmd/aurelius/assets/static/js/`)
- `npm run build-debug` - Build debug assets without minification
- `npm run test` - Run TypeScript tests
- `npm run coverage` - Generate test coverage report in `coverage/lcov-report/index.html`. This requires first running the `debug` backend described in `.vscode/launch.json`. ALWAYS make sure that the backend is actually running before generating the report.

## Full Development Build

```bash
npm install
npm run gobuild
npm run build
```

## Testing

- Go lint: `golangci-lint run`
- Go tests: `npm run gotest`
- Update baselines for Go tests (**ONLY IF** baselines are expected to change): `UPDATE_BASELINES=1 go test -tags sqlite_fts5 -asan ./...`
- TypeScript tests: `npm run test`

# Architecture

Aurelius is a web-based streaming music player with a hybrid Go backend and TypeScript frontend architecture.

## Backend (Go)

- **Main application**: `cmd/aurelius/main.go` - HTTP server with Gorilla Mux router
- **Media library**: `internal/media/` - Core media management, HTTP API for browsing/streaming
- **Media database**: `internal/mediadb/` - SQLite database for persisting track metadata, directory structure, and attached image info. Includes a scanner that walks the filesystem, diffs against the DB, detects moves via partial file hashing, and applies changes transactionally. Uses version-based migrations via `PRAGMA user_version`. Public `DB` methods accept joined library paths (e.g. `"dir/file.mp3"`) rather than split `(dir, name)` pairs, because the `media` package — the primary consumer — works with joined paths.
  - **Watcher/scanner parity**: The filesystem watcher (real-time) and full scanner (startup) must detect the same set of changes. Both use `ChangeSet` and `ScanResult` as shared data structures, and the same `Apply` method processes changes from either source. When adding detection for a new kind of change, ensure both paths handle it — changes missed by the watcher are lost until restart, and changes missed by the scanner are lost across restarts.
- **Audio processing**: `pkg/aurelib/` - FFmpeg wrapper for audio decoding/encoding with CGO bindings
- **Fragment support**: `pkg/fragment/` - Subsection playback of tracks

### Key Backend Components

- **Authentication**: Session-based login with configurable passphrase
- **Media streaming**: FFmpeg-based transcoding with throttling and ReplayGain support
- **File serving**: Custom file server that rejects directory requests
- **Favorites**: M3U playlist storage in configured storage directory

## Frontend (TypeScript)

- **Entry points**: `ts/main.ts` (main app), `ts/login.ts` (login page)
- **Player core**: `ts/core/player.ts` - Main audio player with playlist management
- **UI components**: `ts/ui/` - Player controls, directory browsing, settings
- **Build system**: Rollup with TypeScript plugin, outputs to `cmd/aurelius/assets/static/js/`

### Key Frontend Components

- **Player**: Event-driven audio player with ReplayGain, random play, history tracking
- **Directory browser**: File system navigation with playlist support
- **Settings**: Stream configuration (codec, bitrate, ReplayGain mode)
- **Modal dialogs**: Settings and other overlays

## Configuration

- **Command-line flags**: Defined in `main.go` with iniflags support
- **Media library**: Configurable root path, storage path, URL prefix
- **Streaming**: Optional throttling, ReplayGain support, transcoding options

## Development Environment

- Uses VS Code Remote Containers (`.devcontainer/`)
- Docker support with multiple Dockerfiles for different platforms
- Native development possible with npm, Go, FFmpeg, and pkg-config
- **Browser testing**: The `playwright-cli` Claude skill is installed for headless browser interaction. Use it to navigate to the running app, take screenshots, click elements, type text, and evaluate JS — no custom tooling needed. Start the backend first, then use the skill to verify UI changes visually. Run with `npx playwright-cli`. This **MUST** be run from the source tree root to pick up the config file.

## Implementation Plans

`docs/plans/` contains past implementation plans for reference.

When in planning mode, after the user has finished giving feedback and approved the final implementation plan, write the finalized plan to `docs/plans/` **BEFORE** starting implementation. Use the naming convention `YYYY-MM-DD-<short-descriptor>.md`.

# Code Style

- Go doc comments must end with a period.

# Commit Guidelines

- Commit messages should follow best practices.
- **DO NOT** include implementation details unless requested. Focus on the behavioral and interface changes.
- Commit messages **MUST** additionally include a whimsical but poignant closing haiku at the end of the message, capturing the spirit of the change. The haiku must be placed at the end of the commit message, but before any attribution to Claude or Claude Code. **DO NOT** include any non-ASCII characters in the haiku. The first line **MUST** have 5 syllables; the second line **MUST** have 7 syllables; the third line **MUST** have 5 syllables.
- A commit containing a new SQL migration **MUST** update the combined schema SQL using the `sqlite3` command. After running the Go tests, dump the output of `sqlite3 test/storage/aurelius.db .schema` into `internal/mediadb/schema.sql`.
- **DO NOT** commit unless explicitly asked to by the user.

# important-instruction-reminders
Do what has been asked; nothing more, nothing less.
NEVER create files unless they're absolutely necessary for achieving your goal.
ALWAYS prefer editing an existing file to creating a new one.
NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.
