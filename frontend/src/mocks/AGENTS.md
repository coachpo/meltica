# MSW Guidance

This file adds local rules for `frontend/src/mocks`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

MSW handlers must mirror the gateway REST API used by `src/lib/api`. Keep paths, status codes, response shapes, and error payloads aligned with schemas and generated types.

When adding an API helper or changing a contract, update MSW in the same change so Vitest and Playwright tests keep parity with the gateway.
