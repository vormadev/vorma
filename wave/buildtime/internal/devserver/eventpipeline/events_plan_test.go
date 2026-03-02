package eventpipeline_test

import (
	"encoding/base64"
	"encoding/json"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/wavewatch"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/buildtime/internal/broadcast"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/hooks"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
	"github.com/vormadev/vorma/wave/waveenv"
)

func TestShouldShowRebuildingOverlay(t *testing.T) {
	t.Run("returns false for CSS only changes", func(t *testing.T) {
		classifiedEvents := []eventpipeline.ClassifiedEvent{
			{FileType: eventpipeline.FileTypeCriticalCSS},
			{FileType: eventpipeline.FileTypeNormalCSS},
			{FileType: eventpipeline.FileTypeCriticalAndNormalCSS},
		}

		if eventpipeline.ShouldShowRebuildingOverlay(classifiedEvents) {
			t.Fatal("expected no rebuilding overlay for CSS-only changes")
		}
	})

	t.Run(
		"returns false when non-css changes skip rebuilding notification",
		func(t *testing.T) {
			classifiedEvents := []eventpipeline.ClassifiedEvent{
				{
					FileType: eventpipeline.FileTypeOther,
					WatchedFile: &wavewatch.WatchedFile{
						SkipRebuildingNotification: true,
					},
				},
			}

			if eventpipeline.ShouldShowRebuildingOverlay(classifiedEvents) {
				t.Fatal("expected rebuilding overlay to be skipped")
			}
		},
	)

	t.Run(
		"returns false when non-go change only runs client revalidate",
		func(t *testing.T) {
			classifiedEvents := []eventpipeline.ClassifiedEvent{
				{
					FileType: eventpipeline.FileTypePrivateStatic,
					WatchedFile: &wavewatch.WatchedFile{
						OnlyRunClientDefinedRevalidateFunc: true,
					},
				},
			}

			if eventpipeline.ShouldShowRebuildingOverlay(classifiedEvents) {
				t.Fatal(
					"expected rebuilding overlay to be skipped for revalidate-only change",
				)
			}
		},
	)

	t.Run(
		"returns true for go change even when client revalidate is requested",
		func(t *testing.T) {
			classifiedEvents := []eventpipeline.ClassifiedEvent{
				{
					FileType: eventpipeline.FileTypeGo,
					WatchedFile: &wavewatch.WatchedFile{
						OnlyRunClientDefinedRevalidateFunc: true,
					},
				},
			}

			if !eventpipeline.ShouldShowRebuildingOverlay(classifiedEvents) {
				t.Fatal("expected rebuilding overlay for go change")
			}
		},
	)

	t.Run(
		"returns true when non-css change does not skip rebuilding notification",
		func(t *testing.T) {
			classifiedEvents := []eventpipeline.ClassifiedEvent{
				{FileType: eventpipeline.FileTypeOther},
			}

			if !eventpipeline.ShouldShowRebuildingOverlay(classifiedEvents) {
				t.Fatal("expected rebuilding overlay for non-css change")
			}
		},
	)
}

type configEventPathShapeCaseForEventPlanTests struct {
	Name      string
	BuildPath func(t *testing.T, configFilePath string) string
}

func configEventPathShapeCasesForEventPlanTests() []configEventPathShapeCaseForEventPlanTests {
	return []configEventPathShapeCaseForEventPlanTests{
		{
			Name: "exact",
			BuildPath: func(_ *testing.T, configFilePath string) string {
				return configFilePath
			},
		},
		{
			Name: "dot_alias",
			BuildPath: func(_ *testing.T, configFilePath string) string {
				return filepath.Join(
					filepath.Dir(configFilePath),
					".",
					filepath.Base(configFilePath),
				)
			},
		},
		{
			Name: "sibling_relative_alias",
			BuildPath: func(_ *testing.T, configFilePath string) string {
				return filepath.Join(
					filepath.Dir(configFilePath),
					"nested",
					"..",
					filepath.Base(configFilePath),
				)
			},
		},
		{
			Name: "symlink_alias",
			BuildPath: func(t *testing.T, configFilePath string) string {
				t.Helper()

				configDirectoryPath := filepath.Dir(configFilePath)
				aliasDirectoryPath := filepath.Join(
					configDirectoryPath,
					"config_alias_link",
				)
				if symlinkError := os.Symlink(
					configDirectoryPath,
					aliasDirectoryPath,
				); symlinkError != nil {
					t.Fatalf(
						"create config directory symlink alias: %v",
						symlinkError,
					)
				}

				return filepath.Join(
					aliasDirectoryPath,
					filepath.Base(configFilePath),
				)
			},
		},
	}
}

type configMutationCaseForEventPlanTests struct {
	Name         string
	Op           fsnotify.Op
	PrepareEvent func(t *testing.T, configFilePath string)
}

func configMutationCasesForEventPlanTests() []configMutationCaseForEventPlanTests {
	return []configMutationCaseForEventPlanTests{
		{
			Name: "write",
			Op:   fsnotify.Write,
		},
		{
			Name: "create",
			Op:   fsnotify.Create,
		},
		{
			Name: "remove",
			Op:   fsnotify.Remove,
			PrepareEvent: func(t *testing.T, configFilePath string) {
				t.Helper()
				if removeError := os.Remove(configFilePath); removeError != nil {
					t.Fatalf(
						"failed removing config file before remove Event: %v",
						removeError,
					)
				}
			},
		},
		{
			Name: "rename",
			Op:   fsnotify.Rename,
			PrepareEvent: func(t *testing.T, configFilePath string) {
				t.Helper()
				if renameError := os.Rename(
					configFilePath,
					configFilePath+".renamed",
				); renameError != nil {
					t.Fatalf(
						"failed renaming config file before rename Event: %v",
						renameError,
					)
				}
			},
		},
	}
}

func runConfigMutationAndPathShapeMatrixForEventPlanTests(
	t *testing.T,
	runCase func(
		t *testing.T,
		configMutationCaseForRun configMutationCaseForEventPlanTests,
		pathShapeCaseForRun configEventPathShapeCaseForEventPlanTests,
	),
) {
	t.Helper()

	for _, configMutationCaseForRun := range configMutationCasesForEventPlanTests() {
		for _, pathShapeCaseForRun := range configEventPathShapeCasesForEventPlanTests() {
			configMutationCaseForRun := configMutationCaseForRun
			pathShapeCaseForRun := pathShapeCaseForRun

			t.Run(
				configMutationCaseForRun.Name+"_"+pathShapeCaseForRun.Name,
				func(t *testing.T) {
					runCase(
						t,
						configMutationCaseForRun,
						pathShapeCaseForRun,
					)
				},
			)
		}
	}
}

func setupConfigEventTestConfigForEventPlanTests(
	t *testing.T,
) (*waveconfig.ParsedConfig, string, string) {
	t.Helper()

	root := t.TempDir()
	cfg := newParsedConfigForEventPipelineDedupTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	configFilePath := filepath.Join(root, "backend", "wave.config.json")
	cfg.Core.ConfigLocation = configFilePath
	cfg.Dist.Root = cfg.Core.DistDir

	if mkdirError := os.MkdirAll(filepath.Dir(configFilePath), 0o755); mkdirError != nil {
		t.Fatalf("failed creating config file directory: %v", mkdirError)
	}
	configPayload := map[string]any{
		"Core": map[string]any{
			"MainAppEntry":   "cmd/app",
			"DistDir":        cfg.Core.DistDir,
			"ServerOnlyMode": true,
			"StaticAssetDirs": map[string]any{
				"Public":  cfg.Core.StaticAssetDirs.Public,
				"Private": cfg.Core.StaticAssetDirs.Private,
			},
		},
		"Watch": map[string]any{
			"WatchRoot": root,
		},
	}
	configPayloadBytes, marshalError := json.Marshal(configPayload)
	if marshalError != nil {
		t.Fatalf("failed marshaling config file payload: %v", marshalError)
	}
	if writeError := os.WriteFile(
		configFilePath,
		configPayloadBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("failed writing config file: %v", writeError)
	}

	return cfg, root, configFilePath
}

func TestBuildEventExecutionPlan_ConfigChangeHasNoPlan(t *testing.T) {
	runConfigMutationAndPathShapeMatrixForEventPlanTests(
		t,
		func(
			t *testing.T,
			configMutationCaseForRun configMutationCaseForEventPlanTests,
			pathShapeCaseForRun configEventPathShapeCaseForEventPlanTests,
		) {
			cfg, _, configFilePath := setupConfigEventTestConfigForEventPlanTests(
				t,
			)
			if configMutationCaseForRun.PrepareEvent != nil {
				configMutationCaseForRun.PrepareEvent(t, configFilePath)
			}

			watcherForTest, watcherError := watch.NewWatcher(
				cfg,
				newDiscardLoggerForEventPipelineDedupTests(),
			)
			if watcherError != nil {
				t.Fatalf("watch.NewWatcher returned error: %v", watcherError)
			}
			defer watcherForTest.Close()

			builderForTest := builder.NewBuilder(
				cfg,
				newDiscardLoggerForEventPipelineDedupTests(),
			)
			defer builderForTest.Close()

			configEventPath := pathShapeCaseForRun.BuildPath(t, configFilePath)
			executionPlanningResult := buildEventExecutionPlanForEventPipelineTests(
				cfg,
				[]fsnotify.Event{
					{
						Name: configEventPath,
						Op:   configMutationCaseForRun.Op,
					},
				},
				watcherForTest,
				builderForTest,
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

func TestBuildEventExecutionPlan_BatchPlanIncludesHookBatchContext(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForEventPipelineDedupTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Watch.Include = []wavewatch.WatchedFile{
		{
			Pattern: "**/*.txt",
		},
	}

	fileA := filepath.Join(root, "a.txt")
	fileB := filepath.Join(root, "b.txt")
	if err := os.WriteFile(fileA, []byte("a"), 0o644); err != nil {
		t.Fatalf("failed writing %s: %v", fileA, err)
	}
	if err := os.WriteFile(fileB, []byte("b"), 0o644); err != nil {
		t.Fatalf("failed writing %s: %v", fileB, err)
	}

	watcherForTest, err := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventPipelineDedupTests(),
	)
	if err != nil {
		t.Fatalf("watch.NewWatcher returned error: %v", err)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventPipelineDedupTests(),
	)
	defer builderForTest.Close()

	executionPlanningResult := buildEventExecutionPlanForEventPipelineTests(
		cfg,
		[]fsnotify.Event{
			{Name: fileA, Op: fsnotify.Write},
			{Name: fileB, Op: fsnotify.Write},
		},
		watcherForTest,
		builderForTest,
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
	behavioralDecision := eventpipeline.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		eventsWithHooks,
	)
	if behavioralDecision.AppStopStrategy != eventpipeline.AppStopStrategyNone {
		t.Fatal("expected batchNeedsAppStop=false for txt-only batch")
	}
	if !behavioralDecision.RunImplicitBuild {
		t.Fatal(
			"expected runImplicitBuild=true for non-run-on-change-only batch",
		)
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
		t.Fatalf(
			"expected first hook context to include 2 changed paths, got %#v",
			firstHookContextPaths,
		)
	}
	if len(secondHookContextPaths) != 2 {
		t.Fatalf(
			"expected second hook context to include 2 changed paths, got %#v",
			secondHookContextPaths,
		)
	}

	if eventsWithHooks[0].SkipDuplicateHooks {
		t.Fatal(
			"expected first matched pattern hook set not to be deduplicated",
		)
	}
	if !eventsWithHooks[1].SkipDuplicateHooks {
		t.Fatal("expected second matched pattern hook set to be deduplicated")
	}
}

func TestBuildEventExecutionPlan_MixedFileClassesAndHookShapes(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForEventPipelineDedupTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.Include = []wavewatch.WatchedFile{
		{
			Pattern:         "**/*.txt",
			RunOnChangeOnly: true,
			OnChangeHooks: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						return nil, nil
					},
				},
			},
		},
	}

	goFilePath := filepath.Join(root, "backend", "main.go")
	textFilePathA := filepath.Join(root, "a.txt")
	textFilePathB := filepath.Join(root, "b.txt")
	publicStaticPath := filepath.Join(
		cfg.Core.StaticAssetDirs.Public,
		"logo.svg",
	)

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

	watcherForTest, err := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventPipelineDedupTests(),
	)
	if err != nil {
		t.Fatalf("watch.NewWatcher returned error: %v", err)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForEventPipelineDedupTests(),
	)
	defer builderForTest.Close()

	executionPlanningResult := buildEventExecutionPlanForEventPipelineTests(
		cfg,
		[]fsnotify.Event{
			{Name: goFilePath, Op: fsnotify.Write},
			{Name: textFilePathA, Op: fsnotify.Write},
			{Name: textFilePathB, Op: fsnotify.Write},
			{Name: publicStaticPath, Op: fsnotify.Write},
		},
		watcherForTest,
		builderForTest,
	)

	if executionPlanningResult.ConfigChanged {
		t.Fatal("expected configChanged=false for non-config mixed batch")
	}
	if len(executionPlanningResult.EventsWithHooks) == 0 {
		t.Fatal("expected eventsWithHooks for mixed batch")
	}

	eventsWithHooks := executionPlanningResult.EventsWithHooks
	behavioralDecision := eventpipeline.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		eventsWithHooks,
	)
	if behavioralDecision.AppStopStrategy != eventpipeline.AppStopStrategyBatchHardReload {
		t.Fatalf(
			"expected batch hard reload app stop strategy, got %v",
			behavioralDecision.AppStopStrategy,
		)
	}
	if !behavioralDecision.RunImplicitBuild {
		t.Fatal(
			"expected runImplicitBuild=true for mixed batch with implicit work",
		)
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
		t.Fatalf(
			"expected 2 txt-pattern events in plan, got %d",
			matchedTextPatternCount,
		)
	}
	if deduplicatedTextPatternCount != 1 {
		t.Fatalf(
			"expected 1 deduplicated txt-pattern event, got %d",
			deduplicatedTextPatternCount,
		)
	}
}

func TestResolveAppStopStrategy(t *testing.T) {
	t.Run("returns none for empty plan", func(t *testing.T) {
		if got := eventpipeline.ResolveAppStopStrategy(nil); got != eventpipeline.AppStopStrategyNone {
			t.Fatalf(
				"eventpipeline.ResolveAppStopStrategy(nil)=%v, want %v",
				got,
				eventpipeline.AppStopStrategyNone,
			)
		}
	})

	t.Run(
		"single event hard reload uses single-event strategy",
		func(t *testing.T) {
			eventsWithHooks := []eventpipeline.EventWithHooks{
				{NeedsHardReload: true},
			}
			if got := eventpipeline.ResolveAppStopStrategy(eventsWithHooks); got != eventpipeline.AppStopStrategySingleEventHardReload {
				t.Fatalf(
					"eventpipeline.ResolveAppStopStrategy(single hard reload)=%v, want %v",
					got,
					eventpipeline.AppStopStrategySingleEventHardReload,
				)
			}
		},
	)

	t.Run("batch with any hard reload uses batch strategy", func(t *testing.T) {
		eventsWithHooks := []eventpipeline.EventWithHooks{
			{NeedsHardReload: false},
			{NeedsHardReload: true},
		}
		if got := eventpipeline.ResolveAppStopStrategy(eventsWithHooks); got != eventpipeline.AppStopStrategyBatchHardReload {
			t.Fatalf(
				"eventpipeline.ResolveAppStopStrategy(batch hard reload)=%v, want %v",
				got,
				eventpipeline.AppStopStrategyBatchHardReload,
			)
		}
	})

	t.Run("batch without hard reload uses none strategy", func(t *testing.T) {
		eventsWithHooks := []eventpipeline.EventWithHooks{
			{NeedsHardReload: false},
			{NeedsHardReload: false},
		}
		if got := eventpipeline.ResolveAppStopStrategy(eventsWithHooks); got != eventpipeline.AppStopStrategyNone {
			t.Fatalf(
				"eventpipeline.ResolveAppStopStrategy(batch no hard reload)=%v, want %v",
				got,
				eventpipeline.AppStopStrategyNone,
			)
		}
	})
}

func TestShouldRunImplicitBuildForEvents(t *testing.T) {
	t.Run(
		"returns false when all events are run-on-change-only",
		func(t *testing.T) {
			eventsWithHooks := []eventpipeline.EventWithHooks{
				{RunOnChangeOnly: true},
				{RunOnChangeOnly: true},
			}
			if eventpipeline.ShouldRunImplicitBuildForEvents(eventsWithHooks) {
				t.Fatal("expected implicit build to be skipped")
			}
		},
	)

	t.Run(
		"returns true when any event requires implicit build",
		func(t *testing.T) {
			eventsWithHooks := []eventpipeline.EventWithHooks{
				{RunOnChangeOnly: true},
				{RunOnChangeOnly: false},
			}
			if !eventpipeline.ShouldRunImplicitBuildForEvents(eventsWithHooks) {
				t.Fatal("expected implicit build to run")
			}
		},
	)
}

func TestBuildEventExecutionPlanFromClassifiedEvents(t *testing.T) {
	t.Run(
		"returns nil events-with-hooks for empty classified events",
		func(t *testing.T) {
			if eventsWithHooks := hooks.BuildEventHooksForProcessing(nil); eventsWithHooks != nil {
				t.Fatalf(
					"expected nil eventsWithHooks for empty classified events, got %#v",
					eventsWithHooks,
				)
			}
		},
	)

	t.Run(
		"derives events-with-hooks for mixed classified events",
		func(t *testing.T) {
			classifiedEvents := []eventpipeline.ClassifiedEvent{
				{
					Event: fsnotify.Event{
						Name: "main.go",
						Op:   fsnotify.Write,
					},
					FileType: eventpipeline.FileTypeGo,
				},
				{
					Event:    fsnotify.Event{Name: "a.txt", Op: fsnotify.Write},
					FileType: eventpipeline.FileTypeOther,
					WatchedFile: &wavewatch.WatchedFile{
						Pattern:         "**/*.txt",
						RunOnChangeOnly: true,
					},
				},
				{
					Event:    fsnotify.Event{Name: "b.txt", Op: fsnotify.Write},
					FileType: eventpipeline.FileTypeOther,
					WatchedFile: &wavewatch.WatchedFile{
						Pattern:         "**/*.txt",
						RunOnChangeOnly: true,
					},
				},
			}

			eventsWithHooks := hooks.BuildEventHooksForProcessing(
				classifiedEvents,
			)
			if eventsWithHooks == nil {
				t.Fatal(
					"expected non-nil eventsWithHooks for mixed classified events",
				)
			}
			behavioralDecision := eventpipeline.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
				eventsWithHooks,
			)
			if behavioralDecision.AppStopStrategy != eventpipeline.AppStopStrategyBatchHardReload {
				t.Fatalf(
					"expected batch hard reload app stop strategy, got %v",
					behavioralDecision.AppStopStrategy,
				)
			}
			if !behavioralDecision.RunImplicitBuild {
				t.Fatal(
					"expected runImplicitBuild=true when any event is not run-on-change-only",
				)
			}
			if !behavioralDecision.ShowRebuildingOverlay {
				t.Fatal(
					"expected showRebuildingOverlay=true for non-css changes",
				)
			}
			if len(eventsWithHooks) != 3 {
				t.Fatalf(
					"expected 3 eventsWithHooks, got %d",
					len(eventsWithHooks),
				)
			}

			if eventsWithHooks[1].SkipDuplicateHooks == eventsWithHooks[2].SkipDuplicateHooks {
				t.Fatal(
					"expected one of the txt-pattern events to skip duplicate hooks",
				)
			}
		},
	)
}

func TestBuildEventExecutionPlanFromClassifiedEvents_HookContextFilePathIsAbsolute(
	t *testing.T,
) {
	relativeChangedPath := filepath.Join("relative", "changed.txt")

	eventsWithHooks := hooks.BuildEventHooksForProcessing(
		[]eventpipeline.ClassifiedEvent{
			{
				Event: fsnotify.Event{
					Name: relativeChangedPath,
					Op:   fsnotify.Write,
				},
				FileType: eventpipeline.FileTypeOther,
				WatchedFile: &wavewatch.WatchedFile{
					Pattern: "**/*.txt",
				},
			},
		},
	)
	if len(eventsWithHooks) != 1 {
		t.Fatalf(
			"expected one eventpipeline.EventWithHooks entry, got %#v",
			eventsWithHooks,
		)
	}

	gotHookContextFilePath := eventsWithHooks[0].HookCtx.FilePath
	wantHookContextFilePath := waveenv.Absolute(relativeChangedPath)
	if gotHookContextFilePath != wantHookContextFilePath {
		t.Fatalf(
			"expected hook context FilePath to be absolute %q, got %q",
			wantHookContextFilePath,
			gotHookContextFilePath,
		)
	}
}

func TestDeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
	t *testing.T,
) {
	t.Run("empty inputs produce zero behavioral decision", func(t *testing.T) {
		decision := eventpipeline.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
			nil,
		)
		if decision.ShowRebuildingOverlay {
			t.Fatalf("expected showRebuildingOverlay=false, got %#v", decision)
		}
		if decision.AppStopStrategy != eventpipeline.AppStopStrategyNone {
			t.Fatalf("expected appStopStrategy=none, got %#v", decision)
		}
		if decision.RunImplicitBuild {
			t.Fatalf("expected runImplicitBuild=false, got %#v", decision)
		}
	})

	t.Run(
		"single hard-reload run-on-change event keeps implicit build disabled",
		func(t *testing.T) {
			decision := eventpipeline.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
				[]eventpipeline.EventWithHooks{
					{
						Classified: eventpipeline.ClassifiedEvent{
							Event: fsnotify.Event{
								Name: "notes.txt",
								Op:   fsnotify.Write,
							},
							FileType: eventpipeline.FileTypeOther,
						},
						NeedsHardReload: true,
						RunOnChangeOnly: true,
					},
				},
			)
			if !decision.ShowRebuildingOverlay {
				t.Fatalf(
					"expected showRebuildingOverlay=true for non-css event, got %#v",
					decision,
				)
			}
			if decision.AppStopStrategy != eventpipeline.AppStopStrategySingleEventHardReload {
				t.Fatalf(
					"expected single hard-reload stop strategy, got %#v",
					decision,
				)
			}
			if decision.RunImplicitBuild {
				t.Fatalf(
					"expected runImplicitBuild=false for single run-on-change-only event, got %#v",
					decision,
				)
			}
		},
	)

	t.Run(
		"revalidate-only event suppresses rebuilding overlay",
		func(t *testing.T) {
			decision := eventpipeline.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
				[]eventpipeline.EventWithHooks{
					{
						Classified: eventpipeline.ClassifiedEvent{
							Event: fsnotify.Event{
								Name: "content.md",
								Op:   fsnotify.Write,
							},
							FileType: eventpipeline.FileTypePrivateStatic,
							WatchedFile: &wavewatch.WatchedFile{
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
		},
	)

	t.Run(
		"batch with hard reload and implicit build requested",
		func(t *testing.T) {
			decision := eventpipeline.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
				[]eventpipeline.EventWithHooks{
					{
						Classified: eventpipeline.ClassifiedEvent{
							Event: fsnotify.Event{
								Name: "main.go",
								Op:   fsnotify.Write,
							},
							FileType: eventpipeline.FileTypeGo,
						},
						NeedsHardReload: true,
						RunOnChangeOnly: false,
					},
					{
						Classified: eventpipeline.ClassifiedEvent{
							Event: fsnotify.Event{
								Name: "notes.txt",
								Op:   fsnotify.Write,
							},
							FileType: eventpipeline.FileTypeOther,
						},
						NeedsHardReload: false,
						RunOnChangeOnly: true,
					},
				},
			)
			if !decision.ShowRebuildingOverlay {
				t.Fatalf(
					"expected showRebuildingOverlay=true for mixed non-css batch, got %#v",
					decision,
				)
			}
			if decision.AppStopStrategy != eventpipeline.AppStopStrategyBatchHardReload {
				t.Fatalf(
					"expected batch hard-reload stop strategy, got %#v",
					decision,
				)
			}
			if !decision.RunImplicitBuild {
				t.Fatalf(
					"expected runImplicitBuild=true when any event is not run-on-change-only, got %#v",
					decision,
				)
			}
		},
	)

	t.Run(
		"css-only classified events suppress rebuilding overlay",
		func(t *testing.T) {
			decision := eventpipeline.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
				[]eventpipeline.EventWithHooks{
					{
						Classified: eventpipeline.ClassifiedEvent{
							FileType: eventpipeline.FileTypeCriticalCSS,
						},
						RunOnChangeOnly: false,
					},
					{
						Classified: eventpipeline.ClassifiedEvent{
							FileType: eventpipeline.FileTypeNormalCSS,
						},
						RunOnChangeOnly: false,
					},
				},
			)
			if decision.ShowRebuildingOverlay {
				t.Fatalf(
					"expected showRebuildingOverlay=false for css-only changes, got %#v",
					decision,
				)
			}
			if decision.AppStopStrategy != eventpipeline.AppStopStrategyNone {
				t.Fatalf(
					"expected appStopStrategy=none for css-only no-hard-reload batch, got %#v",
					decision,
				)
			}
			if !decision.RunImplicitBuild {
				t.Fatalf(
					"expected runImplicitBuild=true for non-run-on-change-only css events, got %#v",
					decision,
				)
			}
		},
	)
}

func TestBuildWatcherEventLogPayloadsForEventsWithHooks(t *testing.T) {
	t.Run("returns nil for nil or empty events-with-hooks", func(t *testing.T) {
		if watcherEventLogPayloads := eventpipeline.BuildWatcherEventLogPayloadsForEventsWithHooks(nil); watcherEventLogPayloads != nil {
			t.Fatalf(
				"expected nil watcher event log payloads for nil plan, got %#v",
				watcherEventLogPayloads,
			)
		}

		watcherEventLogPayloads := eventpipeline.BuildWatcherEventLogPayloadsForEventsWithHooks(
			[]eventpipeline.EventWithHooks{},
		)
		if watcherEventLogPayloads != nil {
			t.Fatalf(
				"expected nil watcher event log payloads for empty plan, got %#v",
				watcherEventLogPayloads,
			)
		}
	})

	t.Run("builds payloads in events-with-hooks order", func(t *testing.T) {
		eventsWithHooks := []eventpipeline.EventWithHooks{
			{
				Classified: eventpipeline.ClassifiedEvent{
					Event: fsnotify.Event{Name: "a.go", Op: fsnotify.Create},
				},
			},
			{
				Classified: eventpipeline.ClassifiedEvent{
					Event: fsnotify.Event{Name: "b.txt", Op: fsnotify.Write},
				},
			},
			{
				Classified: eventpipeline.ClassifiedEvent{
					Event: fsnotify.Event{Name: "c.css", Op: fsnotify.Remove},
				},
			},
		}

		watcherEventLogPayloads := eventpipeline.BuildWatcherEventLogPayloadsForEventsWithHooks(
			eventsWithHooks,
		)
		expectedWatcherEventLogPayloads := []eventpipeline.WatcherEventLogPayload{
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
		if !reflect.DeepEqual(
			watcherEventLogPayloads,
			expectedWatcherEventLogPayloads,
		) {
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
		PlanningResult   eventpipeline.EventExecutionPlanningResult
		ExpectedDecision eventpipeline.WatcherEventFlowDecision
	}{
		{
			Name: "config-change planning triggers config restart and skips execution",
			PlanningResult: eventpipeline.EventExecutionPlanningResult{
				ConfigChanged: true,
				EventsWithHooks: []eventpipeline.EventWithHooks{
					{},
				},
			},
			ExpectedDecision: eventpipeline.WatcherEventFlowDecision{
				TriggerConfigRestart: true,
			},
		},
		{
			Name: "empty eventsWithHooks does not execute",
			PlanningResult: eventpipeline.EventExecutionPlanningResult{
				ConfigChanged:   false,
				EventsWithHooks: nil,
			},
			ExpectedDecision: eventpipeline.WatcherEventFlowDecision{},
		},
		{
			Name: "eventsWithHooks executes without rebuilding overlay",
			PlanningResult: eventpipeline.EventExecutionPlanningResult{
				EventsWithHooks: []eventpipeline.EventWithHooks{
					{
						Classified: eventpipeline.ClassifiedEvent{
							FileType: eventpipeline.FileTypeCriticalCSS,
						},
					},
				},
			},
			ExpectedDecision: eventpipeline.WatcherEventFlowDecision{
				BroadcastRebuildingOverlay: false,
				BehavioralDecision: eventpipeline.EventExecutionPlanBehavioralDecision{
					ShowRebuildingOverlay: false,
					AppStopStrategy:       eventpipeline.AppStopStrategyNone,
					RunImplicitBuild:      true,
				},
			},
		},
		{
			Name: "eventsWithHooks executes with rebuilding overlay",
			PlanningResult: eventpipeline.EventExecutionPlanningResult{
				EventsWithHooks: []eventpipeline.EventWithHooks{
					{
						Classified: eventpipeline.ClassifiedEvent{
							FileType: eventpipeline.FileTypeOther,
						},
					},
				},
			},
			ExpectedDecision: eventpipeline.WatcherEventFlowDecision{
				BroadcastRebuildingOverlay: true,
				BehavioralDecision: eventpipeline.EventExecutionPlanBehavioralDecision{
					ShowRebuildingOverlay: true,
					AppStopStrategy:       eventpipeline.AppStopStrategyNone,
					RunImplicitBuild:      true,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := eventpipeline.DeriveWatcherEventFlowDecisionFromPlanningResult(
				testCase.PlanningResult,
			)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf(
					"eventpipeline.DeriveWatcherEventFlowDecisionFromPlanningResult()=%#v, want %#v",
					decision,
					testCase.ExpectedDecision,
				)
			}
		})
	}
}

func TestBuildWatcherEventExecutionInputFromPlanningResult(t *testing.T) {
	t.Run("config change suppresses execution plan", func(t *testing.T) {
		executionInput := eventpipeline.BuildWatcherEventExecutionInputFromPlanningResult(
			eventpipeline.EventExecutionPlanningResult{
				ConfigChanged: true,
				EventsWithHooks: []eventpipeline.EventWithHooks{
					{
						Classified: eventpipeline.ClassifiedEvent{
							FileType: eventpipeline.FileTypeOther,
						},
					},
				},
			},
		)
		if !executionInput.FlowDecision.TriggerConfigRestart {
			t.Fatalf(
				"expected triggerConfigRestart=true, got %#v",
				executionInput.FlowDecision,
			)
		}
		if executionInput.EventsWithHooks != nil {
			t.Fatalf(
				"expected eventsWithHooks to be nil when config restart is requested, got %#v",
				executionInput.EventsWithHooks,
			)
		}
		if executionInput.WatcherEventLogPayloads != nil {
			t.Fatalf(
				"expected watcherEventLogPayloads to be nil when config restart is requested, got %#v",
				executionInput.WatcherEventLogPayloads,
			)
		}
	})

	t.Run("nil plan returns empty execution input", func(t *testing.T) {
		executionInput := eventpipeline.BuildWatcherEventExecutionInputFromPlanningResult(
			eventpipeline.EventExecutionPlanningResult{},
		)
		if executionInput.FlowDecision.TriggerConfigRestart {
			t.Fatalf(
				"expected triggerConfigRestart=false, got %#v",
				executionInput.FlowDecision,
			)
		}
		if executionInput.FlowDecision.BroadcastRebuildingOverlay {
			t.Fatalf(
				"expected broadcastRebuildingOverlay=false, got %#v",
				executionInput.FlowDecision,
			)
		}
		if executionInput.EventsWithHooks != nil {
			t.Fatalf(
				"expected eventsWithHooks to be nil for empty planning result, got %#v",
				executionInput.EventsWithHooks,
			)
		}
		if executionInput.WatcherEventLogPayloads != nil {
			t.Fatalf(
				"expected watcherEventLogPayloads to be nil for empty planning result, got %#v",
				executionInput.WatcherEventLogPayloads,
			)
		}
	})

	t.Run(
		"events-with-hooks and log payload are preserved alongside derived flow decision",
		func(t *testing.T) {
			eventsWithHooks := []eventpipeline.EventWithHooks{
				{
					Classified: eventpipeline.ClassifiedEvent{
						FileType: eventpipeline.FileTypeOther,
						Event: fsnotify.Event{
							Name: "foo.txt",
							Op:   fsnotify.Write,
						},
					},
				},
			}
			executionInput := eventpipeline.BuildWatcherEventExecutionInputFromPlanningResult(
				eventpipeline.EventExecutionPlanningResult{
					EventsWithHooks: eventsWithHooks,
				},
			)
			if !reflect.DeepEqual(
				executionInput.EventsWithHooks,
				eventsWithHooks,
			) {
				t.Fatalf(
					"expected eventsWithHooks to match planning result events, got %#v want %#v",
					executionInput.EventsWithHooks,
					eventsWithHooks,
				)
			}
			expectedWatcherEventLogPayloads := []eventpipeline.WatcherEventLogPayload{
				{
					Operation: fsnotify.Write.String(),
					FilePath:  "foo.txt",
				},
			}
			if !reflect.DeepEqual(
				executionInput.WatcherEventLogPayloads,
				expectedWatcherEventLogPayloads,
			) {
				t.Fatalf(
					"expected watcherEventLogPayloads=%#v, got %#v",
					expectedWatcherEventLogPayloads,
					executionInput.WatcherEventLogPayloads,
				)
			}
			if !executionInput.FlowDecision.BroadcastRebuildingOverlay {
				t.Fatalf(
					"expected broadcastRebuildingOverlay=true for non-css event, got %#v",
					executionInput.FlowDecision,
				)
			}
			if executionInput.FlowDecision.BehavioralDecision.AppStopStrategy != eventpipeline.AppStopStrategyNone {
				t.Fatalf(
					"expected appStopStrategy none for single non-hard-reload event, got %#v",
					executionInput.FlowDecision.BehavioralDecision,
				)
			}
			if !executionInput.FlowDecision.BehavioralDecision.RunImplicitBuild {
				t.Fatalf(
					"expected runImplicitBuild=true, got %#v",
					executionInput.FlowDecision.BehavioralDecision,
				)
			}
		},
	)
}

func TestPlanBrowserReloadForAction(t *testing.T) {
	testCases := []struct {
		Name                  string
		Action                eventpipeline.BrowserPhaseAction
		BrowserDecision       eventpipeline.BrowserPhaseDecision
		ExpectReloadPlan      bool
		ExpectedReloadOptions eventpipeline.ReloadOpts
	}{
		{
			Name:   "hard reload preserves wait and cycle flags",
			Action: eventpipeline.BrowserPhaseActionHardReload,
			BrowserDecision: eventpipeline.BrowserPhaseDecision{
				WaitForApp:  true,
				WaitForVite: true,
				CycleVite:   true,
			},
			ExpectReloadPlan: true,
			ExpectedReloadOptions: eventpipeline.ReloadOpts{
				Payload: broadcast.Payload{
					ChangeType: broadcast.ChangeTypeOther,
				},
				WaitApp:   true,
				WaitVite:  true,
				CycleVite: true,
			},
		},
		{
			Name:   "revalidate preserves wait flags and clears cycle",
			Action: eventpipeline.BrowserPhaseActionRevalidate,
			BrowserDecision: eventpipeline.BrowserPhaseDecision{
				WaitForApp:  true,
				WaitForVite: true,
				CycleVite:   true,
			},
			ExpectReloadPlan: true,
			ExpectedReloadOptions: eventpipeline.ReloadOpts{
				Payload: broadcast.Payload{
					ChangeType: broadcast.ChangeTypeRevalidate,
				},
				WaitApp:   true,
				WaitVite:  true,
				CycleVite: false,
			},
		},
		{
			Name:   "unsupported action has no reload plan",
			Action: eventpipeline.BrowserPhaseActionHotReloadCSS,
			BrowserDecision: eventpipeline.BrowserPhaseDecision{
				WaitForApp:  true,
				WaitForVite: true,
				CycleVite:   true,
			},
			ExpectReloadPlan: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			reloadPlan, hasReloadPlan := eventpipeline.PlanBrowserReloadForAction(
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
				t.Fatalf(
					"reloadPlan=%#v, want %#v",
					reloadPlan,
					testCase.ExpectedReloadOptions,
				)
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
					ChangeType: broadcast.ChangeTypeCriticalCSS,
					CriticalCSS: base64.StdEncoding.EncodeToString(
						[]byte("body { color: red; }"),
					),
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
			payloads := eventpipeline.PlanHotReloadCSSPayloads(
				testCase.IncludeCritical,
				testCase.CriticalCSS,
				testCase.CriticalAvailable,
				testCase.IncludeNormal,
				testCase.NormalURL,
				testCase.NormalAvailable,
			)
			if !reflect.DeepEqual(payloads, testCase.ExpectedPayloads) {
				t.Fatalf(
					"payloads=%#v, want %#v",
					payloads,
					testCase.ExpectedPayloads,
				)
			}
		})
	}
}

func TestPlanInvalidateViteFallbackBrowserDecision(t *testing.T) {
	testCases := []struct {
		Name             string
		UsingVite        bool
		ExpectedDecision eventpipeline.BrowserPhaseDecision
	}{
		{
			Name:      "vite enabled waits for app and vite",
			UsingVite: true,
			ExpectedDecision: eventpipeline.BrowserPhaseDecision{
				Action:      eventpipeline.BrowserPhaseActionHardReload,
				WaitForApp:  true,
				WaitForVite: true,
			},
		},
		{
			Name:      "vite disabled waits for app only",
			UsingVite: false,
			ExpectedDecision: eventpipeline.BrowserPhaseDecision{
				Action:      eventpipeline.BrowserPhaseActionHardReload,
				WaitForApp:  true,
				WaitForVite: false,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := eventpipeline.PlanInvalidateViteFallbackBrowserDecision(
				testCase.UsingVite,
			)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf(
					"decision=%#v, want %#v",
					decision,
					testCase.ExpectedDecision,
				)
			}
		})
	}
}

func TestShouldAttemptViteInvalidateForBrowserDecision(t *testing.T) {
	testCases := []struct {
		Name                  string
		BrowserDecision       eventpipeline.BrowserPhaseDecision
		UsingVite             bool
		ExpectedShouldAttempt bool
	}{
		{
			Name: "invalidate action with vite attempts invalidate",
			BrowserDecision: eventpipeline.BrowserPhaseDecision{
				Action: eventpipeline.BrowserPhaseActionInvalidateVite,
			},
			UsingVite:             true,
			ExpectedShouldAttempt: true,
		},
		{
			Name: "invalidate action without vite skips invalidate attempt",
			BrowserDecision: eventpipeline.BrowserPhaseDecision{
				Action: eventpipeline.BrowserPhaseActionInvalidateVite,
			},
			UsingVite:             false,
			ExpectedShouldAttempt: false,
		},
		{
			Name: "non-invalidate action never attempts invalidate",
			BrowserDecision: eventpipeline.BrowserPhaseDecision{
				Action: eventpipeline.BrowserPhaseActionHardReload,
			},
			UsingVite:             true,
			ExpectedShouldAttempt: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			shouldAttempt := eventpipeline.ShouldAttemptViteInvalidateForBrowserDecision(
				testCase.BrowserDecision,
				testCase.UsingVite,
			)
			if shouldAttempt != testCase.ExpectedShouldAttempt {
				t.Fatalf(
					"shouldAttempt=%t, want %t",
					shouldAttempt,
					testCase.ExpectedShouldAttempt,
				)
			}
		})
	}
}

func TestResolveBrowserDecisionAfterInvalidateViteFallback(t *testing.T) {
	testCases := []struct {
		Name             string
		BrowserDecision  eventpipeline.BrowserPhaseDecision
		UsingVite        bool
		ExpectedDecision eventpipeline.BrowserPhaseDecision
	}{
		{
			Name: "non-invalidate action is preserved",
			BrowserDecision: eventpipeline.BrowserPhaseDecision{
				Action:      eventpipeline.BrowserPhaseActionHardReload,
				WaitForApp:  false,
				WaitForVite: true,
				CycleVite:   true,
			},
			UsingVite: true,
			ExpectedDecision: eventpipeline.BrowserPhaseDecision{
				Action:      eventpipeline.BrowserPhaseActionHardReload,
				WaitForApp:  false,
				WaitForVite: true,
				CycleVite:   true,
			},
		},
		{
			Name: "invalidate fallback with vite enabled waits for app and vite",
			BrowserDecision: eventpipeline.BrowserPhaseDecision{
				Action:      eventpipeline.BrowserPhaseActionInvalidateVite,
				WaitForApp:  false,
				WaitForVite: false,
				CycleVite:   true,
			},
			UsingVite: true,
			ExpectedDecision: eventpipeline.BrowserPhaseDecision{
				Action:      eventpipeline.BrowserPhaseActionHardReload,
				WaitForApp:  true,
				WaitForVite: true,
				CycleVite:   true,
			},
		},
		{
			Name: "invalidate fallback with vite disabled waits for app only",
			BrowserDecision: eventpipeline.BrowserPhaseDecision{
				Action:      eventpipeline.BrowserPhaseActionInvalidateVite,
				WaitForApp:  false,
				WaitForVite: true,
				CycleVite:   true,
			},
			UsingVite: false,
			ExpectedDecision: eventpipeline.BrowserPhaseDecision{
				Action:      eventpipeline.BrowserPhaseActionHardReload,
				WaitForApp:  true,
				WaitForVite: false,
				CycleVite:   true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := eventpipeline.ResolveBrowserDecisionAfterInvalidateViteFallback(
				testCase.BrowserDecision,
				testCase.UsingVite,
			)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf(
					"decision=%#v, want %#v",
					decision,
					testCase.ExpectedDecision,
				)
			}
		})
	}
}

func TestDeriveBrowserPhaseExecutionCategory(t *testing.T) {
	testCases := []struct {
		Name             string
		Action           eventpipeline.BrowserPhaseAction
		ExpectedCategory eventpipeline.BrowserPhaseExecutionCategory
	}{
		{
			Name:             "none action has no execution category",
			Action:           eventpipeline.BrowserPhaseActionNone,
			ExpectedCategory: eventpipeline.BrowserPhaseExecutionCategoryNone,
		},
		{
			Name:             "hard reload maps to reload category",
			Action:           eventpipeline.BrowserPhaseActionHardReload,
			ExpectedCategory: eventpipeline.BrowserPhaseExecutionCategoryReload,
		},
		{
			Name:             "revalidate maps to reload category",
			Action:           eventpipeline.BrowserPhaseActionRevalidate,
			ExpectedCategory: eventpipeline.BrowserPhaseExecutionCategoryReload,
		},
		{
			Name:             "hot reload css maps to css category",
			Action:           eventpipeline.BrowserPhaseActionHotReloadCSS,
			ExpectedCategory: eventpipeline.BrowserPhaseExecutionCategoryHotReloadCSS,
		},
		{
			Name:             "invalidate-vite requires prior resolution and maps to no-op category",
			Action:           eventpipeline.BrowserPhaseActionInvalidateVite,
			ExpectedCategory: eventpipeline.BrowserPhaseExecutionCategoryNone,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			category := eventpipeline.DeriveBrowserPhaseExecutionCategory(
				testCase.Action,
			)
			if category != testCase.ExpectedCategory {
				t.Fatalf(
					"category=%v, want %v",
					category,
					testCase.ExpectedCategory,
				)
			}
		})
	}
}
