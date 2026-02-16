package tooling

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
)

func newServerAndWatcherForEventPipelineTest(
	t *testing.T,
	serverOnly bool,
) (*server, *watcher) {
	t.Helper()

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = serverOnly

	watcher, err := newWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}

	s := &server{
		cfg:            cfg,
		log:            newDiscardLogger(),
		restartIntents: newRestartIntentAccumulator(make(chan restartRequest, 2)),
	}
	return s, watcher
}

func makeExecutableSleepScriptForToolingTest(t *testing.T, cfg *wave.ParsedConfig) {
	t.Helper()

	if err := SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}

	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(cfg.Dist.Binary(), []byte(script), 0755); err != nil {
		t.Fatalf("failed writing executable binary script: %v", err)
	}
}

func TestProcessSingleEvent_PreHookRestartCanRequestGoRecompile(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	work := &workSet{}
	ewh := eventWithHooks{
		classified: classifiedEvent{
			event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
			fileType: fileTypeOther,
		},
		hookCtx: &wave.HookContext{},
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{TriggerRestart: true, RecompileGo: true}, nil
					},
				},
			},
		},
		runOnChangeOnly: false,
	}

	runEventsWithDerivedExecutionPlan(t, s, []eventWithHooks{ewh}, work, watcher)

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		s,
		200*time.Millisecond,
	)
	if !pendingRestartRequest.recompileGo {
		t.Fatalf(
			"expected restart to request Go recompilation, got %#v",
			pendingRestartRequest,
		)
	}
}

func TestProcessSingleEvent_RunOnChangeOnlyWithHardReloadStopsRunningApp(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	makeExecutableSleepScriptForToolingTest(t, s.cfg)
	s.startApp()
	if s.appCmd == nil || s.appCmd.Process == nil {
		t.Fatal("expected test app process to start")
	}

	work := &workSet{}
	ewh := eventWithHooks{
		classified: classifiedEvent{
			event:    waveEvent(filepath.Join(t.TempDir(), "changed.go")),
			fileType: fileTypeGo,
			watchedFile: &wave.WatchedFile{
				RunOnChangeOnly: true,
			},
		},
		hookCtx:         &wave.HookContext{},
		hooks:           &wave.SortedHooks{},
		runOnChangeOnly: true,
		needsHardReload: true,
	}

	runEventsWithDerivedExecutionPlan(t, s, []eventWithHooks{ewh}, work, watcher)

	if s.appCmd != nil {
		t.Fatal("expected hard-reload run-on-change-only event to stop running app")
	}
	if work.build.compileGo || work.restart.restartApp {
		t.Fatalf("expected run-on-change-only event to skip implicit build/restart work, got %#v", work)
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessSingleEvent_PostHookRestartShortCircuitsBrowserReload(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.refreshMgrCtx = context.Background()
	s.refreshMgr = &clientManager{
		broadcast: make(chan refreshPayload, 1),
	}

	work := &workSet{}
	ewh := eventWithHooks{
		classified: classifiedEvent{
			event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
			fileType: fileTypeOther,
		},
		hookCtx: &wave.HookContext{},
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{ReloadBrowser: true}, nil
					},
				},
			},
			Post: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{TriggerRestart: true, RecompileGo: false}, nil
					},
				},
			},
		},
	}

	runEventsWithDerivedExecutionPlan(t, s, []eventWithHooks{ewh}, work, watcher)

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		s,
		200*time.Millisecond,
	)
	if pendingRestartRequest.recompileGo {
		t.Fatalf(
			"expected no-go restart from post hook, got %#v",
			pendingRestartRequest,
		)
	}

	select {
	case msg := <-s.refreshMgr.broadcast:
		t.Fatalf("did not expect browser broadcast after post-hook restart, got %#v", msg)
	default:
	}
}

func TestProcessSingleEvent_ConcurrentActionCanTriggerBrowserReload(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.refreshMgrCtx = context.Background()
	s.refreshMgr = &clientManager{
		broadcast: make(chan refreshPayload, 1),
	}

	work := &workSet{}
	ewh := eventWithHooks{
		classified: classifiedEvent{
			event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
			fileType: fileTypeOther,
		},
		hookCtx: &wave.HookContext{},
		hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{ReloadBrowser: true}, nil
					},
				},
			},
		},
	}

	runEventsWithDerivedExecutionPlan(t, s, []eventWithHooks{ewh}, work, watcher)

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for browser reload payload")
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessSingleEvent_RunOnChangeOnlyPostCallbackCanTriggerBrowserReload(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.refreshMgrCtx = context.Background()
	s.refreshMgr = &clientManager{
		broadcast: make(chan refreshPayload, 1),
	}

	work := &workSet{}
	ewh := eventWithHooks{
		classified: classifiedEvent{
			event:       waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
			fileType:    fileTypeOther,
			watchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
		},
		hookCtx: &wave.HookContext{},
		hooks: &wave.SortedHooks{
			Post: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{ReloadBrowser: true}, nil
					},
				},
			},
		},
		runOnChangeOnly: true,
	}

	runEventsWithDerivedExecutionPlan(t, s, []eventWithHooks{ewh}, work, watcher)

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for run-on-change-only post callback reload payload")
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessSingleEvent_ImplicitRestartStartsApp(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	makeExecutableSleepScriptForToolingTest(t, s.cfg)

	work := &workSet{}
	ewh := eventWithHooks{
		classified: classifiedEvent{
			event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
			fileType: fileTypeOther,
			watchedFile: &wave.WatchedFile{
				RestartApp: true,
			},
		},
		hookCtx: &wave.HookContext{},
		hooks:   &wave.SortedHooks{},
	}

	runEventsWithDerivedExecutionPlan(t, s, []eventWithHooks{ewh}, work, watcher)
	if s.appCmd == nil || s.appCmd.Process == nil {
		t.Fatal("expected implicit restart work to start app")
	}

	if err := s.stopApp(); err != nil {
		t.Fatalf("failed stopping test app: %v", err)
	}
}

func TestProcessSingleEvent_BuildFailureShortCircuitsRestartAndBrowserReload(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.cfg.Core.MainAppEntry = "missing/package/for/compile"
	s.refreshMgrCtx = context.Background()
	s.refreshMgr = &clientManager{
		broadcast: make(chan refreshPayload, 1),
	}
	s.portResolver = wave.NewPortResolver()

	appHealthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer appHealthServer.Close()

	parsedAppHealthURL, parseURLPathError := url.Parse(appHealthServer.URL)
	if parseURLPathError != nil {
		t.Fatalf("failed parsing app health URL: %v", parseURLPathError)
	}
	appHealthPort, parsePortError := strconv.Atoi(parsedAppHealthURL.Port())
	if parsePortError != nil {
		t.Fatalf("failed parsing app health port: %v", parsePortError)
	}
	t.Setenv("PORT", strconv.Itoa(appHealthPort))
	t.Setenv("WAVE_PORT_HAS_BEEN_SET", "true")

	builder := NewBuilder(s.cfg, newDiscardLogger())
	defer builder.Close()
	s.builder = builder

	work := &workSet{}
	ewh := eventWithHooks{
		classified: classifiedEvent{
			event:    waveEvent(filepath.Join(t.TempDir(), "changed.go")),
			fileType: fileTypeGo,
		},
		hookCtx: &wave.HookContext{},
		hooks:   &wave.SortedHooks{},
	}

	runEventsWithDerivedExecutionPlan(t, s, []eventWithHooks{ewh}, work, watcher)

	if s.appCmd != nil {
		t.Fatalf("did not expect app to start after build failure, got %#v", s.appCmd)
	}

	select {
	case msg := <-s.refreshMgr.broadcast:
		t.Fatalf("did not expect browser payload after build failure, got %#v", msg)
	default:
	}
}

func TestProcessBatchedEvents_PrehookRestartShortCircuitsPostAndBrowser(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.refreshMgrCtx = context.Background()
	s.refreshMgr = &clientManager{
		broadcast: make(chan refreshPayload, 1),
	}

	var postHookRan atomic.Bool
	events := []eventWithHooks{
		{
			classified: classifiedEvent{
				event:    waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				fileType: fileTypeOther,
			},
			hookCtx: &wave.HookContext{},
			hooks: &wave.SortedHooks{
				Pre: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{TriggerRestart: true, RecompileGo: false}, nil
						},
					},
				},
			},
		},
		{
			classified: classifiedEvent{
				event:    waveEvent(filepath.Join(t.TempDir(), "b.txt")),
				fileType: fileTypeOther,
			},
			hookCtx: &wave.HookContext{},
			hooks: &wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							postHookRan.Store(true)
							return nil, nil
						},
					},
				},
			},
		},
	}

	work := &workSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		s,
		200*time.Millisecond,
	)
	if pendingRestartRequest.recompileGo {
		t.Fatalf(
			"expected no-go restart from batched pre hook, got %#v",
			pendingRestartRequest,
		)
	}

	if postHookRan.Load() {
		t.Fatal("did not expect post hooks to run after batched pre-hook restart")
	}
	select {
	case msg := <-s.refreshMgr.broadcast:
		t.Fatalf("did not expect browser broadcast after pre-hook restart, got %#v", msg)
	default:
	}
}

func TestProcessBatchedEvents_AggregatesActionsAndBroadcastsSingleReload(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.refreshMgrCtx = context.Background()
	s.refreshMgr = &clientManager{
		broadcast: make(chan refreshPayload, 2),
	}

	events := []eventWithHooks{
		{
			classified: classifiedEvent{
				event:    waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				fileType: fileTypeOther,
			},
			hookCtx: &wave.HookContext{},
			hooks: &wave.SortedHooks{
				Concurrent: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						},
					},
				},
			},
		},
		{
			classified: classifiedEvent{
				event:    waveEvent(filepath.Join(t.TempDir(), "b.txt")),
				fileType: fileTypeOther,
			},
			hookCtx: &wave.HookContext{},
			hooks: &wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						},
					},
				},
			},
		},
	}

	work := &workSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for batched hard reload payload")
	}

	select {
	case extra := <-s.refreshMgr.broadcast:
		t.Fatalf("expected single batched reload payload, got extra %#v", extra)
	default:
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessBatchedEvents_AllRunOnChangeOnlyPostCallbacksCanTriggerBrowserReload(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.refreshMgrCtx = context.Background()
	s.refreshMgr = &clientManager{
		broadcast: make(chan refreshPayload, 1),
	}

	events := []eventWithHooks{
		{
			classified: classifiedEvent{
				event:       waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				fileType:    fileTypeOther,
				watchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			hookCtx: &wave.HookContext{},
			hooks: &wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						},
					},
				},
			},
			runOnChangeOnly: true,
		},
	}

	work := &workSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for batched run-on-change-only reload payload")
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessBatchedEvents_MixedBatchRunsRunOnChangeOnlyPostCallbacks(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	var runOnChangeOnlyPostCallbackRan atomic.Bool
	events := []eventWithHooks{
		{
			classified: classifiedEvent{
				event:       waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				fileType:    fileTypeOther,
				watchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			hookCtx: &wave.HookContext{},
			hooks: &wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							runOnChangeOnlyPostCallbackRan.Store(true)
							return nil, nil
						},
					},
				},
			},
			runOnChangeOnly: true,
		},
		{
			classified: classifiedEvent{
				event:    waveEvent(filepath.Join(t.TempDir(), "b.txt")),
				fileType: fileTypeOther,
			},
			hookCtx:         &wave.HookContext{},
			hooks:           &wave.SortedHooks{},
			runOnChangeOnly: false,
		},
	}

	work := &workSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	if !runOnChangeOnlyPostCallbackRan.Load() {
		t.Fatal("expected run-on-change-only post callback to run in mixed batch")
	}
}

func TestProcessBatchedEvents_ImplicitRestartStartsApp(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	makeExecutableSleepScriptForToolingTest(t, s.cfg)

	events := []eventWithHooks{
		{
			classified: classifiedEvent{
				event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
				fileType: fileTypeOther,
				watchedFile: &wave.WatchedFile{
					RestartApp: true,
				},
			},
			hookCtx: &wave.HookContext{},
			hooks:   &wave.SortedHooks{},
		},
	}

	work := &workSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)
	if s.appCmd == nil || s.appCmd.Process == nil {
		t.Fatal("expected batched implicit restart work to start app")
	}

	if err := s.stopApp(); err != nil {
		t.Fatalf("failed stopping test app: %v", err)
	}
}

func TestRunPreHooks_CallbackAndCommandErrorsAreReturned(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	callbackFailureEvent := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(filepath.Join(t.TempDir(), "callback-failure.txt"))},
		hookCtx:    &wave.HookContext{},
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return nil, os.ErrInvalid
					},
				},
			},
		},
	}

	if _, err := s.runPreHooks(callbackFailureEvent, watcher); err == nil {
		t.Fatal("expected callback failure to be returned by runPreHooks")
	}

	commandFailureEvent := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(filepath.Join(t.TempDir(), "command-failure.txt"))},
		hookCtx:    &wave.HookContext{},
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{ReloadBrowser: true}, nil
					},
					Cmd: "false",
				},
			},
		},
	}

	actions, err := s.runPreHooks(commandFailureEvent, watcher)
	if err == nil {
		t.Fatal("expected command failure to be returned by runPreHooks")
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf("expected callback actions before command failure, got %#v", actions)
	}
}

func TestRunConcurrentHooks_AndRunPostHooks_PropagateCallbackErrors(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	concurrentErrorEvent := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(filepath.Join(t.TempDir(), "concurrent-callback-error.txt"))},
		hookCtx:    &wave.HookContext{},
		hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return nil, os.ErrPermission
					},
				},
			},
		},
	}
	if _, err := s.runConcurrentHooks(concurrentErrorEvent, watcher); err == nil {
		t.Fatal("expected concurrent callback error to be propagated")
	}

	postErrorEvent := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(filepath.Join(t.TempDir(), "post-callback-error.txt"))},
		hookCtx:    &wave.HookContext{},
		hooks: &wave.SortedHooks{
			Post: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return nil, os.ErrNotExist
					},
				},
			},
		},
	}
	if _, err := s.runPostHooks(postErrorEvent, watcher); err == nil {
		t.Fatal("expected post callback error to be propagated")
	}
}
