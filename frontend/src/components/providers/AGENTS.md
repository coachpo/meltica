# Provider Components Guidance

This file adds local rules for `frontend/src/components/providers`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep app-wide client providers here, including React Query hydration and query provider wiring. Route-specific provider forms don't belong in this folder.

When query defaults change, update `@/lib/react-query` and tests that rely on retry, stale time, or hydration behavior.
