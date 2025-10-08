# Implementation Plan: Unified Exchange Adapter Framework

**Branch**: `001-build-a-unified` | **Date**: 2025-10-08 | **Spec**: [Feature Specification](./spec.md)
**Input**: Feature specification from `/specs/001-build-a-unified/spec.md`

## Summary

Deliver a unified, type-safe exchange adapter framework that normalizes trading operations and market data across venues by enforcing the four-layer Transport → Routing → Exchange → Pipeline architecture. The implementation will extend the Binance blueprint to shared abstractions that translate venue REST/WebSocket payloads into canonical domain models with precise `*big.Rat` handling, standardized symbols, exhaustive enum mappings, and capability-aware routing, while preserving venue context inside a common error envelope.

Implementation will proceed in five coordinated tracks:

1. **Canonical Domain Foundation** – finalize shared models, symbol registry, enum mappings, and error envelope to guarantee precision and type safety across layers.
2. **Transport Layer Toolkit** – harden REST/WebSocket connectors with retry/backoff, streaming fan-out, and goroutine-safe buffering primitives tailored for in-memory operation.
3. **Routing Layer Services** – construct capability-aware order/data routers that translate canonical requests into venue-specific payloads and enforce bitset declarations.
4. **Exchange Adapter Composition** – encapsulate venue logic behind compile-time interfaces, wiring transports and routers while exposing a normalized API for trading and data consumers.
5. **Pipeline Orchestration & Quality Gates** – integrate adapters into the pipeline package, certify event propagation latency/ordering, and build race-detected regression suites with sandbox fixtures.

## Technical Context

**Language/Version**: Go 1.22+  
**Primary Dependencies**: Go standard library (net/http, net, crypto, encoding), `math/big` for rationals, standard WebSocket client implementations; minimal third-party packages retained from existing repo  
**Storage**: In-memory state only (no external databases)  
**Testing**: `go test ./...` with race detector, fixture-based mocks, integration shims around sandbox transports  
**Target Platform**: Headless Go services deployed via Make-driven builds for Linux/macOS targets  
**Project Type**: Backend trading framework in a monorepo  
**Performance Goals**: Maintain P99 latency <10 ms for market data normalization (PERF-01) and 99.95% precision-safe trade executions (per spec success criteria)  
**Constraints**: No floats or reflection, exhaustive enum switches, compile-time interface contracts, capability bitsets enforced, concurrency via goroutines/channels, environment-driven configuration  
**Scale/Scope**: Multiple simultaneous exchanges (spot + derivatives), thousands of symbols, tens of thousands of events per minute across public/private streams

## Constitution Check

This plan adheres to the ratified code-quality constitution in `.specify/memory/constitution.md`; all MUST principles (CQ-01…PERF-05) are treated as blocking gates and reflected in the phase checkpoints. Foundational work enforces CQ-01 (no floats), CQ-05 (exhaustive enums), and CQ-08 (compile-time contracts); testing requirements honor TS-01 (coverage thresholds), TS-02 (test-first), and TS-05 (error-path coverage); performance gates track PERF-01 (P99 latency), PERF-03 (propagation), and PERF-05 (rate-limit compliance).

## Project Structure

### Documentation (this feature)

```
specs/001-build-a-unified/
├── plan.md
├── spec.md
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

```
cmd/
config/
core/
    exchanges/
    registry/
    streams/
    transport/
errs/
exchanges/
    binance/
    shared/
internal/
market_data/
pipeline/
    adapter.go
    builder.go
    facade.go
    stages.go
```

**Structure Decision**: Retain monorepo layout with domain logic concentrated in `core/`, venue-specific adapters under `exchanges/`, and orchestration across the `pipeline/` package. Feature work will enhance these existing packages without introducing new top-level projects.

## Phase Delivery Gates

### Phase 1: Foundational Domain & Infrastructure
Establishes canonical contracts, capability bitsets, symbol normalization, and error envelope. All subsequent phases depend on this foundation completing with full test coverage per TS-01.

### Phase 2: User Story 1 – Execute Trades (Priority P1)
Delivers the canonical trading interface with capability-aware routing. Testing narrative includes order lifecycle regression tests and a **10,000-trade precision harness** (T209) that validates ≥99.95% precision adherence across venue adapters, serving as the delivery gate for SC-002. The harness exercises rational math through placement, fills, and cancellations, failing on any discrepancy.

### Phase 3: User Story 2 – Consume Market Data (Priority P2)
Provides normalized market data streams with P99 <10 ms latency guarantees (PERF-01). Soak tests instrument timestamps at transport boundary and post-normalization, logging distribution histograms to certify latency and ordering requirements.

### Phase 4: User Story 3 – Onboard New Exchanges (Priority P3)
Delivers pluggable adapter scaffolding with conformance suites. Integration engineers can compose new exchanges using shared factories while automated tests validate capability declarations and canonical contract adherence.

### Phase 5: Cross-Cutting Quality & Performance
Ensures constitution compliance, benchmarks, and observability. **Telemetry instrumentation** (T507, T508) captures baseline and post-launch support ticket metrics in `internal/telemetry/`, enabling retro-checks against SC-004's 90% reduction target, and tracks onboarding effort metrics to validate SC-001's 5-business-day target before release gates. **Hot-path benchmarks** enforce zero-allocation operation (T501a) in event decoding, symbol translation, and routing paths, satisfying NFR-P02 with `AllocsPerOp == 0` assertions. Constitution checkpoint verification (T506) blocks release until all MUST principles (CQ-01…PERF-05) pass.

## Complexity Tracking

No constitution violations identified; additional complexity tracking not required at this stage.
