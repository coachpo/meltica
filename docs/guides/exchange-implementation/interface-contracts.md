# Interface Contracts

This document describes the core interfaces that exchange providers must implement in Meltica.

## Architecture Overview

Meltica uses a three-layer architecture:

- **Level 1**: Transport layer (REST/WebSocket clients)
- **Level 2**: Routing layer (request/response mapping)
- **Level 3**: Exchange layer (provider interface)

## Level 1: Transport Contracts

### REST Client Interface

```go
type RESTClient interface {
    Connection
    DoRequest(ctx context.Context, req RESTRequest) (*RESTResponse, error)
    HandleResponse(ctx context.Context, req RESTRequest, resp *RESTResponse, out any) error
    HandleError(ctx context.Context, req RESTRequest, err error) error
}
```

**Responsibilities:**
- Execute HTTP requests with proper headers and authentication
- Handle rate limiting and retry logic
- Parse responses and handle errors
- Manage connection lifecycle

### WebSocket Client Interface

```go
type StreamClient interface {
    Connection
    Subscribe(ctx context.Context, topics ...StreamTopic) (StreamSubscription, error)
    Unsubscribe(ctx context.Context, sub StreamSubscription, topics ...StreamTopic) error
    Publish(ctx context.Context, message StreamMessage) error
    HandleError(ctx context.Context, err error) error
}
```

**Responsibilities:**
- Establish and maintain WebSocket connections
- Handle subscription management
- Process incoming messages
- Send outbound messages
- Handle connection errors and reconnects

## Level 2: Routing Contracts

### REST Router

Exchange providers implement REST routing to map normalized requests to exchange-specific endpoints:

```go
// Example from Binance implementation
type RESTRouter struct {
    dispatcher *rest_dispatcher.Dispatcher
}

func (r *RESTRouter) MapRequest(req core.Request) (*exchange.RESTRequest, error) {
    // Implementation maps core requests to exchange-specific REST calls
}
```

**Responsibilities:**
- Map normalized request types to exchange endpoints
- Handle request signing and authentication
- Parse and normalize response data
- Handle exchange-specific error formats

### WebSocket Router

```go
type Router interface {
    SubscribePublic(ctx context.Context, topics ...string) (Subscription, error)
    SubscribePrivate(ctx context.Context) (Subscription, error)
    Close() error
}
```

**Responsibilities:**
- Map normalized topics to exchange-specific stream names
- Parse raw WebSocket messages into normalized events
- Handle subscription management
- Route messages to appropriate handlers

## Level 3: Exchange Contracts

### Provider Interface

The main exchange provider interface that exposes all market data and trading operations:

```go
// Provider struct from Binance implementation
type Provider struct {
    restClient   exchange.RESTClient
    wsClient     exchange.StreamClient
    restRouter   *routing.RESTRouter
    wsRouter     *routing.WSRouter
    symbolLoader *SymbolLoader
}
```

**Market Data Interfaces:**

```go
// Spot market interface
type Spot interface {
    Ticker(ctx context.Context, symbol string) (*core.Ticker, error)
    Instruments(ctx context.Context) ([]core.Instrument, error)
    // Additional spot methods...
}

// Linear futures interface  
type LinearFutures interface {
    Ticker(ctx context.Context, symbol string) (*core.Ticker, error)
    Positions(ctx context.Context, symbol string) ([]core.Position, error)
    // Additional futures methods...
}

// Inverse futures interface
type InverseFutures interface {
    Ticker(ctx context.Context, symbol string) (*core.Ticker, error)
    Positions(ctx context.Context) ([]core.Position, error)
    // Additional inverse futures methods...
}
```

## Core Data Types

### Market Data Events

```go
// Trade event
type TradeEvent struct {
    Symbol   string
    Price    *big.Rat
    Quantity *big.Rat
    Time     time.Time
}

// Ticker event  
type TickerEvent struct {
    Symbol string
    Bid    *big.Rat
    Ask    *big.Rat
    Time   time.Time
}

// Order book event
type BookEvent struct {
    Symbol string
    Bids   []core.BookDepthLevel
    Asks   []core.BookDepthLevel
    Time   time.Time
}
```

### Private Data Events

```go
// Order event
type OrderEvent struct {
    Symbol    string
    OrderID   string
    Status    core.OrderStatus
    FilledQty *big.Rat
    AvgPrice  *big.Rat
    Time      time.Time
}

// Balance event
type BalanceEvent struct {
    Balances []core.Balance
}
```

## Error Handling

All interfaces should use the standardized error types from `errs/` package:

```go
// Example error handling
if err != nil {
    return nil, errs.New(
        errs.CodeExchange,
        "failed to fetch ticker",
        errs.WithProvider("binance"),
        errs.WithRawCode(strconv.Itoa(statusCode)),
        errs.WithRawMsg(string(body)),
    )
}
```

## Testing Contracts

Exchange providers should implement comprehensive testing:

- Unit tests for parsing and normalization
- Integration tests for REST and WebSocket flows
- Error handling tests
- Symbol conversion tests

## Implementation Guidelines

1. **Follow the Binance Pattern**: Use the Binance implementation as a reference
2. **Reuse Shared Infrastructure**: Leverage components from `exchanges/shared/`
3. **Handle All Edge Cases**: Implement proper error handling for all scenarios
4. **Maintain Type Safety**: Use strongly typed interfaces throughout
5. **Document Quirks**: Note any exchange-specific behaviors or limitations

See the Binance implementation in `exchanges/binance/` for complete examples of all interface implementations.
