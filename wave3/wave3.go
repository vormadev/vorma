package wave3

import (
	"context"
	"errors"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/tasks"
)

type supercycle_input struct {
	batch_id                    uint64
	batch_data                  *supercycle_batch_data
	previous_build_id           string
	previous_config_fingerprint string
	current_config_fingerprint  string
}

type supercycle_batch_data struct {
	evts []fsnotify.Event
}

type checkpoint string

const (
	checkpoint_cycle_start                checkpoint = "cycle_start"
	checkpoint_wave_work_start            checkpoint = "wave_work_start"
	checkpoint_wave_work_go_compile_start checkpoint = "wave_work_go_compile_start"
	checkpoint_wave_work_end              checkpoint = "wave_work_end"
	checkpoint_cycle_end                  checkpoint = "cycle_end"
)

/*
Design decisions for this file:

1) This is a skeleton orchestrator, not production side-effect code.
Every task currently models dependency shape and data flow only.
Task internals are intentionally minimal placeholders until behavior is locked.

2) The execution model is one DAG.
Each task is one DAG node implemented with `kit/tasks`.
Dependencies are expressed by calling other tasks directly.
We rely on `tasks.Ctx` for per-cycle dedupe.

3) "Phase" labels are documentation only.
The `phase: N` comments are a human-readable first-to-last timeline.
Actual execution order is always determined by task dependencies.

4) The file is organized in phase order (1 -> 23) for readability.
That is intentionally opposite from terminal-first mental construction.
Reading top-to-bottom should match newcomer intuition.

5) Two terminal roots define the whole cycle.
`run_supercycle` runs:
- `wave__finalize_cycle_cleanup`
- `wave__refresh_watcher_coverage`
All other tasks are reached transitively through these roots.

6) One explicit fact type per task output.
Each task returns a dedicated `*_facts` type placed near the task.
This keeps required inputs/outputs explicit and discourages hidden shared state.

7) No loopback lanes, detached lanes, or no-wait lane concepts are modeled here.
If a future behavior is required, it should be represented as explicit DAG work,
not implicit control-flow side channels.

8) Fail fast on invalid cycle input.
Example: nil batch data is an immediate error.
This file intentionally avoids defensive fallbacks that hide invalid state.
*/

/////////////////////////////////////////////////////////////////////
/////// supercycle task dag (phase order: first -> last)
/////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////
/////// facts: wave__user_config_parsed_facts
/////// phase: 1
/////////////////////////////////////////////////////////////////////

type wave__user_config_parsed_facts struct {
	current_config_fingerprint string
}

/////////////////////////////////////////////////////////////////////
/////// task: wave__capture_previous_run_state
/////// phase: 1
/////////////////////////////////////////////////////////////////////

type wave__run_baseline_captured_facts struct {
	previous_build_id           string
	previous_config_fingerprint string
}

var wave__capture_previous_run_state = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__run_baseline_captured_facts, error) {
	// Role in system: expose prior-run identifiers for lifecycle decisions.
	// This task currently passes through already-prepared input facts.
	_ = tasks_ctx
	return wave__run_baseline_captured_facts{
		previous_build_id:           input.previous_build_id,
		previous_config_fingerprint: input.previous_config_fingerprint,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__normalize_change_batch
/////// phase: 1
/////////////////////////////////////////////////////////////////////

type wave__raw_event_batch_normalized_facts struct {
	evts []fsnotify.Event
}

var wave__normalize_change_batch = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__raw_event_batch_normalized_facts, error) {
	// Role in system: normalize raw file-change input into one canonical batch fact.
	// This task currently validates presence and passes the raw events through.
	_ = tasks_ctx
	if input.batch_data == nil {
		return wave__raw_event_batch_normalized_facts{}, errors.New(
			"wave3: supercycle batch_data is nil",
		)
	}
	return wave__raw_event_batch_normalized_facts{
		evts: input.batch_data.evts,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__refresh_watcher_coverage
/////// phase: 2
/////////////////////////////////////////////////////////////////////

type wave__watcher_coverage_done_facts struct {
	normalized_batch wave__raw_event_batch_normalized_facts
}

var wave__refresh_watcher_coverage = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__watcher_coverage_done_facts, error) {
	// Role in system: keep watcher coverage aligned with the normalized change batch.
	// This task currently depends on normalized events and returns watcher facts.
	normalized_batch, err := wave__normalize_change_batch.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__watcher_coverage_done_facts{}, err
	}
	return wave__watcher_coverage_done_facts{
		normalized_batch: normalized_batch,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__checkpoint_cycle_start
/////// phase: 3
/////////////////////////////////////////////////////////////////////

type wave__checkpoint_cycle_start_done_facts struct {
	user_config    wave__user_config_parsed_facts
	vorma_reactors vorma__cycle_start_reactors_done_facts
}

var wave__checkpoint_cycle_start = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__checkpoint_cycle_start_done_facts, error) {
	// Role in system: cycle_start checkpoint gate.
	// This task flights and awaits non-wave-owned reactors bound to cycle_start.
	vorma_reactors, err := vorma__run_reactors_finished_by_cycle_start.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__checkpoint_cycle_start_done_facts{}, err
	}
	return wave__checkpoint_cycle_start_done_facts{
		user_config: wave__user_config_parsed_facts{
			current_config_fingerprint: input.current_config_fingerprint,
		},
		vorma_reactors: vorma_reactors,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__publish_effective_configuration
/////// phase: 3
/////////////////////////////////////////////////////////////////////

type wave__effective_config_committed_facts struct {
	checkpoint_cycle_start wave__checkpoint_cycle_start_done_facts
}

var wave__publish_effective_configuration = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__effective_config_committed_facts, error) {
	// Role in system: publish one effective config view for all downstream phases.
	// This task currently depends on cycle_start checkpoint completion.
	checkpoint_cycle_start, err := wave__checkpoint_cycle_start.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__effective_config_committed_facts{}, err
	}
	return wave__effective_config_committed_facts{
		checkpoint_cycle_start: checkpoint_cycle_start,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__apply_config_change_gate
/////// phase: 4
/////////////////////////////////////////////////////////////////////

type wave__config_mutation_pre_effects_applied_facts struct {
	effective_config wave__effective_config_committed_facts
	normalized_batch wave__raw_event_batch_normalized_facts
}

var wave__apply_config_change_gate = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input supercycle_input,
	) (wave__config_mutation_pre_effects_applied_facts, error) {
		// Role in system: join config facts and normalized changes before classification.
		// This task currently establishes both prerequisites in parallel.
		var effective_config wave__effective_config_committed_facts
		var normalized_batch wave__raw_event_batch_normalized_facts
		if err := tasks_ctx.RunParallel(
			wave__publish_effective_configuration.Bind(input, &effective_config),
			wave__normalize_change_batch.Bind(input, &normalized_batch),
		); err != nil {
			return wave__config_mutation_pre_effects_applied_facts{}, err
		}
		return wave__config_mutation_pre_effects_applied_facts{
			effective_config: effective_config,
			normalized_batch: normalized_batch,
		}, nil
	},
)

/////////////////////////////////////////////////////////////////////
/////// task: wave__classify_change_semantics
/////// phase: 5
/////////////////////////////////////////////////////////////////////

type wave__event_categories_derived_facts struct {
	config_mutation_pre_effects wave__config_mutation_pre_effects_applied_facts
}

var wave__classify_change_semantics = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__event_categories_derived_facts, error) {
	// Role in system: convert raw changes into semantic change categories.
	// This task currently depends on config-change gating facts.
	config_mutation_pre_effects, err := wave__apply_config_change_gate.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__event_categories_derived_facts{}, err
	}
	return wave__event_categories_derived_facts{
		config_mutation_pre_effects: config_mutation_pre_effects,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__summarize_required_work
/////// phase: 6
/////////////////////////////////////////////////////////////////////

type wave__required_work_reduced_facts struct {
	event_categories wave__event_categories_derived_facts
}

var wave__summarize_required_work = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__required_work_reduced_facts, error) {
	// Role in system: reduce semantic categories into one required-work summary.
	// This task currently forwards classification output as reduced work facts.
	event_categories, err := wave__classify_change_semantics.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__required_work_reduced_facts{}, err
	}
	return wave__required_work_reduced_facts{
		event_categories: event_categories,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__resolve_hook_eligibility
/////// phase: 6
/////////////////////////////////////////////////////////////////////

type wave__hook_applicability_derived_facts struct {
	event_categories wave__event_categories_derived_facts
}

var wave__resolve_hook_eligibility = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__hook_applicability_derived_facts, error) {
	// Role in system: determine which hook lanes are applicable for this cycle.
	// This task currently derives eligibility from semantic classification facts.
	event_categories, err := wave__classify_change_semantics.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__hook_applicability_derived_facts{}, err
	}
	return wave__hook_applicability_derived_facts{
		event_categories: event_categories,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__select_pipeline_path
/////// phase: 7
/////////////////////////////////////////////////////////////////////

type wave__control_flow_decisions_derived_facts struct {
	required_work wave__required_work_reduced_facts
}

var wave__select_pipeline_path = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__control_flow_decisions_derived_facts, error) {
	// Role in system: pick the control-flow path for this cycle.
	// This task currently maps reduced work into control-flow decision facts.
	required_work, err := wave__summarize_required_work.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__control_flow_decisions_derived_facts{}, err
	}
	return wave__control_flow_decisions_derived_facts{
		required_work: required_work,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__plan_app_lifecycle
/////// phase: 8
/////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////
/////// task: wave__checkpoint_wave_work_start
/////// phase: 8
/////////////////////////////////////////////////////////////////////

type wave__checkpoint_wave_work_start_done_facts struct {
	control_flow wave__control_flow_decisions_derived_facts
}

var wave__checkpoint_wave_work_start = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__checkpoint_wave_work_start_done_facts, error) {
	// Role in system: wave_work_start checkpoint gate.
	// This task flights and awaits non-wave-owned reactors bound to wave_work_start.
	control_flow, err := wave__select_pipeline_path.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__checkpoint_wave_work_start_done_facts{}, err
	}
	return wave__checkpoint_wave_work_start_done_facts{
		control_flow: control_flow,
	}, nil
})

type wave__app_stop_strategy_derived_facts struct {
	control_flow wave__control_flow_decisions_derived_facts
}

var wave__plan_app_lifecycle = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__app_stop_strategy_derived_facts, error) {
	// Role in system: derive app lifecycle strategy (stop/restart mode).
	// This task currently maps wave_work_start checkpoint output into lifecycle strategy facts.
	checkpoint_wave_work_start, err := wave__checkpoint_wave_work_start.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__app_stop_strategy_derived_facts{}, err
	}
	return wave__app_stop_strategy_derived_facts{
		control_flow: checkpoint_wave_work_start.control_flow,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__apply_app_lifecycle
/////// phase: 9
/////////////////////////////////////////////////////////////////////

type wave__app_stop_effects_applied_facts struct {
	app_stop_strategy wave__app_stop_strategy_derived_facts
	run_baseline      wave__run_baseline_captured_facts
}

var wave__apply_app_lifecycle = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__app_stop_effects_applied_facts, error) {
	// Role in system: apply lifecycle strategy with previous-run context.
	// This task currently joins strategy and baseline facts in parallel.
	var app_stop_strategy wave__app_stop_strategy_derived_facts
	var run_baseline wave__run_baseline_captured_facts
	if err := tasks_ctx.RunParallel(
		wave__plan_app_lifecycle.Bind(input, &app_stop_strategy),
		wave__capture_previous_run_state.Bind(input, &run_baseline),
	); err != nil {
		return wave__app_stop_effects_applied_facts{}, err
	}
	return wave__app_stop_effects_applied_facts{
		app_stop_strategy: app_stop_strategy,
		run_baseline:      run_baseline,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__run_pre_hooks
/////// phase: 10
/////////////////////////////////////////////////////////////////////

type wave__pre_hooks_executed_facts struct {
	hook_applicability wave__hook_applicability_derived_facts
	app_stop_effects   wave__app_stop_effects_applied_facts
}

var wave__run_pre_hooks = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__pre_hooks_executed_facts, error) {
	// Role in system: execute pre-hook lane once eligibility and lifecycle are known.
	// This task currently joins those prerequisites and returns pre-hook facts.
	var hook_applicability wave__hook_applicability_derived_facts
	var app_stop_effects wave__app_stop_effects_applied_facts
	if err := tasks_ctx.RunParallel(
		wave__resolve_hook_eligibility.Bind(input, &hook_applicability),
		wave__apply_app_lifecycle.Bind(input, &app_stop_effects),
	); err != nil {
		return wave__pre_hooks_executed_facts{}, err
	}
	return wave__pre_hooks_executed_facts{
		hook_applicability: hook_applicability,
		app_stop_effects:   app_stop_effects,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__merge_pre_hook_outcomes
/////// phase: 11
/////////////////////////////////////////////////////////////////////

type wave__pre_hook_outcomes_merged_facts struct {
	pre_hooks_executed wave__pre_hooks_executed_facts
}

var wave__merge_pre_hook_outcomes = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__pre_hook_outcomes_merged_facts, error) {
	// Role in system: fold pre-hook outcomes into the cycle's shared state.
	// This task currently depends on pre-hook execution facts.
	pre_hooks_executed, err := wave__run_pre_hooks.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__pre_hook_outcomes_merged_facts{}, err
	}
	return wave__pre_hook_outcomes_merged_facts{
		pre_hooks_executed: pre_hooks_executed,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__run_concurrent_hooks
/////// phase: 11
/////////////////////////////////////////////////////////////////////

type wave__concurrent_hooks_executed_facts struct {
	pre_hooks_executed wave__pre_hooks_executed_facts
}

var wave__run_concurrent_hooks = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__concurrent_hooks_executed_facts, error) {
	// Role in system: execute concurrent-hook lane after pre-hook setup exists.
	// This task currently depends on pre-hook execution facts.
	pre_hooks_executed, err := wave__run_pre_hooks.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__concurrent_hooks_executed_facts{}, err
	}
	return wave__concurrent_hooks_executed_facts{
		pre_hooks_executed: pre_hooks_executed,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__prepare_go_compile_inputs
/////// phase: 12
/////////////////////////////////////////////////////////////////////

type wave__go_compile_prereqs_done_facts struct {
	pre_hook_outcomes wave__pre_hook_outcomes_merged_facts
}

var wave__prepare_go_compile_inputs = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__go_compile_prereqs_done_facts, error) {
	// Role in system: prepare all compile prerequisites for the Go build lane.
	// This task currently depends on merged pre-hook outcomes.
	pre_hook_outcomes, err := wave__merge_pre_hook_outcomes.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__go_compile_prereqs_done_facts{}, err
	}
	return wave__go_compile_prereqs_done_facts{
		pre_hook_outcomes: pre_hook_outcomes,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__prepare_runtime_artifacts
/////// phase: 12
/////////////////////////////////////////////////////////////////////

type wave__wave_metadata_and_runtime_artifacts_ready_facts struct {
	pre_hook_outcomes wave__pre_hook_outcomes_merged_facts
}

var wave__prepare_runtime_artifacts = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input supercycle_input,
	) (wave__wave_metadata_and_runtime_artifacts_ready_facts, error) {
		// Role in system: prepare runtime/dist artifacts that are not Go compile outputs.
		// This task currently depends on merged pre-hook outcomes.
		pre_hook_outcomes, err := wave__merge_pre_hook_outcomes.Run(
			tasks_ctx,
			input,
		)
		if err != nil {
			return wave__wave_metadata_and_runtime_artifacts_ready_facts{}, err
		}
		return wave__wave_metadata_and_runtime_artifacts_ready_facts{
			pre_hook_outcomes: pre_hook_outcomes,
		}, nil
	},
)

/////////////////////////////////////////////////////////////////////
/////// task: wave__run_non_compile_work
/////// phase: 13
/////////////////////////////////////////////////////////////////////

type wave__wave_non_compile_work_done_facts struct {
	wave_metadata_and_runtime_artifacts_ready wave__wave_metadata_and_runtime_artifacts_ready_facts
}

var wave__run_non_compile_work = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__wave_non_compile_work_done_facts, error) {
	// Role in system: run non-compile Wave work (assets/metadata/runtime prep).
	// This task currently depends on runtime-artifact preparation facts.
	wave_metadata_and_runtime_artifacts_ready, err := wave__prepare_runtime_artifacts.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__wave_non_compile_work_done_facts{}, err
	}
	return wave__wave_non_compile_work_done_facts{
		wave_metadata_and_runtime_artifacts_ready: wave_metadata_and_runtime_artifacts_ready,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__run_go_compile
/////// phase: 14
/////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////
/////// task: wave__checkpoint_wave_work_go_compile_start
/////// phase: 14
/////////////////////////////////////////////////////////////////////

type wave__checkpoint_wave_work_go_compile_start_done_facts struct {
	go_compile_prereqs_done wave__go_compile_prereqs_done_facts
	vorma_reactors          vorma__wave_work_go_compile_start_reactors_done_facts
}

var wave__checkpoint_wave_work_go_compile_start = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__checkpoint_wave_work_go_compile_start_done_facts, error) {
	// Role in system: wave_work_go_compile_start checkpoint gate.
	// This task flights and awaits non-wave-owned reactors bound to wave_work_go_compile_start.
	var go_compile_prereqs_done wave__go_compile_prereqs_done_facts
	var vorma_reactors vorma__wave_work_go_compile_start_reactors_done_facts
	if err := tasks_ctx.RunParallel(
		wave__prepare_go_compile_inputs.Bind(input, &go_compile_prereqs_done),
		vorma__run_reactors_finished_by_wave_work_go_compile_start.Bind(input, &vorma_reactors),
	); err != nil {
		return wave__checkpoint_wave_work_go_compile_start_done_facts{}, err
	}
	return wave__checkpoint_wave_work_go_compile_start_done_facts{
		go_compile_prereqs_done: go_compile_prereqs_done,
		vorma_reactors:          vorma_reactors,
	}, nil
})

type wave__go_compile_done_facts struct {
	go_compile_prereqs_done    wave__go_compile_prereqs_done_facts
	wave_non_compile_work_done wave__wave_non_compile_work_done_facts
}

var wave__run_go_compile = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__go_compile_done_facts, error) {
	// Role in system: execute Go compile lane after all prerequisites are ready.
	// This task currently joins go_compile_start checkpoint output and non-compile lanes in parallel.
	var checkpoint_wave_work_go_compile_start wave__checkpoint_wave_work_go_compile_start_done_facts
	var wave_non_compile_work_done wave__wave_non_compile_work_done_facts
	if err := tasks_ctx.RunParallel(
		wave__checkpoint_wave_work_go_compile_start.Bind(input, &checkpoint_wave_work_go_compile_start),
		wave__run_non_compile_work.Bind(input, &wave_non_compile_work_done),
	); err != nil {
		return wave__go_compile_done_facts{}, err
	}
	return wave__go_compile_done_facts{
		go_compile_prereqs_done:    checkpoint_wave_work_go_compile_start.go_compile_prereqs_done,
		wave_non_compile_work_done: wave_non_compile_work_done,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__finalize_wave_outputs
/////// phase: 15
/////////////////////////////////////////////////////////////////////

type wave__wave_public_map_finalized_facts struct {
	go_compile wave__go_compile_done_facts
}

var wave__finalize_wave_outputs = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__wave_public_map_finalized_facts, error) {
	// Role in system: finalize Wave-owned output state for downstream decisions.
	// This task currently depends on completion of the Go compile lane.
	go_compile, err := wave__run_go_compile.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__wave_public_map_finalized_facts{}, err
	}
	return wave__wave_public_map_finalized_facts{
		go_compile: go_compile,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__run_post_hooks
/////// phase: 16
/////////////////////////////////////////////////////////////////////

/////////////////////////////////////////////////////////////////////
/////// task: wave__checkpoint_wave_work_end
/////// phase: 16
/////////////////////////////////////////////////////////////////////

type wave__checkpoint_wave_work_end_done_facts struct {
	wave_public_map_finalized wave__wave_public_map_finalized_facts
}

var wave__checkpoint_wave_work_end = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__checkpoint_wave_work_end_done_facts, error) {
	// Role in system: wave_work_end checkpoint gate.
	// This task flights and awaits non-wave-owned reactors bound to wave_work_end.
	wave_public_map_finalized, err := wave__finalize_wave_outputs.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__checkpoint_wave_work_end_done_facts{}, err
	}
	return wave__checkpoint_wave_work_end_done_facts{
		wave_public_map_finalized: wave_public_map_finalized,
	}, nil
})

type wave__post_hooks_executed_facts struct {
	wave_public_map_finalized wave__wave_public_map_finalized_facts
	concurrent_hooks_executed wave__concurrent_hooks_executed_facts
}

var wave__run_post_hooks = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__post_hooks_executed_facts, error) {
	// Role in system: run post-hook lane after core Wave outputs exist.
	// This task currently joins wave_work_end checkpoint output with concurrent-hook facts.
	var checkpoint_wave_work_end wave__checkpoint_wave_work_end_done_facts
	var concurrent_hooks_executed wave__concurrent_hooks_executed_facts
	if err := tasks_ctx.RunParallel(
		wave__checkpoint_wave_work_end.Bind(input, &checkpoint_wave_work_end),
		wave__run_concurrent_hooks.Bind(input, &concurrent_hooks_executed),
	); err != nil {
		return wave__post_hooks_executed_facts{}, err
	}
	return wave__post_hooks_executed_facts{
		wave_public_map_finalized: checkpoint_wave_work_end.wave_public_map_finalized,
		concurrent_hooks_executed: concurrent_hooks_executed,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__merge_concurrent_hook_outcomes
/////// phase: 16
/////////////////////////////////////////////////////////////////////

type wave__concurrent_hook_outcomes_merged_facts struct {
	wave_public_map_finalized wave__wave_public_map_finalized_facts
	concurrent_hooks_executed wave__concurrent_hooks_executed_facts
}

var wave__merge_concurrent_hook_outcomes = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__concurrent_hook_outcomes_merged_facts, error) {
	// Role in system: merge concurrent-hook outcomes with finalized Wave outputs.
	// This task currently joins wave_work_end checkpoint output with concurrent-hook facts.
	var checkpoint_wave_work_end wave__checkpoint_wave_work_end_done_facts
	var concurrent_hooks_executed wave__concurrent_hooks_executed_facts
	if err := tasks_ctx.RunParallel(
		wave__checkpoint_wave_work_end.Bind(input, &checkpoint_wave_work_end),
		wave__run_concurrent_hooks.Bind(input, &concurrent_hooks_executed),
	); err != nil {
		return wave__concurrent_hook_outcomes_merged_facts{}, err
	}
	return wave__concurrent_hook_outcomes_merged_facts{
		wave_public_map_finalized: checkpoint_wave_work_end.wave_public_map_finalized,
		concurrent_hooks_executed: concurrent_hooks_executed,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__merge_post_hook_outcomes
/////// phase: 17
/////////////////////////////////////////////////////////////////////

type wave__post_hook_outcomes_merged_facts struct {
	concurrent_hook_outcomes wave__concurrent_hook_outcomes_merged_facts
	post_hooks_executed      wave__post_hooks_executed_facts
}

var wave__merge_post_hook_outcomes = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__post_hook_outcomes_merged_facts, error) {
	// Role in system: build the final hook-adjusted intent surface for this cycle.
	// This task currently joins merged concurrent results and post-hook results.
	var concurrent_hook_outcomes wave__concurrent_hook_outcomes_merged_facts
	var post_hooks_executed wave__post_hooks_executed_facts
	if err := tasks_ctx.RunParallel(
		wave__merge_concurrent_hook_outcomes.Bind(input, &concurrent_hook_outcomes),
		wave__run_post_hooks.Bind(input, &post_hooks_executed),
	); err != nil {
		return wave__post_hook_outcomes_merged_facts{}, err
	}
	return wave__post_hook_outcomes_merged_facts{
		concurrent_hook_outcomes: concurrent_hook_outcomes,
		post_hooks_executed:      post_hooks_executed,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__apply_backend_changes
/////// phase: 18
/////////////////////////////////////////////////////////////////////

type wave__backend_mutation_applied_facts struct {
	post_hook_outcomes wave__post_hook_outcomes_merged_facts
}

var wave__apply_backend_changes = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__backend_mutation_applied_facts, error) {
	// Role in system: apply the selected backend mutation branch for this cycle.
	// This task currently depends on final merged post-hook outcomes.
	post_hook_outcomes, err := wave__merge_post_hook_outcomes.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__backend_mutation_applied_facts{}, err
	}
	return wave__backend_mutation_applied_facts{
		post_hook_outcomes: post_hook_outcomes,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__derive_backend_convergence_requirements
/////// phase: 18
/////////////////////////////////////////////////////////////////////

type wave__backend_convergence_requirements_derived_facts struct {
	post_hook_outcomes wave__post_hook_outcomes_merged_facts
}

var wave__derive_backend_convergence_requirements = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input supercycle_input,
	) (wave__backend_convergence_requirements_derived_facts, error) {
		// Role in system: derive what backend readiness conditions must be satisfied.
		// This task currently depends on final merged post-hook outcomes.
		post_hook_outcomes, err := wave__merge_post_hook_outcomes.Run(
			tasks_ctx,
			input,
		)
		if err != nil {
			return wave__backend_convergence_requirements_derived_facts{}, err
		}
		return wave__backend_convergence_requirements_derived_facts{
			post_hook_outcomes: post_hook_outcomes,
		}, nil
	},
)

/////////////////////////////////////////////////////////////////////
/////// task: wave__wait_for_backend_convergence
/////// phase: 19
/////////////////////////////////////////////////////////////////////

type wave__backend_readiness_waits_done_facts struct {
	backend_mutation_applied                 wave__backend_mutation_applied_facts
	backend_convergence_requirements_derived wave__backend_convergence_requirements_derived_facts
}

var wave__wait_for_backend_convergence = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__backend_readiness_waits_done_facts, error) {
	// Role in system: wait until backend state converges to required readiness.
	// This task currently joins backend mutation and convergence requirements.
	var backend_mutation_applied wave__backend_mutation_applied_facts
	var backend_convergence_requirements_derived wave__backend_convergence_requirements_derived_facts
	if err := tasks_ctx.RunParallel(
		wave__apply_backend_changes.Bind(input, &backend_mutation_applied),
		wave__derive_backend_convergence_requirements.Bind(input, &backend_convergence_requirements_derived),
	); err != nil {
		return wave__backend_readiness_waits_done_facts{}, err
	}
	return wave__backend_readiness_waits_done_facts{
		backend_mutation_applied:                 backend_mutation_applied,
		backend_convergence_requirements_derived: backend_convergence_requirements_derived,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__select_frontend_outcome
/////// phase: 19
/////////////////////////////////////////////////////////////////////

type wave__terminal_frontend_action_selected_facts struct {
	backend_mutation_applied                 wave__backend_mutation_applied_facts
	backend_convergence_requirements_derived wave__backend_convergence_requirements_derived_facts
}

var wave__select_frontend_outcome = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input supercycle_input,
	) (wave__terminal_frontend_action_selected_facts, error) {
		// Role in system: choose the final frontend action for this cycle.
		// This task currently uses the same backend facts as readiness waits.
		var backend_mutation_applied wave__backend_mutation_applied_facts
		var backend_convergence_requirements_derived wave__backend_convergence_requirements_derived_facts
		if err := tasks_ctx.RunParallel(
			wave__apply_backend_changes.Bind(input, &backend_mutation_applied),
			wave__derive_backend_convergence_requirements.Bind(input, &backend_convergence_requirements_derived),
		); err != nil {
			return wave__terminal_frontend_action_selected_facts{}, err
		}
		return wave__terminal_frontend_action_selected_facts{
			backend_mutation_applied:                 backend_mutation_applied,
			backend_convergence_requirements_derived: backend_convergence_requirements_derived,
		}, nil
	},
)

/////////////////////////////////////////////////////////////////////
/////// task: wave__checkpoint_cycle_end
/////// phase: 20
/////////////////////////////////////////////////////////////////////

type wave__checkpoint_cycle_end_done_facts struct {
	backend_readiness_waits    wave__backend_readiness_waits_done_facts
	terminal_frontend_selected wave__terminal_frontend_action_selected_facts
	vorma_reactors             vorma__cycle_end_reactors_done_facts
}

var wave__checkpoint_cycle_end = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input supercycle_input,
	) (wave__checkpoint_cycle_end_done_facts, error) {
		// Role in system: cycle_end checkpoint gate.
		// This task flights and awaits non-wave-owned reactors bound to cycle_end.
		var backend_readiness_waits wave__backend_readiness_waits_done_facts
		var terminal_frontend_selected wave__terminal_frontend_action_selected_facts
		var vorma_reactors vorma__cycle_end_reactors_done_facts
		if err := tasks_ctx.RunParallel(
			wave__wait_for_backend_convergence.Bind(input, &backend_readiness_waits),
			wave__select_frontend_outcome.Bind(input, &terminal_frontend_selected),
			vorma__run_reactors_finished_by_cycle_end.Bind(input, &vorma_reactors),
		); err != nil {
			return wave__checkpoint_cycle_end_done_facts{}, err
		}
		return wave__checkpoint_cycle_end_done_facts{
			backend_readiness_waits:    backend_readiness_waits,
			terminal_frontend_selected: terminal_frontend_selected,
			vorma_reactors:             vorma_reactors,
		}, nil
	},
)

/////////////////////////////////////////////////////////////////////
/////// task: wave__execute_frontend_outcome
/////// phase: 21
/////////////////////////////////////////////////////////////////////

type wave__terminal_frontend_action_executed_facts struct {
	checkpoint_cycle_end wave__checkpoint_cycle_end_done_facts
}

var wave__execute_frontend_outcome = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input supercycle_input,
	) (wave__terminal_frontend_action_executed_facts, error) {
		// Role in system: execute the selected frontend-visible action.
		// This task currently depends on cycle_end checkpoint completion.
		checkpoint_cycle_end, err := wave__checkpoint_cycle_end.Run(
			tasks_ctx,
			input,
		)
		if err != nil {
			return wave__terminal_frontend_action_executed_facts{}, err
		}
		return wave__terminal_frontend_action_executed_facts{
			checkpoint_cycle_end: checkpoint_cycle_end,
		}, nil
	},
)

/////////////////////////////////////////////////////////////////////
/////// task: wave__publish_cycle_completion
/////// phase: 22
/////////////////////////////////////////////////////////////////////

type wave__completion_facts_emitted_facts struct {
	terminal_frontend_action wave__terminal_frontend_action_executed_facts
}

var wave__publish_cycle_completion = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__completion_facts_emitted_facts, error) {
	// Role in system: publish cycle completion facts for downstream observers.
	// This task currently depends on terminal frontend execution.
	terminal_frontend_action, err := wave__execute_frontend_outcome.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__completion_facts_emitted_facts{}, err
	}
	return wave__completion_facts_emitted_facts{
		terminal_frontend_action: terminal_frontend_action,
	}, nil
})

/////////////////////////////////////////////////////////////////////
/////// task: wave__finalize_cycle_cleanup
/////// phase: 23
/////////////////////////////////////////////////////////////////////

type wave__supercycle_cleanup_done_facts struct {
	completion_facts wave__completion_facts_emitted_facts
}

var wave__finalize_cycle_cleanup = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (wave__supercycle_cleanup_done_facts, error) {
	// Role in system: finalize and clean cycle-scoped temporary state.
	// This task currently depends on completion publication.
	completion_facts, err := wave__publish_cycle_completion.Run(
		tasks_ctx,
		input,
	)
	if err != nil {
		return wave__supercycle_cleanup_done_facts{}, err
	}
	return wave__supercycle_cleanup_done_facts{
		completion_facts: completion_facts,
	}, nil
})

func run_supercycle(
	supercycle_ctx context.Context,
	input supercycle_input,
) error {
	tasks_ctx := tasks.NewCtx(supercycle_ctx)
	return tasks_ctx.RunParallel(
		wave__finalize_cycle_cleanup.Bind(input, nil),
		wave__refresh_watcher_coverage.Bind(input, nil),
	)
}
