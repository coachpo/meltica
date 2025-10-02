# Implementing a Provider Adapter

This guide walks through the minimal files, API checkpoints, and validation steps required to add a new exchange adapter that conforms to the meltica protocol.

## Minimal File Set

- `providers/<name>/<name>.go`: adapter entry point implementing `core.Provider`
- `providers/<name>/sign.go`: request signing helpers
- `providers/<name>/errors.go`: HTTP/WS error normalization returning `*errs.E`
- `providers/<name>/status.go`: exchange status → `core.OrderStatus` mapping
- `providers/<name>/ws.go`: public websocket implementation
- `providers/<name>/ws_private.go`: private websocket implementation (if supported)
- `providers/<name>/conformance_test.go`: offline conformance harness hook
- `providers/<name>/golden_test.go`: schema/golden vector regression test
- `providers/<name>/README.md`: adapter-specific notes and credential requirements

## API Checklist

- `Provider.Capabilities()` returns a coherent bitset covering features exposed by the adapter
- `Provider.SupportedProtocolVersion()` returns `protocol.ProtocolVersion`
- Static analysis: `meltilint ./providers/...` passes (no capability ↔ API mismatches)
- Schema validation: `go run ./cmd/validate-schemas` passes
- All REST surfaces return canonical models (`core.Instrument`, `core.Ticker`, `core.Order`, etc.)
- Numeric quantities, prices, and balances use `*big.Rat`
- Canonical symbols use `BASE-QUOTE` format; use helpers in `core/symbols.go`
- Enum conversions (`OrderSide`, `OrderType`, `TimeInForce`, `OrderStatus`) handle every exchange value and return errors for unsupported cases
- Websocket decoders emit concrete `core.*Event` structs with canonical fields

## Error and Status Mapping

- Map HTTP/transport failures to `*errs.E` with provider name and canonical `errs.Code`
- Include raw exchange codes/messages in `RawCode`/`RawMsg`
- Return `errs.CodeInvalid` for unsupported inputs, `errs.CodeExchange` for transport-layer issues, and `errs.CodeAuth` for credential failures
- For unrecognized enum/status values, return an error instead of silently defaulting

## Websocket Decoding

- Public channels must emit `core/ws.TradeEvent`, `core/ws.TickerEvent`, or `core/ws.DepthEvent`
- Private channels emit `core/ws.OrderEvent` and `core/ws.BalanceEvent`
- Attach raw payloads to `core.Message.Raw` and populate `core.Message.Parsed` with the strongly typed event
- Apply canonical symbol conversion before emitting events

## Offline Conformance

- Add an entry to `internal/test/<provider>_conformance_test.go` invoking `conformance.RunAll`
- Supply fixture factories that instantiate the provider without hitting live endpoints (e.g., inject mocked transports)
- Capture fresh fixtures for golden vectors when schemas change; keep them under `protocol/vectors/`

## Optional Live Testing

- Respect `MELTICA_CONFORMANCE=1` to enable live conformance flows
- Document required environment variables in the adapter README
- Skip live tests gracefully when credentials are absent

## Promotion Checklist

1. `go test ./...` (unit + offline conformance)
2. `meltilint ./providers/<name>` (static analysis - capability ↔ API alignment)
3. `go run ./cmd/validate-schemas` (schema validation)
4. `go test ./internal/test -run <Provider>Conformance` (optional live)
5. `go run ./cmd/barista --name <provider>` (ensure scaffolding is up to date)
6. Update `README.md` adapter table with capability status
7. Submit PR with updated fixtures, schemas, and protocol version bump when required

Following this checklist ensures every new adapter enters the repository with predictable behavior and zero drift from the protocol contract.


