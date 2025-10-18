# Orderbook Update Buffering Implementation

## Overview
Provider-agnostic buffering and replay mechanism for orderbook updates to prevent missing events during cold starts and gap recovery. Applicable to any exchange that provides:
- WebSocket incremental updates with sequence numbers
- REST API for periodic full snapshots
- Sequence validation (first_seq, final_seq or equivalent)

## Problem
Previously, updates arriving before the initial snapshot were rejected with `ErrBookNotInitialized`, potentially causing missed events during system startup.

## Solution

### 1. UML Diagram Update
Updated `docs/diagrams/orderbook-assembly-seq.puml` to be **provider-agnostic** and show:
- **Phase 1**: Cold start buffering
- **Phase 2**: Normal operation (contiguous updates)
- **Phase 3**: Gap detection and recovery
- **Phase 4**: Periodic refresh (configurable interval)
- **Phase 5**: Optional checksum verification

**Key Changes for Provider Agnosticism**:
- Generic terminology: `first_seq`/`final_seq` instead of Binance-specific `U`/`u`
- Generic symbols: `BTC-USDT` instead of `BNBBTC`
- Explicit periodic refresh loop (configurable per provider)
- Applicable to Binance, Coinbase, Kraken, OKX, etc.

### 2. BookAssembler Implementation

#### Added Buffering Structure
```go
type bufferedUpdate struct {
    bids          []schema.PriceLevel
    asks          []schema.PriceLevel
    firstUpdateID uint64
    finalUpdateID uint64
}
```

#### Buffering Logic (Cold Start)
When `ApplyUpdate()` is called before snapshot:
- Updates are cloned and stored in buffer
- Returns `ErrBookNotInitialized` (non-fatal)
- Buffer grows until snapshot arrives

#### Replay Logic (Snapshot Arrival)
When `ApplySnapshot()` is called:
1. Initialize book with snapshot data (seq=lastUpdateId)
2. Filter buffered updates: discard those with `u <= snapshot.seq`
3. Sort remaining updates by `firstUpdateID`
4. Replay updates sequentially, verifying continuity
5. Detect gaps during replay (error if `U > seq+1`)
6. Clear buffer after successful replay

#### Gap Detection Enhancement
When sequence gap detected during normal operation:
- **CRITICAL**: Buffer the gap-triggering update BEFORE resetting (preserves the update)
- Reset assembler to cold start state (`ready=false`, `seq=0`)
- Clear book but keep buffer (with gap-triggering update)
- Begin buffering subsequent updates until new snapshot arrives

**Example**: Current seq=1006, update 1009 arrives (1007, 1008 missing)
1. Gap detected: `1009 > 1006+1`
2. Update 1009 is buffered ← **Prevents data loss!**
3. State reset to cold start
4. New snapshot fetched
5. Buffered update 1009 replayed after snapshot

### 3. Periodic Refresh

**Implementation**: REST polling layer (`rest_client.go`)
```go
type RESTPoller struct {
    Name     string
    Endpoint string
    Interval time.Duration  // ← Uses provider's book_refresh_interval
    Parser   string
}
```

**Configuration** (`streaming.yaml`):
```yaml
adapter:
  binance:
    book_refresh_interval: 1m  # ← Global periodic refresh (default: 1 minute)

dispatcher:
  routes:
    ORDERBOOK.DELTA:
      restFns:
        - name: orderbookSnapshot
          endpoint: /api/v3/depth?symbol=BTCUSDT&limit=1000
          # interval: omit to use global book_refresh_interval
          parser: orderbook
```

**Default Behavior**:
- If `book_refresh_interval` is not set: defaults to **1 minute**
- If `restFns[].interval` is 0 or not set: uses `book_refresh_interval`
- Individual routes can override with specific intervals if needed

**Benefits**:
- Prevents memory growth from accumulated deltas
- Recovers from silent corruption
- Eliminates floating-point drift
- Cleans stale price levels

### 4. Test Coverage

Added comprehensive tests:
- `TestBookAssembler_ApplyUpdate_NotInitialized` - Verifies buffering occurs
- `TestBookAssembler_BufferAndReplay` - Validates end-to-end replay
- `TestBookAssembler_BufferReplay_DiscardStaleUpdates` - Filters old updates
- `TestBookAssembler_BufferReplay_SortsBySequence` - Ensures correct order
- `TestBookAssembler_GapDetection_ResetsToBuffering` - Tests restart logic
- `TestBookAssembler_GapDetection_MissingUpdate` - Tests gap-triggering update preservation

All tests pass ✅

### 5. Telemetry

**OpenTelemetry Metrics** (6 metrics total):

1. **`orderbook.gap.detected`** - Counter of sequence gaps requiring recovery
2. **`orderbook.buffer.size`** - Gauge of currently buffered updates
3. **`orderbook.update.stale`** - Counter of rejected stale updates
4. **`orderbook.snapshot.applied`** - Counter of snapshots applied
5. **`orderbook.updates.replayed`** - Counter of updates replayed after snapshot
6. **`orderbook.coldstart.duration`** - Histogram of cold start latency

**Attributes**: All metrics include `symbol` for per-symbol tracking

**Documentation**: See `docs/ORDERBOOK_TELEMETRY.md` for:
- Metric definitions and attributes
- Monitoring queries (Prometheus/Grafana)
- Alert rule examples
- Troubleshooting guide
- Dashboard recommendations

## Benefits

1. **No Lost Events**: Updates arriving before snapshot are preserved
2. **Gap-Triggered Updates Preserved**: Update that detects gap is buffered (not lost!)
3. **Correct Sequencing**: Sequence number validation throughout
4. **Automatic Recovery**: Gap detection triggers rebuffering automatically
5. **Cold Start Resilience**: System gracefully handles startup race conditions
6. **Periodic Refresh**: Prevents memory growth and corruption accumulation
7. **Provider Agnostic**: Pattern applies to any exchange with sequence numbers
8. **Comprehensive Telemetry**: Full observability of gaps, recovery, and health

## Architecture Alignment

Implementation now matches provider-agnostic UML specification in `orderbook-assembly-seq.puml`:
- ✅ Phase 1: Cold start buffering before snapshot
- ✅ Phase 2: Normal operation (contiguous updates)
- ✅ Phase 3: Gap detection and recovery
- ✅ Phase 4: Periodic refresh (configurable)
- ✅ Phase 5: Optional checksum verification
- ✅ Generic terminology (first_seq/final_seq)
- ✅ Applicable to multiple providers

## Provider Implementation Guide

To implement orderbook assembly for a new exchange:

1. **Identify sequence fields**: Map exchange-specific fields to `first_seq`/`final_seq`
   - Binance: `U` (firstUpdateId) → `first_seq`, `u` (finalUpdateId) → `final_seq`
   - Coinbase: `sequence` → `first_seq`, `sequence` → `final_seq`
   - Kraken: `lastSequence` → both fields

2. **Configure REST polling**: Set appropriate `interval` based on:
   - Exchange rate limits
   - Update frequency (faster = shorter interval)
   - Network reliability

3. **Implement parser**: Convert exchange-specific format to canonical `BookSnapshotPayload`

4. **Optional**: Implement checksum verification if exchange provides it
