# Quickstart: Adding a New Message Type

**Target Audience**: Developers extending the routing framework with new message types  
**Time to Complete**: ~30-60 minutes  
**Prerequisites**: Familiarity with Go, understanding of `market_data/events.go` domain models

## Overview

This guide walks you through adding a new message type ("liquidation") to the routing framework. You'll learn how to:
1. Define a typed payload model
2. Implement the Processor interface
3. Register the message type with the router
4. Test end-to-end routing

## Step 1: Define Typed Payload Model

Create or extend the domain model in `market_data/events.go`.

### Example: Liquidation Event

```go
// In market_data/events.go

type LiquidationPayload struct {
    Symbol       string    // Canonical symbol (BTC-USDT)
    Side         core.OrderSide // Buy or Sell
    Quantity     *big.Rat  // Liquidated quantity (arbitrary precision)
    Price        *big.Rat  // Liquidation price (arbitrary precision)
    Timestamp    time.Time // Event timestamp
    LiquidationID string   // Exchange-specific liquidation ID
}

// Implement EventPayload marker interface
func (*LiquidationPayload) eventPayload() {}
```

**Key Requirements**:
- ✅ Use `*big.Rat` for all decimal values (CQ-03 constitution requirement)
- ✅ Use canonical symbols (BASE-QUOTE uppercase)
- ✅ Implement `eventPayload()` marker method
- ❌ No `float32/float64` types (CI will fail)

---

## Step 2: Implement Processor Interface

Create a new processor in `market_data/processors/liquidation.go`.

### Processor Template

```go
package processors

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/coachpo/meltica/core"
    "github.com/coachpo/meltica/errs"
    "github.com/coachpo/meltica/market_data"
)

// LiquidationProcessor converts raw liquidation messages to typed payloads
type LiquidationProcessor struct {
    // Add any configuration or state here (must be thread-safe)
}

// NewLiquidationProcessor creates a new processor instance
func NewLiquidationProcessor() *LiquidationProcessor {
    return &LiquidationProcessor{}
}

// Initialize prepares the processor (called once at startup)
func (p *LiquidationProcessor) Initialize(ctx context.Context) error {
    // Fast initialization only - must complete within 5 seconds
    // Example: Validate configuration, load schemas
    
    select {
    case <-ctx.Done():
        return ctx.Err() // Respect cancellation
    default:
        // Your init logic here
        return nil
    }
}

// Process converts raw bytes to LiquidationPayload
func (p *LiquidationProcessor) Process(ctx context.Context, raw []byte) (interface{}, error) {
    // Check for cancellation (optional for fast processing)
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }
    
    // Parse JSON envelope
    var envelope struct {
        Symbol    string `json:"symbol"`
        Side      string `json:"side"`
        Quantity  string `json:"quantity"`
        Price     string `json:"price"`
        Timestamp int64  `json:"timestamp"` // milliseconds
        LiqID     string `json:"liquidation_id"`
    }
    
    if err := json.Unmarshal(raw, &envelope); err != nil {
        return nil, errs.New(errs.CodeInvalid, "liquidation_parse_failed", err).
            WithValue("raw_length", len(raw))
    }
    
    // Validate and convert fields
    canonical, err := core.CanonicalSymbolFor(envelope.Symbol)
    if err != nil {
        return nil, errs.New(errs.CodeInvalid, "liquidation_invalid_symbol", err).
            WithValue("symbol", envelope.Symbol)
    }
    
    side, err := parseSide(envelope.Side)
    if err != nil {
        return nil, errs.New(errs.CodeInvalid, "liquidation_invalid_side", err).
            WithValue("side", envelope.Side)
    }
    
    quantity, err := parsePrecise("quantity", envelope.Quantity)
    if err != nil {
        return nil, err // Already wrapped with errs.E
    }
    
    price, err := parsePrecise("price", envelope.Price)
    if err != nil {
        return nil, err
    }
    
    // Build payload
    payload := &market_data.LiquidationPayload{
        Symbol:        canonical,
        Side:          side,
        Quantity:      quantity,
        Price:         price,
        Timestamp:     time.Unix(0, envelope.Timestamp*int64(time.Millisecond)),
        LiquidationID: envelope.LiqID,
    }
    
    return payload, nil
}

// MessageTypeID returns the identifier for this processor
func (p *LiquidationProcessor) MessageTypeID() string {
    return "liquidation"
}

// Helper: Parse arbitrary-precision decimal
func parsePrecise(field, value string) (*big.Rat, error) {
    trimmed := strings.TrimSpace(value)
    if trimmed == "" {
        return nil, errs.New(errs.CodeInvalid, "liquidation_empty_field",
            fmt.Errorf("field %s cannot be empty", field))
    }
    rat, ok := new(big.Rat).SetString(trimmed)
    if !ok {
        return nil, errs.New(errs.CodeInvalid, "liquidation_invalid_decimal",
            fmt.Errorf("field %s invalid rational %q", field, value))
    }
    return rat, nil
}

// Helper: Parse side (reuse from events.go if available)
func parseSide(raw string) (core.OrderSide, error) {
    switch strings.ToLower(strings.TrimSpace(raw)) {
    case "buy":
        return core.SideBuy, nil
    case "sell":
        return core.SideSell, nil
    default:
        return "", fmt.Errorf("unsupported side %q", raw)
    }
}
```

**Key Points**:
- ✅ Initialize() completes within 5 seconds or returns error
- ✅ Process() handles malformed input gracefully (no panics)
- ✅ Process() respects context cancellation
- ✅ All errors use `errs.New()` with canonical codes
- ✅ Returns `*big.Rat` for decimals
- ✅ MessageTypeID() matches registration

---

## Step 3: Write Unit Tests

Create `market_data/processors/liquidation_test.go`.

### Test Template

```go
package processors

import (
    "context"
    "math/big"
    "testing"
    "time"
    
    "github.com/coachpo/meltica/core"
    "github.com/coachpo/meltica/market_data"
)

func TestLiquidationProcessor_Process_Success(t *testing.T) {
    processor := NewLiquidationProcessor()
    if err := processor.Initialize(context.Background()); err != nil {
        t.Fatalf("Initialize failed: %v", err)
    }
    
    raw := []byte(`{
        "symbol": "BTCUSDT",
        "side": "sell",
        "quantity": "1.5",
        "price": "50000.00",
        "timestamp": 1672531200000,
        "liquidation_id": "12345"
    }`)
    
    result, err := processor.Process(context.Background(), raw)
    if err != nil {
        t.Fatalf("Process failed: %v", err)
    }
    
    payload, ok := result.(*market_data.LiquidationPayload)
    if !ok {
        t.Fatalf("Expected *LiquidationPayload, got %T", result)
    }
    
    // Assertions
    if payload.Symbol != "BTC-USDT" {
        t.Errorf("Symbol = %s, want BTC-USDT", payload.Symbol)
    }
    if payload.Side != core.SideSell {
        t.Errorf("Side = %s, want Sell", payload.Side)
    }
    
    expectedQty := big.NewRat(15, 10) // 1.5
    if payload.Quantity.Cmp(expectedQty) != 0 {
        t.Errorf("Quantity = %s, want %s", payload.Quantity, expectedQty)
    }
    
    expectedPrice := big.NewRat(50000, 1)
    if payload.Price.Cmp(expectedPrice) != 0 {
        t.Errorf("Price = %s, want %s", payload.Price, expectedPrice)
    }
}

func TestLiquidationProcessor_Process_MalformedJSON(t *testing.T) {
    processor := NewLiquidationProcessor()
    processor.Initialize(context.Background())
    
    raw := []byte(`{invalid json`)
    
    _, err := processor.Process(context.Background(), raw)
    if err == nil {
        t.Fatal("Expected error for malformed JSON, got nil")
    }
    
    // Check error code
    if !strings.Contains(err.Error(), "liquidation_parse_failed") {
        t.Errorf("Expected liquidation_parse_failed error, got %v", err)
    }
}

func TestLiquidationProcessor_Process_ContextCancelled(t *testing.T) {
    processor := NewLiquidationProcessor()
    processor.Initialize(context.Background())
    
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately
    
    raw := []byte(`{"symbol":"BTCUSDT","side":"buy","quantity":"1","price":"50000","timestamp":0,"liquidation_id":"123"}`)
    
    _, err := processor.Process(ctx, raw)
    if err == nil {
        t.Fatal("Expected context.Canceled error, got nil")
    }
    if err != context.Canceled {
        t.Errorf("Expected context.Canceled, got %v", err)
    }
}

func BenchmarkLiquidationProcessor(b *testing.B) {
    processor := NewLiquidationProcessor()
    processor.Initialize(context.Background())
    
    raw := []byte(`{"symbol":"BTCUSDT","side":"sell","quantity":"1.5","price":"50000.00","timestamp":1672531200000,"liquidation_id":"12345"}`)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := processor.Process(context.Background(), raw)
        if err != nil {
            b.Fatal(err)
        }
    }
    // Target: <3ms per operation (p95)
}
```

**Run Tests**:
```bash
go test ./market_data/processors -v
go test ./market_data/processors -bench=. -benchmem
go test ./market_data/processors -race
```

**Coverage Target**: 80%+ for processor package

---

## Step 4: Register with Router

Update router configuration to include the new message type.

### Registration Code

```go
// In market_data/framework/router/registry.go or equivalent

func InitializeRouter() (*RoutingTable, error) {
    rt := NewRoutingTable()
    
    // Existing processors
    rt.Register(&MessageTypeDescriptor{
        ID: "trade",
        DetectionRules: []DetectionRule{{
            Strategy: DetectionStrategyFieldBased,
            FieldPath: "e",
            ExpectedValue: "trade",
        }},
    }, NewTradeProcessor())
    
    rt.Register(&MessageTypeDescriptor{
        ID: "orderbook",
        DetectionRules: []DetectionRule{{
            Strategy: DetectionStrategyFieldBased,
            FieldPath: "e",
            ExpectedValue: "depthUpdate",
        }},
    }, NewOrderBookProcessor())
    
    // NEW: Register liquidation processor
    rt.Register(&MessageTypeDescriptor{
        ID: "liquidation",
        DisplayName: "Liquidation Event",
        DetectionRules: []DetectionRule{{
            Strategy: DetectionStrategyFieldBased,
            FieldPath: "e",          // JSON field to inspect
            ExpectedValue: "forceOrder", // Binance liquidation event type
            Priority: 0,
        }},
        ProcessorRef: "liquidation",
        SchemaVersion: "v1",
    }, NewLiquidationProcessor())
    
    // Set default processor for unrecognized types
    rt.SetDefault(NewDefaultProcessor())
    
    return rt, nil
}
```

**Configuration Options**:
- Multiple detection rules per descriptor (evaluated by priority)
- Different field paths for different exchanges (`e` for Binance, `type` for Coinbase)
- SchemaVersion for future evolution

---

## Step 5: Integration Test

Test end-to-end routing from raw message to typed output.

### Integration Test

```go
// In market_data/framework/router/integration_test.go

func TestRouter_LiquidationRouting(t *testing.T) {
    // Setup router
    rt, err := InitializeRouter()
    if err != nil {
        t.Fatalf("InitializeRouter failed: %v", err)
    }
    
    // Raw liquidation message
    raw := []byte(`{
        "e": "forceOrder",
        "symbol": "BTCUSDT",
        "side": "sell",
        "quantity": "2.5",
        "price": "48000.00",
        "timestamp": 1672531200000,
        "liquidation_id": "67890"
    }`)
    
    // Detect type
    messageTypeID, err := rt.Detect(raw)
    if err != nil {
        t.Fatalf("Detect failed: %v", err)
    }
    if messageTypeID != "liquidation" {
        t.Errorf("Detected type = %s, want liquidation", messageTypeID)
    }
    
    // Lookup processor
    registration := rt.Lookup(messageTypeID)
    if registration.Status != ProcessorStatusAvailable {
        t.Fatalf("Processor unavailable: %v", registration.InitError)
    }
    
    // Process message
    result, err := registration.Processor.Process(context.Background(), raw)
    if err != nil {
        t.Fatalf("Process failed: %v", err)
    }
    
    // Verify typed output
    payload, ok := result.(*market_data.LiquidationPayload)
    if !ok {
        t.Fatalf("Expected *LiquidationPayload, got %T", result)
    }
    
    if payload.Symbol != "BTC-USDT" {
        t.Errorf("Symbol = %s, want BTC-USDT", payload.Symbol)
    }
    
    // Verify metrics incremented
    metrics := rt.GetMetrics()
    if metrics.MessagesRouted["liquidation"] != 1 {
        t.Errorf("MessagesRouted[liquidation] = %d, want 1",
            metrics.MessagesRouted["liquidation"])
    }
}

func TestRouter_MixedStream_IncludesLiquidation(t *testing.T) {
    rt, _ := InitializeRouter()
    
    messages := []struct {
        raw          []byte
        expectedType string
    }{
        {[]byte(`{"e":"trade","symbol":"BTCUSDT",...}`), "trade"},
        {[]byte(`{"e":"forceOrder","symbol":"ETHUSDT",...}`), "liquidation"},
        {[]byte(`{"e":"depthUpdate","symbol":"BTCUSDT",...}`), "orderbook"},
        {[]byte(`{"e":"forceOrder","symbol":"BTCUSDT",...}`), "liquidation"},
    }
    
    for i, msg := range messages {
        messageTypeID, err := rt.Detect(msg.raw)
        if err != nil {
            t.Errorf("Message %d: Detect failed: %v", i, err)
            continue
        }
        if messageTypeID != msg.expectedType {
            t.Errorf("Message %d: type = %s, want %s", i, messageTypeID, msg.expectedType)
        }
    }
    
    // Verify all liquidations routed
    metrics := rt.GetMetrics()
    if metrics.MessagesRouted["liquidation"] != 2 {
        t.Errorf("Routed liquidations = %d, want 2",
            metrics.MessagesRouted["liquidation"])
    }
}
```

---

## Step 6: Update Metrics Dashboard

Add liquidation metrics to observability dashboards.

### Prometheus Queries

```promql
# Liquidation message rate
rate(routing_messages_total{message_type="liquidation"}[5m])

# Liquidation processing latency (p95)
histogram_quantile(0.95,
  rate(processor_conversion_duration_seconds_bucket{message_type="liquidation"}[5m]))

# Liquidation error rate
rate(processor_invocations_total{message_type="liquidation",status="error"}[5m])
```

**Grafana Panel**: Add "liquidation" to existing routing dashboard filters.

---

## Checklist

Before considering the feature complete, verify:

- [ ] Payload model defined with `*big.Rat` for decimals
- [ ] Processor implements all interface methods correctly
- [ ] Unit tests pass with 80%+ coverage
- [ ] Benchmark shows <3ms p95 latency
- [ ] Race detector passes (`go test -race`)
- [ ] Integration test verifies end-to-end routing
- [ ] Message type registered in router configuration
- [ ] Metrics dashboard updated to include new type
- [ ] Documentation updated (godoc comments)
- [ ] Code review checklist from `processor-interface.yaml` satisfied

---

## Troubleshooting

### Processor Marked Unavailable

**Symptom**: Liquidation messages routed to default handler; processor status=unavailable

**Diagnosis**:
```bash
# Check logs for initialization error
grep "processor initialization failed" logs/app.log | grep liquidation

# Query processor status
curl localhost:8080/metrics | grep processor_status{message_type="liquidation"}
```

**Fix**: Check `Initialize()` implementation; ensure it completes within 5 seconds and doesn't return error.

### Detection Not Matching

**Symptom**: Messages not detected as "liquidation" type

**Diagnosis**:
```bash
# Inspect raw message
echo $RAW_MESSAGE | jq .e  # Should match "forceOrder"

# Check detection rules
curl localhost:8080/router/config | jq '.descriptors[] | select(.ID=="liquidation")'
```

**Fix**: Verify `DetectionRule.FieldPath` and `ExpectedValue` match actual message format.

### High Processing Latency

**Symptom**: p95 latency > 5ms

**Diagnosis**:
```bash
# Run benchmark with profiling
go test -bench=BenchmarkLiquidationProcessor -cpuprofile=cpu.prof
go tool pprof cpu.prof
```

**Fix**: Optimize hot paths (reduce allocations, use buffers, avoid `fmt.Sprintf`).

---

## Next Steps

- Add schema versioning support for liquidation events (v1, v2)
- Implement liquidation-specific filters in L4 (aggregation, throttling)
- Add alerting rules for abnormal liquidation volumes
- Extend to support other exchanges (Coinbase, FTX formats)

---

## References

- [Processor Interface Contract](./contracts/processor-interface.yaml)
- [Routing API Contract](./contracts/routing-api.yaml)
- [Metrics Schema](./contracts/metrics-schema.yaml)
- [Data Model](./data-model.md)
- [Research Decisions](./research.md)
