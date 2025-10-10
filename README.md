# Meltica

A high-performance cryptocurrency exchange adapter framework written in Go.

> **Breaking Change (v2.0.0):** The legacy `market_data/framework/parser` package has been removed. Review the upgrade guide in [`BREAKING_CHANGES_v2.md`](./BREAKING_CHANGES_v2.md) before updating existing integrations.

## Supported Providers

| Exchange | Spot | Linear Futures | Inverse Futures | WebSocket Public | WebSocket Private |
|----------|------|----------------|-----------------|------------------|-------------------|
| Binance  | ✅   | ✅             | ✅              | ✅               | ✅                |

**Legend:** ✅ = Fully Supported

## Architecture

Meltica follows a layered architecture with clear separation of concerns:

### Level 1: Transport Layer
- **REST Client**: HTTP request/response handling with rate limiting
- **WebSocket Client**: Connection management and raw message handling
- **Shared Infrastructure**: Rate limiting, numeric helpers, bootstrap wiring

### Level 2: Routing Layer
- **REST Router**: Maps normalized requests to exchange-specific endpoints
- **WebSocket Router**: Routes raw WebSocket messages to normalized topics
- **Data Parsing**: Converts exchange-specific formats to normalized types

### Level 3: Exchange Layer
- **Provider Interface**: Unified exchange abstraction
- **Market Data**: Order books, tickers, trades
- **Private Data**: Orders, balances, positions
- **Core Types**: Standardized data structures across all exchanges

### Level 4: Filter & Client Facade
- **Interaction Facade**: High-level `SubscribePublic`, `SubscribePrivate`, and `FetchREST` helpers
- **Client Events**: Typed payloads (`BookPayload`, `TradePayload`, `RestResponsePayload`, etc.) wrapped in `ClientEvent` with channel metadata
- **Pipeline Stages**: Normalization, throttling, aggregation, analytics, reliability, and observer hooks operating on typed events

## Quick Start

```go
import (
    "github.com/coachpo/meltica/core/registry"
    binanceplugin "github.com/coachpo/meltica/exchanges/binance/plugin"
)

// Create a Binance provider
exchange, err := registry.Resolve(binanceplugin.Name)
if err != nil {
    log.Fatal(err)
}
defer exchange.Close()

// Use the exchange for market data or trading operations
```

## Features

- **Unified API**: Single interface for multiple exchanges (currently Binance)
- **High Performance**: Optimized for low-latency trading
- **Type Safety**: Strongly typed Go interfaces
- **Extensible**: Easy to add new exchanges following the Binance pattern
- **Production Ready**: Comprehensive error handling and logging
- **Typed Event Stream**: Clients consume strongly typed `ClientEvent` payloads instead of raw envelopes

## Development

### Building

```bash
make build
```

### Testing

```bash
make test
```

### Cross-compilation

```bash
make build-linux-arm64
```

## Documentation

- [Getting Started](docs/getting-started/START-HERE.md)
- [Project Overview](docs/getting-started/PROJECT_OVERVIEW.md)
- [Adding New Exchanges](docs/guides/exchange-implementation/onboarding-new-exchange.md)
- [Architecture Standards](docs/standards/expectations/abstractions-guidelines.md)

## License

MIT License - see [LICENSE](LICENSE) for details.
