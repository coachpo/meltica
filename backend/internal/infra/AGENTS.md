# Infrastructure Guidelines

This child file adds only local rules for `internal/infra`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep external I/O, adapters, HTTP, config, persistence, pools, event bus, and telemetry here. Business orchestration belongs in `internal/app`; canonical contracts and store interfaces belong in `internal/domain`.
