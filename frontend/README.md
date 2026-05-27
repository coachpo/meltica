# Meltica Client

Frontend for the Meltica control plane. It gives operators one place to browse strategy catalogs, register JS strategy modules, configure providers/adapters, launch and monitor instances, tune risk limits, and back up or restore the runtime snapshot. All actions flow through the **meltica-gateway** REST API at `NEXT_PUBLIC_API_URL`, and the strategy catalog you see is whatever the gateway exposes from the **meltica-strategy** `registry.json` manifest—no direct file access from the UI.

This frontend now lives in `frontend/` inside the combined Meltica mono-repo.

## At a Glance

- **App Router + React 19** with Tailwind v4 and shadcn/ui primitives.
- **API alignment** via `frontend-api.yaml` → `pnpm generate:api-types`.
- **Data layer** backed by TanStack Query v5 and REST helpers in `src/lib/api`.
- **Testing** with Vitest + MSW for units/hooks and Playwright for E2E.
- **Docker-first** build optimized for Next.js standalone output.

## Requirements

- Node.js ≥ 20
- pnpm ≥ 10.20.0 (Corepack is enabled in the Dockerfile and CI)
- Meltica gateway reachable at `http://localhost:8880` (default)

## Quick Start

From the monorepo root:

```bash
cd frontend
pnpm install
echo "NEXT_PUBLIC_API_URL=http://localhost:8880" > .env.local   # optional override
pnpm dev
# visit http://localhost:3000
```

Key routes while exploring:

- `/strategies/modules` to register or edit strategy modules.
- `/instances` to create/start/stop/delete instances.
- `/providers` to manage exchange/data providers and see adapter capabilities.
- `/risk` to set global guardrails.

## Environment

| Name                  | Required | Default                 | Purpose                                                                           |
| --------------------- | -------- | ----------------------- | --------------------------------------------------------------------------------- |
| `NEXT_PUBLIC_API_URL` | Optional | `http://localhost:8880` | Base URL for all REST calls (set in `.env.local`, Docker args, or container env). |

`vitest.setup.ts` seeds a fallback `NEXT_PUBLIC_API_URL` so tests run without a local gateway.

## Scripts

- `pnpm dev` – Next.js dev server (Turbopack hot reload).
- `pnpm build` / `pnpm start` – Production bundle and runner.
- `pnpm lint` – ESLint (Next.js + Tailwind stack).
- `pnpm test` / `pnpm test:unit(:watch)` – Vitest suites with MSW (`src/mocks/handlers.ts`).
- `pnpm test:e2e` – Playwright specs in `tests/` (`PLAYWRIGHT_BASE_URL` defaults to `http://localhost:3000`).
- `pnpm generate:api-types` – Regenerate `src/lib/api-types.ts` from `frontend-api.yaml`.

## Project Structure

```
src/
├─ app/         # App Router routes: dashboard, strategies/modules, providers/adapters, instances, risk, backups
├─ components/  # Navigation, dialogs, shadcn/ui wrappers (keep feature widgets near their pages)
├─ lib/         # REST helpers, React Query hooks, shared types/utilities
├─ mocks/       # MSW handlers powering Vitest + Playwright
tests/          # Playwright end-to-end specs and helpers
docs/           # Onboarding, theme, and migration notes
```

## Testing

```bash
# Unit & hook tests
pnpm test

# Watch mode
pnpm test:unit:watch

# End-to-end
pnpm dev &
PLAYWRIGHT_BASE_URL=http://localhost:3000 pnpm test:e2e
```

Playwright suites can also run against another host by overriding `PLAYWRIGHT_BASE_URL`. MSW is enabled for isolated UI testing.

## Docker

Build and run locally from `frontend/`:

```bash
docker build -t meltica-frontend:local \
  --build-arg NEXT_PUBLIC_API_URL=https://gateway.example.com .

docker run -p 3000:3000 \
  -e NEXT_PUBLIC_API_URL=https://gateway.example.com \
  meltica-frontend:local
```

GHCR images are published via `.github/workflows/docker-publish.yml` (multi-arch AMD64/ARM64). Daily cleanup lives in `.github/workflows/cleanup.yml`.

## Deployment (without Docker)

```bash
pnpm install
NEXT_PUBLIC_API_URL=https://gateway.production pnpm build
PORT=3000 pnpm start
```

Ensure the gateway is reachable from the host and that CORS/HTTPS rules permit browser access.

## Contributing

1. Branch from `main`.
2. Keep PRs focused; attach screenshots for UI changes; mention if API types or MSW handlers were regenerated.
3. Run `pnpm lint`, `pnpm test`, and when relevant `pnpm test:e2e` before opening the PR.

## License

MIT License; see the root `../LICENSE`.
