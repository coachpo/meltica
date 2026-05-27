# Telemetry Guidelines

This child file adds only local rules for `internal/infra/telemetry`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep OpenTelemetry setup, semantic conventions, metrics, and exporter wiring here. Avoid business decisions in instrumentation code, and keep tests strict about attribute names used by dashboards and alerts.
