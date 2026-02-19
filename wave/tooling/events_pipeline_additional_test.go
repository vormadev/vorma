package tooling

import (
	"context"
	"github.com/vormadev/vorma/wave/tooling/broadcast"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/devserver/devserverengine"
	"github.com/vormadev/vorma/wave/tooling/watch"
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
	"github.com/vormadev/vorma/wave/internal/waveshared"
)

func newServerAndWatcherForEventPipelineTest(
	t *testing.T,
	serverOnly bool,
) (*devserver.Server, *watch.Watcher) {
	t.Helper()

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = serverOnly

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: devserverengine.NewRestartIntentAccumulator(
			make(chan devserverengine.RestartRequest, 2),
		),
	}
	return s, watcher
}

func makeExecutableSleepScriptForToolingTest(
	t *testing.T,
	cfg *wave.ParsedConfig,
) {
	t.Helper()

	if err := toolingbuilder.SetupDistDir(cfg); err != nil {
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

	work := &devserver.WorkSet{}
	ewh := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
			FileType: devserver.FileTypeOther,
		},
		HookCtx: &wave.HookContext{},
		Hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{
							TriggerRestart: true,
							RecompileGo:    true,
						}, nil
					},
				},
			},
		},
		RunOnChangeOnly: false,
	}

	runEventsWithDerivedExecutionPlan(
		t,
		s,
		[]devserver.EventWithHooks{ewh},
		work,
		watcher,
	)

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
}

func TestProcessSingleEvent_RunOnChangeOnlyWithHardReloadStopsRunningApp(
	t *testing.T,
) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	makeExecutableSleepScriptForToolingTest(t, s.Cfg)
	s.StartApp()
	if s.AppCmd == nil || s.AppCmd.Process == nil {
		t.Fatal("expected test app process to start")
	}

	work := &devserver.WorkSet{}
	ewh := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event:    waveEvent(filepath.Join(t.TempDir(), "changed.go")),
			FileType: devserver.FileTypeGo,
			WatchedFile: &wave.WatchedFile{
				RunOnChangeOnly: true,
			},
		},
		HookCtx:         &wave.HookContext{},
		Hooks:           &wave.SortedHooks{},
		RunOnChangeOnly: true,
		NeedsHardReload: true,
	}

	runEventsWithDerivedExecutionPlan(
		t,
		s,
		[]devserver.EventWithHooks{ewh},
		work,
		watcher,
	)

	if s.AppCmd != nil {
		t.Fatal(
			"expected hard-reload run-on-change-only event to stop running app",
		)
	}
	if work.Build.CompileGo || work.Restart.RestartApp {
		t.Fatalf(
			"expected run-on-change-only event to skip implicit build/restart work, got %#v",
			work,
		)
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessSingleEvent_PostHookRestartShortCircuitsBrowserReload(
	t *testing.T,
) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.RefreshMgrCtx = context.Background()
	s.RefreshMgr = &broadcast.Manager{
		Broadcast: make(chan broadcast.Payload, 1),
	}

	work := &devserver.WorkSet{}
	ewh := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
			FileType: devserver.FileTypeOther,
		},
		HookCtx: &wave.HookContext{},
		Hooks: &wave.SortedHooks{
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
						return &wave.RefreshAction{
							TriggerRestart: true,
							RecompileGo:    false,
						}, nil
					},
				},
			},
		},
	}

	runEventsWithDerivedExecutionPlan(
		t,
		s,
		[]devserver.EventWithHooks{ewh},
		work,
		watcher,
	)

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		s,
		200*time.Millisecond,
	)
	if pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected no-go restart from post hook, got %#v",
			pendingRestartRequest,
		)
	}

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		t.Fatalf(
			"did not expect browser broadcast after post-hook restart, got %#v",
			msg,
		)
	default:
	}
}

func TestProcessSingleEvent_ConcurrentActionCanTriggerBrowserReload(
	t *testing.T,
) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.RefreshMgrCtx = context.Background()
	s.RefreshMgr = &broadcast.Manager{
		Broadcast: make(chan broadcast.Payload, 1),
	}

	work := &devserver.WorkSet{}
	ewh := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
			FileType: devserver.FileTypeOther,
		},
		HookCtx: &wave.HookContext{},
		Hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{ReloadBrowser: true}, nil
					},
				},
			},
		},
	}

	runEventsWithDerivedExecutionPlan(
		t,
		s,
		[]devserver.EventWithHooks{ewh},
		work,
		watcher,
	)

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		if msg.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for browser reload payload")
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessSingleEvent_RunOnChangeOnlyPostCallbackCanTriggerBrowserReload(
	t *testing.T,
) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.RefreshMgrCtx = context.Background()
	s.RefreshMgr = &broadcast.Manager{
		Broadcast: make(chan broadcast.Payload, 1),
	}

	work := &devserver.WorkSet{}
	ewh := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event:       waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
			FileType:    devserver.FileTypeOther,
			WatchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
		},
		HookCtx: &wave.HookContext{},
		Hooks: &wave.SortedHooks{
			Post: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{ReloadBrowser: true}, nil
					},
				},
			},
		},
		RunOnChangeOnly: true,
	}

	runEventsWithDerivedExecutionPlan(
		t,
		s,
		[]devserver.EventWithHooks{ewh},
		work,
		watcher,
	)

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		if msg.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal(
			"timed out waiting for run-on-change-only post callback reload payload",
		)
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessSingleEvent_ImplicitRestartStartsApp(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	makeExecutableSleepScriptForToolingTest(t, s.Cfg)

	work := &devserver.WorkSet{}
	ewh := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
			FileType: devserver.FileTypeOther,
			WatchedFile: &wave.WatchedFile{
				RestartApp: true,
			},
		},
		HookCtx: &wave.HookContext{},
		Hooks:   &wave.SortedHooks{},
	}

	runEventsWithDerivedExecutionPlan(
		t,
		s,
		[]devserver.EventWithHooks{ewh},
		work,
		watcher,
	)
	if s.AppCmd == nil || s.AppCmd.Process == nil {
		t.Fatal("expected implicit restart work to start app")
	}

	if err := s.StopApp(); err != nil {
		t.Fatalf("failed stopping test app: %v", err)
	}
}

func TestProcessSingleEvent_BuildFailureShortCircuitsRestartAndBrowserReload(
	t *testing.T,
) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.Cfg.Core.MainAppEntry = "missing/package/for/compile"
	s.RefreshMgrCtx = context.Background()
	s.RefreshMgr = &broadcast.Manager{
		Broadcast: make(chan broadcast.Payload, 1),
	}
	s.PortResolver = waveshared.NewResolver()

	appHealthServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)
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
	t.Setenv("__WAVE_PORT_HAS_BEEN_SET", "true")

	builder := toolingbuilder.NewBuilder(s.Cfg, newDiscardLogger())
	defer builder.Close()
	s.Builder = builder

	work := &devserver.WorkSet{}
	ewh := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event:    waveEvent(filepath.Join(t.TempDir(), "changed.go")),
			FileType: devserver.FileTypeGo,
		},
		HookCtx: &wave.HookContext{},
		Hooks:   &wave.SortedHooks{},
	}

	runEventsWithDerivedExecutionPlan(
		t,
		s,
		[]devserver.EventWithHooks{ewh},
		work,
		watcher,
	)

	if s.AppCmd != nil {
		t.Fatalf(
			"did not expect app to start after build failure, got %#v",
			s.AppCmd,
		)
	}

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		t.Fatalf(
			"did not expect browser payload after build failure, got %#v",
			msg,
		)
	default:
	}
}

func TestProcessBatchedEvents_PrehookRestartShortCircuitsPostAndBrowser(
	t *testing.T,
) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.RefreshMgrCtx = context.Background()
	s.RefreshMgr = &broadcast.Manager{
		Broadcast: make(chan broadcast.Payload, 1),
	}

	var postHookRan atomic.Bool
	events := []devserver.EventWithHooks{
		{
			Classified: devserver.ClassifiedEvent{
				Event:    waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				FileType: devserver.FileTypeOther,
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
				Pre: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{
								TriggerRestart: true,
								RecompileGo:    false,
							}, nil
						},
					},
				},
			},
		},
		{
			Classified: devserver.ClassifiedEvent{
				Event:    waveEvent(filepath.Join(t.TempDir(), "b.txt")),
				FileType: devserver.FileTypeOther,
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
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

	work := &devserver.WorkSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		s,
		200*time.Millisecond,
	)
	if pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected no-go restart from batched pre hook, got %#v",
			pendingRestartRequest,
		)
	}

	if postHookRan.Load() {
		t.Fatal(
			"did not expect post hooks to run after batched pre-hook restart",
		)
	}
	select {
	case msg := <-s.RefreshMgr.Broadcast:
		t.Fatalf(
			"did not expect browser broadcast after pre-hook restart, got %#v",
			msg,
		)
	default:
	}
}

func TestProcessBatchedEvents_AggregatesActionsAndBroadcastsSingleReload(
	t *testing.T,
) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.RefreshMgrCtx = context.Background()
	s.RefreshMgr = &broadcast.Manager{
		Broadcast: make(chan broadcast.Payload, 2),
	}

	events := []devserver.EventWithHooks{
		{
			Classified: devserver.ClassifiedEvent{
				Event:    waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				FileType: devserver.FileTypeOther,
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
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
			Classified: devserver.ClassifiedEvent{
				Event:    waveEvent(filepath.Join(t.TempDir(), "b.txt")),
				FileType: devserver.FileTypeOther,
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
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

	work := &devserver.WorkSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		if msg.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for batched hard reload payload")
	}

	select {
	case extra := <-s.RefreshMgr.Broadcast:
		t.Fatalf("expected single batched reload payload, got extra %#v", extra)
	default:
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessBatchedEvents_AllRunOnChangeOnlyPostCallbacksCanTriggerBrowserReload(
	t *testing.T,
) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, false)
	defer watcher.Close()

	s.RefreshMgrCtx = context.Background()
	s.RefreshMgr = &broadcast.Manager{
		Broadcast: make(chan broadcast.Payload, 1),
	}

	events := []devserver.EventWithHooks{
		{
			Classified: devserver.ClassifiedEvent{
				Event:       waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				FileType:    devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						},
					},
				},
			},
			RunOnChangeOnly: true,
		},
	}

	work := &devserver.WorkSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	select {
	case msg := <-s.RefreshMgr.Broadcast:
		if msg.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal(
			"timed out waiting for batched run-on-change-only reload payload",
		)
	}

	assertNoPendingRestartRequestForToolingTests(t, s)
}

func TestProcessBatchedEvents_MixedBatchRunsRunOnChangeOnlyPostCallbacks(
	t *testing.T,
) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	var runOnChangeOnlyPostCallbackRan atomic.Bool
	events := []devserver.EventWithHooks{
		{
			Classified: devserver.ClassifiedEvent{
				Event:       waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				FileType:    devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							runOnChangeOnlyPostCallbackRan.Store(true)
							return nil, nil
						},
					},
				},
			},
			RunOnChangeOnly: true,
		},
		{
			Classified: devserver.ClassifiedEvent{
				Event:    waveEvent(filepath.Join(t.TempDir(), "b.txt")),
				FileType: devserver.FileTypeOther,
			},
			HookCtx:         &wave.HookContext{},
			Hooks:           &wave.SortedHooks{},
			RunOnChangeOnly: false,
		},
	}

	work := &devserver.WorkSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)

	if !runOnChangeOnlyPostCallbackRan.Load() {
		t.Fatal(
			"expected run-on-change-only post callback to run in mixed batch",
		)
	}
}

func TestProcessBatchedEvents_ImplicitRestartStartsApp(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	makeExecutableSleepScriptForToolingTest(t, s.Cfg)

	events := []devserver.EventWithHooks{
		{
			Classified: devserver.ClassifiedEvent{
				Event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
				FileType: devserver.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					RestartApp: true,
				},
			},
			HookCtx: &wave.HookContext{},
			Hooks:   &wave.SortedHooks{},
		},
	}

	work := &devserver.WorkSet{}
	runEventsWithDerivedExecutionPlan(t, s, events, work, watcher)
	if s.AppCmd == nil || s.AppCmd.Process == nil {
		t.Fatal("expected batched implicit restart work to start app")
	}

	if err := s.StopApp(); err != nil {
		t.Fatalf("failed stopping test app: %v", err)
	}
}

func TestRunPreHooks_CallbackAndCommandErrorsAreReturned(t *testing.T) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	callbackFailureEvent := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event: waveEvent(
				filepath.Join(t.TempDir(), "callback-failure.txt"),
			),
		},
		HookCtx: &wave.HookContext{},
		Hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return nil, os.ErrInvalid
					},
				},
			},
		},
	}

	if _, err := s.RunPreHooks(callbackFailureEvent, watcher); err == nil {
		t.Fatal("expected callback failure to be returned by runPreHooks")
	}

	commandFailureEvent := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event: waveEvent(filepath.Join(t.TempDir(), "command-failure.txt")),
		},
		HookCtx: &wave.HookContext{},
		Hooks: &wave.SortedHooks{
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

	actions, err := s.RunPreHooks(commandFailureEvent, watcher)
	if err == nil {
		t.Fatal("expected command failure to be returned by runPreHooks")
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected callback actions before command failure, got %#v",
			actions,
		)
	}
}

func TestRunConcurrentHooks_AndRunPostHooks_PropagateCallbackErrors(
	t *testing.T,
) {
	s, watcher := newServerAndWatcherForEventPipelineTest(t, true)
	defer watcher.Close()

	concurrentErrorEvent := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event: waveEvent(
				filepath.Join(t.TempDir(), "concurrent-callback-error.txt"),
			),
		},
		HookCtx: &wave.HookContext{},
		Hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return nil, os.ErrPermission
					},
				},
			},
		},
	}
	if _, err := s.RunConcurrentHooks(concurrentErrorEvent, watcher); err == nil {
		t.Fatal("expected concurrent callback error to be propagated")
	}

	postErrorEvent := devserver.EventWithHooks{
		Classified: devserver.ClassifiedEvent{
			Event: waveEvent(
				filepath.Join(t.TempDir(), "post-callback-error.txt"),
			),
		},
		HookCtx: &wave.HookContext{},
		Hooks: &wave.SortedHooks{
			Post: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return nil, os.ErrNotExist
					},
				},
			},
		},
	}
	if _, err := s.RunPostHooks(postErrorEvent, watcher); err == nil {
		t.Fatal("expected post callback error to be propagated")
	}
}
