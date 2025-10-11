# Implementation Plan: Four-Layer Architecture Implementation

**Branch**: `008-architecture-requirements-req` | **Date**: 2025-01-20 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/008-architecture-requirements-req/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Formalize the existing four-layer architecture (Connection, Routing, Business, Filter) by:
1. Defining clear interface contracts for each layer
2. Implementing static analysis tools to enforce layer boundaries
3. Supporting type-specific interfaces for different exchange categories (spot, futures, options)
4. Providing migration path for legacy code to adopt new layer interfaces incrementally
5. Establishing cross-cutting concerns via global accessors

This refactoring will improve testability, onboarding speed, and code review efficiency while enabling incremental adoption without system-wide rewrites.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.25  
**Primary Dependencies**: gorilla/websocket, goccy/go-json, golang.org/x/tools (for static analysis)  
**Storage**: N/A (in-memory state management only)  
**Testing**: Go standard testing package with testify assertions  
**Target Platform**: Linux/macOS servers, library for integration into trading systems
**Project Type**: Single project (Go library/SDK)  
**Performance Goals**: P99 < 10ms WebSocket message latency, zero allocations in hot paths, 100+ concurrent symbol subscriptions  
**Constraints**: No breaking changes to existing public APIs during migration, maintain backward compatibility for non-migrated code  
**Scale/Scope**: ~50K LOC currently, multiple exchange adapters (Binance implemented, others planned), support for spot/futures/options markets

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**GOV-06: Innovation Over Compatibility** ✅ PASS
- This work prioritizes architectural improvements (clear layer boundaries, interface separation, testability) over preserving legacy coupling patterns
- Breaking changes documented: code that violates layer boundaries will break during migration
- Migration path provided: incremental adoption allows components to be refactored one at a time
- Justification: Innovation in architecture and maintainability takes precedence per constitution

**CQ-02: Layered Architecture Enforcement** ✅ ALREADY ALIGNED
- Current system already has 4-layer structure (L1-L4 / Connection-Routing-Business-Filter)
- This feature formalizes existing architecture with explicit interfaces and enforcement

**TS-01: Minimum Coverage Thresholds** ⚠️ WATCH
- Layer interface contracts and static analysis tools must have 90%+ coverage (core packages)
- Migration path testing requires careful integration test coverage

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

```
core/
├── layers/              # NEW: Layer interface definitions
│   ├── connection.go    # Connection layer interfaces
│   ├── routing.go       # Routing layer interfaces
│   ├── business.go      # Business layer interfaces
│   ├── filter.go        # Filter layer interfaces
│   └── categories/      # Type-specific interfaces
│       ├── spot.go      # Spot market interfaces
│       ├── futures.go   # Futures market interfaces
│       └── options.go   # Options market interfaces
├── transport/           # Existing: L1 transport primitives
├── exchanges/           # Existing: Exchange capabilities
└── streams/             # Existing: Stream definitions

exchanges/
├── shared/
│   ├── infra/           # Existing: L1 implementations (transport)
│   └── routing/         # Existing: L2 implementations
└── binance/
    ├── infra/           # Existing: Binance L1 (connection)
    │   ├── ws/          # WebSocket client
    │   └── rest/        # REST client
    ├── routing/         # Existing: Binance L2 (routing)
    │   ├── ws_router.go
    │   ├── rest_router.go
    │   └── stream_registry.go
    ├── bridge/          # Existing: Binance L3 (business logic)
    │   └── session.go
    └── filter/          # Existing: Binance L4 (filters)

pipeline/                # Existing: L4 pipeline orchestration
├── coordinator.go
├── stages.go
└── payloads.go

internal/
└── linter/              # NEW: Static analysis tool for layer boundaries
    ├── analyzer.go      # Import validation
    ├── rules.go         # Layer dependency rules
    └── testdata/        # Test fixtures

tests/
├── unit/                # Existing
├── integration/         # Existing
└── architecture/        # NEW: Architecture conformance tests
    ├── layer_boundaries_test.go
    └── interface_contracts_test.go
```

**Structure Decision**: Single Go project with modular package structure. The existing code already follows the four-layer pattern (L1/infra → L2/routing → L3/bridge → L4/pipeline). This feature adds:
1. Explicit layer interfaces in `core/layers/`
2. Static analysis tooling in `internal/linter/`
3. Architecture conformance tests in `tests/architecture/`

## Complexity Tracking

*No violations - Constitution Check passed all gates*
