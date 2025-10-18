# Provider-Agnostic Orderbook Assembly Pattern

## Overview

The orderbook assembly pattern in `docs/diagrams/orderbook-assembly-seq.puml` is now **provider-agnostic**, enabling any exchange integration to follow the same architectural pattern.

## Design Principles

### 1. Generic Terminology
- **`first_seq`** / **`final_seq`**: Generic sequence numbers
- **`lastSequence`**: Last applied sequence in snapshot
- **`BTC-USDT`**: Generic symbol format (BASE-QUOTE)

### 2. Five Phase Architecture

#### **Phase 1: Cold Start** (Initial Snapshot)
- Buffer all updates until first snapshot arrives
- Prevents data loss during startup race conditions
- Updates preserved for replay

#### **Phase 2: Normal Operation**
- Apply contiguous updates immediately
- No buffering overhead
- Sequence validation on every update

#### **Phase 3: Gap Detection & Recovery**
- Detect missing updates via sequence gaps
- Buffer gap-triggering update (prevents data loss!)
- Transition to cold start for recovery
- Next snapshot triggers replay

#### **Phase 4: Periodic Refresh** ⭐ NEW
- Configurable REST polling interval (e.g., 30s)
- Prevents memory growth from accumulated deltas
- Recovers from silent corruption
- Eliminates floating-point drift
- Provider-specific tuning based on rate limits

#### **Phase 5: Optional Checksum Verification**
- Provider-dependent feature
- Detects orderbook corruption
- Triggers recovery via snapshot fetch

## Provider Implementation Mapping

| Exchange | first_seq | final_seq | Checksum | Notes |
|----------|-----------|-----------|----------|-------|
| **Binance** | `U` (firstUpdateId) | `u` (finalUpdateId) | Optional CRC32 | Documented in API |
| **Coinbase** | `sequence` | `sequence` | Not provided | Single sequence per update |
| **Kraken** | `lastSequence` | `lastSequence` | Optional CRC32 | Checksum in snapshot |
| **OKX** | `seqId` | `seqId` | Optional | Similar to Coinbase |
| **Bybit** | `u` | `u` | Optional | Similar to Binance |

## Configuration Example

### Binance (1m default refresh)
```yaml
adapter:
  binance:
    book_refresh_interval: 1m  # ← Global periodic refresh (default)
    # Can be overridden: 30s, 2m, 5m, etc.

dispatcher:
  routes:
    ORDERBOOK.DELTA:
      wsTopics:
        - btcusdt@depth@100ms
      restFns:
        - name: orderbookSnapshot
          endpoint: /api/v3/depth?symbol=BTCUSDT&limit=1000
          # interval: omit to use global book_refresh_interval
          parser: orderbook
```

### Coinbase (2m refresh - lower rate limit)
```yaml
adapter:
  coinbase:
    book_refresh_interval: 2m  # ← Slower due to stricter rate limits

dispatcher:
  routes:
    ORDERBOOK.DELTA:
      wsTopics:
        - level2_batch
      restFns:
        - name: orderbookSnapshot
          endpoint: /products/BTC-USD/book?level=2
          # interval: omit to use global book_refresh_interval
          parser: orderbook
```

### Kraken (1m default)
```yaml
adapter:
  kraken:
    book_refresh_interval: 1m  # ← Default works well for Kraken

dispatcher:
  routes:
    ORDERBOOK.DELTA:
      wsTopics:
        - book
      restFns:
        - name: orderbookSnapshot
          endpoint: /0/public/Depth?pair=XBTUSD
          # interval: omit to use global book_refresh_interval
          parser: orderbook
```

### Per-Symbol Override (if needed)
```yaml
adapter:
  binance:
    book_refresh_interval: 1m  # ← Global default

dispatcher:
  routes:
    ORDERBOOK.DELTA:
      restFns:
        - name: orderbookSnapshot_BTC
          endpoint: /api/v3/depth?symbol=BTCUSDT&limit=1000
          interval: 30s  # ← Override: faster refresh for BTC
          parser: orderbook
        - name: orderbookSnapshot_ETH
          endpoint: /api/v3/depth?symbol=ETHUSDT&limit=1000
          # interval: omit to use global 1m for ETH
          parser: orderbook
```

## Benefits of Provider Agnosticism

1. **Consistent Pattern**: All exchanges follow same 5-phase lifecycle
2. **Code Reuse**: BookAssembler is exchange-independent
3. **Predictable Behavior**: Same buffering, gap detection, and recovery logic
4. **Easy Onboarding**: New exchange integration follows existing pattern
5. **Testing**: Generic test suite applies to all providers
6. **Documentation**: Single diagram covers all exchanges

## Implementation Checklist for New Provider

- [ ] **1. Map sequence fields**
  - Identify exchange's sequence number field(s)
  - Map to `first_seq` and `final_seq`
  - Document in provider parser

- [ ] **2. Implement parser**
  - Parse WebSocket incremental updates
  - Parse REST snapshot responses
  - Convert to canonical `BookSnapshotPayload`

- [ ] **3. Configure REST polling**
  - Set provider's `book_refresh_interval` (default: 1m)
  - Consider exchange rate limits
  - Balance freshness vs. API quota
  - Can override per-symbol if needed

- [ ] **4. Test gap scenarios**
  - Simulate missing updates
  - Verify buffering behavior
  - Confirm replay correctness

- [ ] **5. Optional: Add checksum verification**
  - If exchange provides checksums
  - Implement validation in BookAssembler
  - Trigger recovery on mismatch

- [ ] **6. Tune for production**
  - Monitor memory usage
  - Adjust refresh interval if needed
  - Set up alerts for gap frequency

## Architecture Guarantees

✅ **No data loss** during cold start
✅ **No data loss** during gap detection
✅ **Automatic recovery** from sequence gaps
✅ **Memory bounded** via periodic refresh
✅ **Corruption detection** via optional checksums
✅ **Provider agnostic** - works for any exchange

## References

- **Diagram**: `docs/diagrams/orderbook-assembly-seq.puml`
- **Implementation**: `internal/adapters/binance/book_assembler.go`
- **Tests**: `internal/adapters/binance/book_assembler_test.go`
- **Documentation**: `docs/BUFFERING_IMPLEMENTATION.md`
