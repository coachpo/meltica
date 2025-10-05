# Exchange Interface Contracts

The shared `core` interfaces formalise how Level 1 connection clients interact with routing layers. Every exchange adapter must provide concrete implementations for these interfaces so higher layers (routing, business logic, CLI tools) can operate without depending on exchange-specific packages.

## Core Exchange Interface

```go
// Exchange is the stable abstraction implemented by every concrete exchange adapter.
type Exchange interface {
    Name() string
    Capabilities() ExchangeCapabilities
    SupportedProtocolVersion() string
}
```

This minimal interface provides basic identification and capability discovery for exchange implementations.

## Market Data APIs

### Spot API

```go
// SpotAPI exposes canonicalized spot REST endpoints.
type SpotAPI interface {
    ServerTime(ctx context.Context) (time.Time, error)
    Instruments(ctx context.Context) ([]Instrument, error)
    Ticker(ctx context.Context, symbol string) (Ticker, error)
    Balances(ctx context.Context) ([]Balance, error)
    Trades(ctx context.Context, symbol string, since int64) ([]Trade, error)
    PlaceOrder(ctx context.Context, req OrderRequest) (Order, error)
    GetOrder(ctx context.Context, symbol, id, clientID string) (Order, error)
    CancelOrder(ctx context.Context, symbol, id, clientID string) error
    // SpotNativeSymbol converts canonical spot symbols to exchange-native format.
    SpotNativeSymbol(spotCanonical string) (string, error)
    // SpotCanonicalSymbol converts native spot symbols to canonical format.
    SpotCanonicalSymbol(spotNative string) (string, error)
}
```

### Futures API

```go
// FuturesAPI exposes canonicalized linear or inverse futures REST endpoints.
type FuturesAPI interface {
    Instruments(ctx context.Context) ([]Instrument, error)
    Ticker(ctx context.Context, symbol string) (Ticker, error)
    PlaceOrder(ctx context.Context, req OrderRequest) (Order, error)
    Positions(ctx context.Context, symbols ...string) ([]Position, error)
    // FutureNativeSymbol converts canonical futures symbols to exchange-native format.
    FutureNativeSymbol(futureCanonical string) (string, error)
    // FutureCanonicalSymbol converts native futures symbols to canonical format.
    FutureCanonicalSymbol(futureNative string) (string, error)
}
```

## WebSocket Interface

```go
// WS exposes public and private websocket subscriptions.
type WS interface {
    SubscribePublic(ctx context.Context, topics ...string) (Subscription, error)
    SubscribePrivate(ctx context.Context, topics ...string) (Subscription, error)
    // WSNativeSymbol converts canonical websocket symbols to exchange-native format.
    WSNativeSymbol(wsCanonical string) (string, error)
    // WSCanonicalSymbol converts native websocket symbols to canonical format.
    WSCanonicalSymbol(wsNative string) (string, error)
}

// Subscription delivers normalized websocket messages for a topic set.
type Subscription interface {
    C() <-chan Message
    Err() <-chan error
    Close() error
}

// Message is the canonical websocket envelope emitted by subscriptions.
type Message struct {
    Topic  string
    Raw    []byte
    At     time.Time
    Event  string
    Parsed any
}
```

## Capability System

The capability system uses a bitmask approach to describe exchange features:

```go
// Capability describes a discrete feature of an exchange.
type Capability uint64

const (
    CapabilitySpotPublicREST Capability = 1 << iota
    CapabilitySpotTradingREST
    CapabilityLinearPublicREST
    CapabilityLinearTradingREST
    CapabilityInversePublicREST
    CapabilityInverseTradingREST
    CapabilityWebsocketPublic
    CapabilityWebsocketPrivate
)

// ExchangeCapabilities is a bitset describing the features available from an exchange implementation.
type ExchangeCapabilities uint64

// Capabilities builds a ExchangeCapabilities bitset from the provided features.
func Capabilities(caps ...Capability) ExchangeCapabilities

// Has reports whether the capability bit is present.
func (pc ExchangeCapabilities) Has(cap Capability) bool
```

## Example Usage

```go
import exchangesrouting "github.com/coachpo/meltica/exchanges/shared/routing"

router := exchangesrouting.NewDefaultRESTRouter(restClient, nil)
msg := exchangesrouting.RESTMessage{
    API:    string(rest.SpotAPI),
    Method: http.MethodGet,
    Path:   "/api/v3/time",
}
if err := router.Dispatch(ctx, msg, &resp); err != nil {
    log.Fatalf("time endpoint failed: %v", err)
}
```

The router converts the `RESTMessage` into exchange-specific requests and handles response parsing. Consumers receive exchange-agnostic models without needing access to exchange internals.

## Testing Support

The `core/exchange/mocks` package provides lightweight doubles for testing:

- `mocks.RESTClient` exposes function hooks for each method, making it easy to validate routing behaviour.
- `mocks.StreamClient` and `mocks.StreamSubscription` help simulate websocket streams in unit tests.

The Binance module includes interface-focused tests (`exchanges/binance/routing/rest_router_test.go`, `exchanges/binance/routing/ws_router_test.go`) that demonstrate how these mocks validate interactions.
