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

func TestBuildWatcherEventPreClassificationPlan(t *testing.T) {
	t.Run("empty inputs returns empty plan", func(t *testing.T) {
		plan := buildWatcherEventPreClassificationPlan(nil)
		if plan.configChanged {
			t.Fatalf("expected configChanged=false for empty inputs, got %#v", plan)
		}
		if len(plan.plannedEvents) != 0 {
			t.Fatalf("expected no planned events for empty inputs, got %#v", plan.plannedEvents)
		}
	})

	t.Run("config change short-circuits plan", func(t *testing.T) {
		plan := buildWatcherEventPreClassificationPlan(
			[]watcherEventPreClassificationInput{
				{
					event:                       fsnotify.Event{Name: "notes.txt", Op: fsnotify.Write},
					isConfigFile:                false,
					eventIsDirectory:            false,
					eventPathStatProbeSucceeded: true,
				},
				{
					event:                       fsnotify.Event{Name: "wave.config.json", Op: fsnotify.Rename},
					isConfigFile:                true,
					eventIsDirectory:            false,
					eventPathStatProbeSucceeded: false,
				},
				{
					event:                       fsnotify.Event{Name: "later.txt", Op: fsnotify.Write},
					isConfigFile:                false,
					eventIsDirectory:            false,
					eventPathStatProbeSucceeded: true,
				},
			},
		)

		if !plan.configChanged {
			t.Fatalf("expected configChanged=true when config event is present, got %#v", plan)
		}
		if len(plan.plannedEvents) != 0 {
			t.Fatalf("expected no planned events when config change short-circuits, got %#v", plan.plannedEvents)
		}
	})

	t.Run("non-config inputs preserve pre-classification decisions", func(t *testing.T) {
		plan := buildWatcherEventPreClassificationPlan(
			[]watcherEventPreClassificationInput{
				{
					event:                       fsnotify.Event{Name: "assets", Op: fsnotify.Write},
					isConfigFile:                false,
					eventIsDirectory:            true,
					eventPathStatProbeSucceeded: true,
				},
				{
					event:                       fsnotify.Event{Name: "new.txt", Op: fsnotify.Create},
					isConfigFile:                false,
					eventIsDirectory:            false,
					eventPathStatProbeSucceeded: false,
				},
				{
					event:                       fsnotify.Event{Name: "app.go", Op: fsnotify.Write},
					isConfigFile:                false,
					eventIsDirectory:            false,
					eventPathStatProbeSucceeded: true,
				},
			},
		)

		if plan.configChanged {
			t.Fatalf("expected configChanged=false for non-config inputs, got %#v", plan)
		}
		if len(plan.plannedEvents) != 2 {
			t.Fatalf("expected exactly two planned events (directory write skipped), got %#v", plan.plannedEvents)
		}

		firstPlannedEvent := plan.plannedEvents[0]
		if firstPlannedEvent.event.Name != "new.txt" {
			t.Fatalf("expected first planned event to be new.txt, got %#v", firstPlannedEvent)
		}
		if !firstPlannedEvent.preClassificationDecision.addDirectoryWatch ||
			!firstPlannedEvent.preClassificationDecision.classifyEvent {
			t.Fatalf(
				"expected missing-stat create to add watch and classify, got %#v",
				firstPlannedEvent.preClassificationDecision,
			)
		}

		secondPlannedEvent := plan.plannedEvents[1]
		if secondPlannedEvent.event.Name != "app.go" {
			t.Fatalf("expected second planned event to be app.go, got %#v", secondPlannedEvent)
		}
		if secondPlannedEvent.preClassificationDecision.addDirectoryWatch ||
			!secondPlannedEvent.preClassificationDecision.classifyEvent {
			t.Fatalf(
				"expected regular file write to classify without addDirectoryWatch, got %#v",
				secondPlannedEvent.preClassificationDecision,
			)
		}
	})
}

func TestBuildWatcherEventPreClassificationPlanFromEvents(t *testing.T) {
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
		if len(plan.plannedEvents) != 0 {
			t.Fatalf(
				"expected no planned events when config mutation short-circuits, got %#v",
				plan.plannedEvents,
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
		if len(plan.plannedEvents) != 3 {
			t.Fatalf("expected three planned events, got %#v", plan.plannedEvents)
		}
		if configProbeCount != 2 {
			t.Fatalf("expected config probe to run once per unique path, got %d", configProbeCount)
		}
		if statProbeCount != 2 {
			t.Fatalf("expected stat probe to run once per unique path, got %d", statProbeCount)
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
