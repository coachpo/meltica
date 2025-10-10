# Data Model: Multi-Stream Message Router

**Feature**: Multi-Stream Message Router with Architecture Migration  
**Date**: 2025-01-09  
**Phase**: 1 - Design

## Overview

This document defines the core entities, relationships, and state transitions for the message routing system. All entities follow constitution requirements (CQ-03 zero floating-point, CQ-06 typed errors).

## Entity Catalog

### 1. MessageTypeDescriptor

**Purpose**: Identifies a category of messages with detection and routing configuration.

**Fields**:
```go
type MessageTypeDescriptor struct {
    ID              string          // Unique identifier: "trade", "orderbook", "account", "funding"
    DisplayName     string          // Human-readable: "Trade Event", "Order Book Update"
    DetectionRules  []DetectionRule // Ordered list of strategies (first match wins)
    ProcessorRef    string          // Links to ProcessorRegistration.MessageTypeID
    SchemaVersion   string          // Optional: "v1", "v2" for versioning
    CreatedAt       time.Time       // Registration timestamp
}
```

**Validation Rules**:
- `ID` must be non-empty, lowercase, alphanumeric with underscores
- `ID` must be unique within routing table
- `DetectionRules` must have at least one entry
- `ProcessorRef` must reference an existing processor

**Relationships**:
- MessageTypeDescriptor 1 → 1 ProcessorRegistration (via ProcessorRef)
- MessageTypeDescriptor 1 → N DetectionRule (composition)

**Invariants**:
- Once created, `ID` is immutable
- `DetectionRules` order determines match precedence (first match wins per clarifications)

---

### 2. DetectionRule

**Purpose**: Defines a strategy for extracting message type from raw payload.

**Fields**:
```go
type DetectionRule struct {
    Strategy      DetectionStrategy // Enum: FieldBased, SchemaBased, MetadataBased
    FieldPath     string            // For FieldBased: JSON path ("type", "e", "stream")
    ExpectedValue string            // For FieldBased: value to match ("trade", "aggTrade")
    SchemaURI     string            // For SchemaBased: JSON schema reference (future use)
    Priority      int               // Lower number = higher priority (0 = highest)
}

type DetectionStrategy int

const (
    DetectionStrategyFieldBased    DetectionStrategy = iota // Default from research
    DetectionStrategySchemaBased                            // Future extension
    DetectionStrategyMetadataBased                          // Stream context-based
)
```

**Validation Rules**:
- `Strategy` must be valid enum value
- For `FieldBased`: `FieldPath` and `ExpectedValue` must be non-empty
- For `SchemaBased`: `SchemaURI` must be valid URI (future)
- `Priority` must be >= 0

**Behavior**:
- Multiple rules allowed per MessageTypeDescriptor
- Rules evaluated in `Priority` order (0 first, then 1, 2, ...)
- First rule returning match determines message type (short-circuit evaluation)

---

### 3. ProcessorRegistration

**Purpose**: Associates a message type with its processor implementation and tracks operational status.

**Fields**:
```go
type ProcessorRegistration struct {
    MessageTypeID string           // Links to MessageTypeDescriptor.ID
    Processor     Processor        // Implementation satisfying Processor interface
    Status        ProcessorStatus  // Current operational state
    InitError     error            // Captured if Status == Unavailable
    RegisteredAt  time.Time        // Timestamp of registration
    InitDuration  time.Duration    // How long initialization took
}

type ProcessorStatus int

const (
    ProcessorStatusInitializing ProcessorStatus = iota // Transient during startup
    ProcessorStatusAvailable                           // Ready to process messages
    ProcessorStatusUnavailable                         // Init failed; routes to default
)

// String returns human-readable status
func (s ProcessorStatus) String() string {
    return [...]string{"initializing", "available", "unavailable"}[s]
}
```

**Validation Rules**:
- `MessageTypeID` must match a MessageTypeDescriptor.ID
- `Processor` must not be nil
- If `Status == Unavailable`, `InitError` must be set
- If `Status == Available`, `InitError` must be nil

**Relationships**:
- ProcessorRegistration N → 1 MessageTypeDescriptor (via MessageTypeID)
- ProcessorRegistration 1 → 1 Processor implementation

**State Transitions**:
```
Initializing → Available   (Initialize() succeeds)
Initializing → Unavailable (Initialize() fails or times out after 5s)
```

**Invariants**:
- Status never transitions from `Unavailable` back to `Available` without manual intervention
- Once `Available`, remains `Available` for lifetime of process

---

### 4. Processor (Interface)

**Purpose**: Defines contract for converting raw message bytes into typed domain models.

**Interface Definition**:
```go
type Processor interface {
    // Initialize prepares the processor for operation.
    // Called once during system startup with 5-second timeout.
    // Returns error if processor cannot become operational.
    Initialize(ctx context.Context) error
    
    // Process converts raw message bytes into a typed domain model.
    // Must respect context cancellation within 100ms.
    // Returns typed model (e.g., *TradePayload) or error if conversion fails.
    // MUST NOT panic on malformed input - return error instead.
    Process(ctx context.Context, raw []byte) (interface{}, error)
    
    // MessageTypeID returns the identifier this processor handles.
    // Must match ProcessorRegistration.MessageTypeID.
    MessageTypeID() string
}
```

**Implementation Requirements** (from processor-interface.yaml contract):
1. `Initialize()` must complete within 5 seconds or return error
2. `Process()` must handle malformed input gracefully (return error, never panic)
3. `Process()` must respect context cancellation within 100ms
4. `Process()` must use `*big.Rat` for all decimal fields (CQ-03 compliance)
5. `Process()` must not retain references to input `[]byte` after return (prevents memory leaks)
6. `MessageTypeID()` must be deterministic and match registration

**Example Implementations**:
- `TradeProcessor` → `*market_data.TradePayload`
- `OrderBookProcessor` → `*market_data.OrderBookPayload`
- `AccountProcessor` → `*market_data.AccountPayload`
- `DefaultProcessor` → `*market_data.RawPayload` (fallback for unrecognized types)

---

### 5. RoutingTable

**Purpose**: Central registry mapping message type IDs to processor registrations with concurrent-safe access.

**Fields**:
```go
type RoutingTable struct {
    entries         sync.Map                  // MessageTypeID → *ProcessorRegistration
    defaultProc     *ProcessorRegistration    // Fallback for unrecognized types
    descriptors     map[string]*MessageTypeDescriptor // MessageTypeID → descriptor
    descriptorsLock sync.RWMutex              // Protects descriptors map
    metrics         *RoutingMetrics           // Observability
}
```

**Methods**:
```go
// Register adds or replaces a processor for a message type.
// Idempotent: calling twice with same MessageTypeID replaces previous registration.
func (rt *RoutingTable) Register(desc *MessageTypeDescriptor, proc Processor) error

// Lookup retrieves processor registration for a message type.
// Returns defaultProc if messageTypeID not found (never nil).
// Lock-free, zero-allocation read operation.
func (rt *RoutingTable) Lookup(messageTypeID string) *ProcessorRegistration

// Detect determines message type from raw payload using descriptor detection rules.
// Returns MessageTypeID or empty string if no match.
func (rt *RoutingTable) Detect(raw []byte) (messageTypeID string, err error)

// GetMetrics returns a snapshot of routing counters.
func (rt *RoutingTable) GetMetrics() *RoutingMetrics

// SetDefault assigns the fallback processor for unrecognized types.
func (rt *RoutingTable) SetDefault(proc Processor) error
```

**Validation Rules**:
- `defaultProc` must never be nil (enforced by constructor)
- `Lookup()` guarantees non-nil return (returns `defaultProc` if not found)

**Relationships**:
- RoutingTable 1 → N ProcessorRegistration (via `entries` sync.Map)
- RoutingTable 1 → 1 ProcessorRegistration (default processor)
- RoutingTable 1 → N MessageTypeDescriptor (via `descriptors` map)

**Concurrency Guarantees**:
- `Lookup()` is lock-free via sync.Map (supports 100+ concurrent goroutines per research)
- `Register()` is safe for concurrent calls (sync.Map handles coordination)
- `Detect()` uses immutable detection rules (no locking needed once registered)

---

### 6. RoutingMetrics

**Purpose**: Captures observability data for routing operations (FR-010 requirement).

**Fields**:
```go
type RoutingMetrics struct {
    // MessagesRouted tracks count per message type
    MessagesRouted map[string]uint64 // MessageTypeID → count
    
    // RoutingErrors counts unrecognized types and detection failures
    RoutingErrors uint64
    
    // ProcessorInvocations tracks calls per processor with success/error breakdown
    ProcessorInvocations map[string]*ProcessorInvocationMetrics
    
    // ConversionDurations tracks p95 latency per processor
    ConversionDurations map[string]*LatencyHistogram
    
    // ProcessorInitFailures counts how many processors failed to initialize
    ProcessorInitFailures uint64
    
    mu sync.RWMutex // Protects all fields for concurrent updates
}

type ProcessorInvocationMetrics struct {
    Successes uint64
    Errors    uint64
}

type LatencyHistogram struct {
    Samples []time.Duration
    P50     time.Duration
    P95     time.Duration
    P99     time.Duration
}
```

**Methods**:
```go
// RecordRoute increments counter for a message type
func (m *RoutingMetrics) RecordRoute(messageTypeID string)

// RecordError increments routing error counter
func (m *RoutingMetrics) RecordError()

// RecordProcessing records processor invocation and latency
func (m *RoutingMetrics) RecordProcessing(messageTypeID string, duration time.Duration, err error)

// Snapshot returns a deep copy of current metrics (thread-safe)
func (m *RoutingMetrics) Snapshot() *RoutingMetrics
```

**Validation Rules**:
- All counters monotonically increase (never decrement)
- P95/P99 calculated from rolling window (last 1000 samples per message type)

**Export Format** (for FR-010 observability):
```
# Prometheus-style metrics
routing_messages_total{message_type="trade"} 1500
routing_messages_total{message_type="orderbook"} 800
routing_errors_total 23
processor_invocations_total{message_type="trade",status="success"} 1495
processor_invocations_total{message_type="trade",status="error"} 5
processor_conversion_duration_seconds{message_type="trade",quantile="0.95"} 0.003
```

---

## Entity Relationships Diagram

```
┌────────────────────────────┐
│  MessageTypeDescriptor     │
│  ├─ ID                     │
│  ├─ DetectionRules[]       │────┐
│  └─ ProcessorRef           │    │
└────────────┬───────────────┘    │
             │ 1                  │ N
             │                    ▼
             │            ┌─────────────────┐
             │            │ DetectionRule   │
             │            │ ├─ Strategy     │
             │            │ ├─ FieldPath    │
             │            │ └─ Priority     │
             │            └─────────────────┘
             │ 1
             ▼ 1
┌──────────────────────────────┐
│  ProcessorRegistration       │
│  ├─ MessageTypeID            │
│  ├─ Processor ◄──────────────┼─── implements ───┐
│  ├─ Status                   │                   │
│  └─ InitError                │                   │
└────────────┬─────────────────┘                   │
             │ N                                    │
             │                              ┌───────┴─────────┐
             ▼ 1                            │   «interface»   │
┌─────────────────────────────┐             │   Processor     │
│      RoutingTable           │             │   ├─ Initialize │
│  ├─ entries (sync.Map)      │             │   ├─ Process    │
│  ├─ defaultProc             │             │   └─ MessageTypeID
│  ├─ descriptors map         │             └────────┬────────┘
│  └─ metrics                 │                      │
└───────────┬─────────────────┘                      │ implements
            │ 1                                      │
            ▼ 1                                      │
┌─────────────────────────────┐             ┌────────▼─────────────┐
│     RoutingMetrics          │             │ TradeProcessor       │
│  ├─ MessagesRouted map      │             │ OrderBookProcessor   │
│  ├─ RoutingErrors           │             │ AccountProcessor     │
│  ├─ ProcessorInvocations    │             │ DefaultProcessor     │
│  └─ ConversionDurations     │             └──────────────────────┘
└─────────────────────────────┘
```

---

## State Transition Diagrams

### Processor Registration Lifecycle

```
     Register()
        │
        ▼
┌───────────────┐  Initialize() success   ┌──────────┐
│ Initializing  │─────────────────────────►│Available │
└───────────────┘                          └──────────┘
        │                                       │
        │ Initialize() fails                    │ (terminal state)
        │ or timeout (5s)                       └──► Process messages
        ▼
┌───────────────┐
│ Unavailable   │──► Route to DefaultProcessor
└───────────────┘
   (terminal state)
```

### Message Routing Flow

```
Raw Message arrives
    │
    ▼
┌──────────────┐
│ Detect Type  │──► No match ──► Route to DefaultProcessor ──► Process ──► Output
└──────┬───────┘
       │ Match found
       ▼
┌──────────────┐
│ Lookup Processor Registration
└──────┬───────┘
       │
       ├─► Status: Available ──► Process ──► Success ──► Output
       │                              └──► Error ──► Log + Metrics
       │
       └─► Status: Unavailable ──► Route to DefaultProcessor
```

### Backpressure Flow

```
Message arrives at Router
    │
    ▼
┌──────────────────┐
│ Select processor │
│ channel          │
└────────┬─────────┘
         │
         ▼
  Channel full?
         │
    ┌────┴────┐
    │         │
   No        Yes
    │         │
    │         ▼
    │   ┌──────────────┐
    │   │Block upstream│──► Processor catches up
    │   │(backpressure)│──► Channel ready
    │   └──────────────┘──► Resume
    │
    ▼
Send to processor channel
    │
    ▼
Processor reads and processes
```

---

## Data Integrity Constraints

1. **Referential Integrity**:
   - `MessageTypeDescriptor.ProcessorRef` must reference existing `ProcessorRegistration.MessageTypeID`
   - `DetectionRule` parent descriptor must exist (cascade delete if descriptor removed)

2. **Uniqueness Constraints**:
   - `MessageTypeDescriptor.ID` unique within RoutingTable
   - `ProcessorRegistration.MessageTypeID` unique within RoutingTable

3. **Nullability Constraints**:
   - `RoutingTable.defaultProc` never nil
   - `Processor` interface implementations never nil in ProcessorRegistration
   - `ProcessorRegistration.MessageTypeID` non-empty

4. **State Consistency**:
   - If `ProcessorStatus == Unavailable`, then `InitError != nil`
   - If `ProcessorStatus == Available`, then `InitError == nil`
   - Status transitions are irreversible (no `Unavailable → Available`)

5. **Concurrency Safety**:
   - RoutingTable.Lookup() safe from all goroutines (lock-free read)
   - RoutingMetrics updates protected by mutex
   - sync.Map handles concurrent Register() calls

---

## Performance Characteristics

| Operation | Complexity | Allocation | Target Latency |
|-----------|-----------|------------|----------------|
| RoutingTable.Lookup() | O(1) | 0 bytes | <100ns p99 |
| RoutingTable.Detect() | O(N) rules | ~500 bytes (goccy parse) | <1ms p95 |
| Processor.Process() | O(M) payload size | ~1KB (model allocation) | <3ms p95 |
| RoutingMetrics.Record() | O(1) | 0 bytes | <50ns |
| End-to-end routing | O(1 + N + M) | ~1.5KB | <5ms p95 (SC-002) |

**Memory Footprint** (100 concurrent streams, 10 message types):
- RoutingTable: ~10KB (10 descriptors × 1KB)
- ProcessorRegistrations: ~5KB (10 registrations × 500 bytes)
- Metrics: ~50KB (histograms for 10 types × 5KB)
- **Total**: ~65KB static + per-message allocations

---

## Validation Checklist

- [ ] All entities have clear purpose and validation rules
- [ ] Relationships documented with cardinality
- [ ] State transitions are deterministic and terminal states identified
- [ ] Concurrency guarantees explicit (lock-free vs protected)
- [ ] Performance targets measurable (latency, allocation)
- [ ] Constitution compliance verified (CQ-03 *big.Rat, CQ-06 errs.E)
- [ ] FR-001 through FR-013 requirements mapped to entities
- [ ] SC-001 through SC-006 success criteria testable against model
