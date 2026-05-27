# Library Guidance

This file adds local rules for `frontend/src/lib`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep reusable frontend logic here: REST helpers, query hooks, shared types, backup formatting, risk helpers, React Query defaults, and small utilities.

Don't put route rendering or component state machines in `lib`. If logic talks to the gateway, prefer `api` plus `hooks` instead of direct fetch calls.
