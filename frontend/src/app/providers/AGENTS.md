# Providers Route Guidance

This file adds local rules for `frontend/src/app/providers`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Provider screens should manage gateway provider resources only. Adapter metadata can be displayed here, but adapter discovery logic belongs in shared API or hook layers.

After provider mutations, keep provider lists, provider details, balances, and dependent instance state in sync through existing query keys.
