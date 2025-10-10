# Implementation Plan: Multi-Stream Message Router with Architecture Migration

**Branch**: `003-we-ve-built` | **Date**: 2025-01-09 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/003-we-ve-built/spec.md`

## Summary

Extend the existing lightweight streaming framework (market_data) with multi-message-type routing capabilities and migrate the binance exchange adapter to conform to the four-level architecture (Connection, Routing, Business, Filter). The framework will automatically detect message types (trades, order books, accounts) from mixed streams, route each to its appropriate processor, and convert raw payloads into strongly-typed domain models. This enables efficient handling of multiplexed WebSocket streams while maintaining architectural integrity and backward compatibility.

## Technical Context

**Language/Version**: Go 1.25  
**Primary Dependencies**: 
- `github.com/gorilla/websocket` (WebSocket client)
- `github.com/goccy/go-json` (fast JSON encoding/decoding)
- `math/big` (arbitrary-precision decimals)
- Internal `errs` package (typed errors)
- Standard library: `context`, `sync`, `testing`

**Storage**: In-memory routing tables with concurrent-safe maps; no persistent storage  
**Testing**: Go standard `testing` package, race detector (`-race`), benchmark tests  
**Target Platform**: Linux/macOS server environments  
**Project Type**: Single project (library/framework)  
**Performance Goals**: 
- 50,000 mixed messages/sec sustained throughput
- <5ms p95 latency from message receipt to typed output
- Zero allocations in routing hot path
- <200ms p95 for processor initialization

**Constraints**: 
- Must preserve backward compatibility with existing binance adapter public interfaces
- Frame reassembly must be transparent (L1 responsibility)
- Backpressure over message dropping when overloaded
- Single cutover deployment (no gradual rollout)

**Scale/Scope**: 
- 100+ concurrent streams per instance
- 10+ message types per stream
- 4 architectural levels with strict boundaries
- Replace ~5,000 LOC in exchanges/binance

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Quality Gates

| Gate | Status | Notes |
|------|--------|-------|
| **CQ-02: Layered Architecture Enforcement** | ⚠️ REQUIRES ATTENTION | Current binance implementation mixes layers; must refactor to strict L1/L2/L3/L4 separation |
| **CQ-03: Zero Floating-Point Policy** | ✅ COMPLIANT | Spec requires *big.Rat for all decimal values |
| **CQ-06: Typed Errors** | ✅ COMPLIANT | Must use `errs.E` with canonical codes for all routing/conversion errors |
| **TS-01: Minimum Coverage Thresholds** | ⚠️ REQUIRES ATTENTION | New routing logic must achieve 80%+ coverage; framework core 90%+ |
| **PERF-01: WebSocket Message Latency** | ✅ COMPLIANT | Spec target <5ms p95 aligns with <10ms p99 constitution requirement |
| **PERF-03: Memory Efficiency** | ✅ COMPLIANT | Must achieve zero allocations in routing hot path per constitution |

### Architecture Decision Record

**Decision**: Refactor binance adapter to four-level architecture with strict boundaries  
**Rationale**: CQ-02 mandates layered enforcement; current implementation violates this with business logic in routing  
**Alternatives Considered**: Gradual migration preserving current structure → Rejected: Technical debt compounds; violates constitution  
**Risk Mitigation**: Comprehensive test coverage (SC-003: 100% existing tests pass); single cutover after validation

## Project Structure

### Documentation (this feature)

```
specs/003-we-ve-built/
├── plan.md              # This file
├── research.md          # Phase 0: Routing strategies, backpressure patterns
├── data-model.md        # Phase 1: Routing table, message descriptors, processor registry
├── quickstart.md        # Phase 1: Developer guide for adding message types
├── contracts/           # Phase 1: Internal API contracts between layers
│   ├── routing-api.yaml
│   ├── processor-interface.yaml
│   └── metrics-schema.yaml
└── tasks.md             # Phase 2: NOT created by this command
```

### Source Code (repository root)

```
market_data/
├── framework/
│   ├── api/             # Existing: Public framework interfaces
│   ├── connection/      # Existing L1: Connection management
│   ├── router/          # NEW: L2 routing logic with type detection
│   │   ├── detector.go      # Message type detection strategies
│   │   ├── registry.go      # Processor registration and lookup
│   │   ├── dispatcher.go    # Message routing and dispatch
│   │   └── backpressure.go  # Flow control implementation
│   ├── handler/         # Existing: Business logic handlers (L3)
│   ├── parser/          # Existing: Message parsers (move to processors/)
│   └── telemetry/       # Existing: Metrics and observability
├── processors/          # NEW: Message type processors (replaces parser/)
│   ├── trade.go
│   ├── orderbook.go
│   ├── account.go
│   ├── funding.go
│   └── default.go       # Fallback for unrecognized types
├── events.go            # Existing: Domain models (TradePayload, etc.)
└── encoder.go           # Existing: JSON encoding utilities

exchanges/binance/
├── routing/             # REFACTOR: Move to pure L2 responsibilities
│   ├── dispatchers.go       # Existing: Public/private dispatchers
│   ├── stream_registry.go   # Existing: Stream type registration
│   └── parse_helpers.go     # Existing: Parsing utilities
├── infra/               # Existing L1: REST/WS clients
├── filter/              # REFACTOR: Move to L4 responsibilities
└── plugin/              # Integration point with market_data framework

tests/
├── market_data/
│   ├── router_test.go           # Unit: Type detection, routing logic
│   ├── processor_test.go        # Unit: Individual processor conversion
│   ├── integration_test.go      # Integration: End-to-end mixed streams
│   └── benchmark_test.go        # Performance: Routing hot path
└── exchanges/binance/
    ├── migration_test.go        # Validation: Existing tests pass
    └── architecture_test.go     # Validation: Layer boundary compliance
```

**Structure Decision**: Extend existing `market_data/framework` with new `router/` package for L2 routing logic. Create `processors/` as sibling to cleanly separate message type conversion from routing decisions. Refactor `exchanges/binance` to use framework router instead of custom routing logic, ensuring strict four-level boundaries.

## Complexity Tracking

*No constitution violations requiring justification - all gates align with feature requirements.*

## Phase 0: Research & Architecture

**Objective**: Resolve unknowns and establish routing architecture patterns.

### Research Topics

1. **Message Type Detection Strategies**
   - **Question**: How to efficiently detect message types from mixed streams?
   - **Approach**: Compare field-based (inspect "type" field), schema-based (JSON schema validation), metadata-based (stream context) strategies
   - **Success Criteria**: Select strategy with <1ms overhead; support first-match semantics from clarifications

2. **Backpressure Implementation Patterns**
   - **Question**: How to apply backpressure transparently to upstream sources?
   - **Approach**: Research Go channel blocking, context cancellation, and buffered channel strategies
   - **Success Criteria**: Mechanism blocks upstream without dropped messages; graceful under load

3. **Concurrent Routing Table Design**
   - **Question**: How to achieve zero-contention routing table lookups?
   - **Approach**: Evaluate `sync.RWMutex`, `sync.Map`, and immutable snapshot patterns
   - **Success Criteria**: Zero allocations on read path; support 100+ concurrent lookups

4. **Processor Registration Lifecycle**
   - **Question**: How to handle processor init failures gracefully?
   - **Approach**: Design health-check registry with unavailable marker state
   - **Success Criteria**: System starts with partial processor availability; routes failures to default handler

5. **Frame Reassembly Location**
   - **Question**: Confirm existing L1 (connection layer) handles WebSocket frame reassembly
   - **Approach**: Audit `market_data/framework/connection` for frame handling
   - **Success Criteria**: Document that gorilla/websocket provides complete messages; no framework changes needed

**Output**: `research.md` with decisions, rationale, alternatives, and code examples for each topic.

## Phase 1: Design & Contracts

**Objective**: Define data models, API contracts, and architectural boundaries.

### 1.1 Data Model (`data-model.md`)

**Entities**:

1. **MessageTypeDescriptor**
   - `ID string` - Unique identifier (e.g., "trade", "orderbook")
   - `DetectionRules []DetectionRule` - Ordered list of detection strategies
   - `ProcessorRef string` - Reference to registered processor
   - `SchemaVersion string` - Optional schema version

2. **DetectionRule**
   - `Strategy string` - "field" | "schema" | "metadata"
   - `FieldPath string` - For field strategy: JSON path (e.g., "$.type")
   - `ExpectedValue string` - For field strategy: value to match
   - `SchemaURI string` - For schema strategy: JSON schema reference

3. **ProcessorRegistration**
   - `MessageTypeID string` - Links to MessageTypeDescriptor
   - `Processor Processor` - Implementation interface
   - `Status ProcessorStatus` - "available" | "unavailable" | "initializing"
   - `InitError error` - Captured initialization error

4. **RoutingTable**
   - `Entries map[string]*ProcessorRegistration` - MessageTypeID → Registration
   - `DefaultProcessor Processor` - Fallback for unrecognized types
   - `lock sync.RWMutex` - Concurrent access control

5. **Processor** (interface)
   ```go
   type Processor interface {
       // Initialize prepares the processor; returns error if cannot start
       Initialize(ctx context.Context) error
       
       // Process converts raw bytes to typed model; returns error if conversion fails
       Process(ctx context.Context, raw []byte) (interface{}, error)
       
       // MessageTypeID returns the message type this processor handles
       MessageTypeID() string
   }
   ```

6. **RoutingMetrics**
   - `MessagesRouted map[string]uint64` - Count per message type
   - `RoutingErrors uint64` - Unrecognized or routing failures
   - `ProcessorInvocations map[string]uint64` - Count per processor
   - `ConversionDurations map[string]time.Duration` - p95 per processor

**State Transitions**:
- Processor: initializing → available | unavailable
- Message: received → type_detected → routed → processed | routed_to_default

**Validation Rules**:
- MessageTypeDescriptor.ID must be non-empty, unique
- DetectionRules list must have at least one entry
- RoutingTable.DefaultProcessor must always be available
- All decimal fields in typed models use *big.Rat (CQ-03 compliance)

### 1.2 API Contracts (`contracts/`)

**routing-api.yaml** (Internal contract between L2 Router and L3 Processors):
```yaml
Router:
  operations:
    - name: RegisterProcessor
      input: { messageTypeID: string, processor: Processor }
      output: { error: Error }
      semantics: Idempotent; replaces existing registration
    
    - name: RouteMessage
      input: { raw: []byte, streamContext: StreamContext }
      output: { messageType: string, processor: Processor, error: Error }
      semantics: First detection strategy match wins; returns DefaultProcessor for unrecognized
    
    - name: GetMetrics
      input: { since: time.Time }
      output: { metrics: RoutingMetrics }
      semantics: Thread-safe snapshot of counters

Processor:
  operations:
    - name: Initialize
      input: { ctx: context.Context }
      output: { error: Error }
      semantics: Called once at system startup; failures mark processor unavailable
    
    - name: Process
      input: { ctx: context.Context, raw: []byte }
      output: { model: interface{}, error: Error }
      semantics: Must preserve context cancellation; errors logged but don't crash router
```

**processor-interface.yaml** (Processor implementation requirements):
```yaml
Processor Implementation Checklist:
  - Initialize must complete within 5 seconds or return error
  - Process must handle malformed input gracefully (return error, don't panic)
  - Process must respect context cancellation within 100ms
  - Process must use *big.Rat for all decimal fields (CQ-03)
  - Process must not retain references to input []byte after return
  - MessageTypeID must match registration in routing table
```

**metrics-schema.yaml** (Observability contract for FR-010):
```yaml
RoutingMetrics:
  fields:
    messages_routed:
      type: map[string]uint64
      description: Count of messages routed per message type
      labels: [message_type]
    
    routing_errors:
      type: uint64
      description: Count of unrecognized or routing failures
    
    processor_invocations:
      type: map[string]uint64
      description: Count of processor calls per message type
      labels: [message_type, status: "success|error"]
    
    conversion_duration_p95:
      type: map[string]time.Duration
      description: 95th percentile conversion latency per processor
      labels: [message_type]
```

### 1.3 Quickstart Guide (`quickstart.md`)

**Target Audience**: Developers adding new message types  
**Structure**:
1. **Prerequisites**: Understand existing Event/Payload types in `market_data/events.go`
2. **Step 1: Define Typed Model** - Create payload struct with *big.Rat fields
3. **Step 2: Implement Processor** - Satisfy Processor interface
4. **Step 3: Register Message Type** - Add to routing table configuration
5. **Step 4: Test** - Unit test processor, integration test routing
6. **Example**: Walk through adding a new "liquidation" message type end-to-end

## Phase 2: Implementation Tasks

**Note**: Detailed task breakdown created by `/speckit.tasks` command - NOT included in this plan output.

**Summary of expected work**:
- Implement router package with detector, registry, dispatcher (est. 8-12 tasks)
- Create processor implementations for existing message types (est. 4-6 tasks)
- Refactor binance adapter to use framework router (est. 10-15 tasks)
- Establish L1/L2/L3/L4 boundaries with integration tests (est. 6-8 tasks)
- Migration validation and cutover preparation (est. 4-6 tasks)

**Total Estimate**: 35-50 implementation tasks spanning 2-3 sprints

## Success Validation

### Phase 0 Exit Criteria
- [ ] All 5 research topics documented in research.md with decisions
- [ ] Frame reassembly confirmed as L1 responsibility (no framework changes)
- [ ] Constitution re-check passes (layered architecture strategy defined)

### Phase 1 Exit Criteria
- [ ] Data model covers all entities in functional requirements
- [ ] API contracts define Router↔Processor interface completely
- [ ] Quickstart guide enables developer to add message type in <1 hour
- [ ] Agent context updated with new packages (router, processors)

### Feature Completion Criteria (from spec)
- [ ] **SC-001**: 1,000 mixed messages route 100% correctly without cross-contamination
- [ ] **SC-002**: <5ms p95 latency under 50K msg/sec sustained load
- [ ] **SC-003**: 100% existing binance tests pass without modification
- [ ] **SC-004**: Architecture review confirms strict L1/L2/L3/L4 boundaries
- [ ] **SC-005**: Production cutover with zero downtime
- [ ] **SC-006**: Adding new message type requires only config change

## Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Binance adapter refactor breaks existing functionality | Medium | Critical | SC-003 gate: 100% test pass; comprehensive integration testing before cutover |
| Routing table contention under load | Low | High | Phase 0 research: select lock-free concurrent map; benchmark with 100+ streams |
| Processor init failures cascade to system crash | Low | Medium | FR-004: Graceful degradation with unavailable marker; route to default handler |
| Backpressure blocks entire system | Medium | High | Phase 0 research: Implement per-stream backpressure; monitor queue depths |
| Frame reassembly assumption incorrect | Low | Critical | Phase 0 audit: Confirm gorilla/websocket guarantees; add integration test |

## Dependencies

### Upstream
- Gorilla WebSocket library must provide complete messages (frame reassembly)
- Existing binance integration tests must be comprehensive (SC-003 relies on this)

### Downstream
- Filter layer (L4) depends on typed models from processors
- Metrics/observability systems consume RoutingMetrics schema

### Parallel Work
- None - this is a standalone refactor with single cutover

## Next Steps

1. **Execute Phase 0**: Run research tasks to resolve detector strategy, backpressure pattern, and routing table design
2. **Generate Phase 1 artifacts**: Create data-model.md, contracts/, and quickstart.md based on research decisions
3. **Update agent context**: Run `.specify/scripts/bash/update-agent-context.sh claude` to register new packages
4. **Proceed to tasking**: Run `/speckit.tasks` to break down implementation work into actionable tasks
