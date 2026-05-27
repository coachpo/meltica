# Provider Guidelines

This child file adds only local rules for `internal/app/provider`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep provider contracts, registry metadata, lifecycle state, and manager orchestration here. Adapter specific network behavior belongs under `internal/infra/adapters`; persistence contracts come from `internal/domain/providerstore`.
