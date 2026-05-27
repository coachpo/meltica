# Command Guidelines

This child file adds only local rules for backend command binaries.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep `cmd/gateway` focused on startup wiring, config selection, migrations, telemetry, provider startup, and HTTP server lifecycle. Keep `cmd/migrate` focused on migration CLI behavior. Move reusable logic into `internal/app` or `internal/infra` instead of growing command packages.
