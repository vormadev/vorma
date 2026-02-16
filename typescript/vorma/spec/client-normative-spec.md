Vorma Browser Client Normative Specification

Purpose

- Define authoritative, user-observable browser-client behavior for Vorma.
- Preserve semantics across implementation refactors and UI adapter changes.

Guiding Principle

- Navigation, prefetching, loader execution, and revalidation perform only the
  minimum work required to keep route data non-stale.

Scope

- In scope: browser-shipped Vorma TypeScript runtime and adapter contracts.
- Out of scope: `create`, `vite`, and server-only runtime behavior.

Correctness Precedence

- Baseline implementation behavior informs intent discovery but does not
  override correctness.
- Clearly incorrect or footgun baseline behavior is excluded from normative
  requirements.

Normative Record Format

- Every requirement contains:
    - user story
    - trigger conditions
    - expected observable behavior
    - protected user interest
    - cancellation/ordering/race constraints

Section Coverage

- This specification defines requirements for:
    - `HL-01` Client bootstrap and initialization lifecycle
    - `HL-02` Global client state model and access surfaces
    - `HL-03` Navigation arbitration and lane ownership
    - `HL-04` Deterministic revalidation lane policy
    - `HL-05` Route-data fetch pipeline
    - `HL-06` Client-only skip/fetch-elision policy
    - `HL-07` Redirect detection and effectuation
    - `HL-08` Successful navigation lifecycle checkpoints
    - `HL-09` Build ID synchronization and artifact gating
    - `HL-10` Client loaders execution model
    - `HL-11` Render commit pipeline
    - `HL-12` View transition policy
    - `HL-13` Link click lifecycle
    - `HL-14` Prefetch intent lifecycle
    - `HL-15` History integration behavior
    - `HL-16` Scroll state persistence and restoration
    - `HL-17` Event model and status signaling
    - `HL-18` Submission lifecycle and staleness control
    - `HL-19` Revalidation-on-focus policy
    - `HL-20` Head element managed-region reconciliation
    - `HL-21` Asset preload and stylesheet application
    - `HL-22` Route matching and registration semantics
    - `HL-23` URL and anchor classification semantics
    - `HL-24` Adapter-generic runtime contracts
    - `HL-25` Development-time HMR behavior
    - `HL-26` Public API behavior surface

Coverage Traceability

- Requirement-to-test traceability is defined in:
  `typescript/vorma/spec/client-normative-test-coverage.md`.

Requirements

## HL-03 Navigation Arbitration and Lane Ownership

### `VRM-HL03-001`

- User Story: A user-initiated navigation replaces competing in-flight work so
  the interface converges on the chosen destination.
- Trigger Conditions: A `userNavigation`, `browserHistory`, `redirect`, or
  `action` begin-navigation request starts for target `T`.
- Expected Observable Behavior: In-flight active, prefetch, and revalidation
  entries not matching `T` are aborted/superseded.
- Protected User Interest: The page does not continue stale or irrelevant work
  after the user changes destination.
- Cancellation/Ordering/Race Constraints: Supersession happens before selecting
  the winner control for `T`.

### `VRM-HL03-002`

- User Story: Repeated navigation to the same destination avoids duplicate
  network and loader work.
- Trigger Conditions: Active lane already targets the same navigation target.
- Expected Observable Behavior: Existing control is reused instead of creating a
  second navigation entry.
- Protected User Interest: Stable performance and reduced duplicate fetches.
- Cancellation/Ordering/Race Constraints: Reuse preserves single ownership of
  the active lane.

### `VRM-HL03-003`

- User Story: Clicking a prefetched link upgrades the same in-flight prefetch
  rather than restarting navigation.
- Trigger Conditions: A matching idle prefetch exists when an active-intent
  navigation to the same target begins.
- Expected Observable Behavior: Prefetch entry is promoted to active navigation
  and its existing control is reused.
- Protected User Interest: Faster click-to-render transition and less duplicate
  work.
- Cancellation/Ordering/Race Constraints: Promotion occurs before any new active
  control would be allocated.

### `VRM-HL03-004`

- User Story: Revalidation work already aimed at the destination can be reused
  when the user navigates to that same destination.
- Trigger Conditions: Revalidation lane currently targets the same navigation
  target as a new active-intent navigation.
- Expected Observable Behavior: Revalidation entry is promoted/reused as active
  navigation.
- Protected User Interest: Avoid duplicate request chains for equivalent
  destination work.
- Cancellation/Ordering/Race Constraints: Revalidation lane is cleared only for
  the promoted entry transition.

### `VRM-HL03-005`

- User Story: Prefetching the current document is avoided.
- Trigger Conditions: Prefetch target resolves to current data target.
- Expected Observable Behavior: Prefetch returns an immediately aborted/no-op
  control and does not create persistent prefetch state.
- Protected User Interest: No wasted request/load work for no-op targets.
- Cancellation/Ordering/Race Constraints: Immediate-abort resolution occurs
  without active lane mutation.

### `VRM-HL03-006`

- User Story: Multiple prefetch intents for the same target coalesce into one
  operation.
- Trigger Conditions: A prefetch begin request arrives for a target with
  matching prefetch, active, or revalidation work.
- Expected Observable Behavior: Existing control is reused; duplicate prefetch
  entries are not created.
- Protected User Interest: Predictable, low-overhead prefetch behavior.
- Cancellation/Ordering/Race Constraints: Target equivalence uses data-target
  semantics, not strict hash equality.

### `VRM-HL03-007`

- User Story: Revalidation is always defined against the current page.
- Trigger Conditions: A `revalidation` begin request starts.
- Expected Observable Behavior: Revalidation target is resolved from current
  location, not caller-provided alternate target.
- Protected User Interest: Revalidation refreshes the page the user is actually
  on.
- Cancellation/Ordering/Race Constraints: Existing same-target revalidation can
  be reused; prior different revalidation is aborted/replaced.

### `VRM-HL03-008`

- User Story: Stale control handles do not mutate newer navigation state.
- Trigger Conditions: A navigation outcome resolves after ownership has changed.
- Expected Observable Behavior: Outcome processing is ignored when control
  operation ownership does not match current entry ownership.
- Protected User Interest: Late stale completions do not overwrite current UI
  intent.
- Cancellation/Ordering/Race Constraints: Ownership checks gate mutation at
  outcome and lifecycle checkpoints.

## HL-04 Deterministic Revalidation Lane Policy

### `VRM-HL04-001`

- User Story: Revalidation requests are deterministic under bursts.
- Trigger Conditions: Multiple `revalidation` calls occur while one pass is
  already in flight.
- Expected Observable Behavior: At most one in-flight pass exists, with at most
  one queued trailing pass.
- Protected User Interest: Stable refresh behavior without revalidation storms.
- Cancellation/Ordering/Race Constraints: Queue is single-slot trailing, not an
  unbounded backlog.

### `VRM-HL04-002`

- User Story: Revalidation calls arriving immediately after a pass starts reuse
  the same pass.
- Trigger Conditions: Revalidation call occurs before trailing eligibility
  window opens for current in-flight pass.
- Expected Observable Behavior: Caller receives current in-flight pass promise.
- Protected User Interest: Coalesced revalidation work.
- Cancellation/Ordering/Race Constraints: Eligibility transition is microtask
  based.

### `VRM-HL04-003`

- User Story: Revalidation calls that arrive after the coalescing window can
  request one follow-up pass.
- Trigger Conditions: Revalidation call occurs when in-flight pass is trailing
  eligible.
- Expected Observable Behavior: A single trailing pass is queued and shared by
  all late callers.
- Protected User Interest: Freshness is preserved without duplicate follow-up
  passes.
- Cancellation/Ordering/Race Constraints: Multiple late calls resolve through
  one trailing promise.

### `VRM-HL04-004`

- User Story: If location target changes during in-flight revalidation, stale
  work is not allowed to continue.
- Trigger Conditions: In-flight revalidation target no longer matches current
  data target.
- Expected Observable Behavior: Mismatched in-flight revalidation is aborted and
  replaced by a fresh pass for current target.
- Protected User Interest: Refresh work remains aligned with current location.
- Cancellation/Ordering/Race Constraints: Target mismatch check executes before
  deciding reuse/queue.

### `VRM-HL04-005`

- User Story: Non-revalidation navigations clear pending revalidation trailing
  work.
- Trigger Conditions: A non-revalidation navigation begins.
- Expected Observable Behavior: Queued trailing revalidation request is cleared.
- Protected User Interest: User-directed navigation is not followed by stale
  queued revalidation from prior state.
- Cancellation/Ordering/Race Constraints: Clearing queued trailing happens
  before executing single-pass navigation.

## HL-05 Route-Data Fetch Pipeline

### `VRM-HL05-001`

- User Story: Navigation first attempts to avoid server fetch when safe.
- Trigger Conditions: `fetchRouteData` starts for non-`revalidation` and
  non-`action` navigation types.
- Expected Observable Behavior: Client-only skip eligibility is evaluated before
  creating server fetch request.
- Protected User Interest: Reduced latency and unnecessary network traffic.
- Cancellation/Ordering/Race Constraints: Skip outcome short-circuits server
  fetch path.

### `VRM-HL05-002`

- User Story: Route-data requests are version-scoped.
- Trigger Conditions: Server fetch URL is built for route data.
- Expected Observable Behavior: `vorma_json=<buildID>` query parameter is
  included.
- Protected User Interest: Route JSON corresponds to current client build
  expectations.
- Cancellation/Ordering/Race Constraints: Build scoping is applied before fetch.

### `VRM-HL05-003`

- User Story: Revalidation request can be deployment scoped.
- Trigger Conditions: Revalidation request URL is built and deployment ID is
  available.
- Expected Observable Behavior: Deployment identifier query key is attached to
  the request URL.
- Protected User Interest: Revalidation can target the active deployment
  context.
- Cancellation/Ordering/Race Constraints: Deployment scoping is limited to
  revalidation requests.

### `VRM-HL05-004`

- User Story: Client redirect-capable requests consistently advertise redirect
  handling support.
- Trigger Conditions: Redirect-capable fetch request init is built.
- Expected Observable Behavior: Request includes `X-Accepts-Client-Redirect: 1`.
- Protected User Interest: Redirect semantics remain explicit and predictable.
- Cancellation/Ordering/Race Constraints: Header is injected regardless of
  request method.

### `VRM-HL05-005`

- User Story: Non-success server responses fail navigation instead of silently
  producing partial state.
- Trigger Conditions: Server response exists and is not acceptable for success
  outcome.
- Expected Observable Behavior: Fetch pipeline aborts navigation and produces a
  failed/cancelled outcome path.
- Protected User Interest: UI does not commit inconsistent route data.
- Cancellation/Ordering/Race Constraints: Response validation occurs before
  success outcome construction.

### `VRM-HL05-006`

- User Story: Parallel client loader execution begins as early as possible.
- Trigger Conditions: Partial client-side match is available while server
  promise is pending.
- Expected Observable Behavior: Eligible client loaders start with server-data
  promises that resolve/reject when server result arrives.
- Protected User Interest: Reduced overall navigation latency by overlapping
  server and client work.
- Cancellation/Ordering/Race Constraints: Loader promises are bound to
  navigation abort signal.

### `VRM-HL05-007`

- User Story: Successful server outcomes preload route dependencies
  opportunistically.
- Trigger Conditions: Server success outcome is built and signal is not aborted.
- Expected Observable Behavior: Preload commands are generated for module
  dependencies and CSS bundles.
- Protected User Interest: Faster post-fetch rendering and reduced visual delay.
- Cancellation/Ordering/Race Constraints: Preload command plan is empty when
  signal is already aborted.

### `VRM-HL05-008`

- User Story: Dev and production environments preload different module sets
  while preserving behavior goals.
- Trigger Conditions: Server success preload execution plan is derived.
- Expected Observable Behavior: Development mode preloads deduped route module
  import URLs; production mode uses dependency metadata.
- Protected User Interest: Equivalent user-facing readiness with environment
  appropriate preload sources.
- Cancellation/Ordering/Race Constraints: Environment choice does not alter
  navigation correctness.

## HL-06 Client-Only Skip/Fetch-Elision Policy

### `VRM-HL06-001`

- User Story: Skip mode only runs when required client metadata exists.
- Trigger Conditions: Skip eligibility evaluation starts.
- Expected Observable Behavior: Skip is denied unless both route manifest and
  pattern registry are initialized.
- Protected User Interest: Skip path never runs on incomplete matching data.
- Cancellation/Ordering/Race Constraints: Missing prerequisites force server
  fetch path.

### `VRM-HL06-002`

- User Story: Skip is blocked if target route graph removes an existing server
  loader branch.
- Trigger Conditions: Current matched server-loader pattern is absent from
  target match set.
- Expected Observable Behavior: Skip eligibility fails.
- Protected User Interest: Required server loader transitions are not skipped.
- Cancellation/Ordering/Race Constraints: Removal check precedes synthetic
  success construction.

### `VRM-HL06-003`

- User Story: Skip is blocked when target introduces a newly matched client
  loader.
- Trigger Conditions: Target match contains client loader pattern not currently
  matched.
- Expected Observable Behavior: Skip eligibility fails.
- Protected User Interest: New client loader dependencies are not missed.
- Cancellation/Ordering/Race Constraints: New-client-loader detection is strict
  membership comparison against current matched patterns.

### `VRM-HL06-004`

- User Story: Loader-relevant search-param changes require fetch.
- Trigger Conditions: Search changes and at least one loader-bearing boundary is
  present.
- Expected Observable Behavior: Skip eligibility fails.
- Protected User Interest: URL query changes that can affect data are not served
  stale.
- Cancellation/Ordering/Race Constraints: Search comparison is performed against
  the current and target URL serialized search strings without
  order-normalization so skip policy does not assume query ordering is
  semantically irrelevant for application data.

### `VRM-HL06-005`

- User Story: Outermost loader-bound param/splat changes require fetch.
- Trigger Conditions: Outermost loader-bearing match has changed dynamic params
  or splat values.
- Expected Observable Behavior: Skip eligibility fails.
- Protected User Interest: Loader inputs remain aligned with URL path params.
- Cancellation/Ordering/Race Constraints: Outermost loader index is derived from
  deepest matched loader-bearing route.

### `VRM-HL06-006`

- User Story: Skip requires complete module-map and loader-data availability for
  all matched patterns.
- Trigger Conditions: Skip result is assembled from match context.
- Expected Observable Behavior: Skip eligibility fails when required module or
  server-loader data is unavailable.
- Protected User Interest: Skip path never commits incomplete route module/data
  sets.
- Cancellation/Ordering/Race Constraints: Any missing required item terminates
  skip construction.

### `VRM-HL06-007`

- User Story: Valid skip produces a full synthetic success outcome compatible
  with normal navigation processing.
- Trigger Conditions: Skip eligibility passes.
- Expected Observable Behavior: Synthetic JSON/Response and wait function
  promise are created and processed as a success navigation outcome.
- Protected User Interest: Skip path preserves equivalent render/load lifecycle
  semantics.
- Cancellation/Ordering/Race Constraints: Synthetic outcome still obeys
  navigation abort signal.

### `VRM-HL06-008`

- User Story: Existing resolved client-loader data is reused during skip when
  valid for matched patterns.
- Trigger Conditions: Skip success outcome is constructed and prior client
  loader outputs exist for same patterns.
- Expected Observable Behavior: Running loader map seeds from existing client
  loader results for matching patterns.
- Protected User Interest: Avoid recomputing already valid client loader data.
- Cancellation/Ordering/Race Constraints: Reuse is pattern-index aligned.

## HL-07 Redirect Detection and Effectuation

### `VRM-HL07-001`

- User Story: Redirect interpretation order is deterministic.
- Trigger Conditions: A response is parsed for redirect data.
- Expected Observable Behavior: Redirect parsing priority is `X-Vorma-Reload`,
  then browser redirect target, then `X-Client-Redirect`.
- Protected User Interest: Stable redirect behavior across equivalent responses.
- Cancellation/Ordering/Race Constraints: First matching redirect source wins.

### `VRM-HL07-002`

- User Story: Only HTTP(S) redirect targets participate in client redirect
  behavior.
- Trigger Conditions: Redirect target candidate is extracted.
- Expected Observable Behavior: Non-HTTP targets are ignored as redirect data.
- Protected User Interest: Invalid/non-web redirect targets do not corrupt
  navigation flow.
- Cancellation/Ordering/Race Constraints: Target validation occurs before
  strategy assignment.

### `VRM-HL07-003`

- User Story: Internal and external redirects choose distinct strategy classes.
- Trigger Conditions: Redirect status is `should` and strategy is not explicitly
  forced by header semantics.
- Expected Observable Behavior: Internal targets default to soft redirects;
  external targets default to hard redirects.
- Protected User Interest: Internal app transitions stay SPA-capable while
  external navigation uses browser navigation.
- Cancellation/Ordering/Race Constraints: `X-Vorma-Reload` always forces hard
  strategy.

### `VRM-HL07-004`

- User Story: Redirect effectuation starts from a clean redirect/revalidation
  state.
- Trigger Conditions: Redirect effectuation command plan runs for hard/soft
  strategy.
- Expected Observable Behavior: Existing redirect and revalidation navigations
  are aborted and removed before effectuation.
- Protected User Interest: Loading state and redirect sequencing avoid stuck or
  conflicting in-flight states.
- Cancellation/Ordering/Race Constraints: Cleanup command executes before
  terminal effectuation command.

### `VRM-HL07-005`

- User Story: Internal hard redirects trigger full reload with build-aware
  marker.
- Trigger Conditions: Hard redirect target is internal HTTP location.
- Expected Observable Behavior: Browser location is set to target URL with
  `vorma_reload=<latestBuildID>` query marker.
- Protected User Interest: Build mismatches can force full document recovery.
- Cancellation/Ordering/Race Constraints: Marker insertion occurs before setting
  `window.location.href`.

### `VRM-HL07-006`

- User Story: Soft redirect preserves navigation options when chaining.
- Trigger Conditions: Soft redirect effectuation executes.
- Expected Observable Behavior: Redirect navigation is issued with incremented
  redirect count and propagated state/replace/scroll options.
- Protected User Interest: Redirected user flow preserves expected history and
  scroll semantics.
- Cancellation/Ordering/Race Constraints: Soft redirect succeeds only when
  downstream navigation reports `didNavigate`.

### `VRM-HL07-007`

- User Story: Redirect loops terminate predictably.
- Trigger Conditions: Redirect count reaches configured maximum.
- Expected Observable Behavior: Redirect request flow stops and reports no
  usable response for continued redirect chain.
- Protected User Interest: Navigation does not enter unbounded redirect loops.
- Cancellation/Ordering/Race Constraints: Max-redirect guard is checked before
  issuing fetch.

## HL-08 Successful Navigation Lifecycle Checkpoints

### `VRM-HL08-001`

- User Story: Success outcomes are committed only for current owned entries.
- Trigger Conditions: Successful outcome processing starts.
- Expected Observable Behavior: Lifecycle checkpoints stop when entry is no
  longer current/owned.
- Protected User Interest: Stale outcomes do not overwrite newer navigations.
- Cancellation/Ordering/Race Constraints: Current-entry checks run at multiple
  checkpoints (`pre_waiting`, `post_waiting`, `post_asset`, `cleanup`).

### `VRM-HL08-002`

- User Story: Idle prefetch completes without render commit.
- Trigger Conditions: Successful entry is idle prefetch (`type=prefetch`,
  `intent=none`).
- Expected Observable Behavior: Navigation reaches complete/cleanup behavior
  without route render commit.
- Protected User Interest: Prefetch warms resources without changing visible UI.
- Cancellation/Ordering/Race Constraints: Cleanup skips deleting non-current
  idle prefetch when defined by cleanup policy.

### `VRM-HL08-003`

- User Story: Stale revalidation does not commit stale state.
- Trigger Conditions: Revalidation entry origin no longer matches current data
  target.
- Expected Observable Behavior: Lifecycle stops and navigation entry is removed
  before commit.
- Protected User Interest: Revalidation cannot apply data for pages user already
  left.
- Cancellation/Ordering/Race Constraints: Stale check runs before waiting and
  after asset wait.

### `VRM-HL08-004`

- User Story: Phase transitions reflect observable lifecycle progress.
- Trigger Conditions: Successful navigation moves through lifecycle checkpoints.
- Expected Observable Behavior: Current entry transitions through
  `fetching -> waiting -> rendering -> complete` where applicable.
- Protected User Interest: Status indicators and lifecycle listeners reflect
  real progress.
- Cancellation/Ordering/Race Constraints: Phase mutations are gated by current
  ownership.

### `VRM-HL08-005`

- User Story: Client loader result state commits before render when commit path
  remains valid.
- Trigger Conditions: Asset wait completed and post-asset plan does not stop.
- Expected Observable Behavior: Client loader state and derived error state are
  committed before render branch execution.
- Protected User Interest: Rendered view reflects synchronized server+client
  loader data.
- Cancellation/Ordering/Race Constraints: Commit is skipped when post-asset plan
  stops.

### `VRM-HL08-006`

- User Story: Build ID synchronization timing depends on navigation intent.
- Trigger Conditions: Successful entry determines build-ID sync timing.
- Expected Observable Behavior: Idle prefetch syncs build ID before asset wait;
  navigational intents sync after asset wait when still valid.
- Protected User Interest: Build transitions are visible without committing
  stale render work.
- Cancellation/Ordering/Race Constraints: Post-asset build sync only occurs for
  non-stopped flows.

### `VRM-HL08-007`

- User Story: Response artifacts are gated to the expected pre-navigation build.
- Trigger Conditions: Post-asset side effects execute.
- Expected Observable Behavior: Client module map merge and response CSS
  application occur only when response build ID equals expected build ID.
- Protected User Interest: Cross-build assets/modules are not applied to
  incompatible runtime state.
- Cancellation/Ordering/Race Constraints: Artifact application runs after wait
  and only in non-stopped post-asset plan.

### `VRM-HL08-008`

- User Story: Render commit can be rejected if entry becomes stale during module
  load boundary.
- Trigger Conditions: Render pipeline reaches pre-module-load or
  post-module-load checkpoint with `shouldCommit` guard.
- Expected Observable Behavior: Commit stops when guard denies commit.
- Protected User Interest: DOM/history/head mutations are not applied from stale
  entries.
- Cancellation/Ordering/Race Constraints: Guard is checked at both checkpoints.

### `VRM-HL08-009`

- User Story: Cleanup always executes regardless of success-path interruption.
- Trigger Conditions: Successful navigation lifecycle exits, including error or
  early-stop paths.
- Expected Observable Behavior: Cleanup checkpoint runs in `finally`.
- Protected User Interest: Navigation lane state does not leak after aborted or
  interrupted lifecycle.
- Cancellation/Ordering/Race Constraints: Cleanup policy decides delete/skip by
  currentness and idle-prefetch status.

## HL-13 Link Click Lifecycle

### `VRM-HL13-001`

- User Story: Only eligible internal anchor interactions are intercepted.
- Trigger Conditions: Link click handler receives an event.
- Expected Observable Behavior: Interception requires a resolvable anchor,
  eligible default prevention conditions, and internal target classification.
- Protected User Interest: Native browser behaviors for
  external/new-tab/modified clicks remain intact.
- Cancellation/Ordering/Race Constraints: Eligibility is validated before
  preventDefault and navigation begin.

### `VRM-HL13-002`

- User Story: Same-document no-op clicks avoid unnecessary work without causing
  browser full-document reload.
- Trigger Conditions: Eligible internal click is classified as same-document
  no-op target.
- Expected Observable Behavior: Browser-default navigation is prevented; no
  client navigation dispatch or additional lifecycle callbacks are executed.
- Protected User Interest: No redundant network/render lifecycle for no-op
  targets and no unexpected hard reload for same-page links.
- Cancellation/Ordering/Race Constraints: Classification short-circuits the
  handler after default-prevention handling is established.

### `VRM-HL13-003`

- User Story: Hash-only same-document clicks preserve scroll restore continuity.
- Trigger Conditions: Eligible internal click is classified as hash-change.
- Expected Observable Behavior: Scroll state is saved and navigation
  interception does not proceed.
- Protected User Interest: Back/forward scroll behavior remains correct across
  hash jumps.
- Cancellation/Ordering/Race Constraints: Hash path exits before user navigation
  begin.

### `VRM-HL13-004`

- User Story: Render callbacks run only for the winning navigation entry.
- Trigger Conditions: Link navigation control resolves outcome.
- Expected Observable Behavior: `beforeRender`/`afterRender` callbacks execute
  only when current entry still belongs to the initiating control.
- Protected User Interest: Stale click callbacks do not run for superseded
  navigations.
- Cancellation/Ordering/Race Constraints: Ownership re-check occurs after
  `beforeRender` and before success commit.

### `VRM-HL13-005`

- User Story: Redirect outcomes from link clicks follow redirect semantics and
  callback order.
- Trigger Conditions: Link click outcome type is redirect.
- Expected Observable Behavior: `beforeRender` runs, redirect effectuation runs,
  and `afterRender` runs only if redirect completes as `did`.
- Protected User Interest: Redirected click experience remains coherent with
  callback lifecycle hooks.
- Cancellation/Ordering/Race Constraints: Owned-entry check gates redirect
  callback execution.

### `VRM-HL13-006`

- User Story: Failed click-navigation attempts do not leave stuck navigation
  state.
- Trigger Conditions: Link navigation control promise rejects.
- Expected Observable Behavior: Owned target entry is removed and failure is
  logged.
- Protected User Interest: Stuck loading/navigation status is avoided after
  failure.
- Cancellation/Ordering/Race Constraints: Removal is ownership-gated.

## HL-14 Prefetch Intent Lifecycle

### `VRM-HL14-001`

- User Story: Prefetch handlers exist only for valid internal HTTP links.
- Trigger Conditions: Prefetch handler factory receives link props.
- Expected Observable Behavior: Handler is created only for HTTP internal target
  with usable relative URL.
- Protected User Interest: No prefetch work is attached to unsupported targets.
- Cancellation/Ordering/Race Constraints: Target validation occurs at handler
  creation.

### `VRM-HL14-002`

- User Story: Prefetch start is delayed and deduplicated.
- Trigger Conditions: Prefetch start event is received.
- Expected Observable Behavior: Timer-based prefetch starts after configured
  delay (default 100ms) and does not enqueue duplicate timers.
- Protected User Interest: Avoid prefetch churn on transient hover/focus.
- Cancellation/Ordering/Race Constraints: Existing pending timer blocks new
  timer creation.

### `VRM-HL14-003`

- User Story: Stopping prefetch cancels pending or idle prefetch work.
- Trigger Conditions: Prefetch stop is invoked (pointer leave, blur, touch
  cancel, explicit stop).
- Expected Observable Behavior: Pending timer clears; matching idle prefetch
  navigation is aborted and removed.
- Protected User Interest: Unwanted prefetch work ends promptly when user intent
  changes.
- Cancellation/Ordering/Race Constraints: Matching uses data-target equivalence
  (hash-insensitive).

### `VRM-HL14-004`

- User Story: Touch interaction avoids false prefetch cancellation on pointer
  leave quirks.
- Trigger Conditions: Pointer leave occurs on detected touch device.
- Expected Observable Behavior: Pointer-leave path does not stop prefetch for
  touch devices.
- Protected User Interest: Tap-driven prefetch/navigation flow is not
  prematurely cancelled.
- Cancellation/Ordering/Race Constraints: Blur and touch-cancel still stop.

### `VRM-HL14-005`

- User Story: Clicking a prefetched link upgrades or reuses existing prefetch
  work rather than duplicating it.
- Trigger Conditions: Prefetch handler click path executes for navigable target.
- Expected Observable Behavior: `vormaNavigate` is invoked with link options and
  begin-navigation arbitration upgrades matching prefetch when present.
- Protected User Interest: Fast click activation with no duplicate request
  chains.
- Cancellation/Ordering/Race Constraints: Pending prefetch timer is cleared
  before click navigation dispatch.

### `VRM-HL14-006`

- User Story: Prefetch click callbacks preserve deterministic ordering.
- Trigger Conditions: Prefetch-click navigation path executes.
- Expected Observable Behavior: `beforeBegin` runs only when prefetch was not
  yet started, then `beforeRender`, navigation, then `afterRender`.
- Protected User Interest: Hook semantics remain predictable for instrumented
  links.
- Cancellation/Ordering/Race Constraints: Timer is cleared before callback
  chain.

### `VRM-HL14-007`

- User Story: Prefetch click short-circuits no-op and hash-change targets.
- Trigger Conditions: Click classification resolves to same-document no-op or
  hash-change.
- Expected Observable Behavior: Browser-default navigation is prevented before
  short-circuiting no-op/hash paths; no navigation dispatch occurs; hash-change
  path saves scroll state.
- Protected User Interest: No redundant route-data work for local-document-only
  target changes and no unexpected hard reload on same-page prefetch clicks.
- Cancellation/Ordering/Race Constraints: Short-circuit happens before callback
  navigation chain and after default-prevention handling is established.

## HL-01 Client Bootstrap and Initialization Lifecycle

### `VRM-HL01-001`

- User Story: Client bootstrap initializes runtime subsystems in a deterministic
  order.
- Trigger Conditions: `initClient` executes.
- Expected Observable Behavior: HMR setup, beforeunload scroll persistence,
  app/global initialization, history initialization, initial component/loading
  bootstrap, initial render, refresh scroll restoration, and touch detection are
  executed in a stable sequence.
- Protected User Interest: First client render converges without partial runtime
  initialization races.
- Cancellation/Ordering/Race Constraints: Initial component load and client
  loader setup complete before initial render callback.

### `VRM-HL01-002`

- User Story: Initial route module metadata is immediately usable for client
  navigation and skip policy.
- Trigger Conditions: `initClient` initializes from SSR-provided route state.
- Expected Observable Behavior: Client module map is built from initial matched
  patterns/import URLs/export keys/error export keys.
- Protected User Interest: Immediate client routing behavior works without
  waiting for later navigation.
- Cancellation/Ordering/Race Constraints: Initial metadata write precedes
  progressive manifest fetch effects.

### `VRM-HL01-003`

- User Story: Pattern matching behavior uses app-configured route syntax.
- Trigger Conditions: `initClient` initializes pattern registry.
- Expected Observable Behavior: Pattern registry is created with configured
  dynamic rune, splat rune, and explicit index segment values.
- Protected User Interest: Client-side route matching semantics align with app
  route definition semantics.
- Cancellation/Ordering/Race Constraints: Registry is set before client loader
  pattern registration use.

### `VRM-HL01-004`

- User Story: Progressive route manifest loading upgrades skip capability
  without destabilizing active runtime state.
- Trigger Conditions: Route manifest URL is configured at init.
- Expected Observable Behavior: Manifest fetch result is accepted only when
  request identity and registry identity are still current; manifest is
  validated as object with `0|1` values and registered into matcher.
- Protected User Interest: Progressive enhancement does not corrupt
  route-matcher state on stale async completion.
- Cancellation/Ordering/Race Constraints: Stale progressive fetch completions
  are ignored.

### `VRM-HL01-005`

- User Story: Hard-reload query marker is removed after client takes control.
- Trigger Conditions: Current URL contains `vorma_reload` marker during init.
- Expected Observable Behavior: Marker query param is removed with history
  replace, preserving controlled URL semantics.
- Protected User Interest: User-visible URL remains canonical after recovery
  reloads.
- Cancellation/Ordering/Race Constraints: URL cleanup runs after history init.

### `VRM-HL01-006`

- User Story: Default error boundary behavior is deterministic when app does not
  provide one.
- Trigger Conditions: `initClient` options are applied.
- Expected Observable Behavior: Provided default error boundary is used when
  supplied; otherwise framework default error boundary is used.
- Protected User Interest: Error rendering remains predictable.
- Cancellation/Ordering/Race Constraints: Default boundary assignment occurs
  before initial error boundary component handling.

### `VRM-HL01-007`

- User Story: View-transition feature flag is explicit and opt-in.
- Trigger Conditions: `initClient` options are applied.
- Expected Observable Behavior: `useViewTransitions` global state is true only
  when explicitly requested.
- Protected User Interest: Transition behavior does not silently change across
  deployments.
- Cancellation/Ordering/Race Constraints: Flag set precedes route rendering.

## HL-02 Global Client State Model and Access Surfaces

### `VRM-HL02-001`

- User Story: Global client state has one canonical runtime store.
- Trigger Conditions: Runtime reads/writes Vorma client global values.
- Expected Observable Behavior: State access routes through one symbol-keyed
  global store with typed get/set accessors.
- Protected User Interest: Consistent cross-module runtime state.
- Cancellation/Ordering/Race Constraints: Modules read/write shared state
  without duplicate local mirrors.

### `VRM-HL02-002`

- User Story: Router data exposes consistent current-route envelope.
- Trigger Conditions: Router data accessor is invoked.
- Expected Observable Behavior: Router data contains build ID, matched patterns,
  params, splat values, and root data (`null` when root loader data is absent).
- Protected User Interest: Adapter hooks and application code consume stable
  routing data contract.
- Cancellation/Ordering/Race Constraints: Root data derivation depends on
  `hasRootData`.

### `VRM-HL02-003`

- User Story: Adapter render stores can read one stable render-state projection.
- Trigger Conditions: Adapter runtime requests client render state.
- Expected Observable Behavior: Render-state projection includes loaders,
  client-loaders, outermost error state, active components/boundary, import
  URLs, and export keys.
- Protected User Interest: Adapter implementations synchronize on identical core
  runtime data.
- Cancellation/Ordering/Race Constraints: Projection reads from global state at
  subscription processing time.

### `VRM-HL02-004`

- User Story: Client loader registration appends without destroying prior
  registrations.
- Trigger Conditions: Adapter registers a client loader wait function.
- Expected Observable Behavior: Pattern-to-wait-function map is merged with
  existing entries.
- Protected User Interest: Multiple client loader registrations coexist.
- Cancellation/Ordering/Race Constraints: New registration overwrites only same
  pattern key.

### `VRM-HL02-005`

- User Story: Redirect/history subsystems require initialized navigation access.
- Trigger Conditions: Runtime attempts to access navigation state bridge before
  initialization.
- Expected Observable Behavior: Access throws instead of silently operating on
  undefined navigation state.
- Protected User Interest: Misordered runtime initialization fails loudly.
- Cancellation/Ordering/Race Constraints: Navigation access must be set during
  client runtime creation.

## HL-09 Build ID Synchronization and Artifact Gating

### `VRM-HL09-001`

- User Story: Build ID update events only fire on actual build transitions.
- Trigger Conditions: Runtime syncs build ID from response or redirect data.
- Expected Observable Behavior: Build ID state changes and `vorma:build-id`
  dispatch occur only when new build ID is non-empty and different from current.
- Protected User Interest: Build listeners do not receive false-positive events.
- Cancellation/Ordering/Race Constraints: Equality check gates event dispatch.

### `VRM-HL09-002`

- User Story: Redirect metadata can proactively move client build state.
- Trigger Conditions: Redirect data has `status=should` with latest build ID.
- Expected Observable Behavior: Build state updates before redirect effectuation
  continuation.
- Protected User Interest: Redirect-driven build transitions are visible before
  next navigation stage.
- Cancellation/Ordering/Race Constraints: Redirect build sync applies only for
  `should` redirects.

### `VRM-HL09-003`

- User Story: Fetch and submit requests are scoped by current build/deployment
  context.
- Trigger Conditions: Route-data fetch and submit requests are prepared.
- Expected Observable Behavior: Route-data fetches carry current build marker;
  submit requests include deployment header when available.
- Protected User Interest: Data operations are aligned with active build and
  deployment context.
- Cancellation/Ordering/Race Constraints: Context metadata is added before
  network dispatch.

## HL-10 Client Loaders Execution Model

### `VRM-HL10-001`

- User Story: Client loaders run against loaded route modules.
- Trigger Conditions: Client loader execution starts.
- Expected Observable Behavior: Route component modules are loaded before
  executing client loaders.
- Protected User Interest: Loader execution can rely on module availability.
- Cancellation/Ordering/Race Constraints: Module load precedes wait-function
  invocations.

### `VRM-HL10-002`

- User Story: Client loaders for branches with server errors are skipped at the
  outermost server-error boundary.
- Trigger Conditions: Outermost server error index exists for matched branch.
- Expected Observable Behavior: Loader at that index is short-circuited and does
  not execute client wait function.
- Protected User Interest: Client loaders do not run on branches already failed
  by server route error boundaries.
- Cancellation/Ordering/Race Constraints: Skip decision is index-aligned with
  matched patterns.

### `VRM-HL10-003`

- User Story: Loader failure in an ancestor branch cancels descendant loader
  work for the same navigation.
- Trigger Conditions: A client loader rejects with non-abort error.
- Expected Observable Behavior: Abort controllers for subsequent loaders are
  aborted immediately.
- Protected User Interest: Avoid wasted client loader work beneath known-failed
  branch.
- Cancellation/Ordering/Race Constraints: Child abort propagation is ordered by
  matched pattern sequence.

### `VRM-HL10-004`

- User Story: Loader result processing stops at first non-abort failure.
- Trigger Conditions: Settled client loader results include a non-abort
  rejection.
- Expected Observable Behavior: Error message is captured from first failing
  loader, result data truncates at failure boundary, and later loader outputs
  are not committed.
- Protected User Interest: Error state corresponds to highest failing branch.
- Cancellation/Ordering/Race Constraints: Abort rejections do not create
  user-facing error messages.

### `VRM-HL10-005`

- User Story: Effective outermost error prefers the earliest failing boundary
  across server and client loader domains.
- Trigger Conditions: Error state derivation runs.
- Expected Observable Behavior: Effective error index is minimum of outermost
  server and client error indices when both exist.
- Protected User Interest: Rendered error boundary corresponds to topmost
  visible failing branch.
- Cancellation/Ordering/Race Constraints: Error derivation runs after client
  loader state updates.

### `VRM-HL10-006`

- User Story: Client loader pattern registration requires initialized matcher
  state.
- Trigger Conditions: Pattern registration is attempted before pattern registry
  is initialized.
- Expected Observable Behavior: Registration throws explicit initialization
  error.
- Protected User Interest: Misordered client-loader setup is not silently
  ignored.
- Cancellation/Ordering/Race Constraints: Registration path validates registry
  existence first.

### `VRM-HL10-007`

- User Story: Partial route matching supports early parallel loader startup for
  deeper URLs.
- Trigger Conditions: Full-path nested match is absent while potential parent
  matches exist.
- Expected Observable Behavior: Matcher attempts progressively shorter parent
  paths and returns first longest partial match.
- Protected User Interest: Parallel loader startup still happens for parent
  branch loaders when deep path exact registration is unavailable.
- Cancellation/Ordering/Race Constraints: Full-path match has precedence.

## HL-11 Render Commit Pipeline

### `VRM-HL11-001`

- User Story: Render commit applies route runtime state in deterministic order.
- Trigger Conditions: Render commit command sequence executes after module load
  checkpoints allow commit.
- Expected Observable Behavior: Route data state, error state, active
  components, active error boundary, history/scroll, title, CSS, route-change
  event, head elements, and finish callback execute in stable order.
- Protected User Interest: UI updates are coherent and race-resilient.
- Cancellation/Ordering/Race Constraints: Command sequence terminates with
  finish command.

### `VRM-HL11-002`

- User Story: User navigations and redirects produce expected history mutation
  behavior.
- Trigger Conditions: Render commit runs with history options for
  `userNavigation` or `redirect`.
- Expected Observable Behavior: Target URL push occurs when target differs and
  replace is false; otherwise replace occurs.
- Protected User Interest: Browser history stack reflects user navigation
  intent.
- Cancellation/Ordering/Race Constraints: History mutation happens before route
  change event dispatch.

### `VRM-HL11-003`

- User Story: Scroll behavior after navigation follows hash and scroll-to-top
  policy.
- Trigger Conditions: Render commit computes dispatch scroll state for
  `userNavigation` or `redirect`.
- Expected Observable Behavior: Hash targets scroll to anchor; otherwise scroll
  resets to top unless `scrollToTop === false`.
- Protected User Interest: Predictable post-navigation viewport positioning.
- Cancellation/Ordering/Race Constraints: Scroll state is carried through
  route-change event payload.

### `VRM-HL11-004`

- User Story: Browser-history POP restores captured scroll state when available.
- Trigger Conditions: Render commit handles `browserHistory` navigation type.
- Expected Observable Behavior: Restored scroll state uses provided stored state
  or falls back to hash target semantics.
- Protected User Interest: Back/forward traversal restores expected viewport
  context.
- Cancellation/Ordering/Race Constraints: Restoration state is computed during
  history command execution.

### `VRM-HL11-005`

- User Story: Document title updates decode HTML entities and avoid redundant
  writes.
- Trigger Conditions: Route payload includes title block.
- Expected Observable Behavior: Title text is decoded and assigned only when
  value differs from current `document.title`.
- Protected User Interest: Accurate document title with low unnecessary DOM
  churn.
- Cancellation/Ordering/Race Constraints: Title assignment occurs after history
  mutation.

### `VRM-HL11-006`

- User Story: Head element blocks are updated only when route payload provides
  explicit head block values.
- Trigger Conditions: Route payload has `metaHeadEls` and/or `restHeadEls`
  defined.
- Expected Observable Behavior: Managed head sections are updated for defined
  blocks; undefined blocks preserve existing managed section state.
- Protected User Interest: Head state is not unintentionally cleared by omitted
  payload sections.
- Cancellation/Ordering/Race Constraints: Head updates occur after route-change
  event dispatch in commit command order.

## HL-12 View Transition Policy

### `VRM-HL12-001`

- User Story: View transitions are opt-in and capability-aware.
- Trigger Conditions: Render path evaluates view-transition usage.
- Expected Observable Behavior: View transition wraps render only when global
  flag is enabled and `document.startViewTransition` is available.
- Protected User Interest: Transition behavior is explicit and environment-safe.
- Cancellation/Ordering/Race Constraints: Transition wrapper is bypassed when
  capability or flag is absent.

### `VRM-HL12-002`

- User Story: Prefetch and revalidation do not invoke visual view transitions.
- Trigger Conditions: Navigation type is `prefetch` or `revalidation`.
- Expected Observable Behavior: Render path uses non-view-transition execution.
- Protected User Interest: Background freshness work does not trigger
  user-facing transition animation.
- Cancellation/Ordering/Race Constraints: Navigation type gate is evaluated
  before transition wrapper call.

## HL-15 History Integration Behavior

### `VRM-HL15-001`

- User Story: History listener processing is serialized.
- Trigger Conditions: Multiple history updates arrive in rapid succession.
- Expected Observable Behavior: Updates are processed through one promise tail
  queue in order.
- Protected User Interest: POP/location side effects are ordered and
  deterministic.
- Cancellation/Ordering/Race Constraints: Tail queue captures both resolved and
  rejected prior update outcomes.

### `VRM-HL15-002`

- User Story: Last-known location updates only from the newest successful
  update.
- Trigger Conditions: A history update completes after newer updates have
  started.
- Expected Observable Behavior: Last-known location mutates only when update
  sequence is latest and navigation succeeded.
- Protected User Interest: Scroll-key and location state are not rolled back by
  stale listener completions.
- Cancellation/Ordering/Race Constraints: Sequence token guards location write.

### `VRM-HL15-003`

- User Story: POP within same data target handles hash-only transitions locally.
- Trigger Conditions: POP action remains within same data target.
- Expected Observable Behavior: Hash add/update scrolls to hash target; hash
  removal restores stored scroll or defaults to top-left.
- Protected User Interest: Same-document POP behavior matches expected scroll
  UX.
- Cancellation/Ordering/Race Constraints: Same-document hash handling runs
  without route-data navigation.

### `VRM-HL15-004`

- User Story: Cross-document POP performs client route navigation fallback.
- Trigger Conditions: POP action crosses data target boundary.
- Expected Observable Behavior: Runtime navigates with `browserHistory` type and
  stored scroll state keyed by destination history key.
- Protected User Interest: Back/forward document traversal remains SPA-aware.
- Cancellation/Ordering/Race Constraints: Scroll state is captured before
  cross-document POP navigation.

### `VRM-HL15-005`

- User Story: Failed cross-document POP attempts recover by hard reload.
- Trigger Conditions: `browserHistory` navigation reports `didNavigate=false`.
- Expected Observable Behavior: Browser hard reload is attempted for destination
  URL (skipped in jsdom environments).
- Protected User Interest: URL and rendered document do not diverge after failed
  POP navigation.
- Cancellation/Ordering/Race Constraints: Hard reload is attempted only after
  navigation failure.

## HL-16 Scroll State Persistence and Restoration

### `VRM-HL16-001`

- User Story: Scroll snapshots are keyed by history entry.
- Trigger Conditions: Scroll state save occurs during navigation/listener flows.
- Expected Observable Behavior: `x/y` snapshot is stored under last-known
  history location key.
- Protected User Interest: Back/forward restores per-entry viewport position.
- Cancellation/Ordering/Race Constraints: Snapshot key source is history
  last-known location.

### `VRM-HL16-002`

- User Story: Scroll snapshot storage remains bounded.
- Trigger Conditions: Scroll map exceeds configured entry cap.
- Expected Observable Behavior: Oldest stored entry is evicted.
- Protected User Interest: Session storage usage remains bounded.
- Cancellation/Ordering/Race Constraints: Eviction executes on save path.

### `VRM-HL16-003`

- User Story: Page-refresh scroll restore applies only to recent same-location
  snapshots.
- Trigger Conditions: Refresh restore runs during init.
- Expected Observable Behavior: Restore applies only when snapshot URL matches
  current location and age is within freshness window.
- Protected User Interest: Refresh restore does not jump to unrelated or stale
  positions.
- Cancellation/Ordering/Race Constraints: Restore executes in
  `requestAnimationFrame` after validation.

### `VRM-HL16-004`

- User Story: Hash scrolling normalizes encoded fragments.
- Trigger Conditions: Scroll apply handles hash-based state or default hash
  fallback.
- Expected Observable Behavior: Hash fragments are normalized/decoded before
  element lookup and `scrollIntoView`.
- Protected User Interest: Encoded fragment anchors scroll correctly.
- Cancellation/Ordering/Race Constraints: Hash normalization precedes DOM
  lookup.

### `VRM-HL16-005`

- User Story: Storage failures do not break navigation behavior.
- Trigger Conditions: Session-storage operations throw.
- Expected Observable Behavior: Read/write/remove failures are ignored and
  runtime continues.
- Protected User Interest: Navigation remains functional in restricted storage
  contexts.
- Cancellation/Ordering/Race Constraints: Failure handling is local and silent.

## HL-17 Event Model and Status Signaling

### `VRM-HL17-001`

- User Story: Runtime event contracts remain stable and typed.
- Trigger Conditions: Route-change, status, build-id, and location events are
  dispatched.
- Expected Observable Behavior: Custom events are dispatched under stable keys
  with expected detail payload shapes.
- Protected User Interest: Application listeners receive predictable event
  contracts.
- Cancellation/Ordering/Race Constraints: Event helper wrappers maintain
  add/remove symmetry.

### `VRM-HL17-002`

- User Story: Navigation status updates avoid noisy duplicate emission.
- Trigger Conditions: Status-signaling scheduler runs.
- Expected Observable Behavior: Status events emit only when newly computed
  status differs from last dispatched status.
- Protected User Interest: Loading indicators and observers do not thrash on
  duplicate status payloads.
- Cancellation/Ordering/Race Constraints: Status updates are debounced.

### `VRM-HL17-003`

- User Story: Status dimensions distinguish navigation, submission, and
  revalidation activity.
- Trigger Conditions: Status is computed from runtime lanes/submissions.
- Expected Observable Behavior: `isNavigating` depends on active lane navigate
  intent not complete; `isRevalidating` depends on revalidation lane not
  complete; `isSubmitting` excludes submissions with
  `skipGlobalLoadingIndicator=true`.
- Protected User Interest: UI loading semantics reflect meaningful user-facing
  work.
- Cancellation/Ordering/Race Constraints: Status recompute follows
  lane/submission state mutation.

### `VRM-HL17-004`

- User Story: Global loading indicator behavior is configurable and race-safe.
- Trigger Conditions: Global loading indicator setup is enabled.
- Expected Observable Behavior: Start/stop transitions are independently
  debounced and filtered by included status domains.
- Protected User Interest: Loading indicator avoids flicker while remaining
  responsive.
- Cancellation/Ordering/Race Constraints: Cleanup removes listener, clears
  timers, and stops indicator if still running.

## HL-18 Submission Lifecycle and Staleness Control

### `VRM-HL18-001`

- User Story: Submissions with same dedupe key replace older in-flight
  submissions.
- Trigger Conditions: New submission begins with dedupe key that already has
  active submission entry.
- Expected Observable Behavior: Existing submission is aborted and transitioned
  to aborted state; new submission becomes current.
- Protected User Interest: Duplicate form intent does not execute conflicting
  concurrent submissions.
- Cancellation/Ordering/Race Constraints: Deduped abort transition records
  causation by newer submission operation ID.

### `VRM-HL18-002`

- User Story: Submission lifecycle status transitions are explicit.
- Trigger Conditions: Submission begin and finish flows execute.
- Expected Observable Behavior: State transitions emit `none -> submitting` at
  begin and `submitting -> removed` at finish when current.
- Protected User Interest: Submission observers can track lifecycle progress
  reliably.
- Cancellation/Ordering/Race Constraints: Finish removal occurs only for current
  submission ownership.

### `VRM-HL18-003`

- User Story: Submission staleness checkpoints prevent stale submission outcomes
  from committing after dedupe or ownership change.
- Trigger Conditions: Submission runtime crosses defined staleness checkpoints.
- Expected Observable Behavior: Non-current submission ownership at checkpoint
  returns aborted result.
- Protected User Interest: Stale submission completions do not overwrite current
  submission intent.
- Cancellation/Ordering/Race Constraints: Checkpoints include post-request,
  pre-finalize, post-response-classification, post-redirect-effectuation,
  pre-success-return, and post-auto-revalidate.

### `VRM-HL18-004`

- User Story: Submit response classification is deterministic.
- Trigger Conditions: Submit response and redirect data are available.
- Expected Observable Behavior: Non-OK responses return error; `should` redirect
  path effectuates redirect; otherwise response data path reads body payload.
- Protected User Interest: Submit caller receives consistent success/error
  semantics.
- Cancellation/Ordering/Race Constraints: Response classification runs after
  staleness and build sync checkpoints.

### `VRM-HL18-005`

- User Story: Successful mutation submissions can trigger automatic
  revalidation.
- Trigger Conditions: Submit response is successful non-redirect and request
  method is non-GET and `revalidate` option is not false.
- Expected Observable Behavior: Runtime issues revalidation navigation before
  returning submit success result.
- Protected User Interest: Mutation results are followed by freshness update.
- Cancellation/Ordering/Race Constraints: Post-auto-revalidate staleness
  checkpoint guards final return.

### `VRM-HL18-006`

- User Story: Submit response body parsing supports JSON and non-JSON success
  payloads.
- Trigger Conditions: Successful non-redirect submit result is read.
- Expected Observable Behavior: No-content responses yield `undefined`;
  JSON-like content types parse as JSON; other content types fall back to
  response text.
- Protected User Interest: Submit API can consume standard response forms
  without implicit parse failure.
- Cancellation/Ordering/Race Constraints: Parsing runs before pre-success-return
  staleness checkpoint.

## HL-19 Revalidation-on-Focus Policy

### `VRM-HL19-001`

- User Story: Focus-driven revalidation triggers only when runtime is idle and
  stale window has elapsed.
- Trigger Conditions: Focus/visibility listener callback runs.
- Expected Observable Behavior: Revalidation triggers only when not navigating,
  not submitting, not revalidating, and elapsed time since last committed
  navigation/revalidation is at least configured stale time.
- Protected User Interest: Focus events do not create redundant refresh churn.
- Cancellation/Ordering/Race Constraints: Policy evaluation uses current status
  snapshot and trigger timestamp runtime.

### `VRM-HL19-002`

- User Story: Focus listeners are debounced and visibility-aware.
- Trigger Conditions: Window focus and visibilitychange events fire.
- Expected Observable Behavior: Callback invocation is debounced and visibility
  event path only invokes callback when document is visible.
- Protected User Interest: Revalidation trigger frequency remains bounded.
- Cancellation/Ordering/Race Constraints: Listener setup returns cleanup that
  removes both event handlers.

## HL-20 Head Element Managed-Region Reconciliation

### `VRM-HL20-001`

- User Story: Head reconciliation mutates only managed marker-delimited regions.
- Trigger Conditions: Head update executes for `meta` or `rest` section.
- Expected Observable Behavior: Updates occur only between valid start/end
  marker comment pair with shared parent and proper order.
- Protected User Interest: Unmanaged head elements are not unintentionally
  modified.
- Cancellation/Ordering/Race Constraints: Invalid marker structure causes update
  no-op.

### `VRM-HL20-002`

- User Story: Duplicate equivalent head elements collapse deterministically.
- Trigger Conditions: New head blocks are converted to elements.
- Expected Observable Behavior: Fingerprint-equivalent duplicates are deduped
  with last occurrence retained.
- Protected User Interest: Head output avoids duplicate equivalent tags.
- Cancellation/Ordering/Race Constraints: Deduplication runs before
  reconciliation against existing DOM nodes.

### `VRM-HL20-003`

- User Story: Existing equivalent DOM elements are reused where possible.
- Trigger Conditions: New deduped element list is reconciled against current
  managed section nodes.
- Expected Observable Behavior: Matching existing elements are retained and
  reordered as needed; unmatched stale nodes are removed; unmatched new nodes
  are inserted.
- Protected User Interest: Minimal head DOM churn with deterministic final
  order.
- Cancellation/Ordering/Race Constraints: Reconciliation order is fingerprint
  and managed-section sequence based.

### `VRM-HL20-004`

- User Story: Unsafe null/undefined attribute values are rejected.
- Trigger Conditions: Head block provides `attributesKnownSafe` entries.
- Expected Observable Behavior: Null/undefined attribute values trigger explicit
  panic/error path.
- Protected User Interest: Invalid head attribute state is not silently emitted.
- Cancellation/Ordering/Race Constraints: Validation occurs before element
  insertion.

## HL-21 Asset Preload and Stylesheet Application

### `VRM-HL21-001`

- User Story: Module preload links are deduplicated by resolved public href.
- Trigger Conditions: Module preload request is issued.
- Expected Observable Behavior: Existing matching `modulepreload` link prevents
  duplicate insertion.
- Protected User Interest: Avoid duplicate preload tag accumulation.
- Cancellation/Ordering/Race Constraints: Resolved href is normalized through
  public-href resolver.

### `VRM-HL21-002`

- User Story: CSS preload links are deduplicated and awaitable.
- Trigger Conditions: CSS preload request is issued.
- Expected Observable Behavior: Existing matching preload link short-circuits;
  newly inserted preload link resolves/rejects promise on load/error.
- Protected User Interest: CSS readiness can be coordinated without duplicate
  tags.
- Cancellation/Ordering/Race Constraints: Promise resolution is tied to inserted
  preload link events.

### `VRM-HL21-003`

- User Story: Applied CSS bundle stylesheet links are deduplicated by bundle
  marker.
- Trigger Conditions: CSS bundle apply command executes.
- Expected Observable Behavior: Stylesheet link insertion is skipped when a link
  with matching `data-vorma-css-bundle` already exists.
- Protected User Interest: Avoid duplicate stylesheet application.
- Cancellation/Ordering/Race Constraints: CSS apply executes in
  `requestAnimationFrame`.

### `VRM-HL21-004`

- User Story: Public asset href resolution uses runtime dev URL when available
  and public prefix fallback otherwise.
- Trigger Conditions: Public href is resolved for module/CSS assets.
- Expected Observable Behavior: Base path is selected from dev URL or public
  prefix with normalized slash behavior.
- Protected User Interest: Asset URLs resolve correctly in both dev and built
  environments.
- Cancellation/Ordering/Race Constraints: Base-path normalization runs before
  concatenation.

## HL-22 Route Matching and Registration Semantics

### `VRM-HL22-001`

- User Story: Pattern registration normalizes dynamic and splat syntax into a
  consistent matcher form.
- Trigger Conditions: Route pattern is registered.
- Expected Observable Behavior: Dynamic segments normalize to `:param`, splat to
  `*`, explicit index handling normalizes trailing index segment behavior.
- Protected User Interest: Matching behavior is consistent regardless of
  original authored syntax markers.
- Cancellation/Ordering/Race Constraints: Normalization happens before trie/map
  insertion.

### `VRM-HL22-002`

- User Story: Nested route matches include parent route lineage.
- Trigger Conditions: `findNestedMatches` executes for a path.
- Expected Observable Behavior: Returned match list contains nested
  parent-to-leaf matches ordered by segment length semantics.
- Protected User Interest: Loader/component branch stacks reflect nested route
  hierarchy.
- Cancellation/Ordering/Race Constraints: Longest-match conflict rules prune
  ambiguous dynamic/splat/index combinations.

### `VRM-HL22-003`

- User Story: Catch-all matches do not shadow more specific matches.
- Trigger Conditions: Catch-all and more specific matches coexist for path.
- Expected Observable Behavior: Catch-all is removed when stronger specific
  matches are present per conflict rules.
- Protected User Interest: Specific route behavior is prioritized over broad
  fallback behavior.
- Cancellation/Ordering/Race Constraints: Conflict resolution runs before final
  match flattening.

### `VRM-HL22-004`

- User Story: Full-match validity checks prevent false-positive dynamic matches.
- Trigger Conditions: Longest non-splat match length and dynamic parameter
  extraction are evaluated.
- Expected Observable Behavior: Match result is rejected when path exceeds
  non-splat match depth or dynamic extraction is invalid.
- Protected User Interest: Route matching does not produce structurally invalid
  match results.
- Cancellation/Ordering/Race Constraints: Validity checks run before returning
  final match bundle.

## HL-23 URL and Anchor Classification Semantics

### `VRM-HL23-001`

- User Story: Navigation target equality for routing ignores hash fragment.
- Trigger Conditions: Target equivalence is evaluated for lane arbitration and
  map matching.
- Expected Observable Behavior: Data-target comparison strips hash and compares
  absolute URL origin/path/search.
- Protected User Interest: Hash-only differences do not duplicate route-data
  navigation work.
- Cancellation/Ordering/Race Constraints: Navigation-target equality may still
  short-circuit on exact href equality first.

### `VRM-HL23-002`

- User Story: Same-document hash-change and same-document-noop are distinct
- Trigger Conditions: Target classification compares target href to current
  href.
- Expected Observable Behavior: Hash-change classification requires same data
  target with different normalized hash; same-document-noop requires same data
  target with same normalized hash.
- Protected User Interest: Hash jumps and no-op clicks follow separate
  interaction paths.
- Cancellation/Ordering/Race Constraints: Hash normalization uses decode-aware
  fragment normalization.

### `VRM-HL23-003`

- User Story: Click interception excludes modified/new-tab/download/non-anchor
  interactions.
- Trigger Conditions: Anchor details are derived from click event.
- Expected Observable Behavior: Interception eligibility rejects modified
  clicks, middle-click, `_blank`, downloads, and non-interceptable targets.
- Protected User Interest: Native browser interaction patterns are preserved.
- Cancellation/Ordering/Race Constraints: Eligibility check occurs before
  preventDefault.

### `VRM-HL23-004`

- User Story: HTTP href classification distinguishes internal versus external
  targets.
- Trigger Conditions: Href details are resolved.
- Expected Observable Behavior: Non-HTTP URLs are rejected from internal-routing
  pathways; HTTP URLs expose internal/external and relative URL details.
- Protected User Interest: Routing logic applies only to valid HTTP navigation
  targets.
- Cancellation/Ordering/Race Constraints: URL parse errors resolve to non-HTTP
  classification.

## HL-24 Adapter-Generic Runtime Contracts

### `VRM-HL24-001`

- User Story: Adapter runtime subscriptions initialize once at root outlet.
- Trigger Conditions: Root outlet (`idx=0`) initial render path executes.
- Expected Observable Behavior: Route-change and location listeners are
  registered once and drive adapter store synchronization.
- Protected User Interest: Adapter state synchronization avoids duplicate
  listeners and redundant updates.
- Cancellation/Ordering/Race Constraints: Root-init guard prevents repeated
  listener registration.

### `VRM-HL24-002`

- User Story: Route-change events drive navigation-state synchronization before
  scroll application.
- Trigger Conditions: Route-change listener receives event.
- Expected Observable Behavior: Adapter store sync runs, then
  `applyScrollState(event.detail.__scrollState)` runs in
  `requestAnimationFrame`.
- Protected User Interest: Rendered branch is current before scroll adjustments.
- Cancellation/Ordering/Race Constraints: Scroll application is deferred one
  animation frame.

### `VRM-HL24-003`

- User Story: Outlet branch selection uses shared branch-state contract across
  adapters.
- Trigger Conditions: Adapter root outlet renders branch at index `idx`.
- Expected Observable Behavior: Branch state chooses error branch, current route
  component, or fallback outlet using shared branch-state rules.
- Protected User Interest: Equivalent visible branch behavior across adapters.
- Cancellation/Ordering/Race Constraints: Route keys and branch input identity
  are used to constrain remount behavior.

### `VRM-HL24-004`

- User Story: Typed loader/query hooks read data using pattern/index alignment.
- Trigger Conditions: Adapter typed hook factories are used.
- Expected Observable Behavior: Loader data by `idx`, pattern loader data by
  matched pattern index, and router data follow shared type contracts.
- Protected User Interest: Type-safe route data access with consistent
  semantics.
- Cancellation/Ordering/Race Constraints: Pattern lookup returns undefined when
  not matched.

### `VRM-HL24-005`

- User Story: Adapter client-loader registration uses shared runtime
  registration path.
- Trigger Conditions: Adapter `makeTypedAddClientLoader` registers a loader.
- Expected Observable Behavior: Pattern registration and wait-function
  registration occur through shared runtime registration API, with optional HMR
  rerun binding.
- Protected User Interest: Client-loader behavior remains consistent across
  adapters.
- Cancellation/Ordering/Race Constraints: Registration fails explicitly when
  pattern registration preconditions fail.

### `VRM-HL24-006`

- User Story: Adapter link components share one prefetch/click lifecycle
  contract.
- Trigger Conditions: Adapter link renders with `VormaLink` props.
- Expected Observable Behavior: Final link props wire shared prefetch start/stop
  and click handling semantics with optional callback hooks.
- Protected User Interest: Equivalent link behavior regardless of adapter.
- Cancellation/Ordering/Race Constraints: Adapter-specific event wrappers do not
  change shared lifecycle ordering rules.

## HL-25 Development-Time HMR Behavior

### `VRM-HL25-001`

- User Story: Dev runtime exposes revalidate hook for external dev tooling.
- Trigger Conditions: HMR init runs in development mode.
- Expected Observable Behavior: Window-global dev revalidate function is set.
- Protected User Interest: Dev workflows can trigger runtime revalidation.
- Cancellation/Ordering/Race Constraints: Hook is only set in dev mode.

### `VRM-HL25-002`

- User Story: Client loaders refresh after relevant JS module updates.
- Trigger Conditions: HMR update event arrives for tracked module pathname.
- Expected Observable Behavior: Runtime recomputes client loaders and dispatches
  route-change event when tracked pattern intersects current matched patterns.
- Protected User Interest: Dev UI reflects module changes without full reload.
- Cancellation/Ordering/Race Constraints: Refresh call path is debounced.

### `VRM-HL25-003`

- User Story: HMR listeners are registered once per runtime/pathname.
- Trigger Conditions: Re-run-on-module-change registration is requested multiple
  times for same runtime/pathname.
- Expected Observable Behavior: Duplicate listener registrations are suppressed.
- Protected User Interest: Avoid duplicated HMR-triggered refresh behavior.
- Cancellation/Ordering/Race Constraints: Runtime/pathname registration sets are
  used for de-duplication.

## HL-26 Public API Behavior Surface

### `VRM-HL26-001`

- User Story: Programmatic navigation resolves canonical absolute target URLs
  with optional search/hash overrides.
- Trigger Conditions: `vormaNavigate` is called.
- Expected Observable Behavior: Navigation dispatches to runtime as
  `userNavigation` with resolved absolute href and provided options.
- Protected User Interest: Programmatic navigation matches link navigation URL
  semantics.
- Cancellation/Ordering/Race Constraints: Search/hash override apply only when
  option values are explicitly provided.

### `VRM-HL26-002`

- User Story: Revalidation API refreshes current page route data.
- Trigger Conditions: `revalidate` is called.
- Expected Observable Behavior: Runtime dispatches `revalidation` navigation for
  current location href.
- Protected User Interest: Explicit API for route freshness without changing
  URL.
- Cancellation/Ordering/Race Constraints: Revalidation path follows
  deterministic revalidation lane policy.

### `VRM-HL26-003`

- User Story: Submit API exposes structured success/error result contract.
- Trigger Conditions: `submit` is called.
- Expected Observable Behavior: Submit returns `{success:true,data}` on success
  and `{success:false,error}` on failure/abort paths.
- Protected User Interest: Callers can branch on stable result contract.
- Cancellation/Ordering/Race Constraints: Submit lifecycle obeys submission
  ownership and staleness checkpoints.

### `VRM-HL26-004`

- User Story: Location API exposes current browser location plus history state.
- Trigger Conditions: `getLocation` is called.
- Expected Observable Behavior: Returns pathname, search, hash, and current
  history location state payload.
- Protected User Interest: Apps can read current location envelope without
  direct history implementation coupling.
- Cancellation/Ordering/Race Constraints: Values are read at call time.

### `VRM-HL26-005`

- User Story: Root element API fails loudly on invalid root mount node shape.
- Trigger Conditions: `getRootEl` is called and root node is missing or wrong
  element type.
- Expected Observable Behavior: Throws explicit error when root node is absent
  or non-`HTMLDivElement`.
- Protected User Interest: Mount-root contract violations fail early.
- Cancellation/Ordering/Race Constraints: Type check follows existence check.

### `VRM-HL26-006`

- User Story: Runtime exposes status and listener APIs for navigation lifecycle
  observation.
- Trigger Conditions: Status getters and listener registration APIs are used.
- Expected Observable Behavior: Consumers can read current status and subscribe
  to status, location, route-change, and build-id events with cleanup callbacks.
- Protected User Interest: Application instrumentation and UI wiring can react
  to runtime transitions.
- Cancellation/Ordering/Race Constraints: Listener adders provide deterministic
  remove callbacks.

### `VRM-HL26-007`

- User Story: Runtime exposes navigation debug journal for deterministic
  introspection.
- Trigger Conditions: Debug journal API is queried or cleared.
- Expected Observable Behavior: Runtime returns bounded transition journal
  entries and allows explicit journal clearing.
- Protected User Interest: Debugging and diagnostics can inspect lifecycle
  transitions.
- Cancellation/Ordering/Race Constraints: Journal capacity is bounded and oldest
  entries are evicted.
