package eventpipeline_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavebuild/builder"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/restartengine"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/runloop"
	"github.com/vormadev/vorma/wave/wavedev/internal/broadcast"
	"github.com/vormadev/vorma/wave/wavedev/internal/watch"
)

type eventPipelineHarness struct {
	engine             *runloop.Engine
	watcher            *watch.Watcher
	restartAccumulator *restartengine.RestartIntentAccumulator
	browserReloads     chan eventpipeline.ReloadOpts
	startAppCallCount  atomic.Int32
	stopAppCallCount   atomic.Int32
	appRunning         atomic.Bool
	buildPhaseError    error
}

func newEventPipelineHarness(
	t *testing.T,
	serverOnly bool,
) *eventPipelineHarness {
	t.Helper()

	root := t.TempDir()
	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry:   "cmd/app",
			DistDir:        filepath.Join(root, "dist"),
			ServerOnlyMode: serverOnly,
			StaticAssetDirs: staticAssetDirsForTests{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	cfg.Dist.Root = cfg.Core.DistDir

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForEventPipelineTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("watch.NewWatcher returned error: %v", watcherCreateError)
	}

	harness := &eventPipelineHarness{
		watcher: watcherForTest,
		restartAccumulator: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 2),
		),
		browserReloads: make(chan eventpipeline.ReloadOpts, 4),
	}

	engine := runloop.New(
		runloop.Dependencies{
			Log:    newDiscardLoggerForEventPipelineTests(),
			Config: cfg,
			GetCurrentWatcher: func() *watch.Watcher {
				return harness.watcher
			},
			GetCurrentBuilder: func() *builder.Builder {
				return nil
			},
			CurrentRunCycleContextOrBackground: func() context.Context {
				return context.Background()
			},
			ExecuteBuildPhase: func(*eventpipeline.WorkSet) error {
				return harness.buildPhaseError
			},
			ExecuteBrowserPhase: func(work *eventpipeline.WorkSet) {
				if work == nil {
					return
				}
				reloadOptions, shouldBroadcast := eventpipeline.PlanBrowserReloadForAction(
					work.Browser.Action,
					work.Browser,
				)
				if shouldBroadcast {
					harness.browserReloads <- reloadOptions
				}
			},
			StartApp: func() {
				harness.appRunning.Store(true)
				harness.startAppCallCount.Add(1)
			},
			StopApp: func() error {
				harness.appRunning.Store(false)
				harness.stopAppCallCount.Add(1)
				return nil
			},
			TriggerRestart: func() {
				harness.restartAccumulator.Queue(
					restartengine.RestartRequest{
						RecompileGo:     true,
						IsConfigRestart: false,
					},
				)
			},
			TriggerRestartNoGo: func() {
				harness.restartAccumulator.Queue(
					restartengine.RestartRequest{
						RecompileGo:     false,
						IsConfigRestart: false,
					},
				)
			},
			TriggerConfigRestart: func() {
				harness.restartAccumulator.Queue(
					restartengine.RestartRequest{
						RecompileGo:     true,
						IsConfigRestart: true,
					},
				)
			},
			BroadcastRebuilding: func() {},
			DeriveWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
				return runloop.WatcherExecutionTraceContext{}
			},
			SetCurrentWatcherExecutionTraceContext: func(runloop.WatcherExecutionTraceContext) {
			},
			ClearCurrentWatcherExecutionTraceContext: func() {},
			GetCurrentWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
				return runloop.WatcherExecutionTraceContext{}
			},
			RunNoWaitHookWithConcurrencyLimit: func(runNoWaitHook func()) {
				if runNoWaitHook != nil {
					go runNoWaitHook()
				}
			},
			GetOrCreateConcurrentNoWaitHookLifecycleContext: func() context.Context {
				return context.Background()
			},
		},
	)

	harness.engine = engine
	return harness
}

func runEventsWithDerivedExecutionPlanForEventPipelineTests(
	harness *eventPipelineHarness,
	events []eventpipeline.EventWithHooks,
	work *eventpipeline.WorkSet,
) {
	behavioralDecision := eventpipeline.DeriveEventExecutionPlanBehavioralDecisionFromEventsWithHooks(
		events,
	)
	harness.engine.ExecuteEventExecutionPlan(
		events,
		behavioralDecision,
		work,
		harness.watcher,
	)
}

func waitForPendingRestartRequestForEventPipelineTests(
	t *testing.T,
	restartAccumulator *restartengine.RestartIntentAccumulator,
	timeout time.Duration,
) restartengine.RestartRequest {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pendingRestartRequest, hasPendingRestartRequest := restartAccumulator.ConsumePending()
		if hasPendingRestartRequest {
			return pendingRestartRequest
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for pending restart request after %s", timeout)
	return restartengine.RestartRequest{}
}

func assertNoPendingRestartRequestForEventPipelineTests(
	t *testing.T,
	restartAccumulator *restartengine.RestartIntentAccumulator,
) {
	t.Helper()

	pendingRestartRequest, hasPendingRestartRequest := restartAccumulator.ConsumePending()
	if hasPendingRestartRequest {
		t.Fatalf(
			"expected no pending restart request, got %#v",
			pendingRestartRequest,
		)
	}
}

func TestProcessSingleEvent_PreHookRestartCanRequestGoRecompile(t *testing.T) {
	harness := newEventPipelineHarness(t, true)
	defer harness.watcher.Close()

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
				filepath.Join(t.TempDir(), "changed.txt"),
			),
			FileType: eventpipeline.FileTypeOther,
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

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)

	pendingRestartRequest := waitForPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
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
	harness := newEventPipelineHarness(t, true)
	defer harness.watcher.Close()

	harness.appRunning.Store(true)

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
				filepath.Join(t.TempDir(), "changed.go"),
			),
			FileType: eventpipeline.FileTypeGo,
			WatchedFile: &wave.WatchedFile{
				RunOnChangeOnly: true,
			},
		},
		HookCtx:         &wave.HookContext{},
		Hooks:           &wave.SortedHooks{},
		RunOnChangeOnly: true,
		NeedsHardReload: true,
	}

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)

	if harness.appRunning.Load() {
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
	assertNoPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
	)
}

func TestProcessSingleEvent_PostHookRestartShortCircuitsBrowserReload(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
				filepath.Join(t.TempDir(), "changed.txt"),
			),
			FileType: eventpipeline.FileTypeOther,
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

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)

	pendingRestartRequest := waitForPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
		200*time.Millisecond,
	)
	if pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected no-go restart from post hook, got %#v",
			pendingRestartRequest,
		)
	}

	select {
	case msg := <-harness.browserReloads:
		t.Fatalf(
			"did not expect browser reload after post-hook restart, got %#v",
			msg,
		)
	default:
	}
}

func TestProcessSingleEvent_ConcurrentActionCanTriggerBrowserReload(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
				filepath.Join(t.TempDir(), "changed.txt"),
			),
			FileType: eventpipeline.FileTypeOther,
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

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)

	select {
	case msg := <-harness.browserReloads:
		if msg.Payload.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for browser reload payload")
	}

	assertNoPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
	)
}

func TestProcessSingleEvent_RunOnChangeOnlyPostCallbackCanTriggerBrowserReload(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
				filepath.Join(t.TempDir(), "changed.txt"),
			),
			FileType:    eventpipeline.FileTypeOther,
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

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)

	select {
	case msg := <-harness.browserReloads:
		if msg.Payload.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal(
			"timed out waiting for run-on-change-only post callback reload payload",
		)
	}

	assertNoPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
	)
}

func TestProcessSingleEvent_SiteStyleRouteRegistryChangeUsesFastReloadOnly(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()

	routeRegistryPath := filepath.Join(
		t.TempDir(),
		"frontend",
		"src",
		"routes",
		"core.vorma.routes.ts",
	)

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event:    waveEventForEventPipelineTests(routeRegistryPath),
			FileType: eventpipeline.FileTypeOther,
			WatchedFile: &wave.WatchedFile{
				RunOnChangeOnly:            true,
				SkipRebuildingNotification: true,
			},
		},
		HookCtx: &wave.HookContext{},
		Hooks: &wave.SortedHooks{
			Post: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{
							ReloadBrowser: true,
							WaitForApp:    true,
							WaitForVite:   true,
						}, nil
					},
				},
			},
		},
		RunOnChangeOnly: true,
	}

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)

	if work.Build.CompileGo ||
		work.Build.BuildCriticalCSS ||
		work.Build.BuildNormalCSS ||
		work.Build.ProcessPublicFiles ||
		work.Build.ProcessPrivateFiles {
		t.Fatalf(
			"expected no implicit build work for route registry fast reload, got %#v",
			work.Build,
		)
	}
	if work.Restart.RestartApp {
		t.Fatalf(
			"expected no app restart for route registry fast reload, got %#v",
			work.Restart,
		)
	}

	select {
	case msg := <-harness.browserReloads:
		if msg.Payload.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
		if !msg.WaitApp || !msg.WaitVite {
			t.Fatalf(
				"expected fast reload to wait for app+vite, got waitApp=%v waitVite=%v",
				msg.WaitApp,
				msg.WaitVite,
			)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for route-registry fast reload payload")
	}

	assertNoPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
	)
}

func TestProcessSingleEvent_SiteStyleTemplateChangeProcessesChangedPathWithoutRestart(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()

	templatePath := filepath.Join(
		t.TempDir(),
		"backend",
		"assets",
		"entry.go.html",
	)

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event:    waveEventForEventPipelineTests(templatePath),
			FileType: eventpipeline.FileTypePrivateStatic,
		},
		HookCtx: &wave.HookContext{},
		Hooks: &wave.SortedHooks{
			Post: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{
							ReloadBrowser: true,
							WaitForApp:    true,
							WaitForVite:   true,
						}, nil
					},
				},
			},
		},
		RunOnChangeOnly: false,
	}

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)

	if !work.Build.ProcessPrivateFiles {
		t.Fatalf(
			"expected template change to process private static changed path, got %#v",
			work.Build,
		)
	}
	if len(work.Build.PrivateStaticChangedFilePaths) != 1 ||
		work.Build.PrivateStaticChangedFilePaths[0] != templatePath {
		t.Fatalf(
			"expected one template changed path %q, got %#v",
			templatePath,
			work.Build.PrivateStaticChangedFilePaths,
		)
	}
	if work.Build.CompileGo ||
		work.Build.BuildCriticalCSS ||
		work.Build.BuildNormalCSS ||
		work.Build.ProcessPublicFiles {
		t.Fatalf(
			"expected no unrelated build work for template change, got %#v",
			work.Build,
		)
	}
	if work.Restart.RestartApp {
		t.Fatalf(
			"expected no app restart for template callback success, got %#v",
			work.Restart,
		)
	}

	select {
	case msg := <-harness.browserReloads:
		if msg.Payload.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
		if !msg.WaitApp || !msg.WaitVite {
			t.Fatalf(
				"expected template fast reload to wait for app+vite, got waitApp=%v waitVite=%v",
				msg.WaitApp,
				msg.WaitVite,
			)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for template fast reload payload")
	}

	assertNoPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
	)
}

func TestProcessSingleEvent_SiteStyleMarkdownChangeRevalidatesWithoutRestart(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()

	markdownPath := filepath.Join(
		t.TempDir(),
		"backend",
		"assets",
		"markdown",
		"blog",
		"post.md",
	)

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event:    waveEventForEventPipelineTests(markdownPath),
			FileType: eventpipeline.FileTypePrivateStatic,
			WatchedFile: &wave.WatchedFile{
				OnlyRunClientDefinedRevalidateFunc: true,
				SkipRebuildingNotification:         true,
			},
		},
		HookCtx: &wave.HookContext{},
		Hooks:   &wave.SortedHooks{},
	}

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)

	if !work.Build.ProcessPrivateFiles {
		t.Fatalf(
			"expected markdown change to process private static changed path, got %#v",
			work.Build,
		)
	}
	if len(work.Build.PrivateStaticChangedFilePaths) != 1 ||
		work.Build.PrivateStaticChangedFilePaths[0] != markdownPath {
		t.Fatalf(
			"expected one markdown changed path %q, got %#v",
			markdownPath,
			work.Build.PrivateStaticChangedFilePaths,
		)
	}
	if !work.PreferRevalidate {
		t.Fatalf(
			"expected markdown change to prefer revalidate, got work=%#v",
			work,
		)
	}
	if work.Restart.RestartApp {
		t.Fatalf(
			"expected markdown revalidate flow not to restart app, got %#v",
			work.Restart,
		)
	}

	select {
	case msg := <-harness.browserReloads:
		if msg.Payload.ChangeType != broadcast.ChangeTypeRevalidate {
			t.Fatalf("expected revalidate payload, got %#v", msg)
		}
		if !msg.WaitApp {
			t.Fatalf(
				"expected markdown revalidate payload to wait for app, got %#v",
				msg,
			)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for markdown revalidate payload")
	}

	assertNoPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
	)
}

func TestProcessSingleEvent_SiteStylePublicStaticChangeUsesChangedPathProcessingOnly(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()

	publicStaticPath := filepath.Join(
		t.TempDir(),
		"frontend",
		"assets",
		"logo.svg",
	)

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event:    waveEventForEventPipelineTests(publicStaticPath),
			FileType: eventpipeline.FileTypePublicStatic,
			WatchedFile: &wave.WatchedFile{
				SkipRebuildingNotification: true,
			},
		},
		HookCtx: &wave.HookContext{},
		Hooks:   &wave.SortedHooks{},
	}

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)

	if !work.Build.ProcessPublicFiles {
		t.Fatalf(
			"expected public static change to process changed public path, got %#v",
			work.Build,
		)
	}
	if len(work.Build.PublicStaticChangedFilePaths) != 1 ||
		work.Build.PublicStaticChangedFilePaths[0] != publicStaticPath {
		t.Fatalf(
			"expected one public changed path %q, got %#v",
			publicStaticPath,
			work.Build.PublicStaticChangedFilePaths,
		)
	}
	if work.Build.CompileGo ||
		work.Build.BuildCriticalCSS ||
		work.Build.BuildNormalCSS ||
		work.Build.ProcessPrivateFiles {
		t.Fatalf(
			"expected no unrelated build work for public static change, got %#v",
			work.Build,
		)
	}
	if work.Restart.RestartApp {
		t.Fatalf(
			"expected no restart for public static changed-path processing, got %#v",
			work.Restart,
		)
	}

	select {
	case msg := <-harness.browserReloads:
		t.Fatalf(
			"did not expect browser reload payload in non-vite harness, got %#v",
			msg,
		)
	default:
	}

	assertNoPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
	)
}

func TestProcessSingleEvent_ImplicitRestartStartsApp(t *testing.T) {
	harness := newEventPipelineHarness(t, true)
	defer harness.watcher.Close()

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
				filepath.Join(t.TempDir(), "changed.txt"),
			),
			FileType: eventpipeline.FileTypeOther,
			WatchedFile: &wave.WatchedFile{
				RestartApp: true,
			},
		},
		HookCtx: &wave.HookContext{},
		Hooks:   &wave.SortedHooks{},
	}

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)
	if harness.startAppCallCount.Load() == 0 {
		t.Fatal("expected implicit restart work to start app")
	}
}

func TestProcessSingleEvent_BuildFailureShortCircuitsRestartAndBrowserReload(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()
	harness.buildPhaseError = errors.New("synthetic build failure")

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
				filepath.Join(t.TempDir(), "changed.go"),
			),
			FileType: eventpipeline.FileTypeGo,
		},
		HookCtx: &wave.HookContext{},
		Hooks:   &wave.SortedHooks{},
	}

	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
	)

	if harness.startAppCallCount.Load() != 0 {
		t.Fatalf(
			"did not expect app to start after build failure, start count=%d",
			harness.startAppCallCount.Load(),
		)
	}
	select {
	case msg := <-harness.browserReloads:
		t.Fatalf(
			"did not expect browser reload after build failure, got %#v",
			msg,
		)
	default:
	}
}

func TestProcessBatchedEvents_PrehookRestartShortCircuitsPostAndBrowser(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()

	var postHookRan atomic.Bool
	events := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEventForEventPipelineTests(
					filepath.Join(t.TempDir(), "a.txt"),
				),
				FileType: eventpipeline.FileTypeOther,
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
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEventForEventPipelineTests(
					filepath.Join(t.TempDir(), "b.txt"),
				),
				FileType: eventpipeline.FileTypeOther,
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

	work := &eventpipeline.WorkSet{}
	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		events,
		work,
	)

	pendingRestartRequest := waitForPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
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
	case msg := <-harness.browserReloads:
		t.Fatalf(
			"did not expect browser reload after pre-hook restart, got %#v",
			msg,
		)
	default:
	}
}

func TestProcessBatchedEvents_AggregatesActionsAndBroadcastsSingleReload(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()

	events := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEventForEventPipelineTests(
					filepath.Join(t.TempDir(), "a.txt"),
				),
				FileType: eventpipeline.FileTypeOther,
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
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEventForEventPipelineTests(
					filepath.Join(t.TempDir(), "b.txt"),
				),
				FileType: eventpipeline.FileTypeOther,
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

	work := &eventpipeline.WorkSet{}
	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		events,
		work,
	)

	select {
	case msg := <-harness.browserReloads:
		if msg.Payload.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for batched hard reload payload")
	}

	select {
	case extra := <-harness.browserReloads:
		t.Fatalf("expected single batched reload payload, got extra %#v", extra)
	default:
	}

	assertNoPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
	)
}

func TestProcessBatchedEvents_AllRunOnChangeOnlyPostCallbacksCanTriggerBrowserReload(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, false)
	defer harness.watcher.Close()

	events := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEventForEventPipelineTests(
					filepath.Join(t.TempDir(), "a.txt"),
				),
				FileType:    eventpipeline.FileTypeOther,
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

	work := &eventpipeline.WorkSet{}
	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		events,
		work,
	)

	select {
	case msg := <-harness.browserReloads:
		if msg.Payload.ChangeType != broadcast.ChangeTypeOther {
			t.Fatalf("expected hard reload payload, got %#v", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal(
			"timed out waiting for batched run-on-change-only reload payload",
		)
	}

	assertNoPendingRestartRequestForEventPipelineTests(
		t,
		harness.restartAccumulator,
	)
}

func TestProcessBatchedEvents_MixedBatchRunsRunOnChangeOnlyPostCallbacks(
	t *testing.T,
) {
	harness := newEventPipelineHarness(t, true)
	defer harness.watcher.Close()

	var runOnChangeOnlyPostCallbackRan atomic.Bool
	events := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEventForEventPipelineTests(
					filepath.Join(t.TempDir(), "a.txt"),
				),
				FileType:    eventpipeline.FileTypeOther,
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
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEventForEventPipelineTests(
					filepath.Join(t.TempDir(), "b.txt"),
				),
				FileType: eventpipeline.FileTypeOther,
			},
			HookCtx:         &wave.HookContext{},
			Hooks:           &wave.SortedHooks{},
			RunOnChangeOnly: false,
		},
	}

	work := &eventpipeline.WorkSet{}
	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		events,
		work,
	)

	if !runOnChangeOnlyPostCallbackRan.Load() {
		t.Fatal(
			"expected run-on-change-only post callback to run in mixed batch",
		)
	}
}

func TestProcessBatchedEvents_ImplicitRestartStartsApp(t *testing.T) {
	harness := newEventPipelineHarness(t, true)
	defer harness.watcher.Close()

	events := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEventForEventPipelineTests(
					filepath.Join(t.TempDir(), "changed.txt"),
				),
				FileType: eventpipeline.FileTypeOther,
				WatchedFile: &wave.WatchedFile{
					RestartApp: true,
				},
			},
			HookCtx: &wave.HookContext{},
			Hooks:   &wave.SortedHooks{},
		},
	}

	work := &eventpipeline.WorkSet{}
	runEventsWithDerivedExecutionPlanForEventPipelineTests(
		harness,
		events,
		work,
	)
	if harness.startAppCallCount.Load() == 0 {
		t.Fatal("expected batched implicit restart work to start app")
	}
}

func TestRunPreHooks_CallbackAndCommandErrorsAreReturned(t *testing.T) {
	harness := newEventPipelineHarness(t, true)
	defer harness.watcher.Close()

	callbackFailureEvent := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
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

	if _, runPreHookError := harness.engine.RunPreHooks(
		callbackFailureEvent,
		harness.watcher,
	); runPreHookError == nil {
		t.Fatal("expected callback failure to be returned by RunPreHooks")
	}

	commandFailureEvent := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
				filepath.Join(t.TempDir(), "command-failure.txt"),
			),
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

	actions, runPreHookError := harness.engine.RunPreHooks(
		commandFailureEvent,
		harness.watcher,
	)
	if runPreHookError == nil {
		t.Fatal("expected command failure to be returned by RunPreHooks")
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
	harness := newEventPipelineHarness(t, true)
	defer harness.watcher.Close()

	concurrentErrorEvent := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
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
	if _, runConcurrentHookError := harness.engine.RunConcurrentHooksWithContext(
		context.Background(),
		concurrentErrorEvent,
		harness.watcher,
	); runConcurrentHookError == nil {
		t.Fatal("expected concurrent callback error to be propagated")
	}

	postErrorEvent := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEventForEventPipelineTests(
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
	if _, runPostHookError := harness.engine.RunPostHooks(
		postErrorEvent,
		harness.watcher,
	); runPostHookError == nil {
		t.Fatal("expected post callback error to be propagated")
	}
}

func newDiscardLoggerForEventPipelineTests() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waveEventForEventPipelineTests(path string) fsnotify.Event {
	return fsnotify.Event{Name: path, Op: fsnotify.Write}
}
