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

// Phase4BroadcastInvalidateAssetsTask executes browser asset invalidation broadcast.
var Phase4BroadcastInvalidateAssetsTask = newPhase4EffectTask(
	PhaseEffectIDFrontendBroadcastInvalidateAssets,
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

// Phase4PlanTerminalFrontendActionTask selects final frontend action by precedence.
var Phase4PlanTerminalFrontendActionTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase4BatchInput,
	) (Phase4TerminalAction, error) {
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

// RunPhase4TaskGraph executes phase 4 and returns completion summary.
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
		if _, hardReloadError := Phase4BroadcastHardReloadTask.Run(
			taskContext,
			input,
		); hardReloadError != nil {
			return Phase4CompletionSummary{}, hardReloadError
		}
	case Phase4TerminalActionInvalidateAssets:
		if _, invalidateError := Phase4BroadcastInvalidateAssetsTask.Run(
			taskContext,
			input,
		); invalidateError != nil {
			return Phase4CompletionSummary{}, invalidateError
		}
	case Phase4TerminalActionRevalidate:
		if _, revalidateError := Phase4BroadcastRevalidateTask.Run(
			taskContext,
			input,
		); revalidateError != nil {
			return Phase4CompletionSummary{}, revalidateError
		}
	case Phase4TerminalActionCSSHotReload:
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

// RunFourPhaseTaskGraph executes all four phases end-to-end.
func RunFourPhaseTaskGraph(
	parentContext context.Context,
	input EventsPhaseBatchInput,
) (FourPhaseRunResult, error) {
	return NewDefaultFourPhaseRunner().Run(parentContext, input)
}
