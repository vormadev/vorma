package eventpipeline_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/hooks"
)

var errSynthetic = errors.New("synthetic error")

func TestDeriveStaticFileProcessingExecutionDecision(t *testing.T) {
	t.Run("processing disabled returns no-op decision", func(t *testing.T) {
		decision := eventpipeline.DeriveStaticFileProcessingExecutionDecision(
			false,
			[]string{"public/logo.png"},
		)
		if decision.Mode != eventpipeline.StaticFileProcessingExecutionModeNone {
			t.Fatalf(
				"mode=%v, want %v",
				decision.Mode,
				eventpipeline.StaticFileProcessingExecutionModeNone,
			)
		}
		if decision.ShouldProcess() {
			t.Fatalf("expected shouldProcess=false, got %#v", decision)
		}
		if decision.ChangedFilePaths != nil {
			t.Fatalf(
				"expected changedFilePaths=nil, got %#v",
				decision.ChangedFilePaths,
			)
		}
	})

	t.Run(
		"processing enabled with empty changed paths selects full scan",
		func(t *testing.T) {
			decision := eventpipeline.DeriveStaticFileProcessingExecutionDecision(
				true,
				nil,
			)
			if decision.Mode != eventpipeline.StaticFileProcessingExecutionModeFullScan {
				t.Fatalf(
					"mode=%v, want %v",
					decision.Mode,
					eventpipeline.StaticFileProcessingExecutionModeFullScan,
				)
			}
			if !decision.ShouldProcess() {
				t.Fatalf("expected shouldProcess=true, got %#v", decision)
			}
			if decision.ChangedFilePaths != nil {
				t.Fatalf(
					"expected changedFilePaths=nil for full scan, got %#v",
					decision.ChangedFilePaths,
				)
			}
		},
	)

	t.Run(
		"processing enabled with changed paths selects changed-path mode and copies slice",
		func(t *testing.T) {
			changedPaths := []string{"public/logo.png", "public/app.js"}
			decision := eventpipeline.DeriveStaticFileProcessingExecutionDecision(
				true,
				changedPaths,
			)
			if decision.Mode != eventpipeline.StaticFileProcessingExecutionModeChangedPaths {
				t.Fatalf(
					"mode=%v, want %v",
					decision.Mode,
					eventpipeline.StaticFileProcessingExecutionModeChangedPaths,
				)
			}
			expectedChangedPaths := []string{"public/logo.png", "public/app.js"}
			if !reflect.DeepEqual(
				decision.ChangedFilePaths,
				expectedChangedPaths,
			) {
				t.Fatalf(
					"changedFilePaths=%#v, want %#v",
					decision.ChangedFilePaths,
					expectedChangedPaths,
				)
			}

			changedPaths[0] = "mutated/path.css"
			if decision.ChangedFilePaths[0] != "public/logo.png" {
				t.Fatalf(
					"expected decision changedFilePaths to be isolated copy, got %#v",
					decision.ChangedFilePaths,
				)
			}
		},
	)
}

func TestShouldWriteFrameworkPublicFileMapTSForBuildDecision(t *testing.T) {
	testCases := []struct {
		Name                           string
		ShouldProcessPublicStaticFiles bool
		FrameworkPublicFileMapOutDir   string
		ExpectedShouldWrite            bool
	}{
		{
			Name:                           "public static disabled never writes framework map",
			ShouldProcessPublicStaticFiles: false,
			FrameworkPublicFileMapOutDir:   "framework",
			ExpectedShouldWrite:            false,
		},
		{
			Name:                           "empty outdir disables framework map write",
			ShouldProcessPublicStaticFiles: true,
			FrameworkPublicFileMapOutDir:   "",
			ExpectedShouldWrite:            false,
		},
		{
			Name:                           "public static enabled with outdir writes framework map",
			ShouldProcessPublicStaticFiles: true,
			FrameworkPublicFileMapOutDir:   "framework",
			ExpectedShouldWrite:            true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			shouldWrite := eventpipeline.ShouldWriteFrameworkPublicFileMapTSForBuildDecision(
				testCase.ShouldProcessPublicStaticFiles,
				testCase.FrameworkPublicFileMapOutDir,
			)
			if shouldWrite != testCase.ExpectedShouldWrite {
				t.Fatalf(
					"shouldWrite=%t, want %t",
					shouldWrite,
					testCase.ExpectedShouldWrite,
				)
			}
		})
	}
}

func TestDeriveBuildPhaseExecutionDecision(t *testing.T) {
	publicChangedPaths := []string{"public/logo.png"}
	privateChangedPaths := []string{"private/templates/home.html"}
	decision := eventpipeline.DeriveBuildPhaseExecutionDecision(
		eventpipeline.BuildPhaseDecision{
			CompileGo:                     true,
			BuildCriticalCSS:              true,
			BuildNormalCSS:                false,
			ProcessPublicFiles:            true,
			ProcessPrivateFiles:           true,
			PublicStaticChangedFilePaths:  publicChangedPaths,
			PrivateStaticChangedFilePaths: privateChangedPaths,
		},
		"framework-out",
	)

	if !decision.CompileGo {
		t.Fatal("expected compileGo=true")
	}
	if !decision.BuildCriticalCSS {
		t.Fatal("expected buildCriticalCSS=true")
	}
	if decision.BuildNormalCSS {
		t.Fatal("expected buildNormalCSS=false")
	}
	if decision.PublicStaticProcessing.Mode != eventpipeline.StaticFileProcessingExecutionModeChangedPaths {
		t.Fatalf(
			"public mode=%v, want %v",
			decision.PublicStaticProcessing.Mode,
			eventpipeline.StaticFileProcessingExecutionModeChangedPaths,
		)
	}
	if decision.PrivateStaticProcessing.Mode != eventpipeline.StaticFileProcessingExecutionModeChangedPaths {
		t.Fatalf(
			"private mode=%v, want %v",
			decision.PrivateStaticProcessing.Mode,
			eventpipeline.StaticFileProcessingExecutionModeChangedPaths,
		)
	}
	if !decision.WriteFrameworkPublicFileMap {
		t.Fatal("expected writeFrameworkPublicFileMap=true")
	}

	publicChangedPaths[0] = "mutated-public-path"
	privateChangedPaths[0] = "mutated-private-path"
	if decision.PublicStaticProcessing.ChangedFilePaths[0] != "public/logo.png" {
		t.Fatalf(
			"expected copied public changed paths, got %#v",
			decision.PublicStaticProcessing.ChangedFilePaths,
		)
	}
	if decision.PrivateStaticProcessing.ChangedFilePaths[0] != "private/templates/home.html" {
		t.Fatalf(
			"expected copied private changed paths, got %#v",
			decision.PrivateStaticProcessing.ChangedFilePaths,
		)
	}
}

func TestShouldExecuteAnyFileProcessingForBuildDecision(t *testing.T) {
	testCases := []struct {
		Name                      string
		ExecutionDecision         eventpipeline.BuildPhaseExecutionDecision
		ExpectedExecuteProcessing bool
	}{
		{
			Name: "no file processing work",
			ExecutionDecision: eventpipeline.BuildPhaseExecutionDecision{
				PublicStaticProcessing:  eventpipeline.StaticFileProcessingExecutionDecision{},
				PrivateStaticProcessing: eventpipeline.StaticFileProcessingExecutionDecision{},
			},
			ExpectedExecuteProcessing: false,
		},
		{
			Name: "public static processing triggers file-processing phase",
			ExecutionDecision: eventpipeline.BuildPhaseExecutionDecision{
				PublicStaticProcessing: eventpipeline.StaticFileProcessingExecutionDecision{
					Mode: eventpipeline.StaticFileProcessingExecutionModeFullScan,
				},
			},
			ExpectedExecuteProcessing: true,
		},
		{
			Name: "private static processing triggers file-processing phase",
			ExecutionDecision: eventpipeline.BuildPhaseExecutionDecision{
				PrivateStaticProcessing: eventpipeline.StaticFileProcessingExecutionDecision{
					Mode: eventpipeline.StaticFileProcessingExecutionModeChangedPaths,
				},
			},
			ExpectedExecuteProcessing: true,
		},
		{
			Name: "critical css build triggers file-processing phase",
			ExecutionDecision: eventpipeline.BuildPhaseExecutionDecision{
				BuildCriticalCSS: true,
			},
			ExpectedExecuteProcessing: true,
		},
		{
			Name: "normal css build triggers file-processing phase",
			ExecutionDecision: eventpipeline.BuildPhaseExecutionDecision{
				BuildNormalCSS: true,
			},
			ExpectedExecuteProcessing: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			shouldExecuteProcessing := eventpipeline.ShouldExecuteAnyFileProcessingForBuildDecision(
				testCase.ExecutionDecision,
			)
			if shouldExecuteProcessing != testCase.ExpectedExecuteProcessing {
				t.Fatalf(
					"eventpipeline.ShouldExecuteAnyFileProcessingForBuildDecision()=%t, want %t",
					shouldExecuteProcessing,
					testCase.ExpectedExecuteProcessing,
				)
			}
		})
	}
}

func TestFilterClassifiedEventsForProcessingByPostClassificationDecision(
	t *testing.T,
) {
	inputClassifiedEvents := []eventpipeline.ClassifiedEvent{
		{
			Event: fsnotify.Event{Name: "first.txt", Op: fsnotify.Write},
		},
		{
			Event:   fsnotify.Event{Name: "ignored.txt", Op: fsnotify.Write},
			Ignored: true,
		},
		{
			Event:     fsnotify.Event{Name: "chmod.txt", Op: fsnotify.Chmod},
			ChmodOnly: true,
		},
		{
			Event: fsnotify.Event{Name: "second.txt", Op: fsnotify.Write},
		},
	}

	filteredClassifiedEvents := eventpipeline.FilterClassifiedEventsForProcessingByPostClassificationDecision(
		inputClassifiedEvents,
	)
	if len(filteredClassifiedEvents) != 2 {
		t.Fatalf(
			"expected two included classified events, got %#v",
			filteredClassifiedEvents,
		)
	}
	if filteredClassifiedEvents[0].Event.Name != "first.txt" {
		t.Fatalf(
			"unexpected first filtered event: %#v",
			filteredClassifiedEvents[0],
		)
	}
	if filteredClassifiedEvents[1].Event.Name != "second.txt" {
		t.Fatalf(
			"unexpected second filtered event: %#v",
			filteredClassifiedEvents[1],
		)
	}
}

func TestShouldExecuteBuildPhaseForExecutionDecision(t *testing.T) {
	testCases := []struct {
		Name                  string
		ExecutionDecision     eventpipeline.BuildPhaseExecutionDecision
		ExpectedShouldExecute bool
	}{
		{
			Name:                  "no compile or file processing work skips build phase",
			ExecutionDecision:     eventpipeline.BuildPhaseExecutionDecision{},
			ExpectedShouldExecute: false,
		},
		{
			Name: "compile-go work executes build phase",
			ExecutionDecision: eventpipeline.BuildPhaseExecutionDecision{
				CompileGo: true,
			},
			ExpectedShouldExecute: true,
		},
		{
			Name: "file processing work executes build phase",
			ExecutionDecision: eventpipeline.BuildPhaseExecutionDecision{
				BuildNormalCSS: true,
			},
			ExpectedShouldExecute: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			shouldExecuteBuildPhase := eventpipeline.ShouldExecuteBuildPhaseForExecutionDecision(
				testCase.ExecutionDecision,
			)
			if shouldExecuteBuildPhase != testCase.ExpectedShouldExecute {
				t.Fatalf(
					"eventpipeline.ShouldExecuteBuildPhaseForExecutionDecision()=%t, want %t",
					shouldExecuteBuildPhase,
					testCase.ExpectedShouldExecute,
				)
			}
		})
	}
}

func TestExecuteStaticFileProcessingForBuildPhase(t *testing.T) {
	t.Run("none mode executes no processors", func(t *testing.T) {
		fullCallCount := 0
		changedCallCount := 0
		executeProcessingError := eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
			func() error {
				fullCallCount++
				return nil
			},
			func([]string) error {
				changedCallCount++
				return nil
			},
			eventpipeline.StaticFileProcessingExecutionDecision{
				Mode: eventpipeline.StaticFileProcessingExecutionModeNone,
			},
		)
		if executeProcessingError != nil {
			t.Fatalf("unexpected error: %v", executeProcessingError)
		}
		if fullCallCount != 0 || changedCallCount != 0 {
			t.Fatalf(
				"expected no processor calls, got full=%d changed=%d",
				fullCallCount,
				changedCallCount,
			)
		}
	})

	t.Run("full-scan mode executes full processor only", func(t *testing.T) {
		fullCallCount := 0
		changedCallCount := 0
		executeProcessingError := eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
			func() error {
				fullCallCount++
				return nil
			},
			func([]string) error {
				changedCallCount++
				return nil
			},
			eventpipeline.StaticFileProcessingExecutionDecision{
				Mode: eventpipeline.StaticFileProcessingExecutionModeFullScan,
			},
		)
		if executeProcessingError != nil {
			t.Fatalf("unexpected error: %v", executeProcessingError)
		}
		if fullCallCount != 1 {
			t.Fatalf(
				"expected full processor call count=1, got %d",
				fullCallCount,
			)
		}
		if changedCallCount != 0 {
			t.Fatalf(
				"expected changed processor call count=0, got %d",
				changedCallCount,
			)
		}
	})

	t.Run(
		"changed-path mode executes changed processor with exact path list",
		func(t *testing.T) {
			fullCallCount := 0
			changedCallCount := 0
			var receivedChangedPaths []string
			expectedChangedPaths := []string{"public/logo.png", "public/app.js"}
			executeProcessingError := eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
				func() error {
					fullCallCount++
					return nil
				},
				func(changedPaths []string) error {
					changedCallCount++
					receivedChangedPaths = append(
						[]string(nil),
						changedPaths...,
					)
					return nil
				},
				eventpipeline.StaticFileProcessingExecutionDecision{
					Mode:             eventpipeline.StaticFileProcessingExecutionModeChangedPaths,
					ChangedFilePaths: expectedChangedPaths,
				},
			)
			if executeProcessingError != nil {
				t.Fatalf("unexpected error: %v", executeProcessingError)
			}
			if fullCallCount != 0 {
				t.Fatalf(
					"expected full processor call count=0, got %d",
					fullCallCount,
				)
			}
			if changedCallCount != 1 {
				t.Fatalf(
					"expected changed processor call count=1, got %d",
					changedCallCount,
				)
			}
			if !reflect.DeepEqual(receivedChangedPaths, expectedChangedPaths) {
				t.Fatalf(
					"changed paths=%#v, want %#v",
					receivedChangedPaths,
					expectedChangedPaths,
				)
			}
		},
	)

	t.Run("full-scan processor error is propagated", func(t *testing.T) {
		expectedError := errors.New("full-scan failed")
		executeProcessingError := eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
			func() error {
				return expectedError
			},
			func([]string) error {
				t.Fatal("changed processor should not run for full-scan mode")
				return nil
			},
			eventpipeline.StaticFileProcessingExecutionDecision{
				Mode: eventpipeline.StaticFileProcessingExecutionModeFullScan,
			},
		)
		if executeProcessingError == nil {
			t.Fatal("expected error from full-scan processor")
		}
		if executeProcessingError != expectedError {
			t.Fatalf("error=%v, want %v", executeProcessingError, expectedError)
		}
	})

	t.Run("changed-path processor error is propagated", func(t *testing.T) {
		expectedError := errors.New("changed-path processing failed")
		executeProcessingError := eventpipeline.ExecuteStaticFileProcessingForBuildPhase(
			func() error {
				t.Fatal("full processor should not run for changed-path mode")
				return nil
			},
			func([]string) error {
				return expectedError
			},
			eventpipeline.StaticFileProcessingExecutionDecision{
				Mode:             eventpipeline.StaticFileProcessingExecutionModeChangedPaths,
				ChangedFilePaths: []string{"a.txt"},
			},
		)
		if executeProcessingError == nil {
			t.Fatal("expected error from changed-path processor")
		}
		if executeProcessingError != expectedError {
			t.Fatalf("error=%v, want %v", executeProcessingError, expectedError)
		}
	})
}

func TestDeriveImplicitBuildExecutionDecision(t *testing.T) {
	testCases := []struct {
		Name                     string
		ShouldRunImplicitBuild   bool
		EventCount               int
		ExpectedShouldRunBuild   bool
		ExpectedSkipBuildLogLine string
	}{
		{
			Name:                   "implicit build enabled keeps build phase",
			ShouldRunImplicitBuild: true,
			EventCount:             1,
			ExpectedShouldRunBuild: true,
		},
		{
			Name:                     "single run-on-change-only event logs single-event skip message",
			ShouldRunImplicitBuild:   false,
			EventCount:               1,
			ExpectedShouldRunBuild:   false,
			ExpectedSkipBuildLogLine: "RunOnChangeOnly: skipping implicit build phase",
		},
		{
			Name:                     "batch run-on-change-only events log batch skip message",
			ShouldRunImplicitBuild:   false,
			EventCount:               3,
			ExpectedShouldRunBuild:   false,
			ExpectedSkipBuildLogLine: "All events are RunOnChangeOnly, skipping implicit build phase",
		},
		{
			Name:                     "empty event batch uses batch skip message",
			ShouldRunImplicitBuild:   false,
			EventCount:               0,
			ExpectedShouldRunBuild:   false,
			ExpectedSkipBuildLogLine: "All events are RunOnChangeOnly, skipping implicit build phase",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			implicitBuildDecision := hooks.DeriveImplicitBuildExecutionDecision(
				testCase.ShouldRunImplicitBuild,
				testCase.EventCount,
			)
			if implicitBuildDecision.ShouldRunImplicitBuild != testCase.ExpectedShouldRunBuild {
				t.Fatalf(
					"hooks.DeriveImplicitBuildExecutionDecision().ShouldRunImplicitBuild=%t, want %t",
					implicitBuildDecision.ShouldRunImplicitBuild,
					testCase.ExpectedShouldRunBuild,
				)
			}
			if implicitBuildDecision.SkipImplicitBuildLogEntry != testCase.ExpectedSkipBuildLogLine {
				t.Fatalf(
					"hooks.DeriveImplicitBuildExecutionDecision().SkipImplicitBuildLogEntry=%q, want %q",
					implicitBuildDecision.SkipImplicitBuildLogEntry,
					testCase.ExpectedSkipBuildLogLine,
				)
			}
		})
	}
}

func TestHookStageRestartGatingTracksRefreshActionRestartFlag(t *testing.T) {
	testCases := []struct {
		Name                   string
		ActionResult           eventpipeline.RefreshActionApplicationResult
		ExpectedShouldContinue bool
	}{
		{
			Name: "restart requested short-circuits pipeline",
			ActionResult: eventpipeline.RefreshActionApplicationResult{
				RestartRequested: true,
			},
			ExpectedShouldContinue: false,
		},
		{
			Name: "no restart continues pipeline",
			ActionResult: eventpipeline.RefreshActionApplicationResult{
				RestartRequested: false,
			},
			ExpectedShouldContinue: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			shouldShortCircuit := hooks.ShouldShortCircuitPipelineForHookStageResult(
				hooks.HookStageResult{
					RefreshActionResult: testCase.ActionResult,
				},
				hooks.HookStageFailurePolicyContinue,
			)
			shouldContinue := !shouldShortCircuit
			if shouldContinue != testCase.ExpectedShouldContinue {
				t.Fatalf(
					"hook-stage restart gating produced continue=%t, want %t",
					shouldContinue,
					testCase.ExpectedShouldContinue,
				)
			}
		})
	}
}

func TestShouldShortCircuitPipelineForHookStageResult(t *testing.T) {
	testCases := []struct {
		Name                   string
		HookStageResult        hooks.HookStageResult
		ExpectedShouldContinue bool
	}{
		{
			Name: "hook-stage restart short-circuits pipeline",
			HookStageResult: hooks.HookStageResult{
				RefreshActionResult: eventpipeline.RefreshActionApplicationResult{
					RestartRequested: true,
				},
			},
			ExpectedShouldContinue: false,
		},
		{
			Name: "hook-stage with no restart continues pipeline",
			HookStageResult: hooks.HookStageResult{
				RefreshActionResult: eventpipeline.RefreshActionApplicationResult{
					RestartRequested: false,
				},
			},
			ExpectedShouldContinue: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			shouldShortCircuit := hooks.ShouldShortCircuitPipelineForHookStageResult(
				testCase.HookStageResult,
				hooks.HookStageFailurePolicyContinue,
			)
			shouldContinue := !shouldShortCircuit
			if shouldContinue != testCase.ExpectedShouldContinue {
				t.Fatalf(
					"hooks.ShouldShortCircuitPipelineForHookStageResult() produced continue=%t, want %t",
					shouldContinue,
					testCase.ExpectedShouldContinue,
				)
			}
		})
	}
}

func TestDeriveHookStageContinuationDecision(t *testing.T) {
	testCases := []struct {
		Name                   string
		HookStageResult        hooks.HookStageResult
		ExpectedShouldContinue bool
		ExpectedRestartResult  eventpipeline.RefreshActionApplicationResult
	}{
		{
			Name: "restart request halts pipeline and returns restart action result",
			HookStageResult: hooks.HookStageResult{
				RefreshActionResult: eventpipeline.RefreshActionApplicationResult{
					RestartRequested: true,
					RecompileGo:      true,
				},
			},
			ExpectedShouldContinue: false,
			ExpectedRestartResult: eventpipeline.RefreshActionApplicationResult{
				RestartRequested: true,
				RecompileGo:      true,
			},
		},
		{
			Name: "no restart request continues pipeline",
			HookStageResult: hooks.HookStageResult{
				RefreshActionResult: eventpipeline.RefreshActionApplicationResult{
					RestartRequested: false,
					RecompileGo:      false,
				},
			},
			ExpectedShouldContinue: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			continuationDecision := hooks.DeriveHookStageContinuationDecision(
				testCase.HookStageResult,
			)
			if continuationDecision.ShouldContinue != testCase.ExpectedShouldContinue {
				t.Fatalf(
					"shouldContinue=%t, want %t",
					continuationDecision.ShouldContinue,
					testCase.ExpectedShouldContinue,
				)
			}
			if !reflect.DeepEqual(
				continuationDecision.RestartActionResult,
				testCase.ExpectedRestartResult,
			) {
				t.Fatalf(
					"restartActionResult=%#v, want %#v",
					continuationDecision.RestartActionResult,
					testCase.ExpectedRestartResult,
				)
			}
		})
	}
}

func TestDeriveHookStageContinuationDecisionWithFailurePolicy(t *testing.T) {
	t.Run(
		"fail-open policy continues despite stage errors when restart not requested",
		func(t *testing.T) {
			continuationDecision := hooks.DeriveHookStageContinuationDecisionWithFailurePolicy(
				hooks.HookStageResult{
					StageType:       hooks.HookStageTypePre,
					ExecutionErrors: []error{errSynthetic},
					RefreshActionResult: eventpipeline.RefreshActionApplicationResult{
						RestartRequested: false,
					},
				},
				hooks.HookStageFailurePolicyContinue,
			)

			if !continuationDecision.ShouldContinue {
				t.Fatalf(
					"expected fail-open hook-stage policy to continue, got %#v",
					continuationDecision,
				)
			}
			if continuationDecision.StopReason != hooks.HookStageContinuationStopReasonNone {
				t.Fatalf(
					"expected no stop reason for fail-open continuation, got %#v",
					continuationDecision,
				)
			}
		},
	)

	t.Run(
		"fail-closed policy stops on stage errors when restart not requested",
		func(t *testing.T) {
			continuationDecision := hooks.DeriveHookStageContinuationDecisionWithFailurePolicy(
				hooks.HookStageResult{
					StageType:       hooks.HookStageTypePost,
					ExecutionErrors: []error{errSynthetic},
					RefreshActionResult: eventpipeline.RefreshActionApplicationResult{
						RestartRequested: false,
					},
				},
				hooks.HookStageFailurePolicyStop,
			)

			if continuationDecision.ShouldContinue {
				t.Fatalf(
					"expected fail-closed hook-stage policy to stop, got %#v",
					continuationDecision,
				)
			}
			if continuationDecision.StopReason != hooks.HookStageContinuationStopReasonStageFailure {
				t.Fatalf(
					"expected stage-failure stop reason, got %#v",
					continuationDecision,
				)
			}
		},
	)

	t.Run(
		"restart request takes precedence over stage failure policy",
		func(t *testing.T) {
			expectedRestartActionResult := eventpipeline.RefreshActionApplicationResult{
				RestartRequested: true,
				RecompileGo:      true,
			}
			continuationDecision := hooks.DeriveHookStageContinuationDecisionWithFailurePolicy(
				hooks.HookStageResult{
					StageType:           hooks.HookStageTypeConcurrent,
					ExecutionErrors:     []error{errSynthetic},
					RefreshActionResult: expectedRestartActionResult,
				},
				hooks.HookStageFailurePolicyStop,
			)

			if continuationDecision.ShouldContinue {
				t.Fatalf(
					"expected restart request to stop continuation, got %#v",
					continuationDecision,
				)
			}
			if continuationDecision.StopReason != hooks.HookStageContinuationStopReasonRestartRequested {
				t.Fatalf(
					"expected restart-request stop reason, got %#v",
					continuationDecision,
				)
			}
			if !reflect.DeepEqual(
				continuationDecision.RestartActionResult,
				expectedRestartActionResult,
			) {
				t.Fatalf(
					"restartActionResult=%#v, want %#v",
					continuationDecision.RestartActionResult,
					expectedRestartActionResult,
				)
			}
		},
	)
}

func TestDeriveHookStageFailurePolicy_FromConfiguredValue(t *testing.T) {
	testCases := []struct {
		Name                             string
		StageType                        hooks.HookStageType
		ConfiguredHookStageFailurePolicy string
		ExpectedHookStageFailurePolicy   hooks.HookStageFailurePolicy
	}{
		{
			Name:                             "empty policy defaults fail-closed",
			StageType:                        hooks.HookStageTypePre,
			ConfiguredHookStageFailurePolicy: "",
			ExpectedHookStageFailurePolicy:   hooks.HookStageFailurePolicyStop,
		},
		{
			Name:                             "explicit fail-open remains fail-open",
			StageType:                        hooks.HookStageTypeConcurrent,
			ConfiguredHookStageFailurePolicy: "fail-open",
			ExpectedHookStageFailurePolicy:   hooks.HookStageFailurePolicyContinue,
		},
		{
			Name:                             "explicit fail-closed applies fail-closed",
			StageType:                        hooks.HookStageTypePost,
			ConfiguredHookStageFailurePolicy: "fail-closed",
			ExpectedHookStageFailurePolicy:   hooks.HookStageFailurePolicyStop,
		},
		{
			Name:                             "invalid configured policy falls back fail-closed",
			StageType:                        hooks.HookStageTypePre,
			ConfiguredHookStageFailurePolicy: "invalid-policy",
			ExpectedHookStageFailurePolicy:   hooks.HookStageFailurePolicyStop,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			hookStageFailurePolicyForStage := hooks.DeriveHookStageFailurePolicy(
				testCase.ConfiguredHookStageFailurePolicy,
			)
			if hookStageFailurePolicyForStage != testCase.ExpectedHookStageFailurePolicy {
				t.Fatalf(
					"hooks.DeriveHookStageFailurePolicy(%q)=%v, want %v",
					testCase.ConfiguredHookStageFailurePolicy,
					hookStageFailurePolicyForStage,
					testCase.ExpectedHookStageFailurePolicy,
				)
			}
		})
	}
}

func TestShouldStartAppAfterImplicitBuild(t *testing.T) {
	testCases := []struct {
		Name                   string
		ShouldRunImplicitBuild bool
		Restart                eventpipeline.RestartPhaseDecision
		ExpectedStartApp       bool
	}{
		{
			Name:                   "build-enabled and restart-app starts app",
			ShouldRunImplicitBuild: true,
			Restart: eventpipeline.RestartPhaseDecision{
				RestartApp: true,
			},
			ExpectedStartApp: true,
		},
		{
			Name:                   "build-disabled never starts app",
			ShouldRunImplicitBuild: false,
			Restart: eventpipeline.RestartPhaseDecision{
				RestartApp: true,
			},
			ExpectedStartApp: false,
		},
		{
			Name:                   "restart-app false does not start app",
			ShouldRunImplicitBuild: true,
			Restart: eventpipeline.RestartPhaseDecision{
				RestartApp: false,
			},
			ExpectedStartApp: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			shouldStartApp := hooks.ShouldStartAppAfterImplicitBuild(
				testCase.ShouldRunImplicitBuild,
				testCase.Restart,
			)
			if shouldStartApp != testCase.ExpectedStartApp {
				t.Fatalf(
					"hooks.ShouldStartAppAfterImplicitBuild()=%t, want %t",
					shouldStartApp,
					testCase.ExpectedStartApp,
				)
			}
		})
	}
}

func TestShouldExecuteBrowserPhaseAfterHookStageResults(t *testing.T) {
	testCases := []struct {
		Name                      string
		PreHookStageResult        hooks.HookStageResult
		ConcurrentHookStageResult hooks.HookStageResult
		PostHookStageResult       hooks.HookStageResult
		ExpectedShouldRunBrowser  bool
	}{
		{
			Name:                      "no restart requests executes browser phase",
			PreHookStageResult:        hooks.HookStageResult{},
			ConcurrentHookStageResult: hooks.HookStageResult{},
			PostHookStageResult:       hooks.HookStageResult{},
			ExpectedShouldRunBrowser:  true,
		},
		{
			Name:               "any restart request skips browser phase",
			PreHookStageResult: hooks.HookStageResult{},
			ConcurrentHookStageResult: hooks.HookStageResult{
				RefreshActionResult: eventpipeline.RefreshActionApplicationResult{
					RestartRequested: true,
				},
			},
			PostHookStageResult:      hooks.HookStageResult{},
			ExpectedShouldRunBrowser: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			shouldRunBrowserPhase := hooks.ShouldExecuteBrowserPhaseAfterHookStageResults(
				testCase.PreHookStageResult,
				testCase.ConcurrentHookStageResult,
				testCase.PostHookStageResult,
			)
			if shouldRunBrowserPhase != testCase.ExpectedShouldRunBrowser {
				t.Fatalf(
					"hooks.ShouldExecuteBrowserPhaseAfterHookStageResults()=%t, want %t",
					shouldRunBrowserPhase,
					testCase.ExpectedShouldRunBrowser,
				)
			}
		})
	}
}

func TestApplyHookStageActionsToWorkSet(t *testing.T) {
	t.Run(
		"nil workset keeps stage actions and zero refresh result",
		func(t *testing.T) {
			stageActions := []wave.RefreshAction{
				{ReloadBrowser: true},
				{WaitForApp: true},
			}
			refreshActionResult := hooks.ApplyHookStageActionsToWorkSet(
				nil,
				stageActions,
			)
			if refreshActionResult.RestartRequested ||
				refreshActionResult.RecompileGo {
				t.Fatalf(
					"expected zero refresh result for nil workset, got %#v",
					refreshActionResult,
				)
			}
		},
	)

	t.Run(
		"workset applies stage actions and returns refresh result",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}
			stageActions := []wave.RefreshAction{
				{ReloadBrowser: true, WaitForApp: true},
				{TriggerRestart: true, RecompileGo: true},
				{WaitForVite: true},
			}

			refreshActionResult := hooks.ApplyHookStageActionsToWorkSet(
				work,
				stageActions,
			)
			if !refreshActionResult.RestartRequested {
				t.Fatal("expected restartRequested=true from stage result")
			}
			if !refreshActionResult.RecompileGo {
				t.Fatal("expected recompileGo=true from first restart action")
			}
			if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload {
				t.Fatalf(
					"expected pre-restart reload action applied, got %v",
					work.Browser.Action,
				)
			}
			if !work.Browser.WaitForApp {
				t.Fatal("expected pre-restart wait-for-app applied")
			}
			if work.Browser.WaitForVite {
				t.Fatal("expected post-restart stage actions not to be applied")
			}
		},
	)
}

func TestRunAndApplyHookStageActionsToWorkSet(t *testing.T) {
	t.Run(
		"nil stage runner is safe and yields empty stage actions",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}
			stageResult := hooks.RunAndApplyHookStageActionsToWorkSet(
				hooks.HookStageTypePre,
				nil,
				work,
			)
			if len(stageResult.Actions) != 0 {
				t.Fatalf(
					"expected zero stage actions from nil runner, got %#v",
					stageResult.Actions,
				)
			}
			if stageResult.RefreshActionResult.RestartRequested ||
				stageResult.RefreshActionResult.RecompileGo {
				t.Fatalf(
					"expected zero refresh result from nil runner, got %#v",
					stageResult.RefreshActionResult,
				)
			}
		},
	)

	t.Run("runs stage actions and applies them to workset", func(t *testing.T) {
		work := &eventpipeline.WorkSet{}
		stageRunCount := 0
		stageResult := hooks.RunAndApplyHookStageActionsToWorkSet(
			hooks.HookStageTypeConcurrent,
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
		if len(stageResult.Actions) != 2 {
			t.Fatalf(
				"expected stage action count=2, got %#v",
				stageResult.Actions,
			)
		}
		if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload {
			t.Fatalf(
				"expected hard reload action applied, got %v",
				work.Browser.Action,
			)
		}
		if !work.Browser.WaitForVite {
			t.Fatal("expected wait-for-vite applied")
		}
	})
}

func TestRunAndApplyHookStageActionsAndErrorsToWorkSet(t *testing.T) {
	t.Run(
		"nil stage runner yields empty stage result metadata",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}
			stageResult := hooks.RunAndApplyHookStageActionsAndErrorsToWorkSet(
				hooks.HookStageTypePre,
				nil,
				work,
			)
			if stageResult.StageType != hooks.HookStageTypePre {
				t.Fatalf(
					"expected stageType=pre, got %#v",
					stageResult.StageType,
				)
			}
			if len(stageResult.Actions) != 0 {
				t.Fatalf(
					"expected no stage actions, got %#v",
					stageResult.Actions,
				)
			}
			if len(stageResult.ExecutionErrors) != 0 {
				t.Fatalf(
					"expected no stage execution errors, got %#v",
					stageResult.ExecutionErrors,
				)
			}
		},
	)

	t.Run(
		"runner stage actions and errors are captured and isolated",
		func(t *testing.T) {
			work := &eventpipeline.WorkSet{}
			stageResult := hooks.RunAndApplyHookStageActionsAndErrorsToWorkSet(
				hooks.HookStageTypeConcurrent,
				func() ([]wave.RefreshAction, []error) {
					return []wave.RefreshAction{
							{ReloadBrowser: true},
							{WaitForApp: true},
						},
						[]error{errSynthetic}
				},
				work,
			)

			if stageResult.StageType != hooks.HookStageTypeConcurrent {
				t.Fatalf(
					"expected stageType=concurrent, got %#v",
					stageResult.StageType,
				)
			}
			if len(stageResult.Actions) != 2 {
				t.Fatalf("expected 2 actions, got %#v", stageResult.Actions)
			}
			if len(stageResult.ExecutionErrors) != 1 {
				t.Fatalf(
					"expected 1 stage execution error, got %#v",
					stageResult.ExecutionErrors,
				)
			}
			if stageResult.ExecutionErrors[0] != errSynthetic {
				t.Fatalf(
					"expected errSynthetic execution error, got %#v",
					stageResult.ExecutionErrors[0],
				)
			}
			if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload {
				t.Fatalf(
					"expected hard reload to be applied, got %v",
					work.Browser.Action,
				)
			}
		},
	)
}

func TestDeriveEventsWithHooksForExecution(t *testing.T) {
	t.Run("batch hard reload uses copied hook contexts", func(t *testing.T) {
		firstHookContext := &wave.HookContext{FilePath: "first.txt"}
		secondHookContext := &wave.HookContext{FilePath: "second.txt"}
		eventsWithHooks := []eventpipeline.EventWithHooks{
			{HookCtx: firstHookContext},
			{HookCtx: nil},
			{HookCtx: secondHookContext},
		}

		executionEventsWithHooks := hooks.DeriveEventsWithHooksForExecution(
			eventsWithHooks,
			eventpipeline.AppStopStrategyBatchHardReload,
		)

		if len(executionEventsWithHooks) != len(eventsWithHooks) {
			t.Fatalf(
				"execution event count=%d, want %d",
				len(executionEventsWithHooks),
				len(eventsWithHooks),
			)
		}
		if &executionEventsWithHooks[0] == &eventsWithHooks[0] {
			t.Fatal(
				"expected batch hard-reload execution events to use a copied event slice",
			)
		}

		if executionEventsWithHooks[0].HookCtx == nil {
			t.Fatal("expected first execution hook context to be non-nil")
		}
		if executionEventsWithHooks[0].HookCtx == firstHookContext {
			t.Fatal("expected first execution hook context to be copied")
		}
		if !executionEventsWithHooks[0].HookCtx.AppStoppedForBatch {
			t.Fatal(
				"expected first execution hook context to set AppStoppedForBatch=true",
			)
		}
		if firstHookContext.AppStoppedForBatch {
			t.Fatal("expected original first hook context to remain unmodified")
		}

		if executionEventsWithHooks[1].HookCtx != nil {
			t.Fatalf(
				"expected nil hook context to remain nil, got %#v",
				executionEventsWithHooks[1].HookCtx,
			)
		}

		if executionEventsWithHooks[2].HookCtx == nil {
			t.Fatal("expected second execution hook context to be non-nil")
		}
		if executionEventsWithHooks[2].HookCtx == secondHookContext {
			t.Fatal("expected second execution hook context to be copied")
		}
		if !executionEventsWithHooks[2].HookCtx.AppStoppedForBatch {
			t.Fatal(
				"expected second execution hook context to set AppStoppedForBatch=true",
			)
		}
		if secondHookContext.AppStoppedForBatch {
			t.Fatal(
				"expected original second hook context to remain unmodified",
			)
		}
	})

	t.Run(
		"non-batch execution reuses original events slice",
		func(t *testing.T) {
			eventsWithHooks := []eventpipeline.EventWithHooks{
				{HookCtx: &wave.HookContext{}},
			}

			executionEventsWithHooks := hooks.DeriveEventsWithHooksForExecution(
				eventsWithHooks,
				eventpipeline.AppStopStrategyNone,
			)

			if len(executionEventsWithHooks) != len(eventsWithHooks) {
				t.Fatalf(
					"execution event count=%d, want %d",
					len(executionEventsWithHooks),
					len(eventsWithHooks),
				)
			}
			if &executionEventsWithHooks[0] != &eventsWithHooks[0] {
				t.Fatal(
					"expected non-batch execution to reuse original events slice",
				)
			}
		},
	)

	t.Run("empty execution events remain nil", func(t *testing.T) {
		executionEventsWithHooks := hooks.DeriveEventsWithHooksForExecution(
			nil,
			eventpipeline.AppStopStrategyBatchHardReload,
		)
		if executionEventsWithHooks != nil {
			t.Fatalf(
				"expected nil execution events for empty input, got %#v",
				executionEventsWithHooks,
			)
		}
	})
}
