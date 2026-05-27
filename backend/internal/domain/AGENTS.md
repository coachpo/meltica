# Domain Guidelines

This child file adds only local rules for `internal/domain`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep this layer free of I/O and runtime orchestration. Store interfaces, canonical schemas, and shared error envelopes live here so app and infra packages can depend on stable domain contracts.
