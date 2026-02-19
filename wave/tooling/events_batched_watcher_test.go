package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/devserver/devserverengine"
	"github.com/vormadev/vorma/wave/tooling/watch"
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
	events := []devserver.EventWithHooks{
		{
			Classified: devserver.ClassifiedEvent{
				Event:       waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				FileType:    devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			HookCtx:         &wave.HookContext{},
			Hooks:           &wave.SortedHooks{Pre: []wave.OnChangeHook{{Callback: func(*wave.HookContext) (*wave.RefreshAction, error) { preCount.Add(1); return nil, nil }}}},
			RunOnChangeOnly: true,
		},
		{
			Classified: devserver.ClassifiedEvent{
				Event:       waveEvent(filepath.Join(t.TempDir(), "b.txt")),
				FileType:    devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			HookCtx:         &wave.HookContext{},
			Hooks:           &wave.SortedHooks{Pre: []wave.OnChangeHook{{Callback: func(*wave.HookContext) (*wave.RefreshAction, error) { preCount.Add(1); return nil, nil }}}},
			RunOnChangeOnly: true,
		},
	}

	work := &devserver.WorkSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	if preCount.Load() != 2 {
		t.Fatalf("expected both pre hooks to run, got %d", preCount.Load())
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessBatchedEvents_ConcurrentRestartSkipsPostHooks(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	var postRan atomic.Bool
	events := []devserver.EventWithHooks{
		{
			Classified: devserver.ClassifiedEvent{
				Event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
				FileType: devserver.FileTypeOther,
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
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
			RunOnChangeOnly: false,
		},
	}

	work := &devserver.WorkSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		s,
		200*time.Millisecond,
	)
	if !pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected restart to request Go recompilation, got %#v",
			pendingRestartRequest,
		)
	}

	if postRan.Load() {
		t.Fatal("did not expect post hooks to run after concurrent-triggered restart")
	}
}

func TestProcessBatchedEvents_PostHookRestartNoGo(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	events := []devserver.EventWithHooks{
		{
			Classified: devserver.ClassifiedEvent{
				Event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
				FileType: devserver.FileTypeOther,
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{TriggerRestart: true, RecompileGo: false}, nil
						},
					},
				},
			},
			RunOnChangeOnly: false,
		},
	}

	work := &devserver.WorkSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		s,
		200*time.Millisecond,
	)
	if pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected no-go restart request, got %#v",
			pendingRestartRequest,
		)
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

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}

	if err := watcher.AddDir(root); err != nil {
		t.Fatalf("AddDir returned error: %v", err)
	}

	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{
		Cfg:            cfg,
		Log:            newDiscardLogger(),
		Watcher:        watcher,
		Builder:        builder,
		RestartIntents: devserverengine.NewRestartIntentAccumulator(make(chan devserverengine.RestartRequest, 1)),
	}

	done := make(chan struct{})
	go func() {
		s.RunWatcher()
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

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	builder := toolingbuilder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &devserver.Server{
		Cfg:            cfg,
		Log:            newDiscardLogger(),
		Watcher:        watcher,
		Builder:        builder,
		RestartIntents: devserverengine.NewRestartIntentAccumulator(make(chan devserverengine.RestartRequest, 1)),
	}
	queueRestartRequestForToolingTests(
		s,
		devserverengine.RestartRequest{RecompileGo: true},
	)

	_ = s.WaitForBuildRetry()

	if s.Watcher != nil {
		t.Fatal("expected waitForBuildRetry to clear watcher during cleanup")
	}
	if s.Builder != nil {
		t.Fatal("expected waitForBuildRetry to clear builder during cleanup")
	}
}
