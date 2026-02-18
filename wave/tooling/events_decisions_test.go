package tooling

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/tooling/internal/watchereventclassification"
)

func buildPreClassificationPlanFromEventsForTest(
	events []fsnotify.Event,
	eventClassificationProber *watcherEventClassificationProber,
) watchereventclassification.PreClassificationPlan {
	return watchereventclassification.BuildPreClassificationPlanFromEvents(
		events,
		func(path string) bool {
			return eventClassificationProber.probeIsConfigFile(path)
		},
		func(path string) watchereventclassification.DirectoryProbeResult {
			directoryProbeResult := eventClassificationProber.probeEventDirectoryStatus(path)
			return watchereventclassification.DirectoryProbeResult{
				StatProbeSucceeded: directoryProbeResult.statProbeSucceeded,
				IsDirectory:        directoryProbeResult.isDirectory,
			}
		},
	)
}

func derivePreClassificationStepResultForTest(
	event fsnotify.Event,
	eventClassificationProber *watcherEventClassificationProber,
) watchereventclassification.PreClassificationStepResult {
	return watchereventclassification.DerivePreClassificationStepResult(
		event,
		func(path string) bool {
			return eventClassificationProber.probeIsConfigFile(path)
		},
		func(path string) watchereventclassification.DirectoryProbeResult {
			directoryProbeResult := eventClassificationProber.probeEventDirectoryStatus(path)
			return watchereventclassification.DirectoryProbeResult{
				StatProbeSucceeded: directoryProbeResult.statProbeSucceeded,
				IsDirectory:        directoryProbeResult.isDirectory,
			}
		},
	)
}

func TestDeriveWatcherEventPreClassificationDecision(t *testing.T) {
	testCases := []struct {
		name                        string
		event                       fsnotify.Event
		isConfigFile                bool
		eventIsDirectory            bool
		eventPathStatProbeSucceeded bool
		expectedDecision            watchereventclassification.PreClassificationDecision
	}{
		{
			name:                        "config write triggers config changed",
			event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Write},
			isConfigFile:                true,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				ConfigChanged: true,
			},
		},
		{
			name:                        "config create triggers config changed",
			event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Create},
			isConfigFile:                true,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				ConfigChanged: true,
			},
		},
		{
			name:                        "config remove triggers config changed",
			event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Remove},
			isConfigFile:                true,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				ConfigChanged: true,
			},
		},
		{
			name:                        "config rename triggers config changed",
			event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Rename},
			isConfigFile:                true,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				ConfigChanged: true,
			},
		},
		{
			name:                        "config chmod does not trigger config changed",
			event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Chmod},
			isConfigFile:                true,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				ClassifyEvent: true,
			},
		},
		{
			name:                        "directory create adds watch",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Create},
			isConfigFile:                false,
			eventIsDirectory:            true,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				AddDirectoryWatch: true,
			},
		},
		{
			name:                        "directory rename adds watch",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Rename},
			isConfigFile:                false,
			eventIsDirectory:            true,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				AddDirectoryWatch: true,
			},
		},
		{
			name:                        "directory write does not classify",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Write},
			isConfigFile:                false,
			eventIsDirectory:            true,
			eventPathStatProbeSucceeded: true,
			expectedDecision:            watchereventclassification.PreClassificationDecision{},
		},
		{
			name:                        "regular file write classifies",
			event:                       fsnotify.Event{Name: "app.go", Op: fsnotify.Write},
			isConfigFile:                false,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				ClassifyEvent: true,
			},
		},
		{
			name:                        "create with failed stat still adds watch and classifies",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Create},
			isConfigFile:                false,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				AddDirectoryWatch: true,
				ClassifyEvent:     true,
			},
		},
		{
			name:                        "rename with failed stat still adds watch and classifies",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Rename},
			isConfigFile:                false,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				AddDirectoryWatch: true,
				ClassifyEvent:     true,
			},
		},
		{
			name:                        "write with failed stat classifies without directory watch",
			event:                       fsnotify.Event{Name: "file.txt", Op: fsnotify.Write},
			isConfigFile:                false,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				ClassifyEvent: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			decision := watchereventclassification.DerivePreClassificationDecision(
				testCase.event,
				testCase.isConfigFile,
				testCase.eventIsDirectory,
				testCase.eventPathStatProbeSucceeded,
			)
			if !reflect.DeepEqual(decision, testCase.expectedDecision) {
				t.Fatalf(
					"watchereventclassification.DerivePreClassificationDecision()=%#v, want %#v",
					decision,
					testCase.expectedDecision,
				)
			}
		})
	}
}

func TestDeriveWatcherEventPreClassificationDecisionForNonConfigEvent(t *testing.T) {
	testCases := []struct {
		name                        string
		event                       fsnotify.Event
		eventIsDirectory            bool
		eventPathStatProbeSucceeded bool
		expectedDecision            watchereventclassification.PreClassificationDecision
	}{
		{
			name:                        "directory create adds watch",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Create},
			eventIsDirectory:            true,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				AddDirectoryWatch: true,
			},
		},
		{
			name:                        "directory write is skipped",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Write},
			eventIsDirectory:            true,
			eventPathStatProbeSucceeded: true,
			expectedDecision:            watchereventclassification.PreClassificationDecision{},
		},
		{
			name:                        "missing-stat rename adds watch and classifies",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Rename},
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				AddDirectoryWatch: true,
				ClassifyEvent:     true,
			},
		},
		{
			name:                        "regular file write classifies",
			event:                       fsnotify.Event{Name: "app.go", Op: fsnotify.Write},
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watchereventclassification.PreClassificationDecision{
				ClassifyEvent: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			decision := watchereventclassification.DerivePreClassificationDecisionForNonConfigEvent(
				testCase.event,
				testCase.eventIsDirectory,
				testCase.eventPathStatProbeSucceeded,
			)
			if !reflect.DeepEqual(decision, testCase.expectedDecision) {
				t.Fatalf(
					"watchereventclassification.DerivePreClassificationDecisionForNonConfigEvent()=%#v, want %#v",
					decision,
					testCase.expectedDecision,
				)
			}
		})
	}
}

func TestBuildWatcherEventPreClassificationPlanFromEvents(t *testing.T) {
	t.Run("empty events returns empty plan", func(t *testing.T) {
		plan := buildPreClassificationPlanFromEventsForTest(
			nil,
			newWatcherEventClassificationProber(nil),
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
		prober := newWatcherEventClassificationProber(func(path string) bool {
			configProbeCount++
			return path == "wave.config.json"
		})
		prober.statPathFn = func(_ string) (os.FileInfo, error) {
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
		prober := newWatcherEventClassificationProber(func(path string) bool {
			configProbeCount++
			return path == "wave.config.json"
		})
		prober.statPathFn = func(path string) (os.FileInfo, error) {
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

		prober := newWatcherEventClassificationProber(func(string) bool {
			return false
		})
		prober.statPathFn = func(path string) (os.FileInfo, error) {
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

		prober := newWatcherEventClassificationProber(func(string) bool {
			return false
		})
		prober.statPathFn = func(string) (os.FileInfo, error) {
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

	prober := newWatcherEventClassificationProber(func(path string) bool {
		return path == configPath
	})
	prober.statPathFn = os.Stat

	testCases := []struct {
		name               string
		event              fsnotify.Event
		expectedStepResult watchereventclassification.PreClassificationStepResult
	}{
		{
			name:  "config mutation short-circuits step",
			event: fsnotify.Event{Name: configPath, Op: fsnotify.Write},
			expectedStepResult: watchereventclassification.PreClassificationStepResult{
				ConfigChanged: true,
			},
		},
		{
			name:  "directory create adds watch path",
			event: fsnotify.Event{Name: directoryPath, Op: fsnotify.Create},
			expectedStepResult: watchereventclassification.PreClassificationStepResult{
				AddDirectoryWatchPath: directoryPath,
			},
		},
		{
			name:  "regular file write classifies event",
			event: fsnotify.Event{Name: regularFilePath, Op: fsnotify.Write},
			expectedStepResult: watchereventclassification.PreClassificationStepResult{
				ClassifyEvent: true,
			},
		},
		{
			name:  "create with missing stat adds watch and classifies",
			event: fsnotify.Event{Name: missingPath, Op: fsnotify.Create},
			expectedStepResult: watchereventclassification.PreClassificationStepResult{
				AddDirectoryWatchPath: missingPath,
				ClassifyEvent:         true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stepResult := derivePreClassificationStepResultForTest(
				testCase.event,
				prober,
			)
			if !reflect.DeepEqual(stepResult, testCase.expectedStepResult) {
				t.Fatalf(
					"derivePreClassificationStepResultForTest()=%#v, want %#v",
					stepResult,
					testCase.expectedStepResult,
				)
			}
		})
	}
}

func TestWatcherEventClassificationProber_CachesConfigProbeByPath(t *testing.T) {
	configProbeCount := 0
	prober := newWatcherEventClassificationProber(func(path string) bool {
		configProbeCount++
		return path == "wave.config.json"
	})

	if !prober.probeIsConfigFile("wave.config.json") {
		t.Fatal("expected config probe to return true for config path")
	}
	if !prober.probeIsConfigFile("wave.config.json") {
		t.Fatal("expected cached config probe to return true for config path")
	}
	if configProbeCount != 1 {
		t.Fatalf("expected config probe function to be called once for repeated path, got %d", configProbeCount)
	}

	if prober.probeIsConfigFile("other.json") {
		t.Fatal("expected non-config path probe to return false")
	}
	if configProbeCount != 2 {
		t.Fatalf("expected config probe function to be called once per unique path, got %d", configProbeCount)
	}
	if len(prober.pathProbeSnapshotByPath) != 2 {
		t.Fatalf("expected one path probe snapshot per unique path, got %d", len(prober.pathProbeSnapshotByPath))
	}
}

func TestWatcherEventClassificationProber_CachesDirectoryProbeByPath(t *testing.T) {
	root := t.TempDir()
	directoryPath := filepath.Join(root, "assets")
	if err := os.MkdirAll(directoryPath, 0o755); err != nil {
		t.Fatalf("failed creating test directory: %v", err)
	}

	statProbeCount := 0
	prober := newWatcherEventClassificationProber(nil)
	prober.statPathFn = func(path string) (os.FileInfo, error) {
		statProbeCount++
		return os.Stat(path)
	}

	directoryProbe := prober.probeEventDirectoryStatus(directoryPath)
	if !directoryProbe.statProbeSucceeded || !directoryProbe.isDirectory {
		t.Fatal("expected directory probe to return true for directory path")
	}
	directoryProbe = prober.probeEventDirectoryStatus(directoryPath)
	if !directoryProbe.statProbeSucceeded || !directoryProbe.isDirectory {
		t.Fatal("expected cached directory probe to return true for directory path")
	}
	if statProbeCount != 1 {
		t.Fatalf("expected directory stat probe to run once for repeated directory path, got %d", statProbeCount)
	}

	missingPath := filepath.Join(root, "missing")
	missingPathProbe := prober.probeEventDirectoryStatus(missingPath)
	if missingPathProbe.statProbeSucceeded || missingPathProbe.isDirectory {
		t.Fatalf("expected missing path probe to report non-directory with failed stat, got %#v", missingPathProbe)
	}
	missingPathProbe = prober.probeEventDirectoryStatus(missingPath)
	if missingPathProbe.statProbeSucceeded || missingPathProbe.isDirectory {
		t.Fatalf("expected cached missing path probe to report non-directory with failed stat, got %#v", missingPathProbe)
	}
	if statProbeCount != 2 {
		t.Fatalf("expected missing path stat probe to run once, total stat probes=%d", statProbeCount)
	}
	if len(prober.pathProbeSnapshotByPath) != 2 {
		t.Fatalf("expected one path probe snapshot per unique path, got %d", len(prober.pathProbeSnapshotByPath))
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
	prober := newWatcherEventClassificationProber(func(path string) bool {
		configProbeCount++
		return path == configPath
	})
	prober.statPathFn = func(path string) (os.FileInfo, error) {
		statProbeCount++
		return os.Stat(path)
	}

	if !prober.probeIsConfigFile(configPath) {
		t.Fatal("expected config probe to return true for config path")
	}
	directoryProbeResult := prober.probeEventDirectoryStatus(configPath)
	if !directoryProbeResult.statProbeSucceeded || directoryProbeResult.isDirectory {
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

	if len(prober.pathProbeSnapshotByPath) != 1 {
		t.Fatalf(
			"expected one shared path probe snapshot for config path, got %d",
			len(prober.pathProbeSnapshotByPath),
		)
	}

	pathProbeSnapshot := prober.pathProbeSnapshotByPath[configPath]
	if pathProbeSnapshot == nil {
		t.Fatalf("expected path probe snapshot for config path %q", configPath)
	}
	if !pathProbeSnapshot.hasConfigFileProbe || !pathProbeSnapshot.isConfigFile {
		t.Fatalf(
			"expected path probe snapshot to cache positive config probe result, got %#v",
			pathProbeSnapshot,
		)
	}
	if !pathProbeSnapshot.hasDirectoryProbe ||
		!pathProbeSnapshot.directoryProbeState.statProbeSucceeded ||
		pathProbeSnapshot.directoryProbeState.isDirectory {
		t.Fatalf(
			"expected path probe snapshot to cache directory probe result, got %#v",
			pathProbeSnapshot,
		)
	}

	if !prober.probeIsConfigFile(configPath) {
		t.Fatal("expected cached config probe to remain true for config path")
	}
	directoryProbeResult = prober.probeEventDirectoryStatus(configPath)
	if !directoryProbeResult.statProbeSucceeded || directoryProbeResult.isDirectory {
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
		name             string
		classifiedEvent  classifiedEvent
		expectedDecision watchereventclassification.PostClassificationDecision
	}{
		{
			name: "ignored classified event is excluded",
			classifiedEvent: classifiedEvent{
				ignored: true,
			},
			expectedDecision: watchereventclassification.PostClassificationDecision{},
		},
		{
			name: "chmod-only classified event is excluded",
			classifiedEvent: classifiedEvent{
				chmodOnly: true,
			},
			expectedDecision: watchereventclassification.PostClassificationDecision{},
		},
		{
			name: "included classified event is retained",
			classifiedEvent: classifiedEvent{
				ignored:   false,
				chmodOnly: false,
			},
			expectedDecision: watchereventclassification.PostClassificationDecision{
				IncludeClassifiedEvent: true,
			},
		},
		{
			name: "ignored chmod-only classified event is excluded",
			classifiedEvent: classifiedEvent{
				ignored:   true,
				chmodOnly: true,
			},
			expectedDecision: watchereventclassification.PostClassificationDecision{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			decision := watchereventclassification.DerivePostClassificationDecision(
				testCase.classifiedEvent.ignored,
				testCase.classifiedEvent.chmodOnly,
			)
			if !reflect.DeepEqual(decision, testCase.expectedDecision) {
				t.Fatalf(
					"watchereventclassification.DerivePostClassificationDecision()=%#v, want %#v",
					decision,
					testCase.expectedDecision,
				)
			}
		})
	}
}

func TestFilterClassifiedEventsForProcessingByPostClassificationDecision(t *testing.T) {
	t.Run("returns nil for empty input", func(t *testing.T) {
		filteredClassifiedEvents := filterClassifiedEventsForProcessingByPostClassificationDecision(nil)
		if filteredClassifiedEvents != nil {
			t.Fatalf("expected nil filtered classified events, got %#v", filteredClassifiedEvents)
		}
	})

	t.Run("filters ignored and chmod-only classified events", func(t *testing.T) {
		inputClassifiedEvents := []classifiedEvent{
			{
				event: fsnotify.Event{
					Name: "first.txt",
					Op:   fsnotify.Write,
				},
				ignored: false,
			},
			{
				event: fsnotify.Event{
					Name: "ignored.txt",
					Op:   fsnotify.Write,
				},
				ignored: true,
			},
			{
				event: fsnotify.Event{
					Name: "chmod.txt",
					Op:   fsnotify.Chmod,
				},
				chmodOnly: true,
			},
			{
				event: fsnotify.Event{
					Name: "second.txt",
					Op:   fsnotify.Write,
				},
				ignored:   false,
				chmodOnly: false,
			},
		}

		filteredClassifiedEvents := filterClassifiedEventsForProcessingByPostClassificationDecision(
			inputClassifiedEvents,
		)
		if len(filteredClassifiedEvents) != 2 {
			t.Fatalf("filtered classified event count=%d, want 2", len(filteredClassifiedEvents))
		}
		if filteredClassifiedEvents[0].event.Name != "first.txt" {
			t.Fatalf("filtered event[0]=%q, want first.txt", filteredClassifiedEvents[0].event.Name)
		}
		if filteredClassifiedEvents[1].event.Name != "second.txt" {
			t.Fatalf("filtered event[1]=%q, want second.txt", filteredClassifiedEvents[1].event.Name)
		}
	})
}

func TestShouldLogWatcherAddDirectoryError(t *testing.T) {
	testCases := []struct {
		name      string
		err       error
		shouldLog bool
	}{
		{
			name:      "nil error does not log",
			err:       nil,
			shouldLog: false,
		},
		{
			name:      "not-exist error does not log",
			err:       fs.ErrNotExist,
			shouldLog: false,
		},
		{
			name: "not-directory path error does not log",
			err: &os.PathError{
				Op:   "open",
				Path: "file.txt",
				Err:  syscall.ENOTDIR,
			},
			shouldLog: false,
		},
		{
			name: "permission error logs",
			err: &os.PathError{
				Op:   "open",
				Path: "secure-dir",
				Err:  syscall.EACCES,
			},
			shouldLog: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shouldLog := watchereventclassification.ShouldLogAddDirectoryWatchError(testCase.err)
			if shouldLog != testCase.shouldLog {
				t.Fatalf(
					"watchereventclassification.ShouldLogAddDirectoryWatchError(%v)=%v, want %v",
					testCase.err,
					shouldLog,
					testCase.shouldLog,
				)
			}
		})
	}
}
