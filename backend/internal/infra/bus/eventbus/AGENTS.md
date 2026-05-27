# Event Bus Guidelines

This child file adds only local rules for `internal/infra/bus/eventbus`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep bus contracts, in memory fan out, durable hooks, and extensions focused on event delivery. Preserve cancellation behavior and subscriber cleanup, and test ordering or backpressure changes directly in this package.
