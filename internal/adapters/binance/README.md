# Binance Adapter

Production-ready Binance market data adapter with comprehensive event support.

## Features

✅ **Modern WebSocket** - Uses `github.com/coder/websocket` (not deprecated gorilla)  
✅ **First-Class Context** - Proper context cancellation throughout  
✅ **Auto-Reconnection** - Exponential backoff with configurable max attempts  
✅ **Rate Limiting** - Token bucket algorithm (5 msg/sec WS, 20 req/sec REST)  
✅ **Connection TTL** - Monitors 24h limit, reconnects proactively  
✅ **Authentication** - HMAC-SHA256 for signed endpoints  
✅ **All Event Types** - BookSnapshot, Trade, Ticker, ExecReport, KlineSummary  
✅ **Clean Architecture** - BookAssembler maintains state, emits only full snapshots  

## Architecture

```
Binance WebSocket → BinanceWSProvider → Parser → BookAssembler → Events
     (deltas)       (reconnection/rate)  (normalize)  (full book)   (clean)

Binance REST API → BinanceRESTFetcher → Parser → Events
  (snapshots)      (auth/rate limit)    (normalize)  (clean)
```

## Files

| File | Purpose | Lines |
|------|---------|-------|
| **provider.go** | Main provider orchestration | 570 |
| **ws_provider.go** | WebSocket connection management | 424 |
| **rest_fetcher.go** | REST API client with auth | 280 |
| **parser.go** | Event normalization | 530 |
| **book_assembler.go** | Orderbook state management | 234 |
| **rate_limiter.go** | Token bucket rate limiting | 68 |
| **ws_client.go** | WebSocket client wrapper | 140 |
| **rest_client.go** | REST client wrapper | 110 |

**Total: 2,356 lines**

## Key Principles

### 1. Adapters Normalize Exchange Diversity

**Binance sends:** Deltas via WebSocket  
**System receives:** Full snapshots always

```
Exchange Quirk → Adapter Internal → Normalized Output
(delta updates)  (BookAssembler)   (full snapshots)
```

### 2. No Shim Code

All exchange-specific logic stays in adapter:
- Delta application
- Checksum validation
- Sequence checking
- Format conversion

Lambda layer sees only clean, normalized events.

### 3. First-Class Context Support

Every operation respects context:
```go
// Dial with timeout
conn, _, err := websocket.Dial(ctx, url, nil)

// Read with deadline
msgType, data, err := conn.Read(ctx)

// Write with timeout
err := conn.Write(ctx, websocket.MessageText, data)

// Ping with timeout
err := conn.Ping(ctx)
```

## Usage

### Environment Variables

```bash
# Required for authenticated streams
export BINANCE_API_KEY="your_api_key"
export BINANCE_SECRET_KEY="your_secret_key"

# Optional: Use testnet
export BINANCE_USE_TESTNET="true"
```

### Code Example

```go
// Create parser
parser := binance.NewParserWithPool("binance", poolMgr)

// Create WebSocket provider with reconnection
wsProvider := binance.NewBinanceWSProvider(binance.WSProviderConfig{
    UseTestnet:    false,
    APIKey:        apiKey,
    MaxReconnects: 10,
})

// Create REST fetcher with authentication
restFetcher := binance.NewBinanceRESTFetcher(apiKey, secretKey, false)

// Create clients
wsClient := binance.NewWSClient("binance", wsProvider, parser, time.Now, poolMgr)
restClient := binance.NewRESTClient(restFetcher, parser, time.Now)

// Create provider
provider := binance.NewProvider("binance", wsClient, restClient, binance.ProviderOptions{
    Topics: []string{
        "btcusdt@depth@100ms",
        "btcusdt@aggTrade",
        "btcusdt@ticker",
        "btcusdt@kline_1m",
    },
    Snapshots: []binance.RESTPoller{
        {
            Name:     "orderbook",
            Endpoint: "https://api.binance.com/api/v3/depth?symbol=BTCUSDT&limit=1000",
            Interval: 30 * time.Second,
            Parser:   "orderbook",
        },
    },
    Pools: poolMgr,
})

// Start provider
provider.Start(ctx)

// Consume events
for event := range provider.Events() {
    // Process event
}
```

## Event Types

All 7 canonical event types supported:

### Market Data (No Auth Required)

- **BookSnapshot** - Full orderbook from `depth@100ms` + REST
- **Trade** - Aggregate trades from `aggTrade`
- **Ticker** - 24hr stats from `ticker`
- **KlineSummary** - Candlesticks from `kline_<interval>`

### User Data (Auth Required)

- **ExecReport** - Order execution reports from user data stream

### System Internal

- **ControlAck** - Control plane acknowledgements
- **ControlResult** - Control plane results

## Dependencies

```
github.com/coder/websocket v1.8.14  - Modern WebSocket library (NOT deprecated)
github.com/goccy/go-json            - Fast JSON parsing
github.com/sourcegraph/conc         - Structured concurrency
```

**Why coder/websocket?**
- ✅ Not deprecated (unlike gorilla/websocket)
- ✅ First-class context support
- ✅ Minimal, idiomatic API
- ✅ Zero dependencies
- ✅ Efficient read/write
- ✅ Concurrent-safe
- ✅ Production-tested

## Rate Limits

### WebSocket (Enforced)

- 5 messages/second per connection
- Max 1024 streams per connection
- Max 300 connection attempts per 5min per IP
- Connection valid for 24 hours

### REST API (Enforced)

- 20 requests/second (= 1200/minute)
- Automatic signature generation
- Server time synchronization

## Reconnection Strategy

```
Attempt 1:  5s delay
Attempt 2: 10s delay
Attempt 3: 20s delay
Attempt 4: 40s delay
Attempt 5+: 60s delay (max)
```

**Triggers:**
- Connection closed unexpectedly
- Ping timeout (60s)
- Read/Write errors
- Connection TTL reached (23h)

**Actions on Reconnect:**
1. Close existing connection
2. Wait (exponential backoff)
3. Dial new connection
4. Resubscribe to all streams
5. Resume operation

## Testing

```bash
# Build
go build ./internal/adapters/binance/...

# Test
go test ./internal/adapters/binance/...

# Lint
golangci-lint run ./internal/adapters/binance/...
```

## Clean Architecture

**No shim code. No backward compatibility hacks.**

Every piece of Binance-specific logic is isolated in this adapter. The rest of the system sees only clean, normalized events conforming to `schema.EventType`.

**Principle:** *Adapters normalize all exchange diversity.*
