# Feature Specification: Extract Market Data Routing into Universal Library (/lib/ws-routing)

**Feature Branch**: `010-extract-the-market`  
**Created**: 2025-10-11  
**Status**: Draft  
**Input**: User description: "Extract the Market Data Routing Framework into a universal library at lib/ws-routing and update all call sites. Migrate reusable components (connection/session runtime, middleware, handler registry, routing engine, telemetry interfaces) into lib/ws-routing with business-agnostic names. Keep domain-specific processors in exchanges/processors. Update adapters to import lib/ws-routing. Remove the deprecation shim (framework/router/shim.go) since backward compatibility is not required. Update docs (ARCHITECTURE.md, README.md; fold MIGRATION.md into BREAKING_CHANGES_v2.md) to reflect the new package. Success criteria: all unit/integration tests pass; imports are clean; no references to market_data/framework remain; make targets and go.mod paths updated; benchmarks for hot path preserved or improved."

## Clarifications

### Session 2025-10-11

- Q: To keep architecture boundaries clean (L1 Transport vs L2 Routing), how should /lib/ws-routing handle connections? → A: B — Library requires an abstract transport (Conn interface); routes only.
- Q: How should event/message schemas change with the new universal library? → A: A — Preserve existing event schemas unchanged.
- Q: What observability baseline must /lib/ws-routing provide? → A: A — Structured logging only; metrics/tracing optional.
- Q: What event ordering/concurrency guarantees must the router provide? → A: B — Preserve per-symbol order; allow parallelism across symbols.
- Q: What backpressure policy should the router use when handlers are slower than input? → A: A — Block upstream (apply backpressure; no drops).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Universal Routing Library Available (Priority: P1)

Platform maintainers provide a universal, business-agnostic routing library under `/lib/ws-routing` that consumes an abstract transport and encapsulates routing session orchestration, middleware, handler registry, routing engine, and telemetry interfaces. The library does not own network connections.

**Why this priority**: Unlocks reuse across domains, reduces maintenance, and establishes clear boundaries between shared infrastructure and domain processors.

**Independent Test**: Consumers can initialize the library, register middleware and handlers, and route messages in a demo flow without importing any domain packages.

**Acceptance Scenarios**:

1. Given the library is installed, When a consumer configures a routing session and subscribes to streams, Then messages flow through middleware and handlers with expected outcomes.
2. Given the library’s public APIs, When inspecting exported identifiers, Then no domain-specific names or types are present.
3. Given interleaved inputs across multiple symbols, When routing occurs, Then per-symbol event order matches input order while different symbols may process concurrently.
4. Given a slow handler, When inputs exceed handler throughput, Then the router applies backpressure (blocks upstream) without dropping messages and preserves per-symbol order.

---

### User Story 2 - Domain Adapters Updated (Priority: P2)

Domain teams update adapters to import from `/lib/ws-routing` while keeping domain-specific processors in `exchanges/processors`.

**Why this priority**: Ensures the domain continues to operate without internal framework dependencies while leveraging shared infrastructure.

**Independent Test**: Existing domain flows run using only the new library imports; a grep shows no references to `market_data/framework`.

**Acceptance Scenarios**:

1. Given updated imports, When building and running tests, Then all domain pipelines pass with no references to `market_data/framework`.
2. Given code review, When scanning adapter files, Then only public APIs from `/lib/ws-routing` are referenced.
3. Given regression contract tests, When routing identical inputs through old vs new paths, Then output event/message schemas are identical.

---

### User Story 3 - Documentation and Tooling Converge (Priority: P3)

Project documentation and build tooling reference the new package location, with migration notes folded into existing breaking-changes records.

**Why this priority**: Avoids confusion for contributors and consumers; streamlines onboarding and upgrades.

**Independent Test**: Readers can follow ARCHITECTURE and README to locate the library; release notes document breaking changes; build targets succeed.

**Acceptance Scenarios**:

1. Given updated docs, When a reader searches for routing capabilities, Then they find `/lib/ws-routing` and current guidance.
2. Given updated tooling, When running standard build/test targets, Then tasks succeed without references to removed paths.

### Edge Cases

- Legacy references to `market_data/framework/router` remain in overlooked files and cause build failures.
- Naming collisions or domain concepts accidentally persist in library exports.
- Hot-path performance degrades after extraction.
- Missing documentation leads to confusion about where to implement domain processors vs. shared handlers.
- Upstream delivers interleaved events with identical timestamps; per-symbol relative arrival order must still be preserved while symbols may process concurrently.
- Handlers stall for extended periods; router must block (not drop) and expose structured logs to diagnose backpressure.

## Requirements *(mandatory)*

**Compatibility Note**: This change is backward incompatible in import paths only; event/message schemas remain unchanged. Remove the deprecation shim and migrate consumers to the new package path. Document impacts and fold migration notes into existing breaking-changes records.

**GOV-07 Compliance Notes**: Constitution v1.5.0 no longer requires deprecation shims or import aliases. This feature documents the breaking import path changes within BREAKING_CHANGES_v2.md, communicates impacts to downstream consumers, and records maintainer approval for the documented migration plan (second maintainer approval pending before merge).

### Functional Requirements

- **FR-001**: Deliver a universal routing library at `/lib/ws-routing` that exposes public, business-agnostic APIs for routing session lifecycle (not network connections), subscription management, middleware composition, handler registration, routing, and telemetry hooks; the library requires an abstract transport provided by callers.
- **FR-002**: Migrate all call sites to import from `/lib/ws-routing`; no references to `market_data/framework` remain across the repository.
- **FR-003**: Keep domain-specific processors under `exchanges/processors`; domain code interacts with the library only via its public APIs.
- **FR-004**: Remove the deprecation shim at `market_data/framework/router/shim.go`; no transitional aliases remain.
- **FR-005**: Update documentation to reflect the new package location: ARCHITECTURE.md and README.md; fold any prior migration notes into `BREAKING_CHANGES_v2.md`, then verify the standalone `MIGRATION.md` file is absent (remove it if still present).
- **FR-006**: Ensure build tooling and module metadata are consistent: update Make targets and module import paths where needed so standard build and test commands succeed.
- **FR-007**: Preserve or improve hot-path routing performance; benchmarks for routing dispatch remain at or better than current baselines.
- **FR-008**: Ensure `/lib/ws-routing` exports remain domain-agnostic: all library exports, public APIs, type names, examples, and documentation must remain free of domain-specific terminology (e.g., no "market", "trade", "ticker" references in exported identifiers or doc comments); verify via automated checks (grep/lint), explicit export validation tests, and code review.
- **FR-009**: Provide exhaustive tests for boundary contracts: positive and negative cases for all exported APIs, and explicit enum/switch coverage.
- **FR-010**: Preserve existing event/message schemas unchanged; no field renames/additions/removals or type changes to routed outputs.
- **FR-011**: Provide a structured logging baseline via a vendor-neutral interface; metrics and tracing remain optional extension points.
- **FR-012**: Preserve per-symbol ordering of routed events while permitting parallel processing across different symbols; no cross-symbol ordering guarantees.
- **FR-013**: Apply backpressure when handlers are slower than input: block upstream dispatch/reads rather than dropping messages; preserve per-symbol order under load.

### Key Entities *(include if feature involves data)*

- **Routing Library**: A reusable component that manages session lifecycle, subscriptions, middleware chains, handler registry, routing decisions, and telemetry hooks.
- **Domain Adapter**: A domain-owned integration layer that consumes the library’s public APIs and translates routed events into domain processors.
- **Telemetry Interfaces**: Business-agnostic hooks with a structured logging baseline; metrics/tracing are optional extension points. No vendor SDKs are exposed in public APIs.
- **Abstract Transport**: A domain-agnostic message stream provider supplied by callers; the library routes messages over this interface and does not manage connection establishment or reconnection.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of previous references to `market_data/framework` are removed; repository-wide search yields zero matches after merge.
- **SC-002**: All unit and integration tests pass on CI with race detection and coverage thresholds unchanged or improved.
- **SC-003**: Standard build/test targets complete successfully with updated Make tasks and module paths.
- **SC-004**: Routing hot-path performance meets or exceeds baseline (no additional allocations; median latency ≤ 500ns, P99 ≤ 2µs per event on reference hardware).
- **SC-005**: Documentation readers can locate and understand the new library via ARCHITECTURE.md and README.md without referencing removed paths.
- **SC-006**: Event/message schemas are unchanged pre/post extraction; regression contract tests confirm parity.
- **SC-007**: Structured logs emitted for session lifecycle, subscribe/unsubscribe, route outcomes, and error paths; no vendor-specific logging dependencies in public API.
- **SC-008**: Ordering contract tests confirm per-symbol ordering is preserved and concurrency across symbols is exercised without violating per-symbol order.
- **SC-009**: Backpressure tests simulate slow handlers with zero message loss and preserved per-symbol order; no drops observed under bounded stress.
- **SC-010**: Export validation confirms zero domain-specific terminology in `/lib/ws-routing` public surface per FR-008 via automated grep/lint checks and explicit review; no "market", "trade", "ticker", or similar domain nouns in exported identifiers or docs.

## Assumptions

- Backward compatibility is not required; no deprecation shim will be maintained.
- Any existing migration notes will be folded into `BREAKING_CHANGES_v2.md`; no standalone `MIGRATION.md` is retained.
- Baseline performance thresholds reflect the project’s standard reference hardware and current benchmarks.
- Domain processors remain in-place under `exchanges/processors` and are not functionally expanded as part of this effort.

## Dependencies

- Availability of existing unit/integration tests and routing benchmarks to validate parity and performance.
- Documentation ownership to update ARCHITECTURE.md, README.md, and breaking-changes records.
- Build tooling (Make targets) and module path updates coordinated to ensure green pipelines.
- Transport connectivity is provided by an external L1 connection layer; the library consumes an abstract transport only.
