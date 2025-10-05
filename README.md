# Meltica

A high-performance cryptocurrency exchange adapter framework written in Go.

## Supported Providers

| Exchange | Spot | Linear Futures | Inverse Futures | WebSocket Public | WebSocket Private |
|----------|------|----------------|-----------------|------------------|-------------------|
| Binance  | ✅   | ✅             | ✅              | ✅               | ✅                |
| OKX      | 🔄   | 🔄             | 🔄              | 🔄               | 🔄                |
| Coinbase | 🔄   | 🔄             | 🔄              | 🔄               | 🔄                |
| Kraken   | 🔄   | 🔄             | 🔄              | 🔄               | 🔄                |

**Legend:** ✅ = Fully Supported, 🔄 = Planned/In Development

## Architecture

Meltica follows a layered architecture with clear separation of concerns:

### Level 1: Transport Layer
- **REST Client**: HTTP request/response handling with rate limiting
- **WebSocket Client**: Connection management and raw message handling
- **Shared Infrastructure**: Rate limiting, numeric helpers, topic mapping

### Level 2: Routing Layer
- **REST Router**: Maps normalized requests to exchange-specific endpoints
- **WebSocket Router**: Routes raw WebSocket messages to normalized topics
- **Data Parsing**: Converts exchange-specific formats to normalized types

### Level 3: Exchange Layer
- **Provider Interface**: Unified exchange abstraction
- **Market Data**: Order books, tickers, trades
- **Private Data**: Orders, balances, positions
- **Core Types**: Standardized data structures across all exchanges

## Quick Start

```go
import "github.com/coachpo/meltica/exchanges/binance"

// Create a Binance provider
provider := binance.NewProvider()

// Use the provider for market data or trading operations
```

## Features

- **Unified API**: Single interface for multiple exchanges
- **High Performance**: Optimized for low-latency trading
- **Type Safety**: Strongly typed Go interfaces
- **Extensible**: Easy to add new exchanges
- **Production Ready**: Comprehensive error handling and logging

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
