package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p2_effects struct {
	build_go_binary                    *tasks.Task[p2_batch_input, struct{}]
	build_critical_css                 *tasks.Task[p2_batch_input, struct{}]
	build_normal_css                   *tasks.Task[p2_batch_input, struct{}]
	process_public_static_assets       *tasks.Task[p2_batch_input, struct{}]
	cleanup_stale_public_static        *tasks.Task[p2_batch_input, p2_build_outcome_facts]
	process_private_static_assets      *tasks.Task[p2_batch_input, struct{}]
	generate_public_file_map_artifacts *tasks.Task[p2_batch_input, p2_build_outcome_facts]
	run_requested_build_effects        *tasks.Task[p2_batch_input, p2_build_outcome_facts]
	plan_p2_output                     *tasks.Task[p2_batch_input, p2_output]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p2_effects_def = p2_effects{
	build_go_binary:                    p2_build_go_binary_task,
	build_critical_css:                 p2_build_critical_css_task,
	build_normal_css:                   p2_build_normal_css_task,
	process_public_static_assets:       p2_process_public_static_assets_task,
	cleanup_stale_public_static:        p2_cleanup_stale_public_static_task,
	process_private_static_assets:      p2_process_private_static_assets_task,
	generate_public_file_map_artifacts: p2_generate_public_file_map_artifacts_task,
	run_requested_build_effects:        p2_run_requested_build_effects_task,
	plan_p2_output:                     p2_plan_p2_output_task,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var p2_build_go_binary_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p2_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P2_BUILD_GO_BINARY) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p2_build_critical_css_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p2_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P2_BUILD_CRITICAL_CSS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p2_build_normal_css_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p2_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P2_BUILD_NORMAL_CSS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p2_process_public_static_assets_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p2_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p2_cleanup_stale_public_static_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p2_batch_input,
	) (p2_build_outcome_facts, error) {
		if record_test_effect(tasks_ctx, _LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC) {
			return p2_build_outcome_facts{
				public_file_map_artifacts_changed: true,
			}, nil
		}
		return p2_build_outcome_facts{}, nil
	},
)

var p2_process_private_static_assets_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p2_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P2_PROCESS_PRIVATE_STATIC_ASSETS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p2_generate_public_file_map_artifacts_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p2_batch_input,
	) (p2_build_outcome_facts, error) {
		if is_test_env() {
			return p2_build_outcome_facts{
				public_file_map_artifacts_changed: true,
			}, nil
		}
		if _, err := p2_process_public_static_assets_task.Run(
			tasks_ctx,
			input,
		); err != nil {
			return p2_build_outcome_facts{}, err
		}
		cleanup_facts, err := p2_cleanup_stale_public_static_task.Run(
			tasks_ctx,
			input,
		)
		if err != nil {
			return p2_build_outcome_facts{}, err
		}
		return cleanup_facts.merge(p2_build_outcome_facts{}), nil
	},
)

var p2_run_requested_build_effects_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p2_batch_input,
	) (p2_build_outcome_facts, error) {
		if !input.p1_requested_effects.run_requested_build_effects {
			return p2_build_outcome_facts{}, nil
		}

		var ignored_result struct{}
		var cleanup_facts p2_build_outcome_facts
		var generate_public_file_map_facts p2_build_outcome_facts
		bound_build_tasks := make([]tasks.BoundTask, 0, 8)
		if input.p1_requested_effects.compile_go_binary {
			bound_build_tasks = append(
				bound_build_tasks,
				p2_build_go_binary_task.Bind(input, &ignored_result),
			)
		}
		if input.p1_requested_effects.build_critical_css {
			bound_build_tasks = append(
				bound_build_tasks,
				p2_build_critical_css_task.Bind(input, &ignored_result),
			)
		}
		if input.p1_requested_effects.build_normal_css {
			bound_build_tasks = append(
				bound_build_tasks,
				p2_build_normal_css_task.Bind(input, &ignored_result),
			)
		}
		if input.p1_requested_effects.process_public_static_assets {
			bound_build_tasks = append(
				bound_build_tasks,
				p2_process_public_static_assets_task.Bind(
					input,
					&ignored_result,
				),
			)
		}
		if input.p1_requested_effects.cleanup_stale_public_static_outputs {
			bound_build_tasks = append(
				bound_build_tasks,
				p2_cleanup_stale_public_static_task.Bind(
					input,
					&cleanup_facts,
				),
			)
		}
		if input.p1_requested_effects.process_private_static_assets {
			bound_build_tasks = append(
				bound_build_tasks,
				p2_process_private_static_assets_task.Bind(
					input,
					&ignored_result,
				),
			)
		}
		if input.p1_requested_effects.generate_public_file_map {
			bound_build_tasks = append(
				bound_build_tasks,
				p2_generate_public_file_map_artifacts_task.Bind(
					input,
					&generate_public_file_map_facts,
				),
			)
		}
		if err := tasks_ctx.RunParallel(bound_build_tasks...); err != nil {
			return p2_build_outcome_facts{}, err
		}
		return cleanup_facts.merge(generate_public_file_map_facts), nil
	},
)

var p2_plan_p2_output_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p2_batch_input,
	) (p2_output, error) {
		build_outcome_facts, err := p2_run_requested_build_effects_task.Run(
			tasks_ctx,
			input,
		)
		if err != nil {
			return p2_output{}, err
		}
		if input.batch.mode == mode_prod {
			return p2_output{
				build_outcome_facts: build_outcome_facts,
			}, nil
		}
		return p2_output{
			p2_requested_effects: input.p1_requested_effects.derive_p2_requested_effects(
				build_outcome_facts,
			),
			build_outcome_facts: build_outcome_facts,
		}, nil
	},
)
