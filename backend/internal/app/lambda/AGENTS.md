# Lambda Guidelines

This child file adds only local rules for `internal/app/lambda`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep lifecycle orchestration in `runtime`, reusable primitives in `core`, JavaScript module loading in `js`, and built in strategy metadata in `strategies`. Any change to strategy registry semantics must keep the sibling `strategy/registry.json`, loader resolution, runtime usage tracking, and control API behavior aligned.
