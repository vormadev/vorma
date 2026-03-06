package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// p5_BatchInput is the frontend-settling input produced by phase 4.
type p5_BatchInput struct {
	batch               phaseBatchInput
	p5_RequestedEffects p5_RequestedEffects
}

// p5_CompletionSummary captures frontend-settling completion output.
type p5_CompletionSummary struct {
	terminalAction             frontendTerminalBrowserAction
	requiresBackendViteHealing bool
}

// fivePhaseRunResult captures the full five-phase pipeline execution.
type fivePhaseRunResult struct {
	p1_RequestedEffects  p1_RequestedEffects
	p2_RequestedEffects  p2_RequestedEffects
	p3_Output            p3_Output
	p4_Output            p4_Output
	p5_CompletionSummary p5_CompletionSummary
	frameworkSignals     []FrameworkSignal
}
