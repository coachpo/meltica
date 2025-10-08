# 03 — WebSocket Public Streams (Team C)

**Goal:** Implement public WS channels (trades, ticker, depth) with robust reconnect and typed event decoding.

---
## What needs to be done
1) Connect and maintain public WS with auto-reconnect, backoff, and heartbeats.
2) Decode exchange messages into `corestreams.TradeEvent`, `corestreams.TickerEvent`, `corestreams.BookEvent`.
3) Canonicalize symbols and ensure typed (not `interface{}`) payloads.

---
## How to do it (follow exactly)
1) **Client**
   - In `exchanges/<name>/infra/ws/client.go`, implement connect logic with:
     - Reconnect + jittered backoff
     - Heartbeats/pings per provider spec
     - Multiplexed subscriptions using canonical topics
   - Preserve subscription list across reconnect.

2) **Decoders**
   - Implement message decoding via the `StreamRegistry` in `exchanges/<name>/routing/stream_registry.go` (registered by `dispatchers.go`).
   - Return **concrete** `corestreams.*` event types.
   - Always convert symbols to canonical `BASE-QUOTE` **before** emitting the event.
   - Attach raw JSON to the message `Raw` field.

3) **Topic System**
   - Expose topic builders from your exchange's routing package (see `exchanges/binance/routing` for a reference implementation):
     ```go
     import bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"

     // Subscribe to trades for BTC-USDT
     topic := bnrouting.Trade("BTC-USDT")
     ```

4) **Ordering & gaps**
   - If provider supplies sequence numbers, track and log gaps; surface via metrics.

---
## How to validate that it is complete
1) **Build & unit tests:**
   ```bash
   go build ./... && go test ./exchanges/<name> -count=1
   ```
2) **WS decoder tests:**
   ```bash
   go test ./exchanges/<name> -run TestPublicWS -count=1
   ```
3) **Soak test (optional local run):**
   ```bash
   go test ./exchanges/<name> -run TestPublicWS -timeout 30m
   ```
   Expect <0.1% message loss and stable memory/FDs under observation.
