# JavaScript Lambda Loader Guidelines

This child file adds only local rules for `internal/app/lambda/js`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep registry parsing, tag and hash resolution, goja compilation, diagnostics, and instance execution isolated here. Preserve deterministic module hashes and clear loader errors because runtime refresh and HTTP module APIs depend on them.
