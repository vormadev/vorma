# vormaclient/vite Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `vormaclient/vite`

## Scope

Package-owned Vite plugin contracts in `vormaclient/vite/**`.

This package provides the Vorma Vite plugin that configures build/dev behavior,
filemap-backed asset replacement, and dev invalidation wiring.

Current evidence note:

- Requirements are currently source-backed in `vormaclient/vite/**`.
- No active legacy tests outside `conformance/**` were found for this package in
  the repo.

## Requirements

- `VORMACLIENT-VITE-001` Plugin factory/config shape contract.
  Default export MUST be a plugin factory accepting config with
  `rollupInput`, `publicPathPrefix`, `staticPublicAssetMap`,
  `buildtimePublicURLFuncName`, `filemapJSONPath`, `ignoredPatterns`, and
  `dedupeList` fields.
- `VORMACLIENT-VITE-002` Dev filemap read-and-cache contract.
  In dev (`command === "serve"`) the plugin MUST resolve `filemapJSONPath` from
  `process.cwd()`, cache parsed JSON by file mtime, and fall back to
  `staticPublicAssetMap` when file stat/read/parse fails.
- `VORMACLIENT-VITE-003` Build/base config contract.
  Plugin `config(...)` MUST set base path to `/` in dev and
  `publicPathPrefix` otherwise; set build target to `es2022`, keep
  `emptyOutDir: false`, disable module-preload polyfill by default, merge
  configured rollup input with existing array-form input, and force Vorma output
  filename patterns (`vorma_out_vite_*`).
- `VORMACLIENT-VITE-004` Dev server headers/watch contract.
  Plugin `config(...)` MUST enforce `cache-control: no-store` in dev server
  headers and append `ignoredPatterns` to existing array-form watch ignored
  patterns.
- `VORMACLIENT-VITE-005` Resolve dedupe-merge contract.
  Plugin `config(...)` MUST append `dedupeList` to existing array-form
  `resolve.dedupe` values.
- `VORMACLIENT-VITE-006` Filemap invalidation endpoint contract.
  `configureServer(...)` MUST handle `"/__vorma_invalidate_filemap"` by clearing
  filemap cache, invalidating all modules in module graph, emitting full reload
  over Vite WS, and returning HTTP 200 with body `ok`.
- `VORMACLIENT-VITE-007` Transform gating contract.
  `transform(...)` MUST no-op for `node_modules` IDs and for files with no
  buildtime public-url calls matching configured function-name regex.
- `VORMACLIENT-VITE-008` Buildtime URL replacement contract.
  `transform(...)` MUST replace calls to configured buildtime public-url function
  with string literal URLs using filemap lookups and `publicPathPrefix`; when
  lookup misses, original asset path string MUST be kept.
- `VORMACLIENT-VITE-009` TypeScript config contract.
  `tsconfig.json` MUST extend repo base config.
