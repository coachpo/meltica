# Implementing a Provider

This guide explains how to implement a new exchange provider in Meltica following the current four-layer architecture.

## Architecture Overview

Meltica composes exchange adapters from four logical layers:

### Level 1: Transport Layer
- **REST Client** – Executes HTTP requests, handles retries, rate limits, and canonical error conversion (`core/transport.RESTClient`).
- **WebSocket Client** – Manages streaming connections, reconnect/backoff, heartbeats, and raw frame delivery (`core/transport.StreamClient`).
- **Bootstrap Helpers** – `core/exchanges/bootstrap` wires transport factories with a unified `TransportConfig`.

### Level 2: Routing Layer
- **REST Dispatcher** – Maps canonical REST requests to exchange endpoints using `exchanges/shared/routing.DefaultRESTRouter` or a custom adapter.
- **WebSocket Router** – Parses raw frames into routed messages via `routing/dispatchers.go`, the `StreamRegistry`, and `ws_router.go` helpers.
- **Data Parsing** – Converts transport payloads into normalized core types while enforcing symbol/decimal policies.

### Level 3: Exchange Layer
- **Provider** – Implements `core.Exchange`, capability reporting, participant accessors, and lifecycle management.
- **Domain Services** – Symbol registries, listen-key managers, book services, and REST bridges that provide higher-level behaviour.
- **Capability Surface** – Spot, linear, inverse, and WebSocket participants expose canonical APIs.

### Level 4: Filter & Client Facade
- **Pipeline Adapter** – Implements `pipeline.Adapter` to expose Level 3 services to the multi-channel filter pipeline.
- **REST Bridge** – Bridges Level 3 REST dispatchers into Level 4 request/response events.
- **Private Session Manager** – Coordinates listen-key lifecycles and private stream decoding for pipeline consumers.

## Implementation Steps

### 1. Create Exchange Package Structure

Start from the Binance adapter layout and replicate the same high-level structure for your exchange:

```
exchanges/your-exchange/
├── your-exchange.go        # Exchange type and bootstrap wiring
├── options.go              # Public constructor options (wrapping bootstrap)
├── spot.go                 # Spot REST implementation
├── futures.go              # Shared futures helpers
├── futures_linear.go       # Linear futures API surface
├── futures_inverse.go      # Inverse futures API surface
├── book_service.go         # Level-3 book aggregation service
├── book_state.go           # In-memory depth state management
├── listen_key_service.go   # Listen-key lifecycle helpers (if required)
├── symbol_service.go       # Symbol discovery & caching
├── symbol_registry.go      # Canonical/native symbol registry
├── ws_service.go           # WS participant wrapper
├── ws_dependencies.go      # Glue between Level 2 routers and Level 3 services
├── filter/                 # Level-4 adapter + REST bridge
├── infra/
│   ├── rest/               # REST client, signing, error mapping
│   └── ws/                 # Streaming client implementation
├── internal/               # Exchange-specific errors/status mapping
├── plugin/                 # Registry + translator registration
└── routing/
    ├── dispatchers.go      # Message dispatch + decode entry points
    ├── parse_helpers.go    # Shared decode helpers
    ├── stream_registry.go  # Topic registry + builders
    ├── ws_router.go        # High level router backed by StreamRegistry
    └── rest_router.go      # REST dispatcher wiring
```

### 2. Bootstrap Transports

Configure transports using `core/exchanges/bootstrap`:

```go
func defaultConstructionParams() *bootstrap.ConstructionParams {
    params := bootstrap.NewConstructionParams()

    params.Transports = bootstrap.TransportFactories{
        NewRESTClient: func(cfg bootstrap.TransportConfig) coretransport.RESTClient {
            return rest.NewClient(rest.Config{
                SpotBaseURL: cfg.SpotBaseURL,
                LinearBaseURL: cfg.LinearBaseURL,
                InverseBaseURL: cfg.InverseBaseURL,
                APIKey: cfg.APIKey,
                Secret: cfg.Secret,
                Timeout: cfg.HTTPTimeout,
            })
        },
        NewWSClient: func(cfg bootstrap.TransportConfig) coretransport.StreamClient {
            return ws.NewClient(ws.Config{
                PublicURL:  cfg.PublicURL,
                PrivateURL: cfg.PrivateURL,
                Handshake:  cfg.HandshakeTimeout,
            })
        },
    }

    params.Routers = bootstrap.RouterFactories{
        NewRESTRouter: func(client coretransport.RESTClient) interface{} {
            return routingrest.NewDefaultRESTRouter(client, nil)
        },
        NewWSRouter: func(client coretransport.StreamClient, deps interface{}) interface{} {
            return routing.NewRouter(client, deps.(routing.WSDependencies))
        },
    }

    return params
}
```

At construction time, resolve configuration via `config.Settings`, build a `bootstrap.TransportBundle`, and inject your Level 3 services:

```go
transportCfg := bootstrap.TransportConfig{
    APIKey:           venueCfg.Credentials.APIKey,
    Secret:           venueCfg.Credentials.APISecret,
    SpotBaseURL:      venueCfg.REST[config.BinanceRESTSurfaceSpot],
    LinearBaseURL:    venueCfg.REST[config.BinanceRESTSurfaceLinear],
    InverseBaseURL:   venueCfg.REST[config.BinanceRESTSurfaceInverse],
    HTTPTimeout:      venueCfg.HTTPTimeout,
    PublicURL:        venueCfg.Websocket.PublicURL,
    PrivateURL:       venueCfg.Websocket.PrivateURL,
    HandshakeTimeout: venueCfg.HandshakeTimeout,
}

bundle := bootstrap.BuildTransportBundle(factories, routers, transportCfg)
restRouter := bundle.Router().(routingrest.RESTDispatcher)
symbols := newSymbolService(restRouter)
deps := newWSDependencies(exchangeName, symbols, listenKeys, nil)
bundle.SetWS(routers.NewWSRouter(bundle.WSInfra(), deps))
```

### 3. Implement Transport Clients

Provide concrete implementations for the transport interfaces:

- `infra/rest/client.go` satisfies `coretransport.RESTClient` (timeouts, retries, signing, canonical error conversion).
- `infra/ws/client.go` satisfies `coretransport.StreamClient` (backoff, heartbeat, multiplexed subscriptions).
- Expose configuration knobs via `options.go` using bootstrap `Option` helpers.

### 4. Implement Routing

Level 2 converts transport payloads into normalized events:

- `routing/dispatchers.go` wires the `StreamRegistry` to public/private dispatchers and emits `corestreams.RoutedMessage`s.
- `routing/stream_registry.go` owns topic builders (`Trade`, `Ticker`, `OrderBook`, etc.) and decodes JSON into `corestreams` events while enforcing canonical symbol formatting.
- `routing/ws_router.go` implements the Level-2 `Router` that Level 3 services use.
- `routing/rest_router.go` adapts REST calls through `routingrest.RESTDispatcher`.

Ensure every decoder returns concrete event types (`corestreams.TradeEvent`, `TickerEvent`, `BookEvent`, `OrderEvent`, `BalanceEvent`) and surfaces route metadata (`RouteTradeUpdate`, etc.).

### 5. Build Level-3 Services & Exchange

- `symbol_service.go` loads and caches instruments per market, exposes canonical/native lookups, and refresh hooks.
- `listen_key_service.go` (if required) manages user-stream keys.
- `book_service.go` couples WS deltas with snapshot REST endpoints using `book_state.go` for gap detection and recovery.
- `ws_dependencies.go` implements `routing.WSDependencies`, bridging symbol translation, listen-key access, and snapshot hydration for Level 2.
- `<name>.go` constructs the `Exchange`, reports capabilities, exposes participant interfaces (`Spot`, `LinearFutures`, `InverseFutures`, `WS`), and manages transport lifecycle.

Register translators and factories in `plugin/plugin.go`:

```go
func Register() {
    coreregistry.Register(Name, func(cfg coreregistry.Config) (core.Exchange, error) {
        x, err := exchange.New(cfg.APIKey, cfg.APISecret)
        if err != nil {
            return nil, err
        }
        core.RegisterSymbolTranslator(Name, exchange.NewSymbolTranslator(x))
        core.RegisterTopicTranslator(Name, exchange.NewTopicTranslator())
        return x, nil
    })
}
```

### 6. Expose the Filter Adapter

Level 4 adapters make the exchange consumable by the pipeline:

- `filter/adapter.go` implements `pipeline.Adapter`, wiring book, trade, ticker, private streams, and REST execution via the Level 3 services.
- `filter/session_manager.go` (if needed) refreshes listen keys and multiplexes private feeds.
- `filter/rest_bridge.go` (or equivalent) converts pipeline REST requests into dispatcher calls with retry policies.

Ensure `Capabilities()` accurately reflects supported feeds (`Books`, `Trades`, `Tickers`, `PrivateStreams`, `RESTEndpoints`).

### 7. Testing

Adopt the Binance test layout and extend it for the new venue:

- Unit tests for routing (`routing/ws_router_test.go`, `routing/dispatchers_test.go`).
- Book service and numeric helpers tests (`book_service_test.go`, `ticker_helpers_test.go`).
- REST configuration tests (`exchange_config_test.go`).
- Integration-style tests exercising the filter adapter and pipeline bridging.
- Keep fixtures deterministic and ensure tests skip cleanly when credentials are absent.

## Best Practices

1. **Leverage shared infrastructure** – Reuse `exchanges/shared` dispatchers, numeric helpers, retry policies, and bootstrap helpers instead of reinventing them.
2. **Enforce canonical models** – All symbols must be `BASE-QUOTE`, numerics must be `*big.Rat`, and enums must be exhaustively mapped with error returns on unknown values.
3. **Surface accurate capabilities** – Capabilities must only advertise features that pass automated tests and integration checks.
4. **Test every layer** – Unit-test routing and services, run `go test ./... -race -count=1`, and add integration smoke tests for the Level-4 pipeline.
5. **Document quirks** – Record exchange-specific behaviour (rate limits, sequencing, auth) in the adapter README to help reviewers and operators.

## Reference Implementation

Use `exchanges/binance/` as the canonical, up-to-date example for new exchanges. It demonstrates the complete stack: bootstrap wiring, routing, services, filter adapter, plugin registration, and tests.
