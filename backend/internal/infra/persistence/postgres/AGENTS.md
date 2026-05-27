# Postgres Persistence Guidelines

This child file adds only local rules for `internal/infra/persistence/postgres`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep handwritten pgx repositories, numeric conversion, pool metrics, and transaction behavior here. Query shape changes belong with SQL, migrations, regenerated sqlc code, and tests that cover persisted store contracts.
