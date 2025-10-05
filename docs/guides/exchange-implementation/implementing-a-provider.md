# Implementing an Exchange Adapter

This guide walks through the minimal files, API checkpoints, and validation steps required to add a new exchange adapter that conforms to the meltica protocol.

## Directory Structure

New exchange adapters should follow this structure:

```
exchanges/<name>/
├── <name>.go                    # adapter entry point implementing core.Exchange
├── exchange/
│   ├── provider.go              # main provider implementation
│   ├── spot.go                  # spot market implementation
│   ├── linear_futures.go        # linear futures implementation
│   ├── inverse_futures.go       # inverse futures implementation
│   ├── symbol_loader.go         # symbol loading and conversion
│   ├── symbol_registry.go       # symbol registry management
│   └── ws_service.go            # WebSocket service implementation
├── infra/
│   ├── rest/
│   │   ├── client.go            # REST client implementation
│   │   ├── errors.go            # HTTP error normalization
│   │   └── sign.go              # request signing helpers
│   └── ws/
│       └── client.go            # WebSocket client implementation
├── internal/
│   ├── errors.go                # exchange-specific error handling
│   └── status.go                # status mapping
├── routing/
│   ├── rest_router.go           # REST request routing
│   ├── ws_router.go             # WebSocket message routing
│   ├── topics.go                # topic mapping configuration
│   ├── parse_public.go          # public WebSocket message parsing
│   ├── parse_private.go         # private WebSocket message parsing
│   └── orderbook.go             # order book management
└── README.md                    # adapter-specific notes and credential requirements
```

## API Checklist

- `Exchange.Capabilities()` returns a coherent bitset covering features exposed by the adapter
- `Exchange.SupportedProtocolVersion()` returns `core.ProtocolVersion`
- All REST surfaces return canonical models (`core.Instrument`, `core.Ticker`, `core.Order`, etc.)
- Numeric quantities, prices, and balances use `*big.Rat`
- Canonical symbols use `BASE-QUOTE` format
- Enum conversions (`OrderSide`, `OrderType`, `TimeInForce`, `OrderStatus`) handle every exchange value and return errors for unsupported cases
- WebSocket decoders emit concrete `core.*Event` structs with canonical fields

## Error and Status Mapping

- Map HTTP/transport failures to `*errs.E` with provider name and canonical `errs.Code`
- Include raw exchange codes/messages in `RawCode`/`RawMsg`
- Return `errs.CodeInvalid` for unsupported inputs, `errs.CodeExchange` for transport-layer issues, and `errs.CodeAuth` for credential failures
- For unrecognized enum/status values, return an error instead of silently defaulting

## WebSocket Implementation

- Use the shared topic system from `core/topics` for canonical topic names
- Implement topic mapping in routing layer using `exchanges/shared/infra/topics/mapper.go`
- Public channels must emit appropriate event types for trades, tickers, and order books
- Private channels emit order and balance events
- Apply canonical symbol conversion before emitting events

## Integration Testing

- Write unit tests for all adapter components
- Add integration tests for end-to-end flows
- Respect environment variables for optional live testing
- Document required environment variables in the adapter README
- Skip live tests gracefully when credentials are absent

## Promotion Checklist

1. `go test ./...` (unit + integration tests)
2. Symbols canonical; decimals via `*big.Rat`
3. Update `README.md` exchange table with capability status
4. Submit PR with updated fixtures and protocol version bump when required

Following this checklist ensures every new adapter enters the repository with predictable behavior and zero drift from the protocol contract.
