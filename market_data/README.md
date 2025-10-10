# Market Data Routing Framework

## Overview

The market-data framework provides the building blocks for streaming exchange
integrations:

- `framework/router`: Message type detection, dispatch, metrics, and
  backpressure control.
- `market_data/processors`: Typed processors that decode raw payloads into
  domain models.
- `framework/connection`: Session management, pooling, and handler dispatch.

## Getting Started

1. Create a processor that satisfies the `Processor` interface and unit-test it
   in `market_data/processors`.
2. Register the processor and its detection rules via `router.InitializeRouter`
   or an exchange-specific helper.
3. Wire the routing table into your exchange adapter (see
   `exchanges/binance/plugin`).

Run the core test suites after making changes:

```bash
go test ./market_data/processors/...
go test ./market_data/framework/router/...
```

## Key References

- [Quickstart: Adding a New Message Type](../specs/003-we-ve-built/quickstart.md)
- [Migration Guide](./MIGRATION.md)
- [Troubleshooting Guide](../docs/guides/router_troubleshooting.md)
- [Deployment Runbook](../docs/guides/multi_stream_router_runbook.md)
