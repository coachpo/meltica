# Protocol & Code Standards

This document defines non-negotiable standards for the protocol and core code.

## STD-01: Freeze Exchange interface surface
- Must have: `core.Exchange` interface exactly matching the current implementation; includes capability reporting and market access.
- Must not have: Extra/removed methods or signature drift.
- Validate:
  ```bash
  go build ./core && go test ./core -count=1
  ```

## STD-02: Freeze Spot Market Interface
- Must have: Provider spot market methods exactly as implemented.
- Must not have: Added/removed methods; type mismatches.
- Validate: `go build ./core`

## STD-03: Freeze Futures Market Interface
- Must have: Provider futures market methods exactly as implemented.
- Must not have: Signature drift.
- Validate: `go build ./core`

## STD-04: Freeze WebSocket Interface
- Must have: Provider WebSocket interface match current signatures.
- Must not have: Loose `interface{}` surfaces.
- Validate: `go build ./core`

## STD-05: Core models exist and are documented
- Must have: Types: `Instrument`, `OrderRequest`, `Order`, `Position`, `Ticker`, `OrderBook`, `Trade`, `Kline` each with doc comment.
- Must not have: Missing types or missing doc comments.
- Validate: `go build ./core`

## STD-06: WebSocket event types present
- Must have: `TradeEvent`, `TickerEvent`, `DepthEvent`, `OrderEvent`, `BalanceEvent`.
- Must not have: Ad-hoc maps for events.
- Validate: `go build ./core`

## STD-07: Canonical error type
- Must have: `errs/`: error type `E`, exported codes enum, public APIs return `*errs.E`.
- Must not have: Plain `error` returns without canonical type.
- Validate: `go build ./errs`

## STD-08: Capabilities bitset
- Must have: `ExchangeCapabilities` as `uint64`, exported constants, `Exchange.Capabilities()` returns it.
- Must not have: Boolean flags scattered.
- Validate: `go build ./core`

## STD-09: No floats anywhere
- Must have: Zero `float32/float64` in exported fields, params/returns, JSON.
- Must not have: Floats in models or APIs.
- Validate: Code review and testing

## STD-10: Decimal policy = *big.Rat
- Must have: All numeric fields as `*big.Rat`.
- Must not have: int, float, string for decimals.
- Validate: Code review and testing

## STD-11: Canonical symbol format
- Must have: `BASE-QUOTE` uppercase, helpers for normalization.
- Must not have: Exchange-native symbols in public API.
- Validate: Code review and testing

## STD-12: Enums are frozen and exhaustive
- Must have: `OrderSide`, `OrderType`, `TimeInForce`, `OrderStatus`, `Market` constant sets complete.
- Must not have: Defaults or missing values.
- Validate: Code review and testing

## STD-13: Mapping functions exhaustive
- Must have: Switch covers all enums, default returns error.
- Must not have: Silent fallthrough.
- Validate: Code review and testing

## STD-14: WS decoders return typed events
- Must have: Concrete `core.*Event` returns.
- Must not have: `interface{}`.
- Validate: Code review and testing
