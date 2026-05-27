# Hook Guidance

This file adds local rules for `frontend/src/lib/hooks`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Hooks wrap `src/lib/api` with TanStack Query. Keep query keys centralized in `query-keys.ts` and make filters serializable and stable.

Every mutation must define success and error notifications where user-visible, then invalidate all affected query keys. Add or update hook tests for cache and error behavior.
