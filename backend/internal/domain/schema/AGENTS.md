# Schema Guidelines

This child file adds only local rules for `internal/domain/schema`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Treat events, orders, instruments, provider payloads, and control messages as canonical gateway contracts. Keep JSON tags intentional, clone helpers safe, and tests close to each schema change.
