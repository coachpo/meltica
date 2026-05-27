# Test Utility Guidelines

This child file adds only local rules for `internal/testutil`.

The repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Keep shared fixtures, fakes, and helpers deterministic and small. Helpers should make tests clearer without hiding important setup for providers, persistence, dispatcher, or lambda runtime behavior.
