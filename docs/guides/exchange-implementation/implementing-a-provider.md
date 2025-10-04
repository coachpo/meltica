# Implementing a Provider Adapter

This guide walks through the minimal files, API checkpoints, and validation steps required to add a new exchange adapter that conforms to the meltica protocol.

## Minimal File Set

- `exchanges/<name>/<name>.go`: adapter entry point implementing `core.Exchange`
- `exchanges/<name>/sign.go`: request signing helpers
- `exchanges/<name>/errors.go`: HTTP/WS error normalization returning `*errs.E`
- `exchanges/<name>/status.go`: exchange status → `core.OrderStatus` mapping
- `exchanges/<name>/ws.go`: public websocket implementation
- `exchanges/<name>/ws_private.go`: private websocket implementation (if supported)
- `exchanges/<name>/README.md`: adapter-specific notes and credential requirements

## API Checklist

- `Exchange.Capabilities()` returns a coherent bitset covering features exposed by the adapter
- `Exchange.SupportedProtocolVersion()` returns `core.ProtocolVersion`
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

- Public channels must emit `core/exchange.TradeEvent`, `core/exchange.TickerEvent`, or `core/exchange.DepthEvent`
- Private channels emit `core/exchange.OrderEvent` and `core/exchange.BalanceEvent`
- Attach raw payloads to `core.Message.Raw` and populate `core.Message.Parsed` with the strongly typed event
- Apply canonical symbol conversion before emitting events

## Integration Testing

- Write unit tests for all adapter components
- Add integration tests for end-to-end flows
- Respect environment variables for optional live testing
- Document required environment variables in the adapter README
- Skip live tests gracefully when credentials are absent

## Promotion Checklist

1. `go test ./...` (unit + integration tests)
2. Symbols canonical; decimals via `*big.Rat` + `core.FormatDecimal`
3. Update `README.md` exchange table with capability status
4. Submit PR with updated fixtures and protocol version bump when required

Following this checklist ensures every new adapter enters the repository with predictable behavior and zero drift from the protocol contract.


