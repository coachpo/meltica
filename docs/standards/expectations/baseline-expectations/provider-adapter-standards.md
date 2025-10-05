# Exchange Adapter Standards

This document defines what every exchange adapter must ship.

## STD-A1: Minimal file set
- Must have: `<name>.go`, `exchange/provider.go`, `exchange/spot.go`, `exchange/linear_futures.go`, `exchange/inverse_futures.go`, `exchange/symbol_loader.go`, `exchange/ws_service.go`, `infra/rest/client.go`, `infra/ws/client.go`, `routing/rest_router.go`, `routing/ws_router.go`, `README.md`.
- Validate:
  ```bash
  test -f exchanges/<name>/<name>.go &&   test -f exchanges/<name>/exchange/provider.go &&   test -f exchanges/<name>/infra/rest/client.go &&   test -f exchanges/<name>/infra/ws/client.go &&   test -f exchanges/<name>/README.md
  ```

## STD-A2: API checklist conformance
- Must have: Correct capability bits, `SupportedProtocolVersion()`, canonical models, *big.Rat numerics, canonical symbols, exhaustive enums, typed WS events.
- Validate:
  ```bash
  go build ./exchanges/<name> &&   go test ./exchanges/<name> -count=1
  ```

## STD-A3: Error & status mapping
- Must have: Normalize HTTP/WS errors to `*errs.E` with provider+code; capture raw code/message; unknown enums error.
- Must not have: Defaults that succeed silently.
- Validate: Unit tests for error mapping

## STD-A4: WS decoding rules
- Must have: Public → Trade/Ticker/DepthEvent, Private → Order/BalanceEvent, raw payload attached, symbols canonicalized.
- Must not have: Passing native symbols or `any`.
- Validate: Unit tests for WS decoding

## STD-A5: Integration testing
- Must have: Unit tests green, integration tests for end-to-end flows.
- Must not have: CI failures when no creds provided.
- Validate:
  ```bash
  go test ./exchanges/<name> -count=1
  ```
