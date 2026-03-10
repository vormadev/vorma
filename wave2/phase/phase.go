package phase

type build_ctx struct {
	prev_build_id    string
	current_build_id string // if available
	prev_cfg_hash    string
	current_cfg_hash string // if available
}

type task struct {
	work         string
	why          string
	require_done []task
}

var wave__read_and_parse_user_config_candidate_for_this_cycle = task{
	work: "read and parse user config candidate for this cycle.",
	why:  "everything else depends on resolved user config inputs.",
}

var wave__capture_run_baseline_state = task{
	work: "capture run baseline state (previous build id, run context metadata).",
	why:  "hooks and runtime decisions need stable per-run context.",
}

var wave__normalize_and_dedupe_raw_event_batch = task{
	work: "normalize and dedupe raw event batch.",
	why:  "downstream semantics must operate on canonicalized event facts.",
}

var vorma__derive_framework_overlay_candidate_from_parsed_user_config = task{
	work: "derive framework overlay candidate from parsed user config.",
	why:  "framework watch/schema/runtime additions must be computed from the same config snapshot Wave is using.",
	require_done: []task{
		wave__read_and_parse_user_config_candidate_for_this_cycle,
	},
}

var wave__ensure_watcher_coverage_for_directories_implied_by_event_paths = task{
	work:         "ensure watcher coverage for directories implied by event paths.",
	why:          "future batches miss changes if watcher coverage is stale.",
	require_done: []task{wave__normalize_and_dedupe_raw_event_batch},
}

var wave__atomically_commit_effective_config_view = task{
	work: "atomically commit effective config view (`user + framework overlay`).",
	why:  "all downstream decisions must read one coherent config snapshot.",
	require_done: []task{
		vorma__derive_framework_overlay_candidate_from_parsed_user_config,
	},
}

var wave__apply_config_mutation_pre_effects = task{
	work:         "apply config-mutation pre-effects (config reload attempt, noop-config detection side effects).",
	why:          "config change is a top-level control-flow domain and must be handled before normal event classification.",
	require_done: []task{wave__atomically_commit_effective_config_view},
}

var wave__classify_events_into_semantic_file_categories_and_path_classes = task{
	work:         "classify events into semantic file categories and path classes.",
	why:          "build/runtime actions are based on semantic category, not raw fs op.",
	require_done: []task{wave__apply_config_mutation_pre_effects},
}

var wave__derive_hook_applicability_and_hook_execution_context_facts = task{
	work: "derive hook applicability and hook execution context facts from classified events.",
	why:  "hook planning depends on classification output and dedupe context.",
	require_done: []task{
		wave__classify_events_into_semantic_file_categories_and_path_classes,
	},
}

var wave__reduce_classified_events_into_base_required_work_union = task{
	work: "reduce classified events into base required-work union.",
	why:  "this is the first canonical intent set before hook adjustments.",
	require_done: []task{
		wave__classify_events_into_semantic_file_categories_and_path_classes,
	},
}

var wave__derive_control_flow_decisions = task{
	work: "derive control-flow decisions (short-circuit/noop/retry/config-restart classes).",
	why:  "orchestration path selection must happen before heavy work begins.",
	require_done: []task{
		wave__reduce_classified_events_into_base_required_work_union,
	},
}

var wave__derive_app_stop_strategy = task{
	work:         "derive app-stop strategy from final event behavior facts.",
	why:          "stop behavior changes hook context semantics and backend/frontend flow.",
	require_done: []task{wave__derive_control_flow_decisions},
}

var wave__apply_app_stop_effects_and_annotate_run_context = task{
	work:         "apply app-stop effects and annotate run context accordingly.",
	why:          "later hooks and reload decisions must know whether app is already stopped.",
	require_done: []task{wave__derive_app_stop_strategy},
}

var wave__launch_detached_hooks_that_do_not_gate_the_critical_path = task{
	work:         "launch detached hooks that do not gate the critical path.",
	why:          "these hooks are required side effects but must not block phase progress.",
	require_done: []task{wave__apply_app_stop_effects_and_annotate_run_context},
}

var wave__execute_pre_window_hooks = task{
	work:         "execute pre-window hooks.",
	why:          "these are the earliest hook adjustments to base required-work intent.",
	require_done: []task{wave__apply_app_stop_effects_and_annotate_run_context},
}

var wave__merge_pre_hook_outcomes_into_required_work_union = task{
	work:         "merge pre-hook outcomes into required-work union.",
	why:          "hook decisions must be folded into one monotonic intent surface.",
	require_done: []task{wave__execute_pre_window_hooks},
}

var wave__run_wave_metadata_dist_runtime_artifact_preparation = task{
	work: "run wave metadata/dist/runtime artifact preparation.",
	why:  "later wave outputs assume directories/artifacts are initialized.",
	require_done: []task{
		wave__merge_pre_hook_outcomes_into_required_work_union,
	},
}

var wave__produce_compile_prerequisites = task{
	work: "produce compile prerequisites (overlay/materialized compile inputs).",
	why:  "go compilation can only start after its required inputs exist.",
	require_done: []task{
		wave__merge_pre_hook_outcomes_into_required_work_union,
	},
}

var wave__execute_go_compile_path_when_requested = task{
	work:         "execute go compile path when requested.",
	why:          "compiled binary state is a core wave output and implies restart class.",
	require_done: []task{wave__produce_compile_prerequisites},
}

var wave__run_non_compile_wave_output_work = task{
	work: "run non-compile wave output work (css/static/frontend asset materialization).",
	why:  "these are required batch outputs independent from compile completion.",
	require_done: []task{
		wave__run_wave_metadata_dist_runtime_artifact_preparation,
	},
}

var wave__finalize_public_map_artifacts_and_derive_changed_or_repaired_result = task{
	work: "finalize public map artifacts and derive changed-or-repaired result.",
	why:  "downstream runtime reload behavior depends on finalized public map state.",
	require_done: []task{
		wave__execute_go_compile_path_when_requested,
		wave__run_non_compile_wave_output_work,
	},
}

var wave__execute_concurrent_window_hooks_and_collect_outcomes = task{
	work:         "execute concurrent-window hooks and collect outcomes.",
	why:          "these hooks intentionally overlap wave work but still affect final intent.",
	require_done: []task{wave__execute_pre_window_hooks},
}

var wave__merge_concurrent_hook_outcomes_into_required_work_union = task{
	work: "merge concurrent-hook outcomes into required-work union.",
	why:  "hook outcomes must be included before backend/frontend settlement.",
	require_done: []task{
		wave__finalize_public_map_artifacts_and_derive_changed_or_repaired_result,
		wave__execute_concurrent_window_hooks_and_collect_outcomes,
	},
}

var wave__execute_post_window_hooks = task{
	work: "execute post-window hooks.",
	why:  "some framework/user hooks must run only after wave outputs are present.",
	require_done: []task{
		wave__finalize_public_map_artifacts_and_derive_changed_or_repaired_result,
		wave__execute_concurrent_window_hooks_and_collect_outcomes,
	},
}

var wave__merge_post_hook_outcomes_into_required_work_union = task{
	work: "merge post-hook outcomes into required-work union.",
	why:  "final backend/frontend decisions must include post-hook effects.",
	require_done: []task{
		wave__merge_concurrent_hook_outcomes_into_required_work_union,
		wave__execute_post_window_hooks,
	},
}

var wave__apply_selected_backend_mutation_branch = task{
	work: "apply selected backend mutation branch (retry queue, config-cycle restart, normal restart path).",
	why:  "this is where final backend mutation side effects are committed.",
	require_done: []task{
		wave__merge_post_hook_outcomes_into_required_work_union,
	},
}

var wave__derive_backend_convergence_requirements_from_final_intent = task{
	work: "derive backend convergence requirements from final intent.",
	why:  "readiness waits/reload notifications should be driven by explicit convergence requirements, not ad hoc checks.",
	require_done: []task{
		wave__merge_post_hook_outcomes_into_required_work_union,
	},
}

var wave__execute_backend_readiness_waits = task{
	work: "execute backend readiness waits (app and/or vite).",
	why:  "framework runtime reload and browser-facing actions need converged backend state.",
	require_done: []task{
		wave__apply_selected_backend_mutation_branch,
		wave__derive_backend_convergence_requirements_from_final_intent,
	},
}

var wave__derive_terminal_frontend_action_from_final_workset = task{
	work: "derive terminal frontend action from final workset.",
	why:  "frontend actions are mutually exclusive and require one selected outcome.",
	require_done: []task{
		wave__apply_selected_backend_mutation_branch,
		wave__derive_backend_convergence_requirements_from_final_intent,
	},
}

var vorma__execute_framework_runtime_reload_requests_and_backend_notifications = task{
	work: "execute framework runtime reload requests and backend notifications; apply failure policy outputs.",
	why:  "framework state convergence is separate from core wave output materialization and may request healing behavior.",
	require_done: []task{
		wave__execute_backend_readiness_waits,
		wave__derive_terminal_frontend_action_from_final_workset,
	},
}

var wave__execute_selected_terminal_frontend_action = task{
	work: "execute selected terminal frontend action.",
	why:  "this is the user-visible end of the cycle (none/revalidate/reload/etc).",
	require_done: []task{
		vorma__execute_framework_runtime_reload_requests_and_backend_notifications,
	},
}

var wave__derive_healing_loopback_vs_finish_decision = task{
	work:         "derive healing loopback vs finish decision.",
	why:          "convergence failure policy may require an additional mutation pass.",
	require_done: []task{wave__execute_selected_terminal_frontend_action},
}

var wave__emit_completion_facts_and_clear_batch_scoped_temporary_state = task{
	work:         "emit completion facts and clear batch-scoped temporary state.",
	why:          "next cycle must start from a clean batch-local state.",
	require_done: []task{wave__derive_healing_loopback_vs_finish_decision},
}

var wave__remove_stale_watcher_paths_and_clear_detached_hook_lifecycle_context = task{
	work: "remove stale watcher paths and clear detached-hook lifecycle context.",
	why:  "stale watch entries and stale detached-hook context corrupt future cycles.",
	require_done: []task{
		wave__emit_completion_facts_and_clear_batch_scoped_temporary_state,
		wave__launch_detached_hooks_that_do_not_gate_the_critical_path,
	},
}
