# Normative Intent Mining Ledger

Status: Active  
Last Updated: 2026-02-08  
Baseline Commit: `446e64a`

This file is the authoritative tracker for what has and has not been mined for
normative intent.

## Status Legend

- `VERIFIED`: Explicitly re-read line-by-line after introducing this ledger and
  reconciled into spec/checklist/issue artifacts.
- `HISTORICAL`: Mined before this ledger existed (documented in
  `SPECS_CHECKLIST.md`), but not yet replay-verified under this explicit
  ledger workflow.
- `PENDING`: Not yet mined to completion under the current workflow.
- `OUT-OF-SCOPE`: Not normative implementation behavior for Vorma runtime
  contracts (for example, test harness scaffolding helpers).

## Exhaustive-Signoff Gates

Before claiming specs are exhaustive:

1. All in-scope rows must be `VERIFIED` (no `PENDING`, no `HISTORICAL`).
2. Open conformance issues in `VORMA_CONFORMANCE_ISSUES.md` must be explicitly
   dispositioned (normative, bug, or intentional divergence policy).
3. Any spec updates discovered during mining must be reflected in:
   `VORMA_*_SPEC.md` + `VORMA_TRACEABILITY_MATRIX.md` + `SPECS_CHECKLIST.md`.

## A) Backend Runtime Source (`vormaruntime/*.go`)

- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/errors.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/get_deps.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/get_root_handler.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/glue.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/gmpd.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/paths.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/route_registry.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/route_reload.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/ssr.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/types.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/vite_url.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/vorma_core.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormaruntime/vorma_init.go`

## B) Frontend Legacy Tests (Truth-Mining Corpus)

All files below are legacy/pre-conformance behavior sources.

- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.component_module_loading.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.core_navigation.link_click_handling.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.core_navigation.navigation_types.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.core_navigation.programmatic_navigation.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.core_navigation.state_management.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.critical_edge_cases.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.error_handling.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.events_system.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.form_submissions.revalidate_function.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.form_submissions.submit_function.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.history_management.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.initialization.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.loading_state_continuity.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.navigation_lifecycle.begin_navigation_phase.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.navigation_lifecycle.complete_navigation_phase.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.navigation_lifecycle.fetch_route_data_phase.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.navigation_lifecycle.rerender_app_phase.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.prefetching.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.redirects.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.scroll_restoration.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.utility_functions.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/head_elements/head.advanced_updates.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/head_elements/head.basic_operations.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/head_elements/head.minimal_dom_changes.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/head_elements/head.rest_and_edge_cases.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/vorma_ctx/vorma_ctx.test.ts`

## C) Frontend Runtime Source (`client/src`, non-test)

- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/asset_manager.ts`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.test.helpers.ts` | Test harness helper, not runtime contract.
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client_loaders.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/component_loader.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/error_boundary.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/events.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/global_loading_indicator/global_loading_indicator.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/hard_reload.ts`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/head_elements/head.test.helpers.ts` | Test helper.
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/head_elements/head_elements.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/history/history.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/history/npm_history_types.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/hmr/hmr.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/init_client.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/links.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/redirects/redirects.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/rendering.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/resolve_public_href.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/scroll_state_manager.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/static_route_defs/route_def_helpers.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/ui_lib_impl_helpers/link_components.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/ui_lib_impl_helpers/route_components.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/ui_lib_impl_helpers/typed_navigate.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/utils/errors.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/utils/logging.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/vorma_app_helpers/vorma_app_helpers.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/vorma_ctx/vorma_ctx.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/window_focus_revalidation/window_focus_revalidation.ts`

## D) TS Adapter + Public Package Source

- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/react/index.tsx`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/react/src/helpers.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/react/src/link.tsx`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/react/src/react.tsx`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/react/tsconfig.json`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/preact/index.tsx`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/preact/src/helpers.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/preact/src/link.tsx`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/preact/src/preact.tsx`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/preact/tsconfig.json`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/solid/index.tsx`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/solid/src/helpers.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/solid/src/link.tsx`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/solid/src/solid.tsx`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/solid/tsconfig.json`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/vite/vite.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/vite/tsconfig.json`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/create/main.ts` | Create-CLI scaffolding/distribution tool source; not Vorma runtime/build/dev-server contract.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/create/package.json` | Create-CLI package metadata; not Vorma runtime/build/dev-server contract.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/create/pnpm-lock.yaml` | Package-manager lock artifact; no direct Vorma runtime/API contract.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/create/tsconfig.json` | Create-CLI toolchain config; not Vorma runtime/build/dev-server contract.

## E) Build Packaging Scripts (`internal/scripts/buildts`, Out-of-Scope)

- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/scripts/buildts/main.go` | TS package/distribution build pipeline; excluded from Vorma runtime/build/dev-server conformance scope.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/scripts/buildts/build-solid.mjs` | TS package/distribution helper script; excluded from Vorma runtime/build/dev-server conformance scope.

## F) Wave Tooling Source (`wave/tooling/*.go`)

- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/broadcast.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/builder.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/cli.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/css.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/devserver.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/events.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/hash.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/lock.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/lock_unix.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/lock_windows.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/schema.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/static.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/url.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/tooling/watcher.go`

## G) Inherited Backend Dependency Source (`kit/*`, Vorma-visible only)

### G.1) `kit/mux` (router + nested mux + task middleware inheritance)

- `VERIFIED` | `/Users/sjc/__code/river/kit/mux/mux.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/mux/nested_mux.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/mux/mux_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/mux/mux_advanced_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/mux/nested_mux_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/mux/README.md` | Documentation only.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/mux/bench.txt` | Benchmark artifact.

### G.2) `kit/matcher` (segment parsing + best/nested matching inheritance)

- `VERIFIED` | `/Users/sjc/__code/river/kit/matcher/matcher.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/matcher/register.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/matcher/parse_segments.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/matcher/find_best_match.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/matcher/find_nested_matches.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/matcher/parse_segments_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/matcher/find_best_match_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/matcher/find_nested_matches_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/matcher/README.md` | Documentation only.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/matcher/bench.txt` | Benchmark artifact.

### G.3) `kit/response` (proxy merge + redirect/header/status inheritance)

- `VERIFIED` | `/Users/sjc/__code/river/kit/response/response.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/response/proxy.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/response/response_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/response/proxy_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/response/README.md` | Documentation only.

### G.4) `kit/headels` (head dedupe/sort/render inheritance)

- `VERIFIED` | `/Users/sjc/__code/river/kit/headels/headblocks.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/headels/headblocks_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/headels/README.md` | Documentation only.

### G.5) `kit/validate` (query/action parse + validation inheritance)

- `VERIFIED` | `/Users/sjc/__code/river/kit/validate/validate.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/validate/rules.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/validate/search_params.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/validate/error_collector.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/validate/validate_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/validate/rules_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/validate/search_params_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/validate/error_collector_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/validate/error_acc_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/validate/more_error_collector_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/validate/README.md` | Documentation only.

### G.6) `kit/tasks` (task runtime + input/memoization inheritance)

- `VERIFIED` | `/Users/sjc/__code/river/kit/tasks/tasks.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/tasks/tasks_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/tasks/tasks_bench_test.go` | Benchmark harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/tasks/README.md` | Documentation only.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/tasks/bench.txt` | Benchmark artifact.

### G.7) Additional Backend Utility Inheritance (`kit/*`)

- `VERIFIED` | `/Users/sjc/__code/river/kit/htmlutil/htmlutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/htmlutil/htmlutil_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/htmlutil/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/envutil/envutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/envutil/envutil_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/envutil/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/netutil/netutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/netutil/netutil_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/netutil/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/reflectutil/reflectutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/reflectutil/reflectutil_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/reflectutil/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/middleware/middleware.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/README.md` | Not used by core Vorma runtime paths.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/etag/etag.go` | App-level middleware package; not used by core Vorma runtime paths.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/etag/etag_test.go` | App-level middleware package tests.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/etag/README.md` | Documentation only.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/healthcheck/healthcheck.go` | App-level middleware package; not used by core Vorma runtime paths.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/healthcheck/healthcheck_test.go` | App-level middleware package tests.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/healthcheck/README.md` | Documentation only.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/robotstxt/robotstxt.go` | App-level middleware package; not used by core Vorma runtime paths.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/robotstxt/robotstxt_test.go` | App-level middleware package tests.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/robotstxt/README.md` | Documentation only.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/secureheaders/secureheaders.go` | App-level middleware package; not used by core Vorma runtime paths.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/secureheaders/secureheaders_test.go` | App-level middleware package tests.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/middleware/secureheaders/README.md` | Documentation only.

### G.8) Remaining Utility Dependencies Discovered by Import Reconciliation

- `VERIFIED` | `/Users/sjc/__code/river/kit/bytesutil/bytesutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/bytesutil/bytesutil_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/bytesutil/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/cryptoutil/cryptoutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/cryptoutil/cryptoutil_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/cryptoutil/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/fsutil/fsutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/fsutil/fsutil_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/fsutil/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/executil/executil.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/executil/executil_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/executil/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/id/id.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/id/id_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/id/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/lru/lru.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/lru/lru_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/lru/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/colorlog/colorlog.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/colorlog/colorlog_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/colorlog/README.md` | Documentation only.

### G.9) Transitive Utility Dependencies (Late-Reconciliation Closure)

- `VERIFIED` | `/Users/sjc/__code/river/kit/contextutil/contextutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/contextutil/contextutil_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/contextutil/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/genericsutil/genericsutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/genericsutil/genericsutil_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/genericsutil/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/grace/grace.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/grace/grace_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/grace/README.md` | Documentation only.
- `VERIFIED` | `/Users/sjc/__code/river/kit/set/set.go`
- `VERIFIED` | `/Users/sjc/__code/river/kit/set/set_test.go`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/set/README.md` | Documentation only.

## H) Inherited Frontend Dependency Source (`kit/_typescript/*`, Vorma-visible only)

### H.1) `kit/_typescript/matcher` (`vorma/kit/matcher/*`)

- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/register.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/parse_segments.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/find_best_match.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/find_nested_matches.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/parse_segments.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/find_best_match.main_cases.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/find_best_match.additional_scenarios.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/find_nested_matches.main_cases.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/find_nested_matches.additional_scenarios.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/find_nested_matches.trailing_slash_behavior.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/matcher/find_nested_matches.partial_matching_with_gaps.test.ts`
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/_typescript/matcher/find_best_match.test.helpers.ts` | Test helper.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/_typescript/matcher/find_nested_matches.test.helpers.ts` | Test helper.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/_typescript/matcher/matcher.bench.ts` | Benchmark harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/kit/_typescript/matcher/bench.txt` | Benchmark artifact.

### H.2) `kit/_typescript/url` (`vorma/kit/url`)

- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/url/url.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/url/url.test.ts`

### H.3) `kit/_typescript/json` (`vorma/kit/json`)

- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/json/json.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/json/deep_equals.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/json/stringify_stable.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/json/search_param_serializer.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/json/deep_equals.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/json/stringify_stable.test.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/json/search_params_serializer.test.ts`

### H.4) `kit/_typescript/debounce` (`vorma/kit/debounce`)

- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/debounce/debounce.ts`
- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/debounce/debounce.test.ts`

### H.5) `kit/_typescript/listeners` (`vorma/kit/listeners`)

- `VERIFIED` | `/Users/sjc/__code/river/kit/_typescript/listeners/listeners.ts`

## I) Additional Vorma Source Domains (Coverage-Reconciliation Expansion)

These rows were added after reconciling the ledger against repository domains
used by Vorma runtime/build behavior to avoid silent omissions.

### I.1) Public Root and Client Entrypoints

- `VERIFIED` | `/Users/sjc/__code/river/vorma.go`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/index.ts`
- `VERIFIED` | `/Users/sjc/__code/river/internal/framework/_typescript/client/tsconfig.json`

### I.2) Wave Core Package (`wave/*.go`, non-tooling)

- `VERIFIED` | `/Users/sjc/__code/river/wave/wave.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/types.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/env.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/refresh.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/filemap.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/css.go`
- `VERIFIED` | `/Users/sjc/__code/river/wave/parse.go`

### I.3) Build Core Package (`vormabuild/*.go`)

- `VERIFIED` | `/Users/sjc/__code/river/vormabuild/vorma_build.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormabuild/vorma_gen_ts.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormabuild/vite_cmd.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormabuild/rebuild_routes.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormabuild/route_registry_build.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormabuild/configschema.go`
- `VERIFIED` | `/Users/sjc/__code/river/vormabuild/fs_to_hash.go`

### I.4) Bootstrap Runtime (`bootstrap/*.go`, Out-of-Scope)

- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/bootstrap.go` | App-scaffold generation runtime; not Vorma runtime/build/dev-server contract.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/utils.go` | App-scaffold helper runtime; not Vorma runtime/build/dev-server contract.

### I.4a) Bootstrap Template/Scaffold Assets

These files define generated scaffold app/distribution content and are outside
Vorma runtime/build/dev-server normative behavior scope.

- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/assets/favicon.svg` | Scaffold asset template.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/api_proxy_ts_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/backend_src_router_router_go_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/backend_static_entry_go_html_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/backend_wave_dev_go_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/backend_wave_prod_go_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/cmd_app_main_go_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/cmd_build_main_go_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/dist_static_keep_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/dockerfile_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_api_client_ts_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_app_utils_tsx_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_css_tailwind_css_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_entry_tsx_preact_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_entry_tsx_react_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_entry_tsx_solid_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_home_tsx_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_links_tsx_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_root_tsx_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_routes_ts_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/frontend_vite_d_ts_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/gitignore_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/main_critical_css_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/main_css_str.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/package_json_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/ts_config_json_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/vercel_json_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/vite_config_ts_tmpl.txt` | Scaffold template source.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/bootstrap/tmpls/wave_config_json_tmpl.txt` | Scaffold template source.

### I.5) Conformance Suite Files (Spec-Validation Harness)

These are post-spec conformance harness files, not legacy truth-mining inputs or
runtime/build implementation sources.

- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.asset_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.component_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.ctx_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.events_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.fetch_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.hmr_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.init_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.link_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.loader_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.nav_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.rendering_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.scroll_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.skip_conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.ui_adapter_parity.conformance.test.ts` | Conformance harness.
- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/client/src/client.ui_root_parity.conformance.test.ts` | Conformance harness.

### I.6) Ancillary Package Artifacts

- `OUT-OF-SCOPE` | `/Users/sjc/__code/river/internal/framework/_typescript/create/.gitignore` | Ancillary package artifact; not normative runtime/build behavior.

## J) Supporting Utility Packages Used by Runtime/Build APIs (`lab/*`)

- `VERIFIED` | `/Users/sjc/__code/river/lab/parseutil/parseutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/tsgen/generate_ts_content.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/tsgen/statements.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/tsgen/to_file.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/tsgen/generate_ts_content_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/tsgen/tsgencore/tsgencore.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/tsgen/tsgencore/tsgencore_test.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/viteutil/cmd.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/viteutil/viteutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/jsonschema/jsonschema.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/stringsutil/collect_lines.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/stringsutil/stringsutil.go`
- `VERIFIED` | `/Users/sjc/__code/river/lab/esbuildutil/esbuildutil.go`

## Next Mining Queue (Strict Order; unresolved `HISTORICAL`/`PENDING` first)

All currently tracked in-scope files are `VERIFIED` (no unresolved queue
entries).
