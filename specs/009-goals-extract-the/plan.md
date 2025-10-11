# Implementation Plan: WebSocket Routing Framework Extraction

**Branch**: `009-goals-extract-the` | **Date**: 2025-10-11 | **Spec**: [/specs/009-goals-extract-the/spec.md](/specs/009-goals-extract-the/spec.md)
**Input**: Feature specification from `/specs/009-goals-extract-the/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Extract reusable WebSocket parsing and routing infrastructure from `market_data` into a domain-agnostic framework under `/lib/ws-routing`, keep market data specific handlers as adapters over the new API, and ship documentation, tests, and deprecation shims to preserve runtime parity.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.25  
**Primary Dependencies**: `github.com/gorilla/websocket`, `github.com/goccy/go-json`, internal `errs` and `core` packages  
**Storage**: N/A (in-memory streaming only)  
**Testing**: `go test ./... -race`, `make lint-layers`, contract suites under `tests/architecture`  
**Target Platform**: Multi-platform Go modules (Linux/macOS CI runners)  
**Project Type**: Go service libraries (monorepo)  
**Performance Goals**: Maintain existing WebSocket routing latency (P99 < 10ms) and zero-allocation hot paths  
**Constraints**: Preserve existing message formats, enforce `/lib` residency, include one-release deprecation shim, no new runtime dependencies  
**Scale/Scope**: Applies to current market data streams (Binance) with headroom for additional exchanges

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

[Gates determined based on constitution file]
- Confirm the extraction delivers architectural reuse while documenting migration impacts and deprecation shim details before merge.
- Verify reusable infrastructure resides under `/lib` and framework modules avoid imports from domain packages.
- Ensure migration notes and a one-release deprecation shim cover any changed import paths.

**Gate Status**: PASS — all planned work keeps reusable infrastructure in `/lib/ws-routing`, retains market data adapters as domain clients, and schedules migration notes plus a deprecation shim for one release.

## Project Structure

### Documentation (this feature)

```
specs/[###-feature]/
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
├── ws-routing/              # New shared framework (to be created)
│   ├── session.go
│   ├── router.go
│   ├── middleware/
│   └── testdata/

market_data/
├── adapters/
│   ├── websocket/
│   │   ├── parser.go
│   │   └── shim.go          # Temporary re-export
│   └── tests/

tests/
├── contract/
│   └── ws-routing/
└── integration/
    └── market_data/
```

**Structure Decision**: Keep existing Go monorepo layout; introduce `/lib/ws-routing` for reusable infrastructure and update `market_data/adapters/websocket` to consume the framework, with shared tests under `tests/contract/ws-routing`.

## Complexity Tracking

*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
