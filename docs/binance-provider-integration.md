# Binance Provider Integration

## Overview

The Binance adapter has been refactored to comply with the architectural principle:
**Adapters normalize all exchange diversity - emit only full orderbook snapshots, never deltas.**

## Architecture

```
Binance WebSocket → Delta Updates → BookAssembler → Full Snapshots → Lambda/Strategies
                                    (internal state)
```

### Key Components

#### 1. BookAssembler (`book_assembler.go`)
- **Maintains full orderbook state internally**
- Receives delta updates from Binance WebSocket
- Always returns complete `BookSnapshotPayload`, never deltas
- Performs checksum validation

**API Changes:**
```go
// OLD (removed):
func (a *BookAssembler) ApplyUpdate(update schema.BookUpdatePayload, seq uint64) (schema.BookUpdatePayload, error)

// NEW:
func (a *BookAssembler) ApplyUpdate(bids, asks []schema.PriceLevel, checksum string, seq uint64) (schema.BookSnapshotPayload, error)
```

#### 2. Parser (`parser.go`)
- Parses Binance WebSocket frames
- **Emits EventTypeBookSnapshot** (not EventTypeBookUpdate)
- Converts Binance delta format to snapshot metadata

**Changes:**
- Changed event type from `EventTypeBookUpdate` → `EventTypeBookSnapshot`
- Changed payload type from `BookUpdatePayload` → `BookSnapshotPayload`

#### 3. Provider (`provider.go`)
- Routes events through BookAssembler
- Distinguishes between REST snapshots (many levels) and WS deltas (few levels)
- Buffers deltas until initial snapshot received
- **All output events are BookSnapshot type**

**Removed:**
- `EventTypeBookUpdate` handling
- `coerceBookUpdate` function
- All BookUpdate-specific logic

## Event Flow

### Initial Snapshot (REST API)
```
1. REST Poll → Full orderbook → Parser
2. Parser → BookSnapshotPayload (many levels)
3. Provider detects full snapshot (>20 levels)
4. BookAssembler.ApplySnapshot() → Initialize state
5. Emit BookSnapshot event
6. Flush any buffered deltas
```

### Delta Updates (WebSocket)
```
1. WebSocket → Delta → Parser
2. Parser → BookSnapshotPayload (few levels, contains delta)
3. Provider detects delta (≤20 levels)
4. BookAssembler.ApplyUpdate() → Apply to internal state → Return full book
5. Emit BookSnapshot event with complete orderbook
```

## Integration with Gateway

### Command Line Usage

```bash
# Use fake provider (default)
./bin/gateway --provider=fake

# Use Binance provider
./bin/gateway --provider=binance
```

### Provider Creation

The gateway supports both providers through a unified interface:

```go
type MarketDataProvider interface {
    Events() <-chan *schema.Event
    Errors() <-chan error
    SubmitOrder(ctx context.Context, req schema.OrderRequest) error
    SubscribeRoute(route dispatcher.Route) error
    UnsubscribeRoute(typ schema.CanonicalType) error
}
```

Both `fake.Provider` and `binance.Provider` implement this interface.

## Event Types Supported

All 7 canonical event types from `schema.EventType` are fully supported:

| Event Type | Description | Binance Source | Status |
|------------|-------------|----------------|--------|
| **BookSnapshot** | Full orderbook depth | REST + WS depth@100ms | ✅ Complete |
| **Trade** | Trade executions | aggTrade stream | ✅ Complete |
| **Ticker** | 24hr ticker stats | 24hrTicker stream | ✅ Complete |
| **ExecReport** | Order execution reports | User data stream (executionReport) | ✅ Complete |
| **KlineSummary** | Candlestick data | kline stream (1m, 5m, 1h, etc.) | ✅ Complete |
| **ControlAck** | Control acknowledgements | System internal | N/A (not from exchange) |
| **ControlResult** | Control results | System internal | N/A (not from exchange) |

### Parser Implementation Details

#### BookSnapshot
- **REST**: `/api/v3/depth` → Full snapshot with 1000 levels
- **WebSocket**: `depth@100ms` → Deltas applied by BookAssembler → Full snapshot emitted

#### Trade
- **WebSocket**: `aggTrade` stream
- Maps `IsBuyerMaker` to `TradeSide` (Buy/Sell)
- Includes trade ID, price, quantity, timestamp

#### Ticker
- **WebSocket**: `24hrTicker` stream
- Provides last price, bid, ask, 24h volume
- Updates on every price change

#### ExecReport
- **WebSocket**: User data stream → `executionReport` event
- Maps Binance order statuses:
  - `NEW` → `ExecReportStateACK`
  - `PARTIALLY_FILLED` → `ExecReportStatePARTIAL`
  - `FILLED` → `ExecReportStateFILLED`
  - `CANCELED/CANCELLED` → `ExecReportStateCANCELLED`
  - `REJECTED` → `ExecReportStateREJECTED`
  - `EXPIRED` → `ExecReportStateEXPIRED`
- Includes client order ID, exchange order ID, fills, prices, reject reasons

#### KlineSummary
- **WebSocket**: `kline` stream (any interval: 1m, 5m, 15m, 1h, 4h, 1d, etc.)
- Provides OHLCV data with open/close timestamps
- Can subscribe to multiple intervals simultaneously

## Configuration Requirements

### Binance Provider Needs

To use Binance in production, you need:

1. **WebSocket Frame Provider**
   - Implement `binance.FrameProvider` interface
   - Connect to Binance WebSocket API
   - Handle reconnection and subscriptions

2. **REST Snapshot Fetcher**
   - Implement `binance.SnapshotFetcher` interface
   - Fetch initial orderbook snapshots
   - Call `/api/v3/depth` endpoint

3. **Configuration**
   ```go
   binance.ProviderOptions{
       Topics: []string{
           "btcusdt@depth@100ms",
           "ethusdt@depth@100ms",
           "btcusdt@aggTrade",
           "btcusdt@ticker",
       },
       Snapshots: []binance.RESTPoller{{
           Name:     "orderbook",
           Endpoint: "https://api.binance.com/api/v3/depth?symbol=BTCUSDT&limit=1000",
           Interval: 30 * time.Second,
           Parser:   "orderbook",
       }},
       Pools: poolMgr,
   }
   ```

## Testing

All tests pass with the new architecture:
```bash
$ go test ./internal/lambda/... ./internal/schema/... ./internal/adapters/binance/...
ok      github.com/coachpo/meltica/internal/lambda
ok      github.com/coachpo/meltica/internal/schema
ok      github.com/coachpo/meltica/internal/adapters/binance
```

Linter clean:
```bash
$ golangci-lint run ./...
0 issues.
```

## Benefits

### 1. **Simplified Lambda Layer**
- Strategies only handle full snapshots
- No delta assembly logic in business code
- Consistent data format across all exchanges

### 2. **Exchange Isolation**
- Binance-specific delta protocol hidden in adapter
- Easy to add other exchanges (Coinbase, Kraken, etc.)
- Each adapter handles its own quirks

### 3. **Data Integrity**
- Checksum validation in adapter
- Sequence validation in adapter
- Lambda receives validated, complete data

### 4. **Performance**
- BookAssembler maintains efficient internal state
- No repeated full snapshot transfers over network
- Optimal memory usage with delta application

## Migration Notes

### Removed Types
- `schema.EventTypeBookUpdate` - No longer exists
- `schema.BookUpdatePayload` - No longer exists
- `schema.BookUpdateType` - No longer exists (Delta/Full enum)

### Breaking Changes
All code that handled `EventTypeBookUpdate` must be updated to handle only `EventTypeBookSnapshot`.

## Future Work

### Binance WebSocket Provider
Current implementation has placeholders for:
- Actual WebSocket connection to Binance
- Authentication for user data streams
- Subscription management

These need real implementations for production use.

### Additional Exchanges
The same pattern can be applied to other exchanges:
```
Exchange Delta Protocol → Adapter BookAssembler → BookSnapshot Events
```

Each exchange adapter maintains its own internal state and emits normalized snapshots.

## Summary

✅ Binance adapter refactored to emit only BookSnapshot events
✅ BookAssembler changed from returning deltas to full snapshots  
✅ Parser updated to emit BookSnapshot type
✅ Provider handles both REST snapshots and WS deltas uniformly
✅ Integrated into gateway with --provider flag
✅ All tests passing, linter clean
✅ Architecture principle enforced: "Adapters normalize exchange diversity"
