# App Router Guidance

This file adds local rules for `frontend/src/app`. Follow `frontend/AGENTS.md` first, then these notes.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Use Next.js App Router conventions. Keep `layout.tsx` and route shells server-side by default, and add `'use client'` only to pages or islands that need state, effects, browser APIs, TanStack Query hooks, or event handlers.

Route code should call gateway data through `@/lib/hooks` and `@/lib/api`. The frontend never reads strategy files directly.

Keep route-owned helpers beside the route, like `instances/spec-utils.ts`, unless they are reused outside `src/app`.
