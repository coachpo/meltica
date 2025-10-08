# Tasks: Unified Exchange Adapter Framework

**Input**: Design documents from `/specs/001-build-a-unified/`
**Prerequisites**: `plan.md`, `spec.md`

## Phase 1: Foundational Domain & Infrastructure (Blocking)

**Purpose**: Establish canonical contracts, capability declarations, symbol normalization, and error handling required by all user stories.

- [X] T101 [ALL] Finalize canonical order, trade, account, and position structs in `core/exchanges/bootstrap/types.go` ensuring all numeric fields use `*big.Rat` and adding table-driven tests in `core/exchanges/bootstrap/types_test.go` for precision boundaries.
- [X] T102 [P] [ALL] Introduce capability bitset definitions and query helpers in `core/exchanges/capabilities/capabilities.go` with enforcement tests in `core/exchanges/capabilities/capabilities_test.go`.
- [X] T103 [ALL] Extend the unified error envelope in `errs/errs.go` to carry canonical codes plus venue metadata, and add failure-path coverage in `errs/errs_test.go`.
- [X] T104 [P] [ALL] Build canonical symbol registry services in `core/symbols.go` and `core/symbols_test.go`, including regex validation and alias resolution fixtures.
- [X] T105 [ALL] Create shared enum mapping utilities in `exchanges/shared/routing/enums.go` with exhaustive switch tests in `exchanges/shared/routing/enums_test.go` (CI blocks on missing cases per CQ-05).
- [X] T106 [P] [ALL] Harden transport retry/backoff primitives in `exchanges/shared/infra/transport.go` and cover rate-limit, network, and timeout branches in `exchanges/shared/infra/transport_test.go`.

**Checkpoint**: Canonical contracts, symbols, capabilities, and error envelope ready; exchange-specific work can begin.

---

## Phase 2: User Story 1 – Execute trades through the canonical interface (Priority: P1)

**Goal**: Allow trading platform engineers to submit orders, amendments, and cancellations via a unified API with capability-aware routing.

**Independent Test**: Issue canonical order lifecycle requests against a sandbox adapter and verify normalized confirmations, cancellations, and capability failures without exchange-specific code.

### Tests (write first)

- [X] T201 [P] [US1] Author order lifecycle regression tests in `exchanges/shared/routing/order_router_test.go` using fixture payloads under `exchanges/shared/routing/testdata/orders/`.
- [X] T202 [US1] Add integration test exercising Binance sandbox order placement/cancel in `exchanges/binance/routing/order_integration_test.go`, gated by sandbox credentials env vars.

### Implementation

- [X] T203 [ALL→US1] Define the canonical trading service interface in `core/exchanges/bootstrap/trading_service.go`, including context-aware method signatures and capability checks.
- [X] T204 [US1] Implement capability-aware order routing in `exchanges/shared/routing/order_router.go`, translating canonical orders into venue payloads with rational math helpers.
- [X] T205 [P] [US1] Update Binance spot/derivatives adapters (`exchanges/binance/spot.go`, `exchanges/binance/futures.go`) to implement the canonical trading interface and register capability bitsets.
- [X] T206 [US1] Propagate the unified error envelope through trading flows in `exchanges/binance/translator.go` and related files, ensuring original venue codes are preserved.
- [X] T207 [P] [US1] Add godoc comments and examples for exported trading APIs across `core/exchanges/bootstrap` per CQ-07.
- [X] T208 [US1] Verify `go test ./exchanges/... -race` passes with new order routing tests and update coverage report to meet TS-01 thresholds for routing packages.
- [X] T209 [US1] Simulate 10,000 canonical trades per venue in `exchanges/shared/routing/trade_precision_test.go` and assert ≥99.95% precision adherence (SC-002), failing on any discrepancy. The harness must validate rational math through order lifecycle (placement, fills, cancels) and log any precision-related failures with venue context.

**Checkpoint**: Canonical trading operations function end-to-end with capability enforcement and precision guarantees.

---

## Phase 3: User Story 2 – Consume normalized market data streams (Priority: P2)

**Goal**: Provide analysts with venue-agnostic market data feeds delivering canonical symbols, rational values, and ordering guarantees across venues.

**Independent Test**: Subscribe to unified feeds spanning multiple venues and confirm schema consistency, P99 <10 ms normalization latency (PERF-01), and rational precision adherence.

### Tests (write first)

- [ ] T301 [P] [US2] Create event schema conformance tests in `market_data/events_test.go` covering trades, order books, funding, and account updates using recorded fixtures in `market_data/testdata/`.
- [ ] T302 [US2] Build latency soak test harness in `pipeline/multi_channel_test.go` simulating 10k events per venue, instrumenting timestamps at transport boundary and post-normalization, and asserting P99 latency <10 ms (PERF-01). The harness must log distribution histograms and fail if the threshold is exceeded.
- [ ] T303a [US2] Add symbol drift regression test in `market_data/symbol_guard_test.go` simulating mid-session symbol format changes (e.g., venue switching from "BTC-USD" to "BTCUSD"). Verify the system halts affected feeds, emits structured errors with remediation guidance, and prevents propagation of malformed events.

### Implementation

- [ ] T303 [ALL→US2] Introduce canonical market data event structs and builders in `market_data/events.go`, ensuring rational math and canonical symbol usage.
- [ ] T304 [US2] Enhance stream fan-out and ordering logic in `pipeline/multi_source.go` and `pipeline/stages.go` to tag events with source metadata and maintain per-venue sequencing.
- [ ] T305 [P] [US2] Update Binance WebSocket handling (`exchanges/binance/ws_service.go`, `exchanges/binance/ws_dependencies.go`) to emit canonical events and propagate contexts through goroutines.
- [ ] T306 [US2] Implement market data capability declarations in `exchanges/binance/plugin/register.go` (or equivalent) referencing the new bitsets.
- [ ] T307 [P] [US2] Add rational precision guards and unit tests for JSON encoding/decoding in `market_data/encoder.go` (new) and `market_data/encoder_test.go`.
- [ ] T308 [US2] Run `go test ./market_data ./pipeline -race -bench=.` ensuring TS-05 error paths are covered and documenting benchmark results against PERF-01 (P99 <10 ms normalization latency) and PERF-03 (end-to-end propagation) in test output. Fail the build if P99 exceeds 10 ms threshold.
- [ ] T309 [P] [US2] Build runtime symbol validation guard in `core/symbol_guard.go` with integration into `pipeline/stages.go`. The guard must detect symbol format drift mid-session, halt affected feeds safely, log structured errors with venue/symbol context, and include remediation guidance (e.g., "Symbol format changed from BASE-QUOTE to BASEQUOTE; check venue API changelog").

**Checkpoint**: Unified market data streams deliver normalized events with latency and precision guarantees.

---

## Phase 4: User Story 3 – Onboard a new exchange adapter (Priority: P3)

**Goal**: Provide integration engineers with a repeatable blueprint to add exchanges via pluggable transports, routers, and domain services without altering consumer APIs.

**Independent Test**: Scaffold a sample adapter using shared factories, declare capabilities, and run automated conformance tests verifying adherence to canonical contracts.

### Tests (write first)

- [ ] T401 [US3] Develop adapter conformance suite in `exchanges/shared/infra/adapter_conformance_test.go` that loads a mock adapter and validates capability bitsets, symbol normalization, and error envelope usage.
- [ ] T402 [P] [US3] Add plugin registration regression tests in `exchanges/shared/plugin/plugin_test.go` to ensure new adapters auto-register canonical services.

### Implementation

- [ ] T403 [ALL→US3] Create adapter composition scaffolding in `exchanges/shared/infra/adapter_factory.go` exposing builders for transports, routers, and services.
- [ ] T404 [US3] Provide template implementations and inline godoc guidance in `exchanges/shared/plugin/template.go` to guide new adapters per CQ-07.
- [ ] T405 [P] [US3] Add capability discovery CLI command in `cmd/capabilities/main.go` leveraging new bitsets to output exchange support matrices.
- [ ] T406 [US3] Document (in code comments) the protocol version handshake in `core/exchanges/bootstrap/protocol.go`, ensuring immutable interface checks per CQ-08.
- [ ] T407 [P] [US3] Build sandbox stub adapter under `exchanges/shared/mock/mock_exchange.go` demonstrating minimal Transport→Pipeline wiring for regression tests.
- [ ] T408 [US3] Execute `go test ./exchanges/shared/... -race` and confirm coverage ≥80% for routing packages per TS-01.

**Checkpoint**: New exchange adapters can be onboarded using shared scaffolding with automated conformance validation.

---

## Phase 5: Cross-Cutting Quality & Performance

**Purpose**: Ensure the entire framework meets governance requirements for coverage, benchmarks, race detection, and rate-limit compliance, and verify all constitution checkpoints before release.

- [ ] T501 [ALL] Expand benchmarks in `pipeline/pipeline_benchmark_test.go` and `market_data/events_benchmark_test.go` targeting PERF-01/03 thresholds, recording allocations.
- [ ] T501a [ALL] Run `go test ./pipeline ./market_data -bench=HotPath -benchmem` and fail if any benchmark reports non-zero allocations in event decoding, symbol translation, or routing hot paths (NFR-P02). Update benchmarks to assert `AllocsPerOp == 0` and document failures with function names and allocation sources. This gate blocks release until all critical paths achieve zero-allocation operation.
- [ ] T502 [P] [ALL] Add error-path regression tests for rate-limit handling in `exchanges/shared/infra/transport_rate_limit_test.go` and ensure token-bucket logic complies with PERF-05.
- [ ] T503 [ALL] Introduce observability hooks (callbacks, structured logging) in `pipeline/facade.go` and `pipeline/adapter.go` with unit tests asserting optional usage (UX-05).
- [ ] T504 [P] [ALL] Run full-suite `go test ./... -race -cover` ensuring TS-01 thresholds are met and generate coverage report for review.
- [ ] T505 [ALL] Validate idiomatic API ergonomics by adding examples in `core/exchanges/bootstrap/example_test.go` and running `go vet ./...`.
- [ ] T506 [ALL] **Constitution Checkpoint Verification**: Confirm all MUST principles (CQ-01…PERF-05) are satisfied—verify CQ-01 (no floats), CQ-05 (exhaustive enums with CI enforcement), CQ-08 (compile-time contracts), TS-01 (coverage ≥80% routing packages), TS-02 (test-first commit history), TS-05 (error-path coverage), PERF-01 (P99 <10 ms documented), PERF-03 (propagation benchmarks), and PERF-05 (rate-limit compliance). Block release until all gates pass.
- [ ] T507 [ALL] Implement telemetry instrumentation for support ticket reduction metrics (SC-004) in `internal/telemetry/metrics.go` and `internal/telemetry/metrics_test.go`. Capture baseline symbol/enum-related ticket counts pre-launch and emit post-launch comparison metrics. Store collection scripts/configs under `internal/telemetry/` and validate metric emission in unit tests.
- [ ] T508 [US3] Capture onboarding effort metrics in `internal/telemetry/onboarding_tracker.go` to validate SC-001. Log start/end timestamps for new adapter projects, calculate business-day duration (excluding weekends), and emit reporting to ensure 5-business-day target compliance. Include tests in `internal/telemetry/onboarding_tracker_test.go` verifying duration calculations and metric emission.

**Checkpoint**: Framework satisfies constitution mandates, performance benchmarks, telemetry instrumentation for post-launch validation, and readiness for release.

---

## Dependencies & Execution Order

1. **Phase 1** must be completed before any user story work; it establishes shared contracts and infrastructure.
2. **Phase 2 (US1)** depends on Phase 1 but can run in parallel with Phases 3 and 4 once foundational tasks complete; prioritize US1 for MVP delivery.
3. **Phase 3 (US2)** and **Phase 4 (US3)** both depend on foundational work but do not depend on each other; they may proceed concurrently after Phase 1.
4. **Phase 5** requires completion of all targeted user stories to verify cross-cutting requirements and performance.

Within each phase, tasks marked **[P]** target different files or independent areas and can be executed in parallel. Tests listed at the top of each user story should be authored before implementation tasks to honor TS-02 and TS-05.
