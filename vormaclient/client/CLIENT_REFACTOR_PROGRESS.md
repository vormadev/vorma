# vormaclient/client Major Refactor Progress

## Snapshot

- Date: 2026-02-10
- Phase: Monolith decomposition (ongoing)
- Contract gate status: passing

## Completed This Pass

1. Extracted navigation runtime out of `src/client.ts`

- Added `src/navigation_runtime/manager.ts`.
- Moved the full navigation state machine and lifecycle behavior into
  `createNavigationStateManager()` in the new module.
- Kept behavior-compatible semantics for:
    - navigation dedupe/upgrade (`userNavigation`/`prefetch`/`revalidation`)
    - submit dedupe and auto-revalidation
    - status event debouncing and dedupe
    - redirect/buildID sequencing
    - prefetch/non-prefetch loading behavior

2. Kept skip-server-fetch logic split and reused

- `src/navigation_runtime/skip_server_fetch.ts` remains the source of skip
  checks.
- Navigation fetch pipeline consumes `canSkipServerFetch()` instead of
  re-implementing skip checks.

3. Turned `src/client.ts` into a thin public facade

- `src/client.ts` now wires singleton creation and exports public API only.
- Re-exported navigation/runtime types from `src/navigation_runtime/types.ts` to
  preserve consumer-facing type imports.
- Preserved `navigationStateManager` singleton export and
  `setNavigationStateAccess()` wiring.
- Preserved `getLastTriggeredNavOrRevalidateTimestampMS()` semantics via
  callback hook from manager creation.

4. Reduced type coupling to `client.ts`

- `src/navigation_state_access.ts` now depends on
  `src/navigation_runtime/types.ts` instead of `client.ts`.
- `src/redirects/redirects.ts` and `src/rendering.ts` type imports now point to
  `src/navigation_runtime/types.ts`.

5. Extracted navigation fetch/outcome builder from manager

- Added `src/navigation_runtime/fetch_route_data.ts`.
- Moved route-data fetch, client-only outcome construction, and parallel
  loader/bootstrap logic into that module.
- `src/navigation_runtime/manager.ts` now focuses on orchestration/state
  transitions and delegates fetch construction to the extracted module.

6. Extracted submission pipeline from manager

- Added `src/navigation_runtime/submit.ts`.
- Moved submit flow (dedupe, request wiring, redirect handling, buildID update,
  auto-revalidate trigger, and cleanup semantics) into the new module.
- `manager.ts` now delegates submit behavior via `executeSubmit(...)` while
  preserving status continuity and strict contracts.

7. Extracted successful-navigation lifecycle application pipeline

- Added `src/navigation_runtime/process_successful_navigation.ts`.
- Moved successful navigation application flow (buildID/module-map updates,
  loader/CSS waits, render gating, and finish/cleanup transitions) into the new
  module.
- `manager.ts` keeps orchestration hooks (`transitionPhase`,
  `findNavigationEntry`, `deleteNavigation`) and delegates the lifecycle
  application to the extracted module.

8. Extracted navigation slot bookkeeping and status signaling

- Added `src/navigation_runtime/navigation_bookkeeping.ts`.
- Added `src/navigation_runtime/status_signaler.ts`.
- Moved active/prefetch/revalidation slot bookkeeping, lookup/delete helpers,
  phase transitions, and clear-all abort semantics into
  `createNavigationBookkeeping(...)`.
- Moved debounced status dispatch + duplicate suppression logic into
  `createStatusSignaler(...)`.
- `manager.ts` now delegates these responsibilities and keeps orchestration
  behavior.

9. Extracted navigation-outcome handling switch

- Added `src/navigation_runtime/handle_navigation_outcome.ts`.
- Moved `navigate()` outcome branching (`aborted`/`redirect`/`success`) into the
  new helper while preserving redirect buildID ordering and prefetch semantics.
- `manager.ts` now delegates outcome resolution to this module.
- Fixed a regression during extraction by awaiting delegated outcome handling
  inside `navigate()` so async lifecycle errors remain catchable at the manager
  boundary.

10. Extracted navigation entry construction

- Added `src/navigation_runtime/navigation_entry_factory.ts`.
- Moved active/prefetch/revalidation entry construction into dedicated factory
  helpers.
- `manager.ts` now delegates entry construction while keeping slot orchestration
  and begin-phase decisions.
- Preserved strict cleanup behavior, including stable target URL usage for
  error-path deletion of active navigations.

11. Extracted begin-phase navigation decisions

- Added `src/navigation_runtime/begin_navigation.ts`.
- Moved begin-phase decision logic (`beginUserNavigation`, `beginPrefetch`,
  `beginRevalidation`) into dedicated helpers.
- `manager.ts` now delegates begin-phase policy through context wiring while
  retaining orchestration ownership.

12. Extracted begin-navigation context assembly

- Added `src/navigation_runtime/context.ts`.
- Moved begin-phase context object assembly into a dedicated helper.
- `manager.ts` now composes begin-phase helpers through explicit context
  construction instead of inline object assembly.

13. Reduced begin-phase context wiring duplication

- `manager.ts` now creates one composed begin-navigation context instance and
  reuses it across begin-phase calls.
- Preserved behavior while removing repeated context construction code.

14. Extracted runtime composition from manager

- Added `src/navigation_runtime/runtime.ts`.
- Moved the full runtime composition and orchestration internals from
  `manager.ts` into `runtime.ts`.
- Reduced `src/navigation_runtime/manager.ts` to a thin adapter that keeps
  public naming (`createNavigationStateManager`) while delegating to
  `createNavigationRuntime(...)`.

15. Extracted navigation control creation orchestration

- Added `src/navigation_runtime/navigation_controls.ts`.
- Moved active/prefetch/revalidation control construction + error-path cleanup
  wiring into dedicated control builders.
- `runtime.ts` now composes controls via `createNavigationControls(...)` and
  passes them into begin-phase policy wiring.

16. Extracted runtime API orchestration from runtime composition

- Added `src/navigation_runtime/runtime_api.ts`.
- Moved runtime API method orchestration (`navigate`, `beginNavigation`,
  `submit`, `processSuccessfulNavigation`, plus bookkeeping passthrough methods)
  into `createNavigationRuntimeAPI(...)`.
- `runtime.ts` now focuses on composition/dependency assembly and delegates API
  shape behavior to `runtime_api.ts`.

17. Switched client facade to the runtime factory directly

- `src/client.ts` now imports `createNavigationRuntime` from
  `src/navigation_runtime/runtime.ts` directly.
- `src/navigation_runtime/manager.ts` remains as a compatibility adapter for
  external imports that still reference `createNavigationStateManager(...)`.

18. Extracted successful-navigation side effects

- Added `src/navigation_runtime/successful_navigation_effects.ts`.
- Moved build-matched response artifact application (client module-map updates
  and immediate CSS activation) into
  `applyResponseArtifactsWhenBuildMatches(...)`.
- Moved build-ID synchronization and event dispatch into
  `syncBuildIDFromResponse(...)`.
- `src/navigation_runtime/process_successful_navigation.ts` now focuses on
  lifecycle orchestration (phase transitions, revalidation guards, loader/CSS
  waits, render handoff, and cleanup), delegating side-effect clusters to the
  new helper.

19. Extracted navigation dispatch/orchestration helpers

- Added `src/navigation_runtime/navigate.ts`.
- Moved navigation-type dispatch branching into
  `beginNavigationWithHandlers(...)`.
- Moved high-level navigate flow (`begin`, await outcome, resolve outcome, and
  abort cleanup fallback) into `navigateWithHandlers(...)`.
- `src/navigation_runtime/runtime_api.ts` now delegates begin/navigate
  orchestration to this module and remains focused on wiring runtime
  dependencies.

20. Extracted runtime bookkeeping adapter assembly

- Added `src/navigation_runtime/bookkeeping_adapter.ts`.
- Moved runtime-facing bookkeeping passthrough assembly (`transitionPhase`,
  `find/delete/remove/get/has`, size/list, `clearAll`) into
  `createNavigationBookkeepingAdapter(...)`.
- `src/navigation_runtime/runtime_api.ts` now consumes the adapter and focuses
  on begin/navigate/submit/successful-navigation wiring.

21. Extracted runtime submit wiring

- Added `src/navigation_runtime/runtime_submit.ts`.
- Moved runtime-side submit assembly into `createRuntimeSubmit(...)`.
- `src/navigation_runtime/runtime_api.ts` now delegates submit wiring to that
  adapter and no longer assembles submit context inline.

22. Extracted runtime begin-navigation wiring

- Added `src/navigation_runtime/runtime_begin_navigation.ts`.
- Moved runtime-side begin-navigation assembly (user/prefetch/revalidation
  dispatch + active navigation fallback) into
  `createRuntimeBeginNavigation(...)`.
- `src/navigation_runtime/runtime_api.ts` now delegates begin-navigation wiring
  to that adapter.

23. Extracted runtime successful-navigation wiring

- Added `src/navigation_runtime/runtime_process_successful_navigation.ts`.
- Moved runtime-side successful-navigation assembly into
  `createRuntimeProcessSuccessfulNavigation(...)`.
- `src/navigation_runtime/runtime_api.ts` now delegates successful-navigation
  wiring to that adapter.

24. Extracted runtime navigate wiring

- Added `src/navigation_runtime/runtime_navigate.ts`.
- Moved runtime-side navigate assembly into `createRuntimeNavigate(...)`.
- `src/navigation_runtime/runtime_api.ts` now delegates navigate wiring to that
  adapter.

25. Extracted client-only fetch fast-path builder

- Added `src/navigation_runtime/client_only_outcome.ts`.
- Moved client-only fast-path outcome construction into
  `buildClientOnlyOutcome(...)`.
- `src/navigation_runtime/fetch_route_data.ts` now delegates the skip-fetch
  success path to that helper.

26. Extracted fetch server result handling + parallel loader startup

- Added `src/navigation_runtime/fetch_route_data_server.ts`.
- Moved server fetch/json parsing into `createServerRouteDataPromise(...)`.
- Moved server response resolution (aborted/redirect/error/success split) into
  `resolveServerRouteDataResult(...)`.
- Added `src/navigation_runtime/parallel_client_loaders.ts`.
- Moved match-based parallel client-loader startup into
  `startParallelClientLoaders(...)`.
- `src/navigation_runtime/fetch_route_data.ts` now orchestrates these helpers
  while preserving existing phase/order semantics.

27. Extracted successful server fetch outcome assembly

- Added `src/navigation_runtime/server_success_outcome.ts`.
- Moved successful server outcome assembly (module preloads, loader completion
  wiring, and CSS preload promise setup) into `buildServerSuccessOutcome(...)`.
- `src/navigation_runtime/fetch_route_data.ts` now delegates successful outcome
  construction to that helper.

28. Extracted fetch request preparation + skip gating

- Added `src/navigation_runtime/fetch_route_data_request.ts`.
- Moved skip-server-fetch eligibility/outcome gating into
  `getClientOnlyOutcomeIfSkippable(...)`.
- Moved request URL preparation (`vorma_json` + conditional `dpl`) into
  `buildRouteDataRequestURL(...)`.
- `src/navigation_runtime/fetch_route_data.ts` now orchestrates those request
  helpers.

29. Extracted submit request wiring and build-ID sync reuse

- Added `src/navigation_runtime/submit_request.ts`.
- Moved submit request-init/deployment-header wiring into
  `buildSubmitRequestInit(...)`.
- Moved submit request execution wiring into `executeSubmitRequest(...)`.
- `src/navigation_runtime/submit.ts` now reuses `syncBuildIDFromResponse(...)`
  from `src/navigation_runtime/successful_navigation_effects.ts` instead of
  inline build-ID update logic.

30. Extracted submit response decision flow

- Added `src/navigation_runtime/submit_response.ts`.
- Moved submit response handling (non-ok handling, redirect effectuation, JSON
  parse path, and auto-revalidation decision) into
  `finalizeSubmitResponse(...)`.
- `src/navigation_runtime/submit.ts` now delegates response decision flow to the
  new helper while keeping submission lifecycle ownership.
- Preserved original try/catch behavior by awaiting the helper
  (`return await finalizeSubmitResponse(...)`) so downstream parse errors remain
  mapped to submit failure results rather than escaping uncaught.

31. Extracted navigation slot query/snapshot helpers

- Added `src/navigation_runtime/navigation_slots.ts`.
- Moved slot-query and snapshot logic into focused helpers:
  `findNavigationEntryInSlots(...)`, `getNavigationsSizeFromSlots(...)`, and
  `buildNavigationsMapFromSlots(...)`.
- `src/navigation_runtime/navigation_bookkeeping.ts` now uses an explicit
  `slots` state object and delegates those query/snapshot responsibilities to
  the new helper module.

32. Extracted navigation slot mutation/state-transition helpers

- Added `src/navigation_runtime/navigation_slot_mutations.ts`.
- Moved slot mutation and transition behavior into focused helpers:
  `deleteNavigationFromSlots(...)`, `transitionNavigationPhaseInSlots(...)`, and
  `clearSlotsAndSubmissions(...)`.
- `src/navigation_runtime/navigation_bookkeeping.ts` now delegates those
  mutation/transition responsibilities to the new helper module while keeping
  public bookkeeping API behavior unchanged.

33. Extracted skip-server-fetch context + type definitions

- Added `src/navigation_runtime/skip_server_fetch_types.ts`.
- Moved skip-check context/result type definitions into the new type module.
- Added `src/navigation_runtime/skip_server_fetch_context.ts`.
- Moved skip-check context assembly/early prerequisite checks into
  `buildSkipCheckContext(...)`.
- `src/navigation_runtime/skip_server_fetch.ts` now focuses on skip-decision
  rule evaluation and consumes the extracted context helper.

34. Extracted skip-server-fetch rule grouping

- Added `src/navigation_runtime/skip_server_fetch_rules.ts`.
- Moved skip eligibility rule checks into `isSkipEligibilityViolated(...)`.
- Moved skip-result assembly into `buildSkipResultFromContext(...)`.
- Reduced `src/navigation_runtime/skip_server_fetch.ts` to a thin orchestrator
  that composes context-building + rule evaluation + result assembly.

35. Extracted navigation-bookkeeping state/core composition

- Added `src/navigation_runtime/navigation_slot_state_access.ts`.
- Added `src/navigation_runtime/navigation_bookkeeping_core.ts`.
- Moved slot state accessor methods into `createNavigationSlotStateAccess(...)`.
- Moved bookkeeping core operations (`find/delete/remove/get/has/size/map`,
  phase transition, and clear-all) into `createNavigationBookkeepingCore(...)`.
- Reduced `src/navigation_runtime/navigation_bookkeeping.ts` to composition of
  slot state + core operations.

36. Extracted submit lifecycle helpers

- Added `src/navigation_runtime/submit_lifecycle.ts`.
- Moved submit lifecycle mechanics (active-submission creation, dedupe-aware
  registration, and ownership-safe cleanup) into `createActiveSubmission(...)`,
  `beginSubmissionLifecycle(...)`, and `finishSubmissionLifecycle(...)`.
- `src/navigation_runtime/submit.ts` now composes those lifecycle helpers with
  request/response helpers and remains behavior-compatible.

37. Split begin-navigation flows into dedicated modules

- Added `src/navigation_runtime/begin_navigation_user.ts`.
- Added `src/navigation_runtime/begin_navigation_prefetch.ts`.
- Added `src/navigation_runtime/begin_navigation_revalidation.ts`.
- `src/navigation_runtime/begin_navigation.ts` now serves as shared context/type
  surface and delegates each flow to its focused module.

38. Split navigation-control creation flows

- Added `src/navigation_runtime/navigation_control_active.ts`.
- Added `src/navigation_runtime/navigation_control_prefetch.ts`.
- Added `src/navigation_runtime/navigation_control_revalidation.ts`.
- `src/navigation_runtime/navigation_controls.ts` now composes these focused
  control-creation modules and remains the public factory surface.

39. Split successful-navigation lifecycle helpers

- Added `src/navigation_runtime/process_successful_navigation_revalidation.ts`.
- Added `src/navigation_runtime/process_successful_navigation_wait.ts`.
- Added `src/navigation_runtime/process_successful_navigation_render.ts`.
- Moved stale revalidation checks into `isStaleRevalidationEntry(...)` and
  reused it across both pre-wait and pre-render guards.
- Moved loader/CSS wait behavior into `waitForSuccessfulNavigationAssets(...)`.
- Moved rendering-phase handoff + completion/error transition behavior into
  `renderSuccessfulNavigation(...)`.
- `src/navigation_runtime/process_successful_navigation.ts` now orchestrates the
  successful-navigation lifecycle through those helpers.

40. Extracted link-click outcome handling helper

- Added `src/link_navigation_outcome.ts`.
- Moved `__makeLinkOnClickFn(...)` outcome handling
  (`aborted`/`redirect`/`success`) into `handleLinkNavigationOutcome(...)`.
- `src/links.ts` now delegates outcome branching to this helper and remains
  focused on click eligibility/prefetch orchestration.

41. Extracted link-prefetch and hash-change helpers

- Added `src/link_prefetch_handlers.ts`.
- Added `src/link_hash_change.ts`.
- Moved `__getPrefetchHandlers(...)` behavior into
  `createPrefetchHandlers(...)`.
- Moved hash-only navigation detection into `isJustAHashChange(...)`.
- `src/links.ts` now acts as a thin public facade over these link helpers.

42. Split redirect request + response parsing helpers

- Added `src/redirects/redirect_request_init.ts`.
- Added `src/redirects/redirect_response_parsing.ts`.
- Moved redirect-request `RequestInit` construction into
  `buildRedirectRequestInit(...)`.
- Moved response redirect-header/redirected-url parsing into
  `parseResponseForRedirectData(...)`.
- `src/redirects/redirects.ts` now delegates request-init + parse flow to these
  helpers while preserving existing public exports and behavior.

43. Split redirect effectuation helpers

- Added `src/redirects/redirect_effectuation.ts`.
- Moved redirect/revalidation navigation cleanup into
  `cleanupRedirectRelatedNavigations(...)`.
- Moved hard-redirect effectuation into `effectuateHardRedirect(...)`.
- Moved soft-redirect effectuation into `effectuateSoftRedirect(...)`.
- `src/redirects/redirects.ts` now delegates redirect effectuation and cleanup
  to this helper module.

44. Extracted client-loader execution core

- Added `src/client_loader_execution.ts`.
- Moved client-loader execution internals from `src/client_loaders.ts` into
  `executeClientLoaders(...)` with the same execution semantics (parallel loader
  start, abort cascading, and first true-error handling).
- Moved client-loader execution types (`PartialWaitFnJSON`,
  `ClientLoadersResult`) into the extracted module.
- `src/client_loaders.ts` now focuses on public orchestration APIs and
  re-exports `ClientLoadersResult`.

45. Extracted runtime status computation helper

- Added `src/navigation_runtime/navigation_status.ts`.
- Moved runtime status derivation (`isNavigating`, `isSubmitting`,
  `isRevalidating`) into `computeNavigationStatus(...)`.
- `src/navigation_runtime/runtime.ts` now delegates status computation to this
  helper and remains focused on runtime composition wiring.

46. Split init-client setup helpers

- Added `src/init_client_module_map.ts`.
- Added `src/init_client_manifest.ts`.
- Added `src/init_client_options.ts`.
- Moved initial client-module-map population into
  `initializeClientModuleMapFromInitialRouteState(...)`.
- Moved progressive route-manifest loading/registration into
  `loadRouteManifestProgressively(...)`.
- Moved default error-boundary/view-transition option application into
  `applyInitClientOptions(...)`.
- `src/init_client.ts` now acts as an orchestration surface over these setup
  helpers.

47. Extracted init-client event wiring helpers

- Added `src/init_client_events.ts`.
- Moved before-unload scroll-state persistence listener setup into
  `registerBeforeUnloadScrollStatePersistence(...)`.
- Moved touch-device detection listener setup into
  `registerTouchDetection(...)`.
- `src/init_client.ts` now delegates event wiring to these helper functions
  while preserving original registration ordering.

48. Extracted history POP navigation helper

- Added `src/history/history_pop_navigation.ts`.
- Moved hash-driven same-document POP scroll restoration behavior into
  `handlePopNavigationForHistoryUpdate(...)` helper internals.
- Moved cross-document POP navigation attempt + reload fallback into
  `handlePopNavigationForHistoryUpdate(...)`.
- `src/history/history.ts` now delegates POP-specific navigation handling to the
  helper and remains focused on history listener orchestration/state tracking.

49. Extracted runtime operation assembly helper

- Added `src/navigation_runtime/runtime_operations.ts`.
- Moved runtime operation assembly (`processSuccessfulNavigation`,
  `beginNavigation`, `navigate`, `submit`) out of
  `src/navigation_runtime/runtime_api.ts` into `createRuntimeOperations(...)`.
- `src/navigation_runtime/runtime_api.ts` now focuses on bookkeeping adapter
  wiring and public API surface assembly.

50. Extracted runtime begin-context setup helper

- Added `src/navigation_runtime/runtime_begin_context_setup.ts`.
- Moved begin-context setup wiring (control creation + begin-context assembly)
  out of `src/navigation_runtime/runtime.ts` into
  `createRuntimeBeginContextSetup(...)`.
- `src/navigation_runtime/runtime.ts` now focuses on runtime composition with
  slimmer status-signaling and API-factory wiring.

51. Extracted link click-handler helper

- Added `src/link_click_handler.ts`.
- Moved `__makeLinkOnClickFn(...)` click-flow orchestration into
  `createLinkOnClickFn(...)`.
- `src/links.ts` now acts as a thin façade over `createPrefetchHandlers(...)`
  and `createLinkOnClickFn(...)`.

52. Extracted history state/singleton helper

- Added `src/history/history_state.ts`.
- Moved browser-history singleton state management (`instance`,
  `lastKnownLocation`) into the new helper.
- `src/history/history.ts` now delegates instance/location state access to
  `getHistoryInstance(...)`, `getLastKnownHistoryLocation(...)`, and
  `setLastKnownHistoryLocation(...)`.

53. Extracted client-loader partial-match helper

- Added `src/client_loader_partial_matches.ts`.
- Moved partial route-pattern match lookup logic from
  `findPartialMatchesOnClient(...)` into `findClientLoaderPartialMatches(...)`.
- `src/client_loaders.ts` now delegates partial-match lookup to the helper and
  remains focused on client-loader orchestration APIs.

54. Extracted init-client URL cleanup helper

- Added `src/init_client_url_cleanup.ts`.
- Moved hard-reload query-param cleanup (`VORMA_HARD_RELOAD_QUERY_PARAM`)
  handling out of `src/init_client.ts` into `cleanupHardReloadQueryParam(...)`.
- `src/init_client.ts` now delegates URL cleanup and stays focused on init
  orchestration sequence.

55. Extracted redirect request-flow helper

- Added `src/redirects/redirect_request_flow.ts`.
- Moved max-redirect guard + request execution flow from `handleRedirects(...)`
  into `executeRedirectRequestFlow(...)`.
- `src/redirects/redirects.ts` now delegates request-flow orchestration to the
  helper and remains focused on redirect parsing/effectuation API behavior.

56. Extracted init-client bootstrap helper

- Added `src/init_client_bootstrap.ts`.
- Moved initial runtime bootstrap sequence (initial component load, client
  loader setup, and error-boundary component handling) from `src/init_client.ts`
  into `bootstrapInitialClientRuntime(...)`.
- `src/init_client.ts` now delegates bootstrap sequencing and remains focused on
  top-level init orchestration.

57. Extracted runtime API surface helper

- Added `src/navigation_runtime/runtime_api_surface.ts`.
- Moved `NavigationStateManager` return-object assembly from
  `src/navigation_runtime/runtime_api.ts` into `createRuntimeAPISurface(...)`.
- `src/navigation_runtime/runtime_api.ts` now focuses on runtime wiring and
  delegates public-surface assembly to the helper.

58. Extracted runtime bookkeeping/status setup helper

- Added `src/navigation_runtime/runtime_bookkeeping_status_setup.ts`.
- Moved runtime bookkeeping creation + status signal wiring from
  `src/navigation_runtime/runtime.ts` into
  `createRuntimeBookkeepingStatusSetup(...)`.
- `src/navigation_runtime/runtime.ts` now delegates bookkeeping/status setup and
  remains focused on runtime composition orchestration.

59. Extracted head-element candidate/fingerprint helpers

- Added `src/head_elements/head_element_fingerprint.ts`.
- Added `src/head_elements/head_element_candidates.ts`.
- Moved element fingerprint construction into `createElementFingerprint(...)`.
- Moved block-to-element construction/dedup logic into
  `buildDedupedElementsFromBlocks(...)`.
- Moved current-element fingerprint map construction into
  `buildCurrentElementsMap(...)`.
- Moved new-vs-current element match selection into
  `matchElementsByFingerprint(...)`.
- `src/head_elements/head_elements.ts` now delegates these pure helpers while
  preserving reconciliation behavior.

60. Extracted head-element reconciliation/mutation helpers

- Added `src/head_elements/head_element_reconcile.ts`.
- Moved current/new element reconciliation selection into
  `reconcileHeadElements(...)`.
- Moved stale managed-node removal pass into `removeStaleManagedNodes(...)`.
- Moved ordered placement pass into `placeReconciledHeadElements(...)`.
- `src/head_elements/head_elements.ts` now orchestrates reconciliation through
  dedicated helper calls and remains behavior-compatible.

61. Extracted runtime constants module

- Added `src/navigation_runtime/constants.ts`.
- Moved `REVALIDATION_COALESCE_MS` constant out of
  `src/navigation_runtime/runtime.ts` into the new constants module.
- `src/navigation_runtime/runtime.ts` now imports the constant and remains
  focused on composition wiring.

62. Extracted head comment marker helper

- Added `src/head_elements/head_comment_markers.ts`.
- Moved `getStartAndEndComments(...)` + comment tree-walk lookup logic out of
  `src/head_elements/head_elements.ts` into the new helper module.
- Re-exported `getStartAndEndComments(...)` from
  `src/head_elements/head_elements.ts` to preserve existing import behavior.

63. Extracted rendering history/scroll helper

- Added `src/rendering_history_scroll.ts`.
- Moved history push/replace side effects and scroll-state dispatch derivation
  out of `src/rendering.ts` into `runHistoryAndDeriveScrollState(...)`.
- `src/rendering.ts` now delegates history/scroll branch handling and remains a
  top-level render orchestration module.

64. Extracted rendering global-state apply helper

- Added `src/rendering_state_apply.ts`.
- Moved route-data global state assignment keys + loop out of `src/rendering.ts`
  into `applyRouteDataToGlobalState(...)`.
- `src/rendering.ts` now delegates the state-application block and remains
  focused on render lifecycle sequencing.

65. Extracted component-loader selection helpers

- Added `src/component_loader_selection.ts`.
- Moved active component selection from module map into
  `buildActiveComponents(...)`.
- Moved error-boundary resolution selection into
  `resolveErrorBoundaryComponent(...)`.
- `src/component_loader.ts` now delegates selection logic and remains focused on
  module loading + state writes.

66. Extracted component-loader import helper

- Added `src/component_loader_imports.ts`.
- Moved dedupe + dynamic import module-map construction from
  `src/component_loader.ts` into `loadComponentModules(...)`.
- `src/component_loader.ts` now delegates import-map loading and remains focused
  on load orchestration and state updates.

67. Extracted scroll-state storage helpers

- Added `src/scroll_state_types.ts`.
- Added `src/scroll_state_storage.ts`.
- Moved session storage map read/write/FIFO eviction logic out of
  `src/scroll_state_manager.ts` into `saveStoredScrollState(...)` and
  `getStoredScrollState(...)`.
- `src/scroll_state_manager.ts` now delegates storage operations while
  preserving public exports (`scrollStateManager`, `__applyScrollState`,
  `saveScrollState`, and `ScrollState`).

68. Extracted rendering document-update helpers

- Added `src/rendering_document_updates.ts`.
- Moved document title update + HTML entity decode block out of
  `src/rendering.ts` into `applyRouteDocumentTitle(...)`.
- Moved conditional head element update branching out of `src/rendering.ts` into
  `applyRouteHeadElements(...)`.
- `src/rendering.ts` now delegates document/head mutation blocks and remains
  focused on render flow orchestration.

69. Fixed prefetch-stop target parity + extracted target href helper

- Added `src/link_prefetch_target_href.ts`.
- Prefetch start and prefetch stop now both derive target href via
  `buildPrefetchTargetHref(...)`, including `search` and `hash` overrides.
- Added strict contract coverage in
  `src/contracts/client.prefetch.contract.test.ts`:
  `aborts in-flight prefetch with search/hash overrides when stop is called`.
- This removes a first-principles correctness gap where `stop()` could fail to
  abort an active prefetch when override query/hash changed the navigation key.

70. Extracted scroll-state page-refresh helpers

- Added `src/scroll_state_refresh_state.ts`.
- Moved page-refresh save snapshot behavior into
  `savePageRefreshScrollStateSnapshot(...)`.
- Moved page-refresh restore + freshness gating behavior into
  `restoreRecentPageRefreshScrollState(...)`.
- `src/scroll_state_manager.ts` now delegates page-refresh persistence/restore
  while preserving its existing public API.

71. Extracted init-client pattern-registry helper

- Added `src/init_client_pattern_registry.ts`.
- Moved loader matcher registry construction + global assignment out of
  `src/init_client.ts` into `initializeClientPatternRegistry(...)`.
- `src/init_client.ts` now delegates registry setup and remains focused on
  top-level init sequencing.

72. Extracted vorma app-helper runtime modules

- Added `src/vorma_app_helpers/path_resolution.ts`.
- Added `src/vorma_app_helpers/url_build.ts`.
- Added `src/vorma_app_helpers/body_resolution.ts`.
- Moved API path resolution internals out of
  `src/vorma_app_helpers/vorma_app_helpers.ts` into `resolveVormaPath(...)`.
- Moved API URL construction internals out of
  `src/vorma_app_helpers/vorma_app_helpers.ts` into `buildVormaURL(...)`.
- Moved request body normalization internals out of
  `src/vorma_app_helpers/vorma_app_helpers.ts` into
  `resolveVormaRequestBody(...)`.
- Preserved existing public exports (`buildQueryURL`, `buildMutationURL`,
  `resolveBody`, and `__resolvePath`) by delegating through wrappers.

73. Hardened prefetch timer cleanup semantics

- Refactored `src/link_prefetch_handlers.ts` to use a shared
  `clearPendingTimer()` helper for timer cleanup in both `stop()` and
  `onClick()` flows.
- Fixed timer cleanup guards to use explicit `undefined` checks rather than
  truthy checks, so timer id `0` is still cleared correctly.
- Added strict contract coverage in
  `src/contracts/client.prefetch.contract.test.ts`:
  `clears pending prefetch timer even when timer id is zero`.

74. Extracted component-loader effective-error helper

- Added `src/component_loader_error_data.ts`.
- Moved effective server/client error index selection logic out of
  `src/component_loader.ts` into `getEffectiveErrorData(...)`.
- Updated `src/client_loaders.ts` to consume the helper directly.
- Preserved compatibility by re-exporting `getEffectiveErrorData(...)` from
  `src/component_loader.ts`.

75. Extracted history listener prelude helper

- Added `src/history/history_listener_prelude.ts`.
- Moved custom history-listener prelude decisions (location key-change
  detection, same-document POP detection, and scroll-save gating) out of
  `src/history/history.ts` into `analyzeHistoryListenerPrelude(...)`.
- `src/history/history.ts` now delegates these prelude calculations and remains
  focused on side effects + POP handling orchestration.

76. Extracted redirect HTTP target-resolution helper

- Added `src/redirects/redirect_href_resolution.ts`.
- Moved repeated redirect target URL + HTTP href-details resolution out of
  `src/redirects/redirect_response_parsing.ts` into
  `resolveHTTPRedirectTarget(...)`.
- `src/redirects/redirect_response_parsing.ts` now delegates shared target
  parsing while preserving existing redirect priority and strategy semantics.

77. Extracted client-loader global snapshot helper

- Added `src/client_loader_snapshot.ts`.
- Moved global client-loader execution snapshot assembly out of
  `src/client_loaders.ts` into `buildClientLoaderSnapshotFromGlobal(...)`.
- `src/client_loaders.ts` now delegates snapshot construction while preserving
  setup, execution, and state-write semantics.

78. Extracted runtime navigation-operations wiring helper

- Added `src/navigation_runtime/runtime_navigation_operations.ts`.
- Moved begin/process/navigate operation wiring out of
  `src/navigation_runtime/runtime_operations.ts` into
  `createRuntimeNavigationOperations(...)`.
- `src/navigation_runtime/runtime_operations.ts` now delegates navigation
  operation assembly and remains focused on top-level runtime operation
  composition (including submit wiring).

79. Extracted runtime API operation-wiring helper

- Added `src/navigation_runtime/runtime_api_wiring.ts`.
- Moved runtime API operation-wiring assembly out of
  `src/navigation_runtime/runtime_api.ts` into `createRuntimeAPIWiring(...)`.
- `src/navigation_runtime/runtime_api.ts` now delegates operation wiring and
  remains focused on bookkeeping adapter + surface composition.

80. Extracted skip-server-fetch eligibility helper

- Added `src/navigation_runtime/skip_server_fetch_eligibility.ts`.
- Moved skip eligibility/change-detection logic out of
  `src/navigation_runtime/skip_server_fetch_rules.ts` into
  `isSkipEligibilityViolated(...)`.
- `src/navigation_runtime/skip_server_fetch_rules.ts` now focuses on skip-result
  construction and re-exports eligibility checks for compatibility.

81. Extracted runtime target-url normalization helper

- Added `src/navigation_runtime/target_url.ts`.
- Moved repeated navigation target URL normalization
  (`new URL(props.href, window.location.href).href`) from runtime modules into
  `resolveNavigationTargetURL(...)`.
- Updated runtime modules to consume the shared helper:
  `src/navigation_runtime/handle_navigation_outcome.ts`,
  `src/navigation_runtime/navigation_control_revalidation.ts`,
  `src/navigation_runtime/navigation_entry_factory.ts`,
  `src/navigation_runtime/navigation_control_active.ts`, and
  `src/navigation_runtime/navigate.ts`.

82. Extracted client-loader execution helpers

- Added `src/client_loader_promise_wrapping.ts`.
- Added `src/client_loader_result_processing.ts`.
- Moved child-aborting loader promise wrapper logic out of
  `src/client_loader_execution.ts` into `wrapLoaderPromisesWithChildAbort(...)`.
- Moved settled loader result processing + first-error selection out of
  `src/client_loader_execution.ts` into
  `processSettledClientLoaderResults(...)`.
- `src/client_loader_execution.ts` now focuses on loader orchestration and
  delegates wrapping/result handling details.

83. Extracted prefetch navigation manager helper

- Added `src/link_prefetch_navigation.ts`.
- Moved prefetch navigation start wiring out of `src/link_prefetch_handlers.ts`
  into `startPrefetchNavigation(...)`.
- Moved idle-prefetch abort/remove logic out of `src/link_prefetch_handlers.ts`
  into `abortIdlePrefetchNavigation(...)`.
- `src/link_prefetch_handlers.ts` now focuses on event/timer orchestration and
  reuses a single computed prefetch target href per handler instance.

84. Extracted client-loader state mutation helper

- Added `src/client_loader_state.ts`.
- Moved client-loader global-state writes out of `src/client_loaders.ts` into
  `setClientLoadersState(...)` and `deriveAndSetErrorState(...)`.
- `src/client_loaders.ts` now focuses on loader execution flow + matcher
  integration and re-exports state helper functions for compatibility.

85. Simplified client-loader setup flow wiring

- Removed the single-use `runWaitFns(...)` indirection from
  `src/client_loaders.ts`.
- `setupClientLoaders(...)` now delegates directly to
  `executeClientLoaders(...)` while preserving behavior.

86. Extracted prefetch click-path helper

- Added `src/link_prefetch_click.ts`.
- Moved prefetch click-path eligibility checks, hash-only fast path, callback
  sequencing, and navigation invocation out of `src/link_prefetch_handlers.ts`
  into `handlePrefetchClick(...)`.
- `src/link_prefetch_handlers.ts` now focuses on prefetch timer/state
  orchestration and delegates click-path behavior.

87. Extracted shared prefetch callback types

- Added `src/link_prefetch_callbacks.ts`.
- Moved shared prefetch callback type definitions out of
  `src/link_prefetch_handlers.ts` and `src/link_prefetch_click.ts` into
  `LinkOnClickCallbacks`/`LinkOnClickCallback`.
- Prefetch runtime modules now consume one callback-type source.

88. Added oxlint gate and fixed outstanding warnings

- Added `pnpm oxlint vormaclient/client/src` to the active validation gate for
  each `vormaclient/client` refactor slice.
- Enforced refactor scope as `vormaclient/client` only.

89. Extracted skip-server-fetch result item helper

- Added `src/navigation_runtime/skip_server_fetch_result_item.ts`.
- Moved per-pattern skip-result item assembly out of
  `src/navigation_runtime/skip_server_fetch_rules.ts` into
  `buildSkipResultItem(...)`.
- `src/navigation_runtime/skip_server_fetch_rules.ts` now focuses on ordered
  match iteration + aggregate result construction.

90. Added TypeScript gate and fixed client TS errors

- Added `pnpm tsgo --noEmit --project ./vormaclient/client` to the active
  validation gate for each refactor slice.
- Added `src/import_meta.d.ts` so `ImportMeta.env`/`ImportMeta.hot` usages are
  typed in the client package.
- Fixed strict typing issues in client runtime/tests, including:
  `src/navigation_runtime/submit.ts`,
  `src/redirects/redirect_href_resolution.ts`,
  `src/contracts/contract_test_harness.ts`,
  `src/contracts/client.error_and_edge.contract.test.ts`, and
  `src/contracts/client.history_and_init.contract.test.ts`.

91. Fixed `resolveVormaRequestBody` strict `tsc` incompatibility

- Updated `src/vorma_app_helpers/body_resolution.ts` to normalize
  `ArrayBufferView<ArrayBufferLike>` values into `BodyInit`-compatible payloads
  under strict DOM typings.
- Added explicit handling for SharedArrayBuffer-backed views by cloning into an
  ArrayBuffer-backed `Uint8Array`.
- Added `pnpm tsc --noEmit --project ./vormaclient/client` to the active
  validation gate alongside `tsgo`.

## Verification

- `pnpm oxlint vormaclient/client/src`
- Result: `0` warnings, `0` errors.
- `pnpm tsgo --noEmit --project ./vormaclient/client`
- Result: passing.
- `pnpm tsc --noEmit --project ./vormaclient/client`
- Result: passing.
- `pnpm vitest --run vormaclient/client/src/contracts`
- Result: `14` files, `156` tests, all passing.
- `pnpm vitest --run vormaclient/client/src`
- Result: `40` files, `350` tests, all passing.

## Current State

- Runtime production code is class-free.
- `src/client.ts` is no longer a large monolith for navigation internals.
- Navigation runtime is isolated under `src/navigation_runtime/`.
- Fetch/outcome construction is isolated in
  `src/navigation_runtime/fetch_route_data.ts`.
- Submission flow is isolated in `src/navigation_runtime/submit.ts`.
- Successful-navigation lifecycle application is isolated in
  `src/navigation_runtime/process_successful_navigation.ts`.
- Navigation slot bookkeeping is isolated in
  `src/navigation_runtime/navigation_bookkeeping.ts`.
- Status signaling is isolated in `src/navigation_runtime/status_signaler.ts`.
- Navigation outcome resolution is isolated in
  `src/navigation_runtime/handle_navigation_outcome.ts`.
- Navigation entry construction is isolated in
  `src/navigation_runtime/navigation_entry_factory.ts`.
- Begin-phase navigation policy is isolated in
  `src/navigation_runtime/begin_navigation.ts`.
- Begin-phase context assembly is isolated in
  `src/navigation_runtime/context.ts`.
- Runtime composition is isolated in `src/navigation_runtime/runtime.ts`.
- Control construction is isolated in
  `src/navigation_runtime/navigation_controls.ts`.
- Runtime API orchestration is isolated in
  `src/navigation_runtime/runtime_api.ts`.
- Navigation dispatch/orchestration helpers are isolated in
  `src/navigation_runtime/navigate.ts`.
- Runtime bookkeeping passthrough assembly is isolated in
  `src/navigation_runtime/bookkeeping_adapter.ts`.
- Runtime submit wiring is isolated in
  `src/navigation_runtime/runtime_submit.ts`.
- Runtime begin-navigation wiring is isolated in
  `src/navigation_runtime/runtime_begin_navigation.ts`.
- Runtime successful-navigation wiring is isolated in
  `src/navigation_runtime/runtime_process_successful_navigation.ts`.
- Runtime navigate wiring is isolated in
  `src/navigation_runtime/runtime_navigate.ts`.
- `src/client.ts` now consumes `createNavigationRuntime(...)` directly.
- Successful-navigation side effects are isolated in
  `src/navigation_runtime/successful_navigation_effects.ts`.
- Client-only fetch fast-path outcome construction is isolated in
  `src/navigation_runtime/client_only_outcome.ts`.
- Fetch server result handling is isolated in
  `src/navigation_runtime/fetch_route_data_server.ts`.
- Parallel client-loader startup is isolated in
  `src/navigation_runtime/parallel_client_loaders.ts`.
- Successful server fetch outcome assembly is isolated in
  `src/navigation_runtime/server_success_outcome.ts`.
- Fetch request preparation and skip-gating are isolated in
  `src/navigation_runtime/fetch_route_data_request.ts`.
- Submit request wiring is isolated in
  `src/navigation_runtime/submit_request.ts`.
- Submit response decision flow is isolated in
  `src/navigation_runtime/submit_response.ts`.
- Navigation slot query/snapshot helpers are isolated in
  `src/navigation_runtime/navigation_slots.ts`.
- Navigation slot mutation/state-transition helpers are isolated in
  `src/navigation_runtime/navigation_slot_mutations.ts`.
- Navigation bookkeeping state/core composition helpers are isolated in
  `src/navigation_runtime/navigation_slot_state_access.ts` and
  `src/navigation_runtime/navigation_bookkeeping_core.ts`.
- Submit lifecycle helpers are isolated in
  `src/navigation_runtime/submit_lifecycle.ts`.
- Begin-navigation flow modules are isolated in
  `src/navigation_runtime/begin_navigation_user.ts`,
  `src/navigation_runtime/begin_navigation_prefetch.ts`, and
  `src/navigation_runtime/begin_navigation_revalidation.ts`.
- Navigation-control creation flows are isolated in
  `src/navigation_runtime/navigation_control_active.ts`,
  `src/navigation_runtime/navigation_control_prefetch.ts`, and
  `src/navigation_runtime/navigation_control_revalidation.ts`.
- Successful-navigation lifecycle helpers are isolated in
  `src/navigation_runtime/process_successful_navigation_revalidation.ts`,
  `src/navigation_runtime/process_successful_navigation_wait.ts`, and
  `src/navigation_runtime/process_successful_navigation_render.ts`.
- Skip-server-fetch context/type definitions are isolated in
  `src/navigation_runtime/skip_server_fetch_context.ts` and
  `src/navigation_runtime/skip_server_fetch_types.ts`.
- Skip-server-fetch rule grouping is isolated in
  `src/navigation_runtime/skip_server_fetch_rules.ts`.
- Skip-server-fetch eligibility/change checks are isolated in
  `src/navigation_runtime/skip_server_fetch_eligibility.ts`.
- Skip-server-fetch per-match result-item assembly is isolated in
  `src/navigation_runtime/skip_server_fetch_result_item.ts`.
- Link-click outcome branching is isolated in `src/link_navigation_outcome.ts`.
- Link-prefetch and hash-change behavior are isolated in
  `src/link_prefetch_handlers.ts`, `src/link_prefetch_navigation.ts`,
  `src/link_prefetch_click.ts`, `src/link_prefetch_callbacks.ts`, and
  `src/link_hash_change.ts`.
- Link click-handler flow is isolated in `src/link_click_handler.ts`.
- Redirect request-init + response parsing are isolated in
  `src/redirects/redirect_request_init.ts` and
  `src/redirects/redirect_response_parsing.ts`.
- Redirect target URL + HTTP href-details parsing is isolated in
  `src/redirects/redirect_href_resolution.ts`.
- Redirect request-flow orchestration is isolated in
  `src/redirects/redirect_request_flow.ts`.
- Redirect effectuation + cleanup are isolated in
  `src/redirects/redirect_effectuation.ts`.
- Client-loader execution helper modules are isolated in
  `src/client_loader_execution.ts`, `src/client_loader_promise_wrapping.ts`, and
  `src/client_loader_result_processing.ts`.
- Client-loader state mutation helpers are isolated in
  `src/client_loader_state.ts`.
- Client-loader partial-match lookup is isolated in
  `src/client_loader_partial_matches.ts`.
- Client-loader global snapshot assembly is isolated in
  `src/client_loader_snapshot.ts`.
- Runtime status computation is isolated in
  `src/navigation_runtime/navigation_status.ts`.
- Init-client setup helpers are isolated in `src/init_client_module_map.ts`,
  `src/init_client_manifest.ts`, and `src/init_client_options.ts`.
- Init-client event wiring is isolated in `src/init_client_events.ts`.
- Init-client hard-reload URL cleanup is isolated in
  `src/init_client_url_cleanup.ts`.
- Init-client runtime bootstrap sequencing is isolated in
  `src/init_client_bootstrap.ts`.
- History POP navigation handling is isolated in
  `src/history/history_pop_navigation.ts`.
- History singleton/state handling is isolated in
  `src/history/history_state.ts`.
- Runtime operation assembly is isolated in
  `src/navigation_runtime/runtime_operations.ts`.
- Runtime begin/process/navigate operation wiring is isolated in
  `src/navigation_runtime/runtime_navigation_operations.ts`.
- Runtime target URL normalization is isolated in
  `src/navigation_runtime/target_url.ts`.
- Runtime begin-context setup is isolated in
  `src/navigation_runtime/runtime_begin_context_setup.ts`.
- Runtime API surface assembly is isolated in
  `src/navigation_runtime/runtime_api_surface.ts`.
- Runtime API operation wiring is isolated in
  `src/navigation_runtime/runtime_api_wiring.ts`.
- Runtime bookkeeping/status setup is isolated in
  `src/navigation_runtime/runtime_bookkeeping_status_setup.ts`.
- Runtime constants are isolated in `src/navigation_runtime/constants.ts`.
- Head-element fingerprint/candidate helpers are isolated in
  `src/head_elements/head_element_fingerprint.ts` and
  `src/head_elements/head_element_candidates.ts`.
- Head-element reconciliation/mutation helpers are isolated in
  `src/head_elements/head_element_reconcile.ts`.
- Head comment marker lookup is isolated in
  `src/head_elements/head_comment_markers.ts`.
- Render-time history/scroll branch handling is isolated in
  `src/rendering_history_scroll.ts`.
- Render-time route-data global-state application is isolated in
  `src/rendering_state_apply.ts`.
- Component selection + error-boundary resolution helpers are isolated in
  `src/component_loader_selection.ts`.
- Component dynamic import-map loading is isolated in
  `src/component_loader_imports.ts`.
- Component-loader effective error selection is isolated in
  `src/component_loader_error_data.ts`.
- History-listener prelude calculations are isolated in
  `src/history/history_listener_prelude.ts`.
- Scroll-state type + storage helpers are isolated in
  `src/scroll_state_types.ts` and `src/scroll_state_storage.ts`.
- Render-time document title/head update helpers are isolated in
  `src/rendering_document_updates.ts`.
- Prefetch target href derivation is isolated in
  `src/link_prefetch_target_href.ts`.
- Scroll page-refresh persistence/restore helpers are isolated in
  `src/scroll_state_refresh_state.ts`.
- Init-client pattern-registry setup is isolated in
  `src/init_client_pattern_registry.ts`.
- Vorma app-helper runtime URL/path/body helpers are isolated in
  `src/vorma_app_helpers/path_resolution.ts`,
  `src/vorma_app_helpers/url_build.ts`, and
  `src/vorma_app_helpers/body_resolution.ts`.
- `src/navigation_runtime/manager.ts` is now a thin adapter over runtime
  composition.
- Strict contract suite remains green.
- Legacy migration/parity ledgers remain in place and unchanged in policy.

## Next High-Leverage Steps

1. Keep `src/navigation_runtime/manager.ts` as compatibility-only surface while
   internal code paths use `createNavigationRuntime(...)` directly.
2. Continue decomposition only where it materially improves cohesion in
   medium-sized modules (`runtime.ts`, `runtime_api.ts`, `links.ts`,
   `redirects.ts`, `client_loaders.ts`, `init_client.ts`, `history.ts`) without
   changing semantics.
3. Continue running both suites after every structural slice.

## Safe Takeover Notes

- Start with `CLIENT_REFACTOR_PLAN.md`, then this file.
- Treat `src/contracts/` as the authoritative behavior gate.
- Preserve public exports in `index.ts` and public type exports in
  `src/client.ts`.
- Run both suites after each slice:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - `pnpm vitest --run vormaclient/client/src`
