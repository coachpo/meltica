# Step 1: New Exchange Scaffold and Capabilities

This step creates the basic directory structure and capability definitions for a new exchange provider.

## Architecture Context

Meltica uses a three-layer architecture:
- **Level 1**: Transport layer (REST/WebSocket clients)
- **Level 2**: Routing layer (request/response mapping)  
- **Level 3**: Exchange layer (provider interface)

## Step 1.1: Create Directory Structure

Create the following directory structure under `exchanges/`:

```
exchanges/your-exchange/
├── exchange/
│   ├── provider.go      # Main provider implementation
│   ├── spot.go          # Spot market implementation
│   ├── linear_futures.go # Linear futures implementation
│   ├── inverse_futures.go # Inverse futures implementation
│   ├── symbol_loader.go # Symbol loading logic
│   ├── symbol_registry.go # Symbol registry
│   ├── orderbook_stream.go # Order book streaming
│   └── ws_service.go    # WebSocket service
├── infra/
│   ├── rest/
│   │   ├── client.go    # REST client
│   │   ├── errors.go    # REST error handling
│   │   └── sign.go      # Request signing
│   └── ws/
│       └── client.go    # WebSocket client
├── internal/
│   ├── errors.go        # Internal error types
│   └── status.go        # Status mapping
├── routing/
│   ├── rest_router.go   # REST routing
│   ├── ws_router.go     # WebSocket routing
│   ├── parse_public.go  # Public data parsing
│   ├── parse_private.go # Private data parsing
│   ├── orderbook.go     # Order book handling
│   └── topics.go        # Topic mapping
└── your-exchange.go     # Package entry point
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

## Step 1.3: Create Basic Provider Structure

Create the main provider file `exchanges/your-exchange/exchange/provider.go`:

```go
package exchange

import (
    "context"
    
    "github.com/coachpo/meltica/config"
    "github.com/coachpo/meltica/core/streams"
    "github.com/coachpo/meltica/exchanges/your-exchange/infra/rest"
    "github.com/coachpo/meltica/exchanges/your-exchange/infra/ws"
    "github.com/coachpo/meltica/exchanges/your-exchange/routing"
)

type Provider struct {
    restClient   exchange.RESTClient
    wsClient     exchange.StreamClient
    restRouter   *routing.RESTRouter
    wsRouter     *routing.WSRouter
    symbolLoader *SymbolLoader
}

func NewProvider(cfg *config.Config) *Provider {
    settings := cfg.Exchange(config.ExchangeYourExchange)
    
    restClient := rest.NewClient(settings)
    wsClient := ws.NewClient(settings)
    
    return &Provider{
        restClient: restClient,
        wsClient:   wsClient,
        restRouter: routing.NewRESTRouter(restClient),
        wsRouter:   routing.NewWSRouter(wsClient),
        symbolLoader: NewSymbolLoader(restClient),
    }
}

// Implement market interfaces
func (p *Provider) Spot() *Spot {
    return &Spot{provider: p}
}

func (p *Provider) LinearFutures() *LinearFutures {
    return &LinearFutures{provider: p}
}

func (p *Provider) InverseFutures() *InverseFutures {
    return &InverseFutures{provider: p}
}
```

## Step 1.4: Create Package Entry Point

Create `exchanges/your-exchange/your-exchange.go`:

```go
package your_exchange

import (
    "github.com/coachpo/meltica/config"
    "github.com/coachpo/meltica/exchanges/your-exchange/exchange"
)

func NewProvider(cfg *config.Config) *exchange.Provider {
    return exchange.NewProvider(cfg)
}
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

- Use the Binance implementation as a reference for all patterns
- Follow the established directory structure
- Ensure all interfaces match the current `core/streams` contracts
- Use shared infrastructure from `exchanges/shared/` where possible
