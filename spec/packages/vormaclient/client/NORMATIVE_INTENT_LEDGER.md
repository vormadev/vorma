# vormaclient/client Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use implementation source and incorporate legacy tests
  outside `conformance/**` when present.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File                                                                                   | Scope    | Mining Inputs       | Passes | Clean Passes | Epoch State     | Notes |
| -------------------------------------------------------------------------------------- | -------- | ------------------- | -----: | -----------: | --------------- | ----- |
| `vormaclient/client/index.ts`                                                          | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/asset_manager.ts`                                              | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.component_module_loading.test.ts`                       | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.core_navigation.link_click_handling.test.ts`            | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.core_navigation.navigation_types.test.ts`               | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.core_navigation.programmatic_navigation.test.ts`        | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.core_navigation.state_management.test.ts`               | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.critical_edge_cases.test.ts`                            | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.error_handling.test.ts`                                 | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.events_system.test.ts`                                  | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.form_submissions.revalidate_function.test.ts`           | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.form_submissions.submit_function.test.ts`               | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.history_management.test.ts`                             | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.initialization.test.ts`                                 | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.loading_state_continuity.test.ts`                       | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.navigation_lifecycle.begin_navigation_phase.test.ts`    | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.navigation_lifecycle.complete_navigation_phase.test.ts` | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.navigation_lifecycle.fetch_route_data_phase.test.ts`    | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.navigation_lifecycle.rerender_app_phase.test.ts`        | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.prefetching.test.ts`                                    | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.redirects.test.ts`                                      | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.scroll_restoration.test.ts`                             | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.test.helpers.ts`                                        | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.ts`                                                     | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client.utility_functions.test.ts`                              | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/client_loaders.ts`                                             | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/component_loader.ts`                                           | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/error_boundary.ts`                                             | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/events.ts`                                                     | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/global_loading_indicator/global_loading_indicator.ts`          | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/hard_reload.ts`                                                | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/head_elements/head.advanced_updates.test.ts`                   | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/head_elements/head.basic_operations.test.ts`                   | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/head_elements/head.minimal_dom_changes.test.ts`                | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/head_elements/head.rest_and_edge_cases.test.ts`                | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/head_elements/head.test.helpers.ts`                            | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/head_elements/head_elements.ts`                                | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/history/history.ts`                                            | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/history/npm_history_types.ts`                                  | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/hmr/hmr.ts`                                                    | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/init_client.ts`                                                | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/links.ts`                                                      | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/redirects/redirects.ts`                                        | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/rendering.ts`                                                  | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/resolve_public_href.ts`                                        | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/scroll_state_manager.ts`                                       | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/static_route_defs/route_def_helpers.ts`                        | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/ui_lib_impl_helpers/link_components.ts`                        | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/ui_lib_impl_helpers/route_components.ts`                       | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/ui_lib_impl_helpers/typed_navigate.ts`                         | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/utils/errors.ts`                                               | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/utils/logging.ts`                                              | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/vorma_app_helpers/vorma_app_helpers.ts`                        | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/vorma_ctx/vorma_ctx.test.ts`                                   | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/vorma_ctx/vorma_ctx.ts`                                        | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/src/window_focus_revalidation/window_focus_revalidation.ts`        | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |
| `vormaclient/client/tsconfig.json`                                                     | in-scope | source+legacy-tests |      2 |            2 | mined_gaps_open |       |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                                                                               |
| ------- | --------- | -------: | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        0 | Rough replay completed across all in-scope files; active `VCI-*` backlog remained open.                                                             |
| `E2-R2` | completed |        0 | Full structural/boundary/semantic reconciliation completed across all in-scope files; no additional gap IDs surfaced; open `VCI-*` backlog remains. |
| `E2-R3` | pending   |    `TBD` | No-gap closure rounds remain blocked until open `VCI-*` issues are resolved or explicitly accepted.                                                 |
