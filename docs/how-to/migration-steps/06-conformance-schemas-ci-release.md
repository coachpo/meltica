# 06 — Conformance, Schemas, CI & Release (Team F)

**Goal:** Make the adapter provably compliant and wired into CI so regressions cannot merge.

---
## What needs to be done
1) Hook `conformance.RunAll` for the new provider in offline mode.
2) Ensure JSON Schemas and golden vectors validate.
3) Add/verify CI gates (build, test, conformance matrix, protocol version guard).
4) Prepare optional live test path, versioning, and release notes.

---
## How to do it (follow exactly)
1) **Offline conformance**
   - In `providers/<name>/conformance_test.go`, invoke the harness factory to construct the provider with mocked transport and run:
     ```go
     conformance.RunAll(t, factory, conformance.Options{})
     ```

2) **Schemas & vectors**
   - Confirm schemas exist for all models/events you emit and that vectors cover each type.
   - Validate:
     ```bash
     go run ./cmd/validate-schemas
     ```

3) **CI gates**
   - Ensure pipeline runs:
     ```bash
     go build ./...
     go test ./... -race -count=1
     go build ./internal/protolint/cmd/protolint && ./protolint ./core ./providers/...
     go test ./conformance -run TestOffline
     ```
   - Add protocol version guard (fail if protocol files change without bump).

4) **Optional live tests (gated)**
   - Document required env vars in `providers/<name>/README.md`.
   - Only run when:
     ```bash
     export MELTICA_CONFORMANCE=1
     ```

5) **Versioning & release**
   - Adapter declares `SupportedProtocolVersion() == protocol.ProtocolVersion`.
   - Tag releases per SemVer once green; update top-level README provider matrix.

### Helper tools (optional)
- Schema validator: validates JSON Schemas and vectors.
- Protocol linter: enforces capability alignment and coding standards.

---
## How to validate that it is complete
1) **All green locally:**
   ```bash
   make test || go test ./... -race -count=1
   go build ./internal/protolint/cmd/protolint && ./protolint ./core ./providers/<name>
   go run ./cmd/validate-schemas
   go test ./conformance -run TestOffline
   ```
2) **CI shows:**
   - Build & unit tests green.
   - Offline conformance green across matrix.
   - Protocol version guard enforced.
   - Capability mismatch gate passes (only claim what you pass).
3) **Docs updated:**
   - `providers/<name>/README.md` includes creds and endpoints.
   - Root `README.md` provider table updated.
