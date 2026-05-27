# Repository Guidelines

**Generated:** 2026-05-27
**Commit:** b3a456d
**Branch:** main

## Scope & Hierarchy
- This file applies repo-wide.
- `backend/AGENTS.md`, `frontend/AGENTS.md`, and `strategy/AGENTS.md` override this file inside their trees.
- Many maintained source subtrees now have their own child `AGENTS.md` files under `backend/` and `frontend/`; prefer the nearest file when rules differ.

## Monorepo Map
- `backend/` — Go gateway, runtime orchestration, adapters, persistence, and the control-plane HTTP API.
- `frontend/` — Next.js 16 / React 19 operator UI. It talks to the gateway REST API only.
- `strategy/` — versioned JavaScript strategy registry. `registry.json` is the source of truth.

## Working Rules
- The repo has no external users yet. Prefer clean architecture and current best practices over backward-compatibility shims, speculative migration layers, or legacy aliases unless a task explicitly asks for them.
- Keep backend, frontend, and strategy changes isolated unless a feature or contract change genuinely crosses boundaries.
- Do not hand-edit generated files. Regenerate instead:
  - `backend/internal/infra/persistence/postgres/sqlc/*.go`
  - `frontend/src/lib/api-types.ts`
- Do not commit secrets. Use example configs and local env files instead.

## Where To Work
| Task | Primary location | Notes |
| --- | --- | --- |
| Gateway runtime, adapters, persistence, control API | `backend/` | See `backend/AGENTS.md` for commands and layering rules. |
| Operator UI, routes, hooks, tests | `frontend/` | See `frontend/AGENTS.md` for Next.js, Query, and Playwright/Vitest rules. |
| Strategy bundle publishing and registry changes | `strategy/` | See `strategy/AGENTS.md` for hash, tag, and manifest invariants. |
| Cross-stack API contract updates | `backend/` + `frontend/` | Keep handlers, `frontend-api.yaml`, generated types, hooks, mocks, and tests in sync. |

## Cross-Stack Rules
- Frontend must not read strategy files directly; operator flows go through the gateway API.
- Backend API/schema changes require coordinated updates to `frontend/frontend-api.yaml`, `frontend/src/lib/api-types.ts`, frontend hooks/validators, MSW handlers, and relevant tests.
- Strategy registry semantics changes require coordinated updates to `strategy/registry.json`, backend loader/runtime behavior, and frontend strategy-module workflows.
- Prefer atomic changes by subsystem. If a task crosses subsystems, state the contract boundary in the change description.

## Common Commands
```bash
(cd backend && make run)
(cd backend && make test && make coverage)
(cd frontend && pnpm dev)
(cd frontend && pnpm test && pnpm test:e2e)
(cd strategy && node gc.js)
```

## Verification
- Docs-only changes: reread the touched AGENTS files and remove parent/child duplication.
- Backend changes: `make lint && make test`.
- Frontend changes: `pnpm lint && pnpm test`.
- Strategy changes: verify `registry.json`, bundle path, and SHA-256 digest stay aligned.
