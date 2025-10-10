# Implementation Tasks: Multi-Stream Message Router with Architecture Migration

**Feature Branch**: `003-we-ve-built`  
**Date**: 2025-01-09  
**Based On**: [plan.md](./plan.md) | [spec.md](./spec.md) | [data-model.md](./data-model.md)

## Overview

This document breaks down the implementation into executable tasks organized by user story priority. Each phase delivers an independently testable increment of functionality.

**Total Tasks**: 54  
**Estimated Duration**: 2-3 sprints  
**MVP Scope**: Phase 3 (User Story 1 - P1)

---

## Phase 1: Setup & Infrastructure (6 tasks)

**Objective**: Initialize project structure and shared infrastructure needed by all user stories.

### T001 [P] - Create router package structure [X]
**File**: `market_data/framework/router/`
- Create directory: `market_data/framework/router/`
- Add package declaration and initial doc comments
- Create empty files: `detector.go`, `registry.go`, `dispatcher.go`, `backpressure.go`, `types.go`
**Exit Criteria**: Package compiles with `go build ./market_data/framework/router`

### T002 [P] - Create processors package structure [X]
**File**: `market_data/processors/`
- Create directory: `market_data/processors/`
- Add package declaration
- Create empty files: `trade.go`, `orderbook.go`, `account.go`, `funding.go`, `default.go`, `interface.go`
**Exit Criteria**: Package compiles with `go build ./market_data/processors`

### T003 [P] - Define core routing types [X]
**File**: `market_data/framework/router/types.go`
- Define `MessageTypeDescriptor` struct (from data-model.md § 1)
- Define `DetectionRule` struct and `DetectionStrategy` enum (from data-model.md § 2)
- Define `ProcessorRegistration` struct and `ProcessorStatus` enum (from data-model.md § 3)
- Add validation methods for each type
**Dependencies**: None (pure types)
**Exit Criteria**: Types compile; validation methods have unit tests

### T004 [P] - Define Processor interface [X]
**File**: `market_data/processors/interface.go`
- Define `Processor` interface with `Initialize()`, `Process()`, `MessageTypeID()` methods (from data-model.md § 4)
- Add interface doc comments explaining implementation requirements
- Reference processor-interface.yaml contract
**Dependencies**: None (pure interface)
**Exit Criteria**: Interface compiles; godoc generated

### T005 [P] - Extend events.go with routing context [X]
**File**: `market_data/events.go`
- Add `StreamContext` struct (optional metadata: streamID, exchange, isPrivate)
- Ensure existing payloads (Trade, OrderBook, Account, Funding) unchanged
**Dependencies**: None (extends existing)
**Exit Criteria**: Existing code compiles; no breaking changes to public API

### T006 [P] - Setup test infrastructure [X]
**File**: `market_data/framework/router/testutil.go`, `market_data/processors/testutil.go`
- Create test fixtures for common message types (trade, orderbook, account JSON samples)
- Create mock processor implementation for testing
- Create helper: `loadTestPayload(filename string) []byte`
**Dependencies**: T001, T002
**Exit Criteria**: Test utilities compile; fixtures loadable

**🚧 Checkpoint**: Setup complete - router and processors packages exist with core types defined

---

## Phase 2: Foundational - Core Routing Infrastructure (8 tasks)

**Objective**: Implement the routing engine that ALL user stories depend on. These tasks MUST complete before any user story can be fully implemented.

### T007 - Implement RoutingTable with sync.Map [X]
**File**: `market_data/framework/router/registry.go`
- Implement `RoutingTable` struct using `sync.Map` for entries (from research.md decision)
- Implement `Register(desc *MessageTypeDescriptor, proc Processor) error`
- Implement `Lookup(messageTypeID string) *ProcessorRegistration` (lock-free read)
- Implement `SetDefault(proc Processor) error`
- Handle processor initialization with 5-second timeout; mark unavailable on failure (FR-004)
**Dependencies**: T003, T004
**Constitution**: CQ-06 (typed errors with errs.E)
**Exit Criteria**: 
- Lookup() benchmark < 100ns p99
- Register() handles init failures gracefully (processor marked unavailable)
- Unit tests: registration, lookup, default fallback

### T008 - Implement field-based message type detector [X]
**File**: `market_data/framework/router/detector.go`
- Implement `FieldDetectionStrategy` struct
- Implement `Detect(raw []byte) (messageTypeID string, err error)`
- Use goccy/go-json for fast partial parse (research.md decision)
- Support configurable field paths (e.g., "e", "type", "stream")
- Return error with errs.E on parse failure or no match
**Dependencies**: T003
**Constitution**: CQ-06 (typed errors)
**Exit Criteria**:
- Detection benchmark < 1ms p95 (FR-002 target)
- Unit tests: valid detection, malformed JSON, no match

### T009 - Implement RoutingTable.Detect() orchestration [X]
**File**: `market_data/framework/router/registry.go`
- Implement `RoutingTable.Detect(raw []byte) (messageTypeID string, err error)`
- Iterate through descriptors' detection rules in priority order
- Return first match (short-circuit evaluation per clarification)
- Return empty string if no match (routes to default processor)
**Dependencies**: T007, T008
**Exit Criteria**:
- Unit tests: multiple descriptors, priority ordering, first-match wins
- Integration test: Detect() → Lookup() flow

### T010 - Implement message dispatcher with backpressure [X]
**File**: `market_data/framework/router/dispatcher.go`
- Implement `RouterDispatcher` struct with processor channels map
- Implement `Dispatch(messageTypeID string, raw []byte) error`
- Use unbuffered channels for strict backpressure (research.md decision)
- Respect context cancellation for graceful shutdown
**Dependencies**: T007
**Constitution**: UX-03 (context propagation)
**Exit Criteria**:
- Backpressure blocks upstream when processor slow (no message drops per FR-012)
- Unit tests: successful dispatch, context cancellation
- Integration test: backpressure under load

### T011 - Implement routing metrics collection [X]
**File**: `market_data/framework/router/metrics.go`
- Implement `RoutingMetrics` struct (from data-model.md § 6)
- Implement `RecordRoute(messageTypeID string)`
- Implement `RecordError()`
- Implement `RecordProcessing(messageTypeID string, duration time.Duration, err error)`
- Implement `Snapshot() *RoutingMetrics` (thread-safe deep copy)
- Use sync.RWMutex for concurrent updates
**Dependencies**: T003
**Constitution**: FR-010 (observability)
**Exit Criteria**:
- Metrics accurately track routing operations
- Snapshot() non-blocking
- Unit tests: concurrent updates, snapshot correctness

### T012 - Wire RoutingTable with metrics [X]
**File**: `market_data/framework/router/registry.go`
- Add `metrics *RoutingMetrics` field to RoutingTable
- Update Register() to record processor init failures
- Update Detect() to record routing successes/errors
- Implement `GetMetrics() *RoutingMetrics` method
**Dependencies**: T007, T011
**Exit Criteria**:
- Metrics correctly increment during routing operations
- Unit tests: metrics updated on register, detect, dispatch

### T013 - Implement backpressure flow control [X]
**File**: `market_data/framework/router/backpressure.go`
- Implement channel depth monitoring
- Implement backpressure event tracking metric
- Add `routing_channel_depth` gauge metric
**Dependencies**: T010, T011
**Exit Criteria**:
- Channel depth reported accurately
- Backpressure events tracked in metrics

### T013A [P] - Integration test: Frame reassembly validation [X]
**File**: `market_data/framework/connection/frame_reassembly_test.go`
- Send large message (100KB+ JSON payload) that exceeds WebSocket frame size
- Verify gorilla/websocket automatically reassembles fragmented frames
- Confirm framework's higher layers (router, processors) receive complete message
- Validate research.md assumption: "gorilla/websocket handles frame reassembly per RFC 6455"
**Dependencies**: Existing connection package (no new code, validation only)
**Constitution**: FR-007 (L1 must handle frame reassembly transparently)
**Exit Criteria**:
- Large message sent and received without partial frame visibility
- Integration test documents reassembly guarantee
- FR-007 compliance validated

### T014 - Integration test: Complete routing pipeline [X]
**File**: `market_data/framework/router/integration_test.go`
- Test: Register processor → Detect message → Dispatch → Process
- Test: Multiple message types routed concurrently without cross-contamination
- Test: Unrecognized message routes to default processor
- Test: Processor init failure → unavailable status → routes to default
**Dependencies**: T007-T013
**Exit Criteria**: End-to-end routing works; all FR-001 through FR-004 validated

**🚧 Checkpoint**: Core routing engine complete - ready to implement processors for user stories

---

## Phase 3: User Story 1 (P1) - Route Mixed Message Types (12 tasks)

**Story Goal**: Enable automatic routing of mixed message types from a single stream to appropriate processors.

**Independent Test**: Connect to test stream emitting trades, order books, and accounts in random order. Verify 100% routing accuracy with zero cross-contamination (AS1.1).

### T015 [P] - Implement TradeProcessor [X]
**Story**: US1  
**File**: `market_data/processors/trade.go`
- Implement `TradeProcessor` struct satisfying Processor interface
- Implement `Initialize(ctx context.Context) error` (complete < 5s)
- Implement `Process(ctx context.Context, raw []byte) (interface{}, error)` returning `*TradePayload`
- Use `*big.Rat` for price/quantity (CQ-03 compliance)
- Handle malformed input gracefully (no panics per processor-interface.yaml)
- Respect context cancellation within 100ms
**Dependencies**: T004, T006 (fixtures)
**Constitution**: CQ-03 (zero floating-point), CQ-06 (typed errors)
**Exit Criteria**:
- Unit tests: valid trade, malformed JSON, context cancellation
- Benchmark: <3ms p95 (PERF-01 target)
- Race detector passes

### T016 [P] - Implement OrderBookProcessor [X]
**Story**: US1  
**File**: `market_data/processors/orderbook.go`
- Implement `OrderBookProcessor` struct
- Parse bids/asks arrays into `[]core.BookDepthLevel` with `*big.Rat` for price/qty
- Same requirements as T015 (initialization, error handling, context respect)
**Dependencies**: T004, T006
**Constitution**: CQ-03, CQ-06
**Exit Criteria**: Same as T015 but for orderbook payloads

### T017 [P] - Implement AccountProcessor [X]
**Story**: US1  
**File**: `market_data/processors/account.go`
- Implement `AccountProcessor` struct
- Parse balance arrays into `[]AccountBalance` with `*big.Rat` for totals/available
- Same requirements as T015
**Dependencies**: T004, T006
**Constitution**: CQ-03, CQ-06
**Exit Criteria**: Same as T015 but for account payloads

### T018 [P] - Implement FundingProcessor [X]
**Story**: US1  
**File**: `market_data/processors/funding.go`
- Implement `FundingProcessor` struct
- Parse funding rate with `*big.Rat`, interval duration, effective timestamp
- Same requirements as T015
**Dependencies**: T004, T006
**Constitution**: CQ-03, CQ-06
**Exit Criteria**: Same as T015 but for funding payloads

### T019 - Implement DefaultProcessor (fallback) [X]
**Story**: US1  
**File**: `market_data/processors/default.go`
- Implement `DefaultProcessor` for unrecognized message types
- Returns `*RawPayload` wrapper with raw bytes and metadata
- Always succeeds (never fails initialization)
- Logs warning for unrecognized types
**Dependencies**: T004
**Exit Criteria**:
- Unit test: processes arbitrary message without error
- Integration test: routes unrecognized type to default (AS1.2)

### T020 - Register core processors in router [X]
**Story**: US1  
**File**: `market_data/framework/router/registry.go` (add InitializeRouter function)
- Create `InitializeRouter() (*RoutingTable, error)` helper
- Register trade, orderbook, account, funding processors with detection rules
- Set DefaultProcessor as fallback
- Use field-based detection with appropriate field paths (e.g., Binance "e" field)
**Dependencies**: T007, T015-T019
**Exit Criteria**:
- InitializeRouter() returns fully configured routing table
- All processors status=Available
- Unit test: lookup by type returns correct processor

### T021 - Test: Mixed stream routing accuracy [X]
**Story**: US1  
**File**: `market_data/framework/router/mixed_stream_test.go`
- Test AS1.1: Send 100 trades + 50 orderbooks in random order → verify zero cross-contamination
- Measure routing accuracy: 100% correct type assignment
- Verify output channels segregated by type
**Dependencies**: T020, T014
**Exit Criteria**: SC-001 validated (1,000 messages route 100% correctly)

### T022 - Test: Concurrent streams without interference [X]
**Story**: US1  
**File**: `market_data/framework/router/concurrent_test.go`
- Test AS1.3: Create 10 streams with different type configurations
- Send messages on all streams simultaneously
- Verify no cross-stream contamination of processor instances
**Dependencies**: T020
**Exit Criteria**: Messages correctly routed per-stream configuration

### T023 - Test: Unrecognized message handling [X]
**Story**: US1  
**File**: `market_data/framework/router/unrecognized_test.go`
- Test AS1.2: Send message with unknown type → routes to default handler
- Verify processing continues for subsequent valid messages
- Verify routing error metric incremented
**Dependencies**: T019, T020
**Exit Criteria**: System resilient to unknown types (FR-006)

### T024 - Benchmark: Routing hot path performance [X]
**Story**: US1  
**File**: `market_data/framework/router/benchmark_test.go`
- Benchmark complete routing pipeline: Detect → Lookup → Dispatch → Process
- Target: <5ms p95 end-to-end latency under 50K msg/sec sustained load
- Measure allocations: zero on Lookup() path (PERF-03)
**Dependencies**: T020
**Constitution**: PERF-01 (<10ms p99 latency), PERF-03 (zero allocations)
**Exit Criteria**: 
- SC-002 validated: <5ms p95 latency under 50,000 mixed messages/sec sustained for 60 seconds minimum
- Memory usage stable over 5-minute benchmark run (no leaks; heap growth <10% after initial ramp)
- GC pressure acceptable (pause times <1ms p99)
- Benchmark runs with -race detector enabled; zero data races detected

### T025 - Integration test: Backpressure under load [X]
**Story**: US1  
**File**: `market_data/framework/router/backpressure_test.go`
- Simulate slow processor (processing slower than message arrival)
- Verify backpressure blocks upstream (no messages dropped)
- Verify system recovers when processor catches up
**Dependencies**: T010, T013, T020
**Exit Criteria**: FR-012 validated (backpressure applied correctly)

### T026 - User Story 1 acceptance validation [X]
**Story**: US1  
**File**: `market_data/framework/router/acceptance_test.go`
- Run all AS1.1, AS1.2, AS1.3 acceptance scenarios
- Verify independent test criteria: mixed stream routing with zero cross-contamination
- Generate coverage report: target 85%+ for router package
**Dependencies**: T021-T023
**Constitution**: TS-01 (80%+ coverage)
**Exit Criteria**: User Story 1 complete; all P1 acceptance scenarios pass

**🎯 MVP Milestone**: User Story 1 complete - core routing capability delivered

---

## Phase 4: User Story 2 (P1) - Convert Payloads to Typed Models (6 tasks)

**Story Goal**: Ensure raw messages automatically convert to strongly-typed domain models with validation.

**Independent Test**: Send raw messages for each event type; verify typed instances with all fields correctly populated (AS2).

### T027 [P] - Add comprehensive validation to TradeProcessor [X]
**Story**: US2  
**File**: `market_data/processors/trade.go`
- Add schema validation for required fields (price, quantity, side, trade_id)
- Validate price/quantity are positive *big.Rat values
- Validate side is valid OrderSide enum
- Return validation errors with field-specific diagnostic info
**Dependencies**: T015
**Constitution**: CQ-06 (typed errors with field context)
**Exit Criteria**:
- Test AS2.1: Valid trade → all fields correctly typed
- Test: Missing required field → validation error with field name
- Test: Negative price → validation error

### T028 [P] - Add comprehensive validation to OrderBookProcessor [X]
**Story**: US2  
**File**: `market_data/processors/orderbook.go`
- Validate bids/asks arrays present
- Validate each price level has positive price and quantity
- Ensure arbitrary-precision preserved (no float conversion)
**Dependencies**: T016
**Exit Criteria**:
- Test AS2.2: Valid orderbook → typed BookDepthLevel slices with *big.Rat preservation
- Test: Empty bids/asks → validation error

### T029 [P] - Add comprehensive validation to AccountProcessor [X]
**Story**: US2  
**File**: `market_data/processors/account.go`
- Validate asset identifiers non-empty
- Validate total/available amounts are non-negative *big.Rat
- Validate available <= total
**Dependencies**: T017
**Exit Criteria**:
- Test AS2.3: Valid account update → typed AccountBalance with *big.Rat amounts
- Test: Invalid balance (available > total) → validation error

### T030 - Test: Malformed payload handling [X]
**Story**: US2  
**File**: `market_data/processors/malformed_test.go`
- Test AS2.4: Send malformed JSON → processor returns error with diagnostic info
- Verify processing continues for subsequent valid messages
- Verify processor invocation error metric incremented
**Dependencies**: T027-T029
**Exit Criteria**: FR-005 validated (graceful error handling)

### T031 - Integration test: Typed model correctness [X]
**Story**: US2  
**File**: `market_data/processors/integration_test.go`
- For each processor: Parse real exchange message fixtures
- Assert output matches expected typed model exactly
- Verify *big.Rat precision preserved (no rounding errors)
- Test decimal edge cases: very large numbers, microcents, satoshis
**Dependencies**: T027-T029
**Constitution**: CQ-03 (zero floating-point)
**Exit Criteria**: All typed models correctly populated from real messages

### T032 - User Story 2 acceptance validation [X]
**Story**: US2  
**File**: `market_data/processors/acceptance_test.go`
- Run all AS2.1, AS2.2, AS2.3, AS2.4 acceptance scenarios
- Verify independent test criteria: raw → typed conversion with validation
- Generate coverage report: target 85%+ for processors package
**Dependencies**: T027-T031
**Constitution**: TS-01 (80%+ coverage)
**Exit Criteria**: User Story 2 complete; all typed conversions validated

**🎯 Milestone**: User Story 1 + 2 complete - Full P1 routing and conversion delivered

---

## Phase 5: User Story 3 (P2) - Adopt Four-Level Architecture (12 tasks)

**Story Goal**: Migrate binance adapter to use framework router; establish strict L1/L2/L3/L4 boundaries.

**Independent Test**: Run binance integration tests; verify 100% pass without modification (AS3.3). Architectural review confirms layer boundaries (AS3.2).

### T033 - Audit current binance routing layer [X]
**Story**: US3  
**File**: `exchanges/binance/routing/` (analysis task)
- Document current routing logic in dispatchers.go
- Identify L1/L2/L3 boundary violations
- Create refactoring plan: what stays in L2, what moves to framework router
**Dependencies**: None (analysis)
**Exit Criteria**: Refactoring strategy documented; boundary violations identified

### T034 - Extract binance message type detection rules [X]
**Story**: US3  
**File**: `exchanges/binance/routing/message_types.go` (new file)
- Define Binance-specific MessageTypeDescriptor configurations
- Map Binance stream types to framework message type IDs
- Document field paths ("e", "stream") used for detection
**Dependencies**: T003, T033
**Exit Criteria**: Binance detection rules documented; ready for framework registration

### T035 - Create BinanceProcessorAdapter for existing parsers [X]
**Story**: US3  
**File**: `exchanges/binance/routing/processor_adapters.go`
- Wrap existing parse functions in Processor interface
- Implement Initialize() (no-op for stateless parsers)
- Implement Process() calling existing parse logic
- Implement MessageTypeID() returning Binance type
**Dependencies**: T004, T034
**Exit Criteria**: Existing Binance parsers usable with framework router

### T036 - Replace WSRouter with framework RoutingTable [X]
**Story**: US3  
**File**: `exchanges/binance/routing/ws_router.go` (refactor)
- Replace custom routing logic with RoutingTable.Detect() + Dispatch()
- Keep PublicDispatcher/PrivateDispatcher for Binance-specific dispatch logic (L2)
- Remove parse functions (now in processor adapters)
**Dependencies**: T020, T035
**Exit Criteria**: WSRouter delegates to framework; Binance-specific logic preserved

### T037 - Move filter logic to dedicated L4 package [X]
**Story**: US3  
**File**: `exchanges/binance/filter/` (refactor existing)
- Ensure filter package only contains policy filters (throttle, aggregate, enrich)
- Remove any routing or parsing logic (belongs in L2/L3)
- Document L4 responsibilities per architecture
**Dependencies**: T033
**Exit Criteria**: Filter package pure L4 (no routing/parsing)

### T038 - Update binance plugin integration [X]
**Story**: US3  
**File**: `exchanges/binance/plugin/plugin.go`
- Wire framework router into binance adapter initialization
- Register Binance message types and processors
- Preserve existing public interfaces (backward compatibility per FR-009)
**Dependencies**: T035, T036
**Exit Criteria**: Binance adapter initializes with framework router

### T039 - Test: Binance routing with framework [X]
**Story**: US3  
**File**: `exchanges/binance/routing/framework_integration_test.go`
- Test Binance message samples route correctly through framework
- Test public/private stream separation preserved
- Test listen key keepalive still functions
**Dependencies**: T036, T038
**Exit Criteria**: Binance-specific routing works with framework

### T040 - Run existing binance integration tests [X]
**Story**: US3  
**File**: `exchanges/binance/` (existing test suite)
- Test AS3.3: Run full binance test suite without modification
- Target: 100% pass rate
- Identify any failures; trace to refactoring issues
**Dependencies**: T038
**Exit Criteria**: SC-003 validated (100% existing tests pass)

### T041 - Architecture compliance validation [X]
**Story**: US3  
**File**: `tests/architecture_test.go`
- Test AS3.2: Verify each component maps to exactly one level (L1/L2/L3/L4)
- Check for cross-level dependencies (imports violating boundaries)
- Generate architecture diagram showing layer separation
**Dependencies**: T036, T037
**Exit Criteria**: SC-004 validated (architectural review confirms boundaries)

### T042 - Performance regression test [X]
**Story**: US3  
**File**: `exchanges/binance/performance_test.go`
- Test AS3.1: Measure message throughput with framework vs old implementation
- Verify 10 concurrent streams maintain throughput (no degradation)
- Benchmark latency: should meet or exceed previous performance
**Dependencies**: T038
**Exit Criteria**: No performance regression; SC-005 readiness (zero degradation)

### T043 - Telemetry layer separation [X]
**Story**: US3  
**File**: `exchanges/binance/telemetry/metrics.go`
- Test AS3.4: Verify metrics report per-layer (connection health L1, routing latency L2, etc.)
- Ensure framework metrics integrated with binance telemetry
- Update dashboards to show layer breakdown
**Dependencies**: T011, T038
**Exit Criteria**: Observability reflects L1/L2/L3/L4 separation

### T044 - User Story 3 acceptance validation [X]
**Story**: US3  
**File**: `exchanges/binance/acceptance_test.go`
- Run all AS3.1, AS3.2, AS3.3, AS3.4 acceptance scenarios
- Verify independent test criteria: architecture compliance + backward compatibility
- Document layer boundaries in architecture diagram
**Dependencies**: T040-T043
**Exit Criteria**: User Story 3 complete; binance adapter conforms to architecture

**🎯 Milestone**: User Story 3 complete - Architecture migration successful

---

## Phase 6: User Story 4 (P2) - Preserve Existing Functionality (6 tasks)

**Story Goal**: Validate that framework replacement maintains all current exchange adapter capabilities with zero behavior changes.

**Independent Test**: Run full exchange integration and smoke tests; verify session management, auth, reconnection work identically (AS4).

### T045 - Session lifecycle validation [X]
**Story**: US4  
**File**: `exchanges/binance/session_test.go`
- Test AS4.1: Start with framework → session established and kept alive
- Test periodic keepalive continues without interruption
- Test private stream stays connected for sustained period (30+ minutes)
**Dependencies**: T038
**Exit Criteria**: Session management unchanged from previous implementation

### T046 - Authentication validation [X]
**Story**: US4  
**File**: `exchanges/binance/auth_test.go`
- Test AS4.2: Authenticated requests generate correct signatures
- Test signature validation with Binance API
- Test listen key creation/refresh still functional
**Dependencies**: T038
**Exit Criteria**: All authenticated operations work identically

### T047 - Reconnection logic validation [X]
**Story**: US4  
**File**: `exchanges/binance/reconnection_test.go`
- Test AS4.3: Simulate connection drop → verify exponential backoff reconnection
- Test subscription resumption after reconnect
- Test message ordering preserved across reconnection
**Dependencies**: T038
**Exit Criteria**: Reconnection behavior identical to previous implementation

### T048 - Subscription lifecycle validation [X]
**Story**: US4  
**File**: `exchanges/binance/subscription_test.go`
- Test AS4.4: Subscribe → receive confirmation → messages flow
- Test unsubscribe → receive confirmation → messages stop
- Test subscription sequencing (multiple subscribe/unsubscribe operations)
**Dependencies**: T038
**Exit Criteria**: Subscription management unchanged

### T049 - Production smoke test suite [X]
**Story**: US4  
**File**: `exchanges/binance/smoke_test.go`
- Run smoke tests covering: connection, authentication, streaming, reconnection
- Execute against Binance testnet
- Measure: error rates, reconnection frequency, message loss
- Compare metrics to baseline from previous implementation
**Dependencies**: T045-T048
**Exit Criteria**: Smoke tests pass; metrics match baseline

### T050 - User Story 4 acceptance validation [X]
**Story**: US4  
**File**: `exchanges/binance/acceptance_test.go`
- Run all AS4.1, AS4.2, AS4.3, AS4.4 acceptance scenarios
- Verify independent test criteria: functionality preserved with zero changes
- Document any behavioral differences (should be none)
**Dependencies**: T049
**Exit Criteria**: User Story 4 complete; all capabilities preserved

**🎯 Milestone**: User Story 4 complete - Full backward compatibility validated

---

## Phase 7: Polish & Cross-Cutting Concerns (3 tasks)

**Objective**: Final integration, documentation, and deployment readiness.

### T051 - Update developer documentation [X]
**File**: `market_data/README.md`, `exchanges/binance/README.md`
- Document new router package usage
- Update quickstart.md examples with framework integration
- Document migration from old routing to framework routing
- Add troubleshooting guide
**Dependencies**: All previous phases
**Exit Criteria**: Documentation complete and reviewed

### T052A - Deprecate legacy parser package [X]
**File**: `market_data/framework/parser/` (mark deprecated)
- Add deprecation notices to all public functions in `parser/` package:
  ```go
  // Deprecated: Use processors.TradeProcessor instead.
  // This package is maintained for backward compatibility only.
  ```
- Create migration guide: `market_data/MIGRATION.md` documenting parser → processor transition
- Update all internal imports to use `processors/` instead of `parser/`
- Run `go doc` to verify deprecation warnings visible
**Dependencies**: T015-T019 (new processors implemented and stable)
**Exit Criteria**: 
- All parser/ functions marked deprecated with migration guidance
- Zero internal usage of parser/ (only external consumers remain)
- Migration guide published
- CI warns on parser/ imports in new code (optional linter rule)

### T052 - Deployment cutover preparation [X]
**File**: `.github/workflows/`, deployment docs
- Create deployment checklist for single cutover (per FR-013)
- Document rollback procedure
- Prepare monitoring alerts for post-deployment
- Create runbook for operational issues
**Dependencies**: All previous phases
**Exit Criteria**: SC-005 readiness (zero downtime deployment plan validated)

**🎯 Final Milestone**: Feature complete - ready for production deployment

---

## Task Dependencies Graph

```
Phase 1 (Setup):
  T001 [P] ─┐
  T002 [P] ─┤
  T003 [P] ─┼─► Phase 2
  T004 [P] ─┤
  T005 [P] ─┤
  T006 [P] ─┘

Phase 2 (Foundational - BLOCKING):
  T003 ──► T007 ──► T009 ──┐
  T003 ──► T008 ──► T009 ──┤
  T007 ──► T010 ──────────┼──► T013A [P] ──┐
  T003 ──► T011 ──► T012 ──┤                ├──► T014 ──► Phase 3, 5, 6
  T010 ──► T013 ──────────┘                │
  Connection pkg ──► T013A [P] ────────────┘

Phase 3 (US1 - P1):
  T004,T006 ──► T015 [P] ─┐
  T004,T006 ──► T016 [P] ─┤
  T004,T006 ──► T017 [P] ─┼──► T020 ──► T021-T026
  T004,T006 ──► T018 [P] ─┤
  T004 ──────► T019 ──────┘

Phase 4 (US2 - P1):
  T015 ──► T027 [P] ─┐
  T016 ──► T028 [P] ─┼──► T030 ──► T031 ──► T032
  T017 ──► T029 [P] ─┘

Phase 5 (US3 - P2):
  - ──► T033 ──► T034 ──► T035 ──► T036 ──┐
  T033 ─────────────────────────► T037 ──┼──► T038 ──► T039-T044
                                          │
  T020,T035 ─────────────────────────────┘

Phase 6 (US4 - P2):
  T038 ──► T045 [P] ─┐
  T038 ──► T046 [P] ─┼──► T049 ──► T050
  T038 ──► T047 [P] ─┤
  T038 ──► T048 [P] ─┘

Phase 7 (Polish):
  All phases ──► T051, T052A, T052
```

**Critical Path**: T001-T006 → T007-T014 → T015-T020 → T021-T026 (36 days assuming 1 task/day, +1 for T013A)

---

## Parallel Execution Opportunities

### Phase 1 Setup (All Parallel):
- T001, T002, T003, T004, T005 can execute in parallel (different files)
- T006 depends on T001, T002 but can start once directories exist

### Phase 3 User Story 1:
- T015, T016, T017, T018 can execute in parallel (different processor files)
- Merge into T020 (registration)

### Phase 4 User Story 2:
- T027, T028, T029 can execute in parallel (enhancing separate processors)

### Phase 5 User Story 3:
- T034, T037 can execute in parallel (separate concerns)
- T039, T040, T041, T042, T043 can execute in parallel (different test files)

### Phase 6 User Story 4:
- T045, T046, T047, T048 can execute in parallel (different test aspects)

**Estimated Speedup**: 30-40% reduction in calendar time with 3-4 parallel developers

---

## Implementation Strategy

### MVP Scope (Minimum Viable Product)
**Recommended First Deliverable**: Phase 1-3 (Setup + Foundational + User Story 1)

**Rationale**:
- Delivers core routing capability (independently testable per US1)
- Enables downstream development without blocking
- Validates technical approach before architecture migration
- ~20 tasks, ~2-3 weeks with 2-3 developers

### Incremental Delivery Plan

1. **Sprint 1** (Setup + Foundational): T001-T014
   - Deliverable: Working router infrastructure
   - Gate: T014 integration test passes

2. **Sprint 2** (US1 + US2): T015-T032
   - Deliverable: Complete routing with typed conversion
   - Gate: SC-001, SC-002 validated (routing accuracy + performance)

3. **Sprint 3** (US3 + US4): T033-T050
   - Deliverable: Binance migration complete
   - Gate: SC-003, SC-004, SC-005 validated (backward compat + architecture + deployment)

4. **Sprint 4** (Polish): T051-T052
   - Deliverable: Production-ready release
   - Gate: All documentation complete; deployment checklist validated

### Testing Strategy

**Test Coverage Targets** (Constitution TS-01):
- Router package: 85%+ (core routing logic)
- Processors package: 85%+ (message conversion)
- Binance routing: 80%+ (adapter integration)

**Test Types**:
- Unit tests: T003, T007-T008, T011, T015-T019, T027-T029 (parallelizable)
- Integration tests: T014, T021-T023, T031, T039-T041 (sequential per story)
- Performance tests: T024, T042 (benchmark gates)
- Acceptance tests: T026, T032, T044, T050 (story validation)

---

## Success Validation Checklist

### User Story 1 (P1) - Route Mixed Message Types
- [X] T026: AS1.1 - 100 trades + 50 orderbooks routed without cross-contamination
- [X] T023: AS1.2 - Unrecognized messages route to default handler
- [X] T022: AS1.3 - Concurrent streams route correctly without interference
- [X] SC-001: 1,000 messages route 100% correctly

### User Story 2 (P1) - Convert Payloads to Typed Models
- [X] T027-T029: AS2.1-2.3 - Typed models correctly populated for trade/orderbook/account
- [X] T030: AS2.4 - Malformed payloads handled gracefully with diagnostic errors
- [X] SC-002: <5ms p95 latency under 50K msg/sec

### User Story 3 (P2) - Adopt Four-Level Architecture
- [X] T042: AS3.1 - 10 concurrent streams maintain throughput
- [X] T041: AS3.2 - Architecture review confirms L1/L2/L3/L4 boundaries
- [X] T040: AS3.3 - 100% existing tests pass without modification
- [X] T043: AS3.4 - Telemetry reports layer separation
- [X] SC-003, SC-004: Tests pass + architecture compliant

### User Story 4 (P2) - Preserve Existing Functionality
- [X] T045: AS4.1 - Session lifecycle unchanged
- [X] T046: AS4.2 - Authentication signatures valid
- [X] T047: AS4.3 - Reconnection logic identical
- [X] T048: AS4.4 - Subscription lifecycle unchanged
- [X] SC-005: Zero downtime deployment plan validated

### Final Validation
- [X] SC-006: Adding new message type takes <1 hour (quickstart.md validated)
- [X] All constitution gates pass (CQ-02, CQ-03, CQ-06, TS-01, PERF-01, PERF-03)
- [X] T051: Documentation complete
- [X] T052: Deployment runbook ready

---

## Risk Mitigation Checklist

| Risk | Mitigation Task | Status |
|------|-----------------|--------|
| Routing table contention | T007: sync.Map implementation | ✅ Mitigated in Phase 2 |
| Processor init failures | T007: Graceful degradation | ✅ Mitigated in Phase 2 |
| Performance regression | T024, T042: Benchmarks gate progress | Validated in Phase 3, 5 |
| Backward compatibility break | T040: 100% test pass requirement | Validated in Phase 5 |
| Architecture violations | T041: Automated boundary checks | Validated in Phase 5 |
| Deployment complexity | T052: Single cutover checklist | Prepared in Phase 7 |

---

**Total**: 54 tasks across 7 phases  
**MVP**: 21 tasks (Phase 1-3, including T013A)  
**Parallel Opportunities**: 16+ tasks can execute concurrently  
**Critical Path**: ~36 task-days  
**With Parallelization**: ~21-26 calendar days (3-4 developers)
