# wave/tooling Specification

Status: Active  
Last Updated: 2026-02-09  
Applies To: `wave/tooling` package behavior (build/dev control plane)

## 1. Purpose

This is the canonical contract for Wave build/dev control-plane behavior
implemented in `wave/tooling/*`.

Framework specs (including Vorma) SHOULD reference these requirement IDs instead
of duplicating `wave/tooling` internals.

## 2. Ownership Boundary

`wave/tooling` owns:

- CLI build/dev entry helper behavior,
- dev control-plane restart/readiness lifecycle,
- watcher/event-loop semantics,
- static/CSS artifact processing control-plane behavior,
- schema-merge behavior for framework extensions.

Frameworks own:

- framework-specific callbacks injected into tooling,
- framework-specific endpoint interactions and resulting app-visible behavior.

Conflict rule:

- if a consumer alias requirement conflicts with this spec for a
  `wave/tooling`-owned concern, this spec is authoritative.

## 3. Requirement Catalog

### 3.1 CLI Entry Contracts

#### WAVE-CLI-001: CLI Logger Defaulting Contract

Given CLI entry helper `BuildWaveWithHook` or wrapper `BuildWave` is invoked
with nil logger  
When entrypoint initializes runtime dependencies  
Then helper MUST construct default color logger named `wave` before invoking
dev/build flows.

#### WAVE-CLI-002: Hook Callback Gate and Precedence Contract

Given CLI flags include `--hook` and a non-nil hook callback is provided  
When entry helper executes  
Then helper MUST invoke callback exactly once with parsed `--dev` value and MUST
return immediately without entering `RunDev` or builder build path.

Given CLI flags include `--hook` but callback is nil  
When entry helper executes  
Then helper MUST ignore hook-only callback path and continue through standard
`--dev`/production flow selection.

#### WAVE-CLI-003: CLI Error Propagation and Builder Lifecycle Contract

Given hook callback, `RunDev`, or production builder path returns error  
When CLI helper handles that failure  
Then helper MUST fail fast via panic (no error return contract).

Given production builder path is selected  
When helper completes (success or panic unwind)  
Then builder cleanup (`Close`) MUST be deferred for execution.

### 3.2 Dev Control Plane

#### WAVE-DEV-012: Cycle-Vite Reload Source Exclusivity

Cycle-vite reload orchestration MUST have one authoritative browser reload
source per cycle and MUST NOT emit duplicate reload triggers.

#### WAVE-DEV-030: Revalidate Overlay-Cleanup Robustness

Refresh-script revalidate flow MUST clear rebuilding overlay for both resolve
and reject outcomes.

#### WAVE-DEV-032: Vite Startup Failure Handling

Vite startup failure in dev loop MUST enter explicit failure handling (not
log-only success continuation).

#### WAVE-DEV-033: Build-Retry Request Strength Preservation

Build-retry wake-up handling MUST preserve restart request strength bits
(`recompileGo`, config-restart intent).

#### WAVE-DEV-034: App Startup Failure Handling

App startup failure MUST propagate as actionable orchestration failure.

#### WAVE-DEV-035: Readiness-Gate Failure Handling

App/Vite readiness timeout/failure outcomes MUST be actionable and MUST gate
reload/cycle success continuation.

#### WAVE-DEV-036: Config Reload Framework-Field Preservation

Config reload replacement MUST preserve framework-injected runtime-only fields
required for control-plane correctness.

#### WAVE-DEV-041: App-Stop Failure Propagation

App stop (`Kill`/`Wait`) failures MUST be surfaced to callers.

### 3.3 Watch and Event Processing

#### WAVE-EVT-001: Batch Dedupe Must Preserve Strongest Effective Work

Event dedupe MUST preserve non-lossy effective work semantics, including
unmatched-event distinctness and strongest-intent aggregation.

#### WAVE-EVT-019: Blocking Event-Phase Failure Gating

Blocking phase failures (hooks/build units) MUST gate success-phase continuation
and success-style browser signaling.

#### WAVE-EVT-023: Concurrent Restart Arbitration Determinism

Concurrent restart actions MUST resolve deterministically to strongest intent.

#### WAVE-EVT-028: Directory Watch-Expansion Failure Handling

Dynamic watch-expansion failure MUST be actionable and MUST NOT silently
continue as success.

#### WAVE-EVT-029: CSS Hot-Reload Artifact Read-Failure Handling

CSS artifact read failures in hot-reload path MUST be actionable and MUST
suppress success-style CSS payload emission.

#### WAVE-EVT-030: Implicit-vs-Callback Restart Strength Preservation

Implicit rebuild work MUST NOT be downgraded by weaker callback restart actions.

#### WAVE-EVT-031: Stable Watcher Channel Source Under Teardown

Watch-loop channel consumption MUST use a stable watcher source under teardown
races.

#### WAVE-EVT-032: Stale-Watch Removal Failure Handling

Stale watch removal failures MUST be explicit and tracked-state updates MUST
stay consistent with actual removal outcome.

#### WAVE-EVT-033: Closed Error-Channel Watch-Loop Termination

Closed watcher error-channel receive MUST terminate watcher loop (not nil-error
spin/log behavior).

### 3.4 Static, CSS, and Schema

#### WAVE-STATIC-005: Filemap Artifact Rotation Cleanup Guarantees

Filemap artifact rotation MUST obey explicit cleanup-failure policy (actionable
failure or explicitly bounded tolerated-stale policy).

#### WAVE-STATIC-015: Granular Stale-Artifact Removal Failure Handling

Granular stale artifact removal failures MUST be explicit and policy-aligned.

#### WAVE-CSS-003: Normal CSS Rotation Cleanup Guarantees

Normal CSS hashed artifact rotation MUST obey explicit cleanup-failure policy.

#### WAVE-SCHEMA-007: Reserved Schema-Key Collision Guard

Framework schema extensions MUST NOT silently override reserved top-level Wave
schema sections.

### 3.5 Builder and Config Processing

#### WAVE-BUILD-001: Config Validation Gate

Given `ValidateConfig` is invoked for build/dev startup  
When required core fields are missing (`Core`, `Core.MainAppEntry`,
`Core.DistDir`)  
Then validation MUST fail before build/dev execution continues.

Given `Core.ServerOnlyMode` is false  
When validating static paths  
Then `Core.StaticAssetDirs.Private` and `Core.StaticAssetDirs.Public` MUST be
required.

Given Vite config is present  
When validating config  
Then `Vite.JSPackageManagerBaseCmd` MUST be required.

Given a watched include entry has `RunOnChangeOnly=true`  
When any `OnChangeHooks[*].Cmd` uses non-`pre` timing  
Then validation MUST fail.

#### WAVE-BUILD-002: Build Pipeline Ordering and File-Only Behavior

Given `Builder.Build(opts)` executes  
When build starts  
Then config validation MUST run before file processing, hooks, schema write, or
Go compilation.

Given build enters main pipeline  
When `Build` executes  
Then first file-processing phase MUST run before hooks and second post-hook
file-processing phase.

Given `opts.FileOnlyMode=true`  
When first file-processing phase succeeds  
Then build MUST return without running hooks, schema writing, or Go compilation.

Given `opts.FileOnlyMode=false` and compile is enabled  
When build runs  
Then schema write failures MUST be warning-only (non-fatal), while Go
compilation failures MUST fail build.

#### WAVE-BUILD-003: Build Hook Selection, Order, and Failure Semantics

Given `runHooks` executes in dev or prod mode  
When both user and framework hooks are configured  
Then user hook MUST execute before framework hook.

Given selected hook command fails  
When `runHooks` processes hooks  
Then failure MUST abort the hook phase and propagate as build failure.

#### WAVE-BUILD-004: Non-Granular File Processing Cleanup Discipline

Given `processFiles(granular=false, ...)` executes  
When dist static directory already has entries  
Then processing MUST remove prior entries except Wave lock files before
recreating dist structure.

Given dist structure setup is required  
When non-granular file processing runs  
Then `SetupDistDir` MUST run and MUST write the `.keep` embed sentinel file.

#### WAVE-BUILD-005: Dev-Loop Go Compile Scheduling Contract

Given dev loop restart handling requires Go recompilation  
When `Core.SequentialGoBuild` is false  
Then Go compilation MUST run concurrently with the build phase.

Given dev loop restart handling requires Go recompilation  
When `Core.SequentialGoBuild` is true  
Then Go compilation MUST run only after build phase succeeds.

#### WAVE-BUILD-006: Static Source Absence Fallback Contract

Given public/private static source directory does not exist  
When static processing runs for that source  
Then tooling MUST write an empty file-map artifact and continue without failing
build.

Given missing source is public static  
When static processing completes  
Then tooling MUST also refresh public file-map JS output from the empty map.

### 3.6 Watcher, Lock, and Build-Time URL Helpers

#### WAVE-WATCH-001: Watch Pattern Normalization and Pre-Sort Contract

Given watcher initialization runs  
When patterns are loaded from framework/user config  
Then watcher MUST normalize paths to absolute forward-slash form before matching
and MUST pre-sort hook timing groups before event processing.

Given watcher ignore defaults are initialized  
When watch set is assembled  
Then dist static output, `.git`, and `node_modules` ignore patterns MUST be
included.

#### WAVE-WATCH-002: Multi-Match WatchedFile Merge Semantics

Given multiple watched-file configs match one path  
When watcher resolves effective config  
Then merged behavior MUST be strongest-work preserving (`RecompileGoBinary` /
`RestartApp` OR semantics, skip-style flags merged with conservative semantics)
and hooks MUST preserve framework-before-user ordering.

#### WAVE-LOCK-001: Single-Instance Dev Lock Discipline

Given dev startup acquires lock at `dist/static/.wave-dev.lock`  
When lock file references a live PID  
Then acquisition MUST fail with lock-held error.

Given lock file is stale or malformed  
When lock acquisition runs  
Then startup MUST overwrite lock with current PID and continue.

#### WAVE-URL-001: Build-Time Public URL Resolution Fallback Semantics

Given build-time URL resolution cannot load public file-map artifact  
When `GetPublicURLBuildtime` is called  
Then it MUST return a public-prefix fallback URL plus non-nil error.

Given `MustGetPublicURLBuildtime` cannot load file-map artifact  
When invoked  
Then it MUST panic.

Given mapped key is absent but file-map load succeeded  
When either resolver is called  
Then it MUST return fallback URL and warn without failing call.

#### WAVE-URL-002: Build-Time Public Filemap Helper Contract

Given `PublicFileMapKeys()`  
When public file-map loads (or is built successfully via fallback path)  
Then helper MUST return sorted keys for non-prehashed entries only.

Given `SimplePublicFileMap()`  
When public file-map loads (or is built successfully via fallback path)  
Then helper MUST return `map[originalPath]distName` for non-prehashed entries
only.

Given `loadOrBuildFileMap()` helper load step fails  
When fallback processing runs  
Then helper MUST run non-granular file processing once and return
`build files: ...` wrapping error if processing fails.

Given `LoadPublicFileMap()`  
When called  
Then helper MUST return direct public file-map load results without extra
fallback logic.

Given `AddPublicAssetKeys(statements)`  
When `statements` is nil  
Then helper MUST allocate statements, append serialized `WAVE_PUBLIC_ASSETS`
from `PublicFileMapKeys()`, and append `WavePublicAsset` template-literal type
export.

Given `AddPublicAssetKeys(statements)`  
When `PublicFileMapKeys()` returns error  
Then helper MUST panic.

## 4. Scenario Catalog

### WDC-CLI-001 (covers WAVE-CLI-001)

CLI helper invocation variants with nil logger input MUST verify default `wave`
color logger initialization occurs before delegated work.

### WDC-CLI-002 (covers WAVE-CLI-002)

`--hook` callback-present and callback-nil variants MUST verify callback gating,
dev-flag forwarding, and early-return vs standard mode-selection semantics.

### WDC-CLI-003 (covers WAVE-CLI-003)

Hook/dev/prod delegated-path failure fixtures MUST verify panic-based failure
propagation and production builder deferred-close lifecycle.

### WDC-DEV-012 (covers WAVE-DEV-012)

Cycle-vite orchestration fixtures MUST verify single authoritative reload source
per cycle.

### WDC-DEV-030 (covers WAVE-DEV-030)

Revalidate helper resolve/reject fixtures MUST both clear rebuilding overlay.

### WDC-DEV-032 (covers WAVE-DEV-032)

Vite startup failure fixture MUST verify explicit failure-state handling.

### WDC-DEV-033 (covers WAVE-DEV-033)

Build-retry restart-strength fixtures MUST verify strength bits are preserved.

### WDC-DEV-034 (covers WAVE-DEV-034)

App startup failure fixture MUST verify actionable propagation.

### WDC-DEV-035 (covers WAVE-DEV-035)

Readiness timeout fixtures MUST verify failure-state gating.

### WDC-DEV-036 (covers WAVE-DEV-036)

Config-reload fixtures MUST verify full framework-injected field preservation.

### WDC-DEV-041 (covers WAVE-DEV-041)

App stop failure fixtures MUST verify caller-observable error propagation.

### WDC-EVT-001 (covers WAVE-EVT-001)

Mixed-strength/mixed-match dedupe fixtures MUST verify non-lossy strongest-work
aggregation.

### WDC-EVT-019 (covers WAVE-EVT-019)

Blocking phase failure fixtures MUST verify success-signal suppression.

### WDC-EVT-023 (covers WAVE-EVT-023)

Concurrent restart fixtures with completion-order variance MUST verify
deterministic strongest-intent outcome.

### WDC-EVT-028 (covers WAVE-EVT-028)

Dynamic directory watch-expansion failure fixtures MUST verify actionable
failure.

### WDC-EVT-029 (covers WAVE-EVT-029)

CSS artifact read-failure fixtures MUST verify no success-style CSS payload
emission.

### WDC-EVT-030 (covers WAVE-EVT-030)

Implicit-work + callback-restart fixtures MUST verify no downgrade of
recompile-required outcome.

### WDC-EVT-031 (covers WAVE-EVT-031)

Watcher teardown-race fixtures MUST verify stable watcher-channel source.

### WDC-EVT-032 (covers WAVE-EVT-032)

Stale-watch removal-failure fixtures MUST verify explicit failure handling and
state consistency.

### WDC-EVT-033 (covers WAVE-EVT-033)

Closed error-channel fixtures MUST verify bounded watch-loop termination without
nil-error spin.

### WDC-STATIC-005 (covers WAVE-STATIC-005)

Filemap rotation cleanup-failure fixtures MUST verify explicit policy
conformance.

### WDC-STATIC-015 (covers WAVE-STATIC-015)

Granular stale-artifact removal-failure fixtures MUST verify explicit policy
conformance.

### WDC-CSS-003 (covers WAVE-CSS-003)

Normal CSS rotation cleanup-failure fixtures MUST verify explicit policy
conformance.

### WDC-SCHEMA-007 (covers WAVE-SCHEMA-007)

Reserved schema-key collision fixtures MUST fail instead of silently overriding
reserved sections.

### WDC-BUILD-001 (covers WAVE-BUILD-001)

Config-validation fixtures MUST verify required core/static/vite fields and
`RunOnChangeOnly` command-hook timing constraints.

### WDC-BUILD-002 (covers WAVE-BUILD-002)

Build-pipeline fixtures MUST verify validation-first ordering, pre/post file
phases, file-only short-circuit behavior, and schema-warning-vs-compile-failure
policies.

### WDC-BUILD-003 (covers WAVE-BUILD-003)

Hook-selection fixtures MUST verify user-before-framework execution order and
fail-fast propagation semantics.

### WDC-BUILD-004 (covers WAVE-BUILD-004)

Non-granular file-processing fixtures MUST verify lock-file-preserving cleanup
and `SetupDistDir` `.keep` sentinel creation.

### WDC-BUILD-005 (covers WAVE-BUILD-005)

Dev-loop compile fixtures MUST verify sequential-vs-concurrent Go compile
scheduling under `SequentialGoBuild`.

### WDC-BUILD-006 (covers WAVE-BUILD-006)

Missing-static-directory fixtures MUST verify empty file-map fallback handling
for both public and private paths.

### WDC-WATCH-001 (covers WAVE-WATCH-001)

Watcher-bootstrap fixtures MUST verify absolute-path normalization, ignore-set
defaults, and hook pre-sort behavior.

### WDC-WATCH-002 (covers WAVE-WATCH-002)

Overlapping-watch-pattern fixtures MUST verify strongest-work merge semantics
and framework-before-user hook ordering.

### WDC-LOCK-001 (covers WAVE-LOCK-001)

Dev-lock fixtures MUST verify live-PID lock rejection and stale-lock takeover
behavior.

### WDC-URL-001 (covers WAVE-URL-001)

Build-time URL resolution fixtures MUST verify fallback URL semantics,
error-return behavior for non-must path, and panic behavior for must path.

### WDC-URL-002 (covers WAVE-URL-002)

Public file-map helper fixtures MUST verify key sorting/filtering semantics,
load-or-build fallback behavior, and `AddPublicAssetKeys` nil+panic contracts.

## 5. Relation to Other Specs

- Wave runtime-serving owner contracts: `spec/packages/wave/SPEC.md`
- Wave runtime traceability: `spec/packages/wave/TRACEABILITY_MATRIX.md`
- Wave runtime conformance issues: `spec/packages/wave/CONFORMANCE_ISSUES.md`
