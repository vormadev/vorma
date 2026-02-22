package hooks_test

import (
	"errors"
	"testing"

	"github.com/vormadev/vorma/wave/tooling/devserver/internal/hooks"
)

func TestContinuePipelineAfterHookStageOrTriggerRestart_UsesConfiguredFailurePolicy(
	t *testing.T,
) {
	syntheticError := errors.New("synthetic stage error")

	t.Run("default fail-open continues on stage errors", func(t *testing.T) {
		configuredFailurePolicy := hooks.DeriveHookStageFailurePolicy(
			hooks.HookStageTypePre,
			"",
		)
		continuationDecision := hooks.DeriveHookStageContinuationDecisionWithFailurePolicy(
			hooks.HookStageResult{
				StageType:       hooks.HookStageTypePre,
				ExecutionErrors: []error{syntheticError},
			},
			configuredFailurePolicy,
		)
		if !continuationDecision.ShouldContinue {
			t.Fatalf(
				"expected default fail-open policy to continue, got %#v",
				continuationDecision,
			)
		}
	})

	t.Run("configured fail-closed stops on stage errors", func(t *testing.T) {
		configuredFailurePolicy := hooks.DeriveHookStageFailurePolicy(
			hooks.HookStageTypeConcurrent,
			"stop",
		)
		continuationDecision := hooks.DeriveHookStageContinuationDecisionWithFailurePolicy(
			hooks.HookStageResult{
				StageType:       hooks.HookStageTypeConcurrent,
				ExecutionErrors: []error{syntheticError},
			},
			configuredFailurePolicy,
		)
		if continuationDecision.ShouldContinue {
			t.Fatalf(
				"expected configured fail-closed policy to stop, got %#v",
				continuationDecision,
			)
		}
		if continuationDecision.StopReason != hooks.HookStageContinuationStopReasonStageFailure {
			t.Fatalf(
				"expected stop reason stage failure, got %#v",
				continuationDecision,
			)
		}
	})
}
