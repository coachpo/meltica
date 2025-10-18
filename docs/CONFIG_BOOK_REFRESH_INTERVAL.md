# Book Refresh Interval Configuration

## Overview

Periodic orderbook refresh is now configured globally at the provider level with a **default of 1 minute**, making it easy to tune for different exchanges without modifying individual routes.

## Configuration

### Provider Level (Recommended)

Set `book_refresh_interval` at the provider level:

```yaml
adapter:
  binance:
    ws:
      publicUrl: wss://stream.binance.com:9443/stream
      handshakeTimeout: 10s
    rest:
      snapshot:
        endpoint: /api/v3/depth
        interval: 5s
        limit: 100
    book_refresh_interval: 1m  # ← Global periodic refresh
```

### Route Level (Optional Override)

Individual routes inherit the global setting by omitting `interval`:

```yaml
dispatcher:
  routes:
    ORDERBOOK.DELTA:
      wsTopics:
        - btcusdt@depth@100ms
      restFns:
        - name: orderbookSnapshot
          endpoint: /api/v3/depth?symbol=BTCUSDT&limit=1000
          # interval: omit to use global book_refresh_interval (1m)
          parser: orderbook
        - name: orderbookSnapshot_FastRefresh
          endpoint: /api/v3/depth?symbol=ETHUSDT&limit=1000
          interval: 30s  # ← Override: faster refresh for specific symbol
          parser: orderbook
```

## Default Behavior

| Configuration | Behavior |
|---------------|----------|
| `book_refresh_interval` not set | Defaults to **1 minute** |
| `book_refresh_interval: 30s` | All routes use 30s |
| `restFns[].interval` omitted | Uses global `book_refresh_interval` |
| `restFns[].interval: 2m` | Overrides global with 2m for this route |

## Example Configurations

### Conservative (2 minutes - lower API usage)
```yaml
adapter:
  binance:
    book_refresh_interval: 2m
```

### Standard (1 minute - default)
```yaml
adapter:
  binance:
    book_refresh_interval: 1m
```

### Aggressive (30 seconds - higher API usage)
```yaml
adapter:
  binance:
    book_refresh_interval: 30s
```

### Per-Exchange Tuning
```yaml
adapter:
  binance:
    book_refresh_interval: 1m   # Binance has generous rate limits
  
  coinbase:
    book_refresh_interval: 2m   # Coinbase has stricter limits
  
  kraken:
    book_refresh_interval: 45s  # Kraken middle ground
```

## Benefits

### 1. **Centralized Control**
- One setting controls all orderbook refresh intervals
- Easy to tune for different exchanges
- Clear visibility of refresh rate

### 2. **Provider-Specific Tuning**
- Binance: 1m (generous rate limits)
- Coinbase: 2m (stricter limits)
- Kraken: 1m (moderate limits)

### 3. **Per-Symbol Override**
- Most symbols use global default
- High-priority symbols can have faster refresh
- No need to change every route

### 4. **Prevents Common Issues**
- Memory growth from accumulated deltas
- Silent corruption accumulation
- Floating-point rounding drift
- Stale price levels

## Migration Guide

### Old Configuration (Hardcoded per route)
```yaml
# ❌ Old way - interval in every restFn
dispatcher:
  routes:
    ORDERBOOK.DELTA:
      restFns:
        - endpoint: /api/v3/depth?symbol=BTCUSDT&limit=1000
          interval: 30s
          parser: orderbook
        - endpoint: /api/v3/depth?symbol=ETHUSDT&limit=1000
          interval: 30s
          parser: orderbook
```

### New Configuration (Global setting)
```yaml
# ✅ New way - global book_refresh_interval
adapter:
  binance:
    book_refresh_interval: 30s

dispatcher:
  routes:
    ORDERBOOK.DELTA:
      restFns:
        - endpoint: /api/v3/depth?symbol=BTCUSDT&limit=1000
          # interval: omit to use global
          parser: orderbook
        - endpoint: /api/v3/depth?symbol=ETHUSDT&limit=1000
          # interval: omit to use global
          parser: orderbook
```

## Implementation Details

### Code Location
- **Config struct**: `internal/config/streaming.go` (BinanceAdapterConfig)
- **Validation**: Automatic default to 1m if not set
- **Application**: Route intervals inherit if omitted (0 or not specified)

### How It Works
1. Config loads `book_refresh_interval` from YAML
2. If not set, defaults to `1 * time.Minute` during validation
3. Routes with omitted `interval` use the global setting
4. Routes with explicit `interval` override the global setting

### Testing
```bash
# Verify configuration loads correctly
go test ./internal/config/...

# Build and verify
go build ./cmd/gateway
```

## Recommendations

### Production Settings

| Exchange | Recommended | Reason |
|----------|-------------|--------|
| Binance | 1m | Default - generous limits |
| Coinbase | 2m | Stricter rate limits |
| Kraken | 1m | Moderate limits |
| OKX | 1m | Similar to Binance |
| Bybit | 1m | Generous limits |

### Monitoring

Monitor these metrics to tune `book_refresh_interval`:

1. **API rate limit usage**: Stay under 80% of limit
2. **Memory growth**: Lower interval if growth is too slow
3. **Orderbook drift**: If checksums fail frequently, increase refresh rate
4. **Latency**: Balance freshness vs. network overhead

### Best Practices

✅ **DO**:
- Use global `book_refresh_interval` for consistency
- Override per-symbol only when necessary
- Monitor API rate limit usage
- Start with 1m default

❌ **DON'T**:
- Set interval too low (< 30s) without monitoring
- Forget to check exchange rate limits
- Override every route individually
- Use 0 or empty string (omit field instead)

## References

- **Main config**: `streaming.yaml`
- **Config struct**: `internal/config/streaming.go`
- **Architecture doc**: `docs/PROVIDER_AGNOSTIC_ORDERBOOK.md`
- **Buffering doc**: `docs/BUFFERING_IMPLEMENTATION.md`
