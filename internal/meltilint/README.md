# meltilint

Static analysis tool for enforcing meltica protocol compliance.

## Overview

`meltilint` performs compile-time checks to ensure the repository complies with the meltica protocol specification. It validates:

- **Core interface and model contracts (STD-01 → STD-13)**
- **Provider adapter guarantees (STD-24, STD-26, STD-38)**
- **Protocol documentation, schemas, and vectors (STD-16 → STD-18, STD-34/35)**
- **Conformance harness entrypoint (STD-19)**
- **Repository documentation expectations (START-HERE, how-to guides, validation rules)**
  - Validates `protocol-and-code-standards.md` enumerates STD-01 through STD-21
  - Validates `protocol-validation-rules.md` lists 42 rules with IDs, severities, intent, scope, and validation standards across 10 phases
  - Validates `provider-adapter-standards.md` enumerates STD-A1 through STD-A5
  - Validates `abstractions-guidelines.md` covers phases 1-10 with acceptance criteria

## Usage

```bash
# Build the tool
go build ./internal/meltilint/cmd/meltilint

# Run against the entire module (default)
./meltilint

# Run on specific provider
./meltilint ./providers/binance

# Run only core + protocol checks
./meltilint ./core ./protocol
```

## Checks

### Core Protocol Standards (STD-01 → STD-13)

- Provider, SpotAPI, FuturesAPI, WS, and Subscription interfaces are frozen to the golden signatures
- Canonical models (`Instrument`, `OrderRequest`, `Order`, `Position`, `Ticker`, `OrderBook`, `Trade`, `Kline`) exist and include doc comments
- Websocket event types (`TradeEvent`, `TickerEvent`, `DepthEvent`, `OrderEvent`, `BalanceEvent`) exist
- `ProviderCapabilities` remains a `uint64` bitset and decimal-bearing fields stay `*big.Rat`
- Exported APIs and struct fields in `core/` must not use `float32`/`float64`
- Enumerations (`Market`, `OrderSide`, `OrderType`, `TimeInForce`) include the full constant set

### Provider Adapter Enforcement (STD-24, STD-26, STD-38)

- Declared capability bits must match the concrete APIs exposed by each provider
- Providers must not expose floats in exported structs/functions
- `SupportedProtocolVersion()` must return `protocol.ProtocolVersion`

### Protocol Artifacts & Documentation (STD-16 → STD-18, STD-34/35)

- `protocol/README.md` must exist
- JSON Schemas are required for every core model and WS event (`<Type>.schema.json`) and must advertise Draft 2020-12
- Each schema needs at least one matching vector sample in `protocol/vectors/`
- `protocol/version.go` must define a SemVer-compatible `ProtocolVersion`

### Conformance Harness (STD-19)

- Ensures the `conformance.RunAll` entrypoint is exported with the canonical signature and supporting types

### Repository Documentation Set

- Verifies presence of START-HERE, baseline expectations, abstractions guidelines, how-to guides (6-step migration), and protocol validation rules

## Integration

The tool is integrated into CI via `.github/workflows/ci.yml` and runs automatically on all provider code changes.

## Exit Codes

- `0`: No issues found
- `1`: Issues detected or error occurred

## Output

Issues are reported to stderr in the format:
```
filename:line:column: [package] message
```
