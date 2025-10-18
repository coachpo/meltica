# Refactoring Needed: Remove BookUpdate Event Type

## Context

Per architectural decision, **adapters MUST always emit full orderbook snapshots**. The `EventTypeBookUpdate` has been removed from the schema to enforce this.

## Completed
✅ Removed `EventTypeBookUpdate` and `BookUpdatePayload` from schema  
✅ Removed `OnBookUpdate` from `TradingStrategy` interface  
✅ Removed `handleBookUpdate` from base lambda  
✅ Updated all trading strategies  
✅ Updated schema tests  
✅ Cleaned fake provider BookUpdate methods
✅ Fixed main.go

## Remaining Work - Binance Adapter (CRITICAL)

### Files Affected:
1. `internal/adapters/binance/book_assembler.go` - Uses BookUpdatePayload for delta application
2. `internal/adapters/binance/parser.go` - Emits BookUpdate events
3. `internal/adapters/binance/provider.go` - Handles BookUpdate routing

### Required Refactoring Strategy:

**The BookAssembler should remain internal to binance adapter but:**

1. **book_assembler.go**:
   - Change `ApplyUpdate` to return `BookSnapshotPayload` instead of `BookUpdatePayload`
   - Keep delta application logic internally
   - Always return full orderbook state

```go
// OLD:
func (a *BookAssembler) ApplyUpdate(update schema.BookUpdatePayload, seq uint64) (schema.BookUpdatePayload, error)

// NEW:
func (a *BookAssembler) ApplyUpdate(update BinanceDepthUpdate, seq uint64) (schema.BookSnapshotPayload, error)
```

2. **parser.go**:
   - Change from emitting `EventTypeBookUpdate` to `EventTypeBookSnapshot`
   - Change payload from `BookUpdatePayload` to `BookSnapshotPayload`
   - Assembler will provide full book state

```go
// OLD:
evt.Type = schema.EventTypeBookUpdate
evt.Payload = schema.BookUpdatePayload{
    UpdateType: schema.BookUpdateTypeDelta,
    ...
}

// NEW:
evt.Type = schema.EventTypeBookSnapshot
evt.Payload = schema.BookSnapshotPayload{
    Bids: fullBook.Bids,
    Asks: fullBook.Asks,
    ...
}
```

3. **provider.go**:
   - Remove `case schema.EventTypeBookUpdate` from routing
   - Remove `bufferBookUpdate` function
   - Remove `coerceBookUpdate` function
   - All orderbook events flow as BookSnapshot

### Key Architecture Points:

**Adapter Internal (binance package):**
- BookAssembler maintains full orderbook state
- Receives deltas from Binance WebSocket
- Applies deltas to internal state

**Adapter Output (to system):**
- ONLY emits `EventTypeBookSnapshot`
- Always contains full bid/ask arrays
- Lambda consumers never see deltas

### Binance Delta Protocol:

The Binance adapter receives:
1. Initial snapshot via REST API
2. Delta updates via WebSocket (only changed levels)

The adapter must:
1. Maintain full orderbook in BookAssembler
2. Apply each delta to internal state
3. Emit complete BookSnapshot after each update

This way:
- Binance-specific delta complexity stays in binance package
- Lambda layer only sees normalized full snapshots
- Adapter handles all exchange-specific quirks

## Testing Strategy

After refactoring:
1. Test BookAssembler delta application still works
2. Verify snapshots contain full orderbook
3. Test lambda receives correct data
4. Integration test with real Binance feed (if available)

## Priority

**HIGH** - Binance adapter won't compile until fixed
