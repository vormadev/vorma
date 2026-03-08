package wavebuild

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/kit/phaselane"
	"github.com/vormadev/vorma/kit/tasks"
)

/////////////////////////////////////////////////////////////////////
/////// Phase Task Contracts
/////////////////////////////////////////////////////////////////////

type phase_task_input struct {
	mode       mode
	gen_id     string
	step_index int

	app_stop_strategy app_stop_strategy

	backend_mutation_branch  backend_mutation_branch
	frontend_terminal_action frontend_terminal_action
	should_request_loop_back bool
	hook_stage               hook_stage

	should_short_circuit_for_config_restart   bool
	should_short_circuit_for_build_retry_wait bool
	should_short_circuit_for_noop_batch       bool

	batch_event_categorization batch_event_categorization
}

type batch_watcher_event struct {
	path                                    string
	op                                      batch_watcher_event_op
	treat_as_non_go                         bool
	recompile_go_binary                     bool
	restart_app                             bool
	run_on_change_only                      bool
	only_run_client_defined_revalidate_func bool
	waiting_for_build_retry                 bool
}

type batch_watcher_event_op string

const (
	batch_watcher_event_op_create batch_watcher_event_op = "create"
	batch_watcher_event_op_write  batch_watcher_event_op = "write"
	batch_watcher_event_op_remove batch_watcher_event_op = "remove"
	batch_watcher_event_op_rename batch_watcher_event_op = "rename"
	batch_watcher_event_op_chmod  batch_watcher_event_op = "chmod"
)

type batch_event_categorization struct {
	event_count               int
	config_change_count       int
	hard_reload_count         int
	build_go_binary           bool
	build_critical_css        bool
	build_normal_css          bool
	process_public_static     bool
	process_private_static    bool
	requires_backend_restart  bool
	prefer_revalidate         bool
	short_circuit_build_retry bool
}

type backend_mutation_branch string

const (
	backend_mutation_branch_queue_retry_wait_restart       backend_mutation_branch = "queue_retry_wait_restart"
	backend_mutation_branch_restart_dev_server_cycle       backend_mutation_branch = "restart_dev_server_cycle"
	backend_mutation_branch_apply_normal_backend_mutations backend_mutation_branch = "apply_normal_backend_mutations"
)

type frontend_terminal_action string

const (
	frontend_terminal_action_none                               frontend_terminal_action = "none"
	frontend_terminal_action_css_hot_reload                     frontend_terminal_action = "css_hot_reload"
	frontend_terminal_action_revalidate                         frontend_terminal_action = "revalidate"
	frontend_terminal_action_notify_vite_public_filemap_changed frontend_terminal_action = "notify_vite_public_filemap_changed"
	frontend_terminal_action_hard_reload                        frontend_terminal_action = "hard_reload"
)

type hook_stage string

const (
	hook_stage_none               hook_stage = "none"
	hook_stage_pre                hook_stage = "pre"
	hook_stage_concurrent         hook_stage = "concurrent"
	hook_stage_concurrent_no_wait hook_stage = "concurrent_no_wait"
	hook_stage_post               hook_stage = "post"
)

type app_stop_strategy string

const (
	app_stop_strategy_none                     app_stop_strategy = "none"
	app_stop_strategy_single_event_hard_reload app_stop_strategy = "single_event_hard_reload"
	app_stop_strategy_batch_hard_reload        app_stop_strategy = "batch_hard_reload"
)

type event_execution_flow_decision struct {
	app_stop_strategy                         app_stop_strategy
	should_short_circuit_for_config_restart   bool
	should_short_circuit_for_build_retry_wait bool
	should_short_circuit_for_noop_batch       bool
}

func normalize_batch_watcher_event_path_for_categorization(
	raw_path string,
) string {
	trimmed_path := strings.TrimSpace(raw_path)
	if trimmed_path == "" {
		return ""
	}
	normalized_slashes_path := strings.ReplaceAll(trimmed_path, "\\", "/")
	normalized_path := filepath.Clean(normalized_slashes_path)
	if normalized_path == "." {
		return ""
	}
	return strings.ReplaceAll(normalized_path, "\\", "/")
}

func is_config_change_path_for_batch_event_categorization(
	normalized_path string,
) bool {
	config_file_name := strings.ToLower(
		strings.TrimSpace(filepath.Base(normalized_path)),
	)
	if config_file_name == "wave.config.json" {
		return true
	}
	return strings.HasPrefix(config_file_name, "wave.config.")
}

func should_treat_batch_event_as_critical_css(
	normalized_path string,
) bool {
	return strings.HasSuffix(strings.ToLower(normalized_path), ".critical.css")
}

func should_treat_batch_event_as_normal_css(
	normalized_path string,
) bool {
	normalized_lower_path := strings.ToLower(normalized_path)
	return strings.HasSuffix(normalized_lower_path, ".css") &&
		!should_treat_batch_event_as_critical_css(normalized_path)
}

func should_treat_batch_event_as_go_source(
	normalized_path string,
) bool {
	return strings.HasSuffix(strings.ToLower(normalized_path), ".go")
}

func should_treat_batch_event_as_public_static(
	normalized_path string,
) bool {
	normalized_lower_path := strings.ToLower(normalized_path)
	return strings.Contains(normalized_lower_path, "/public/")
}

func should_treat_batch_event_as_private_static(
	normalized_path string,
) bool {
	normalized_lower_path := strings.ToLower(normalized_path)
	return strings.Contains(normalized_lower_path, "/private/")
}

func derive_batch_event_categorization(
	batch_events []batch_watcher_event,
) batch_event_categorization {
	categorization := batch_event_categorization{}
	for _, batch_event := range batch_events {
		if batch_event.op == batch_watcher_event_op_chmod {
			continue
		}
		normalized_path := normalize_batch_watcher_event_path_for_categorization(
			batch_event.path,
		)
		if normalized_path == "" {
			continue
		}
		if batch_event.waiting_for_build_retry {
			categorization.short_circuit_build_retry = true
		}
		if is_config_change_path_for_batch_event_categorization(normalized_path) {
			categorization.event_count++
			categorization.config_change_count++
			continue
		}
		if batch_event.run_on_change_only {
			categorization.event_count++
			continue
		}
		is_go_source := should_treat_batch_event_as_go_source(normalized_path)
		if is_go_source && batch_event.treat_as_non_go {
			is_go_source = false
		}
		if is_go_source {
			categorization.event_count++
			categorization.build_go_binary = true
			categorization.hard_reload_count++
			categorization.requires_backend_restart = true
			if batch_event.only_run_client_defined_revalidate_func {
				categorization.prefer_revalidate = true
			}
			continue
		}
		if should_treat_batch_event_as_critical_css(normalized_path) {
			categorization.event_count++
			categorization.build_critical_css = true
			if batch_event.only_run_client_defined_revalidate_func {
				categorization.prefer_revalidate = true
			}
			if batch_event.recompile_go_binary || batch_event.restart_app {
				categorization.hard_reload_count++
				categorization.requires_backend_restart = true
			}
			continue
		}
		if should_treat_batch_event_as_normal_css(normalized_path) {
			categorization.event_count++
			categorization.build_normal_css = true
			if batch_event.only_run_client_defined_revalidate_func {
				categorization.prefer_revalidate = true
			}
			if batch_event.recompile_go_binary || batch_event.restart_app {
				categorization.hard_reload_count++
				categorization.requires_backend_restart = true
			}
			continue
		}
		if should_treat_batch_event_as_public_static(normalized_path) {
			categorization.event_count++
			categorization.process_public_static = true
			if batch_event.only_run_client_defined_revalidate_func {
				categorization.prefer_revalidate = true
			}
			if batch_event.recompile_go_binary || batch_event.restart_app {
				categorization.hard_reload_count++
				categorization.requires_backend_restart = true
			}
			continue
		}
		if should_treat_batch_event_as_private_static(normalized_path) {
			categorization.event_count++
			categorization.process_private_static = true
			categorization.hard_reload_count++
			categorization.requires_backend_restart = true
			if batch_event.only_run_client_defined_revalidate_func {
				categorization.prefer_revalidate = true
			}
			continue
		}
		categorization.event_count++
		if batch_event.only_run_client_defined_revalidate_func {
			categorization.prefer_revalidate = true
		}
		if batch_event.run_on_change_only {
			continue
		}
		if batch_event.recompile_go_binary {
			categorization.build_go_binary = true
		}
		if batch_event.recompile_go_binary || batch_event.restart_app {
			categorization.hard_reload_count++
			categorization.requires_backend_restart = true
		}
	}
	return categorization
}

func derive_defaults_from_batch_event_categorization(
	categorization batch_event_categorization,
) (
	derived_app_stop_strategy app_stop_strategy,
	derived_frontend_terminal_action frontend_terminal_action,
	derived_should_short_circuit_for_config_restart bool,
	derived_should_short_circuit_for_build_retry_wait bool,
	derived_should_short_circuit_for_noop_batch bool,
	derived_backend_mutation_branch backend_mutation_branch,
) {
	derived_app_stop_strategy = app_stop_strategy_none
	derived_frontend_terminal_action = frontend_terminal_action_none
	derived_should_short_circuit_for_config_restart = categorization.config_change_count > 0
	derived_should_short_circuit_for_build_retry_wait = categorization.short_circuit_build_retry
	derived_should_short_circuit_for_noop_batch = categorization.event_count == 0
	derived_backend_mutation_branch = backend_mutation_branch_apply_normal_backend_mutations

	if derived_should_short_circuit_for_config_restart {
		derived_backend_mutation_branch = backend_mutation_branch_restart_dev_server_cycle
		return derived_app_stop_strategy,
			derived_frontend_terminal_action,
			derived_should_short_circuit_for_config_restart,
			derived_should_short_circuit_for_build_retry_wait,
			derived_should_short_circuit_for_noop_batch,
			derived_backend_mutation_branch
	}
	if derived_should_short_circuit_for_build_retry_wait {
		derived_backend_mutation_branch = backend_mutation_branch_queue_retry_wait_restart
		return derived_app_stop_strategy,
			derived_frontend_terminal_action,
			derived_should_short_circuit_for_config_restart,
			derived_should_short_circuit_for_build_retry_wait,
			derived_should_short_circuit_for_noop_batch,
			derived_backend_mutation_branch
	}
	if derived_should_short_circuit_for_noop_batch {
		return derived_app_stop_strategy,
			derived_frontend_terminal_action,
			derived_should_short_circuit_for_config_restart,
			derived_should_short_circuit_for_build_retry_wait,
			derived_should_short_circuit_for_noop_batch,
			derived_backend_mutation_branch
	}

	switch {
	case categorization.hard_reload_count == 1 &&
		categorization.event_count == 1:
		derived_app_stop_strategy = app_stop_strategy_single_event_hard_reload
	case categorization.hard_reload_count > 0:
		derived_app_stop_strategy = app_stop_strategy_batch_hard_reload
	}

	switch {
	case categorization.hard_reload_count > 0:
		derived_frontend_terminal_action = frontend_terminal_action_hard_reload
	case categorization.process_public_static:
		derived_frontend_terminal_action = frontend_terminal_action_notify_vite_public_filemap_changed
	case categorization.prefer_revalidate:
		derived_frontend_terminal_action = frontend_terminal_action_revalidate
	case categorization.build_critical_css || categorization.build_normal_css:
		derived_frontend_terminal_action = frontend_terminal_action_css_hot_reload
	default:
		derived_frontend_terminal_action = frontend_terminal_action_none
	}

	return derived_app_stop_strategy,
		derived_frontend_terminal_action,
		derived_should_short_circuit_for_config_restart,
		derived_should_short_circuit_for_build_retry_wait,
		derived_should_short_circuit_for_noop_batch,
		derived_backend_mutation_branch
}

/////////////////////////////////////////////////////////////////////
/////// Phase 1 Tasks
/////////////////////////////////////////////////////////////////////

var derive_batch_facts_task = tracked_task(_LABEL_TASK_DERIVE_BATCH_FACTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := track_watcher_events_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := ensure_directory_watch_for_event_paths_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := normalize_batch_watcher_events_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := apply_config_reload_pre_classification_side_effects_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := classify_watcher_events_for_processing_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := build_event_hooks_for_processing_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_initial_requested_outcomes_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var track_watcher_events_task = tracked_task(_LABEL_TASK_TRACK_WATCHER_EVENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var ensure_directory_watch_for_event_paths_task = tracked_task(_LABEL_TASK_ENSURE_DIRECTORY_WATCH_FOR_EVENT_PATHS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var normalize_batch_watcher_events_task = tracked_task(_LABEL_TASK_NORMALIZE_BATCH_WATCHER_EVENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := deduplicate_watcher_events_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := normalize_watcher_event_paths_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var deduplicate_watcher_events_task = tracked_task(_LABEL_TASK_DEDUPLICATE_WATCHER_EVENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var normalize_watcher_event_paths_task = tracked_task(_LABEL_TASK_NORMALIZE_WATCHER_EVENT_PATHS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var apply_config_reload_pre_classification_side_effects_task = tracked_task(_LABEL_TASK_APPLY_CONFIG_RELOAD_PRE_CLASSIFICATION_SIDE_EFFECTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := detect_config_mutation_watcher_events_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_config_change_batch_skip_non_config_hook_processing_decision_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := reload_config_if_changed_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := emit_noop_config_reload_notice_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var detect_config_mutation_watcher_events_task = tracked_task(_LABEL_TASK_DETECT_CONFIG_MUTATION_WATCHER_EVENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_config_change_batch_skip_non_config_hook_processing_decision_task = tracked_task(_LABEL_TASK_DERIVE_CONFIG_CHANGE_BATCH_SKIP_NON_CONFIG_HOOK_PROCESSING_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var reload_config_if_changed_task = tracked_task(_LABEL_TASK_RELOAD_CONFIG_IF_CHANGED, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var emit_noop_config_reload_notice_task = tracked_task(_LABEL_TASK_EMIT_NOOP_CONFIG_RELOAD_NOTICE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var classify_watcher_events_for_processing_task = tracked_task(_LABEL_TASK_CLASSIFY_WATCHER_EVENTS_FOR_PROCESSING, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_pre_classification_decisions_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_initial_file_type_for_watcher_events_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := apply_watched_file_overrides_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_watcher_event_ignored_status_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_chmod_only_decisions_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := filter_classified_events_for_processing_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_pre_classification_decisions_task = tracked_task(_LABEL_TASK_DERIVE_PRE_CLASSIFICATION_DECISIONS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_initial_file_type_for_watcher_events_task = tracked_task(_LABEL_TASK_DERIVE_INITIAL_FILE_TYPE_FOR_WATCHER_EVENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var apply_watched_file_overrides_task = tracked_task(_LABEL_TASK_APPLY_WATCHED_FILE_OVERRIDES, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_watcher_event_ignored_status_task = tracked_task(_LABEL_TASK_DERIVE_WATCHER_EVENT_IGNORED_STATUS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_chmod_only_decisions_task = tracked_task(_LABEL_TASK_DERIVE_CHMOD_ONLY_DECISIONS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var filter_classified_events_for_processing_task = tracked_task(_LABEL_TASK_FILTER_CLASSIFIED_EVENTS_FOR_PROCESSING, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var build_event_hooks_for_processing_task = tracked_task(_LABEL_TASK_BUILD_EVENT_HOOKS_FOR_PROCESSING, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := build_normalized_changed_file_paths_by_watched_pattern_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := build_skip_duplicate_hooks_by_classified_event_index_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := build_hook_contexts_for_events_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var build_normalized_changed_file_paths_by_watched_pattern_task = tracked_task(_LABEL_TASK_BUILD_NORMALIZED_CHANGED_FILE_PATHS_BY_WATCHED_PATTERN, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var build_skip_duplicate_hooks_by_classified_event_index_task = tracked_task(_LABEL_TASK_BUILD_SKIP_DUPLICATE_HOOKS_BY_CLASSIFIED_EVENT_INDEX, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var build_hook_contexts_for_events_task = tracked_task(_LABEL_TASK_BUILD_HOOK_CONTEXTS_FOR_EVENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_event_execution_flow_decision_task = tracked_task(_LABEL_TASK_DERIVE_EVENT_EXECUTION_FLOW_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (event_execution_flow_decision, error) {

	if _, err := apply_control_flow_short_circuit_side_effects_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return event_execution_flow_decision{}, err
	}

	selected_app_stop_strategy, err := derive_event_execution_plan_behavioral_decision_task.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return event_execution_flow_decision{}, err
	}
	should_short_circuit_for_config_restart, err := derive_config_restart_short_circuit_decision_task.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return event_execution_flow_decision{}, err
	}
	should_short_circuit_for_build_retry_wait, err := derive_waiting_for_build_retry_short_circuit_decision_task.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return event_execution_flow_decision{}, err
	}
	if _, err := build_watcher_event_log_payloads_for_events_with_hooks_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return event_execution_flow_decision{}, err
	}
	if _, err := build_watcher_event_execution_input_from_planning_result_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return event_execution_flow_decision{}, err
	}
	if _, err := log_watcher_batch_from_planned_payloads_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return event_execution_flow_decision{}, err
	}
	should_short_circuit_for_noop_batch, err := derive_noop_pipeline_short_circuit_decision_task.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return event_execution_flow_decision{}, err
	}
	derived_input := input
	derived_input.app_stop_strategy = selected_app_stop_strategy
	derived_input.should_short_circuit_for_config_restart = should_short_circuit_for_config_restart
	derived_input.should_short_circuit_for_build_retry_wait = should_short_circuit_for_build_retry_wait
	derived_input.should_short_circuit_for_noop_batch = should_short_circuit_for_noop_batch
	if _, err := apply_control_flow_short_circuit_side_effects_task.Run(
		tasks_ctx,
		derived_input,
	); err != nil {
		return event_execution_flow_decision{}, err
	}
	return event_execution_flow_decision{
		app_stop_strategy:                         selected_app_stop_strategy,
		should_short_circuit_for_config_restart:   should_short_circuit_for_config_restart,
		should_short_circuit_for_build_retry_wait: should_short_circuit_for_build_retry_wait,
		should_short_circuit_for_noop_batch:       should_short_circuit_for_noop_batch,
	}, nil
},
)

var derive_event_execution_plan_behavioral_decision_task = tracked_task(_LABEL_TASK_DERIVE_EVENT_EXECUTION_PLAN_BEHAVIORAL_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (app_stop_strategy, error) {

	return input.app_stop_strategy, nil
},
)

var derive_config_restart_short_circuit_decision_task = tracked_task(_LABEL_TASK_DERIVE_CONFIG_RESTART_SHORT_CIRCUIT_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (bool, error) {

	return input.should_short_circuit_for_config_restart, nil
},
)

var derive_waiting_for_build_retry_short_circuit_decision_task = tracked_task(_LABEL_TASK_DERIVE_WAITING_FOR_BUILD_RETRY_SHORT_CIRCUIT_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (bool, error) {

	return input.should_short_circuit_for_build_retry_wait, nil
},
)

var build_watcher_event_log_payloads_for_events_with_hooks_task = tracked_task(_LABEL_TASK_BUILD_WATCHER_EVENT_LOG_PAYLOADS_FOR_EVENTS_WITH_HOOKS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var build_watcher_event_execution_input_from_planning_result_task = tracked_task(_LABEL_TASK_BUILD_WATCHER_EVENT_EXECUTION_INPUT_FROM_PLANNING_RESULT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var log_watcher_batch_from_planned_payloads_task = tracked_task(_LABEL_TASK_LOG_WATCHER_BATCH_FROM_PLANNED_PAYLOADS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_noop_pipeline_short_circuit_decision_task = tracked_task(_LABEL_TASK_DERIVE_NOOP_PIPELINE_SHORT_CIRCUIT_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (bool, error) {

	return input.should_short_circuit_for_noop_batch, nil
},
)

var apply_control_flow_short_circuit_side_effects_task = tracked_task(_LABEL_TASK_APPLY_CONTROL_FLOW_SHORT_CIRCUIT_SIDE_EFFECTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if input.should_short_circuit_for_config_restart {
		if _, err := broadcast_rebuilding_overlay_for_config_restart_task.Run(
			tasks_ctx,
			input,
		); err != nil {
			return struct{}{}, err
		}
		if _, err := restart_dev_server_cycle_task.Run(tasks_ctx, input); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	}
	if input.should_short_circuit_for_build_retry_wait {
		if _, err := emit_build_retry_wait_notice_task.Run(tasks_ctx, input); err != nil {
			return struct{}{}, err
		}
		if _, err := queue_retry_wait_restart_task.Run(
			tasks_ctx,
			input,
		); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	}
	if input.should_short_circuit_for_noop_batch {
		if _, err := publish_no_reload_needed_notice_task.Run(tasks_ctx, input); err != nil {
			return struct{}{}, err
		}
	}
	return struct{}{}, nil
},
)

var broadcast_rebuilding_overlay_for_config_restart_task = tracked_task(_LABEL_TASK_BROADCAST_REBUILDING_OVERLAY_FOR_CONFIG_RESTART, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var emit_build_retry_wait_notice_task = tracked_task(_LABEL_TASK_EMIT_BUILD_RETRY_WAIT_NOTICE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_initial_requested_outcomes_task = tracked_task(_LABEL_TASK_DERIVE_INITIAL_REQUESTED_OUTCOMES, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_app_requested_outcomes_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_framework_requested_outcomes_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_initial_build_and_browser_intents_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_app_requested_outcomes_task = tracked_task(_LABEL_TASK_DERIVE_APP_REQUESTED_OUTCOMES, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_framework_requested_outcomes_task = tracked_task(_LABEL_TASK_DERIVE_FRAMEWORK_REQUESTED_OUTCOMES, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_initial_build_and_browser_intents_task = tracked_task(_LABEL_TASK_DERIVE_INITIAL_BUILD_AND_BROWSER_INTENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

/////////////////////////////////////////////////////////////////////
/////// Phase 2 Tasks
/////////////////////////////////////////////////////////////////////

var apply_pre_hook_adjustments_task = tracked_task(_LABEL_TASK_APPLY_PRE_HOOK_ADJUSTMENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := prepare_execution_pipeline_for_events_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := fire_concurrent_no_wait_hooks_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := run_pre_hooks_for_events_with_errors_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := apply_pre_hook_refresh_actions_to_work_set_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := continue_pipeline_after_pre_hook_stage_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var prepare_execution_pipeline_for_events_task = tracked_task(_LABEL_TASK_PREPARE_EXECUTION_PIPELINE_FOR_EVENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_events_with_hooks_for_execution_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	selected_app_stop_strategy, err := derive_app_stop_strategy_for_execution_task.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return struct{}{}, err
	}
	execution_pipeline_input := input
	execution_pipeline_input.app_stop_strategy = selected_app_stop_strategy
	if _, err := apply_app_stop_strategy_for_execution_task.Run(
		tasks_ctx,
		execution_pipeline_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_events_with_hooks_for_execution_task = tracked_task(_LABEL_TASK_DERIVE_EVENTS_WITH_HOOKS_FOR_EXECUTION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_app_stop_strategy_for_execution_task = tracked_task(_LABEL_TASK_DERIVE_APP_STOP_STRATEGY_FOR_EXECUTION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (app_stop_strategy, error) {

	return input.app_stop_strategy, nil
},
)

var apply_app_stop_strategy_for_execution_task = tracked_task(_LABEL_TASK_APPLY_APP_STOP_STRATEGY_FOR_EXECUTION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	switch input.app_stop_strategy {
	case app_stop_strategy_none:
		return struct{}{}, nil
	case app_stop_strategy_single_event_hard_reload:
		if _, err := stop_app_runtime_for_single_event_hard_reload_task.Run(
			tasks_ctx,
			input,
		); err != nil {
			return struct{}{}, err
		}
	case app_stop_strategy_batch_hard_reload:
		if _, err := stop_app_runtime_for_batch_hard_reload_task.Run(
			tasks_ctx,
			input,
		); err != nil {
			return struct{}{}, err
		}
		if _, err := mark_hook_contexts_app_stopped_for_batch_task.Run(
			tasks_ctx,
			input,
		); err != nil {
			return struct{}{}, err
		}
	default:
		return struct{}{}, fmt.Errorf(
			"wavebuild: unsupported app stop strategy %q",
			input.app_stop_strategy,
		)
	}
	return struct{}{}, nil
},
)

var stop_app_runtime_for_single_event_hard_reload_task = tracked_task(_LABEL_TASK_STOP_APP_RUNTIME_FOR_SINGLE_EVENT_HARD_RELOAD, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var stop_app_runtime_for_batch_hard_reload_task = tracked_task(_LABEL_TASK_STOP_APP_RUNTIME_FOR_BATCH_HARD_RELOAD, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var mark_hook_contexts_app_stopped_for_batch_task = tracked_task(_LABEL_TASK_MARK_HOOK_CONTEXTS_APP_STOPPED_FOR_BATCH, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var fire_concurrent_no_wait_hooks_task = tracked_task(_LABEL_TASK_FIRE_CONCURRENT_NO_WAIT_HOOKS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_hook_stage_execution_descriptors_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_concurrent_no_wait_hook_execution_plans_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := get_or_create_concurrent_no_wait_hook_lifecycle_context_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if err := tasks_ctx.RunParallel(
		launch_concurrent_no_wait_hook_callbacks_task.Bind(input, nil),
		launch_concurrent_no_wait_hook_commands_task.Bind(input, nil),
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_hook_stage_execution_descriptors_task = tracked_task(_LABEL_TASK_DERIVE_HOOK_STAGE_EXECUTION_DESCRIPTORS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_concurrent_no_wait_hook_execution_plans_task = tracked_task(_LABEL_TASK_DERIVE_CONCURRENT_NO_WAIT_HOOK_EXECUTION_PLANS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var get_or_create_concurrent_no_wait_hook_lifecycle_context_task = tracked_task(_LABEL_TASK_GET_OR_CREATE_CONCURRENT_NO_WAIT_HOOK_LIFECYCLE_CONTEXT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var launch_concurrent_no_wait_hook_callbacks_task = tracked_task(_LABEL_TASK_LAUNCH_CONCURRENT_NO_WAIT_HOOK_CALLBACKS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	concurrent_no_wait_hook_stage_input := input
	concurrent_no_wait_hook_stage_input.hook_stage = hook_stage_concurrent_no_wait
	if _, err := derive_hook_callback_timeout_for_execution_plan_task.Run(
		tasks_ctx,
		concurrent_no_wait_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := clone_hook_context_for_execution_task.Run(
		tasks_ctx,
		concurrent_no_wait_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := run_no_wait_hook_with_concurrency_limit_task.Run(
		tasks_ctx,
		concurrent_no_wait_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := execute_hook_callback_safely_task.Run(
		tasks_ctx,
		concurrent_no_wait_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := tolerate_no_wait_hook_callback_failures_task.Run(
		tasks_ctx,
		concurrent_no_wait_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var launch_concurrent_no_wait_hook_commands_task = tracked_task(_LABEL_TASK_LAUNCH_CONCURRENT_NO_WAIT_HOOK_COMMANDS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	concurrent_no_wait_hook_stage_input := input
	concurrent_no_wait_hook_stage_input.hook_stage = hook_stage_concurrent_no_wait
	if _, err := derive_hook_command_timeout_for_execution_plan_task.Run(
		tasks_ctx,
		concurrent_no_wait_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := run_no_wait_hook_with_concurrency_limit_task.Run(
		tasks_ctx,
		concurrent_no_wait_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := execute_hook_command_with_context_task.Run(
		tasks_ctx,
		concurrent_no_wait_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := tolerate_no_wait_hook_command_failures_task.Run(
		tasks_ctx,
		concurrent_no_wait_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var run_pre_hooks_for_events_with_errors_task = tracked_task(_LABEL_TASK_RUN_PRE_HOOKS_FOR_EVENTS_WITH_ERRORS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := add_implicit_work_for_non_run_on_change_events_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_hook_stage_execution_descriptors_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := run_pre_hooks_for_each_event_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var add_implicit_work_for_non_run_on_change_events_task = tracked_task(_LABEL_TASK_ADD_IMPLICIT_WORK_FOR_NON_RUN_ON_CHANGE_EVENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var run_pre_hooks_for_each_event_task = tracked_task(_LABEL_TASK_RUN_PRE_HOOKS_FOR_EACH_EVENT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_pre_hook_execution_plans_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := execute_pre_hook_execution_plans_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_pre_hook_execution_plans_task = tracked_task(_LABEL_TASK_DERIVE_PRE_HOOK_EXECUTION_PLANS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var execute_pre_hook_execution_plans_task = tracked_task(_LABEL_TASK_EXECUTE_PRE_HOOK_EXECUTION_PLANS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	pre_hook_stage_input := input
	pre_hook_stage_input.hook_stage = hook_stage_pre
	if _, err := execute_hook_execution_plan_with_context_task.Run(
		tasks_ctx,
		pre_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var execute_hook_execution_plan_with_context_task = tracked_task(_LABEL_TASK_EXECUTE_HOOK_EXECUTION_PLAN_WITH_CONTEXT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_hook_callback_timeout_for_execution_plan_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_hook_command_timeout_for_execution_plan_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := execute_hook_callback_safely_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := execute_hook_command_with_context_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_hook_callback_timeout_for_execution_plan_task = tracked_task(_LABEL_TASK_DERIVE_HOOK_CALLBACK_TIMEOUT_FOR_EXECUTION_PLAN, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_hook_command_timeout_for_execution_plan_task = tracked_task(_LABEL_TASK_DERIVE_HOOK_COMMAND_TIMEOUT_FOR_EXECUTION_PLAN, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var execute_hook_callback_safely_task = tracked_task(_LABEL_TASK_EXECUTE_HOOK_CALLBACK_SAFELY, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var execute_hook_command_with_context_task = tracked_task(_LABEL_TASK_EXECUTE_HOOK_COMMAND_WITH_CONTEXT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var clone_hook_context_for_execution_task = tracked_task(_LABEL_TASK_CLONE_HOOK_CONTEXT_FOR_EXECUTION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var run_no_wait_hook_with_concurrency_limit_task = tracked_task(_LABEL_TASK_RUN_NO_WAIT_HOOK_WITH_CONCURRENCY_LIMIT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var tolerate_no_wait_hook_callback_failures_task = tracked_task(_LABEL_TASK_TOLERATE_NO_WAIT_HOOK_CALLBACK_FAILURES, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var tolerate_no_wait_hook_command_failures_task = tracked_task(_LABEL_TASK_TOLERATE_NO_WAIT_HOOK_COMMAND_FAILURES, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var apply_pre_hook_refresh_actions_to_work_set_task = tracked_task(_LABEL_TASK_APPLY_PRE_HOOK_REFRESH_ACTIONS_TO_WORK_SET, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	pre_hook_stage_input := input
	pre_hook_stage_input.hook_stage = hook_stage_pre
	if _, err := reduce_refresh_actions_in_stable_order_task.Run(
		tasks_ctx,
		pre_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := apply_refresh_action_work_mutations_task.Run(
		tasks_ctx,
		pre_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var reduce_refresh_actions_in_stable_order_task = tracked_task(_LABEL_TASK_REDUCE_REFRESH_ACTIONS_IN_STABLE_ORDER, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var apply_refresh_action_work_mutations_task = tracked_task(_LABEL_TASK_APPLY_REFRESH_ACTION_WORK_MUTATIONS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var continue_pipeline_after_pre_hook_stage_task = tracked_task(_LABEL_TASK_CONTINUE_PIPELINE_AFTER_PRE_HOOK_STAGE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	pre_hook_stage_input := input
	pre_hook_stage_input.hook_stage = hook_stage_pre
	if _, err := derive_hook_stage_failure_policy_task.Run(
		tasks_ctx,
		pre_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_hook_stage_continuation_decision_task.Run(
		tasks_ctx,
		pre_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := trigger_restart_from_refresh_actions_task.Run(
		tasks_ctx,
		pre_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_hook_stage_failure_policy_task = tracked_task(_LABEL_TASK_DERIVE_HOOK_STAGE_FAILURE_POLICY, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_hook_stage_continuation_decision_task = tracked_task(_LABEL_TASK_DERIVE_HOOK_STAGE_CONTINUATION_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var trigger_restart_from_refresh_actions_task = tracked_task(_LABEL_TASK_TRIGGER_RESTART_FROM_REFRESH_ACTIONS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

/////////////////////////////////////////////////////////////////////
/////// Phase 3 Tasks
/////////////////////////////////////////////////////////////////////

var execute_materialization_build_lane_task = tracked_task(_LABEL_TASK_EXECUTE_MATERIALIZATION_BUILD_LANE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := ensure_dist_output_directories_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := materialize_wave_metadata_state_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	parallel_calls := make([]tasks.BoundTask, 0, 4)
	should_materialize_any_build_state := false
	if input.batch_event_categorization.build_go_binary {
		parallel_calls = append(
			parallel_calls,
			materialize_go_binary_state_task.Bind(input, nil),
		)
		should_materialize_any_build_state = true
	}
	if input.batch_event_categorization.build_critical_css ||
		input.batch_event_categorization.build_normal_css {
		parallel_calls = append(
			parallel_calls,
			materialize_css_state_task.Bind(input, nil),
		)
		should_materialize_any_build_state = true
	}
	if input.batch_event_categorization.process_public_static ||
		input.batch_event_categorization.process_private_static {
		parallel_calls = append(
			parallel_calls,
			materialize_static_asset_state_task.Bind(input, nil),
		)
		should_materialize_any_build_state = true
	}
	if should_materialize_any_build_state {
		parallel_calls = append(
			parallel_calls,
			materialize_frontend_bundle_state_task.Bind(input, nil),
		)
	}
	if len(parallel_calls) == 0 {
		return struct{}{}, nil
	}
	if err := tasks_ctx.RunParallel(parallel_calls...); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var materialize_wave_metadata_state_task = tracked_task(_LABEL_TASK_MATERIALIZE_WAVE_METADATA_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if err := tasks_ctx.RunParallel(
		emit_runtime_config_artifact_task.Bind(input, nil),
		write_schema_artifact_task.Bind(input, nil),
		ensure_dist_keep_file_task.Bind(input, nil),
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var ensure_dist_output_directories_task = tracked_task(_LABEL_TASK_ENSURE_DIST_OUTPUT_DIRECTORIES, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var emit_runtime_config_artifact_task = tracked_task(_LABEL_TASK_EMIT_RUNTIME_CONFIG_ARTIFACT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var write_schema_artifact_task = tracked_task(_LABEL_TASK_WRITE_SCHEMA_ARTIFACT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var ensure_dist_keep_file_task = tracked_task(_LABEL_TASK_ENSURE_DIST_KEEP_FILE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var materialize_go_binary_state_task = tracked_task(_LABEL_TASK_MATERIALIZE_GO_BINARY_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := prepare_go_build_overlay_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := ensure_go_binary_output_directory_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := compile_go_binary_for_mode_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := cleanup_go_build_overlay_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var prepare_go_build_overlay_task = tracked_task(_LABEL_TASK_PREPARE_GO_BUILD_OVERLAY, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var ensure_go_binary_output_directory_task = tracked_task(_LABEL_TASK_ENSURE_GO_BINARY_OUTPUT_DIRECTORY, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var compile_go_binary_for_mode_task = tracked_task(_LABEL_TASK_COMPILE_GO_BINARY_FOR_MODE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var cleanup_go_build_overlay_task = tracked_task(_LABEL_TASK_CLEANUP_GO_BUILD_OVERLAY, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var materialize_css_state_task = tracked_task(_LABEL_TASK_MATERIALIZE_CSS_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := materialize_critical_css_state_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := materialize_normal_css_state_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var materialize_critical_css_state_task = tracked_task(_LABEL_TASK_MATERIALIZE_CRITICAL_CSS_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_critical_css_build_inputs_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := build_critical_css_pipeline_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := write_critical_css_artifact_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_critical_css_build_inputs_task = tracked_task(_LABEL_TASK_DERIVE_CRITICAL_CSS_BUILD_INPUTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var build_critical_css_pipeline_task = tracked_task(_LABEL_TASK_BUILD_CRITICAL_CSS_PIPELINE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var write_critical_css_artifact_task = tracked_task(_LABEL_TASK_WRITE_CRITICAL_CSS_ARTIFACT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var materialize_normal_css_state_task = tracked_task(_LABEL_TASK_MATERIALIZE_NORMAL_CSS_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_normal_css_build_inputs_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := build_normal_css_pipeline_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := write_normal_css_artifact_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := write_normal_css_ref_artifact_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_normal_css_build_inputs_task = tracked_task(_LABEL_TASK_DERIVE_NORMAL_CSS_BUILD_INPUTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var build_normal_css_pipeline_task = tracked_task(_LABEL_TASK_BUILD_NORMAL_CSS_PIPELINE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var write_normal_css_artifact_task = tracked_task(_LABEL_TASK_WRITE_NORMAL_CSS_ARTIFACT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var write_normal_css_ref_artifact_task = tracked_task(_LABEL_TASK_WRITE_NORMAL_CSS_REF_ARTIFACT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var materialize_static_asset_state_task = tracked_task(_LABEL_TASK_MATERIALIZE_STATIC_ASSET_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := materialize_public_asset_state_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := materialize_private_asset_state_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var materialize_public_asset_state_task = tracked_task(_LABEL_TASK_MATERIALIZE_PUBLIC_ASSET_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := snapshot_public_file_map_artifacts_before_processing_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_public_static_processing_mode_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := process_public_static_assets_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := cleanup_stale_public_static_outputs_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := save_public_filemap_gob_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := write_canonical_public_file_map_json_and_ref_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_public_file_map_artifact_change_or_repair_decision_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := apply_public_file_map_artifact_change_side_effects_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var snapshot_public_file_map_artifacts_before_processing_task = tracked_task(_LABEL_TASK_SNAPSHOT_PUBLIC_FILE_MAP_ARTIFACTS_BEFORE_PROCESSING, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_public_static_processing_mode_task = tracked_task(_LABEL_TASK_DERIVE_PUBLIC_STATIC_PROCESSING_MODE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var process_public_static_assets_task = tracked_task(_LABEL_TASK_PROCESS_PUBLIC_STATIC_ASSETS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var cleanup_stale_public_static_outputs_task = tracked_task(_LABEL_TASK_CLEANUP_STALE_PUBLIC_STATIC_OUTPUTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var save_public_filemap_gob_task = tracked_task(_LABEL_TASK_SAVE_PUBLIC_FILEMAP_GOB, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var write_canonical_public_file_map_json_and_ref_task = tracked_task(_LABEL_TASK_WRITE_CANONICAL_PUBLIC_FILE_MAP_JSON_AND_REF, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_public_file_map_artifact_change_or_repair_decision_task = tracked_task(_LABEL_TASK_DERIVE_PUBLIC_FILE_MAP_ARTIFACT_CHANGE_OR_REPAIR_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var apply_public_file_map_artifact_change_side_effects_task = tracked_task(_LABEL_TASK_APPLY_PUBLIC_FILE_MAP_ARTIFACT_CHANGE_SIDE_EFFECTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := call_framework_runtime_reload_endpoint_for_public_file_map_change_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := run_framework_build_hook_for_public_file_map_change_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := clear_invalidate_vite_browser_action_when_public_file_map_unchanged_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var call_framework_runtime_reload_endpoint_for_public_file_map_change_task = tracked_task(_LABEL_TASK_CALL_FRAMEWORK_RUNTIME_RELOAD_ENDPOINT_FOR_PUBLIC_FILE_MAP_CHANGE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var run_framework_build_hook_for_public_file_map_change_task = tracked_task(_LABEL_TASK_RUN_FRAMEWORK_BUILD_HOOK_FOR_PUBLIC_FILE_MAP_CHANGE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var clear_invalidate_vite_browser_action_when_public_file_map_unchanged_task = tracked_task(_LABEL_TASK_CLEAR_INVALIDATE_VITE_BROWSER_ACTION_WHEN_PUBLIC_FILE_MAP_UNCHANGED, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var materialize_private_asset_state_task = tracked_task(_LABEL_TASK_MATERIALIZE_PRIVATE_ASSET_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_private_static_processing_mode_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := process_private_static_assets_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := save_private_filemap_gob_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_private_static_processing_mode_task = tracked_task(_LABEL_TASK_DERIVE_PRIVATE_STATIC_PROCESSING_MODE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var process_private_static_assets_task = tracked_task(_LABEL_TASK_PROCESS_PRIVATE_STATIC_ASSETS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var save_private_filemap_gob_task = tracked_task(_LABEL_TASK_SAVE_PRIVATE_FILEMAP_GOB, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var materialize_frontend_bundle_state_task = tracked_task(_LABEL_TASK_MATERIALIZE_FRONTEND_BUNDLE_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := run_vite_prod_build_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := emit_vite_manifest_artifact_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var run_vite_prod_build_task = tracked_task(_LABEL_TASK_RUN_VITE_PROD_BUILD, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var emit_vite_manifest_artifact_task = tracked_task(_LABEL_TASK_EMIT_VITE_MANIFEST_ARTIFACT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var execute_concurrent_hooks_task = tracked_task(_LABEL_TASK_EXECUTE_CONCURRENT_HOOKS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := run_concurrent_hooks_for_events_with_context_and_errors_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := apply_concurrent_hook_refresh_actions_to_work_set_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := continue_pipeline_after_concurrent_hook_stage_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var run_concurrent_hooks_for_events_with_context_and_errors_task = tracked_task(_LABEL_TASK_RUN_CONCURRENT_HOOKS_FOR_EVENTS_WITH_CONTEXT_AND_ERRORS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_hook_stage_execution_descriptors_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := run_concurrent_hooks_for_each_event_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := preserve_concurrent_hook_actions_in_event_order_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := join_concurrent_hook_execution_errors_in_order_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var preserve_concurrent_hook_actions_in_event_order_task = tracked_task(_LABEL_TASK_PRESERVE_CONCURRENT_HOOK_ACTIONS_IN_EVENT_ORDER, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var join_concurrent_hook_execution_errors_in_order_task = tracked_task(_LABEL_TASK_JOIN_CONCURRENT_HOOK_EXECUTION_ERRORS_IN_ORDER, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var run_concurrent_hooks_for_each_event_task = tracked_task(_LABEL_TASK_RUN_CONCURRENT_HOOKS_FOR_EACH_EVENT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_concurrent_hook_execution_plans_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := execute_concurrent_hook_execution_plans_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_concurrent_hook_execution_plans_task = tracked_task(_LABEL_TASK_DERIVE_CONCURRENT_HOOK_EXECUTION_PLANS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var execute_concurrent_hook_execution_plans_task = tracked_task(_LABEL_TASK_EXECUTE_CONCURRENT_HOOK_EXECUTION_PLANS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	concurrent_hook_stage_input := input
	concurrent_hook_stage_input.hook_stage = hook_stage_concurrent
	if _, err := execute_hook_execution_plan_with_context_task.Run(
		tasks_ctx,
		concurrent_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var apply_concurrent_hook_refresh_actions_to_work_set_task = tracked_task(_LABEL_TASK_APPLY_CONCURRENT_HOOK_REFRESH_ACTIONS_TO_WORK_SET, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	concurrent_hook_stage_input := input
	concurrent_hook_stage_input.hook_stage = hook_stage_concurrent
	if _, err := reduce_refresh_actions_in_stable_order_task.Run(
		tasks_ctx,
		concurrent_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := apply_refresh_action_work_mutations_task.Run(
		tasks_ctx,
		concurrent_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var continue_pipeline_after_concurrent_hook_stage_task = tracked_task(_LABEL_TASK_CONTINUE_PIPELINE_AFTER_CONCURRENT_HOOK_STAGE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	concurrent_hook_stage_input := input
	concurrent_hook_stage_input.hook_stage = hook_stage_concurrent
	if _, err := derive_hook_stage_failure_policy_task.Run(
		tasks_ctx,
		concurrent_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_hook_stage_continuation_decision_task.Run(
		tasks_ctx,
		concurrent_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := trigger_restart_from_refresh_actions_task.Run(
		tasks_ctx,
		concurrent_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

/////////////////////////////////////////////////////////////////////
/////// Phase 4 Tasks
/////////////////////////////////////////////////////////////////////

var apply_post_hook_adjustments_task = tracked_task(_LABEL_TASK_APPLY_POST_HOOK_ADJUSTMENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := run_post_hooks_for_events_with_errors_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := apply_post_hook_refresh_actions_to_work_set_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := continue_pipeline_after_post_hook_stage_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var run_post_hooks_for_events_with_errors_task = tracked_task(_LABEL_TASK_RUN_POST_HOOKS_FOR_EVENTS_WITH_ERRORS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_hook_stage_execution_descriptors_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := run_post_hooks_for_each_event_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var run_post_hooks_for_each_event_task = tracked_task(_LABEL_TASK_RUN_POST_HOOKS_FOR_EACH_EVENT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := derive_post_hook_execution_plans_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := execute_post_hook_execution_plans_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var derive_post_hook_execution_plans_task = tracked_task(_LABEL_TASK_DERIVE_POST_HOOK_EXECUTION_PLANS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var execute_post_hook_execution_plans_task = tracked_task(_LABEL_TASK_EXECUTE_POST_HOOK_EXECUTION_PLANS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	post_hook_stage_input := input
	post_hook_stage_input.hook_stage = hook_stage_post
	if _, err := execute_hook_execution_plan_with_context_task.Run(
		tasks_ctx,
		post_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var apply_post_hook_refresh_actions_to_work_set_task = tracked_task(_LABEL_TASK_APPLY_POST_HOOK_REFRESH_ACTIONS_TO_WORK_SET, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	post_hook_stage_input := input
	post_hook_stage_input.hook_stage = hook_stage_post
	if _, err := reduce_refresh_actions_in_stable_order_task.Run(
		tasks_ctx,
		post_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := apply_refresh_action_work_mutations_task.Run(
		tasks_ctx,
		post_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var continue_pipeline_after_post_hook_stage_task = tracked_task(_LABEL_TASK_CONTINUE_PIPELINE_AFTER_POST_HOOK_STAGE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	post_hook_stage_input := input
	post_hook_stage_input.hook_stage = hook_stage_post
	if _, err := derive_hook_stage_failure_policy_task.Run(
		tasks_ctx,
		post_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_hook_stage_continuation_decision_task.Run(
		tasks_ctx,
		post_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := trigger_restart_from_refresh_actions_task.Run(
		tasks_ctx,
		post_hook_stage_input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

/////////////////////////////////////////////////////////////////////
/////// Phase 5 Tasks
/////////////////////////////////////////////////////////////////////

var apply_backend_mutation_plan_task = tracked_task(_LABEL_TASK_APPLY_BACKEND_MUTATION_PLAN, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (backend_mutation_branch, error) {

	selected_backend_mutation_branch, err := derive_backend_mutation_plan_task.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return "", err
	}
	selected_backend_mutation_input := input
	selected_backend_mutation_input.backend_mutation_branch = selected_backend_mutation_branch
	if _, err := execute_selected_backend_mutation_branch_task.Run(
		tasks_ctx,
		selected_backend_mutation_input,
	); err != nil {
		return "", err
	}
	return selected_backend_mutation_branch, nil
},
)

var execute_selected_backend_mutation_branch_task = tracked_task(_LABEL_TASK_EXECUTE_SELECTED_BACKEND_MUTATION_BRANCH, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	switch input.backend_mutation_branch {
	case backend_mutation_branch_queue_retry_wait_restart:
		_, err := queue_retry_wait_restart_task.Run(tasks_ctx, input)
		return struct{}{}, err
	case backend_mutation_branch_restart_dev_server_cycle:
		_, err := restart_dev_server_cycle_task.Run(tasks_ctx, input)
		return struct{}{}, err
	case backend_mutation_branch_apply_normal_backend_mutations:
		_, err := apply_normal_backend_mutations_task.Run(tasks_ctx, input)
		return struct{}{}, err
	default:
		return struct{}{}, fmt.Errorf(
			"wavebuild: unsupported backend mutation branch %q",
			input.backend_mutation_branch,
		)
	}
},
)

var derive_backend_mutation_plan_task = tracked_task(_LABEL_TASK_DERIVE_BACKEND_MUTATION_PLAN, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (backend_mutation_branch, error) {

	if _, err := derive_run_build_execution_ordering_decision_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return "", err
	}
	if _, err := resolve_queued_restart_request_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return "", err
	}
	if _, err := merge_restart_requests_task.Run(tasks_ctx, input); err != nil {
		return "", err
	}
	if _, err := consume_pending_restart_request_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return "", err
	}
	return input.backend_mutation_branch, nil
},
)

var derive_run_build_execution_ordering_decision_task = tracked_task(_LABEL_TASK_DERIVE_RUN_BUILD_EXECUTION_ORDERING_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var resolve_queued_restart_request_task = tracked_task(_LABEL_TASK_RESOLVE_QUEUED_RESTART_REQUEST, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var merge_restart_requests_task = tracked_task(_LABEL_TASK_MERGE_RESTART_REQUESTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var consume_pending_restart_request_task = tracked_task(_LABEL_TASK_CONSUME_PENDING_RESTART_REQUEST, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var queue_retry_wait_restart_task = tracked_task(_LABEL_TASK_QUEUE_RETRY_WAIT_RESTART, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := normalize_restart_request_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := queue_restart_request_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := set_waiting_for_build_retry_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var normalize_restart_request_task = tracked_task(_LABEL_TASK_NORMALIZE_RESTART_REQUEST, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var queue_restart_request_task = tracked_task(_LABEL_TASK_QUEUE_RESTART_REQUEST, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var set_waiting_for_build_retry_task = tracked_task(_LABEL_TASK_SET_WAITING_FOR_BUILD_RETRY, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var restart_dev_server_cycle_task = tracked_task(_LABEL_TASK_RESTART_DEV_SERVER_CYCLE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := cancel_and_join_current_run_cycle_scope_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := queue_config_restart_request_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var cancel_and_join_current_run_cycle_scope_task = tracked_task(_LABEL_TASK_CANCEL_AND_JOIN_CURRENT_RUN_CYCLE_SCOPE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var queue_config_restart_request_task = tracked_task(_LABEL_TASK_QUEUE_CONFIG_RESTART_REQUEST, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var apply_normal_backend_mutations_task = tracked_task(_LABEL_TASK_APPLY_NORMAL_BACKEND_MUTATIONS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if !input.batch_event_categorization.requires_backend_restart {
		return struct{}{}, nil
	}
	if _, err := restart_app_runtime_process_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := restart_vite_runtime_process_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := execute_framework_backend_mutation_effects_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var restart_app_runtime_process_task = tracked_task(_LABEL_TASK_RESTART_APP_RUNTIME_PROCESS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := stop_app_runtime_process_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := start_app_runtime_process_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var stop_app_runtime_process_task = tracked_task(_LABEL_TASK_STOP_APP_RUNTIME_PROCESS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var start_app_runtime_process_task = tracked_task(_LABEL_TASK_START_APP_RUNTIME_PROCESS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var restart_vite_runtime_process_task = tracked_task(_LABEL_TASK_RESTART_VITE_RUNTIME_PROCESS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var execute_framework_backend_mutation_effects_task = tracked_task(_LABEL_TASK_EXECUTE_FRAMEWORK_BACKEND_MUTATION_EFFECTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

/////////////////////////////////////////////////////////////////////
/////// Phase 6 Tasks
/////////////////////////////////////////////////////////////////////

var converge_backend_state_task = tracked_task(_LABEL_TASK_CONVERGE_BACKEND_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (bool, error) {

	if !input.batch_event_categorization.requires_backend_restart {
		return input.should_request_loop_back, nil
	}
	if _, err := derive_backend_convergence_requirements_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return false, err
	}
	if err := tasks_ctx.RunParallel(
		await_app_readiness_if_required_task.Bind(input, nil),
		await_vite_readiness_if_required_task.Bind(input, nil),
	); err != nil {
		return false, err
	}
	if _, err := execute_framework_runtime_reload_requests_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return false, err
	}
	if _, err := execute_framework_backend_notifications_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return false, err
	}
	should_request_loop_back, err := apply_framework_notification_failure_policy_task.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return false, err
	}
	return should_request_loop_back, nil
},
)

var derive_backend_convergence_requirements_task = tracked_task(_LABEL_TASK_DERIVE_BACKEND_CONVERGENCE_REQUIREMENTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var await_app_readiness_if_required_task = tracked_task(_LABEL_TASK_AWAIT_APP_READINESS_IF_REQUIRED, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	_ = input
	return struct{}{}, nil
},
)

var await_vite_readiness_if_required_task = tracked_task(_LABEL_TASK_AWAIT_VITE_READINESS_IF_REQUIRED, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var execute_framework_runtime_reload_requests_task = tracked_task(_LABEL_TASK_EXECUTE_FRAMEWORK_RUNTIME_RELOAD_REQUESTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var execute_framework_backend_notifications_task = tracked_task(_LABEL_TASK_EXECUTE_FRAMEWORK_BACKEND_NOTIFICATIONS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var apply_framework_notification_failure_policy_task = tracked_task(_LABEL_TASK_APPLY_FRAMEWORK_NOTIFICATION_FAILURE_POLICY, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (bool, error) {

	should_request_loop_back, err := derive_framework_notification_failure_policy_decisions_task.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return false, err
	}
	if should_request_loop_back {
		if _, err := request_backend_healing_restart_without_go_compile_task.Run(
			tasks_ctx,
			input,
		); err != nil {
			return false, err
		}
	}
	return should_request_loop_back, nil
},
)

var execute_selected_frontend_terminal_action_task = tracked_task(_LABEL_TASK_EXECUTE_SELECTED_FRONTEND_TERMINAL_ACTION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	switch input.frontend_terminal_action {
	case frontend_terminal_action_css_hot_reload:
		_, err := broadcast_css_hot_reload_task.Run(tasks_ctx, input)
		return struct{}{}, err
	case frontend_terminal_action_revalidate:
		_, err := broadcast_revalidate_task.Run(tasks_ctx, input)
		return struct{}{}, err
	case frontend_terminal_action_notify_vite_public_filemap_changed:
		_, err := notify_vite_public_filemap_changed_task.Run(tasks_ctx, input)
		return struct{}{}, err
	case frontend_terminal_action_hard_reload:
		_, err := broadcast_hard_reload_task.Run(tasks_ctx, input)
		return struct{}{}, err
	case frontend_terminal_action_none:
		_, err := publish_no_reload_needed_notice_task.Run(tasks_ctx, input)
		return struct{}{}, err
	default:
		return struct{}{}, fmt.Errorf(
			"wavebuild: unsupported frontend terminal action %q",
			input.frontend_terminal_action,
		)
	}
},
)

/////////////////////////////////////////////////////////////////////
/////// Phase 7 Tasks
/////////////////////////////////////////////////////////////////////

var apply_frontend_terminal_action_task = tracked_task(_LABEL_TASK_APPLY_FRONTEND_TERMINAL_ACTION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (frontend_terminal_action, error) {

	selected_frontend_terminal_action, err := derive_frontend_terminal_action_task.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return "", err
	}
	selected_frontend_terminal_action_input := input
	selected_frontend_terminal_action_input.frontend_terminal_action = selected_frontend_terminal_action
	if _, err := execute_selected_frontend_terminal_action_task.Run(
		tasks_ctx,
		selected_frontend_terminal_action_input,
	); err != nil {
		return "", err
	}
	return selected_frontend_terminal_action, nil
},
)

var derive_framework_notification_failure_policy_decisions_task = tracked_task(_LABEL_TASK_DERIVE_FRAMEWORK_NOTIFICATION_FAILURE_POLICY_DECISIONS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (bool, error) {

	return input.should_request_loop_back, nil
},
)

var request_backend_healing_restart_without_go_compile_task = tracked_task(_LABEL_TASK_REQUEST_BACKEND_HEALING_RESTART_WITHOUT_GO_COMPILE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_frontend_terminal_action_task = tracked_task(_LABEL_TASK_DERIVE_FRONTEND_TERMINAL_ACTION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (frontend_terminal_action, error) {

	return input.frontend_terminal_action, nil
},
)

var broadcast_css_hot_reload_task = tracked_task(_LABEL_TASK_BROADCAST_CSS_HOT_RELOAD, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := read_critical_css_hot_reload_payload_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := read_normal_css_hot_reload_url_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := build_css_hot_reload_payloads_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var read_critical_css_hot_reload_payload_task = tracked_task(_LABEL_TASK_READ_CRITICAL_CSS_HOT_RELOAD_PAYLOAD, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var read_normal_css_hot_reload_url_task = tracked_task(_LABEL_TASK_READ_NORMAL_CSS_HOT_RELOAD_URL, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var build_css_hot_reload_payloads_task = tracked_task(_LABEL_TASK_BUILD_CSS_HOT_RELOAD_PAYLOADS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var broadcast_revalidate_task = tracked_task(_LABEL_TASK_BROADCAST_REVALIDATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var notify_vite_public_filemap_changed_task = tracked_task(_LABEL_TASK_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := should_attempt_vite_invalidate_for_browser_decision_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := notify_vite_file_map_changed_endpoint_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := derive_notify_vite_fallback_browser_decision_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := resolve_browser_decision_after_invalidate_vite_fallback_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var should_attempt_vite_invalidate_for_browser_decision_task = tracked_task(_LABEL_TASK_SHOULD_ATTEMPT_VITE_INVALIDATE_FOR_BROWSER_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var notify_vite_file_map_changed_endpoint_task = tracked_task(_LABEL_TASK_NOTIFY_VITE_FILE_MAP_CHANGED_ENDPOINT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var derive_notify_vite_fallback_browser_decision_task = tracked_task(_LABEL_TASK_DERIVE_NOTIFY_VITE_FALLBACK_BROWSER_DECISION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var resolve_browser_decision_after_invalidate_vite_fallback_task = tracked_task(_LABEL_TASK_RESOLVE_BROWSER_DECISION_AFTER_INVALIDATE_VITE_FALLBACK, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var broadcast_hard_reload_task = tracked_task(_LABEL_TASK_BROADCAST_HARD_RELOAD, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var publish_no_reload_needed_notice_task = tracked_task(_LABEL_TASK_PUBLISH_NO_RELOAD_NEEDED_NOTICE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

/////////////////////////////////////////////////////////////////////
/////// Phase 8-9 Tasks
/////////////////////////////////////////////////////////////////////

var plan_healing_loopback_transition_task = tracked_task(_LABEL_TASK_PLAN_HEALING_LOOPBACK_TRANSITION, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (bool, error) {

	return input.should_request_loop_back, nil
},
)

var complete_batch_task = tracked_task(_LABEL_TASK_COMPLETE_BATCH, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {

	if _, err := emit_batch_completion_facts_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := clear_concurrent_no_wait_hook_lifecycle_context_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	if _, err := remove_stale_watcher_paths_task.Run(tasks_ctx, input); err != nil {
		return struct{}{}, err
	}
	if _, err := clear_batch_scoped_temporary_state_task.Run(
		tasks_ctx,
		input,
	); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, nil
},
)

var emit_batch_completion_facts_task = tracked_task(_LABEL_TASK_EMIT_BATCH_COMPLETION_FACTS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var clear_concurrent_no_wait_hook_lifecycle_context_task = tracked_task(_LABEL_TASK_CLEAR_CONCURRENT_NO_WAIT_HOOK_LIFECYCLE_CONTEXT, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var remove_stale_watcher_paths_task = tracked_task(_LABEL_TASK_REMOVE_STALE_WATCHER_PATHS, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

var clear_batch_scoped_temporary_state_task = tracked_task(_LABEL_TASK_CLEAR_BATCH_SCOPED_TEMPORARY_STATE, func(
	tasks_ctx *tasks.Ctx,
	input phase_task_input,
) (struct{}, error) {
	_ = input

	return struct{}{}, nil
},
)

/////////////////////////////////////////////////////////////////////
/////// Orchestrator Plan
/////////////////////////////////////////////////////////////////////

const (
	phase_lane_policy_advance               = "phase-lane-policy-advance"
	phase_lane_policy_control_flow_gate     = "phase-lane-policy-control-flow-gate"
	phase_lane_policy_healing_loopback_gate = "phase-lane-policy-healing-loopback-gate"
	phase_lane_policy_stop                  = "phase-lane-policy-stop"
)

const (
	orchestrator_state_key_app_stop_strategy                         = "app_stop_strategy"
	orchestrator_state_key_mode                                      = "mode"
	orchestrator_state_key_generation_id                             = "generation_id"
	orchestrator_state_key_batch_watcher_events                      = "batch_watcher_events"
	orchestrator_state_key_backend_mutation_branch                   = "backend_mutation_branch"
	orchestrator_state_key_frontend_terminal_action                  = "frontend_terminal_action"
	orchestrator_state_key_should_short_circuit_for_config_restart   = "should_short_circuit_for_config_restart"
	orchestrator_state_key_should_short_circuit_for_build_retry_wait = "should_short_circuit_for_build_retry_wait"
	orchestrator_state_key_should_short_circuit_for_noop_batch       = "should_short_circuit_for_noop_batch"
	orchestrator_state_key_should_loop_back_to_backend_mutation      = "should_loop_back_to_backend_mutation"
)

func phaselane_plan_with_tasks_ctx(
	tasks_ctx *tasks.Ctx,
) phaselane.Plan {
	return phaselane.Plan{
		Phases: []phaselane.Phase{
			{
				ID: "phase_1_batch_facts",
				Cells: []phaselane.PhaseCell{
					{
						LaneID:   "critical_path",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							if _, err := derive_batch_facts_task.Run(cell_tasks_ctx, phase_input); err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{}, nil
						},
					},
				},
				TransitionPolicyID: phase_lane_policy_advance,
			},
			{
				ID: "phase_2_control_flow_gate",
				Cells: []phaselane.PhaseCell{
					{
						LaneID:   "critical_path",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							flow_decision, err := derive_event_execution_flow_decision_task.Run(
								cell_tasks_ctx,
								phase_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{
								StatePatch: map[string]any{
									orchestrator_state_key_app_stop_strategy:                         flow_decision.app_stop_strategy,
									orchestrator_state_key_should_short_circuit_for_config_restart:   flow_decision.should_short_circuit_for_config_restart,
									orchestrator_state_key_should_short_circuit_for_build_retry_wait: flow_decision.should_short_circuit_for_build_retry_wait,
									orchestrator_state_key_should_short_circuit_for_noop_batch:       flow_decision.should_short_circuit_for_noop_batch,
								},
							}, nil
						},
					},
				},
				TransitionPolicyID: phase_lane_policy_control_flow_gate,
			},
			{
				ID: "phase_3_pre_hook_adjustment",
				Cells: []phaselane.PhaseCell{
					{
						LaneID:   "critical_path",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							if _, err := apply_pre_hook_adjustments_task.Run(
								cell_tasks_ctx,
								phase_input,
							); err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{}, nil
						},
					},
				},
				TransitionPolicyID: phase_lane_policy_advance,
			},
			{
				ID: "phase_4_materialization_and_concurrent_hooks",
				Cells: []phaselane.PhaseCell{
					{
						LaneID:   "build_materialization",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							if _, err := execute_materialization_build_lane_task.Run(
								cell_tasks_ctx,
								phase_input,
							); err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{}, nil
						},
					},
					{
						LaneID:   "concurrent_hooks",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							if _, err := execute_concurrent_hooks_task.Run(
								cell_tasks_ctx,
								phase_input,
							); err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{}, nil
						},
					},
				},
				TransitionPolicyID: phase_lane_policy_advance,
			},
			{
				ID: "phase_5_post_hook_adjustment",
				Cells: []phaselane.PhaseCell{
					{
						LaneID:   "critical_path",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							if _, err := apply_post_hook_adjustments_task.Run(
								cell_tasks_ctx,
								phase_input,
							); err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{}, nil
						},
					},
				},
				TransitionPolicyID: phase_lane_policy_advance,
			},
			{
				ID: "phase_6_backend_mutation",
				Cells: []phaselane.PhaseCell{
					{
						LaneID:   "critical_path",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							selected_backend_mutation_branch, err := apply_backend_mutation_plan_task.Run(
								cell_tasks_ctx,
								phase_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{
								StatePatch: map[string]any{
									orchestrator_state_key_backend_mutation_branch: selected_backend_mutation_branch,
								},
							}, nil
						},
					},
				},
				TransitionPolicyID: phase_lane_policy_advance,
			},
			{
				ID: "phase_7_backend_convergence",
				Cells: []phaselane.PhaseCell{
					{
						LaneID:   "critical_path",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							should_request_loop_back, err := converge_backend_state_task.Run(
								cell_tasks_ctx,
								phase_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{
								StatePatch: map[string]any{
									orchestrator_state_key_should_loop_back_to_backend_mutation: should_request_loop_back,
								},
							}, nil
						},
					},
				},
				TransitionPolicyID: phase_lane_policy_advance,
			},
			{
				ID: "phase_8_frontend_settling",
				Cells: []phaselane.PhaseCell{
					{
						LaneID:   "critical_path",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							selected_frontend_terminal_action, err := apply_frontend_terminal_action_task.Run(
								cell_tasks_ctx,
								phase_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{
								StatePatch: map[string]any{
									orchestrator_state_key_frontend_terminal_action: selected_frontend_terminal_action,
								},
							}, nil
						},
					},
				},
				TransitionPolicyID: phase_lane_policy_advance,
			},
			{
				ID: "phase_9_healing_loopback_gate",
				Cells: []phaselane.PhaseCell{
					{
						LaneID:   "critical_path",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							should_loop_back_to_backend_mutation, err := plan_healing_loopback_transition_task.Run(
								cell_tasks_ctx,
								phase_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{
								StatePatch: map[string]any{
									orchestrator_state_key_should_loop_back_to_backend_mutation: should_loop_back_to_backend_mutation,
								},
							}, nil
						},
					},
				},
				TransitionPolicyID: phase_lane_policy_healing_loopback_gate,
			},
			{
				ID: "phase_10_batch_complete",
				Cells: []phaselane.PhaseCell{
					{
						LaneID:   "critical_path",
						WaitMode: phaselane.CellWaitModeAwaitCompletion,
						Run: func(
							run_context context.Context,
							cell_run_input phaselane.CellRunInput,
						) (phaselane.CellRunResult, error) {
							phase_input, err := phase_task_input_from_cell_run_input(
								cell_run_input,
							)
							if err != nil {
								return phaselane.CellRunResult{}, err
							}
							cell_tasks_ctx := tasks_ctx.WithNativeContext(run_context)
							if _, err := complete_batch_task.Run(
								cell_tasks_ctx,
								phase_input,
							); err != nil {
								return phaselane.CellRunResult{}, err
							}
							return phaselane.CellRunResult{}, nil
						},
					},
				},
				TransitionPolicyID: phase_lane_policy_stop,
			},
		},
		TransitionPlanner: phaselane.TransitionPlanner{
			Policies: map[string]phaselane.TransitionPolicy{
				phase_lane_policy_advance: {
					Rules: []phaselane.TransitionRule{
						{
							RuleID: "always-advance",
							Matches: func(
								transition_plan_input phaselane.TransitionPlanInput,
							) bool {
								_ = transition_plan_input
								return true
							},
							Decision: phaselane.TransitionDecision{
								Kind: phaselane.DecisionKindAdvance,
							},
						},
					},
				},
				phase_lane_policy_control_flow_gate: {
					Rules: []phaselane.TransitionRule{
						{
							RuleID: "stop-when-config-restart-short-circuit-is-requested",
							Matches: func(
								transition_plan_input phaselane.TransitionPlanInput,
							) bool {
								if transition_plan_input.State == nil ||
									transition_plan_input.State.Values == nil {
									return false
								}
								raw_value, has_value := transition_plan_input.State.Values[orchestrator_state_key_should_short_circuit_for_config_restart]
								if !has_value {
									return false
								}
								should_short_circuit, ok := raw_value.(bool)
								return ok && should_short_circuit
							},
							Decision: phaselane.TransitionDecision{
								Kind: phaselane.DecisionKindStop,
							},
						},
						{
							RuleID: "stop-when-build-retry-short-circuit-is-requested",
							Matches: func(
								transition_plan_input phaselane.TransitionPlanInput,
							) bool {
								if transition_plan_input.State == nil ||
									transition_plan_input.State.Values == nil {
									return false
								}
								raw_value, has_value := transition_plan_input.State.Values[orchestrator_state_key_should_short_circuit_for_build_retry_wait]
								if !has_value {
									return false
								}
								should_short_circuit, ok := raw_value.(bool)
								return ok && should_short_circuit
							},
							Decision: phaselane.TransitionDecision{
								Kind: phaselane.DecisionKindStop,
							},
						},
						{
							RuleID: "stop-when-noop-batch-short-circuit-is-requested",
							Matches: func(
								transition_plan_input phaselane.TransitionPlanInput,
							) bool {
								if transition_plan_input.State == nil ||
									transition_plan_input.State.Values == nil {
									return false
								}
								raw_value, has_value := transition_plan_input.State.Values[orchestrator_state_key_should_short_circuit_for_noop_batch]
								if !has_value {
									return false
								}
								should_short_circuit, ok := raw_value.(bool)
								return ok && should_short_circuit
							},
							Decision: phaselane.TransitionDecision{
								Kind: phaselane.DecisionKindStop,
							},
						},
						{
							RuleID: "advance-when-no-control-flow-short-circuit-is-requested",
							Matches: func(
								transition_plan_input phaselane.TransitionPlanInput,
							) bool {
								_ = transition_plan_input
								return true
							},
							Decision: phaselane.TransitionDecision{
								Kind: phaselane.DecisionKindAdvance,
							},
						},
					},
				},
				phase_lane_policy_healing_loopback_gate: {
					Rules: []phaselane.TransitionRule{
						{
							RuleID: "loop-back-to-phase-6-on-healing-request",
							Matches: func(
								transition_plan_input phaselane.TransitionPlanInput,
							) bool {
								if transition_plan_input.State == nil ||
									transition_plan_input.State.Values == nil {
									return false
								}
								raw_value, has_value := transition_plan_input.State.Values[orchestrator_state_key_should_loop_back_to_backend_mutation]
								if !has_value {
									return false
								}
								should_loop_back, ok := raw_value.(bool)
								return ok && should_loop_back
							},
							Decision: phaselane.TransitionDecision{
								Kind:          phaselane.DecisionKindLoopBack,
								TargetPhaseID: "phase_6_backend_mutation",
							},
						},
						{
							RuleID: "advance-when-no-healing-loopback-is-requested",
							Matches: func(
								transition_plan_input phaselane.TransitionPlanInput,
							) bool {
								_ = transition_plan_input
								return true
							},
							Decision: phaselane.TransitionDecision{
								Kind: phaselane.DecisionKindAdvance,
							},
						},
					},
				},
				phase_lane_policy_stop: {
					Rules: []phaselane.TransitionRule{
						{
							RuleID: "always-stop",
							Matches: func(
								transition_plan_input phaselane.TransitionPlanInput,
							) bool {
								_ = transition_plan_input
								return true
							},
							Decision: phaselane.TransitionDecision{
								Kind: phaselane.DecisionKindStop,
							},
						},
					},
				},
			},
		},
	}
}

func phase_task_input_from_cell_run_input(
	cell_run_input phaselane.CellRunInput,
) (phase_task_input, error) {
	if cell_run_input.StepIndex < 0 {
		return phase_task_input{}, fmt.Errorf(
			"wavebuild: orchestrator phase step index %d is invalid",
			cell_run_input.StepIndex,
		)
	}
	if cell_run_input.StateSnapshot == nil {
		return phase_task_input{}, errors.New(
			"wavebuild: orchestrator phase state is required",
		)
	}
	state_values := cell_run_input.StateSnapshot
	batch_watcher_events, has_batch_watcher_events := state_values[orchestrator_state_key_batch_watcher_events]
	parsed_batch_watcher_events := []batch_watcher_event(nil)
	if has_batch_watcher_events {
		typed_batch_watcher_events, ok := batch_watcher_events.([]batch_watcher_event)
		if !ok {
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator batch watcher events has unexpected type %T",
				batch_watcher_events,
			)
		}
		parsed_batch_watcher_events = typed_batch_watcher_events
	}
	derived_batch_event_categorization := derive_batch_event_categorization(
		parsed_batch_watcher_events,
	)
	derived_app_stop_strategy,
		derived_frontend_terminal_action,
		derived_should_short_circuit_for_config_restart,
		derived_should_short_circuit_for_build_retry_wait,
		derived_should_short_circuit_for_noop_batch,
		derived_backend_mutation_branch := derive_defaults_from_batch_event_categorization(
		derived_batch_event_categorization,
	)

	raw_mode, has_mode := state_values[orchestrator_state_key_mode]
	if !has_mode {
		return phase_task_input{}, errors.New(
			"wavebuild: orchestrator phase state mode is required",
		)
	}
	normalized_mode := ""
	switch typed_mode := raw_mode.(type) {
	case mode:
		normalized_mode = strings.TrimSpace(string(typed_mode))
	case string:
		normalized_mode = strings.TrimSpace(typed_mode)
	default:
		return phase_task_input{}, fmt.Errorf(
			"wavebuild: orchestrator phase state mode has unexpected type %T",
			raw_mode,
		)
	}
	parsed_mode := mode(normalized_mode)
	switch parsed_mode {
	case mode_dev, mode_prod:
	default:
		return phase_task_input{}, fmt.Errorf(
			"wavebuild: orchestrator phase state mode %q is unsupported",
			normalized_mode,
		)
	}
	current_mode := parsed_mode

	selected_app_stop_strategy := derived_app_stop_strategy
	if raw_app_stop_strategy, has_app_stop_strategy := state_values[orchestrator_state_key_app_stop_strategy]; has_app_stop_strategy {
		normalized_app_stop_strategy := ""
		switch typed_app_stop_strategy := raw_app_stop_strategy.(type) {
		case app_stop_strategy:
			normalized_app_stop_strategy = strings.TrimSpace(
				string(typed_app_stop_strategy),
			)
		case string:
			normalized_app_stop_strategy = strings.TrimSpace(
				typed_app_stop_strategy,
			)
		default:
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator app stop strategy has unexpected type %T",
				raw_app_stop_strategy,
			)
		}
		if normalized_app_stop_strategy == "" {
			return phase_task_input{}, errors.New(
				"wavebuild: orchestrator app stop strategy is empty",
			)
		}
		parsed_app_stop_strategy := app_stop_strategy(
			normalized_app_stop_strategy,
		)
		switch parsed_app_stop_strategy {
		case app_stop_strategy_none:
			selected_app_stop_strategy = parsed_app_stop_strategy
		case app_stop_strategy_single_event_hard_reload:
			selected_app_stop_strategy = parsed_app_stop_strategy
		case app_stop_strategy_batch_hard_reload:
			selected_app_stop_strategy = parsed_app_stop_strategy
		default:
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator app stop strategy %q is unsupported",
				normalized_app_stop_strategy,
			)
		}
	}

	raw_generation_id, has_generation_id := state_values[orchestrator_state_key_generation_id]
	if !has_generation_id {
		return phase_task_input{}, errors.New(
			"wavebuild: orchestrator phase state generation id is required",
		)
	}
	generation_id, generation_id_ok := raw_generation_id.(string)
	if !generation_id_ok {
		return phase_task_input{}, fmt.Errorf(
			"wavebuild: orchestrator phase state generation id has unexpected type %T",
			raw_generation_id,
		)
	}
	generation_id = strings.TrimSpace(generation_id)
	if generation_id == "" {
		return phase_task_input{}, err_generation_id_required
	}

	selected_backend_mutation_branch := derived_backend_mutation_branch
	if raw_backend_mutation_branch, has_backend_mutation_branch := state_values[orchestrator_state_key_backend_mutation_branch]; has_backend_mutation_branch {
		normalized_backend_mutation_branch := ""
		switch typed_backend_mutation_branch := raw_backend_mutation_branch.(type) {
		case backend_mutation_branch:
			normalized_backend_mutation_branch = strings.TrimSpace(
				string(typed_backend_mutation_branch),
			)
		case string:
			normalized_backend_mutation_branch = strings.TrimSpace(
				typed_backend_mutation_branch,
			)
		default:
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator backend mutation branch has unexpected type %T",
				raw_backend_mutation_branch,
			)
		}
		if normalized_backend_mutation_branch == "" {
			return phase_task_input{}, errors.New(
				"wavebuild: orchestrator backend mutation branch is empty",
			)
		}
		parsed_backend_mutation_branch := backend_mutation_branch(
			normalized_backend_mutation_branch,
		)
		switch parsed_backend_mutation_branch {
		case backend_mutation_branch_queue_retry_wait_restart:
			selected_backend_mutation_branch = parsed_backend_mutation_branch
		case backend_mutation_branch_restart_dev_server_cycle:
			selected_backend_mutation_branch = parsed_backend_mutation_branch
		case backend_mutation_branch_apply_normal_backend_mutations:
			selected_backend_mutation_branch = parsed_backend_mutation_branch
		default:
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator backend mutation branch %q is unsupported",
				normalized_backend_mutation_branch,
			)
		}
	}

	selected_frontend_terminal_action := derived_frontend_terminal_action
	if raw_frontend_terminal_action, has_frontend_terminal_action := state_values[orchestrator_state_key_frontend_terminal_action]; has_frontend_terminal_action {
		normalized_frontend_terminal_action := ""
		switch typed_frontend_terminal_action := raw_frontend_terminal_action.(type) {
		case frontend_terminal_action:
			normalized_frontend_terminal_action = strings.TrimSpace(
				string(typed_frontend_terminal_action),
			)
		case string:
			normalized_frontend_terminal_action = strings.TrimSpace(
				typed_frontend_terminal_action,
			)
		default:
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator frontend terminal action has unexpected type %T",
				raw_frontend_terminal_action,
			)
		}
		if normalized_frontend_terminal_action == "" {
			return phase_task_input{}, errors.New(
				"wavebuild: orchestrator frontend terminal action is empty",
			)
		}
		parsed_frontend_terminal_action := frontend_terminal_action(
			normalized_frontend_terminal_action,
		)
		switch parsed_frontend_terminal_action {
		case frontend_terminal_action_none:
			selected_frontend_terminal_action = parsed_frontend_terminal_action
		case frontend_terminal_action_css_hot_reload:
			selected_frontend_terminal_action = parsed_frontend_terminal_action
		case frontend_terminal_action_revalidate:
			selected_frontend_terminal_action = parsed_frontend_terminal_action
		case frontend_terminal_action_notify_vite_public_filemap_changed:
			selected_frontend_terminal_action = parsed_frontend_terminal_action
		case frontend_terminal_action_hard_reload:
			selected_frontend_terminal_action = parsed_frontend_terminal_action
		default:
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator frontend terminal action %q is unsupported",
				normalized_frontend_terminal_action,
			)
		}
	}

	should_request_loop_back := false
	if raw_should_request_loop_back, has_should_request_loop_back := state_values[orchestrator_state_key_should_loop_back_to_backend_mutation]; has_should_request_loop_back {
		typed_should_request_loop_back, ok := raw_should_request_loop_back.(bool)
		if !ok {
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator healing loop-back flag has unexpected type %T",
				raw_should_request_loop_back,
			)
		}
		should_request_loop_back = typed_should_request_loop_back
	}

	should_short_circuit_for_config_restart := derived_should_short_circuit_for_config_restart
	if raw_should_short_circuit_for_config_restart, has_should_short_circuit_for_config_restart := state_values[orchestrator_state_key_should_short_circuit_for_config_restart]; has_should_short_circuit_for_config_restart {
		typed_should_short_circuit_for_config_restart, ok := raw_should_short_circuit_for_config_restart.(bool)
		if !ok {
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator config restart short-circuit flag has unexpected type %T",
				raw_should_short_circuit_for_config_restart,
			)
		}
		should_short_circuit_for_config_restart = typed_should_short_circuit_for_config_restart
	}

	should_short_circuit_for_build_retry_wait := derived_should_short_circuit_for_build_retry_wait
	if raw_should_short_circuit_for_build_retry_wait, has_should_short_circuit_for_build_retry_wait := state_values[orchestrator_state_key_should_short_circuit_for_build_retry_wait]; has_should_short_circuit_for_build_retry_wait {
		typed_should_short_circuit_for_build_retry_wait, ok := raw_should_short_circuit_for_build_retry_wait.(bool)
		if !ok {
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator build retry short-circuit flag has unexpected type %T",
				raw_should_short_circuit_for_build_retry_wait,
			)
		}
		should_short_circuit_for_build_retry_wait = typed_should_short_circuit_for_build_retry_wait
	}

	should_short_circuit_for_noop_batch := derived_should_short_circuit_for_noop_batch
	if raw_should_short_circuit_for_noop_batch, has_should_short_circuit_for_noop_batch := state_values[orchestrator_state_key_should_short_circuit_for_noop_batch]; has_should_short_circuit_for_noop_batch {
		typed_should_short_circuit_for_noop_batch, ok := raw_should_short_circuit_for_noop_batch.(bool)
		if !ok {
			return phase_task_input{}, fmt.Errorf(
				"wavebuild: orchestrator noop batch short-circuit flag has unexpected type %T",
				raw_should_short_circuit_for_noop_batch,
			)
		}
		should_short_circuit_for_noop_batch = typed_should_short_circuit_for_noop_batch
	}

	return phase_task_input{
		mode:                                    current_mode,
		gen_id:                                  generation_id,
		step_index:                              cell_run_input.StepIndex,
		app_stop_strategy:                       selected_app_stop_strategy,
		backend_mutation_branch:                 selected_backend_mutation_branch,
		frontend_terminal_action:                selected_frontend_terminal_action,
		should_request_loop_back:                should_request_loop_back,
		hook_stage:                              hook_stage_none,
		batch_event_categorization:              derived_batch_event_categorization,
		should_short_circuit_for_config_restart: should_short_circuit_for_config_restart,
		should_short_circuit_for_build_retry_wait: should_short_circuit_for_build_retry_wait,
		should_short_circuit_for_noop_batch:       should_short_circuit_for_noop_batch,
	}, nil
}

/////////////////////////////////////////////////////////////////////
/////// Phase Pipeline Entry
/////////////////////////////////////////////////////////////////////

func run_phase_task_pipeline(
	parent_context context.Context,
	input p1_batch_input,
) error {
	normalized_generation_id, err := input.normalize_generation_id()
	if err != nil {
		return err
	}
	batch_tasks_context := tasks.NewCtx(parent_context)
	_, err = phaselane.Run(
		phaselane.ExecutionInput{
			Context: parent_context,
			Plan:    phaselane_plan_with_tasks_ctx(batch_tasks_context),
			InitialState: phaselane.State{
				Values: map[string]any{
					orchestrator_state_key_mode:                 input.p1.mode,
					orchestrator_state_key_generation_id:        normalized_generation_id,
					orchestrator_state_key_batch_watcher_events: input.p1.batch_watcher_events,
				},
			},
			MaxSteps: 64,
		},
	)
	return err
}
