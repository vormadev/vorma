package wave3

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// temporary vorma reactor placeholders (to be replaced by plugin api)
/////////////////////////////////////////////////////////////////////

type vorma__cycle_start_reactors_done_facts struct {
	user_config wave__user_config_parsed_facts
}

var vorma__run_reactors_finished_by_cycle_start = tasks.NewTask(func(
	tasks_ctx *tasks.Ctx,
	input supercycle_input,
) (vorma__cycle_start_reactors_done_facts, error) {
	// Role in system: temporary Vorma-owned reactors bound to cycle_start.
	_ = tasks_ctx
	return vorma__cycle_start_reactors_done_facts{
		user_config: wave__user_config_parsed_facts{
			current_config_fingerprint: input.current_config_fingerprint,
		},
	}, nil
})

type vorma__wave_work_go_compile_start_reactors_done_facts struct {
	pre_hook_outcomes wave__pre_hook_outcomes_merged_facts
}

var vorma__run_reactors_finished_by_wave_work_go_compile_start = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input supercycle_input,
	) (vorma__wave_work_go_compile_start_reactors_done_facts, error) {
		// Role in system: temporary Vorma-owned reactors bound to wave_work_go_compile_start.
		pre_hook_outcomes, err := wave__merge_pre_hook_outcomes.Run(
			tasks_ctx,
			input,
		)
		if err != nil {
			return vorma__wave_work_go_compile_start_reactors_done_facts{}, err
		}
		return vorma__wave_work_go_compile_start_reactors_done_facts{
			pre_hook_outcomes: pre_hook_outcomes,
		}, nil
	},
)

type vorma__cycle_end_reactors_done_facts struct {
	backend_readiness_waits    wave__backend_readiness_waits_done_facts
	terminal_frontend_selected wave__terminal_frontend_action_selected_facts
}

var vorma__run_reactors_finished_by_cycle_end = tasks.NewTask(
	func(
		tasks_ctx *tasks.Ctx,
		input supercycle_input,
	) (vorma__cycle_end_reactors_done_facts, error) {
		// Role in system: temporary Vorma-owned reactors bound to cycle_end.
		var backend_readiness_waits wave__backend_readiness_waits_done_facts
		var terminal_frontend_selected wave__terminal_frontend_action_selected_facts
		if err := tasks_ctx.RunParallel(
			wave__wait_for_backend_convergence.Bind(input, &backend_readiness_waits),
			wave__select_frontend_outcome.Bind(input, &terminal_frontend_selected),
		); err != nil {
			return vorma__cycle_end_reactors_done_facts{}, err
		}
		return vorma__cycle_end_reactors_done_facts{
			backend_readiness_waits:    backend_readiness_waits,
			terminal_frontend_selected: terminal_frontend_selected,
		}, nil
	},
)
