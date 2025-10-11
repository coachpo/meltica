# Quickstart: Four-Layer Architecture

**Date**: 2025-01-20  
**Feature**: 008-architecture-requirements-req

## Overview

This guide helps developers understand and work with the four-layer architecture. Whether you're adding features, integrating exchanges, or migrating legacy code, this document provides the essential patterns and practices.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Layer Responsibilities](#layer-responsibilities)
3. [Working with Layer Interfaces](#working-with-layer-interfaces)
4. [Adding a New Exchange](#adding-a-new-exchange)
5. [Migrating Legacy Code](#migrating-legacy-code)
6. [Testing](#testing)
7. [Common Patterns](#common-patterns)
8. [Troubleshooting](#troubleshooting)

---

## Architecture Overview

The system uses a **four-layer architecture** with strict dependency rules:

```
┌─────────────────────────────────────┐
│  Layer 4: Filter                    │  ← Pipeline orchestration, filtering
├─────────────────────────────────────┤
│  Layer 3: Business                  │  ← Domain logic, state management
├─────────────────────────────────────┤
│  Layer 2: Routing                   │  ← Message routing, translation
├─────────────────────────────────────┤
│  Layer 1: Connection                │  ← Network I/O, protocols
└─────────────────────────────────────┘
```

**Dependency Rule**: Each layer can only depend on layers **below** it.
- ✅ Layer 3 can use Layer 2 and Layer 1
- ❌ Layer 2 **cannot** use Layer 3
- ✅ All layers can use `core/layers/` interfaces
- ✅ All layers can use cross-cutting concerns (logging, metrics)

---

## Layer Responsibilities

### Layer 1: Connection

**What it does**: Manages raw network connections

**Responsibilities**:
- Establish and maintain WebSocket/HTTP connections
- Handle reconnection logic with exponential backoff
- Manage connection lifecycle (connect, ping/pong, close)
- Report connection errors to Layer 2

**Package Locations**:
- `exchanges/*/infra/ws/` - WebSocket clients
- `exchanges/*/infra/rest/` - REST clients
- `exchanges/shared/infra/transport/` - Shared transport logic

**Example Usage**:
```go
// Create a connection
conn := ws.NewClient(config)

// Connect
if err := conn.Connect(ctx); err != nil {
    return err
}
defer conn.Close()

// Check status
if conn.IsConnected() {
    // Read messages
    msgType, data, err := conn.ReadMessage()
}
```

### Layer 2: Routing

**What it does**: Routes and translates messages

**Responsibilities**:
- Subscribe/unsubscribe to data streams
- Parse exchange-specific message formats
- Translate to normalized internal formats
- Route messages to appropriate handlers
- Manage listen keys for private streams

**Package Locations**:
- `exchanges/*/routing/` - Exchange-specific routers
- `exchanges/shared/routing/` - Shared routing logic

**Example Usage**:
```go
// Create router
router := routing.NewWSRouter(conn)

// Subscribe to order book updates
req := routing.SubscriptionRequest{
    Symbol: "BTC-USDT",
    Type:   "book",
}
router.Subscribe(ctx, req)

// Handle messages
router.OnMessage(func(msg NormalizedMessage) {
    // Process normalized message
})
```

### Layer 3: Business

**What it does**: Implements domain logic

**Responsibilities**:
- Process normalized messages with business rules
- Maintain domain state (order books, positions, etc.)
- Validate business requests
- Generate business-level events
- Handle error recovery

**Package Locations**:
- `exchanges/*/bridge/` - Exchange-specific business logic
- `exchanges/*/` - Top-level exchange services

**Example Usage**:
```go
// Create business service
bookService := binance.NewBookService(router)

// Process updates
bookService.OnBookChange(func(symbol string, book OrderBook) {
    // React to book changes
    log.Info("Book updated", "symbol", symbol, "bidTop", book.Bids[0])
})
```

### Layer 4: Filter

**What it does**: Orchestrates pipelines and applies filters

**Responsibilities**:
- Coordinate multi-exchange data sources
- Apply filters (throttle, aggregate, deduplicate)
- Normalize events across exchanges
- Provide consumer-facing API
- Handle pipeline-level errors

**Package Locations**:
- `pipeline/` - Pipeline coordination and stages

**Example Usage**:
```go
// Create pipeline
pipeline := pipeline.NewCoordinator(adapter, auth)

// Add filters
pipeline.AddFilter(pipeline.ThrottleStage(100 * time.Millisecond))
pipeline.AddFilter(pipeline.AggregateStage(maxDepth))

// Start pipeline
events, err := pipeline.Start(ctx)
for event := range events {
    // Consume filtered events
}
```

---

## Working with Layer Interfaces

### Using Interfaces Instead of Concrete Types

**✅ DO:**
```go
// Accept interfaces in function signatures
func ProcessMessages(router layers.Routing) error {
    router.Subscribe(ctx, req)
    // ...
}

// Declare fields as interfaces
type Service struct {
    conn   layers.Connection
    router layers.Routing
}
```

**❌ DON'T:**
```go
// Don't use concrete types from other layers
func ProcessMessages(router *binance.WSRouter) error { // Bad
    // ...
}
```

### Type-Specific Interfaces

For exchange-specific functionality, use type-specific interfaces:

```go
// Check if connection supports spot features
if spotConn, ok := conn.(layers.SpotConnection); ok {
    endpoint := spotConn.GetSpotEndpoint()
}

// Check if routing supports futures
if futuresRouter, ok := router.(layers.FuturesRouting); ok {
    futuresRouter.SubscribeFundingRate(ctx, symbol)
}
```

### Mocking for Tests

Use the reusable mocks in `tests/architecture/mocks` to isolate layers without hand-written stubs:

```go
import archmocks "github.com/coachpo/meltica/tests/architecture/mocks"

func TestConnectionUsage(t *testing.T) {
    ctx := context.Background()
    conn := archmocks.NewMockConnection()

    called := false
    conn.ConnectFn = func(context.Context) error {
        called = true
        return nil
    }

    require.NoError(t, conn.Connect(ctx))
    require.True(t, called)
}
```

See `tests/architecture/examples/` for full examples covering routing, business, and filter layers.

---

## Adding a New Exchange

Follow this checklist to integrate a new exchange:

### 1. Generate the Skeleton

```bash
scripts/new-exchange.sh newexchange
# Optional: custom output directory or overwrite existing files
scripts/new-exchange.sh newexchange ./custom/newexchange --force
```

This renders the templates in `internal/templates/exchange/templates/` (covering `infra/`, `routing/`, `bridge/`, `filter/`) into `./exchanges/newexchange/`.

### 2. Implement Layer 1: Connection

- WebSocket scaffold: `infra/ws/client.go`
- REST scaffold: `infra/rest/client.go`
- Replace the `TODO` sections with transport setup, heartbeat handling, rate limiting, and deadline enforcement.

### 3. Implement Layer 2: Routing

- WebSocket routing scaffold: `routing/ws_router.go`
- REST routing scaffold: `routing/rest_router.go`
- Translate `layers.SubscriptionRequest` / `layers.APIRequest` into exchange-specific topics and HTTP requests, emit `layers.NormalizedMessage` to registered handlers.

### 4. Implement Layer 3: Business Logic

- Business scaffold: `bridge/session.go`
- Wire any REST routing helpers, maintain `layers.BusinessState`, and orchestrate downstream operations.

### 5. Implement Layer 4: Filter / Pipeline Integration

- Filter scaffold: `filter/adapter.go`
- Register custom filters with the pipeline coordinator and close resources via the provided helper methods.

### 6. Add Tests

- Reuse the mocks in `tests/architecture/mocks`.
- Add architecture contract tests (Connection / Routing / Business / Filter) for the new implementations.
- Create isolated unit tests similar to those in `tests/architecture/examples/`.

### 7. Run Static Analysis

```bash
go vet -vettool=$(which layer-analyzer) ./exchanges/newexchange/...
```

---

## Migrating Legacy Code

### Migration Path

**Current State**: Legacy code doesn't use layer interfaces

**Target State**: All code uses layer interfaces and passes contract tests

**Strategy**: Bottom-up migration (Layer 1 → 4)

### Step 1: Identify Layer

Determine which layer your code belongs to:

| Code Location | Layer | Interface to Implement |
|---------------|-------|----------------------|
| `*/infra/ws/` or `*/infra/rest/` | L1: Connection | `layers.Connection` |
| `*/routing/` | L2: Routing | `layers.Routing` |
| `*/bridge/` or top-level services | L3: Business | `layers.Business` |
| `pipeline/` | L4: Filter | `layers.Filter` |

### Step 2: Create Interface Adapter (if needed)

If full refactor is too large, start with an adapter:

```go
// Adapter wraps legacy code
type ConnectionAdapter struct {
    legacy *oldpackage.LegacyClient
}

func (a *ConnectionAdapter) Connect(ctx context.Context) error {
    // Delegate to legacy code
    return a.legacy.Start()
}

func (a *ConnectionAdapter) Close() error {
    return a.legacy.Stop()
}

// ... implement other interface methods
```

### Step 3: Update Consumers

Replace concrete type usage with interface:

```go
// Before
type Service struct {
    client *oldpackage.LegacyClient
}

// After
type Service struct {
    conn layers.Connection
}

// Constructor accepts interface
func NewService(conn layers.Connection) *Service {
    return &Service{conn: conn}
}
```

### Step 4: Run Contract Tests

```go
func TestLegacyConnectionContract(t *testing.T) {
    adapter := &ConnectionAdapter{legacy: oldpackage.NewClient()}
    testConnectionContract(t, adapter)
}
```

### Step 5: Refactor Implementation (optional)

Once adapter works, optionally refactor to native interface:

```go
type Connection struct {
    // New implementation
}

func (c *Connection) Connect(ctx context.Context) error {
    // Direct implementation, no delegation
}
```

---

## Testing

### Unit Tests

Test each layer in isolation using mocks:

```go
func TestRoutingLayer(t *testing.T) {
    // Mock lower layer
    mockConn := &mockConnection{}
    
    // Test this layer
    router := routing.NewWSRouter(mockConn)
    err := router.Subscribe(ctx, req)
    
    assert.NoError(t, err)
    assert.True(t, mockConn.subscribeCalled)
}
```

### Contract Tests

Verify behavioral contracts:

```go
func TestConnectionContract(t *testing.T) {
    impls := []layers.Connection{
        binanceWS.NewClient(config),
        coinbaseWS.NewClient(config),
    }
    
    for _, impl := range impls {
        t.Run(fmt.Sprintf("%T", impl), func(t *testing.T) {
            testIdempotency(t, impl)
            testCleanup(t, impl)
            testDeadlines(t, impl)
        })
    }
}
```

### Integration Tests

Test cross-layer interactions:

```go
func TestFullStack(t *testing.T) {
    // Create real implementations
    conn := ws.NewClient(config)
    router := routing.NewWSRouter(conn)
    service := bridge.NewSession(router)
    
    // Test full flow
    err := service.Start(ctx)
    assert.NoError(t, err)
    
    // Verify data flows through all layers
}
```

### Architecture Tests

Verify layer boundaries:

```go
// Run static analysis as test
func TestLayerBoundaries(t *testing.T) {
    analyzer := linter.NewAnalyzer()
    violations := analyzer.Check("./...")
    
    assert.Empty(t, violations, "Layer boundary violations found")
}
```

---

## Common Patterns

### Pattern 1: Dependency Injection

Pass dependencies as interfaces:

```go
type Service struct {
    conn   layers.Connection
    router layers.Routing
}

func NewService(conn layers.Connection, router layers.Routing) *Service {
    return &Service{
        conn:   conn,
        router: router,
    }
}
```

### Pattern 2: Factory Functions

Create factories that return interfaces:

```go
func NewBinanceConnection(config Config) layers.Connection {
    return &binanceConnection{config: config}
}
```

### Pattern 3: Interface Composition

Build complex interfaces from simple ones:

```go
type Exchange interface {
    layers.Connection
    layers.Routing
    layers.Business
}
```

### Pattern 4: Type Assertions for Extensions

Check for optional features:

```go
func DoSomething(conn layers.Connection) {
    // Use base interface features
    conn.Connect(ctx)
    
    // Check for spot-specific features
    if spotConn, ok := conn.(layers.SpotConnection); ok {
        endpoint := spotConn.GetSpotEndpoint()
    }
}
```

### Pattern 5: Cross-Cutting Concerns

Access global singletons from any layer:

```go
import "github.com/coachpo/meltica/internal/observability"

func (c *Connection) Connect(ctx context.Context) error {
    observability.Log().Info("Connecting", 
        observability.Field("exchange", c.name))
    
    // ... connection logic
    
    observability.Metrics().Increment("connections.established")
}
```

---

## Troubleshooting

### Static Analysis Errors

**Error**: `layer boundary violation: cannot import X from Y`

**Solution**: Check layer assignment and allowed imports

```bash
# Check which layer a package belongs to
go run internal/linter/analyzer.go -package=./exchanges/binance/routing

# See allowed imports for that layer
go run internal/linter/analyzer.go -rules
```

**Fix**: Refactor to use interface from `core/layers/` instead of direct import

### Interface Not Satisfied

**Error**: `type X does not implement interface Y (missing method Z)`

**Solution**: Implement all interface methods

```bash
# Check which methods are missing
go build ./...
```

**Fix**: Add missing methods to your type

### Circular Dependencies

**Error**: `import cycle not allowed`

**Solution**: Move shared types to `core/` or use interfaces

**Fix**:
```go
// Before (cycle)
// routing imports bridge
// bridge imports routing

// After (no cycle)
// routing imports core/layers
// bridge imports core/layers
// both use interface types
```

### Contract Test Failures

**Error**: Contract test fails but type checks pass

**Solution**: Fix behavioral issue, not just signature

**Example**:
```go
// Problem: Connect not idempotent
func (c *Connection) Connect(ctx context.Context) error {
    if c.connected {
        return errors.New("already connected")  // ❌ Violates contract
    }
    // ...
}

// Fix: Make idempotent
func (c *Connection) Connect(ctx context.Context) error {
    if c.connected {
        return nil  // ✅ No-op on subsequent calls
    }
    // ...
}
```

---

## Next Steps

1. **Read the full specification**: [spec.md](./spec.md)
2. **Review layer interfaces**: [contracts/layer_interfaces.go](./contracts/layer_interfaces.go)
3. **Check the data model**: [data-model.md](./data-model.md)
4. **Review research decisions**: [research.md](./research.md)

---

## Questions?

- Layer responsibility unclear? → Review [Layer Responsibilities](#layer-responsibilities)
- Can't satisfy interface? → Check [Working with Layer Interfaces](#working-with-layer-interfaces)
- Migration issues? → Follow [Migrating Legacy Code](#migrating-legacy-code)
- Tests failing? → See [Testing](#testing) and [Troubleshooting](#troubleshooting)
