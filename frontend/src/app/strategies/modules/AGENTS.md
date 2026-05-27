# Strategy Modules Route Guidance

This file adds local rules for `frontend/src/app/strategies/modules`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

This route manages module registration through the gateway API. Never read `strategy/` files or registry manifests directly from the frontend.

Keep source editing, validation feedback, tag actions, usage checks, and template insertion aligned with the strategy-module hooks and Playwright cases `TC_006` through `TC_010`.
