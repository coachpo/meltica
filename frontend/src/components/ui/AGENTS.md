# UI Primitive Guidance

This file adds local rules for `frontend/src/components/ui`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Treat these files as shadcn and Radix source wrappers. Keep primitive APIs close to upstream, avoid local compatibility props, and compose behavior in callers when possible.

Use semantic tokens, component variants, `cn()`, accessible titles for overlays, and Radix composition patterns. Don't add raw color systems or hand-rolled primitives here.
