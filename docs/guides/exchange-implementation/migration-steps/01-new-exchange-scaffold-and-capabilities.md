# Step 1: New Exchange Scaffold and Capabilities

This step creates the basic directory structure and capability definitions for a new exchange provider.

## Architecture Context

Meltica uses a three-layer architecture:
- **Level 1**: Transport layer (REST/WebSocket clients)
- **Level 2**: Routing layer (request/response mapping)  
- **Level 3**: Exchange layer (provider interface)

## Step 1.1: Create Directory Structure

Create the following directory structure under `exchanges/` following the Binance pattern:

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

## Step 1.2: Define Exchange Configuration

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

## Step 1.3: Create Basic Exchange Structure

Create the main exchange file `exchanges/your-exchange/your-exchange.go` following the Binance pattern:

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
    name       string
    transports *transportBundle
    symbols    *symbolService
    cfg        config.Settings
    cfgMu      sync.Mutex
}

// transportBundle and symbolService mirror the Binance adapter helpers for managing
// transport lifecycles and symbol caches. Implement equivalents for your exchange.

var capabilities = core.Capabilities(
    // Define your exchange's capabilities here
    core.CapabilitySpotPublicREST,
    core.CapabilitySpotTradingREST,
    core.CapabilityWebsocketPublic,
)

func NewWithSettings(settings config.Settings) (*Exchange, error) {
    // Resolve exchange-specific settings
    yourCfg := resolveYourExchangeSettings(settings)
    
    restClient := rest.NewClient(rest.Config{
        APIKey:      yourCfg.Credentials.APIKey,
        Secret:      yourCfg.Credentials.APISecret,
        SpotBaseURL: yourCfg.REST["spot"],
        Timeout:     yourCfg.HTTPTimeout,
    })
    
    restRouter := routing.NewRESTRouter(restClient)
    wsInfra := ws.NewClient(ws.Config{
        PublicURL:        yourCfg.Websocket.PublicURL,
        PrivateURL:       yourCfg.Websocket.PrivateURL,
        HandshakeTimeout: yourCfg.HandshakeTimeout,
    })

    transports := newTransportBundle(restClient, restRouter, wsInfra, nil)
    symbols := newSymbolService(restRouter)

    x := &Exchange{
        name:       "your-exchange",
        transports: transports,
        symbols:    symbols,
        cfg:        settings,
    }
    transports.ws = routing.NewWSRouter(wsInfra, x)
    return x, nil
}

func (x *Exchange) Name() string { return x.name }

func (x *Exchange) Capabilities() core.ExchangeCapabilities { return capabilities }

func (x *Exchange) SupportedProtocolVersion() string { return core.ProtocolVersion }

func (x *Exchange) Spot(ctx context.Context) core.SpotAPI { return spotAPI{x} }

func (x *Exchange) WS() core.WS { return newWSService(x.wsRouter) }

func (x *Exchange) Close() error {
    if x.wsRouter != nil {
        _ = x.wsRouter.Close()
    }
    return nil
}
```

## Step 1.4: Create Market Implementations

Create market-specific implementations following the Binance pattern:

```go
// exchanges/your-exchange/spot.go
package your_exchange

import (
    "context"
    "github.com/coachpo/meltica/core"
)

type spotAPI struct {
    exchange *Exchange
}

func (s spotAPI) ServerTime(ctx context.Context) (time.Time, error) {
    // Implementation
}

func (s spotAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
    // Implementation
}

func (s spotAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
    // Implementation
}

// ... other spot methods
```

## Step 1.5: Define Supported Capabilities

Document which features your exchange supports:

- [ ] Spot trading
- [ ] Linear futures
- [ ] Inverse futures  
- [ ] WebSocket public streams
- [ ] WebSocket private streams
- [ ] Order book streaming
- [ ] Real-time tickers
- [ ] Trade history
- [ ] Account balances
- [ ] Order management

## Step 1.6: Verify Build

Test that the basic structure compiles:

```bash
cd exchanges/your-exchange
go build ./...
```

## Success Criteria

- [ ] Directory structure created
- [ ] Configuration added to `config/config.go`
- [ ] Basic provider structure implemented
- [ ] Package entry point created
- [ ] Code compiles without errors
- [ ] Capabilities documented

## Next Steps

After completing this step, proceed to:

- **Step 2**: Implement REST surfaces for spot and futures
- **Step 3**: Implement WebSocket public streams
- **Step 4**: Implement WebSocket private streams
- **Step 5**: Error status mapping and normalization
- **Step 6**: Conformance schemas, CI, and release

## Notes

- Use the Binance implementation in `exchanges/binance/` as a reference for all patterns
- Follow the established directory structure without nested `exchange/` subdirectory
- Use shared routing infrastructure from `exchanges/shared/routing/`
- Ensure all interfaces match the current `core` contracts
- Implement symbol conversion and caching similar to Binance pattern
