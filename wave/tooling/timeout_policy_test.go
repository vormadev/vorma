package tooling

import (
	"context"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
)

func TestDeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(t *testing.T) {
	testCases := []struct {
		name                            string
		stageTimeoutMilliseconds        int
		executionTimeoutMilliseconds    int
		disableStageTimeout             bool
		expectedResolvedTimeoutDuration time.Duration
	}{
		{
			name:                            "uses stage timeout when no per-execution override is configured",
			stageTimeoutMilliseconds:        1200,
			executionTimeoutMilliseconds:    0,
			disableStageTimeout:             false,
			expectedResolvedTimeoutDuration: 1200 * time.Millisecond,
		},
		{
			name:                            "uses per-execution timeout when provided",
			stageTimeoutMilliseconds:        1200,
			executionTimeoutMilliseconds:    180,
			disableStageTimeout:             false,
			expectedResolvedTimeoutDuration: 180 * time.Millisecond,
		},
		{
			name:                            "uses per-execution timeout when stage timeout is unset",
			stageTimeoutMilliseconds:        0,
			executionTimeoutMilliseconds:    180,
			disableStageTimeout:             false,
			expectedResolvedTimeoutDuration: 180 * time.Millisecond,
		},
		{
			name:                            "disables timeout when stage timeout is explicitly disabled",
			stageTimeoutMilliseconds:        1200,
			executionTimeoutMilliseconds:    0,
			disableStageTimeout:             true,
			expectedResolvedTimeoutDuration: 0,
		},
		{
			name:                            "stage timeout disable wins even when per-execution override is present",
			stageTimeoutMilliseconds:        1200,
			executionTimeoutMilliseconds:    180,
			disableStageTimeout:             true,
			expectedResolvedTimeoutDuration: 0,
		},
		{
			name:                            "treats negative stage timeout as disabled",
			stageTimeoutMilliseconds:        -10,
			executionTimeoutMilliseconds:    0,
			disableStageTimeout:             false,
			expectedResolvedTimeoutDuration: 0,
		},
		{
			name:                            "ignores non-positive per-execution timeout and falls back to stage timeout",
			stageTimeoutMilliseconds:        1200,
			executionTimeoutMilliseconds:    -10,
			disableStageTimeout:             false,
			expectedResolvedTimeoutDuration: 1200 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resolvedTimeoutDuration := deriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
				testCase.stageTimeoutMilliseconds,
				testCase.executionTimeoutMilliseconds,
				testCase.disableStageTimeout,
			)
			if resolvedTimeoutDuration != testCase.expectedResolvedTimeoutDuration {
				t.Fatalf(
					"resolved timeout=%s, expected=%s",
					resolvedTimeoutDuration,
					testCase.expectedResolvedTimeoutDuration,
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
		name                                    string
		stageType                               hookStageType
		expectedStageCommandTimeoutMilliseconds int
	}{
		{
			name:                                    "pre stage timeout is mapped correctly",
			stageType:                               hookStageTypePre,
			expectedStageCommandTimeoutMilliseconds: 11,
		},
		{
			name:                                    "concurrent stage timeout is mapped correctly",
			stageType:                               hookStageTypeConcurrent,
			expectedStageCommandTimeoutMilliseconds: 22,
		},
		{
			name:                                    "concurrent-no-wait stage timeout is mapped correctly",
			stageType:                               hookStageTypeConcurrentNoWait,
			expectedStageCommandTimeoutMilliseconds: 27,
		},
		{
			name:                                    "post stage timeout is mapped correctly",
			stageType:                               hookStageTypePost,
			expectedStageCommandTimeoutMilliseconds: 33,
		},
		{
			name:                                    "unknown stage maps to no timeout",
			stageType:                               hookStageType(-1),
			expectedStageCommandTimeoutMilliseconds: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stageTimeoutMilliseconds := deriveHookCommandStageTimeoutMilliseconds(
				watchConfig,
				testCase.stageType,
			)
			if stageTimeoutMilliseconds != testCase.expectedStageCommandTimeoutMilliseconds {
				t.Fatalf(
					"stage timeout milliseconds=%d, expected=%d",
					stageTimeoutMilliseconds,
					testCase.expectedStageCommandTimeoutMilliseconds,
				)
			}
		})
	}

	if got := deriveHookCommandStageTimeoutMilliseconds(nil, hookStageTypePre); got != 0 {
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
		name                                     string
		stageType                                hookStageType
		expectedStageCallbackTimeoutMilliseconds int
	}{
		{
			name:                                     "pre stage timeout is mapped correctly",
			stageType:                                hookStageTypePre,
			expectedStageCallbackTimeoutMilliseconds: 11,
		},
		{
			name:                                     "concurrent stage timeout is mapped correctly",
			stageType:                                hookStageTypeConcurrent,
			expectedStageCallbackTimeoutMilliseconds: 22,
		},
		{
			name:                                     "concurrent-no-wait stage timeout is mapped correctly",
			stageType:                                hookStageTypeConcurrentNoWait,
			expectedStageCallbackTimeoutMilliseconds: 27,
		},
		{
			name:                                     "post stage timeout is mapped correctly",
			stageType:                                hookStageTypePost,
			expectedStageCallbackTimeoutMilliseconds: 33,
		},
		{
			name:                                     "unknown stage maps to no timeout",
			stageType:                                hookStageType(-1),
			expectedStageCallbackTimeoutMilliseconds: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stageTimeoutMilliseconds := deriveHookCallbackStageTimeoutMilliseconds(
				watchConfig,
				testCase.stageType,
			)
			if stageTimeoutMilliseconds != testCase.expectedStageCallbackTimeoutMilliseconds {
				t.Fatalf(
					"stage callback timeout milliseconds=%d, expected=%d",
					stageTimeoutMilliseconds,
					testCase.expectedStageCallbackTimeoutMilliseconds,
				)
			}
		})
	}

	if got := deriveHookCallbackStageTimeoutMilliseconds(nil, hookStageTypePre); got != 0 {
		t.Fatalf("nil watch config callback timeout milliseconds=%d, expected=0", got)
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
		name                           string
		stageType                      hookStageType
		executionPlan                  hookExecutionPlan
		expectedCommandTimeoutDuration time.Duration
	}{
		{
			name:                           "uses stage command timeout by default",
			stageType:                      hookStageTypePre,
			executionPlan:                  hookExecutionPlan{},
			expectedCommandTimeoutDuration: 1000 * time.Millisecond,
		},
		{
			name:      "uses per-hook command timeout override",
			stageType: hookStageTypePre,
			executionPlan: hookExecutionPlan{
				commandTimeoutMilliseconds: 250,
			},
			expectedCommandTimeoutDuration: 250 * time.Millisecond,
		},
		{
			name:      "disables command timeout when stage timeout is disabled",
			stageType: hookStageTypePre,
			executionPlan: hookExecutionPlan{
				disableStageCommandTimeout: true,
			},
			expectedCommandTimeoutDuration: 0,
		},
		{
			name:      "stage timeout disable wins over per-hook command timeout override",
			stageType: hookStageTypePre,
			executionPlan: hookExecutionPlan{
				commandTimeoutMilliseconds: 250,
				disableStageCommandTimeout: true,
			},
			expectedCommandTimeoutDuration: 0,
		},
		{
			name:      "unknown stage falls back to per-hook command timeout override",
			stageType: hookStageType(-1),
			executionPlan: hookExecutionPlan{
				commandTimeoutMilliseconds: 150,
			},
			expectedCommandTimeoutDuration: 150 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			commandTimeoutDuration := deriveHookCommandTimeoutDurationForExecutionPlan(
				watchConfig,
				testCase.stageType,
				testCase.executionPlan,
			)
			if commandTimeoutDuration != testCase.expectedCommandTimeoutDuration {
				t.Fatalf(
					"command timeout duration=%s, expected=%s",
					commandTimeoutDuration,
					testCase.expectedCommandTimeoutDuration,
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
		name                            string
		stageType                       hookStageType
		executionPlan                   hookExecutionPlan
		expectedCallbackTimeoutDuration time.Duration
	}{
		{
			name:                            "uses stage callback timeout by default",
			stageType:                       hookStageTypePre,
			executionPlan:                   hookExecutionPlan{},
			expectedCallbackTimeoutDuration: 1000 * time.Millisecond,
		},
		{
			name:      "uses per-hook callback timeout override",
			stageType: hookStageTypePre,
			executionPlan: hookExecutionPlan{
				callbackTimeoutMilliseconds: 250,
			},
			expectedCallbackTimeoutDuration: 250 * time.Millisecond,
		},
		{
			name:      "disables callback timeout when stage timeout is disabled",
			stageType: hookStageTypePre,
			executionPlan: hookExecutionPlan{
				disableStageCallbackTimeout: true,
			},
			expectedCallbackTimeoutDuration: 0,
		},
		{
			name:      "stage timeout disable wins over per-hook callback timeout override",
			stageType: hookStageTypePre,
			executionPlan: hookExecutionPlan{
				callbackTimeoutMilliseconds: 250,
				disableStageCallbackTimeout: true,
			},
			expectedCallbackTimeoutDuration: 0,
		},
		{
			name:      "unknown stage falls back to per-hook callback timeout override",
			stageType: hookStageType(-1),
			executionPlan: hookExecutionPlan{
				callbackTimeoutMilliseconds: 150,
			},
			expectedCallbackTimeoutDuration: 150 * time.Millisecond,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			callbackTimeoutDuration := deriveHookCallbackTimeoutDurationForExecutionPlan(
				watchConfig,
				testCase.stageType,
				testCase.executionPlan,
			)
			if callbackTimeoutDuration != testCase.expectedCallbackTimeoutDuration {
				t.Fatalf(
					"callback timeout duration=%s, expected=%s",
					callbackTimeoutDuration,
					testCase.expectedCallbackTimeoutDuration,
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

	if got := deriveBuildHookCommandTimeoutDuration(coreConfig, true); got != 111*time.Millisecond {
		t.Fatalf("dev build hook timeout duration=%s, expected=%s", got, 111*time.Millisecond)
	}
	if got := deriveBuildHookCommandTimeoutDuration(coreConfig, false); got != 222*time.Millisecond {
		t.Fatalf("prod build hook timeout duration=%s, expected=%s", got, 222*time.Millisecond)
	}
	if got := deriveBuildHookCommandTimeoutDuration(nil, true); got != 0 {
		t.Fatalf("nil core config timeout duration=%s, expected=0", got)
	}

	coreConfigWithNegativeTimeouts := &wave.CoreConfig{
		DevBuildHookTimeoutMilliseconds:  -1,
		ProdBuildHookTimeoutMilliseconds: -2,
	}
	if got := deriveBuildHookCommandTimeoutDuration(coreConfigWithNegativeTimeouts, true); got != 0 {
		t.Fatalf("negative dev timeout duration=%s, expected=0", got)
	}
	if got := deriveBuildHookCommandTimeoutDuration(coreConfigWithNegativeTimeouts, false); got != 0 {
		t.Fatalf("negative prod timeout duration=%s, expected=0", got)
	}
}

func TestDeriveExecutionContextWithOptionalTimeout(t *testing.T) {
	parentExecutionContext, cancelParentExecutionContext := context.WithCancel(
		context.Background(),
	)
	defer cancelParentExecutionContext()

	unchangedExecutionContext, cancelUnchangedExecutionContext := deriveExecutionContextWithOptionalTimeout(
		parentExecutionContext,
		0,
	)
	if cancelUnchangedExecutionContext != nil {
		t.Fatal("expected nil cancel function when timeout is disabled")
	}
	if unchangedExecutionContext != parentExecutionContext {
		t.Fatal("expected unchanged context when timeout is disabled")
	}

	backgroundExecutionContext, cancelBackgroundExecutionContext := deriveExecutionContextWithOptionalTimeout(
		context.TODO(),
		0,
	)
	if cancelBackgroundExecutionContext != nil {
		t.Fatal("expected nil cancel function when timeout is disabled with default context")
	}
	if backgroundExecutionContext == nil {
		t.Fatal("expected non-nil background context when timeout parent context is omitted")
	}

	timeoutExecutionContext, cancelTimeoutExecutionContext := deriveExecutionContextWithOptionalTimeout(
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
		t.Fatalf("expected timeout deadline in the future, got %s", timeoutDeadline)
	}
	cancelTimeoutExecutionContext()

	childExecutionContext, cancelChildExecutionContext := deriveExecutionContextWithOptionalTimeout(
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
		t.Fatal("expected child execution context cancellation when parent is canceled")
	}
	cancelChildExecutionContext()
}
