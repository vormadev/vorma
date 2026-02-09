# Vorma Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2` (package-local reset)

## Mining Rules

- Intent mining MUST use both implementation source and existing tests.
- Per-file counters are epoch-scoped.
- `passes`: total number of full mining passes for that file in current epoch.
- `clean_passes`: number of passes with no newly surfaced normative gap for that file in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Legacy State | Notes |
|---|---|---|---:|---:|---|---|---|
| `vormaclient/client/index.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/asset_manager.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.asset_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.component_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.component_module_loading.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.core_navigation.link_click_handling.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.core_navigation.navigation_types.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.core_navigation.programmatic_navigation.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.core_navigation.state_management.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.critical_edge_cases.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.ctx_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.error_handling.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.events_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.events_system.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.fetch_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.form_submissions.revalidate_function.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.form_submissions.submit_function.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.history_management.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.hmr_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.init_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.initialization.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.link_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.loader_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.loading_state_continuity.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.nav_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.navigation_lifecycle.begin_navigation_phase.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.navigation_lifecycle.complete_navigation_phase.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.navigation_lifecycle.fetch_route_data_phase.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.navigation_lifecycle.rerender_app_phase.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.prefetching.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.redirects.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.rendering_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.scroll_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.scroll_restoration.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.skip_conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.test.helpers.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Test harness helper, not runtime contract. |
| `vormaclient/client/src/client.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client.ui_adapter_parity.conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.ui_root_parity.conformance.test.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Conformance harness. |
| `vormaclient/client/src/client.utility_functions.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/client_loaders.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/component_loader.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/error_boundary.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/events.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/global_loading_indicator/global_loading_indicator.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/hard_reload.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/head_elements/head.advanced_updates.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/head_elements/head.basic_operations.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/head_elements/head.minimal_dom_changes.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/head_elements/head.rest_and_edge_cases.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/head_elements/head.test.helpers.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Test helper. |
| `vormaclient/client/src/head_elements/head_elements.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/history/history.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/history/npm_history_types.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/hmr/hmr.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/init_client.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/links.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/redirects/redirects.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/rendering.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/resolve_public_href.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/scroll_state_manager.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/static_route_defs/route_def_helpers.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/ui_lib_impl_helpers/link_components.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/ui_lib_impl_helpers/route_components.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/ui_lib_impl_helpers/typed_navigate.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/utils/errors.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/utils/logging.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/vorma_app_helpers/vorma_app_helpers.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/vorma_ctx/vorma_ctx.test.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/vorma_ctx/vorma_ctx.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/src/window_focus_revalidation/window_focus_revalidation.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/client/tsconfig.json` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/create/.gitignore` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Ancillary package artifact; not normative runtime/build behavior. |
| `vormaclient/create/dist/**` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Generated build output for create-CLI package. |
| `vormaclient/create/main.ts` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Create-CLI scaffolding/distribution tool source; not Vorma runtime/build/dev-server contract. |
| `vormaclient/create/node_modules/**` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Vendored package-manager dependency tree for create-CLI package. |
| `vormaclient/create/package.json` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Create-CLI package metadata; not Vorma runtime/build/dev-server contract. |
| `vormaclient/create/pnpm-lock.yaml` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Package-manager lock artifact; no direct Vorma runtime/API contract. |
| `vormaclient/create/tsconfig.json` | out-of-scope | n/a | 0 | 0 | out-of-scope | out-of-scope | Create-CLI toolchain config; not Vorma runtime/build/dev-server contract. |
| `vormaclient/preact/index.tsx` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/preact/src/helpers.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/preact/src/link.tsx` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/preact/src/preact.tsx` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/preact/tsconfig.json` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/react/index.tsx` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/react/src/helpers.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/react/src/link.tsx` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/react/src/react.tsx` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/react/tsconfig.json` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/solid/index.tsx` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/solid/src/helpers.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/solid/src/link.tsx` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/solid/src/solid.tsx` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/solid/tsconfig.json` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/vite/tsconfig.json` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaclient/vite/vite.ts` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vorma.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormabuild/configschema.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormabuild/fs_to_hash.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormabuild/rebuild_routes.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormabuild/route_registry_build.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormabuild/vite_cmd.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormabuild/vorma_build.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormabuild/vorma_gen_ts.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/errors.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/get_deps.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/get_root_handler.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/glue.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/gmpd.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/paths.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/route_registry.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/route_reload.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/ssr.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/types.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/vite_url.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/vorma_core.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |
| `vormaruntime/vorma_init.go` | in-scope | source+tests | 0 | 0 | pending | verified |  |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | in_progress | `TBD` | Epoch reset baseline created; per-file counters initialized. |
