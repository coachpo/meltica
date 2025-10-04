# Provider Adapter Standards

This document defines what every provider adapter must ship.

## STD-A1: Minimal file set
- Must have: `<name>.go`, `sign.go`, `errors.go`, `status.go`, `ws.go`, `ws_private.go` (if supported), `conformance_test.go`, `golden_test.go`, `README.md`.
- Validate:
  ```bash
  test -f providers/<name>/<name>.go &&   test -f providers/<name>/errors.go &&   test -f providers/<name>/status.go &&   test -f providers/<name>/ws.go &&   test -f providers/<name>/conformance_test.go &&   test -f providers/<name>/golden_test.go &&   test -f providers/<name>/README.md
  ```

## STD-A2: API checklist conformance
- Must have: Correct capability bits, `SupportedProtocolVersion()`, validated schemas, canonical models, *big.Rat numerics, canonical symbols, exhaustive enums, typed WS events.
- Validate:
  ```bash
  ./meltilint ./providers/<name> &&   go run ./cmd/validate-schemas &&   go test ./conformance -run TestOffline
  ```

## STD-A3: Error & status mapping
- Must have: Normalize HTTP/WS errors to `*errs.E` with provider+code; capture raw code/message; unknown enums error.
- Must not have: Defaults that succeed silently.
- Validate: Golden tests for error mapping

## STD-A4: WS decoding rules
- Must have: Public → Trade/Ticker/DepthEvent, Private → Order/BalanceEvent, raw payload attached, symbols canonicalized.
- Must not have: Passing native symbols or `any`.
- Validate: Offline WS tests + `./meltilint`

## STD-A5: Offline & live conformance
- Must have: Offline suite green, live tests gated by `MELTICA_CONFORMANCE=1` with documented env vars.
- Must not have: CI failures when no creds provided.
- Validate:
  ```bash
  go test ./internal/test -run <Provider>Conformance
  ```
