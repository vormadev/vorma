Main vs Current Wave Regression Report

Scope

- Compare baseline `main` behavior and reference baseline `refactor-2026-1`
  behavior against current branch behavior using requirement IDs from
  `wave/spec/wave-normative-spec.md`.
- API shape changes alone are not regressions.
- A regression is recorded only when the same user story is no longer satisfied
  after appropriate app migration.
- Current behavior that fixes a clear baseline defect is not a regression.

Status

- Requirement-level comparison is complete for all normative requirement IDs.
- Compared IDs currently include: `WAVE-HL01-001`..`WAVE-HL01-017`,
  `WAVE-HL02-001`..`WAVE-HL02-008`, `WAVE-HL03-001`..`WAVE-HL03-007`,
  `WAVE-HL04-001`..`WAVE-HL04-010`, `WAVE-HL05-001`..`WAVE-HL05-008`,
  `WAVE-HL06-001`..`WAVE-HL06-008`, `WAVE-HL07-001`..`WAVE-HL07-009`,
  `WAVE-HL08-001`..`WAVE-HL08-004`, `WAVE-HL09-001`..`WAVE-HL09-004`,
  `WAVE-HL10-001`..`WAVE-HL10-004`, `WAVE-HL11-001`..`WAVE-HL11-003`,
  `WAVE-HL12-001`..`WAVE-HL12-009`, `WAVE-HL13-001`..`WAVE-HL13-009`,
  `WAVE-HL14-001`..`WAVE-HL14-008`, `WAVE-HL15-001`..`WAVE-HL15-009`,
  `WAVE-HL16-001`..`WAVE-HL16-010`, `WAVE-HL17-001`..`WAVE-HL17-010`,
  `WAVE-HL18-001`..`WAVE-HL18-008`, `WAVE-HL19-001`..`WAVE-HL19-007`,
  `WAVE-HL20-001`..`WAVE-HL20-010`, `WAVE-HL21-001`..`WAVE-HL21-007`,
  `WAVE-HL22-001`..`WAVE-HL22-010`, `WAVE-HL23-001`..`WAVE-HL23-007`,
  `WAVE-HL24-001`..`WAVE-HL24-008`, `WAVE-HL25-001`..`WAVE-HL25-008`, and
  `WAVE-HL26-001`..`WAVE-HL26-012`.
- Full-file read coverage for `refactor-2026-1` `wave/*` is complete and mapped
  into requirement extraction.
- No requirement-level regressions are currently confirmed for compared IDs.
- Current branch behavior under active verification includes: none.

Validated Non-Regressions

- `WAVE-HL01-001` Baseline observable behavior statement: Invalid config JSON
  fails during config unmarshal/initialization. Current observable behavior
  statement: Invalid config JSON fails during `ParseConfig`/build validation.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL01-002` Baseline observable behavior statement: Core config is
  required before normal build/runtime logic runs. Current observable behavior
  statement: Core config is required before normal build/runtime logic runs.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL01-003` Baseline observable behavior statement: Dist root is cleaned
  before layout/use. Current observable behavior statement: Dist root is cleaned
  during parse into `ParsedConfig.Dist.Root`. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL01-004` Baseline observable behavior statement: Config-file based flow
  fails when read path is unusable. Current observable behavior statement:
  `ParseConfigFile` rejects empty/whitespace path before read. Determination
  statement: Changed implementation with preserved fail-fast semantics, not a
  regression.
- `WAVE-HL01-005` Baseline observable behavior statement: Config location may
  remain non-canonical. Current observable behavior statement: Config location
  is normalized to absolute path when file parsing succeeds. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL01-006` Baseline observable behavior statement: Main app entry and
  dist directory are required. Current observable behavior statement: Main app
  entry and dist directory are required. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL01-007` Baseline observable behavior statement: Static asset dirs are
  required outside server-only mode. Current observable behavior statement:
  Static asset dirs are required outside server-only mode. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL01-008` Baseline observable behavior statement: Vite config requires a
  package-manager base command. Current observable behavior statement: Vite
  config requires a package-manager base command. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL01-009` Baseline observable behavior statement: Healthcheck endpoint
  accepts broad string input. Current observable behavior statement: Healthcheck
  endpoint is strictly validated as a path-only value. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL01-010` Baseline observable behavior statement: Hook timeout
  coherence/constraints are less explicit. Current observable behavior
  statement: Timeout values and disable/override conflicts are explicitly
  validated. Determination statement: Baseline-defect correction, not a
  regression.
- `WAVE-HL01-011` Baseline observable behavior statement: `RunOnChangeOnly`
  command timing constraints are not explicitly enforced. Current observable
  behavior statement: `RunOnChangeOnly` command hooks enforce pre/default
  timing. Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL01-012` Baseline observable behavior statement: Dual command source
  hook configuration is not explicitly rejected. Current observable behavior
  statement: Hooks reject simultaneous `Cmd` and
  `RunCombinedDevBuildHookCommands`. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL01-013` Baseline observable behavior statement: Public path prefix is
  normalized to safe slash form. Current observable behavior statement: Public
  path prefix is normalized to safe slash form. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL01-014` Baseline observable behavior statement: Browser runtime
  integration names use fixed defaults. Current observable behavior statement:
  Browser runtime integration names keep defaults but allow explicit overrides.
  Determination statement: Changed implementation with preserved default
  semantics, not a regression.
- `WAVE-HL01-015` Baseline observable behavior statement: Port env parsing
  accepts some invalid-range values. Current observable behavior statement: Port
  env parsing enforces valid range and returns `0` otherwise. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL01-016` Baseline observable behavior statement: Dev mode can resolve
  and persist a free app port when needed. Current observable behavior
  statement: Dev mode resolves and persists a free app port when needed.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL01-017` Baseline observable behavior statement: Non-dev/pre-set port
  path can yield unusable default when env is invalid. Current observable
  behavior statement: Non-dev/pre-set path falls back to `8080` when env is
  invalid. Determination statement: Baseline-defect correction, not a
  regression.
- `WAVE-HL02-001` Baseline observable behavior statement: Runtime initialization
  fails fast on missing/invalid config input. Current observable behavior
  statement: Runtime initialization fails fast on missing/invalid config input.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL02-002` Baseline observable behavior statement: Config bytes are held
  by reference after init. Current observable behavior statement: Config bytes
  are cloned before parse/storage. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL02-003` Baseline observable behavior statement: Runtime cache surfaces
  are initialized centrally during init. Current observable behavior statement:
  Runtime cache surfaces are initialized centrally during init. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL02-004` Baseline observable behavior statement: Dev runtime cache
  reads recompute instead of using sticky prod cache. Current observable
  behavior statement: Dev runtime cache reads recompute instead of using sticky
  prod cache. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL02-005` Baseline observable behavior statement: Prod runtime cache
  reads are once-resolved and reused. Current observable behavior statement:
  Prod runtime cache reads are once-resolved and reused. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL02-006` Baseline observable behavior statement: Public-asset existence
  cache behavior can retain stale negatives. Current observable behavior
  statement: Public-asset cache stores only positive/no-error entries.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL02-007` Baseline observable behavior statement: Public file-map
  accessor can expose mutable shared map. Current observable behavior statement:
  Public file-map accessor returns cloned map values. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL02-008` Baseline observable behavior statement: Error-returning and
  panic-on-failure access paths exist for runtime internals. Current observable
  behavior statement: Error-returning and panic-on-failure access paths exist
  for runtime internals. Determination statement: Changed implementation with
  preserved semantics, not a regression.
- `WAVE-HL03-001` Baseline observable behavior statement: Dist layout follows
  stable static/assets/internal contract. Current observable behavior statement:
  Dist layout follows stable static/assets/internal contract. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL03-002` Baseline observable behavior statement: Binary naming is
  platform-correct (`main` vs `main.exe`). Current observable behavior
  statement: Binary naming is platform-correct (`main` vs `main.exe`).
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL03-003` Baseline observable behavior statement: Runtime/build path
  constants operate as relative fs paths. Current observable behavior statement:
  Runtime/build path constants operate as relative fs paths. Determination
  statement: Changed implementation with preserved semantics, not a regression.
- `WAVE-HL03-004` Baseline observable behavior statement: Dist bootstrap creates
  required internal/public/private directories and `.keep`. Current observable
  behavior statement: Dist bootstrap creates required internal/public/private
  directories and `.keep`. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL03-005` Baseline observable behavior statement: Full processing wipes
  static output and rebuilds structure. Current observable behavior statement:
  Full processing removes static contents while preserving lock files, then
  rebuilds structure. Determination statement: Changed implementation with
  preserved clean-build semantics, not a regression.
- `WAVE-HL03-006` Baseline observable behavior statement: Dev runtime reads from
  dist directory while prod reads provided static fs. Current observable
  behavior statement: Dev runtime reads from dist directory while prod reads
  provided static fs. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL03-007` Baseline observable behavior statement: Public/private
  filesystem views are sub-FS projections from base static root. Current
  observable behavior statement: Public/private filesystem views are sub-FS
  projections from base static root. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL04-001` Baseline observable behavior statement: Runtime public filemap
  is loaded from internal gob artifact. Current observable behavior statement:
  Runtime public filemap is loaded from internal gob artifact. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL04-002` Baseline observable behavior statement: Filemap lookup
  normalizes keys and falls back to prefixed original path when missing. Current
  observable behavior statement: Filemap lookup normalizes keys and falls back
  to prefixed original path when missing. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL04-003` Baseline observable behavior statement: Passthrough URL
  behavior is narrower. Current observable behavior statement: Passthrough URL
  behavior explicitly covers additional absolute URL schemes/prefixes.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL04-004` Baseline observable behavior statement: Missing public-filemap
  entries log warning and fall back to unhashed prefixed path. Current
  observable behavior statement: Missing public-filemap entries log warning and
  fall back to unhashed prefixed path. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL04-005` Baseline observable behavior statement: Ref-file URL joining
  is less strict about trimming/normalization edge cases. Current observable
  behavior statement: Ref-file URL joining trims/normalizes and returns empty on
  invalid ref content. Determination statement: Baseline-defect correction, not
  a regression.
- `WAVE-HL04-006` Baseline observable behavior statement: Public filemap URL is
  sourced from internal ref file. Current observable behavior statement: Public
  filemap URL is sourced from internal ref file. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL04-007` Baseline observable behavior statement: Runtime emits preload
  and module script bootstrap with CSP hash for public filemap resolver. Current
  observable behavior statement: Runtime emits preload and module script
  bootstrap with CSP hash for public filemap resolver. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL04-008` Baseline observable behavior statement: Runtime installs
  browser-side public URL resolver under default namespace/function naming.
  Current observable behavior statement: Runtime installs browser-side public
  URL resolver with same default behavior and configurable names. Determination
  statement: Changed implementation with preserved default semantics, not a
  regression.
- `WAVE-HL04-009` Baseline observable behavior statement: Prefix-based static
  routing can claim non-asset requests under prefixed paths. Current observable
  behavior statement: Static routing checks actual public-asset existence before
  static serving. Determination statement: Baseline-defect correction, not a
  regression.
- `WAVE-HL04-010` Baseline observable behavior statement: Favicon redirect uses
  public URL fallback heuristics to detect absence. Current observable behavior
  statement: Favicon redirect uses filemap lookup presence and 404 fallback.
  Determination statement: Changed implementation with preserved observable
  favicon redirect/not-found semantics, not a regression.
- `WAVE-HL05-001` Baseline observable behavior statement: CSS build order runs
  critical before normal (`buildCSS`/`buildAll`). Current observable behavior
  statement: `cssProcessor.buildAll` runs critical build before normal build.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL05-002` Baseline observable behavior statement: Missing CSS entry path
  disables only that CSS branch. Current observable behavior statement: Missing
  entry branch clears tracked branch state and exits without writes.
  Determination statement: Changed implementation with preserved opt-in
  semantics, not a regression.
- `WAVE-HL05-003` Baseline observable behavior statement: Empty esbuild output
  can pass through to output indexing in `main`. Current observable behavior
  statement: Build fails explicitly when esbuild output is empty. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL05-004` Baseline observable behavior statement: CSS URL resolver
  preserves absolute/protocol-relative URLs and resolves relative URLs through
  Wave public-URL mapping. Current observable behavior statement: Same resolver
  passthrough and relative-resolution behavior is preserved. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL05-005` Baseline observable behavior statement: CSS import tracking
  stores per-input absolute paths. Current observable behavior statement:
  Import-set tracking stores normalized absolute/symlink-resolved paths.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL05-006` Baseline observable behavior statement: Critical CSS writes to
  stable internal `critical.css` output. Current observable behavior statement:
  Critical CSS still writes to stable internal `critical.css` output with
  atomic-write no-op-on-unchanged behavior. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL05-007` Baseline observable behavior statement: Normal CSS publishes
  hashed public artifact and writes internal ref to active hashed name. Current
  observable behavior statement: Same hashed-public artifact + internal-ref
  contract is preserved with helperized stale-artifact cleanup. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL05-008` Baseline observable behavior statement: Hot-reload CSS reads
  may fall back to disk after rebuild invalidation. Current observable behavior
  statement: Hot-reload readers can require fresh rebuild output and return
  explicit unavailability instead of stale fallback. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL06-001` Baseline observable behavior statement: Public static outputs
  are hashed by default while private outputs keep relative names. Current
  observable behavior statement: Same public-hashed/private-stable naming policy
  is preserved. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL06-002` Baseline observable behavior statement: `prehashed`/`__nohash`
  source prefixes strip from logical path and mark unhashed behavior. Current
  observable behavior statement: Same logical-path stripping and prehash/nohash
  behavior is preserved. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL06-003` Baseline observable behavior statement: Static ignore-list
  entries (for example `.DS_Store`) are skipped. Current observable behavior
  statement: Ignore-list skip behavior is preserved. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL06-004` Baseline observable behavior statement: Logical-path
  collisions can survive discovery in baseline. Current observable behavior
  statement: Colliding logical static paths fail with explicit collision errors.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL06-005` Baseline observable behavior statement: Full static processing
  discovers source files and processes with worker concurrency. Current
  observable behavior statement: Full static processing keeps concurrent worker
  processing with cancellation on first error. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL06-006` Baseline observable behavior statement: Baseline rebuild path
  primarily re-walks full source maps for granular updates. Current observable
  behavior statement: Changed-path incremental processing updates only affected
  logical entries when safe and falls back to full build when required.
  Determination statement: Changed implementation with preserved semantics and
  reduced unnecessary work, not a regression.
- `WAVE-HL06-007` Baseline observable behavior statement: Missing source
  directory stores empty map but can leave stale output artifacts. Current
  observable behavior statement: Missing source directory clears stale artifacts
  and writes empty map state. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL06-008` Baseline observable behavior statement: Granular map-change
  cleanup removes stale mapped dist artifacts. Current observable behavior
  statement: Stale artifact cleanup remains explicit for map removals/dist-name
  changes, including changed-path removal flows. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL07-001` Baseline observable behavior statement: Build validates config
  before file processing/hooks/schema/compile stages. Current observable
  behavior statement: Build validation remains first gate before mutation
  stages. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL07-002` Baseline observable behavior statement: File processing runs
  before hooks and runs again after hooks. Current observable behavior
  statement: Pre-hook and post-hook processing passes are preserved.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL07-003` Baseline observable behavior statement: User hooks run before
  framework hooks. Current observable behavior statement: User-before-framework
  order is preserved, with framework callback override support added.
  Determination statement: Changed implementation with preserved semantics, not
  a regression.
- `WAVE-HL07-004` Baseline observable behavior statement: Hook-command timeout
  policy is less explicit in baseline. Current observable behavior statement:
  Mode-aware build-hook command timeout policy is explicitly derived and
  applied. Determination statement: Baseline-defect correction, not a
  regression.
- `WAVE-HL07-005` Baseline observable behavior statement: Schema write can be a
  hard build failure in baseline. Current observable behavior statement: Schema
  write failure logs warning and build continues. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL07-006` Baseline observable behavior statement: Compile-go stage is
  optional and production path applies `-tags=prod`. Current observable behavior
  statement: Compile-go optionality and mode-aware prod tag behavior are
  preserved. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL07-007` Baseline observable behavior statement: Baseline compile stage
  runs without explicit framework overlay lifecycle support. Current observable
  behavior statement: Compile stage supports framework go-build overlays with
  explicit cleanup and surfaced cleanup errors. Determination statement: Changed
  implementation with preserved compile semantics, not a regression.
- `WAVE-HL07-008` Baseline observable behavior statement: Server-only mode skips
  browser-asset processing. Current observable behavior statement: Server-only
  mode still skips public/private static and CSS processing branches.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL07-009` Baseline observable behavior statement: Browser processing
  runs public static pass before CSS to satisfy URL map dependencies. Current
  observable behavior statement: Public-static-first ordering is preserved
  before concurrent private-static and CSS branches. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL08-001` Baseline observable behavior statement: Watcher intake batches
  fsnotify bursts through ~30ms debouncing. Current observable behavior
  statement: Debounced watcher intake keeps 30ms window and batch dispatch.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL08-002` Baseline observable behavior statement: Watcher-loop shutdown
  handling is less deterministic in baseline channel-close/cancel races. Current
  observable behavior statement: Watcher loop exits deterministically on context
  cancellation and channel close without extra dispatch. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL08-003` Baseline observable behavior statement: Event processing
  requires live watcher and builder references. Current observable behavior
  statement: Event processing still no-ops when watcher or builder is nil.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL08-004` Baseline observable behavior statement: Config-change events
  short-circuit normal event execution and trigger config restart. Current
  observable behavior statement: Config-change trigger still short-circuits
  normal event execution and enqueues config restart semantics. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL09-001` Baseline observable behavior statement: Event intake applies
  one pass that decides config restart, add-dir behavior, and event
  classification eligibility per batch. Current observable behavior statement:
  Pre-classification builds one deterministic plan containing config-change
  flag, add-directory-watch paths, and events-to-classify. Determination
  statement: Changed implementation with preserved semantics, not a regression.
- `WAVE-HL09-002` Baseline observable behavior statement: Config-file detection
  in baseline relies on simpler absolute-path equality and is less resilient to
  alias/symlink/missing-target path shapes. Current observable behavior
  statement: Config detection uses location-equivalence path comparison.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL09-003` Baseline observable behavior statement: Create/rename
  directory events add new directories into watcher coverage. Current observable
  behavior statement: Pre-classification add-dir plan + side effects preserve
  add-directory watch semantics before classification. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL09-004` Baseline observable behavior statement: Add-directory watch
  failures are broadly logged. Current observable behavior statement: Expected
  transient add-dir errors (`not exist`, `ENOTDIR`) are suppressed while
  actionable errors are logged. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL10-001` Baseline observable behavior statement: Duplicate watcher
  paths in one batch collapse to one effective event key. Current observable
  behavior statement: Duplicate events merge operations per deduplicated logical
  path key. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL10-002` Baseline observable behavior statement: Deduplicated event map
  iteration order is non-deterministic in baseline. Current observable behavior
  statement: Deduplicated event output is sorted by path for deterministic
  ordering. Determination statement: Baseline-defect correction, not a
  regression.
- `WAVE-HL10-003` Baseline observable behavior statement: Path aliases for the
  same absolute location can survive as separate dedupe keys. Current observable
  behavior statement: Canonical-path and missing-alias probes merge alias keys
  into one logical event path. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL10-004` Baseline observable behavior statement: Ignored/chmod-only
  events are filtered from actionable event flow. Current observable behavior
  statement: Post-classification decision filter removes ignored/chmod-only
  events before execution planning. Determination statement: Preserved behavior,
  not a regression.
- `WAVE-HL11-001` Baseline observable behavior statement: Empty watcher input
  batches produce no work plan. Current observable behavior statement: Empty
  classify/plan inputs return no processable events and no config-restart
  trigger. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL11-002` Baseline observable behavior statement: File classes
  (go/css/static/other) drive distinct downstream semantics. Current observable
  behavior statement: Classified events preserve per-file `fileType` through
  hook/build/browser planning. Determination statement: Preserved behavior, not
  a regression.
- `WAVE-HL11-003` Baseline observable behavior statement: Duplicate-pattern
  handling suppresses redundant hook execution but can collapse event-level
  traceability. Current observable behavior statement: Planning preserves
  one-to-one event descriptors while duplicate-hook suppression is tracked by
  `skipDuplicateHooks`. Determination statement: Changed implementation with
  preserved semantics, not a regression.
- `WAVE-HL12-001` Baseline observable behavior statement: Hook callback context
  path surfaces are less normalized and batch path metadata is less explicit.
  Current observable behavior statement: Hook contexts use normalized absolute
  `FilePath` and pattern-scoped deduped `ChangedFilePaths`. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL12-002` Baseline observable behavior statement: Repeated matched
  patterns execute hook stages once per batch intent. Current observable
  behavior statement: `skipDuplicateHooks` preserves one execution per pattern
  per stage while keeping event descriptors. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL12-003` Baseline observable behavior statement: Shared hook context
  objects can leak callback-local mutation across hook callbacks. Current
  observable behavior statement: Callback execution clones hook context and
  changed-path slices per callback run. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL12-004` Baseline observable behavior statement: Run-on-change-only
  semantics suppress standard build side effects but command/callback behavior
  is less explicit by stage. Current observable behavior statement:
  Run-on-change-only stage resolution suppresses command actions while
  preserving callback execution where applicable. Determination statement:
  Changed implementation with preserved semantics, not a regression.
- `WAVE-HL12-005` Baseline observable behavior statement: Hook callback panics
  can terminate execution without structured hook error return. Current
  observable behavior statement: Callback panics are recovered as execution
  errors and command phase for that plan is skipped. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL12-006` Baseline observable behavior statement: Hook execution errors
  have less consistent stage/path attribution in baseline. Current observable
  behavior statement: Hook errors are wrapped with stage label and changed-path
  attribution. Determination statement: Baseline-defect correction, not a
  regression.
- `WAVE-HL12-007` Baseline observable behavior statement: Concurrent stage
  action accumulation order can vary by goroutine completion timing. Current
  observable behavior statement: Concurrent stage runs in parallel but flattens
  actions in descriptor index order. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL12-008` Baseline observable behavior statement: No-wait hook execution
  is asynchronous without explicit bounded concurrency/lifecycle gating. Current
  observable behavior statement: No-wait hook execution uses bounded limiter
  capacity and run-cycle/lifecycle cancellation checks. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL12-009` Baseline observable behavior statement: Stage timeout policy
  and per-hook overrides are less explicit. Current observable behavior
  statement: Stage timeout defaults and per-hook override/disable flags are
  resolved per execution plan and applied to command/callback contexts.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL13-001` Baseline observable behavior statement: Hard-reload
  sensitivity derives from go-file detection and watched-file restart/recompile
  flags. Current observable behavior statement: Event-to-hook planning sets
  `needsHardReload` when file type is go or watched-file hard-reload flags are
  present. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL13-002` Baseline observable behavior statement: Single hard-reload
  events stop app inline, and batches with any hard-reload event stop app once
  before batch processing. Current observable behavior statement: App-stop
  strategy resolves deterministically to `single-event-hard-reload`,
  `batch-hard-reload`, or `none` before execution. Determination statement:
  Changed implementation with preserved semantics, not a regression.
- `WAVE-HL13-003` Baseline observable behavior statement: Implicit build work is
  skipped only when every event in scope is run-on-change-only. Current
  observable behavior statement: `shouldRunImplicitBuildForEvents` returns false
  only when all events are run-on-change-only. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL13-004` Baseline observable behavior statement: Restart actions from
  hook execution short-circuit remaining pipeline work and enqueue restart.
  Current observable behavior statement: Hook-stage continuation decisions stop
  the pipeline and trigger restart request enqueue when restart actions are
  present. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL13-005` Baseline observable behavior statement: Hook-stage failures
  are effectively fail-open and not policy-configurable. Current observable
  behavior statement: Hook-stage continuation policy resolves `fail-open` vs
  `fail-closed` explicitly per stage. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL13-006` Baseline observable behavior statement: Run-on-change-only
  implicit-build skip logs differ between single and batch paths. Current
  observable behavior statement: Skip logging emits deterministic single-event
  vs all-events messages before build-phase branch selection. Determination
  statement: Changed implementation with preserved semantics, not a regression.
- `WAVE-HL13-007` Baseline observable behavior statement: Browser-phase work is
  suppressed when restart short-circuit occurs. Current observable behavior
  statement: Browser phase executes only when all stage continuation decisions
  allow continuation. Determination statement: Changed implementation with
  preserved restart short-circuit semantics, not a regression.
- `WAVE-HL13-008` Baseline observable behavior statement: Build execution
  decisions control compile-go, CSS, and static work, with static paths
  typically reprocessed as full passes. Current observable behavior statement:
  Build-phase execution decomposes compile-go, CSS flags, static mode
  (`none/full/changed-paths`), and framework filemap-write decisions
  deterministically. Determination statement: Changed implementation with
  preserved semantics and reduced unnecessary work, not a regression.
- `WAVE-HL13-009` Baseline observable behavior statement: Invalidate-vite action
  attempts invalidate when Vite is active and falls back to hard reload when
  unavailable/failing. Current observable behavior statement: Browser decision
  fallback resolution preserves invalidate-attempt with deterministic
  hard-reload fallback and category dispatch. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL14-001` Baseline observable behavior statement: App runtime starts by
  executing the dist binary as a child process with stdout/stderr attached.
  Current observable behavior statement: App process manager starts the dist
  binary with stdout/stderr attached and stores process handle on success.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL14-002` Baseline observable behavior statement: App process state is
  managed as persistent devserver runtime state across restart operations.
  Current observable behavior statement: App process manager is lazily
  initialized once and reused for subsequent start/stop operations.
  Determination statement: Changed implementation with preserved semantics, not
  a regression.
- `WAVE-HL14-003` Baseline observable behavior statement: Graceful app shutdown
  is attempted before force termination fallback. Current observable behavior
  statement: Stop path sends interrupt and waits for graceful completion before
  kill fallback. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL14-004` Baseline observable behavior statement: Hung app shutdown is
  bounded and force-terminated after timeout. Current observable behavior
  statement: Graceful-stop timeout triggers force kill and wait completion.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL14-005` Baseline observable behavior statement: Benign
  already-terminated process races can surface as noisy stop errors. Current
  observable behavior statement: Known benign termination/wait errors are
  ignored before joined error return. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL14-006` Baseline observable behavior statement: Rebuild cleanup can
  leave prior-cycle asynchronous contexts active while teardown begins. Current
  observable behavior statement: Cleanup cancels run-cycle scope and no-wait
  hook lifecycle context before stopping app/watcher/builder. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL14-007` Baseline observable behavior statement: Watcher and builder
  pointers are swapped to nil under lock before close in rebuild cleanup.
  Current observable behavior statement: Same lock-guarded pointer nil-before-
  close teardown contract is preserved. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL14-008` Baseline observable behavior statement: Full shutdown stops
  refresh HTTP server and cancels/waits refresh manager loop. Current observable
  behavior statement: Full shutdown performs bounded refresh-server shutdown
  plus refresh-manager cancel-and-wait cleanup. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL15-001` Baseline observable behavior statement: Vite dev context
  creation is a no-op when Vite is not configured. Current observable behavior
  statement: `NewViteDevContext` returns nil and startup path no-ops when Vite
  is disabled. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL15-002` Baseline observable behavior statement: Vite startup occurs
  after build completion and before app start for Vite-enabled runs. Current
  observable behavior statement: Run-cycle runtime starts Vite after successful
  build and before app start when Vite context is absent. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL15-003` Baseline observable behavior statement: Stopping Vite cleans
  process state and clears active context pointer. Current observable behavior
  statement: `stopVite` cleanup clears active Vite context state. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL15-004` Baseline observable behavior statement: Vite cycle logic is
  gated on Vite-enabled config and active Vite context presence. Current
  observable behavior statement: Cycle path exits unless Vite is enabled and
  active context exists. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL15-005` Baseline observable behavior statement: Vite cycle flow does
  not surface explicit success/failure outcome after readiness verification.
  Current observable behavior statement: Cycle flow reports applied outcome only
  when stop/start and readiness all succeed. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL15-006` Baseline observable behavior statement: Vite readiness probing
  uses a single localhost endpoint. Current observable behavior statement:
  Readiness probes both `127.0.0.1` and `localhost` `/@vite/client` endpoints.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL15-007` Baseline observable behavior statement: Invalidate-vite flow
  POSTs to `__vorma_invalidate_filemap` and treats non-200 as failure. Current
  observable behavior statement: Endpoint contract and status handling are
  preserved. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL15-008` Baseline observable behavior statement: Cycle-vite reload flow
  can emit both Vite reconnect reload and Wave hard-reload payload. Current
  observable behavior statement: Reload orchestration suppresses Wave payload
  broadcast when Vite cycle is successfully applied. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL15-009` Baseline observable behavior statement: Production Vite build
  is optional and runs only when Vite is configured, using configured command
  context/output targets. Current observable behavior statement: `ViteProdBuild`
  remains config-gated and uses configured command, cwd, outdir, manifest, and
  config file. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL16-001` Baseline observable behavior statement: Refresh infrastructure
  starts only when browser mode is enabled. Current observable behavior
  statement: Refresh manager/server startup remains browser-mode scoped.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL16-002` Baseline observable behavior statement: Refresh startup seeks
  a usable port before serving. Current observable behavior statement: Listener
  bind falls back to ephemeral port when requested/default port bind fails.
  Determination statement: Changed implementation with preserved semantics and
  stronger fallback behavior, not a regression.
- `WAVE-HL16-003` Baseline observable behavior statement: Effective refresh port
  is published through wave refresh-port setter for script/runtime consumers.
  Current observable behavior statement: Actual listener port is published
  before serving begins. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL16-004` Baseline observable behavior statement: Refresh mux exposes
  `/events` websocket and `/get-refresh-script-inner` JS script endpoints with
  permissive CORS headers. Current observable behavior statement: Endpoint and
  CORS contracts are preserved. Determination statement: Preserved behavior, not
  a regression.
- `WAVE-HL16-005` Baseline observable behavior statement: Refresh script runtime
  naming uses fixed defaults. Current observable behavior statement: Script
  endpoint resolves runtime naming from parsed-config defaults/overrides.
  Determination statement: Changed implementation with preserved default
  semantics and explicit override support, not a regression.
- `WAVE-HL16-006` Baseline observable behavior statement: Register/unregister
  channels are buffered and slow-client notify sends are skipped to prevent
  broadcaster blocking. Current observable behavior statement: Buffered manager
  channels and non-blocking per-client notify send behavior are preserved.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL16-007` Baseline observable behavior statement: Refresh manager
  shutdown closes clients and drains pending channels. Current observable
  behavior statement: Shutdown cleanup closes client resources and drains
  register/unregister/broadcast channels before done signal. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL16-008` Baseline observable behavior statement: Websocket upgrades are
  rejected with `503` while refresh manager context is canceled. Current
  observable behavior statement: Shutdown gating rejects new websocket clients
  during context cancellation. Determination statement: Preserved behavior, not
  a regression.
- `WAVE-HL16-009` Baseline observable behavior statement: Client
  disconnect/write failure paths attempt unregister without blocking on
  full/closing channels. Current observable behavior statement: Read/write loop
  exits preserve guarded non-blocking unregister behavior. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL16-010` Baseline observable behavior statement: Refresh server
  shutdown is timeout-bounded and clears server pointer after successful
  shutdown. Current observable behavior statement: Timeout-bounded `Shutdown`
  and pointer clear behavior are preserved. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL17-001` Baseline observable behavior statement: Rebuilding overlay
  payload broadcasts are emitted only when browser refresh infrastructure is
  active. Current observable behavior statement: Rebuilding broadcasts are gated
  by browser/manager enablement and active refresh context. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL17-002` Baseline observable behavior statement: Reload orchestration
  applies app/vite readiness waits according to reload options. Current
  observable behavior statement: Readiness waits remain option-driven and
  execute before payload broadcast decision. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL17-003` Baseline observable behavior statement: Invalidate-vite action
  attempts invalidate when Vite is active and falls back to hard reload on
  failure/unavailability. Current observable behavior statement: Invalidate
  attempt and hard-reload fallback semantics are preserved. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL17-004` Baseline observable behavior statement: Reload payload mapping
  uses `changeType=other` for hard reload and `changeType=revalidate` for
  revalidate. Current observable behavior statement: Action-to-payload mapping
  is preserved. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL17-005` Baseline observable behavior statement: Hot-reload CSS payload
  reads can fall back to potentially stale disk outputs. Current observable
  behavior statement: Hot-reload payload generation requires fresh outputs and
  skips unavailable branches with warnings. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL17-006` Baseline observable behavior statement: Refresh script saves
  scroll position before hard reload and restores it after reconnect. Current
  observable behavior statement: Scroll save/restore behavior and session key
  contract are preserved. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL17-007` Baseline observable behavior statement: Normal CSS hot update
  inserts a new stylesheet link and removes prior link after load when present.
  Current observable behavior statement: In-place normal CSS link swap semantics
  are preserved. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL17-008` Baseline observable behavior statement: Critical CSS update
  decodes base64 and replaces existing critical style element in-place when
  present. Current observable behavior statement: Critical CSS replacement
  semantics and element-ID contract are preserved. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL17-009` Baseline observable behavior statement: Revalidate flow
  invokes browser revalidate hook but overlay cleanup on asynchronous failure is
  less explicit. Current observable behavior statement: Revalidate branch
  removes overlay in success, failure, and missing-function paths. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL17-010` Baseline observable behavior statement: Refresh websocket
  close/error triggers hard reload, and beforeunload suppresses
  intentional-close reload loop. Current observable behavior statement:
  Disconnection fallback and beforeunload suppression behavior are preserved.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL18-001` Baseline observable behavior statement: Readiness polling uses
  bounded attempts, linear delay growth, max total wait budget, and request
  timeout bounds. Current observable behavior statement: Same bounded readiness
  policy is preserved via centralized wait-policy derivation. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL18-002` Baseline observable behavior statement: Multi-endpoint
  readiness probing can include empty/duplicate URLs when candidates are
  aggregated. Current observable behavior statement: Readiness probe input drops
  empty URLs and deduplicates candidates before polling. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL18-003` Baseline observable behavior statement: Readiness success
  requires HTTP 200 but response-body cleanup is less consistent in baseline
  probing paths. Current observable behavior statement: Readiness returns
  success only for HTTP 200 and closes response bodies on every attempt.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL18-004` Baseline observable behavior statement: App readiness URL
  composition uses local host + resolved app port + configured healthcheck path.
  Current observable behavior statement: App readiness URL composition remains
  deterministic through helper-based URL resolution. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL18-005` Baseline observable behavior statement: Vite readiness probe
  host handling is narrower in baseline. Current observable behavior statement:
  Candidate probe URLs include both loopback IPv4 and localhost aliases.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL18-006` Baseline observable behavior statement: Hook timeout policy is
  less explicit for stage defaults, per-hook overrides, and stage-disable flags.
  Current observable behavior statement: Hook timeout resolution explicitly
  applies stage policy with per-hook override and disable semantics per
  execution plan. Determination statement: Baseline-defect correction, not a
  regression.
- `WAVE-HL18-007` Baseline observable behavior statement: Build-hook timeout
  behavior is not mode-scoped in baseline. Current observable behavior
  statement: Build-hook timeout derivation is mode-specific
  (`DevBuildHookTimeoutMilliseconds` vs `ProdBuildHookTimeoutMilliseconds`).
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL18-008` Baseline observable behavior statement: Optional-timeout
  execution-context derivation is less explicit in baseline hook/build paths.
  Current observable behavior statement: Optional timeout returns parent context
  when disabled and timeout-derived context with cancel function when positive.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL19-001` Baseline observable behavior statement: Public URL passthrough
  handling is narrower and primarily data-URL scoped. Current observable
  behavior statement: Public URL resolver returns absolute/protocol URLs
  unchanged (`data/http/https/ws/wss/blob/file` and protocol-relative forms)
  before filemap lookup. Determination statement: Baseline-defect correction,
  not a regression.
- `WAVE-HL19-002` Baseline observable behavior statement: Public-asset
  classification can rely on raw prefix checks without full path normalization
  and root/empty guards. Current observable behavior statement: Public-asset
  classification path-cleans input, enforces prefix membership, and rejects
  root/empty/dot asset paths before fs stat. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL19-003` Baseline observable behavior statement: Public-asset existence
  checks may treat directories as assets when stat succeeds. Current observable
  behavior statement: Public-asset existence requires stat success and
  non-directory file info; missing/stat errors resolve false. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL19-004` Baseline observable behavior statement: Static middleware
  serves requests classified as public assets and delegates all other requests
  to the next handler. Current observable behavior statement: Static middleware
  keeps explicit serve-vs-delegate branching by asset predicate. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL19-005` Baseline observable behavior statement: Immutable static
  handler mode injects `Cache-Control: public, max-age=31536000, immutable`.
  Current observable behavior statement: Same immutable cache header contract is
  preserved. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL19-006` Baseline observable behavior statement: Static serving strips
  configured public prefix before filesystem lookup. Current observable behavior
  statement: Prefix-strip behavior via static handler adapter is preserved.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL19-007` Baseline observable behavior statement: Favicon middleware
  resolves `favicon.ico` through public URL/filemap semantics and returns `404`
  when unresolved. Current observable behavior statement: Middleware resolves
  favicon via explicit filemap lookup and redirects when found; missing/read
  failure paths return `404`. Determination statement: Changed implementation
  with preserved semantics, not a regression.
- `WAVE-HL20-001` Baseline observable behavior statement: Dev run acquires
  project lock before startup and releases it on run exit. Current observable
  behavior statement: Dev lock acquisition/release lifecycle remains enforced
  around `RunDev`. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL20-002` Baseline observable behavior statement: Lock path resolves to
  `dist/static/.wave-dev.lock`. Current observable behavior statement:
  Deterministic project-scoped lock location is preserved. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL20-003` Baseline observable behavior statement: Lock creation in
  baseline can race because existing lock check and write are not atomic.
  Current observable behavior statement: Lock acquisition uses exclusive-create
  semantics and writes/closes PID before considering lock acquired.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL20-004` Baseline observable behavior statement: Active lock owner
  detection returns lock-held error including PID metadata. Current observable
  behavior statement: Lock-held error with PID metadata is preserved.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL20-005` Baseline observable behavior statement: Stale lock recovery
  can remove lock files without verifying ownership did not change. Current
  observable behavior statement: Recovery re-reads lock contents and removes
  only when bytes still match original stale-read snapshot. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL20-006` Baseline observable behavior statement: Invalid/non-parseable
  lock content can be treated as immediately recoverable. Current observable
  behavior statement: Invalid lock content is treated as held until stale-age
  threshold is exceeded. Determination statement: Baseline-defect correction,
  not a regression.
- `WAVE-HL20-007` Baseline observable behavior statement: Lock release can fail
  when lock file is already absent. Current observable behavior statement:
  Missing lock-file removal is ignored for idempotent release behavior.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL20-008` Baseline observable behavior statement: Full static cleanup in
  baseline can remove lock coordination files with other static entries. Current
  observable behavior statement: Full static cleanup preserves wave lock files
  while removing other static entries. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL20-009` Baseline observable behavior statement: Restart queue
  coalescing merges pending/incoming requests with strongest-action semantics
  and config-restart precedence. Current observable behavior statement: Restart
  intent accumulator preserves OR merge semantics for non-config requests and
  config restart precedence under queue mutex. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL20-010` Baseline observable behavior statement: Build-retry wait mode
  consumes first pending restart request deterministically for the next retry
  pass. Current observable behavior statement: Waiting-for-build-retry path
  bypasses merge upgrades and preserves first queued restart intent.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL21-001` Baseline observable behavior statement: CLI parsing recognizes
  `-dev`, `-hook`, and `-no-binary` flags and dispatches run mode from those
  flags. Current observable behavior statement: Explicit CLI option parsing
  preserves the same flag contract and surfaces parse errors as returned errors.
  Determination statement: Changed implementation with preserved semantics, not
  a regression.
- `WAVE-HL21-002` Baseline observable behavior statement: Hook-only mode invokes
  caller hook with current dev flag and skips build/dev loops. Current
  observable behavior statement: Hook-only branch executes caller hook
  (`isDev`-aware) and bypasses dev/build paths. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL21-003` Baseline observable behavior statement: Dev CLI mode delegates
  to devserver runtime and skips production build path. Current observable
  behavior statement: Dev branch dispatches to `RunDev` and returns without
  entering build branch. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL21-004` Baseline observable behavior statement: Non-dev CLI path runs
  build with compile-go disabled only when `-no-binary` is set. Current
  observable behavior statement: Build dispatch preserves compile-go toggle
  semantics (`CompileGo: !NoBinary`). Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL21-005` Baseline observable behavior statement: Convenience CLI entry
  path parses command-line args from `os.Args[1:]`. Current observable behavior
  statement: `BuildWaveWithHook` delegates to args-based helper using
  `os.Args[1:]`. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL21-006` Baseline observable behavior statement: Build-only convenience
  entrypoint behaves as hook-enabled entrypoint with nil hook callback. Current
  observable behavior statement: `BuildWave` remains a thin wrapper over
  `BuildWaveWithHook(..., nil)`. Determination statement: Preserved behavior,
  not a regression.
- `WAVE-HL21-007` Baseline observable behavior statement: Nil logger input is
  normalized to colorlog default before run operations. Current observable
  behavior statement: CLI/build/dev entrypoints still normalize nil logger to
  deterministic colorlog default. Determination statement: Preserved behavior,
  not a regression.
- `WAVE-HL22-001` Baseline observable behavior statement: Reload parse/read
  failure returns error before config swap. Current observable behavior
  statement: Reload parse/load failures return without mutating active config.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL22-002` Baseline observable behavior statement: Reload validation
  failure returns error before config swap. Current observable behavior
  statement: Validation failure returns without mutating active config.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL22-003` Baseline observable behavior statement: Reload preserves
  framework runtime fields before installing replacement config. Current
  observable behavior statement: Runtime-field preservation is expanded and
  cloned across schema extensions, watch patterns, callbacks, overlays, and
  browser runtime naming fields. Determination statement: Changed implementation
  with preserved semantics, not a regression.
- `WAVE-HL22-004` Baseline observable behavior statement: Event processing exits
  early when watcher or builder references are unavailable. Current observable
  behavior statement: Nil watcher/builder guard remains a strict no-op gate
  before planning/classification/execution. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL22-005` Baseline observable behavior statement: Build-phase failures
  can continue to downstream reload paths in baseline execution flow. Current
  observable behavior statement: Build failure cancels combined stage and stops
  pipeline before post hooks/browser phase. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL22-006` Baseline observable behavior statement: Parsed-config accessor
  can expose mutable live config object including unstable internal fields.
  Current observable behavior statement: Public parsed-config accessor returns
  defensive cloned snapshot and omits internal-only callback/schema fields.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL22-007` Baseline observable behavior statement: Tooling access to
  buildtime framework internals relies on shared mutable parsed-config object.
  Current observable behavior statement: Buildtime parsed-config accessor
  returns defensive clone while including required framework schema extensions
  and build callbacks. Determination statement: Baseline-defect correction, not
  a regression.
- `WAVE-HL22-008` Baseline observable behavior statement: Raw config bytes
  accessor can expose mutable internal slice. Current observable behavior
  statement: `RawConfigJSON` returns cloned bytes on each call. Determination
  statement: Baseline-defect correction, not a regression.
- `WAVE-HL22-009` Baseline observable behavior statement: Public filemap
  accessor can expose mutable internal map reference. Current observable
  behavior statement: `GetPublicFileMap` returns cloned map snapshot per call.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL22-010` Baseline observable behavior statement: Hook callback panic
  can crash event execution. Current observable behavior statement: Callback
  panics are recovered and surfaced as execution errors wrapped with stage/path
  attribution. Determination statement: Baseline-defect correction, not a
  regression.
- `WAVE-HL23-001` Baseline observable behavior statement: Dist binary path uses
  platform-correct executable naming (`main` vs `main.exe`). Current observable
  behavior statement: Dist binary path uses platform-correct executable naming
  (`main` vs `main.exe`). Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL23-002` Baseline observable behavior statement: Relative path
  constants for runtime/build internals are slash-based and `fs.FS`-compatible.
  Current observable behavior statement: Relative path constants remain
  slash-based and `fs.FS`-compatible. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL23-003` Baseline observable behavior statement: Path
  trim/normalization behavior is less explicit for whitespace-only inputs.
  Current observable behavior statement: Path normalization explicitly trims
  whitespace and maps empty input to an empty sentinel before clean/abs
  resolution. Determination statement: Baseline-defect correction, not a
  regression.
- `WAVE-HL23-004` Baseline observable behavior statement: Config-path
  equivalence checks use straightforward absolute-path equality and are less
  robust across symlink/missing-leaf aliasing. Current observable behavior
  statement: Path-equivalence canonicalization resolves symlinks with
  parent-symlink fallback for missing targets before equality checks.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL23-005` Baseline observable behavior statement: Config watch-directory
  derivation and subscription for config paths outside watch root are less
  explicit. Current observable behavior statement: Config watch-directory
  derivation explicitly handles file-vs-directory inputs and subscribes to the
  resolved config directory. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL23-006` Baseline observable behavior statement: Lock PID liveness
  probes are platform-specific (`signal(0)` on Unix and process-handle query on
  Windows). Current observable behavior statement: Lock PID liveness probes
  remain platform-specific (`signal(0)` on Unix and process-handle query on
  Windows). Determination statement: Preserved behavior, not a regression.
- `WAVE-HL23-007` Baseline observable behavior statement: Watch patterns and
  ignored paths are normalized from watch-root context into absolute slash form
  before matching. Current observable behavior statement: Watch patterns and
  ignored paths are normalized from watch-root context into absolute slash form
  before matching. Determination statement: Changed implementation with
  preserved semantics, not a regression.
- `WAVE-HL24-001` Baseline observable behavior statement: Framework watch
  pattern registration can retain caller-owned mutable slices. Current
  observable behavior statement: Framework watch pattern registration
  defensively clones watched-file and hook-exclude data before storing runtime
  state. Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL24-002` Baseline observable behavior statement: Framework ignored
  patterns and public filemap TS out-dir runtime settings feed watcher/build
  behavior. Current observable behavior statement: Framework ignored patterns
  and public filemap TS out-dir runtime settings continue feeding watcher/build
  behavior. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL24-003` Baseline observable behavior statement: Framework build-hook
  integration is command-centric with narrower runtime hook/overlay integration.
  Current observable behavior statement: Framework build-hook integration
  includes command hooks, callback-based runner hooks, and Go build-overlay
  lifecycle integration. Determination statement: Changed implementation with
  preserved and expanded semantics, not a regression.
- `WAVE-HL24-004` Baseline observable behavior statement: Browser runtime
  namespace and function identifiers are fixed defaults. Current observable
  behavior statement: Browser runtime namespace and function identifiers are
  configurable with stable default fallback behavior. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL24-005` Baseline observable behavior statement: Framework schema
  extensions are merged into generated config schema properties. Current
  observable behavior statement: Framework schema extensions remain merged into
  generated config schema properties. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL24-006` Baseline observable behavior statement: Config reload
  preserves a narrower subset of framework runtime fields before config swap.
  Current observable behavior statement: Config reload preserves and clones
  expanded framework runtime fields (schema extensions, watch patterns,
  callbacks, overlays, and browser runtime naming fields) before config swap.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL24-007` Baseline observable behavior statement: Framework callback
  hook context provides narrower path/batch metadata with less explicit
  per-callback isolation. Current observable behavior statement: Framework
  callback hook context uses normalized absolute file paths, batch-aware changed
  file lists, and per-callback cloned context values. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL24-008` Baseline observable behavior statement: Framework integration
  behavior is driven through shared wave runtime/tooling surfaces rather than
  adapter-specific semantics. Current observable behavior statement: Framework
  integration behavior remains driven through shared wave runtime/tooling
  surfaces rather than adapter-specific semantics. Determination statement:
  Preserved behavior, not a regression.
- `WAVE-HL25-001` Baseline observable behavior statement: Development mode is
  environment-signaled and dev entrypoints set dev mode before lifecycle work.
  Current observable behavior statement: Development mode remains
  environment-signaled and dev entrypoints set dev mode before lifecycle work.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL25-002` Baseline observable behavior statement: Runtime cache access
  recomputes in dev and memoizes in production. Current observable behavior
  statement: Runtime cache access recomputes in dev and memoizes in production.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL25-003` Baseline observable behavior statement: Port-resolution
  behavior in non-dev/pre-set paths is less strict for invalid range values.
  Current observable behavior statement: Port-resolution behavior explicitly
  validates range, uses deterministic fixed-mode fallback, and uses one-time dev
  free-port allocation when needed. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL25-004` Baseline observable behavior statement: Dev runtime base FS
  reads from live dist static output and production uses provided static FS.
  Current observable behavior statement: Dev runtime base FS reads from live
  dist static output and production uses provided static FS. Determination
  statement: Changed implementation with preserved semantics, not a regression.
- `WAVE-HL25-005` Baseline observable behavior statement: CLI/dev dispatch
  separates long-lived dev lifecycle from bounded production build execution.
  Current observable behavior statement: CLI/dev dispatch continues separating
  long-lived dev lifecycle from bounded production build execution.
  Determination statement: Preserved behavior, not a regression.
- `WAVE-HL25-006` Baseline observable behavior statement: Production Go build
  compilation includes `-tags=prod`, while dev compilation omits prod tags.
  Current observable behavior statement: Production Go build compilation
  includes `-tags=prod`, while dev compilation omits prod tags. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL25-007` Baseline observable behavior statement: Refresh script markup
  and hash surfaces return empty values outside dev mode. Current observable
  behavior statement: Refresh script markup and hash surfaces return empty
  values outside dev mode. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL25-008` Baseline observable behavior statement: Browser lifecycle
  gating in server-only mode is less strict in older baseline paths. Current
  observable behavior statement: Browser-specific refresh server and browser
  reload orchestration are gated off in server-only mode, and browser/static
  build branches are skipped. Determination statement: Baseline-defect
  correction, not a regression.
- `WAVE-HL26-001` Baseline observable behavior statement: Constructor fails fast
  on missing or invalid config payload. Current observable behavior statement:
  Constructor fails fast on missing or invalid config payload. Determination
  statement: Changed implementation with preserved semantics, not a regression.
- `WAVE-HL26-002` Baseline observable behavior statement: Constructor can retain
  caller-owned config byte slice by reference. Current observable behavior
  statement: Constructor clones caller config bytes before parse/storage.
  Determination statement: Baseline-defect correction, not a regression.
- `WAVE-HL26-003` Baseline observable behavior statement: Runtime logger
  resolves to a default logger when nil input is provided, and logger accessor
  returns active logger. Current observable behavior statement: Runtime logger
  resolves to a default logger when nil input is provided, and logger accessor
  returns active logger. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL26-004` Baseline observable behavior statement: Filesystem APIs expose
  both error-returning getters and panic-on-failure must variants. Current
  observable behavior statement: Filesystem APIs expose both error-returning
  getters and panic-on-failure must variants. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL26-005` Baseline observable behavior statement: Public URL lookup uses
  hashed-map lookup with prefixed fallback, but public filemap accessor can
  expose mutable internal map state. Current observable behavior statement:
  Public URL lookup keeps hashed-map lookup with prefixed fallback and public
  filemap accessor returns defensive cloned snapshots. Determination statement:
  Baseline-defect correction, not a regression.
- `WAVE-HL26-006` Baseline observable behavior statement: CSS runtime helper
  APIs return empty outputs when CSS entries or dist outputs are unavailable.
  Current observable behavior statement: CSS runtime helper APIs return empty
  outputs when CSS entries or dist outputs are unavailable. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL26-007` Baseline observable behavior statement: Public filemap helper
  APIs provide bootstrap elements and CSP hash outputs from cached detail data.
  Current observable behavior statement: Public filemap helper APIs provide
  bootstrap elements and CSP hash outputs from cached detail data. Determination
  statement: Preserved behavior, not a regression.
- `WAVE-HL26-008` Baseline observable behavior statement: Static serving API
  provides both handler and middleware composition forms. Current observable
  behavior statement: Static serving API provides both handler and middleware
  composition forms. Determination statement: Preserved behavior, not a
  regression.
- `WAVE-HL26-009` Baseline observable behavior statement: Favicon middleware
  redirects to resolved favicon asset and returns `404` when unresolved. Current
  observable behavior statement: Favicon middleware redirects to resolved
  favicon asset and returns `404` when unresolved. Determination statement:
  Changed implementation with preserved semantics, not a regression.
- `WAVE-HL26-010` Baseline observable behavior statement: Refresh script APIs
  expose inline markup and SHA-256 hash values in dev mode and empty values in
  non-dev mode. Current observable behavior statement: Refresh script APIs
  expose inline markup and SHA-256 hash values in dev mode and empty values in
  non-dev mode. Determination statement: Preserved behavior, not a regression.
- `WAVE-HL26-011` Baseline observable behavior statement: Framework integration
  setter APIs mutate framework-owned config fields. Current observable behavior
  statement: Framework integration setter APIs mutate framework-owned config
  fields with targeted direct assignments. Determination statement: Preserved
  behavior, not a regression.
- `WAVE-HL26-012` Baseline observable behavior statement: Config accessor APIs
  return canonical dist/static/config/prefix/vite metadata derived from parsed
  config state. Current observable behavior statement: Config accessor APIs
  return canonical dist/static/config/prefix/vite metadata derived from parsed
  config state. Determination statement: Preserved behavior, not a regression.

Confirmed Regressions

- None recorded yet.

Comparison Method

- Evaluate each normative requirement by observable behavior.
- Record only true behavior regressions.
- Keep this document temporary and disposable before merge.

Regression Entry Format

- Requirement ID
- Baseline observable behavior statement
- Current observable behavior statement
- Regression statement
- Reproduction conditions
