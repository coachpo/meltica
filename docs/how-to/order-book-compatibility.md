# Order Book Compatibility Guide

## Overview

This guide explains how to handle both static snapshots (`book`) and incremental updates (`depth`) for order book data in a unified way.

## Problem

Different exchanges provide order book data in different formats:
- **Static Snapshots (`book`)**: Full order book state sent periodically
- **Incremental Updates (`depth`)**: Only changes to the order book, reducing bandwidth

## Solution

The `DepthEvent` structure has been extended with an `UpdateType` field to distinguish between these two modes:

```go
type DepthUpdateType int

const (
    DepthUpdateSnapshot DepthUpdateType = iota // Full order book snapshot
    DepthUpdateDelta                           // Incremental changes only
)

type DepthEvent struct {
    Symbol     string
    Bids       []DepthLevel
    Asks       []DepthLevel
    Time       time.Time
    UpdateType DepthUpdateType // NEW: Indicates snapshot vs delta
}
```

## Usage Examples

### Handling Order Book Updates

```go
func handleDepthEvent(event *corews.DepthEvent) {
    switch event.UpdateType {
    case corews.DepthUpdateSnapshot:
        // Full order book snapshot - replace entire book
        log.Printf("Received full snapshot for %s with %d bids, %d asks", 
            event.Symbol, len(event.Bids), len(event.Asks))
        // Replace your local order book with this snapshot
        
    case corews.DepthUpdateDelta:
        // Incremental update - apply changes to existing book
        log.Printf("Received delta update for %s with %d bid changes, %d ask changes", 
            event.Symbol, len(event.Bids), len(event.Asks))
        // Apply these changes to your existing order book
    }
}
```

### Provider-Specific Behavior

| Provider | Channel Type | UpdateType | Description |
|----------|-------------|------------|-------------|
| **Binance** | `depthUpdate` | `DepthUpdateDelta` | Incremental updates only |
| **Coinbase** | `l2update` | `DepthUpdateDelta` | Incremental updates |
| **Coinbase** | `snapshot` | `DepthUpdateSnapshot` | Full order book snapshots |
| **OKX** | `books` | `DepthUpdateSnapshot` | Full order book snapshots |

### Topic Mapping

- `depth:SYMBOL` - For incremental updates
- `book:SYMBOL` - For static snapshots

Both topics use the same `DepthEvent` structure, but with different `UpdateType` values.

## Migration Guide

If you have existing code that uses `DepthEvent`:

1. **No breaking changes**: The `UpdateType` field defaults to `0` (snapshot)
2. **Add type checking**: Use the `UpdateType` field to determine how to process the event
3. **Update handlers**: Modify your order book management logic to handle both types

## Best Practices

1. **Always check UpdateType**: Don't assume all events are the same type
2. **Handle snapshots first**: When receiving a snapshot, clear your local book first
3. **Apply deltas incrementally**: For delta updates, merge changes into existing book
4. **Maintain sequence**: Some providers send sequence numbers for ordering (future enhancement)

## Example Implementation

```go
type OrderBook struct {
    Symbol string
    Bids   map[string]*big.Rat // price -> quantity
    Asks   map[string]*big.Rat
}

func (ob *OrderBook) Update(event *corews.DepthEvent) {
    switch event.UpdateType {
    case corews.DepthUpdateSnapshot:
        // Clear and rebuild
        ob.Bids = make(map[string]*big.Rat)
        ob.Asks = make(map[string]*big.Rat)
        fallthrough // Continue to process levels
        
    case corews.DepthUpdateDelta:
        // Apply changes incrementally
        for _, level := range event.Bids {
            if level.Qty.Sign() == 0 {
                delete(ob.Bids, level.Price.String())
            } else {
                ob.Bids[level.Price.String()] = level.Qty
            }
        }
        
        for _, level := range event.Asks {
            if level.Qty.Sign() == 0 {
                delete(ob.Asks, level.Price.String())
            } else {
                ob.Asks[level.Price.String()] = level.Qty
            }
        }
    }
}
```
