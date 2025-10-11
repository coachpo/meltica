# Migration: Adopt `/lib/ws-routing`

## Summary

This migration extracts the generic WebSocket routing infrastructure from `market_data/framework/router` into the reusable package `lib/ws-routing`. Domain adapters now consume a stable API (`Init`, `Start`, `Subscribe`, `Publish`, `UseMiddleware`) while market-data-specific processors remain unchanged.

## Why

- Eliminate duplicated router logic across exchanges
- Provide contract-tested lifecycle primitives that downstream teams can adopt confidently
- Reduce risk during adapter refactors by supplying deprecation shims and smoke tests

## Scope

| Area | Change |
| ---- | ------ |
| Framework | New package `github.com/coachpo/meltica/lib/ws-routing` with shared session + routing primitives |
| Market Data | `market_data/framework/api/wsrouting.Manager` bridges the engine into the framework. Legacy router internals have been removed in favour of a re-export shim. |
| Exchanges | Binance pipeline now lives under `exchanges/binance/wsrouting` and consumes the shared framework |
| Tooling | Contract, smoke, and architecture tests were updated to enforce parity |

## Migration Steps

1. Replace imports of `market_data/framework/router` with the deprecation shim (`market_data/framework/router/shim.go`) or directly with `lib/ws-routing`
2. Initialize sessions via `wsrouting.Init(ctx, options)` where `options` injects your dialer, parser, publisher, and backoff policy
3. Queue subscriptions using `wsrouting.Subscribe` before calling `Start`; the session replays them once the dialer succeeds
4. Register middleware for metrics/validation using `wsrouting.UseMiddleware`
5. Route raw payloads through `wsrouting.RouteRaw` or call `Publish` with normalized messages
6. Run the parity suite: `go test ./tests/integration/market_data -run TestWSRoutingParityFixtures` and the smoke test `-run TestQuickstartSmokeReconnectFlow`

## Validation

- `go test ./lib/ws-routing/...`
- `go test ./tests/contract/ws-routing`
- `go test ./tests/integration/market_data`
- `make lint-layers`

## Rollout Guidance

- The shim in `market_data/framework/router/shim.go` will remain for one release. Consumers should pin to `lib/ws-routing` before the shim is removed.
- Communicate the migration using the release checklist in `specs/009-goals-extract-the/spec.md` and link to the quickstart guide.
- Capture structured log snapshots from the smoke test to prove runtime parity before and after adoption.
