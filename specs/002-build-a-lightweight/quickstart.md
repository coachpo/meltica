# Quickstart — Lightweight Real-Time WebSocket Framework

## 1. Install dependencies
```bash
go get github.com/goccy/go-json@latest
```
The repository already depends on `github.com/gorilla/websocket`; ensure `go mod tidy` keeps versions aligned.

## 2. Initialize the framework
```go
import (
    "context"
    "time"

    "github.com/coachpo/meltica/core/stream"
    "github.com/coachpo/meltica/market_data/framework"
)

func boot() (*framework.Engine, error) {
    cfg := framework.Config{
        Endpoint: "wss://example-feed.meltica.dev",
        DialTimeout: 5 * time.Second,
        MaxMessageBytes: 1 << 16,
        MetricsWindow: time.Minute,
    }
    return framework.NewEngine(cfg)
}
```

## 3. Register a custom handler
```go
type quoteHandler struct{}

func (quoteHandler) Handle(ctx context.Context, env *framework.MessageEnvelope) framework.HandlerOutcome {
    payload := env.Decoded.(*stream.QuoteUpdate)
    // domain-specific validation/processing
    return framework.HandlerOutcome{OutcomeType: framework.OutcomeAck}
}

engine.RegisterHandler(framework.HandlerRegistration{
    Name:       "quotes",
    Channels:   []string{"BTC-USDT"},
    Factory:    func() framework.Handler { return quoteHandler{} },
    Middleware: nil,
})
```

## 4. Start streaming
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

session, err := engine.Dial(ctx)
if err != nil {
    return err
}

go session.Run(ctx)
```

The session manages read/write pumps, pooling, heartbeats, and calls the registered handler for each decoded message. Invalid payloads emit `errs.CodeInvalid` events without closing the connection.

## 5. Observe metrics and tuning signals
```go
snapshot := engine.Metrics().ForSession(session.ID())
fmt.Printf("p95 latency: %s, alloc bytes: %d
", snapshot.P95Latency, snapshot.AllocBytes)
```

Use metrics to adjust pool sizes, throttling thresholds, and handler concurrency. Exposed counters feed existing Meltica observability tooling via the telemetry hooks described in the spec.

## 6. Handler onboarding checklist (finish in < 1 day)

- [ ] Confirm the target feed endpoint and symbols with the integrating team.
- [ ] Define or reuse the destination payload struct in `core/stream` so the handler receives typed data.
- [ ] Copy the `quoteHandler` template into your service and swap in the payload type created above.
- [ ] Implement any domain validation or enrichment inside the handler’s `Handle` method.
- [ ] Register the handler with `engine.RegisterHandler`, setting `Name`, `Channels`, and any required middleware.
- [ ] Add custom middleware hooks if auth, rate governance, or tracing is required for the new feed.
- [ ] Run `go test ./...` and `go test -bench . ./internal/benchmarks/market_data/framework` to confirm latency and allocations stay within targets.
- [ ] Deploy the handler behind a feature flag, monitor metrics from Section 5, and remove the flag once performance is stable.
