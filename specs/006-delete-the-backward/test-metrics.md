# Test Metrics Comparison

| Metric | Baseline | Post-Removal | Delta |
|--------|----------|--------------|-------|
| Test count (`go test ./... -v`) | 271 | 290 | +19 |
| Verbose test runtime (sum of `ok` timings) | 35.02s | 38.85s | +3.83s |
| Race-enabled runtime (`go test ./... -race -count=1`) | — | 7.57s | — |
| Overall coverage (`go test ./... -cover`) | 45.5% | 50.4% | +4.9pp |
| Core package coverage | — | 90.2% | — |

Overall coverage remains below the desired 75% threshold even with the expanded suite, although core package coverage now meets the 90% target. Significant additional authoring is required across large adapter packages to hit the policy limits.

## Observations

- Expanded the suite with registry, symbol guard, translator, config, and framework type coverage, boosting total test cases to 290.
- Race suite continues to pass without regression, completing in ~7.6s on the current hardware snapshot.
- Added coverage for the new `invalidTracker` helper plus broader core/config/framework surfaces, nudging overall coverage upward to 50.4% while bringing core modules to 90.2%.
- No parser imports remain under `tests/`, aligning documentation with the simplified processor/router scope.
- Logs and coverage artifacts: `post-tests.log`, `post-race-tests.log`, and `post.cover` archived alongside this spec for audit.
