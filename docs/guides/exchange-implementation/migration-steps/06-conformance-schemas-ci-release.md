# 06 — Conformance, Schemas, CI & Release (Team F)

**Goal:** Make the adapter provably compliant and wired into CI so regressions cannot merge.

---
## What needs to be done
1) Ensure all unit tests pass for the new exchange.
2) Add integration tests for end-to-end flows.
3) Add/verify CI gates (build, test, integration matrix, protocol version guard).
4) Prepare optional live test path, versioning, and release notes.

---
## How to do it (follow exactly)
1) **Unit and integration testing**
   - Ensure all unit tests pass: `go test ./exchanges/<name> -count=1`
   - Add integration tests for end-to-end flows
   - Test with mocked transports for deterministic behavior

2) **CI gates**
   - Ensure pipeline runs:
     ```bash
     go build ./...
     go test ./... -race -count=1
     ```
   - Add protocol version guard (fail if protocol files change without bump).

3) **Optional live tests (gated)**
   - Document required env vars in `exchanges/<name>/README.md`.
   - Only run when credentials are provided via environment variables.

4) **Versioning & release**
   - Exchange declares `SupportedProtocolVersion() == core.ProtocolVersion`.
   - Tag releases per SemVer once green; update top-level README exchange matrix.

### Helper tools (optional)
- Schema validator: validates JSON Schemas and vectors.
- Protocol linter: enforces capability alignment and coding standards.

---
## How to validate that it is complete
1) **All green locally:**
   ```bash
   make test || go test ./... -race -count=1
   go test ./exchanges/<name> -count=1
   ```
2) **CI shows:**
   - Build & unit tests green.
   - Integration tests pass.
   - Protocol version guard enforced.
   - Capability mismatch gate passes (only claim what you pass).
3) **Docs updated:**
   - `exchanges/<name>/README.md` includes creds and endpoints.
   - Root `README.md` exchange table updated.
