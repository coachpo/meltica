# Orderbook Event Architecture Discussion

## Current Design

The system currently has TWO orderbook event types:

1. **EventTypeBookSnapshot** - Full orderbook snapshots
2. **EventTypeBookUpdate** - Incremental updates (with UpdateType: Delta or Full)

## Question: Should We Consolidate?

### User's Suggestion
If providers **always** send full orderbooks, then having two separate event types adds unnecessary complexity. We could simplify to just `EventTypeBookSnapshot`.

### Current Reality Check

**Different exchanges have different behaviors:**

1. **Snapshot-Only Exchanges** (e.g., some REST APIs)
   - Always send full orderbooks
   - No incremental updates
   - Currently use: EventTypeBookSnapshot

2. **Delta-Update Exchanges** (e.g., WebSocket feeds)
   - Send initial snapshot
   - Then send incremental deltas (only changed levels)
   - More bandwidth efficient
   - Currently use: EventTypeBookSnapshot (initial) + EventTypeBookUpdate (deltas)

3. **Periodic Snapshot Exchanges**
   - Send snapshots periodically (e.g., every 1000ms)
   - Send deltas in between
   - Currently use: Both event types

### Architecture Decision Points

#### Option A: Keep Both Events (Current)
**Pros:**
- Supports all exchange types natively
- Efficient for delta-streaming exchanges
- Clear semantics (snapshot vs update)

**Cons:**
- More complex for consumers
- Consumers must handle both event types
- Duplicate handler code in lambdas

#### Option B: Consolidate to BookSnapshot Only
**Pros:**
- Simpler consumer code
- Single handler for all orderbook data
- Forces consistency across providers

**Cons:**
- Providers must maintain full book locally
- More memory usage in providers
- More bandwidth for delta-capable exchanges
- Loses efficiency of incremental updates

#### Option C: Consolidate to BookUpdate Only (with type flag)
**Pros:**
- Single event type
- UpdateType field distinguishes full vs delta
- Flexible for all provider types

**Cons:**
- Name doesn't reflect "snapshot" clearly
- Still requires type checking in handlers

### Recommendation

**KEEP BOTH EVENT TYPES** for the following reasons:

1. **Provider Flexibility**: Some exchanges naturally provide deltas, others provide snapshots. Let providers choose what's natural for their source.

2. **Efficiency**: Delta updates are much more bandwidth-efficient for high-frequency orderbook updates.

3. **Clear Semantics**: 
   - BookSnapshot = "Here's the complete state"
   - BookUpdate = "Here's what changed"

4. **Lambda Simplification**: We can simplify lambda code by:
   - Having lambdas subscribe to only what they need
   - For strategies that need full book always, subscribe to BookSnapshot only
   - For high-frequency strategies, subscribe to both (snapshot for init, updates for speed)

### Proposed Enhancement

Instead of removing one event type, **improve the lambda API**:

```go
// Option 1: Lambda subscribes to what it needs
lambda.SubscribeToOrderbook(FullSnapshotsOnly) // or DeltaUpdates, or Both

// Option 2: Strategy declares preference
type TradingStrategy interface {
    OrderbookPreference() OrderbookMode // FullOnly, DeltaAware, EitherOk
}
```

This way:
- Providers send what's natural for them
- Lambdas get what they need
- System handles conversion when needed

### Implementation Impact

If we were to consolidate to snapshot-only:
- **Providers**: Must maintain full orderbook in memory, rebuild on every delta
- **Bus**: More data transmitted
- **Lambdas**: Simpler, but get data more frequently than needed

## Conclusion

**KEEP both event types** but consider adding convenience methods in lambda for strategies that only care about full snapshots. The current design is architecturally sound for a multi-exchange system.

The perceived complexity in lambda handlers can be addressed with better abstractions rather than removing a valuable event type.
