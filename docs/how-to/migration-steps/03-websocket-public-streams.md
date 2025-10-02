# 03 — WebSocket Public Streams (Team C)

**Goal:** Implement public WS channels (trades, ticker, depth) with robust reconnect and typed event decoding.

---
## What needs to be done
1) Connect and maintain public WS with auto-reconnect, backoff, and heartbeats.
2) Decode exchange messages into `core/ws.TradeEvent`, `core/ws.TickerEvent`, `core/ws.DepthEvent`.
3) Canonicalize symbols and ensure typed (not `interface{}`) payloads.

---
## How to do it (follow exactly)
1) **Client**
   - In `providers/<name>/ws.go`, implement connect logic with:
     - Reconnect + jittered backoff
     - Heartbeats/pings per provider spec
     - Multiplexed subscriptions (topics like `trades:BTC-USDT`)
   - Preserve subscription list across reconnect.

2) **Decoders**
   - Implement `decodeTradeToEvent`, `decodeTickerToEvent`, `decodeDepthToEvent` (name pattern ok) returning **concrete** `core.*Event` types.
   - Always convert symbols to canonical `BASE-QUOTE` **before** emitting the event.
   - Attach raw JSON to the event/message `Raw` field if the model supports it.

3) **Ordering & gaps**
   - If provider supplies sequence numbers, track and log gaps; surface via metrics.

---
## How to validate that it is complete
1) **Static analysis (typed decoders, no floats, symbol normalization present):**
   ```bash
   ./meltilint ./providers/<name>
   ```
2) **Offline WS decoder tests:**
   ```bash
   go test ./conformance -run TestOffline
   ```
3) **Soak test (optional local run):**
   ```bash
   go test ./providers/<name> -run TestPublicWS -timeout 30m
   ```
   Expect <0.1% message loss and stable memory/FDs under observation.
