# Application Layer Guidelines

This child file adds only local rules for `internal/app`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep this layer about orchestration and runtime lifecycle. Domain types and store contracts come from `internal/domain`; side effects, persistence, adapters, config, pools, telemetry, and HTTP stay in `internal/infra`.
