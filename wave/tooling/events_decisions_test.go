package tooling

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestDeriveWatcherEventPreClassificationDecision(t *testing.T) {
	testCases := []struct {
		name                        string
		event                       fsnotify.Event
		isConfigFile                bool
		eventIsDirectory            bool
		eventPathStatProbeSucceeded bool
		expectedDecision            watcherEventPreClassificationDecision
	}{
		{
			name:                        "config write triggers config changed",
			event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Write},
			isConfigFile:                true,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watcherEventPreClassificationDecision{
				configChanged: true,
			},
		},
		{
			name:                        "config create triggers config changed",
			event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Create},
			isConfigFile:                true,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watcherEventPreClassificationDecision{
				configChanged: true,
			},
		},
		{
			name:                        "config remove triggers config changed",
			event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Remove},
			isConfigFile:                true,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watcherEventPreClassificationDecision{
				configChanged: true,
			},
		},
		{
			name:                        "config rename triggers config changed",
			event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Rename},
			isConfigFile:                true,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watcherEventPreClassificationDecision{
				configChanged: true,
			},
		},
		{
			name:                        "config chmod does not trigger config changed",
			event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Chmod},
			isConfigFile:                true,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watcherEventPreClassificationDecision{
				classifyEvent: true,
			},
		},
		{
			name:                        "directory create adds watch",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Create},
			isConfigFile:                false,
			eventIsDirectory:            true,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watcherEventPreClassificationDecision{
				addDirectoryWatch: true,
			},
		},
		{
			name:                        "directory rename adds watch",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Rename},
			isConfigFile:                false,
			eventIsDirectory:            true,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watcherEventPreClassificationDecision{
				addDirectoryWatch: true,
			},
		},
		{
			name:                        "directory write does not classify",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Write},
			isConfigFile:                false,
			eventIsDirectory:            true,
			eventPathStatProbeSucceeded: true,
			expectedDecision:            watcherEventPreClassificationDecision{},
		},
		{
			name:                        "regular file write classifies",
			event:                       fsnotify.Event{Name: "app.go", Op: fsnotify.Write},
			isConfigFile:                false,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watcherEventPreClassificationDecision{
				classifyEvent: true,
			},
		},
		{
			name:                        "create with failed stat still adds watch and classifies",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Create},
			isConfigFile:                false,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watcherEventPreClassificationDecision{
				addDirectoryWatch: true,
				classifyEvent:     true,
			},
		},
		{
			name:                        "rename with failed stat still adds watch and classifies",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Rename},
			isConfigFile:                false,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watcherEventPreClassificationDecision{
				addDirectoryWatch: true,
				classifyEvent:     true,
			},
		},
		{
			name:                        "write with failed stat classifies without directory watch",
			event:                       fsnotify.Event{Name: "file.txt", Op: fsnotify.Write},
			isConfigFile:                false,
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watcherEventPreClassificationDecision{
				classifyEvent: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			decision := deriveWatcherEventPreClassificationDecision(
				testCase.event,
				testCase.isConfigFile,
				testCase.eventIsDirectory,
				testCase.eventPathStatProbeSucceeded,
			)
			if !reflect.DeepEqual(decision, testCase.expectedDecision) {
				t.Fatalf(
					"deriveWatcherEventPreClassificationDecision()=%#v, want %#v",
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
		expectedDecision            watcherEventPreClassificationDecision
	}{
		{
			name:                        "directory create adds watch",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Create},
			eventIsDirectory:            true,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watcherEventPreClassificationDecision{
				addDirectoryWatch: true,
			},
		},
		{
			name:                        "directory write is skipped",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Write},
			eventIsDirectory:            true,
			eventPathStatProbeSucceeded: true,
			expectedDecision:            watcherEventPreClassificationDecision{},
		},
		{
			name:                        "missing-stat rename adds watch and classifies",
			event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Rename},
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: false,
			expectedDecision: watcherEventPreClassificationDecision{
				addDirectoryWatch: true,
				classifyEvent:     true,
			},
		},
		{
			name:                        "regular file write classifies",
			event:                       fsnotify.Event{Name: "app.go", Op: fsnotify.Write},
			eventIsDirectory:            false,
			eventPathStatProbeSucceeded: true,
			expectedDecision: watcherEventPreClassificationDecision{
				classifyEvent: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			decision := deriveWatcherEventPreClassificationDecisionForNonConfigEvent(
				testCase.event,
				testCase.eventIsDirectory,
				testCase.eventPathStatProbeSucceeded,
			)
			if !reflect.DeepEqual(decision, testCase.expectedDecision) {
				t.Fatalf(
					"deriveWatcherEventPreClassificationDecisionForNonConfigEvent()=%#v, want %#v",
					decision,
					testCase.expectedDecision,
				)
			}
		})
	}
}

func TestBuildWatcherEventPreClassificationPlanFromEvents(t *testing.T) {
	t.Run("empty events returns empty plan", func(t *testing.T) {
		plan := buildWatcherEventPreClassificationPlanFromEvents(
			nil,
			newWatcherEventClassificationProber(nil),
		)
		if plan.configChanged {
			t.Fatalf("expected configChanged=false for empty events, got %#v", plan)
		}
		if len(plan.addDirectoryWatchPaths) != 0 {
			t.Fatalf("expected no addDirectoryWatchPaths for empty events, got %#v", plan.addDirectoryWatchPaths)
		}
		if len(plan.eventsToClassify) != 0 {
			t.Fatalf("expected no eventsToClassify for empty events, got %#v", plan.eventsToClassify)
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

		plan := buildWatcherEventPreClassificationPlanFromEvents(
			[]fsnotify.Event{
				{Name: "notes.txt", Op: fsnotify.Write},
				{Name: "wave.config.json", Op: fsnotify.Rename},
				{Name: "later.txt", Op: fsnotify.Write},
			},
			prober,
		)

		if !plan.configChanged {
			t.Fatal("expected configChanged=true when config mutation event is present")
		}
		if len(plan.addDirectoryWatchPaths) != 0 {
			t.Fatalf(
				"expected no addDirectoryWatchPaths when config mutation short-circuits, got %#v",
				plan.addDirectoryWatchPaths,
			)
		}
		if len(plan.eventsToClassify) != 0 {
			t.Fatalf(
				"expected no eventsToClassify when config mutation short-circuits, got %#v",
				plan.eventsToClassify,
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

		plan := buildWatcherEventPreClassificationPlanFromEvents(
			[]fsnotify.Event{
				{Name: "notes.txt", Op: fsnotify.Write},
				{Name: "notes.txt", Op: fsnotify.Create},
				{Name: "wave.config.json", Op: fsnotify.Chmod},
			},
			prober,
		)

		if plan.configChanged {
			t.Fatalf("expected configChanged=false when config mutation is absent, got %#v", plan)
		}
		if len(plan.addDirectoryWatchPaths) != 1 {
			t.Fatalf("expected one addDirectoryWatchPath, got %#v", plan.addDirectoryWatchPaths)
		}
		if len(plan.eventsToClassify) != 3 {
			t.Fatalf("expected three eventsToClassify, got %#v", plan.eventsToClassify)
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

		plan := buildWatcherEventPreClassificationPlanFromEvents(
			[]fsnotify.Event{
				{Name: directoryPath, Op: fsnotify.Write},
				{Name: filepath.Join(root, "new.txt"), Op: fsnotify.Create},
				{Name: regularFilePath, Op: fsnotify.Write},
			},
			prober,
		)

		if plan.configChanged {
			t.Fatalf("expected configChanged=false for non-config events, got %#v", plan)
		}
		if len(plan.addDirectoryWatchPaths) != 1 {
			t.Fatalf("expected one addDirectoryWatchPath, got %#v", plan.addDirectoryWatchPaths)
		}
		if len(plan.eventsToClassify) != 2 {
			t.Fatalf("expected two eventsToClassify (directory write skipped), got %#v", plan.eventsToClassify)
		}

		firstAddDirectoryPath := plan.addDirectoryWatchPaths[0]
		if firstAddDirectoryPath != filepath.Join(root, "new.txt") {
			t.Fatalf("expected addDirectoryWatch path to be new.txt create path, got %#v", firstAddDirectoryPath)
		}

		firstEventToClassify := plan.eventsToClassify[0]
		if firstEventToClassify.Name != filepath.Join(root, "new.txt") {
			t.Fatalf("expected first eventToClassify to be new.txt create, got %#v", firstEventToClassify)
		}

		secondEventToClassify := plan.eventsToClassify[1]
		if secondEventToClassify.Name != regularFilePath {
			t.Fatalf("expected second eventToClassify to be regular file write, got %#v", secondEventToClassify)
		}
	})
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
		expectedDecision watcherEventPostClassificationDecision
	}{
		{
			name: "ignored classified event is excluded",
			classifiedEvent: classifiedEvent{
				ignored: true,
			},
			expectedDecision: watcherEventPostClassificationDecision{},
		},
		{
			name: "chmod-only classified event is excluded",
			classifiedEvent: classifiedEvent{
				chmodOnly: true,
			},
			expectedDecision: watcherEventPostClassificationDecision{},
		},
		{
			name: "included classified event is retained",
			classifiedEvent: classifiedEvent{
				ignored:   false,
				chmodOnly: false,
			},
			expectedDecision: watcherEventPostClassificationDecision{
				includeClassifiedEvent: true,
			},
		},
		{
			name: "ignored chmod-only classified event is excluded",
			classifiedEvent: classifiedEvent{
				ignored:   true,
				chmodOnly: true,
			},
			expectedDecision: watcherEventPostClassificationDecision{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			decision := deriveWatcherEventPostClassificationDecision(testCase.classifiedEvent)
			if !reflect.DeepEqual(decision, testCase.expectedDecision) {
				t.Fatalf(
					"deriveWatcherEventPostClassificationDecision()=%#v, want %#v",
					decision,
					testCase.expectedDecision,
				)
			}
		})
	}
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
			shouldLog := shouldLogWatcherAddDirectoryError(testCase.err)
			if shouldLog != testCase.shouldLog {
				t.Fatalf(
					"shouldLogWatcherAddDirectoryError(%v)=%v, want %v",
					testCase.err,
					shouldLog,
					testCase.shouldLog,
				)
			}
		})
	}
}
