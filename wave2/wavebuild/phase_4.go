package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// p4_batch_input is the backend-convergence input produced by phase 3.
type p4_batch_input struct {
	batch                phase_batch_input
	p4_requested_effects p4_requested_effects
}

// p4_fw_notif_task_input is one backend-convergence fw
// notification execution request.
type p4_fw_notif_task_input struct {
	batch phase_batch_input
	notif *fw_notif_request
}

// p5_requested_effects are frontend-settling effects requested by phase 4.
type p5_requested_effects struct {
	terminal_browser_action frontend_terminal_browser_action
}

// p4_output is the full phase-4 planner output consumed by phase 5.
type p4_output struct {
	p5_requested_effects                        p5_requested_effects
	requires_backend_restart_without_go_compile bool
	skip_frontend_settling                      bool
}
