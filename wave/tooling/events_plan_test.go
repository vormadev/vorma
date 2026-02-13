package tooling

import (
	"os"
	"path/filepath"
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
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	configFilePath := filepath.Join(root, "backend", "wave.config.json")
	cfg.Core.ConfigLocation = configFilePath

	if err := os.MkdirAll(filepath.Dir(configFilePath), 0755); err != nil {
		t.Fatalf("failed creating config file directory: %v", err)
	}
	if err := os.WriteFile(configFilePath, []byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`), 0644); err != nil {
		t.Fatalf("failed writing config file: %v", err)
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
			{
				Name: configFilePath,
				Op:   fsnotify.Write,
			},
		},
		watcher,
		builder,
	)

	if !executionPlanningResult.configChanged {
		t.Fatal("expected configChanged=true for config file write")
	}
	if executionPlanningResult.plan != nil {
		t.Fatalf("expected no plan when config changed, got %#v", executionPlanningResult.plan)
	}
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
