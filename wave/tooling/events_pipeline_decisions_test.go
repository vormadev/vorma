package tooling

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestDeriveStaticFileProcessingExecutionDecision(t *testing.T) {
	t.Run("processing disabled returns no-op decision", func(t *testing.T) {
		decision := deriveStaticFileProcessingExecutionDecision(
			false,
			[]string{"public/logo.png"},
		)
		if decision.mode != staticFileProcessingExecutionModeNone {
			t.Fatalf("mode=%v, want %v", decision.mode, staticFileProcessingExecutionModeNone)
		}
		if decision.shouldProcess() {
			t.Fatalf("expected shouldProcess=false, got %#v", decision)
		}
		if decision.changedFilePaths != nil {
			t.Fatalf("expected changedFilePaths=nil, got %#v", decision.changedFilePaths)
		}
	})

	t.Run("processing enabled with empty changed paths selects full scan", func(t *testing.T) {
		decision := deriveStaticFileProcessingExecutionDecision(true, nil)
		if decision.mode != staticFileProcessingExecutionModeFullScan {
			t.Fatalf("mode=%v, want %v", decision.mode, staticFileProcessingExecutionModeFullScan)
		}
		if !decision.shouldProcess() {
			t.Fatalf("expected shouldProcess=true, got %#v", decision)
		}
		if decision.changedFilePaths != nil {
			t.Fatalf("expected changedFilePaths=nil for full scan, got %#v", decision.changedFilePaths)
		}
	})

	t.Run("processing enabled with changed paths selects changed-path mode and copies slice", func(t *testing.T) {
		changedPaths := []string{"public/logo.png", "public/app.js"}
		decision := deriveStaticFileProcessingExecutionDecision(true, changedPaths)
		if decision.mode != staticFileProcessingExecutionModeChangedPaths {
			t.Fatalf("mode=%v, want %v", decision.mode, staticFileProcessingExecutionModeChangedPaths)
		}
		expectedChangedPaths := []string{"public/logo.png", "public/app.js"}
		if !reflect.DeepEqual(decision.changedFilePaths, expectedChangedPaths) {
			t.Fatalf("changedFilePaths=%#v, want %#v", decision.changedFilePaths, expectedChangedPaths)
		}

		changedPaths[0] = "mutated/path.css"
		if decision.changedFilePaths[0] != "public/logo.png" {
			t.Fatalf(
				"expected decision changedFilePaths to be isolated copy, got %#v",
				decision.changedFilePaths,
			)
		}
	})
}

func TestShouldWriteFrameworkPublicFileMapTSForBuildDecision(t *testing.T) {
	testCases := []struct {
		name                           string
		shouldProcessPublicStaticFiles bool
		frameworkPublicFileMapOutDir   string
		expectedShouldWrite            bool
	}{
		{
			name:                           "public static disabled never writes framework map",
			shouldProcessPublicStaticFiles: false,
			frameworkPublicFileMapOutDir:   "framework",
			expectedShouldWrite:            false,
		},
		{
			name:                           "empty outdir disables framework map write",
			shouldProcessPublicStaticFiles: true,
			frameworkPublicFileMapOutDir:   "",
			expectedShouldWrite:            false,
		},
		{
			name:                           "public static enabled with outdir writes framework map",
			shouldProcessPublicStaticFiles: true,
			frameworkPublicFileMapOutDir:   "framework",
			expectedShouldWrite:            true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shouldWrite := shouldWriteFrameworkPublicFileMapTSForBuildDecision(
				testCase.shouldProcessPublicStaticFiles,
				testCase.frameworkPublicFileMapOutDir,
			)
			if shouldWrite != testCase.expectedShouldWrite {
				t.Fatalf("shouldWrite=%t, want %t", shouldWrite, testCase.expectedShouldWrite)
			}
		})
	}
}

func TestDeriveBuildPhaseExecutionDecision(t *testing.T) {
	publicChangedPaths := []string{"public/logo.png"}
	privateChangedPaths := []string{"private/templates/home.html"}
	decision := deriveBuildPhaseExecutionDecision(
		buildPhaseDecision{
			compileGo:                     true,
			buildCriticalCSS:              true,
			buildNormalCSS:                false,
			processPublicFiles:            true,
			processPrivateFiles:           true,
			publicStaticChangedFilePaths:  publicChangedPaths,
			privateStaticChangedFilePaths: privateChangedPaths,
		},
		"framework-out",
	)

	if !decision.compileGo {
		t.Fatal("expected compileGo=true")
	}
	if !decision.buildCriticalCSS {
		t.Fatal("expected buildCriticalCSS=true")
	}
	if decision.buildNormalCSS {
		t.Fatal("expected buildNormalCSS=false")
	}
	if decision.publicStaticProcessing.mode != staticFileProcessingExecutionModeChangedPaths {
		t.Fatalf(
			"public mode=%v, want %v",
			decision.publicStaticProcessing.mode,
			staticFileProcessingExecutionModeChangedPaths,
		)
	}
	if decision.privateStaticProcessing.mode != staticFileProcessingExecutionModeChangedPaths {
		t.Fatalf(
			"private mode=%v, want %v",
			decision.privateStaticProcessing.mode,
			staticFileProcessingExecutionModeChangedPaths,
		)
	}
	if !decision.writeFrameworkPublicFileMap {
		t.Fatal("expected writeFrameworkPublicFileMap=true")
	}

	publicChangedPaths[0] = "mutated-public-path"
	privateChangedPaths[0] = "mutated-private-path"
	if decision.publicStaticProcessing.changedFilePaths[0] != "public/logo.png" {
		t.Fatalf("expected copied public changed paths, got %#v", decision.publicStaticProcessing.changedFilePaths)
	}
	if decision.privateStaticProcessing.changedFilePaths[0] != "private/templates/home.html" {
		t.Fatalf("expected copied private changed paths, got %#v", decision.privateStaticProcessing.changedFilePaths)
	}
}

func TestShouldExecuteAnyFileProcessingForBuildDecision(t *testing.T) {
	testCases := []struct {
		name                      string
		executionDecision         buildPhaseExecutionDecision
		expectedExecuteProcessing bool
	}{
		{
			name: "no file processing work",
			executionDecision: buildPhaseExecutionDecision{
				publicStaticProcessing:  staticFileProcessingExecutionDecision{},
				privateStaticProcessing: staticFileProcessingExecutionDecision{},
			},
			expectedExecuteProcessing: false,
		},
		{
			name: "public static processing triggers file-processing phase",
			executionDecision: buildPhaseExecutionDecision{
				publicStaticProcessing: staticFileProcessingExecutionDecision{
					mode: staticFileProcessingExecutionModeFullScan,
				},
			},
			expectedExecuteProcessing: true,
		},
		{
			name: "private static processing triggers file-processing phase",
			executionDecision: buildPhaseExecutionDecision{
				privateStaticProcessing: staticFileProcessingExecutionDecision{
					mode: staticFileProcessingExecutionModeChangedPaths,
				},
			},
			expectedExecuteProcessing: true,
		},
		{
			name: "critical css build triggers file-processing phase",
			executionDecision: buildPhaseExecutionDecision{
				buildCriticalCSS: true,
			},
			expectedExecuteProcessing: true,
		},
		{
			name: "normal css build triggers file-processing phase",
			executionDecision: buildPhaseExecutionDecision{
				buildNormalCSS: true,
			},
			expectedExecuteProcessing: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shouldExecuteProcessing := shouldExecuteAnyFileProcessingForBuildDecision(
				testCase.executionDecision,
			)
			if shouldExecuteProcessing != testCase.expectedExecuteProcessing {
				t.Fatalf(
					"shouldExecuteAnyFileProcessingForBuildDecision()=%t, want %t",
					shouldExecuteProcessing,
					testCase.expectedExecuteProcessing,
				)
			}
		})
	}
}

func TestExecuteStaticFileProcessingForBuildPhase(t *testing.T) {
	t.Run("none mode executes no processors", func(t *testing.T) {
		fullCallCount := 0
		changedCallCount := 0
		err := executeStaticFileProcessingForBuildPhase(
			func() error {
				fullCallCount++
				return nil
			},
			func([]string) error {
				changedCallCount++
				return nil
			},
			staticFileProcessingExecutionDecision{
				mode: staticFileProcessingExecutionModeNone,
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fullCallCount != 0 || changedCallCount != 0 {
			t.Fatalf("expected no processor calls, got full=%d changed=%d", fullCallCount, changedCallCount)
		}
	})

	t.Run("full-scan mode executes full processor only", func(t *testing.T) {
		fullCallCount := 0
		changedCallCount := 0
		err := executeStaticFileProcessingForBuildPhase(
			func() error {
				fullCallCount++
				return nil
			},
			func([]string) error {
				changedCallCount++
				return nil
			},
			staticFileProcessingExecutionDecision{
				mode: staticFileProcessingExecutionModeFullScan,
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fullCallCount != 1 {
			t.Fatalf("expected full processor call count=1, got %d", fullCallCount)
		}
		if changedCallCount != 0 {
			t.Fatalf("expected changed processor call count=0, got %d", changedCallCount)
		}
	})

	t.Run("changed-path mode executes changed processor with exact path list", func(t *testing.T) {
		fullCallCount := 0
		changedCallCount := 0
		var receivedChangedPaths []string
		expectedChangedPaths := []string{"public/logo.png", "public/app.js"}
		err := executeStaticFileProcessingForBuildPhase(
			func() error {
				fullCallCount++
				return nil
			},
			func(changedPaths []string) error {
				changedCallCount++
				receivedChangedPaths = append([]string(nil), changedPaths...)
				return nil
			},
			staticFileProcessingExecutionDecision{
				mode:             staticFileProcessingExecutionModeChangedPaths,
				changedFilePaths: expectedChangedPaths,
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fullCallCount != 0 {
			t.Fatalf("expected full processor call count=0, got %d", fullCallCount)
		}
		if changedCallCount != 1 {
			t.Fatalf("expected changed processor call count=1, got %d", changedCallCount)
		}
		if !reflect.DeepEqual(receivedChangedPaths, expectedChangedPaths) {
			t.Fatalf("changed paths=%#v, want %#v", receivedChangedPaths, expectedChangedPaths)
		}
	})

	t.Run("full-scan processor error is propagated", func(t *testing.T) {
		expectedError := errors.New("full-scan failed")
		err := executeStaticFileProcessingForBuildPhase(
			func() error {
				return expectedError
			},
			func([]string) error {
				t.Fatal("changed processor should not run for full-scan mode")
				return nil
			},
			staticFileProcessingExecutionDecision{
				mode: staticFileProcessingExecutionModeFullScan,
			},
		)
		if err == nil {
			t.Fatal("expected error from full-scan processor")
		}
		if err != expectedError {
			t.Fatalf("error=%v, want %v", err, expectedError)
		}
	})

	t.Run("changed-path processor error is propagated", func(t *testing.T) {
		expectedError := errors.New("changed-path processing failed")
		err := executeStaticFileProcessingForBuildPhase(
			func() error {
				t.Fatal("full processor should not run for changed-path mode")
				return nil
			},
			func([]string) error {
				return expectedError
			},
			staticFileProcessingExecutionDecision{
				mode:             staticFileProcessingExecutionModeChangedPaths,
				changedFilePaths: []string{"a.txt"},
			},
		)
		if err == nil {
			t.Fatal("expected error from changed-path processor")
		}
		if err != expectedError {
			t.Fatalf("error=%v, want %v", err, expectedError)
		}
	})
}

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

func TestHookStageRestartGatingTracksRefreshActionRestartFlag(t *testing.T) {
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
			shouldShortCircuit := shouldShortCircuitPipelineForHookStageResult(
				hookStageResult{
					refreshActionResult: testCase.actionResult,
				},
			)
			shouldContinue := !shouldShortCircuit
			if shouldContinue != testCase.expectedShouldContinue {
				t.Fatalf(
					"hook-stage restart gating produced continue=%t, want %t",
					shouldContinue,
					testCase.expectedShouldContinue,
				)
			}
		})
	}
}

func TestShouldShortCircuitPipelineForHookStageResult(t *testing.T) {
	testCases := []struct {
		name                   string
		hookStageResult        hookStageResult
		expectedShouldContinue bool
	}{
		{
			name: "hook-stage restart short-circuits pipeline",
			hookStageResult: hookStageResult{
				refreshActionResult: refreshActionApplicationResult{restartRequested: true},
			},
			expectedShouldContinue: false,
		},
		{
			name: "hook-stage with no restart continues pipeline",
			hookStageResult: hookStageResult{
				refreshActionResult: refreshActionApplicationResult{restartRequested: false},
			},
			expectedShouldContinue: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shouldShortCircuit := shouldShortCircuitPipelineForHookStageResult(
				testCase.hookStageResult,
			)
			shouldContinue := !shouldShortCircuit
			if shouldContinue != testCase.expectedShouldContinue {
				t.Fatalf(
					"shouldShortCircuitPipelineForHookStageResult() produced continue=%t, want %t",
					shouldContinue,
					testCase.expectedShouldContinue,
				)
			}
		})
	}
}

func TestDeriveHookStageContinuationDecision(t *testing.T) {
	testCases := []struct {
		name                   string
		hookStageResult        hookStageResult
		expectedShouldContinue bool
		expectedRestartResult  refreshActionApplicationResult
	}{
		{
			name: "restart request halts pipeline and returns restart action result",
			hookStageResult: hookStageResult{
				refreshActionResult: refreshActionApplicationResult{
					restartRequested: true,
					recompileGo:      true,
				},
			},
			expectedShouldContinue: false,
			expectedRestartResult: refreshActionApplicationResult{
				restartRequested: true,
				recompileGo:      true,
			},
		},
		{
			name: "no restart request continues pipeline",
			hookStageResult: hookStageResult{
				refreshActionResult: refreshActionApplicationResult{
					restartRequested: false,
					recompileGo:      false,
				},
			},
			expectedShouldContinue: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			continuationDecision := deriveHookStageContinuationDecision(
				testCase.hookStageResult,
			)
			if continuationDecision.shouldContinue != testCase.expectedShouldContinue {
				t.Fatalf(
					"shouldContinue=%t, want %t",
					continuationDecision.shouldContinue,
					testCase.expectedShouldContinue,
				)
			}
			if !reflect.DeepEqual(
				continuationDecision.restartActionResult,
				testCase.expectedRestartResult,
			) {
				t.Fatalf(
					"restartActionResult=%#v, want %#v",
					continuationDecision.restartActionResult,
					testCase.expectedRestartResult,
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

func TestShouldExecuteBrowserPhaseAfterHookStageResults(t *testing.T) {
	testCases := []struct {
		name                     string
		hookStageResults         []hookStageResult
		expectedShouldRunBrowser bool
	}{
		{
			name:                     "no restart requests executes browser phase",
			hookStageResults:         []hookStageResult{{}, {}, {}},
			expectedShouldRunBrowser: true,
		},
		{
			name: "any restart request skips browser phase",
			hookStageResults: []hookStageResult{
				{},
				{refreshActionResult: refreshActionApplicationResult{restartRequested: true}},
				{},
			},
			expectedShouldRunBrowser: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shouldRunBrowserPhase := shouldExecuteBrowserPhaseAfterHookStageResults(
				testCase.hookStageResults...,
			)
			if shouldRunBrowserPhase != testCase.expectedShouldRunBrowser {
				t.Fatalf(
					"shouldExecuteBrowserPhaseAfterHookStageResults()=%t, want %t",
					shouldRunBrowserPhase,
					testCase.expectedShouldRunBrowser,
				)
			}
		})
	}
}

func TestApplyHookStageActionsToWorkSet(t *testing.T) {
	t.Run("nil workset keeps stage actions and zero refresh result", func(t *testing.T) {
		stageActions := []wave.RefreshAction{
			{ReloadBrowser: true},
			{WaitForApp: true},
		}
		stageResult := applyHookStageActionsToWorkSet(stageActions, nil)
		if len(stageResult.actions) != len(stageActions) {
			t.Fatalf("stage action count=%d, want %d", len(stageResult.actions), len(stageActions))
		}
		if stageResult.refreshActionResult.restartRequested || stageResult.refreshActionResult.recompileGo {
			t.Fatalf("expected zero refresh result for nil workset, got %#v", stageResult.refreshActionResult)
		}
	})

	t.Run("workset applies stage actions and returns refresh result", func(t *testing.T) {
		work := &workSet{}
		stageActions := []wave.RefreshAction{
			{ReloadBrowser: true, WaitForApp: true},
			{TriggerRestart: true, RecompileGo: true},
			{WaitForVite: true},
		}

		stageResult := applyHookStageActionsToWorkSet(stageActions, work)
		if len(stageResult.actions) != len(stageActions) {
			t.Fatalf("stage action count=%d, want %d", len(stageResult.actions), len(stageActions))
		}
		if !stageResult.refreshActionResult.restartRequested {
			t.Fatal("expected restartRequested=true from stage result")
		}
		if !stageResult.refreshActionResult.recompileGo {
			t.Fatal("expected recompileGo=true from first restart action")
		}
		if work.browser.action != browserPhaseActionHardReload {
			t.Fatalf("expected pre-restart reload action applied, got %v", work.browser.action)
		}
		if !work.browser.waitForApp {
			t.Fatal("expected pre-restart wait-for-app applied")
		}
		if work.browser.waitForVite {
			t.Fatal("expected post-restart stage actions not to be applied")
		}
	})
}

func TestRunAndApplyHookStageActionsToWorkSet(t *testing.T) {
	t.Run("nil stage runner is safe and yields empty stage actions", func(t *testing.T) {
		work := &workSet{}
		stageResult := runAndApplyHookStageActionsToWorkSet(nil, work)
		if len(stageResult.actions) != 0 {
			t.Fatalf("expected zero stage actions from nil runner, got %#v", stageResult.actions)
		}
		if stageResult.refreshActionResult.restartRequested || stageResult.refreshActionResult.recompileGo {
			t.Fatalf("expected zero refresh result from nil runner, got %#v", stageResult.refreshActionResult)
		}
	})

	t.Run("runs stage actions and applies them to workset", func(t *testing.T) {
		work := &workSet{}
		stageRunCount := 0
		stageResult := runAndApplyHookStageActionsToWorkSet(
			func() []wave.RefreshAction {
				stageRunCount++
				return []wave.RefreshAction{
					{ReloadBrowser: true},
					{WaitForVite: true},
				}
			},
			work,
		)
		if stageRunCount != 1 {
			t.Fatalf("expected stage runner count=1, got %d", stageRunCount)
		}
		if len(stageResult.actions) != 2 {
			t.Fatalf("expected stage action count=2, got %#v", stageResult.actions)
		}
		if work.browser.action != browserPhaseActionHardReload {
			t.Fatalf("expected hard reload action applied, got %v", work.browser.action)
		}
		if !work.browser.waitForVite {
			t.Fatal("expected wait-for-vite applied")
		}
	})
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
