# Core Package

The `core` package defines the canonical provider abstraction and shared domain models for the Meltica SDK. The types exported here form the stable protocol surface that all concrete exchange adapters must implement. The abstractions cover REST (spot + futures), WebSocket events, canonical topic helpers and utility primitives shared across adapters.

## Overview

This package provides a unified interface for cryptocurrency exchange operations, abstracting away exchange-specific differences while maintaining type safety and precision through the use of `*big.Rat` for all decimal values.

## Key Features

- **Type-safe decimal handling**: All prices, quantities, and amounts use `*big.Rat` to avoid floating-point precision issues
- **Unified API**: Single interface for spot trading, futures trading, and WebSocket subscriptions
- **Exchange agnostic**: Canonical symbol format and normalized data structures
- **Capability-based**: Providers declare their supported features via capability bitsets
- **Event-driven**: Structured WebSocket events for real-time market data

## Subpackages

- `ws`: WebSocket domain helpers, including canonical topics, channel mappers, and normalized event types

## Core Types

### Markets

```go
type Market string

const (
    MarketSpot              Market = "spot"
    MarketLinearFutures     Market = "linear_futures"
    MarketInverseFutures    Market = "inverse_futures"
)
```

### Order Management

```go
type OrderSide string
const (
    SideBuy  OrderSide = "buy"
    SideSell OrderSide = "sell"
)

type OrderType string
const (
    TypeMarket OrderType = "market"
    TypeLimit  OrderType = "limit"
)

type TimeInForce string
const (
    TIFGTC TimeInForce = "gtc"  // Good Till Cancel
    TIFFOK TimeInForce = "fok"  // Fill Or Kill
    TIFIC  TimeInForce = "ioc"  // Immediate Or Cancel
)

type OrderStatus string
const (
    OrderNew        OrderStatus = "new"
    OrderPartFilled OrderStatus = "part_filled"
    OrderFilled     OrderStatus = "filled"
    OrderCanceled   OrderStatus = "canceled"
    OrderRejected   OrderStatus = "rejected"
)
```

### Data Models

#### Instrument
Represents a trading instrument with precision information:
```go
type Instrument struct {
    Symbol     string  // Canonical format: BASE-QUOTE (e.g., "BTC-USDT")
    Base       string  // Base asset (e.g., "BTC")
    Quote      string  // Quote asset (e.g., "USDT")
    Market     Market  // Market type
    PriceScale int     // Decimal places for price
    QtyScale   int     // Decimal places for quantity
}
```

#### OrderRequest
Normalized order placement payload:
```go
type OrderRequest struct {
    Symbol      string
    Side        OrderSide
    Type        OrderType
    Quantity    *big.Rat  // Use big.Rat for precision
    Price       *big.Rat  // Use big.Rat for precision
    TimeInForce TimeInForce
    ClientID    string
    ReduceOnly  bool
}
```

#### Ticker
Top-of-book quote information:
```go
type Ticker struct {
    Symbol string
    Bid    *big.Rat
    Ask    *big.Rat
    Time   time.Time
}
```

#### OrderBook
Market depth snapshot:
```go
type OrderBook struct {
    Symbol string
    Bids   []DepthLevel
    Asks   []DepthLevel
    Time   time.Time
}

type DepthLevel struct {
    Price *big.Rat
    Qty   *big.Rat
}
```

### WebSocket Events (`core/exchange`)

The `core/exchange` package defines normalized WebSocket events for real-time data:

```go
// core/exchange/exchange.go
type TradeEvent struct {
    Symbol   string
    Price    *big.Rat
    Quantity *big.Rat
    Time     time.Time
}

type TickerEvent struct {
    Symbol string
    Bid    *big.Rat
    Ask    *big.Rat
    Time   time.Time
}

type DepthEvent struct {
    Symbol string
    Bids   []DepthLevel
    Asks   []DepthLevel
    Time   time.Time
}

type OrderEvent struct {
    Symbol    string
    OrderID   string
    Status    OrderStatus
    FilledQty *big.Rat
    AvgPrice  *big.Rat
    Time      time.Time
}

type BalanceEvent struct {
    Balances []Balance
}
```

### WebSocket Topics (`core/topics`)

`core/topics` provides canonical topic helpers used across the platform.

```go
topic := topics.Trade("BTC-USDT")
channel, symbol := topics.Parse("trade:BTC-USDT")
```

### Topic Mappers (`exchanges/infra/topics`)

`exchanges/infra/topics` exposes channel mapping helpers for exchange adapters.

```go
mapper := infratopics.NewMapper(infratopics.MappingConfig{ProtocolToExchange: map[string]string{topics.TopicTrade: "trade"}})
channel := mapper.ExchangeChannelID(topics.TopicTrade)
```

## Provider Interface

The main `Provider` interface defines the contract all exchange adapters must implement:

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

### API Interfaces

#### SpotAPI
Handles spot market operations:
```go
type SpotAPI interface {
    ServerTime(ctx context.Context) (time.Time, error)
    Instruments(ctx context.Context) ([]Instrument, error)
    Ticker(ctx context.Context, symbol string) (Ticker, error)
    Balances(ctx context.Context) ([]Balance, error)
    Trades(ctx context.Context, symbol string, since int64) ([]Trade, error)
    PlaceOrder(ctx context.Context, req OrderRequest) (Order, error)
    GetOrder(ctx context.Context, symbol, id, clientID string) (Order, error)
    CancelOrder(ctx context.Context, symbol, id, clientID string) error
}
```

#### FuturesAPI
Handles futures market operations:
```go
type FuturesAPI interface {
    Instruments(ctx context.Context) ([]Instrument, error)
    Ticker(ctx context.Context, symbol string) (Ticker, error)
    PlaceOrder(ctx context.Context, req OrderRequest) (Order, error)
    Positions(ctx context.Context, symbols ...string) ([]Position, error)
}
```

#### WebSocket
Manages real-time subscriptions:
```go
type WS interface {
    SubscribePublic(ctx context.Context, topics ...string) (Subscription, error)
    SubscribePrivate(ctx context.Context, topics ...string) (Subscription, error)
}
```

## Capabilities System

Providers declare their supported features using capability bitsets:

```go
const (
    CapabilitySpotPublicREST      Capability = 1 << iota
    CapabilitySpotTradingREST
    CapabilityLinearPublicREST
    CapabilityLinearTradingREST
    CapabilityInversePublicREST
    CapabilityInverseTradingREST
    CapabilityWebsocketPublic
    CapabilityWebsocketPrivate
)

type ProviderCapabilities uint64

// Usage
caps := Capabilities(
    CapabilitySpotPublicREST,
    CapabilitySpotTradingREST,
    CapabilityWebsocketPublic,
)

if caps.Has(CapabilityWebsocketPublic) {
    // Provider supports public WebSocket
}
```

## Symbol Normalization

The package provides utilities for canonical symbol handling:

```go
// Canonical format: BASE-QUOTE (e.g., "BTC-USDT")
symbol := CanonicalSymbol("BTC", "USDT")  // Returns "BTC-USDT"

// Convert to exchange-specific formats
binanceSymbol := CanonicalToBinance("BTC-USDT")  // Returns "BTCUSDT"
okxSymbol := CanonicalToOKX("BTC-USDT")          // Returns "BTC-USDT"
```

## Provider Registry

Providers can be registered and instantiated through the global registry:

```go
// Register a provider
Register("binance", func(cfg any) (Provider, error) {
    // Return configured provider
})

// Create a provider instance
provider, err := New("binance", config)
```

## Decimal Precision

All monetary values use `*big.Rat` to maintain precision:

```go
// Create precise decimal values
price := big.NewRat(50000, 1)      // 50000.00
qty := big.NewRat(1, 100)          // 0.01

// Use numeric.Format for JSON serialization
jsonData := numeric.Format(price, 2)  // Returns "50000.00"
```

## Usage Example

```go
// Create a provider
provider, err := core.New("binance", config)
if err != nil {
    log.Fatal(err)
}
defer provider.Close()

// Check capabilities
if provider.Capabilities().Has(core.CapabilitySpotPublicREST) {
    // Get spot API
    spot := provider.Spot(ctx)
    
    // Get ticker data
    ticker, err := spot.Ticker(ctx, "BTC-USDT")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("BTC-USDT: Bid=%s Ask=%s\n", 
        ticker.Bid.RatString(), 
        ticker.Ask.RatString())
}

// Subscribe to WebSocket events
ws := provider.WS()
sub, err := ws.SubscribePublic(ctx, "ticker:BTC-USDT")
if err != nil {
    log.Fatal(err)
}

for {
    select {
    case msg := <-sub.C():
        // Handle message
        fmt.Printf("Received: %s\n", msg.Topic)
    case err := <-sub.Err():
        log.Printf("WebSocket error: %v", err)
    }
}
```

## Standards Compliance

This package follows the Meltica protocol standards:

- **STD-09**: No floats anywhere - all decimals use `*big.Rat`
- **STD-10**: Decimal policy enforced with `*big.Rat`
- **STD-11**: Use `numeric.Format` for JSON marshaling
- **STD-12**: Canonical symbol format `BASE-QUOTE`
- **STD-13**: Enums are frozen and exhaustive
- **STD-15**: WebSocket decoders return typed events

## Command Line Tools

The project provides several command-line tools for development and testing:

### market-stream
Stream real-time market data from cryptocurrency exchanges:
```bash
# Stream BTC-USDT ticker data from Binance
go run ./cmd/market-stream -exchange binance -symbol BTC-USDT -channel ticker

# Stream ETH-USDT trade data from OKX
go run ./cmd/market-stream -exchange okx -symbol ETH-USDT -channel trades
```

### barista
Generate new exchange provider scaffolds:
```bash
# Generate a new provider scaffold
go run ./cmd/barista -name bybit

# Generate with custom output directory
go run ./cmd/barista -name ftx -out custom-providers/ftx
```

### validate-schemas
Validate JSON schemas against golden vectors:
```bash
go run ./cmd/validate-schemas
```

## Related Packages

- `providers/` - Concrete exchange adapter implementations
- `protocol/` - JSON schemas and golden vectors
- `conformance/` - Validation and testing harness
- `cmd/` - Command-line tools for development and testing
