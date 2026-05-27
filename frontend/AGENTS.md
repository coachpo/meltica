# Repository Guidelines

## Project Structure & Module Organization
- `src/app` houses App Router routes (dashboard, instances, strategies, risk); default to server components and add `'use client'` only for interactive islands. `src/components` holds navigation, dialogs, and shadcn/ui wrappers—keep feature widgets in nested folders to keep imports flat.
- `src/lib` groups REST helpers (`api/`), React Query hooks, shared types, and utilities; when `docs/frontend-api.yaml` shifts, run `pnpm generate:api-types` to refresh `src/lib/api-types.ts` and review dependent hooks.
- `src/mocks` powers MSW stubs, `public/` stores static assets, and Playwright specs live in `tests/`; keep them aligned as workflows move.

## Build, Test, and Development Commands
- `pnpm install` – install dependencies pinned in `pnpm-lock.yaml`.
- `pnpm dev` – serve the Next.js dev build (http://localhost:3000) with Turbopack reloads.
- `pnpm build && pnpm start` – compile and run the production bundle.
- `pnpm lint [--fix]` – enforce the Next.js + Tailwind ESLint stack.
- `pnpm test` / `pnpm test:unit(:watch)` – execute the Vitest suite with MSW handlers.
- `pnpm test:e2e` – run Playwright specs in `tests/` against a live frontend (override host via `PLAYWRIGHT_BASE_URL`).

## Coding Style & Naming Conventions
- TypeScript + React 19 with 2-space indentation, single quotes, and trailing commas where valid.
- Export components/hooks in PascalCase but keep filenames kebab-cased (`strategy-modules-panel.tsx` → `StrategyModulesPanel`).
- Favor Tailwind utilities and shadcn/ui primitives; colocate React Query keys in `src/lib/react-query.ts` when introducing new hooks.

## Testing Guidelines
- Keep unit tests beside the code (`*.test.tsx` or `__tests__`) and stub control-plane traffic through `src/mocks/handlers.ts`.
- Cover new hooks, serializers, and forms with Vitest + Testing Library, asserting cache behavior, optimistic updates, and toast errors.
- Maintain scenario-focused Playwright specs (`tests/*.spec.ts[x]`) and update them, Zod validators, and regenerated API types whenever schemas change.

## Commit & Pull Request Guidelines
- Use short, imperative subjects with optional scopes, mirroring history (`docs: add Playwright manual testing log`, `original UI`); stay ≤72 characters.
- Reference related issues (`Fixes #123`) and list verification steps (`pnpm lint`, `pnpm test`, `pnpm test:e2e`) in the PR body.
- Attach screenshots or clips for UI work and note API/schema impacts plus whether types or mocks were refreshed.

## Security & Configuration Tips
- Keep secrets out of Git; load `NEXT_PUBLIC_API_URL` via `.env.local`, update `frontend-api.yaml` when endpoints change, rerun the generator, and align MSW handlers so mocks match production contracts.
