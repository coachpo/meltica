# ParseFrame Rename Summary

Final rename from `ProviderRaw` → `ParserBuffer` → `ParseFrame`

## Rationale

### Name Evolution
1. **`ProviderRaw`** (original) ❌
   - Misleading: suggests provider-specific raw data
   - Doesn't indicate temporary nature
   - No pattern consistency

2. **`ParserBuffer`** (intermediate) ⚠️
   - Better: indicates parsing phase
   - "Buffer" doesn't match existing patterns
   - Still lacks clarity

3. **`ParseFrame`** (final) ✅
   - **Consistent**: Matches existing `WsFrame` naming pattern
   - **Clear**: "Frame" indicates discrete data unit
   - **Architectural**: Shows progression WsFrame → ParseFrame → Event
   - **Concise**: Short and memorable

---

## Data Flow Pattern

### Before (Inconsistent)
```
WsFrame → Parser → ProviderRaw → Normalizer → Event
(frame)           (raw? buffer?)              (event)
```

### After (Consistent "Frame" Pattern)
```
WsFrame → Parser → ParseFrame → Normalizer → Event
(frame)            (frame)                   (event)
```

**Clear progression:** Raw frame → Parsed frame → Canonical event

---

## Code Changes

### Type Definition
```go
// Before
type ProviderRaw struct { ... }

// After
type ParseFrame struct {
    returned   bool
    Provider   string
    StreamName string
    ReceivedAt int64
    Payload    json.RawMessage
}

// With enhanced documentation
// Complements WsFrame: WsFrame → Parser → ParseFrame → Event
```

### Pool Registration
```go
// Before
registerPool("ProviderRaw", 200, func() interface{} { 
    return new(schema.ProviderRaw) 
})

// After (with flow documentation)
registerPool("ParseFrame", 200, func() interface{} { 
    return new(schema.ParseFrame) 
})

// Enhanced comments show data flow:
// WsFrame (200) → Parser → ParseFrame (200) → Event (1000)
```

### Usage in Parser
```go
// Before
raw, releaseRaw, err := p.acquireProviderRaw(ctx)
raw.Provider = p.providerName
raw.Payload = append(raw.Payload[:0], frame...)

// After (cleaner variable names)
parseFrame, releaseFrame, err := p.acquireParseFrame(ctx)
parseFrame.Provider = p.providerName
parseFrame.Payload = append(parseFrame.Payload[:0], frame...)
```

### Test Functions
```go
// Before
func TestProviderRawReset(t *testing.T) { ... }
func TestProviderRawSetReturned(t *testing.T) { ... }
func TestProviderRawIsReturned(t *testing.T) { ... }
func TestProviderRawNilHandling(t *testing.T) { ... }

// After (consistent naming)
func TestParseFrameReset(t *testing.T) { ... }
func TestParseFrameSetReturned(t *testing.T) { ... }
func TestParseFrameIsReturned(t *testing.T) { ... }
func TestParseFrameNilHandling(t *testing.T) { ... }
```

---

## Files Modified

| File | Changes | Lines |
|------|---------|-------|
| `internal/schema/provider.go` | Type rename + docs | +17/-16 |
| `cmd/gateway/main.go` | Pool registration + enhanced comments | +9/-7 |
| `internal/adapters/binance/parser.go` | Usage + function rename | +15/-15 |
| `internal/schema/pooled_types_test.go` | All test functions | +40/-40 |
| `docs/improvements-analysis.md` | Documentation update | +15/-12 |

**Total:** 5 files changed, +96/-90 lines

---

## Naming Pattern Benefits

### Consistency
```go
// Frame family (pre-event data)
WsFrame        // Raw WebSocket frame from network
ParseFrame     // Frame during parsing phase
Event          // Canonical event (post-parsing)

// Request family (trading operations)
OrderRequest   // Trading order request
```

### Clarity in Data Flow
```
Network Layer:      WebSocket bytes
↓ (WsFrame pool)
Parser Layer:       WsFrame → JSON parsing
↓ (ParseFrame pool)
Normalization:      ParseFrame → Canonicalization
↓ (Event pool)
Business Logic:     Event → Processing
```

### Memory Efficiency
```go
// All "Frame" types are pooled for hot paths
WsFrame     (200 capacity)  // Every WS message
ParseFrame  (200 capacity)  // Every parse operation
Event       (1000 capacity) // Main message type

// Pattern: Temporary frame objects reused via pools
```

---

## Test Results

```bash
$ go test ./internal/schema/... -v
=== RUN   TestParseFrameReset
--- PASS: TestParseFrameReset (0.00s)
=== RUN   TestParseFrameSetReturned
--- PASS: TestParseFrameSetReturned (0.00s)
=== RUN   TestParseFrameIsReturned
--- PASS: TestParseFrameIsReturned (0.00s)
=== RUN   TestParseFrameNilHandling
--- PASS: TestParseFrameNilHandling (0.00s)
PASS

$ go test ./...
ok  	github.com/coachpo/meltica/internal/schema	0.003s
✅ All tests passing
```

---

## Build Verification

```bash
$ go build ./...
✅ Success

$ go build -o bin/gateway ./cmd/gateway
✅ Binary: 24MB

$ ./bin/gateway --help
  -providers string
    	Comma-separated provider types: fake,binance (runs all simultaneously) (default "fake")
✅ Multi-provider support confirmed
```

---

## Architecture Alignment

### Schema Types (Pooled)
```go
// Network → Parsing → Canonical → Trading
WsFrame      // Network layer
ParseFrame   // Parsing layer  ← NEW NAME
Event        // Canonical layer
OrderRequest // Trading layer
```

### Naming Principles
1. ✅ **Consistent patterns**: "Frame" for pre-event data
2. ✅ **Clear intent**: Name reflects purpose
3. ✅ **Short names**: Easy to type and read
4. ✅ **Architecture fit**: Aligns with data flow

---

## Developer Experience

### Before (Confusing)
```go
// What is "ProviderRaw"?
// - Provider-specific? (No, exchange-agnostic)
// - Raw format? (No, already JSON parsed)
// - Related to providers? (Only by origin)

raw, release, err := p.acquireProviderRaw(ctx)
```

### After (Clear)
```go
// ParseFrame = Frame during parsing phase
// - Complements WsFrame
// - Part of frame progression: Ws → Parse → Event
// - Temporary parsing buffer

frame, release, err := p.acquireParseFrame(ctx)
```

---

## Documentation Updates

### Pool Comments (cmd/gateway/main.go)
```go
// Object pools for memory efficiency - avoid allocations on hot paths:
//
// WsFrame (200 capacity):
//   - Used by WebSocket parsers to receive raw frames before canonicalization
//   - Shared across all exchanges (Binance, Coinbase, Kraken, etc.)
//   - Hot path: Every WebSocket message allocates/returns one frame
//
// ParseFrame (200 capacity):
//   - Frame during parsing phase, before canonicalization to Event
//   - Holds raw exchange-specific JSON during transformation
//   - Exchange-agnostic: Supports all provider types (Binance, Coinbase, etc.)
//   - Data flow: WsFrame → Parser → ParseFrame → Event
//   - Hot path: Temporary holder during JSON parsing and normalization
//
// Event (1000 capacity):
//   - The canonical event objects sent through the system
//   - Highest capacity: Main message type flowing through all components
```

### Type Comment (internal/schema/provider.go)
```go
// ParseFrame captures provider-specific payloads prior to normalization.
// This is an intermediate structure used during parsing - raw exchange data
// before conversion to canonical events. Supports all exchange types.
// Complements WsFrame: WsFrame → Parser → ParseFrame → Event
type ParseFrame struct { ... }
```

---

## Comparison: All Name Options

| Name | Consistency | Clarity | Brevity | Architecture | Final Score |
|------|-------------|---------|---------|--------------|-------------|
| **ParseFrame** ⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | **20/20** |
| ParserBuffer | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | 15/20 |
| ExchangeFrame | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 18/20 |
| SourceFrame | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | 17/20 |
| IngestBuffer | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | 15/20 |
| ProviderRaw (original) | ⭐⭐ | ⭐⭐ | ⭐⭐⭐ | ⭐⭐ | 9/20 |

**Winner:** `ParseFrame` - Perfect consistency with WsFrame, clear intent, architectural fit

---

## Future Extensibility

### Adding New Providers
```go
// Pattern works for any exchange
type CoinbaseParser struct { ... }

func (p *CoinbaseParser) Parse(ctx context.Context, frame []byte, ts time.Time) ([]*schema.Event, error) {
    parseFrame, release, err := p.acquireParseFrame(ctx)  // ✅ Same pattern
    defer release()
    
    // Coinbase-specific parsing
    parseFrame.Provider = "coinbase"
    parseFrame.Payload = frame
    
    // Normalize to canonical Event
    return []*schema.Event{...}, nil
}
```

### Pattern Consistency
```go
// All parsers follow the same pattern:
WsFrame → Parser.Parse() → ParseFrame → Event

// No matter the exchange:
- Binance: WsFrame → ParseFrame → Event
- Coinbase: WsFrame → ParseFrame → Event  
- Kraken: WsFrame → ParseFrame → Event
```

---

## Summary

✅ **Renamed** `ProviderRaw` → `ParseFrame`  
✅ **Consistent** with existing `WsFrame` pattern  
✅ **Clear** data flow: WsFrame → ParseFrame → Event  
✅ **Documented** with enhanced comments  
✅ **Tested** - all tests passing  
✅ **Built** - binary working correctly  

**Result:** Cleaner, more maintainable codebase with consistent naming patterns! 🚀
