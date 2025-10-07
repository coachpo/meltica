# Unified Book Processing

## Overview

The system now uses a unified approach for book data processing. All book updates are converted to complete snapshots and published under the book topic exposed by each exchange routing package (for Binance this is `bnrouting.Book`), regardless of whether the provider sends incremental updates or full snapshots.

## Key Implementation Details

### 1. Book State Management

Each provider maintains local book state for its symbols using a `BookManager` defined in `exchanges/<name>/routing/book.go`:

```go
type BookManager struct {
    mu    sync.RWMutex
    books map[string]*Book
}

type Book struct {
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
  - Applies delta updates to local book state
  - Outputs complete book snapshot after each update
- **Sequence Management**:
  - Ignores updates with `u < LastUpdateID`
  - Resets book if `U > LastUpdateID + 1` (indicates missing updates)

#### **Future Exchange Implementations**

When implementing additional exchanges, follow these patterns:

- **Coinbase**: Use `l2update` (incremental) and `snapshot` (full) events
- **OKX**: Use `books` channel with `action: "snapshot"` and `action: "update"`
- **Kraken**: Use `book` channel for full snapshots

### 3. Topic Structure

Book data uses the topic helpers defined in each exchange's routing package. For Binance the helpers live in `exchanges/binance/routing`:

```go
import bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"

// Create book topic for a symbol
topic := bnrouting.Book("BTC-USDT")
```

### 4. Event Structure

The book event structure:

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

### Subscribing to Book Data

```go
import bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"

// Subscribe to book updates for BTC-USDT
topic := bnrouting.Book("BTC-USDT")
```

### Processing Book Events

```go
func handleBookEvent(event *core.DepthEvent) {
    // All events are treated as complete snapshots
    log.Printf("Received book for %s with %d bids, %d asks",
        event.Symbol, len(event.Bids), len(event.Asks))

    // Replace your local book with this data
    updateLocalBook(event.Symbol, event.Bids, event.Asks)
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
    replaceBook(event)
case corews.DepthUpdateDelta:
    mergeBook(event)
}

// After (simple)
// All events are snapshots - just replace the book
replaceBook(event)
```
