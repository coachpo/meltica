# Config Guidelines

This child file adds only local rules for `internal/infra/config`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep YAML loading, defaults, provider specs, lambda specs, risk config, and validation here. When config fields change, update example and CI config files with safe non secret values.
