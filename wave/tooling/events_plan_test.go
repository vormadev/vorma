package tooling

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
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
			if executionPlanningResult.plan != nil {
				t.Fatalf("expected no plan when config changed, got %#v", executionPlanningResult.plan)
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

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
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
	if executionPlanningResult.plan == nil {
		t.Fatal("expected execution plan for batch change")
	}

	executionPlan := executionPlanningResult.plan
	if len(executionPlan.eventsWithHooks) <= 1 {
		t.Fatal("expected isBatch=true")
	}
	if executionPlan.appStopStrategy != appStopStrategyNone {
		t.Fatal("expected batchNeedsAppStop=false for txt-only batch")
	}
	if !executionPlan.runImplicitBuild {
		t.Fatal("expected runImplicitBuild=true for non-run-on-change-only batch")
	}
	if !executionPlan.showRebuildingOverlay {
		t.Fatal("expected showRebuildingOverlay=true for txt batch")
	}
	if len(executionPlan.eventsWithHooks) != 2 {
		t.Fatalf("expected 2 eventsWithHooks, got %d", len(executionPlan.eventsWithHooks))
	}

	firstHookContextPaths := executionPlan.eventsWithHooks[0].hookCtx.ChangedFilePaths
	secondHookContextPaths := executionPlan.eventsWithHooks[1].hookCtx.ChangedFilePaths
	if len(firstHookContextPaths) != 2 {
		t.Fatalf("expected first hook context to include 2 changed paths, got %#v", firstHookContextPaths)
	}
	if len(secondHookContextPaths) != 2 {
		t.Fatalf("expected second hook context to include 2 changed paths, got %#v", secondHookContextPaths)
	}

	if executionPlan.eventsWithHooks[0].skipDuplicateHooks {
		t.Fatal("expected first matched pattern hook set not to be deduplicated")
	}
	if !executionPlan.eventsWithHooks[1].skipDuplicateHooks {
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

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
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
	if executionPlanningResult.plan == nil {
		t.Fatal("expected execution plan for mixed batch")
	}

	executionPlan := executionPlanningResult.plan
	if executionPlan.appStopStrategy != appStopStrategyBatchHardReload {
		t.Fatalf(
			"expected batch hard reload app stop strategy, got %v",
			executionPlan.appStopStrategy,
		)
	}
	if !executionPlan.runImplicitBuild {
		t.Fatal("expected runImplicitBuild=true for mixed batch with implicit work")
	}
	if !executionPlan.showRebuildingOverlay {
		t.Fatal("expected showRebuildingOverlay=true for mixed non-css batch")
	}
	if len(executionPlan.eventsWithHooks) != 4 {
		t.Fatalf("expected 4 eventsWithHooks, got %d", len(executionPlan.eventsWithHooks))
	}

	var matchedTextPatternCount int
	var deduplicatedTextPatternCount int
	for _, eventWithHooksForPlan := range executionPlan.eventsWithHooks {
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
	t.Run("returns nil for empty classified events", func(t *testing.T) {
		if plan := buildEventExecutionPlanFromClassifiedEvents(nil); plan != nil {
			t.Fatalf("expected nil plan for empty classified events, got %#v", plan)
		}
	})

	t.Run("derives plan fields for mixed classified events", func(t *testing.T) {
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

		plan := buildEventExecutionPlanFromClassifiedEvents(classifiedEvents)
		if plan == nil {
			t.Fatal("expected non-nil plan for mixed classified events")
		}
		if plan.appStopStrategy != appStopStrategyBatchHardReload {
			t.Fatalf("expected batch hard reload app stop strategy, got %v", plan.appStopStrategy)
		}
		if !plan.runImplicitBuild {
			t.Fatal("expected runImplicitBuild=true when any event is not run-on-change-only")
		}
		if !plan.showRebuildingOverlay {
			t.Fatal("expected showRebuildingOverlay=true for non-css changes")
		}
		if len(plan.eventsWithHooks) != 3 {
			t.Fatalf("expected 3 eventsWithHooks, got %d", len(plan.eventsWithHooks))
		}

		if plan.eventsWithHooks[0].skipDuplicateHooks {
			t.Fatal("expected first txt-pattern event to execute hooks")
		}
		if !plan.eventsWithHooks[1].skipDuplicateHooks &&
			!plan.eventsWithHooks[2].skipDuplicateHooks {
			t.Fatal("expected one of the txt-pattern events to skip duplicate hooks")
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
