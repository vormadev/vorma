package runloop_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavebuild/builder"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/restartengine"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/runloop"
	"github.com/vormadev/vorma/wave/wavedev/internal/watch"
)

type batchedWatcherHarness struct {
	engine               *runloop.Engine
	restartAccumulator   *restartengine.RestartIntentAccumulator
	waitingForBuildRetry *atomic.Bool
}

type buildRetryRuntimeHarness struct {
	watcher      *watch.Watcher
	builder      *builder.Builder
	restartQueue *restartIntentQueueHarness
}

func newBuildRetryRuntimeHarness(
	watcherForRuntime *watch.Watcher,
	builderForRuntime *builder.Builder,
) *buildRetryRuntimeHarness {
	return &buildRetryRuntimeHarness{
		watcher:      watcherForRuntime,
		builder:      builderForRuntime,
		restartQueue: newRestartIntentQueueHarness(),
	}
}

func (harness *buildRetryRuntimeHarness) QueueRestartRequest(
	request restartengine.RestartRequest,
) {
	if harness == nil || harness.restartQueue == nil {
		return
	}
	harness.restartQueue.QueueRestartRequest(request)
}

func (harness *buildRetryRuntimeHarness) WaitForBuildRetry() restartengine.RestartRequest {
	if harness == nil || harness.restartQueue == nil {
		return restartengine.RestartRequest{}
	}
	return harness.restartQueue.WaitForBuildRetry()
}

func TestProcessBatchedEvents_AllRunOnChangeOnlySkipsBuildAndRestart(
	t *testing.T,
) {
	harness := newBatchedWatcherHarness(
		t,
		newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t.TempDir()),
		nil,
		nil,
	)

	var preCount atomic.Int32
	events := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event:       waveEvent(filepath.Join(t.TempDir(), "a.txt")),
				FileType:    eventpipeline.FileTypeOther,
				WatchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
				Pre: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							preCount.Add(1)
							return nil, nil
						},
					},
				},
			},
			RunOnChangeOnly: true,
		},
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event:       waveEvent(filepath.Join(t.TempDir(), "b.txt")),
				FileType:    eventpipeline.FileTypeOther,
				WatchedFile: &wave.WatchedFile{RunOnChangeOnly: true},
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
				Pre: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							preCount.Add(1)
							return nil, nil
						},
					},
				},
			},
			RunOnChangeOnly: true,
		},
	}

	work := &eventpipeline.WorkSet{}
	runEventsWithDerivedExecutionPlan(
		harness.engine,
		events,
		work,
		nil,
	)

	if preCount.Load() != 2 {
		t.Fatalf("expected both pre hooks to run, got %d", preCount.Load())
	}
	assertNoPendingRestartRequestForRunloopTests(
		t,
		harness.restartAccumulator,
	)
}

func TestProcessBatchedEvents_ConcurrentRestartSkipsPostHooks(t *testing.T) {
	harness := newBatchedWatcherHarness(
		t,
		newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t.TempDir()),
		nil,
		nil,
	)

	var postRan atomic.Bool
	events := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
				FileType: eventpipeline.FileTypeOther,
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
				Concurrent: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							return &wave.RefreshAction{
								TriggerRestart: true,
								RecompileGo:    true,
							}, nil
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

	work := &eventpipeline.WorkSet{}
	runEventsWithDerivedExecutionPlan(
		harness.engine,
		events,
		work,
		nil,
	)

	pendingRestartRequest := waitForPendingRestartRequestForRunloopTests(
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
	if postRan.Load() {
		t.Fatal(
			"did not expect post hooks to run after concurrent-triggered restart",
		)
	}
}

func TestProcessBatchedEvents_PostHookRestartNoGo(t *testing.T) {
	harness := newBatchedWatcherHarness(
		t,
		newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t.TempDir()),
		nil,
		nil,
	)

	events := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event:    waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
				FileType: eventpipeline.FileTypeOther,
			},
			HookCtx: &wave.HookContext{},
			Hooks: &wave.SortedHooks{
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
			RunOnChangeOnly: false,
		},
	}

	work := &eventpipeline.WorkSet{}
	runEventsWithDerivedExecutionPlan(
		harness.engine,
		events,
		work,
		nil,
	)

	pendingRestartRequest := waitForPendingRestartRequestForRunloopTests(
		t,
		harness.restartAccumulator,
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
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
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
	cfg.Watch.Include[0].Sort()
	cfg.Dist.Root = cfg.Core.DistDir

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()

	harness := newBatchedWatcherHarness(
		t,
		cfg,
		watcherForTest,
		builderForTest,
	)

	done := make(chan struct{})
	go func() {
		harness.engine.RunWatcherWithContext(context.Background())
		close(done)
	}()

	targetPath := filepath.Join(root, "watcher.txt")
	if writeError := os.WriteFile(targetPath, []byte("hello"), 0o644); writeError != nil {
		t.Fatalf("failed writing watched file: %v", writeError)
	}

	deadline := time.Now().Add(2 * time.Second)
	for callbackCount.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if callbackCount.Load() == 0 {
		t.Fatal(
			"expected RunWatcherWithContext to process at least one fsnotify event",
		)
	}

	if closeError := watcherForTest.Close(); closeError != nil {
		t.Fatalf("watcher.Close returned error: %v", closeError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("RunWatcherWithContext did not exit after watcher close")
	}
}

func TestRunWatcher_NilContextDefaultsToBackground(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
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
	cfg.Watch.Include[0].Sort()
	cfg.Dist.Root = cfg.Core.DistDir

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()

	harness := newBatchedWatcherHarness(
		t,
		cfg,
		watcherForTest,
		builderForTest,
	)

	done := make(chan struct{})
	go func() {
		var nilWatcherContext context.Context
		harness.engine.RunWatcherWithContext(nilWatcherContext)
		close(done)
	}()

	targetPath := filepath.Join(root, "watcher_nil_ctx.txt")
	if writeError := os.WriteFile(targetPath, []byte("hello"), 0o644); writeError != nil {
		t.Fatalf("failed writing watched file: %v", writeError)
	}

	deadline := time.Now().Add(2 * time.Second)
	for callbackCount.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if callbackCount.Load() == 0 {
		t.Fatal(
			"expected RunWatcherWithContext(nil) to process at least one fsnotify event",
		)
	}

	if closeError := watcherForTest.Close(); closeError != nil {
		t.Fatalf("watcher.Close returned error: %v", closeError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("RunWatcherWithContext(nil) did not exit after watcher close")
	}
}

func TestWaitForBuildRetry_ConsumesRestartAndCleansUp(t *testing.T) {
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
	}
	defer watcherForTest.Close()

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()

	serverForTest := newBuildRetryRuntimeHarness(
		watcherForTest,
		builderForTest,
	)
	serverForTest.QueueRestartRequest(
		restartengine.RestartRequest{RecompileGo: true},
	)

	restartRequest := serverForTest.WaitForBuildRetry()
	if !restartRequest.RecompileGo {
		t.Fatalf(
			"expected WaitForBuildRetry to return queued go-recompile intent, got %#v",
			restartRequest,
		)
	}

	if serverForTest.watcher == nil {
		t.Fatal(
			"expected WaitForBuildRetry to preserve watcher for cleanup stage",
		)
	}
	if serverForTest.builder == nil {
		t.Fatal(
			"expected WaitForBuildRetry to preserve builder for cleanup stage",
		)
	}
}

func TestProcessEvents_WaitingForBuildRetryQueuesRestartAndSkipsPipeline(
	t *testing.T,
) {
	testCases := []struct {
		name      string
		operation fsnotify.Op
	}{
		{name: "write", operation: fsnotify.Write},
		{name: "create", operation: fsnotify.Create},
		{name: "rename", operation: fsnotify.Rename},
		{name: "remove", operation: fsnotify.Remove},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(root)
			cfg.Core.ServerOnlyMode = true

			var preHookCallbackCount atomic.Int32
			cfg.Watch.Include = []wave.WatchedFile{
				{
					Pattern: "**/*.go",
					OnChangeHooks: []wave.OnChangeHook{
						{
							Timing: wave.OnChangeStrategyPre,
							Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
								preHookCallbackCount.Add(1)
								return nil, nil
							},
						},
					},
				},
			}
			cfg.Watch.Include[0].Sort()
			cfg.Dist.Root = cfg.Core.DistDir

			watcherForTest, watcherCreateError := watch.NewWatcher(
				cfg,
				newDiscardLoggerForRunloopBatchedWatcherTests(),
			)
			if watcherCreateError != nil {
				t.Fatalf("NewWatcher returned error: %v", watcherCreateError)
			}
			defer watcherForTest.Close()

			builderForTest := builder.NewBuilder(
				cfg,
				newDiscardLoggerForRunloopBatchedWatcherTests(),
			)
			defer builderForTest.Close()

			harness := newBatchedWatcherHarness(
				t,
				cfg,
				watcherForTest,
				builderForTest,
			)
			harness.waitingForBuildRetry.Store(true)

			changedPath := filepath.Join(root, "retry_wait_test.go")
			if writeError := os.WriteFile(
				changedPath,
				[]byte("package retrywait\n"),
				0o644,
			); writeError != nil {
				t.Fatalf("write changed file: %v", writeError)
			}

			harness.engine.ProcessEvents([]fsnotify.Event{{
				Name: changedPath,
				Op:   testCase.operation,
			}})

			if preHookCallbackCount.Load() != 0 {
				t.Fatalf(
					"expected waiting-for-build-retry path to skip hook pipeline, pre hook count=%d",
					preHookCallbackCount.Load(),
				)
			}

			restartRequest := waitForPendingRestartRequestForRunloopTests(
				t,
				harness.restartAccumulator,
				500*time.Millisecond,
			)
			if !restartRequest.RecompileGo || restartRequest.IsConfigRestart {
				t.Fatalf(
					"expected waiting-for-build-retry event to queue go-recompile restart, got %#v",
					restartRequest,
				)
			}
		})
	}
}

func newBatchedWatcherHarness(
	t *testing.T,
	cfg *wave.ParsedConfig,
	watcherForTest *watch.Watcher,
	builderForTest *builder.Builder,
) batchedWatcherHarness {
	t.Helper()

	var traceContextMutex sync.Mutex
	traceContext := runloop.WatcherExecutionTraceContext{}
	restartAccumulator := restartengine.NewRestartIntentAccumulator(
		make(chan restartengine.RestartRequest, 1),
	)
	waitingForBuildRetry := &atomic.Bool{}

	engine := runloop.New(
		runloop.Dependencies{
			Log:    newDiscardLoggerForRunloopBatchedWatcherTests(),
			Config: cfg,
			GetCurrentWatcher: func() *watch.Watcher {
				return watcherForTest
			},
			GetCurrentBuilder: func() *builder.Builder {
				return builderForTest
			},
			CurrentRunCycleContextOrBackground: func() context.Context {
				return context.Background()
			},
			ExecuteBuildPhase: func(*eventpipeline.WorkSet) error {
				return nil
			},
			ExecuteBrowserPhase: func(*eventpipeline.WorkSet) {},
			StartApp:            func() {},
			StopApp: func() error {
				return nil
			},
			TriggerRestart: func() {
				restartAccumulator.Queue(
					restartengine.RestartRequest{
						RecompileGo:     true,
						IsConfigRestart: false,
					},
				)
			},
			TriggerRestartNoGo: func() {
				restartAccumulator.Queue(
					restartengine.RestartRequest{
						RecompileGo:     false,
						IsConfigRestart: false,
					},
				)
			},
			TriggerConfigRestart: func() {
				restartAccumulator.Queue(
					restartengine.RestartRequest{
						RecompileGo:     true,
						IsConfigRestart: true,
					},
				)
			},
			BroadcastRebuilding: func() {},
			BuildEventExecutionPlan: func(
				events []fsnotify.Event,
				watcher *watch.Watcher,
				builder *builder.Builder,
			) eventpipeline.EventExecutionPlanningResult {
				return buildEventExecutionPlanForRunloopTests(
					cfg,
					events,
					watcher,
					builder,
				)
			},
			DeriveWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
				traceContextMutex.Lock()
				defer traceContextMutex.Unlock()
				traceContext.BatchID++
				return traceContext
			},
			SetCurrentWatcherExecutionTraceContext: func(
				contextForSet runloop.WatcherExecutionTraceContext,
			) {
				traceContextMutex.Lock()
				defer traceContextMutex.Unlock()
				traceContext = contextForSet
			},
			ClearCurrentWatcherExecutionTraceContext: func() {
				traceContextMutex.Lock()
				defer traceContextMutex.Unlock()
				traceContext = runloop.WatcherExecutionTraceContext{}
			},
			GetCurrentWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
				traceContextMutex.Lock()
				defer traceContextMutex.Unlock()
				return traceContext
			},
			RunNoWaitHookWithConcurrencyLimit: func(runNoWaitHook func()) {
				if runNoWaitHook != nil {
					runNoWaitHook()
				}
			},
			GetOrCreateConcurrentNoWaitHookLifecycleContext: func() context.Context {
				return context.Background()
			},
			IsWaitingForBuildRetry: func() bool {
				return waitingForBuildRetry.Load()
			},
		},
	)

	return batchedWatcherHarness{
		engine:               engine,
		restartAccumulator:   restartAccumulator,
		waitingForBuildRetry: waitingForBuildRetry,
	}
}

func assertNoPendingRestartRequestForRunloopTests(
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

func waitForPendingRestartRequestForRunloopTests(
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

func waveEvent(path string) fsnotify.Event {
	return fsnotify.Event{
		Name: path,
		Op:   fsnotify.Write,
	}
}

func newParsedConfigForRunloopBatchedWatcherTestsAtRoot(
	root string,
) *wave.ParsedConfig {
	return wavetest.NewParsedConfigAtRoot(root)
}

func newDiscardLoggerForRunloopBatchedWatcherTests() *slog.Logger {
	return wavetest.NewDiscardLogger()
}
