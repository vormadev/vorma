package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// p4_BatchInput is the frontend-settling input produced by phase 3.
type p4_BatchInput struct {
	batch               phaseBatchInput
	p3_RequestedEffects p3_RequestedEffects
}

// p4_CompletionSummary captures frontend-settling completion output.
type p4_CompletionSummary struct {
	terminalAction             frontendTerminalBrowserAction
	requiresBackendViteHealing bool
}

// fourPhaseRunResult captures the full four-phase pipeline execution.
type fourPhaseRunResult struct {
	p1_RequestedEffects  p1_RequestedEffects
	p2_RequestedEffects  p2_RequestedEffects
	p3_RequestedEffects  p3_RequestedEffects
	p4_CompletionSummary p4_CompletionSummary
	frameworkSignals     []FrameworkSignal
}
