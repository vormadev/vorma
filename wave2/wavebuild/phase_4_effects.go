package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type phase4Effects struct {
	broadcastCSSHotReload          *tasks.Task[phase4BatchInput, struct{}]
	notifyVitePublicFileMapChanged *tasks.Task[phase4BatchInput, struct{}]
	broadcastRevalidate            *tasks.Task[phase4BatchInput, struct{}]
	broadcastHardReload            *tasks.Task[phase4BatchInput, struct{}]
	publishNoReloadNeededNotice    *tasks.Task[phase4BatchInput, struct{}]
	executeTerminalBrowserAction   *tasks.Task[phase4BatchInput, phase4CompletionSummary]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var phase4EffectsDef = phase4Effects{
	broadcastCSSHotReload:          phase4BroadcastCSSHotReloadTask,
	notifyVitePublicFileMapChanged: phase4NotifyVitePublicFileMapChangedTask,
	broadcastRevalidate:            phase4BroadcastRevalidateTask,
	broadcastHardReload:            phase4BroadcastHardReloadTask,
	publishNoReloadNeededNotice:    phase4PublishNoReloadNeededNoticeTask,
	executeTerminalBrowserAction:   phase4ExecuteTerminalBrowserActionTask,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var noopPhase4EffectTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase4BatchInput,
	) (struct{}, error) {
		return struct{}{}, nil
	},
)

var phase4BroadcastCSSHotReloadTask = noopPhase4EffectTask
var phase4NotifyVitePublicFileMapChangedTask = noopPhase4EffectTask
var phase4BroadcastRevalidateTask = noopPhase4EffectTask
var phase4BroadcastHardReloadTask = noopPhase4EffectTask
var phase4PublishNoReloadNeededNoticeTask = noopPhase4EffectTask

var phase4ExecuteTerminalBrowserActionTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase4BatchInput,
	) (phase4CompletionSummary, error) {
		terminalAction := input.phase3RequestedEffects.terminalBrowserAction
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
