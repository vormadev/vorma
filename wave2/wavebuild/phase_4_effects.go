package wavebuild

import (
	"errors"

	"github.com/vormadev/vorma/kit/tasks"
)

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p4_effects struct {
	await_backend_readiness   *tasks.Task[p4_batch_input, struct{}]
	fw_execute_notifs         *tasks.Task[p4_batch_input, p4_output]
	plan_p5_requested_effects *tasks.Task[p4_batch_input, p4_output]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p4_effects_def = p4_effects{
	await_backend_readiness:   p4_await_backend_readiness_task,
	fw_execute_notifs:         p4_fw_execute_notifs_task,
	plan_p5_requested_effects: p4_plan_p5_requested_effects_task,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var p4_await_backend_readiness_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p4_batch_input,
	) (struct{}, error) {
		if !input.p4_requested_effects.await_backend_readiness {
			return struct{}{}, nil
		}
		if record_test_effect(tasks_ctx, _LABEL_P4_AWAIT_BACKEND_READINESS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p4_fw_execute_notifs_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p4_batch_input,
	) (p4_output, error) {
		fw_requested_effects := fw_requested_effects_from_pointer(
			input.p4_requested_effects.fw_requested_effects,
		)
		if !fw_requested_effects.has_backend_convergence_notifs() {
			return p4_output{}, nil
		}
		if is_test_env() {
			for _, notif := range fw_requested_effects.backend_convergence_notif_queue {
				normalized_notif := notif.normalize()
				record_test_effect(
					tasks_ctx,
					_LABEL_P4_EXECUTE_Fw_NOTIFICATION+
						"["+
						string(normalized_notif.destination_key)+
						"]",
				)
			}
			return p4_output{}, nil
		}
		registrations := input.p4_requested_effects.fw_execution_registrations
		if registrations == nil {
			return p4_output{}, errors.New(
				"wavebuild: fw execution registrations are required for backend convergence notifications",
			)
		}
		if len(
			registrations.backend_convergence_notifs_by_destination,
		) == 0 {
			return p4_output{}, errors.New(
				"wavebuild: backend convergence notification registry is empty",
			)
		}
		for _, notif := range fw_requested_effects.backend_convergence_notif_queue {
			normalized_notif := notif.normalize()
			if normalized_notif.destination_key == "" {
				continue
			}
			notif_task, has_notif_task := registrations.backend_convergence_notifs_by_destination[normalized_notif.destination_key]
			if !has_notif_task || notif_task == nil {
				return p4_output{}, errors.New(
					"wavebuild: backend convergence notification task is not registered for destination " +
						string(
							normalized_notif.destination_key,
						),
				)
			}
			notif_for_task := normalized_notif
			_, err := notif_task.Run(
				tasks_ctx,
				p4_fw_notif_task_input{
					batch: input.batch,
					notif: &notif_for_task,
				},
			)
			if err == nil {
				continue
			}
			if normalized_notif.failure_policy ==
				fw_notif_failure_policy_restart_backend_without_go_compile {
				return p4_output{
					requires_backend_restart_without_go_compile: true,
					skip_frontend_settling:                      true,
				}, nil
			}
			return p4_output{}, err
		}
		return p4_output{}, nil
	},
)

var p4_plan_p5_requested_effects_task = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input p4_batch_input,
	) (p4_output, error) {
		if _, err := p4_await_backend_readiness_task.Run(
			tasks_ctx,
			input,
		); err != nil {
			return p4_output{}, err
		}
		fw_notif_execution_output, err := p4_fw_execute_notifs_task.Run(
			tasks_ctx,
			input,
		)
		if err != nil {
			return p4_output{}, err
		}
		return p4_output{
			p5_requested_effects:                        input.p4_requested_effects.p5_requested_effects,
			requires_backend_restart_without_go_compile: fw_notif_execution_output.requires_backend_restart_without_go_compile,
			skip_frontend_settling:                      fw_notif_execution_output.skip_frontend_settling,
		}, nil
	},
)
