# HTTP Server Guidelines

This child file adds only local rules for `internal/infra/server/http`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep REST routing, request parsing, response envelopes, and control API wiring here. Any HTTP contract change must stay in sync with `frontend/frontend-api.yaml`, regenerated `frontend/src/lib/api-types.ts`, frontend hooks or validators, MSW handlers, and relevant frontend tests.
