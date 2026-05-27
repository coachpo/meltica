# Persistence Guidelines

This child file adds only local rules for `internal/infra/persistence`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep root persistence wiring and migration helpers here, with concrete repository implementations in subpackages. Schema changes should move with migrations, sqlc query updates, and focused persistence tests.
