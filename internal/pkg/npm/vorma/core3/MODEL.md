# Core3 Domain Model

Core3 is an internal model for the same public product contract. The existing
full gate is the acceptance target. This document describes the shape the code
should grow into.

## Center

Core3 should be organized around this equation:

```ts
operation + classified_outcome + publication_transaction -> next_model + effects
```

The model should make policy readable before the browser, network, DOM, module
loader, timer, or user callback layers exist.

## Size Discipline

Core3 ships to browsers. The model should be elegant partly because it is
smaller: fewer duplicated branches, fewer hidden lifecycle variants, fewer
special-purpose side paths, and fewer runtime objects created only to name
things.

Type-only vocabulary is free. Runtime vocabulary is not. Introduce runtime
constants, wrappers, registries, and command builders only when they delete more
code than they add or make an entire class of bugs impossible.

The preferred result is smaller gzip output than the current baseline. A larger
bundle is acceptable only when the added bytes buy obvious correctness and
maintainability.

## Layers

Core3 should have four conceptual layers.

1. Public adapter

    The public adapter normalizes public options, allocates identities,
    registers public waiters, and sends requests into the model. It does not
    inspect model state to decide route policy.

2. Domain model

    The domain model stores facts and makes decisions. It classifies outcomes,
    owns operation lifecycles, grants publication rights, builds publication
    transactions, derives work state, and emits effects.

3. Effect runner

    The effect runner executes effect descriptions and returns facts to the
    domain model. It owns abort controllers, timers, public waiter resolution,
    and other resources that cannot live in pure model state.

4. Browser host

    The browser host performs browser-specific IO. It fetches, imports modules,
    runs client loaders, waits for CSS, reads and writes history and scroll
    storage, mutates DOM from publication plans, and invokes user callbacks when
    commanded.

## Behavior Target

Core3 must preserve the current product contract:

- boot reads the server payload, prepares route modules, runs initial client
  loaders, publishes the initial route, applies head and CSS, emits route
  updates, installs browser listeners, restores reload or hash scroll, and
  starts any deferred refresh or redirect work.
- navigation preserves same-origin hard redirect behavior, same-document
  navigation behavior, active navigation merge and retarget behavior, redirect
  loop protection, prefetch promotion, route hook gating, view transition
  timing, scroll intent, history state, and public navigation settlement.
- popstate handles browser-owned URL changes, leaving-scroll persistence,
  same-document restoration, fetched route publication, and reload fallback when
  the app cannot recover a route.
- revalidation preserves debounce, retry, build-skew, focus-triggered,
  API-triggered, background, and waiter settlement behavior.
- API submit preserves same-origin validation, method normalization, JSON body
  shaping, deployment headers, dedupe replacement, redirect handling, mutation
  revalidation, failure revalidation, and separate API result versus
  revalidation settlement.
- prefetch warms route data and client loaders, can be canceled, can be
  promoted, cannot publish by itself, and leaves public work state when it is
  prepared.
- HMR remains dev-only and updates current route modules without becoming a
  second production route path.
- work state and work indicator behavior remain derived from live route,
  refresh, prefetch, and API operation facts.

## Core Facts

The model should store facts, not side effects.

Required facts:

- lifecycle phase: unbooted, booting, ready, or disposed
- published browser position: href, browser key, user history state
- current route snapshot: route state, render facts, publication position, and
  whether the snapshot is provisional
- client build ID and deployment ID
- option facts that alter policy, such as view transitions and focus
  revalidation stale time
- active route operation, if any
- publication owner, if any
- prefetch operation, if any
- refresh demand and retry state
- API submissions by submission key
- API submission refresh identity when a submission can trigger follow-up
  revalidation
- focus revalidation freshness facts
- last emitted work projection, if needed for duplicate suppression

Dev-only HMR may extend the model with HMR operation identity, but that state is
not part of the production core model.

Forbidden facts:

- DOM nodes
- promises
- abort controllers
- timers
- live callbacks as policy inputs
- global browser objects
- generated random values
- current time reads

Time, browser positions, route responses, API responses, and generated IDs enter
as input facts.

## Identities

Core3 should separate identities by what they mean.

- `operation_id`: one unit of async work or one model-owned operation lifecycle
- `public_call_id`: one public API waiter that must settle once
- `browser_key`: one browser history entry identity
- `submission_key`: one visible API submission slot, often chosen by `dedupeKey`
- `resource_key`: one cacheable resource identity if the resource graph becomes
  part of the implementation
- `timer_id`: one scheduled model timer

No code should infer one identity from another unless the model explicitly owns
that relationship.

## Operations

An operation is a domain record with explicit rights.

Operation kinds:

- boot
- navigation
- popstate
- route_revalidation
- route_prefetch
- api_submit

Dev-only operation extensions:

- hmr_update

Operation lifecycle:

- requested
- started
- superseded
- aborted
- completed
- classified
- prepared
- hooks_running
- publishable
- publishing
- published
- settled
- failed
- ignored_stale

Operation rights:

- may_abort_effects
- may_publish_route
- may_own_visible_transition
- may_satisfy_refresh_demand

Publication rights must be explicit. More than one operation can run, but only
the operation holding route publication rights can publish the visible route.

## Inputs

Inputs to the model should be facts or requests.

Request inputs:

- boot_requested
- navigation_requested
- revalidation_requested
- api_submit_requested
- prefetch_requested
- prefetch_canceled
- view_defined
- dispose_requested

Observed browser inputs:

- popstate_observed
- focus_observed
- beforeunload_observed

Effect outcome inputs:

- boot_payload_read
- route_response_received
- route_response_failed
- route_preparation_completed
- route_preparation_failed
- route_hooks_completed
- route_hooks_failed
- api_response_received
- api_response_failed
- timer_fired
- view_transition_completed

The names can change during implementation, but this split should not: requests
ask for work, observed browser inputs report browser facts, and effect outcomes
report completed effects.

## Effects

Effects are declarative descriptions, not callbacks.

Initial effect families:

- read_boot_payload
- fetch_route
- prepare_route
- run_route_hooks, with immutable public current and next route states
- fetch_api
- abort_operation
- start_timer
- clear_timer
- save_current_scroll
- save_scroll_position
- write_history
- apply_publication_dom
- commit, carrying the operation id and next route snapshot
- render
- apply_scroll
- run_view_transition
- hard_redirect
- reload
- install_browser_listeners
- report_build_skew
- resolve_public_call
- reject_public_call

The effect runner preserves ordered immediate effects within a transaction.
Async effects return operation-correlated outcome inputs.

## Outcome Classification

No response should mutate state before classification.

Route response classification should answer:

- Is this response stale?
- Does this response report build skew?
- Is the default build-skew behavior notify, drop, reload, or settle?
- Is this a soft redirect, hard redirect, invalid redirect, or redirect loop?
- Is this data publishable?
- Is this failure retryable for the current refresh demand?
- Does this failure settle navigation, revalidation, or neither?

API response classification should answer:

- Is this submission stale?
- Was the request dispatched?
- Does this response report build skew?
- Does the public API result settle as success or failure?
- Does the response request hard redirect, soft redirect, or no redirect?
- Should route revalidation be scheduled?
- Does the revalidation promise settle now or wait for route refresh?

Preparation classification should answer:

- Did preparation produce inert prepared route data?
- Was preparation aborted?
- Did route preparation produce a route error state?
- Did route hooks allow publication?
- Did route hooks fail or become stale?

## Publication Transaction

Prepared data is inert until consumed by a publication transaction.

A publication transaction should include:

- operation identity
- publication reason: boot, navigation, popstate, or revalidation
- previous route snapshot
- next route snapshot
- browser position change
- history write plan
- scroll persistence plan
- DOM/head/CSS plan
- route render commit
- route update notification facts
- render callback effect
- scroll intent
- work projection update
- public settlement
- refresh settlement or continuation
- deferred redirect or deferred refresh continuation
- view transition boundary

The transaction should have named phases:

- before_transition: effects needed before the browser captures a transition
  snapshot
- inside_transition: history, DOM, current route, commit, and render changes
  that must appear as one visual publication
- after_transition: public settlement, deferred continuation, and cleanup that
  must wait for publication to finish

If a phase ordering matters, the transaction should say so locally.

Route render commits and route update notifications are related but not the
same. Publication may need to re-render the current route state even when public
route facts did not change. Route update notification only exists when the
public route state changes: href, history state, build id, params, splats,
error, or match input/data facts. The route update payload is the public route
state shape, not internal render facts. Module identity and module URL changes
are not public route-update facts by themselves.

Same-document publication is still publication. Hash navigation, replace on the
current document, and same-document popstate should retarget the current route
snapshot to the browser position, commit route-render state, emit route-update
only when public route facts changed, and settle public waiters without fetching
or preparing route data. Same-hash navigation without replace is the narrower
case: it applies scroll and settles the public waiter without publishing a route
commit.

The publication commit boundary installs the next route snapshot in the model
before route-render commit data or route-update notification data is observed by
callbacks. This keeps the model, browser position, render commit, and public
route update facts on one side of the same transaction boundary.

Publication settlement is separate from commit. Settlement happens after the
transaction's after-transition effects have run: the route operation becomes
settled, boot moves to ready, a running refresh satisfied by that publication
returns to idle, and any pending refresh whose predecessor just settled can
start as a real route revalidation operation.

Prepared route data is not automatically publishable. Boot can publish prepared
data directly, because there is no previous route to yield from. All other route
publication paths must pass through route readiness: either no hooks are present
and the route becomes publishable, or the model emits a route-hook effect and
waits for the correlated hook outcome.

Boot provisional state is current-route state, but not publication. It exists so
initial client loaders can read route facts and submit API work while boot is
still preparing the final route. Provisional state must be replaced by final
publication, and route update notifications treat provisional previous state as
no previous public route.

## Resource Graph

Core3 should validate whether route publication is clearer as a resource graph.

Candidate resource nodes:

- route_payload
- route_match
- route_module
- route_css
- route_head
- server_loader_data
- client_loader_data
- prepared_route
- published_route
- api_response
- build_identity

Candidate edges:

- route_payload depends on href, client build ID, deployment ID, and trigger
- route_match depends on route_payload and current URL
- route_module depends on module URL
- client_loader_data depends on route_module, route_match, server loader data,
  current route state, history state, and trigger
- prepared_route depends on modules, loader outcomes, CSS readiness, head data,
  and route payload
- published_route depends on prepared_route, route hooks, publication rights,
  and browser position

The graph is optional until it proves its value. It should be adopted only if it
removes special cases around prefetch promotion, revalidation, HMR, CSS, and
client loader reuse.

## Route Flows

Boot:

1. Read boot payload and browser position.
2. Store build and deployment facts.
3. Prepare modules.
4. Install provisional route snapshot so public route reads and initial loader
   submits work during boot.
5. Run client loaders.
6. Wait for CSS.
7. Build final prepared route.
8. Publish initial route with reload-scroll or hash-scroll intent.
9. Configure callbacks, work indicator, focus revalidation, browser listeners,
   and HMR.
10. Start deferred API redirect or refresh demand if present.

Navigation:

1. Resolve href against current browser position.
2. Hard redirect off-origin requests.
3. Handle same-document navigation without fetch.
4. Merge or retarget matching active navigation.
5. Supersede incompatible active route work.
6. Promote matching prefetch or fetch route data.
7. Classify route response.
8. Prepare route data.
9. Run route hooks.
10. Publish transaction.
11. Settle public navigation and any satisfied refresh demand.

Popstate:

1. Observe browser-owned next position and leaving scroll.
2. Ignore duplicate browser position.
3. Persist leaving scroll for previous browser key.
4. Handle same-document popstate without fetch.
5. Start popstate route operation.
6. Publish fetched or promoted route.
7. Reload if browser and route snapshot cannot be reconciled.

Revalidation:

1. Record refresh demand with waiters and skip-work facts.
2. Debounce when requested.
3. Wait behind active visible route work unless the active work will satisfy the
   demand.
4. Run route revalidation against the current browser position.
5. Retry with capped backoff on retryable failure.
6. Drop stale revalidation if the browser position changed.
7. Publish if still current and publishable.
8. Settle waiters as ok, build_skew, or max_retries_exhausted.

API submit:

1. Validate same-origin target before dispatch.
2. Normalize method and body.
3. Replace prior submission in the same submission slot.
4. Dispatch fetch with deployment and redirect headers.
5. Classify response.
6. Settle public API result.
7. Schedule revalidation when policy requires it. The refresh demand owns its
   own operation identity and optional public waiter; it is not the API result
   promise.
8. Route hard redirects, soft redirects, and deferred boot redirects through the
   route operation model.
9. Settle revalidation promise independently from API result.

Prefetch:

1. Ignore before ready, off-origin, current route, and matching active
   navigation targets.
2. Cancel stale prefetch work.
3. Fetch route data and prestart known client loaders.
4. Prepare resources without publication rights.
5. Keep prepared data inert for promotion.
6. Remove prepared prefetch from public work state.
7. Abort on explicit cancellation or incompatible route work.

HMR:

1. Ignore outside dev mode.
2. Match the updated module against current route entries.
3. Update module resource.
4. Optionally rerun client loader for opted-in patterns.
5. Publish as a revalidation-flavored route update without becoming a production
   route path.

HMR operation, observation, and preparation effects belong to the dev-only HMR
extension. Production host code must not statically import that extension.

## Work Projection

Public work state should be a projection from operation facts:

- navigation comes from visible navigation or popstate route work
- revalidation comes from refresh debounce, retry, or running route revalidation
- prefetch comes from pending route prefetch
- apiRequests come from active submissions

Private work-indicator activity should be derived from the same facts plus
skip-work flags. Public work and private activity must not be pushed from
branch-local code.

## Invariants

- Behavior truth overrides architecture preference.
- Public call settlement is outcome policy, not an operation right.
- A public call settles exactly once.
- A stale async completion cannot publish.
- A prepared route cannot publish without publication rights.
- Prefetch never publishes directly.
- Route hooks gate publication but do not publish.
- Browser host facts do not decide route policy.
- Hard redirect and reload effects are explicit outcomes.
- Work state is derived, not manually maintained.
- View transitions wrap publication; they are not an alternate publication path.
- Build skew is classified before route or API state mutation.
- Current route state is updated before route update callbacks observe it.
- Public API promises and follow-up revalidation promises settle independently.
