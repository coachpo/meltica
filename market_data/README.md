# Market Data Routing Framework (Shim)

## Overview

The market-data framework provides the building blocks for streaming exchange
integrations:

- `framework/router`: Deprecation shim that re-exports the shared
  `lib/ws-routing` package for one release window.
- `market_data/processors`: Typed processors that decode raw payloads into
  domain models.
- `framework/connection`: Session management, pooling, and handler dispatch.
- `lib/ws-routing`: New home for the reusable routing session, middleware, and
  publish helpers adopted by all adapters.

## Getting Started

1. Create a processor that satisfies the `Processor` interface and unit-test it
   in `market_data/processors`.
2. Register the processor and its detection rules via `exchanges/<venue>/wsrouting`
   helpers that consume `lib/ws-routing`.
3. Wire the routing table into your exchange adapter (see
   `exchanges/binance/wsrouting`).

Run the core test suites after making changes:

```bash
go test ./market_data/processors/...
go test ./tests/integration/market_data
```

## Key References

- [Quickstart: Adding a New Message Type](../specs/003-we-ve-built/quickstart.md)
- [Migration Guide](./MIGRATION.md)
- [Troubleshooting Guide](../docs/guides/router_troubleshooting.md)
- [Deployment Runbook](../docs/guides/multi_stream_router_runbook.md)
