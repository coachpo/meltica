# Unified Order Book Processing

## Overview

The system now uses a unified approach for order book data processing. All order book updates are converted to complete snapshots and published under the order book topic exposed by each exchange routing package (for Binance this is `bnrouting.Orderbook`), regardless of whether the provider sends incremental updates or full snapshots.

## Key Implementation Details

### 1. Order Book State Management

Each provider maintains local order book state for its symbols using an `OrderBookManager` defined in `exchanges/<name>/routing/orderbook.go`:

```go
type OrderBookManager struct {
    mu    sync.RWMutex
    books map[string]*OrderBook
}

type OrderBook struct {
    Symbol       string
    Bids         map[string]*big.Rat // price -> quantity
    Asks         map[string]*big.Rat // price -> quantity
    LastUpdateID int64               // For sequence tracking (Binance)
    LastUpdate   time.Time
}
```

### 2. Provider-Specific Processing

#### **Binance** - Delta Updates with Sequence Tracking
- **Input**: `depthUpdate` events with incremental changes
- **Processing**: Following [Binance documentation](https://developers.binance.com/docs/binance-spot-api-docs/web-socket-streams#diff-depth-stream)
  - Maintains sequence consistency using `U` (first update ID) and `u` (last update ID)
  - Applies delta updates to local order book state
  - Outputs complete order book snapshot after each update
- **Sequence Management**: 
  - Ignores updates with `u < LastUpdateID`
  - Resets order book if `U > LastUpdateID + 1` (indicates missing updates)

#### **Future Exchange Implementations**

When implementing additional exchanges, follow these patterns:

- **Coinbase**: Use `l2update` (incremental) and `snapshot` (full) events
- **OKX**: Use `books` channel with `action: "snapshot"` and `action: "update"`
- **Kraken**: Use `book` channel for full snapshots

### 3. Topic Structure

Order book data uses the topic helpers defined in each exchange's routing package. For Binance the helpers live in `exchanges/binance/routing`:

```go
import bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"

// Create order book topic for a symbol
topic := bnrouting.Orderbook("BTC-USDT")
```

### 4. Event Structure

The order book event structure:

```go
type DepthEvent struct {
    Symbol string
    Bids   []DepthLevel
    Asks   []DepthLevel
    Time   time.Time
}
```

### 5. Provider Behavior Summary

|| Provider | Input Type | Processing Method | Output |
||----------|------------|-------------------|---------|
|| **Binance** | `depthUpdate` (incremental) | Delta merge with sequence tracking | Complete snapshot |

**Note**: Additional exchange implementations will follow similar patterns when added.

## Usage

### Subscribing to Order Book Data

```go
import bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"

// Subscribe to order book updates for BTC-USDT
topic := bnrouting.Orderbook("BTC-USDT")
```

### Processing Order Book Events

```go
func handleOrderBookEvent(event *core.DepthEvent) {
    // All events are treated as complete snapshots
    log.Printf("Received order book for %s with %d bids, %d asks", 
        event.Symbol, len(event.Bids), len(event.Asks))
    
    // Replace your local order book with this data
    updateLocalOrderBook(event.Symbol, event.Bids, event.Asks)
}
```

## Benefits

1. **Simplified API**: Only one topic type to handle
2. **Consistent Behavior**: All providers output the same format
3. **Easier Integration**: No need to distinguish between snapshot/delta types
4. **Reduced Complexity**: Removed UpdateType field and related logic

## Migration Notes

- **No Breaking Changes**: Existing code using `DepthEvent` will continue to work
- **Topic Changes**: Replace `depth:SYMBOL` with `book:SYMBOL` in subscriptions
- **Simplified Logic**: Remove any UpdateType checking logic from your code

## Example

```go
// Before (complex)
switch event.UpdateType {
case corews.DepthUpdateSnapshot:
    replaceOrderBook(event)
case corews.DepthUpdateDelta:
    mergeOrderBook(event)
}

// After (simple)
// All events are snapshots - just replace the order book
replaceOrderBook(event)
```
