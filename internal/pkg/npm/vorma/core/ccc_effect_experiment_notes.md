# Effect Client Runtime Experiment Notes

This experiment is trying to answer whether Vorma's frontend client runtime can
be rebuilt as an internal Effect kernel while preserving the current public app
API first.

The goal is not to port `create_client_core.ts` line by line. That file is the
thing being tested against, not the shape to imitate. Each slice should be
written from first principles as an Effect-owned subsystem with explicit state,
fiber lifetimes, cancellation, typed failure, and testable side-effect
boundaries.

## Current Judgment

The public API should probably stay mostly promise-shaped for now. App authors
should not need to adopt Effect to call `navigate`, `revalidate`, `submit`, or
write a normal client loader.

The internal runtime should become much more Effect-shaped than the current
compatibility surface. A better final architecture is:

1. An Effect kernel made of services for navigation, revalidation, submission,
   route preparation, route publication, work state, browser history, fetch,
   module loading, CSS/head updates, build-skew reporting, scroll, and HMR.
2. A thin compatibility adapter that lowers the Effect kernel into today's
   `ClientCore` shape.
3. Optional Effect-facing APIs later, after the internal runtime has proven
   parity and after we can see which user-facing Effect affordances are worth
   exposing.

That gives Vorma the robustness and simplification benefits without turning the
main app API into an Effect doctrine test.

## What Feels Validated

- Revalidation wants to be an actor with queued demand, debounced requests,
  retry timing, waiter sharing, and typed build-skew failure.
- Revalidation also wants a higher-level route service above the generic actor:
  one place that owns "fetch current route, classify response, prepare payload,
  publish guarded freshness, follow soft redirects as replace navigations, and
  report build skew".
- Navigation wants a single owner for the active navigation fiber, waiter
  transfer, supersession, redirect chains, and stale publication protection.
- Submission wants keyed fiber ownership, interruptible dedupe, typed dispatch
  failures, redirect handoff, and revalidation as a returned Effect.
- Submission now looks best as an API-facing actor plus a thin client adapter:
  the actor returns an Effect revalidation handle, and the adapter lowers that
  into today's `revalidationPromise` without making the public API
  Effect-shaped.
- Route preparation wants explicit services for payload decoding, module
  loading, client-loader registration, client-loader fibers, CSS waiting, and
  delayed DOM effects.
- Route publication wants an owned route snapshot, Effectful transition hooks,
  guarded publication, and a precise projection into the existing `ClientCommit`
  shape.
- Route publication also wants a first-class position-only transition. Hash-only
  navigation and same-route popstate should update the route snapshot and emit a
  render/route update without pretending they are fetch/preparation work.
- Prefetch wants a small actor/cache rather than route-side special cases: start
  owns one abortable fetch/prepare fiber, stop aborts synchronously enough for
  the public API, completion stores a prepared route, and navigation consumes
  that route without refetching.
- Build-skew reporting is cleaner as a shared Effect service than as
  navigation/revalidation/submission branches. The service owns the common event
  envelope and lets each caller provide only the response, trigger details, and
  default behavior.
- Focus-triggered revalidation wants its own tiny state service. The browser
  listener can remain at the adapter edge for now, but stale-time policy,
  work-state gating, and the window-focus revalidation request belong in Effect.
- Browser history wants an owned service too. Navigation should commit history
  through that service, popstate should adopt the browser's existing state
  instead of pushing or replacing, and route publication should consume a
  `HistoryPosition` instead of rediscovering it from ambient globals.
- Scroll restoration fits the same pattern: saved scroll positions, reload
  handoff, hash intent, default top-scroll behavior, and popstate restoration
  are cleaner as a browser service that returns explicit `ScrollState` values to
  route publication.
- Runtime lifecycle should own listeners and actor shutdown as finalizers. The
  adapter can keep the public boot shape, but boot should install one kernel
  lifecycle and replace the previous one atomically after the new initial route
  is ready.
- Work indicators fit as an Effect-owned state machine too. App-owned tracked
  promises and Vorma-owned work commits can share one token model, while
  renderer replacement remains a configuration transition instead of leaking
  timer state into the adapter.
- Per-operation work-indicator skips belong in the work actor's internal
  projection, not in public `WorkState`. The public state remains clean, while
  navigation/API request metadata can still drive the indicator accurately.
- Mixed work ordering is a good fit for the token model. App-owned promises can
  overlap Vorma navigation/revalidation/API work without show-hide thrash, and
  renderer replacement can happen while work is visible without orphaning state.
- The real switch-over pressure test should reuse the existing
  `create_client_core.test.ts` suite, not copy it. The experiment now has a tiny
  runner that swaps only `create_client_core` for the Effect adapter and imports
  the existing suite. That runner moved from 73/104 passing to 104/104 passing
  after fixing boot-time provisional route state, boot submit revalidation, DOM
  side effects, work-indicator reconciliation, client-loader error
  normalization, view-transition publication, client-loader prestarts, prefetch
  reuse/cancellation, navigation/revalidation supersession, and HMR route
  replacement.
- Client-loader prestart is the clearest proof that this should be a real
  runtime, not a port. It works best as an owned capability of the route
  preparation service, with navigation and prefetch actors acquiring abortable
  prestarts before the server payload resolves and handing them back to
  preparation when payload data arrives.
- HMR also fits the service model. Module replacement should update the module
  cache, refresh the active route snapshot through the publisher, and optionally
  rerun client loaders for opted-in patterns without entering the navigation or
  revalidation actors.
- Module loading and HMR now have a dedicated `module_runtime` service. The
  adapter no longer owns the module cache or the HMR route replacement logic; it
  only asks the service to load modules, remember HMR rerun preferences, and
  install the browser callback for the current kernel.
- Route DOM effects now have a dedicated `route_dom_runtime` service. The
  adapter no longer owns title decoding, CSS preloading/waiting, head/title
  application, stylesheet application, or modulepreload side effects; route
  preparation receives those as service capabilities.
- Browser location now has a dedicated `browser_location` service. Same-origin
  checks, absolute href resolution, route keys, hash extraction, current href
  reads, and hard redirects are no longer local adapter helpers.
- The compatibility pressure test has expanded beyond `create_client_core` into
  split runners for `router.test.ts`, `router_revalidation.test.ts`, and
  `router_submit.test.ts`. Keeping those runners split matters because the
  revalidation suite intentionally installs fake timers; importing all router
  suites into one file polluted unrelated tests.
- The Effect adapter now passes the existing client-core, router, revalidation,
  and submit suites through those swap runners. That is 284 existing tests
  exercising the Effect implementation behind the current public client
  contract.
- The broader suites forced a useful architecture correction: browser-facing
  APIs need synchronous entry edges for observable state, cancellation, and
  fetch dispatch, while long-running work still belongs in owned Effect fibers.
  Navigation, submission, work state, prefetch completion, and revalidation
  cancellation now use direct Effect state transitions or explicitly owned
  fibers where the public contract needs immediate visibility.
- Navigation now has a first-class idle effect, which lets revalidation defer
  behind active navigation without polling or smuggling router state through the
  revalidation layer.
- Prefetch promotion is cleaner with an active completion handle. An in-flight
  prefetch is now a value-producing fiber that navigation can await and consume,
  rather than just an opaque active fetch plus a later prepared cache slot.
- Submit redirects are best treated as handoffs. The submit actor classifies and
  reports the response, then starts client navigation without awaiting the route
  fetch so the submit result can resolve under the existing API contract.

## What Still Needs To Become More Effect-Native

- Service dependencies should probably move from broad options objects into
  `Context` / `Layer` once the slices settle.
- Route payload decoding should move to a real decoder, likely Effect Schema or
  an equivalent local schema layer.
- Revalidation retry/backoff should be revisited with `Schedule`.
- Browser APIs need explicit services for fetch and timers. Location, history,
  scroll, module import, Vite HMR, and route DOM side effects now have
  first-pass services, but they still need a later `Context` / `Layer` cleanup.
- Work-state emission should be owned by a service instead of being derived
  opportunistically from mutable outer variables.
- Work indicator parity now covers category-level skips, per-operation skips for
  navigation and API requests, mixed app/Vorma work ordering, and renderer
  replacement while work is visible.
- Browser-facing void APIs expose an interesting boundary pressure. Internally
  Effect wants async acknowledgement, but public calls like `stop_prefetch`
  still need immediate observable cancellation, so the service needs explicit
  synchronous ownership of the abort controller.
- Public promise APIs expose the same pressure in a milder form. Navigation can
  remain promise-shaped, but supersession needs a synchronous signal-abort path
  so already-started loaders and transition hooks observe cancellation at the
  same moment the public call is made.
- Browser listeners need explicit lifecycle ownership before switch-over. The
  experiment now removes the previous kernel's focus, popstate, and beforeunload
  listeners through a lifecycle service. Popstate runs through the Effect
  navigation actor, scroll restoration is now an Effect service, and hash-only
  movement is now represented as route publication rather than fetch work.
- The final client assembly should be scoped. Starting the client should acquire
  fibers/listeners/resources, and shutdown should release them.
- Actors created inside the compatibility shell need daemon or explicit runtime
  ownership. Otherwise fibers created by `Effect.runSync` can be scoped away
  before browser callbacks get to use them.
- Lifecycle finalizers need cause-level containment. Actor shutdown can die with
  interruption causes, so finalizer handling must use cause-aware recovery
  rather than only catching typed errors.
- The existing-suite runners are now green. The next work should be about making
  the service graph cleaner, not chasing broad parity gaps.

## Switch-Over Bar

The Effect runtime should not be wired directly into `create_client_core.ts`
piece by piece. That would pull the experiment back into port-shaped code.

The better switch-over path is:

1. Keep building the parallel kernel beside the current implementation.
2. Build an adapter that exposes the current `ClientCore` contract.
3. Run the existing client-core tests against both implementations.
4. Fill parity gaps in the kernel, not by copying legacy internals.
5. Once parity is real, flip the default implementation behind the same public
   API.

Long term, the best outcome is that `create_client_core.ts` disappears as a
large imperative runtime. What remains is a small compatibility shell around a
scoped Effect system whose services are independently testable, replaceable in
tests, and explicit about ownership.
