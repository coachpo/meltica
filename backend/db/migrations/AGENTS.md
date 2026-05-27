# Migration Guidelines

This child file adds only local rules for `db/migrations`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Pair every schema change with matching `.up.sql` and `.down.sql` files using the next numbered prefix. Keep `embed.go` as the migration embed point and update persistence queries or sqlc output in the same change when table shape changes.
