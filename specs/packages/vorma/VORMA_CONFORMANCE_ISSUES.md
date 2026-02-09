# Vorma Conformance Issues Backlog

Status: Draft  
Last Updated: 2026-02-09  
Purpose: Track active Vorma-owned implementation divergences affecting current Vorma requirements.

## Scope Rule

- This file contains only active (`open`) Vorma-owned issues.
- Resolved historical rows are intentionally removed.
- Wave-owned issues are tracked canonically in:
  `specs/packages/wave/WAVE_CONFORMANCE_ISSUES.md`.

## Issue Format

- `ID`: stable issue ID (`VCI-*`)
- `Type`: `impl-bug-candidate` or `harness-constraint`
- `Affected Requirements`: current Vorma requirement IDs
- `Current Test Accommodation`: what conformance does today
- `Follow-Up`: implementation/spec action needed for closure

## Open Issues

### VCI-015 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-HTML-008`
- `Summary`: HTML render path is not nil-safe when `GetRootTemplateData` returns `(nil, nil)`.
- `Observed Symptom`: In `vormaruntime/get_root_handler.go`, runtime writes required template keys directly into `rootTemplateData`; if callback returns nil map and no error, this can panic via assignment to entry in nil map.
- `Current Test Accommodation`: no accommodation added; requirement is tracked as missing so strict conformance can expose behavior.
- `Follow-Up`: treat nil callback result as empty map before key assignment, then add/enable `BRC-HTML-009` conformance coverage.

### VCI-016 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-SKIP-005`
- `Summary`: Skip param/splat guard appears to inspect innermost loader-bearing match rather than required outermost match.
- `Observed Symptom`: In `vormaclient/client/src/client.ts`, helper `findOutermostLoaderIndex` iterates from deepest index to root (`for i := len-1; i >= 0` style), so the first loader-bearing match returned is innermost. `didOutermostParamsChange(...)` then evaluates param/splat-change guard against that index.
- `Current Test Accommodation`: none; current skip conformance does not yet include a multi-loader hierarchy case that differentiates outermost vs innermost guard behavior.
- `Follow-Up`: align implementation with spec intent (evaluate loader-relevant param/splat changes on outermost loader-bearing match), then add strict conformance coverage for a nested multi-loader route where only outer route params/splats change.

### VCI-017 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-FETCH-005`, `FE-FETCH-006`
- `Summary`: Invalid highest-priority reload signal currently suppresses fallback evaluation of lower-priority redirect signals.
- `Observed Symptom`: In `vormaclient/client/src/redirects/redirects.ts`, `parseFetchResponseForRedirectData` returns `null` immediately when `X-Vorma-Reload` is present but non-HTTP; it does not continue to inspect `response.redirected` or `X-Client-Redirect` for a valid fallback redirect target.
- `Current Test Accommodation`: none; current fetch/redirect conformance does not include a mixed-signal fixture where `X-Vorma-Reload` is invalid but lower-priority signals are valid.
- `Follow-Up`: Decide intended policy (strict short-circuit vs fallback-to-next-valid-signal), then align implementation, frontend runtime spec text, and redirect conformance scenarios accordingly.

### VCI-018 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-CONC-001`, `BR-CONC-002`, `BR-DEV-007`
- `Summary`: SSR bootstrap DTO reads mutable runtime fields without lock while dev reload writes those fields under lock.
- `Observed Symptom`: `vormaruntime/ssr.go` reads `v._isDev`, `v._buildID`, and `v._routeManifestFile` directly in `getSSRInnerHTML` while `vormaruntime/route_reload.go` mutates build/manifest fields under `v.mu.Lock()`, creating potential read/write race under concurrent requests + reload.
- `Current Test Accommodation`: none; existing backend concurrency scenarios are broad and do not yet include a focused assertion around SSR bootstrap field reads during reload churn.
- `Follow-Up`: snapshot required SSR fields via lock-safe getters (or `WithRLock`) before template execution, then add targeted race/conformance coverage for concurrent HTML requests plus repeated route reloads.

### VCI-020 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-LOAD-014`
- `Summary`: Route-data cache key uses direct normalized-pattern concatenation without tuple-boundary separators.
- `Observed Symptom`: `vormaruntime/gmpd.go` builds cache key by appending each normalized pattern into one string (`sb.WriteString(...)`) with no delimiter/length framing, allowing distinct ordered tuples to alias if concatenated forms match.
- `Current Test Accommodation`: none; strict conformance now tracks this as missing coverage instead of narrowing scope to only non-aliasing tuples.
- `Follow-Up`: use a collision-resistant tuple key strategy (e.g., explicit separator escaping, length-prefix framing, or structured hash over ordered tuple), then add/enable `BRC-LOAD-014` coverage.

### VCI-021 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-LOAD-015`
- `Summary`: Route-data snapshot cache is process-global and not scoped per app instance.
- `Observed Symptom`: `vormaruntime/gmpd.go` stores snapshot cache in package-global `sync.Map` keyed only by matched-pattern tuple shape, so distinct Vorma app instances in one process can read each other's cached metadata.
- `Current Test Accommodation`: none; no harness isolation workaround is used to hide cross-instance leakage risk.
- `Follow-Up`: scope cache entries by app identity/build state (or move cache into app instance state), then add/enable `BRC-LOAD-015` coverage.

### VCI-022 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-CONC-003`, `API-GO-014`
- `Summary`: Snapshot-style accessors currently return mutable aliases into internal runtime state.
- `Observed Symptom`: `vormaruntime/vorma_core.go` returns internal structures directly from accessors such as `GetPathsSnapshot()` (map), `GetClientEntryDeps()` (slice), `GetDepToCSSBundleMap()` (map), and `GetAdHocTypes()` (slice), allowing caller mutation after lock release.
- `Current Test Accommodation`: none; strict spec encodes non-aliasing snapshot intent instead of tolerating mutable leakage.
- `Follow-Up`: return defensive copies/immutable snapshot views for snapshot-style map/slice accessors and add/enable `BRC-CONC-003` conformance coverage.

### VCI-023 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-ACT-005`, `API-GO-014`
- `Summary`: `Actions().SupportedMethods()` currently returns the mutable backing map.
- `Observed Symptom`: `vormaruntime/glue.go` returns `h.vorma.ActionsRouter().supportedMethods` directly, so callers can mutate method flags and potentially affect runtime-visible supported-method behavior.
- `Current Test Accommodation`: none; strict spec now encodes accessor immutability intent instead of permitting caller mutation side effects.
- `Follow-Up`: return defensive copy/read-only view from `SupportedMethods()` and add/enable `BRC-ACT-005` conformance coverage.

### VCI-024 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-ROUTE-005`
- `Summary`: Route DSL parser does not currently enforce strict required signature for `route(pattern, module, ...)`.
- `Observed Symptom`: In `vormabuild/vorma_build.go`, non-string first-argument patterns are silently dropped (`extractStringArg(0)` miss returns early), and omitted module arguments can flow through with empty `route.Module` and later pass `os.Stat` as a directory path instead of a module file.
- `Current Test Accommodation`: none; route-dsl conformance currently does not include strict invalid-signature fixtures for non-string pattern and missing module argument.
- `Follow-Up`: enforce signature validation with explicit diagnostics (reject non-string pattern and missing/non-file module argument), then add/enable `BDC-ROUTE-005` coverage.

### VCI-025 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-INIT-007`
- `Summary`: Head dedupe rule state appears process-global rather than app-instance scoped.
- `Observed Symptom`: `vormaruntime/get_root_handler.go` defines package-global `headElsInstance`, while `vormaruntime/vorma_init.go` reinitializes unique rules via `headElsInstance.InitUniqueRules(...)` during each app `Init()`. Multiple app instances with different dedupe configs can overwrite one another's active head dedupe behavior.
- `Current Test Accommodation`: none; no workaround was added to hide cross-app contamination risk.
- `Follow-Up`: scope head dedupe state per app instance (or provide instance-keyed isolation) and add/enable `BRC-INIT-007` coverage to assert cross-app isolation behavior.

### VCI-026 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-ROUTE-006`
- `Summary`: Duplicate route-pattern collisions are currently silent last-write-wins with no diagnostics.
- `Observed Symptom`: `vormabuild/vorma_build.go` materializes parsed routes into a map keyed by pattern (`paths[rc.Pattern] = ...`), so later duplicate definitions overwrite earlier ones without warning/error.
- `Current Test Accommodation`: none; behavior is now explicitly codified as current contract but no focused duplicate-collision conformance case is yet enabled.
- `Follow-Up`: decide duplicate-pattern policy (explicit error, warning, or intentional last-write semantics), then align parser behavior, build spec text, and `BDC-ROUTE-006` coverage accordingly.

### VCI-028 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-HEAD-004`
- `Summary`: Default-head callback failures can currently mask terminal non-render outcomes.
- `Observed Symptom`: `vormaruntime/gmpd.go` launches `GetDefaultHeadEls` concurrently and waits on it before honoring stage-1 terminal outcomes. If callback errors, handler returns `500` (`didErr`) even when stage-1 already determined terminal path semantics (`notFound` / proxy redirect / proxy error) that should have short-circuited response handling.
- `Current Test Accommodation`: none; strict spec now encodes terminal-outcome precedence as normative intent instead of preserving current fail-overwrite behavior.
- `Follow-Up`: reorder precedence so terminal stage-1 outcomes are returned without being overwritten by default-head callback failure, or gate callback execution to non-terminal render paths; then add/enable `BRC-HEAD-004` coverage.

### VCI-030 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-INIT-006`
- `Summary`: Repeated `Init()` on the same app instance can preserve stale route-path entries that are no longer present in selected paths artifact.
- `Observed Symptom`: In `vormaruntime/vorma_init.go`, `initInner` only allocates `_paths` when nil and then writes artifact entries into the existing map without clearing removed keys, so removed patterns from prior init runs can remain resident.
- `Current Test Accommodation`: none; backend spec now explicitly requires init-time route snapshot replacement semantics instead of additive merge.
- `Follow-Up`: replace init-time `_paths` state from decoded artifact snapshot (or clear map before repopulating), then add focused init-reentry conformance coverage that removes a route between two init runs and asserts stale key absence.

### VCI-032 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-NAV-009`
- `Summary`: Stale-origin revalidation short-circuit currently occurs after stale payload side effects can already run.
- `Observed Symptom`: In `vormaclient/client/src/client.ts`, `processSuccessfulNavigation(...)` applies build-match gated side effects (`clientModuleMap` writes and `AssetManager.applyCSS(...)`) before stale-origin revalidation guard (`currentUrl !== entry.originUrl`) returns.
- `Current Test Accommodation`: none; frontend nav conformance currently asserts no stale rerender/route-data commit but does not yet assert absence of stale module-map/CSS side effects.
- `Follow-Up`: move stale-origin guard earlier (before stale-payload side effects) or gate those side effects on origin safety, then add strict `FEC-NAV-006` coverage for no-commit side-effect boundaries.

### VCI-033 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-LINK-013`
- `Summary`: Prefetch stop path appears to compute lookup key without applying `search`/`hash` overrides used by prefetch start path.
- `Observed Symptom`: In `vormaclient/client/src/links.ts`, `prefetch()` constructs target URL from `relativeURL` plus optional `input.search`/`input.hash`, but `stop()` computes lookup key from `relativeURL` alone before `navigationStateManager.getNavigation(...)`/`removeNavigation(...)`; overridden search/hash prefetch entries may therefore not be found/aborted by `stop()`.
- `Current Test Accommodation`: none; current link conformance does not yet include an override-target case that asserts start/stop/click URL-key coherence.
- `Follow-Up`: normalize/centralize effective target URL resolution for prefetch handler paths (`start`, `stop`, `onClick`) and add strict `FEC-LINK-011` coverage.

### VCI-036 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `API-UI-004`, `FE-UI-010`, `FE-UI-011`
- `Summary`: Preact typed pattern-based helper memoization appears stale across route-change matched-pattern updates.
- `Observed Symptom`: In `vormaclient/preact/src/helpers.ts`, `makeTypedUsePatternLoaderData` memoizes lookup index with deps `[pattern]`, and `makeTypedAddClientLoader` no-props accessor memoizes index with deps `[props]`; both reads derive from `routerData.value.matchedPatterns` but do not include matched-pattern snapshot in memo deps. Pattern-match index can therefore remain stale when route snapshots change while pattern input stays constant.
- `Current Test Accommodation`: none; new UI-helper reactivity requirements are tracked as missing (`FE-UI-010`, `FE-UI-011`) rather than narrowed to current behavior.
- `Follow-Up`: make Preact helper pattern-index derivation reactive to current matched-pattern snapshot (for example include matched-pattern dependencies or use signal-native derived computation), then add strict adapter parity coverage for pattern move/remove transitions (`FEC-UI-008`, `FEC-UI-009`).

### VCI-037 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-UI-012`
- `Summary`: UI adapters can get stuck when route-level component identity transitions from undefined to defined after initial render.
- `Observed Symptom`: In `vormaclient/react/src/react.tsx`, `vormaclient/preact/src/preact.tsx`, and `vormaclient/solid/src/solid.tsx`, route-change sync logic for `currentImportURL` short-circuits when current identity is falsy (`if (!currentImportURL...) return`). If a level starts without identity and a later route snapshot introduces identity for that same level, update logic may never promote from undefined to defined identity.
- `Current Test Accommodation`: none; requirement is tracked as missing (`FE-UI-012`) rather than mirrored to current behavior.
- `Follow-Up`: update adapter sync logic to allow undefined->defined identity promotion on route-change snapshots (while preserving remount behavior), then add strict parity coverage (`FEC-UI-010`) for transition from fallback/absent to concrete component at the same route level.

### VCI-038 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-UI-013`
- `Summary`: Preact terminal component-absent outlet branch inserts a synthetic wrapper node while React/Solid remain node-empty.
- `Observed Symptom`: In `vormaclient/preact/src/preact.tsx`, terminal branch `if (!CurrentComp.value && !shouldFallbackOutlet.value)` returns `h("div", {})`; corresponding React branch in `vormaclient/react/src/react.tsx` returns empty fragment and Solid branch in `vormaclient/solid/src/solid.tsx` emits no node in the equivalent state.
- `Current Test Accommodation`: none; node-empty parity requirement is tracked as missing (`FE-UI-013`) rather than narrowed to current divergence.
- `Follow-Up`: align terminal absent-component rendering across adapters to be node-empty (no synthetic wrapper insertion), then add strict parity coverage (`FEC-UI-011`) for this branch.

### VCI-039 (open)

- `Type`: `harness-constraint`
- `Affected Requirements`: `FE-NAV-008`, `FE-NAV-011`
- `Summary`: Legacy frontend tests encode stale debounce/coalescing timing commentary (`5ms`) while current runtime and strict spec contract use `8ms`.
- `Observed Symptom`: Legacy suites such as `vormaclient/client/src/client.events_system.test.ts`, `vormaclient/client/src/client.navigation_lifecycle.begin_navigation_phase.test.ts`, and `vormaclient/client/src/client.form_submissions.revalidate_function.test.ts` contain 5ms timing comments/assertion intent, while runtime source in `vormaclient/client/src/client.ts` uses `REVALIDATION_COALESCE_MS = 8` and debounced status dispatch at `8ms`.
- `Current Test Accommodation`: strict conformance specs/tests use `8ms` contract (`FE-NAV-008`, `FE-NAV-011`) and do not preserve legacy 5ms commentary as normative behavior.
- `Follow-Up`: refresh or retire stale legacy 5ms timing assertions/comments so legacy suites no longer imply conflicting normative timing intent.

### VCI-040 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-VITE-010`
- `Summary`: Vite dev-port helper currently ignores configured/default candidate port during free-port selection.
- `Observed Symptom`: `lab/viteutil/viteutil.go` defines `InitPort(defaultPort int)` but calls `netutil.GetFreePort(5199)` unconditionally, so `defaultPort` input (and upstream `Vite.DefaultPort` wiring from `wave/tooling/builder.go`) is not used for candidate selection.
- `Current Test Accommodation`: none; contract is now explicitly tracked as missing instead of mirroring current hard-coded-candidate behavior.
- `Follow-Up`: use `defaultPort` as the candidate passed to free-port selection, preserve env propagation to `__VITE_PORT`, and add/enable `BDC-VITE-010` conformance coverage.

### VCI-041 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-VITE-011`
- `Summary`: Vite dev-start command failure is currently logged but not propagated as an error.
- `Observed Symptom`: `lab/viteutil/cmd.go` logs `c.cmd.Start()` failure in `DevBuild()` but still returns `nil`, allowing `wave/tooling/builder.go` `NewViteDevContext()` and `wave/tooling/devserver.go` `startVite()` to treat startup as success and publish a Vite context.
- `Current Test Accommodation`: none; requirement is tracked as missing instead of accepting logged-and-suppressed startup failure semantics.
- `Follow-Up`: return startup error from `DevBuild()` on `cmd.Start()` failure, propagate through builder/devserver startup path, and add/enable strict `BDC-VITE-011` coverage.

### VCI-050 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BUILD-ART-004`
- `Summary`: Client-defined loader-only route typing currently derives params/splat with action matcher runes instead of loader matcher runes.
- `Observed Symptom`: In `vormabuild/vorma_gen_ts.go`, the branch that adds client-defined paths without Go loaders (`extraPathPatterns`) uses `actionsDynamicRune` / `actionsSplatRune` for loader-category param/splat extraction. This conflicts with loader typing contract requiring loader matcher rune settings (`loadersDynamicRune`, `loadersSplatRune`) for loader-category entries.
- `Current Test Accommodation`: none; strict build spec already requires loader-category typing to use loader matcher runes (including client-defined loader paths) and has not been narrowed to current implementation behavior.
- `Follow-Up`: switch client-defined loader-path param/splat extraction to loader matcher rune settings, then add/enable focused `BDC-ART-004` coverage for divergent loader/action rune configurations.

### VCI-054 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-FETCH-012`
- `Summary`: Route-data handling currently treats HTTP 304 as allowed but then fails due to unconditional JSON-required path.
- `Observed Symptom`: In `vormaclient/client/src/client.ts`, `responseNotOK` excludes `304` from non-OK failure (`!response.ok && status !== 304`), but JSON parsing is only performed when `response.ok` is true and later `!json` triggers `throw new Error("No JSON response")`. A `304` response therefore still fails navigation through the no-JSON error path.
- `Current Test Accommodation`: none; strict spec keeps `FE-FETCH-012` contract that `304` is non-fatal and does not narrow to current implementation behavior.
- `Follow-Up`: align 304 handling with one explicit policy: either support non-fatal 304 semantics with a valid route-data fallback path or revise the requirement to fail 304 explicitly; then add/enable focused conformance coverage for the chosen policy branch.

### VCI-055 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `FE-LINK-019`
- `Summary`: Repeated prefetch `start()` calls can stack pending timers while `stop()` clears only the latest handle.
- `Observed Symptom`: In `vormaclient/client/src/links.ts`, `start()` overwrites a single `timer` field on each call before begin, and `stop()` clears only that one stored handle. Earlier pending timeout handles can remain active and still invoke prefetch begin/fetch after `stop()` was called.
- `Current Test Accommodation`: none; strict spec now requires lossless pending-begin cancellation and single pending timer semantics rather than mirroring current overwrite behavior.
- `Follow-Up`: make `start()` idempotent for pre-begin phase (single scheduled timer) or track/clear all pending timer handles in `stop()`, then add/enable focused `FEC-LINK-017` coverage.

### VCI-061 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-CONC-004`
- `Summary`: `WithRLock` currently exposes mutating setters through `LockedVorma`, so read-lock callbacks can write lock-protected runtime state.
- `Observed Symptom`: In `vormaruntime/vorma_core.go`, `WithRLock` passes `*LockedVorma` to callback while holding `RLock`, and `LockedVorma` includes mutating setters (`SetPaths`, `SetBuildID`, `SetRouteManifestFile`, `SetRootTemplate`). Callback code can therefore mutate runtime state while only read lock is held, violating documented read-only callback intent.
- `Current Test Accommodation`: none; strict backend concurrency contract now requires `WithRLock` to enforce read-only access (`BR-CONC-004`) and does not mirror mutable-under-read-lock behavior.
- `Follow-Up`: separate read-only vs writable locked callback views (for example distinct wrapper types), or add explicit runtime guards preventing setter use in `WithRLock` callback context; then add/enable focused `BRC-CONC-004` coverage.

### VCI-062 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-LOAD-023`
- `Summary`: `RegisterPatternIfNeeded` has a check-then-register race that can panic under concurrent calls for the same new pattern.
- `Observed Symptom`: In `vormaruntime/route_registry.go`, `RegisterPatternIfNeeded` performs `if !nestedRouter.IsRegistered(pattern) { RegisterNestedPatternWithoutHandler(...) }`. Under concurrent callers, multiple goroutines can observe unregistered state before any registration commits. `RegisterNestedPatternWithoutHandler` ultimately calls duplicate-guarded registration in `kit/mux/nested_mux.go` (`mustRegisterNestedRoute`) which panics on second registration attempt.
- `Current Test Accommodation`: none; strict loader contract now requires concurrent idempotent registration behavior (`BR-LOAD-023`) and does not mirror panic-on-race behavior.
- `Follow-Up`: make registration path atomic/idempotent across the check+register window (for example router-level helper that registers-if-absent under one lock), then add/enable focused `BRC-LOAD-023` coverage.

### VCI-069 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-CONC-003`, `API-GO-014`
- `Summary`: Router route-collection accessors currently return mutable backing collections.
- `Observed Symptom`: `kit/mux/mux.go` returns `rt.allRoutes` directly from `Router.AllRoutes()`, and `kit/mux/nested_mux.go` returns `nr.routes` directly from `NestedRouter.AllRoutes()`. These are exposed through `vormaruntime/vorma_core.go` (`LoadersRouter()` / `ActionsRouter()` accessors) and consumed by runtime/build paths such as `vormaruntime/route_registry.go` and `vormabuild/vorma_gen_ts.go`, so caller mutation can leak into authoritative route metadata flows.
- `Current Test Accommodation`: none; strict spec now explicitly treats router-exposed route collections as snapshot/read-only access surfaces under `BR-CONC-003`/`API-GO-014`.
- `Follow-Up`: return defensive copies from route-collection accessors (or equivalent read-only view semantics) for both router types, then add/enable focused `BRC-CONC-003` coverage that mutates returned route collections and asserts no runtime/build-authoritative state mutation.

### VCI-070 (open)

- `Type`: `impl-bug-candidate`
- `Affected Requirements`: `BR-ACT-006`
- `Summary`: Typed GET-action `HEAD` fallback currently fails default parse flow with unsupported-method error.
- `Observed Symptom`: In `vormaruntime/glue.go`, default action parsing treats only `GET` as query-parse branch and otherwise gates by `supportedMethods[r.Method]`, while default supported methods do not include `HEAD`. In `kit/mux/mux.go`, `HEAD` fallback selects GET matcher (`findBestMatcherAndMatch`) but request method remains `HEAD` when parse input runs (`createReqDataGetter`), so typed GET actions reached via fallback can hit `unsupported method` parse error and return parser-failure status instead of GET-equivalent parse semantics.
- `Current Test Accommodation`: none; strict backend action contract now requires HEAD-fallback parse parity for typed GET actions (`BR-ACT-006`) and does not mirror unsupported-method fallback failure.
- `Follow-Up`: normalize action parse branching for fallback HEAD requests (for example treat `HEAD` as GET-equivalent in default parse path when GET fallback is selected), then add/enable focused `BRC-ACT-006` coverage.
