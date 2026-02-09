# vormabuild Conformance Specification

Status: Active  
Last Updated: 2026-02-09  
Applies To: vormabuild package behavior observable via CLI, filesystem outputs, and dev callbacks

## 1. Why This Spec Exists

This is a black-box conformance spec for build/dev behavior.

It is intended to:

- make refactors safe without implementation lock-in,
- drive artifact- and workflow-level tests,
- define observable contracts for build modes, generated outputs, and dev reload flows.

## 2. Conformance Boundaries

Conformance tests for this spec MUST rely on observable outcomes:

- CLI exit behavior,
- generated artifacts and file names,
- generated file content shape,
- dev callback endpoint effects,
- Vite plugin observable behavior.

Tests MUST NOT assert package-private implementation details.

## 2.1 Inherited Wave Contract Policy

Wave-owned build/dev control-plane semantics are canonicalized in:

- `specs/packages/wave/tooling/SPEC.md`
- `specs/packages/wave/tooling/TRACEABILITY_MATRIX.md`
- `specs/packages/wave/tooling/CONFORMANCE_ISSUES.md`

For this Vorma spec:

- Vorma-owned build requirements are defined here.
- Wave-owned requirement prose is intentionally omitted (no duplication).
- When Vorma behavior depends on Wave-owned contracts, reference Wave owner IDs
  at integration boundaries.

## 3. Build Mode Matrix

The Vorma build entrypoint supports flags:

- `--dev`
- `--hook`
- `--no-binary`

## 4. Requirement Catalog

## 4.1 CLI and Mode Semantics

### BUILD-CLI-001: Hook Mode Runs Build Inner

Given Vorma build is invoked with `--hook`  
When command executes  
Then it MUST run framework build-inner logic and exit (non-dev hook continues to post-Vite phase).

### BUILD-CLI-002: Dev Hook Skips Post-Vite Phase

Given Vorma build is invoked with `--hook --dev`  
When command executes  
Then it MUST run build-inner dev logic and MUST NOT run Vite production build/post-processing.

### BUILD-CLI-003: Prod Hook Runs Post-Vite Phase

Given Vorma build is invoked with `--hook` and no `--dev`  
When command executes successfully  
Then it MUST run Vite production build and post-Vite stage-two processing.

### BUILD-CLI-004: Dev Mode Starts Dev Build Flow

Given Vorma build is invoked with `--dev` and without `--hook`  
When command executes  
Then it MUST enter dev-server build flow (long-running workflow).

### BUILD-CLI-005: Prod Mode Build

Given Vorma build is invoked without `--dev` and without `--hook`  
When command executes  
Then it MUST run production build flow through Wave builder.

### BUILD-CLI-006: No-Binary Option

Given production build mode with `--no-binary`  
When command executes  
Then final Go binary compilation MUST be skipped.

Wave-owned CLI helper contracts (`BuildWaveWithHook` / `BuildWave`) are
canonicalized in `specs/packages/wave/tooling/SPEC.md`
(`WAVE-CLI-001` through `WAVE-CLI-003`) and are intentionally not duplicated in
this Vorma-owned spec.

## 4.2 Framework Hook Injection

### BUILD-HOOK-001: Framework Build Hook Command Shape

Given valid Vorma config  
When framework hooks are injected  
Then dev hook command MUST be:

`go run ./<MainBuildEntry> --dev --hook`

and prod hook command MUST be:

`go run ./<MainBuildEntry> --hook`.

### BUILD-HOOK-002: Hook Ordering Compatibility

Given Wave build flow with both user hooks and framework hooks  
When hooks run  
Then user hooks MUST run before framework hooks (Wave contract compatibility).

## 4.3 Default Watch Pattern Injection

### BUILD-WATCH-001: IncludeDefaults Gate

Given `Vorma.IncludeDefaults=false`  
When build config is prepared  
Then Vorma MUST NOT inject default framework watch patterns.

### BUILD-WATCH-002: Route Definitions Fast-Path Watch

Given defaults are enabled and `ClientRouteDefsFile` is configured  
When watch patterns are injected  
Then route defs file MUST be watched with:

- run-on-change-only callback flow, and
- rebuilding-notification suppression (`skipRebuildingNotification=true`).

### BUILD-WATCH-003: Route Change Callback Success Path

Given route defs file changes in dev and app is running  
When callback succeeds  
Then callback MUST:

1. rebuild route artifacts via fast route rebuild,  
2. call `GET /__vorma/reload-routes`,  
3. request browser reload waiting for app and Vite readiness.

Given app is already stopped for the active batch  
When route change callback runs  
Then callback MUST skip reload-endpoint calls and MUST let batch restart handle
state refresh.

### BUILD-WATCH-004: Route Change Callback Fallback

Given route reload endpoint call fails  
When route callback handles failure  
Then callback MUST request app restart without Go recompilation.

### BUILD-WATCH-005: Template Change Callback

Given defaults enabled and HTML template file changes in dev  
When callback succeeds  
Then callback MUST call `GET /__vorma/reload-template` and request browser reload waiting for app/Vite.

Given app is already stopped for the active batch  
When template callback runs  
Then callback MUST skip reload-endpoint calls and MUST let batch restart handle
state refresh.

Template-watch injection precondition:

- template watch entry MUST be injected only when both
  `HTMLTemplateLocation` and private-static-dir context are available.

### BUILD-WATCH-006: Template Callback Fallback

Given template reload endpoint call fails  
When callback handles failure  
Then callback MUST request app restart without Go recompilation.

### BUILD-WATCH-007: Go File Watch Hook

Given defaults enabled  
When watch patterns are injected  
Then `**/*.go` MUST be watched with `DevBuildHook` at concurrent timing.

### BUILD-WATCH-008: Generated Output Ignore Patterns

Given defaults enabled and `TSGenOutDir` configured  
When ignored patterns are injected  
Then generated files in that directory MUST include ignores for:

- `index.ts`
- `filemap.ts`
- `filemap.json`.

### BUILD-WATCH-012: Reload Endpoint Call Failure Classification

Given route/template watch callback attempts reload endpoint call  
When endpoint call times out, request transport fails, or endpoint response
status is non-`200`  
Then callback MUST treat endpoint call as failed and MUST execute documented
restart fallback behavior (`BUILD-WATCH-004` / `BUILD-WATCH-006`).

Current endpoint call contract uses `GET` with bounded timeout budget
(approximately 10 seconds) and requires status `200` for success.

## 4.4 Route Definition Parsing Contract

### BUILD-ROUTE-001: Route DSL Source

Given `ClientRouteDefsFile` path  
When parsing runs  
Then parser MUST discover `route(...)` calls imported from `vorma/client`.

Route-import discovery MUST support direct import and aliased binding forms from
the same module source.

### BUILD-ROUTE-002: Statically Unresolvable Modules

Given route module expression is not statically resolvable  
When parsing runs  
Then route MUST be ignored and warning SHOULD be emitted.

Conformance scope for "statically resolvable":

- static string literals MUST be accepted,
- const-string indirections MUST be accepted,
- `import("...")` with static literal MUST be accepted,
- dynamic expressions/function-call module args MUST be treated as unresolved
  and ignored with warning (build continues).
- unresolved diagnostics SHOULD identify reason class (for example unknown
  variable reference, non-static `import(...)`, function-call expression, or
  unsupported expression shape).

### BUILD-ROUTE-003: Missing Module Path

Given a resolved route module path does not exist  
When parsing runs  
Then build MUST fail with module-not-found error.

### BUILD-ROUTE-004: Key Defaults

Given route call omits export key  
When parsing runs  
Then export key MUST default to `"default"`.

### BUILD-ROUTE-005: Route Signature Validity

Given `route(...)` definitions in `ClientRouteDefsFile`  
When parsing runs  
Then parser MUST enforce valid route signature shape:

- argument 1 (pattern) MUST be a non-empty static string literal,
- argument 2 (module) MUST be present and statically resolvable to a module
  file path.

Given route signature is invalid (for example non-string pattern, omitted module
argument, or unresolved module argument expression)  
When parsing runs  
Then build MUST fail with actionable route-definition diagnostics and MUST NOT
silently accept or reinterpret the route.

### BUILD-ROUTE-006: Duplicate Pattern Collision Resolution

Given multiple resolved `route(...)` definitions produce the same normalized
pattern string `P`  
When parsed routes are materialized into runtime route map  
Then final route entry for `P` MUST equal the last discovered definition for `P`
in parser traversal order, and earlier conflicting definitions for `P` MUST be
overwritten.

Current contract does not require duplicate-pattern diagnostics; collision
visibility policy is tracked separately in conformance issues.

### BUILD-ROUTE-007: Unresolved-Route Warning Payload Includes Pattern and Expression Context

Given a `route(...)` definition is ignored because module argument cannot be
statically resolved  
When parser emits unresolved-route diagnostics  
Then diagnostics SHOULD include:

- route pattern context,
- source-file context,
- raw unresolved module-expression context,
- reason-class context.

Given unresolved route diagnostics are emitted  
When parsing/build continues  
Then unresolved routes MUST be excluded from emitted route map while all
resolvable routes continue normally.

### BUILD-ROUTE-008: Route-Definitions Transform/Syntax Errors Are Build-Fatal

Given route-definitions source cannot be transformed/parsed by the route-extract
pipeline (for example invalid TS/JS syntax in `ClientRouteDefsFile`)  
When route parsing runs  
Then build MUST fail and MUST surface transform diagnostics.

### BUILD-ROUTE-009: Resolved Route Module Path Normalization Contract

Given parsed route module path resolves relative to route-definitions file
location  
When route metadata is materialized  
Then stored `SrcPath` MUST be normalized to workspace-root-relative slash path
format for cross-platform-stable artifact output.

## 4.5 Fast Route Rebuild Contract (Dev)

### BUILD-FAST-001: Dev-Only Guard

Given fast route rebuild is invoked outside dev mode  
When function executes  
Then it MUST fail.

### BUILD-FAST-002: Fast Rebuild Output

Given fast route rebuild in dev mode  
When it succeeds  
Then it MUST:

- assign new dev-fast build ID (`dev_fast_*`),
- resync route registry,
- clean old route manifests,
- rewrite route artifacts (manifest + stage1 paths + generated TS).

Manifest cleanup in fast path MUST remove only files with Vorma route-manifest
prefix and MUST NOT clear unrelated public assets.

Given static-public output directory is missing  
When fast-path manifest cleanup runs  
Then fast rebuild SHOULD continue without failing solely due to missing
directory.

## 4.6 Artifact Generation Contract

### BUILD-ART-001: Stage 1 Paths File

Given successful build-inner artifact write  
When stage 1 paths file is written  
Then file MUST exist at:

`<static-private-out>/vorma_out/vorma_paths_stage_1.json`

with `stage` value `"one"`.

Stage-one manifest reference refinement:

- stage-one payload `routeManifestFile` MUST reference the route-manifest
  filename generated in the same artifact-write pass.

### BUILD-ART-002: Stage 2 Paths File

Given successful prod post-Vite phase  
When stage 2 paths file is written  
Then file MUST exist at:

`<static-private-out>/vorma_out/vorma_paths_stage_2.json`

with `stage` value `"two"`.

### BUILD-ART-003: Route Manifest

Given route artifacts are written  
When route manifest file is generated  
Then:

- filename MUST start with `vorma_out_vorma_internal_route_manifest_`,
- filename MUST end with `.json`,
- file content MUST map pattern -> `0|1` for server-loader presence.

Filename derivation refinement:

- filename suffix payload between prefix and `.json` MUST equal base64url(raw)
  encoding of first 8 bytes of SHA-256 over serialized manifest JSON bytes.

### BUILD-ART-004: Generated TS Entry File

Given route artifacts are written  
When TS generation completes  
Then generated TS file MUST be written to:

`<TSGenOutDir>/index.ts`.

Generated-content invariants:

- file MUST define `vormaAppConfig` using current router mount root and dynamic/splat/index rune settings,
- generated collection MUST be deterministic (loader entries sorted by pattern;
  action entries sorted by pattern then method),
- loader-category param/splat typing (including client-defined loader paths
  without Go loader handlers) MUST use loader matcher rune settings
  (`loadersDynamicRune`, `loadersSplatRune`),
- query/mutation param/splat typing MUST use action matcher rune settings
  (`actionsDynamicRune`, `actionsSplatRune`),
- action category mapping MUST treat `GET`/`HEAD` as query and
  `POST`/`PUT`/`PATCH`/`DELETE` as mutation (unsupported methods excluded),
- non-POST mutation entries MUST encode explicit method metadata.

### BUILD-ART-005: Filemap Outputs for Vite/TS

Given build-inner writes filemap outputs  
When operation succeeds  
Then files MUST exist in `<TSGenOutDir>`:

- `filemap.ts`
- `filemap.json`.

### BUILD-ART-006: Deterministic Write Skipping

Given generated TS content is unchanged  
When generation runs  
Then writer SHOULD skip rewriting the file.

### BUILD-ART-007: Dev Build ID Prefix Semantics

Given standard dev build-inner flow runs (non-fast-path)  
When a new build id is assigned for that run  
Then dev build id MUST use `dev_` prefix.

Given fast route rebuild flow runs in dev  
When a new build id is assigned for that run  
Then fast-path build id MUST use `dev_fast_` prefix.

### BUILD-ART-008: Generated RootData Type Derivation Contract

Given TS generation runs with a loader-handler registered at effective root-data
pattern  
When generated `index.ts` type surface is inspected  
Then generated `VormaRootData` alias MUST resolve to that root loader output
type and `VormaApp.rootData` MUST reference that alias.

Given TS generation runs without an effective root-data loader handler  
When generated `index.ts` is inspected  
Then generated `VormaRootData` alias MUST be `null`.

### BUILD-ART-009: Extra TypeScript Code Passthrough Contract

Given Vorma app config includes non-empty extra TypeScript code payload  
When generated `index.ts` is emitted  
Then extra code MUST be appended into generated output (preserved as caller
provided text, after core generated config/type prelude).

## 4.7 Production Stage-Two Semantics

### BUILD-STAGE2-001: Client Entry Output Resolution

Given valid Vite manifest and configured client entry  
When stage-two conversion runs  
Then it MUST compute and write `clientEntryOut` and `clientEntryDeps`.

`clientEntryDeps` MUST exclude the client entry chunk itself (deps list without
self-reference).

### BUILD-STAGE2-002: Route Output Resolution

Given route source paths and Vite manifest chunks  
When stage-two conversion runs  
Then route entries MUST be updated with:

- `outPath` (chunk filename basename),
- `deps` (dependency list from Vite graph traversal).

### BUILD-STAGE2-003: CSS Bundle Mapping

Given Vite manifest with CSS chunk metadata  
When stage-two conversion runs  
Then `depToCSSBundleMap` MUST map JS chunk basenames to CSS basenames.

### BUILD-STAGE2-004: Production Build ID Determinism Inputs

Given stage-two conversion  
When production build ID is computed  
Then hash inputs MUST include:

- HTML template content hash,
- stage-two paths JSON hash,
- summary hash of public output FS.

Computation refinement:

- public-output summary hash MUST be derived from walked file entries using
  path+size tuples,
- summary walk input MUST exclude root marker `"."` and directory entries,
- tuple encoding MUST use textual form `<relative-path>|<size-bytes>` for each
  included file entry,
- final build id MUST be base64url(raw) of first 16 bytes of combined SHA-256
  digest over the three input hashes.

### BUILD-STAGE2-005: Stage-Two BuildID and Route-Manifest Propagation Contract

Given stage-two conversion succeeds  
When stage-two paths payload and runtime state are observed  
Then:

- computed production build ID MUST be written into stage-two paths payload
  `buildID`,
- runtime `GetBuildID()` value MUST be updated to that same computed build ID,
- stage-two payload `routeManifestFile` MUST equal active route-manifest
  reference at conversion time.

## 4.8 Vite Integration Contract

### BUILD-VITE-001: Rollup Input Coverage

Given generated Vorma Vite config  
When rollup inputs are emitted  
Then inputs MUST include configured client entry plus all route source entry paths.

Rollup-input determinism refinement:

- rollup input list MUST be de-duplicated by path value,
- rollup input list MUST be emitted in deterministic sorted order.

### BUILD-VITE-002: Variant Dedupe Lists

Given `UIVariant` config  
When generated Vorma Vite config is emitted  
Then dedupe list MUST match selected variant family (React/Preact/Solid).

### BUILD-VITE-003: Vite Output Naming Prefix

Given Vite plugin is active  
When production bundles are emitted  
Then asset/chunk/entry filenames MUST use prefix `vorma_out_vite_`.

### BUILD-VITE-004: Buildtime Public URL Rewriting

Given plugin transform sees calls to configured buildtime public URL function  
When filemap contains target asset  
Then transform MUST rewrite call result to prefixed hashed URL string.

Given filemap does not contain target asset  
When transform runs  
Then asset literal MUST remain unchanged (non-hashed fallback).

Given source id is inside `node_modules` or source contains no matching calls  
When transform runs  
Then transform MUST leave module unchanged.

### BUILD-VITE-005: Filemap Invalidation Endpoint

Given Vite dev server plugin is active  
When `/__vorma_invalidate_filemap` is requested (regardless of HTTP method)  
Then endpoint MUST respond `200 ok`, clear plugin filemap cache, invalidate all
known modules in module graph, and trigger full reload.

Given request URL does not exactly equal `/__vorma_invalidate_filemap`  
When middleware runs  
Then plugin MUST delegate to next middleware without invalidation side effects.

### BUILD-VITE-006: Vite Config Merge and Mode Contract

Given plugin config hook runs with incoming Vite config  
When command is `serve`  
Then plugin output MUST set:

- `base` to `/`,
- server header `cache-control: no-store` while preserving other user headers,
- watch ignored list as user ignored entries plus plugin ignored patterns,
- resolve dedupe list as user dedupe entries plus plugin dedupe entries.

Given command is non-serve build mode  
When plugin output config is generated  
Then `base` MUST be `publicPathPrefix`.

Shared config merge requirements:

- build target MUST be `es2022`,
- `emptyOutDir` MUST remain `false`,
- `modulePreload.polyfill` MUST be `false` while preserving user object fields,
- rollup input MUST append plugin rollup inputs before existing array-form user
  rollup inputs,
- `preserveEntrySignatures` MUST be `exports-only`.

### BUILD-VITE-007: Dev Filemap Read Cache Contract

Given plugin runs in dev mode and transform needs filemap lookup  
When configured JSON file mtime is unchanged  
Then cached parsed map MUST be reused.

Given mtime changes  
When transform needs filemap lookup  
Then JSON map MUST be re-read and reparsed.

Given JSON file cannot be read or parsed  
When lookup runs  
Then plugin MUST fall back to static map from plugin config.

### BUILD-VITE-008: Generated Vite Helper Module Surface Contract

Given Vorma-generated `index.ts` Vite-helper section is emitted  
When helper-module exports are inspected  
Then generated output MUST include:

- exported `staticPublicAssetMap` passthrough binding from `./filemap`,
- exported `publicPathPrefix` constant,
- exported runtime URL helper `waveRuntimeURL(...)` that maps through
  `staticPublicAssetMap` with miss-fallback to original key and prefixes result
  with `publicPathPrefix`,
- exported `vormaViteConfig` object containing
  `rollupInput`, `publicPathPrefix`, `staticPublicAssetMap`,
  `buildtimePublicURLFuncName`, `filemapJSONPath`, `ignoredPatterns`, and
  `dedupeList`,
- global declaration for configured buildtime public URL function accepting
  `StaticPublicAsset` key type.

### BUILD-VITE-009: Generated Vite Ignore-Baseline Contract

Given Vorma-generated Vite helper config is emitted  
When `ignoredPatterns` baseline is inspected  
Then baseline MUST include patterns for:

- Go sources (`**/*.go`),
- dist directory subtree under configured dist root,
- private static source subtree,
- active config file path,
- generated TS output subtree (`TSGenOutDir`),
- client route-definitions source file path.

### BUILD-VITE-010: Vite Dev Port Candidate Propagation Contract

Given Vite dev context is created with `DefaultPort=P` (or unset)  
When dev Vite port is resolved and exported for runtime use  
Then:

- effective candidate default port MUST be `P` when set, otherwise `5173`,
- selected free port resolution MUST be based on that effective candidate (with
  free-port fallback behavior allowed by the underlying port helper),
- selected port MUST be written to environment key `__VITE_PORT`,
- runtime dev URL/script helpers consuming `__VITE_PORT` MUST observe that same
  selected value.

### BUILD-VITE-011: Vite Dev Start-Failure Propagation Contract

Given Vite dev-process command startup fails (for example exec spawn/permission
error)  
When dev-context startup flow runs (`DevBuild` -> `NewViteDevContext` ->
dev-server `startVite`)  
Then startup failure MUST be returned to caller (not logged-and-suppressed), and
caller-visible Vite runtime state MUST NOT present a false-positive "running"
context after that failure.

### BUILD-VITE-012: Buildtime URL Transform Match-Surface and Literal Output Contract

Given plugin transform scans module source for buildtime public URL calls  
When replacement matching is performed  
Then matcher surface MUST be limited to calls of configured function name with a
single same-delimiter string literal argument (`"..."`, `'...'`, or `` `...` ``).

Given call sites that use non-literal argument expressions  
When transform runs  
Then those call sites MUST remain unchanged by URL rewrite.

Given transformed known-asset and missing-asset call sites  
When replacement strings are emitted  
Then rewritten output MUST use canonical double-quoted string literals in both
cases (hashed+prefixed URL for hits; original asset path for misses).

### BUILD-VITE-013: Vite Config Merge Type-Guard Contract

Given plugin config merge runs against incoming Vite config  
When incoming user fields are non-array/non-object shapes  
Then plugin merge MUST apply explicit type guards:

- rollup user input entries are appended only for array-form
  `build.rollupOptions.input`,
- server watch ignored user entries are preserved only for array-form
  `server.watch.ignored`,
- resolve dedupe user entries are preserved only for array-form
  `resolve.dedupe`,
- modulePreload user fields are preserved only for object-form
  `build.modulePreload`.

## 4.9 Output Cleanup Contract

### BUILD-CLEAN-001: Vorma-Generated Public File Cleanup

Given build-inner cleanup runs  
When static public out dir exists  
Then files with prefixes:

- `vorma_out_vite_`
- `vorma_out_vorma_internal_route_manifest_`

MUST be removed prior to new artifact generation.

Cleanup MUST be selective: files not matching those prefixes MUST be preserved.

### BUILD-CLEAN-002: Missing Static Public Dir

Given static public out dir does not exist  
When cleanup runs  
Then cleanup SHOULD not fail solely for missing directory.

### BUILD-CLEAN-003: Static Public Path Type Guard

Given cleanup target path exists but is not a directory  
When cleanup runs  
Then cleanup MUST fail fast with invalid-path-type error.

## 4.10 Delegated Wave Schema and Validation Contracts

Wave owns core config-schema emission and core config-validation semantics.
Vorma inherits those behaviors at runtime/build boundaries but does not duplicate
their requirement prose here.

Canonical owner references:

- `specs/packages/wave/tooling/SPEC.md`
- `specs/packages/wave/tooling/TRACEABILITY_MATRIX.md`
- `specs/packages/wave/tooling/CONFORMANCE_ISSUES.md`

Vorma-owned build/dev schema coverage in this spec is limited to framework
extension behavior directly injected by Vorma (`Vorma_Schema` and hook/watch
injection contracts).

## 5. Executable Conformance Scenario Catalog

This section defines concrete black-box scenarios that SHOULD be used as the
default test vectors for requirements above.

Scenario IDs are stable references for test planning and CI reporting.

## 5.1 CLI and Mode Scenarios

### BDC-CLI-001 (covers BUILD-CLI-001)

Given build command is invoked with `--hook`  
When command executes  
Then framework build-inner flow MUST run and command MUST exit from hook path.

### BDC-CLI-002 (covers BUILD-CLI-002)

Given build command is invoked with `--hook --dev`  
When command executes  
Then dev hook flow MUST run and Vite production/post-Vite phase MUST NOT run.

### BDC-CLI-003 (covers BUILD-CLI-003)

Given build command is invoked with `--hook` (without `--dev`)  
When command executes successfully  
Then it MUST run Vite production build and post-Vite stage-two processing.

### BDC-CLI-004 (covers BUILD-CLI-004)

Given build command is invoked with `--dev` and without `--hook`  
When command executes  
Then it MUST start dev-server build workflow.

### BDC-CLI-005 (covers BUILD-CLI-005)

Given build command is invoked without `--dev` and without `--hook`  
When command executes  
Then production flow MUST run through Wave builder path.

### BDC-CLI-006 (covers BUILD-CLI-006)

Given production build command includes `--no-binary`  
When command executes  
Then Go binary compilation step MUST be skipped.

Wave-owned CLI helper scenario coverage is tracked in
`specs/packages/wave/tooling/SPEC.md`
(`WDC-CLI-001` through `WDC-CLI-003`) and is intentionally not duplicated in
this Vorma-owned scenario catalog.

## 5.2 Framework Hook Scenarios

### BDC-HOOK-001 (covers BUILD-HOOK-001)

Given valid `MainBuildEntry`  
When framework hooks are injected  
Then configured hook commands MUST be:

- dev: `go run ./<MainBuildEntry> --dev --hook`
- prod: `go run ./<MainBuildEntry> --hook`.

### BDC-HOOK-002 (covers BUILD-HOOK-002)

Given both user and framework build hooks are configured  
When Wave builder runs hooks  
Then user hook MUST execute before framework hook.

## 5.3 Watch Pattern Scenarios

### BDC-WATCH-001 (covers BUILD-WATCH-001)

Given `Vorma.IncludeDefaults=false`  
When build config is prepared  
Then framework default watch patterns MUST NOT be injected.

### BDC-WATCH-002 (covers BUILD-WATCH-002)

Given defaults enabled and `ClientRouteDefsFile` configured  
When patterns are injected  
Then route-definitions watch entry MUST use callback flow with:

- `RunOnChangeOnly=true`, and
- `SkipRebuildingNotification=true`.

### BDC-WATCH-003 (covers BUILD-WATCH-003)

Given route-definitions file changes in dev and app is running  
When route callback succeeds  
Then callback MUST perform fast route rebuild, call
`GET /__vorma/reload-routes`, and request browser reload with app+Vite wait.

Given app is already stopped for active batch  
When route callback executes  
Then callback MUST skip reload-endpoint call and defer refresh to batch restart.

### BDC-WATCH-004 (covers BUILD-WATCH-004)

Given route reload endpoint call fails  
When route callback handles failure  
Then callback MUST return restart action with `RecompileGo=false`.

### BDC-WATCH-005 (covers BUILD-WATCH-005)

Given template file changes in dev and app is running  
When template callback succeeds  
Then callback MUST call `GET /__vorma/reload-template` and request browser
reload with app+Vite wait.

Given app is already stopped for active batch  
When template callback executes  
Then callback MUST skip reload-endpoint call and defer refresh to batch restart.

Given one configuration with both `HTMLTemplateLocation` and private-static-dir
available, and one configuration missing either prerequisite  
When default watch-pattern injection runs  
Then template watch callback entry MUST exist only in the fully configured case.

### BDC-WATCH-006 (covers BUILD-WATCH-006)

Given template reload endpoint call fails  
When template callback handles failure  
Then callback MUST return restart action with `RecompileGo=false`.

### BDC-WATCH-007 (covers BUILD-WATCH-007)

Given defaults enabled  
When patterns are injected  
Then `**/*.go` watch MUST include `DevBuildHook` with concurrent timing.

### BDC-WATCH-008 (covers BUILD-WATCH-008)

Given defaults enabled and `TSGenOutDir` configured  
When ignored patterns are injected  
Then ignore list MUST include generated `index.ts`, `filemap.ts`,
`filemap.json` in that directory.

### BDC-WATCH-012 (covers BUILD-WATCH-012)

Given reload endpoint call outcomes including timeout/transport error/non-`200`
status  
When watch callback failure handling is validated  
Then each outcome MUST be classified as endpoint failure and MUST trigger
restart fallback semantics for that callback path.

## 5.4 Route Parsing Scenarios

### BDC-ROUTE-001 (covers BUILD-ROUTE-001)

Given `ClientRouteDefsFile` with `route(...)` calls imported from
`vorma/client`  
When parser runs  
Then those route calls MUST be discovered.

### BDC-ROUTE-002 (covers BUILD-ROUTE-002)

Given route definitions with dynamic/non-static module expressions  
When parser runs  
Then unresolved routes MUST be ignored and warnings SHOULD be emitted.

### BDC-ROUTE-003 (covers BUILD-ROUTE-003)

Given parsed route module resolves to non-existent path  
When parsing/build proceeds  
Then build MUST fail with module-not-found error.

### BDC-ROUTE-004 (covers BUILD-ROUTE-004)

Given route definition omits export key  
When parsing runs  
Then route key MUST default to `"default"`.

### BDC-ROUTE-005 (covers BUILD-ROUTE-005)

Given route definitions with invalid signature shapes (non-string pattern and/or
omitted module argument)  
When parser/build runs  
Then build MUST fail with route-definition diagnostics and MUST NOT silently
drop or coerce those entries into usable routes.

### BDC-ROUTE-006 (covers BUILD-ROUTE-006)

Given route definitions containing duplicate pattern strings with different
module/key payloads  
When parser output map is inspected  
Then final entry for that pattern MUST match the last discovered duplicate
definition and earlier duplicates MUST not remain in output map.

### BDC-ROUTE-007 (covers BUILD-ROUTE-007)

Given a route file containing both resolvable and unresolved module expressions
for `route(...)` calls  
When parser/build logs and output map are inspected  
Then unresolved diagnostics SHOULD include pattern/source/expression/reason
context and unresolved entries MUST be absent from emitted route map while
resolvable entries remain present.

### BDC-ROUTE-008 (covers BUILD-ROUTE-008)

Given route-definitions file contains transform/parsing-invalid syntax  
When parser/build executes route extraction  
Then build MUST fail with transform diagnostics and MUST NOT emit partial route
map output.

### BDC-ROUTE-009 (covers BUILD-ROUTE-009)

Given route-definition module imports using nested and platform-native separator
variants  
When parsed route metadata output is inspected  
Then `SrcPath` values MUST be workspace-relative slash-normalized paths.

## 5.5 Fast Rebuild Scenarios

### BDC-FAST-001 (covers BUILD-FAST-001)

Given fast route rebuild is called while runtime is not in dev mode  
When function executes  
Then it MUST fail.

### BDC-FAST-002 (covers BUILD-FAST-002)

Given fast route rebuild in dev mode plus missing-dir cleanup variant  
When operation and cleanup behavior are observed  
Then it MUST emit `dev_fast_*` build id, sync route registry, clean prior route
manifests without deleting unrelated assets, rewrite route artifacts, and
continue in missing-dir cleanup case without failing solely due to absence.

## 5.6 Artifact Output Scenarios

### BDC-ART-001 (covers BUILD-ART-001)

Given successful build-inner artifact write  
When stage-one paths output is checked  
Then `vorma_paths_stage_1.json` MUST exist under private `vorma_out/` with
`stage="one"` and `routeManifestFile` value matching manifest generated in that
artifact-write pass.

### BDC-ART-002 (covers BUILD-ART-002)

Given successful post-Vite production phase  
When stage-two paths output is checked  
Then `vorma_paths_stage_2.json` MUST exist under private `vorma_out/` with
`stage="two"`.

### BDC-ART-003 (covers BUILD-ART-003)

Given route artifacts are written  
When route manifest file is checked  
Then filename MUST use required prefix/suffix and JSON content MUST map pattern
to `0|1`, and filename hash payload MUST match manifest-byte hash derivation
contract.

### BDC-ART-004 (covers BUILD-ART-004)

Given TypeScript generation succeeds  
When output files are checked  
Then `<TSGenOutDir>/index.ts` MUST exist and generated content MUST satisfy
documented `vormaAppConfig`/deterministic-ordering/action-category invariants.

Given generated output includes loader entries sourced from both Go loaders and
client-defined-only paths  
When generated param/splat typing is inspected  
Then loader entries MUST use loader rune settings (not action rune settings).

### BDC-ART-005 (covers BUILD-ART-005)

Given build-inner writes filemap outputs  
When output files are checked  
Then `<TSGenOutDir>/filemap.ts` and `<TSGenOutDir>/filemap.json` MUST exist.

### BDC-ART-006 (covers BUILD-ART-006)

Given generated TS content is unchanged across two runs  
When second run executes  
Then generator SHOULD skip rewriting `index.ts` (mtime/content unchanged).

### BDC-ART-007 (covers BUILD-ART-007)

Given standard dev build and fast-route dev rebuild paths  
When emitted build ids are inspected  
Then standard dev ids MUST start with `dev_` and fast-route ids MUST start with
`dev_fast_`.

### BDC-ART-008 (covers BUILD-ART-008)

Given TS generation fixtures with and without effective root-data loader handler  
When generated type aliases are inspected  
Then `VormaRootData` MUST resolve to root loader output type in handler-present
case and MUST resolve to `null` in handler-absent case.

### BDC-ART-009 (covers BUILD-ART-009)

Given TS generation fixture with non-empty `ExtraTSCode` payload  
When generated `index.ts` is inspected  
Then payload text MUST be present in output after core generated app-config/type
scaffold.

## 5.7 Stage-Two Production Scenarios

### BDC-STAGE2-001 (covers BUILD-STAGE2-001)

Given valid Vite manifest and configured client entry  
When stage-two conversion runs  
Then `clientEntryOut` and `clientEntryDeps` MUST be populated in stage-two
paths output.

### BDC-STAGE2-002 (covers BUILD-STAGE2-002)

Given parsed routes and Vite manifest chunks  
When stage-two conversion runs  
Then route entries MUST be updated with output chunk basename and dependency
list.

### BDC-STAGE2-003 (covers BUILD-STAGE2-003)

Given Vite manifest with CSS metadata  
When stage-two conversion runs  
Then `depToCSSBundleMap` MUST map JS chunk basenames to CSS basenames.

### BDC-STAGE2-004 (covers BUILD-STAGE2-004)

Given unchanged template/stage-two/public-fs summary inputs  
When production build ID is computed across repeated runs  
Then build ID MUST remain stable.

Given any of those inputs changes  
When build ID recomputes  
Then build ID MUST change.

Public-fs summary fixture refinement:

- summary input fixture MUST derive from non-directory walked entries encoded as
  `<path>|<size>` tuples.

### BDC-STAGE2-005 (covers BUILD-STAGE2-005)

Given successful stage-two conversion  
When stage-two payload and runtime accessors are inspected  
Then payload `buildID`, runtime `GetBuildID()`, and payload
`routeManifestFile` propagation MUST match contract.

## 5.8 Vite Integration Scenarios

### BDC-VITE-001 (covers BUILD-VITE-001)

Given generated Vorma Vite config  
When rollup input array is inspected  
Then it MUST include client entry plus all route source entry paths.

Given that same rollup input array  
When ordering/duplication is inspected  
Then entries MUST be de-duplicated and deterministically sorted.

### BDC-VITE-002 (covers BUILD-VITE-002)

Given each `UIVariant` (React/Preact/Solid)  
When Vite config is generated  
Then dedupe list MUST match variant family contract.

### BDC-VITE-003 (covers BUILD-VITE-003)

Given Vite production output generated under plugin  
When file names are inspected  
Then asset/chunk/entry names MUST use `vorma_out_vite_` prefix.

### BDC-VITE-004 (covers BUILD-VITE-004)

Given source containing configured buildtime public URL function calls  
When plugin transform runs with filemap entry present and missing-entry variants  
Then known assets MUST rewrite to prefixed hashed URL string literals, while
missing assets MUST remain original literals.

### BDC-VITE-005 (covers BUILD-VITE-005)

Given Vite dev server plugin active  
When matching and non-matching URLs are requested  
Then endpoint MUST return `200` with `ok`, clear filemap cache, invalidate module
graph, and trigger full reload event for matching path only; non-matching paths
MUST flow through middleware chain.

### BDC-VITE-006 (covers BUILD-VITE-006)

Given plugin config hook is evaluated for `serve` and `build` commands  
When resulting config is inspected  
Then base/mode behavior and merge semantics (headers/watch/dedupe/modulePreload/rollup input/preserve-entry-signatures) MUST match contract.

### BDC-VITE-007 (covers BUILD-VITE-007)

Given dev-mode transform reads filemap JSON across unchanged/changed mtime and unreadable-file conditions  
When repeated transforms run  
Then filemap lookup MUST follow cache reuse, refresh, and static-map fallback rules.

### BDC-VITE-008 (covers BUILD-VITE-008)

Given generated `index.ts` helper-module section  
When exported bindings and object keys are inspected  
Then helper surface (`staticPublicAssetMap`, `publicPathPrefix`,
`waveRuntimeURL`, `vormaViteConfig`, and global buildtime function declaration)
MUST match documented contract.

### BDC-VITE-009 (covers BUILD-VITE-009)

Given generated Vite helper config for representative project paths  
When `ignoredPatterns` baseline is inspected  
Then baseline entries for Go/dist/private-static/config/TSGenOutDir/route-defs
MUST be present.

### BDC-VITE-010 (covers BUILD-VITE-010)

Given one Vite dev context with explicit `DefaultPort` and one with default
port unset  
When dev-port initialization path, `__VITE_PORT` assignment, and runtime
dev-script URL consumption are inspected  
Then effective default-candidate selection (`P` vs `5173`), candidate-based
free-port resolution, and env/value propagation MUST match contract.

### BDC-VITE-011 (covers BUILD-VITE-011)

Given Vite dev startup is forced into command-start failure path  
When startup flows are observed through `DevBuild`, builder `NewViteDevContext`,
and dev-server `startVite`  
Then failure MUST propagate as error (no silent success return) and no
stale/running Vite context MUST be published to runtime consumers.

### BDC-VITE-012 (covers BUILD-VITE-012)

Given transform source variants containing:

- literal buildtime URL calls with `"..."`, `'...'`, and `` `...` `` arguments,
- non-literal argument calls,

When plugin transform runs under hit and miss filemap conditions  
Then only literal-call variants MUST rewrite and rewritten outputs MUST be
double-quoted literals (`"<prefix><hashed>"` for hits, `"<original>"` for
misses).

### BDC-VITE-013 (covers BUILD-VITE-013)

Given plugin config merge input variants spanning array-form and non-array/non-
object user fields for rollup input, watch ignored, dedupe, and modulePreload  
When merged config output is inspected  
Then user entries MUST be preserved only for accepted guarded shapes and ignored
for other shapes, while plugin-required values remain applied.

## 5.9 Cleanup Scenarios

### BDC-CLEAN-001 (covers BUILD-CLEAN-001)

Given static public output contains prior Vorma-generated public artifacts  
When cleanup runs  
Then files with required Vorma prefixes MUST be removed before new generation,
while unrelated non-prefixed files remain intact.

### BDC-CLEAN-002 (covers BUILD-CLEAN-002)

Given static public output directory is absent  
When cleanup runs  
Then operation SHOULD continue without failing solely due to missing directory.

### BDC-CLEAN-003 (covers BUILD-CLEAN-003)

Given cleanup target exists as non-directory filesystem node  
When cleanup runs  
Then operation MUST fail with invalid-path-type error class.

## 5.10 Delegated Wave Scenario Coverage

Wave-owned schema/validation scenarios are tracked in Wave owner artifacts and
are intentionally not duplicated in this Vorma-owned scenario catalog.

## 6. Conformance Test Guidance

Build/dev conformance suites SHOULD:

1. Keep each requirement ID and scenario ID independently reportable.
2. Cover CLI matrix (`--dev`, `--hook`, `--no-binary`) with observable phase assertions.
3. Validate artifact existence/content shape for stage1/stage2/manifest/TS outputs.
4. Validate fast route rebuild callbacks and fallback restart behavior.
5. Include route DSL negative cases (dynamic module expressions, missing modules).
6. Validate Vite integration contracts (naming prefix, transform rewriting, invalidation endpoint).
7. Validate deterministic behavior (TS write skipping and build ID sensitivity to declared inputs).
8. Keep delegated Wave owner suites separately traceable in Wave spec artifacts.
9. Track traceability with IDs (`BUILD-*`, `BDC-*`) in CI output.

## 7. Relation to Other Specs

- Backend runtime behavior:
  `specs/packages/vormaruntime/SPEC.md`
- Wire protocol behavior:
  `specs/packages/vormaruntime/SPEC.md`
- Frontend runtime behavior:
  `specs/packages/vormaclient/client/SPEC.md`
- Kit interop boundaries: `specs/packages/vorma/SPEC.md`
- Testing strategy:
  `specs/SPEC_GOVERNANCE.md`
- Overall roadmap: `specs/packages/vormabuild/SPEC_CHECKLIST.md`
