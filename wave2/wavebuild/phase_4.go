package wavebuild

/////////////////////////////////////////////////////////////////////
/////// Phase Contracts
/////////////////////////////////////////////////////////////////////

// phase4BatchInput is the frontend-settling input produced by phase 3.
type phase4BatchInput struct {
	batch                  phaseBatchInput
	phase3RequestedEffects phase3RequestedEffects
}

// phase4CompletionSummary captures frontend-settling completion output.
type phase4CompletionSummary struct {
	terminalAction             frontendTerminalBrowserAction
	requiresBackendViteHealing bool
}

// fourPhaseRunResult captures the full four-phase pipeline execution.
type fourPhaseRunResult struct {
	phase1RequestedEffects  phase1RequestedEffects
	phase2RequestedEffects  phase2RequestedEffects
	phase3RequestedEffects  phase3RequestedEffects
	phase4CompletionSummary phase4CompletionSummary
	frameworkSignals        []FrameworkSignal
}
