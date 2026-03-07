package wavebuild

import (
	"errors"

	"github.com/vormadev/vorma/kit/tasks"
)

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p3_effects struct {
	apply_dev_server_restart    *tasks.Task[p3_batch_input, struct{}]
	queue_retry_wait_restart    *tasks.Task[p3_batch_input, struct{}]
	restart_app_process         *tasks.Task[p3_batch_input, struct{}]
	restart_vite_process        *tasks.Task[p3_batch_input, struct{}]
	fw_execute_mutation_effects *tasks.Task[p3_batch_input, struct{}]
	plan_p4_requested_effects   *tasks.Task[p3_batch_input, p3_output]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p3_effects_def = p3_effects{
	apply_dev_server_restart:    p3_apply_dev_server_restart_task,
	queue_retry_wait_restart:    p3_queue_retry_wait_restart_task,
	restart_app_process:         p3_restart_app_process_task,
	restart_vite_process:        p3_restart_vite_process_task,
	fw_execute_mutation_effects: p3_fw_execute_mutation_effects_task,
	plan_p4_requested_effects:   p3_plan_p4_requested_effects_task,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var p3_apply_dev_server_restart_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p3_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P3_APPLY_DEV_SERVER_RESTART) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_queue_retry_wait_restart_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p3_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P3_QUEUE_RETRY_WAIT_RESTART) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_restart_app_process_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p3_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P3_RESTART_APP_PROCESS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_restart_vite_process_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p3_batch_input,
	) (struct{}, error) {
		if record_test_effect(tasks_ctx, _LABEL_P3_RESTART_VITE_PROCESS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_fw_execute_mutation_effects_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p3_batch_input,
	) (struct{}, error) {
		fw_requested_effects := fw_requested_effects_from_pointer(
			input.p2_requested_effects.fw_requested_effects,
		)
		if !fw_requested_effects.has_backend_mutation_effects() {
			return struct{}{}, nil
		}
		if is_test_env() {
			for _, effect_key := range fw_requested_effects.backend_mutation_effect_keys {
				record_test_effect(
					tasks_ctx,
					_LABEL_P3_EXECUTE_Fw_MUTATION_EFFECT+
						"["+
						string(effect_key)+
						"]",
				)
			}
			return struct{}{}, nil
		}
		registrations := input.p2_requested_effects.fw_execution_registrations
		if registrations == nil {
			return struct{}{}, errors.New(
				"wavebuild: fw execution registrations are required for backend mutation effects",
			)
		}
		if len(registrations.backend_mutation_effects_by_key) == 0 {
			return struct{}{}, errors.New(
				"wavebuild: backend mutation fw effect registry is empty",
			)
		}
		var ignored_result struct{}
		bound_fw_mutation_tasks := make(
			[]tasks.BoundTask,
			0,
			len(fw_requested_effects.backend_mutation_effect_keys),
		)
		for _, effect_key := range fw_requested_effects.backend_mutation_effect_keys {
			fw_mutation_task, fw_has_mutation_task := registrations.backend_mutation_effects_by_key[effect_key]
			if !fw_has_mutation_task || fw_mutation_task == nil {
				return struct{}{}, errors.New(
					"wavebuild: backend mutation fw effect task is not registered for key " +
						string(
							effect_key,
						),
				)
			}
			bound_fw_mutation_tasks = append(
				bound_fw_mutation_tasks,
				fw_mutation_task.Bind(input, &ignored_result),
			)
		}
		if err := tasks_ctx.RunParallel(
			bound_fw_mutation_tasks...,
		); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	},
)

var p3_plan_p4_requested_effects_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p3_batch_input,
	) (p3_output, error) {
		if input.p2_requested_effects.queue_retry_wait_restart {
			if _, err := p3_queue_retry_wait_restart_task.Run(
				tasks_ctx,
				input,
			); err != nil {
				return p3_output{}, err
			}
			return p3_output{
				p4_requested_effects: input.p2_requested_effects.derive_p4_requested_effects(),
			}, nil
		}

		var ignored_result struct{}
		backend_mutation_tasks := make([]tasks.BoundTask, 0, 4)
		if input.p2_requested_effects.restart_dev_server_cycle {
			backend_mutation_tasks = append(
				backend_mutation_tasks,
				p3_apply_dev_server_restart_task.Bind(
					input,
					&ignored_result,
				),
			)
		}
		if input.p2_requested_effects.restart_app_process {
			backend_mutation_tasks = append(
				backend_mutation_tasks,
				p3_restart_app_process_task.Bind(input, &ignored_result),
			)
		}
		if input.p2_requested_effects.restart_vite_process {
			backend_mutation_tasks = append(
				backend_mutation_tasks,
				p3_restart_vite_process_task.Bind(
					input,
					&ignored_result,
				),
			)
		}
		if fw_requested_effects_from_pointer(
			input.p2_requested_effects.fw_requested_effects,
		).has_backend_mutation_effects() {
			backend_mutation_tasks = append(
				backend_mutation_tasks,
				p3_fw_execute_mutation_effects_task.Bind(
					input,
					&ignored_result,
				),
			)
		}
		if len(backend_mutation_tasks) > 0 {
			if err := tasks_ctx.RunParallel(backend_mutation_tasks...); err != nil {
				return p3_output{}, err
			}
		}
		return p3_output{
			p4_requested_effects: input.p2_requested_effects.derive_p4_requested_effects(),
		}, nil
	},
)
