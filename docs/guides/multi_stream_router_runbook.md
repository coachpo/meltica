# Multi-Stream Router Deployment Runbook

## Preconditions

- All tests pass: `go test ./...`
- Binance adapter acceptance suites pass on staging.
- Observability dashboards updated with new telemetry metrics.
- Rollback artifacts prepared (previous container image + config snapshot).

## Cutover Steps

1. **Freeze Deployments**
   - Announce routing cutover in #ops 30 minutes in advance.
   - Disable automated deploys for the market-data stack.

2. **Deploy Router Services**
   - Build and publish the release image.
   - Apply database/schema migrations (none required for this release).
   - Deploy router services with the new processors enabled (use rolling update with 25% surge).

3. **Binance Adapter Switch**
   - Update configuration to point Binance WS sessions at the framework router entrypoint.
   - Validate keep-alive ticks and session counts via telemetry (expect no drop).

4. **Smoke Verification**
   - Run `go test ./exchanges/binance/... -run Smoke` against production mirrors.
   - Monitor Prometheus dashboards for routing latency and backpressure metrics.
   - Confirm no increase in invalid message counts.

5. **Release Gate**
   - If metrics remain nominal for 30 minutes, announce completion and re-enable automated deploys.

## Rollback Plan

1. Revert configuration to the previous router implementation.
2. Redeploy the prior release image.
3. Monitor sessions to confirm listen keys are stable.
4. Open an incident report documenting the trigger and timeline.

## Validation Checklist

- [ ] Cutover announcement sent
- [ ] Rolling update completed
- [ ] Smoke tests green
- [ ] Metrics steady for 30 minutes
- [ ] Ops handoff recorded in runbook log
