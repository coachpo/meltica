# Contract Test Guidelines

This child file adds only local rules for `tests/contract`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Use this tree for cross package behavior such as Postgres persistence contracts and API integration flows. Keep unit tests beside implementation code, and make contract fixtures explicit about database or runtime assumptions.
