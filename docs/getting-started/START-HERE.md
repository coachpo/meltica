## Start Here

Use this index to implement or migrate an exchange adapter in the right order. Links are relative to this folder.

### TL;DR sequence
1. Expectations (read first)
   - [Abstractions guidelines](../standards/expectations/abstractions-guidelines.md)
   - [Protocol and code standards](../standards/expectations/baseline-expectations/protocol-and-code-standards.md)
   - [Exchange adapter standards](../standards/expectations/baseline-expectations/provider-adapter-standards.md)
   - [Delivery and CI standards](../standards/expectations/baseline-expectations/delivery-and-ci-standards.md)
2. Choose your path
   - New exchange: [Implementing an exchange adapter](../guides/exchange-implementation/implementing-a-provider.md)
   - Migrating existing exchange: [Migration guidelines](../guides/exchange-implementation/migration-guidelines.md)
3. Implement in steps (do in order)
   - Step 01: [New exchange scaffold and capabilities](../guides/exchange-implementation/migration-steps/01-new-exchange-scaffold-and-capabilities.md)
   - Step 02: [REST surfaces (spot and futures)](../guides/exchange-implementation/migration-steps/02-rest-surfaces-spot-and-futures.md)
   - Step 03: [WebSocket public streams](../guides/exchange-implementation/migration-steps/03-websocket-public-streams.md)
   - Step 04: [WebSocket private streams](../guides/exchange-implementation/migration-steps/04-websocket-private-streams.md)
   - Step 05: [Error/status mapping and normalization](../guides/exchange-implementation/migration-steps/05-error-status-mapping-and-normalization.md)
   - Step 06: [Integration, CI, release](../guides/exchange-implementation/migration-steps/06-conformance-schemas-ci-release.md)
4. Validate (before merge/release)
   - [Protocol validation rules](../validation/protocol-validation-rules.md)
   - Run `go test ./... -race -count=1`

### When to use what
- New exchange from scratch: start with [Implementing an exchange adapter](../guides/exchange-implementation/implementing-a-provider.md), then Steps 01 → 06.
- Migrating an existing exchange: read [Migration guidelines](../guides/exchange-implementation/migration-guidelines.md), then Steps 01 → 06.
- Only building REST endpoints right now: focus on Step 02.
- Adding public market data via WebSocket: do Step 03.
- Adding private user streams (orders/balances): do Step 04.
- Normalizing error/status codes: do Step 05.
- Pre-merge gates (integration/CI/release): do Step 06 and then run through [Protocol validation rules](../validation/protocol-validation-rules.md).
- Confused about modeling or entities: check [Abstractions guidelines](../standards/expectations/abstractions-guidelines.md).
- Unsure about code/protocol style: see [Protocol and code standards](../standards/expectations/baseline-expectations/protocol-and-code-standards.md).
- Unsure what the adapter must implement: see [Exchange adapter standards](../standards/expectations/baseline-expectations/provider-adapter-standards.md).
- Unsure about delivery/branching/CI: see [Delivery and CI standards](../standards/expectations/baseline-expectations/delivery-and-ci-standards.md).


