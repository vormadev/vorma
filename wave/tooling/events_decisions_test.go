package tooling

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/watch/classification"
)

func buildPreClassificationPlanFromEventsForTest(
	events []fsnotify.Event,
	eventClassificationProber *classification.EventClassificationProber,
) classification.PreClassificationPlan {
	return classification.BuildPreClassificationPlanFromEvents(
		events,
		eventClassificationProber.ProbeIsConfigFile,
		eventClassificationProber.ProbeEventDirectoryStatus,
	)
}

func derivePreClassificationStepResultForTest(
	event fsnotify.Event,
	eventClassificationProber *classification.EventClassificationProber,
) classification.PreClassificationStepResult {
	return classification.DerivePreClassificationStepResult(
		event,
		eventClassificationProber.ProbeIsConfigFile,
		eventClassificationProber.ProbeEventDirectoryStatus,
	)
}

func TestDeriveWatcherEventPreClassificationDecision(t *testing.T) {
	testCases := []struct {
		Name                        string
		Event                       fsnotify.Event
		IsConfigFile                bool
		EventIsDirectory            bool
		EventPathStatProbeSucceeded bool
		ExpectedDecision            classification.PreClassificationDecision
	}{
		{
			Name:                        "config write triggers config changed",
			Event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Write},
			IsConfigFile:                true,
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: true,
			ExpectedDecision: classification.PreClassificationDecision{
				ConfigChanged: true,
			},
		},
		{
			Name:                        "config create triggers config changed",
			Event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Create},
			IsConfigFile:                true,
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: true,
			ExpectedDecision: classification.PreClassificationDecision{
				ConfigChanged: true,
			},
		},
		{
			Name:                        "config remove triggers config changed",
			Event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Remove},
			IsConfigFile:                true,
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: false,
			ExpectedDecision: classification.PreClassificationDecision{
				ConfigChanged: true,
			},
		},
		{
			Name:                        "config rename triggers config changed",
			Event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Rename},
			IsConfigFile:                true,
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: false,
			ExpectedDecision: classification.PreClassificationDecision{
				ConfigChanged: true,
			},
		},
		{
			Name:                        "config chmod does not trigger config changed",
			Event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Chmod},
			IsConfigFile:                true,
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: true,
			ExpectedDecision: classification.PreClassificationDecision{
				ClassifyEvent: true,
			},
		},
		{
			Name:                        "directory create adds watch",
			Event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Create},
			IsConfigFile:                false,
			EventIsDirectory:            true,
			EventPathStatProbeSucceeded: true,
			ExpectedDecision: classification.PreClassificationDecision{
				AddDirectoryWatch: true,
			},
		},
		{
			Name:                        "directory rename adds watch",
			Event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Rename},
			IsConfigFile:                false,
			EventIsDirectory:            true,
			EventPathStatProbeSucceeded: true,
			ExpectedDecision: classification.PreClassificationDecision{
				AddDirectoryWatch: true,
			},
		},
		{
			Name:                        "directory write does not classify",
			Event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Write},
			IsConfigFile:                false,
			EventIsDirectory:            true,
			EventPathStatProbeSucceeded: true,
			ExpectedDecision:            classification.PreClassificationDecision{},
		},
		{
			Name:                        "regular file write classifies",
			Event:                       fsnotify.Event{Name: "app.go", Op: fsnotify.Write},
			IsConfigFile:                false,
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: true,
			ExpectedDecision: classification.PreClassificationDecision{
				ClassifyEvent: true,
			},
		},
		{
			Name:                        "create with failed stat still adds watch and classifies",
			Event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Create},
			IsConfigFile:                false,
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: false,
			ExpectedDecision: classification.PreClassificationDecision{
				AddDirectoryWatch: true,
				ClassifyEvent:     true,
			},
		},
		{
			Name:                        "rename with failed stat still adds watch and classifies",
			Event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Rename},
			IsConfigFile:                false,
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: false,
			ExpectedDecision: classification.PreClassificationDecision{
				AddDirectoryWatch: true,
				ClassifyEvent:     true,
			},
		},
		{
			Name:                        "write with failed stat classifies without directory watch",
			Event:                       fsnotify.Event{Name: "file.txt", Op: fsnotify.Write},
			IsConfigFile:                false,
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: false,
			ExpectedDecision: classification.PreClassificationDecision{
				ClassifyEvent: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := classification.DerivePreClassificationDecision(
				testCase.Event,
				testCase.IsConfigFile,
				testCase.EventIsDirectory,
				testCase.EventPathStatProbeSucceeded,
			)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf(
					"classification.DerivePreClassificationDecision()=%#v, want %#v",
					decision,
					testCase.ExpectedDecision,
				)
			}
		})
	}
}

func TestDeriveWatcherEventPreClassificationDecisionForNonConfigEvent(t *testing.T) {
	testCases := []struct {
		Name                        string
		Event                       fsnotify.Event
		EventIsDirectory            bool
		EventPathStatProbeSucceeded bool
		ExpectedDecision            classification.PreClassificationDecision
	}{
		{
			Name:                        "directory create adds watch",
			Event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Create},
			EventIsDirectory:            true,
			EventPathStatProbeSucceeded: true,
			ExpectedDecision: classification.PreClassificationDecision{
				AddDirectoryWatch: true,
			},
		},
		{
			Name:                        "directory write is skipped",
			Event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Write},
			EventIsDirectory:            true,
			EventPathStatProbeSucceeded: true,
			ExpectedDecision:            classification.PreClassificationDecision{},
		},
		{
			Name:                        "missing-stat rename adds watch and classifies",
			Event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Rename},
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: false,
			ExpectedDecision: classification.PreClassificationDecision{
				AddDirectoryWatch: true,
				ClassifyEvent:     true,
			},
		},
		{
			Name:                        "regular file write classifies",
			Event:                       fsnotify.Event{Name: "app.go", Op: fsnotify.Write},
			EventIsDirectory:            false,
			EventPathStatProbeSucceeded: true,
			ExpectedDecision: classification.PreClassificationDecision{
				ClassifyEvent: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := classification.DerivePreClassificationDecisionForNonConfigEvent(
				testCase.Event,
				testCase.EventIsDirectory,
				testCase.EventPathStatProbeSucceeded,
			)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf(
					"classification.DerivePreClassificationDecisionForNonConfigEvent()=%#v, want %#v",
					decision,
					testCase.ExpectedDecision,
				)
			}
		})
	}
}

func TestBuildWatcherEventPreClassificationPlanFromEvents(t *testing.T) {
	t.Run("empty events returns empty plan", func(t *testing.T) {
		plan := buildPreClassificationPlanFromEventsForTest(
			nil,
			classification.NewEventClassificationProber(nil),
		)
		if plan.ConfigChanged {
			t.Fatalf("expected ConfigChanged=false for empty events, got %#v", plan)
		}
		if len(plan.AddDirectoryWatchPaths) != 0 {
			t.Fatalf("expected no AddDirectoryWatchPaths for empty events, got %#v", plan.AddDirectoryWatchPaths)
		}
		if len(plan.EventsToClassify) != 0 {
			t.Fatalf("expected no eventsToClassify for empty events, got %#v", plan.EventsToClassify)
		}
	})

	t.Run("short-circuits after config mutation and skips trailing probes", func(t *testing.T) {
		configProbeCount := 0
		statProbeCount := 0
		prober := classification.NewEventClassificationProber(func(path string) bool {
			configProbeCount++
			return path == "wave.config.json"
		})
		prober.StatPathFn = func(_ string) (os.FileInfo, error) {
			statProbeCount++
			return nil, os.ErrNotExist
		}

		plan := buildPreClassificationPlanFromEventsForTest(
			[]fsnotify.Event{
				{Name: "notes.txt", Op: fsnotify.Write},
				{Name: "wave.config.json", Op: fsnotify.Rename},
				{Name: "later.txt", Op: fsnotify.Write},
			},
			prober,
		)

		if !plan.ConfigChanged {
			t.Fatal("expected ConfigChanged=true when config mutation event is present")
		}
		if len(plan.AddDirectoryWatchPaths) != 0 {
			t.Fatalf(
				"expected no AddDirectoryWatchPaths when config mutation short-circuits, got %#v",
				plan.AddDirectoryWatchPaths,
			)
		}
		if len(plan.EventsToClassify) != 0 {
			t.Fatalf(
				"expected no eventsToClassify when config mutation short-circuits, got %#v",
				plan.EventsToClassify,
			)
		}
		if configProbeCount != 2 {
			t.Fatalf("expected config probe count=2 (up to config mutation), got %d", configProbeCount)
		}
		if statProbeCount != 1 {
			t.Fatalf("expected stat probe count=1 (pre-config event only), got %d", statProbeCount)
		}
	})

	t.Run("collects all inputs when no config mutation is present", func(t *testing.T) {
		configProbeCount := 0
		statProbeCount := 0
		prober := classification.NewEventClassificationProber(func(path string) bool {
			configProbeCount++
			return path == "wave.config.json"
		})
		prober.StatPathFn = func(path string) (os.FileInfo, error) {
			statProbeCount++
			return nil, os.ErrNotExist
		}

		plan := buildPreClassificationPlanFromEventsForTest(
			[]fsnotify.Event{
				{Name: "notes.txt", Op: fsnotify.Write},
				{Name: "notes.txt", Op: fsnotify.Create},
				{Name: "wave.config.json", Op: fsnotify.Chmod},
			},
			prober,
		)

		if plan.ConfigChanged {
			t.Fatalf("expected ConfigChanged=false when config mutation is absent, got %#v", plan)
		}
		if len(plan.AddDirectoryWatchPaths) != 1 {
			t.Fatalf("expected one AddDirectoryWatchPath, got %#v", plan.AddDirectoryWatchPaths)
		}
		if len(plan.EventsToClassify) != 3 {
			t.Fatalf("expected three eventsToClassify, got %#v", plan.EventsToClassify)
		}
		if configProbeCount != 2 {
			t.Fatalf("expected config probe to run once per unique path, got %d", configProbeCount)
		}
		if statProbeCount != 2 {
			t.Fatalf("expected stat probe to run once per unique path, got %d", statProbeCount)
		}
	})

	t.Run("preserves decisions and skips non-actionable directory write", func(t *testing.T) {
		root := t.TempDir()
		directoryPath := filepath.Join(root, "assets")
		regularFilePath := filepath.Join(root, "app.go")
		if err := os.MkdirAll(directoryPath, 0o755); err != nil {
			t.Fatalf("failed creating directory path: %v", err)
		}
		if err := os.WriteFile(regularFilePath, []byte("package main"), 0o644); err != nil {
			t.Fatalf("failed writing regular file: %v", err)
		}

		prober := classification.NewEventClassificationProber(func(string) bool {
			return false
		})
		prober.StatPathFn = func(path string) (os.FileInfo, error) {
			return os.Stat(path)
		}

		plan := buildPreClassificationPlanFromEventsForTest(
			[]fsnotify.Event{
				{Name: directoryPath, Op: fsnotify.Write},
				{Name: filepath.Join(root, "new.txt"), Op: fsnotify.Create},
				{Name: regularFilePath, Op: fsnotify.Write},
			},
			prober,
		)

		if plan.ConfigChanged {
			t.Fatalf("expected ConfigChanged=false for non-config events, got %#v", plan)
		}
		if len(plan.AddDirectoryWatchPaths) != 1 {
			t.Fatalf("expected one AddDirectoryWatchPath, got %#v", plan.AddDirectoryWatchPaths)
		}
		if len(plan.EventsToClassify) != 2 {
			t.Fatalf("expected two eventsToClassify (directory write skipped), got %#v", plan.EventsToClassify)
		}

		firstAddDirectoryPath := plan.AddDirectoryWatchPaths[0]
		if firstAddDirectoryPath != filepath.Join(root, "new.txt") {
			t.Fatalf("expected AddDirectoryWatch path to be new.txt create path, got %#v", firstAddDirectoryPath)
		}

		firstEventToClassify := plan.EventsToClassify[0]
		if firstEventToClassify.Name != filepath.Join(root, "new.txt") {
			t.Fatalf("expected first eventToClassify to be new.txt create, got %#v", firstEventToClassify)
		}

		secondEventToClassify := plan.EventsToClassify[1]
		if secondEventToClassify.Name != regularFilePath {
			t.Fatalf("expected second eventToClassify to be regular file write, got %#v", secondEventToClassify)
		}
	})

	t.Run("deduplicates add-directory-watch paths in stable first-seen order", func(t *testing.T) {
		root := t.TempDir()
		firstCreatedPath := filepath.Join(root, "first-missing-path")
		secondCreatedPath := filepath.Join(root, "second-missing-path")

		prober := classification.NewEventClassificationProber(func(string) bool {
			return false
		})
		prober.StatPathFn = func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		}

		plan := buildPreClassificationPlanFromEventsForTest(
			[]fsnotify.Event{
				{Name: firstCreatedPath, Op: fsnotify.Create},
				{Name: firstCreatedPath, Op: fsnotify.Rename},
				{Name: secondCreatedPath, Op: fsnotify.Create},
			},
			prober,
		)

		if plan.ConfigChanged {
			t.Fatalf("expected ConfigChanged=false for non-config events, got %#v", plan)
		}
		expectedAddDirectoryWatchPaths := []string{
			firstCreatedPath,
			secondCreatedPath,
		}
		if !reflect.DeepEqual(plan.AddDirectoryWatchPaths, expectedAddDirectoryWatchPaths) {
			t.Fatalf(
				"AddDirectoryWatchPaths=%#v, want %#v",
				plan.AddDirectoryWatchPaths,
				expectedAddDirectoryWatchPaths,
			)
		}
		if len(plan.EventsToClassify) != 3 {
			t.Fatalf("expected all three events to be classified, got %#v", plan.EventsToClassify)
		}
	})
}

func TestDeriveWatcherEventPreClassificationStepResult(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "wave.config.json")
	directoryPath := filepath.Join(root, "assets")
	regularFilePath := filepath.Join(root, "app.go")
	missingPath := filepath.Join(root, "missing.txt")

	if err := os.WriteFile(configPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed writing config file: %v", err)
	}
	if err := os.MkdirAll(directoryPath, 0o755); err != nil {
		t.Fatalf("failed creating directory path: %v", err)
	}
	if err := os.WriteFile(regularFilePath, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed writing regular file: %v", err)
	}

	prober := classification.NewEventClassificationProber(func(path string) bool {
		return path == configPath
	})
	prober.StatPathFn = os.Stat

	testCases := []struct {
		Name               string
		Event              fsnotify.Event
		ExpectedStepResult classification.PreClassificationStepResult
	}{
		{
			Name:  "config mutation short-circuits step",
			Event: fsnotify.Event{Name: configPath, Op: fsnotify.Write},
			ExpectedStepResult: classification.PreClassificationStepResult{
				ConfigChanged: true,
			},
		},
		{
			Name:  "directory create adds watch path",
			Event: fsnotify.Event{Name: directoryPath, Op: fsnotify.Create},
			ExpectedStepResult: classification.PreClassificationStepResult{
				AddDirectoryWatchPath: directoryPath,
			},
		},
		{
			Name:  "regular file write classifies event",
			Event: fsnotify.Event{Name: regularFilePath, Op: fsnotify.Write},
			ExpectedStepResult: classification.PreClassificationStepResult{
				ClassifyEvent: true,
			},
		},
		{
			Name:  "create with missing stat adds watch and classifies",
			Event: fsnotify.Event{Name: missingPath, Op: fsnotify.Create},
			ExpectedStepResult: classification.PreClassificationStepResult{
				AddDirectoryWatchPath: missingPath,
				ClassifyEvent:         true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			stepResult := derivePreClassificationStepResultForTest(
				testCase.Event,
				prober,
			)
			if !reflect.DeepEqual(stepResult, testCase.ExpectedStepResult) {
				t.Fatalf(
					"derivePreClassificationStepResultForTest()=%#v, want %#v",
					stepResult,
					testCase.ExpectedStepResult,
				)
			}
		})
	}
}

func TestWatcherEventClassificationProber_CachesConfigProbeByPath(t *testing.T) {
	configProbeCount := 0
	prober := classification.NewEventClassificationProber(func(path string) bool {
		configProbeCount++
		return path == "wave.config.json"
	})

	if !prober.ProbeIsConfigFile("wave.config.json") {
		t.Fatal("expected config probe to return true for config path")
	}
	if !prober.ProbeIsConfigFile("wave.config.json") {
		t.Fatal("expected cached config probe to return true for config path")
	}
	if configProbeCount != 1 {
		t.Fatalf("expected config probe function to be called once for repeated path, got %d", configProbeCount)
	}

	if prober.ProbeIsConfigFile("other.json") {
		t.Fatal("expected non-config path probe to return false")
	}
	if configProbeCount != 2 {
		t.Fatalf("expected config probe function to be called once per unique path, got %d", configProbeCount)
	}
	if len(prober.PathProbeSnapshotByPath) != 2 {
		t.Fatalf("expected one path probe snapshot per unique path, got %d", len(prober.PathProbeSnapshotByPath))
	}
}

func TestWatcherEventClassificationProber_CachesDirectoryProbeByPath(t *testing.T) {
	root := t.TempDir()
	directoryPath := filepath.Join(root, "assets")
	if err := os.MkdirAll(directoryPath, 0o755); err != nil {
		t.Fatalf("failed creating test directory: %v", err)
	}

	statProbeCount := 0
	prober := classification.NewEventClassificationProber(nil)
	prober.StatPathFn = func(path string) (os.FileInfo, error) {
		statProbeCount++
		return os.Stat(path)
	}

	directoryProbe := prober.ProbeEventDirectoryStatus(directoryPath)
	if !directoryProbe.StatProbeSucceeded || !directoryProbe.IsDirectory {
		t.Fatal("expected directory probe to return true for directory path")
	}
	directoryProbe = prober.ProbeEventDirectoryStatus(directoryPath)
	if !directoryProbe.StatProbeSucceeded || !directoryProbe.IsDirectory {
		t.Fatal("expected cached directory probe to return true for directory path")
	}
	if statProbeCount != 1 {
		t.Fatalf("expected directory stat probe to run once for repeated directory path, got %d", statProbeCount)
	}

	missingPath := filepath.Join(root, "missing")
	missingPathProbe := prober.ProbeEventDirectoryStatus(missingPath)
	if missingPathProbe.StatProbeSucceeded || missingPathProbe.IsDirectory {
		t.Fatalf("expected missing path probe to report non-directory with failed stat, got %#v", missingPathProbe)
	}
	missingPathProbe = prober.ProbeEventDirectoryStatus(missingPath)
	if missingPathProbe.StatProbeSucceeded || missingPathProbe.IsDirectory {
		t.Fatalf("expected cached missing path probe to report non-directory with failed stat, got %#v", missingPathProbe)
	}
	if statProbeCount != 2 {
		t.Fatalf("expected missing path stat probe to run once, total stat probes=%d", statProbeCount)
	}
	if len(prober.PathProbeSnapshotByPath) != 2 {
		t.Fatalf("expected one path probe snapshot per unique path, got %d", len(prober.PathProbeSnapshotByPath))
	}
}

func TestWatcherEventClassificationProber_SharesSingleSnapshotAcrossProbeTypes(
	t *testing.T,
) {
	root := t.TempDir()
	configPath := filepath.Join(root, "wave.config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed creating test config file: %v", err)
	}

	configProbeCount := 0
	statProbeCount := 0
	prober := classification.NewEventClassificationProber(func(path string) bool {
		configProbeCount++
		return path == configPath
	})
	prober.StatPathFn = func(path string) (os.FileInfo, error) {
		statProbeCount++
		return os.Stat(path)
	}

	if !prober.ProbeIsConfigFile(configPath) {
		t.Fatal("expected config probe to return true for config path")
	}
	directoryProbeResult := prober.ProbeEventDirectoryStatus(configPath)
	if !directoryProbeResult.StatProbeSucceeded || directoryProbeResult.IsDirectory {
		t.Fatalf(
			"expected config file directory probe to report existing non-directory path, got %#v",
			directoryProbeResult,
		)
	}
	if configProbeCount != 1 {
		t.Fatalf("expected config probe function called once, got %d", configProbeCount)
	}
	if statProbeCount != 1 {
		t.Fatalf("expected stat probe function called once, got %d", statProbeCount)
	}

	if len(prober.PathProbeSnapshotByPath) != 1 {
		t.Fatalf(
			"expected one shared path probe snapshot for config path, got %d",
			len(prober.PathProbeSnapshotByPath),
		)
	}

	pathProbeSnapshot := prober.PathProbeSnapshotByPath[configPath]
	if pathProbeSnapshot == nil {
		t.Fatalf("expected path probe snapshot for config path %q", configPath)
	}
	if !pathProbeSnapshot.HasConfigFileProbe || !pathProbeSnapshot.IsConfigFile {
		t.Fatalf(
			"expected path probe snapshot to cache positive config probe result, got %#v",
			pathProbeSnapshot,
		)
	}
	if !pathProbeSnapshot.HasDirectoryProbe ||
		!pathProbeSnapshot.DirectoryProbeState.StatProbeSucceeded ||
		pathProbeSnapshot.DirectoryProbeState.IsDirectory {
		t.Fatalf(
			"expected path probe snapshot to cache directory probe result, got %#v",
			pathProbeSnapshot,
		)
	}

	if !prober.ProbeIsConfigFile(configPath) {
		t.Fatal("expected cached config probe to remain true for config path")
	}
	directoryProbeResult = prober.ProbeEventDirectoryStatus(configPath)
	if !directoryProbeResult.StatProbeSucceeded || directoryProbeResult.IsDirectory {
		t.Fatalf(
			"expected cached directory probe to remain existing non-directory path, got %#v",
			directoryProbeResult,
		)
	}
	if configProbeCount != 1 {
		t.Fatalf("expected config probe count to remain cached at one call, got %d", configProbeCount)
	}
	if statProbeCount != 1 {
		t.Fatalf("expected stat probe count to remain cached at one call, got %d", statProbeCount)
	}
}

func TestDeriveWatcherEventPostClassificationDecision(t *testing.T) {
	testCases := []struct {
		Name             string
		ClassifiedEvent  devserver.ClassifiedEvent
		ExpectedDecision classification.PostClassificationDecision
	}{
		{
			Name: "ignored classified event is excluded",
			ClassifiedEvent: devserver.ClassifiedEvent{
				Ignored: true,
			},
			ExpectedDecision: classification.PostClassificationDecision{},
		},
		{
			Name: "chmod-only classified event is excluded",
			ClassifiedEvent: devserver.ClassifiedEvent{
				ChmodOnly: true,
			},
			ExpectedDecision: classification.PostClassificationDecision{},
		},
		{
			Name: "included classified event is retained",
			ClassifiedEvent: devserver.ClassifiedEvent{
				Ignored:   false,
				ChmodOnly: false,
			},
			ExpectedDecision: classification.PostClassificationDecision{
				IncludeClassifiedEvent: true,
			},
		},
		{
			Name: "ignored chmod-only classified event is excluded",
			ClassifiedEvent: devserver.ClassifiedEvent{
				Ignored:   true,
				ChmodOnly: true,
			},
			ExpectedDecision: classification.PostClassificationDecision{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			decision := classification.DerivePostClassificationDecision(
				testCase.ClassifiedEvent.Ignored,
				testCase.ClassifiedEvent.ChmodOnly,
			)
			if !reflect.DeepEqual(decision, testCase.ExpectedDecision) {
				t.Fatalf(
					"classification.DerivePostClassificationDecision()=%#v, want %#v",
					decision,
					testCase.ExpectedDecision,
				)
			}
		})
	}
}

func TestFilterClassifiedEventsForProcessingByPostClassificationDecision(t *testing.T) {
	t.Run("returns nil for empty input", func(t *testing.T) {
		filteredClassifiedEvents := devserver.FilterClassifiedEventsForProcessingByPostClassificationDecision(nil)
		if filteredClassifiedEvents != nil {
			t.Fatalf("expected nil filtered classified events, got %#v", filteredClassifiedEvents)
		}
	})

	t.Run("filters ignored and chmod-only classified events", func(t *testing.T) {
		inputClassifiedEvents := []devserver.ClassifiedEvent{
			{
				Event: fsnotify.Event{
					Name: "first.txt",
					Op:   fsnotify.Write,
				},
				Ignored: false,
			},
			{
				Event: fsnotify.Event{
					Name: "ignored.txt",
					Op:   fsnotify.Write,
				},
				Ignored: true,
			},
			{
				Event: fsnotify.Event{
					Name: "chmod.txt",
					Op:   fsnotify.Chmod,
				},
				ChmodOnly: true,
			},
			{
				Event: fsnotify.Event{
					Name: "second.txt",
					Op:   fsnotify.Write,
				},
				Ignored:   false,
				ChmodOnly: false,
			},
		}

		filteredClassifiedEvents := devserver.FilterClassifiedEventsForProcessingByPostClassificationDecision(
			inputClassifiedEvents,
		)
		if len(filteredClassifiedEvents) != 2 {
			t.Fatalf("filtered classified event count=%d, want 2", len(filteredClassifiedEvents))
		}
		if filteredClassifiedEvents[0].Event.Name != "first.txt" {
			t.Fatalf("filtered event[0]=%q, want first.txt", filteredClassifiedEvents[0].Event.Name)
		}
		if filteredClassifiedEvents[1].Event.Name != "second.txt" {
			t.Fatalf("filtered event[1]=%q, want second.txt", filteredClassifiedEvents[1].Event.Name)
		}
	})
}

func TestShouldLogWatcherAddDirectoryError(t *testing.T) {
	testCases := []struct {
		Name      string
		Err       error
		ShouldLog bool
	}{
		{
			Name:      "nil error does not log",
			Err:       nil,
			ShouldLog: false,
		},
		{
			Name:      "not-exist error does not log",
			Err:       fs.ErrNotExist,
			ShouldLog: false,
		},
		{
			Name: "not-directory path error does not log",
			Err: &os.PathError{
				Op:   "open",
				Path: "file.txt",
				Err:  syscall.ENOTDIR,
			},
			ShouldLog: false,
		},
		{
			Name: "permission error logs",
			Err: &os.PathError{
				Op:   "open",
				Path: "secure-dir",
				Err:  syscall.EACCES,
			},
			ShouldLog: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			shouldLog := classification.ShouldLogAddDirectoryWatchError(testCase.Err)
			if shouldLog != testCase.ShouldLog {
				t.Fatalf(
					"classification.ShouldLogAddDirectoryWatchError(%v)=%v, want %v",
					testCase.Err,
					shouldLog,
					testCase.ShouldLog,
				)
			}
		})
	}
}
