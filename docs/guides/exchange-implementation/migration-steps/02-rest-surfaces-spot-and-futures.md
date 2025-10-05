# 02 — REST Surfaces: Spot & Futures

**Goal:** Implement REST endpoints for Spot and (if supported) Futures (linear/inverse) to return canonical models with correct numerics and enums.

---
## What needs to be done
1) Implement required Spot endpoints (server time, instruments, ticker, order lifecycle).
2) If supported, implement Futures endpoints (instruments, positions, orders).
3) Normalize symbols to canonical form and use `*big.Rat` for all numeric quantities.
4) Map all enums/statuses to core enums with exhaustive switches.

---
## How to do it (follow exactly)
1) **Wire clients**
   - In `exchange/provider.go`, implement constructors for HTTP clients with timeouts, retry/backoff, and rate-limiting hooks (reuse shared transport if available).
   - Ensure methods on `SpotAPI` / `FuturesAPI` return **core models** (`Instrument`, `Ticker`, `Order`, `Position`, etc.).

2) **Canonical symbols**
   - Before constructing any model, convert exchange symbols to `BASE-QUOTE` uppercase (e.g., `"BTC-USDT"`). Use helpers from `core/symbols.go` such as `ToCanonicalSymbol(...)`.

3) **Numerics with `*big.Rat`**
   - All price, size, quantity, balance fields **must** be `*big.Rat`.
   - In JSON marshaling, use proper formatting for `*big.Rat` values.

4) **Enum/status mapping**
   - Implement functions like `mapOrderSide`, `mapOrderType`, `mapTimeInForce`, `mapOrderStatus`.
   - Use a `switch` that **covers every** known provider value; in `default`, return a canonical error (do not silently succeed).

5) **Orders**
   - Implement create/cancel/get and open-orders using canonical `OrderRequest` and `Order`.
   - Validate idempotency where supported by the exchange.

### Transport: retries and rate limiting (explicit knobs)
- Configure exponential backoff using the retry policy knobs: maximum retries, base delay (doubles per retry), and an optional maximum delay cap.
- Attach a rate limiter (token bucket or equivalent) sized to the provider's documented limits (capacity and refill-per-second).

---
## How to validate that it is complete
1) **Build & unit tests:**
   ```bash
   go build ./... && go test ./exchanges/<name> -count=1
   ```
2) **Symbols canonical; decimals via *big.Rat:**
   - Verify all symbols use `BASE-QUOTE` format
   - Verify all numeric fields use `*big.Rat`
   - Verify proper JSON marshaling
