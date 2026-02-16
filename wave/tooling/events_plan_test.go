package tooling

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/pathnorm"
)

func TestShouldShowRebuildingOverlay(t *testing.T) {
	t.Run("returns false for CSS only changes", func(t *testing.T) {
		classifiedEvents := []classifiedEvent{
			{fileType: fileTypeCriticalCSS},
			{fileType: fileTypeNormalCSS},
			{fileType: fileTypeCriticalAndNormalCSS},
		}

		if shouldShowRebuildingOverlay(classifiedEvents) {
			t.Fatal("expected no rebuilding overlay for CSS-only changes")
		}
	})

	t.Run("returns false when non-css changes skip rebuilding notification", func(t *testing.T) {
		classifiedEvents := []classifiedEvent{
			{
				fileType: fileTypeOther,
				watchedFile: &wave.WatchedFile{
					SkipRebuildingNotification: true,
				},
			},
		}

		if shouldShowRebuildingOverlay(classifiedEvents) {
			t.Fatal("expected rebuilding overlay to be skipped")
		}
	})

	t.Run("returns false when non-go change only runs client revalidate", func(t *testing.T) {
		classifiedEvents := []classifiedEvent{
			{
				fileType: fileTypePrivateStatic,
				watchedFile: &wave.WatchedFile{
					OnlyRunClientDefinedRevalidateFunc: true,
				},
			},
		}

		if shouldShowRebuildingOverlay(classifiedEvents) {
			t.Fatal("expected rebuilding overlay to be skipped for revalidate-only change")
		}
	})

	t.Run("returns true for go change even when client revalidate is requested", func(t *testing.T) {
		classifiedEvents := []classifiedEvent{
			{
				fileType: fileTypeGo,
				watchedFile: &wave.WatchedFile{
					OnlyRunClientDefinedRevalidateFunc: true,
				},
			},
		}

		if !shouldShowRebuildingOverlay(classifiedEvents) {
			t.Fatal("expected rebuilding overlay for go change")
		}
	})

	t.Run("returns true when non-css change does not skip rebuilding notification", func(t *testing.T) {
		classifiedEvents := []classifiedEvent{
			{fileType: fileTypeOther},
		}

		if !shouldShowRebuildingOverlay(classifiedEvents) {
			t.Fatal("expected rebuilding overlay for non-css change")
		}
	})
}

func TestBuildEventExecutionPlan_ConfigChangeHasNoPlan(t *testing.T) {
	runConfigMutationAndPathShapeMatrix(
		t,
		func(
			t *testing.T,
			configMutationCaseForRun configMutationCase,
			pathShapeCaseForRun configEventPathShapeCase,
		) {
			cfg, _, configFilePath := setupConfigEventTestConfig(t)
			if configMutationCaseForRun.prepareEvent != nil {
				configMutationCaseForRun.prepareEvent(t, configFilePath)
			}

			watcher, builder := setupWatcherAndBuilderForToolingTests(t, cfg)

			serverForTest := &server{
				cfg: cfg,
				log: newDiscardLogger(),
			}

			configEventPath := pathShapeCaseForRun.buildPath(t, configFilePath)
			executionPlanningResult := serverForTest.buildEventExecutionPlan(
				[]fsnotify.Event{
					{
						Name: configEventPath,
						Op:   configMutationCaseForRun.op,
					},
				},
				watcher,
				builder,
			)

			if !executionPlanningResult.configChanged {
				t.Fatalf(
					"expected configChanged=true for config file %s/%s",
					configMutationCaseForRun.name,
					pathShapeCaseForRun.name,
				)
			}
			if executionPlanningResult.eventsWithHooks != nil {
				t.Fatalf(
					"expected no eventsWithHooks when config changed, got %#v",
					executionPlanningResult.eventsWithHooks,
				)
			}
		},
	)
}

func TestBuildEventExecutionPlan_BatchPlanIncludesHookBatchContext(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern: "**/*.txt",
		},
	}

	fileA := filepath.Join(root, "a.txt")
	fileB := filepath.Join(root, "b.txt")
	if err := os.WriteFile(fileA, []byte("a"), 0644); err != nil {
		t.Fatalf("failed writing %s: %v", fileA, err)
	}
	if err := os.WriteFile(fileB, []byte("b"), 0644); err != nil {
		t.Fatalf("failed writing %s: %v", fileB, err)
	}

	watcher, err := newWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	serverForTest := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	executionPlanningResult := serverForTest.buildEventExecutionPlan(
		[]fsnotify.Event{
			{Name: fileA, Op: fsnotify.Write},
			{Name: fileB, Op: fsnotify.Write},
		},
		watcher,
		builder,
	)

	if executionPlanningResult.configChanged {
		t.Fatal("expected configChanged=false for normal file batch")
	}
	if len(executionPlanningResult.eventsWithHooks) == 0 {
		t.Fatal("expected eventsWithHooks for batch change")
	}

	eventsWithHooks := executionPlanningResult.eventsWithHooks
	if len(eventsWithHooks) <= 1 {
		t.Fatal("expected isBatch=true")
	}
	behavioralDecision := deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		eventsWithHooks,
	)
	if behavioralDecision.appStopStrategy != appStopStrategyNone {
		t.Fatal("expected batchNeedsAppStop=false for txt-only batch")
	}
	if !behavioralDecision.runImplicitBuild {
		t.Fatal("expected runImplicitBuild=true for non-run-on-change-only batch")
	}
	if !behavioralDecision.showRebuildingOverlay {
		t.Fatal("expected showRebuildingOverlay=true for txt batch")
	}
	if len(eventsWithHooks) != 2 {
		t.Fatalf("expected 2 eventsWithHooks, got %d", len(eventsWithHooks))
	}

	firstHookContextPaths := eventsWithHooks[0].hookCtx.ChangedFilePaths
	secondHookContextPaths := eventsWithHooks[1].hookCtx.ChangedFilePaths
	if len(firstHookContextPaths) != 2 {
		t.Fatalf("expected first hook context to include 2 changed paths, got %#v", firstHookContextPaths)
	}
	if len(secondHookContextPaths) != 2 {
		t.Fatalf("expected second hook context to include 2 changed paths, got %#v", secondHookContextPaths)
	}

	if eventsWithHooks[0].skipDuplicateHooks {
		t.Fatal("expected first matched pattern hook set not to be deduplicated")
	}
	if !eventsWithHooks[1].skipDuplicateHooks {
		t.Fatal("expected second matched pattern hook set to be deduplicated")
	}
}

func TestBuildEventExecutionPlan_MixedFileClassesAndHookShapes(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*.txt",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return nil, nil
					},
				},
			},
		},
	}

	goFilePath := filepath.Join(root, "backend", "main.go")
	textFilePathA := filepath.Join(root, "a.txt")
	textFilePathB := filepath.Join(root, "b.txt")
	publicStaticPath := filepath.Join(cfg.Core.StaticAssetDirs.Public, "logo.svg")

	if err := os.MkdirAll(filepath.Dir(goFilePath), 0o755); err != nil {
		t.Fatalf("failed creating go file dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(publicStaticPath), 0o755); err != nil {
		t.Fatalf("failed creating public static dir: %v", err)
	}
	if err := os.WriteFile(goFilePath, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed writing go file: %v", err)
	}
	if err := os.WriteFile(textFilePathA, []byte("a"), 0o644); err != nil {
		t.Fatalf("failed writing first text file: %v", err)
	}
	if err := os.WriteFile(textFilePathB, []byte("b"), 0o644); err != nil {
		t.Fatalf("failed writing second text file: %v", err)
	}
	if err := os.WriteFile(publicStaticPath, []byte("logo"), 0o644); err != nil {
		t.Fatalf("failed writing public static file: %v", err)
	}

	watcher, err := newWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	serverForTest := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	executionPlanningResult := serverForTest.buildEventExecutionPlan(
		[]fsnotify.Event{
			{Name: goFilePath, Op: fsnotify.Write},
			{Name: textFilePathA, Op: fsnotify.Write},
			{Name: textFilePathB, Op: fsnotify.Write},
			{Name: publicStaticPath, Op: fsnotify.Write},
		},
		watcher,
		builder,
	)

	if executionPlanningResult.configChanged {
		t.Fatal("expected configChanged=false for non-config mixed batch")
	}
	if len(executionPlanningResult.eventsWithHooks) == 0 {
		t.Fatal("expected eventsWithHooks for mixed batch")
	}

	eventsWithHooks := executionPlanningResult.eventsWithHooks
	behavioralDecision := deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		eventsWithHooks,
	)
	if behavioralDecision.appStopStrategy != appStopStrategyBatchHardReload {
		t.Fatalf(
			"expected batch hard reload app stop strategy, got %v",
			behavioralDecision.appStopStrategy,
		)
	}
	if !behavioralDecision.runImplicitBuild {
		t.Fatal("expected runImplicitBuild=true for mixed batch with implicit work")
	}
	if !behavioralDecision.showRebuildingOverlay {
		t.Fatal("expected showRebuildingOverlay=true for mixed non-css batch")
	}
	if len(eventsWithHooks) != 4 {
		t.Fatalf("expected 4 eventsWithHooks, got %d", len(eventsWithHooks))
	}

	var matchedTextPatternCount int
	var deduplicatedTextPatternCount int
	for _, eventWithHooksForPlan := range eventsWithHooks {
		isTextEvent := eventWithHooksForPlan.classified.event.Name == textFilePathA ||
			eventWithHooksForPlan.classified.event.Name == textFilePathB
		if !isTextEvent {
			continue
		}

		matchedTextPatternCount++
		if eventWithHooksForPlan.skipDuplicateHooks {
			deduplicatedTextPatternCount++
		}

		changedFilePaths := eventWithHooksForPlan.hookCtx.ChangedFilePaths
		if len(changedFilePaths) != 2 {
			t.Fatalf(
				"expected text-pattern hook context to include exactly 2 changed paths, got %#v",
				changedFilePaths,
			)
		}
	}

	if matchedTextPatternCount != 2 {
		t.Fatalf("expected 2 txt-pattern events in plan, got %d", matchedTextPatternCount)
	}
	if deduplicatedTextPatternCount != 1 {
		t.Fatalf("expected 1 deduplicated txt-pattern event, got %d", deduplicatedTextPatternCount)
	}
}

func TestResolveAppStopStrategy(t *testing.T) {
	t.Run("returns none for empty plan", func(t *testing.T) {
		if got := resolveAppStopStrategy(nil); got != appStopStrategyNone {
			t.Fatalf("resolveAppStopStrategy(nil)=%v, want %v", got, appStopStrategyNone)
		}
	})

	t.Run("single event hard reload uses single-event strategy", func(t *testing.T) {
		eventsWithHooks := []eventWithHooks{
			{needsHardReload: true},
		}
		if got := resolveAppStopStrategy(eventsWithHooks); got != appStopStrategySingleEventHardReload {
			t.Fatalf(
				"resolveAppStopStrategy(single hard reload)=%v, want %v",
				got,
				appStopStrategySingleEventHardReload,
			)
		}
	})

	t.Run("batch with any hard reload uses batch strategy", func(t *testing.T) {
		eventsWithHooks := []eventWithHooks{
			{needsHardReload: false},
			{needsHardReload: true},
		}
		if got := resolveAppStopStrategy(eventsWithHooks); got != appStopStrategyBatchHardReload {
			t.Fatalf(
				"resolveAppStopStrategy(batch hard reload)=%v, want %v",
				got,
				appStopStrategyBatchHardReload,
			)
		}
	})

	t.Run("batch without hard reload uses none strategy", func(t *testing.T) {
		eventsWithHooks := []eventWithHooks{
			{needsHardReload: false},
			{needsHardReload: false},
		}
		if got := resolveAppStopStrategy(eventsWithHooks); got != appStopStrategyNone {
			t.Fatalf("resolveAppStopStrategy(batch no hard reload)=%v, want %v", got, appStopStrategyNone)
		}
	})
}

func TestShouldRunImplicitBuildForEvents(t *testing.T) {
	t.Run("returns false when all events are run-on-change-only", func(t *testing.T) {
		eventsWithHooks := []eventWithHooks{
			{runOnChangeOnly: true},
			{runOnChangeOnly: true},
		}
		if shouldRunImplicitBuildForEvents(eventsWithHooks) {
			t.Fatal("expected implicit build to be skipped")
		}
	})

	t.Run("returns true when any event requires implicit build", func(t *testing.T) {
		eventsWithHooks := []eventWithHooks{
			{runOnChangeOnly: true},
			{runOnChangeOnly: false},
		}
		if !shouldRunImplicitBuildForEvents(eventsWithHooks) {
			t.Fatal("expected implicit build to run")
		}
	})
}

func TestBuildEventExecutionPlanFromClassifiedEvents(t *testing.T) {
	t.Run("returns nil events-with-hooks for empty classified events", func(t *testing.T) {
		if eventsWithHooks := buildEventExecutionPlanFromClassifiedEvents(nil); eventsWithHooks != nil {
			t.Fatalf("expected nil eventsWithHooks for empty classified events, got %#v", eventsWithHooks)
		}
	})

	t.Run("derives events-with-hooks for mixed classified events", func(t *testing.T) {
		classifiedEvents := []classifiedEvent{
			{
				event:    fsnotify.Event{Name: "main.go", Op: fsnotify.Write},
				fileType: fileTypeGo,
			},
			{
				event:    fsnotify.Event{Name: "a.txt", Op: fsnotify.Write},
				fileType: fileTypeOther,
				watchedFile: &wave.WatchedFile{
					Pattern:         "**/*.txt",
					RunOnChangeOnly: true,
				},
			},
			{
				event:    fsnotify.Event{Name: "b.txt", Op: fsnotify.Write},
				fileType: fileTypeOther,
				watchedFile: &wave.WatchedFile{
					Pattern:         "**/*.txt",
					RunOnChangeOnly: true,
				},
			},
		}

		eventsWithHooks := buildEventExecutionPlanFromClassifiedEvents(classifiedEvents)
		if eventsWithHooks == nil {
			t.Fatal("expected non-nil eventsWithHooks for mixed classified events")
		}
		behavioralDecision := deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			eventsWithHooks,
		)
		if behavioralDecision.appStopStrategy != appStopStrategyBatchHardReload {
			t.Fatalf("expected batch hard reload app stop strategy, got %v", behavioralDecision.appStopStrategy)
		}
		if !behavioralDecision.runImplicitBuild {
			t.Fatal("expected runImplicitBuild=true when any event is not run-on-change-only")
		}
		if !behavioralDecision.showRebuildingOverlay {
			t.Fatal("expected showRebuildingOverlay=true for non-css changes")
		}
		if len(eventsWithHooks) != 3 {
			t.Fatalf("expected 3 eventsWithHooks, got %d", len(eventsWithHooks))
		}

		if eventsWithHooks[0].skipDuplicateHooks {
			t.Fatal("expected first txt-pattern event to execute hooks")
		}
		if !eventsWithHooks[1].skipDuplicateHooks &&
			!eventsWithHooks[2].skipDuplicateHooks {
			t.Fatal("expected one of the txt-pattern events to skip duplicate hooks")
		}
	})
}

func TestBuildEventExecutionPlanFromClassifiedEvents_HookContextFilePathIsAbsolute(
	t *testing.T,
) {
	relativeChangedPath := filepath.Join("relative", "changed.txt")

	eventsWithHooks := buildEventExecutionPlanFromClassifiedEvents(
		[]classifiedEvent{
			{
				event:    fsnotify.Event{Name: relativeChangedPath, Op: fsnotify.Write},
				fileType: fileTypeOther,
				watchedFile: &wave.WatchedFile{
					Pattern: "**/*.txt",
				},
			},
		},
	)
	if len(eventsWithHooks) != 1 {
		t.Fatalf("expected one eventWithHooks entry, got %#v", eventsWithHooks)
	}

	gotHookContextFilePath := eventsWithHooks[0].hookCtx.FilePath
	wantHookContextFilePath := pathnorm.Absolute(relativeChangedPath)
	if gotHookContextFilePath != wantHookContextFilePath {
		t.Fatalf(
			"expected hook context FilePath to be absolute %q, got %q",
			wantHookContextFilePath,
			gotHookContextFilePath,
		)
	}
}

func TestDeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(t *testing.T) {
	t.Run("empty inputs produce zero behavioral decision", func(t *testing.T) {
		decision := deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(nil)
		if decision.showRebuildingOverlay {
			t.Fatalf("expected showRebuildingOverlay=false, got %#v", decision)
		}
		if decision.appStopStrategy != appStopStrategyNone {
			t.Fatalf("expected appStopStrategy=none, got %#v", decision)
		}
		if decision.runImplicitBuild {
			t.Fatalf("expected runImplicitBuild=false, got %#v", decision)
		}
	})

	t.Run("single hard-reload run-on-change event keeps implicit build disabled", func(t *testing.T) {
		decision := deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			[]eventWithHooks{
				{
					classified: classifiedEvent{
						event:    fsnotify.Event{Name: "notes.txt", Op: fsnotify.Write},
						fileType: fileTypeOther,
					},
					needsHardReload: true,
					runOnChangeOnly: true,
				},
			},
		)
		if !decision.showRebuildingOverlay {
			t.Fatalf("expected showRebuildingOverlay=true for non-css event, got %#v", decision)
		}
		if decision.appStopStrategy != appStopStrategySingleEventHardReload {
			t.Fatalf("expected single hard-reload stop strategy, got %#v", decision)
		}
		if decision.runImplicitBuild {
			t.Fatalf("expected runImplicitBuild=false for single run-on-change-only event, got %#v", decision)
		}
	})

	t.Run("revalidate-only event suppresses rebuilding overlay", func(t *testing.T) {
		decision := deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			[]eventWithHooks{
				{
					classified: classifiedEvent{
						event:    fsnotify.Event{Name: "content.md", Op: fsnotify.Write},
						fileType: fileTypePrivateStatic,
						watchedFile: &wave.WatchedFile{
							OnlyRunClientDefinedRevalidateFunc: true,
						},
					},
					needsHardReload: false,
					runOnChangeOnly: false,
				},
			},
		)
		if decision.showRebuildingOverlay {
			t.Fatalf(
				"expected showRebuildingOverlay=false for revalidate-only event, got %#v",
				decision,
			)
		}
	})

	t.Run("batch with hard reload and implicit build requested", func(t *testing.T) {
		decision := deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			[]eventWithHooks{
				{
					classified: classifiedEvent{
						event:    fsnotify.Event{Name: "main.go", Op: fsnotify.Write},
						fileType: fileTypeGo,
					},
					needsHardReload: true,
					runOnChangeOnly: false,
				},
				{
					classified: classifiedEvent{
						event:    fsnotify.Event{Name: "notes.txt", Op: fsnotify.Write},
						fileType: fileTypeOther,
					},
					needsHardReload: false,
					runOnChangeOnly: true,
				},
			},
		)
		if !decision.showRebuildingOverlay {
			t.Fatalf("expected showRebuildingOverlay=true for mixed non-css batch, got %#v", decision)
		}
		if decision.appStopStrategy != appStopStrategyBatchHardReload {
			t.Fatalf("expected batch hard-reload stop strategy, got %#v", decision)
		}
		if !decision.runImplicitBuild {
			t.Fatalf("expected runImplicitBuild=true when any event is not run-on-change-only, got %#v", decision)
		}
	})

	t.Run("css-only classified events suppress rebuilding overlay", func(t *testing.T) {
		decision := deriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			[]eventWithHooks{
				{
					classified:      classifiedEvent{fileType: fileTypeCriticalCSS},
					runOnChangeOnly: false,
				},
				{
					classified:      classifiedEvent{fileType: fileTypeNormalCSS},
					runOnChangeOnly: false,
				},
			},
		)
		if decision.showRebuildingOverlay {
			t.Fatalf("expected showRebuildingOverlay=false for css-only changes, got %#v", decision)
		}
		if decision.appStopStrategy != appStopStrategyNone {
			t.Fatalf("expected appStopStrategy=none for css-only no-hard-reload batch, got %#v", decision)
		}
		if !decision.runImplicitBuild {
			t.Fatalf("expected runImplicitBuild=true for non-run-on-change-only css events, got %#v", decision)
		}
	})
}

func TestBuildWatcherEventLogPayloadsForEventsWithHooks(t *testing.T) {
	t.Run("returns nil for nil or empty events-with-hooks", func(t *testing.T) {
		if watcherEventLogPayloads := buildWatcherEventLogPayloadsForEventsWithHooks(nil); watcherEventLogPayloads != nil {
			t.Fatalf("expected nil watcher event log payloads for nil plan, got %#v", watcherEventLogPayloads)
		}

		watcherEventLogPayloads := buildWatcherEventLogPayloadsForEventsWithHooks(
			[]eventWithHooks{},
		)
		if watcherEventLogPayloads != nil {
			t.Fatalf("expected nil watcher event log payloads for empty plan, got %#v", watcherEventLogPayloads)
		}
	})

	t.Run("builds payloads in events-with-hooks order", func(t *testing.T) {
		eventsWithHooks := []eventWithHooks{
			{
				classified: classifiedEvent{
					event: fsnotify.Event{Name: "a.go", Op: fsnotify.Create},
				},
			},
			{
				classified: classifiedEvent{
					event: fsnotify.Event{Name: "b.txt", Op: fsnotify.Write},
				},
			},
			{
				classified: classifiedEvent{
					event: fsnotify.Event{Name: "c.css", Op: fsnotify.Remove},
				},
			},
		}

		watcherEventLogPayloads := buildWatcherEventLogPayloadsForEventsWithHooks(eventsWithHooks)
		expectedWatcherEventLogPayloads := []watcherEventLogPayload{
			{
				operation: fsnotify.Create.String(),
				filePath:  "a.go",
			},
			{
				operation: fsnotify.Write.String(),
				filePath:  "b.txt",
			},
			{
				operation: fsnotify.Remove.String(),
				filePath:  "c.css",
			},
		}
		if !reflect.DeepEqual(watcherEventLogPayloads, expectedWatcherEventLogPayloads) {
			t.Fatalf(
				"watcher event log payloads=%#v, want %#v",
				watcherEventLogPayloads,
				expectedWatcherEventLogPayloads,
			)
		}
	})
}

func TestDeriveWatcherEventFlowDecisionFromPlanningResult(t *testing.T) {
	testCases := []struct {
		name             string
		planningResult   eventExecutionPlanningResult
		expectedDecision watcherEventFlowDecision
	}{
		{
			name: "config-change planning triggers config restart and skips execution",
			planningResult: eventExecutionPlanningResult{
				configChanged: true,
				eventsWithHooks: []eventWithHooks{
					{},
				},
			},
			expectedDecision: watcherEventFlowDecision{
				triggerConfigRestart: true,
			},
		},
		{
			name: "empty eventsWithHooks does not execute",
			planningResult: eventExecutionPlanningResult{
				configChanged:   false,
				eventsWithHooks: nil,
			},
			expectedDecision: watcherEventFlowDecision{},
		},
		{
			name: "eventsWithHooks executes without rebuilding overlay",
			planningResult: eventExecutionPlanningResult{
				eventsWithHooks: []eventWithHooks{
					{
						classified: classifiedEvent{
							fileType: fileTypeCriticalCSS,
						},
					},
				},
			},
			expectedDecision: watcherEventFlowDecision{
				broadcastRebuildingOverlay: false,
				behavioralDecision: eventExecutionPlanBehavioralDecision{
					showRebuildingOverlay: false,
					appStopStrategy:       appStopStrategyNone,
					runImplicitBuild:      true,
				},
			},
		},
		{
			name: "eventsWithHooks executes with rebuilding overlay",
			planningResult: eventExecutionPlanningResult{
				eventsWithHooks: []eventWithHooks{
					{
						classified: classifiedEvent{
							fileType: fileTypeOther,
						},
					},
				},
			},
			expectedDecision: watcherEventFlowDecision{
				broadcastRebuildingOverlay: true,
				behavioralDecision: eventExecutionPlanBehavioralDecision{
					showRebuildingOverlay: true,
					appStopStrategy:       appStopStrategyNone,
					runImplicitBuild:      true,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			decision := deriveWatcherEventFlowDecisionFromPlanningResult(testCase.planningResult)
			if !reflect.DeepEqual(decision, testCase.expectedDecision) {
				t.Fatalf(
					"deriveWatcherEventFlowDecisionFromPlanningResult()=%#v, want %#v",
					decision,
					testCase.expectedDecision,
				)
			}
		})
	}
}

func TestBuildWatcherEventExecutionInputFromPlanningResult(t *testing.T) {
	t.Run("config change suppresses execution plan", func(t *testing.T) {
		executionInput := buildWatcherEventExecutionInputFromPlanningResult(
			eventExecutionPlanningResult{
				configChanged: true,
				eventsWithHooks: []eventWithHooks{
					{
						classified: classifiedEvent{fileType: fileTypeOther},
					},
				},
			},
		)
		if !executionInput.flowDecision.triggerConfigRestart {
			t.Fatalf("expected triggerConfigRestart=true, got %#v", executionInput.flowDecision)
		}
		if executionInput.eventsWithHooks != nil {
			t.Fatalf("expected eventsWithHooks to be nil when config restart is requested, got %#v", executionInput.eventsWithHooks)
		}
		if executionInput.watcherEventLogPayloads != nil {
			t.Fatalf("expected watcherEventLogPayloads to be nil when config restart is requested, got %#v", executionInput.watcherEventLogPayloads)
		}
	})

	t.Run("nil plan returns empty execution input", func(t *testing.T) {
		executionInput := buildWatcherEventExecutionInputFromPlanningResult(
			eventExecutionPlanningResult{},
		)
		if executionInput.flowDecision.triggerConfigRestart {
			t.Fatalf("expected triggerConfigRestart=false, got %#v", executionInput.flowDecision)
		}
		if executionInput.flowDecision.broadcastRebuildingOverlay {
			t.Fatalf("expected broadcastRebuildingOverlay=false, got %#v", executionInput.flowDecision)
		}
		if executionInput.eventsWithHooks != nil {
			t.Fatalf("expected eventsWithHooks to be nil for empty planning result, got %#v", executionInput.eventsWithHooks)
		}
		if executionInput.watcherEventLogPayloads != nil {
			t.Fatalf("expected watcherEventLogPayloads to be nil for empty planning result, got %#v", executionInput.watcherEventLogPayloads)
		}
	})

	t.Run("events-with-hooks and log payload are preserved alongside derived flow decision", func(t *testing.T) {
		eventsWithHooks := []eventWithHooks{
			{
				classified: classifiedEvent{
					fileType: fileTypeOther,
					event: fsnotify.Event{
						Name: "foo.txt",
						Op:   fsnotify.Write,
					},
				},
			},
		}
		executionInput := buildWatcherEventExecutionInputFromPlanningResult(
			eventExecutionPlanningResult{
				eventsWithHooks: eventsWithHooks,
			},
		)
		if !reflect.DeepEqual(executionInput.eventsWithHooks, eventsWithHooks) {
			t.Fatalf(
				"expected eventsWithHooks to match planning result events, got %#v want %#v",
				executionInput.eventsWithHooks,
				eventsWithHooks,
			)
		}
		expectedWatcherEventLogPayloads := []watcherEventLogPayload{
			{
				operation: fsnotify.Write.String(),
				filePath:  "foo.txt",
			},
		}
		if !reflect.DeepEqual(executionInput.watcherEventLogPayloads, expectedWatcherEventLogPayloads) {
			t.Fatalf(
				"expected watcherEventLogPayloads=%#v, got %#v",
				expectedWatcherEventLogPayloads,
				executionInput.watcherEventLogPayloads,
			)
		}
		if !executionInput.flowDecision.broadcastRebuildingOverlay {
			t.Fatalf("expected broadcastRebuildingOverlay=true for non-css event, got %#v", executionInput.flowDecision)
		}
		if executionInput.flowDecision.behavioralDecision.appStopStrategy != appStopStrategyNone {
			t.Fatalf("expected appStopStrategy none for single non-hard-reload event, got %#v", executionInput.flowDecision.behavioralDecision)
		}
		if !executionInput.flowDecision.behavioralDecision.runImplicitBuild {
			t.Fatalf("expected runImplicitBuild=true, got %#v", executionInput.flowDecision.behavioralDecision)
		}
	})
}

func TestPlanBrowserReloadForAction(t *testing.T) {
	testCases := []struct {
		name                  string
		action                browserPhaseAction
		browserDecision       browserPhaseDecision
		expectReloadPlan      bool
		expectedReloadOptions reloadOpts
	}{
		{
			name:   "hard reload preserves wait and cycle flags",
			action: browserPhaseActionHardReload,
			browserDecision: browserPhaseDecision{
				waitForApp:  true,
				waitForVite: true,
				cycleVite:   true,
			},
			expectReloadPlan: true,
			expectedReloadOptions: reloadOpts{
				payload:   refreshPayload{ChangeType: changeTypeOther},
				waitApp:   true,
				waitVite:  true,
				cycleVite: true,
			},
		},
		{
			name:   "revalidate preserves wait flags and clears cycle",
			action: browserPhaseActionRevalidate,
			browserDecision: browserPhaseDecision{
				waitForApp:  true,
				waitForVite: true,
				cycleVite:   true,
			},
			expectReloadPlan: true,
			expectedReloadOptions: reloadOpts{
				payload:   refreshPayload{ChangeType: changeTypeRevalidate},
				waitApp:   true,
				waitVite:  true,
				cycleVite: false,
			},
		},
		{
			name:             "unsupported action has no reload plan",
			action:           browserPhaseActionHotReloadCSS,
			browserDecision:  browserPhaseDecision{waitForApp: true, waitForVite: true, cycleVite: true},
			expectReloadPlan: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			reloadPlan, hasReloadPlan := planBrowserReloadForAction(
				testCase.action,
				testCase.browserDecision,
			)
			if hasReloadPlan != testCase.expectReloadPlan {
				t.Fatalf(
					"hasReloadPlan=%v, want %v (plan=%#v)",
					hasReloadPlan,
					testCase.expectReloadPlan,
					reloadPlan,
				)
			}
			if !testCase.expectReloadPlan {
				return
			}
			if !reflect.DeepEqual(reloadPlan, testCase.expectedReloadOptions) {
				t.Fatalf("reloadPlan=%#v, want %#v", reloadPlan, testCase.expectedReloadOptions)
			}
		})
	}
}

func TestPlanHotReloadCSSPayloads(t *testing.T) {
	testCases := []struct {
		name              string
		includeCritical   bool
		criticalCSS       string
		criticalAvailable bool
		includeNormal     bool
		normalURL         string
		normalAvailable   bool
		expectedPayloads  []refreshPayload
	}{
		{
			name:              "returns critical then normal when both are available",
			includeCritical:   true,
			criticalCSS:       "body { color: red; }",
			criticalAvailable: true,
			includeNormal:     true,
			normalURL:         "/styles.css",
			normalAvailable:   true,
			expectedPayloads: []refreshPayload{
				{
					ChangeType:  changeTypeCriticalCSS,
					CriticalCSS: base64.StdEncoding.EncodeToString([]byte("body { color: red; }")),
				},
				{
					ChangeType:   changeTypeNormalCSS,
					NormalCSSURL: "/styles.css",
				},
			},
		},
		{
			name:              "returns only normal when critical is unavailable",
			includeCritical:   true,
			criticalCSS:       "body { color: red; }",
			criticalAvailable: false,
			includeNormal:     true,
			normalURL:         "/styles.css",
			normalAvailable:   true,
			expectedPayloads: []refreshPayload{
				{
					ChangeType:   changeTypeNormalCSS,
					NormalCSSURL: "/styles.css",
				},
			},
		},
		{
			name:              "returns no payloads when requested payloads are unavailable",
			includeCritical:   true,
			criticalCSS:       "body { color: red; }",
			criticalAvailable: false,
			includeNormal:     true,
			normalURL:         "/styles.css",
			normalAvailable:   false,
			expectedPayloads:  []refreshPayload{},
		},
		{
			name:              "returns no payloads when no css payloads are requested",
			includeCritical:   false,
			criticalCSS:       "body { color: red; }",
			criticalAvailable: true,
			includeNormal:     false,
			normalURL:         "/styles.css",
			normalAvailable:   true,
			expectedPayloads:  []refreshPayload{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			payloads := planHotReloadCSSPayloads(
				testCase.includeCritical,
				testCase.criticalCSS,
				testCase.criticalAvailable,
				testCase.includeNormal,
				testCase.normalURL,
				testCase.normalAvailable,
			)
			if !reflect.DeepEqual(payloads, testCase.expectedPayloads) {
				t.Fatalf("payloads=%#v, want %#v", payloads, testCase.expectedPayloads)
			}
		})
	}
}

func TestPlanInvalidateViteFallbackBrowserDecision(t *testing.T) {
	testCases := []struct {
		name             string
		usingVite        bool
		expectedDecision browserPhaseDecision
	}{
		{
			name:      "vite enabled waits for app and vite",
			usingVite: true,
			expectedDecision: browserPhaseDecision{
				action:      browserPhaseActionHardReload,
				waitForApp:  true,
				waitForVite: true,
			},
		},
		{
			name:      "vite disabled waits for app only",
			usingVite: false,
			expectedDecision: browserPhaseDecision{
				action:      browserPhaseActionHardReload,
				waitForApp:  true,
				waitForVite: false,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			decision := planInvalidateViteFallbackBrowserDecision(testCase.usingVite)
			if !reflect.DeepEqual(decision, testCase.expectedDecision) {
				t.Fatalf("decision=%#v, want %#v", decision, testCase.expectedDecision)
			}
		})
	}
}

func TestShouldAttemptViteInvalidateForBrowserDecision(t *testing.T) {
	testCases := []struct {
		name                  string
		browserDecision       browserPhaseDecision
		usingVite             bool
		expectedShouldAttempt bool
	}{
		{
			name: "invalidate action with vite attempts invalidate",
			browserDecision: browserPhaseDecision{
				action: browserPhaseActionInvalidateVite,
			},
			usingVite:             true,
			expectedShouldAttempt: true,
		},
		{
			name: "invalidate action without vite skips invalidate attempt",
			browserDecision: browserPhaseDecision{
				action: browserPhaseActionInvalidateVite,
			},
			usingVite:             false,
			expectedShouldAttempt: false,
		},
		{
			name: "non-invalidate action never attempts invalidate",
			browserDecision: browserPhaseDecision{
				action: browserPhaseActionHardReload,
			},
			usingVite:             true,
			expectedShouldAttempt: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shouldAttempt := shouldAttemptViteInvalidateForBrowserDecision(
				testCase.browserDecision,
				testCase.usingVite,
			)
			if shouldAttempt != testCase.expectedShouldAttempt {
				t.Fatalf("shouldAttempt=%t, want %t", shouldAttempt, testCase.expectedShouldAttempt)
			}
		})
	}
}

func TestResolveBrowserDecisionAfterInvalidateViteFallback(t *testing.T) {
	testCases := []struct {
		name             string
		browserDecision  browserPhaseDecision
		usingVite        bool
		expectedDecision browserPhaseDecision
	}{
		{
			name: "non-invalidate action is preserved",
			browserDecision: browserPhaseDecision{
				action:      browserPhaseActionHardReload,
				waitForApp:  false,
				waitForVite: true,
				cycleVite:   true,
			},
			usingVite: true,
			expectedDecision: browserPhaseDecision{
				action:      browserPhaseActionHardReload,
				waitForApp:  false,
				waitForVite: true,
				cycleVite:   true,
			},
		},
		{
			name: "invalidate fallback with vite enabled waits for app and vite",
			browserDecision: browserPhaseDecision{
				action:      browserPhaseActionInvalidateVite,
				waitForApp:  false,
				waitForVite: false,
				cycleVite:   true,
			},
			usingVite: true,
			expectedDecision: browserPhaseDecision{
				action:      browserPhaseActionHardReload,
				waitForApp:  true,
				waitForVite: true,
				cycleVite:   true,
			},
		},
		{
			name: "invalidate fallback with vite disabled waits for app only",
			browserDecision: browserPhaseDecision{
				action:      browserPhaseActionInvalidateVite,
				waitForApp:  false,
				waitForVite: true,
				cycleVite:   true,
			},
			usingVite: false,
			expectedDecision: browserPhaseDecision{
				action:      browserPhaseActionHardReload,
				waitForApp:  true,
				waitForVite: false,
				cycleVite:   true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			decision := resolveBrowserDecisionAfterInvalidateViteFallback(
				testCase.browserDecision,
				testCase.usingVite,
			)
			if !reflect.DeepEqual(decision, testCase.expectedDecision) {
				t.Fatalf("decision=%#v, want %#v", decision, testCase.expectedDecision)
			}
		})
	}
}

func TestDeriveBrowserPhaseExecutionCategory(t *testing.T) {
	testCases := []struct {
		name             string
		action           browserPhaseAction
		expectedCategory browserPhaseExecutionCategory
	}{
		{
			name:             "none action has no execution category",
			action:           browserPhaseActionNone,
			expectedCategory: browserPhaseExecutionCategoryNone,
		},
		{
			name:             "hard reload maps to reload category",
			action:           browserPhaseActionHardReload,
			expectedCategory: browserPhaseExecutionCategoryReload,
		},
		{
			name:             "revalidate maps to reload category",
			action:           browserPhaseActionRevalidate,
			expectedCategory: browserPhaseExecutionCategoryReload,
		},
		{
			name:             "hot reload css maps to css category",
			action:           browserPhaseActionHotReloadCSS,
			expectedCategory: browserPhaseExecutionCategoryHotReloadCSS,
		},
		{
			name:             "invalidate-vite requires prior resolution and maps to no-op category",
			action:           browserPhaseActionInvalidateVite,
			expectedCategory: browserPhaseExecutionCategoryNone,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			category := deriveBrowserPhaseExecutionCategory(testCase.action)
			if category != testCase.expectedCategory {
				t.Fatalf("category=%v, want %v", category, testCase.expectedCategory)
			}
		})
	}
}
