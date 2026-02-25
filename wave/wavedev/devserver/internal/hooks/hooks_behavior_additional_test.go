package hooks_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/hooks"
)

func TestBuildNormalizedChangedFilePathsByWatchedPatternForHookContexts(
	t *testing.T,
) {
	root := t.TempDir()
	pattern := filepath.ToSlash(filepath.Join(root, "assets", "**", "*.css"))
	firstPath := filepath.Join(root, "assets", "first.css")
	secondPath := filepath.Join(root, "assets", "nested", "second.css")

	classifiedEvents := []eventpipeline.ClassifiedEvent{
		{
			Event: fsnotify.Event{Name: firstPath},
			WatchedFile: &wave.WatchedFile{
				Pattern: pattern,
			},
		},
		{
			Event: fsnotify.Event{Name: firstPath},
			WatchedFile: &wave.WatchedFile{
				Pattern: pattern,
			},
		},
		{
			Event: fsnotify.Event{Name: secondPath},
			WatchedFile: &wave.WatchedFile{
				Pattern: pattern,
			},
		},
		{
			Event: fsnotify.Event{Name: " "},
			WatchedFile: &wave.WatchedFile{
				Pattern: pattern,
			},
		},
		{
			Event:       fsnotify.Event{Name: firstPath},
			WatchedFile: nil,
		},
	}

	got := hooks.BuildNormalizedChangedFilePathsByWatchedPatternForHookContexts(
		classifiedEvents,
	)
	normalizedPattern := hooks.NormalizeHookContextPathShape(pattern)
	gotPaths := got[normalizedPattern]
	wantPaths := []string{
		hooks.NormalizeHookContextPathShape(firstPath),
		hooks.NormalizeHookContextPathShape(secondPath),
	}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("changed paths=%v, expected=%v", gotPaths, wantPaths)
	}
	if len(got) != 1 {
		t.Fatalf("map size=%d, expected=1", len(got))
	}
}

func TestBuildSkipDuplicateHooksByClassifiedEventIndex(t *testing.T) {
	root := t.TempDir()
	firstPattern := filepath.ToSlash(
		filepath.Join(root, "assets", "**", "*.css"),
	)
	secondPattern := filepath.ToSlash(
		filepath.Join(root, "scripts", "**", "*.go"),
	)

	classifiedEvents := []eventpipeline.ClassifiedEvent{
		{
			WatchedFile: &wave.WatchedFile{Pattern: firstPattern},
		},
		{
			WatchedFile: &wave.WatchedFile{Pattern: firstPattern},
		},
		{
			WatchedFile: &wave.WatchedFile{Pattern: secondPattern},
		},
		{
			WatchedFile: nil,
		},
		{
			WatchedFile: &wave.WatchedFile{Pattern: secondPattern},
		},
		{
			WatchedFile: &wave.WatchedFile{Pattern: " "},
		},
	}

	got := hooks.BuildSkipDuplicateHooksByClassifiedEventIndex(classifiedEvents)
	want := []bool{false, true, false, false, true, false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("skip duplicate result=%v, expected=%v", got, want)
	}
}

func TestBuildEventHooksForProcessing(t *testing.T) {
	root := t.TempDir()
	pattern := filepath.ToSlash(filepath.Join(root, "assets", "**", "*.css"))
	firstEventPath := filepath.Join(root, "assets", "first.css")
	secondEventPath := filepath.Join(root, "assets", "second.css")
	otherEventPath := filepath.Join(root, "notes.txt")

	watchedFile := &wave.WatchedFile{
		Pattern: pattern,
		OnChangeHooks: []wave.OnChangeHook{
			{
				Cmd:     "pre-hook",
				Exclude: []string{"*.tmp"},
				Timing:  wave.OnChangeStrategyPre,
			},
			{
				Cmd:    "post-hook",
				Timing: wave.OnChangeStrategyPost,
			},
		},
		RunOnChangeOnly: true,
	}

	classifiedEvents := []eventpipeline.ClassifiedEvent{
		{
			Event:       fsnotify.Event{Name: firstEventPath},
			FileType:    eventpipeline.FileTypeGo,
			WatchedFile: watchedFile,
		},
		{
			Event:       fsnotify.Event{Name: secondEventPath},
			FileType:    eventpipeline.FileTypePublicStatic,
			WatchedFile: watchedFile,
		},
		{
			Event:       fsnotify.Event{Name: otherEventPath},
			FileType:    eventpipeline.FileTypeOther,
			WatchedFile: nil,
		},
	}

	eventsWithHooks := hooks.BuildEventHooksForProcessing(classifiedEvents)
	if len(eventsWithHooks) != 3 {
		t.Fatalf("events with hooks len=%d, expected=3", len(eventsWithHooks))
	}
	if watchedFile.SortedHooks != nil {
		t.Fatal("expected watched file sorted hooks cache to remain unmodified")
	}

	first := eventsWithHooks[0]
	second := eventsWithHooks[1]
	third := eventsWithHooks[2]

	if first.SkipDuplicateHooks {
		t.Fatal("first event should not skip duplicate hooks")
	}
	if !second.SkipDuplicateHooks {
		t.Fatal("second event should skip duplicate hooks")
	}
	if !first.RunOnChangeOnly {
		t.Fatal("first event should keep run-on-change-only setting")
	}
	if !first.NeedsHardReload {
		t.Fatal("go event should require hard reload")
	}
	if second.NeedsHardReload {
		t.Fatal(
			"public-static event should not require hard reload when watcher file has no hard-reload flags",
		)
	}
	if third.Hooks == nil {
		t.Fatal("third event should include non-nil hooks container")
	}

	wantGroupedPaths := []string{
		hooks.NormalizeHookContextPathShape(firstEventPath),
		hooks.NormalizeHookContextPathShape(secondEventPath),
	}
	if !reflect.DeepEqual(first.HookCtx.ChangedFilePaths, wantGroupedPaths) {
		t.Fatalf(
			"grouped changed paths=%v, expected=%v",
			first.HookCtx.ChangedFilePaths,
			wantGroupedPaths,
		)
	}
	wantThirdPaths := []string{
		hooks.NormalizeHookContextPathShape(otherEventPath),
	}
	if !reflect.DeepEqual(third.HookCtx.ChangedFilePaths, wantThirdPaths) {
		t.Fatalf(
			"third changed paths=%v, expected=%v",
			third.HookCtx.ChangedFilePaths,
			wantThirdPaths,
		)
	}

	if first.Hooks == nil || len(first.Hooks.Pre) != 1 {
		t.Fatalf("first hooks pre=%v, expected one hook", first.Hooks)
	}
	first.Hooks.Pre[0].Cmd = "mutated-cmd"
	first.Hooks.Pre[0].Exclude[0] = "*.bak"
	if watchedFile.OnChangeHooks[0].Cmd != "pre-hook" {
		t.Fatalf(
			"source watched file hook cmd was mutated to %q",
			watchedFile.OnChangeHooks[0].Cmd,
		)
	}
	if watchedFile.OnChangeHooks[0].Exclude[0] != "*.tmp" {
		t.Fatalf(
			"source watched file hook exclude was mutated to %q",
			watchedFile.OnChangeHooks[0].Exclude[0],
		)
	}
}

func TestDeriveEventsWithHooksForExecution(t *testing.T) {
	originalEventsWithHooks := []eventpipeline.EventWithHooks{
		{
			HookCtx: &wave.HookContext{
				FilePath:         "/tmp/a.go",
				ChangedFilePaths: []string{"/tmp/a.go"},
			},
		},
		{
			HookCtx: &wave.HookContext{
				FilePath:         "/tmp/b.go",
				ChangedFilePaths: []string{"/tmp/b.go"},
			},
		},
	}

	singleEvent := hooks.DeriveEventsWithHooksForExecution(
		originalEventsWithHooks,
		eventpipeline.AppStopStrategySingleEventHardReload,
	)
	if len(singleEvent) != 1 {
		t.Fatalf("single-event strategy len=%d, expected=1", len(singleEvent))
	}
	if singleEvent[0].HookCtx.FilePath != "/tmp/a.go" {
		t.Fatalf(
			"single-event hook path=%q, expected=/tmp/a.go",
			singleEvent[0].HookCtx.FilePath,
		)
	}

	batchEvents := hooks.DeriveEventsWithHooksForExecution(
		originalEventsWithHooks,
		eventpipeline.AppStopStrategyBatchHardReload,
	)
	if len(batchEvents) != 2 {
		t.Fatalf("batch strategy len=%d, expected=2", len(batchEvents))
	}
	if !batchEvents[0].HookCtx.AppStoppedForBatch ||
		!batchEvents[1].HookCtx.AppStoppedForBatch {
		t.Fatal(
			"batch strategy should set AppStoppedForBatch on all hook contexts",
		)
	}
	batchEvents[0].HookCtx.ChangedFilePaths[0] = "mutated"
	if originalEventsWithHooks[0].HookCtx.ChangedFilePaths[0] != "/tmp/a.go" {
		t.Fatalf(
			"original hook context was mutated to %q",
			originalEventsWithHooks[0].HookCtx.ChangedFilePaths[0],
		)
	}

	defaultEvents := hooks.DeriveEventsWithHooksForExecution(
		originalEventsWithHooks,
		eventpipeline.AppStopStrategyNone,
	)
	if !reflect.DeepEqual(defaultEvents, originalEventsWithHooks) {
		t.Fatalf(
			"default strategy=%v, expected=%v",
			defaultEvents,
			originalEventsWithHooks,
		)
	}
}

func TestDeriveImplicitBuildExecutionDecision(t *testing.T) {
	noImplicitSingle := hooks.DeriveImplicitBuildExecutionDecision(false, 1)
	if noImplicitSingle.ShouldRunImplicitBuild {
		t.Fatal(
			"expected implicit build disabled for single run-on-change-only event",
		)
	}
	if noImplicitSingle.SkipImplicitBuildLogEntry !=
		"RunOnChangeOnly: skipping implicit build phase" {
		t.Fatalf(
			"skip log=%q, expected single-event skip log",
			noImplicitSingle.SkipImplicitBuildLogEntry,
		)
	}

	noImplicitBatch := hooks.DeriveImplicitBuildExecutionDecision(false, 2)
	if noImplicitBatch.ShouldRunImplicitBuild {
		t.Fatal(
			"expected implicit build disabled for all run-on-change-only events",
		)
	}
	if noImplicitBatch.SkipImplicitBuildLogEntry !=
		"All events are RunOnChangeOnly, skipping implicit build phase" {
		t.Fatalf(
			"skip log=%q, expected batch skip log",
			noImplicitBatch.SkipImplicitBuildLogEntry,
		)
	}

	implicitWithZeroEvents := hooks.DeriveImplicitBuildExecutionDecision(
		true,
		0,
	)
	if implicitWithZeroEvents.ShouldRunImplicitBuild {
		t.Fatal("expected implicit build disabled for zero events")
	}

	implicitWithEvents := hooks.DeriveImplicitBuildExecutionDecision(true, 2)
	if !implicitWithEvents.ShouldRunImplicitBuild {
		t.Fatal("expected implicit build enabled with non-zero events")
	}
}

func TestHookStageContinuationAndShortCircuitPolicies(t *testing.T) {
	restartStageResult := hooks.HookStageResult{
		RefreshActionResult: eventpipeline.RefreshActionApplicationResult{
			RestartRequested: true,
			RecompileGo:      true,
		},
	}
	if !hooks.ShouldShortCircuitPipelineForHookStageResult(
		restartStageResult,
		hooks.HookStageFailurePolicyContinue,
	) {
		t.Fatal("expected short-circuit when restart is requested")
	}

	stageFailureResult := hooks.HookStageResult{
		ExecutionErrors: []error{errors.New("hook failure")},
	}
	if !hooks.ShouldShortCircuitPipelineForHookStageResult(
		stageFailureResult,
		hooks.HookStageFailurePolicyStop,
	) {
		t.Fatal(
			"expected short-circuit when stage failure policy is stop and errors exist",
		)
	}
	if hooks.ShouldShortCircuitPipelineForHookStageResult(
		stageFailureResult,
		hooks.HookStageFailurePolicyContinue,
	) {
		t.Fatal("did not expect short-circuit when failure policy is continue")
	}

	restartDecision := hooks.DeriveHookStageContinuationDecisionWithFailurePolicy(
		restartStageResult,
		hooks.HookStageFailurePolicyStop,
	)
	if restartDecision.ShouldContinue {
		t.Fatal("expected restart continuation decision to stop pipeline")
	}
	if restartDecision.StopReason != hooks.HookStageContinuationStopReasonRestartRequested {
		t.Fatalf(
			"stop reason=%v, expected restart requested",
			restartDecision.StopReason,
		)
	}

	failureDecision := hooks.DeriveHookStageContinuationDecisionWithFailurePolicy(
		stageFailureResult,
		hooks.HookStageFailurePolicyStop,
	)
	if failureDecision.ShouldContinue {
		t.Fatal("expected stop decision for stage failure under stop policy")
	}
	if failureDecision.StopReason != hooks.HookStageContinuationStopReasonStageFailure {
		t.Fatalf(
			"stop reason=%v, expected stage failure",
			failureDecision.StopReason,
		)
	}

	continueDecision := hooks.DeriveHookStageContinuationDecision(
		stageFailureResult,
	)
	if !continueDecision.ShouldContinue {
		t.Fatal(
			"expected default continuation policy to continue after hook errors",
		)
	}
	if continueDecision.StopReason != hooks.HookStageContinuationStopReasonNone {
		t.Fatalf("stop reason=%v, expected none", continueDecision.StopReason)
	}
}

func TestHookStageFailurePolicyParsing(t *testing.T) {
	testCases := []struct {
		configuredValue string
		expectedPolicy  hooks.HookStageFailurePolicy
	}{
		{
			configuredValue: "continue",
			expectedPolicy:  hooks.HookStageFailurePolicyContinue,
		},
		{
			configuredValue: "fail-open",
			expectedPolicy:  hooks.HookStageFailurePolicyContinue,
		},
		{
			configuredValue: "failclosed",
			expectedPolicy:  hooks.HookStageFailurePolicyStop,
		},
		{
			configuredValue: "strict",
			expectedPolicy:  hooks.HookStageFailurePolicyStop,
		},
		{
			configuredValue: "unknown",
			expectedPolicy:  hooks.HookStageFailurePolicyContinue,
		},
	}

	for _, testCase := range testCases {
		got := hooks.DeriveHookStageFailurePolicyFromConfiguredValue(
			testCase.configuredValue,
		)
		if got != testCase.expectedPolicy {
			t.Fatalf(
				"configured value=%q policy=%v, expected=%v",
				testCase.configuredValue,
				got,
				testCase.expectedPolicy,
			)
		}
	}
}

func TestHookExecutionGateHelpers(t *testing.T) {
	if hooks.ShouldStartAppAfterImplicitBuild(
		false,
		eventpipeline.RestartPhaseDecision{RestartApp: true},
	) {
		t.Fatal("did not expect app start when implicit build is disabled")
	}
	if !hooks.ShouldStartAppAfterImplicitBuild(
		true,
		eventpipeline.RestartPhaseDecision{RestartApp: true},
	) {
		t.Fatal(
			"expected app start when implicit build runs and restart decision is true",
		)
	}

	noRestartResult := hooks.HookStageResult{}
	restartResult := hooks.HookStageResult{
		RefreshActionResult: eventpipeline.RefreshActionApplicationResult{
			RestartRequested: true,
		},
	}
	if !hooks.ShouldExecuteBrowserPhaseAfterHookStageResults(
		noRestartResult,
		noRestartResult,
		noRestartResult,
	) {
		t.Fatal(
			"expected browser phase execution when no hook stage requested restart",
		)
	}
	if hooks.ShouldExecuteBrowserPhaseAfterHookStageResults(
		noRestartResult,
		restartResult,
		noRestartResult,
	) {
		t.Fatal(
			"did not expect browser phase execution when any stage requested restart",
		)
	}
}

func TestHookContextAndErrorUtilities(t *testing.T) {
	if hooks.DeriveHookStageLabel(hooks.HookStageTypePre) != "pre" {
		t.Fatal("expected pre stage label")
	}
	if hooks.DeriveHookStageLabel(hooks.HookStageType(999)) != "unknown" {
		t.Fatal("expected unknown stage label for unsupported stage")
	}

	var nilExecutionContext context.Context
	backgroundExecutionContext := hooks.DeriveHookExecutionContext(
		nilExecutionContext,
	)
	if backgroundExecutionContext == nil {
		t.Fatal("expected non-nil background context fallback")
	}

	parentExecutionContext, cancelParentExecutionContext := context.WithCancel(
		context.Background(),
	)
	defer cancelParentExecutionContext()

	originalHookContext := &wave.HookContext{
		ExecutionContext:   context.TODO(),
		FilePath:           "/tmp/source.go",
		ChangedFilePaths:   []string{"/tmp/source.go"},
		AppStoppedForBatch: true,
	}
	clonedHookContext := hooks.CloneHookContextForExecution(
		originalHookContext,
		parentExecutionContext,
	)
	if clonedHookContext.ExecutionContext != parentExecutionContext {
		t.Fatal("expected clone execution context override")
	}
	clonedHookContext.ChangedFilePaths[0] = "mutated"
	if originalHookContext.ChangedFilePaths[0] != "/tmp/source.go" {
		t.Fatalf(
			"original hook context changed paths were mutated to %q",
			originalHookContext.ChangedFilePaths[0],
		)
	}

	clonedNilHookContext := hooks.CloneHookContextForExecution(nil, nil)
	if clonedNilHookContext == nil ||
		clonedNilHookContext.ExecutionContext == nil {
		t.Fatal("expected non-nil cloned hook context for nil input")
	}

	wrappedWithPath := hooks.WrapHookExecutionErrorWithStageAndPath(
		hooks.HookStageTypePost,
		"/tmp/main.go",
		errors.New("boom"),
	)
	if wrappedWithPath == nil || !strings.Contains(
		wrappedWithPath.Error(),
		"post hook failed for /tmp/main.go: boom",
	) {
		t.Fatalf("wrapped error with path=%v", wrappedWithPath)
	}

	wrappedWithoutPath := hooks.WrapHookExecutionErrorWithStageAndPath(
		hooks.HookStageTypePre,
		" ",
		errors.New("boom"),
	)
	if wrappedWithoutPath == nil || !strings.Contains(
		wrappedWithoutPath.Error(),
		"pre hook failed: boom",
	) {
		t.Fatalf("wrapped error without path=%v", wrappedWithoutPath)
	}

	if hooks.WrapHookExecutionErrorWithStageAndPath(
		hooks.HookStageTypePre,
		"/tmp/main.go",
		nil,
	) != nil {
		t.Fatal("expected nil wrapped error when source error is nil")
	}

	joined := hooks.JoinHookExecutionErrorsInOrder(
		[]error{errors.New("first"), nil, errors.New("second")},
	)
	if joined == nil {
		t.Fatal("expected joined error for non-empty input")
	}
	joinedErrorText := joined.Error()
	if !strings.Contains(joinedErrorText, "first") ||
		!strings.Contains(joinedErrorText, "second") {
		t.Fatalf("joined error text=%q", joinedErrorText)
	}
	if hooks.JoinHookExecutionErrorsInOrder(nil) != nil {
		t.Fatal("expected nil joined error for empty input")
	}
}

func TestHookCallbackAndContextExecutionUtilities(t *testing.T) {
	if action, callbackError := hooks.ExecuteHookCallbackSafely(nil, nil); action != nil ||
		callbackError != nil {
		t.Fatalf(
			"nil callback action=%v error=%v, expected nil,nil",
			action,
			callbackError,
		)
	}

	okAction, okError := hooks.ExecuteHookCallbackSafely(
		func(_ *wave.HookContext) (*wave.RefreshAction, error) {
			return &wave.RefreshAction{ReloadBrowser: true}, nil
		},
		&wave.HookContext{},
	)
	if okError != nil {
		t.Fatalf("expected nil callback error, got %v", okError)
	}
	if okAction == nil || !okAction.ReloadBrowser {
		t.Fatalf("callback action=%v, expected reload browser action", okAction)
	}

	panicAction, panicError := hooks.ExecuteHookCallbackSafely(
		func(_ *wave.HookContext) (*wave.RefreshAction, error) {
			panic("kaboom")
		},
		&wave.HookContext{},
	)
	if panicAction != nil {
		t.Fatalf("panic callback action=%v, expected nil", panicAction)
	}
	if panicError == nil || !strings.Contains(panicError.Error(), "panicked") {
		t.Fatalf(
			"panic callback error=%v, expected panic conversion error",
			panicError,
		)
	}

	var nilConcurrentExecutionContext context.Context
	if !hooks.ShouldContinueConcurrentHookExecution(
		nilConcurrentExecutionContext,
	) {
		t.Fatal("expected nil concurrent context to allow continuation")
	}

	activeConcurrentContext := context.Background()
	if !hooks.ShouldContinueConcurrentHookExecution(activeConcurrentContext) {
		t.Fatal("expected active context to allow continuation")
	}

	canceledConcurrentContext, cancelConcurrentContext := context.WithCancel(
		context.Background(),
	)
	cancelConcurrentContext()
	if hooks.ShouldContinueConcurrentHookExecution(canceledConcurrentContext) {
		t.Fatal("did not expect continuation for canceled concurrent context")
	}

	var nilExecutionContextForError context.Context
	if hooks.DeriveHookExecutionContextError(
		nilExecutionContextForError,
	) != nil {
		t.Fatal("expected nil context error for nil context")
	}
	if hooks.DeriveHookExecutionContextError(context.Background()) != nil {
		t.Fatal("expected nil context error for active context")
	}
	if contextError := hooks.DeriveHookExecutionContextError(
		canceledConcurrentContext,
	); !errors.Is(contextError, context.Canceled) {
		t.Fatalf("context error=%v, expected context.Canceled", contextError)
	}

	if commandError := hooks.ExecuteHookCommandWithContext(
		context.Background(),
		" \t ",
	); commandError != nil {
		t.Fatalf("blank command error=%v, expected nil", commandError)
	}

	canceledCommandContext, cancelCommandContext := context.WithCancel(
		context.Background(),
	)
	cancelCommandContext()
	commandError := hooks.ExecuteHookCommandWithContext(
		canceledCommandContext,
		"echo hook",
	)
	if commandError == nil {
		t.Fatal("expected command execution error for canceled context")
	}
	if !strings.Contains(commandError.Error(), "execute hook command") {
		t.Fatalf(
			"command error=%v, expected wrapped execution error",
			commandError,
		)
	}
}

func TestActionOrderingAndStageActionApplication(t *testing.T) {
	actions := []wave.RefreshAction{
		{TriggerRestart: true, RecompileGo: true},
		{ReloadBrowser: true},
		{TriggerRestart: true},
		{WaitForApp: true},
	}
	hooks.SortActionsByRestartPriority(actions)
	if actions[0].TriggerRestart || actions[1].TriggerRestart {
		t.Fatalf("restart actions should sort to end, got=%v", actions)
	}
	if !actions[2].TriggerRestart || !actions[3].TriggerRestart {
		t.Fatalf("restart actions should be in tail positions, got=%v", actions)
	}
	if !actions[0].ReloadBrowser || !actions[1].WaitForApp {
		t.Fatalf(
			"non-restart action relative order should be preserved, got=%v",
			actions,
		)
	}

	if applied := hooks.ApplyHookStageActionsToWorkSet(nil, nil); applied != (eventpipeline.RefreshActionApplicationResult{}) {
		t.Fatalf("nil work apply result=%v, expected zero value", applied)
	}

	work := &eventpipeline.WorkSet{}
	applied := hooks.ApplyHookStageActionsToWorkSet(
		work,
		[]wave.RefreshAction{{ReloadBrowser: true, WaitForApp: true}},
	)
	if applied.RestartRequested {
		t.Fatalf(
			"restart requested=%v, expected false",
			applied.RestartRequested,
		)
	}
	if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
		!work.Browser.WaitForApp {
		t.Fatalf(
			"work browser decision=%+v, expected hard reload with wait-for-app",
			work.Browser,
		)
	}

	stageResult := hooks.RunAndApplyHookStageActionsToWorkSet(
		hooks.HookStageTypeConcurrent,
		func() []wave.RefreshAction {
			return []wave.RefreshAction{
				{TriggerRestart: true, RecompileGo: true},
			}
		},
		&eventpipeline.WorkSet{},
	)
	if stageResult.StageType != hooks.HookStageTypeConcurrent {
		t.Fatalf("stage type=%v, expected concurrent", stageResult.StageType)
	}
	if !stageResult.RefreshActionResult.RestartRequested ||
		!stageResult.RefreshActionResult.RecompileGo {
		t.Fatalf(
			"stage refresh result=%+v, expected restart+recompile",
			stageResult.RefreshActionResult,
		)
	}

	nilStageResult := hooks.RunAndApplyHookStageActionsToWorkSet(
		hooks.HookStageTypePost,
		nil,
		nil,
	)
	if nilStageResult.StageType != hooks.HookStageTypePost {
		t.Fatalf("nil stage type=%v, expected post", nilStageResult.StageType)
	}
	if len(nilStageResult.Actions) != 0 ||
		len(nilStageResult.ExecutionErrors) != 0 {
		t.Fatalf(
			"expected empty stage result for nil runner, got=%+v",
			nilStageResult,
		)
	}

	actionAndErrorResult := hooks.RunAndApplyHookStageActionsAndErrorsToWorkSet(
		hooks.HookStageTypePre,
		func() ([]wave.RefreshAction, []error) {
			return []wave.RefreshAction{
					{ReloadBrowser: true},
				}, []error{
					errors.New("hook error"),
				}
		},
		&eventpipeline.WorkSet{},
	)
	if actionAndErrorResult.StageType != hooks.HookStageTypePre {
		t.Fatalf("stage type=%v, expected pre", actionAndErrorResult.StageType)
	}
	if len(actionAndErrorResult.Actions) != 1 ||
		len(actionAndErrorResult.ExecutionErrors) != 1 {
		t.Fatalf(
			"stage result=%+v, expected one action and one error",
			actionAndErrorResult,
		)
	}
}

func TestHookTimeoutResolutionPolicies_HooksPackage(t *testing.T) {
	watchConfig := &wave.WatchConfig{
		HookCommandTimeouts: wave.HookCommandTimeoutConfig{
			PreCommandTimeoutMilliseconds:              111,
			ConcurrentCommandTimeoutMilliseconds:       222,
			ConcurrentNoWaitCommandTimeoutMilliseconds: 333,
			PostCommandTimeoutMilliseconds:             444,
		},
		HookCallbackTimeouts: wave.HookCallbackTimeoutConfig{
			PreCallbackTimeoutMilliseconds:              555,
			ConcurrentCallbackTimeoutMilliseconds:       666,
			ConcurrentNoWaitCallbackTimeoutMilliseconds: 777,
			PostCallbackTimeoutMilliseconds:             888,
		},
	}

	if got := hooks.DeriveHookCommandStageTimeoutMilliseconds(
		watchConfig,
		hooks.HookStageTypePre,
	); got != 111 {
		t.Fatalf("pre command timeout=%d, expected=111", got)
	}
	if got := hooks.DeriveHookCommandStageTimeoutMilliseconds(
		watchConfig,
		hooks.HookStageTypeConcurrent,
	); got != 222 {
		t.Fatalf("concurrent command timeout=%d, expected=222", got)
	}
	if got := hooks.DeriveHookCommandStageTimeoutMilliseconds(
		watchConfig,
		hooks.HookStageTypeConcurrentNoWait,
	); got != 333 {
		t.Fatalf("concurrent-no-wait command timeout=%d, expected=333", got)
	}
	if got := hooks.DeriveHookCommandStageTimeoutMilliseconds(
		watchConfig,
		hooks.HookStageTypePost,
	); got != 444 {
		t.Fatalf("post command timeout=%d, expected=444", got)
	}
	if got := hooks.DeriveHookCommandStageTimeoutMilliseconds(
		nil,
		hooks.HookStageTypePre,
	); got != 0 {
		t.Fatalf("nil command timeout=%d, expected=0", got)
	}

	if got := hooks.DeriveHookCallbackStageTimeoutMilliseconds(
		watchConfig,
		hooks.HookStageTypePre,
	); got != 555 {
		t.Fatalf("pre callback timeout=%d, expected=555", got)
	}
	if got := hooks.DeriveHookCallbackStageTimeoutMilliseconds(
		watchConfig,
		hooks.HookStageTypeConcurrent,
	); got != 666 {
		t.Fatalf("concurrent callback timeout=%d, expected=666", got)
	}
	if got := hooks.DeriveHookCallbackStageTimeoutMilliseconds(
		watchConfig,
		hooks.HookStageTypeConcurrentNoWait,
	); got != 777 {
		t.Fatalf("concurrent-no-wait callback timeout=%d, expected=777", got)
	}
	if got := hooks.DeriveHookCallbackStageTimeoutMilliseconds(
		watchConfig,
		hooks.HookStageTypePost,
	); got != 888 {
		t.Fatalf("post callback timeout=%d, expected=888", got)
	}
	if got := hooks.DeriveHookCallbackStageTimeoutMilliseconds(
		nil,
		hooks.HookStageTypePre,
	); got != 0 {
		t.Fatalf("nil callback timeout=%d, expected=0", got)
	}

	if got := hooks.DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		0,
		1200,
		false,
	); got != 1200*time.Millisecond {
		t.Fatalf("stage timeout duration=%s, expected=1200ms", got)
	}
	if got := hooks.DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		1200,
		250,
		false,
	); got != 250*time.Millisecond {
		t.Fatalf("override timeout duration=%s, expected=250ms", got)
	}
	if got := hooks.DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		1200,
		250,
		true,
	); got != 0 {
		t.Fatalf("override-with-disable timeout duration=%s, expected=0", got)
	}
	if got := hooks.DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		0,
		1200,
		true,
	); got != 0 {
		t.Fatalf("disabled stage timeout duration=%s, expected=0", got)
	}
	if got := hooks.DeriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		0,
		-10,
		false,
	); got != 0 {
		t.Fatalf("negative stage timeout duration=%s, expected=0", got)
	}

	commandTimeoutDuration := hooks.DeriveHookCommandTimeoutDurationForExecutionPlan(
		watchConfig,
		hooks.HookStageTypePre,
		hooks.HookExecutionPlan{},
	)
	if commandTimeoutDuration != 111*time.Millisecond {
		t.Fatalf(
			"command timeout duration=%s, expected=111ms",
			commandTimeoutDuration,
		)
	}

	commandOverrideTimeoutDuration := hooks.DeriveHookCommandTimeoutDurationForExecutionPlan(
		watchConfig,
		hooks.HookStageTypePre,
		hooks.HookExecutionPlan{
			CommandTimeoutMilliseconds: 99,
			DisableStageCommandTimeout: true,
		},
	)
	if commandOverrideTimeoutDuration != 0 {
		t.Fatalf(
			"command override timeout duration=%s, expected=0",
			commandOverrideTimeoutDuration,
		)
	}

	callbackTimeoutDuration := hooks.DeriveHookCallbackTimeoutDurationForExecutionPlan(
		watchConfig,
		hooks.HookStageTypePost,
		hooks.HookExecutionPlan{},
	)
	if callbackTimeoutDuration != 888*time.Millisecond {
		t.Fatalf(
			"callback timeout duration=%s, expected=888ms",
			callbackTimeoutDuration,
		)
	}

	callbackOverrideTimeoutDuration := hooks.DeriveHookCallbackTimeoutDurationForExecutionPlan(
		watchConfig,
		hooks.HookStageTypeConcurrent,
		hooks.HookExecutionPlan{
			CallbackTimeoutMilliseconds: 77,
			DisableStageCallbackTimeout: true,
		},
	)
	if callbackOverrideTimeoutDuration != 0 {
		t.Fatalf(
			"callback override timeout duration=%s, expected=0",
			callbackOverrideTimeoutDuration,
		)
	}

	coreConfig := &wave.CoreConfig{
		DevBuildHookTimeoutMilliseconds:  333,
		ProdBuildHookTimeoutMilliseconds: 444,
	}
	if got := hooks.DeriveBuildHookCommandTimeoutDuration(coreConfig, true); got != 333*time.Millisecond {
		t.Fatalf("dev build timeout duration=%s, expected=333ms", got)
	}
	if got := hooks.DeriveBuildHookCommandTimeoutDuration(coreConfig, false); got != 444*time.Millisecond {
		t.Fatalf("prod build timeout duration=%s, expected=444ms", got)
	}
	if got := hooks.DeriveBuildHookCommandTimeoutDuration(nil, false); got != 0 {
		t.Fatalf("nil core config timeout duration=%s, expected=0", got)
	}
}

func TestDeriveExecutionContextWithOptionalTimeout_HooksPackage(t *testing.T) {
	parentExecutionContext := context.Background()

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

	timeoutExecutionContext, cancelTimeoutExecutionContext := hooks.DeriveExecutionContextWithOptionalTimeout(
		parentExecutionContext,
		10*time.Millisecond,
	)
	if timeoutExecutionContext == nil || cancelTimeoutExecutionContext == nil {
		t.Fatal(
			"expected timeout context and cancel function when timeout is enabled",
		)
	}
	cancelTimeoutExecutionContext()
}
