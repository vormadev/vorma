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
- Work indicator delays are now Effect-owned fibers rather than raw browser
  timers. The runtime schedules show/hide through `Effect.sleep` and cancels
  pending delays by interrupting fibers, so timer ownership participates in the
  same execution model as the rest of the kernel.
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
- Browser view concerns now have a dedicated `browser_view_runtime` service. The
  adapter no longer owns boot payload script reads, root element creation,
  hash/coordinate scroll application, or the `document.startViewTransition`
  bridge. View transitions now resume the surrounding Effect with the original
  typed publication failure when publication fails, and only map browser
  transition failures back into route commit failure at the adapter edge.
- Browser fetch now has a shared `browser_fetch_runtime` service. Route fetching
  and submission no longer own ambient `fetch` lookup, abort-signal merging, or
  transport-level abort classification; they consume typed browser transport
  failures and map them into route/submission domain errors.
- Abort semantics now have one shared `abort_signal` module. Browser fetch and
  client-loader execution use the same abort-signal merge behavior and the same
  DOM abort error contract, instead of duplicating `"AbortError"` handling in
  separate services.
- Runtime service construction now has a first assembly boundary. The
  compatibility shell asks `client_runtime_services` for location, view, fetch,
  DOM, module, submit-dispatch, and work-indicator services instead of creating
  each one directly. Those services now have explicit Effect `Context` tags and
  a Layer-shaped assembly, including submit dispatch depending on browser fetch.
  The shell still receives a plain service record for compatibility, but the
  ownership shape is now Effect-native enough to grow into the final graph.
- The kernel shape and actor finalizer registration now live in `client_kernel`.
  That is a small but important direction marker: lifecycle ownership belongs to
  the Effect system, while `create_client_core_effect` should keep shrinking
  toward public API adaptation and boot orchestration.
- Per-boot kernel resources now have a matching service boundary in
  `client_kernel_resources`: lifecycle, browser history, scroll restoration, and
  work actor are constructed together and can be represented as Context
  services. That makes the remaining shell body more obviously about composing
  actors rather than allocating raw state objects.
- Route service construction now has the same shape. `client_route_services`
  builds route fetching, route preparation, route publication, and build-skew
  reporting from the runtime service context, then lowers them into the
  compatibility shell. Build-skew notification is now an Effect callback too, so
  the reporter no longer hides a synchronous side effect inside its own program.
- Navigation assembly is now out of the compatibility shell too.
  `client_navigation_services` builds prefetch, navigation, revalidation, and
  submission as one dependency-driven service bundle. The shell still owns the
  public compatibility decisions: client redirects are lowered to `navigate`,
  and API-triggered revalidation during boot records the boot revalidation flag.
  The actual actors now compose through runtime, kernel-resource, and route
  service contexts instead of being born directly inside `assemble_kernel`.
- Kernel construction now has a single assembly program in
  `client_kernel_assembly`. The compatibility shell passes callbacks for public
  API behavior and runs one Effect that acquires resources, route services,
  navigation services, and lifecycle finalizers before returning the kernel.
- Kernel acquisition now returns a scoped handle. The shell still lowers the
  acquisition through today's synchronous client factory, but failed boot and
  kernel replacement now close a `Scope` instead of treating lifecycle shutdown
  as a loose callback. That gives future scoped services a real ownership root
  without changing the public API.
- Runtime lifecycle is now backed by Effect `Scope` instead of a hand-rolled
  `Ref` plus finalizer array. Window listeners and actor shutdown finalizers
  still expose the same small compatibility surface, but the lifetime semantics
  are Effect's LIFO close semantics now.
- The Vite HMR route-update bridge is now lifecycle-owned. Installing the global
  browser callback registers a scoped finalizer that clears it when the kernel
  shuts down, while preserving replacement safety if a newer kernel has already
  installed its own handler.
- Browser event listeners now capture the kernel resources owned by their own
  lifecycle instead of looking up the mutable outer current-kernel slot. The
  outer slot remains for the public API methods, but browser callbacks are
  closer to resource-local behavior now.
- Window-focus revalidation now installs its own browser listener through the
  focus service. The compatibility shell still decides whether the feature is
  enabled for this boot, but listener ownership and cleanup live with the Effect
  revalidator.
- Initial route boot is now kernel behavior. The kernel owns setting manual
  scroll restoration, reading the current history position, preparing the boot
  route, and publishing the initial render as one Effect transaction. The shell
  keeps public boot bookkeeping, but it no longer scripts the route pipeline
  step-by-step.
- Popstate handling is now kernel behavior too. The kernel owns adopting the
  browser history position, saving the previous scroll slot, handling hash-only
  movement, cancelling stale revalidation, and dispatching popstate navigation.
  The shell now only attaches the browser event to that kernel effect.
- Reload-scroll persistence is now owned by the scroll restoration service. The
  service installs the `beforeunload` listener through lifecycle shutdown, so
  the shell no longer knows how reload scroll is captured.
- Browser handler installation is now a kernel operation. The compatibility
  shell asks the kernel to install persistent browser handlers instead of
  sequencing popstate, reload-scroll, and HMR wiring itself.
- Boot-time API revalidation is now a small Effect gate backed by `Ref`. API
  requests during boot return the immediate compatibility result while recording
  one post-boot revalidation, and cancelled boots clear that request.
- Provisional boot route state is now an Effect-owned cell. The route preparer
  can publish the boot state for `getRouteState()` without the shell carrying a
  bespoke mutable slot.
- Current-route movement is now kernel behavior. Hash-only movement, optional
  history replacement, scroll intent, history commits, and route-position
  publication moved out of the shell and into the Effect kernel.
- The compatibility pressure test has expanded beyond `create_client_core` into
  split runners for `router.test.ts`, `router_revalidation.test.ts`, and
  `router_submit.test.ts`. Keeping those runners split matters because the
  revalidation suite intentionally installs fake timers; importing all router
  suites into one file polluted unrelated tests.
- The Effect adapter passes the existing client-core, router, revalidation, and
  submit suites through those swap runners. That is 284 existing tests
  exercising the Effect implementation behind the current public client
  contract.
- The broader suites exposed an important boundary question: navigation should
  start promptly, but same-stack observability after `core.navigate(...)` is not
  the public contract. The router tests now assert the real invariant by waiting
  for prompt fetch/work events before resolving any response, instead of
  requiring synchronous same-tick visibility. That let the navigation and
  submission actors keep Effect-native `forkDaemon` scheduling without hidden
  `Effect.runFork` port residue.
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
- Browser APIs now have first-pass Effect ownership for fetch, location,
  history, scroll, browser view, module import, Vite HMR, route DOM side
  effects, and work-indicator timing. Vite HMR now has lifecycle cleanup too.
  These services still need a later `Context` / `Layer` cleanup.
- Work-state emission should be owned by a service instead of being derived
  opportunistically from mutable outer variables.
- Work indicator parity now covers category-level skips, per-operation skips for
  navigation and API requests, mixed app/Vorma work ordering, and renderer
  replacement while work is visible.
- Browser-facing void APIs expose an interesting boundary pressure. Internally
  Effect wants async acknowledgement, but public calls like `stop_prefetch`
  still need immediate observable cancellation, so the service needs explicit
  synchronous ownership of the abort controller.
- Public promise APIs expose a real boundary pressure. Navigation can remain
  promise-shaped, but the desired timing guarantees need to be named instead of
  inherited from the legacy closure. Supersession probably still needs immediate
  signal abortion for already-started loaders and transition hooks, while fetch
  dispatch and work-state visibility should be decided as public compatibility
  behavior rather than smuggled into the Effect services.
- Browser listeners now have explicit lifecycle ownership. The experiment
  removes the previous kernel's focus, popstate, and beforeunload listeners
  through lifecycle services, and the shell asks the kernel to install its
  persistent browser handlers. Popstate runs through the Effect navigation
  actor, scroll restoration is now an Effect service, and hash-only movement is
  represented as route publication rather than fetch work.
- The final client assembly is partly scoped now. Starting the client acquires a
  kernel handle, and shutdown closes that handle. The remaining cleanup is to
  keep moving resources into that acquisition path until lifecycle shutdown is
  just one finalizer among many.
- The new `client_runtime_services`, `client_kernel_resources`,
  `client_route_services`, and `client_navigation_services` Layers should keep
  expanding inward. The next cleanup target is the shell/runtime boundary:
  `create_client_core_effect` should keep shrinking toward adapter logic while
  the Effect acquisition owns more browser resources directly.
- Actors created inside the compatibility shell need daemon or explicit runtime
  ownership. Otherwise fibers created by `Effect.runSync` can be scoped away
  before browser callbacks get to use them.
- Lifecycle finalizers now have cause-level containment. The remaining work is
  to move more browser and actor resources into scoped acquisition instead of
  registering them later from the shell.
- The existing-suite runners are green with prompt-start tests rather than
  same-stack timing tests. The next work should be about making the service
  graph cleaner, not chasing broad parity gaps.

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
