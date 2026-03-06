package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p5_Effects struct {
	broadcastCSSHotReload          *tasks.Task[p5_BatchInput, struct{}]
	notifyVitePublicFileMapChanged *tasks.Task[p5_BatchInput, struct{}]
	broadcastRevalidate            *tasks.Task[p5_BatchInput, struct{}]
	broadcastHardReload            *tasks.Task[p5_BatchInput, struct{}]
	publishNoReloadNeededNotice    *tasks.Task[p5_BatchInput, struct{}]
	executeTerminalBrowserAction   *tasks.Task[p5_BatchInput, p5_CompletionSummary]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p5_EffectsDef = p5_Effects{
	broadcastCSSHotReload:          p5_BroadcastCSSHotReloadTask,
	notifyVitePublicFileMapChanged: p5_NotifyVitePublicFileMapChangedTask,
	broadcastRevalidate:            p5_BroadcastRevalidateTask,
	broadcastHardReload:            p5_BroadcastHardReloadTask,
	publishNoReloadNeededNotice:    p5_PublishNoReloadNeededNoticeTask,
	executeTerminalBrowserAction:   p5_ExecuteTerminalBrowserActionTask,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var p5_BroadcastCSSHotReloadTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p5_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P5_BROADCAST_CSS_HOT_RELOAD) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p5_NotifyVitePublicFileMapChangedTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p5_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(
			tasksCtx,
			_LABEL_P5_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
		) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p5_BroadcastRevalidateTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p5_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P5_BROADCAST_REVALIDATE) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p5_BroadcastHardReloadTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p5_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P5_BROADCAST_HARD_RELOAD) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p5_PublishNoReloadNeededNoticeTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p5_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(
			tasksCtx,
			_LABEL_P5_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
		) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p5_ExecuteTerminalBrowserActionTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p5_BatchInput,
	) (p5_CompletionSummary, error) {
		terminalAction := input.p5_RequestedEffects.terminalBrowserAction
		switch terminalAction {
		case frontendTerminalBrowserActionHardReload:
			if _, hardReloadError := p5_BroadcastHardReloadTask.Run(
				tasksCtx,
				input,
			); hardReloadError != nil {
				return p5_CompletionSummary{}, hardReloadError
			}
		case frontendTerminalBrowserActionNotifyVitePublicFileMapChanged:
			if _, notifyViteError := p5_NotifyVitePublicFileMapChangedTask.Run(
				tasksCtx,
				input,
			); notifyViteError != nil {
				return p5_CompletionSummary{
					terminalAction:             frontendTerminalBrowserActionNone,
					requiresBackendViteHealing: true,
				}, nil
			}
		case frontendTerminalBrowserActionRevalidate:
			if _, revalidateError := p5_BroadcastRevalidateTask.Run(
				tasksCtx,
				input,
			); revalidateError != nil {
				return p5_CompletionSummary{}, revalidateError
			}
		case frontendTerminalBrowserActionCSSHotReload:
			if _, cssHotReloadError := p5_BroadcastCSSHotReloadTask.Run(
				tasksCtx,
				input,
			); cssHotReloadError != nil {
				return p5_CompletionSummary{}, cssHotReloadError
			}
		default:
			if _, noReloadNoticeError := p5_PublishNoReloadNeededNoticeTask.Run(
				tasksCtx,
				input,
			); noReloadNoticeError != nil {
				return p5_CompletionSummary{}, noReloadNoticeError
			}
		}

		return p5_CompletionSummary{terminalAction: terminalAction}, nil
	},
)
