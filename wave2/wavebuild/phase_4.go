package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

// phase4BatchInput is the frontend-settling input produced by phase 3.
type phase4BatchInput struct {
	batch         phaseBatchInput
	frontendGoals phase3FrontendSettlingGoals
}

// phase4CompletionSummary captures frontend-settling completion output.
type phase4CompletionSummary struct {
	terminalAction             frontendTerminalBrowserAction
	requiresBackendViteHealing bool
}

// fourPhaseRunResult captures the full four-phase pipeline execution.
type fourPhaseRunResult struct {
	phase1BuildGoals        phase1BuildGoals
	phase2BackendGoals      phase2BackendSettlingGoals
	phase3FrontendGoals     phase3FrontendSettlingGoals
	phase4CompletionSummary phase4CompletionSummary
	frameworkSignals        []FrameworkSignal
}

func newPhase4EffectTask(
	effectID phaseEffectID,
) *tasks.Task[phase4BatchInput, struct{}] {
	return tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			input phase4BatchInput,
		) (struct{}, error) {
			if _, effectError := input.batch.runPhaseEffect(
				taskContext,
				effectID,
			); effectError != nil {
				return struct{}{}, effectError
			}
			return struct{}{}, nil
		},
	)
}

// phase4BroadcastCSSHotReloadTask executes CSS hot reload broadcast.
var phase4BroadcastCSSHotReloadTask = newPhase4EffectTask(
	phaseEffectIDFrontendBroadcastCSSHotReload,
)

// phase4NotifyVitePublicFileMapChangedTask notifies Vite that public file-map artifacts changed.
var phase4NotifyVitePublicFileMapChangedTask = newPhase4EffectTask(
	phaseEffectIDFrontendNotifyVitePublicFileMapChanged,
)

// phase4BroadcastRevalidateTask executes browser revalidation broadcast.
var phase4BroadcastRevalidateTask = newPhase4EffectTask(
	phaseEffectIDFrontendBroadcastRevalidate,
)

// phase4BroadcastHardReloadTask executes browser hard reload broadcast.
var phase4BroadcastHardReloadTask = newPhase4EffectTask(
	phaseEffectIDFrontendBroadcastHardReload,
)

// phase4PublishNoReloadNeededNoticeTask executes no-reload user messaging.
var phase4PublishNoReloadNeededNoticeTask = newPhase4EffectTask(
	phaseEffectIDFrontendPublishNoReloadNeededNotice,
)

// phase4ExecuteTerminalBrowserActionTask executes phase-4 terminal browser
// action and returns completion summary.
var phase4ExecuteTerminalBrowserActionTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase4BatchInput,
	) (phase4CompletionSummary, error) {
		terminalAction := input.frontendGoals.terminalBrowserAction
		switch terminalAction {
		case frontendTerminalBrowserActionHardReload:
			if _, hardReloadError := phase4BroadcastHardReloadTask.Run(
				taskContext,
				input,
			); hardReloadError != nil {
				return phase4CompletionSummary{}, hardReloadError
			}
		case frontendTerminalBrowserActionNotifyVitePublicFileMapChanged:
			if _, notifyViteError := phase4NotifyVitePublicFileMapChangedTask.Run(
				taskContext,
				input,
			); notifyViteError != nil {
				return phase4CompletionSummary{
					terminalAction:             frontendTerminalBrowserActionNone,
					requiresBackendViteHealing: true,
				}, nil
			}
		case frontendTerminalBrowserActionRevalidate:
			if _, revalidateError := phase4BroadcastRevalidateTask.Run(
				taskContext,
				input,
			); revalidateError != nil {
				return phase4CompletionSummary{}, revalidateError
			}
		case frontendTerminalBrowserActionCSSHotReload:
			if _, cssHotReloadError := phase4BroadcastCSSHotReloadTask.Run(
				taskContext,
				input,
			); cssHotReloadError != nil {
				return phase4CompletionSummary{}, cssHotReloadError
			}
		default:
			if _, noReloadNoticeError := phase4PublishNoReloadNeededNoticeTask.Run(
				taskContext,
				input,
			); noReloadNoticeError != nil {
				return phase4CompletionSummary{}, noReloadNoticeError
			}
		}

		return phase4CompletionSummary{terminalAction: terminalAction}, nil
	},
)
