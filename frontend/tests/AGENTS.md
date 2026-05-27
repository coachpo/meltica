# Playwright Guidance

This file adds local rules for `frontend/tests`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep end-to-end scenarios in the existing `TC_###_description.test.ts` naming style, with test titles that repeat the TC number and operator behavior.

Use `BASE_URL` from `test-helpers.ts` unless a spec intentionally targets another host. Prefer visible user flows and stable role or text locators over implementation details.
