# Project Overview

Meltica is a high-performance cryptocurrency exchange adapter framework that provides a unified interface for trading across multiple exchanges.

## Architecture

Meltica follows a layered architecture with clear separation of concerns:

### Level 1: Transport Layer
- **REST Client**: HTTP request/response handling with rate limiting and error handling
- **WebSocket Client**: Connection management, subscription handling, and raw message processing
- **Shared Infrastructure**: Rate limiting, numeric helpers, topic mapping utilities

### Level 2: Routing Layer
- **REST Router**: Maps normalized requests to exchange-specific endpoints and handles response parsing
- **WebSocket Router**: Routes raw WebSocket messages to normalized topics and handles subscription management
- **Data Parsing**: Converts exchange-specific data formats to normalized core types

### Level 3: Exchange Layer
- **Provider Interface**: Unified exchange abstraction that exposes market data and trading operations
- **Market Data**: Order books, tickers, trades, and other public market information
- **Private Data**: Orders, balances, positions, and other account-specific information
- **Core Types**: Standardized data structures that work across all exchanges

## Current Status

### Supported Exchanges
- **Binance**: Full implementation with spot, linear futures, and inverse futures support

### Future Exchange Development
Additional exchanges can be implemented following the established Binance patterns and architecture.

## Key Design Principles

1. **Unified Interface**: Single API for all exchanges
2. **Type Safety**: Strongly typed interfaces prevent runtime errors
3. **Performance**: Optimized for low-latency trading operations
4. **Extensibility**: Easy to add new exchanges following established patterns
5. **Reliability**: Comprehensive error handling and recovery mechanisms

## Getting Started

See [START-HERE.md](START-HERE.md) for a step-by-step guide to understanding and contributing to the project. 
