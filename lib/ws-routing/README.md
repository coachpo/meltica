# ws-routing

`ws-routing` is a lightweight, domain-agnostic framework for building resilient WebSocket integrations. It wraps connection lifecycle management, retry/backoff policy, middleware chaining, and message publication behind a stable API so that exchanges can focus on domain processors.

## Features

- **Deterministic session lifecycle** via `Init`, `Start`, and `Publish` helpers
- **Subscription deduping** with automatic replay after reconnects
- **Middleware pipeline** for observability, validation, and mutation
- **Pluggable parser & publisher** interfaces to adapt any message schema
- **Contract & integration tests** to enforce API semantics across adapters

## Quickstart

```go
session, err := wsrouting.Init(ctx, wsrouting.Options{
    SessionID: "market-data",
    Dialer:    engineDialer,
    Parser:    parser,
    Publish:   publisher,
    Backoff: wsrouting.BackoffConfig{
        Initial: 500 * time.Millisecond,
        Max:     5 * time.Second,
    },
})
if err != nil {
    log.Fatal(err)
}

_ = wsrouting.UseMiddleware(session, metricsMiddleware)
_ = wsrouting.Subscribe(ctx, session, wsrouting.SubscriptionSpec{
    Exchange: "binance",
    Channel:  "trade",
    Symbols:  []string{"BTC-USDT"},
})

if err := wsrouting.Start(ctx, session); err != nil {
    log.Fatal(err)
}

if err := wsrouting.RouteRaw(ctx, session, rawBytes); err != nil {
    log.Printf("route failed: %v", err)
}
```

See [`examples/basic`](./examples/basic) for a runnable demonstration that dials a faux engine, registers middleware, and publishes messages.

## Testing

- `go test ./lib/ws-routing/...` – unit & contract coverage for error handling, middleware, and subscription replay
- `go test ./tests/contract/ws-routing -count=1` – OpenAPI contract enforcing request/response semantics
- `go test ./tests/integration/market_data` – parity smoke suite validating reconnect + logging behaviour

## Migration Notes

Existing projects can move from `market_data/framework/router` to this package by:

1. Replacing direct router imports with `github.com/coachpo/meltica/lib/ws-routing`
2. Wiring engine dialers and domain publishers into `wsrouting.Options`
3. Removing any remaining references to the deprecated shim and depending on `lib/ws-routing` directly

Refer to [`specs/009-goals-extract-the`](../../specs/009-goals-extract-the) for migration planning and architectural guidance.
