## API Change Brief – Minimal Strategy Module Upload Payload

### 1. Summary

`POST /strategies/modules` (and `PUT /strategies/modules/{selector}`) will now accept only the raw JavaScript source. All descriptive metadata—`name`, `tag`, `displayName`, `description`, etc.—must come from the strategy module’s own `module.exports.metadata` block (e.g., `logging.js`). The server ignores external hints and uses the compiled metadata exclusively.

### 2. Breaking Changes

1. **Request body schema** – `StrategyModulePayload` drops the following fields:
   - `filename`
   - `name`
   - `tag`
   - `aliases`
   - `reassignTags`
   - `promoteLatest`
   Only `source` remains (string, required).
2. **Tag helpers** – Uploads no longer accept inline tag fields. The current server auto-promotes `latest`, while other aliases are managed through `PUT /strategies/modules/{name}/tags/{tag}`.
3. **Validation** – If the uploaded JS is missing `metadata.name` or `metadata.tag`, the upload fails with the existing validation errors. Clients can no longer compensate via request fields.

### 3. Client Impact & Migration

| Client behavior | Action required |
| --- | --- |
| Upload forms supplying filename/name/tag | Remove those fields. Send `{ "source": "<full JS text>" }`. |
| Workflows that reassigned tags during upload | After a successful upload, call the tag endpoints for extra aliases such as `prod`; the current server already auto-promotes `latest`. |
| CI/CD pipelines setting `promoteLatest` | Remove the request field. Current uploads and updates auto-promote `latest`; use tag endpoints only for additional aliases. |

### 4. Error Handling

- Missing metadata results in HTTP `422` with the existing diagnostics array (compile/metadata errors). There is no fallback to request data.
- If clients still send the removed fields, the current handler ignores them and reads only `source`. Remove those fields from callers anyway so the request shape matches the documented contract.

### 5. Rollout Checklist

1. Update OpenAPI (`frontend-api.yaml`) to redefine `StrategyModulePayload` (`source` only) and remove references to the deleted fields.
2. Regenerate client libraries.
3. Update dashboards/CLI to drop the extra inputs and, if necessary, add post-upload tag reassignment flows for aliases other than `latest`.
4. Communicate to strategy authors that `module.exports.metadata` is now the single source of truth.

With these changes, the upload API mirrors the metadata embedded in the strategy files, reducing duplication and keeping registry entries consistent with the code. 
