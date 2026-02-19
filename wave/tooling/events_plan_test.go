package tooling

import (
	"encoding/base64"
	"github.com/vormadev/vorma/wave/tooling/broadcast"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/watch"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/waveshared"
)

func TestShouldShowRebuildingOverlay(t *testing.T) {
	t.Run("returns false for CSS only changes", func(t *testing.T) {
		classifiedEvents := []devserver.ClassifiedEvent{
			{FileType: devserver.FileTypeCriticalCSS},
			{FileType: devserver.FileTypeNormalCSS},
			{FileType: devserver.FileTypeCriticalAndNormalCSS},
		}

		if devserver.ShouldShowRebuildingOverlay(classifiedEvents) {
			t.Fatal("expected no rebuilding overlay for CSS-only changes")
		}
	})

	t.Run("returns false when non-css changes skip rebuilding notification", func(t *testing.T) {
		classifiedEvents := []devserver.ClassifiedEvent{
			{
				FileType: devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					SkipRebuildingNotification: true,
				},
			},
		}

		if devserver.ShouldShowRebuildingOverlay(classifiedEvents) {
			t.Fatal("expected rebuilding overlay to be skipped")
		}
	})

	t.Run("returns false when non-go change only runs client revalidate", func(t *testing.T) {
		classifiedEvents := []devserver.ClassifiedEvent{
			{
				FileType: devserver.FileTypePrivateStatic,
				WatchedFile: &wave.WatchedFile{
					OnlyRunClientDefinedRevalidateFunc: true,
				},
			},
		}

		if devserver.ShouldShowRebuildingOverlay(classifiedEvents) {
			t.Fatal("expected rebuilding overlay to be skipped for revalidate-only change")
		}
	})

	t.Run("returns true for go change even when client revalidate is requested", func(t *testing.T) {
		classifiedEvents := []devserver.ClassifiedEvent{
			{
				FileType: devserver.FileTypeGo,
				WatchedFile: &wave.WatchedFile{
					OnlyRunClientDefinedRevalidateFunc: true,
				},
			},
		}

		if !devserver.ShouldShowRebuildingOverlay(classifiedEvents) {
			t.Fatal("expected rebuilding overlay for go change")
		}
	})

	t.Run("returns true when non-css change does not skip rebuilding notification", func(t *testing.T) {
		classifiedEvents := []devserver.ClassifiedEvent{
			{FileType: devserver.FileTypeOther},
		}

		if !devserver.ShouldShowRebuildingOverlay(classifiedEvents) {
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
			if configMutationCaseForRun.PrepareEvent != nil {
				configMutationCaseForRun.PrepareEvent(t, configFilePath)
			}

			watcher, builder := setupWatcherAndBuilderForToolingTests(t, cfg)

			serverForTest := &devserver.Server{
				Cfg: cfg,
				Log: newDiscardLogger(),
			}

			configEventPath := pathShapeCaseForRun.BuildPath(t, configFilePath)
			executionPlanningResult := serverForTest.BuildEventExecutionPlan(
				[]fsnotify.Event{
					{
						Name: configEventPath,
						Op:   configMutationCaseForRun.Op,
					},
				},
				watcher,
				builder,
			)

			if !executionPlanningResult.ConfigChanged {
				t.Fatalf(
					"expected configChanged=true for config file %s/%s",
					configMutationCaseForRun.Name,
					pathShapeCaseForRun.Name,
				)
			}
			if executionPlanningResult.EventsWithHooks != nil {
				t.Fatalf(
					"expected no eventsWithHooks when config changed, got %#v",
					executionPlanningResult.EventsWithHooks,
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

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	serverForTest := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	executionPlanningResult := serverForTest.BuildEventExecutionPlan(
		[]fsnotify.Event{
			{Name: fileA, Op: fsnotify.Write},
			{Name: fileB, Op: fsnotify.Write},
		},
		watcher,
		builder,
	)

	if executionPlanningResult.ConfigChanged {
		t.Fatal("expected configChanged=false for normal file batch")
	}
	if len(executionPlanningResult.EventsWithHooks) == 0 {
		t.Fatal("expected eventsWithHooks for batch change")
	}

	eventsWithHooks := executionPlanningResult.EventsWithHooks
	if len(eventsWithHooks) <= 1 {
		t.Fatal("expected isBatch=true")
	}
	behavioralDecision := devserver.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		eventsWithHooks,
	)
	if behavioralDecision.AppStopStrategy != devserver.AppStopStrategyNone {
		t.Fatal("expected batchNeedsAppStop=false for txt-only batch")
	}
	if !behavioralDecision.RunImplicitBuild {
		t.Fatal("expected runImplicitBuild=true for non-run-on-change-only batch")
	}
	if !behavioralDecision.ShowRebuildingOverlay {
		t.Fatal("expected showRebuildingOverlay=true for txt batch")
	}
	if len(eventsWithHooks) != 2 {
		t.Fatalf("expected 2 eventsWithHooks, got %d", len(eventsWithHooks))
	}

	firstHookContextPaths := eventsWithHooks[0].HookCtx.ChangedFilePaths
	secondHookContextPaths := eventsWithHooks[1].HookCtx.ChangedFilePaths
	if len(firstHookContextPaths) != 2 {
		t.Fatalf("expected first hook context to include 2 changed paths, got %#v", firstHookContextPaths)
	}
	if len(secondHookContextPaths) != 2 {
		t.Fatalf("expected second hook context to include 2 changed paths, got %#v", secondHookContextPaths)
	}

	if eventsWithHooks[0].SkipDuplicateHooks {
		t.Fatal("expected first matched pattern hook set not to be deduplicated")
	}
	if !eventsWithHooks[1].SkipDuplicateHooks {
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

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	serverForTest := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	executionPlanningResult := serverForTest.BuildEventExecutionPlan(
		[]fsnotify.Event{
			{Name: goFilePath, Op: fsnotify.Write},
			{Name: textFilePathA, Op: fsnotify.Write},
			{Name: textFilePathB, Op: fsnotify.Write},
			{Name: publicStaticPath, Op: fsnotify.Write},
		},
		watcher,
		builder,
	)

	if executionPlanningResult.ConfigChanged {
		t.Fatal("expected configChanged=false for non-config mixed batch")
	}
	if len(executionPlanningResult.EventsWithHooks) == 0 {
		t.Fatal("expected eventsWithHooks for mixed batch")
	}

	eventsWithHooks := executionPlanningResult.EventsWithHooks
	behavioralDecision := devserver.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		eventsWithHooks,
	)
	if behavioralDecision.AppStopStrategy != devserver.AppStopStrategyBatchHardReload {
		t.Fatalf(
			"expected batch hard reload app stop strategy, got %v",
			behavioralDecision.AppStopStrategy,
		)
	}
	if !behavioralDecision.RunImplicitBuild {
		t.Fatal("expected runImplicitBuild=true for mixed batch with implicit work")
	}
	if !behavioralDecision.ShowRebuildingOverlay {
		t.Fatal("expected showRebuildingOverlay=true for mixed non-css batch")
	}
	if len(eventsWithHooks) != 4 {
		t.Fatalf("expected 4 eventsWithHooks, got %d", len(eventsWithHooks))
	}

	var matchedTextPatternCount int
	var deduplicatedTextPatternCount int
	for _, eventWithHooksForPlan := range eventsWithHooks {
		isTextEvent := eventWithHooksForPlan.Classified.Event.Name == textFilePathA ||
			eventWithHooksForPlan.Classified.Event.Name == textFilePathB
		if !isTextEvent {
			continue
		}

		matchedTextPatternCount++
		if eventWithHooksForPlan.SkipDuplicateHooks {
			deduplicatedTextPatternCount++
		}

		changedFilePaths := eventWithHooksForPlan.HookCtx.ChangedFilePaths
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
		if got := devserver.ResolveAppStopStrategy(nil); got != devserver.AppStopStrategyNone {
			t.Fatalf("devserver.ResolveAppStopStrategy(nil)=%v, want %v", got, devserver.AppStopStrategyNone)
		}
	})

	t.Run("single event hard reload uses single-event strategy", func(t *testing.T) {
		eventsWithHooks := []devserver.EventWithHooks{
			{NeedsHardReload: true},
		}
		if got := devserver.ResolveAppStopStrategy(eventsWithHooks); got != devserver.AppStopStrategySingleEventHardReload {
			t.Fatalf(
				"devserver.ResolveAppStopStrategy(single hard reload)=%v, want %v",
				got,
				devserver.AppStopStrategySingleEventHardReload,
			)
		}
	})

	t.Run("batch with any hard reload uses batch strategy", func(t *testing.T) {
		eventsWithHooks := []devserver.EventWithHooks{
			{NeedsHardReload: false},
			{NeedsHardReload: true},
		}
		if got := devserver.ResolveAppStopStrategy(eventsWithHooks); got != devserver.AppStopStrategyBatchHardReload {
			t.Fatalf(
				"devserver.ResolveAppStopStrategy(batch hard reload)=%v, want %v",
				got,
				devserver.AppStopStrategyBatchHardReload,
			)
		}
	})

	t.Run("batch without hard reload uses none strategy", func(t *testing.T) {
		eventsWithHooks := []devserver.EventWithHooks{
			{NeedsHardReload: false},
			{NeedsHardReload: false},
		}
		if got := devserver.ResolveAppStopStrategy(eventsWithHooks); got != devserver.AppStopStrategyNone {
			t.Fatalf("devserver.ResolveAppStopStrategy(batch no hard reload)=%v, want %v", got, devserver.AppStopStrategyNone)
		}
	})
}

func TestShouldRunImplicitBuildForEvents(t *testing.T) {
	t.Run("returns false when all events are run-on-change-only", func(t *testing.T) {
		eventsWithHooks := []devserver.EventWithHooks{
			{RunOnChangeOnly: true},
			{RunOnChangeOnly: true},
		}
		if devserver.ShouldRunImplicitBuildForEvents(eventsWithHooks) {
			t.Fatal("expected implicit build to be skipped")
		}
	})

	t.Run("returns true when any event requires implicit build", func(t *testing.T) {
		eventsWithHooks := []devserver.EventWithHooks{
			{RunOnChangeOnly: true},
			{RunOnChangeOnly: false},
		}
		if !devserver.ShouldRunImplicitBuildForEvents(eventsWithHooks) {
			t.Fatal("expected implicit build to run")
		}
	})
}

func TestBuildEventExecutionPlanFromClassifiedEvents(t *testing.T) {
	t.Run("returns nil events-with-hooks for empty classified events", func(t *testing.T) {
		if eventsWithHooks := devserver.BuildEventExecutionPlanFromClassifiedEvents(nil); eventsWithHooks != nil {
			t.Fatalf("expected nil eventsWithHooks for empty classified events, got %#v", eventsWithHooks)
		}
	})

	t.Run("derives events-with-hooks for mixed classified events", func(t *testing.T) {
		classifiedEvents := []devserver.ClassifiedEvent{
			{
				Event:    fsnotify.Event{Name: "main.go", Op: fsnotify.Write},
				FileType: devserver.FileTypeGo,
			},
			{
				Event:    fsnotify.Event{Name: "a.txt", Op: fsnotify.Write},
				FileType: devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					Pattern:         "**/*.txt",
					RunOnChangeOnly: true,
				},
			},
			{
				Event:    fsnotify.Event{Name: "b.txt", Op: fsnotify.Write},
				FileType: devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					Pattern:         "**/*.txt",
					RunOnChangeOnly: true,
				},
			},
		}

		eventsWithHooks := devserver.BuildEventExecutionPlanFromClassifiedEvents(classifiedEvents)
		if eventsWithHooks == nil {
			t.Fatal("expected non-nil eventsWithHooks for mixed classified events")
		}
		behavioralDecision := devserver.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			eventsWithHooks,
		)
		if behavioralDecision.AppStopStrategy != devserver.AppStopStrategyBatchHardReload {
			t.Fatalf("expected batch hard reload app stop strategy, got %v", behavioralDecision.AppStopStrategy)
		}
		if !behavioralDecision.RunImplicitBuild {
			t.Fatal("expected runImplicitBuild=true when any event is not run-on-change-only")
		}
		if !behavioralDecision.ShowRebuildingOverlay {
			t.Fatal("expected showRebuildingOverlay=true for non-css changes")
		}
		if len(eventsWithHooks) != 3 {
			t.Fatalf("expected 3 eventsWithHooks, got %d", len(eventsWithHooks))
		}

		if eventsWithHooks[0].SkipDuplicateHooks {
			t.Fatal("expected first txt-pattern event to execute hooks")
		}
		if !eventsWithHooks[1].SkipDuplicateHooks &&
			!eventsWithHooks[2].SkipDuplicateHooks {
			t.Fatal("expected one of the txt-pattern events to skip duplicate hooks")
		}
	})
}

func TestBuildEventExecutionPlanFromClassifiedEvents_HookContextFilePathIsAbsolute(
	t *testing.T,
) {
	relativeChangedPath := filepath.Join("relative", "changed.txt")

	eventsWithHooks := devserver.BuildEventExecutionPlanFromClassifiedEvents(
		[]devserver.ClassifiedEvent{
			{
				Event:    fsnotify.Event{Name: relativeChangedPath, Op: fsnotify.Write},
				FileType: devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					Pattern: "**/*.txt",
				},
			},
		},
	)
	if len(eventsWithHooks) != 1 {
		t.Fatalf("expected one devserver.EventWithHooks entry, got %#v", eventsWithHooks)
	}

	gotHookContextFilePath := eventsWithHooks[0].HookCtx.FilePath
	wantHookContextFilePath := waveshared.Absolute(relativeChangedPath)
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
		decision := devserver.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(nil)
		if decision.ShowRebuildingOverlay {
			t.Fatalf("expected showRebuildingOverlay=false, got %#v", decision)
		}
		if decision.AppStopStrategy != devserver.AppStopStrategyNone {
			t.Fatalf("expected appStopStrategy=none, got %#v", decision)
		}
		if decision.RunImplicitBuild {
			t.Fatalf("expected runImplicitBuild=false, got %#v", decision)
		}
	})

	t.Run("single hard-reload run-on-change event keeps implicit build disabled", func(t *testing.T) {
		decision := devserver.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			[]devserver.EventWithHooks{
				{
					Classified: devserver.ClassifiedEvent{
						Event:    fsnotify.Event{Name: "notes.txt", Op: fsnotify.Write},
						FileType: devserver.FileTypeOther,
					},
					NeedsHardReload: true,
					RunOnChangeOnly: true,
				},
			},
		)
		if !decision.ShowRebuildingOverlay {
			t.Fatalf("expected showRebuildingOverlay=true for non-css event, got %#v", decision)
		}
		if decision.AppStopStrategy != devserver.AppStopStrategySingleEventHardReload {
			t.Fatalf("expected single hard-reload stop strategy, got %#v", decision)
		}
		if decision.RunImplicitBuild {
			t.Fatalf("expected runImplicitBuild=false for single run-on-change-only event, got %#v", decision)
		}
	})

	t.Run("revalidate-only event suppresses rebuilding overlay", func(t *testing.T) {
		decision := devserver.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			[]devserver.EventWithHooks{
				{
					Classified: devserver.ClassifiedEvent{
						Event:    fsnotify.Event{Name: "content.md", Op: fsnotify.Write},
						FileType: devserver.FileTypePrivateStatic,
						WatchedFile: &wave.WatchedFile{
							OnlyRunClientDefinedRevalidateFunc: true,
						},
					},
					NeedsHardReload: false,
					RunOnChangeOnly: false,
				},
			},
		)
		if decision.ShowRebuildingOverlay {
			t.Fatalf(
				"expected showRebuildingOverlay=false for revalidate-only event, got %#v",
				decision,
			)
		}
	})

	t.Run("batch with hard reload and implicit build requested", func(t *testing.T) {
		decision := devserver.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			[]devserver.EventWithHooks{
				{
					Classified: devserver.ClassifiedEvent{
						Event:    fsnotify.Event{Name: "main.go", Op: fsnotify.Write},
						FileType: devserver.FileTypeGo,
					},
					NeedsHardReload: true,
					RunOnChangeOnly: false,
				},
				{
					Classified: devserver.ClassifiedEvent{
						Event:    fsnotify.Event{Name: "notes.txt", Op: fsnotify.Write},
						FileType: devserver.FileTypeOther,
					},
					NeedsHardReload: false,
					RunOnChangeOnly: true,
				},
			},
		)
		if !decision.ShowRebuildingOverlay {
			t.Fatalf("expected showRebuildingOverlay=true for mixed non-css batch, got %#v", decision)
		}
		if decision.AppStopStrategy != devserver.AppStopStrategyBatchHardReload {
			t.Fatalf("expected batch hard-reload stop strategy, got %#v", decision)
		}
		if !decision.RunImplicitBuild {
			t.Fatalf("expected runImplicitBuild=true when any event is not run-on-change-only, got %#v", decision)
		}
	})

	t.Run("css-only classified events suppress rebuilding overlay", func(t *testing.T) {
		decision := devserver.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			[]devserver.EventWithHooks{
				{
					Classified:      devserver.ClassifiedEvent{FileType: devserver.FileTypeCriticalCSS},
					RunOnChangeOnly: false,
				},
				{
					Classified:      devserver.ClassifiedEvent{FileType: devserver.FileTypeNormalCSS},
					RunOnChangeOnly: false,
				},
			},
		)
		if decision.ShowRebuildingOverlay {
			t.Fatalf("expected showRebuildingOverlay=false for css-only changes, got %#v", decision)
		}
		if decision.AppStopStrategy != devserver.AppStopStrategyNone {
			t.Fatalf("expected appStopStrategy=none for css-only no-hard-reload batch, got %#v", decision)
		}
		if !decision.RunImplicitBuild {
			t.Fatalf("expected runImplicitBuild=true for non-run-on-change-only css events, got %#v", decision)
		}
	})
}

func TestBuildWatcherEventLogPayloadsForEventsWithHooks(t *testing.T) {
	t.Run("returns nil for nil or empty events-with-hooks", func(t *testing.T) {
		if watcherEventLogPayloads := devserver.BuildWatcherEventLogPayloadsForEventsWithHooks(nil); watcherEventLogPayloads != nil {
			t.Fatalf("expected nil watcher event log payloads for nil plan, got %#v", watcherEventLogPayloads)
		}

		watcherEventLogPayloads := devserver.BuildWatcherEventLogPayloadsForEventsWithHooks(
			[]devserver.EventWithHooks{},
		)
		if watcherEventLogPayloads != nil {
			t.Fatalf("expected nil watcher event log payloads for empty plan, got %#v", watcherEventLogPayloads)
		}
	})

	t.Run("builds payloads in events-with-hooks order", func(t *testing.T) {
		eventsWithHooks := []devserver.EventWithHooks{
			{
				Classified: devserver.ClassifiedEvent{
					Event: fsnotify.Event{Name: "a.go", Op: fsnotify.Create},
				},
			},
			{
				Classified: devserver.ClassifiedEvent{
					Event: fsnotify.Event{Name: "b.txt", Op: fsnotify.Write},
				},
			},
			{
				Classified: devserver.ClassifiedEvent{
					Event: fsnotify.Event{Name: "c.css", Op: fsnotify.Remove},
				},
			},
		}

		watcherEventLogPayloads := devserver.BuildWatcherEventLogPayloadsForEventsWithHooks(eventsWithHooks)
		expectedWatcherEventLogPayloads := []devserver.WatcherEventLogPayload{
			{
				Operation: fsnotify.Create.String(),
				FilePath:  "a.go",
			},
			{
				Operation: fsnotify.Write.String(),
				FilePath:  "b.txt",
			},
			{
				Operation: fsnotify.Remove.String(),
				FilePath:  "c.css",
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
		Name             string
		PlanningResult   devserver.EventExecutionPlanningResult
		ExpectedDecision devserver.WatcherEventFlowDecision
	}{
		{
			Name: "config-change planning triggers config restart and skips execution",
			PlanningResult: devserver.EventExecutionPlanningResult{
				ConfigChanged: true,
				EventsWithHooks: []devserver.EventWithHooks{
					{},
				},
			},
			ExpectedDecision: devserver.WatcherEventFlowDecision{
				TriggerConfigRestart: true,
			},
		},
		{
			Name: "empty eventsWithHooks does not execute",
			PlanningResult: devserver.EventExecutionPlanningResult{
				ConfigChanged:   false,
				EventsWithHooks: nil,
			},
			ExpectedDecision: devserver.WatcherEventFlowDecision{},
		},
		{
			Name: "eventsWithHooks executes without rebuilding overlay",
			PlanningResult: devserver.EventExecutionPlanningResult{
				EventsWithHooks: []devserver.EventWithHooks{
					{
						Classified: devserver.ClassifiedEvent{
							FileType: devserver.FileTypeCriticalCSS,
						},
					},
				},
			},
			ExpectedDecision: devserver.WatcherEventFlowDecision{
				BroadcastRebuildingOverlay: false,
				BehavioralDecision: devserver.EventExecutionPlanBehavioralDecision{
					ShowRebuildingOverlay: false,
					AppStopStrategy:       devserver.AppStopStrategyNone,
					RunImplicitBuild:      true,
				},
			},
		},
		{
			Name: "eventsWithHooks executes with rebuilding overlay",
			PlanningResult: devserver.EventExecutionPlanningResult{
				EventsWithHooks: []devserver.EventWithHooks{
					{
						Classified: devserver.ClassifiedEvent{
							FileType: devserver.FileTypeOther,
						},
					},
				},
			},
			ExpectedDecision: devserver.WatcherEventFlowDecision{
				BroadcastRebuildingOverlay: true,
				BehavioralDecision: devserver.EventExecutionPlanBehavioralDecision{
					ShowRebuildingOverlay: true,
					AppStopStrategy:       devserver.AppStopStrategyNone,
					RunImplicitBuild:      true,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := devserver.DeriveWatcherEventFlowDecisionFromPlanningResult(testCase.PlanningResult)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf(
					"devserver.DeriveWatcherEventFlowDecisionFromPlanningResult()=%#v, want %#v",
					decision,
					testCase.ExpectedDecision,
				)
			}
		})
	}
}

func TestBuildWatcherEventExecutionInputFromPlanningResult(t *testing.T) {
	t.Run("config change suppresses execution plan", func(t *testing.T) {
		executionInput := devserver.BuildWatcherEventExecutionInputFromPlanningResult(
			devserver.EventExecutionPlanningResult{
				ConfigChanged: true,
				EventsWithHooks: []devserver.EventWithHooks{
					{
						Classified: devserver.ClassifiedEvent{FileType: devserver.FileTypeOther},
					},
				},
			},
		)
		if !executionInput.FlowDecision.TriggerConfigRestart {
			t.Fatalf("expected triggerConfigRestart=true, got %#v", executionInput.FlowDecision)
		}
		if executionInput.EventsWithHooks != nil {
			t.Fatalf("expected eventsWithHooks to be nil when config restart is requested, got %#v", executionInput.EventsWithHooks)
		}
		if executionInput.WatcherEventLogPayloads != nil {
			t.Fatalf("expected watcherEventLogPayloads to be nil when config restart is requested, got %#v", executionInput.WatcherEventLogPayloads)
		}
	})

	t.Run("nil plan returns empty execution input", func(t *testing.T) {
		executionInput := devserver.BuildWatcherEventExecutionInputFromPlanningResult(
			devserver.EventExecutionPlanningResult{},
		)
		if executionInput.FlowDecision.TriggerConfigRestart {
			t.Fatalf("expected triggerConfigRestart=false, got %#v", executionInput.FlowDecision)
		}
		if executionInput.FlowDecision.BroadcastRebuildingOverlay {
			t.Fatalf("expected broadcastRebuildingOverlay=false, got %#v", executionInput.FlowDecision)
		}
		if executionInput.EventsWithHooks != nil {
			t.Fatalf("expected eventsWithHooks to be nil for empty planning result, got %#v", executionInput.EventsWithHooks)
		}
		if executionInput.WatcherEventLogPayloads != nil {
			t.Fatalf("expected watcherEventLogPayloads to be nil for empty planning result, got %#v", executionInput.WatcherEventLogPayloads)
		}
	})

	t.Run("events-with-hooks and log payload are preserved alongside derived flow decision", func(t *testing.T) {
		eventsWithHooks := []devserver.EventWithHooks{
			{
				Classified: devserver.ClassifiedEvent{
					FileType: devserver.FileTypeOther,
					Event: fsnotify.Event{
						Name: "foo.txt",
						Op:   fsnotify.Write,
					},
				},
			},
		}
		executionInput := devserver.BuildWatcherEventExecutionInputFromPlanningResult(
			devserver.EventExecutionPlanningResult{
				EventsWithHooks: eventsWithHooks,
			},
		)
		if !reflect.DeepEqual(executionInput.EventsWithHooks, eventsWithHooks) {
			t.Fatalf(
				"expected eventsWithHooks to match planning result events, got %#v want %#v",
				executionInput.EventsWithHooks,
				eventsWithHooks,
			)
		}
		expectedWatcherEventLogPayloads := []devserver.WatcherEventLogPayload{
			{
				Operation: fsnotify.Write.String(),
				FilePath:  "foo.txt",
			},
		}
		if !reflect.DeepEqual(executionInput.WatcherEventLogPayloads, expectedWatcherEventLogPayloads) {
			t.Fatalf(
				"expected watcherEventLogPayloads=%#v, got %#v",
				expectedWatcherEventLogPayloads,
				executionInput.WatcherEventLogPayloads,
			)
		}
		if !executionInput.FlowDecision.BroadcastRebuildingOverlay {
			t.Fatalf("expected broadcastRebuildingOverlay=true for non-css event, got %#v", executionInput.FlowDecision)
		}
		if executionInput.FlowDecision.BehavioralDecision.AppStopStrategy != devserver.AppStopStrategyNone {
			t.Fatalf("expected appStopStrategy none for single non-hard-reload event, got %#v", executionInput.FlowDecision.BehavioralDecision)
		}
		if !executionInput.FlowDecision.BehavioralDecision.RunImplicitBuild {
			t.Fatalf("expected runImplicitBuild=true, got %#v", executionInput.FlowDecision.BehavioralDecision)
		}
	})
}

func TestPlanBrowserReloadForAction(t *testing.T) {
	testCases := []struct {
		Name                  string
		Action                devserver.BrowserPhaseAction
		BrowserDecision       devserver.BrowserPhaseDecision
		ExpectReloadPlan      bool
		ExpectedReloadOptions devserver.ReloadOpts
	}{
		{
			Name:   "hard reload preserves wait and cycle flags",
			Action: devserver.BrowserPhaseActionHardReload,
			BrowserDecision: devserver.BrowserPhaseDecision{
				WaitForApp:  true,
				WaitForVite: true,
				CycleVite:   true,
			},
			ExpectReloadPlan: true,
			ExpectedReloadOptions: devserver.ReloadOpts{
				Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
				WaitApp:   true,
				WaitVite:  true,
				CycleVite: true,
			},
		},
		{
			Name:   "revalidate preserves wait flags and clears cycle",
			Action: devserver.BrowserPhaseActionRevalidate,
			BrowserDecision: devserver.BrowserPhaseDecision{
				WaitForApp:  true,
				WaitForVite: true,
				CycleVite:   true,
			},
			ExpectReloadPlan: true,
			ExpectedReloadOptions: devserver.ReloadOpts{
				Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeRevalidate},
				WaitApp:   true,
				WaitVite:  true,
				CycleVite: false,
			},
		},
		{
			Name:             "unsupported action has no reload plan",
			Action:           devserver.BrowserPhaseActionHotReloadCSS,
			BrowserDecision:  devserver.BrowserPhaseDecision{WaitForApp: true, WaitForVite: true, CycleVite: true},
			ExpectReloadPlan: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			reloadPlan, hasReloadPlan := devserver.PlanBrowserReloadForAction(
				testCase.Action,
				testCase.BrowserDecision,
			)
			if hasReloadPlan != testCase.ExpectReloadPlan {
				t.Fatalf(
					"hasReloadPlan=%v, want %v (plan=%#v)",
					hasReloadPlan,
					testCase.ExpectReloadPlan,
					reloadPlan,
				)
			}
			if !testCase.ExpectReloadPlan {
				return
			}
			if !reflect.DeepEqual(reloadPlan, testCase.ExpectedReloadOptions) {
				t.Fatalf("reloadPlan=%#v, want %#v", reloadPlan, testCase.ExpectedReloadOptions)
			}
		})
	}
}

func TestPlanHotReloadCSSPayloads(t *testing.T) {
	testCases := []struct {
		Name              string
		IncludeCritical   bool
		CriticalCSS       string
		CriticalAvailable bool
		IncludeNormal     bool
		NormalURL         string
		NormalAvailable   bool
		ExpectedPayloads  []broadcast.Payload
	}{
		{
			Name:              "returns critical then normal when both are available",
			IncludeCritical:   true,
			CriticalCSS:       "body { color: red; }",
			CriticalAvailable: true,
			IncludeNormal:     true,
			NormalURL:         "/styles.css",
			NormalAvailable:   true,
			ExpectedPayloads: []broadcast.Payload{
				{
					ChangeType:  broadcast.ChangeTypeCriticalCSS,
					CriticalCSS: base64.StdEncoding.EncodeToString([]byte("body { color: red; }")),
				},
				{
					ChangeType:   broadcast.ChangeTypeNormalCSS,
					NormalCSSURL: "/styles.css",
				},
			},
		},
		{
			Name:              "returns only normal when critical is unavailable",
			IncludeCritical:   true,
			CriticalCSS:       "body { color: red; }",
			CriticalAvailable: false,
			IncludeNormal:     true,
			NormalURL:         "/styles.css",
			NormalAvailable:   true,
			ExpectedPayloads: []broadcast.Payload{
				{
					ChangeType:   broadcast.ChangeTypeNormalCSS,
					NormalCSSURL: "/styles.css",
				},
			},
		},
		{
			Name:              "returns no payloads when requested payloads are unavailable",
			IncludeCritical:   true,
			CriticalCSS:       "body { color: red; }",
			CriticalAvailable: false,
			IncludeNormal:     true,
			NormalURL:         "/styles.css",
			NormalAvailable:   false,
			ExpectedPayloads:  []broadcast.Payload{},
		},
		{
			Name:              "returns no payloads when no css payloads are requested",
			IncludeCritical:   false,
			CriticalCSS:       "body { color: red; }",
			CriticalAvailable: true,
			IncludeNormal:     false,
			NormalURL:         "/styles.css",
			NormalAvailable:   true,
			ExpectedPayloads:  []broadcast.Payload{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			payloads := devserver.PlanHotReloadCSSPayloads(
				testCase.IncludeCritical,
				testCase.CriticalCSS,
				testCase.CriticalAvailable,
				testCase.IncludeNormal,
				testCase.NormalURL,
				testCase.NormalAvailable,
			)
			if !reflect.DeepEqual(payloads, testCase.ExpectedPayloads) {
				t.Fatalf("payloads=%#v, want %#v", payloads, testCase.ExpectedPayloads)
			}
		})
	}
}

func TestPlanInvalidateViteFallbackBrowserDecision(t *testing.T) {
	testCases := []struct {
		Name             string
		UsingVite        bool
		ExpectedDecision devserver.BrowserPhaseDecision
	}{
		{
			Name:      "vite enabled waits for app and vite",
			UsingVite: true,
			ExpectedDecision: devserver.BrowserPhaseDecision{
				Action:      devserver.BrowserPhaseActionHardReload,
				WaitForApp:  true,
				WaitForVite: true,
			},
		},
		{
			Name:      "vite disabled waits for app only",
			UsingVite: false,
			ExpectedDecision: devserver.BrowserPhaseDecision{
				Action:      devserver.BrowserPhaseActionHardReload,
				WaitForApp:  true,
				WaitForVite: false,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := devserver.PlanInvalidateViteFallbackBrowserDecision(testCase.UsingVite)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf("decision=%#v, want %#v", decision, testCase.ExpectedDecision)
			}
		})
	}
}

func TestShouldAttemptViteInvalidateForBrowserDecision(t *testing.T) {
	testCases := []struct {
		Name                  string
		BrowserDecision       devserver.BrowserPhaseDecision
		UsingVite             bool
		ExpectedShouldAttempt bool
	}{
		{
			Name: "invalidate action with vite attempts invalidate",
			BrowserDecision: devserver.BrowserPhaseDecision{
				Action: devserver.BrowserPhaseActionInvalidateVite,
			},
			UsingVite:             true,
			ExpectedShouldAttempt: true,
		},
		{
			Name: "invalidate action without vite skips invalidate attempt",
			BrowserDecision: devserver.BrowserPhaseDecision{
				Action: devserver.BrowserPhaseActionInvalidateVite,
			},
			UsingVite:             false,
			ExpectedShouldAttempt: false,
		},
		{
			Name: "non-invalidate action never attempts invalidate",
			BrowserDecision: devserver.BrowserPhaseDecision{
				Action: devserver.BrowserPhaseActionHardReload,
			},
			UsingVite:             true,
			ExpectedShouldAttempt: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			shouldAttempt := devserver.ShouldAttemptViteInvalidateForBrowserDecision(
				testCase.BrowserDecision,
				testCase.UsingVite,
			)
			if shouldAttempt != testCase.ExpectedShouldAttempt {
				t.Fatalf("shouldAttempt=%t, want %t", shouldAttempt, testCase.ExpectedShouldAttempt)
			}
		})
	}
}

func TestResolveBrowserDecisionAfterInvalidateViteFallback(t *testing.T) {
	testCases := []struct {
		Name             string
		BrowserDecision  devserver.BrowserPhaseDecision
		UsingVite        bool
		ExpectedDecision devserver.BrowserPhaseDecision
	}{
		{
			Name: "non-invalidate action is preserved",
			BrowserDecision: devserver.BrowserPhaseDecision{
				Action:      devserver.BrowserPhaseActionHardReload,
				WaitForApp:  false,
				WaitForVite: true,
				CycleVite:   true,
			},
			UsingVite: true,
			ExpectedDecision: devserver.BrowserPhaseDecision{
				Action:      devserver.BrowserPhaseActionHardReload,
				WaitForApp:  false,
				WaitForVite: true,
				CycleVite:   true,
			},
		},
		{
			Name: "invalidate fallback with vite enabled waits for app and vite",
			BrowserDecision: devserver.BrowserPhaseDecision{
				Action:      devserver.BrowserPhaseActionInvalidateVite,
				WaitForApp:  false,
				WaitForVite: false,
				CycleVite:   true,
			},
			UsingVite: true,
			ExpectedDecision: devserver.BrowserPhaseDecision{
				Action:      devserver.BrowserPhaseActionHardReload,
				WaitForApp:  true,
				WaitForVite: true,
				CycleVite:   true,
			},
		},
		{
			Name: "invalidate fallback with vite disabled waits for app only",
			BrowserDecision: devserver.BrowserPhaseDecision{
				Action:      devserver.BrowserPhaseActionInvalidateVite,
				WaitForApp:  false,
				WaitForVite: true,
				CycleVite:   true,
			},
			UsingVite: false,
			ExpectedDecision: devserver.BrowserPhaseDecision{
				Action:      devserver.BrowserPhaseActionHardReload,
				WaitForApp:  true,
				WaitForVite: false,
				CycleVite:   true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := devserver.ResolveBrowserDecisionAfterInvalidateViteFallback(
				testCase.BrowserDecision,
				testCase.UsingVite,
			)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf("decision=%#v, want %#v", decision, testCase.ExpectedDecision)
			}
		})
	}
}

func TestDeriveBrowserPhaseExecutionCategory(t *testing.T) {
	testCases := []struct {
		Name             string
		Action           devserver.BrowserPhaseAction
		ExpectedCategory devserver.BrowserPhaseExecutionCategory
	}{
		{
			Name:             "none action has no execution category",
			Action:           devserver.BrowserPhaseActionNone,
			ExpectedCategory: devserver.BrowserPhaseExecutionCategoryNone,
		},
		{
			Name:             "hard reload maps to reload category",
			Action:           devserver.BrowserPhaseActionHardReload,
			ExpectedCategory: devserver.BrowserPhaseExecutionCategoryReload,
		},
		{
			Name:             "revalidate maps to reload category",
			Action:           devserver.BrowserPhaseActionRevalidate,
			ExpectedCategory: devserver.BrowserPhaseExecutionCategoryReload,
		},
		{
			Name:             "hot reload css maps to css category",
			Action:           devserver.BrowserPhaseActionHotReloadCSS,
			ExpectedCategory: devserver.BrowserPhaseExecutionCategoryHotReloadCSS,
		},
		{
			Name:             "invalidate-vite requires prior resolution and maps to no-op category",
			Action:           devserver.BrowserPhaseActionInvalidateVite,
			ExpectedCategory: devserver.BrowserPhaseExecutionCategoryNone,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			category := devserver.DeriveBrowserPhaseExecutionCategory(testCase.Action)
			if category != testCase.ExpectedCategory {
				t.Fatalf("category=%v, want %v", category, testCase.ExpectedCategory)
			}
		})
	}
}
