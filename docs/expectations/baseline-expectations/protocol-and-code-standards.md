# Protocol & Code Standards

This document defines non-negotiable standards for the protocol and core code.

## STD-01: Freeze Provider interface surface
- Must have: `core.Provider` interface exactly matching the golden spec; includes capability reporting and WS access.
- Must not have: Extra/removed methods or signature drift.
- Validate:
  ```bash
  go build ./internal/meltilint/cmd/meltilint && ./meltilint ./core
  ```

## STD-02: Freeze SpotAPI
- Must have: `core.SpotAPI` methods exactly as golden.
- Must not have: Added/removed methods; type mismatches.
- Validate: `./meltilint ./core`

## STD-03: Freeze FuturesAPI
- Must have: `core.FuturesAPI` methods exactly as golden.
- Must not have: Signature drift.
- Validate: `./meltilint ./core`

## STD-04: Freeze WS + Subscription
- Must have: `core.WS` and `core.Subscription` match golden signatures.
- Must not have: Loose `interface{}` surfaces.
- Validate: `./meltilint ./core`

## STD-05: Core models exist and are documented
- Must have: Types: `Instrument`, `OrderRequest`, `Order`, `Position`, `Ticker`, `OrderBook`, `Trade`, `Kline` each with doc comment.
- Must not have: Missing types or missing doc comments.
- Validate: `./meltilint ./core`

## STD-06: WebSocket event types present
- Must have: `TradeEvent`, `TickerEvent`, `DepthEvent`, `OrderEvent`, `BalanceEvent`.
- Must not have: Ad-hoc maps for events.
- Validate: `./meltilint ./core`

## STD-07: Canonical error type
- Must have: `core/errs`: error type `E`, exported codes enum, public APIs return `*errs.E`.
- Must not have: Plain `error` returns without canonical type.
- Validate: `./meltilint ./core ./providers/...`

## STD-08: Capabilities bitset
- Must have: `ProviderCapabilities` as `uint64`, exported constants, `Provider.Capabilities()` returns it.
- Must not have: Boolean flags scattered.
- Validate: `./meltilint ./core`

## STD-09: No floats anywhere
- Must have: Zero `float32/float64` in exported fields, params/returns, JSON.
- Must not have: Floats in models or APIs.
- Validate: `./meltilint ./core ./providers/...`

## STD-10: Decimal policy = *big.Rat
- Must have: All numeric fields as `*big.Rat`.
- Must not have: int, float, string for decimals.
- Validate: `./meltilint`

## STD-11: Use core.FormatDecimal when marshaling
- Must have: Every JSON marshal of `*big.Rat` calls `core.FormatDecimal`.
- Must not have: Custom `fmt.Sprintf`.
- Validate: `./meltilint`

## STD-12: Canonical symbol format
- Must have: `BASE-QUOTE` uppercase, helpers for normalization.
- Must not have: Exchange-native symbols in public API.
- Validate: `./meltilint ./core`

## STD-13: Enums are frozen and exhaustive
- Must have: `OrderSide`, `OrderType`, `TimeInForce`, `OrderStatus`, `Market` constant sets complete.
- Must not have: Defaults or missing values.
- Validate: `./meltilint ./core`

## STD-14: Mapping functions exhaustive
- Must have: Switch covers all enums, default returns error.
- Must not have: Silent fallthrough.
- Validate: `./meltilint ./providers/...`

## STD-15: WS decoders return typed events
- Must have: Concrete `core.*Event` returns.
- Must not have: `interface{}`.
- Validate: `./meltilint ./providers/...`

## STD-16: Protocol docs present
- Must have: `protocol/README.md` covering symbols, decimals, errors, WS.
- Must not have: Missing docs.
- Validate: CI presence check

## STD-17: JSON Schemas complete
- Must have: Draft 2020-12 schemas for all models/events.
- Must not have: Partial schema coverage.
- Validate: `go run ./cmd/validate-schemas`

## STD-18: Golden vectors validate
- Must have: Vectors per schema, pass validation.
- Must not have: Missing vectors.
- Validate: Schema validation step

## STD-19: Conformance harness entrypoint
- Must have: `conformance.RunAll(t, factory, opts)` exported.
- Must not have: Ad-hoc harnesses.
- Validate: `go test ./conformance -run TestOffline`

## STD-20: Offline suites must pass
- Must have: JSON mapping, enums, decimals, errors, WS decoding.
- Must not have: Live dependencies in offline tests.
- Validate: `go test ./conformance/...`

## STD-21: Capability-gated tests
- Must have: Tests skip based on `Capabilities()`.
- Must not have: Hard-coded assumptions.
- Validate: Inspect + `./meltilint`

