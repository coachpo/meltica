# Command Line Tools

This package contains command-line utilities for working with the meltica cryptocurrency exchange integration framework.

## Tools

### market-stream

Stream real-time market data from cryptocurrency exchanges via WebSocket connections.

**Usage:**
```bash
market-stream [flags]
```

**Flags:**
- `-exchange` - Exchange to connect to (binance, okx, coinbase, kraken) [default: binance]
- `-symbol` - Trading symbol in canonical BASE-QUOTE form [default: BTC-USDT]
- `-channel` - Market data stream to subscribe to (ticker, trades, depth) [default: ticker]

**Examples:**
```bash
# Stream BTC-USDT ticker data from Binance
market-stream -exchange binance -symbol BTC-USDT -channel ticker

# Stream ETH-USDT trade data from OKX
market-stream -exchange okx -symbol ETH-USDT -channel trades

# Stream order book depth from Coinbase
market-stream -exchange coinbase -symbol BTC-USD -channel depth
```

**Features:**
- Supports multiple exchanges with unified interface
- Real-time streaming with automatic reconnection
- Formatted output with timestamps
- Graceful shutdown with Ctrl+C

### validate-schemas

Validate all JSON schemas against their golden test vectors to ensure protocol consistency.

**Usage:**
```bash
validate-schemas
```

**What it does:**
- Validates protocol schemas in `protocol/schemas/`
- Tests against golden vectors in `protocol/vectors/`
- Reports validation success or detailed error messages
- Used in CI/CD pipelines for protocol verification

**Example output:**
```
All schemas validated successfully
```

### barista

Brew fresh scaffold code for new cryptocurrency exchange providers. Like a skilled barista crafting the perfect cup, this tool expertly prepares all the essential ingredients for your exchange integration.

**Usage:**
```bash
barista -name <provider> [-out <directory>]
```

**Flags:**
- `-name` - Provider short name (e.g., bybit, ftx, huobi) [required]
- `-out` - Optional output directory [default: providers/<name>]

**Examples:**
```bash
# Brew a provider scaffold in default location
barista -name bybit

# Brew a provider scaffold in custom directory
barista -name ftx -out custom-providers/ftx
```

**Generated Files:**
- `{provider}.go` - Main provider implementation with all core interfaces
- `sign.go` - Request signing utilities (placeholder)
- `errors.go` - Error mapping functions (placeholder)
- `status.go` - Order status translation (placeholder)
- `ws.go` - Public WebSocket implementation (placeholder)
- `ws_private.go` - Private WebSocket implementation (placeholder)
- `conformance_test.go` - Conformance test skeleton
- `golden_test.go` - Golden test vectors (placeholder)
- `README.md` - Documentation template

## Building

To build all tools:

```bash
# From the meltica root directory
make build-cmd
```

Or build individual tools:

```bash
go build -o market-stream ./cmd/market-stream
go build -o validate-schemas ./cmd/validate-schemas
go build -o barista ./cmd/barista
```

## Development Workflow

1. **Adding a new exchange provider:**
   ```bash
   barista -name myexchange
   cd providers/myexchange
   # Implement the TODO sections in generated files
   ```

2. **Validating protocol changes:**
   ```bash
   validate-schemas
   ```

3. **Testing with live data:**
   ```bash
   market-stream -exchange binance -symbol BTC-USDT -channel ticker
   ```

## Integration

These tools are designed to work together:

- Use `barista` to brew new provider scaffolds
- Use `validate-schemas` to ensure protocol compliance
- Use `market-stream` to test live data connectivity

All tools use the meltica core framework and follow the same patterns for error handling, configuration, and output formatting.
