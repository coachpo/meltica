# Meltica

A high-performance cryptocurrency exchange adapter framework written in Go.

> **Breaking Change (v2.0.0):** The legacy `market_data/framework/parser` package has been removed. Review the upgrade guide in [`BREAKING_CHANGES_v2.md`](./BREAKING_CHANGES_v2.md) before updating existing integrations.

## Supported Providers

| Exchange | Spot | Linear Futures | Inverse Futures | WebSocket Public | WebSocket Private |
|----------|------|----------------|-----------------|------------------|-------------------|
| Binance  | ✅   | ✅             | ✅              | ✅               | ✅                |

**Legend:** ✅ = Fully Supported

## Architecture

The system follows a formal four-layer architecture:

1. **Layer 1 – Connection** (`core/layers/connection.go`): WebSocket and REST transport adapters with legacy provider shims.
2. **Layer 2 – Routing** (`core/layers/routing.go`): Normalizes transport payloads, manages subscription lifecycles, and translates API requests.
3. **Layer 3 – Business** (`core/layers/business.go`): Coordinates domain workflows, maintains business state, and bridges routing outputs to filters.
4. **Layer 4 – Filter** (`core/layers/filter.go`): Final pipeline stage that transforms events for downstream clients and handles cleanup.

Supporting assets:
- `internal/linter/`: Static analyzer enforcing layer boundaries (also runs via `make lint-layers`).
- `tests/architecture/`: Contract tests, reusable mocks, and isolation examples.
- `internal/templates/exchange/` + `scripts/new-exchange.sh`: Exchange skeleton generator aligned with the four layers.

See [`specs/008-architecture-requirements-req/quickstart.md`](specs/008-architecture-requirements-req/quickstart.md) for end-to-end guidance.

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
