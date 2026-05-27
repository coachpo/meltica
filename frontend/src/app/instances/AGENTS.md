# Instances Route Guidance

This file adds local rules for `frontend/src/app/instances`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep instance create, update, start, stop, delete, and history flows wired through `@/lib/hooks`. Mutations must invalidate the instance list, the affected instance, and any related provider or strategy-module cache touched by runtime state.

Preserve the JSON and guided spec paths in `spec-utils.ts`; don't add parallel instance spec formats.
