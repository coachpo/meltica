# Step 1: New Exchange Scaffold and Capabilities

This step creates the basic directory structure and capability definitions for a new exchange provider.

## Architecture Context

Meltica uses a four-layer architecture:
- **Level 1**: Transport layer (REST/WebSocket clients)
- **Level 2**: Routing layer (request/response mapping)
- **Level 3**: Exchange layer (provider interface)
- **Level 4**: Pipeline layer (filtering, aggregation, and client facade)

## Step 1.1: Create Directory Structure

Create the following directory structure under `exchanges/` following the Binance pattern (trim or expand pieces that your venue needs):

```
exchanges/your-exchange/
├── your-exchange.go       # Exchange entry point + bootstrap wiring
├── options.go             # Public constructors & bootstrap options
├── spot.go                # Spot REST implementation
├── futures.go             # Shared futures helpers
├── futures_linear.go      # Linear futures API
├── futures_inverse.go     # Inverse futures API
├── book_service.go        # Level-3 order book service
├── book_state.go          # In-memory depth state tracking
├── listen_key_service.go  # Listen-key lifecycle helpers (if needed)
├── symbol_service.go      # Symbol loading + caching
├── symbol_registry.go     # Canonical/native registry
├── ws_service.go          # WS participant façade
├── ws_dependencies.go     # Bridge between routing and services
├── filter/                # Level-4 pipeline adapter + REST bridge
├── infra/
│   ├── rest/              # REST client, signing, error handling
│   └── ws/                # WebSocket client implementation
├── internal/              # Exchange-specific errors, status mapping
├── plugin/                # Registry + translator registration
└── routing/
    ├── dispatchers.go     # Stream dispatch entry points
    ├── parse_helpers.go   # Shared decode helpers
    ├── stream_registry.go # Topic builders + decoders
    ├── ws_router.go       # Level-2 router implementation
    └── rest_router.go     # REST dispatcher wiring
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
    "github.com/coachpo/meltica/core/exchanges/bootstrap"
    routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type Exchange struct {
    name            string
    transportBundle *bootstrap.TransportBundle
    symbolSvc       *symbolService
    listenKeySvc    *listenKeyService
    bookSvc         *BookService
    cfg             config.Settings
    cfgMu           sync.Mutex
}

// transportBundle and the services mirror the Binance adapter helpers for managing
// transport lifecycles, symbol caches, listen keys, and book state. Implement
// equivalents for your exchange.

var capabilities = core.Capabilities(
    // Define your exchange's capabilities here
    core.CapabilitySpotPublicREST,
    core.CapabilitySpotTradingREST,
    core.CapabilityWebsocketPublic,
)

func NewWithSettings(settings config.Settings) (*Exchange, error) {
    yourCfg := resolveYourExchangeSettings(config.Apply(settings))

    transportCfg := bootstrap.TransportConfig{
        APIKey:           yourCfg.Credentials.APIKey,
        Secret:           yourCfg.Credentials.APISecret,
        SpotBaseURL:      yourCfg.REST["spot"],
        LinearBaseURL:    yourCfg.REST["linear"],
        InverseBaseURL:   yourCfg.REST["inverse"],
        HTTPTimeout:      yourCfg.HTTPTimeout,
        PublicURL:        yourCfg.Websocket.PublicURL,
        PrivateURL:       yourCfg.Websocket.PrivateURL,
        HandshakeTimeout: yourCfg.HandshakeTimeout,
    }

    bundle := bootstrap.BuildTransportBundle(defaultTransportFactories(), defaultRouterFactories(), transportCfg)
    restRouter := bundle.Router().(routingrest.RESTDispatcher)
    symbolSvc := newSymbolService(restRouter)
    listenKeySvc := newListenKeyService(restRouter)
    deps := newWSDependencies(exchangeName, symbolSvc, listenKeySvc, nil)
    bundle.SetWS(defaultRouterFactories().NewWSRouter(bundle.WSInfra(), deps))
    bookSvc := newBookService(bundle.WS().(wsRouter), restRouter, symbolSvc)

    return &Exchange{
        name:            string(exchangeName),
        transportBundle: bundle,
        symbolSvc:       symbolSvc,
        listenKeySvc:    listenKeySvc,
        bookSvc:         bookSvc,
        cfg:             config.Apply(settings),
    }, nil
}

func (x *Exchange) Name() string { return x.name }

func (x *Exchange) Capabilities() core.ExchangeCapabilities { return capabilities }

func (x *Exchange) SupportedProtocolVersion() string { return core.ProtocolVersion }

func (x *Exchange) Spot(ctx context.Context) core.SpotAPI         { return spotAPI{x} }
func (x *Exchange) LinearFutures(ctx context.Context) core.FuturesAPI { return newLinearFuturesAPI(x) }
func (x *Exchange) InverseFutures(ctx context.Context) core.FuturesAPI { return newInverseFuturesAPI(x) }

func (x *Exchange) WS() core.WS { return newWSService(x.wsRouter()) }

func (x *Exchange) Close() error {
    if x.transportBundle != nil {
        return x.transportBundle.Close()
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

- Use the Binance implementation in `exchanges/binance/` as a reference for all patterns, including `filter/`, `routing/`, and `plugin/`
- Follow the established directory structure without nested `exchange/` subdirectories
- Use shared bootstrap and routing infrastructure (`core/exchanges/bootstrap`, `exchanges/shared/routing`)
- Ensure all interfaces match the current `core` contracts
- Implement symbol conversion and caching similar to the Binance pattern and register translators in the plugin package
