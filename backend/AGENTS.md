# Backend Guidelines

## Scope and nearest guidance
This file applies to all of `backend/`. More specific AGENTS files now live under `cmd`, `db/migrations`, `internal/app`, `internal/domain`, `internal/domain/schema`, `internal/infra`, `internal/testutil`, and `tests/contract`; follow the nearest file when it adds local rules.

## Project structure
`cmd/gateway` starts the runtime and `cmd/migrate` runs database migrations. Orchestration belongs in `internal/app`, canonical types and store contracts belong in `internal/domain`, and adapters, config, persistence, HTTP, pools, and telemetry belong in `internal/infra`. Shared test helpers live in `internal/testutil`, migrations live in `db/migrations`, and cross package suites live in `tests/contract`.

## Local priority
The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths. Prefer clear package boundaries, direct contract cleanup, and small focused changes over aliases or fallback paths that preserve old behavior.

## Commands
- `make run CONFIG_FILE=config/app.yaml`: start the gateway locally.
- `make build` and `make build-linux-arm64`: compile binaries into `bin/`.
- `make lint`: run `golangci-lint` with `.golangci.yml`.
- `make test`: run `go test ./... -race -count=1 -timeout=30s`.
- `make coverage`: enforce the TS-01 coverage bar.
- `make migrate` and `make migrate-down`: apply or roll back `db/migrations` against `DATABASE_URL`.
- `make sqlc`: regenerate `internal/infra/persistence/postgres/sqlc` after SQL changes.

## Coding and tests
Target Go 1.25. Run `gofmt` and `goimports`, keep package names short, and prefer constructors over mutable globals. Keep `_test.go` files beside the code they cover, then use `tests/contract/` for cross package persistence or API behavior. Do not hand edit generated sqlc files.

## Secrets and config
Never commit credentials. Use `.env` for `DATABASE_URL` and provider secrets, keep `config/app.example.yaml` safe for sharing, and keep `config/app.ci.yaml` deterministic for CI.
