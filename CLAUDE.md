# meltica Development Guidelines

Auto-generated from all feature plans. Last updated: 2025-10-10

## Active Technologies
- Go 1.25 (003-we-ve-built)
- Go 1.25 + gorilla/websocket, goccy/go-json, golang.org/x/tools (for static analysis) (008-architecture-requirements-req)
- N/A (in-memory state management only) (008-architecture-requirements-req)
- Go 1.25 + `github.com/gorilla/websocket`, `github.com/goccy/go-json`, internal `errs` and `core` packages (009-goals-extract-the)
- N/A (in-memory streaming only) (009-goals-extract-the)

## Project Structure
```
src/
tests/
```

## Commands
# Add commands for Go 1.25

## Code Style
Go 1.25: Follow standard conventions

## Recent Changes
- 009-goals-extract-the: Added Go 1.25 + `github.com/gorilla/websocket`, `github.com/goccy/go-json`, internal `errs` and `core` packages
- 008-architecture-requirements-req: Added Go 1.25 + gorilla/websocket, goccy/go-json, golang.org/x/tools (for static analysis)
- 003-we-ve-built: Added Go 1.25

<!-- MANUAL ADDITIONS START -->
## Architecture Notes
- Four-layer contracts live under `core/layers/` (`connection.go`, `routing.go`, `business.go`, `filter.go`).
- Static analyzer (`internal/linter`) and contract tests (`tests/architecture`) enforce layer boundaries.
- Exchange scaffolding resides in `internal/templates/exchange` with the `scripts/new-exchange.sh` helper.
<!-- MANUAL ADDITIONS END -->
