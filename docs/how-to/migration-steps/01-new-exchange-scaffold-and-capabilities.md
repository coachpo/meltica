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
   go run ./cmd/xprovgen --name <name>
   tree providers/<name>
   ```
   Expected files: `<name>.go`, `sign.go`, `errors.go`, `status.go`, `ws.go`, `ws_private.go` (if supported), `conformance_test.go`, `golden_test.go`, `README.md`.

2) **Implement Provider interface in `providers/<name>/<name>.go`**
   - Implement methods exactly as in the current repository’s Provider interface (see README for shape):
     ```go
     type Provider interface {
         Name() string
         Capabilities() ProviderCapabilities
         SupportedProtocolVersion() string
         Spot(ctx context.Context) SpotAPI
         LinearFutures(ctx context.Context) FuturesAPI
         InverseFutures(ctx context.Context) FuturesAPI
         WS() WS
         Close() error
     }
     ```
   - `Name()` returns a stable lowercase identifier (e.g., `"bybit"`).
   - `SupportedProtocolVersion()` returns `protocol.ProtocolVersion`.
   - Return concrete Spot/Futures clients (can be lightweight stubs initially that compile).

3) **Declare capabilities**
   - In `<name>.go`, implement `Capabilities()` returning a bitset consistent with what you will implement in Teams B–D (Spot/Linear/Inverse/WS public/WS private etc.).
   - Keep bits **off** for features you won’t implement in this sprint.

4) **Add adapter README**
   - Document required env vars for optional live tests (`<NAME>_KEY`, `<NAME>_SECRET`, etc.).
   - Document any base URLs (prod/testnet) and signing quirks.

---
## How to validate that it is complete
1) Files exist:
   ```bash
   test -f providers/<name>/<name>.go &&    test -f providers/<name>/errors.go &&    test -f providers/<name>/status.go &&    test -f providers/<name>/ws.go &&    test -f providers/<name>/conformance_test.go &&    test -f providers/<name>/golden_test.go &&    test -f providers/<name>/README.md
   ```
2) Build + basic tests:
   ```bash
   go build ./... && go test ./... -count=1
   ```
3) Static checks (capabilities/API alignment, interface surface, no floats, etc.):
   ```bash
   go build ./internal/protolint/cmd/protolint
   ./protolint ./core ./providers/<name>
   ```
4) Protocol version alignment is exact:
   ```bash
   grep -R "SupportedProtocolVersion()" -n providers/<name> &&    grep -R "protocol.ProtocolVersion" -n providers/<name>
   ```
