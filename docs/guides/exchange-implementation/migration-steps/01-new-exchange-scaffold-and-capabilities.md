# 01 — New Exchange Scaffold & Capabilities

**Goal:** Create the adapter skeleton for a new exchange (codename: `<name>`) and wire up basic metadata and capability bits.

---
## What needs to be done
1) Generate the exchange folder with all required files.
2) Implement the adapter's constructor and registration.
3) Declare capabilities and supported protocol version.
4) Add the adapter README with credential/env requirements.

---
## How to do it (follow exactly)
1) **Generate files**
   ```bash
   # Create the exchange directory structure manually
   mkdir -p exchanges/<name>
   mkdir -p exchanges/<name>/exchange
   mkdir -p exchanges/<name>/infra/rest
   mkdir -p exchanges/<name>/infra/ws
   mkdir -p exchanges/<name>/routing
   mkdir -p exchanges/<name>/internal
   ```
   Expected files: `<name>.go`, `exchange/provider.go`, `infra/rest/client.go`, `infra/ws/client.go`, `routing/rest_router.go`, `routing/ws_router.go`, `README.md`.

2) **Implement Exchange interface in `exchanges/<name>/<name>.go`**
   - Implement methods exactly as in the current repository's Exchange interface (see README for shape):
     ```go
     type Exchange interface {
         Name() string
         Capabilities() ExchangeCapabilities
         SupportedProtocolVersion() string
     }
     ```
   - `Name()` returns a stable lowercase identifier (e.g., `"binance"`).
   - `SupportedProtocolVersion()` returns `core.ProtocolVersion`.

3) **Declare capabilities**
   - In `<name>.go`, implement `Capabilities()` returning a bitset consistent with what you will implement.
   - Keep bits **off** for features you won't implement in this sprint.

4) **Add adapter README**
   - Document required env vars for optional live tests (`<NAME>_KEY`, `<NAME>_SECRET`, etc.).
   - Document any base URLs (prod/testnet) and signing quirks.

---
## How to validate that it is complete
1) Files exist:
   ```bash
   test -f exchanges/<name>/<name>.go &&    test -f exchanges/<name>/exchange/provider.go &&    test -f exchanges/<name>/infra/rest/client.go &&    test -f exchanges/<name>/infra/ws/client.go &&    test -f exchanges/<name>/README.md
   ```
2) Build + basic tests:
   ```bash
   go build ./... && go test ./... -count=1
   ```
3) Protocol version alignment is exact:
   ```bash
   grep -R "SupportedProtocolVersion()" -n exchanges/<name> &&    grep -R "core.ProtocolVersion" -n exchanges/<name>
   ```
