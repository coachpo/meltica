# 04 — WebSocket Private Streams (Team D)

**Goal:** Implement authenticated/private WS (orders, balances, positions) with sequencing and recovery.

---
## What needs to be done
1) Establish authenticated WS session (listen keys or signed channels).
2) Emit `core.OrderEvent` and `core.BalanceEvent` from private updates.
3) Handle sequencing, gap detection, and catch-up on reconnect.

---
## How to do it (follow exactly)
1) **Session & auth**
   - In `ws_private.go`, implement session creation per provider (listen key endpoints, signed subscribe frames, or cookies).
   - Keep-alive/refresh tokens or listen keys on schedule.

2) **Decoders**
   - Implement decoder functions returning **concrete** `core.OrderEvent` and `core.BalanceEvent` only.
   - Canonicalize symbols before emitting.

3) **Sequencing & recovery**
   - Track last sequence/ts; on reconnect, fetch deltas via REST if the provider supports it (orders/positions since `lastSeq`).
   - Ensure no duplicate or missing updates reach callers.

---
## How to validate that it is complete
1) **Static analysis:**
   ```bash
   ./meltilint ./providers/<name>
   ```
2) **Offline conformance (private topics):**
   ```bash
   go test ./conformance -run TestOffline
   ```
3) **Optional live (gated):**
   ```bash
   export MELTICA_CONFORMANCE=1
   export <NAME>_KEY=...
   export <NAME>_SECRET=...
   go test ./internal/test -run <Provider>Conformance -v
   ```
   Without creds, tests must skip cleanly.
