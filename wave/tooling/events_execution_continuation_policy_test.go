package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/devserver/devserverengine"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestContinuePipelineAfterHookStageOrTriggerRestart_UsesConfiguredFailurePolicy(t *testing.T) {
	t.Run("default fail-open continues on stage errors", func(t *testing.T) {
		s := &devserver.Server{
			Cfg: &wave.ParsedConfig{
				Core:  &wave.CoreConfig{},
				Watch: &wave.WatchConfig{},
			},
			Log:            newDiscardLogger(),
			RestartIntents: devserverengine.NewRestartIntentAccumulator(make(chan devserverengine.RestartRequest, 1)),
		}

		shouldContinue := s.ContinuePipelineAfterHookStageOrTriggerRestart(
			devserver.HookStageResult{
				StageType:       devserver.HookStageTypePre,
				ExecutionErrors: []error{errSynthetic},
			},
		)
		if !shouldContinue {
			t.Fatal("expected fail-open default policy to continue on stage errors")
		}
		assertNoPendingRestartRequestForToolingTests(t, s)
	})

	t.Run("configured fail-closed stops on stage errors", func(t *testing.T) {
		s := &devserver.Server{
			Cfg: &wave.ParsedConfig{
				Core: &wave.CoreConfig{},
				Watch: &wave.WatchConfig{
					HookStageFailurePolicy: devserver.ConfiguredHookStageFailurePolicyFailClosed,
				},
			},
			Log:            newDiscardLogger(),
			RestartIntents: devserverengine.NewRestartIntentAccumulator(make(chan devserverengine.RestartRequest, 1)),
		}

		shouldContinue := s.ContinuePipelineAfterHookStageOrTriggerRestart(
			devserver.HookStageResult{
				StageType:       devserver.HookStageTypeConcurrent,
				ExecutionErrors: []error{errSynthetic},
			},
		)
		if shouldContinue {
			t.Fatal("expected configured fail-closed policy to stop on stage errors")
		}
		assertNoPendingRestartRequestForToolingTests(t, s)
	})
}
