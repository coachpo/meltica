# Meltica SDK v2.0.0 Breaking Changes

## Overview

Meltica SDK v2.0.0 removes the final backward compatibility layer that proxied legacy parser-based market data integrations:

- Protocol version increases from `1.2.0` to `2.0.0` and all exchanges must advertise the new value via `SupportedProtocolVersion`.
- The deprecated `market_data/framework/parser` package has been deleted, along with helper APIs such as `parser.NewJSONPipeline`, `parser.NewValidationStage`, and `parser.WithInvalidThreshold`.
- Session decoding now flows directly through the connection runtime using pooled JSON decoders and the router/processor architecture introduced in v1.2.0.
- Exchange integrations must now use the four-layer architecture (`layers.Connection` → `layers.Routing` → `layers.Business` → `layers.Filter`) enforced by the static analyzer.
- WebSocket routing logic previously embedded in `market_data/framework/router` now lives under `lib/ws-routing`. Adapters should depend on the public API rather than internal packages. A temporary shim re-exports the new API during the transition.

## Removed Components

| Component | Notes |
|-----------|-------|
| `market_data/framework/parser` | Entire package removed (doc, JSON pipeline, validation stage).
| `parser.JSONPipeline` types | Inline decoding now handled by `market_data/framework/connection`.
| `parser.ValidationStage` | Validation and invalid-count tracking handled internally by the runtime.
| Legacy parser benchmarks/tests | `internal/benchmarks/.../engine_bench_test.go` and parser unit tests deleted.
| Documentation references | `market_data/MIGRATION.md` removed in favor of this guide.

## Migration Guide

### 1. Update Imports

- Migrate WebSocket adapters to `github.com/coachpo/meltica/lib/ws-routing`. The temporary shim (`market_data/framework/router/shim.go`) can be used while downstream packages adopt the new module.
- Remove `github.com/coachpo/meltica/market_data/framework/parser` from all modules.
- Add or retain imports for `market_data/framework/router`, `market_data/processors`, or exchange processors as appropriate.

### 2. Replace Parser Pipeline Construction

**Before (v1.x):**

```go
pipeline := parser.NewJSONPipeline()
validator := parser.NewValidationStage(parser.WithInvalidThreshold(5))
runtime := connection.NewSessionRuntime(conn, pipeline, validator)
```

**After (v2.0.0):**

```go
ctx := context.Background()
metrics := router.NewRoutingMetrics()
dispatcher := router.NewRouterDispatcher(ctx, metrics)

table := router.NewRoutingTable()
tradeDesc := &router.MessageTypeDescriptor{
    ID:          "trade",
    DisplayName: "Trade",
    DetectionRules: []router.DetectionRule{
        {Strategy: router.DetectionStrategyFieldBased, FieldPath: "type", ExpectedValue: "trade"},
    },
    ProcessorRef: "processors.trade",
}
if err := table.Register(tradeDesc, processors.NewTradeProcessor()); err != nil {
    log.Fatalf("register processor: %v", err)
}

if reg := table.Lookup("trade"); reg != nil {
    dispatcher.Bind("trade", reg)
}

session, err := engine.Dial(connection.WithDialOptions(ctx, connection.DialOptions{
    InvalidThreshold: 5,
}))
if err != nil {
    log.Fatal(err)
}
defer session.Close()
```

Session runtime now decodes payloads internally; configure routing by wiring `RoutingTable` registrations to a `RouterDispatcher` and dialing the engine without constructing parser stages.

### 3. Validation & Error Handling

- Invalid message tracking is handled by the runtime—configure thresholds via `connection.DialOptions.InvalidThreshold` when dialing a session.
- Use `Session.SetInvalidCount` values surfaced by the runtime (or telemetry events) to monitor invalid payloads.
- Remove custom panic recovery built around parser validation, as the runtime emits telemetry events instead.

### 4. Protocol Negotiation

- Ensure every exchange implementation returns `"2.0.0"` from `SupportedProtocolVersion`.
- Update integration tests that asserted protocol mismatches to reference the legacy version string (`"1.2.0"`) when expecting a failure.

### 5. Documentation & Scripts

- Replace references to parser helpers inside internal documentation or onboarding scripts with router/processor equivalents.
- Regenerate any examples that previously showed parser setup to instead demonstrate router dispatch and processor registration.

### 6. Four-Layer Architecture Adoption

- Update exchange adapters to return the new layer interfaces (`core/layers` package). Adapters that previously exposed concrete Binance types must now satisfy `layers.Connection`, `layers.Routing`, and `layers.Business` contracts.
- Run `make lint-layers` to ensure layer boundary rules are honored; violations will fail CI once coverage gates are enabled.
- For incremental migrations, use the provided legacy adapter helpers (e.g., `LegacyRESTDispatcher`) to bridge old entry points until the exchange is fully migrated.

## Version Timeline

| Version | Status | Notes |
|---------|--------|-------|
| v1.0.0 – v1.1.x | Historical | Initial parser-only architecture.
| v1.2.0 | Deprecated | Introduced router/processor flow and marked parser package as legacy.
| v2.0.0 | Current | Removes parser package entirely; router/processor is the sole integration path.

## Support & Further Resources

- Deployment runbook: `docs/guides/multi_stream_router_runbook.md`
- Troubleshooting tips: `docs/guides/router_troubleshooting.md`
- Processor examples: `market_data/processors`
- Community support: open an issue in the Meltica repository or contact Meltica support via the usual channels.

Please upgrade all integrations before deploying Meltica SDK v2.0.0. The legacy parser architecture is no longer available and new features will target the streamlined processor/router stack exclusively.
