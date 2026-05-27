# Repository Guidelines

## Role Of This Directory
- This tree stores versioned JavaScript strategy bundles for the gateway.
- `registry.json` is authoritative. The gateway reads bundles from the manifest, and the frontend only sees registry state through gateway APIs.
- Each bundle lives at `<name>/<64-char sha256>/<name>.js`.
- `gc.js` prunes JS files and empty directories that are not referenced by `registry.json`.
- This top-level file intentionally covers every strategy bundle subtree; do not create extra `AGENTS.md` files inside content-addressed strategy revision folders.

## Add Or Update A Strategy
1. Write or update the JS bundle so it exports `metadata` and `create(env)`.
2. Compute the digest with `shasum -a 256`.
3. Place the file at `<name>/<digest>/<name>.js`.
4. Update `registry.json` `tags` and `hashes` together.
5. Run `node gc.js`.
6. Reload or restart the gateway so the new registry state is discovered.

## Editing Rules
- The repo has no external users yet. Prefer one clean manifest format and one clean bundle layout; do not add legacy aliases, duplicate tag conventions, or compatibility loaders unless explicitly requested.
- Keep tag changes, hash changes, and file moves in the same change set.
- `latest` should point at the newest stable digest.
- If registry semantics change, update backend lambda/runtime expectations and frontend strategy-module flows in the same task.

## Must Not Do
- Do not rename hash directories without recalculating the digest.
- Do not rely on loose or unregistered files; the loader ignores them and `gc.js` may delete them.
- Do not edit bundle contents without updating registry references when the digest changes.

## Verification
```bash
shasum -a 256 path/to/strategy.js
node gc.js
```

## References
- `strategy/README.md` has the longer publishing flow.
- `backend/internal/app/lambda/AGENTS.md` covers the runtime side of registry and loader changes.
