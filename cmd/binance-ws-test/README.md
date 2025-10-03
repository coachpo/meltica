# Binance WebSocket Test Client

A simple test client that demonstrates real-time WebSocket subscription to Binance public market data streams.

## Features

- **Real-time Trade Data**: Subscribe to trade streams for BTC-USDT
- **Ticker Updates**: Get best bid/ask prices
- **Order Book Updates**: Receive depth stream updates (partial snapshots)
- **Graceful Shutdown**: Clean exit with Ctrl+C
- **Formatted Output**: Human-readable display of market data

## Usage

### Using Makefile
```bash
make binance-ws-test
```

### Manual Build and Run
```bash
# Build the client
go build -o bin/binance-ws-test ./cmd/binance-ws-test

# Run the client
./bin/binance-ws-test
```

## Output Format

The client displays messages in the following format:

### Trade Events
```
[15:04:05.123] TRADE BTC-USDT: Price=45000.12345678, Qty=0.00123456
```

### Ticker Events  
```
[15:04:05.124] TICKER BTC-USDT: Bid=44999.87654321, Ask=45000.12345678
```

### Order Book Events
```
[15:04:05.125] BOOK BTC-USDT: Bids=20, Asks=20
  Top Bid: 1.23456789 @ 44999.87654321
  Top Ask: 0.98765432 @ 45000.12345678
```

## Supported Symbols

Currently supports:
- `BTC-USDT`

## Supported Topics

- `trade`: Individual trade executions
- `ticker`: Best bid/ask prices  
- `book`: Order book depth updates

## Implementation Details

- Uses the official Binance WebSocket API
- Connects to `wss://stream.binance.com:9443/stream`
- No API key required for public streams
- Automatically handles reconnection on errors
- Graceful shutdown with signal handling

## Dependencies

- Go 1.16+
- Binance provider from the meltica library
- Gorilla WebSocket library
