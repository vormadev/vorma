# Core2 Architecture Plan

Core2 is a clean restart for one specific experiment: can Vorma's client router
be simpler and more robust if the core is an event reducer plus command
interpreter instead of a mutable closure?

This is not an Effect rewrite. It is not a transition-object rewrite. It is not
legacy with different filenames. The reducer is the product.

## The Bet

Legacy already has many good concepts: active fetches, prepared routes,
snapshots, publish gates, refresh demand, prefetch promotion, submit/revalidate
interaction, and derived work state.

The problem is that those concepts are implemented as shared closure mutation
spread across many local functions. Core2 should use a different architecture:

```ts
update(state, event) -> { state, commands }
```

That is the center of the design.

The reducer decides what changed and what side effects should happen. The
interpreter performs side effects and dispatches completion events back into the
reducer. Public APIs only dispatch events and await correlated outcomes.

## Why This Is Actually Different From Legacy

Legacy runs imperative flows directly:

- `navigate` calls `start_nav_inner`
- `start_nav_inner` mutates `active`
- `run_active` awaits fetch and preparation
- `publish` mutates route state, history, DOM, active work, and commits
- revalidation and boot have parallel paths with similar but not identical rules

Core2 must invert that:

- public APIs dispatch events
- reducer mutates only state
- reducer emits commands
- interpreter runs commands
- command completions dispatch more events
- stale completions are ignored by reducer identity checks

The real improvement is not a nicer name. It is that the core routing rules
become pure data transitions that do not need DOM, fetch, timers, history,
module imports, or render callbacks.

## Non-Negotiables

1. Core2 must not import Effect.
2. Core2 must not copy structure from legacy `create_client_core.ts`.
3. Core2 must not copy structure from the failed Effect runtime.
4. Core2 must not use a central `kernel.ts` that owns everything through closure
   locals.
5. Core2 must not start with a `ClientRuntime` full of imperative methods.
6. No fake helpers that only wrap a one-line operation.
7. No manager-per-concern ceremony.
8. Public API compatibility is an adapter concern, not the internal design.
9. The first implementation artifact must be the reducer.
10. No route behavior may be implemented outside event handling and command
    interpretation.
11. If the code starts looking like legacy with renamed nouns, delete it.

## Architectural Center

The center is a pure reducer:

```ts
type UpdateResult = {
	state: ClientState;
	commands: ClientCommand[];
};

function update(state: ClientState, event: ClientEvent): UpdateResult;
```

The reducer must be deterministic and side-effect free.

Forbidden inside `update`:

- `fetch`
- `import`
- DOM access
- history writes
- timers
- promises
- callbacks into user code
- random IDs
- current time
- global reads

Allowed inside `update`:

- compare event IDs against state IDs
- update route, work, refresh, submission, and prefetch facts
- derive commits as command data
- request effects as command data
- ignore stale events
- emit outcome-resolution commands for public API promises

## State Model

`ClientState` is the complete reducer state.

It should contain facts, not side effects:

- phase: `idle`, `booting`, or `ready`
- current route snapshot
- browser position as last known by the reducer
- active route operation, if any
- prefetch operation, if any
- refresh demand/state
- API submissions by key
- registered view metadata needed by pure decisions
- client build and deployment IDs
- options selected at boot that affect reducer decisions
- last emitted work state, if needed to suppress duplicate work commits

It must not contain:

- DOM elements
- live module objects unless represented as data passed in from interpreter
- timers
- promises
- abort controllers
- functions from user code, except opaque references stored as data if needed

## Events

Events are the only input to the reducer.

Event names are past-tense for facts that happened and request-tense for public
requests.

Initial event families:

```ts
type ClientEvent =
	| BootRequested
	| BootPayloadRead
	| BootFailed
	| ViewDefined
	| NavigationRequested
	| PopstateObserved
	| RouteResponseReceived
	| RouteResponseFailed
	| RoutePrepared
	| RoutePreparationFailed
	| RoutePublished
	| RevalidationRequested
	| RefreshTimerFired
	| APISubmitRequested
	| APIResponseReceived
	| APIResponseFailed
	| PrefetchRequested
	| PrefetchCanceled
	| WorkSettled
	| Disposed;
```

Events that correspond to async work must carry a correlation ID. The reducer
uses that ID to ignore stale completions.

Example:

```ts
type NavigationRequested = {
	type: "navigation_requested";
	requestID: string;
	href: string;
	replace: boolean;
	historyState: unknown;
	scrollToTop: boolean | undefined;
};

type RouteResponseReceived = {
	type: "route_response_received";
	operationID: string;
	response: RouteResponseData;
};
```

## Commands

Commands are the only output that may cause effects.

Commands are declarative descriptions, not functions.

Initial command families:

```ts
type ClientCommand =
	| ReadBootPayloadCommand
	| FetchRouteCommand
	| PrepareRouteCommand
	| AbortOperationCommand
	| PushHistoryCommand
	| ReplaceHistoryCommand
	| CommitCommand
	| RenderCommand
	| FetchAPICommand
	| StartTimerCommand
	| ClearTimerCommand
	| PreloadCommand
	| ApplyDOMCommand
	| ResolvePublicCallCommand
	| RejectPublicCallCommand;
```

The interpreter owns command execution. If a side effect is needed and there is
no command for it, add a command. Do not sneak the effect into the reducer or
public API adapter.

## Interpreter

The interpreter is boring plumbing:

1. Receive commands from `update`.
2. Run host IO, timers, module imports, CSS/head operations, fetches, and user
   callbacks.
3. Dispatch completion events back into the reducer.

The interpreter may maintain effect resources that cannot live in reducer state:

- abort controllers by operation ID
- promise resolvers for public API calls
- timer IDs
- module cache
- event listener cleanup functions

The interpreter must not own routing decisions. It asks the reducer what to do
next by dispatching events.

## Public API Adapter

`create_client_core` should be a shell over dispatch.

Examples:

```ts
navigate(href, options) {
  const requestID = nextID()
  dispatch({ type: "navigation_requested", requestID, href, options })
  return waitForPublicResult(requestID)
}
```

The public API adapter may:

- allocate request IDs
- expose promises for public calls
- adapt camelCase public options into event payloads
- convert final failures to the existing public result shape

It must not:

- mutate route state
- decide stale response behavior
- decide refresh behavior
- write history
- run client loaders
- commit directly

## Core Route Flow

Navigation should look like this:

1. Public API dispatches `navigation_requested`.
2. Reducer:
    - aborts/supersedes the previous active route operation by command
    - records the active operation
    - derives navigation work
    - emits `fetch_route`
    - emits work commit if work changed
3. Interpreter runs route fetch.
4. Interpreter dispatches `route_response_received` or `route_response_failed`.
5. Reducer ignores stale responses or emits `prepare_route`.
6. Interpreter prepares route data, including module imports, CSS waits, and
   client loaders.
7. Interpreter dispatches `route_prepared` or `route_preparation_failed`.
8. Reducer ignores stale prepared routes or emits:
    - history command
    - DOM apply command
    - route/render commit command
    - render command
    - public result resolution command
    - work clear commit, if work changed

Boot and revalidation should reuse this flow. They may have different events and
commands, but not different architecture.

## Reducer-Owned Invariants

The reducer owns these rules:

- only one active route operation can publish
- stale route responses do not publish
- stale prepared routes do not publish
- superseded route operations emit abort commands
- navigation work belongs to the active navigation operation
- revalidation work belongs to refresh or active revalidation state
- API work belongs to individual submission IDs
- a public navigation promise resolves exactly once
- a public revalidation promise resolves exactly once
- route commits happen only from publish events
- work commits happen only when derived work changes
- boot is the only path from `idle` to `ready`

## Pure Route Preparation Boundary

Route preparation has two layers:

1. Pure transforms:
    - decode payload into route data
    - match URL to known patterns
    - build route snapshot
    - apply client loader results
    - build route render state

2. Interpreter effects:
    - fetch route JSON
    - import modules
    - run client loaders
    - wait for CSS
    - apply head and CSS

The reducer can request preparation with a `prepare_route` command. The
interpreter runs the effects and dispatches a `route_prepared` event containing
plain prepared data.

## Work State

Work state should be derived from reducer facts.

Facts:

- active route operation
- refresh state
- submissions map
- prefetch operation

Derived public `WorkState`:

- navigation
- revalidation
- prefetch
- API requests

Derived private work activity:

- navigation `skipWorkIndicator`
- revalidation `skipWorkIndicator`
- API request `skipWorkIndicator`
- prefetch presence

The public `WorkState` stays free of private indicator flags. The reducer
compares both the public work projection and the private activity projection
against the last emitted values, then emits a work commit command only when one
changes. No standalone helper may push a work commit; the command belongs next
to the state transition that caused it. Small projection helpers are allowed
when they keep public work and private activity from drifting apart.

## Refresh And Revalidation

Refresh should be an event flow, not a side process hidden in timers.

Example:

1. `revalidation_requested`
2. reducer records refresh demand and emits timer/fetch commands
3. `refresh_timer_fired`
4. reducer emits `fetch_route` for revalidation if no active route operation
   blocks it
5. route response/preparation/publish follows the same route flow
6. reducer resolves refresh waiters through commands

Retries are state plus timer commands. Build skew and exhausted retries are
events and reducer outcomes.

## API Submission Flow

Submissions are independent operations.

1. `api_submit_requested`
2. reducer records submission and emits `fetch_api`
3. interpreter fetches
4. interpreter dispatches `api_response_received` or `api_response_failed`
5. reducer clears submission work
6. reducer emits public result resolution command
7. reducer may emit `revalidation_requested` or navigation events for redirects

This must allow a client loader to submit without deadlock because client
loaders run in the interpreter, and submission starts by dispatching an event to
the reducer.

## Cancellation

Cancellation is command-driven.

The reducer does not own abort controllers. It emits:

```ts
{
	type: ("abort_operation", operationID);
}
```

The interpreter maps operation IDs to abort controllers and aborts them.

Completion events from aborted operations may still arrive. The reducer ignores
them because the operation ID is no longer current.

## Design Pressure Notes

These are the current architectural danger zones. Preserve these ideas across
context compaction.

1. Do not let `update.ts` become a god reducer by accident. If it needs to
   split, split into reducer-owned subdomains that keep the same
   `state + event -> state + commands` algebra. Do not introduce managers,
   runtimes, service objects, or imperative methods as the split mechanism.

2. Host convenience is the biggest backslide risk. Browser code may return facts
   and execute commands, but it must not decide route policy. API redirects,
   build-skew behavior, stale completion handling, refresh timing, navigation
   supersession, and public promise settlement belong in events, reducer state,
   and commands.

3. The public shell must remain boring. It may allocate IDs, adapt public option
   names, create public waiters, and dispatch events. It must not inspect router
   state to decide route behavior, start hidden async flows, special-case
   redirects, or directly commit/render.

4. Route preparation is the legitimate hybrid boundary. Effectful preparation
   may fetch/import/run loaders/wait CSS, but its output to the reducer must be
   plain prepared data. Pure payload decoding and route-state construction
   should stay outside the host where possible.

5. Prefer stronger domain events and commands over clever helpers. If the
   reducer feels repetitive, first ask whether the event/command vocabulary is
   missing a real concept such as `publish_result`, `api_redirect_received`, or
   `refresh_demand_scheduled`. Do not hide policy in one-line wrappers.

6. Browser keys, operation IDs, public call IDs, and submission IDs are reducer
   facts once created. Generate them at public/host edges and pass them in
   events. Do not generate identity inside the reducer or inside a side-effect
   command that the reducer later needs to know about.

7. Compatibility pressure is real but not allowed to punch holes through the
   architecture. If an existing public API bundles multiple concepts, model
   those as multiple correlated public calls or events rather than rebuilding a
   hidden imperative chain in the adapter.

8. Browser lifecycle is command-owned. Boot may ask the interpreter to install
   listeners, and the interpreter owns cleanup. The host reports facts such as
   popstate browser position, restored scroll, and leaving scroll; it does not
   decide navigation policy.

9. Same-document navigation is a reducer transition, not a fetch shortcut hidden
   in the browser host. The reducer updates browser/route facts, emits history
   and scroll-related commands, commits the existing route render state with a
   new scroll intent, and resolves public calls.

10. Scroll persistence has two distinct edges. Normal navigation saves the
    current browser key before writing history. Popstate saves the reducer's
    previous browser key using scroll coordinates measured at the popstate event
    edge.

11. Route transition hooks are host effects gated by reducer identity. A
    prepared route may turn into a `run_route_hooks` command, and only the
    correlated `route_hooks_completed` event may publish it. Hook callbacks must
    not live in the public shell or in a hidden imperative publish method.

12. Build-skew reporting is reducer policy plus host callback. Fetching returns
    server build facts, the reducer decides the default behavior and builds the
    public event, and the interpreter executes `report_build_skew`. Browser code
    must not decide skew policy.

13. The interpreter preserves command order for immediate host effects. Fetches,
    timers, and hook runs may create later completion events, but history, DOM,
    commit, render, scroll, hard redirect, and reporting commands run in the
    order the reducer emitted them.

14. View transitions are command composition, not a second publish path. The
    reducer may wrap immediate publication commands in `run_view_transition`;
    follow-up commands such as deferred redirects or pending revalidations stay
    outside that visual transaction.

15. Time is an input fact. The reducer may compare timestamps carried by events,
    but it must never read the clock. Focus-triggered revalidation stores
    freshness in reducer state and turns stale focus events into normal
    revalidation demand.

16. The initial browser history key is created at the boot edge and passed
    through the boot event/command path. Scroll, popstate, and focus logic must
    not operate on an empty initial browser identity.

17. Hard-reload scroll restoration is a boot fact, not a browser shortcut. The
    host may persist and read recent reload scroll, but only the reducer carries
    that fact on the boot operation and turns it into the initial scroll intent.

18. Off-origin navigation is reducer policy. The public shell should not decide
    that a navigation is external, and the browser host should not discover it
    by accidentally attempting a route fetch. The reducer compares the requested
    URL against the current browser position and emits `hard_redirect`.

19. Boot may install a provisional current route before client loaders finish.
    The host reports the provisional prepared route after module loading; the
    reducer records it as provisional state so public APIs work during initial
    client-loader execution. The final boot publish must still report
    `previous_route: null`, because provisional state is an internal
    availability fact, not a public route update.

20. Boot options that are callbacks are commit/host-edge resources, not reducer
    policy. `render`, `onRouteUpdate`, `onWorkUpdate`, `onBuildSkewDetected`,
    and work-indicator timing are configured by the public shell and consumed at
    the commit/host edge. The reducer only stores option facts that change route
    decisions.

21. The switch-over seam should assemble browser host, callback edge, shell, and
    controller in one place. Package-level adoption should be a single factory
    replacement, not piecemeal calls from the legacy implementation into core2.

22. Prefetch promotion is a reducer transition. A prepared prefetch is stored as
    route data under the prefetch operation; matching navigation or popstate
    promotes it into the normal route-hook and publish path. Non-matching
    navigation/popstate cancels the stale prefetch by command.

23. HMR runtime work is dev-build-only. Production may retain small event or
    command vocabulary, but browser HMR callback installation and HMR route
    preparation must sit behind direct `import.meta.env.DEV` branches so Vite
    can collapse them to no-ops.

24. In-flight navigation reuse belongs to the reducer. Identical navigation
    requests append public waiters to the active route operation; same-route
    requests with different options replace the publish intent while keeping the
    existing fetch identity. Popstate can also retarget an active same-route
    navigation without refetching. The shell must not share promises or swap
    navigation state behind the reducer.

25. The interpreter has one abortable-operation lifecycle. Route fetches, API
    submits, boot payload reads, HMR preparation, route preparation, and route
    hooks all start through the same controller/forget/failure path. Individual
    command cases may choose success and failure events, but they must not
    hand-roll cancellation bookkeeping. Dispose must abort every recorded
    operation identity, including route, prefetch, HMR, and API work.

26. API submission keys and async identities are separate facts. A public
    `dedupeKey` chooses the visible submission slot; each actual fetch gets a
    fresh operation ID. Replacing a keyed submission aborts and settles the old
    public call, while stale network completions are ignored by operation ID.

27. Browser URL validation belongs at the host edge, and redirect policy belongs
    in the reducer. The host resolves API URLs against browser location, rejects
    cross-origin submits before fetch, and annotates redirect targets as HTTP or
    non-HTTP. The reducer decides whether a redirect means follow, settle, hard
    redirect, or drop stale work.

28. API revalidation is completion policy, not success policy. A submitted
    mutation may request revalidation after success, non-ok response, or fetch
    failure once the request was actually dispatched. Host-shaped failures that
    happen before dispatch, such as cross-origin submit validation, must be able
    to opt out.

## Implementation Order

1. `model.ts`
    - `ClientState`
    - `ClientEvent`
    - `ClientCommand`
    - IDs, route operation records, refresh state, submission state

2. `update.ts`
    - pure reducer
    - no imports from browser host modules

3. `commands.ts`
    - command constructors only if they remove real repetition
    - no one-line wrappers

4. `interpreter.ts`
    - command execution
    - abort controller registry
    - public promise resolver registry
    - dispatch loop

5. `route_prepare.ts`
    - pure transforms and preparation result types
    - effectful preparation called by interpreter only

6. `host.ts`
    - browser boundary
    - only real IO seams

7. `create_client_core.ts`
    - public API adapter over dispatch

The file list is not the architecture. The reducer/command rule is the
architecture. Files only exist to protect that rule.

## Failure Criteria

Delete or redesign immediately if any of these happen:

- public methods directly mutate router state
- route flow is implemented as an imperative async method instead of
  event-command steps
- side effects occur in the reducer
- stale response logic lives outside reducer identity checks
- work commit commands move into wrapper helpers instead of staying beside the
  transition that caused them
- a new central runtime file accumulates unrelated local helper functions
- commands become callbacks instead of data
- interpreter starts deciding routing policy
- reducer code requires DOM, fetch, timers, or module imports

## Success Criteria

Core2 is succeeding only if all are true:

- the reducer can be understood without a browser
- most behavior questions are answered by reading event cases
- async work is visible as commands and completion events
- stale work is handled uniformly by IDs
- boot, navigation, and revalidation share one route operation flow
- API submissions are independent event flows
- the public API shell is boring dispatch plumbing
- production runtime code is materially smaller and easier to reason about than
  legacy while approaching parity
