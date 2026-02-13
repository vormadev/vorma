package tooling

import (
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
)

func newServerAndWatcherForHookExecutionTest(t *testing.T) (*server, *Watcher) {
	t.Helper()

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		restartCh: make(chan restartRequest, 1),
	}

	return s, watcher
}

func TestRunConcurrentHooks_RespectsExcludesAndCollectsActions(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	commandOut := filepath.Join(root, "concurrent.log")
	if err := os.WriteFile(changedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	var callbackCalled atomic.Bool
	var excludedCalled atomic.Bool

	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						callbackCalled.Store(true)
						return &wave.RefreshAction{ReloadBrowser: true}, nil
					},
				},
				{
					Cmd: "printf 'cmd\\n' >> " + strconv.Quote(commandOut),
				},
				{
					Exclude: []string{changedPath},
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						excludedCalled.Store(true)
						return nil, nil
					},
				},
			},
		},
	}

	actions, err := s.runConcurrentHooks(ewh, watcher)
	if err != nil {
		t.Fatalf("runConcurrentHooks returned error: %v", err)
	}

	if !callbackCalled.Load() {
		t.Fatal("expected non-excluded concurrent callback to run")
	}
	if excludedCalled.Load() {
		t.Fatal("did not expect excluded concurrent callback to run")
	}

	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf("unexpected concurrent hook actions: %#v", actions)
	}

	data, readErr := os.ReadFile(commandOut)
	if readErr != nil {
		t.Fatalf("failed reading concurrent command output: %v", readErr)
	}
	if string(data) != "cmd\n" {
		t.Fatalf("unexpected concurrent command output: %q", string(data))
	}
}

func TestRunConcurrentHooks_ReturnsActionsInHookOrder(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						time.Sleep(20 * time.Millisecond)
						return &wave.RefreshAction{ReloadBrowser: true}, nil
					},
				},
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{WaitForApp: true}, nil
					},
				},
			},
		},
	}

	actions, err := s.runConcurrentHooks(ewh, watcher)
	if err != nil {
		t.Fatalf("runConcurrentHooks returned error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("action count=%d, want 2", len(actions))
	}
	if !actions[0].ReloadBrowser {
		t.Fatalf("expected first action to be ReloadBrowser=true, got %#v", actions[0])
	}
	if !actions[1].WaitForApp {
		t.Fatalf("expected second action to be WaitForApp=true, got %#v", actions[1])
	}
}

func TestRunConcurrentHooksForEvents_ReturnsActionsInEventOrder(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	firstChangedPath := filepath.Join(root, "first.txt")
	secondChangedPath := filepath.Join(root, "second.txt")
	if err := os.WriteFile(firstChangedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing first changed file: %v", err)
	}
	if err := os.WriteFile(secondChangedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing second changed file: %v", err)
	}

	eventsWithHooks := []eventWithHooks{
		{
			classified: classifiedEvent{event: waveEvent(firstChangedPath)},
			hookCtx:    &wave.HookContext{FilePath: firstChangedPath},
			hooks: &wave.SortedHooks{
				Concurrent: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							time.Sleep(20 * time.Millisecond)
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						},
					},
				},
			},
		},
		{
			classified: classifiedEvent{event: waveEvent(secondChangedPath)},
			hookCtx:    &wave.HookContext{FilePath: secondChangedPath},
			hooks: &wave.SortedHooks{
				Concurrent: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{TriggerRestart: true, RecompileGo: false}, nil
						},
					},
				},
			},
		},
	}

	actions := s.runConcurrentHooksForEvents(eventsWithHooks, watcher)
	if len(actions) != 2 {
		t.Fatalf("action count=%d, want 2", len(actions))
	}
	if !actions[0].ReloadBrowser {
		t.Fatalf("expected first action to come from first event, got %#v", actions[0])
	}
	if !actions[1].TriggerRestart {
		t.Fatalf("expected second action to come from second event, got %#v", actions[1])
	}
}

func TestRunPostHooks_StopsOnCommandError(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	var lateCallbackCalled atomic.Bool
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(filepath.Join(t.TempDir(), "changed.txt"))},
		hookCtx:    &wave.HookContext{},
		hooks: &wave.SortedHooks{
			Post: []wave.OnChangeHook{
				{Cmd: "false"},
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						lateCallbackCalled.Store(true)
						return nil, nil
					},
				},
			},
		},
	}

	actions, err := s.runPostHooks(ewh, watcher)
	if err == nil {
		t.Fatal("expected runPostHooks to fail on command error")
	}
	if len(actions) != 0 {
		t.Fatalf("expected no actions before failure, got %#v", actions)
	}
	if lateCallbackCalled.Load() {
		t.Fatal("did not expect hooks after failing command to run")
	}
}

func TestRunPostHooks_RunOnChangeOnlySkipsCommandAndKeepsCallback(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	commandOut := filepath.Join(root, "post-command.log")
	if err := os.WriteFile(changedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	var callbackCalled atomic.Bool
	ewh := eventWithHooks{
		classified:      classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:         &wave.HookContext{FilePath: changedPath},
		runOnChangeOnly: true,
		hooks: &wave.SortedHooks{
			Post: []wave.OnChangeHook{
				{
					Cmd: "printf 'should-not-run\\n' >> " + strconv.Quote(commandOut),
				},
				{
					Cmd: "printf 'should-not-run\\n' >> " + strconv.Quote(commandOut),
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						callbackCalled.Store(true)
						return &wave.RefreshAction{ReloadBrowser: true}, nil
					},
				},
			},
		},
	}

	actions, err := s.runPostHooks(ewh, watcher)
	if err != nil {
		t.Fatalf("runPostHooks returned error: %v", err)
	}
	if !callbackCalled.Load() {
		t.Fatal("expected callback hook to run for run-on-change-only post hooks")
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf("unexpected post hook actions: %#v", actions)
	}
	if _, statErr := os.Stat(commandOut); !os.IsNotExist(statErr) {
		t.Fatalf("expected run-on-change-only post commands to be skipped, stat error: %v", statErr)
	}
}

func TestFireNoWaitHooks_RunsAsyncCallbackAndCommand(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	commandOut := filepath.Join(root, "nowait.log")
	if err := os.WriteFile(changedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	callbackDone := make(chan struct{}, 1)

	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			ConcurrentNoWait: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						callbackDone <- struct{}{}
						return nil, nil
					},
				},
				{
					Cmd: "printf 'async\\n' >> " + strconv.Quote(commandOut),
				},
			},
		},
	}

	s.fireNoWaitHooks(ewh, watcher)

	select {
	case <-callbackDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for no-wait callback")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		data, err := os.ReadFile(commandOut)
		if err == nil && string(data) == "async\n" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for no-wait command output (last read err=%v)", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestFireNoWaitHooks_ExcludesMatchingHooksAndToleratesFailures(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	var excludedHookRan atomic.Bool
	failingHookCalled := make(chan struct{}, 1)

	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			ConcurrentNoWait: []wave.OnChangeHook{
				{
					Exclude: []string{changedPath},
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						excludedHookRan.Store(true)
						return nil, nil
					},
				},
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						failingHookCalled <- struct{}{}
						return nil, os.ErrInvalid
					},
					Cmd: "false",
				},
			},
		},
	}

	s.fireNoWaitHooks(ewh, watcher)

	select {
	case <-failingHookCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for failing no-wait callback to be invoked")
	}

	if excludedHookRan.Load() {
		t.Fatal("did not expect excluded no-wait hook callback to run")
	}
}

func TestProcessSingleEvent_PrehookRestartShortCircuitsPipeline(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	work := &workSet{}
	ewh := eventWithHooks{
		classified: classifiedEvent{
			event:       waveEvent(filepath.Join(t.TempDir(), "file.txt")),
			fileType:    fileTypeOther,
			watchedFile: &wave.WatchedFile{},
		},
		hookCtx: &wave.HookContext{},
		hooks: &wave.SortedHooks{Pre: []wave.OnChangeHook{{Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
			return &wave.RefreshAction{TriggerRestart: true}, nil
		}}}},
		runOnChangeOnly: false,
		needsHardReload: false,
	}

	runEventsWithDerivedExecutionPlan(t, s, []eventWithHooks{ewh}, work, watcher)

	select {
	case req := <-s.restartCh:
		if req.recompileGo {
			t.Fatalf("expected no-go restart from prehook action, got %#v", req)
		}
	default:
		t.Fatal("expected restart request from prehook action")
	}
}

func TestProcessSingleEvent_ConcurrentRestartCanRequestGoRecompile(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	work := &workSet{}
	ewh := eventWithHooks{
		classified: classifiedEvent{
			event:    waveEvent(filepath.Join(t.TempDir(), "file.txt")),
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
		},
		runOnChangeOnly: false,
		needsHardReload: false,
	}

	runEventsWithDerivedExecutionPlan(t, s, []eventWithHooks{ewh}, work, watcher)

	select {
	case req := <-s.restartCh:
		if !req.recompileGo {
			t.Fatalf("expected Go recompilation restart, got %#v", req)
		}
	default:
		t.Fatal("expected restart request from concurrent hook action")
	}
}

func TestExecuteBuildPhase_ProcessesStaticFilesAndWritesFrameworkFileMapTS(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.FrameworkPublicFileMapOutDir = filepath.Join(root, "framework")
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	publicFile := filepath.Join(cfg.Core.StaticAssetDirs.Public, "assets", "logo.png")
	privateFile := filepath.Join(cfg.Core.StaticAssetDirs.Private, "templates", "home.html")

	if err := os.MkdirAll(filepath.Dir(publicFile), 0755); err != nil {
		t.Fatalf("failed creating public file parent dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(privateFile), 0755); err != nil {
		t.Fatalf("failed creating private file parent dir: %v", err)
	}
	if err := os.WriteFile(publicFile, []byte("logo"), 0644); err != nil {
		t.Fatalf("failed writing public file: %v", err)
	}
	if err := os.WriteFile(privateFile, []byte("<h1>home</h1>"), 0644); err != nil {
		t.Fatalf("failed writing private file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
	}

	work := &workSet{
		build: buildPhaseDecision{
			processPublicFiles:  true,
			processPrivateFiles: true,
			buildCriticalCSS:    true,
			buildNormalCSS:      true,
		},
	}
	s.executeBuildPhase(work)

	requiredOutputs := []string{
		cfg.Dist.PublicFileMapGob(),
		cfg.Dist.PrivateFileMapGob(),
		filepath.Join(cfg.FrameworkPublicFileMapOutDir, wave.RelPaths.PublicFileMapTSName()),
		filepath.Join(cfg.FrameworkPublicFileMapOutDir, wave.RelPaths.PublicFileMapJSONName()),
	}
	for _, output := range requiredOutputs {
		if _, err := os.Stat(output); err != nil {
			t.Fatalf("expected build phase output to exist: %s (error: %v)", output, err)
		}
	}
}

func TestExecuteBuildPhase_CompileGoErrorDoesNotPanic(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "missing/package/for/compile"

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
	}

	work := &workSet{
		build: buildPhaseDecision{
			compileGo: true,
		},
	}
	s.executeBuildPhase(work)
}

func TestExecuteBuildPhase_WithNilBuilderDoesNotPanic(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	work := &workSet{
		build: buildPhaseDecision{
			compileGo:           true,
			processPublicFiles:  true,
			processPrivateFiles: true,
			buildCriticalCSS:    true,
			buildNormalCSS:      true,
		},
	}
	s.executeBuildPhase(work)
}

func TestExecuteBuildPhase_WritePublicFileMapTSErrorDoesNotPanic(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.FrameworkPublicFileMapOutDir = filepath.Join(root, "framework-output-blocker")
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	if err := os.WriteFile(cfg.FrameworkPublicFileMapOutDir, []byte("not-a-directory"), 0644); err != nil {
		t.Fatalf("failed writing framework output blocker file: %v", err)
	}

	publicFile := filepath.Join(cfg.Core.StaticAssetDirs.Public, "assets", "logo.png")
	if err := os.MkdirAll(filepath.Dir(publicFile), 0755); err != nil {
		t.Fatalf("failed creating public static dir: %v", err)
	}
	if err := os.WriteFile(publicFile, []byte("logo"), 0644); err != nil {
		t.Fatalf("failed writing public static file: %v", err)
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
	}

	work := &workSet{
		build: buildPhaseDecision{
			processPublicFiles: true,
		},
	}
	s.executeBuildPhase(work)

	statInfo, err := os.Stat(cfg.FrameworkPublicFileMapOutDir)
	if err != nil {
		t.Fatalf("failed stating framework output blocker: %v", err)
	}
	if statInfo.IsDir() {
		t.Fatal("expected framework output blocker to remain a file")
	}
}

func waveEvent(path string) fsnotify.Event {
	return fsnotify.Event{Name: path, Op: fsnotify.Write}
}
