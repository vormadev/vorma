# Checkpoint Plan

## Task library

This section is remodeled as a terminal-first DAG.

Each task is `ensure_*_if_needed`. Each `require_done` list contains only direct
prerequisites. No ancestor prerequisites are repeated.

| id  | task_symbol                                                             | require_done                                                                                                         |
| --- | ----------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| 1   | `wave__ensure_supercycle_cleanup_done_if_needed`                        | `wave__ensure_completion_facts_emitted_if_needed`                                                                    |
| 2   | `wave__ensure_completion_facts_emitted_if_needed`                       | `wave__ensure_terminal_frontend_action_executed_if_needed`                                                           |
| 3   | `wave__ensure_terminal_frontend_action_executed_if_needed`              | `vorma__ensure_runtime_reload_and_backend_notifications_done_if_needed`                                              |
| 4   | `vorma__ensure_runtime_reload_and_backend_notifications_done_if_needed` | `wave__ensure_backend_readiness_waits_done_if_needed`, `wave__ensure_terminal_frontend_action_selected_if_needed`    |
| 5   | `wave__ensure_backend_readiness_waits_done_if_needed`                   | `wave__ensure_backend_mutation_applied_if_needed`, `wave__ensure_backend_convergence_requirements_derived_if_needed` |
| 6   | `wave__ensure_terminal_frontend_action_selected_if_needed`              | `wave__ensure_backend_mutation_applied_if_needed`, `wave__ensure_backend_convergence_requirements_derived_if_needed` |
| 7   | `wave__ensure_backend_mutation_applied_if_needed`                       | `wave__ensure_post_hook_outcomes_merged_if_needed`                                                                   |
| 8   | `wave__ensure_backend_convergence_requirements_derived_if_needed`       | `wave__ensure_post_hook_outcomes_merged_if_needed`                                                                   |
| 9   | `wave__ensure_post_hook_outcomes_merged_if_needed`                      | `wave__ensure_concurrent_hook_outcomes_merged_if_needed`, `wave__ensure_post_hooks_executed_if_needed`               |
| 10  | `wave__ensure_post_hooks_executed_if_needed`                            | `wave__ensure_wave_public_map_finalized_if_needed`, `wave__ensure_concurrent_hooks_executed_if_needed`               |
| 11  | `wave__ensure_concurrent_hook_outcomes_merged_if_needed`                | `wave__ensure_wave_public_map_finalized_if_needed`, `wave__ensure_concurrent_hooks_executed_if_needed`               |
| 12  | `wave__ensure_wave_public_map_finalized_if_needed`                      | `wave__ensure_go_compile_done_if_needed`                                                                             |
| 13  | `wave__ensure_concurrent_hooks_executed_if_needed`                      | `wave__ensure_pre_hooks_executed_if_needed`                                                                          |
| 14  | `wave__ensure_go_compile_done_if_needed`                                | `wave__ensure_go_compile_prereqs_done_if_needed`, `wave__ensure_wave_non_compile_work_done_if_needed`                |
| 15  | `wave__ensure_wave_non_compile_work_done_if_needed`                     | `wave__ensure_wave_metadata_and_runtime_artifacts_ready_if_needed`                                                   |
| 16  | `wave__ensure_go_compile_prereqs_done_if_needed`                        | `wave__ensure_pre_hook_outcomes_merged_if_needed`                                                                    |
| 17  | `wave__ensure_wave_metadata_and_runtime_artifacts_ready_if_needed`      | `wave__ensure_pre_hook_outcomes_merged_if_needed`                                                                    |
| 18  | `wave__ensure_pre_hook_outcomes_merged_if_needed`                       | `wave__ensure_pre_hooks_executed_if_needed`                                                                          |
| 19  | `wave__ensure_pre_hooks_executed_if_needed`                             | `wave__ensure_hook_applicability_derived_if_needed`, `wave__ensure_app_stop_effects_applied_if_needed`               |
| 20  | `wave__ensure_app_stop_effects_applied_if_needed`                       | `wave__ensure_app_stop_strategy_derived_if_needed`, `wave__ensure_run_baseline_captured_if_needed`                   |
| 21  | `wave__ensure_app_stop_strategy_derived_if_needed`                      | `wave__ensure_control_flow_decisions_derived_if_needed`                                                              |
| 22  | `wave__ensure_control_flow_decisions_derived_if_needed`                 | `wave__ensure_required_work_reduced_if_needed`                                                                       |
| 23  | `wave__ensure_required_work_reduced_if_needed`                          | `wave__ensure_event_categories_derived_if_needed`                                                                    |
| 24  | `wave__ensure_hook_applicability_derived_if_needed`                     | `wave__ensure_event_categories_derived_if_needed`                                                                    |
| 25  | `wave__ensure_event_categories_derived_if_needed`                       | `wave__ensure_config_mutation_pre_effects_applied_if_needed`                                                         |
| 26  | `wave__ensure_config_mutation_pre_effects_applied_if_needed`            | `wave__ensure_effective_config_committed_if_needed`                                                                  |
| 27  | `wave__ensure_effective_config_committed_if_needed`                     | `vorma__ensure_framework_overlay_derived_if_needed`                                                                  |
| 28  | `vorma__ensure_framework_overlay_derived_if_needed`                     | `wave__ensure_user_config_parsed_if_needed`                                                                          |
| 29  | `wave__ensure_user_config_parsed_if_needed`                             |                                                                                                                      |
| 30  | `wave__ensure_run_baseline_captured_if_needed`                          |                                                                                                                      |
| 31  | `wave__ensure_raw_event_batch_normalized_if_needed`                     |                                                                                                                      |
| 32  | `wave__ensure_watcher_coverage_done_if_needed`                          | `wave__ensure_raw_event_batch_normalized_if_needed`                                                                  |

## Supercycle entrypoint

The supervisor should trigger a flat `RunParallel` over terminal tasks only:

- `wave__ensure_supercycle_cleanup_done_if_needed`
- `wave__ensure_watcher_coverage_done_if_needed`

Everything else is reached through `require_done`.
