# Vorma Build and Dev Conformance Specification

Status: Draft  
Last Updated: 2026-02-08  
Applies To: Vorma build-time and dev-time behavior observable via CLI, filesystem outputs, and dev callbacks

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

### BUILD-CLI-007: CLI Logger Defaulting Contract

Given CLI entry helper `BuildWaveWithHook` or wrapper `BuildWave` is invoked
with nil logger  
When entrypoint initializes runtime dependencies  
Then helper MUST construct default color logger named `wave` before invoking
dev/build flows.

### BUILD-CLI-008: Hook Callback Gate and Precedence Contract

Given CLI flags include `--hook` and a non-nil hook callback is provided  
When entry helper executes  
Then helper MUST invoke callback exactly once with parsed `--dev` value and MUST
return immediately without entering `RunDev` or builder build path.

Given CLI flags include `--hook` but callback is nil  
When entry helper executes  
Then helper MUST ignore hook-only callback path and continue through standard
`--dev`/production flow selection.

### BUILD-CLI-009: CLI Error Propagation and Builder Lifecycle Contract

Given hook callback, `RunDev`, or production builder path returns error  
When CLI helper handles that failure  
Then helper MUST fail fast via panic (no error return contract).

Given production builder path is selected  
When helper completes (success or panic unwind)  
Then builder cleanup (`Close`) MUST be deferred for execution.

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
Then route defs file MUST be watched with run-on-change-only callback flow.

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

### BUILD-WATCH-013: Framework Injection Helper Accumulation Contract

Given `AddFrameworkWatchPatterns` is called multiple times with pattern sets in
order `A`, then `B`  
When effective framework watch pattern list is inspected  
Then list MUST preserve append semantics (entries from `A` remain and entries
from `B` are appended in call order).

Given `AddIgnoredPatterns` is called multiple times with ignore sets in order
`I1`, then `I2`  
When effective framework ignored-pattern list is inspected  
Then list MUST preserve append semantics (entries from `I1` remain and entries
from `I2` are appended in call order).

Given `SetPublicFileMapOutDir` is called repeatedly with values `D1...Dn`  
When effective framework public-filemap output directory is inspected  
Then effective value MUST equal last call value (`Dn`).

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

## 4.10 Dev Server Control-Plane Contract

### BUILD-DEV-001: Single-Instance Project Lock

Given `wave dev` starts for a project  
When lock acquisition runs  
Then runtime MUST enforce single active dev instance per project via lock file
in dist-static scope, and MUST reject startup when lock owner PID is live.

Given existing lock PID is stale/dead  
When startup retries  
Then lock takeover MUST succeed.

### BUILD-DEV-002: Refresh Server Lifecycle

Given browser mode dev server  
When rebuild loops occur  
Then refresh server/client-manager MUST stay alive across rebuild iterations and
MUST be cleaned only on full process shutdown.

### BUILD-DEV-003: Config Reload Preservation Rules

Given post-first-run config reload  
When config file parses and validates  
Then framework-injected runtime fields (`FrameworkWatchPatterns`,
`FrameworkIgnoredPatterns`, `FrameworkPublicFileMapOutDir`) MUST be preserved
across reload.

### BUILD-DEV-015: Config Reload Failure Is Non-Terminal and Reuses Prior Active Config

Given post-first-run config reload attempt fails (parse error or validation
failure)  
When dev loop continues  
Then runtime MUST log reload failure and MUST continue using the prior
last-known-good active config rather than terminating the dev process.

### BUILD-DEV-016: Vite Filemap Invalidation Call Contract

Given dev control plane triggers Vite filemap invalidation  
When Vite context is unavailable  
Then invalidation call MUST fail with explicit "vite not running" class error.

Given Vite context is available  
When invalidation call is executed  
Then control plane MUST:

- issue `POST` to `http://localhost:<vite-port>/__vorma_invalidate_filemap`,
- run request with bounded context and client timeouts (approximately 5s each),
- treat non-`200` status as failure,
- treat request construction/transport failures as failure.

### BUILD-DEV-004: Sequential Go Build Mode

Given `Core.SequentialGoBuild=false`  
When rebuild runs with Go recompilation enabled  
Then Go compile MAY run concurrently with build phase.

Given `Core.SequentialGoBuild=true`  
When rebuild runs with Go recompilation enabled  
Then Go compile MUST run only after non-Go build phase completes.

### BUILD-DEV-005: Build Failure Retry Gate

Given build/compile step fails during dev loop  
When failure handling runs  
Then dev server MUST wait for watcher-triggered restart request before retrying
build loop.

### BUILD-DEV-006: Config-Restart Reload Ordering

Given restart reason is config change  
When app restart completes  
Then server MUST broadcast reload-before-watcher-start for that iteration
(preventing watcher-triggered race reload from preceding config reload).

### BUILD-DEV-007: Restart Queue Upgrade Semantics

Given restart channel already has pending non-config restart  
When stronger request arrives  
Then queue semantics MUST enforce:

- config restart supersedes all pending requests,
- non-config restart with `recompileGo=true` upgrades pending weaker request,
- weaker/equivalent non-config request MUST NOT downgrade pending stronger request.

### BUILD-DEV-008: Readiness Polling Contract

Given app/Vite readiness waits are enabled for a reload path  
When readiness polling runs  
Then polling MUST treat HTTP 200 as ready and bound total wait time to finite
budget (approximately 10 seconds with bounded per-request timeout).

### BUILD-DEV-010: Refresh Server Endpoint Contract

Given browser-mode dev server starts refresh server  
When endpoint handlers are mounted  
Then refresh server MUST expose:

- WebSocket events endpoint at `/events`,
- JavaScript endpoint at `/get-refresh-script-inner`,
- `Access-Control-Allow-Origin: *` on both endpoints,
- `Content-Type: text/javascript` on refresh script endpoint.

### BUILD-DEV-011: Refresh Manager Shutdown Guard

Given refresh manager context is canceled during shutdown  
When new websocket upgrade is attempted  
Then refresh endpoint MUST reject with service-unavailable response.

Given broadcast send paths execute during or after shutdown  
When context cancellation is observed  
Then broadcast logic MUST return without blocking indefinitely.

### BUILD-DEV-012: Cycle-Vite Reload Signal Exclusivity

Given browser reload orchestration runs with `cycleVite=true`  
When app/Vite readiness and Vite cycle steps complete  
Then runtime MUST rely on Vite client reconnect as the reload trigger and MUST
NOT additionally emit Wave refresh reload payload for the same cycle.

Given browser reload orchestration runs with `cycleVite=false`  
When readiness waits complete  
Then runtime MUST emit Wave refresh reload payload according to resolved
browser action.

### BUILD-DEV-013: Readiness Retry Envelope and Termination Semantics

Given readiness polling helper executes for app/Vite endpoint URL  
When endpoint repeatedly remains unavailable  
Then polling MUST:

- cap attempts at 100,
- use request-timeout budget of approximately 500ms per probe,
- use linear backoff starting at 20ms and increasing by 20ms per attempt,
- terminate and return not-ready once cumulative delay budget exceeds
  approximately 10 seconds.

Given any probe returns HTTP `200`  
When polling loop evaluates that response  
Then helper MUST terminate early and return ready.

### BUILD-DEV-017: Refresh Broadcast Fan-Out and Shutdown Drain Semantics

Given refresh manager broadcasts payload to connected websocket clients  
When one or more clients are not currently ready to receive  
Then fan-out MUST remain non-blocking and MUST continue delivering to other
ready clients without stalling server event loop progress.

Given refresh manager context is canceled  
When manager shutdown cleanup runs  
Then manager MUST close active client connections and drain pending
register/unregister/broadcast channel work before final manager stop
notification completes.

### BUILD-DEV-018: Lock File Materialization and Stale Payload Semantics

Given dev lock acquisition runs  
When lock parent directory does not yet exist  
Then runtime MUST create lock parent directory before reading/writing lock file.

Given lock file read fails for reasons other than file-not-found  
When acquisition evaluates existing lock state  
Then acquisition MUST fail and surface read error.

Given lock file payload cannot be parsed into a positive PID (for example empty,
non-numeric, or `<=0`)  
When acquisition runs  
Then lock MUST be treated as stale and current process MUST overwrite lock file
with its own PID.

Given lock file payload contains a positive PID and process is no longer running  
When acquisition runs  
Then lock takeover MUST succeed by overwriting lock file with current PID.

Given lock is acquired and dev process exits  
When deferred release runs  
Then lock file MUST be removed from dist-static lock path.

### BUILD-DEV-019: Cross-Platform PID Liveness Probe Contract

Given non-Windows build target evaluates lock-owner PID liveness  
When probe executes  
Then probe MUST use process existence check equivalent to `FindProcess` +
signal-`0` semantics.

Given Windows build target evaluates lock-owner PID liveness  
When probe executes  
Then probe MUST use limited-information process-open semantics and MUST treat
open success as running and open failure as not-running.

### BUILD-DEV-020: Watch Root Default and Normalization Contract

Given watch config omits `Watch.WatchRoot`  
When runtime resolves effective watch root  
Then effective watch root MUST default to `"."`.

Given watch config provides `Watch.WatchRoot`  
When runtime resolves effective watch root  
Then resolved root MUST be normalized via path-clean semantics before watcher
path use.

### BUILD-DEV-021: Healthcheck Endpoint Default Contract

Given watch config omits `Watch.HealthcheckEndpoint`  
When runtime resolves readiness healthcheck endpoint for app polling  
Then endpoint MUST default to `"/"`.

Given watch config provides `Watch.HealthcheckEndpoint`  
When runtime resolves readiness healthcheck endpoint  
Then provided endpoint value MUST be used.

### BUILD-DEV-022: Browser/Vite Mode Derivation Contract

Given parsed config has `Core.ServerOnlyMode=true`  
When runtime resolves browser-usage mode  
Then `UsingBrowser()` MUST be `false`.

Given parsed config has `Core.ServerOnlyMode=false`  
When runtime resolves browser-usage mode  
Then `UsingBrowser()` MUST be `true`.

Given parsed config includes `Vite` block vs omits it  
When runtime resolves Vite-usage mode  
Then `UsingVite()` MUST be `true` iff `Vite` config is present.

### BUILD-DEV-023: App Port Resolution and Latching Contract

Given runtime resolves app port through `MustGetPort()` in non-dev mode, or
while `WAVE_PORT_HAS_BEEN_SET=true`  
When `PORT` is parsed as positive integer `P`  
Then resolved port MUST be `P`.

Given same non-dev/latched gate and `PORT` is absent/invalid/non-positive  
When app port resolves  
Then resolved port MUST default to `8080`.

Given runtime resolves app port in dev mode with
`WAVE_PORT_HAS_BEEN_SET!=true`  
When default candidate port is computed from `PORT` (or `8080` fallback) and
free-port probe succeeds/fails  
Then runtime MUST use probed free port on success and default candidate on
probe failure, and MUST set both:

- `PORT=<resolved-port>`,
- `WAVE_PORT_HAS_BEEN_SET=true`.

Free-port probe semantics inherited by runtime port helper MUST preserve:

- candidate default port normalization to `8080` when env-derived default is
  invalid/non-positive/out-of-range,
- first-choice use of normalized default when available,
- bounded ascending scan up to next `1024` ports before random fallback probe,
- random fallback probe using OS-assigned free port when bounded scan fails,
- if random fallback probe fails, resolution returns normalized default and
  surfaces probe error.

Given app port has been resolved once in process lifetime  
When `MustGetPort()` is called again  
Then returned value MUST remain latched to first resolved value.

### BUILD-DEV-024: Dev-Mode Environment Helper Contract

Given `WAVE_MODE` environment variable equals `development`  
When `GetIsDev()` is called  
Then helper MUST return `true`.

Given `WAVE_MODE` is absent or set to any other value  
When `GetIsDev()` is called  
Then helper MUST return `false`.

Given `SetModeToDev()` is called  
When subsequent mode reads occur  
Then helper MUST set `WAVE_MODE=development`.

### BUILD-DEV-025: Refresh Script Dev-Gating and Default Port Contract

Given runtime is not in dev mode  
When refresh-script helpers are evaluated (`GetRefreshScript`,
`GetRefreshScriptSha256Hash`)  
Then both helpers MUST return empty string output.

Given runtime is in dev mode and refresh-server port is unset/invalid/non-positive  
When refresh-script helpers are evaluated  
Then helpers MUST resolve script port to default `10000`.

Given runtime is in dev mode and refresh-server port is positive `P`  
When refresh-script helper output is evaluated  
Then emitted script content MUST target websocket endpoint
`ws://localhost:P/events`.

### BUILD-DEV-026: Refresh Script Hash Derivation Contract

Given runtime is in dev mode and resolved refresh-script port is `P`  
When `GetRefreshScriptSha256Hash()` is evaluated  
Then returned hash MUST equal base64-encoded SHA-256 digest of raw
`RefreshScriptInner(P)` bytes.

### BUILD-DEV-027: Refresh Script Rebuilding Overlay Contract

Given refresh script websocket receives payload `changeType="rebuilding"`  
When message handler executes  
Then script MUST ensure a rebuilding overlay element exists with id
`wave-refreshscript-rebuilding` and MUST NOT create duplicate overlay elements
for repeated rebuilding messages while one already exists.

Given same rebuilding overlay creation path  
When overlay element is initialized  
Then overlay MUST start with opacity `0` and transition to visible state
shortly after insertion (fade-in effect).

### BUILD-DEV-028: Refresh Script Hard-Reload Scroll-Persistence Contract

Given refresh script websocket receives payload `changeType="other"`  
When message handler executes  
Then script MUST trigger full document reload (`window.location.reload()`).

Given same `other` payload and current `window.scrollY` is positive  
When reload path runs  
Then script MUST persist current scroll position into session-storage key
`__wave_internal__devScrollY` before reload.

Given script initializes and persisted refresh-scroll key exists  
When startup restore path runs  
Then script MUST remove that key and attempt smooth scroll restoration using the
stored value.

### BUILD-DEV-029: Refresh Script CSS Hot-Swap DOM Contract

Given refresh script websocket receives payload `changeType="normal"` with
`normalCSSURL`  
When message handler executes  
Then script MUST target stylesheet element id `wave-normal-css` and apply
non-critical CSS update by inserting new `<link rel="stylesheet">` with that id
and new URL.

Given prior non-critical stylesheet element exists  
When new stylesheet link loads  
Then script MUST remove old stylesheet element.

Given refresh script websocket receives payload `changeType="critical"` with
base64 payload `criticalCSS`  
When message handler executes  
Then script MUST target style element id `wave-critical-css` and replace or
append critical CSS style element using decoded payload content.

### BUILD-DEV-030: Refresh Script Revalidate Hook Contract

Given refresh script websocket receives payload `changeType="revalidate"`  
When message handler executes  
Then script MUST look for global `window.__waveRevalidate` and, if present,
invoke it and remove rebuilding overlay after successful promise resolution.

Given same revalidate payload and `window.__waveRevalidate` promise rejects  
When rejection outcome is observed  
Then script MUST log rejection-class error diagnostics and MUST remove
rebuilding overlay (failed revalidate MUST NOT leave rebuilding overlay stuck).

Given same revalidate payload and `window.__waveRevalidate` is absent  
When handler executes  
Then script MUST log error and remove rebuilding overlay without throwing.

### BUILD-DEV-031: Refresh Script Socket Failure Fallback Contract

Given refresh websocket closes unexpectedly during active page lifecycle  
When `onclose` handler executes  
Then script MUST trigger full document reload.

Given refresh websocket errors  
When `onerror` handler executes  
Then script MUST close websocket and trigger full document reload.

Given browser `beforeunload` fires  
When unload handler executes  
Then script MUST disable reload-on-close behavior before closing websocket to
avoid duplicate/unwanted reload loop during intentional page unload.

### BUILD-DEV-032: Vite Startup Failure Handling Contract

Given dev server is configured to use Vite  
When Vite startup fails during initial run or Vite-cycle restart  
Then dev control plane MUST treat startup failure as actionable failure state
rather than silent success:

- failure MUST be surfaced through dev-loop control flow (not only debug logs),
- runtime MUST NOT continue presenting Vite as healthy/ready for subsequent
  waits/reload orchestration until successful startup occurs.

### BUILD-DEV-033: Build-Failure Retry Request Strength Preservation Contract

Given dev loop is waiting in build-failure retry state  
When watcher-driven restart request is consumed to resume loop  
Then control flow MUST preserve request strength for the next iteration:

- `recompileGo` intent MUST be preserved (for example, Go-file-triggered retry
  MUST NOT be downgraded to no-Go retry),
- config-restart intent MUST be preserved (so config-restart-specific ordering
  and reload signaling still applies on the resumed iteration).

Given multiple restart requests race while retry-wait is active  
When queue supersede/upgrade rules resolve strongest effective request  
Then resumed iteration MUST observe that strongest effective request semantics
consistent with restart-queue policy.

### BUILD-DEV-034: App Startup Failure Handling Contract

Given build/compile phases succeeded and runtime attempts to launch app binary  
When app process startup fails (for example exec/start error)  
Then dev control plane MUST treat that failure as actionable startup failure
state rather than silent continuation:

- failure MUST be surfaced through control flow (not only log output),
- loop MUST NOT transition into normal running iteration semantics that assume a
  live app process (for example watcher-ready idle wait plus browser reload
  success signaling) until successful app startup occurs.

### BUILD-DEV-035: Readiness-Gate Failure Handling Contract

Given reload orchestration requests readiness waits (`waitApp` and/or
`waitVite`)  
When readiness polling exhausts retry budget without HTTP-200 success  
Then runtime MUST treat that as actionable readiness-gate failure state rather
than warning-only continuation:

- readiness failure MUST be surfaced through control flow (not only warning
  logs),
- reload/cycle path MUST NOT proceed under false-ready success semantics for
  the failed dependency (app and/or Vite) until readiness succeeds.

### BUILD-DEV-036: Config Reload Full Framework-Field Preservation Contract

Given post-first-run config reload with framework-injected non-JSON runtime
fields populated in active config  
When config file parses and validates successfully  
Then reload replacement config MUST preserve the full framework-injected field
set across reload:

- `FrameworkWatchPatterns`
- `FrameworkIgnoredPatterns`
- `FrameworkPublicFileMapOutDir`
- `FrameworkSchemaExtensions`
- `FrameworkDevBuildHook`
- `FrameworkProdBuildHook`

Given preserved framework fields include schema extensions and framework build
hooks  
When subsequent build/dev iterations execute after reload  
Then framework schema emission and framework hook execution behavior MUST remain
equivalent to pre-reload active config state (unless framework code explicitly
mutates those fields at runtime).

### BUILD-DEV-037: Dev Exit-Override Timer Contract

Given env `WAVE_DEV_EXIT_AFTER_MS` is unset, non-integer, or non-positive  
When dev control loop reaches restart-request wait phase  
Then exit-override timer MUST be disabled and loop MUST wait for restart request
without override-triggered exit.

Given env `WAVE_DEV_EXIT_AFTER_MS` is positive integer `N` and no restart
request arrives before `N` milliseconds  
When override timer expires in restart-request wait phase  
Then loop MUST:

- log exit due to env override with configured env key and duration,
- run rebuild cleanup (app/watcher/builder teardown),
- stop Vite context if present,
- return successful loop termination (no error) so deferred shutdown cleanup
  (refresh-manager/server and dev lock release) can complete normally.

Given same positive override and restart request arrives before timer expiry  
When wait phase resolves  
Then restart request MUST win and timer-exit path MUST NOT run for that cycle.

### BUILD-DEV-038: App-Port Alias Compatibility Contract

Given runtime exposes compatibility alias `MustGetAppPort`  
When alias and canonical helper are invoked under equivalent process/env state  
Then alias output and side effects MUST be behaviorally identical to
`MustGetPort` (including latching and environment side effects).

## 4.12 Watch/Event Processing Contract

### BUILD-EVT-001: Debounced Batch and Path Dedupe

Given rapid fsnotify events for same file path  
When debounced processing runs  
Then batch MUST deduplicate work by path within that batch.

Batch-callback discipline:

- watcher debounce callback execution MUST be non-overlapping; if a callback is
  in flight, newly arrived events MUST be queued and flushed in a subsequent
  debounce cycle.
- after path-level dedupe, any additional watched-pattern-key dedupe MUST apply
  only to non-empty effective watched-file pattern keys and MUST NOT collapse
  distinct classified events that have no matched watched-file pattern key.
- path-level dedupe MUST preserve effective content-change signal for that path:
  a trailing chmod-only event in the same batch MUST NOT erase an earlier
  content-changing event for that path.

### BUILD-EVT-002: Config File Change Short-Circuit

Given watched config file receives write/create event  
When event batch processes  
Then runtime MUST request config restart and skip remaining event work in that
batch.

### BUILD-EVT-003: Non-Empty chmod-Only Event Suppression

Given fsnotify event is chmod-only on non-empty file  
When classification runs  
Then event SHOULD be ignored as non-content-changing noise.

### BUILD-EVT-004: Event Classification Priority

Given changed path may match multiple categories  
When classification runs  
Then priority MUST be:

1. tracked critical CSS import,
2. tracked normal CSS import,
3. `.go` file (unless matched config sets `TreatAsNonGo=true`),
4. public static,
5. private static,
6. other watched pattern.

### BUILD-EVT-005: Watched-File Merge Semantics

Given a path matches multiple watched-file configs (framework and/or user)  
When merged effective config is computed  
Then merge MUST obey union semantics:

- work-trigger flags (`RecompileGoBinary`, `RestartApp`) are OR,
- suppressive flags (`TreatAsNonGo`, `RunOnChangeOnly`,
  `SkipRebuildingNotification`) are AND-like (any false wins),
- `OnlyRunClientDefinedRevalidateFunc` is trump/OR,
- hook order is framework/default hooks before user hooks.

### BUILD-EVT-006: Hook Phase Ordering (Single Event)

Given one classified event with hooks  
When processing runs  
Then phase order MUST be:

1. fire concurrent-no-wait hooks,
2. run pre hooks,
3. run build phase and concurrent hooks in parallel,
4. await hard-reload app stop if needed,
5. run post hooks,
6. restart app if required,
7. execute browser phase.

### BUILD-EVT-007: Batch Hard-Reload App Stop Once

Given multi-event batch where any event needs hard reload  
When batch processing runs and flow does not finalize through all-run-on-change-only
short-circuit  
Then app MUST be stopped once up front for the batch, and browser refresh
decision MUST execute once for the batch.

### BUILD-EVT-008: Browser Action Resolution Precedence

Given accumulated work-set includes restart/revalidate/css/static actions  
When browser behavior resolves  
Then precedence MUST be:

1. app restart -> hard reload with readiness waits,
2. explicit client-defined revalidate preference,
3. CSS-only work -> CSS hot reload path,
4. public-static change -> Vite invalidate path,
5. private-static or mixed CSS/static -> hard reload path.

### BUILD-EVT-009: Vite Invalidate Fallback

Given browser action selects Vite filemap invalidation and call fails  
When fallback executes  
Then runtime MUST degrade to hard reload path with app/Vite readiness waits.

### BUILD-EVT-010: RunOnChangeOnly Build Skip

Given effective watched-file policy is `RunOnChangeOnly=true`  
When event processing runs  
Then standard build phase MUST be skipped for that event path.

Action-propagation refinement:

- `RunOnChangeOnly` MUST skip only standard build work; it MUST NOT discard
  callback-returned `RefreshAction` intent.
- if callback actions request restart/reload/wait semantics, runtime MUST still
  honor those actions in the event/batch outcome.
- batch path where all events are `RunOnChangeOnly=true` MUST preserve merged
  callback actions the same way.
- `RunOnChangeOnly` MUST still execute configured `OnChangeHooks` across
  supported timing phases for that event path (for example no-wait/pre/concurrent/post
  callback hooks), while skipping standard build/reload work.
- this callback-phase execution requirement is per-event and MUST hold for
  `RunOnChangeOnly` entries even when those entries are processed inside mixed
  multi-event batches that include non-`RunOnChangeOnly` entries.
- when `RunOnChangeOnly=true`, command hooks remain constrained by schema
  validation (`BUILD-SCHEMA-005`: command timing must be pre/default), but callback
  hooks with other timings MUST still execute at their configured phase.
- `RunOnChangeOnly` event paths with no callback-requested restart action MUST
  NOT stop/kill the currently running app process as a side effect of
  pre-build hard-reload guard logic.

### BUILD-EVT-011: New Directory Watch Expansion

Given fsnotify create/rename event for a directory under watch root  
When event batch is processed  
Then watcher MUST add that directory subtree to watched set before continuing
file-classification work.

### BUILD-EVT-012: Rebuilding Signal Emission Gate

Given classified event batch  
When rebuilding-signal decision is computed  
Then "rebuilding" broadcast MUST emit only if at least one non-CSS event does
not opt out via `SkipRebuildingNotification=true`.

Given batch is CSS-only or all qualifying events opt out  
When rebuilding-signal decision is computed  
Then rebuilding broadcast MUST NOT emit.

### BUILD-EVT-013: Batch-Wide RunOnChangeOnly Short-Circuit

Given multi-event batch where all effective watched-file configs have
`RunOnChangeOnly=true`  
When batch processing runs  
Then standard build phase MUST be skipped for the batch.

Action-propagation refinement:

- skipping standard build in this batch-wide short-circuit MUST NOT discard
  merged callback-returned `RefreshAction` intent.
- if merged callback actions request restart/reload/wait semantics, runtime
  MUST honor them in final batch outcome.
- batch-wide run-on-change-only short-circuit MUST still execute each event's
  configured callback hooks for supported timing phases before finalizing merged
  callback action outcome.
- in mixed batches (not all run-on-change-only), `RunOnChangeOnly` entries MUST
  still execute their configured callback hooks for supported timing phases as
  per-event behavior.
- batch-wide run-on-change-only short-circuit with no callback-requested restart
  action MUST NOT leave app process stopped after short-circuit return.

### BUILD-EVT-014: Vite Invalidate Success Must Short-Circuit Browser Phase

Given browser phase selects Vite invalidate action and invalidate call succeeds  
When browser phase continues  
Then phase MUST return immediately after successful invalidation and MUST NOT
also emit hard-reload/revalidate/CSS-hot-reload payloads for that same phase
pass.

Given Vite invalidate action is selected while Vite is not enabled  
When browser phase resolves fallback  
Then runtime MUST convert action to hard reload with app wait enabled and no
Vite wait requirement.

### BUILD-EVT-015: CSS Hot-Reload Payload Shape and Ordering

Given browser phase selects CSS hot-reload path and builder context is available  
When critical and normal CSS are both rebuilt  
Then runtime MUST emit two payloads in order:

1. critical payload (`changeType=critical`) with base64-encoded critical CSS,
2. normal payload (`changeType=normal`) with current normal CSS URL.

Given only critical CSS is rebuilt  
When CSS hot-reload payload is emitted  
Then runtime MUST emit only critical payload.

Given only normal CSS is rebuilt  
When CSS hot-reload payload is emitted  
Then runtime MUST emit only normal payload.

Given CSS hot-reload path is selected but builder context is unavailable  
When browser phase executes  
Then runtime MUST skip CSS payload emission for that phase pass.

### BUILD-EVT-016: Browser-Phase No-Op in Non-Browser Mode

Given runtime is not using browser mode  
When browser phase executes  
Then browser phase MUST no-op and MUST NOT execute invalidate/reload/revalidate
or CSS hot-reload signaling paths.

### BUILD-EVT-017: Watcher Debounce Window Contract

Given watcher loop is running and fsnotify events arrive in rapid succession  
When debounce batching is applied  
Then watcher debounce window MUST be 30ms.

Window behavior refinement:

- events arriving within that 30ms window MUST be coalesced into one batch
  callback invocation,
- each new event arrival before timer fire MUST reset the debounce window for
  the pending batch.

### BUILD-EVT-018: Debouncer Shutdown Cleanup Contract

Given watcher loop exits due shutdown or closed watcher channels  
When deferred watcher cleanup runs  
Then debouncer stop logic MUST execute and clear pending timer/event state so no
delayed callback can run after watcher exit.

### BUILD-EVT-019: Event-Phase Blocking Failure Handling Contract

Given event-driven processing executes (single-event or batched path)  
When any blocking event-phase unit fails (for example pre/concurrent/post hook
execution, Go compile, public/private file processing, CSS build, filemap TS
write)  
Then runtime MUST treat that as actionable failure state rather than silent
success continuation:

- failure MUST be surfaced through event-flow control (not only error logs),
- success-oriented browser signaling for that failed event cycle (reload,
  revalidate, or CSS hot-reload success payloads) MUST NOT be emitted.

Given app process was already stopped for hard-reload-class work in that failed
event cycle  
When failure state is handled  
Then runtime MUST transition through explicit failure/retry policy rather than
continuing as if rebuild succeeded.

### BUILD-EVT-020: Concurrent-No-Wait Hook Isolation Contract

Given concurrent-no-wait hooks are configured for an event  
When callback or command execution in that phase fails  
Then failure MUST remain isolated to no-wait path:

- failure MUST be surfaced as diagnostics (warning-level logging),
- failure MUST NOT block event-flow progression for blocking phases,
- no-wait hook execution MUST remain fire-and-forget and non-blocking relative
  to pre/build/post/restart/browser orchestration.

### BUILD-EVT-021: Hook Command Token Resolution Contract

Given hook command token equals literal `DevBuildHook`  
When command execution is resolved for pre/concurrent/post/no-wait phases  
Then runtime MUST substitute current `Core.DevBuildHook` command value.

Given resolved command value is empty string  
When phase executes  
Then command execution path MUST be skipped for that hook entry.

Given hook command token is any value other than `DevBuildHook`  
When command execution is resolved  
Then runtime MUST execute the command token verbatim.

### BUILD-EVT-022: Event Classification Ignore-Gate Contract

Given fsnotify event has empty event-name path  
When classification runs  
Then event MUST be marked ignored and omitted from downstream event processing.

Given classified file type resolves to `other` and no watched-file pattern
matches that path  
When classification runs  
Then event MUST be marked ignored and omitted from downstream event processing.

### BUILD-EVT-023: Concurrent Restart-Action Strength Arbitration Contract

Given concurrent hook callbacks return multiple restart actions in one event
cycle (single-event or batch)  
When restart decision is derived from collected concurrent actions  
Then effective restart action MUST be deterministic and strongest-intent:

- if any action requests restart, effective outcome MUST restart,
- if any restart action requests `RecompileGo=true`, effective restart outcome
  MUST preserve `RecompileGo=true` (it MUST NOT be downgraded by weaker restart
  actions),
- effective restart decision MUST NOT depend on goroutine completion order.

### BUILD-EVT-024: Hook Timing Bucket and Default-Pre Classification Contract

Given watched-file hook sorting runs over `OnChangeHooks` entries  
When timing values are mapped into execution buckets  
Then bucket assignment MUST be:

- `post` -> post bucket,
- `concurrent` -> concurrent bucket,
- `concurrent-no-wait` -> concurrent-no-wait bucket,
- empty/unknown/unspecified timing -> pre bucket.

Given sorted hooks are consumed by event processing  
When phase execution runs  
Then bucketed hook sets MUST execute through corresponding phase order contract
(`BUILD-EVT-006`) with default-pre hooks participating in pre phase.

## 4.13 Static Asset and Filemap Contract

### BUILD-STATIC-001: Missing Static Source Behavior

Given static source directory does not exist  
When static processing runs  
Then filemap output MUST be emitted as empty map, and public mode MUST still
emit public filemap JS/ref outputs.

### BUILD-STATIC-002: Prehashed/Nohash Path Rules

Given file path under `prehashed/` or `nohash/` static subdirectory  
When processing runs  
Then output dist name MUST preserve original relative name (prefix stripped)
instead of content-hashed filename.

### BUILD-STATIC-003: Dist Name Strategy by Mode

Given static file processing mode  
When output dist name is selected  
Then:

- public hashed mode MUST use content-addressed name,
- private mode MUST preserve relative path,
- prehashed/nohash override MUST preserve relative path.

### BUILD-STATIC-004: Granular Rebuild Delta Rules

Given granular processing with prior filemap available  
When file content hash for a key is unchanged  
Then copy step MUST be skipped for that file.

Given previous output file no longer mapped or mapped dist name changes  
When granular cleanup runs  
Then stale prior dist file MUST be removed.

### BUILD-STATIC-005: Public Filemap JS/Ref Output Contract

Given public filemap write runs  
When outputs are emitted  
Then runtime MUST:

- generate JS export `wavePublicFileMap`,
- remove prior hashed filemap JS outputs matching filemap glob,
- atomically write ref file and new hashed JS file.

### BUILD-STATIC-006: Public Filemap TS/JSON Dev Output

Given `WritePublicFileMapTS` runs  
When output is written  
Then TS map keys MUST be sorted deterministically and companion JSON map MUST
also be written for Vite plugin dev invalidation.

### BUILD-STATIC-007: Atomic Write Primitive

Given atomic write helper runs  
When writing target file  
Then write MUST occur through temp-file + rename sequence in target directory,
with temp cleanup on failure.

### BUILD-STATIC-008: Non-Granular Static Cleanup Preserves Lock Files

Given non-granular file processing path runs before static generation  
When dist static directory is cleaned  
Then cleanup MUST remove prior entries except Wave lock files with `.wave-`
prefix, and then MUST recreate required dist directory structure.

### BUILD-STATIC-009: Content Hash Collision Guard Input

Given two static files share identical bytes but differ in normalized names  
When hashed output names are computed  
Then hash input MUST include normalized original name so resulting hashed output
names differ.

### BUILD-STATIC-010: Static Ignore Baseline

Given static file walk processes source tree  
When ignored-file rules apply  
Then `.DS_Store` entries MUST be excluded from copy and filemap outputs.

### BUILD-STATIC-011: Buildtime Public URL Lookup Helper Modes

Given buildtime public URL lookup runs through `MustGetPublicURLBuildtime`  
When public filemap load fails  
Then helper MUST fail fast (panic) instead of returning best-effort URL.

Given buildtime public URL lookup runs through `GetPublicURLBuildtime`  
When public filemap load fails  
Then helper MUST return:

- fallback URL computed from `PublicPathPrefix` + original path (leading-slash
  normalized),
- non-nil error value from failed filemap load,
- no panic.

Given either helper resolves a key not present in filemap  
When lookup completes  
Then returned URL MUST be fallback public-prefixed original URL.

### BUILD-STATIC-012: Public Filemap Load-Or-Build Fallback

Given helper filemap access runs through `loadOrBuildFileMap`  
When initial gob load of public filemap fails  
Then runtime MUST execute full file-processing fallback (`processFiles` with
`granular=false`, `isDev=false`) before retrying gob load.

Given fallback build step fails  
When helper exits  
Then returned error MUST indicate build-phase failure and MUST preserve wrapped
underlying error context.

### BUILD-STATIC-013: Public Filemap Key/Map Helper Filtering Contract

Given public filemap helper APIs produce exported key/map views  
When source filemap contains prehashed and non-prehashed entries  
Then:

- `PublicFileMapKeys` MUST include only non-prehashed keys and MUST sort keys
  deterministically,
- `SimplePublicFileMap` MUST include only non-prehashed entries and MUST map
  original key -> `DistName`.

### BUILD-STATIC-014: Public Asset Key Type-Generation Contract

Given TypeScript statement generation calls `AddPublicAssetKeys`  
When `statements` input is nil  
Then helper MUST allocate a new statements container.

Given public key retrieval fails  
When helper executes  
Then helper MUST fail fast (panic) instead of silently skipping type emission.

Given key retrieval succeeds  
When helper emits symbols  
Then output MUST define:

- serialized const `WAVE_PUBLIC_ASSETS` from public filemap keys,
- exported type `WavePublicAsset` that accepts both slash-prefixed and
  non-prefixed variants of those keys.

## 4.14 Config Schema Emission Contract

### BUILD-SCHEMA-001: Core and Conditional StaticAssetDirs Requirement

Given schema generation runs  
When root/core sections are emitted  
Then schema MUST require `Core`, and `Core.StaticAssetDirs` MUST be required
unless `Core.ServerOnlyMode=true`.

### BUILD-SCHEMA-002: Framework Schema Extension Merge

Given framework registered config schema extensions  
When schema generation runs  
Then extension sections MUST be merged into top-level schema properties.

### BUILD-SCHEMA-003: Schema Output Path and Formatting

Given schema generation succeeds  
When file is written  
Then output MUST be `<dist-internal>/schema.json` and SHOULD be pretty JSON with
trailing newline.

### BUILD-SCHEMA-004: Vite Section Required Child Validation

Given config includes `Vite` section  
When validation runs  
Then `Vite.JSPackageManagerBaseCmd` MUST be required and missing/empty value
MUST fail validation.

### BUILD-SCHEMA-005: RunOnChangeOnly Command Timing Validation

Given watch include item has `RunOnChangeOnly=true`  
When validation runs over command hooks  
Then hooks with `Cmd` MUST use default/explicit `pre` timing, and non-`pre`
timings MUST fail validation.

Callback-timing compatibility refinement:

- timing validation gate applies only to command hooks (`Cmd`),
- callback hooks (`Callback`) MUST NOT be rejected solely for non-`pre` timing
  values and remain eligible for event-phase execution semantics.

### BUILD-SCHEMA-006: Vorma Extension Shape and Defaults Contract

Given generated schema includes framework `Vorma` extension section  
When `Vorma` section is inspected  
Then schema MUST:

- require `MainBuildEntry`, `UIVariant`, `HTMLTemplateLocation`, `ClientEntry`,
  `ClientRouteDefsFile`, and `TSGenOutDir`,
- restrict `UIVariant` enum to `react`, `preact`, and `solid`,
- define optional `IncludeDefaults` with default `true`,
- define optional `BuildtimePublicURLFuncName` with default
  `waveBuildtimeURL`.

## 4.15 Builder Orchestration and Dist Priming Contract

### BUILD-BLD-001: Two-Pass File Processing Around Hooks

Given builder `Build(...)` executes with `FileOnlyMode=false`  
When pipeline runs  
Then runtime MUST:

1. process files before hooks using configured build granularity (`IsRebuild`),
2. execute build hooks,
3. process files again with granular mode enabled before schema/go-compile
   stages.

### BUILD-BLD-002: File-Only Mode Skips Hook/Schema/Compile Stages

Given builder `Build(...)` executes with `FileOnlyMode=true`  
When initial file-processing pass succeeds  
Then build MUST return without running build hooks, schema emission, or Go
compilation regardless of `CompileGo` flag value.

### BUILD-BLD-003: Go Compile Tag Selection by Mode

Given Go compile stage runs in dev mode  
When compile command is constructed  
Then command MUST compile without production build tags.

Given Go compile stage runs in non-dev mode  
When compile command is constructed  
Then command MUST include production tag set (`-tags=prod`).

### BUILD-BLD-004: Dist Setup Directory and Keep-File Contract

Given `SetupDistDir` executes  
When dist priming completes  
Then runtime MUST create internal/public/private dist directories and MUST write
the keep file used for go-embed compatibility.

### BUILD-BLD-005: Schema Write Failure Is Non-Fatal

Given build reaches schema-emission stage and schema write fails  
When failure is handled  
Then build MUST treat schema-write failure as warning-only and continue
subsequent pipeline stages (for example Go compile when enabled).

### BUILD-BLD-006: Browser-Mode Gate and Public-First File Processing Order

Given file-processing pipeline runs while browser mode is disabled  
When `processFiles` executes  
Then runtime MUST skip browser-asset processing stages (public-static copy,
private-static copy, and CSS builds).

Given file-processing pipeline runs while browser mode is enabled  
When `processFiles` executes  
Then runtime MUST process public static files first, and only after that stage
MAY execute private-static and CSS stages in parallel.

## 4.16 CSS Build Artifact and URL-Rewrite Contract

### BUILD-CSS-001: Missing CSS Entry Is a No-Op

Given critical or normal CSS entrypoint is unset/empty  
When CSS build stage for that nature executes  
Then stage MUST no-op successfully (no error and no forced output generation for
the missing entry).

### BUILD-CSS-002: Critical CSS Output Path Contract

Given critical CSS build stage succeeds  
When output artifact is written  
Then emitted artifact MUST be written to the internal dist location as
`critical.css`.

### BUILD-CSS-003: Normal CSS Hashed Output and Ref Pointer Contract

Given normal CSS build stage succeeds  
When output artifacts are written  
Then runtime MUST:

1. remove prior normal-CSS hashed artifacts matching configured glob pattern,
2. emit a newly hashed normal-CSS artifact in public static output,
3. atomically update normal-CSS ref file to the new hashed filename.

### BUILD-CSS-004: URL Resolver Rewrite Boundary Contract

Given CSS build resolves `url(...)` tokens  
When token is relative/asset-like path  
Then resolver MUST rewrite token through public-filemap buildtime URL lookup and
mark it external in emitted CSS bundle resolution graph.

Given token is absolute/protocol URL (for example `https://...`) or
protocol-relative URL (`//...`)  
When resolver evaluates token  
Then resolver MUST leave token unresolved by buildtime filemap rewriting.

### BUILD-CSS-005: Buildtime URL Lookup Failure Is Fail-Fast in CSS Pipeline

Given CSS URL rewrite path requires cached public filemap lookup and filemap load
fails  
When lookup helper executes  
Then CSS pipeline MUST fail fast (panic/error path) instead of silently emitting
unmapped URL output.

### BUILD-CSS-006: CSS Import Tracking Set Refresh and Path Normalization

Given critical or normal CSS build stage succeeds  
When esbuild metafile inputs are materialized for watcher classification  
Then runtime MUST refresh the corresponding import-tracking set from the current
metafile input graph and MUST normalize tracked file keys to absolute paths.

Given watcher classification checks whether a changed path belongs to tracked CSS
imports  
When path is present in critical/normal tracked set  
Then classification MUST treat it as the corresponding CSS file type for
event-priority resolution.

### BUILD-CSS-007: Dev/Prod CSS Minification Mode Contract

Given CSS build stage executes in dev mode  
When bundler options are configured  
Then CSS minification options MUST be disabled.

Given CSS build stage executes in non-dev mode  
When bundler options are configured  
Then CSS minification options for whitespace, identifiers, and syntax MUST be
enabled.

## 4.17 Runtime Config Validation Gate Contract

### BUILD-VAL-001: Build/Dev Entry Validation Gate

Given Wave build or dev entrypoint starts with invalid parsed config  
When `Build(...)` or `RunDev(...)` entry validation executes  
Then operation MUST fail before normal build/dev workflow proceeds.

### BUILD-VAL-002: Core Field Validation Contract

Given parsed config omits required core fields  
When runtime config validation executes  
Then validation MUST fail for missing `Core`, `Core.MainAppEntry`, or
`Core.DistDir`.

Given `Core.ServerOnlyMode=false`  
When runtime config validation executes  
Then validation MUST fail if `Core.StaticAssetDirs.Private` or
`Core.StaticAssetDirs.Public` is missing.

### BUILD-VAL-003: Optional Vite Block Validation Contract

Given parsed config includes `Vite` section  
When runtime config validation executes  
Then validation MUST fail if `Vite.JSPackageManagerBaseCmd` is empty/missing.

### BUILD-VAL-004: Config Parse Minimal-Safety and Dist-Root Normalization Contract

Given config JSON bytes are parsed through `ParseConfig`  
When payload is invalid JSON  
Then helper MUST fail with parse-class error.

Given payload parses but omits `Core` section  
When helper finalizes parsed config  
Then helper MUST fail before dist-layout derivation (no nil-core dereference
behavior).

Given payload includes `Core.DistDir`  
When parsed config is returned  
Then parsed `Dist.Root` MUST equal cleaned filesystem path derived from
`Core.DistDir`.

### BUILD-VAL-005: Config File Parse Delegation and Error-Wrapping Contract

Given `ParseConfigFile(path)` is called with unreadable/missing file  
When helper attempts file read  
Then helper MUST return read-class failure (no partial config output).

Given file read succeeds  
When helper returns parsed config  
Then returned parse behavior MUST be equivalent to calling `ParseConfig` on file
contents (including parse/minimal-safety/dist-root contracts).

### BUILD-VAL-006: ParsedConfig Helper Normalization/Default Contract

Given parsed config with `Core.PublicPathPrefix` as empty string or `/`  
When `PublicPathPrefix()` is evaluated  
Then helper MUST return `/`.

Given parsed config with `Core.PublicPathPrefix` as non-root path token `P`  
When `PublicPathPrefix()` is evaluated  
Then helper MUST return a leading-and-trailing-slash normalized form of `P`.

Given `Watch` section is absent or `Watch.WatchRoot` is empty  
When `WatchRoot()` is evaluated  
Then helper MUST return `"."`.

Given `Watch.WatchRoot` is set  
When `WatchRoot()` is evaluated  
Then helper MUST return cleaned filesystem path value of configured root.

Given `Watch` section is absent or `Watch.HealthcheckEndpoint` is empty  
When `HealthcheckEndpoint()` is evaluated  
Then helper MUST return `/`.

Given `Watch.HealthcheckEndpoint` is set  
When `HealthcheckEndpoint()` is evaluated  
Then helper MUST return configured endpoint unchanged.

Given CSS entry values are unset  
When `CriticalCSSEntry()` / `NonCriticalCSSEntry()` are evaluated  
Then each helper MUST return empty string for its unset entry.

Given CSS entry values are set  
When `CriticalCSSEntry()` / `NonCriticalCSSEntry()` are evaluated  
Then each helper MUST return cleaned filesystem path for its configured value.

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

### BDC-CLI-007 (covers BUILD-CLI-007)

Given CLI helper invocation variants with nil logger input  
When selected execution path initializes  
Then default `wave` color logger initialization MUST occur before delegated work.

### BDC-CLI-008 (covers BUILD-CLI-008)

Given `--hook` flag under callback-present and callback-nil variants  
When helper executes  
Then callback gating, dev-flag forwarding, and early-return vs standard mode
selection semantics MUST match contract.

### BDC-CLI-009 (covers BUILD-CLI-009)

Given hook/dev/prod delegated paths return failure under controlled fixtures  
When CLI helper exits  
Then panic-based failure propagation and production builder deferred-close
lifecycle MUST match contract.

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
Then route-definitions watch entry MUST be `RunOnChangeOnly` with callback flow.

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

### BDC-WATCH-013 (covers BUILD-WATCH-013)

Given repeated invocation fixtures for framework pattern/ignore append helpers
and public-filemap output-dir setter  
When effective config state is observed  
Then watch/ignore helpers MUST accumulate by append-in-call-order and
public-filemap output directory MUST reflect last-write-wins setter semantics.

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

## 5.10 Dev Server Control-Plane Scenarios

### BDC-DEV-001 (covers BUILD-DEV-001)

Given existing lock file with live owner PID  
When dev server startup is attempted  
Then startup MUST fail with lock-held signal.

Given existing stale lock owner PID  
When startup is attempted  
Then lock takeover MUST succeed.

### BDC-DEV-002 (covers BUILD-DEV-002)

Given browser-mode dev rebuild loop with at least one restart  
When refresh connectivity is observed before/after rebuild  
Then refresh server session MUST remain available across rebuild and cleanly
close on full shutdown.

### BDC-DEV-003 (covers BUILD-DEV-003)

Given config reload after initial run with framework-injected watch fields  
When reload completes  
Then framework-injected fields MUST remain present in active config.

### BDC-DEV-004 (covers BUILD-DEV-004)

Given sequential and non-sequential Go build modes  
When rebuild execution order is observed  
Then compile timing MUST follow configured concurrency policy.

### BDC-DEV-005 (covers BUILD-DEV-005)

Given rebuild failure  
When no file-change restart request arrives  
Then dev loop MUST stay in retry-wait state and MUST NOT spin rebuild attempts.

### BDC-DEV-006 (covers BUILD-DEV-006)

Given config-triggered restart  
When app returns ready  
Then reload broadcast MUST occur before watcher begins consuming new events for
that iteration.

### BDC-DEV-007 (covers BUILD-DEV-007)

Given mixed pending restart requests  
When stronger request is queued  
Then queue output MUST preserve upgrade/supersede semantics.

### BDC-DEV-008 (covers BUILD-DEV-008)

Given readiness checks for app/Vite URLs  
When targets are unavailable  
Then wait MUST terminate within bounded timeout budget rather than hanging.

### BDC-DEV-015 (covers BUILD-DEV-015)

Given dev loop has an existing active config and a subsequent reload attempt
fails  
When next rebuild iteration proceeds  
Then dev server process MUST continue running and build/watch behavior MUST
still execute under prior active config rather than exiting on reload failure.

### BDC-DEV-016 (covers BUILD-DEV-016)

Given invalidation helper runs once without active Vite and once with active
Vite context  
When request shape and outcome handling are inspected  
Then helper MUST fail fast for missing Vite context, and for active context
MUST use bounded-timeout `POST` to invalidate endpoint while treating transport
and non-`200` responses as failures.

### BDC-DEV-010 (covers BUILD-DEV-010)

Given browser-mode dev server startup  
When refresh server routes are inspected  
Then `/events` and `/get-refresh-script-inner` MUST be mounted with CORS `*`,
and refresh script endpoint MUST return JavaScript content type.

### BDC-DEV-011 (covers BUILD-DEV-011)

Given refresh manager shutdown  
When websocket upgrades and broadcast sends are exercised  
Then upgrades MUST reject with service-unavailable and broadcast logic MUST
terminate without shutdown deadlock.

### BDC-DEV-012 (covers BUILD-DEV-012)

Given browser reload path is exercised once with `cycleVite=true` and once with
`cycleVite=false`  
When refresh broadcast send behavior is inspected  
Then cycle-vite path MUST not emit Wave refresh payload, while non-cycle path
MUST emit it after readiness waits.

### BDC-DEV-013 (covers BUILD-DEV-013)

Given readiness helper execution against an always-unavailable endpoint and a
separate `200`-ready endpoint fixture  
When retry timing and termination behavior are observed  
Then unavailable path MUST terminate within documented bounded retry envelope
(attempt cap, per-request timeout, cumulative timeout/backoff budget), and
ready path MUST short-circuit immediately on first `200`.

### BDC-DEV-017 (covers BUILD-DEV-017)

Given refresh manager has at least one backpressured websocket client and one
ready client  
When a broadcast payload is emitted  
Then ready-client delivery MUST proceed without global broadcast blocking.

Given refresh manager is canceled with pending manager-channel work  
When manager shutdown completes  
Then active connections MUST be closed and pending channel work MUST be drained
before manager stop/wait completion returns.

### BDC-DEV-018 (covers BUILD-DEV-018)

Given lock acquisition under missing-directory, malformed-PID, stale-PID, live-PID,
and unreadable-lock variants  
When startup lock behavior is observed  
Then directory creation, stale takeover, live-owner rejection, and read-error
failure semantics MUST match contract.

Given lock-acquired dev run exits  
When deferred cleanup executes  
Then lock file MUST be removed.

### BDC-DEV-019 (covers BUILD-DEV-019)

Given platform-specific lock liveness probe implementations  
When Unix and Windows source variants are inspected  
Then non-Windows probe and Windows probe primitives MUST match documented
cross-platform contract.

### BDC-DEV-020 (covers BUILD-DEV-020)

Given config variants with omitted and explicit `Watch.WatchRoot` values  
When effective watch-root resolution is observed  
Then omitted case MUST resolve to `"."` and explicit case MUST resolve to
cleaned path semantics.

### BDC-DEV-021 (covers BUILD-DEV-021)

Given config variants with omitted and explicit `Watch.HealthcheckEndpoint`  
When readiness endpoint resolution is observed  
Then omitted case MUST resolve to `"/"` and explicit case MUST preserve
configured endpoint.

### BDC-DEV-022 (covers BUILD-DEV-022)

Given config variants spanning server-only on/off and Vite present/absent  
When mode helper outputs are inspected  
Then browser and Vite mode helpers MUST match documented derivation contract.

### BDC-DEV-023 (covers BUILD-DEV-023)

Given env variants spanning dev/non-dev, latched/non-latched flag, valid/invalid
`PORT`, and free-port probe success/failure  
When app-port resolution behavior is exercised  
Then default/probe/fallback behavior, env side effects, and one-time latching
MUST match contract, including bounded `+1024` scan and random-fallback/error
branches when primary candidate is unavailable.

### BDC-DEV-024 (covers BUILD-DEV-024)

Given environment variants for `WAVE_MODE` and explicit `SetModeToDev()` call  
When mode helper behavior is observed  
Then `GetIsDev()` and `SetModeToDev()` MUST match documented environment
contract.

### BDC-DEV-025 (covers BUILD-DEV-025)

Given dev and non-dev mode fixtures with explicit/missing refresh-server port
values  
When refresh-script helper output is observed  
Then non-dev output MUST be empty and dev output MUST use configured port or
default `10000` fallback.

### BDC-DEV-026 (covers BUILD-DEV-026)

Given dev-mode fixture with resolved refresh-script port `P`  
When refresh script and script-hash helper outputs are sampled  
Then hash value MUST match base64 SHA-256 of `RefreshScriptInner(P)`.

### BDC-DEV-027 (covers BUILD-DEV-027)

Given refresh script receives repeated `rebuilding` websocket payloads  
When DOM state is observed after each payload  
Then rebuilding overlay element id `wave-refreshscript-rebuilding` MUST be
singleton and overlay visibility transition MUST follow fade-in contract.

### BDC-DEV-028 (covers BUILD-DEV-028)

Given refresh script receives `other` payload with positive and zero scroll
variants plus startup fixture with persisted refresh-scroll key  
When reload and startup paths are exercised  
Then reload MUST always trigger, positive-scroll variant MUST persist key, and
startup restore path MUST consume key and attempt scroll restoration.

### BDC-DEV-029 (covers BUILD-DEV-029)

Given refresh script receives `normal` and `critical` payload variants with and
without existing target CSS elements  
When DOM updates are observed  
Then normal-css and critical-css update behavior MUST follow documented id-based
insert/replace/remove semantics.

### BDC-DEV-030 (covers BUILD-DEV-030)

Given refresh script receives `revalidate` payload under helper-present and
helper-rejecting, and helper-absent variants  
When handler behavior is observed  
Then helper-present case MUST invoke `window.__waveRevalidate` and remove
overlay after successful completion, helper-rejecting case MUST log failure and
still remove overlay, and helper-absent case MUST log error and still remove
overlay.

### BDC-DEV-031 (covers BUILD-DEV-031)

Given refresh websocket close/error/unload lifecycle variants  
When handlers are exercised  
Then close/error variants MUST trigger reload fallback, and unload variant MUST
suppress close-triggered reload before socket close.

### BDC-DEV-032 (covers BUILD-DEV-032)

Given Vite startup failure on initial dev start and on cycle-restart path  
When dev control loop behavior is observed  
Then failure MUST be surfaced as actionable dev-loop failure state and runtime
MUST avoid reporting/using a false-ready Vite state until startup succeeds.

### BDC-DEV-033 (covers BUILD-DEV-033)

Given build loop is paused in retry-wait after a failure and restart requests
arrive with differing strengths  
When retry iteration resumes  
Then resumed iteration MUST honor strongest effective restart request flags
(`recompileGo` and config-restart intent) rather than treating retry requests
as equivalent wake-up signals.

### BDC-DEV-034 (covers BUILD-DEV-034)

Given app binary startup is forced into process-start failure on a dev iteration  
When control-loop behavior is observed  
Then failure MUST enter actionable startup-failure handling and runtime MUST NOT
proceed as if app is running (no false-success running iteration semantics)
until startup succeeds.

### BDC-DEV-035 (covers BUILD-DEV-035)

Given reload path requests app and/or Vite readiness waits and readiness
polling times out  
When orchestration outcome is observed  
Then control flow MUST enter explicit readiness-failure handling and MUST NOT
continue as if readiness gate succeeded for the timed-out dependency.

### BDC-DEV-036 (covers BUILD-DEV-036)

Given active dev config includes framework-injected watch/ignore/public-map,
schema-extension, and framework build-hook fields before reload  
When post-first-run config reload succeeds and next build iteration executes  
Then all framework-injected fields MUST remain present in active config, and
schema emission plus framework hook invocation behavior MUST remain intact.

### BDC-DEV-037 (covers BUILD-DEV-037)

Given restart-wait phase fixtures for override env unset/invalid/non-positive,
positive-without-restart, and positive-with-restart-before-timeout  
When control-loop outcomes are observed  
Then wait behavior MUST match override contract: disabled timer for invalid
inputs, clean success exit on positive timeout, and restart-precedence when a
restart request arrives before timer expiration.

### BDC-DEV-038 (covers BUILD-DEV-038)

Given equivalent fixtures for alias call path (`MustGetAppPort`) and canonical
call path (`MustGetPort`) across dev/non-dev and latched/non-latched variants  
When outputs and env side effects are compared  
Then alias path MUST remain strictly equivalent to canonical helper behavior.

## 5.12 Watch/Event Processing Scenarios

### BDC-EVT-001 (covers BUILD-EVT-001)

Given duplicate fsnotify path events within one debounce window  
When batch executes  
Then path MUST be processed once while preserving content-change signal
equivalence for that path (for example trailing chmod-only events MUST NOT mask
earlier write/create/remove/rename events in the same batch).

### BDC-EVT-002 (covers BUILD-EVT-002)

Given config file write/create event in batch  
When processing begins  
Then config restart MUST short-circuit remaining batch handling.

### BDC-EVT-003 (covers BUILD-EVT-003)

Given chmod-only event on non-empty file  
When classifier runs  
Then event MUST be ignored.

### BDC-EVT-004 (covers BUILD-EVT-004)

Given path matching multiple type candidates  
When file type resolves  
Then classification priority and `TreatAsNonGo` override MUST match contract.

### BDC-EVT-005 (covers BUILD-EVT-005)

Given path matching framework and user watched-file entries  
When effective watched config is computed  
Then merge result MUST follow union/trump/hook-order semantics.

### BDC-EVT-006 (covers BUILD-EVT-006)

Given single event with hooks across all timings  
When processing runs  
Then hook/build/app/browser phases MUST execute in required order.

### BDC-EVT-007 (covers BUILD-EVT-007)

Given multi-event batch with at least one hard-reload requirement  
When processing runs without all-run-on-change-only short-circuit finalization  
Then app stop MUST happen once up front and browser phase MUST emit once.

### BDC-EVT-008 (covers BUILD-EVT-008)

Given work-set combinations spanning restart/revalidate/css/static updates  
When browser action resolves  
Then selected action MUST follow precedence contract.

### BDC-EVT-009 (covers BUILD-EVT-009)

Given Vite invalidate endpoint call fails  
When browser phase continues  
Then fallback MUST become hard reload with readiness waits.

### BDC-EVT-010 (covers BUILD-EVT-010)

Given `RunOnChangeOnly=true`  
When event processing runs  
Then standard build phase MUST be skipped while callback-returned refresh
actions (restart/reload/wait flags) remain effective in final event outcome,
and callback hooks configured for no-wait/pre/concurrent/post timing MUST still
execute, and app process MUST remain running when callback actions do not
request restart.

Given mixed event batch containing both `RunOnChangeOnly=true` and
non-`RunOnChangeOnly` entries  
When per-entry callback phases execute  
Then callback hooks for the run-on-change-only entry MUST still execute for
supported timing phases (no-wait/pre/concurrent/post) and MUST NOT be skipped
just because another batch entry requires standard build work.

### BDC-EVT-011 (covers BUILD-EVT-011)

Given directory create/rename event under watch root  
When event batch is processed  
Then new subtree MUST be added to watcher before file-level handling continues.

### BDC-EVT-012 (covers BUILD-EVT-012)

Given batches with mixed/CSS-only/skip-notification combinations  
When rebuilding broadcast decision runs  
Then rebuilding signal MUST emit only for qualifying non-CSS non-skipped work.

### BDC-EVT-013 (covers BUILD-EVT-013)

Given multi-event batch where all effective entries are run-on-change-only  
When batch processing runs  
Then build phase MUST be skipped while merged callback refresh actions
(restart/reload/wait flags) remain effective in final batch outcome, and
callback hooks for supported timing phases across batch entries MUST still
execute before outcome finalization, and app process MUST not be left stopped
when merged callback actions do not request restart.

Given mixed multi-event batch that includes at least one run-on-change-only
entry but not all entries are run-on-change-only  
When batch processing runs  
Then run-on-change-only entries MUST still execute callback hooks for supported
timing phases as per-entry behavior, even though standard build work is
performed for other entries.

### BDC-EVT-014 (covers BUILD-EVT-014)

Given one browser-phase case where invalidate succeeds and one where Vite is
disabled  
When resulting browser actions are observed  
Then successful invalidate case MUST short-circuit without additional reload or
CSS/revalidate payloads, and non-Vite case MUST convert to hard reload with app
wait semantics.

### BDC-EVT-015 (covers BUILD-EVT-015)

Given CSS hot-reload cases for both-CSS, critical-only, normal-only, and
builder-unavailable variants  
When payload emissions are observed  
Then payload count, ordering, and field shape (`critical` base64 body vs
`normal` URL) MUST match contract, and builder-unavailable case MUST emit no
CSS payload.

### BDC-EVT-016 (covers BUILD-EVT-016)

Given runtime is configured without browser mode  
When browser phase executes for work-set variants  
Then invalidate/reload/revalidate/CSS payload signaling MUST not run.

### BDC-EVT-017 (covers BUILD-EVT-017)

Given watcher loop construction is inspected  
When debounce helper is instantiated  
Then debounce interval MUST be 30ms and event arrivals inside that interval
MUST be batch-coalesced under timer-reset semantics.

### BDC-EVT-018 (covers BUILD-EVT-018)

Given watcher loop shutdown path is inspected  
When loop exits  
Then deferred debouncer stop MUST run and clear timer/pending-event state so no
post-shutdown callback flush is possible.

### BDC-EVT-019 (covers BUILD-EVT-019)

Given event-driven processing is forced into blocking-hook failures and
build-unit failures across single-event and batched variants  
When event-flow outcomes are observed  
Then failure MUST become actionable event-flow failure state and success
browser-signaling payloads for that failed cycle MUST NOT be emitted.

### BDC-EVT-020 (covers BUILD-EVT-020)

Given concurrent-no-wait callbacks/commands fail during event processing  
When hook outcomes and event-flow progression are observed  
Then failures MUST be warning-diagnostic only and MUST NOT block later blocking
event phases.

### BDC-EVT-021 (covers BUILD-EVT-021)

Given hook command entries include literal `DevBuildHook`, explicit command
strings, and empty resolved command values  
When pre/concurrent/post/no-wait execution paths resolve commands  
Then `DevBuildHook` entries MUST execute current `Core.DevBuildHook`, explicit
commands MUST remain verbatim, and empty resolved command values MUST skip
command execution.

### BDC-EVT-022 (covers BUILD-EVT-022)

Given one event with empty path and one `other`-type event with no watched-file
match  
When classification/filtering is applied before event handling  
Then both events MUST be treated as ignored and MUST NOT enter downstream event
processing flow.

### BDC-EVT-023 (covers BUILD-EVT-023)

Given concurrent hook fixtures that return mixed restart actions (including
`RecompileGo=true` and `RecompileGo=false`) under varying goroutine completion
orders  
When effective restart decision is observed for single-event and batch paths  
Then outcome MUST be deterministic and preserve strongest restart strength
(`RecompileGo=true` wins over weaker restart actions).

### BDC-EVT-024 (covers BUILD-EVT-024)

Given watched-hook fixtures spanning explicit post/concurrent/concurrent-no-wait
timings plus empty/unknown timing values  
When hook sorting and subsequent phase execution are observed  
Then bucket assignment and default-pre timing behavior MUST match contract.

## 5.13 Static Asset and Filemap Scenarios

### BDC-STATIC-001 (covers BUILD-STATIC-001)

Given missing static source directory  
When static processing runs  
Then empty filemap output MUST be emitted with public map outputs.

### BDC-STATIC-002 (covers BUILD-STATIC-002)

Given files under `prehashed/` and `nohash/`  
When static outputs are generated  
Then dist names MUST preserve original relative names.

### BDC-STATIC-003 (covers BUILD-STATIC-003)

Given public/private/prehashed static files  
When dist names are assigned  
Then naming strategy MUST match mode-specific rules.

### BDC-STATIC-004 (covers BUILD-STATIC-004)

Given granular rebuild with unchanged and removed files  
When delta processing runs  
Then unchanged copies MUST skip and stale outputs MUST be removed.

### BDC-STATIC-005 (covers BUILD-STATIC-005)

Given public filemap write with prior hashed outputs present  
When write completes  
Then old hashed map artifacts MUST be removed and new ref/JS outputs MUST be
atomically updated.

### BDC-STATIC-006 (covers BUILD-STATIC-006)

Given TS/JSON public filemap dev output generation  
When output is inspected  
Then TS keys MUST be deterministic and JSON companion MUST be present.

### BDC-STATIC-007 (covers BUILD-STATIC-007)

Given atomic write helper under success/failure cases  
When filesystem side effects are inspected  
Then final write MUST use temp+rename and failure MUST not leave leaked temp
files.

### BDC-STATIC-008 (covers BUILD-STATIC-008)

Given non-granular file processing start on populated dist static directory  
When cleanup is applied  
Then non-lock entries MUST be removed, `.wave-*` lock entries MUST remain, and
required dist directories MUST be recreated.

### BDC-STATIC-009 (covers BUILD-STATIC-009)

Given same-content static files with different normalized names  
When hashed output names are generated  
Then emitted hashed names MUST differ.

### BDC-STATIC-010 (covers BUILD-STATIC-010)

Given static source contains `.DS_Store` alongside real assets  
When static processing runs  
Then `.DS_Store` MUST be absent from filemap and dist output set.

### BDC-STATIC-011 (covers BUILD-STATIC-011)

Given buildtime public URL lookup helper variants under filemap-load failure and
missing-key paths  
When observable outputs are compared  
Then panic/error-return modes and fallback URL resolution MUST match contract.

### BDC-STATIC-012 (covers BUILD-STATIC-012)

Given helper filemap access starts with missing/unreadable public filemap gob  
When load-or-build fallback executes  
Then full file-processing fallback + retry semantics and wrapped build-failure
error behavior MUST match contract.

### BDC-STATIC-013 (covers BUILD-STATIC-013)

Given mixed prehashed and non-prehashed entries in source filemap  
When helper key/map APIs are evaluated  
Then non-prehashed-only filtering and deterministic key ordering MUST hold.

### BDC-STATIC-014 (covers BUILD-STATIC-014)

Given public asset key TS generation helper under nil/non-nil statement input and
error/success key-loading paths  
When emitted declarations are inspected  
Then nil-allocation, fail-fast behavior, and emitted const/type shape MUST match
contract.

## 5.14 Config Schema Emission Scenarios

### BDC-SCHEMA-001 (covers BUILD-SCHEMA-001)

Given generated schema  
When `Core.ServerOnlyMode` condition is toggled in validation checks  
Then `Core.StaticAssetDirs` requirement MUST be conditional per contract.

### BDC-SCHEMA-002 (covers BUILD-SCHEMA-002)

Given framework schema extensions configured  
When schema is generated  
Then extension sections MUST appear at top-level properties.

### BDC-SCHEMA-003 (covers BUILD-SCHEMA-003)

Given schema generation succeeds  
When output file is inspected  
Then path/formatting contract (`<dist-internal>/schema.json`, pretty + newline)
MUST hold.

### BDC-SCHEMA-004 (covers BUILD-SCHEMA-004)

Given config with and without `Vite.JSPackageManagerBaseCmd`  
When validation runs  
Then missing/empty command MUST fail and present command MUST pass this gate.

### BDC-SCHEMA-005 (covers BUILD-SCHEMA-005)

Given `RunOnChangeOnly=true` include with command hooks using varied timings  
When validation runs  
Then non-`pre` command timings MUST fail while `pre` (or default) passes, and
callback hooks with non-`pre` timing MUST remain validation-legal.

### BDC-SCHEMA-006 (covers BUILD-SCHEMA-006)

Given generated schema output  
When `Vorma` extension section is inspected  
Then required fields, `UIVariant` enum values, and documented default values for
`IncludeDefaults` and `BuildtimePublicURLFuncName` MUST match contract.

## 5.15 Builder Orchestration and Dist Priming Scenarios

### BDC-BLD-001 (covers BUILD-BLD-001)

Given standard build path with observable hook-generated static artifact  
When build executes  
Then file processing MUST occur before hooks and rerun after hooks so
hook-generated artifact is present in final filemap/dist outputs.

### BDC-BLD-002 (covers BUILD-BLD-002)

Given build is executed with `FileOnlyMode=true` and `CompileGo=true`  
When pipeline completes  
Then hook execution, schema write, and Go compile stages MUST not run.

### BDC-BLD-003 (covers BUILD-BLD-003)

Given one dev compile case and one non-dev compile case  
When compile command invocation is observed  
Then dev compile MUST omit production tags and non-dev compile MUST include
`-tags=prod`.

### BDC-BLD-004 (covers BUILD-BLD-004)

Given empty dist tree  
When `SetupDistDir` runs  
Then required dist subdirectories and keep file MUST exist and keep file content
MUST be non-empty.

### BDC-BLD-005 (covers BUILD-BLD-005)

Given schema-write stage is forced to fail while remaining build stages are
otherwise valid  
When build executes  
Then schema failure MUST be warning-only and build MUST continue to subsequent
pipeline stage behavior.

### BDC-BLD-006 (covers BUILD-BLD-006)

Given one fixture with browser mode disabled and one with browser mode enabled  
When file-processing phase execution order is observed  
Then non-browser fixture MUST skip public/private/CSS processing stages, while
browser fixture MUST run public-file processing before entering private/CSS
processing paths.

## 5.16 CSS Build Artifact and URL-Rewrite Scenarios

### BDC-CSS-001 (covers BUILD-CSS-001)

Given one build fixture with CSS entry configured and one without  
When CSS build stages execute  
Then missing-entry stage MUST return success no-op while configured-entry stage
still produces its expected artifacts.

### BDC-CSS-002 (covers BUILD-CSS-002)

Given critical CSS entrypoint is configured  
When build completes critical CSS stage  
Then internal dist output MUST contain `critical.css` content for the current
build.

### BDC-CSS-003 (covers BUILD-CSS-003)

Given normal CSS stage runs across two successive builds with changed content  
When second build completes  
Then old normal hashed output MUST be removed, new hashed output MUST exist, and
normal ref pointer MUST target the new hashed filename.

### BDC-CSS-004 (covers BUILD-CSS-004)

Given CSS source containing relative asset URL plus absolute/protocol-relative
URLs  
When bundle output and resolver behavior are inspected  
Then relative token MUST route through filemap lookup rewrite while
absolute/protocol-relative tokens MUST remain exempt from rewrite path.

### BDC-CSS-005 (covers BUILD-CSS-005)

Given CSS URL rewrite path executes with forced public-filemap load failure  
When helper behavior is observed  
Then build MUST fail fast rather than silently continuing with unresolved mapped
asset lookup path.

### BDC-CSS-006 (covers BUILD-CSS-006)

Given successive CSS builds with changed import graph and watcher classification
fixtures  
When tracked CSS-import sets are inspected and then used by file-type
classification  
Then tracked sets MUST reflect current build metafile inputs using absolute-path
keys, and classification MUST recognize changed files in those sets as
critical/normal CSS events.

### BDC-CSS-007 (covers BUILD-CSS-007)

Given one dev-mode CSS build and one non-dev CSS build  
When bundler option contracts are inspected  
Then dev build MUST disable minification flags, and non-dev build MUST enable
whitespace/identifier/syntax minification flags.

## 5.17 Runtime Config Validation Gate Scenarios

### BDC-VAL-001 (covers BUILD-VAL-001)

Given one invalid config fixture for builder path and one for dev path  
When `Build(...)` and `RunDev(...)` are invoked  
Then each invocation MUST fail at validation gate before normal workflow.

### BDC-VAL-002 (covers BUILD-VAL-002)

Given config variants missing `Core`, `Core.MainAppEntry`, `Core.DistDir`, and
browser-mode static dir children  
When runtime validation executes  
Then each invalid variant MUST fail with core-field validation error class.

### BDC-VAL-003 (covers BUILD-VAL-003)

Given config variants with `Vite` block present but
`JSPackageManagerBaseCmd` missing/empty  
When runtime validation executes  
Then validation MUST fail that Vite gate.

### BDC-VAL-004 (covers BUILD-VAL-004)

Given config-byte fixtures for invalid JSON, missing `Core`, and messy
`Core.DistDir` path variants  
When `ParseConfig` runs  
Then parse failure, missing-core failure, and cleaned `Dist.Root` derivation
MUST match contract.

### BDC-VAL-005 (covers BUILD-VAL-005)

Given file-path fixtures for unreadable config file and readable valid config
file  
When `ParseConfigFile` runs  
Then unreadable case MUST fail with read-class error, and readable case MUST
match `ParseConfig`-equivalent output semantics.

### BDC-VAL-006 (covers BUILD-VAL-006)

Given parsed-config fixtures covering root/non-root public path prefix,
missing/present watch root, missing/present healthcheck endpoint, and
empty/non-empty CSS entries  
When helper outputs are sampled  
Then normalization/default behavior for `PublicPathPrefix()`, `WatchRoot()`,
`HealthcheckEndpoint()`, `CriticalCSSEntry()`, and `NonCriticalCSSEntry()`
MUST match contract.

## 6. Conformance Test Guidance

Build/dev conformance suites SHOULD:

1. Keep each requirement ID and scenario ID independently reportable.
2. Cover CLI matrix (`--dev`, `--hook`, `--no-binary`) with observable phase assertions.
3. Validate artifact existence/content shape for stage1/stage2/manifest/TS outputs.
4. Validate fast route rebuild callbacks and fallback restart behavior.
5. Include route DSL negative cases (dynamic module expressions, missing modules).
6. Validate Vite integration contracts (naming prefix, transform rewriting, invalidation endpoint).
7. Validate deterministic behavior (TS write skipping and build ID sensitivity to declared inputs).
8. Validate dev control-plane invariants (single-instance lock, restart queue upgrade semantics, config-restart ordering, readiness timeout bounds).
9. Validate watch/event semantics (classification priority, hook phase ordering, run-on-change-only behavior, Vite-invalidate fallback).
10. Validate static/filemap contracts (prehashed/nohash handling, granular delta behavior, deterministic TS/JSON filemap outputs, atomic-write discipline).
11. Track traceability with IDs (`BUILD-*`, `BDC-*`) in CI output.

## 7. Relation to Other Specs

- Backend runtime behavior:
  `/Users/sjc/__code/river/specs/VORMA_BACKEND_RUNTIME_SPEC.md`
- Wire protocol behavior:
  `/Users/sjc/__code/river/specs/VORMA_WIRE_CONTRACT_SPEC.md`
- Frontend runtime behavior:
  `/Users/sjc/__code/river/specs/VORMA_FRONTEND_RUNTIME_SPEC.md`
- Kit interop boundaries: `/Users/sjc/__code/river/specs/VORMA_KIT_INTEROP_SPEC.md`
- Testing strategy:
  `/Users/sjc/__code/river/specs/VORMA_TESTING_STRATEGY_SPEC.md`
- Overall roadmap: `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`
