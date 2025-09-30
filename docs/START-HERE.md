## Start Here

Use this index to implement or migrate a provider in the right order. Links are relative to this folder.

### TL;DR sequence
1. Expectations (read first)
   - [Abstractions guidelines](expectations/abstractions-guidelines.md)
   - [Protocol and code standards](expectations/baseline-expectations/protocol-and-code-standards.md)
   - [Provider adapter standards](expectations/baseline-expectations/provider-adapter-standards.md)
   - [Delivery and CI standards](expectations/baseline-expectations/delivery-and-ci-standards.md)
2. Choose your path
   - New provider: [Implementing a provider](how-to/implementing-a-provider.md)
   - Migrating existing provider: [Migration guidelines](how-to/migration-guidelines.md)
3. Implement in steps (do in order)
   - Step 01: [New exchange scaffold and capabilities](how-to/migration-steps/01-new-exchange-scaffold-and-capabilities.md)
   - Step 02: [REST surfaces (spot and futures)](how-to/migration-steps/02-rest-surfaces-spot-and-futures.md)
   - Step 03: [WebSocket public streams](how-to/migration-steps/03-websocket-public-streams.md)
   - Step 04: [WebSocket private streams](how-to/migration-steps/04-websocket-private-streams.md)
   - Step 05: [Error/status mapping and normalization](how-to/migration-steps/05-error-status-mapping-and-normalization.md)
   - Step 06: [Conformance, schemas, CI, release](how-to/migration-steps/06-conformance-schemas-ci-release.md)
4. Validate (before merge/release)
   - [Protocol validation rules](validation/protocol-validation-rules.md)
   - Optional helpers: run `go run ./cmd/validate-schemas` and `./protolint ./core ./providers/...`

### When to use what
- New provider from scratch: start with [Implementing a provider](how-to/implementing-a-provider.md), then Steps 01 → 06.
- Migrating an existing provider: read [Migration guidelines](how-to/migration-guidelines.md), then Steps 01 → 06.
- Only building REST endpoints right now: focus on Step 02.
- Adding public market data via WebSocket: do Step 03.
- Adding private user streams (orders/balances): do Step 04.
- Normalizing error/status codes: do Step 05.
- Pre-merge gates (conformance/schemas/CI/release): do Step 06 and then run through [Protocol validation rules](validation/protocol-validation-rules.md).
- Confused about modeling or entities: check [Abstractions guidelines](expectations/abstractions-guidelines.md).
- Unsure about code/protocol style: see [Protocol and code standards](expectations/baseline-expectations/protocol-and-code-standards.md).
- Unsure what the adapter must implement: see [Provider adapter standards](expectations/baseline-expectations/provider-adapter-standards.md).
- Unsure about delivery/branching/CI: see [Delivery and CI standards](expectations/baseline-expectations/delivery-and-ci-standards.md).


