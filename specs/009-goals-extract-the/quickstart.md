# Quickstart: Adopting `/lib/ws-routing`

1. **Add Framework Dependency**
   - Update Go files in `market_data` (or other domain package) to import `github.com/coachpo/meltica/lib/ws-routing` instead of legacy internal routing paths.

2. **Initialize a Session**
   - Call `wsrouting.Init(ctx, wsrouting.Options{SessionID: "market-data"})` during adapter startup.
   - Supply existing logger and backoff configuration to match prior behavior.

3. **Register Middleware (Optional)**
   - Use `wsrouting.UseMiddleware(session, middlewareFn)` to add domain-specific hooks for metrics or transformation.

4. **Start Streaming**
   - Invoke `wsrouting.Start(ctx, session)` and wait for successful connection callbacks.
   - Subscribe to required channels via `wsrouting.Subscribe` with canonical symbols.

5. **Publish to Domain Handlers**
   - Implement `Publish` callbacks that convert `routing.Message` payloads into market data events.
   - Verify outputs align with legacy schemas using provided contract tests.

6. **Maintain Backward Compatibility**
   - Leave the generated deprecation shim in `market_data/adapters/websocket/shim.go` until the next release.
   - Reference the migration notes for removal timeline and communication checklist.

7. **Validate**
   - Run `go test ./... -race`, `make lint-layers`, and the new smoke test under `tests/integration/market_data` to confirm parity. *(Last run 2025-10-11: all passing.)*
   - Capture structured logs from `tests/integration/market_data/smoke_test.go` to confirm the new framework replays the same `session_start`, `subscription`, and `route_dispatch` fields observed prior to migration. No new keys or severity changes were detected.
