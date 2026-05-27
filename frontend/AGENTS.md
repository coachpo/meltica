# Frontend Guidelines

## Scope and Nearest Guidance
This file applies to all of `frontend/`. More specific AGENTS files now live under maintained source subtrees in `src/app`, `src/components`, `src/lib`, `src/mocks`, and `tests`; follow the nearest file when it adds local rules.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

## Project Structure
`src/app` contains the Next.js App Router surface. `src/components` holds shared navigation, dialogs, code surfaces, providers, and shadcn/ui wrappers. `src/lib/api` is the REST boundary, `src/lib/hooks` wraps it with TanStack Query, and `src/mocks` mirrors the gateway API for tests.

`frontend-api.yaml` is the source for `src/lib/api-types.ts`; regenerate types whenever the contract changes. The frontend talks to the gateway API only and never reads strategy files directly.

## Commands
Use the frontend package scripts from this directory: `pnpm lint`, `pnpm test`, `pnpm test:e2e`, and `pnpm generate:api-types` when the touched area calls for them.

## Coding Style
TypeScript is strict, 2-space, single-quoted, and uses kebab-case filenames with PascalCase exports. Route code should go through `src/lib/api` and hooks rather than raw `fetch`. Favor Tailwind and shadcn/ui primitives, keep query keys centralized, and keep feature-only UI near its route.

## Testing
Keep unit tests beside code as `*.test.ts(x)` or under `__tests__/`. Keep Playwright scenarios in `tests/` with the existing `TC_###_*.test.ts` convention. API contract changes must update `frontend-api.yaml`, generated types, hooks, Zod validators, MSW handlers, and relevant Vitest or Playwright coverage together.

## Security and Configuration
Keep secrets out of Git and set `NEXT_PUBLIC_API_URL` via `.env.local`, Docker args, or runtime env. Don't hand-edit `src/lib/api-types.ts`; regenerate it.
