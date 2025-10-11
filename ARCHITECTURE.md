# Meltica Architecture Overview

Meltica implements a layered trading framework that separates concerns across four explicit layers.

## Layers

| Layer | Location | Responsibilities |
|-------|----------|------------------|
| L1 – Connection | [`core/layers/connection.go`](core/layers/connection.go) | Manage WebSocket/REST transports, deadlines, backoff, and legacy client shims. |
| L2 – Routing | [`core/layers/routing.go`](core/layers/routing.go) | Translate exchange protocols into normalized events, handle subscriptions, and surface API helpers. |
| L3 – Business | [`core/layers/business.go`](core/layers/business.go) | Coordinate exchange-specific workflows, maintain `layers.BusinessState`, and delegate to routing adapters. |
| L4 – Filter | [`core/layers/filter.go`](core/layers/filter.go) | Transform normalized events into client-facing payloads within the pipeline coordinator. |

## Supporting Components

- **Static Analysis**: [`internal/linter`](internal/linter) enforces package boundaries via `make lint-layers` and runs in CI.
- **Tests**: [`tests/architecture`](tests/architecture) hosts contract tests, reusable mocks, and isolated usage examples.
- **Templates**: [`internal/templates/exchange`](internal/templates/exchange) plus [`scripts/new-exchange.sh`](scripts/new-exchange.sh) scaffold new exchanges following the four-layer pattern.

## Further Reading

- [`specs/008-architecture-requirements-req/quickstart.md`](specs/008-architecture-requirements-req/quickstart.md)
- [`docs/architecture-layers.md`](docs/architecture-layers.md)
- [`BREAKING_CHANGES_v2.md`](BREAKING_CHANGES_v2.md)
