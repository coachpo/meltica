# 01 — New Exchange Scaffold & Capabilities (Team A)

**Goal:** Create the adapter skeleton for a new exchange (codename: `<name>`) and wire up basic metadata and capability bits.

---
## What needs to be done
1) Generate the provider folder with all required files.
2) Implement the adapter's constructor and registration.
3) Declare capabilities and supported protocol version.
4) Add the adapter README with credential/env requirements.

---
## How to do it (follow exactly)
1) **Generate files**
   ```bash
   # Create the exchange directory structure manually
   mkdir -p exchanges/<name>
   ```
   Expected files: `<name>.go`, `sign.go`, `errors.go`, `status.go`, `ws.go`, `ws_private.go` (if supported), `README.md`.

2) **Implement Exchange interface in `exchanges/<name>/<name>.go`**
   - Implement methods exactly as in the current repository's Exchange interface (see README for shape):
     ```go
     type Exchange interface {
         Name() string
         Capabilities() ExchangeCapabilities
         SupportedProtocolVersion() string
         Spot(ctx context.Context) SpotAPI
         LinearFutures(ctx context.Context) FuturesAPI
         InverseFutures(ctx context.Context) FuturesAPI
         WS() WS
         Close() error
     }
     ```
   - `Name()` returns a stable lowercase identifier (e.g., `"bybit"`).
   - `SupportedProtocolVersion()` returns `core.ProtocolVersion`.
   - Return concrete Spot/Futures clients (can be lightweight stubs initially that compile).

3) **Declare capabilities**
   - In `<name>.go`, implement `Capabilities()` returning a bitset consistent with what you will implement in Teams B–D (Spot/Linear/Inverse/WS public/WS private etc.).
   - Keep bits **off** for features you won't implement in this sprint.

4) **Add adapter README**
   - Document required env vars for optional live tests (`<NAME>_KEY`, `<NAME>_SECRET`, etc.).
   - Document any base URLs (prod/testnet) and signing quirks.

---
## How to validate that it is complete
1) Files exist:
   ```bash
   test -f exchanges/<name>/<name>.go &&    test -f exchanges/<name>/errors.go &&    test -f exchanges/<name>/status.go &&    test -f exchanges/<name>/ws.go &&    test -f exchanges/<name>/README.md
   ```
2) Build + basic tests:
   ```bash
   go build ./... && go test ./... -count=1
   ```
3) Protocol version alignment is exact:
   ```bash
   grep -R "SupportedProtocolVersion()" -n exchanges/<name> &&    grep -R "core.ProtocolVersion" -n exchanges/<name>
   ```
