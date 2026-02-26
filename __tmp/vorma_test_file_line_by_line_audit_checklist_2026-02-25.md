# Vorma Test File Line-by-Line Audit Checklist (2026-02-25)

Scope: all test files under `typescript/vorma/*`. Rule: each file must be
reviewed line-by-line for first-principles correctness; no quirks/bugs enshrined
as expected behavior.

- [x] typescript/vorma/client/src/tests/contracts/client.error_and_edge.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.events.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.history_and_init.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.link_click.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.loading_and_focus.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.module_loading_and_fetch.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.navigation_lifecycle.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.navigation_modes.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.navigation_state_machine.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.prefetch.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.state_and_revalidation.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.submit_and_redirect.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/client.utilities.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/head_elements.contract.test.ts
- [x] typescript/vorma/client/src/tests/contracts/vorma_ctx.contract.test.ts
- [x] typescript/vorma/client/src/tests/dist/npm_dist_adapters.test.ts
- [x] typescript/vorma/client/src/tests/dist/npm_dist_adapters_helpers_link_mocked.test.ts
- [x] typescript/vorma/client/src/tests/dist/npm_dist_adapters_root_outlet.test.ts
- [x] typescript/vorma/client/src/tests/dist/npm_dist_adapters_root_outlet_branches.test.ts
- [x] typescript/vorma/client/src/tests/dist/npm_dist_adapters_root_outlet_runtime_state.test.ts
- [x] typescript/vorma/client/src/tests/dist/npm_dist_client_runtime.test.ts
- [x] typescript/vorma/client/src/tests/unit/app_helpers.test.ts
- [x] typescript/vorma/client/src/tests/unit/begin_navigation_state_machine_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/client_runtime_initialization.test.ts
- [x] typescript/vorma/client/src/tests/unit/events_platform.test.ts
- [x] typescript/vorma/client/src/tests/unit/extras_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/fetch_route_data_preload_commands_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/fetch_route_data_preload_state_machine_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/hash_fragment.test.ts
- [x] typescript/vorma/client/src/tests/unit/history_listener_prelude.test.ts
- [x] typescript/vorma/client/src/tests/unit/links_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/make_typed_api.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_lifecycle_runtime_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_lifecycle_transitions_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_outcome_runtime_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_outcome_state_machine_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_pass_runtime_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_revalidation_lane_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_runtime_slots_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_state_machine_bundle_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_state_machine_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/navigation_successful_runtime_commit_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/redirect_request_init.test.ts
- [x] typescript/vorma/client/src/tests/unit/redirects_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/render_runtime_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/revalidation_focus_trigger_policy_state_machine_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/revalidation_trigger_timestamp_state_machine_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/route_outlet_runtime_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/scroll_apply_state.test.ts
- [x] typescript/vorma/client/src/tests/unit/scroll_state_refresh_state.test.ts
- [x] typescript/vorma/client/src/tests/unit/scroll_state_storage.test.ts
- [x] typescript/vorma/client/src/tests/unit/submission_lifecycle_commands_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/typed_adapter_helpers_runtime_internal.test.ts
- [x] typescript/vorma/client/src/tests/unit/utils_runtime.test.ts
- [x] typescript/vorma/client/src/tests/unit/vorma_ctx.test.ts

Audit notes:

- The prior stale route-props expectation bug was corrected in
  `npm_dist_adapters_root_outlet_runtime_state.test.ts`.
- Render-phase token activation leak is now covered by unit regression in
  `typed_adapter_helpers_runtime_internal.test.ts`.
- No additional files were found to enshrine first-principles violations.
