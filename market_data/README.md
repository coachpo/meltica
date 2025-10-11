# Market Data Package

## Overview

The `market_data` module now focuses on exchange-agnostic event definitions,
encoding helpers, and hot-path benchmarks. All routing lifecycle logic lives in
[`lib/ws-routing`](../lib/ws-routing), while executable processors have moved to
[`exchanges/processors`](../exchanges/processors).

- `events.go`: Canonical event shapes surfaced to downstream consumers.
- `encoder.go`: Utilities for serializing events into transport payloads.
- `testdata/`: Shared fixtures used by encoding and validation tests.

To build end-to-end pipelines, import `github.com/coachpo/meltica/lib/ws-routing`
for session orchestration and `github.com/coachpo/meltica/exchanges/processors`
for domain-specific decoding.

## Testing

```bash
go test ./market_data/...
go test ./tests/integration/market_data
```

## Key References

- [Quickstart: Adding a New Message Type](../specs/003-we-ve-built/quickstart.md)
- [Troubleshooting Guide](../docs/guides/router_troubleshooting.md)
- [Deployment Runbook](../docs/guides/multi_stream_router_runbook.md)
