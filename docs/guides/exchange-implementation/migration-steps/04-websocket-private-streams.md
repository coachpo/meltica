# 04 — WebSocket Private Streams (Team D)

**Goal:** Implement authenticated/private WS (orders, balances, positions) with sequencing and recovery.

---
## What needs to be done
1) Establish authenticated WS session (listen keys or signed channels).
2) Emit `core/ws.OrderEvent` and `core/ws.BalanceEvent` from private updates.
3) Handle sequencing, gap detection, and catch-up on reconnect.

---
## How to do it (follow exactly)
1) **Session & auth**
   - In `ws_private.go`, implement session creation per provider (listen key endpoints, signed subscribe frames, or cookies).
   - Keep-alive/refresh tokens or listen keys on schedule.

2) **Decoders**
   - Implement decoder functions returning **concrete** `core/ws.OrderEvent` and `core/ws.BalanceEvent` only.
   - Canonicalize symbols before emitting.

3) **Sequencing & recovery**
   - Track last sequence/ts; on reconnect, fetch deltas via REST if the provider supports it (orders/positions since `lastSeq`).
   - Ensure no duplicate or missing updates reach callers.

---
## How to validate that it is complete
1) **Build & unit tests:**
   ```bash
   go build ./... && go test ./exchanges/<name> -count=1
   ```
2) **WS private decoder tests:**
   ```bash
   go test ./exchanges/<name> -run TestPrivateWS -count=1
   ```
3) **Optional live test (gated):**
   ```bash
   export <NAME>_KEY=...
   export <NAME>_SECRET=...
   go test ./exchanges/<name> -run TestPrivateWS -v
   ```
   Without creds, tests must skip cleanly.
