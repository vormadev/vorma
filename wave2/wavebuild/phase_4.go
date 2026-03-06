package wavebuild

import (
	"context"

	"github.com/vormadev/vorma/kit/tasks"
)

// Phase4BatchInput is the frontend-settling input produced by phase 3.
type Phase4BatchInput struct {
	Batch         PhaseBatchInput
	FrontendGoals Phase3FrontendSettlingGoals
}

// Phase4TerminalAction identifies one final frontend-settling action.
type Phase4TerminalAction string

const (
	// Phase4TerminalActionNone represents no browser action.
	Phase4TerminalActionNone Phase4TerminalAction = "none"
	// Phase4TerminalActionCSSHotReload represents CSS-only hot reload.
	Phase4TerminalActionCSSHotReload Phase4TerminalAction = "css_hot_reload"
	// Phase4TerminalActionInvalidateAssets represents browser asset invalidation.
	Phase4TerminalActionInvalidateAssets Phase4TerminalAction = "invalidate_assets"
	// Phase4TerminalActionRevalidate represents browser revalidation.
	Phase4TerminalActionRevalidate Phase4TerminalAction = "revalidate"
	// Phase4TerminalActionHardReload represents browser hard reload.
	Phase4TerminalActionHardReload Phase4TerminalAction = "hard_reload"
)

// Phase4CompletionSummary captures frontend-settling completion output.
type Phase4CompletionSummary struct {
	TerminalAction Phase4TerminalAction
}

// FourPhaseRunResult captures the full four-phase pipeline execution.
type FourPhaseRunResult struct {
	Phase1BuildGoals        Phase1BuildGoals
	Phase2BackendGoals      Phase2BackendSettlingGoals
	Phase3FrontendGoals     Phase3FrontendSettlingGoals
	Phase4CompletionSummary Phase4CompletionSummary
	FrameworkSignals        []FrameworkSignal
}

// Phase4EnsureFrontendSettlingEnvelopeTask captures frontend-settling envelope.
var Phase4EnsureFrontendSettlingEnvelopeTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (Phase4BatchInput, error) {
		return input, nil
	},
)

// Phase4EnsureBrowserBroadcastChannelReadyTask validates browser channel readiness.
var Phase4EnsureBrowserBroadcastChannelReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, envelopeError := Phase4EnsureFrontendSettlingEnvelopeTask.Run(
			taskContext,
			input,
		)
		if envelopeError != nil {
			return struct{}{}, envelopeError
		}
		return struct{}{}, nil
	},
)

// Phase4EnsureBrowserClientSessionReadyTask validates browser session readiness.
var Phase4EnsureBrowserClientSessionReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, browserChannelError := Phase4EnsureBrowserBroadcastChannelReadyTask.Run(
			taskContext,
			input,
		)
		if browserChannelError != nil {
			return struct{}{}, browserChannelError
		}
		return struct{}{}, nil
	},
)

// Phase4EnsureCSSHotReloadPayloadReadyTask prepares CSS payload readiness.
var Phase4EnsureCSSHotReloadPayloadReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, browserSessionError := Phase4EnsureBrowserClientSessionReadyTask.Run(
			taskContext,
			input,
		)
		if browserSessionError != nil {
			return struct{}{}, browserSessionError
		}
		return struct{}{}, nil
	},
)

// Phase4EnsureInvalidatePayloadReadyTask prepares invalidate payload readiness.
var Phase4EnsureInvalidatePayloadReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, browserSessionError := Phase4EnsureBrowserClientSessionReadyTask.Run(
			taskContext,
			input,
		)
		if browserSessionError != nil {
			return struct{}{}, browserSessionError
		}
		return struct{}{}, nil
	},
)

// Phase4EnsureRevalidatePayloadReadyTask prepares revalidate payload readiness.
var Phase4EnsureRevalidatePayloadReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, browserSessionError := Phase4EnsureBrowserClientSessionReadyTask.Run(
			taskContext,
			input,
		)
		if browserSessionError != nil {
			return struct{}{}, browserSessionError
		}
		return struct{}{}, nil
	},
)

// Phase4EnsureHardReloadPayloadReadyTask prepares hard-reload payload readiness.
var Phase4EnsureHardReloadPayloadReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, browserSessionError := Phase4EnsureBrowserClientSessionReadyTask.Run(
			taskContext,
			input,
		)
		if browserSessionError != nil {
			return struct{}{}, browserSessionError
		}
		return struct{}{}, nil
	},
)

// Phase4BroadcastCSSHotReloadTask executes CSS hot reload broadcast.
var Phase4BroadcastCSSHotReloadTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, payloadError := Phase4EnsureCSSHotReloadPayloadReadyTask.Run(
			taskContext,
			input,
		)
		if payloadError != nil {
			return struct{}{}, payloadError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDFrontendBroadcastCSSHotReload,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase4BroadcastInvalidateAssetsTask executes browser asset invalidation broadcast.
var Phase4BroadcastInvalidateAssetsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, payloadError := Phase4EnsureInvalidatePayloadReadyTask.Run(
			taskContext,
			input,
		)
		if payloadError != nil {
			return struct{}{}, payloadError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDFrontendBroadcastInvalidateAssets,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase4BroadcastRevalidateTask executes browser revalidation broadcast.
var Phase4BroadcastRevalidateTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, payloadError := Phase4EnsureRevalidatePayloadReadyTask.Run(
			taskContext,
			input,
		)
		if payloadError != nil {
			return struct{}{}, payloadError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDFrontendBroadcastRevalidate,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase4BroadcastHardReloadTask executes browser hard reload broadcast.
var Phase4BroadcastHardReloadTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, payloadError := Phase4EnsureHardReloadPayloadReadyTask.Run(
			taskContext,
			input,
		)
		if payloadError != nil {
			return struct{}{}, payloadError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDFrontendBroadcastHardReload,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase4PublishNoReloadNeededNoticeTask executes no-reload user messaging.
var Phase4PublishNoReloadNeededNoticeTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		_, browserChannelError := Phase4EnsureBrowserBroadcastChannelReadyTask.Run(
			taskContext,
			input,
		)
		if browserChannelError != nil {
			return struct{}{}, browserChannelError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDFrontendPublishNoReloadNeededNotice,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase4PlanTerminalFrontendActionTask selects final frontend action by precedence.
var Phase4PlanTerminalFrontendActionTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (Phase4TerminalAction, error) {
		_, envelopeError := Phase4EnsureFrontendSettlingEnvelopeTask.Run(
			taskContext,
			input,
		)
		if envelopeError != nil {
			return Phase4TerminalActionNone, envelopeError
		}
		return reducePhase4TerminalFrontendAction(input.FrontendGoals), nil
	},
)

func reducePhase4TerminalFrontendAction(
	frontendGoals Phase3FrontendSettlingGoals,
) Phase4TerminalAction {
	if frontendGoals.PerformHardReload {
		return Phase4TerminalActionHardReload
	}
	if frontendGoals.PerformInvalidateAssets {
		return Phase4TerminalActionInvalidateAssets
	}
	if frontendGoals.PerformRevalidate {
		return Phase4TerminalActionRevalidate
	}
	if frontendGoals.PerformCSSHotReload {
		return Phase4TerminalActionCSSHotReload
	}
	return Phase4TerminalActionNone
}

// Phase4FinalizeCompletionSummaryTask finalizes frontend-settling completion.
var Phase4FinalizeCompletionSummaryTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (Phase4CompletionSummary, error) {
		terminalAction, terminalActionError := Phase4PlanTerminalFrontendActionTask.Run(
			taskContext,
			input,
		)
		if terminalActionError != nil {
			return Phase4CompletionSummary{}, terminalActionError
		}
		return Phase4CompletionSummary{
			TerminalAction: terminalAction,
		}, nil
	},
)

// Phase4RootBroadcastCSSHotReloadTask is terminal CSS-hot-reload root.
var Phase4RootBroadcastCSSHotReloadTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		return Phase4BroadcastCSSHotReloadTask.Run(taskContext, input)
	},
)

// Phase4RootBroadcastInvalidateAssetsTask is terminal invalidate-assets root.
var Phase4RootBroadcastInvalidateAssetsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		return Phase4BroadcastInvalidateAssetsTask.Run(taskContext, input)
	},
)

// Phase4RootBroadcastRevalidateTask is terminal revalidate root.
var Phase4RootBroadcastRevalidateTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		return Phase4BroadcastRevalidateTask.Run(taskContext, input)
	},
)

// Phase4RootBroadcastHardReloadTask is terminal hard-reload root.
var Phase4RootBroadcastHardReloadTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		return Phase4BroadcastHardReloadTask.Run(taskContext, input)
	},
)

// Phase4RootPublishNoReloadNeededNoticeTask is terminal no-reload root.
var Phase4RootPublishNoReloadNeededNoticeTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (struct{}, error) {
		return Phase4PublishNoReloadNeededNoticeTask.Run(taskContext, input)
	},
)

// RunPhase4TaskGraph executes phase-4 roots by precedence and finalizes.
func RunPhase4TaskGraph(
	taskContext *tasks.Ctx,
	input Phase4BatchInput,
) (Phase4CompletionSummary, error) {
	terminalAction, terminalActionError := Phase4PlanTerminalFrontendActionTask.Run(
		taskContext,
		input,
	)
	if terminalActionError != nil {
		return Phase4CompletionSummary{}, terminalActionError
	}

	switch terminalAction {
	case Phase4TerminalActionHardReload:
		if _, hardReloadError := Phase4RootBroadcastHardReloadTask.Run(
			taskContext,
			input,
		); hardReloadError != nil {
			return Phase4CompletionSummary{}, hardReloadError
		}
	case Phase4TerminalActionInvalidateAssets:
		if _, invalidateError := Phase4RootBroadcastInvalidateAssetsTask.Run(
			taskContext,
			input,
		); invalidateError != nil {
			return Phase4CompletionSummary{}, invalidateError
		}
	case Phase4TerminalActionRevalidate:
		if _, revalidateError := Phase4RootBroadcastRevalidateTask.Run(
			taskContext,
			input,
		); revalidateError != nil {
			return Phase4CompletionSummary{}, revalidateError
		}
	case Phase4TerminalActionCSSHotReload:
		if _, cssHotReloadError := Phase4RootBroadcastCSSHotReloadTask.Run(
			taskContext,
			input,
		); cssHotReloadError != nil {
			return Phase4CompletionSummary{}, cssHotReloadError
		}
	default:
		if _, noReloadNoticeError := Phase4RootPublishNoReloadNeededNoticeTask.Run(
			taskContext,
			input,
		); noReloadNoticeError != nil {
			return Phase4CompletionSummary{}, noReloadNoticeError
		}
	}

	return Phase4FinalizeCompletionSummaryTask.Run(taskContext, input)
}

// RunFourPhaseTaskGraph executes all four phases end-to-end.
func RunFourPhaseTaskGraph(
	parentContext context.Context,
	input EventsPhaseBatchInput,
) (FourPhaseRunResult, error) {
	return NewDefaultFourPhaseRunner().Run(parentContext, input)
}
