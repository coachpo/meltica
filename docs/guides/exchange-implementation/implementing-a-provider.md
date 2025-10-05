# Implementing a Provider

This guide explains how to implement a new exchange provider in Meltica following the Level 1-3 architecture.

## Architecture Overview

Meltica uses a three-layer architecture for exchange integration:

### Level 1: Transport Layer
- **REST Client**: HTTP request/response handling
- **WebSocket Client**: Connection and message management
- **Shared Infrastructure**: Rate limiting, numeric helpers

### Level 2: Routing Layer  
- **REST Router**: Maps normalized requests to exchange endpoints
- **WebSocket Router**: Routes messages to normalized topics
- **Data Parsing**: Converts exchange formats to core types

### Level 3: Exchange Layer
- **Provider**: Unified exchange interface
- **Market Data**: Order books, tickers, trades
- **Private Data**: Orders, balances, positions

## Implementation Steps

### 1. Create Exchange Package Structure

Create a new directory under `exchanges/` following the Binance structure:

```
exchanges/your-exchange/
├── your-exchange.go         # Package entry point and main Exchange implementation
├── spot.go                  # Spot market implementation
├── linear_futures.go        # Linear futures implementation
├── inverse_futures.go       # Inverse futures implementation
├── depth.go                 # Depth/order book handling
├── orderbook_snapshot.go    # Order book snapshot logic
├── symbol_loader.go         # Symbol loading logic
├── symbol_registry.go       # Symbol registry
├── ws_service.go            # WebSocket service
├── infra/
│   ├── rest/
│   │   ├── client.go        # REST client
│   │   ├── errors.go        # REST error handling
│   │   └── sign.go          # Request signing
│   └── ws/
│       └── client.go        # WebSocket client
├── internal/
│   ├── errors.go            # Internal error types
│   └── status.go            # Status mapping
└── routing/
    ├── rest_router.go       # REST routing using shared dispatcher
    ├── ws_router.go         # WebSocket routing
    ├── parse_public.go      # Public data parsing
    ├── parse_private.go     # Private data parsing
    ├── orderbook_state.go   # Order book state management
    └── topics.go            # Topic mapping
```

### 2. Implement Level 1: Transport Layer

#### REST Client
Implement the `RESTClient` interface from `core/transport/transport_contracts.go`:

```go
type RESTClient interface {
    Connection
    DoRequest(ctx context.Context, req RESTRequest) (*RESTResponse, error)
    HandleResponse(ctx context.Context, req RESTRequest, resp *RESTResponse, out any) error
    HandleError(ctx context.Context, req RESTRequest, err error) error
}
```

#### WebSocket Client
Implement the `StreamClient` interface:

```go
type StreamClient interface {
    Connection
    Subscribe(ctx context.Context, topics ...StreamTopic) (StreamSubscription, error)
    Unsubscribe(ctx context.Context, sub StreamSubscription, topics ...StreamTopic) error
    Publish(ctx context.Context, message StreamMessage) error
    HandleError(ctx context.Context, err error) error
}
```

### 3. Implement Level 2: Routing Layer

#### REST Router
Use the shared REST dispatcher pattern from `exchanges/shared/routing/rest_dispatcher.go`:

```go
// Map normalized requests to exchange-specific endpoints
func (r *Router) MapRequest(req core.Request) (*exchange.RESTRequest, error) {
    // Implementation details
}
```

#### WebSocket Router
Implement the `Router` interface from `core/streams/routing.go`:

```go
type Router interface {
    SubscribePublic(ctx context.Context, topics ...string) (Subscription, error)
    SubscribePrivate(ctx context.Context) (Subscription, error)
    Close() error
}
```

### 4. Implement Level 3: Exchange Layer

#### Provider Implementation
Create the main exchange that implements the Exchange interface and relevant participant interfaces following the Binance pattern:

```go
package your_exchange

import (
    "context"
    "sync"
    
    "github.com/coachpo/meltica/config"
    "github.com/coachpo/meltica/core"
    "github.com/coachpo/meltica/exchanges/your-exchange/infra/rest"
    "github.com/coachpo/meltica/exchanges/your-exchange/infra/ws"
    "github.com/coachpo/meltica/exchanges/your-exchange/routing"
    routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type Exchange struct {
    name string

    restClient *rest.Client
    restRouter routingrest.RESTDispatcher

    wsInfra  *ws.Client
    wsRouter *routing.WSRouter

    instCache map[core.Market]map[string]core.Instrument
    symbols   *symbolRegistry
    symbolsMu sync.RWMutex
    cfg       config.Settings
    cfgMu     sync.Mutex
}

// Implement Exchange interface
func (x *Exchange) Name() string {
    return "your-exchange"
}

func (x *Exchange) Capabilities() core.ExchangeCapabilities {
    return core.Capabilities(
        core.CapabilitySpotPublicREST,
        core.CapabilitySpotTradingREST,
        core.CapabilityLinearFutures: true,
        core.CapabilityInverseFutures: true,
    )
}

func (x *Exchange) SupportedProtocolVersion() string {
    return core.ProtocolVersion
}

func (x *Exchange) Close() error {
    if x.wsRouter != nil {
        _ = x.wsRouter.Close()
    }
    return nil
}

// Implement participant interfaces
func (x *Exchange) Spot(ctx context.Context) core.SpotAPI {
    return &spotAPI{exchange: x}
}

func (x *Exchange) LinearFutures(ctx context.Context) core.FuturesAPI {
    return &linearAPI{exchange: x}
}

func (x *Exchange) InverseFutures(ctx context.Context) core.FuturesAPI {
    return &inverseAPI{exchange: x}
}

func (x *Exchange) WS() core.WS {
    return newWSService(x.wsRouter)
}
```

#### Market Data Implementation
Implement spot, linear futures, and inverse futures interfaces following the Binance pattern:

```go
// Spot market implementation
type spotAPI struct {
    exchange *Exchange
}

func (s *spotAPI) Ticker(ctx context.Context, symbol string) (*core.Ticker, error) {
    // Implementation using shared REST dispatcher
}
```

### 5. Configuration

Add exchange configuration in `config/config.go`:

```go
// Add exchange constant
const ExchangeYourExchange Exchange = "your-exchange"

// Add to default settings
func Default() *Config {
    return &Config{
        Exchanges: map[Exchange]ExchangeSettings{
            // ... existing exchanges
            ExchangeYourExchange: {
                REST: map[string]string{
                    "spot": "https://api.your-exchange.com",
                    "futures": "https://fapi.your-exchange.com",
                },
                Websocket: map[string]string{
                    "public": "wss://stream.your-exchange.com",
                    "private": "wss://stream.your-exchange.com",
                },
            },
        },
    }
}
```

### 6. Testing

Follow the testing patterns from the Binance implementation:

- Unit tests for parsing and normalization
- Integration tests for REST and WebSocket flows
- Use recorded fixtures for reliable testing

## Best Practices

1. **Reuse Shared Infrastructure**: Use routing patterns from `exchanges/shared/routing/` and numeric helpers from `exchanges/shared/infra/numeric/`
2. **Follow Error Handling Patterns**: Use the standardized error types and status mapping from the `errs` package
3. **Implement Comprehensive Testing**: Cover all market types and data flows using the Binance testing patterns
4. **Document Edge Cases**: Note any exchange-specific quirks or limitations

## Example Implementation

See the Binance implementation in `exchanges/binance/` for a complete reference implementation that follows all current patterns.
