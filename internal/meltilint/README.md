# meltilint

Static analysis tool for enforcing meltica protocol compliance.

## Overview

`meltilint` performs compile-time checks to ensure provider adapters conform to the meltica protocol specification. It validates:

- **Capability ↔ API Alignment**: Declared capabilities match implemented APIs
- **Protocol Version Compliance**: `SupportedProtocolVersion()` returns canonical version
- **Interface Implementation**: Required provider methods are present

## Usage

```bash
# Build the tool
go build ./internal/meltilint/cmd/meltilint

# Run on all providers
./meltilint ./providers/...

# Run on specific provider
./meltilint ./providers/binance

# Default behavior (providers/...)
./meltilint
```

## Checks

### Capability Alignment

Ensures declared capabilities in `Provider.Capabilities()` match the actual API implementations:

- `core.CapabilitySpotPublicREST` ↔ `Provider.Spot()` not returning unsupported type
- `core.CapabilityLinearPublicREST` ↔ `Provider.LinearFutures()` not returning unsupported type  
- `core.CapabilityInversePublicREST` ↔ `Provider.InverseFutures()` not returning unsupported type
- `core.CapabilityWebsocketPublic` ↔ `Provider.WS()` not returning unsupported type

### Protocol Version

Validates that `Provider.SupportedProtocolVersion()` returns `protocol.ProtocolVersion`.

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
