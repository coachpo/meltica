# Adapter Guidelines

This child file adds only local rules for `internal/infra/adapters`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep venue specific REST, websocket, manifests, timestamp handling, and accounting here. Publish canonical schema events through shared adapter helpers and keep provider lifecycle decisions in `internal/app/provider`.
