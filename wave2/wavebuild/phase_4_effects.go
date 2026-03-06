package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p4_Effects struct {
	broadcastCSSHotReload          *tasks.Task[p4_BatchInput, struct{}]
	notifyVitePublicFileMapChanged *tasks.Task[p4_BatchInput, struct{}]
	broadcastRevalidate            *tasks.Task[p4_BatchInput, struct{}]
	broadcastHardReload            *tasks.Task[p4_BatchInput, struct{}]
	publishNoReloadNeededNotice    *tasks.Task[p4_BatchInput, struct{}]
	executeTerminalBrowserAction   *tasks.Task[p4_BatchInput, p4_CompletionSummary]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p4_EffectsDef = p4_Effects{
	broadcastCSSHotReload:          p4_BroadcastCSSHotReloadTask,
	notifyVitePublicFileMapChanged: p4_NotifyVitePublicFileMapChangedTask,
	broadcastRevalidate:            p4_BroadcastRevalidateTask,
	broadcastHardReload:            p4_BroadcastHardReloadTask,
	publishNoReloadNeededNotice:    p4_PublishNoReloadNeededNoticeTask,
	executeTerminalBrowserAction:   p4_ExecuteTerminalBrowserActionTask,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var p4_BroadcastCSSHotReloadTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p4_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P4_BROADCAST_CSS_HOT_RELOAD) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p4_NotifyVitePublicFileMapChangedTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p4_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(
			tasksCtx,
			_LABEL_P4_NOTIFY_VITE_PUBLIC_FILEMAP_CHANGED,
		) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p4_BroadcastRevalidateTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p4_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P4_BROADCAST_REVALIDATE) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p4_BroadcastHardReloadTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p4_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P4_BROADCAST_HARD_RELOAD) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p4_PublishNoReloadNeededNoticeTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p4_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(
			tasksCtx,
			_LABEL_P4_PUBLISH_NO_RELOAD_NEEDED_NOTICE,
		) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p4_ExecuteTerminalBrowserActionTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p4_BatchInput,
	) (p4_CompletionSummary, error) {
		if recordTestEffect(
			tasksCtx,
			_LABEL_P4_EXECUTE_TERMINAL_BROWSER_ACTION,
		) {
			return p4_CompletionSummary{}, nil
		}
		terminalAction := input.p3_RequestedEffects.terminalBrowserAction
		switch terminalAction {
		case frontendTerminalBrowserActionHardReload:
			if _, hardReloadError := p4_BroadcastHardReloadTask.Run(
				tasksCtx,
				input,
			); hardReloadError != nil {
				return p4_CompletionSummary{}, hardReloadError
			}
		case frontendTerminalBrowserActionNotifyVitePublicFileMapChanged:
			if _, notifyViteError := p4_NotifyVitePublicFileMapChangedTask.Run(
				tasksCtx,
				input,
			); notifyViteError != nil {
				return p4_CompletionSummary{
					terminalAction:             frontendTerminalBrowserActionNone,
					requiresBackendViteHealing: true,
				}, nil
			}
		case frontendTerminalBrowserActionRevalidate:
			if _, revalidateError := p4_BroadcastRevalidateTask.Run(
				tasksCtx,
				input,
			); revalidateError != nil {
				return p4_CompletionSummary{}, revalidateError
			}
		case frontendTerminalBrowserActionCSSHotReload:
			if _, cssHotReloadError := p4_BroadcastCSSHotReloadTask.Run(
				tasksCtx,
				input,
			); cssHotReloadError != nil {
				return p4_CompletionSummary{}, cssHotReloadError
			}
		default:
			if _, noReloadNoticeError := p4_PublishNoReloadNeededNoticeTask.Run(
				tasksCtx,
				input,
			); noReloadNoticeError != nil {
				return p4_CompletionSummary{}, noReloadNoticeError
			}
		}

		return p4_CompletionSummary{terminalAction: terminalAction}, nil
	},
)
