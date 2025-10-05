# meltica

Provider-agnostic cryptocurrency exchange SDK with pluggable adapters for unified trading across multiple exchanges.

## About the Name

**meltica** comes from "melting coffee" ☕ - representing the idea of blending different exchange APIs into a single, unified, smooth experience. Just like melting coffee dissolves and combines flavors, meltica dissolves the differences between exchanges, giving you one consistent interface to trade across multiple platforms.

This coffee inspiration reflects our approach to blending different exchange APIs seamlessly.

## Features

- **Unified API**: Single interface for trading across multiple exchanges
- **Multiple Markets**: Support for spot, linear futures, and inverse futures trading
- **Real-time Data**: WebSocket subscriptions for market data and private account updates
- **Extensible**: Easy to add new exchange providers

## Supported Providers

|| Provider | Spot REST | Spot Trading | Linear Futures | Inverse Futures | WebSocket Public | WebSocket Private |
||----------|-----------|--------------|----------------|-----------------|------------------|-------------------|
|| **Binance** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
|| **OKX** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
|| **Coinbase** | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ |
|| **Kraken** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

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
    
    "github.com/coachpo/meltica/exchanges/binance"
)

func main() {
    ctx := context.Background()
    
    // Create exchange provider (no credentials needed for public data)
    client, _ := binance.New("", "")
    
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
import "github.com/coachpo/meltica/core/topics"

// Public market data
sub, _ := client.WS().SubscribePublic(ctx, topics.Trade("BTC-USDT"), topics.Ticker("BTC-USDT"))
for msg := range sub.C() {
    fmt.Printf("Received: %s - %s\n", msg.Topic, string(msg.Raw))
}

// Private account updates
privateSub, _ := client.WS().SubscribePrivate(ctx, topics.UserOrder("BTC-USDT"), topics.UserBalance())
for msg := range privateSub.C() {
    // Handle order updates, position changes, etc.
}
```

### Trading with Authentication

```go
// Using environment variables
client, _ := binance.New(os.Getenv("BINANCE_KEY"), os.Getenv("BINANCE_SECRET"))

// Place an order
order, _ := spot.PlaceOrder(ctx, core.OrderRequest{
    Symbol:   "BTC-USDT",
    Side:     core.SideBuy,
    Type:     core.TypeLimit,
    Quantity: big.NewRat(1, 10), // 0.1 BTC
    Price:    big.NewRat(50000, 1), // $50,000
})
```

## Development

### Prerequisites
- Go 1.25+
- Network access for public endpoints
- API credentials for private endpoints (optional)

### Command Line Tools

The project provides several command-line tools in the `cmd/` directory for development and testing:

```bash
# Navigate to project directory
cd meltica

# Build all tools
make build

# Or build individual tools
go build -o bin/ ./cmd/binance-ws-test
```

### Testing

```bash
# Run all tests
go test ./... -count=1

# Run with race detection
go test ./... -race -count=1

# Run specific exchange tests
go test ./exchanges/binance -v
```

### Building & Tools

All compiled binaries are output to the `bin/` directory in the project root.

```bash
# Build all binaries
make build

# Development commands
make test      # Run tests with race detection
make lint      # Run linter (if golangci-lint is installed)
make tidy      # Clean up Go modules

# Clean build artifacts
make clean
```

**Available Tools:**

- `binance-ws-test`: WebSocket connection testing for Binance
- `binance-ws-validation`: WebSocket message validation
- `binance-orderbook-validation`: Order book validation
- `binance-snapshot-test`: Snapshot testing

**Directory Structure:**
```
meltica/
├── exchanges/           # Exchange-specific implementations
│   ├── binance/        # Binance exchange adapter
│   └── shared/         # Shared infrastructure
├── core/               # Universal abstractions and interfaces
├── cmd/                # Command-line tools
└── bin/                # Build output directory
```

The `bin/` directory is automatically ignored by git.

## Architecture

### Core Components

- **`core/`**: Universal abstractions and interfaces
- **`exchanges/`**: Exchange-specific implementations
- **`errs/`**: Error handling and normalization
- **`config/`**: Configuration management

### Exchange Interface

All exchanges implement the unified `Exchange` interface:

```go
type Exchange interface {
    Name() string
    Capabilities() ExchangeCapabilities
    SupportedProtocolVersion() string
}
```

## Contributing

1. Start with `docs/START-HERE.md` for the ordered guide
2. Implement new exchanges using `docs/guides/exchange-implementation/implementing-a-provider.md`
3. Run all tests and validation tools before submitting

## License

This project is licensed under the MIT [LICENSE](LICENSE).
