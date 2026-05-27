## API Change Brief – Docker-Style Strategy Module Listing

### 1. Overview

The `/strategies/modules` family of endpoints will behave like `docker images`: they expose registry metadata only. Runtime usage data (instances, counts, selectors) is no longer embedded in listing/detail payloads. Operators now consult `/strategies/modules/{selector}/usage` when they need live usage information.

### 2. Breaking Changes

1. **`running` array removed** – `StrategyModuleSummary` no longer includes the `running` field. The OpenAPI schema drops `running` from `StrategyModuleSummary` and removes the nested `ModuleRunningSummary` references from listing responses.
2. **`runningOnly` query deprecated** – `GET /strategies/modules?runningOnly=true` previously filtered by active instances; the parameter is removed. Requests that include it return `400 invalid query` to catch stale clients.
3. **Delete/tag errors stay generic** – Since listings no longer include usage, clients should call `/strategies/modules/{selector}/usage` themselves before destructive operations when they need runtime pin details. The current handlers return 404 for missing selectors and 400 for other strategy-module errors.
4. **UI/CLI impact** – Dashboards that previously relied on inline `running` data must issue explicit usage calls before showing “active” badges.

### 3. Endpoint Details

| Endpoint | Change |
| --- | --- |
| `GET /strategies/modules` | Response items omit `running`. Query parameters drop `runningOnly`. Documentation updated to describe purely registry data (name, file, tags, revisions, metadata). |
| `GET /strategies/modules/{selector}` | Same contract as listing: `running` removed from the summary. Clients fetch `/strategies/modules/{selector}/usage` for instance details. |
| `GET /strategies/modules/{selector}/usage` | Unchanged schema. Clients must rely on this endpoint for runtime insights. |
| `PUT/DELETE /strategies/modules/{name}/tags/{tag}` | Clients should call `/strategies/modules/{selector}/usage` themselves when they need runtime pin details before tag changes. Current handlers return 404 for missing selectors and 400 for other strategy-module errors. |
| `DELETE /strategies/modules/{selector}` | Same pattern as tag routes: query usage explicitly before deletes when you need to understand which revisions are active. |

### 4. Migration Guidance

1. **Client updates**
   - Remove any parsing logic for `module.running` in listing/detail responses.
   - Stop passing `runningOnly`; expect HTTP 400 if still provided.
   - When you need to display running status, issue `GET /strategies/modules/{selector}/usage` (optionally cached).
2. **UI changes**
   - Replace inline “running instances” badges with either: (a) lazy-loaded usage overlays, or (b) state derived from dedicated usage calls.
   - Update confirmation dialogs for destructive actions to fetch usage proactively when operators need to understand which revisions are active.
3. **Automation/CLI**
   - Scripts that filtered modules by `runningOnly` should switch to `GET /strategies/modules/{selector}/usage` and inspect the returned `total` count.

### 5. Error Handling

When operations fail due to active instances, the current handlers still return generic strategy-module errors (404 for missing selectors, 400 for other failures). Clients that need richer context should fetch `/strategies/modules/{selector}/usage` before retrying destructive actions.

### 6. Timeline & Validation

- Update OpenAPI (`frontend-api.yaml`) and regen clients **before** deploying the server change.
- Ensure automated tests assert absence of `running` in listing responses and continued availability of the dedicated `/strategies/modules/{selector}/usage` endpoint.
- Coordinate UI/CLI releases so they no longer depend on the removed fields before enabling the server flag.

With these changes, the strategy-module manager now mirrors Docker’s “images vs. containers” split: listings enumerate stored revisions only, while runtime usage is queried explicitly when needed.
ag.

With these changes, the strategy-module manager now mirrors Docker’s “images vs. containers” split: listings enumerate stored revisions only, while runtime usage is queried explicitly when needed.
