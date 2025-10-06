# Exchange Bootstrap Package

This package provides a reusable scaffold for building exchange integrations in Meltica. It abstracts the common patterns for constructing transport clients, routers, and services across all exchanges.

## Overview

The bootstrap package enables:

- **Uniform construction pattern** - All exchanges follow the same initialization flow
- **Pluggable factories** - Exchange-specific implementations via factory functions
- **Flexible configuration** - Functional options for customization
- **Lifecycle management** - Centralized connect/close semantics for transports

## Core Types

### `ConstructionParams`

Holds all configuration needed to construct an exchange:
- `ConfigOpts` - config.Option slice for settings overrides
- `Transports` - Factory functions for REST/WS clients
- `Routers` - Factory functions for REST/WS routers

### `TransportBundle`

Encapsulates all transport components:
- REST client (coretransport.RESTClient)
- REST router (exchange-specific dispatcher)
- WebSocket client (coretransport.StreamClient)
- WebSocket router (exchange-specific router)

Provides lifecycle methods: `Close()` for graceful shutdown

### `Option`

Functional option pattern: `func(*ConstructionParams)`

Use to configure construction params before building the exchange.

## Usage Pattern

### 1. Define Exchange-Specific Defaults

```go
func defaultConstructionParams() *bootstrap.ConstructionParams {
    params := bootstrap.NewConstructionParams()

    // Set transport factories
    params.Transports = bootstrap.TransportFactories{
        NewRESTClient: func(cfg interface{}) coretransport.RESTClient {
            return myexchange.NewRESTClient(cfg.(myexchange.RESTConfig))
        },
        NewWSClient: func(cfg interface{}) coretransport.StreamClient {
            return myexchange.NewWSClient(cfg.(myexchange.WSConfig))
        },
    }

    // Set router factories
    params.Routers = bootstrap.RouterFactories{
        NewRESTRouter: func(client coretransport.RESTClient) interface{} {
            return myexchange.NewRESTRouter(client)
        },
        NewWSRouter: func(client coretransport.StreamClient, deps interface{}) interface{} {
            return myexchange.NewWSRouter(client, deps.(myexchange.WSDependencies))
        },
    }

    return params
}
```

### 2. Build Transport Bundle

```go
func newExchange(settings config.Settings, params *bootstrap.ConstructionParams) (*Exchange, error) {
    // Extract exchange-specific config
    restCfg := myexchange.RESTConfig{ /* ... */ }
    wsCfg := myexchange.WSConfig{ /* ... */ }

    // Build transports
    bundle := bootstrap.BuildTransportBundle(
        params.Transports,
        params.Routers,
        restCfg,
        wsCfg,
    )

    // Wire services
    restRouter := bundle.Router().(myexchange.RESTDispatcher)
    symbolService := myexchange.NewSymbolService(restRouter)
    // ... other services

    // Set WS router after wiring dependencies
    wsDeps := myexchange.NewWSDependencies(symbolService, /* ... */)
    bundle.SetWS(params.Routers.NewWSRouter(bundle.WSInfra(), wsDeps))

    return &Exchange{transports: bundle, /* ... */}, nil
}
```

### 3. Provide Public Options

```go
// WithRESTClient overrides the default REST client
func WithRESTClient(client coretransport.RESTClient) bootstrap.Option {
    return func(params *bootstrap.ConstructionParams) {
        params.Transports.NewRESTClient = func(interface{}) coretransport.RESTClient {
            return client
        }
    }
}

// WithCustomTimeout adds a config option
func WithCustomTimeout(timeout time.Duration) bootstrap.Option {
    return func(params *bootstrap.ConstructionParams) {
        params.ConfigOpts = append(params.ConfigOpts,
            config.WithExchangeHTTPTimeout("myexchange", timeout))
    }
}
```

### 4. Expose Constructor

```go
func New(apiKey, secret string, opts ...bootstrap.Option) (*Exchange, error) {
    params := defaultConstructionParams()
    params.ConfigOpts = append(params.ConfigOpts,
        config.WithExchangeCredentials("myexchange", apiKey, secret))

    bootstrap.ApplyOptions(params, opts...)

    settings := config.FromEnv()
    if len(params.ConfigOpts) > 0 {
        settings = config.Apply(settings, params.ConfigOpts...)
    }

    return newExchange(settings, params)
}
```

## Service Interfaces

Exchanges should provide service constructors that accept shared types:

- **Symbol Service** - `routingrest.RESTDispatcher` for exchange info queries
- **Listen Key Service** - `routingrest.RESTDispatcher` for user stream keys
- **Depth Snapshot** - `routingrest.RESTDispatcher` + symbol service for book snapshots

These services form the `WSDependencies` required by WebSocket routers.

## Example: Binance

See `exchanges/binance/` for a complete implementation:

1. `options.go` - Defines default factories and public options
2. `binance.go` - Uses bootstrap to build exchange
3. Service constructors accept `routingrest.RESTDispatcher`
4. WS dependencies implement `routing.WSDependencies`

## Testing

For testing, provide mock factories via options:

```go
mockClient := &MockRESTClient{}
exchange, err := myexchange.New("key", "secret",
    myexchange.WithRESTClient(mockClient))
```

## Future Exchanges

To add a new exchange (e.g., Bybit, OKX):

1. Create `exchanges/{name}/options.go` with default factories
2. Implement exchange-specific REST/WS clients
3. Create routing layer (REST dispatcher, WS router)
4. Wire services using shared interfaces
5. Use bootstrap to assemble in main constructor

The bootstrap package ensures consistency while allowing exchange-specific customization.
