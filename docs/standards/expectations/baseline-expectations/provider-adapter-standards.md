# Provider Adapter Standards

This document defines the standards and expectations for implementing exchange providers in Meltica.

## Architecture Compliance

All exchange providers must follow the three-layer architecture:

### Level 1: Transport Layer
- Implement `RESTClient` interface from `core/exchange/transport_contracts.go`
- Implement `StreamClient` interface for WebSocket connections
- Use shared infrastructure from `exchanges/shared/`
- Handle rate limiting, connection pooling, and error recovery

### Level 2: Routing Layer
- Implement REST request/response mapping
- Implement WebSocket topic mapping and message routing
- Use shared routing patterns from `exchanges/shared/routing/`
- Handle data parsing and normalization

### Level 3: Exchange Layer
- Implement the provider interface following the Binance pattern
- Support spot, linear futures, and inverse futures markets
- Provide symbol loading and conversion
- Implement order book streaming

## Testing Standards

### Unit Testing Requirements

All exchange providers must include comprehensive unit tests:

#### REST Router Tests
- Test request mapping for all endpoints
- Test response parsing and normalization
- Test error handling and status code mapping
- Test request signing and authentication

Example test structure:
```go
func TestRESTRouter_Ticker(t *testing.T) {
    router := NewRESTRouter()
    
    req, err := router.MapRequest(core.Request{
        Type: core.RequestTypeTicker,
        Symbol: "BTC-USDT",
    })
    
    // Assert request mapping
    // Assert response parsing
    // Assert error handling
}
```

#### WebSocket Router Tests
- Test topic mapping and subscription management
- Test message parsing and event normalization
- Test connection lifecycle and error handling
- Test reconnection logic

#### Data Parsing Tests
- Test symbol conversion and normalization
- Test numeric precision handling with `*big.Rat`
- Test enum value mapping (order types, sides, statuses)
- Test timestamp parsing and timezone handling

### Integration Testing

- Include integration tests for REST endpoints
- Include integration tests for WebSocket streams
- Use recorded fixtures for reliable testing
- Test with both success and error scenarios
- Respect environment variables for optional live testing

### Test Coverage

- Minimum 70% code coverage for adapter code
- 80%+ coverage for core routing logic
- All error paths must be tested
- All data parsing functions must be tested

## Error Handling Standards

### Error Creation

Use the standardized error types from the `errs` package:

```go
if err != nil {
    return nil, errs.New(
        errs.CodeExchange,
        "failed to fetch ticker",
        errs.WithProvider("your-exchange"),
        errs.WithRawCode(strconv.Itoa(statusCode)),
        errs.WithRawMsg(string(body)),
    )
}
```

### Error Mapping

- Map HTTP status codes to appropriate error codes
- Include exchange-specific error codes in `RawCode`
- Provide meaningful error messages
- Handle network errors and timeouts appropriately

## Performance Standards

### REST Client Performance

- Implement connection pooling
- Use appropriate timeouts and retry logic
- Handle rate limiting gracefully
- Minimize memory allocations

### WebSocket Client Performance

- Efficient message processing
- Proper connection lifecycle management
- Handle backpressure appropriately
- Minimize goroutine usage

### Memory Management

- Avoid memory leaks in long-running connections
- Properly close connections and release resources
- Use appropriate buffer sizes
- Handle large datasets efficiently

## Code Quality Standards

### Code Organization

- Follow the established directory structure
- Use clear and descriptive naming
- Maintain separation of concerns between layers
- Avoid circular dependencies

### Documentation

- Document exchange-specific quirks and limitations
- Provide examples for common use cases
- Document required environment variables
- Include API rate limit information

### Security

- Never log sensitive information (API keys, secrets)
- Use secure request signing
- Validate all inputs
- Handle authentication errors gracefully

## Release Standards

### Pre-release Checklist

Before releasing a new exchange provider:

- [ ] All unit tests pass
- [ ] Integration tests pass
- [ ] Code coverage meets standards
- [ ] Error handling is comprehensive
- [ ] Performance meets requirements
- [ ] Documentation is complete
- [ ] Security review completed

### Version Compatibility

- Maintain backward compatibility for public interfaces
- Follow semantic versioning
- Document breaking changes
- Provide migration guides when needed

## Example Implementation

See the Binance implementation in `exchanges/binance/` for a complete example that meets all these standards.

## Compliance Verification

Use the following commands to verify compliance:

```bash
# Run all tests
make test

# Check code coverage
go test -cover ./exchanges/your-exchange/...

# Verify build
make build

# Run linting (if configured)
make lint
```

Providers that meet these standards will be considered production-ready and suitable for inclusion in the main repository.
