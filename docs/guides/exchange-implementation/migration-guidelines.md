# Migration Guidelines

This document provides guidelines for migrating existing exchange implementations or adapting new exchanges to the current Meltica architecture.

## Current Architecture

Meltica follows a three-layer architecture:

- **Level 1**: Transport layer (REST/WebSocket clients)
- **Level 2**: Routing layer (request/response mapping)
- **Level 3**: Exchange layer (provider interface)

## Migration Strategy

### For New Exchange Implementations

1. **Start with the Binance Template**: Use the Binance implementation as your reference
2. **Follow the Directory Structure**: Copy the folder structure from `exchanges/binance/`
3. **Implement Level 1 First**: Start with REST and WebSocket clients
4. **Add Level 2 Routing**: Implement request/response mapping
5. **Complete Level 3 Provider**: Build the main exchange interface

### For Existing Implementations

If you have an existing exchange implementation that needs to be updated:

1. **Review Current Interfaces**: Compare with the current `core/exchange` interfaces
2. **Update Transport Layer**: Ensure REST and WebSocket clients match current contracts
3. **Modernize Routing**: Use the shared routing patterns from `exchanges/shared/`
4. **Standardize Error Handling**: Use the `errs` package for consistent error reporting
5. **Update Testing**: Follow the current testing patterns

## Key Changes from Previous Versions

### Architecture Simplification

- **Removed Level 4**: The architecture now uses only Levels 1-3
- **Simplified Interfaces**: Core interfaces have been streamlined for better performance
- **Shared Infrastructure**: Increased reuse of shared components

### Interface Updates

- **REST Client**: Updated to use `RESTClient` interface from `core/exchange/transport_contracts.go`
- **WebSocket Client**: Updated to use `StreamClient` interface
- **Provider Pattern**: Standardized provider structure across all exchanges

### Error Handling

- **Standardized Errors**: All errors now use the `errs` package
- **Better Status Mapping**: Improved exchange-specific status code handling
- **Consistent Error Codes**: Unified error code system across all exchanges

## Implementation Checklist

### Level 1: Transport Layer

- [ ] Implement `RESTClient` interface
- [ ] Implement `StreamClient` interface  
- [ ] Add proper rate limiting
- [ ] Implement request signing (if required)
- [ ] Handle connection lifecycle

### Level 2: Routing Layer

- [ ] Implement REST request mapping
- [ ] Implement WebSocket topic mapping
- [ ] Add data parsing for exchange formats
- [ ] Handle response normalization
- [ ] Implement error mapping

### Level 3: Exchange Layer

- [ ] Create main provider struct
- [ ] Implement spot market interface
- [ ] Implement linear futures interface (if supported)
- [ ] Implement inverse futures interface (if supported)
- [ ] Add symbol loading and conversion
- [ ] Implement order book streaming

### Testing

- [ ] Unit tests for parsing and normalization
- [ ] Integration tests for REST flows
- [ ] Integration tests for WebSocket flows
- [ ] Error handling tests
- [ ] Symbol conversion tests

## Best Practices

### Code Organization

- Follow the Binance directory structure
- Use shared infrastructure from `exchanges/shared/`
- Keep exchange-specific logic in appropriate packages
- Maintain clear separation between layers

### Error Handling

- Always use `errs.New()` for creating errors
- Include provider name in error context
- Map exchange-specific error codes to standard codes
- Provide raw exchange messages for debugging

### Performance

- Use connection pooling for REST clients
- Implement efficient WebSocket message processing
- Cache symbol mappings where appropriate
- Use appropriate timeouts and retry logic

### Testing

- Use recorded fixtures for reliable testing
- Test both success and error scenarios
- Include integration tests with live endpoints (when possible)
- Test all market types (spot, linear futures, inverse futures)

## Example Migration

See the Binance implementation in `exchanges/binance/` for a complete example of the current architecture pattern.

## Getting Help

If you encounter issues during migration:

1. Review the Binance implementation as a reference
2. Check the interface contracts in `core/exchange/`
3. Look at shared infrastructure in `exchanges/shared/`
4. Review the testing patterns in existing implementations

## Next Steps

After completing your migration:

1. Run comprehensive tests: `make test`
2. Verify build: `make build`
3. Update documentation if needed
4. Submit your changes for review