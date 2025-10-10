# Multi-Stream Router Troubleshooting

## Common Issues

### Processor Not Invoked
- Ensure the message type descriptor matches the incoming payload.
- Run `go test ./market_data/framework/router -run Detect` to verify detection logic.
- Enable `RoutingMetrics.RecordRoute` logging to confirm detection events.

### Backpressure Warnings
- Check processor throughput; update processor implementation to avoid long blocking operations.
- Increase worker concurrency via processor registration if the processor supports it.
- Monitor `routing_backpressure_total` metric for spikes.

### Invalid Message Spike
- Review processor validation errors (`errs.CodeInvalid`).
- Confirm upstream exchange schema changes; update processors accordingly.
- Adjust validation thresholds via `connection.DialOptions.InvalidThreshold` or processor-specific configuration.

### Session Disconnects
- Inspect telemetry emitted from `exchanges/binance/telemetry` for heartbeat timeouts.
- Verify keep-alive intervals using `routing.SetPrivateKeepAliveInterval` in tests and production configuration.

## Escalation
- If issues persist, contact the routing framework maintainers (listed in `docs/README.md`).
- Provide relevant logs, metrics snapshots, and reproduction steps when filing incidents.
