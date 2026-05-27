# Adapters Route Guidance

This file adds local rules for `frontend/src/app/adapters`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep this route focused on adapter capability discovery from the gateway API. Don't duplicate provider setup flows that belong under `providers`.

When adapter schemas change, update the API schema, generated types, validators, hooks, MSW handlers, and the adapter Playwright coverage together.
