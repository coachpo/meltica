# Implementation Plan: Extract Market Data Routing into Universal Library

**Branch**: `010-extract-the-market` | **Date**: 2025-10-11 | **Spec**: [/specs/010-extract-the-market/spec.md](/specs/010-extract-the-market/spec.md)
**Input**: Feature specification from `/specs/010-extract-the-market/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Extract the market data routing runtime into a domain-agnostic library rooted at `/lib/ws-routing`, decomposing it into session, connection, router, handler, middleware, telemetry, and API packages that consume an abstract transport. Remove the `market_data/framework` copies and shim, update imports/tests/tooling across the repo, and preserve routing benchmarks, ordering, and backpressure semantics.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.25  
**Primary Dependencies**: Go stdlib packages, `github.com/gorilla/websocket`, `github.com/goccy/go-json`, internal `core/stream`, `errs`, and `/lib` observability helpers  
**Storage**: N/A (in-memory streaming only)  
**Testing**: `go test ./...`, `go test -tags=integration ./tests/integration/...`, benchmarks under `market_data` and planned `/lib/ws-routing`  
**Target Platform**: Linux/macOS build targets (Go toolchain)  
**Project Type**: Go monorepo with shared `/lib` libraries and domain packages (e.g., `/market_data`)  
**Performance Goals**: Preserve routing dispatch median < 500ns and P99 < 2µs with zero allocations (PERF-07)  
**Constraints**: Maintain layered architecture boundaries (CQ-02, ARCH-01..05), domain-agnostic exports, per-symbol ordering with cross-symbol concurrency, upstream backpressure, structured logging baseline  
**Scale/Scope**: Updates span `/lib/ws-routing`, removal of `market_data/framework`, adapters, integration tests, benchmarks, Makefile/CI tooling, and module metadata

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

[Gates determined based on constitution file]
- The spec explicitly authorizes breaking import path changes (GOV-06, GOV-08); plan must pair them with updated migration notes in `BREAKING_CHANGES_v2.md` (GOV-07).
- `/lib/ws-routing` must remain business-agnostic and forbid imports from `/market_data/**` or other domain packages (ARCH-01, ARCH-03, ARCH-05).
- Routing hot-path benchmarks and exhaustive boundary tests are required to satisfy PERF-07 and TS-10; plan must preserve `market_data/events_benchmark_test.go` coverage or relocate it to the new library.
- Structured logging must rely on `/lib` observability interfaces without vendor SDKs (CQ-09, UX-04/05 compliance).
- Coverage and race-detector expectations remain unchanged (TS-01, TS-03); plan must ensure `go test ./...` and tagged integration suites remain green post-migration.

## Project Structure

### Documentation (this feature)

```
specs/010-extract-the-market/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```
lib/
└── ws-routing/              # Target library location (new multi-package layout)
    ├── api/
    ├── connection/
    ├── handler/
    ├── middleware/
    ├── router/
    ├── session/
    └── telemetry/

market_data/
├── framework/               # Current routing implementation slated for removal
│   ├── api/
│   ├── connection/
│   ├── handler/
│   ├── router/
│   └── telemetry/
├── processors/
└── events_benchmark_test.go

tests/
├── integration/
│   └── market_data/
└── market_data/
```

**Structure Decision**: Go monorepo with shared `/lib` libraries and domain modules; this feature relocates routing infrastructure from `market_data/framework` into `/lib/ws-routing` while updating tests/benchmarks and integration harnesses accordingly.

## Complexity Tracking

*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
