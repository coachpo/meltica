# Implementation Plan: Lightweight Real-Time WebSocket Framework

**Branch**: `002-build-a-lightweight` | **Date**: 2025-10-09 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-build-a-lightweight/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Create a reusable Go streaming framework that manages WebSocket connections, decodes and validates pooled JSON payloads, and dispatches them to pluggable business handlers with sub-10ms latency and minimal allocations. The plan focuses on layered abstractions for connection orchestration, optional handshake authentication hooks, parsing pipelines powered by `goccy/go-json`, configurable validation, and extensibility hooks so multiple market data services can share the same core while meeting Meltica performance gates.

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: standard library (`net`, `context`, `sync`, `testing`), `github.com/gorilla/websocket`, `github.com/goccy/go-json` (new), internal `errs` package for typed errors  
**Storage**: N/A (streaming, operates entirely in memory)  
**Testing**: `go test` with `-race`, benchmarks under `testing.B`, assertions via `testify/require`  
**Target Platform**: Linux containers and on-prem servers running Meltica routing stack  
**Project Type**: Backend SDK/library consumed by exchanges and pipeline services  
**Performance Goals**: Sustain ≥50k msgs/sec, median processing latency <10ms, connection churn <1%  
**Constraints**: Zero-alloc hot paths, sync.Pool reuse, context propagation, observability hooks, compliance with CQ/UX/PERF gates  
**Scale/Scope**: Up to 5k concurrent clients per deployment; supports multi-feed fan-out across exchanges

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **CQ-01 SDK-First Architecture**: Framework resides in core-facing package with adapter-facing interfaces only; adapters implement without leaking transport types. ✅
- **CQ-02 Layered Architecture**: Plan preserves clear separation between transport (WebSocket client), routing/validation, and business handler layers. ✅
- **CQ-03 Zero Floating-Point Policy**: Numeric message fields will rely on existing decimal helpers / big.Rat conversions; no floats in public structs. ✅
- **CQ-06 Typed Errors**: All surface-level errors flow through `errs.E` codes (`CodeInvalid`, `CodeRateLimited`, `CodeNetwork`). ✅
- **UX-03 Context Propagation**: Every public API accepts `context.Context`; long-running loops honor cancellation. ✅
- **PERF-01 & PERF-03**: Design includes benchmarks and pooling to satisfy latency and allocation targets; failure to meet benchmarks blocks release. ✅

*Post-Design Verification*: Phase 1 artifacts (data model, contracts, quickstart) reaffirm compliance with CQ-01, CQ-02, CQ-03, CQ-06, UX-03, and PERF-01/03. No new violations detected.

## Project Structure

### Documentation (this feature)

```
specs/002-build-a-lightweight/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
└── tasks.md
```

### Source Code (repository root)

```
core/
├── transport/
│   └── websocket/          # Connection dialer, heartbeat, backpressure control
├── stream/                 # New canonical interfaces for high-throughput pipelines
└── validation/             # Reusable validation profiles shared with handlers

market_data/
├── framework/
│   ├── connection/         # Session lifecycle, pooling, rate governance
│   ├── parser/             # goccy/go-json decoding + schema guards
│   ├── handler/            # Extensible business logic interfaces & adapters
│   └── telemetry/          # Metrics/logging hooks for throughput + errors
└── adapters/               # Exchange-specific bindings consuming the framework

internal/
└── benchmarks/
    └── market_data/framework/   # Benchmark suites for latency & allocations

tests/
├── market_data/framework/
│   ├── unit/
│   ├── integration/
│   └── benchmarks/
└── fixtures/
    └── websocket/               # Recorded JSON payloads & error frames
```

**Structure Decision**: Introduce a `core/stream` package that defines canonical streaming contracts enforced by CQ-01 while `market_data/framework` provides the concrete implementation. Exchange adapters consume the framework via `market_data/adapters/*` ensuring CQ-02 layering; dedicated telemetry and benchmark directories uphold PERF-01/03 obligations.

## Complexity Tracking

*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
