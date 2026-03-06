package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

// Phase4BatchInput is the frontend-settling input produced by phase 3.
type Phase4BatchInput struct {
	Batch         PhaseBatchInput
	FrontendGoals Phase3FrontendSettlingGoals
}

// Phase4CompletionSummary captures frontend-settling completion output.
type Phase4CompletionSummary struct {
	TerminalAction FrontendTerminalBrowserAction
}

// FourPhaseRunResult captures the full four-phase pipeline execution.
type FourPhaseRunResult struct {
	Phase1BuildGoals        Phase1BuildGoals
	Phase2BackendGoals      Phase2BackendSettlingGoals
	Phase3FrontendGoals     Phase3FrontendSettlingGoals
	Phase4CompletionSummary Phase4CompletionSummary
	FrameworkSignals        []FrameworkSignal
}

func newPhase4EffectTask(
	effectID PhaseEffectID,
) *tasks.Task[Phase4BatchInput, struct{}] {
	return tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			input Phase4BatchInput,
		) (struct{}, error) {
			if effectError := executePhaseEffect(
				taskContext,
				input.Batch,
				effectID,
			); effectError != nil {
				return struct{}{}, effectError
			}
			return struct{}{}, nil
		},
	)
}

// Phase4BroadcastCSSHotReloadTask executes CSS hot reload broadcast.
var Phase4BroadcastCSSHotReloadTask = newPhase4EffectTask(
	PhaseEffectIDFrontendBroadcastCSSHotReload,
)

// Phase4NotifyVitePublicFileMapChangedTask notifies Vite that public file-map artifacts changed.
var Phase4NotifyVitePublicFileMapChangedTask = newPhase4EffectTask(
	PhaseEffectIDFrontendNotifyVitePublicFileMapChanged,
)

// Phase4BroadcastRevalidateTask executes browser revalidation broadcast.
var Phase4BroadcastRevalidateTask = newPhase4EffectTask(
	PhaseEffectIDFrontendBroadcastRevalidate,
)

// Phase4BroadcastHardReloadTask executes browser hard reload broadcast.
var Phase4BroadcastHardReloadTask = newPhase4EffectTask(
	PhaseEffectIDFrontendBroadcastHardReload,
)

// Phase4PublishNoReloadNeededNoticeTask executes no-reload user messaging.
var Phase4PublishNoReloadNeededNoticeTask = newPhase4EffectTask(
	PhaseEffectIDFrontendPublishNoReloadNeededNotice,
)

// RunPhase4TaskGraph executes phase 4 and returns completion summary.
func RunPhase4TaskGraph(
	taskContext *tasks.Ctx,
	input Phase4BatchInput,
) (Phase4CompletionSummary, error) {
	terminalAction := input.FrontendGoals.TerminalBrowserAction
	switch terminalAction {
	case FrontendTerminalBrowserActionHardReload:
		if _, hardReloadError := Phase4BroadcastHardReloadTask.Run(
			taskContext,
			input,
		); hardReloadError != nil {
			return Phase4CompletionSummary{}, hardReloadError
		}
	case FrontendTerminalBrowserActionNotifyVitePublicFileMapChanged:
		if _, notifyViteError := Phase4NotifyVitePublicFileMapChangedTask.Run(
			taskContext,
			input,
		); notifyViteError != nil {
			return Phase4CompletionSummary{}, notifyViteError
		}
	case FrontendTerminalBrowserActionRevalidate:
		if _, revalidateError := Phase4BroadcastRevalidateTask.Run(
			taskContext,
			input,
		); revalidateError != nil {
			return Phase4CompletionSummary{}, revalidateError
		}
	case FrontendTerminalBrowserActionCSSHotReload:
		if _, cssHotReloadError := Phase4BroadcastCSSHotReloadTask.Run(
			taskContext,
			input,
		); cssHotReloadError != nil {
			return Phase4CompletionSummary{}, cssHotReloadError
		}
	default:
		if _, noReloadNoticeError := Phase4PublishNoReloadNeededNoticeTask.Run(
			taskContext,
			input,
		); noReloadNoticeError != nil {
			return Phase4CompletionSummary{}, noReloadNoticeError
		}
	}

	return Phase4CompletionSummary{TerminalAction: terminalAction}, nil
}
