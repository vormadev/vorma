package tooling

import (
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestContinuePipelineAfterHookStageOrTriggerRestart_UsesConfiguredFailurePolicy(t *testing.T) {
	t.Run("default fail-open continues on stage errors", func(t *testing.T) {
		s := &server{
			cfg: &wave.ParsedConfig{
				Core:  &wave.CoreConfig{},
				Watch: &wave.WatchConfig{},
			},
			log:            newDiscardLogger(),
			restartIntents: newRestartIntentAccumulator(make(chan restartRequest, 1)),
		}

		shouldContinue := s.continuePipelineAfterHookStageOrTriggerRestart(
			hookStageResult{
				stageType:       hookStageTypePre,
				executionErrors: []error{errSynthetic},
			},
		)
		if !shouldContinue {
			t.Fatal("expected fail-open default policy to continue on stage errors")
		}
		assertNoPendingRestartRequestForToolingTests(t, s)
	})

	t.Run("configured fail-closed stops on stage errors", func(t *testing.T) {
		s := &server{
			cfg: &wave.ParsedConfig{
				Core: &wave.CoreConfig{},
				Watch: &wave.WatchConfig{
					HookStageFailurePolicy: configuredHookStageFailurePolicyFailClosed,
				},
			},
			log:            newDiscardLogger(),
			restartIntents: newRestartIntentAccumulator(make(chan restartRequest, 1)),
		}

		shouldContinue := s.continuePipelineAfterHookStageOrTriggerRestart(
			hookStageResult{
				stageType:       hookStageTypeConcurrent,
				executionErrors: []error{errSynthetic},
			},
		)
		if shouldContinue {
			t.Fatal("expected configured fail-closed policy to stop on stage errors")
		}
		assertNoPendingRestartRequestForToolingTests(t, s)
	})
}
