# Vorma Conformance Issues Backlog

Status: Draft  
Last Updated: 2026-02-08  
Purpose: Track places where conformance tests/specs exposed likely implementation defects or required harness-only accommodations.

## Issue Format

- `ID`: stable issue ID (`VCI-*`)
- `Type`: `impl-bug-candidate` or `harness-constraint`
- `Affected Requirements`: spec requirement IDs
- `Current Test Accommodation`: what the test does today
- `Follow-Up`: implementation or tooling change to remove accommodation

## Issues

### VCI-001 (resolved)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `WIRE-JSON-004`
- `Summary`: Outermost server error index can become nondeterministic under parallel loader cancellation.
- `Observed Symptom`: `outermostServerErrorIdx` intermittently surfaced as `0` instead of `1` when a deeper failing loader causes sibling cancellation (`context canceled`) to appear at an earlier index.
- `Current Test Accommodation`: none (removed).
- `Follow-Up`: implemented in `/Users/sjc/__code/river/vormaruntime/gmpd.go` by preferring first non-cancellation loader error and only falling back to cancellation errors if no semantic error exists.

### VCI-002 (resolved)

- `Type`: `harness-constraint`
- `Affected Requirements`: `BUILD-CLI-004`
- `Summary`: Dev-mode conformance probing now has deterministic clean-exit path after entering steady-state workflow.
- `Observed Symptom`: Earlier `BUILD-CLI-004` relied on timeout/process-group kill to avoid indefinite `--dev` lifetime in harness runs.
- `Current Test Accommodation`: none for `BUILD-CLI-004`. Resolved by adding optional dev exit override (`WAVE_DEV_EXIT_AFTER_MS`) in `/Users/sjc/__code/river/wave/tooling/devserver.go` and using it from `/Users/sjc/__code/river/conformance/build/build_cli_modes_conformance_test.go`.
- `Follow-Up`: Keep env-gated exit behavior limited to explicit override usage and preserve default long-running dev semantics when override is unset.

### VCI-003 (resolved)

- `Type`: `harness-constraint`
- `Affected Requirements`: `FE-LINK-007`, `FE-UI-002`, `FE-UI-004`, `FE-UI-005`
- `Summary`: Solid TSX conformance execution in Vitest previously failed before behavioral assertions due transform/runtime mismatch.
- `Observed Symptom`: Strict runs initially failed with `React is not defined` for Solid TSX imports.
- `Current Test Accommodation`: none. Resolved by Vitest transform/runtime alignment in `/Users/sjc/__code/river/vitest.config.ts` (Solid TSX Babel pre-transform + `solid-js`/`solid-js/web` browser-runtime aliases), without global React injection.
- `Follow-Up`: Keep Vitest Solid transform path aligned with `/Users/sjc/__code/river/internal/scripts/buildts/build-solid.mjs`; re-validate on build/test tooling upgrades.

### VCI-004 (resolved)

- `Type`: `harness-constraint`
- `Affected Requirements`: `BUILD-VITE-003`, `BUILD-VITE-004`, `BUILD-VITE-005`
- `Summary`: Vite plugin conformance probe now executes against source deterministically without packaged-artifact fallback.
- `Observed Symptom`: Earlier probe path depended on Node TS runtime support and could fail with `bad option: --experimental-strip-types` or `ERR_UNKNOWN_FILE_EXTENSION ".ts"` for `/Users/sjc/__code/river/internal/framework/_typescript/vite/vite.ts`.
- `Current Test Accommodation`: none. Resolved in `/Users/sjc/__code/river/conformance/build/build_vite_integration_conformance_test.go` by bundling source plugin with local `esbuild` into a temp ESM probe module and executing that module in Node.
- `Follow-Up`: Keep probe transpilation wrapper aligned with source plugin import surface and local toolchain paths.

### VCI-005 (resolved)

- `Type`: `harness-constraint`
- `Affected Requirements`: `FE-UI-003`
- `Summary`: Solid scroll-timing parity failure was caused by Vitest resolving `solid-js` to server runtime.
- `Observed Symptom`: Earlier strict run failed `FEC-UI-003` for Solid with missing `requestAnimationFrame` scheduling.
- `Current Test Accommodation`: none. Resolved by mapping `solid-js` and `solid-js/web` to browser runtime entries in `/Users/sjc/__code/river/vitest.config.ts`.
- `Follow-Up`: Keep runtime aliasing in test config aligned with jsdom/browser expectations.

### VCI-006 (resolved)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-UI-001`
- `Summary`: Solid error-boundary signal assignment treated boundary functions as updater callbacks.
- `Observed Symptom`: Earlier strict run failed `FEC-UI-001` with `Cannot destructure property 'error' of 'undefined'` because boundary function was invoked during signal set.
- `Current Test Accommodation`: none. Resolved by storing boundary functions via wrapped setter in `/Users/sjc/__code/river/internal/framework/_typescript/solid/src/solid.tsx` (`setActiveErrorBoundary(() => ctx.get(\"activeErrorBoundary\"))`).
- `Follow-Up`: Keep function-valued signal assignments wrapped to avoid updater invocation semantics regressions.

### VCI-007 (resolved)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-COMP-004`
- `Summary`: Missing route-specific error exports now reliably fall back to default boundary in strict component conformance.
- `Observed Symptom`: Earlier strict runs could leave `activeErrorBoundary` undefined when `errorExportKeys[i]` referenced a missing export on an existing module.
- `Current Test Accommodation`: none. Resolved in `/Users/sjc/__code/river/internal/framework/_typescript/client/src/component_loader.ts` by guarding missing export access and falling back cleanly to `defaultErrorBoundary` (including proxy/module-namespace edge cases).
- `Follow-Up`: Keep fallback behavior strict and avoid direct missing-export reads that may throw under proxied module namespaces.

### VCI-008 (resolved)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-FETCH-010`, `FE-FETCH-011`
- `Summary`: Redirect-origin build-id timing now satisfies strict event/update ordering guarantees.
- `Observed Symptom`: Earlier strict runs observed no build-id event at redirect handoff boundary when response included both redirect and build-id headers.
- `Current Test Accommodation`: none. Resolved in `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.ts` by applying build-id updates immediately on response receipt (including redirect-origin path) and dispatching events atomically with state update.
- `Follow-Up`: Preserve “update global build ID before event” invariant for any future fetch/redirect refactors.

### VCI-009 (resolved)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-NAV-012`
- `Summary`: Redirect-loop termination now clears navigation state on aborted outcomes.
- `Observed Symptom`: Earlier strict loop-limit probing could leave `navigationStateManager` with a stale active navigation entry after “Too many redirects.”
- `Current Test Accommodation`: none. Resolved in `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.ts` by deleting the target navigation entry in the `aborted` outcome branch before returning.
- `Follow-Up`: Keep aborted-path cleanup symmetric with success/error paths so loading state cannot leak.

### VCI-010 (resolved)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-CL-004`
- `Summary`: Client loader server-error cutoff previously skipped only the exact server-error index, not deeper descendants.
- `Observed Symptom`: Strict `FEC-CL-003` probing showed client loaders below `outermostServerErrorIdx` still executed.
- `Current Test Accommodation`: none. Resolved in `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client_loaders.ts` by applying cutoff for `i >= outermostServerErrorIdx`.
- `Follow-Up`: Preserve cutoff semantics for any client-loader scheduling refactor so descendants never run under server-error branches.

### VCI-011 (resolved)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-CL-006`
- `Summary`: Abort-like loader rejections were not consistently detected as aborts in all runtimes.
- `Observed Symptom`: Strict `FEC-CL-004` probe surfaced abort rejection projected as a client error message instead of cancellation.
- `Current Test Accommodation`: none. Resolved in `/Users/sjc/__code/river/internal/framework/_typescript/client/src/utils/errors.ts` by broadening `isAbortError` detection to AbortError-shaped objects in addition to `Error` instances.
- `Follow-Up`: Keep abort classification robust across browser/runtime error object variants.

### VCI-012 (resolved)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-HMR-002`, `FE-HMR-003`, `FE-HMR-005`
- `Summary`: HMR update registration depended only on `hmr.ts` module hot context, not the caller module hot context.
- `Observed Symptom`: Strict `FEC-HMR-001/FEC-HMR-002` probing showed `__runClientLoadersAfterHMRUpdate(importMeta, pattern)` could fail to register `vite:afterUpdate` when caller `importMeta.hot` was available but `hmr.ts` local hot context was absent.
- `Current Test Accommodation`: none. Resolved in `/Users/sjc/__code/river/internal/framework/_typescript/client/src/hmr/hmr.ts` by resolving hot context as `importMeta.hot ?? import.meta.hot` before listener registration.
- `Follow-Up`: Preserve caller-module HMR context compatibility when refactoring HMR registration logic.

### VCI-013 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `REL-CREATE-005`
- `Summary`: `create-vorma` runtime Node guard currently checks major-version floor only, while package engine/spec contract is `>=22.11.0`.
- `Observed Symptom`: `internal/framework/_typescript/create/main.ts` accepts Node `v22.0.x` as valid because guard is `if (nodeMajor < 22)`, despite user-facing message and package engine floor claiming `22.11+`.
- `Current Test Accommodation`: release conformance currently verifies presence of major-floor check text and does not enforce semver-minor/patch floor behavior.
- `Follow-Up`: Parse and enforce full semver floor at runtime (`22.11.0` minimum), keep message/engine/runtime checks aligned, and add explicit conformance coverage for minor/patch rejection.

### VCI-014 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-DEV-012`
- `Summary`: `broadcastReload` cycle-vite path appears to contradict its own documented orchestration contract.
- `Observed Symptom`: In `/Users/sjc/__code/river/wave/tooling/broadcast.go`, comments state cycle-vite mode should avoid sending Wave reload signal to prevent double reload, but implementation still performs the final broadcast send after `cycleVite()` completes.
- `Current Test Accommodation`: no dedicated conformance assertion currently distinguishes "cycle only" vs "cycle + explicit reload signal" behavior.
- `Follow-Up`: Decide intended behavior for cycle-vite path (single reload source vs dual signal), then align implementation with `BUILD-DEV-012` and add explicit `BDC-DEV-012` coverage.

### VCI-015 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-HTML-008`
- `Summary`: HTML render path is not nil-safe when `GetRootTemplateData` returns `(nil, nil)`.
- `Observed Symptom`: In `/Users/sjc/__code/river/vormaruntime/get_root_handler.go`, runtime writes required template keys directly into `rootTemplateData`; if callback returns nil map and no error, this can panic via assignment to entry in nil map.
- `Current Test Accommodation`: no accommodation added; requirement is tracked as missing so strict conformance can expose behavior.
- `Follow-Up`: treat nil callback result as empty map before key assignment, then add/enable `BRC-HTML-009` conformance coverage.

### VCI-016 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-SKIP-005`
- `Summary`: Skip param/splat guard appears to inspect innermost loader-bearing match rather than required outermost match.
- `Observed Symptom`: In `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.ts`, helper `findOutermostLoaderIndex` iterates from deepest index to root (`for i := len-1; i >= 0` style), so the first loader-bearing match returned is innermost. `didOutermostParamsChange(...)` then evaluates param/splat-change guard against that index.
- `Current Test Accommodation`: none; current skip conformance does not yet include a multi-loader hierarchy case that differentiates outermost vs innermost guard behavior.
- `Follow-Up`: align implementation with spec intent (evaluate loader-relevant param/splat changes on outermost loader-bearing match), then add strict conformance coverage for a nested multi-loader route where only outer route params/splats change.

### VCI-017 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-FETCH-005`, `FE-FETCH-006`
- `Summary`: Invalid highest-priority reload signal currently suppresses fallback evaluation of lower-priority redirect signals.
- `Observed Symptom`: In `/Users/sjc/__code/river/internal/framework/_typescript/client/src/redirects/redirects.ts`, `parseFetchResponseForRedirectData` returns `null` immediately when `X-Vorma-Reload` is present but non-HTTP; it does not continue to inspect `response.redirected` or `X-Client-Redirect` for a valid fallback redirect target.
- `Current Test Accommodation`: none; current fetch/redirect conformance does not include a mixed-signal fixture where `X-Vorma-Reload` is invalid but lower-priority signals are valid.
- `Follow-Up`: Decide intended policy (strict short-circuit vs fallback-to-next-valid-signal), then align implementation, frontend runtime spec text, and redirect conformance scenarios accordingly.

### VCI-018 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-CONC-001`, `BR-CONC-002`, `BR-DEV-007`
- `Summary`: SSR bootstrap DTO reads mutable runtime fields without lock while dev reload writes those fields under lock.
- `Observed Symptom`: `/Users/sjc/__code/river/vormaruntime/ssr.go` reads `v._isDev`, `v._buildID`, and `v._routeManifestFile` directly in `getSSRInnerHTML` while `/Users/sjc/__code/river/vormaruntime/route_reload.go` mutates build/manifest fields under `v.mu.Lock()`, creating potential read/write race under concurrent requests + reload.
- `Current Test Accommodation`: none; existing backend concurrency scenarios are broad and do not yet include a focused assertion around SSR bootstrap field reads during reload churn.
- `Follow-Up`: snapshot required SSR fields via lock-safe getters (or `WithRLock`) before template execution, then add targeted race/conformance coverage for concurrent HTML requests plus repeated route reloads.

### VCI-019 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-TSPKG-007`
- `Summary`: Solid TypeScript packaging path currently does not enforce warning-fatal behavior used by the standard esbuild helper path.
- `Observed Symptom`: `/Users/sjc/__code/river/internal/scripts/buildts/main.go` enforces warning failure only in `build(...)`, while Solid packaging runs through `/Users/sjc/__code/river/internal/scripts/buildts/build-solid.mjs` which invokes `esbuild.build(...)` without warning inspection.
- `Current Test Accommodation`: none; existing TypeScript packaging conformance covers helper warning intolerance (`BUILD-TSPKG-003`) but does not yet assert warning-fatal parity for the Solid helper path.
- `Follow-Up`: enforce warning-fatal parity for Solid packaging (either in helper script or caller wrapper), then add strict `BDC-TSPKG-007` source-contract coverage.

### VCI-020 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-LOAD-014`
- `Summary`: Route-data cache key uses direct normalized-pattern concatenation without tuple-boundary separators.
- `Observed Symptom`: `/Users/sjc/__code/river/vormaruntime/gmpd.go` builds cache key by appending each normalized pattern into one string (`sb.WriteString(...)`) with no delimiter/length framing, allowing distinct ordered tuples to alias if concatenated forms match.
- `Current Test Accommodation`: none; strict conformance now tracks this as missing coverage instead of narrowing scope to only non-aliasing tuples.
- `Follow-Up`: use a collision-resistant tuple key strategy (e.g., explicit separator escaping, length-prefix framing, or structured hash over ordered tuple), then add/enable `BRC-LOAD-014` coverage.

### VCI-021 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-LOAD-015`
- `Summary`: Route-data snapshot cache is process-global and not scoped per app instance.
- `Observed Symptom`: `/Users/sjc/__code/river/vormaruntime/gmpd.go` stores snapshot cache in package-global `sync.Map` keyed only by matched-pattern tuple shape, so distinct Vorma app instances in one process can read each other's cached metadata.
- `Current Test Accommodation`: none; no harness isolation workaround is used to hide cross-instance leakage risk.
- `Follow-Up`: scope cache entries by app identity/build state (or move cache into app instance state), then add/enable `BRC-LOAD-015` coverage.

### VCI-022 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-CONC-003`, `API-GO-014`
- `Summary`: Snapshot-style accessors currently return mutable aliases into internal runtime state.
- `Observed Symptom`: `/Users/sjc/__code/river/vormaruntime/vorma_core.go` returns internal structures directly from accessors such as `GetPathsSnapshot()` (map), `GetClientEntryDeps()` (slice), and `GetDepToCSSBundleMap()` (map), allowing caller mutation after lock release.
- `Current Test Accommodation`: none; strict spec encodes non-aliasing snapshot intent instead of tolerating mutable leakage.
- `Follow-Up`: return defensive copies/immutable snapshot views for snapshot-style map/slice accessors and add/enable `BRC-CONC-003` conformance coverage.

### VCI-023 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-ACT-005`, `API-GO-014`
- `Summary`: `Actions().SupportedMethods()` currently returns the mutable backing map.
- `Observed Symptom`: `/Users/sjc/__code/river/vormaruntime/glue.go` returns `h.vorma.ActionsRouter().supportedMethods` directly, so callers can mutate method flags and potentially affect runtime-visible supported-method behavior.
- `Current Test Accommodation`: none; strict spec now encodes accessor immutability intent instead of permitting caller mutation side effects.
- `Follow-Up`: return defensive copy/read-only view from `SupportedMethods()` and add/enable `BRC-ACT-005` conformance coverage.

### VCI-024 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-ROUTE-005`
- `Summary`: Route DSL parser does not currently enforce strict required signature for `route(pattern, module, ...)`.
- `Observed Symptom`: In `/Users/sjc/__code/river/vormabuild/vorma_build.go`, non-string first-argument patterns are silently dropped (`extractStringArg(0)` miss returns early), and omitted module arguments can flow through with empty `route.Module` and later pass `os.Stat` as a directory path instead of a module file.
- `Current Test Accommodation`: none; route-dsl conformance currently does not include strict invalid-signature fixtures for non-string pattern and missing module argument.
- `Follow-Up`: enforce signature validation with explicit diagnostics (reject non-string pattern and missing/non-file module argument), then add/enable `BDC-ROUTE-005` coverage.

### VCI-025 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-INIT-007`
- `Summary`: Head dedupe rule state appears process-global rather than app-instance scoped.
- `Observed Symptom`: `/Users/sjc/__code/river/vormaruntime/get_root_handler.go` defines package-global `headElsInstance`, while `/Users/sjc/__code/river/vormaruntime/vorma_init.go` reinitializes unique rules via `headElsInstance.InitUniqueRules(...)` during each app `Init()`. Multiple app instances with different dedupe configs can overwrite one another's active head dedupe behavior.
- `Current Test Accommodation`: none; no workaround was added to hide cross-app contamination risk.
- `Follow-Up`: scope head dedupe state per app instance (or provide instance-keyed isolation) and add/enable `BRC-INIT-007` coverage to assert cross-app isolation behavior.

### VCI-026 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-ROUTE-006`
- `Summary`: Duplicate route-pattern collisions are currently silent last-write-wins with no diagnostics.
- `Observed Symptom`: `/Users/sjc/__code/river/vormabuild/vorma_build.go` materializes parsed routes into a map keyed by pattern (`paths[rc.Pattern] = ...`), so later duplicate definitions overwrite earlier ones without warning/error.
- `Current Test Accommodation`: none; behavior is now explicitly codified as current contract but no focused duplicate-collision conformance case is yet enabled.
- `Follow-Up`: decide duplicate-pattern policy (explicit error, warning, or intentional last-write semantics), then align parser behavior, build spec text, and `BDC-ROUTE-006` coverage accordingly.

### VCI-027 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `REL-CREATE-005`
- `Summary`: `create-vorma` Go-version guard does not fail when `go version` output is present but unparsable.
- `Observed Symptom`: `/Users/sjc/__code/river/internal/framework/_typescript/create/main.ts` parses Go version with `/go(\\d+)\\.(\\d+)/`; when match is absent, no parse-failure branch triggers and script continues instead of rejecting unknown version format.
- `Current Test Accommodation`: release conformance currently validates declared floor checks but does not include an unparsable-`go version` fixture path.
- `Follow-Up`: treat unparsable Go-version output as guard failure with explicit error message and add release conformance coverage for parse-failure guard path.

### VCI-028 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-HEAD-004`
- `Summary`: Default-head callback failures can currently mask terminal non-render outcomes.
- `Observed Symptom`: `/Users/sjc/__code/river/vormaruntime/gmpd.go` launches `GetDefaultHeadEls` concurrently and waits on it before honoring stage-1 terminal outcomes. If callback errors, handler returns `500` (`didErr`) even when stage-1 already determined terminal path semantics (`notFound` / proxy redirect / proxy error) that should have short-circuited response handling.
- `Current Test Accommodation`: none; strict spec now encodes terminal-outcome precedence as normative intent instead of preserving current fail-overwrite behavior.
- `Follow-Up`: reorder precedence so terminal stage-1 outcomes are returned without being overwritten by default-head callback failure, or gate callback execution to non-terminal render paths; then add/enable `BRC-HEAD-004` coverage.

### VCI-029 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-EVT-001`
- `Summary`: Event-batch watched-pattern dedupe currently collapses unrelated unpatterned classified events.
- `Observed Symptom`: In `/Users/sjc/__code/river/wave/tooling/events.go`, batch processing uses `handledPatterns` keyed by watched-file pattern; events without matched watched-file config use empty key (`""`), so only the first such event in a batch is processed while later distinct unpatterned events are skipped.
- `Current Test Accommodation`: none; strict build spec now requires that pattern-key dedupe not collapse distinct events lacking a matched watched-file key.
- `Follow-Up`: guard pattern-key dedupe to non-empty keys (or incorporate file-type/path discriminators), then add focused conformance coverage for same-batch unpatterned multi-event cases (for example critical+normal CSS tracked imports changed together).

### VCI-030 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-INIT-006`
- `Summary`: Repeated `Init()` on the same app instance can preserve stale route-path entries that are no longer present in selected paths artifact.
- `Observed Symptom`: In `/Users/sjc/__code/river/vormaruntime/vorma_init.go`, `initInner` only allocates `_paths` when nil and then writes artifact entries into the existing map without clearing removed keys, so removed patterns from prior init runs can remain resident.
- `Current Test Accommodation`: none; backend spec now explicitly requires init-time route snapshot replacement semantics instead of additive merge.
- `Follow-Up`: replace init-time `_paths` state from decoded artifact snapshot (or clear map before repopulating), then add focused init-reentry conformance coverage that removes a route between two init runs and asserts stale key absence.

### VCI-031 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-ART-004`
- `Summary`: Generated TypeScript currently derives params/splat typing for client-defined loader paths using action runes instead of loader runes.
- `Observed Symptom`: In `/Users/sjc/__code/river/vormabuild/vorma_gen_ts.go`, client-defined loader-path synthesis branch uses `extractDynamicParamsFromPattern(..., actionsDynamicRune)` and `isSplat(..., actionsSplatRune)` while emitting loader-category entries; this can mis-type loader params/splats when action and loader rune settings differ.
- `Current Test Accommodation`: none; build spec now encodes loader-entry rune-source intent explicitly instead of mirroring current mixed-rune implementation.
- `Follow-Up`: switch client-defined loader-path param/splat extraction to loader rune settings and add focused `BDC-ART-004` coverage for divergent action-vs-loader rune configurations.

### VCI-032 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-NAV-009`
- `Summary`: Stale-origin revalidation short-circuit currently occurs after stale payload side effects can already run.
- `Observed Symptom`: In `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.ts`, `processSuccessfulNavigation(...)` applies build-match gated side effects (`clientModuleMap` writes and `AssetManager.applyCSS(...)`) before stale-origin revalidation guard (`currentUrl !== entry.originUrl`) returns.
- `Current Test Accommodation`: none; frontend nav conformance currently asserts no stale rerender/route-data commit but does not yet assert absence of stale module-map/CSS side effects.
- `Follow-Up`: move stale-origin guard earlier (before stale-payload side effects) or gate those side effects on origin safety, then add strict `FEC-NAV-006` coverage for no-commit side-effect boundaries.

### VCI-033 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-LINK-013`
- `Summary`: Prefetch stop path appears to compute lookup key without applying `search`/`hash` overrides used by prefetch start path.
- `Observed Symptom`: In `/Users/sjc/__code/river/internal/framework/_typescript/client/src/links.ts`, `prefetch()` constructs target URL from `relativeURL` plus optional `input.search`/`input.hash`, but `stop()` computes lookup key from `relativeURL` alone before `navigationStateManager.getNavigation(...)`/`removeNavigation(...)`; overridden search/hash prefetch entries may therefore not be found/aborted by `stop()`.
- `Current Test Accommodation`: none; current link conformance does not yet include an override-target case that asserts start/stop/click URL-key coherence.
- `Follow-Up`: normalize/centralize effective target URL resolution for prefetch handler paths (`start`, `stop`, `onClick`) and add strict `FEC-LINK-011` coverage.

### VCI-034 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-EVT-010`, `BUILD-EVT-013`
- `Summary`: `RunOnChangeOnly` early-return path appears to drop non-restart `RefreshAction` outcomes from callbacks.
- `Observed Symptom`: In `/Users/sjc/__code/river/wave/tooling/events.go`, single-event path returns immediately at `RunOnChangeOnly` check (after pre-hooks) and batch path returns when `allRunOnChangeOnly` is true, before browser phase execution. Callback actions merged via `work.addFromRefreshAction(...)` (for example `ReloadBrowser`, `WaitForApp`, `WaitForVite`) can therefore be ignored even though `RunOnChangeOnly` hooks are documented as controlling refresh behavior.
- `Current Test Accommodation`: none; existing build event conformance for `BUILD-EVT-010`/`BUILD-EVT-013` asserts build-skip behavior but does not yet assert callback action propagation on run-on-change-only paths.
- `Follow-Up`: preserve callback `RefreshAction` propagation in run-on-change-only flows (single and all-run-on-change-only batch) and add strict `BDC-EVT-010` and `BDC-EVT-013` coverage for non-restart reload/wait action honoring.

### VCI-035 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-EVT-010`, `BUILD-EVT-013`
- `Summary`: `RunOnChangeOnly` short-circuit currently skips concurrent/post callback-hook execution.
- `Observed Symptom`: In `/Users/sjc/__code/river/wave/tooling/events.go`, single-event path returns immediately on `ewh.runOnChangeOnly` and all-run-on-change-only batch path returns on `allRunOnChangeOnly` before concurrent/post hook phases. This conflicts with schema/runtime intent that `RunOnChangeOnly` means "only OnChangeHooks run" (not "only pre/no-wait callbacks run").
- `Current Test Accommodation`: none; current source-contract conformance around run-on-change-only focuses on build-skip and does not yet assert callback-timing execution coverage.
- `Follow-Up`: ensure run-on-change-only flows execute supported callback-hook timing phases (including concurrent/post callbacks) while still skipping standard build work, then add strict `BDC-EVT-010`/`BDC-EVT-013` timing-coverage assertions.

### VCI-036 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `API-UI-004`, `FE-UI-010`, `FE-UI-011`
- `Summary`: Preact typed pattern-based helper memoization appears stale across route-change matched-pattern updates.
- `Observed Symptom`: In `/Users/sjc/__code/river/internal/framework/_typescript/preact/src/helpers.ts`, `makeTypedUsePatternLoaderData` memoizes lookup index with deps `[pattern]`, and `makeTypedAddClientLoader` no-props accessor memoizes index with deps `[props]`; both reads derive from `routerData.value.matchedPatterns` but do not include matched-pattern snapshot in memo deps. Pattern-match index can therefore remain stale when route snapshots change while pattern input stays constant.
- `Current Test Accommodation`: none; new UI-helper reactivity requirements are tracked as missing (`FE-UI-010`, `FE-UI-011`) rather than narrowed to current behavior.
- `Follow-Up`: make Preact helper pattern-index derivation reactive to current matched-pattern snapshot (for example include matched-pattern dependencies or use signal-native derived computation), then add strict adapter parity coverage for pattern move/remove transitions (`FEC-UI-008`, `FEC-UI-009`).

### VCI-037 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-UI-012`
- `Summary`: UI adapters can get stuck when route-level component identity transitions from undefined to defined after initial render.
- `Observed Symptom`: In `/Users/sjc/__code/river/internal/framework/_typescript/react/src/react.tsx`, `/Users/sjc/__code/river/internal/framework/_typescript/preact/src/preact.tsx`, and `/Users/sjc/__code/river/internal/framework/_typescript/solid/src/solid.tsx`, route-change sync logic for `currentImportURL` short-circuits when current identity is falsy (`if (!currentImportURL...) return`). If a level starts without identity and a later route snapshot introduces identity for that same level, update logic may never promote from undefined to defined identity.
- `Current Test Accommodation`: none; requirement is tracked as missing (`FE-UI-012`) rather than mirrored to current behavior.
- `Follow-Up`: update adapter sync logic to allow undefined->defined identity promotion on route-change snapshots (while preserving remount behavior), then add strict parity coverage (`FEC-UI-010`) for transition from fallback/absent to concrete component at the same route level.

### VCI-038 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-UI-013`
- `Summary`: Preact terminal component-absent outlet branch inserts a synthetic wrapper node while React/Solid remain node-empty.
- `Observed Symptom`: In `/Users/sjc/__code/river/internal/framework/_typescript/preact/src/preact.tsx`, terminal branch `if (!CurrentComp.value && !shouldFallbackOutlet.value)` returns `h("div", {})`; corresponding React branch in `/Users/sjc/__code/river/internal/framework/_typescript/react/src/react.tsx` returns empty fragment and Solid branch in `/Users/sjc/__code/river/internal/framework/_typescript/solid/src/solid.tsx` emits no node in the equivalent state.
- `Current Test Accommodation`: none; node-empty parity requirement is tracked as missing (`FE-UI-013`) rather than narrowed to current divergence.
- `Follow-Up`: align terminal absent-component rendering across adapters to be node-empty (no synthetic wrapper insertion), then add strict parity coverage (`FEC-UI-011`) for this branch.

### VCI-039 (open)

- `Type`: `harness-constraint`
- `Affected Requirements`: `FE-NAV-008`, `FE-NAV-011`
- `Summary`: Legacy frontend tests encode stale debounce/coalescing timing commentary (`5ms`) while current runtime and strict spec contract use `8ms`.
- `Observed Symptom`: Legacy suites such as `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.events_system.test.ts`, `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.navigation_lifecycle.begin_navigation_phase.test.ts`, and `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.form_submissions.revalidate_function.test.ts` contain 5ms timing comments/assertion intent, while runtime source in `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.ts` uses `REVALIDATION_COALESCE_MS = 8` and debounced status dispatch at `8ms`.
- `Current Test Accommodation`: strict conformance specs/tests use `8ms` contract (`FE-NAV-008`, `FE-NAV-011`) and do not preserve legacy 5ms commentary as normative behavior.
- `Follow-Up`: refresh or retire stale legacy 5ms timing assertions/comments so legacy suites no longer imply conflicting normative timing intent.

### VCI-040 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-VITE-010`
- `Summary`: Vite dev-port helper currently ignores configured/default candidate port during free-port selection.
- `Observed Symptom`: `/Users/sjc/__code/river/lab/viteutil/viteutil.go` defines `InitPort(defaultPort int)` but calls `netutil.GetFreePort(5199)` unconditionally, so `defaultPort` input (and upstream `Vite.DefaultPort` wiring from `/Users/sjc/__code/river/wave/tooling/builder.go`) is not used for candidate selection.
- `Current Test Accommodation`: none; contract is now explicitly tracked as missing instead of mirroring current hard-coded-candidate behavior.
- `Follow-Up`: use `defaultPort` as the candidate passed to free-port selection, preserve env propagation to `__VITE_PORT`, and add/enable `BDC-VITE-010` conformance coverage.
