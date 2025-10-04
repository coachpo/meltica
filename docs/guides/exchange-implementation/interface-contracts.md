# Exchange Interface Contracts

The shared `core/exchange` interfaces formalise how Level 1 connection clients interact with routing layers. Every exchange adapter must provide concrete implementations for these interfaces so higher layers (routing, business logic, CLI tools) can operate without depending on exchange-specific packages.

## Connection lifecycle

```go
// Connection is embedded by both RESTClient and StreamClient
// Connect should verify connectivity and honour context cancellation.
type Connection interface {
    Connect(ctx context.Context) error
    Close() error
}
```

Implementations may lazily establish transport state; `Connect` exists so callers can perform readiness checks or warm connection pools when required.

## RESTClient contract

```go
type RESTClient interface {
    Connection
    DoRequest(ctx context.Context, req RESTRequest) (*RESTResponse, error)
    HandleResponse(ctx context.Context, req RESTRequest, resp *RESTResponse, out any) error
    HandleError(ctx context.Context, req RESTRequest, err error) error
}
```

- **DoRequest** executes a single HTTP call and returns the raw status, headers, and payload bytes.
- **HandleResponse** performs protocol-specific decoding (JSON unmarshalling, envelope flattening) into `out` when present.
- **HandleError** maps transport failures into canonical `error` values (`*errs.E`) so upper layers can reason about rate limits, auth failures, or invalid input.

`RESTRequest` carries the normalised method, path, query/body parameters, and the exchange specific surface identifier (`API`).

### Example usage

```go
router := routing.NewRESTRouter(restClient)
msg := routing.RESTMessage{
    API:    rest.SpotAPI,
    Method: http.MethodGet,
    Path:   "/api/v3/time",
}
if err := router.Dispatch(ctx, msg, &resp); err != nil {
    log.Fatalf("time endpoint failed: %v", err)
}
```

The router converts the `RESTMessage` into a `coreexchange.RESTRequest`, calls `DoRequest`, then delegates to `HandleResponse` or `HandleError`. Consumers (such as `cmd/main.go`) receive exchange-agnostic models without needing access to Binance internals.

## StreamClient contract

```go
type StreamClient interface {
    Connection
    Subscribe(ctx context.Context, topics ...StreamTopic) (StreamSubscription, error)
    Unsubscribe(ctx context.Context, sub StreamSubscription, topics ...StreamTopic) error
    Publish(ctx context.Context, message StreamMessage) error
    HandleError(ctx context.Context, err error) error
}
```

- **Subscribe** establishes one or more websocket streams and returns a `StreamSubscription` exposing raw frames and error channels.
- **Unsubscribe** detaches topics or closes the connection; the default behaviour may simply call `sub.Close()`.
- **Publish** is optional—exchanges without outbound commands can return a `NotSupported` error.
- **HandleError** converts network/protocol failures into canonical errors for routing layers.

`StreamTopic` identifies the scope (public or private) and the exchange-native stream name. `StreamSubscription` mirrors the Level 1 contract used by routers and higher layers.

### Example usage

```go
router := routing.NewWSRouter(streamClient, deps)
sub, err := router.SubscribePublic(ctx, corews.BookTopic("BTC-USDT"))
if err != nil {
    log.Fatalf("stream subscribe failed: %v", err)
}
go func() {
    defer sub.Close()
    for msg := range sub.C() {
        switch evt := msg.Parsed.(type) {
        case *coreexchange.BookEvent:
            render(evt)
        }
    }
}()
```

The router converts protocol topics to Binance stream names (`btcusdt@depth@100ms`), invokes `StreamClient.Subscribe`, and translates raw frames into `coreexchange.RoutedMessage` instances for business handlers.

## Testing support

The `core/exchange/mocks` package provides lightweight doubles for both interfaces:

- `mocks.RESTClient` exposes function hooks for each method, making it easy to validate routing behaviour.
- `mocks.StreamClient` and `mocks.StreamSubscription` help simulate websocket streams in unit tests without touching concrete Binance infrastructure.

The Binance module includes interface-focused tests (`exchanges/binance/routing/rest_router_test.go`, `exchanges/binance/routing/ws_router_test.go`) that demonstrate how these mocks validate interactions. Future exchanges (e.g., OKX) can reuse the same mocks to assert compliance with the shared contracts before wiring in their own transport layers.
