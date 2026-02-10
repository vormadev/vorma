# vormaclient/client Test Rewrite Progress

## Snapshot

- Date: 2026-02-10
- Phase: Phase 1 (contract foundation + iterative migration slices)
- Owner intent: enable safe takeover by any follow-up agent at any point.

## Current Status

- [x] Rewrite plan documented (`TEST_REWRITE_PLAN.md`).
- [x] Contract test area created (`src/contracts/`).
- [x] Minimal contract harness added (`src/contracts/contract_test_harness.ts`).
- [x] First contract slice added (`client.loading_and_focus.contract.test.ts`).
- [x] Contract slice validated in isolation.
- [x] Full `vormaclient/client/src` suite validated with new slice.
- [x] Full legacy inventory created (`TEST_REWRITE_LEGACY_INVENTORY.md`).
- [x] Legacy dedupe + migration completed for loading continuity.
- [x] Prefetch contract slice completed and legacy prefetch suite migrated.
- [x] Link-click contract slice completed and legacy link-click suite migrated.
- [x] Events-system migration completed and legacy events suite migrated.
- [x] Utility-contract slice completed and legacy utility suite migrated.
- [x] Core-navigation mode/programmatic migration completed and legacy suites
      migrated.
- [x] State/revalidation contract slice completed and legacy suites migrated.
- [x] Submit/redirect contract slice completed and legacy suites migrated.
- [x] Error/edge contract slice completed and legacy suites migrated.
- [x] History/init contract slice completed and legacy suites migrated.
- [x] Strict decision ledger created (`TEST_REWRITE_BUG_CANDIDATES.md`).
- [x] Contract guardrails tightened to avoid implementation-coupled assertions.
- [x] Strictness workflow updated: no deferred review queue; fix now or ask the
      user immediately for ambiguous product decisions.
- [x] Previously open BC items resolved with strict assertions and source fixes
      (BC1-BC10).
- [x] Case-level parity ledger created (`TEST_REWRITE_CASE_PARITY.md`) with all
      `194` legacy cases enumerated.
- [x] Generated should/title map created (`TEST_REWRITE_SHOULD_MAP.json`) for
      machine-checkable consolidation and dedupe analysis.
- [x] Case-level parity certified for:
    - `src/client.component_module_loading.test.ts` (`7/7` mapped strict)
    - `src/client.core_navigation.link_click_handling.test.ts` (`5/5` mapped
      strict)
    - `src/client.core_navigation.navigation_types.test.ts` (`6/6` mapped
      strict)
    - `src/client.core_navigation.programmatic_navigation.test.ts` (`1/1` mapped
      strict)
    - `src/client.core_navigation.state_management.test.ts` (`5/5` mapped
      strict)
    - `src/client.critical_edge_cases.test.ts` (`6/6` mapped strict)
    - `src/client.error_handling.test.ts` (`9/9` mapped strict)
    - `src/client.events_system.test.ts` (`14/14` mapped strict)
    - `src/client.navigation_lifecycle.begin_navigation_phase.test.ts` (`4/4`
      mapped strict)
    - `src/client.navigation_lifecycle.complete_navigation_phase.test.ts` (`3/3`
      mapped strict)
    - `src/client.navigation_lifecycle.fetch_route_data_phase.test.ts` (`8/8`
      mapped strict)
    - `src/client.navigation_lifecycle.rerender_app_phase.test.ts` (`11/11`
      mapped strict)
    - `src/client.form_submissions.revalidate_function.test.ts` (`3/3` mapped
      strict)
    - `src/client.form_submissions.submit_function.test.ts` (`19/19` mapped
      strict)
    - `src/client.history_management.test.ts` (`6/6` mapped strict)
    - `src/client.initialization.test.ts` (`9/10` mapped strict, `1/10` dropped
      legacy weakness)
    - `src/client.loading_state_continuity.test.ts` (`7/7` mapped strict)
    - `src/client.prefetching.test.ts` (`8/8` mapped strict)
    - `src/client.redirects.test.ts` (`13/13` mapped strict)
    - `src/client.scroll_restoration.test.ts` (`14/14` mapped strict)
    - `src/client.utility_functions.test.ts` (`6/6` mapped strict)
    - `src/head_elements/head.advanced_updates.test.ts` (`6/6` mapped strict)
    - `src/head_elements/head.basic_operations.test.ts` (`10/10` mapped strict)
    - `src/head_elements/head.minimal_dom_changes.test.ts` (`3/3` mapped strict)
    - `src/head_elements/head.rest_and_edge_cases.test.ts` (`7/7` mapped strict)
    - `src/vorma_ctx/vorma_ctx.test.ts` (`3/3` mapped strict)
- [x] Legacy overlap map completed for every existing legacy file.
- [x] Previously deleted legacy files restored and marked
      `READY_TO_DELETE_AFTER_SIGNOFF` at line 1.
- [x] Physical deletion policy switched to explicit signoff only.

## Process Revision (2026-02-10)

- Legacy files are no longer physically deleted during migration slices.
- If migrated coverage exists, the legacy file remains in tree with first-line
  marker `READY_TO_DELETE_AFTER_SIGNOFF`.
- A file can only be physically deleted after explicit human signoff.
- Strict assertions must remain first-principles; do not add deferred
  relaxations. Fix source immediately or ask the user immediately if ambiguity
  is genuinely unresolved.

## Parity Reality Check (2026-02-10)

- Full legacy suite footprint: `26` files / `194` test cases.
- New contract suite footprint: `14` files / `154` test cases.
- Current mapping is complete at case level across all legacy files.
- Certified as `100%` case-level parity audited:
    - `src/client.component_module_loading.test.ts`
    - `src/client.core_navigation.link_click_handling.test.ts`
    - `src/client.core_navigation.navigation_types.test.ts`
    - `src/client.core_navigation.programmatic_navigation.test.ts`
    - `src/client.core_navigation.state_management.test.ts`
    - `src/client.critical_edge_cases.test.ts`
    - `src/client.error_handling.test.ts`
    - `src/client.events_system.test.ts`
    - `src/client.navigation_lifecycle.begin_navigation_phase.test.ts`
    - `src/client.navigation_lifecycle.complete_navigation_phase.test.ts`
    - `src/client.navigation_lifecycle.fetch_route_data_phase.test.ts`
    - `src/client.navigation_lifecycle.rerender_app_phase.test.ts`
    - `src/client.form_submissions.revalidate_function.test.ts`
    - `src/client.form_submissions.submit_function.test.ts`
    - `src/client.history_management.test.ts`
    - `src/client.initialization.test.ts`
    - `src/client.loading_state_continuity.test.ts`
    - `src/client.prefetching.test.ts`
    - `src/client.redirects.test.ts`
    - `src/client.scroll_restoration.test.ts`
    - `src/client.utility_functions.test.ts`
    - `src/head_elements/head.advanced_updates.test.ts`
    - `src/head_elements/head.basic_operations.test.ts`
    - `src/head_elements/head.minimal_dom_changes.test.ts`
    - `src/head_elements/head.rest_and_edge_cases.test.ts`
    - `src/vorma_ctx/vorma_ctx.test.ts`
- Legacy deletion remains signoff-gated by policy even with completed parity.
- Case-level and title-level mapping artifacts now exist on disk:
    - `TEST_REWRITE_CASE_PARITY.md` (human checklist)
    - `TEST_REWRITE_SHOULD_MAP.json` (machine-readable case/title map)
- Current strict certification milestone: `193/194` legacy cases are explicitly
  certified `mapped-strict` (`1/194` dropped as non-behavioral legacy weakness).

## Completed Scope

Behavior covered by new contract suite:

- Navigation status transitions while in-flight and on completion.
- Submission status transitions using `submit()`.
- Continuous loading invariant across submit -> auto-revalidate.
- Continuous loading invariant with overlapping submissions.
- Continuous loading invariant through soft redirect handoff.
- Continuous loading invariant through redirect chains.
- Global loading indicator start/stop behavior with include filters.
- Window focus revalidation after stale threshold.
- Guard against focus revalidation while navigation is active.
- Cleanup correctness for `revalidateOnWindowFocus` listeners.
- Prefetch eligibility filtering for internal HTTP links.
- Prefetch delay, cancellation, and callback behavior.
- Prefetch reuse on click without refetching.
- Click while prefetch is in-flight, including render callbacks.
- Link click default-prevention behavior for internal/external/modifier clicks.
- Hash-only link behavior without navigation fetch.
- Route-change event payload behavior for hash scroll and title updates.
- Revalidation status event behavior.
- Status event debouncing and duplicate suppression behavior.
- Synchronous status reads via `getStatus()`.
- Location-change event behavior on history key updates.
- Build-ID event payload behavior on navigation build mismatches.
- Listener cleanup semantics and event-target wiring.
- Public utility behavior for `getRootEl`, `getLocation`, `getBuildID`, and
  `__applyScrollState`.
- User navigation fetch behavior with `vorma_json=1`.
- Browser-history (`POP`) navigation fetch behavior.
- Revalidation behavior that avoids history mutation.
- Server redirect-follow behavior for navigation outcomes.
- Prefetch navigation behavior that does not emit loading status.
- Programmatic navigation `replace` semantics.
- Programmatic navigation search/hash override behavior.
- In-flight user-navigation replacement (previous target aborted).
- Mixed active status semantics (`isNavigating` + `isRevalidating` together).
- Revalidation coalescing for rapid repeated calls.
- Revalidation targeting of current `window.location.href` search params.
- Prefetch stop cleanup semantics for pure prefetches.
- Submit dedupe semantics for same vs different dedupe keys.
- Submit body serialization behavior for `FormData`, string, and object bodies.
- Submit auto-revalidation behavior for non-GET vs GET requests.
- Submit internal redirect-follow behavior.
- Redirect accept-header behavior for navigation and submit requests.
- Non-HTTP redirect target ignore behavior.
- Redirect loop max-limit behavior with explicit error logging.
- Submit error-shape behavior for HTTP, network, and abort failures.
- AbortError navigation behavior without error logging.
- Non-abort navigation failure behavior (error logged, page unchanged).
- Empty JSON and non-OK navigation failure recovery behavior.
- Revalidation/prefetch race behavior where stale revalidation does not clobber
  newer navigated content.
- History instance usability and location-event behavior.
- Same-document hash POP scroll behavior.
- Cross-document POP navigation fetch behavior.
- Scroll-state persistence before cross-document movement.
- Init option wiring for error boundary/view transitions and render invocation.
- Init URL cleanup for `vorma_reload` query parameter.
- Init page-refresh scroll restore behavior.
- Init touch-device detection behavior.
- Begin-phase stale prefetch/revalidation abort behavior on new user navigation.
- Begin-phase prefetch dedupe behavior for concurrent same-URL starts.
- Complete-phase loader-gating behavior before route-change dispatch.
- View-transition opt-in behavior for user navigation only.
- Exposed router-data state refresh after successful navigation.
- User-navigation history semantics for push-on-change and replace-on-same-URL.
- Browser-history restored scroll-state payload semantics.
- Document title HTML-entity decoding behavior.
- Stylesheet dedupe behavior across repeated navigations with identical bundles.
- Head section reconciliation behavior for add/update/remove/reorder,
  fingerprint reuse, marker scoping, and invalid block handling.
- Vorma context helper behavior for symbol-backed global reads/writes and safe
  router-data defaults.

Coverage gain in this slice:

- Adds direct tests for previously uncovered
  `src/global_loading_indicator/global_loading_indicator.ts` behavior.
- Adds direct tests for previously uncovered
  `src/window_focus_revalidation/window_focus_revalidation.ts` behavior.
- Adds direct strict contract coverage for `src/head_elements/head_elements.ts`
  reconciliation behavior.
- Adds direct strict contract coverage for `src/vorma_ctx/vorma_ctx.ts`
  symbol-backed context access and router-data shaping.

Resolved strictness decisions:

- BC1 (native GET redirect): treated as a real redirect and followed to target.
- BC2 (native non-GET redirect): treated as a real redirect and followed to
  target.
- BC3 (redirect-limit failure state): fixed; aborted outcomes now explicitly
  delete navigation entries, preventing stuck loading state.
- BC4 (mixed loading status): accepted as intentional; navigating and
  revalidating can be true simultaneously when operations overlap.
- BC5 (failed navigation behavior): fixed as strict contract; failed navigations
  keep current URL/title stable, clear loading state, log once, and preserve
  future operability.
- BC6 (submit error model): fixed as strict contract; errors are exact and
  deterministic (`"500"`, `"Network failure"`, `"Aborted"`).
- BC7 (redirect-limit count): fixed as strict contract; redirect chain caps at
  exactly 10 fetches with explicit "Too many redirects" logging.
- BC8 (submit dedupe handoff): fixed as strict contract; stale aborted submit
  cleanup cannot delete the active replacement submission entry.
- BC9 (redirect buildID ordering): fixed as strict contract; redirect responses
  now update/dispatch build ID before redirect-follow target fetches.
- BC10 (head section text-node reconciliation): fixed as strict contract;
  managed head sections now remove interleaved non-element nodes between
  markers.

## Legacy Mapping (Current)

- Restored legacy files now marked `READY_TO_DELETE_AFTER_SIGNOFF` (signoff
  required before physical deletion):
    - `client.loading_state_continuity.test.ts`
    - `client.component_module_loading.test.ts`
    - `client.navigation_lifecycle.begin_navigation_phase.test.ts`
    - `client.navigation_lifecycle.complete_navigation_phase.test.ts`
    - `client.navigation_lifecycle.fetch_route_data_phase.test.ts`
    - `client.navigation_lifecycle.rerender_app_phase.test.ts`
    - `client.prefetching.test.ts`
    - `client.core_navigation.link_click_handling.test.ts`
    - `client.core_navigation.navigation_types.test.ts`
    - `client.core_navigation.programmatic_navigation.test.ts`
    - `client.core_navigation.state_management.test.ts`
    - `client.form_submissions.submit_function.test.ts`
    - `client.error_handling.test.ts`
    - `client.history_management.test.ts`
    - `client.utility_functions.test.ts`
    - `client.events_system.test.ts`
    - `client.form_submissions.revalidate_function.test.ts`
    - `client.redirects.test.ts`
    - `client.scroll_restoration.test.ts`
    - `client.critical_edge_cases.test.ts`
    - `client.initialization.test.ts`
    - `head_elements/head.basic_operations.test.ts`
    - `head_elements/head.advanced_updates.test.ts`
    - `head_elements/head.minimal_dom_changes.test.ts`
    - `head_elements/head.rest_and_edge_cases.test.ts`
    - `vorma_ctx/vorma_ctx.test.ts`

- Replacement coverage maturity:
    - Explicit case-level parity certification is complete for all legacy files
      (`26/26`) in `TEST_REWRITE_CASE_PARITY.md`.
    - Final case totals are `193` mapped strict and `1` dropped legacy weakness
      (no pending cases).

## Verification Log

- Baseline before new slice:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `26` files, `194` tests, all passing.
- New contract slice:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `5` files, `40` tests, all passing.
- Full suite after adding contract slice:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `26` files, `194` tests, all passing.
- Contract suite after core-navigation migration:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `6` files, `47` tests, all passing.
- Full suite after core-navigation migration:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `25` files, `194` tests, all passing.
- Final confirmation after formatting/docs updates:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `6` files, `47` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `25` files, `194` tests, all passing.
- Contract suite after state/revalidation migration:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `7` files, `52` tests, all passing.
- Full suite after state/revalidation migration:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `24` files, `191` tests, all passing.
- Final confirmation after this slice docs/format updates:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `7` files, `52` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `24` files, `191` tests, all passing.
- Contract suite after submit/redirect migration:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `8` files, `62` tests, all passing.
- Full suite after submit/redirect migration:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `23` files, `169` tests, all passing.
- Final confirmation after submit/redirect docs/format updates:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `8` files, `62` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `23` files, `169` tests, all passing.
- Contract suite after error/edge migration:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `9` files, `67` tests, all passing.
- Full suite after error/edge migration:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `22` files, `159` tests, all passing.
- Final confirmation after error/edge docs/format updates:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `9` files, `67` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `22` files, `159` tests, all passing.
- Contract suite after history/init migration:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `10` files, `76` tests, all passing.
- Full suite after history/init migration:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `21` files, `152` tests, all passing.
- Final confirmation after history/init docs/format updates:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `10` files, `76` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `21` files, `152` tests, all passing.
- Strict-resolution validation (BC1-BC7):
    - `pnpm vitest --run vormaclient/client/src/contracts/client.submit_and_redirect.contract.test.ts`
    - Result: `11` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src/contracts/client.navigation_modes.contract.test.ts`
    - Result: `8` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src/contracts/client.error_and_edge.contract.test.ts`
    - Result: `5` tests, all passing.
- Full contract suite after strict-resolution updates:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `11` files, `83` tests, all passing.
- Full client suite after strict-resolution updates:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `37` files, `277` tests, all passing.
- Final confirmation after strict-process docs + formatting updates:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `11` files, `83` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `37` files, `277` tests, all passing.
- Generated should/title map artifact:
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` generated with `194` legacy cases,
      `194` legacy `should` titles, and `88` contract cases (`0` contract
      `should` titles under current naming).
- Contract suite after module-loading/fetch parity additions:
    - `pnpm vitest --run vormaclient/client/src/contracts/client.module_loading_and_fetch.contract.test.ts`
    - Result: `10` tests, all passing.
- Full contract suite after module-loading/fetch parity additions:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `11` files, `88` tests, all passing.
- Full client suite after module-loading/fetch parity additions:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `37` files, `282` tests, all passing.
- Lifecycle contract suite after begin/complete/rerender parity additions:
    - `pnpm vitest --run vormaclient/client/src/contracts/client.navigation_lifecycle.contract.test.ts`
    - Result: `1` file, `11` tests, all passing.
- Full contract suite after lifecycle parity additions:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `99` tests, all passing.
- Full client suite after lifecycle parity additions:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `293` tests, all passing.
- Refreshed should/title map artifact:
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` regenerated with `194` legacy
      cases, `194` legacy `should` titles, and `99` contract cases (`0` contract
      `should` titles under current naming).
- Prefetch contract suite after completed-prefetch cleanup addition:
    - `pnpm vitest --run vormaclient/client/src/contracts/client.prefetch.contract.test.ts`
    - Result: `1` file, `9` tests, all passing.
- Full contract suite after core-navigation case-level parity certification:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `100` tests, all passing.
- Full client suite after core-navigation case-level parity certification:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `294` tests, all passing.
- Refreshed should/title map artifact after core-navigation parity updates:
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` regenerated with `194` legacy
      cases, `194` legacy `should` titles, and `100` contract cases (`0`
      contract `should` titles under current naming).
- Final confirmation after formatting and doc synchronization:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `100` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `294` tests, all passing.
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` confirms `194` legacy cases and
      `100` contract cases.
- Targeted strict-gap verification for error-boundary and failure-state
  additions:
    - `pnpm vitest --run vormaclient/client/src/contracts/client.module_loading_and_fetch.contract.test.ts`
    - Result: `1` file, `11` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src/contracts/client.error_and_edge.contract.test.ts`
    - Result: `1` file, `6` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src/contracts/client.events.contract.test.ts`
    - Result: `1` file, `3` tests, all passing.
- Full contract suite after events/error/critical parity updates:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `103` tests, all passing.
- Full client suite after events/error/critical parity updates:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `297` tests, all passing.
- Refreshed should/title map artifact after events/error/critical parity
  updates:
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` regenerated with `194` legacy
      cases, `194` legacy `should` titles, and `103` contract cases (`0`
      contract `should` titles under current naming).
- Final post-format verification:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `103` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `297` tests, all passing.
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` summary confirms `legacyCases=194`,
      `contractCases=103`.
- Form/history parity verification before submit dedupe-handoff fix:
    - `pnpm vitest --run vormaclient/client/src/contracts/client.state_and_revalidation.contract.test.ts vormaclient/client/src/contracts/client.submit_and_redirect.contract.test.ts vormaclient/client/src/contracts/client.loading_and_focus.contract.test.ts vormaclient/client/src/contracts/client.history_and_init.contract.test.ts`
    - Result: `4` files, `51` tests; `1` failing strict test
      (`keeps submitting state active through same-key dedupe handoff`).
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `109` tests; same strict failure reproduced.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `303` tests; same strict failure reproduced.
- Source fix for submit dedupe-handoff continuity:
    - `src/client.ts`: guarded submission cleanup so only the submission entry
      that currently owns a dedupe key can delete it.
- Re-verification after BC8 fix:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `109` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `303` tests, all passing.
- Refreshed should/title map artifact after form/history parity mapping and BC8
  fix:
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` regenerated with `legacyCases=194`,
      `legacyShouldCases=194`, `contractCases=109`, `contractShouldCases=0`.
- Final verification after formatting/doc synchronization:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `109` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `303` tests, all passing.
- Targeted history/init verification after adding init parity assertions:
    - `pnpm vitest --run vormaclient/client/src/contracts/client.history_and_init.contract.test.ts`
    - Result: `1` file, `14` tests, all passing.
- Full suites after initialization/loading/prefetch parity certification:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `113` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `307` tests, all passing.
- Refreshed should/title map artifact after initialization/loading/prefetch
  parity certification:
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` regenerated with `legacyCases=194`,
      `legacyShouldCases=194`, `contractCases=113`, `contractShouldCases=0`.
- Final verification after formatting/doc synchronization for this tranche:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `113` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `307` tests, all passing.
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` confirms `legacyCases=194`,
      `contractCases=113`.
- Targeted strict-gap verification for redirects and scroll restoration:
    - `pnpm vitest --run vormaclient/client/src/contracts/client.submit_and_redirect.contract.test.ts`
    - Result: `1` file, `18` tests; `1` failing strict test
      (`updates build ID and dispatches event before following navigation redirects`).
    - `pnpm vitest --run vormaclient/client/src/contracts/client.history_and_init.contract.test.ts`
    - Result: `1` file, `21` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src/contracts/client.loading_and_focus.contract.test.ts`
    - Result: `1` file, `21` tests, all passing.
- Source fix for BC9 redirect buildID ordering:
    - `src/client.ts`: redirect-outcome path now persists new build ID and
      dispatches build-id event before redirect effectuation.
- Re-verification after BC9 fix:
    - `pnpm vitest --run vormaclient/client/src/contracts/client.submit_and_redirect.contract.test.ts`
    - Result: `1` file, `18` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `123` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `317` tests, all passing.
- Refreshed should/title map artifact after redirects/scroll/utilities parity
  certification:
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` regenerated with `legacyCases=194`,
      `legacyShouldCases=194`, `contractCases=123`, `contractShouldCases=0`.
- Final post-format verification for redirects/scroll/utilities tranche:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `12` files, `123` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `38` files, `317` tests, all passing.
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` confirms `legacyCases=194`,
      `contractCases=123`.
- Targeted head/vorma parity verification before BC10 source fix:
    - `pnpm vitest --run vormaclient/client/src/contracts/head_elements.contract.test.ts vormaclient/client/src/contracts/vorma_ctx.contract.test.ts vormaclient/client/src/head_elements/head.basic_operations.test.ts vormaclient/client/src/head_elements/head.advanced_updates.test.ts vormaclient/client/src/head_elements/head.minimal_dom_changes.test.ts vormaclient/client/src/head_elements/head.rest_and_edge_cases.test.ts vormaclient/client/src/vorma_ctx/vorma_ctx.test.ts`
    - Result: `7` files, `60` tests; `2` failing strict tests (text-node
      reconciliation in managed head sections).
- Source fix for BC10 head section reconciliation:
    - `src/head_elements/head_elements.ts`: removal pass now deletes non-element
      nodes between section markers before positioning reconciled elements.
- Re-verification after BC10 fix:
    - `pnpm vitest --run vormaclient/client/src/contracts/head_elements.contract.test.ts vormaclient/client/src/contracts/vorma_ctx.contract.test.ts vormaclient/client/src/head_elements/head.basic_operations.test.ts vormaclient/client/src/head_elements/head.advanced_updates.test.ts vormaclient/client/src/head_elements/head.minimal_dom_changes.test.ts vormaclient/client/src/head_elements/head.rest_and_edge_cases.test.ts vormaclient/client/src/vorma_ctx/vorma_ctx.test.ts`
    - Result: `7` files, `60` tests, all passing.
- Full client suite after head/vorma parity completion:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `40` files, `348` tests, all passing.
- Full contract suite after head/vorma parity completion:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `14` files, `154` tests, all passing.
- Refreshed should/title map artifact after full parity completion:
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` regenerated with `legacyCases=194`,
      `legacyShouldCases=194`, `contractCases=154`, `contractShouldCases=0`.
- Final post-format verification for head/vorma parity tranche:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - Result: `14` files, `154` tests, all passing.
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `40` files, `348` tests, all passing.
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` confirms `legacyCases=194`,
      `contractCases=154`.
- Legacy marker consistency audit:
    - Verified first-line marker compliance for every legacy file listed in
      `TEST_REWRITE_LEGACY_INVENTORY.md`.
    - Added missing `READY_TO_DELETE_AFTER_SIGNOFF` markers to:
        - `src/client.navigation_lifecycle.begin_navigation_phase.test.ts`
        - `src/client.navigation_lifecycle.complete_navigation_phase.test.ts`
        - `src/client.navigation_lifecycle.rerender_app_phase.test.ts`
- Full client suite after marker consistency fix:
    - `pnpm vitest --run vormaclient/client/src`
    - Result: `40` files, `348` tests, all passing.
- Refreshed should/title map after marker line-number sync:
    - `node vormaclient/client/scripts/generate_test_case_maps.mjs`
    - Result: `TEST_REWRITE_SHOULD_MAP.json` regenerated with `legacyCases=194`,
      `legacyShouldCases=194`, `contractCases=154`, `contractShouldCases=0`.

## Safe Takeover Protocol

1. Read `TEST_REWRITE_PLAN.md` first.
2. Read `TEST_REWRITE_PROGRESS.md` top-to-bottom.
3. Read `TEST_REWRITE_LEGACY_INVENTORY.md` to pick the next slice.
4. Read `TEST_REWRITE_BUG_CANDIDATES.md` for resolved strict decisions and prior
   fixes.
5. Never physically delete `READY_TO_DELETE_AFTER_SIGNOFF` files without
   explicit signoff.
6. If any behavior is truly ambiguous, ask the user immediately before adding or
   relaxing assertions.
7. Re-run verification commands before making additional edits.
8. Continue with next slice only after updating checklist + log + inventory.
9. Re-run `node vormaclient/client/scripts/generate_test_case_maps.mjs` after
   any test-title additions/changes.

## Next Safe Step

- Keep legacy suites in-tree with `READY_TO_DELETE_AFTER_SIGNOFF` markers until
  explicit human signoff authorizes physical deletion.
- Use `src/contracts/` as the acceptance gate for upcoming large client
  refactors; run both:
    - `pnpm vitest --run vormaclient/client/src/contracts`
    - `pnpm vitest --run vormaclient/client/src`
- If refactor work introduces a behavior ambiguity, stop and ask the user
  immediately before altering strict assertions.
