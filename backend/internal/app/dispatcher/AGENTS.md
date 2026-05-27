# Dispatcher Guidelines

This child file adds only local rules for `internal/app/dispatcher`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep routing tables, registrar behavior, trading state, and runtime fan out deterministic. Preserve canonical `schema.Event` boundaries and test route ordering, subscription changes, and state transitions beside the dispatcher code.
