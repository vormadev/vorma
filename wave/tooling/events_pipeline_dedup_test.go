package tooling

import (
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
)

func TestDeduplicateWatcherEventsByPath(
	t *testing.T,
) {
	deduplicatedEvents := deduplicateWatcherEventsByPath([]fsnotify.Event{
		{Name: "b.go", Op: fsnotify.Create},
		{Name: "a.go", Op: fsnotify.Write},
		{Name: "b.go", Op: fsnotify.Write},
	})

	if len(deduplicatedEvents) != 2 {
		t.Fatalf("deduplicated event count = %d, want 2", len(deduplicatedEvents))
	}
	if deduplicatedEvents[0].Name != "a.go" {
		t.Fatalf("deduplicated event[0] name = %q, want a.go", deduplicatedEvents[0].Name)
	}
	if deduplicatedEvents[1].Name != "b.go" {
		t.Fatalf("deduplicated event[1] name = %q, want b.go", deduplicatedEvents[1].Name)
	}
	if !deduplicatedEvents[1].Has(fsnotify.Write) {
		t.Fatalf("deduplicated event[1] op = %v, want write", deduplicatedEvents[1].Op)
	}
	if !deduplicatedEvents[1].Has(fsnotify.Create) {
		t.Fatalf("deduplicated event[1] op = %v, want create", deduplicatedEvents[1].Op)
	}
}

func TestDeduplicateWatcherEventsByPathMergesAllOps(
	t *testing.T,
) {
	deduplicatedEvents := deduplicateWatcherEventsByPath([]fsnotify.Event{
		{Name: "a.go", Op: fsnotify.Create},
		{Name: "a.go", Op: fsnotify.Write},
		{Name: "a.go", Op: fsnotify.Remove},
	})

	if len(deduplicatedEvents) != 1 {
		t.Fatalf("deduplicated event count = %d, want 1", len(deduplicatedEvents))
	}
	if !deduplicatedEvents[0].Has(fsnotify.Create) {
		t.Fatalf("deduplicated op = %v, want create", deduplicatedEvents[0].Op)
	}
	if !deduplicatedEvents[0].Has(fsnotify.Write) {
		t.Fatalf("deduplicated op = %v, want write", deduplicatedEvents[0].Op)
	}
	if !deduplicatedEvents[0].Has(fsnotify.Remove) {
		t.Fatalf("deduplicated op = %v, want remove", deduplicatedEvents[0].Op)
	}
}

func TestClassifyWatcherEventsForProcessingDoesNotCollapseImplicitFileTypes(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)

	publicStaticFilePath := filepath.Join(cfg.Core.StaticAssetDirs.Public, "logo.svg")
	goFilePath := filepath.Join(root, "backend", "main.go")

	if err := os.MkdirAll(filepath.Dir(publicStaticFilePath), 0755); err != nil {
		t.Fatalf("failed creating public static file directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(goFilePath), 0755); err != nil {
		t.Fatalf("failed creating go file directory: %v", err)
	}
	if err := os.WriteFile(publicStaticFilePath, []byte("logo"), 0644); err != nil {
		t.Fatalf("failed writing public static file: %v", err)
	}
	if err := os.WriteFile(goFilePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed writing go file: %v", err)
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

	classifiedEvents, configChanged := serverForTest.classifyWatcherEventsForProcessing(
		[]fsnotify.Event{
			{Name: publicStaticFilePath, Op: fsnotify.Write},
			{Name: goFilePath, Op: fsnotify.Write},
		},
		watcher,
		builder,
	)

	if configChanged {
		t.Fatal("expected configChanged=false for non-config files")
	}
	if len(classifiedEvents) != 2 {
		t.Fatalf("classified event count = %d, want 2", len(classifiedEvents))
	}

	foundGoEvent := false
	foundPublicStaticEvent := false
	for _, classifiedEventForProcessing := range classifiedEvents {
		if classifiedEventForProcessing.fileType == fileTypeGo {
			foundGoEvent = true
		}
		if classifiedEventForProcessing.fileType == fileTypePublicStatic {
			foundPublicStaticEvent = true
		}
	}

	if !foundGoEvent {
		t.Fatal("expected go event classification to be preserved")
	}
	if !foundPublicStaticEvent {
		t.Fatal("expected public static event classification to be preserved")
	}
}

func TestClassifyWatcherEventsForProcessingPreservesSharedPatternEvents(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*",
			RunOnChangeOnly: true,
		},
	}

	goFilePath := filepath.Join(root, "backend", "main.go")
	textFilePath := filepath.Join(root, "backend", "notes.txt")

	if err := os.MkdirAll(filepath.Dir(goFilePath), 0755); err != nil {
		t.Fatalf("failed creating go file directory: %v", err)
	}
	if err := os.WriteFile(goFilePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed writing go file: %v", err)
	}
	if err := os.WriteFile(textFilePath, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed writing text file: %v", err)
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

	classifiedEvents, configChanged := serverForTest.classifyWatcherEventsForProcessing(
		[]fsnotify.Event{
			{Name: goFilePath, Op: fsnotify.Write},
			{Name: textFilePath, Op: fsnotify.Write},
		},
		watcher,
		builder,
	)

	if configChanged {
		t.Fatal("expected configChanged=false for non-config files")
	}
	if len(classifiedEvents) != 2 {
		t.Fatalf("classified event count = %d, want 2", len(classifiedEvents))
	}
}

func TestBuildEventHooksForProcessingDeduplicatesHooksByPattern(
	t *testing.T,
) {
	classifiedEvents := []classifiedEvent{
		{
			event:    fsnotify.Event{Name: "a.go", Op: fsnotify.Write},
			fileType: fileTypeGo,
			watchedFile: &wave.WatchedFile{
				Pattern: "**/*",
			},
		},
		{
			event:    fsnotify.Event{Name: "b.txt", Op: fsnotify.Write},
			fileType: fileTypeOther,
			watchedFile: &wave.WatchedFile{
				Pattern: "**/*",
			},
		},
	}

	eventsWithHooks, batchNeedsAppStop := buildEventHooksForProcessing(classifiedEvents)
	if len(eventsWithHooks) != 2 {
		t.Fatalf("eventsWithHooks count = %d, want 2", len(eventsWithHooks))
	}
	if eventsWithHooks[0].skipDuplicateHooks {
		t.Fatal("expected first event with shared pattern to run hooks")
	}
	if !eventsWithHooks[1].skipDuplicateHooks {
		t.Fatal("expected second event with shared pattern to skip duplicate hooks")
	}
	if !batchNeedsAppStop {
		t.Fatal("expected batchNeedsAppStop=true when one event requires hard reload")
	}
	if got, want := eventsWithHooks[0].hookCtx.ChangedFilePaths, []string{"a.go", "b.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first hook context changed file paths = %v, want %v", got, want)
	}
	if got, want := eventsWithHooks[1].hookCtx.ChangedFilePaths, []string{"a.go", "b.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second hook context changed file paths = %v, want %v", got, want)
	}
}

func TestBuildEventHooksForProcessingNoPatternUsesSingleChangedFilePath(
	t *testing.T,
) {
	classifiedEvents := []classifiedEvent{
		{
			event:    fsnotify.Event{Name: "/tmp/standalone.go", Op: fsnotify.Write},
			fileType: fileTypeGo,
		},
	}

	eventsWithHooks, _ := buildEventHooksForProcessing(classifiedEvents)
	if len(eventsWithHooks) != 1 {
		t.Fatalf("eventsWithHooks count = %d, want 1", len(eventsWithHooks))
	}
	if got, want := eventsWithHooks[0].hookCtx.ChangedFilePaths, []string{"/tmp/standalone.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("hook context changed file paths = %v, want %v", got, want)
	}
}

func TestProcessEvents_DeduplicatesHooksByPatternForMixedFileTypes(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	var callbackCount int32
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						atomic.AddInt32(&callbackCount, 1)
						return nil, nil
					},
				},
			},
		},
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	goFilePath := filepath.Join(root, "backend", "main.go")
	textFilePath := filepath.Join(root, "backend", "notes.txt")

	if err := os.MkdirAll(filepath.Dir(goFilePath), 0755); err != nil {
		t.Fatalf("failed creating go file directory: %v", err)
	}
	if err := os.WriteFile(goFilePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed writing go file: %v", err)
	}
	if err := os.WriteFile(textFilePath, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed writing text file: %v", err)
	}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	defer watcher.Close()

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	serverForTest := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
	}

	serverForTest.processEvents([]fsnotify.Event{
		{Name: goFilePath, Op: fsnotify.Write},
		{Name: textFilePath, Op: fsnotify.Write},
	})

	if got := atomic.LoadInt32(&callbackCount); got != 1 {
		t.Fatalf("expected callback to run once for shared pattern across mixed file types, got %d", got)
	}
}
