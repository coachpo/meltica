# Feature Specification: Unified Exchange Adapter Framework

**Feature Branch**: `001-build-a-unified`  
**Created**: 2025-10-08  
**Status**: Draft  
**Input**: User description: "Build a unified cryptocurrency exchange adapter framework in Go that normalizes trading operations and market data across multiple exchanges through a single, type-safe API. The system follows a four-layer architecture—Transport, Routing, Exchange, Pipeline—where Transport handles HTTP/WebSocket connectivity and low-level protocol concerns, Routing translates venue-specific payloads into canonical requests/events, Exchange exposes domain services (trading, market data, account) with capability declarations, and Pipeline orchestrates cross-venue workflows and event distribution. Each exchange adapter translates venue-specific REST endpoints and WebSocket streams into canonical domain models with precise decimal handling (using `*big.Rat` for all numeric fields), standardized symbol formats (BASE-QUOTE normalization), and exhaustive enum mappings. Exchanges declare their capabilities through bitsets and are composed using pluggable transport factories, routing layers, and domain services. The framework currently supports Binance fully (spot, linear futures, inverse futures, public and private WebSocket streams) and provides a complete blueprint for onboarding additional exchanges following the same architectural patterns. All numeric values use arbitrary-precision rationals to avoid floating-point errors, symbols are normalized to canonical format, and errors are wrapped in a unified error envelope with exchange-specific context preserved for debugging."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Execute trades through the canonical interface (Priority: P1)

A trading platform engineer needs to place, amend, and cancel orders across supported exchanges using a single, normalized trading interface.

**Why this priority**: Trading operations generate direct revenue; inconsistent behavior across venues would block adoption of the framework.

**Independent Test**: Submit canonical order workflows against a sandboxed venue and verify orders execute, update, and cancel without venue-specific code paths.

**Acceptance Scenarios**:

1. **Given** an exchange adapter advertising order placement capability, **When** the engineer submits a canonical limit order, **Then** the framework routes it to the correct adapter and returns a normalized confirmation with precise price and size values.
2. **Given** an existing live order, **When** the engineer issues a canonical cancel request, **Then** the framework cancels the order at the venue and returns a normalized order status transition.
3. **Given** an exchange that lacks a requested capability, **When** the engineer attempts the unsupported action, **Then** the framework rejects the request with a structured error describing the missing capability and original venue details.

---

### User Story 2 - Consume normalized market data streams (Priority: P2)

A market data analyst subscribes to unified market data feeds to power pricing models without handling venue-specific formats.

**Why this priority**: Accurate, timely data is essential for strategy performance and requires consistent schemas to plug into existing analytics.

**Independent Test**: Subscribe to aggregated order book and trade feeds, validate schema consistency, decimal precision, and symbol normalization for multiple venues in parallel.

**Acceptance Scenarios**:

1. **Given** an exchange adapter connected to public feeds, **When** the analyst subscribes to a canonical symbol, **Then** the framework delivers normalized market depth and trade events with canonical symbols and rational-valued fields.
2. **Given** multiple venues emitting updates for the same asset, **When** the analyst consumes the combined feed, **Then** the framework maintains ordering guarantees per venue and tags events with source metadata for downstream routing.

---

### User Story 3 - Onboard a new exchange adapter (Priority: P3)

An integration engineer adds a new exchange by composing transport, routing, and domain services while relying on shared abstractions.

**Why this priority**: Fast onboarding unlocks coverage expansion and de-risks reliance on a single venue.

**Independent Test**: Implement a prototype adapter using the provided factories, enable required capabilities, and verify conformance through automated capability and regression tests.

**Acceptance Scenarios**:

1. **Given** the blueprint and canonical interfaces, **When** the engineer registers a new adapter with declared capabilities, **Then** the framework exposes the exchange through the unified API without additional consumer changes.
2. **Given** venue-specific REST and WebSocket endpoints, **When** the engineer maps them using transport factories, **Then** the framework produces normalized domain models and enumerations without manual schema translation by downstream teams.

---

### Edge Cases

- What happens when an exchange changes symbol formats mid-session? The framework must detect mismatches, halt the affected feed, and surface a structured error with remediation guidance.
- How does the system handle partial capability coverage (e.g., cancel-only markets)? Requests must fail fast with capability-specific errors while allowing supported operations to proceed.
- How does the framework behave during precision conflicts (e.g., venue rounding constraints)? It must reconcile values using rational math and signal any loss of precision before executing trades.
- What occurs if transport connectivity drops? Automatic reconnection with backoff and notification to consumers must ensure no silent data gaps.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The framework MUST expose a canonical trading interface covering order placement, amendment, cancellation, and position queries across all supported venues.
- **FR-002**: The framework MUST translate venue-specific REST and streaming payloads into canonical domain models while preserving arbitrary-precision rational values for all numeric fields.
- **FR-003**: The framework MUST maintain standardized BASE-QUOTE symbol normalization, including alias resolution and validation against canonical symbol registries.
- **FR-004**: Each exchange adapter MUST declare its supported capabilities via bitsets that are queryable by consumer services and enforced at runtime.
- **FR-005**: The framework MUST allow exchange adapters to be composed from pluggable transport factories, routing layers, and domain services without modifying consumer-facing interfaces.
- **FR-006**: The framework MUST deliver normalized market data streams (trades, order books, funding, account events) with consistent schemas and venue metadata across public and private channels.
- **FR-007**: All errors MUST be wrapped in a unified error envelope that includes canonical error codes, human-readable context, and the originating venue details for debugging.
- **FR-008**: The framework MUST provide exhaustive enum mappings that reconcile venue-specific constants (order types, time-in-force, account modes) to canonical enumerations with validation.

### Non-Functional Requirements

- **Performance**:
  - **NFR-P01**: Market data normalization MUST achieve P99 latency <10 ms from raw venue payload receipt to canonical event emission (PERF-01).
  - **NFR-P02**: Critical paths (parsing, translation, event emission) MUST operate with zero allocations in hot loops to minimize GC pressure (PERF-03).
  - **NFR-P03**: Rate-limit handling MUST implement token-bucket or equivalent algorithms that prevent API throttling while maximizing throughput (PERF-05).

- **Precision & Type Safety**:
  - **NFR-PS01**: All numeric fields MUST use `*big.Rat` for arbitrary-precision rational arithmetic; floating-point types are prohibited (CQ-01).
  - **NFR-PS02**: Trade execution precision MUST achieve ≥99.95% accuracy across 10,000-trade regression harnesses per venue (SC-002).
  - **NFR-PS03**: Enum mappings MUST be exhaustive with compile-time enforcement; missing cases MUST fail CI builds (CQ-05).

- **Testing & Quality**:
  - **NFR-TQ01**: Routing packages MUST maintain ≥80% test coverage with race-detector validation across all test suites (TS-01).
  - **NFR-TQ02**: Implementation MUST follow test-first practices; tests are authored before production code for all user stories (TS-02).
  - **NFR-TQ03**: Error paths MUST achieve comprehensive coverage including rate-limits, network failures, and malformed venue responses (TS-05).

- **Observability & User Experience**:
  - **NFR-UX01**: Support ticket metrics MUST be instrumented to capture symbol/enum-related incidents pre- and post-launch for 90% reduction validation (SC-004, T507).
  - **NFR-UX02**: Context propagation MUST flow through all goroutines and enable distributed tracing for cross-venue request correlation (UX-03).
  - **NFR-UX03**: Error envelopes MUST include remediation guidance with venue-specific context to accelerate debugging (FR-007).

- **Architecture & Contracts**:
  - **NFR-AC01**: All cross-layer boundaries MUST use compile-time interface contracts without runtime reflection (CQ-08).
  - **NFR-AC02**: Capability declarations MUST be queryable at runtime and enforced before routing requests to adapters (FR-004).
  - **NFR-AC03**: Adapter onboarding MUST complete within 5 business days using provided scaffolding and conformance tests (SC-001).

### Key Entities *(include if feature involves data)*

- **ExchangeAdapter**: Represents a venue integration composed of transport connectors, routing rules, capability declarations, and domain translators.
- **CapabilityProfile**: Describes the bitset of trading and data features an exchange supports, enabling capability-aware routing and validation.
- **SymbolMapping**: Stores canonical BASE-QUOTE representations, aliases, and validation rules used during request normalization and feed processing.
- **MarketDataEvent**: Captures normalized trades, order book updates, and account events including venue metadata, timestamps, and rational-valued fields.
- **ErrorEnvelope**: Encapsulates standardized error codes, user-facing messaging, and original venue payloads for diagnostics.

### Assumptions

- Additional exchanges will follow the documented blueprint established by the Binance implementation for layering and capability declaration.
- Consumers rely on strongly typed domain models and will not accept loosely typed or string-based fallbacks.
- Network connectivity and credential management are handled by existing infrastructure, allowing the framework to focus on normalization and routing.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Trading teams can onboard a new exchange adapter to parity with Binance core operations within 5 business days of starting integration using the framework’s abstractions.
- **SC-002**: During regression testing of 10,000 simulated trades per venue, at least 99.95% of executions complete without precision-related discrepancies or manual intervention. Validated via the precision regression harness (T209) in `exchanges/shared/routing/trade_precision_test.go`, which exercises rational math through the complete order lifecycle and fails on any precision loss.
- **SC-003**: Market data normalization exhibits P99 latency <10 ms as measured from raw venue payload receipt to canonical event emission (PERF-01), with zero schema validation failures during a 24-hour soak test. Measurement instrumentation captures timestamps at transport boundary and post-normalization, logging distribution histograms for regression analysis.
- **SC-004**: Post-launch, support tickets related to symbol or enum mismatches decrease by 90% compared to the baseline measured prior to framework adoption. Progress measured via telemetry instrumentation (T507) capturing pre-launch baseline and post-launch metrics in `internal/telemetry/`, with automated comparison reporting.
