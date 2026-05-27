# Shared Components Guidance

This file adds local rules for `frontend/src/components`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Put only cross-route UI here, such as navigation, confirmation dialogs, provider symbol picking, code surfaces, and app providers. Feature-only widgets should stay near their route.

Use shadcn and Radix wrappers from `components/ui` before writing styled primitives. Keep interactive components explicit with `'use client'`.
