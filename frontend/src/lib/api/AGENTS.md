# API Boundary Guidance

This file adds local rules for `frontend/src/lib/api`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

This folder is the REST boundary for the gateway. Use `requestJson` or `requestText`, Zod response schemas, and `ApiError` or `StrategyValidationError` handling rather than raw fetch.

`src/lib/api-types.ts` is generated from `frontend-api.yaml`; never hand-edit it. Contract changes must update the OpenAPI file, regenerate types, then align schemas, hooks, MSW, and tests.
