# Meltica Strategy Registry

This repository stores versioned JavaScript strategy bundles for the Meltica gateway. The gateway and UI read the manifest in `registry.json` to discover which bundles are available, which tag is `latest`, and where to load the corresponding file.

## Layout

- `<strategy>/<hash>/<strategy>.js` — strategy source file. `hash` is a **64-character SHA-256 digest of the JS file**.
- `registry.json` — manifest that maps strategy names to tags and hashes.
- `gc.js` — helper that prunes JS files and empty directories not referenced by `registry.json`.

## Strategy Module Contract

Each bundle should `module.exports` an object shaped like:

```js
module.exports = {
  metadata: {
    name: 'marketmaking',
    tag: '1.0.0',              // semantic tag stored in registry.json
    displayName: 'Market Making',
    description: 'Quotes bid/ask around mid price.',
    config: [ /* name, type, default, description */ ],
    events: ['Trade', 'Ticker', ...], // event types the strategy listens to
  },
  create: function (env) {
    // return handlers that use env.runtime helpers (submitOrder, providers, telemetry, etc.)
  },
};
```

Use the existing strategies (e.g., `marketmaking`, `momentum`, `meanreversion`) as working references.

## Add or Update a Strategy

1. Write your strategy JS file and ensure it exports `metadata` and `create(env)`.
2. Compute its digest: `shasum -a 256 path/to/strategy.js` → `<digest>`.
3. Place the file at `<name>/<digest>/<name>.js`.
4. Edit `registry.json`:
   - Add/refresh `tags` (e.g., `"1.1.0": "sha256:<digest>", "latest": "sha256:<digest>"`).
   - Add the `hashes` entry with the relative path `"<name>/<digest>/<name>.js"`.
5. Run garbage collection to drop unreferenced files: `node gc.js`.
6. Point the gateway `strategies.directory` config to this folder (or a copy) so it can load the bundles.

## Notes for Operators

- Tags in `registry.json` are how the control plane surfaces versions; `latest` should always point at the newest stable digest.
- Keep this repo under version control and promote tags via PRs to keep history auditable.
- After updating the registry, restart or reload the gateway so the new bundles are discovered.

## Current Catalog (from `registry.json`)

- delay `1.0.0` (latest)
- extension-listener `1.0.0` (latest)
- grid `1.0.0` (latest)
- logging `v1.0.1` (latest)
- marketmaking `1.0.0` (latest)
- meanreversion `1.0.0` (latest)
- momentum `1.0.0` (latest)
- noop `1.0.0` (latest)
