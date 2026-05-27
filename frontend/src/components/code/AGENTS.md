# Code Component Guidance

This file adds local rules for `frontend/src/components/code`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

`CodeEditor` and `CodeViewer` are client components. Keep browser-only editor logic contained here and expose small props to routes.

Preserve readable fallback behavior for tests and environments without enhanced editor features.
