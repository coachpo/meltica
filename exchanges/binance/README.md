# Binance Adapter (Four-Layer Architecture)

## Layer Overview

- **L1 – Infrastructure**: Connection and REST tooling (`infra/rest`, `infra/ws`).
- **L2 – Routing**: Framework integration (`routing`, processor hub, message types).
- **L3 – Business**: Bridge and telemetry packages expose REST dispatchers and metrics aggregation.
- **L4 – Filter**: Policy filters that forward typed events to downstream pipelines.

## Wiring the Adapter

1. Register processors using `routing.RegisterProcessors`.
2. Build the framework router via `routing.NewWSRouter` and pass it into
   `binance.NewFilterAdapter` or the plugin helper.
3. For REST bridging, construct `bridge.NewRouterBridge` and inject it into the
   session manager.

## Verification

```bash
go test ./exchanges/binance/... -run Acceptance
```

Key suites: `acceptance_test.go`, `session_test.go`, `smoke_test.go`, and
`performance_test.go` ensure compatibility with the framework router.
