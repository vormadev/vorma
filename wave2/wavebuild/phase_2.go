package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// p2_batch_input is the build-phase input produced by phase 1.
type p2_batch_input struct {
	batch                phase_batch_input
	p1_requested_effects p1_requested_effects
}

// p2_build_outcome_facts are observable build outcome facts produced by phase 2.
//
// These facts are phase-2 outputs consumed by later phase planners; they are
// not direct watcher-event classifications.
type p2_build_outcome_facts struct {
	public_file_map_artifacts_changed  bool
	public_file_map_artifacts_repaired bool
}

// p2_requested_effects are backend-mutation effects requested by phase 2.
type p2_requested_effects struct {
	restart_dev_server_cycle          bool
	restart_app_process               bool
	restart_vite_process              bool
	await_backend_readiness           bool
	queue_retry_wait_restart          bool
	requested_terminal_browser_action frontend_terminal_browser_action
	fw_execution_registrations        *fw_execution_registrations
	fw_requested_effects              *fw_requested_effects
}

// p2_output is the full phase-2 planner output consumed by phase 3.
type p2_output struct {
	p2_requested_effects p2_requested_effects
	build_outcome_facts  p2_build_outcome_facts
}

/////////////////////////////////////////////////////////////////////
/////// Phase Reductions
/////////////////////////////////////////////////////////////////////

func (left p2_build_outcome_facts) merge(
	right p2_build_outcome_facts,
) p2_build_outcome_facts {
	return p2_build_outcome_facts{
		public_file_map_artifacts_changed: left.public_file_map_artifacts_changed ||
			right.public_file_map_artifacts_changed,
		public_file_map_artifacts_repaired: left.public_file_map_artifacts_repaired ||
			right.public_file_map_artifacts_repaired,
	}
}

func (re p1_requested_effects) derive_p2_requested_effects(
	build_outcome_facts p2_build_outcome_facts,
) p2_requested_effects {
	if re.queue_retry_wait_restart {
		return p2_requested_effects{
			queue_retry_wait_restart: true,
		}
	}

	requested_terminal_browser_action := re.requested_terminal_browser_action
	fw_requested_effects_val := fw_requested_effects_from_pointer(
		re.fw_requested_effects,
	)
	if build_outcome_facts.public_file_map_artifacts_changed ||
		build_outcome_facts.public_file_map_artifacts_repaired {
		requested_terminal_browser_action = requested_terminal_browser_action.dominant_with(
			frontend_terminal_browser_action_notify_vite_public_file_map_changed,
		)
		if re.wave_public_file_map_notif_destination_key != "" {
			fw_requested_effects_val = fw_requested_effects_val.merge(
				fw_requested_effects{
					backend_convergence_notif_queue: []fw_notif_request{
						{
							destination_key: re.wave_public_file_map_notif_destination_key,
							trigger:         "wave-public-filemap-artifacts-changed",
						},
					},
				},
			)
		}
	}

	requested_effects := p2_requested_effects{
		restart_dev_server_cycle: re.restart_dev_server_cycle,
		restart_app_process: re.request_backend_restart ||
			re.compile_go_binary,
		restart_vite_process:              re.request_vite_restart,
		requested_terminal_browser_action: requested_terminal_browser_action,
		fw_execution_registrations:        re.fw_execution_registrations,
		fw_requested_effects: fw_new_requested_effects_pointer_if_any(
			fw_requested_effects_val,
		),
	}
	requested_effects.await_backend_readiness =
		requested_effects.restart_dev_server_cycle ||
			requested_effects.restart_app_process ||
			requested_effects.restart_vite_process ||
			fw_requested_effects_from_pointer(
				requested_effects.fw_requested_effects,
			).has_any()
	return requested_effects
}

func (left p2_requested_effects) merge(
	right p2_requested_effects,
) p2_requested_effects {
	fw_execution_registrations := left.fw_execution_registrations
	if fw_execution_registrations == nil {
		fw_execution_registrations = right.fw_execution_registrations
	}
	return p2_requested_effects{
		restart_dev_server_cycle: left.restart_dev_server_cycle ||
			right.restart_dev_server_cycle,
		restart_app_process: left.restart_app_process ||
			right.restart_app_process,
		restart_vite_process: left.restart_vite_process ||
			right.restart_vite_process,
		await_backend_readiness: left.await_backend_readiness ||
			right.await_backend_readiness,
		queue_retry_wait_restart: left.queue_retry_wait_restart ||
			right.queue_retry_wait_restart,
		requested_terminal_browser_action: left.requested_terminal_browser_action.dominant_with(
			right.requested_terminal_browser_action,
		),
		fw_execution_registrations: fw_execution_registrations,
		fw_requested_effects: fw_new_requested_effects_pointer_if_any(
			fw_requested_effects_from_pointer(
				left.fw_requested_effects,
			).merge(
				fw_requested_effects_from_pointer(
					right.fw_requested_effects,
				),
			),
		),
	}
}
