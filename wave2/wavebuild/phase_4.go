package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// p4_BatchInput is the backend-convergence input produced by phase 3.
type p4_BatchInput struct {
	batch               phaseBatchInput
	p4_RequestedEffects p4_RequestedEffects
}

// p5_RequestedEffects are frontend-settling effects requested by phase 4.
type p5_RequestedEffects struct {
	terminalBrowserAction frontendTerminalBrowserAction
}

// p4_Output is the full phase-4 planner output consumed by phase 5.
type p4_Output struct {
	p5_RequestedEffects p5_RequestedEffects
}
