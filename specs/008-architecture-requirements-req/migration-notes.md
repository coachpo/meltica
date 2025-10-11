# Binance Migration Notes

## Overview

- **Scope**: Incremental adoption of the four-layer contracts (Connection → Routing → Business → Filter) inside the Binance exchange while keeping legacy entry points operational.
- **Drivers**: Provide a reference implementation for other venues, exercise static layer analyzer, and validate that adapters can coexist with legacy dispatchers during rollout.
- **Outcome**: All Binance surfaces now expose layer interfaces with compatibility shims and passing acceptance, migration, and full test suites.

## Key Steps & Decisions

1. **Layer Adapters**
   - Wrapped websocket and REST clients in `layerWSConnection` / `layerRESTConnection` with `Legacy*` accessors so routing could continue using existing transports.
   - Added routing adapters that surface `layers.WSRouting` / `layers.RESTRouting` while delegating to dispatcher providers for legacy code paths.
   - Exposed `layers.Business` via `NewBusinessAdapter` and `Wrapper.AsLayerInterface()` for bridge consumers.
2. **Filter Integration**
   - Coordinator now tracks `layers.Filter` instances, ensures `Close()` cascades to adapters and filters, and inserts `newLayerFilterStage` conversions between pipeline events and layer events.
   - Implemented conversion helpers and conservative defaults (`UnknownPayload`) to protect existing Payload contract.
3. **Testing**
   - Architecture tests extended to include Business/Filter contracts (<100 ms runtime guard).
   - Added isolated examples backed by reusable mocks to demonstrate layer-only testing.
   - Acceptance, migration, integration, and full test suite executed post-changes to ensure regressions were not introduced.
4. **Compatibility**
   - Routing constructors enforce layer interfaces but still accept legacy stream/REST providers via adapter shims.
   - Plugin wiring resolves `layers.RESTRouting` with graceful fallback to legacy dispatcher during partial migrations.

## Challenges & Resolutions

- **Payload Type Mismatch**: Layer events initially returned `any` payloads that did not satisfy pipeline `Payload`. Solution: wrap non-conforming values with `UnknownPayload` before re-emitting.
- **Filter Stage Cleanup**: Early version leaked filter resources; resolved by storing filters on the coordinator and closing them during teardown.
- **Mock Reuse**: Binance tests duplicated websocket stubs; replaced with shared architecture mocks to reduce drift and underline isolation guarantees.
- **Contract Coverage**: Performance assertions required careful context deadlines; standardized `contractDeadline` constant ensured stable CI behavior.

## Analyzer Status

- Latest `make lint-layers` run completed without reporting any layer boundary violations.

## Follow-Up Recommendations

- Promote the shared mocks and templates to other exchanges once Phase 6 completes.
- Consider adding golden-file samples for layer adapters to catch accidental contract regressions.
- Monitor analyzer output as more venues migrate; document any false positives for rule tuning.
