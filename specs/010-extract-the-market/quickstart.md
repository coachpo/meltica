# Quickstart: /lib/ws-routing

## Goal
Route WebSocket events with per-symbol ordering using an abstract transport and structured logging.

## Steps

1) Provide an AbstractTransport implementation (outside ws-routing):
- Must expose Receive() <- Event, Send(Event) error, Close() error

2) Initialize ws-routing components:
- Create telemetry logger (vendor-neutral interface)
- Build middleware chain and handler registry
- Configure router options (per-symbol ordering, backpressure)

3) Start a Session with the abstract transport
- Pass the transport to session; do not create network connections inside the library

4) Subscribe and handle events
- Register handlers by event type; verify ordering in tests

5) Admin API (optional)
- Expose read-only endpoints defined in contracts/admin-api.yaml for health/subscriptions

## Notes
- Event schemas are unchanged; update imports to new lib paths
- Preserve benchmarks and run `go test ./... -race -bench . -benchmem`
