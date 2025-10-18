# Multi-Provider Architecture

Run multiple market data providers simultaneously with automatic event stream merging.

## Overview

The gateway now supports running **multiple providers at the same time**:
- ✅ **Fake + Binance** together
- ✅ **Future-proof** for Coinbase, Kraken, FTX, etc.
- ✅ **Event stream fan-in** - All providers merge into single dispatcher
- ✅ **Per-provider lambdas** - Each lambda gets events from its specific provider
- ✅ **Memory efficient** - Shared object pools across all providers

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Gateway Main                              │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │  Fake    │  │ Binance  │  │ Coinbase │  │  Kraken  │       │
│  │ Provider │  │ Provider │  │ Provider │  │ Provider │       │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘       │
│       │             │              │             │              │
│       │ Events      │ Events       │ Events      │ Events       │
│       │             │              │             │              │
│       └─────────────┴──────────────┴─────────────┘              │
│                          │                                       │
│                    Fan-In Merge                                  │
│                          │                                       │
│                  ┌───────▼───────┐                              │
│                  │  Merged Stream │                              │
│                  └───────┬───────┘                              │
│                          │                                       │
│                  ┌───────▼───────┐                              │
│                  │   Dispatcher   │                              │
│                  └───────┬───────┘                              │
│                          │                                       │
│              ┌───────────┴────────────┐                         │
│              │                        │                          │
│        ┌─────▼─────┐           ┌─────▼─────┐                   │
│        │  Lambda 1  │           │  Lambda 2  │                   │
│        │ (fake/BTC) │           │ (binance/  │                   │
│        │            │           │   BTC)     │                   │
│        └───────────┘           └───────────┘                   │
└─────────────────────────────────────────────────────────────────┘
```

## Usage

### Command-Line Flag

```bash
# Single provider (default)
./bin/gateway --providers=fake

# Multiple providers (comma-separated)
./bin/gateway --providers=fake,binance

# Future: Many providers
./bin/gateway --providers=fake,binance,coinbase,kraken
```

### Configuration Examples

#### Development: Fake Only
```bash
./bin/gateway --providers=fake
```
- Fast startup
- No API keys needed
- Simulated market data

#### Testing: Both Fake and Binance
```bash
export BINANCE_API_KEY="testnet_key"
export BINANCE_SECRET_KEY="testnet_secret"
export BINANCE_USE_TESTNET="true"

./bin/gateway --providers=fake,binance
```
- Compare fake vs real data side-by-side
- Validate strategy behavior
- Test reconnection logic

#### Production: Binance Only
```bash
export BINANCE_API_KEY="live_key"
export BINANCE_SECRET_KEY="live_secret"

./bin/gateway --providers=binance
```
- Real market data only
- Production trading

#### Production: Multiple Live Exchanges
```bash
export BINANCE_API_KEY="..."
export BINANCE_SECRET_KEY="..."
export COINBASE_API_KEY="..."
export COINBASE_SECRET_KEY="..."

./bin/gateway --providers=binance,coinbase
```
- Aggregate liquidity
- Cross-exchange arbitrage
- Price comparison

## Event Flow

### Provider-Specific Events

Each event has a `Provider` field identifying its source:

```go
type Event struct {
    Type      EventType
    Provider  string      // "fake", "binance", "coinbase", etc.
    Symbol    string
    Timestamp time.Time
    // ... payload
}
```

### Lambda Filtering

Lambdas filter events by provider:

```go
lambdaConfigs := []lambda.Config{
    // Fake provider lambdas
    {Symbol: "BTC-USDT", Provider: "fake"},
    {Symbol: "ETH-USDT", Provider: "fake"},
    
    // Binance provider lambdas
    {Symbol: "BTC-USDT", Provider: "binance"},
    {Symbol: "ETH-USDT", Provider: "binance"},
}
```

Each lambda:
1. Subscribes to data bus
2. Filters events matching `cfg.Provider`
3. Processes only its provider's events
4. Submits orders back to its provider

### Fan-In Pattern

```go
// Create providers
fake := createFakeProvider()
binance := createBinanceProvider()
coinbase := createCoinbaseProvider()

// Collect event channels
eventChannels := []<-chan *schema.Event{
    fake.Events(),
    binance.Events(),
    coinbase.Events(),
}

// Merge into single channel
mergedEvents := make(chan *schema.Event, 512)
for _, ch := range eventChannels {
    go func(src <-chan *schema.Event) {
        for evt := range src {
            mergedEvents <- evt
        }
    }(ch)
}

// Dispatcher consumes merged stream
dispatcher.Start(ctx, mergedEvents)
```

## Memory Efficiency

### Shared Object Pools

All providers share the same object pools:

```go
poolMgr := pool.NewPoolManager()

// WsFrame (200 capacity):
//   - Used by ALL WebSocket parsers
//   - Binance, Coinbase, Kraken all share this pool
//   - Hot path: Every WebSocket message
registerPool("WsFrame", 200, func() interface{} {
    return new(schema.WsFrame)
})

// ProviderRaw (200 capacity):
//   - Exchange-agnostic intermediate storage
//   - Used during JSON parsing and normalization
//   - Supports all provider types
registerPool("ProviderRaw", 200, func() interface{} {
    return new(schema.ProviderRaw)
})

// Event (1000 capacity):
//   - Canonical events from all providers
//   - Main message type flowing through system
registerPool("Event", 1000, func() interface{} {
    return new(schema.Event)
})
```

**Benefits:**
- ✅ Single pool instance per type
- ✅ No per-provider pool overhead
- ✅ Automatic reuse across exchanges
- ✅ Reduced GC pressure
- ✅ Better cache locality

### Memory Flow Example

```
┌────────────────┐
│ Binance WS Msg │  ← Raw bytes from network
└───────┬────────┘
        │ Get from pool
        ▼
┌────────────────┐
│    WsFrame     │  ← Shared pool (200 capacity)
└───────┬────────┘
        │ Parse JSON
        ▼
┌────────────────┐
│  ProviderRaw   │  ← Shared pool (200 capacity)
└───────┬────────┘
        │ Normalize
        ▼
┌────────────────┐
│     Event      │  ← Shared pool (1000 capacity)
└───────┬────────┘
        │
        ▼
    Dispatcher

After processing:
- Return WsFrame to pool
- Return ProviderRaw to pool
- Return Event to pool (after all consumers done)
```

## Order Routing

Currently uses **primary provider** (first in list) for order submission:

```go
primaryProvider := activeProviders[0].provider

controller := dispatcher.NewController(
    table,
    controlBus,
    subscriptionManager,
    dispatcher.WithOrderSubmitter(primaryProvider),  // Orders go here
    dispatcher.WithTradingState(tradingState),
)
```

**Future Enhancement:**
- Smart routing based on provider availability
- Best execution across multiple venues
- Failover to backup providers

## Subscription Management

Routes are subscribed on **all active providers**:

```go
for _, route := range table.Routes() {
    for _, p := range activeProviders {
        if err := p.provider.SubscribeRoute(route); err != nil {
            logger.Printf("subscribe %s on %s: %v", 
                route.Type, p.name, err)
        }
    }
}
```

This ensures:
- ✅ All providers send relevant data
- ✅ Redundancy if one provider fails
- ✅ Data comparison across exchanges

## Error Handling

Errors from all providers are merged and logged:

```go
mergedErrors := make(chan error, 64)

// Fan-in errors from all providers
for _, errChan := range errorChannels {
    go func(ch <-chan error, name string) {
        for err := range ch {
            mergedErrors <- fmt.Errorf("%s: %w", name, err)
        }
    }(errChan, providerName)
}

// Log with provider identification
logger.Printf("providers: %v", err)  // "binance: connection lost"
```

## Implementation Details

### Provider Creation

```go
func createMultipleProviders(
    ctx context.Context, 
    providerTypes []string, 
    poolMgr *pool.PoolManager, 
    logger *log.Logger,
) ([]providerInstance, <-chan *schema.Event, <-chan error) {
    instances := []providerInstance{}
    eventChannels := []<-chan *schema.Event{}
    errorChannels := []<-chan error{}

    // Start each provider
    for _, provType := range providerTypes {
        provider, name, err := createProvider(ctx, provType, poolMgr, logger)
        if err != nil {
            logger.Printf("failed to create %s: %v", provType, err)
            continue
        }
        instances = append(instances, providerInstance{
            name:     name,
            provider: provider,
        })
        eventChannels = append(eventChannels, provider.Events())
        errorChannels = append(errorChannels, provider.Errors())
    }

    // Fan-in merge
    mergedEvents := fanInEvents(ctx, eventChannels)
    mergedErrors := fanInErrors(ctx, errorChannels)

    return instances, mergedEvents, mergedErrors
}
```

### Provider Parsing

```go
func parseProviders(input string) []string {
    // "fake,binance" → ["fake", "binance"]
    // "binance" → ["binance"]
    // "" → ["fake"]
    
    if input == "" {
        return []string{"fake"}
    }
    
    parts := []string{}
    for _, supported := range []string{"fake", "binance"} {
        if contains(input, supported) {
            parts = append(parts, supported)
        }
    }
    
    return parts
}
```

## Benefits

### 1. **Development Flexibility**
- Test with fake data while real provider is being implemented
- Compare synthetic vs real data behavior
- Validate strategies without market risk

### 2. **Resilience**
- Continue operating if one provider fails
- Redundant data sources
- Automatic failover potential

### 3. **Performance**
- Shared memory pools reduce allocations
- Parallel provider operations
- Single merged stream simplifies dispatcher

### 4. **Extensibility**
- Add new exchanges without changing core logic
- Each provider is independent
- Clean separation of concerns

### 5. **Testing**
- Run fake alongside real for validation
- A/B test different providers
- Monitor latency differences

## Future Enhancements

### Smart Order Routing
```go
// Route orders to best venue
type SmartRouter struct {
    providers map[string]MarketDataProvider
}

func (r *SmartRouter) SubmitOrder(ctx context.Context, req OrderRequest) error {
    // Choose provider based on:
    // - Liquidity
    // - Latency
    // - Fees
    // - Availability
    provider := r.selectBestProvider(req)
    return provider.SubmitOrder(ctx, req)
}
```

### Cross-Exchange Arbitrage
```go
// Compare prices across venues
type ArbitrageDetector struct {
    prices map[string]map[string]float64 // provider -> symbol -> price
}

func (a *ArbitrageDetector) OnBookSnapshot(evt *Event) {
    a.prices[evt.Provider][evt.Symbol] = evt.BestBid
    
    // Detect arbitrage opportunities
    if priceDiff := a.calculateSpread(evt.Symbol); priceDiff > threshold {
        // Buy on cheaper, sell on expensive
    }
}
```

### Dynamic Provider Management
```go
// Add/remove providers at runtime
func (g *Gateway) AddProvider(ctx context.Context, name string, provider MarketDataProvider) {
    g.providers[name] = provider
    g.mergeEventStream(provider.Events())
}

func (g *Gateway) RemoveProvider(name string) {
    provider := g.providers[name]
    provider.Close()
    delete(g.providers, name)
}
```

## Monitoring

### Logs

```
gateway: starting providers: [fake binance]
gateway: provider fake started successfully
gateway: provider binance started successfully
gateway: providers running: 2
gateway: binance: testnet=false, api_key=abcd...wxyz, streams=12
gateway: subscribe route book on fake: success
gateway: subscribe route book on binance: success
lambda fake/BTC-USDT: started
lambda binance/BTC-USDT: started
gateway: control API listening on :8880
```

### Metrics (Future)

```
provider_events_total{provider="fake"} 1523
provider_events_total{provider="binance"} 987
provider_errors_total{provider="fake"} 0
provider_errors_total{provider="binance"} 2
provider_latency_ms{provider="fake",p99="0.1"}
provider_latency_ms{provider="binance",p99="45.3"}
```

## Summary

The multi-provider architecture enables:
- ✅ **Simultaneous execution** of multiple data sources
- ✅ **Clean fan-in pattern** for event merging
- ✅ **Memory efficiency** via shared object pools
- ✅ **Future-proof design** for any number of exchanges
- ✅ **Independent provider lifecycles** with fault isolation

**Usage:**
```bash
./bin/gateway --providers=fake,binance,coinbase,kraken
```

All providers run in parallel, events merge automatically, lambdas filter by provider!
