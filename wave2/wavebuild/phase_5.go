package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// p5_batch_input is the frontend-settling input produced by phase 4.
type p5_batch_input struct {
	batch                phase_batch_input
	p5_requested_effects p5_requested_effects
}

// p5_completion_summary captures frontend-settling completion output.
type p5_completion_summary struct {
	terminal_action               frontend_terminal_browser_action
	requires_backend_vite_healing bool
}

// five_phase_run_result captures the full five-phase pipeline execution.
type five_phase_run_result struct {
	p1_requested_effects  p1_requested_effects
	p2_requested_effects  p2_requested_effects
	p3_output             p3_output
	p4_output             p4_output
	p5_completion_summary p5_completion_summary
	fw_notifications      []fw_notif
}
