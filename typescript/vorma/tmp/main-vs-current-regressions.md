Main vs Current Vorma Browser Client Regression Report

Scope

- Compare baseline `main` behavior against current branch behavior using
  requirement IDs from `typescript/vorma/spec/client-normative-spec.md`.
- API shape changes alone are not regressions.
- A regression is recorded only when the same user story is no longer satisfied
  after appropriate app migration.
- Current behavior that fixes a clear baseline defect is not a regression.

Status

- No requirement-level regressions are currently confirmed.
- Spec-ID coverage check is complete: 142/142 normative requirement IDs are
  mapped with no missing or extra IDs.
- Full-item sniff-test review across `Validated Non-Regressions` entries is
  complete, with no regressions currently confirmed.

Validated Non-Regressions

- `VRM-HL01-001` Baseline observable behavior statement: `initClient` executes
  bootstrap in stable order: HMR setup, beforeunload scroll persistence,
  runtime/global initialization, history initialization, initial
  component/loader bootstrap, initial render, refresh-scroll restore, and touch
  detection. Current observable behavior statement: `initClient` executes the
  same bootstrap lifecycle in stable order. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL01-002` Baseline observable behavior statement: Initial client module
  map is built from SSR route metadata (`matchedPatterns`, `importURLs`,
  `exportKeys`, `errorExportKeys`) during init. Current observable behavior
  statement: Initial client module map is built from SSR route metadata during
  init via shared route-metadata merge helper. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL01-003` Baseline observable behavior statement: Pattern registry is
  initialized from app-configured dynamic rune, splat rune, and explicit index
  segment values. Current observable behavior statement: Pattern registry is
  initialized from app-configured dynamic rune, splat rune, and explicit index
  segment values. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL01-004` Baseline observable behavior statement: Progressive route
  manifest completion is accepted without request-generation identity checks or
  manifest payload value validation. Current observable behavior statement:
  Progressive manifest completion is accepted only when load identity and
  registry identity still match, and manifest payload values are validated as
  `0|1` flags before registration. Determination statement: This change is a
  baseline-defect correction, not a regression, because stale or malformed
  progressive manifest completion does not mutate live matcher/runtime state.
- `VRM-HL01-005` Baseline observable behavior statement: Hard-reload marker
  query param is removed via history replace after history initialization.
  Current observable behavior statement: Hard-reload marker query param is
  removed via history replace after history initialization. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL01-006` Baseline observable behavior statement: Init applies provided
  default error boundary when supplied, otherwise uses framework default.
  Current observable behavior statement: Init applies provided default error
  boundary when supplied, otherwise uses framework default. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL01-007` Baseline observable behavior statement: View-transition global
  flag is enabled only when init options request it. Current observable behavior
  statement: View-transition global flag is set to true only when explicitly
  requested and otherwise remains false. Determination statement: This is
  preserved opt-in behavior with explicit false-state assignment, not a
  regression.
- `VRM-HL02-001` Baseline observable behavior statement: Client runtime state is
  accessed through one symbol-keyed global store accessor surface. Current
  observable behavior statement: Client runtime state is accessed through one
  symbol-keyed global store accessor surface. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL02-002` Baseline observable behavior statement: Router-data accessor
  returns build ID, matched patterns, params, splat values, and root data with
  null root fallback when root loader is absent. Current observable behavior
  statement: Router-data accessor returns build ID, matched patterns, params,
  splat values, and root data with null root fallback when root loader is
  absent. Determination statement: This is preserved behavior, not a regression.
- `VRM-HL02-003` Baseline observable behavior statement: Adapter state sync
  reads individual render-state fields from shared global state. Current
  observable behavior statement: Adapter runtime can read one render-state
  projection accessor that returns loader/client-loader/error/component/import/
  export state snapshot. Determination statement: This change is a baseline
  defect correction, not a regression, because adapter synchronization contracts
  are centralized and consistent.
- `VRM-HL02-004` Baseline observable behavior statement: Client-loader
  registration appends per-pattern wait functions without clearing prior
  registrations. Current observable behavior statement: Client-loader
  registration merges per-pattern wait functions without clearing prior
  registrations. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL02-005` Baseline observable behavior statement: Redirect/history
  subsystems access navigation runtime via direct global manager references.
  Current observable behavior statement: Redirect/history subsystems access
  navigation runtime through initialized navigation-state bridge accessor that
  throws if not set. Determination statement: This change is a baseline-defect
  correction, not a regression, because misordered initialization fails loudly.
- `VRM-HL06-004` Baseline observable behavior statement: Skip eligibility treats
  reordered query parameter entry lists as unchanged by sorting query entries
  before comparison. Current observable behavior statement: Skip eligibility
  compares serialized `search` strings directly, so any serialized query change
  blocks skip. Determination statement: This change is a baseline-defect
  correction, not a regression, because skip policy does not assume applications
  treat query entry ordering as semantically irrelevant for data freshness.
- `VRM-HL24-005` Baseline observable behavior statement: Adapter client-loader
  pattern registration failures are caught and logged while wait-function
  registration still proceeds, leaving runtime behavior partially configured.
  Current observable behavior statement: Adapter registration fails explicitly
  when pattern-registration preconditions are missing, and registration does not
  continue in a silently degraded state. Determination statement: This change is
  a baseline-defect correction, not a regression, because explicit failure
  preserves deterministic runtime correctness.
- `VRM-HL03-001` Baseline observable behavior statement: Only `userNavigation`
  proactively aborts competing in-flight work before choosing the winner target;
  `browserHistory`, `redirect`, and `action` create new work without the same
  cross-lane supersession pass. Current observable behavior statement: All
  active-intent navigation types arbitrate active, prefetch, and revalidation
  lanes first and abort non-matching entries before reuse/create. Determination
  statement: This change is a baseline-defect correction, not a regression,
  because user-intent navigations converge deterministically on the selected
  destination.
- `VRM-HL03-002` Baseline observable behavior statement: Same-target active
  reuse is guaranteed for `userNavigation`, while other active-intent types can
  start duplicate same-target operations. Current observable behavior statement:
  Same-target active entries are reused for all active-intent navigation types.
  Determination statement: This change is a baseline-defect correction, not a
  regression, because duplicate same-target active work is removed.
- `VRM-HL03-003` Baseline observable behavior statement: Matching-prefetch
  upgrade semantics are guaranteed for `userNavigation`. Current observable
  behavior statement: Matching-prefetch promotion to active is applied for all
  active-intent navigation types. Determination statement: This change is a
  baseline-defect correction, not a regression, because existing prefetch work
  is consistently upgraded instead of duplicated.
- `VRM-HL03-004` Baseline observable behavior statement: Same-target
  revalidation work can be reused but is not promoted into an explicit active
  navigation-intent lane with updated navigation options. Current observable
  behavior statement: Same-target revalidation entries are promoted/reused as
  active navigation entries with active-intent metadata. Determination
  statement: This change is a baseline-defect correction, not a regression,
  because same-target destination work is unified under one active owner.
- `VRM-HL03-005` Baseline observable behavior statement: Prefetch for the
  current document short-circuits and does not persist prefetch state. Current
  observable behavior statement: Prefetch for the current document returns an
  immediately aborted control and does not persist prefetch state. Determination
  statement: This change is a changed implementation with preserved
  user-observable no-op semantics, not a regression.
- `VRM-HL03-006` Baseline observable behavior statement: Prefetch reuse/dedupe
  is keyed by exact target URL key matches. Current observable behavior
  statement: Prefetch reuse/dedupe and promotion use navigation-target
  equivalence across active, prefetch, and revalidation lanes. Determination
  statement: This change is a baseline-defect correction, not a regression,
  because duplicate work for equivalent data targets is prevented.
- `VRM-HL03-007` Baseline observable behavior statement: Revalidation always
  resolves against the current location URL. Current observable behavior
  statement: Revalidation target resolution remains pinned to current location
  URL. Determination statement: This is preserved behavior, not a regression.
- `VRM-HL03-008` Baseline observable behavior statement: Late navigation
  outcomes are not gated by operation ownership identity and can target replaced
  entries. Current observable behavior statement: Navigation outcome and
  lifecycle mutation paths are gated by operation ownership checks.
  Determination statement: This change is a baseline-defect correction, not a
  regression, because stale completions do not mutate newer ownership state.
- `VRM-HL04-001` Baseline observable behavior statement: Revalidation coalescing
  is window-based and does not enforce a deterministic one-in-flight plus
  one-trailing-slot policy. Current observable behavior statement: Revalidation
  lane enforces at most one in-flight pass plus at most one shared trailing
  pass. Determination statement: This change is a baseline-defect correction,
  not a regression, because burst revalidation behavior becomes deterministic.
- `VRM-HL04-002` Baseline observable behavior statement: Immediate reuse is
  governed by a fixed elapsed-time window from pass start. Current observable
  behavior statement: Immediate reuse is governed by the
  pre-trailing-eligibility phase of the current in-flight pass. Determination
  statement: This change is a baseline-defect correction, not a regression,
  because early callers reliably reuse in-flight work without time-window
  brittleness.
- `VRM-HL04-003` Baseline observable behavior statement: Late revalidation
  requests can trigger repeated abort/restart behavior instead of a single
  queued follow-up pass. Current observable behavior statement: Late requests
  share one queued trailing pass. Determination statement: This change is a
  baseline-defect correction, not a regression, because freshness follow-up work
  is coalesced to one pass.
- `VRM-HL04-004` Baseline observable behavior statement: In-flight revalidation
  target mismatch handling depends on caller replacement timing and does not
  centrally enforce mismatch abort. Current observable behavior statement:
  In-flight target mismatch is detected and stale pass replacement is enforced
  for current-location alignment. Determination statement: This change is a
  baseline-defect correction, not a regression, because revalidation remains
  bound to the current page.
- `VRM-HL04-005` Baseline observable behavior statement: No explicit queued
  trailing slot exists to clear when non-revalidation navigation starts. Current
  observable behavior statement: Non-revalidation navigation clears queued
  trailing revalidation requests before single-pass execution. Determination
  statement: This change is a deterministic queue-hygiene extension, not a
  regression.
- `VRM-HL05-001` Baseline observable behavior statement: Non-`revalidation` and
  non-`action` fetch starts evaluate client-only skip eligibility before
  creating the server route-data request. Current observable behavior statement:
  Non-`revalidation` and non-`action` fetch starts evaluate skip eligibility and
  short-circuit server fetch when client-only outcome is available.
  Determination statement: This is preserved behavior, not a regression.
- `VRM-HL05-002` Baseline observable behavior statement: Route-data request URL
  always includes `vorma_json=<buildID>`. Current observable behavior statement:
  Route-data request URL construction always includes
  `vorma_json=<current_buildID>`. Determination statement: This is preserved
  behavior, not a regression.
- `VRM-HL05-003` Baseline observable behavior statement: Revalidation route-data
  requests attach deployment identifier query key when deployment ID is
  available. Current observable behavior statement: Revalidation route-data
  requests attach deployment identifier query key when deployment ID is
  available. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL05-004` Baseline observable behavior statement: Redirect-capable fetch
  request init injects `X-Accepts-Client-Redirect: 1` irrespective of method.
  Current observable behavior statement: Redirect request-init builder injects
  `X-Accepts-Client-Redirect: 1` irrespective of method. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL05-005` Baseline observable behavior statement: Missing/redirected
  responses resolve to aborted outcome, and non-success response states fail the
  navigation path instead of constructing success output. Current observable
  behavior statement: Missing/redirected responses resolve to aborted outcome,
  `should` redirects resolve to redirect outcome, and other non-success states
  throw failure before success construction. Determination statement: This is a
  changed implementation with preserved fail-fast semantics, not a regression.
- `VRM-HL05-006` Baseline observable behavior statement: Partial client matches
  start eligible client loaders before server completion, with
  server-data-coupled promises and abort signal propagation. Current observable
  behavior statement: Partial client matches start eligible client loaders
  before server completion, with server-data-coupled promises and abort signal
  propagation. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL05-007` Baseline observable behavior statement: Successful server
  outcomes trigger dependency and CSS preload attempts on the success path.
  Current observable behavior statement: Successful server outcomes generate
  deterministic preload commands for dependencies and CSS, and preload command
  generation is skipped when signal is already aborted. Determination statement:
  This change is a baseline-defect correction, not a regression, because stale
  aborted paths avoid unnecessary preload work.
- `VRM-HL05-008` Baseline observable behavior statement: Development mode uses
  deduped route-module import URLs for module preloading, while production mode
  uses dependency metadata. Current observable behavior statement: Development
  mode uses deduped route-module import URLs for preload planning, while
  production mode uses dependency metadata. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL06-001` Baseline observable behavior statement: Skip eligibility is
  denied unless both route manifest and pattern registry are initialized.
  Current observable behavior statement: Skip eligibility context construction
  is denied unless both route manifest and pattern registry are initialized.
  Determination statement: This is preserved behavior, not a regression.
- `VRM-HL06-002` Baseline observable behavior statement: Skip eligibility fails
  when any currently matched server-loader pattern is absent from the target
  match set. Current observable behavior statement: Skip eligibility fails when
  any currently matched server-loader pattern is absent from the target match
  set. Determination statement: This is preserved behavior, not a regression.
- `VRM-HL06-003` Baseline observable behavior statement: Skip eligibility fails
  when the target match introduces a newly matched client-loader pattern.
  Current observable behavior statement: Skip eligibility fails when the target
  match introduces a newly matched client-loader pattern. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL06-005` Baseline observable behavior statement: Outermost
  loader-bearing dynamic-param and splat changes fail skip eligibility. Current
  observable behavior statement: Outermost loader-bearing dynamic-param and
  splat changes fail skip eligibility. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL06-006` Baseline observable behavior statement: Skip assembly enforces
  module-map presence and server-loader pattern presence in current matches, but
  can still pass through undefined current loader payloads for server-loader
  entries. Current observable behavior statement: Skip assembly enforces
  module-map presence and rejects skip when required current server-loader data
  is undefined. Determination statement: This change is a baseline-defect
  correction, not a regression, because skip path does not commit incomplete
  server-loader data sets.
- `VRM-HL06-007` Baseline observable behavior statement: Passing skip
  eligibility builds synthetic JSON/Response and wait-function promise for
  normal navigation processing. Current observable behavior statement: Passing
  skip eligibility builds synthetic success navigation outcome with
  JSON/Response and wait-function promise for normal navigation processing.
  Determination statement: This is a changed implementation with preserved
  lifecycle semantics, not a regression.
- `VRM-HL06-008` Baseline observable behavior statement: Skip synthetic
  completion seeds running loader map with existing resolved client-loader
  values for matched patterns. Current observable behavior statement: Skip
  synthetic completion seeds running loader map with existing resolved
  client-loader values for matched patterns. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL07-001` Baseline observable behavior statement: Redirect parsing
  priority is `X-Vorma-Reload`, then browser redirect target, then
  `X-Client-Redirect`. Current observable behavior statement: Redirect parsing
  priority is `X-Vorma-Reload`, then browser redirect target, then
  `X-Client-Redirect`. Determination statement: This is preserved behavior, not
  a regression.
- `VRM-HL07-002` Baseline observable behavior statement: Redirect candidates are
  validated as HTTP(S), and non-HTTP targets are ignored. Current observable
  behavior statement: Redirect candidates are validated as HTTP(S), and non-HTTP
  targets are ignored. Determination statement: This is preserved behavior, not
  a regression.
- `VRM-HL07-003` Baseline observable behavior statement: Redirect strategy
  defaults to soft for internal targets and hard for external targets, with
  `X-Vorma-Reload` forcing hard strategy. Current observable behavior statement:
  Redirect strategy defaults to soft for internal targets and hard for external
  targets, with `X-Vorma-Reload` forcing hard strategy. Determination statement:
  This is preserved behavior, not a regression.
- `VRM-HL07-004` Baseline observable behavior statement: Redirect effectuation
  aborts/removes redirect and revalidation navigations before hard/soft terminal
  effectuation. Current observable behavior statement: Redirect effectuation
  command plans perform redirect/revalidation cleanup before hard/soft terminal
  effectuation. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL07-005` Baseline observable behavior statement: Internal hard redirect
  appends `vorma_reload=<latestBuildID>` and assigns `window.location.href`.
  Current observable behavior statement: Internal hard redirect appends
  `vorma_reload=<latestBuildID>` and assigns `window.location.href`.
  Determination statement: This is preserved behavior, not a regression.
- `VRM-HL07-006` Baseline observable behavior statement: Soft redirect issues
  redirect navigation with incremented redirect count and propagated
  state/replace/scroll options. Current observable behavior statement: Soft
  redirect issues redirect navigation with incremented redirect count and
  propagated state/replace/scroll options, and only returns `did` redirect data
  when downstream navigation reports `didNavigate`. Determination statement:
  This change is a baseline-defect correction, not a regression, because
  redirect lifecycle reports `did` only when downstream navigation succeeds.
- `VRM-HL07-007` Baseline observable behavior statement: Redirect request flow
  checks max redirect threshold before issuing fetch and returns no usable
  response when limit is reached. Current observable behavior statement:
  Redirect request flow checks max redirect threshold before issuing fetch and
  returns no usable response when limit is reached. Determination statement:
  This is preserved behavior, not a regression.
- `VRM-HL08-001` Baseline observable behavior statement: Successful processing
  performs partial currentness checks, but lacks operation-ownership-gated
  checkpoint arbitration across all lifecycle stages. Current observable
  behavior statement: Successful processing executes
  `pre_waiting`/`post_waiting`/`post_asset`/`cleanup` checkpoints with
  operation-ownership-gated stop/delete behavior. Determination statement: This
  change is a baseline-defect correction, not a regression, because stale
  successful outcomes do not commit over newer ownership.
- `VRM-HL08-002` Baseline observable behavior statement: Idle prefetch success
  transitions to complete and exits without render commit. Current observable
  behavior statement: Idle prefetch post-asset checkpoint completes without
  render commit. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL08-003` Baseline observable behavior statement: Revalidation success is
  blocked when current location diverges from revalidation origin, preventing
  stale commit. Current observable behavior statement: Revalidation success is
  blocked at pre-waiting and post-asset checkpoints when current data target
  diverges from revalidation origin. Determination statement: This is preserved
  behavior with strengthened stale-target checks, not a regression.
- `VRM-HL08-004` Baseline observable behavior statement: Successful navigations
  transition through waiting/rendering/complete phases when commit path remains
  active. Current observable behavior statement: Successful navigations
  transition through waiting/rendering/complete phases when checkpoint policy
  allows commit. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL08-005` Baseline observable behavior statement: Client loader state is
  committed before render, but can still commit after entry ownership becomes
  stale during waits. Current observable behavior statement: Client loader state
  commit is post-asset checkpoint-gated and skipped when post-asset plan stops.
  Determination statement: This change is a baseline-defect correction, not a
  regression, because stale entries do not commit loader state.
- `VRM-HL08-006` Baseline observable behavior statement: Build ID sync occurs in
  one timing location regardless of idle-prefetch vs navigational intent.
  Current observable behavior statement: Build ID sync timing is intent-aware:
  idle prefetch syncs before asset wait, while navigational intents sync after
  asset wait only when flow remains valid. Determination statement: This change
  is a baseline-defect correction, not a regression, because build transitions
  remain synchronized without stale commit risk.
- `VRM-HL08-007` Baseline observable behavior statement: Response artifacts are
  build-ID-gated but can be applied before later stale-stop checks. Current
  observable behavior statement: Response artifacts are build-ID-gated and
  post-asset checkpoint-gated to non-stopped flows. Determination statement:
  This change is a baseline-defect correction, not a regression, because stale
  flows do not apply response artifacts.
- `VRM-HL08-008` Baseline observable behavior statement: Render path does not
  enforce explicit ownership commit guards at module-load checkpoints. Current
  observable behavior statement: Render path passes `shouldCommit` ownership
  guard and aborts commit when guard denies at render checkpoints. Determination
  statement: This change is a baseline-defect correction, not a regression,
  because stale render commits are denied.
- `VRM-HL08-009` Baseline observable behavior statement: Successful navigation
  cleanup runs in `finally`, including interruption paths. Current observable
  behavior statement: Successful navigation cleanup checkpoint runs in
  `finally`, including interruption paths. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL09-001` Baseline observable behavior statement: Build-ID events are
  emitted only when `newID` is non-empty and differs from `oldID`, but client
  build state is not synchronously updated in the same path. Current observable
  behavior statement: Build-ID sync mutates client build state and dispatches
  `vorma:build-id` only when `newID` is non-empty and differs from current build
  state. Determination statement: This change is a baseline-defect correction,
  not a regression, because build listeners and runtime build state remain
  aligned.
- `VRM-HL09-002` Baseline observable behavior statement: Redirect metadata
  carries latest build ID, but redirect handling does not proactively sync
  client build state before redirect effectuation continuation. Current
  observable behavior statement: `status=should` redirect metadata synchronizes
  build state before redirect effectuation continuation. Determination
  statement: This change is a baseline-defect correction, not a regression,
  because redirect-driven build transitions are visible to downstream runtime
  stages.
- `VRM-HL09-003` Baseline observable behavior statement: Route-data fetches are
  build-scoped and submit requests include deployment header when available.
  Current observable behavior statement: Route-data fetches are build-scoped and
  submit requests include deployment header when available. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL10-001` Baseline observable behavior statement: Client component
  modules are loaded before executing client loaders. Current observable
  behavior statement: Client component modules are loaded before executing
  client loaders. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL10-002` Baseline observable behavior statement: Client loader execution
  skips the branch at `outermostServerErrorIdx`. Current observable behavior
  statement: Client loader execution skips the branch at
  `outermostServerErrorIdx`. Determination statement: This is preserved
  behavior, not a regression.
- `VRM-HL10-003` Baseline observable behavior statement: Non-abort client loader
  rejection aborts descendant loader controllers. Current observable behavior
  statement: Non-abort client loader rejection aborts descendant loader
  controllers. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL10-004` Baseline observable behavior statement: Settled loader-result
  processing captures first non-abort failure, truncates data at failure
  boundary, and does not commit later outputs. Current observable behavior
  statement: Settled loader-result processing captures first non-abort failure,
  truncates data at failure boundary, and does not commit later outputs.
  Determination statement: This is preserved behavior, not a regression.
- `VRM-HL10-005` Baseline observable behavior statement: Effective error
  boundary index derives as minimum of server/client outermost error indices
  when both are present. Current observable behavior statement: Effective error
  boundary index derives as minimum of server/client outermost error indices
  when both are present. Determination statement: This is preserved behavior,
  not a regression.
- `VRM-HL10-006` Baseline observable behavior statement: Client-loader pattern
  registration assumes matcher registry availability and can fail without a
  dedicated initialization guard message. Current observable behavior statement:
  Client-loader pattern registration validates matcher registry initialization
  and throws explicit initialization error when absent. Determination statement:
  This change is a baseline-defect correction, not a regression, because
  misordered setup fails deterministically.
- `VRM-HL10-007` Baseline observable behavior statement: Partial matching checks
  full path first, then progressively shorter parent paths, and returns the
  first longest partial match. Current observable behavior statement: Partial
  matching checks full path first, then progressively shorter parent paths, and
  returns the first longest partial match. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL11-001` Baseline observable behavior statement: Render commit applies
  route state, error derivation, active components/error boundary, history and
  scroll, title, CSS, route-change dispatch, head updates, then finish in a
  stable sequence. Current observable behavior statement: Render commit executes
  the same lifecycle effects via explicit command sequence with deterministic
  ordering and terminal finish command. Determination statement: This is
  preserved behavior with a stricter command runtime, not a regression.
- `VRM-HL11-002` Baseline observable behavior statement: User-navigation and
  redirect commits push history when target differs and `replace` is false;
  otherwise replace is used. Current observable behavior statement:
  User-navigation and redirect commits push history when target differs and
  `replace` is false; otherwise replace is used. Determination statement: This
  is preserved behavior, not a regression.
- `VRM-HL11-003` Baseline observable behavior statement: Navigation scroll state
  dispatch uses hash anchor when present, otherwise top reset unless
  `scrollToTop === false`. Current observable behavior statement: Navigation
  scroll state dispatch uses hash anchor when present, otherwise top reset
  unless `scrollToTop === false`. Determination statement: This is preserved
  behavior, not a regression.
- `VRM-HL11-004` Baseline observable behavior statement: `browserHistory` commit
  restores provided scroll state or falls back to hash target semantics. Current
  observable behavior statement: `browserHistory` commit restores provided
  scroll state or falls back to hash target semantics. Determination statement:
  This is preserved behavior, not a regression.
- `VRM-HL11-005` Baseline observable behavior statement: Title updates decode
  HTML entities and skip redundant assignments when value is unchanged. Current
  observable behavior statement: Title updates decode HTML entities and skip
  redundant assignments when value is unchanged. Determination statement: This
  is preserved behavior, not a regression.
- `VRM-HL11-006` Baseline observable behavior statement: Managed head sections
  update only for explicitly defined `metaHeadEls`/`restHeadEls`; undefined
  blocks preserve current managed section state. Current observable behavior
  statement: Managed head sections update only for explicitly defined
  `metaHeadEls`/`restHeadEls`; undefined blocks preserve current managed section
  state. Determination statement: This is preserved behavior, not a regression.
- `VRM-HL12-001` Baseline observable behavior statement: View transitions are
  used only when global view-transition flag is enabled and
  `document.startViewTransition` is available. Current observable behavior
  statement: View transitions are used only when global view-transition flag is
  enabled and `document.startViewTransition` is available. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL12-002` Baseline observable behavior statement: `prefetch` and
  `revalidation` render paths bypass view-transition wrapper. Current observable
  behavior statement: `prefetch` and `revalidation` render paths bypass
  view-transition wrapper. Determination statement: This is preserved behavior,
  not a regression.
- `VRM-HL13-001` Baseline observable behavior statement: Link interception
  requires resolvable eligible internal anchor details before preventDefault and
  navigation begin. Current observable behavior statement: Link interception
  requires resolvable eligible internal anchor details before preventDefault and
  navigation begin. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL13-002` Baseline observable behavior statement: Eligible internal
  same-document no-op link clicks are intercepted by the client click handler,
  browser-default full-document navigation is prevented, and SPA navigation
  state does not hard-reload the page. Current observable behavior statement:
  Same-document no-op classification short-circuits client navigation/callback
  work while still preventing default browser navigation. Determination
  statement: This change is a baseline-defect correction, not a regression,
  because redundant no-op lifecycle work is removed without allowing hard
  reload.
- `VRM-HL13-003` Baseline observable behavior statement: Hash-change internal
  clicks save scroll state and bypass navigation interception. Current
  observable behavior statement: Hash-change internal clicks save scroll state
  and bypass navigation interception. Determination statement: This is preserved
  behavior, not a regression.
- `VRM-HL13-004` Baseline observable behavior statement: Link render callbacks
  are not operation-ownership-gated at callback checkpoints. Current observable
  behavior statement: Link render callbacks are operation-ownership-gated so
  only the winning entry runs `beforeRender`/`afterRender`. Determination
  statement: This change is a baseline-defect correction, not a regression,
  because stale click callbacks do not run for superseded entries.
- `VRM-HL13-005` Baseline observable behavior statement: Redirect click callback
  handling does not require redirect effectuation to resolve as `did` before
  `afterRender`. Current observable behavior statement: Redirect click callback
  handling runs `beforeRender`, effectuates redirect, and runs `afterRender`
  only when redirect result resolves as `did`. Determination statement: This
  change is a baseline-defect correction, not a regression, because redirect
  callback lifecycle is outcome-aligned.
- `VRM-HL13-006` Baseline observable behavior statement: Link click rejection
  cleanup/logging relies on broader navigation cleanup paths rather than
  ownership-gated link-handler catch handling. Current observable behavior
  statement: Link click rejection path performs ownership-gated target cleanup
  and explicit link-navigation failure logging. Determination statement: This
  change is a baseline-defect correction, not a regression, because failed click
  navigations do not leave stale owned entries.
- `VRM-HL14-001` Baseline observable behavior statement: Prefetch handler
  factory returns handlers only for HTTP internal targets with usable relative
  URL. Current observable behavior statement: Prefetch handler factory returns
  handlers only for HTTP internal targets with usable relative URL.
  Determination statement: This is preserved behavior, not a regression.
- `VRM-HL14-002` Baseline observable behavior statement: Prefetch start uses
  delayed timer (default 100ms) and avoids duplicate timer scheduling. Current
  observable behavior statement: Prefetch start uses delayed timer (default
  100ms) and avoids duplicate timer scheduling. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL14-003` Baseline observable behavior statement: Prefetch stop clears
  pending timer and aborts/removes idle prefetch based on exact target lookup.
  Current observable behavior statement: Prefetch stop clears pending timer and
  aborts/removes idle prefetch using data-target-equivalent matching
  (hash-insensitive and resilient to target variants). Determination statement:
  This change is a baseline-defect correction, not a regression, because stop
  reliably cancels matching idle prefetch work.
- `VRM-HL14-004` Baseline observable behavior statement: Touch-device
  pointer-leave path does not stop prefetch, while blur/touch-cancel still stop.
  Current observable behavior statement: Touch-device pointer-leave path does
  not stop prefetch, while blur/touch-cancel still stop. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL14-005` Baseline observable behavior statement: Prefetch click clears
  pending timer, invokes `vormaNavigate` with link options, and uses navigation
  arbitration to reuse/upgrade matching prefetch work. Current observable
  behavior statement: Prefetch click clears pending timer, invokes
  `vormaNavigate` with link options, and uses navigation arbitration to
  reuse/upgrade matching prefetch work. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL14-006` Baseline observable behavior statement: Prefetch click callback
  order is `beforeBegin` (only when prefetch not started), then `beforeRender`,
  navigation, then `afterRender`. Current observable behavior statement:
  Prefetch click callback order is `beforeBegin` (only when prefetch not
  started), then `beforeRender`, navigation, then `afterRender`. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL14-007` Baseline observable behavior statement: Prefetch click path
  intercepts eligible internal clicks and prevents browser-default full-document
  navigation before dispatching client navigation behavior. Current observable
  behavior statement: Prefetch click path short-circuits same-document no-op
  targets without navigation dispatch while still preventing default browser
  navigation. Determination statement: This change is a baseline-defect
  correction, not a regression, because redundant no-op prefetch-click
  navigation work is removed without allowing hard reload.
- `VRM-HL15-001` Baseline observable behavior statement: History listener update
  handling runs per listener callback without serialized promise-tail queueing.
  Current observable behavior statement: History listener update handling is
  serialized through a promise-tail queue that processes updates in order.
  Determination statement: This change is a baseline-defect correction, not a
  regression, because POP/location side effects are deterministic under bursts.
- `VRM-HL15-002` Baseline observable behavior statement: Last-known location
  writes are not sequence-token-gated against newer in-flight history updates.
  Current observable behavior statement: Last-known location writes are gated to
  newest successful history update sequence. Determination statement: This
  change is a baseline-defect correction, not a regression, because stale
  history listener completions do not roll back location state.
- `VRM-HL15-003` Baseline observable behavior statement: Same-data-target POP
  hash add/update/removal handling applies local scroll behavior without
  cross-document navigation. Current observable behavior statement:
  Same-data-target POP hash add/update/removal handling applies local scroll
  behavior without cross-document navigation. Determination statement: This is
  preserved behavior with normalized-hash/data-target matching improvements, not
  a regression.
- `VRM-HL15-004` Baseline observable behavior statement: Cross-document POP uses
  `browserHistory` navigation with destination-keyed stored scroll state.
  Current observable behavior statement: Cross-document POP uses
  `browserHistory` navigation with destination-keyed stored scroll state.
  Determination statement: This is preserved behavior, not a regression.
- `VRM-HL15-005` Baseline observable behavior statement: Failed cross-document
  POP triggers hard reload attempt after failed client navigation. Current
  observable behavior statement: Failed cross-document POP triggers hard reload
  attempt after failed client navigation, with jsdom environment bypass.
  Determination statement: This is preserved failure-recovery behavior with test
  environment safeguard, not a regression.
- `VRM-HL16-001` Baseline observable behavior statement: Scroll snapshots are
  saved under last-known history location key. Current observable behavior
  statement: Scroll snapshots are saved under last-known history location key.
  Determination statement: This is preserved behavior, not a regression.
- `VRM-HL16-002` Baseline observable behavior statement: Scroll snapshot map is
  bounded and evicts oldest entry when cap is exceeded. Current observable
  behavior statement: Scroll snapshot map is bounded and evicts oldest entry
  when cap is exceeded. Determination statement: This is preserved behavior, not
  a regression.
- `VRM-HL16-003` Baseline observable behavior statement: Page-refresh restore
  applies only for recent same-location snapshots and executes in
  `requestAnimationFrame`. Current observable behavior statement: Page-refresh
  restore applies only for recent same-document-location snapshots, with shape
  validation and `requestAnimationFrame` application. Determination statement:
  This is preserved behavior with stronger snapshot validation, not a
  regression.
- `VRM-HL16-004` Baseline observable behavior statement: Hash scroll apply uses
  raw fragment values for element lookup. Current observable behavior statement:
  Hash scroll apply normalizes/decodes fragments before element lookup and
  `scrollIntoView`. Determination statement: This change is a baseline-defect
  correction, not a regression, because encoded anchor fragments resolve
  correctly.
- `VRM-HL16-005` Baseline observable behavior statement: Session-storage
  operations can throw through runtime paths. Current observable behavior
  statement: Session-storage read/write/remove failures are locally caught and
  ignored. Determination statement: This change is a baseline-defect correction,
  not a regression, because restricted storage contexts do not break navigation
  behavior.
- `VRM-HL17-001` Baseline observable behavior statement: Route-change, status,
  build-id, and location events dispatch under stable keys with typed detail
  contracts and add/remove listener helpers. Current observable behavior
  statement: Route-change, status, build-id, and location events dispatch under
  stable keys with typed detail contracts and add/remove listener helpers.
  Determination statement: This is preserved behavior, not a regression.
- `VRM-HL17-002` Baseline observable behavior statement: Status signaling is
  debounced and duplicate payloads are suppressed by deep-equality comparison.
  Current observable behavior statement: Status signaling is debounced and
  duplicate payloads are suppressed by deep-equality comparison. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL17-003` Baseline observable behavior statement: Status dimensions use
  active navigate intent and phase completion for navigation/revalidation, and
  exclude `skipGlobalLoadingIndicator` submissions from `isSubmitting`. Current
  observable behavior statement: Status dimensions use active navigate intent
  and phase completion for navigation/revalidation, and exclude
  `skipGlobalLoadingIndicator` submissions from `isSubmitting`. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL17-004` Baseline observable behavior statement: Global loading
  indicator setup supports include-domain filtering, start/stop debounce, and
  cleanup that removes listener/clears timers/stops running indicator. Current
  observable behavior statement: Global loading indicator setup supports
  include-domain filtering, start/stop debounce, and cleanup that removes
  listener/clears timers/stops running indicator. Determination statement: This
  is preserved behavior, not a regression.
- `VRM-HL18-001` Baseline observable behavior statement: Submissions with the
  same dedupe key abort the existing in-flight submission before replacing it.
  Current observable behavior statement: Submissions with the same dedupe key
  abort the existing in-flight submission, emit dedupe transition causation, and
  replace it with the new current submission. Determination statement: This is
  preserved behavior with explicit lifecycle causation metadata, not a
  regression.
- `VRM-HL18-002` Baseline observable behavior statement: Submission status
  updates are scheduled, but explicit `none -> submitting` and
  `submitting -> removed` transition emission is not modeled as first-class
  lifecycle commands. Current observable behavior statement: Submission begin
  emits `none -> submitting`, and finish emits `submitting -> removed` when
  current ownership still holds. Determination statement: This change is a
  baseline-defect correction, not a regression, because submission lifecycle
  observation is explicit and ownership-safe.
- `VRM-HL18-003` Baseline observable behavior statement: Submission flow does
  not run explicit ownership-gated staleness checkpoints across request,
  classification, redirect, and success-return stages. Current observable
  behavior statement: Submission flow runs ownership-gated staleness checkpoints
  (`post_request`, `pre_finalize`, `post_response_classification`,
  `post_redirect_effectuation`, `pre_success_return`, `post_auto_revalidate`)
  and returns aborted when stale. Determination statement: This change is a
  baseline-defect correction, not a regression, because stale deduped submission
  outcomes do not commit.
- `VRM-HL18-004` Baseline observable behavior statement: Submit response
  classification is deterministic: non-OK responses error, `should` redirect
  effectuates redirect, otherwise success path parses response body. Current
  observable behavior statement: Submit response classification remains
  deterministic with the same branch semantics, executed within staleness-gated
  runtime checkpoints. Determination statement: This is preserved behavior, not
  a regression.
- `VRM-HL18-005` Baseline observable behavior statement: Successful non-GET,
  non-redirect mutation submits auto-revalidate unless `revalidate` option is
  false. Current observable behavior statement: Successful non-GET, non-redirect
  mutation submits auto-revalidate unless `revalidate` option is false, with
  post-auto-revalidate staleness guard before final return. Determination
  statement: This is preserved behavior with stronger staleness safety, not a
  regression.
- `VRM-HL18-006` Baseline observable behavior statement: Successful non-redirect
  submit responses are parsed as JSON only. Current observable behavior
  statement: Successful non-redirect submit responses support no-content as
  `undefined`, JSON content as JSON parse, and non-JSON content as text fallback
  (empty text => `undefined`). Determination statement: This change is a
  baseline-defect correction, not a regression, because submit response parsing
  supports standard success payload forms.
- `VRM-HL19-001` Baseline observable behavior statement: Focus-triggered
  revalidation runs only when runtime status is idle and stale-time window has
  elapsed since last navigation/revalidation trigger timestamp. Current
  observable behavior statement: Focus-triggered revalidation runs only when
  runtime status is idle and stale-time window has elapsed since last
  navigation/revalidation trigger timestamp. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL19-002` Baseline observable behavior statement: Focus listener utility
  debounces callbacks and gates visibilitychange callback on
  `document.visibilityState === "visible"`, with cleanup removing both
  listeners. Current observable behavior statement: Focus listener utility
  debounces callbacks and gates visibilitychange callback on
  `document.visibilityState === "visible"`, with cleanup removing both
  listeners. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL20-001` Baseline observable behavior statement: Head updates use marker
  comments, but marker validation does not require shared-parent and ordered
  boundary verification before mutation. Current observable behavior statement:
  Head updates mutate only when managed start/end marker boundaries are valid,
  shared-parented, and properly ordered. Determination statement: This change is
  a baseline-defect correction, not a regression, because invalid marker
  structure no-ops instead of risking unmanaged mutation.
- `VRM-HL20-002` Baseline observable behavior statement: Fingerprint-equivalent
  duplicate head blocks are deduped with last occurrence retained. Current
  observable behavior statement: Fingerprint-equivalent duplicate head blocks
  are deduped with last occurrence retained. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL20-003` Baseline observable behavior statement: Head reconciliation
  reuses matching existing elements, removes stale unmatched nodes, inserts new
  unmatched nodes, and preserves deterministic managed-order output. Current
  observable behavior statement: Head reconciliation reuses matching existing
  elements, removes stale unmatched nodes, inserts new unmatched nodes, and
  preserves deterministic managed-order output. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL20-004` Baseline observable behavior statement: Null/undefined
  `attributesKnownSafe` values trigger explicit panic/error path during head
  element creation. Current observable behavior statement: Null/undefined
  `attributesKnownSafe` values trigger explicit panic/error path during head
  element creation. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL21-001` Baseline observable behavior statement: Module preload requests
  dedupe on resolved public href before inserting `modulepreload` link. Current
  observable behavior statement: Module preload requests dedupe on resolved
  public href before inserting `modulepreload` link. Determination statement:
  This is preserved behavior, not a regression.
- `VRM-HL21-002` Baseline observable behavior statement: CSS preload requests
  dedupe on resolved href and return awaitable promise tied to inserted preload
  link load/error events. Current observable behavior statement: CSS preload
  requests dedupe on resolved href and return awaitable promise tied to inserted
  preload link load/error events. Determination statement: This is preserved
  behavior, not a regression.
- `VRM-HL21-003` Baseline observable behavior statement: CSS bundle application
  dedupes by `data-vorma-css-bundle` marker and executes in
  `requestAnimationFrame`. Current observable behavior statement: CSS bundle
  application dedupes by `data-vorma-css-bundle` marker and executes in
  `requestAnimationFrame`. Determination statement: This is preserved behavior,
  not a regression.
- `VRM-HL21-004` Baseline observable behavior statement: Public href resolver
  supports dev URL/public-prefix fallback, but stylesheet-apply path uses direct
  public-prefix concatenation instead of shared resolver behavior. Current
  observable behavior statement: Module preload, CSS preload, and stylesheet
  apply paths use shared public-href resolver with dev URL/public-prefix
  fallback and slash normalization. Determination statement: This change is a
  baseline-defect correction, not a regression, because asset URL resolution is
  consistent across dev and built runtime paths.
- `VRM-HL22-001` Baseline observable behavior statement: Pattern registration
  normalizes dynamic segments to `:param`, splat segments to `*`, and explicit
  index syntax before registry insertion. Current observable behavior statement:
  Pattern registration normalizes dynamic segments to `:param`, splat segments
  to `*`, and explicit index syntax before registry insertion. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL22-002` Baseline observable behavior statement: Nested route matching
  returns parent-to-leaf lineage ordering with conflict pruning rules. Current
  observable behavior statement: Nested route matching returns parent-to-leaf
  lineage ordering with conflict pruning rules. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL22-003` Baseline observable behavior statement: Catch-all matches are
  removed when stronger specific matches exist under matcher conflict rules.
  Current observable behavior statement: Catch-all matches are removed when
  stronger specific matches exist under matcher conflict rules. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL22-004` Baseline observable behavior statement: Final match validity
  checks reject structurally invalid dynamic/splat depth combinations before
  returning match bundle. Current observable behavior statement: Final match
  validity checks reject structurally invalid dynamic/splat depth combinations
  before returning match bundle. Determination statement: This is preserved
  behavior, not a regression.
- `VRM-HL23-001` Baseline observable behavior statement: Navigation target
  equality is often exact-href keyed in runtime maps, with only limited
  hash-insensitive equivalence handling in specific paths. Current observable
  behavior statement: Navigation target equality and lane matching consistently
  use hash-insensitive data-target comparison with exact-href short-circuit.
  Determination statement: This change is a baseline-defect correction, not a
  regression, because hash-only differences do not duplicate route-data
  navigation work.
- `VRM-HL23-002` Baseline observable behavior statement: Hash-only same-document
  change detection exists, but same-document no-op is not modeled as a distinct
  classification and fragment normalization is not decode-aware across all
  paths. Current observable behavior statement: Same-document hash-change and
  same-document-noop are distinct classifications using decode-aware normalized
  hash comparison. Determination statement: This change is a baseline-defect
  correction, not a regression, because local-document interactions follow
  deterministic separate paths.
- `VRM-HL23-003` Baseline observable behavior statement: Anchor interception
  eligibility excludes modified/new-tab/download/non-anchor interactions before
  preventDefault. Current observable behavior statement: Anchor interception
  eligibility excludes modified/new-tab/download/non-anchor interactions before
  preventDefault. Determination statement: This is preserved behavior, not a
  regression.
- `VRM-HL23-004` Baseline observable behavior statement: Href classification
  distinguishes HTTP internal/external targets and rejects non-HTTP/parse-failed
  targets from internal-routing paths. Current observable behavior statement:
  Href classification distinguishes HTTP internal/external targets and rejects
  non-HTTP/parse-failed targets from internal-routing paths. Determination
  statement: This is preserved behavior, not a regression.
- `VRM-HL24-001` Baseline observable behavior statement: Root outlet
  initialization guards listener setup to one registration pass. Current
  observable behavior statement: Root outlet initialization guards listener
  setup to one registration pass. Determination statement: This is preserved
  behavior, not a regression.
- `VRM-HL24-002` Baseline observable behavior statement: Route-change listener
  path synchronizes adapter navigation state, and scroll application runs in
  `requestAnimationFrame` from route-change payload state. Current observable
  behavior statement: Route-change listener path synchronizes adapter navigation
  state, and scroll application runs in `requestAnimationFrame` from
  route-change payload state. Determination statement: This is preserved
  behavior, not a regression.
- `VRM-HL24-003` Baseline observable behavior statement: Outlet branch
  selection/error/fallback behavior is implemented per adapter with equivalent
  intent but duplicated logic surfaces. Current observable behavior statement:
  Outlet branch selection/error/fallback behavior uses shared branch-state
  runtime contract across adapters. Determination statement: This change is a
  baseline-defect correction, not a regression, because cross-adapter branch
  semantics are centralized and consistent.
- `VRM-HL24-004` Baseline observable behavior statement: Typed loader/query
  hooks read route data by outlet index or matched-pattern index and return
  undefined for unmatched patterns. Current observable behavior statement: Typed
  loader/query hooks read route data by outlet index or matched-pattern index
  and return undefined for unmatched patterns. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL24-006` Baseline observable behavior statement: Adapter link components
  wire shared prefetch start/stop and click lifecycle contract through common
  final-link-props helper. Current observable behavior statement: Adapter link
  components wire shared prefetch start/stop and click lifecycle contract
  through common final-link-props helper. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL25-001` Baseline observable behavior statement: Development HMR init
  sets window-global dev revalidate hook only in dev mode. Current observable
  behavior statement: Development HMR init sets window-global dev revalidate
  hook only in dev mode. Determination statement: This is preserved behavior,
  not a regression.
- `VRM-HL25-002` Baseline observable behavior statement: HMR JS updates can
  trigger debounced client-loader refresh for tracked module pathname/pattern.
  Current observable behavior statement: HMR JS updates trigger debounced
  client-loader refresh when tracked pathname updates intersect currently
  matched tracked pattern set. Determination statement: This change is a
  baseline-defect correction, not a regression, because relevant multi-pattern
  module updates refresh deterministically.
- `VRM-HL25-003` Baseline observable behavior statement: HMR listener
  de-duplication suppresses repeated registration by pathname. Current
  observable behavior statement: HMR listener de-duplication suppresses repeated
  registration by runtime and pathname, with tracked-pattern set reuse.
  Determination statement: This is preserved behavior with stronger
  de-duplication scope, not a regression.
- `VRM-HL26-001` Baseline observable behavior statement: `vormaNavigate`
  resolves target href to absolute URL with optional search/hash overrides and
  dispatches `userNavigation`. Current observable behavior statement:
  `vormaNavigate` resolves target href to absolute URL with optional search/hash
  overrides and dispatches `userNavigation`. Determination statement: This is
  preserved behavior, not a regression.
- `VRM-HL26-002` Baseline observable behavior statement: `revalidate` dispatches
  `revalidation` navigation for current location href. Current observable
  behavior statement: `revalidate` dispatches `revalidation` navigation for
  current location href. Determination statement: This is preserved behavior,
  not a regression.
- `VRM-HL26-003` Baseline observable behavior statement: `submit` returns
  structured `{success:true,data}` or `{success:false,error}` result contract.
  Current observable behavior statement: `submit` returns structured
  `{success:true,data}` or `{success:false,error}` result contract.
  Determination statement: This is preserved behavior, not a regression.
- `VRM-HL26-004` Baseline observable behavior statement: `getLocation` returns
  pathname/search/hash with current history-state payload. Current observable
  behavior statement: `getLocation` returns pathname/search/hash with current
  history-state payload. Determination statement: This is preserved behavior,
  not a regression.
- `VRM-HL26-005` Baseline observable behavior statement: `getRootEl` returns
  cast root element without explicit missing/type validation. Current observable
  behavior statement: `getRootEl` throws explicit errors for missing root node
  and non-`HTMLDivElement` root shape. Determination statement: This change is a
  baseline-defect correction, not a regression, because root contract violations
  fail loudly and early.
- `VRM-HL26-006` Baseline observable behavior statement: Runtime exports status
  getter and event listener APIs (`status`, `location`, `route-change`,
  `build-id`) with cleanup callbacks. Current observable behavior statement:
  Runtime exports status getter and event listener APIs (`status`, `location`,
  `route-change`, `build-id`) with cleanup callbacks. Determination statement:
  This is preserved behavior, not a regression.
- `VRM-HL26-007` Baseline observable behavior statement: Runtime does not expose
  bounded navigation transition journal APIs for deterministic introspection.
  Current observable behavior statement: Runtime state machine maintains bounded
  navigation debug journal entries with explicit get/clear APIs. Determination
  statement: This change is a baseline-defect correction, not a regression,
  because runtime diagnostics support deterministic lifecycle inspection.

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
