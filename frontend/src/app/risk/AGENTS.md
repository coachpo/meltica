# Risk Route Guidance

This file adds local rules for `frontend/src/app/risk`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Risk edits should stay explicit and operator-readable. Keep validation close to the form and send only gateway-supported risk fields through the hook layer.

When risk schema changes, update API schemas, hooks, MSW, and `TC_013` or related Playwright coverage in the same change.
