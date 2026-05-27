# Meltica

Meltica is a mono-repo that combines the Go gateway, the Next.js control-plane client, and the JavaScript strategy registry that previously lived in separate repositories.

## Repository Layout

- `backend/` — Go gateway and control-plane API.
- `frontend/` — Next.js operator UI.
- `strategy/` — versioned JavaScript strategy bundles plus `registry.json`.

## Start Here

- `backend/README.md` — gateway architecture, config, and Make targets.
- `frontend/README.md` — frontend setup, scripts, and deployment notes.
- `frontend/QUICKSTART.md` — short UI bring-up guide.
- `strategy/README.md` — strategy registry layout and publishing workflow.

## Local Development

1. Start the gateway.
   - `cd backend`
   - Copy `config/app.example.yaml` to `config/app.yaml`.
   - Set `DATABASE_URL`.
   - Point `strategies.directory` at `../strategy` if you want to use the bundled registry in this repo.
   - Run `make run`.
2. Start the frontend.
   - `cd frontend`
   - Run `pnpm install`.
   - Set `NEXT_PUBLIC_API_URL=http://localhost:8880`.
   - Run `pnpm dev`.
3. Manage strategy bundles in `strategy/`.
   - Update `registry.json` and run `node gc.js` when pruning unreferenced bundles.

## Repository History

This mono-repo consolidates content that previously lived in `coachpo/meltica`, `coachpo/meltica-client`, and `coachpo/meltica-gateway`.

## License

MIT; see `LICENSE`.
