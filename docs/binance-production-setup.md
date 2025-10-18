# Binance Production Setup Guide

## Overview

Complete production-ready Binance integration with:
✅ Real WebSocket connections with auto-reconnection  
✅ Modern **coder/websocket** library (replaces deprecated gorilla/websocket)  
✅ First-class context support throughout  
✅ REST API client with HMAC-SHA256 authentication  
✅ Rate limiting (5 msg/sec WebSocket, 20 req/sec REST)  
✅ Exponential backoff reconnection  
✅ Connection TTL monitoring (24h lifecycle)  
✅ User data stream support  
✅ All 7 event types fully supported  

---

## Quick Start

### 1. Set Environment Variables

```bash
# Required for live trading
export BINANCE_API_KEY="your_api_key_here"
export BINANCE_SECRET_KEY="your_secret_key_here"

# Optional: Use testnet for testing
export BINANCE_USE_TESTNET="true"
```

### 2. Run Gateway with Binance

```bash
# Start with Binance provider
./bin/gateway --provider=binance

# Or with explicit config
./bin/gateway --provider=binance --config=streaming.yaml
```

### 3. Verify Connection

Check logs for:
```
binance: testnet=false, api_key=abcd...xyz, streams=12
binance ws: connected to wss://stream.binance.com:9443/stream
```

---

## Architecture

### WebSocket Flow

```
Binance WebSocket → Auto-Reconnect → Rate Limiter → Parser → Events
    ↓                     ↓              ↓
  Ping/Pong          Connection TTL   5 msg/sec
  (20s interval)     (24h monitor)    (enforced)
```

### REST API Flow

```
REST Poller → Rate Limiter → Authentication → Binance API → Parser
   (30s)         (20 req/sec)    (HMAC-SHA256)              ↓
                                                          Events
```

---

## Components

### 1. BinanceWSProvider (`ws_provider.go`)

**Features:**
- Real WebSocket connection to Binance
- Auto-reconnection with exponential backoff
- Connection TTL monitoring (reconnects before 24h expiry)
- Rate limiting (5 messages/second)
- Ping/Pong handling (20s interval)
- Error handling and recovery

**Configuration:**
```go
wsProvider := binance.NewBinanceWSProvider(binance.WSProviderConfig{
    BaseURL:       "wss://stream.binance.com:9443", // or testnet
    UseTestnet:    false,
    APIKey:        "your_api_key", // optional for public streams
    MaxReconnects: 10,
    RateLimiter:   binance.NewRateLimiter(5), // 5 msg/sec
})
```

**Reconnection Logic:**
- Initial delay: 5 seconds
- Exponential backoff: 5s → 10s → 20s → 40s → 60s (max)
- Max reconnect attempts: configurable (default: 10)
- Automatic resubscription to all streams on reconnect

### 2. BinanceRESTFetcher (`rest_fetcher.go`)

**Features:**
- HMAC-SHA256 authentication
- Rate limiting (20 requests/second = 1200/minute)
- Automatic timestamp inclusion
- Server time synchronization
- User data stream management (listen keys)

**Methods:**
```go
// Public endpoints (no auth)
body, err := fetcher.Fetch(ctx, "/api/v3/depth?symbol=BTCUSDT&limit=1000")

// Signed endpoints (requires auth)
params := url.Values{}
params.Set("symbol", "BTCUSDT")
body, err := fetcher.FetchSigned(ctx, "/api/v3/account", params)

// Get server time (for sync)
serverTime, err := fetcher.GetServerTime(ctx)

// User data stream
listenKey, err := fetcher.CreateListenKey(ctx)
err = fetcher.KeepAliveListenKey(ctx, listenKey)
```

### 3. RateLimiter (`rate_limiter.go`)

**Token Bucket Algorithm:**
- Capacity: 5 tokens (WebSocket) or 20 tokens (REST)
- Refill rate: 5 or 20 tokens per second
- Blocks when tokens exhausted
- Thread-safe

**Usage:**
```go
limiter := binance.NewRateLimiter(5) // 5 per second

if limiter.Allow() {
    // Send message
} else {
    // Rate limit exceeded
}

// Or block until available
limiter.Wait() // Blocks until token available
```

---

## Event Types Supported

| Type | Source | Update Frequency |
|------|--------|------------------|
| **BookSnapshot** | `depth@100ms` + REST snapshots | 100ms + 30s |
| **Trade** | `aggTrade` stream | Real-time |
| **Ticker** | `ticker` stream | Real-time |
| **KlineSummary** | `kline_1m` stream | 1 minute |
| **ExecReport** | User data stream | Real-time |
| **ControlAck** | System internal | N/A |
| **ControlResult** | System internal | N/A |

---

## Configuration Examples

### Minimal Setup (Public Market Data Only)

```bash
# No authentication needed for public streams
unset BINANCE_API_KEY
unset BINANCE_SECRET_KEY

./bin/gateway --provider=binance
```

Streams: Depth, Trades, Ticker, Klines

### Full Setup (With User Data Stream)

```bash
# Authentication required
export BINANCE_API_KEY="your_api_key"
export BINANCE_SECRET_KEY="your_secret_key"

./bin/gateway --provider=binance
```

Streams: All public streams + Execution Reports

### Testnet Setup

```bash
export BINANCE_API_KEY="testnet_api_key"
export BINANCE_SECRET_KEY="testnet_secret_key"
export BINANCE_USE_TESTNET="true"

./bin/gateway --provider=binance
```

**Testnet URLs:**
- WebSocket: `wss://testnet.binance.vision/ws`
- REST API: `https://testnet.binance.vision`

---

## Stream Configuration

### Default Streams (in main.go)

```go
topics := []string{
    // Orderbook depth (100ms updates)
    "btcusdt@depth@100ms",
    "ethusdt@depth@100ms",
    "xrpusdt@depth@100ms",
    
    // Aggregate trades
    "btcusdt@aggTrade",
    "ethusdt@aggTrade",
    "xrpusdt@aggTrade",
    
    // 24hr ticker
    "btcusdt@ticker",
    "ethusdt@ticker",
    "xrpusdt@ticker",
    
    // 1-minute klines
    "btcusdt@kline_1m",
    "ethusdt@kline_1m",
    "xrpusdt@kline_1m",
}
```

### Custom Streams

Edit `cmd/gateway/main.go` and add/modify topics:

```go
topics := []string{
    // Different intervals
    "btcusdt@depth@1000ms",  // 1 second depth
    "btcusdt@kline_5m",      // 5-minute klines
    "btcusdt@kline_1h",      // 1-hour klines
    
    // More symbols
    "adausdt@aggTrade",
    "solusdt@ticker",
    
    // Mini ticker (faster, less data)
    "btcusdt@miniTicker",
}
```

---

## Rate Limits

### WebSocket Limits (Per Connection)

| Limit | Value | Enforced By |
|-------|-------|-------------|
| Messages/second | 5 | RateLimiter |
| Streams/connection | 1024 | Binance server |
| Connection attempts/IP | 300 per 5min | Binance server |
| Connection TTL | 24 hours | Monitored & auto-reconnect |

**Consequences of exceeding:**
- Message limit → Connection closed by server
- Repeated violations → IP ban

### REST API Limits

| Limit | Value | Enforced By |
|-------|-------|-------------|
| Requests/minute | 1200 | RateLimiter (20/sec) |
| Orders/second | 10 | Binance server |
| Orders/day | 200,000 | Binance server |

**Response headers:**
- `X-MBX-USED-WEIGHT-1M`: Used weight
- `X-MBX-ORDER-COUNT-10S`: Order count

---

## Error Handling

### Auto-Recovery Scenarios

✅ **WebSocket disconnection** → Auto-reconnect  
✅ **Network timeout** → Exponential backoff retry  
✅ **Connection TTL expired** → Proactive reconnect  
✅ **Rate limit exceeded** → Wait and retry  
✅ **Ping timeout** → Trigger reconnect  

### Manual Intervention Required

❌ **Invalid API credentials** → Check environment variables  
❌ **IP banned** → Wait 2-24 hours or contact Binance  
❌ **Max reconnect attempts** → Check network/firewall  
❌ **Invalid stream names** → Fix configuration  

---

## Monitoring & Logs

### Success Indicators

```
binance ws: connected to wss://stream.binance.com:9443/stream
binance: testnet=false, api_key=abcd...wxyz, streams=12
gateway started; awaiting shutdown signal
```

### Warning Signs

```
binance ws: ping failed: write tcp: broken pipe
binance ws: reconnecting (attempt 3/10) after 20s
binance ws: resubscribe failed: context canceled
```

### Critical Errors

```
binance ws: max reconnect attempts reached (10)
binance ws: http status 429: Too many requests
start binance provider: dial failed: connection refused
```

---

## Security Best Practices

### API Key Management

1. **Never commit API keys** to version control
2. **Use environment variables** for credentials
3. **Restrict API key permissions:**
   - Enable: Spot Trading (if needed)
   - Enable: Margin Trading (if needed)
   - Disable: Withdrawals
   - Disable: Universal Transfer
4. **IP Whitelisting:** Configure in Binance account
5. **Rotate keys periodically** (every 90 days)

### Secret Key Protection

```bash
# Set permissions
chmod 600 ~/.env

# Store in .env file
echo "BINANCE_API_KEY=xxx" >> ~/.env
echo "BINANCE_SECRET_KEY=yyy" >> ~/.env

# Load before starting
source ~/.env
./bin/gateway --provider=binance
```

### Testnet for Development

Always develop against testnet first:
```bash
export BINANCE_USE_TESTNET="true"
```

Register at: https://testnet.binance.vision/

---

## Troubleshooting

### Connection Issues

**Problem:** `dial failed: connection refused`
```bash
# Check firewall
curl -v https://api.binance.com/api/v3/ping

# Check DNS
nslookup stream.binance.com

# Try testnet
export BINANCE_USE_TESTNET="true"
```

### Authentication Failures

**Problem:** `http status 401: Unauthorized`
```bash
# Verify API key
echo $BINANCE_API_KEY

# Test REST API
curl -H "X-MBX-APIKEY: $BINANCE_API_KEY" \
  https://api.binance.com/api/v3/account
```

### Rate Limit Errors

**Problem:** `http status 429: Too many requests`
- Reduce polling frequency
- Increase rate limiter capacity
- Wait 1-2 minutes before retry

### No Events Received

**Problem:** Connected but no market data
```bash
# Check stream subscriptions in logs
grep "SUBSCRIBE" gateway.log

# Verify symbol format (lowercase)
# Correct: btcusdt@depth
# Wrong: BTCUSDT@depth
```

---

## Performance Tuning

### Connection Pooling

Current: 1 WebSocket connection (12 streams)
Maximum: 1024 streams per connection

Add more symbols:
```go
// Still 1 connection, more streams
topics := append(topics, 
    "bnbusdt@depth@100ms",
    "dogeusdt@aggTrade",
    // ... up to 1024 streams
)
```

### Buffer Sizes

Adjust in main.go:
```go
frames := make(chan []byte, 256)  // Increase for high volume
errs := make(chan error, 16)      // Increase for error bursts
```

### Polling Intervals

Adjust REST snapshot frequency:
```go
{
    Interval: 30 * time.Second,  // Reduce for more frequent snapshots
}
```

---

## Next Steps

1. **Test with testnet** before going live
2. **Monitor logs** for warnings/errors
3. **Set up alerts** for connection failures
4. **Implement strategies** using event streams
5. **Add order submission** for automated trading
6. **Configure user data streams** for order updates

---

## Support

- **Binance API Docs:** https://binance-docs.github.io/apidocs/spot/en/
- **WebSocket Streams:** https://github.com/binance/binance-spot-api-docs/blob/master/web-socket-streams.md
- **REST API:** https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md
- **Testnet:** https://testnet.binance.vision/

---

## Summary

✅ **Production-ready** Binance integration  
✅ **Auto-reconnection** with exponential backoff  
✅ **Rate limiting** enforced  
✅ **Authentication** supported  
✅ **All event types** working  
✅ **Clean architecture** maintained  
✅ **No shim code** - professional implementation  

Ready for live trading! 🚀
