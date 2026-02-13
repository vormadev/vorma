package tooling

import (
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestDeriveImplicitBuildExecutionDecision(t *testing.T) {
	testCases := []struct {
		name                     string
		shouldRunImplicitBuild   bool
		eventCount               int
		expectedShouldRunBuild   bool
		expectedSkipBuildLogLine string
	}{
		{
			name:                   "implicit build enabled keeps build phase",
			shouldRunImplicitBuild: true,
			eventCount:             1,
			expectedShouldRunBuild: true,
		},
		{
			name:                     "single run-on-change-only event logs single-event skip message",
			shouldRunImplicitBuild:   false,
			eventCount:               1,
			expectedShouldRunBuild:   false,
			expectedSkipBuildLogLine: "RunOnChangeOnly: skipping implicit build phase",
		},
		{
			name:                     "batch run-on-change-only events log batch skip message",
			shouldRunImplicitBuild:   false,
			eventCount:               3,
			expectedShouldRunBuild:   false,
			expectedSkipBuildLogLine: "All events are RunOnChangeOnly, skipping implicit build phase",
		},
		{
			name:                     "empty event batch uses batch skip message",
			shouldRunImplicitBuild:   false,
			eventCount:               0,
			expectedShouldRunBuild:   false,
			expectedSkipBuildLogLine: "All events are RunOnChangeOnly, skipping implicit build phase",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			implicitBuildDecision := deriveImplicitBuildExecutionDecision(
				testCase.shouldRunImplicitBuild,
				testCase.eventCount,
			)
			if implicitBuildDecision.shouldRunImplicitBuild != testCase.expectedShouldRunBuild {
				t.Fatalf(
					"deriveImplicitBuildExecutionDecision().shouldRunImplicitBuild=%t, want %t",
					implicitBuildDecision.shouldRunImplicitBuild,
					testCase.expectedShouldRunBuild,
				)
			}
			if implicitBuildDecision.skipImplicitBuildLogEntry != testCase.expectedSkipBuildLogLine {
				t.Fatalf(
					"deriveImplicitBuildExecutionDecision().skipImplicitBuildLogEntry=%q, want %q",
					implicitBuildDecision.skipImplicitBuildLogEntry,
					testCase.expectedSkipBuildLogLine,
				)
			}
		})
	}
}

func TestShouldShortCircuitPipelineForRefreshActions(t *testing.T) {
	testCases := []struct {
		name                   string
		actionResult           refreshActionApplicationResult
		expectedShouldContinue bool
	}{
		{
			name:                   "restart requested short-circuits pipeline",
			actionResult:           refreshActionApplicationResult{restartRequested: true},
			expectedShouldContinue: false,
		},
		{
			name:                   "no restart continues pipeline",
			actionResult:           refreshActionApplicationResult{restartRequested: false},
			expectedShouldContinue: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shouldShortCircuit := shouldShortCircuitPipelineForRefreshActions(
				testCase.actionResult,
			)
			shouldContinue := !shouldShortCircuit
			if shouldContinue != testCase.expectedShouldContinue {
				t.Fatalf(
					"shouldShortCircuitPipelineForRefreshActions() produced continue=%t, want %t",
					shouldContinue,
					testCase.expectedShouldContinue,
				)
			}
		})
	}
}

func TestShouldStartAppAfterImplicitBuild(t *testing.T) {
	testCases := []struct {
		name                   string
		shouldRunImplicitBuild bool
		restart                restartPhaseDecision
		expectedStartApp       bool
	}{
		{
			name:                   "build-enabled and restart-app starts app",
			shouldRunImplicitBuild: true,
			restart:                restartPhaseDecision{restartApp: true},
			expectedStartApp:       true,
		},
		{
			name:                   "build-disabled never starts app",
			shouldRunImplicitBuild: false,
			restart:                restartPhaseDecision{restartApp: true},
			expectedStartApp:       false,
		},
		{
			name:                   "restart-app false does not start app",
			shouldRunImplicitBuild: true,
			restart:                restartPhaseDecision{restartApp: false},
			expectedStartApp:       false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shouldStartApp := shouldStartAppAfterImplicitBuild(
				testCase.shouldRunImplicitBuild,
				testCase.restart,
			)
			if shouldStartApp != testCase.expectedStartApp {
				t.Fatalf(
					"shouldStartAppAfterImplicitBuild()=%t, want %t",
					shouldStartApp,
					testCase.expectedStartApp,
				)
			}
		})
	}
}

func TestShouldExecuteBrowserPhaseAfterHookActionResults(t *testing.T) {
	testCases := []struct {
		name                     string
		preActionResult          refreshActionApplicationResult
		concurrentActionResult   refreshActionApplicationResult
		postActionResult         refreshActionApplicationResult
		expectedShouldRunBrowser bool
	}{
		{
			name:                     "no restart requests executes browser phase",
			preActionResult:          refreshActionApplicationResult{},
			concurrentActionResult:   refreshActionApplicationResult{},
			postActionResult:         refreshActionApplicationResult{},
			expectedShouldRunBrowser: true,
		},
		{
			name:                     "pre-stage restart skips browser phase",
			preActionResult:          refreshActionApplicationResult{restartRequested: true},
			concurrentActionResult:   refreshActionApplicationResult{},
			postActionResult:         refreshActionApplicationResult{},
			expectedShouldRunBrowser: false,
		},
		{
			name:                     "concurrent-stage restart skips browser phase",
			preActionResult:          refreshActionApplicationResult{},
			concurrentActionResult:   refreshActionApplicationResult{restartRequested: true},
			postActionResult:         refreshActionApplicationResult{},
			expectedShouldRunBrowser: false,
		},
		{
			name:                     "post-stage restart skips browser phase",
			preActionResult:          refreshActionApplicationResult{},
			concurrentActionResult:   refreshActionApplicationResult{},
			postActionResult:         refreshActionApplicationResult{restartRequested: true},
			expectedShouldRunBrowser: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shouldRunBrowserPhase := shouldExecuteBrowserPhaseAfterHookActionResults(
				testCase.preActionResult,
				testCase.concurrentActionResult,
				testCase.postActionResult,
			)
			if shouldRunBrowserPhase != testCase.expectedShouldRunBrowser {
				t.Fatalf(
					"shouldExecuteBrowserPhaseAfterHookActionResults()=%t, want %t",
					shouldRunBrowserPhase,
					testCase.expectedShouldRunBrowser,
				)
			}
		})
	}
}

func TestDeriveEventsWithHooksForExecution(t *testing.T) {
	t.Run("batch hard reload uses copied hook contexts", func(t *testing.T) {
		firstHookContext := &wave.HookContext{FilePath: "first.txt"}
		secondHookContext := &wave.HookContext{FilePath: "second.txt"}
		eventsWithHooks := []eventWithHooks{
			{hookCtx: firstHookContext},
			{hookCtx: nil},
			{hookCtx: secondHookContext},
		}

		executionEventsWithHooks := deriveEventsWithHooksForExecution(
			eventsWithHooks,
			appStopStrategyBatchHardReload,
		)

		if len(executionEventsWithHooks) != len(eventsWithHooks) {
			t.Fatalf("execution event count=%d, want %d", len(executionEventsWithHooks), len(eventsWithHooks))
		}
		if &executionEventsWithHooks[0] == &eventsWithHooks[0] {
			t.Fatal("expected batch hard-reload execution events to use a copied event slice")
		}

		if executionEventsWithHooks[0].hookCtx == nil {
			t.Fatal("expected first execution hook context to be non-nil")
		}
		if executionEventsWithHooks[0].hookCtx == firstHookContext {
			t.Fatal("expected first execution hook context to be copied")
		}
		if !executionEventsWithHooks[0].hookCtx.AppStoppedForBatch {
			t.Fatal("expected first execution hook context to set AppStoppedForBatch=true")
		}
		if firstHookContext.AppStoppedForBatch {
			t.Fatal("expected original first hook context to remain unmodified")
		}

		if executionEventsWithHooks[1].hookCtx != nil {
			t.Fatalf("expected nil hook context to remain nil, got %#v", executionEventsWithHooks[1].hookCtx)
		}

		if executionEventsWithHooks[2].hookCtx == nil {
			t.Fatal("expected second execution hook context to be non-nil")
		}
		if executionEventsWithHooks[2].hookCtx == secondHookContext {
			t.Fatal("expected second execution hook context to be copied")
		}
		if !executionEventsWithHooks[2].hookCtx.AppStoppedForBatch {
			t.Fatal("expected second execution hook context to set AppStoppedForBatch=true")
		}
		if secondHookContext.AppStoppedForBatch {
			t.Fatal("expected original second hook context to remain unmodified")
		}
	})

	t.Run("non-batch execution reuses original events slice", func(t *testing.T) {
		eventsWithHooks := []eventWithHooks{
			{hookCtx: &wave.HookContext{}},
		}

		executionEventsWithHooks := deriveEventsWithHooksForExecution(
			eventsWithHooks,
			appStopStrategyNone,
		)

		if len(executionEventsWithHooks) != len(eventsWithHooks) {
			t.Fatalf("execution event count=%d, want %d", len(executionEventsWithHooks), len(eventsWithHooks))
		}
		if &executionEventsWithHooks[0] != &eventsWithHooks[0] {
			t.Fatal("expected non-batch execution to reuse original events slice")
		}
	})

	t.Run("empty execution events remain nil", func(t *testing.T) {
		executionEventsWithHooks := deriveEventsWithHooksForExecution(
			nil,
			appStopStrategyBatchHardReload,
		)
		if executionEventsWithHooks != nil {
			t.Fatalf("expected nil execution events for empty input, got %#v", executionEventsWithHooks)
		}
	})
}
