# meltica

Provider-agnostic cryptocurrency exchange SDK with pluggable adapters for unified trading across multiple exchanges.

## About the Name

**meltica** comes from "melting coffee" ☕ - representing the idea of blending different exchange APIs into a single, unified, smooth experience. Just like melting coffee dissolves and combines flavors, meltica dissolves the differences between exchanges, giving you one consistent interface to trade across multiple platforms.

This coffee inspiration extends to our tools:
- **`barista`** - Our code generator that brews fresh exchange provider scaffolds

## Features

- **Unified API**: Single interface for trading across multiple exchanges
- **Multiple Markets**: Support for spot, linear futures, and inverse futures trading
- **Real-time Data**: WebSocket subscriptions for market data and private account updates
- **Protocol Compliance**: JSON Schema validation and conformance testing
- **Extensible**: Easy to add new exchange providers

## Supported Providers

| Provider | Spot REST | Spot Trading | Linear Futures | Inverse Futures | WebSocket Public | WebSocket Private |
|----------|-----------|--------------|----------------|-----------------|------------------|-------------------|
| **Binance** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **OKX** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Coinbase** | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ |
| **Kraken** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

### Capabilities by Provider

- **Binance**: Full feature support across all markets (spot, linear futures, inverse futures) with comprehensive REST and WebSocket APIs
- **OKX**: Complete feature support across all markets (spot, linear futures, inverse futures) with advanced order types and position management
- **Coinbase**: Spot trading with robust REST API and WebSocket streams
- **Kraken**: Complete feature support across all markets (spot, linear futures, inverse futures) with extensive market coverage and WebSocket support

## Installation

```bash
go get github.com/coachpo/meltica
```

## Quick Start

### Basic Usage (Public Data)

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/coachpo/meltica/providers/binance"
)

func main() {
    ctx := context.Background()
    
    // Create provider (no credentials needed for public data)
    client, _ := binance.New("", "")
    defer client.Close()
    
    // Get server time
    spot := client.Spot(ctx)
    serverTime, _ := spot.ServerTime(ctx)
    fmt.Printf("Server time: %v\n", serverTime)
    
    // List available instruments
    instruments, _ := spot.Instruments(ctx)
    fmt.Printf("Available instruments: %d\n", len(instruments))
    
    // Get ticker data
    ticker, _ := spot.Ticker(ctx, "BTC-USDT")
    fmt.Printf("BTC-USDT: Bid=%s Ask=%s\n", ticker.Bid, ticker.Ask)
}
```

### Futures Trading

```go
// Linear futures (USDT/USDC margined)
futures := client.LinearFutures(ctx)
positions, _ := futures.Positions(ctx, "BTC-USDT")
fmt.Printf("BTC-USDT position: %s\n", positions[0].Quantity)

// Inverse futures (coin margined)
inverseFutures := client.InverseFutures(ctx)
inversePositions, _ := inverseFutures.Positions(ctx)
```

### WebSocket Subscriptions

```go
// Public market data
sub, _ := client.WS().SubscribePublic(ctx, "trades:BTC-USDT", "ticker:BTC-USDT")
for msg := range sub.C() {
    fmt.Printf("Received: %s - %s\n", msg.Topic, string(msg.Raw))
}

// Private account updates
privateSub, _ := client.WS().SubscribePrivate(ctx, "orders", "positions")
for msg := range privateSub.C() {
    // Handle order updates, position changes, etc.
}
```

### Trading with Authentication

```go
// Using environment variables
client, _ := binance.New(os.Getenv("BINANCE_KEY"), os.Getenv("BINANCE_SECRET"))

// Using the registry system
client, _ := core.New("binance", struct{ APIKey, Secret string }{
    APIKey: os.Getenv("BINANCE_KEY"),
    Secret: os.Getenv("BINANCE_SECRET"),
})

// Place an order
order, _ := spot.PlaceOrder(ctx, core.OrderRequest{
    Symbol:   "BTC-USDT",
    Side:     core.SideBuy,
    Type:     core.TypeLimit,
    Quantity: big.NewRat(1, 10), // 0.1 BTC
    Price:    big.NewRat(50000, 1), // $50,000
})
```

## Multi-Provider Example

```go
import (
    "github.com/coachpo/meltica/core"
    "github.com/coachpo/meltica/providers/binance"
    "github.com/coachpo/meltica/providers/okx"
)

func comparePrices() {
    // Initialize multiple providers
    binanceClient, _ := binance.New("", "")
    okxClient, _ := okx.New("", "", "")
    
    // Get tickers from both exchanges
    binanceTicker, _ := binanceClient.Spot(ctx).Ticker(ctx, "BTC-USDT")
    okxTicker, _ := okxClient.Spot(ctx).Ticker(ctx, "BTC-USDT")
    
    fmt.Printf("Binance BTC-USDT: %s/%s\n", binanceTicker.Bid, binanceTicker.Ask)
    fmt.Printf("OKX BTC-USDT: %s/%s\n", okxTicker.Bid, okxTicker.Ask)
}
```

## Development

### Prerequisites
- Go 1.25+
- Network access for public endpoints
- API credentials for private endpoints (optional)

### Running Examples

```bash
# Navigate to project directory
cd meltica

# Run spot trading example
go run ./examples/spot

# Run futures example
go run ./examples/futures

# Run WebSocket example
go run ./examples/ws_public

# Run OKX-specific example
go run ./examples/okx/spot
```

### Market Data Stream CLI

```bash
# Stream Binance trades for BTC-USDT
go run ./cmd/market-stream --exchange binance --symbol BTC-USDT --channel trades

# Subscribe to OKX top-of-book quotes
go run ./cmd/market-stream --exchange okx --symbol ETH-USDT --channel ticker
```

Available channels depend on the provider. Common options are `ticker`, `trades`, and `depth`.

### Testing

```bash
# Run all tests
go test ./... -count=1

# Run with race detection
go test ./... -race -count=1

# Run conformance tests (requires API keys)
export MELTICA_CONFORMANCE=1
export BINANCE_KEY=your_key
export BINANCE_SECRET=your_secret
go test ./internal/test -count=1

# Run specific provider tests
go test ./providers/binance -v
```

### Protocol Validation

```bash
# Validate provider conformance
go build ./internal/meltilint/cmd/meltilint
./meltilint ./providers/...

# Validate JSON schemas and vectors
go test ./conformance
```

### Building & Tools

All compiled binaries are output to the `out/` directory in the project root.

```bash
# Build all binaries
make build

# Build specific tools
make build-meltilint        # Protocol linter
make build-validate-schemas # Schema validator
make build-barista          # Provider scaffold generator

# Development commands
make test      # Run tests with race detection
make lint      # Run linter (if golangci-lint is installed)
make tidy      # Clean up Go modules

# Clean build artifacts
rm -rf out/
```

**Available Tools:**

- **`out/meltilint`** - Static analysis tool for provider protocol compliance
- **`out/validate-schemas`** - JSON schema validation against golden vectors
- **`out/barista`** - Code generator to brew fresh exchange provider scaffolds

**Directory Structure:**
```
meltica/
├── out/                 # Build output directory
│   ├── meltilint
│   ├── validate-schemas
│   └── barista
└── ...
```

The `out/` directory is automatically ignored by git.

## Architecture

### Core Components

- **`core/`**: Universal abstractions and interfaces
- **`providers/`**: Exchange-specific implementations
- **`protocol/`**: JSON Schema definitions and validation
- **`transport/`**: HTTP client with retry logic and rate limiting
- **`conformance/`**: Protocol compliance testing

### Provider Interface

All providers implement the unified `Provider` interface:

```go
type Provider interface {
    Name() string
    Capabilities() ProviderCapabilities
    SupportedProtocolVersion() string
    Spot(ctx context.Context) SpotAPI
    LinearFutures(ctx context.Context) FuturesAPI
    InverseFutures(ctx context.Context) FuturesAPI
    WS() WS
    Close() error
}
```

### Protocol Compliance

The project enforces protocol compliance through:

- **JSON Schema validation** for WebSocket events and data structures
- **Static analysis** via `meltilint` to ensure capability alignment
- **Conformance testing** to validate provider implementations
- **Golden file testing** for consistent API responses

## Contributing

1. Start with `docs/START-HERE.md` for the ordered guide
2. Implement new providers using `docs/how-to/implementing-a-provider.md`
3. Ensure protocol compliance with `docs/validation/protocol-validation-rules.md`
4. Run all tests and validation tools before submitting

## License

This project is licensed under the MIT [LICENSE](LICENSE).
