package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// p3_batch_input is the backend-mutation input produced by phase 2.
type p3_batch_input struct {
	batch                phase_batch_input
	p2_requested_effects p2_requested_effects
}

// p4_requested_effects are backend-convergence effects requested by phase 3.
type p4_requested_effects struct {
	await_backend_readiness    bool
	fw_execution_registrations *fw_execution_registrations
	fw_requested_effects       *fw_requested_effects
	p5_requested_effects       p5_requested_effects
}

// p3_output is the full phase-3 planner output consumed by phase 4.
type p3_output struct {
	p4_requested_effects p4_requested_effects
}

/////////////////////////////////////////////////////////////////////
/////// Phase Reductions
/////////////////////////////////////////////////////////////////////

func (requested p2_requested_effects) derive_p4_requested_effects() p4_requested_effects {
	if requested.queue_retry_wait_restart {
		return p4_requested_effects{
			p5_requested_effects: p5_requested_effects{
				terminal_browser_action: frontend_terminal_browser_action_none,
			},
		}
	}

	terminal_browser_action := requested.requested_terminal_browser_action
	if requested.restart_vite_process &&
		terminal_browser_action == frontend_terminal_browser_action_notify_vite_public_file_map_changed {
		terminal_browser_action = frontend_terminal_browser_action_none
	}

	// Restarting Vite currently requires terminal hard reload so browser clients
	// reconnect against the active Vite endpoint.
	// Potential policy refinement: require this only when restart changes the
	// effective browser-facing Vite endpoint (for example, port change).
	if requested.restart_dev_server_cycle ||
		requested.restart_app_process ||
		requested.restart_vite_process {
		terminal_browser_action = terminal_browser_action.dominant_with(
			frontend_terminal_browser_action_hard_reload,
		)
	}
	return p4_requested_effects{
		await_backend_readiness:    requested.await_backend_readiness,
		fw_execution_registrations: requested.fw_execution_registrations,
		fw_requested_effects:       requested.fw_requested_effects,
		p5_requested_effects: p5_requested_effects{
			terminal_browser_action: terminal_browser_action,
		},
	}
}
