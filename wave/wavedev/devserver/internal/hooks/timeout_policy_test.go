package hooks_test

import (
	"context"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/hooks"
)

func TestDeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
	t *testing.T,
) {
	testCases := []struct {
		Name                            string
		StageTimeoutMilliseconds        int
		ExecutionTimeoutMilliseconds    int
		DisableStageTimeout             bool
		ExpectedResolvedTimeoutDuration time.Duration
	}{
		{
			Name:                            "uses stage timeout when no per-execution override is configured",
			StageTimeoutMilliseconds:        1200,
			ExecutionTimeoutMilliseconds:    0,
			DisableStageTimeout:             false,
			ExpectedResolvedTimeoutDuration: 1200 * time.Millisecond,
		},
		{
			Name:                            "uses per-execution timeout when provided",
			StageTimeoutMilliseconds:        1200,
			ExecutionTimeoutMilliseconds:    180,
			DisableStageTimeout:             false,
			ExpectedResolvedTimeoutDuration: 180 * time.Millisecond,
		},
		{
			Name:                            "uses per-execution timeout when stage timeout is unset",
			StageTimeoutMilliseconds:        0,
			ExecutionTimeoutMilliseconds:    180,
			DisableStageTimeout:             false,
			ExpectedResolvedTimeoutDuration: 180 * time.Millisecond,
		},
		{
			Name:                            "disables timeout when stage timeout is explicitly disabled",
			StageTimeoutMilliseconds:        1200,
			ExecutionTimeoutMilliseconds:    0,
			DisableStageTimeout:             true,
			ExpectedResolvedTimeoutDuration: 0,
		},
		{
			Name:                            "stage timeout disable wins even when per-execution override is present",
			StageTimeoutMilliseconds:        1200,
			ExecutionTimeoutMilliseconds:    180,
			DisableStageTimeout:             true,
			ExpectedResolvedTimeoutDuration: 0,
		},
		{
			Name:                            "treats negative stage timeout as disabled",
			StageTimeoutMilliseconds:        -10,
			ExecutionTimeoutMilliseconds:    0,
			DisableStageTimeout:             false,
			ExpectedResolvedTimeoutDuration: 0,
		},
		{
			Name:                            "ignores non-positive per-execution timeout and falls back to stage timeout",
			StageTimeoutMilliseconds:        1200,
			ExecutionTimeoutMilliseconds:    -10,
			DisableStageTimeout:             false,
			ExpectedResolvedTimeoutDuration: 1200 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			resolvedTimeoutDuration := hooks.DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
				testCase.StageTimeoutMilliseconds,
				testCase.ExecutionTimeoutMilliseconds,
				testCase.DisableStageTimeout,
			)
			if resolvedTimeoutDuration != testCase.ExpectedResolvedTimeoutDuration {
				t.Fatalf(
					"resolved timeout=%s, expected=%s",
					resolvedTimeoutDuration,
					testCase.ExpectedResolvedTimeoutDuration,
				)
			}
		})
	}
}

func TestDeriveHookCommandStageTimeoutMilliseconds(t *testing.T) {
	watchConfig := &wave.WatchConfig{
		HookCommandTimeouts: wave.HookCommandTimeoutConfig{
			PreCommandTimeoutMilliseconds:              11,
			ConcurrentCommandTimeoutMilliseconds:       22,
			ConcurrentNoWaitCommandTimeoutMilliseconds: 27,
			PostCommandTimeoutMilliseconds:             33,
		},
	}

	testCases := []struct {
		Name                                    string
		StageType                               hooks.HookStageType
		ExpectedStageCommandTimeoutMilliseconds int
	}{
		{
			Name:                                    "pre stage timeout is mapped correctly",
			StageType:                               hooks.HookStageTypePre,
			ExpectedStageCommandTimeoutMilliseconds: 11,
		},
		{
			Name:                                    "concurrent stage timeout is mapped correctly",
			StageType:                               hooks.HookStageTypeConcurrent,
			ExpectedStageCommandTimeoutMilliseconds: 22,
		},
		{
			Name:                                    "concurrent-no-wait stage timeout is mapped correctly",
			StageType:                               hooks.HookStageTypeConcurrentNoWait,
			ExpectedStageCommandTimeoutMilliseconds: 27,
		},
		{
			Name:                                    "post stage timeout is mapped correctly",
			StageType:                               hooks.HookStageTypePost,
			ExpectedStageCommandTimeoutMilliseconds: 33,
		},
		{
			Name:                                    "unknown stage maps to no timeout",
			StageType:                               hooks.HookStageType(-1),
			ExpectedStageCommandTimeoutMilliseconds: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			stageTimeoutMilliseconds := hooks.DeriveHookCommandStageTimeoutMilliseconds(
				watchConfig,
				testCase.StageType,
			)
			if stageTimeoutMilliseconds != testCase.ExpectedStageCommandTimeoutMilliseconds {
				t.Fatalf(
					"stage timeout milliseconds=%d, expected=%d",
					stageTimeoutMilliseconds,
					testCase.ExpectedStageCommandTimeoutMilliseconds,
				)
			}
		})
	}

	if got := hooks.DeriveHookCommandStageTimeoutMilliseconds(nil, hooks.HookStageTypePre); got != 0 {
		t.Fatalf("nil watch config timeout milliseconds=%d, expected=0", got)
	}
}

func TestDeriveHookCallbackStageTimeoutMilliseconds(t *testing.T) {
	watchConfig := &wave.WatchConfig{
		HookCallbackTimeouts: wave.HookCallbackTimeoutConfig{
			PreCallbackTimeoutMilliseconds:              11,
			ConcurrentCallbackTimeoutMilliseconds:       22,
			ConcurrentNoWaitCallbackTimeoutMilliseconds: 27,
			PostCallbackTimeoutMilliseconds:             33,
		},
	}

	testCases := []struct {
		Name                                     string
		StageType                                hooks.HookStageType
		ExpectedStageCallbackTimeoutMilliseconds int
	}{
		{
			Name:                                     "pre stage timeout is mapped correctly",
			StageType:                                hooks.HookStageTypePre,
			ExpectedStageCallbackTimeoutMilliseconds: 11,
		},
		{
			Name:                                     "concurrent stage timeout is mapped correctly",
			StageType:                                hooks.HookStageTypeConcurrent,
			ExpectedStageCallbackTimeoutMilliseconds: 22,
		},
		{
			Name:                                     "concurrent-no-wait stage timeout is mapped correctly",
			StageType:                                hooks.HookStageTypeConcurrentNoWait,
			ExpectedStageCallbackTimeoutMilliseconds: 27,
		},
		{
			Name:                                     "post stage timeout is mapped correctly",
			StageType:                                hooks.HookStageTypePost,
			ExpectedStageCallbackTimeoutMilliseconds: 33,
		},
		{
			Name:                                     "unknown stage maps to no timeout",
			StageType:                                hooks.HookStageType(-1),
			ExpectedStageCallbackTimeoutMilliseconds: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			stageTimeoutMilliseconds := hooks.DeriveHookCallbackStageTimeoutMilliseconds(
				watchConfig,
				testCase.StageType,
			)
			if stageTimeoutMilliseconds != testCase.ExpectedStageCallbackTimeoutMilliseconds {
				t.Fatalf(
					"stage callback timeout milliseconds=%d, expected=%d",
					stageTimeoutMilliseconds,
					testCase.ExpectedStageCallbackTimeoutMilliseconds,
				)
			}
		})
	}

	if got := hooks.DeriveHookCallbackStageTimeoutMilliseconds(nil, hooks.HookStageTypePre); got != 0 {
		t.Fatalf(
			"nil watch config callback timeout milliseconds=%d, expected=0",
			got,
		)
	}
}

func TestDeriveHookCommandTimeoutDurationForExecutionPlan(t *testing.T) {
	watchConfig := &wave.WatchConfig{
		HookCommandTimeouts: wave.HookCommandTimeoutConfig{
			PreCommandTimeoutMilliseconds:        1000,
			ConcurrentCommandTimeoutMilliseconds: 2000,
		},
	}

	testCases := []struct {
		Name                           string
		StageType                      hooks.HookStageType
		ExecutionPlan                  hooks.HookExecutionPlan
		ExpectedCommandTimeoutDuration time.Duration
	}{
		{
			Name:                           "uses stage command timeout by default",
			StageType:                      hooks.HookStageTypePre,
			ExecutionPlan:                  hooks.HookExecutionPlan{},
			ExpectedCommandTimeoutDuration: 1000 * time.Millisecond,
		},
		{
			Name:      "uses per-hook command timeout override",
			StageType: hooks.HookStageTypePre,
			ExecutionPlan: hooks.HookExecutionPlan{
				CommandTimeoutMilliseconds: 250,
			},
			ExpectedCommandTimeoutDuration: 250 * time.Millisecond,
		},
		{
			Name:      "disables command timeout when stage timeout is disabled",
			StageType: hooks.HookStageTypePre,
			ExecutionPlan: hooks.HookExecutionPlan{
				DisableStageCommandTimeout: true,
			},
			ExpectedCommandTimeoutDuration: 0,
		},
		{
			Name:      "stage timeout disable wins over per-hook command timeout override",
			StageType: hooks.HookStageTypePre,
			ExecutionPlan: hooks.HookExecutionPlan{
				CommandTimeoutMilliseconds: 250,
				DisableStageCommandTimeout: true,
			},
			ExpectedCommandTimeoutDuration: 0,
		},
		{
			Name:      "unknown stage falls back to per-hook command timeout override",
			StageType: hooks.HookStageType(-1),
			ExecutionPlan: hooks.HookExecutionPlan{
				CommandTimeoutMilliseconds: 150,
			},
			ExpectedCommandTimeoutDuration: 150 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			commandTimeoutDuration := hooks.DeriveHookCommandTimeoutDurationForExecutionPlan(
				watchConfig,
				testCase.StageType,
				testCase.ExecutionPlan,
			)
			if commandTimeoutDuration != testCase.ExpectedCommandTimeoutDuration {
				t.Fatalf(
					"command timeout duration=%s, expected=%s",
					commandTimeoutDuration,
					testCase.ExpectedCommandTimeoutDuration,
				)
			}
		})
	}
}

func TestDeriveHookCallbackTimeoutDurationForExecutionPlan(t *testing.T) {
	watchConfig := &wave.WatchConfig{
		HookCallbackTimeouts: wave.HookCallbackTimeoutConfig{
			PreCallbackTimeoutMilliseconds:        1000,
			ConcurrentCallbackTimeoutMilliseconds: 2000,
		},
	}

	testCases := []struct {
		Name                            string
		StageType                       hooks.HookStageType
		ExecutionPlan                   hooks.HookExecutionPlan
		ExpectedCallbackTimeoutDuration time.Duration
	}{
		{
			Name:                            "uses stage callback timeout by default",
			StageType:                       hooks.HookStageTypePre,
			ExecutionPlan:                   hooks.HookExecutionPlan{},
			ExpectedCallbackTimeoutDuration: 1000 * time.Millisecond,
		},
		{
			Name:      "uses per-hook callback timeout override",
			StageType: hooks.HookStageTypePre,
			ExecutionPlan: hooks.HookExecutionPlan{
				CallbackTimeoutMilliseconds: 250,
			},
			ExpectedCallbackTimeoutDuration: 250 * time.Millisecond,
		},
		{
			Name:      "disables callback timeout when stage timeout is disabled",
			StageType: hooks.HookStageTypePre,
			ExecutionPlan: hooks.HookExecutionPlan{
				DisableStageCallbackTimeout: true,
			},
			ExpectedCallbackTimeoutDuration: 0,
		},
		{
			Name:      "stage timeout disable wins over per-hook callback timeout override",
			StageType: hooks.HookStageTypePre,
			ExecutionPlan: hooks.HookExecutionPlan{
				CallbackTimeoutMilliseconds: 250,
				DisableStageCallbackTimeout: true,
			},
			ExpectedCallbackTimeoutDuration: 0,
		},
		{
			Name:      "unknown stage falls back to per-hook callback timeout override",
			StageType: hooks.HookStageType(-1),
			ExecutionPlan: hooks.HookExecutionPlan{
				CallbackTimeoutMilliseconds: 150,
			},
			ExpectedCallbackTimeoutDuration: 150 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			callbackTimeoutDuration := hooks.DeriveHookCallbackTimeoutDurationForExecutionPlan(
				watchConfig,
				testCase.StageType,
				testCase.ExecutionPlan,
			)
			if callbackTimeoutDuration != testCase.ExpectedCallbackTimeoutDuration {
				t.Fatalf(
					"callback timeout duration=%s, expected=%s",
					callbackTimeoutDuration,
					testCase.ExpectedCallbackTimeoutDuration,
				)
			}
		})
	}
}

func TestDeriveBuildHookCommandTimeoutDuration(t *testing.T) {
	coreConfig := &wave.CoreConfig{
		DevBuildHookTimeoutMilliseconds:  111,
		ProdBuildHookTimeoutMilliseconds: 222,
	}

	if got := hooks.DeriveBuildHookCommandTimeoutDuration(coreConfig, true); got != 111*time.Millisecond {
		t.Fatalf(
			"dev build hook timeout duration=%s, expected=%s",
			got,
			111*time.Millisecond,
		)
	}
	if got := hooks.DeriveBuildHookCommandTimeoutDuration(coreConfig, false); got != 222*time.Millisecond {
		t.Fatalf(
			"prod build hook timeout duration=%s, expected=%s",
			got,
			222*time.Millisecond,
		)
	}
	if got := hooks.DeriveBuildHookCommandTimeoutDuration(nil, true); got != 0 {
		t.Fatalf("nil core config timeout duration=%s, expected=0", got)
	}

	coreConfigWithNegativeTimeouts := &wave.CoreConfig{
		DevBuildHookTimeoutMilliseconds:  -1,
		ProdBuildHookTimeoutMilliseconds: -2,
	}
	if got := hooks.DeriveBuildHookCommandTimeoutDuration(coreConfigWithNegativeTimeouts, true); got != 0 {
		t.Fatalf("negative dev timeout duration=%s, expected=0", got)
	}
	if got := hooks.DeriveBuildHookCommandTimeoutDuration(coreConfigWithNegativeTimeouts, false); got != 0 {
		t.Fatalf("negative prod timeout duration=%s, expected=0", got)
	}
}

func TestDeriveExecutionContextWithOptionalTimeout(t *testing.T) {
	parentExecutionContext, cancelParentExecutionContext := context.WithCancel(
		context.Background(),
	)
	defer cancelParentExecutionContext()

	unchangedExecutionContext, cancelUnchangedExecutionContext := hooks.DeriveExecutionContextWithOptionalTimeout(
		parentExecutionContext,
		0,
	)
	if cancelUnchangedExecutionContext != nil {
		t.Fatal("expected nil cancel function when timeout is disabled")
	}
	if unchangedExecutionContext != parentExecutionContext {
		t.Fatal("expected unchanged context when timeout is disabled")
	}

	backgroundExecutionContext, cancelBackgroundExecutionContext := hooks.DeriveExecutionContextWithOptionalTimeout(
		context.TODO(),
		0,
	)
	if cancelBackgroundExecutionContext != nil {
		t.Fatal(
			"expected nil cancel function when timeout is disabled with default context",
		)
	}
	if backgroundExecutionContext == nil {
		t.Fatal(
			"expected non-nil background context when timeout parent context is omitted",
		)
	}

	timeoutExecutionContext, cancelTimeoutExecutionContext := hooks.DeriveExecutionContextWithOptionalTimeout(
		context.TODO(),
		100*time.Millisecond,
	)
	if timeoutExecutionContext == nil {
		t.Fatal("expected timeout execution context when timeout is enabled")
	}
	if cancelTimeoutExecutionContext == nil {
		t.Fatal("expected cancel function when timeout is enabled")
	}
	timeoutDeadline, timeoutDeadlineExists := timeoutExecutionContext.Deadline()
	if !timeoutDeadlineExists {
		t.Fatal("expected timeout execution context deadline")
	}
	if time.Until(timeoutDeadline) <= 0 {
		t.Fatalf(
			"expected timeout deadline in the future, got %s",
			timeoutDeadline,
		)
	}
	cancelTimeoutExecutionContext()

	childExecutionContext, cancelChildExecutionContext := hooks.DeriveExecutionContextWithOptionalTimeout(
		parentExecutionContext,
		1*time.Second,
	)
	if cancelChildExecutionContext == nil {
		t.Fatal("expected child cancel function when timeout is enabled")
	}
	cancelParentExecutionContext()

	select {
	case <-childExecutionContext.Done():
	case <-time.After(250 * time.Millisecond):
		t.Fatal(
			"expected child execution context cancellation when parent is canceled",
		)
	}
	cancelChildExecutionContext()
}
