package tooling

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
)

func TestProcessBatchedEvents_AllRunOnChangeOnlySkipsBuildAndRestart(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	var preCount atomic.Int32
	events := []eventWithHooks{
		{
			classified: classifiedEvent{
				event:       waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				fileType:    fileTypeOther,
				watchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			hookCtx:         &wave.HookContext{},
			hooks:           &wave.SortedHooks{Pre: []wave.OnChangeHook{{Callback: func(*wave.HookContext) (*wave.RefreshAction, error) { preCount.Add(1); return nil, nil }}}},
			runOnChangeOnly: true,
		},
		{
			classified: classifiedEvent{
				event:       waveEvent(filepath.Join(t.TempDir(), "b.txt")),
				fileType:    fileTypeOther,
				watchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			hookCtx:         &wave.HookContext{},
			hooks:           &wave.SortedHooks{Pre: []wave.OnChangeHook{{Callback: func(*wave.HookContext) (*wave.RefreshAction, error) { preCount.Add(1); return nil, nil }}}},
			runOnChangeOnly: true,
		},
	}

	work := &workSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	if preCount.Load() != 2 {
		t.Fatalf("expected both pre hooks to run, got %d", preCount.Load())
	}

	select {
	case req := <-s.restartCh:
		t.Fatalf("did not expect restart request for run-on-change-only batch, got %#v", req)
	default:
	}
}

func TestProcessBatchedEvents_ConcurrentRestartSkipsPostHooks(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	var postRan atomic.Bool
	events := []eventWithHooks{
		{
			classified: classifiedEvent{
				event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
				fileType: fileTypeOther,
			},
			hookCtx: &wave.HookContext{},
			hooks: &wave.SortedHooks{
				Concurrent: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{TriggerRestart: true, RecompileGo: true}, nil
						},
					},
				},
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							postRan.Store(true)
							return nil, nil
						},
					},
				},
			},
			runOnChangeOnly: false,
		},
	}

	work := &workSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	select {
	case req := <-s.restartCh:
		if !req.recompileGo {
			t.Fatalf("expected restart to request Go recompilation, got %#v", req)
		}
	default:
		t.Fatal("expected restart request from concurrent hook action")
	}

	if postRan.Load() {
		t.Fatal("did not expect post hooks to run after concurrent-triggered restart")
	}
}

func TestProcessBatchedEvents_PostHookRestartNoGo(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	events := []eventWithHooks{
		{
			classified: classifiedEvent{
				event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
				fileType: fileTypeOther,
			},
			hookCtx: &wave.HookContext{},
			hooks: &wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{TriggerRestart: true, RecompileGo: false}, nil
						},
					},
				},
			},
			runOnChangeOnly: false,
		},
	}

	work := &workSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	select {
	case req := <-s.restartCh:
		if req.recompileGo {
			t.Fatalf("expected no-go restart request, got %#v", req)
		}
	default:
		t.Fatal("expected restart request from post hook action")
	}
}

func TestRunWatcher_ProcessesFsnotifyEventsUntilWatcherCloses(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	var callbackCount atomic.Int32
	cfg.Watch.Include = []wave.WatchedFile{
		{
			Pattern:         "**/*.txt",
			RunOnChangeOnly: true,
			OnChangeHooks: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						callbackCount.Add(1)
						return nil, nil
					},
				},
			},
		},
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}

	if err := watcher.AddDir(root); err != nil {
		t.Fatalf("AddDir returned error: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
	}

	done := make(chan struct{})
	go func() {
		s.runWatcher()
		close(done)
	}()

	target := filepath.Join(root, "watcher.txt")
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed writing watched file: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for callbackCount.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if callbackCount.Load() == 0 {
		t.Fatal("expected runWatcher to process at least one fsnotify event")
	}

	if err := watcher.Close(); err != nil {
		t.Fatalf("watcher.Close returned error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("runWatcher did not exit after watcher close")
	}
}

func TestWaitForBuildRetry_ConsumesRestartAndCleansUp(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		watcher:   watcher,
		builder:   builder,
		restartCh: make(chan restartRequest, 1),
	}
	s.restartCh <- restartRequest{recompileGo: true}

	s.waitForBuildRetry()

	if s.watcher != nil {
		t.Fatal("expected waitForBuildRetry to clear watcher during cleanup")
	}
	if s.builder != nil {
		t.Fatal("expected waitForBuildRetry to clear builder during cleanup")
	}
}
