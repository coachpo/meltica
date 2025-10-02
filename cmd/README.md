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


## Building

To build all tools:

```bash
# From the meltica root directory
make build-cmd
```

Or build individual tools:

```bash
go build -o market-stream ./cmd/market-stream
# Build tools as needed
```

## Development Workflow

1. **Adding a new exchange provider:**
   ```bash
   # Create provider directory and implement core interfaces
   mkdir providers/myexchange
   # Implement the required interfaces in the provider package
   ```

2. **Testing with live data:**
   ```bash
   market-stream -exchange binance -symbol BTC-USDT -channel ticker
   ```

## Integration

These tools are designed to work together:

- Use `market-stream` to test live data connectivity
- Implement providers following the core interface patterns

All tools use the meltica core framework and follow the same patterns for error handling, configuration, and output formatting.
