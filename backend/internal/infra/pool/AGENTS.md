# Pool Guidelines

This child file adds only local rules for `internal/infra/pool`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep hot path object pooling, JSON helpers, and order allocation behavior here. Be explicit about ownership, reset state before reuse, and test for stale data when adding pooled types.
