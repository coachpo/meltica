# Risk Guidelines

This child file adds only local rules for `internal/app/risk`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep risk checks deterministic and side effect free unless the caller explicitly performs the action. Test limit decisions with decimal inputs and make failure reasons useful to lambda runtime and HTTP callers.
