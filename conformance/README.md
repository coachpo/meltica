# Conformance Harness (WIP)

This package will host the offline conformance runner that executes schema validation,
JSON vector comparison, and adapter capability checks. Until implemented, adapters rely on unit tests only.

Planned coverage:
- Schema validation against `protocol/schemas`
- Golden vector comparison for canonical models/events
- Capability-aware test matrix for providers
- Optional live suites gated by environment variables
