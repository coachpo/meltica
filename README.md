# Meltica Monorepo

Meltica is a modular trading stack built from three pieces:

- **Gateway** (`meltica-gateway`): Go control-plane and execution engine that loads strategies, routes events, and exposes a REST API.
- **Client** (`meltica-client`): Next.js UI for operating the control plane—register strategy bundles, configure providers, start/stop instances, and watch telemetry.
- **Strategy Registry** (`meltica-strategy`): Versioned JavaScript strategy bundles plus a manifest (`registry.json`) the gateway consumes.

## Runtime Relationships

- The client speaks only to the gateway’s REST API (`NEXT_PUBLIC_API_URL`, default `http://localhost:8880`)—no direct file access.
- The gateway loads strategies from `strategies.directory`, expecting the `meltica-strategy` layout and `registry.json` as the single source of truth.
- Strategy uploads or tag changes initiated in the client are persisted by the gateway into `registry.json` and become visible to both the UI and runtime after refresh.

## Prerequisites

- Go **1.25+**
- Node.js **20+** and pnpm **10.20+**
- PostgreSQL reachable via `DATABASE_URL`
- Docker (optional) for containerized runs and telemetry stack

## Quickstart (local dev)

1. **Gateway**

```bash
cd meltica-gateway
cp config/app.example.yaml config/app.yaml   # edit DSN, telemetry, strategy dir
export DATABASE_URL=postgresql://postgres:root@localhost:5432/meltica?sslmode=disable
make migrate
make run                                     # serves control API on :8880 by default
```

Set `MELTICA_CONFIG_PATH` or `-config` if you keep `app.yaml` elsewhere.

2. **Client**

```bash
cd ../meltica-client
pnpm install
echo "NEXT_PUBLIC_API_URL=http://localhost:8880" > .env.local
pnpm dev                                      # http://localhost:3000
```

3. **Strategies**

- The gateway reads JS bundles from the directory configured at `strategies.directory` (defaults to `meltica-strategy`).
- `meltica-strategy/registry.json` maps strategy names → tagged hashes → bundle paths. Keep it in sync when adding strategies.
- From the client UI, register modules and launch instances after the gateway sees the registry.

## Repository Layout

- `meltica-gateway/` — Go control plane, migrations, API contracts, telemetry assets.
- `meltica-client/` — Next.js 14+ app router UI with Vitest/Playwright suites.
- `meltica-strategy/` — Versioned JS strategies and manifest; includes GC helper.

## Common Commands

- Gateway: `make run`, `make test`, `make migrate`, `make coverage`, `make lint`.
- Client: `pnpm dev`, `pnpm build && pnpm start`, `pnpm lint`, `pnpm test`, `pnpm test:e2e`.
- Strategies: update `registry.json`, place bundles under `<name>/<hash>/<name>.js`, run `node gc.js` to prune unregistered files.

## Notes on Strategy Workflow

- Each strategy exports `metadata` (name, tag, config schema, events) and `create(env)` that returns handlers using the gateway runtime helpers.
- Hashes in `registry.json` are SHA-256 digests of the JS file; tags (e.g., `1.0.0`, `latest`) point to a digest.
- Configure `strategies.directory` in the gateway to the absolute path of `meltica-strategy` (or copy/symlink it) so bundles are discoverable.

## Contributing

- Branch from `main`, keep PRs small, and include screenshots for UI changes.
- Run the relevant test suites: `make test` / `make lint` (gateway) and `pnpm lint` / `pnpm test` / `pnpm test:e2e` (client).

## License

MIT license files exist in each subproject. Use accordingly for client, gateway, and strategies.
