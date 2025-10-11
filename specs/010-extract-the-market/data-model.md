# Data Model: /lib/ws-routing

## Entities

- Session
  - Fields: ID (string), StartedAt (time), Status (enum: starting|running|stopped)
  - Relations: uses AbstractTransport; owns MiddlewareChain, HandlerRegistry, Router
  - Notes: Does not own network connection creation; lifecycle limited to routing orchestration

- AbstractTransport (interface)
  - Methods: Receive() <- Event, Send(Event) error, Close() error
  - Notes: Domain-agnostic; implemented by L1 connection layer

- Event
  - Fields: Symbol (string), Type (enum), Payload (bytes/JSON), Ts (time)
  - Notes: Schema preserved; no renames/shape changes

- HandlerRegistry
  - Fields: map[Type][]Handler
  - Notes: Supports per-type handlers; tested for exhaustive mappings

- MiddlewareChain
  - Fields: []Middleware
  - Notes: Applied before routing; must be allocation-free on hot path

- Router
  - Fields: Options (ordering: per-symbol), Concurrency (by symbol)
  - Notes: Guarantees per-symbol order; parallel across symbols; applies backpressure (block) on slow handlers

- TelemetryLogger (interface)
  - Methods: Debug(ctx,msg,fields...), Info(...), Warn(...), Error(...)
  - Notes: Vendor-neutral; no SDK types in signatures

## Constraints

- No domain-specific names/types/docs in public APIs
- Stable package names; import path changes require explicit spec approval
- Zero allocations on hot path; PERF-07 latencies enforced
- Exhaustive boundary tests for exported APIs and enums (TS-10)
