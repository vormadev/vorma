package runloop_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/vormadev/vorma/internal/wavetest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveframework"
	"github.com/vormadev/vorma/wave/wavewatch"

	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/hooks"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/restartengine"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/runloop"
	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
)

type hookExecutionHarness struct {
	server             *runloopTestServer
	engine             *runloop.Engine
	watcher            *watch.Watcher
	restartAccumulator *restartengine.RestartIntentAccumulator
}

func newEngineAndWatcherForHookExecutionTest(
	t *testing.T,
) hookExecutionHarness {
	t.Helper()

	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)

	watcherForTest, watcherCreateError := watch.NewWatcher(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	if watcherCreateError != nil {
		t.Fatalf("watch.NewWatcher returned error: %v", watcherCreateError)
	}

	restartAccumulator := restartengine.NewRestartIntentAccumulator(
		make(chan restartengine.RestartRequest, 1),
	)
	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)
	serverForTest.RestartIntents = restartAccumulator

	engine := runloop.New(
		runloop.Dependencies{
			Log:    newDiscardLoggerForRunloopBatchedWatcherTests(),
			Config: cfg,
			GetCurrentWatcher: func() *watch.Watcher {
				return watcherForTest
			},
			GetCurrentBuilder: func() *builder.Builder {
				return nil
			},
			CurrentRunCycleContextOrBackground: serverForTest.CurrentRunCycleContextOrBackground,
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
			DeriveWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
				return runloop.WatcherExecutionTraceContext{}
			},
			SetCurrentWatcherExecutionTraceContext: func(runloop.WatcherExecutionTraceContext) {
			},
			ClearCurrentWatcherExecutionTraceContext: func() {},
			GetCurrentWatcherExecutionTraceContext: func() runloop.WatcherExecutionTraceContext {
				return runloop.WatcherExecutionTraceContext{}
			},
			RunNoWaitHookWithConcurrencyLimit:               serverForTest.RunNoWaitHookWithConcurrencyLimit,
			GetOrCreateConcurrentNoWaitHookLifecycleContext: serverForTest.GetOrCreateConcurrentNoWaitHookLifecycleContext,
			ResolveHookExecutionPlan:                        serverForTest.ResolveHookExecutionPlan,
		},
	)

	return hookExecutionHarness{
		server:             serverForTest,
		engine:             engine,
		watcher:            watcherForTest,
		restartAccumulator: restartAccumulator,
	}
}

func TestRunConcurrentHooks_RespectsExcludesAndCollectsActions(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	commandOut := filepath.Join(root, "concurrent.log")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	var callbackCalled atomic.Bool
	var excludedCalled atomic.Bool

	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						callbackCalled.Store(true)
						return &wavewatch.RefreshAction{
							ReloadBrowser: true,
						}, nil
					},
				},
				{
					Cmd: "printf 'cmd\\n' >> " + strconv.Quote(commandOut),
				},
				{
					Exclude: []string{changedPath},
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						excludedCalled.Store(true)
						return nil, nil
					},
				},
			},
		},
	}

	actions, runConcurrentHookError := harness.engine.RunConcurrentHooksWithContext(
		context.Background(),
		eventWithHooks,
		harness.watcher,
	)
	if runConcurrentHookError != nil {
		t.Fatalf(
			"RunConcurrentHooksWithContext returned error: %v",
			runConcurrentHookError,
		)
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

	data, readError := os.ReadFile(commandOut)
	if readError != nil {
		t.Fatalf("failed reading concurrent command output: %v", readError)
	}
	if string(data) != "cmd\n" {
		t.Fatalf("unexpected concurrent command output: %q", string(data))
	}
}

func TestRunConcurrentHooks_RunCombinedDevBuildHookCommands_UsesFrameworkBuildHookRunner(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.go")
	combinedExecutionLogPath := filepath.Join(root, "combined-execution.log")
	frameworkCommandFallbackLogPath := filepath.Join(
		root,
		"framework-command-fallback.log",
	)
	if err := os.WriteFile(changedPath, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	wavetest.SetCoreDevBuildHook(harness.server.Cfg,
		"printf 'user\\n' >> "+strconv.Quote(combinedExecutionLogPath),
	)
	waveframework.StateForConfig(harness.server.Cfg).DevBuildHook = "printf 'framework-command\\n' >> " + strconv.Quote(
		frameworkCommandFallbackLogPath,
	)

	var frameworkRunnerCallCount atomic.Int32
	waveframework.StateForConfig(harness.server.Cfg).RunBuildHook = func(
		hookExecutionContext context.Context,
		runInDevelopmentMode bool,
	) error {
		frameworkRunnerCallCount.Add(1)
		if !runInDevelopmentMode {
			return errors.New(
				"expected framework build hook runner to execute in development mode",
			)
		}
		if hookExecutionContext == nil {
			return errors.New(
				"expected non-nil framework build hook execution context",
			)
		}

		frameworkRunnerOutputFile, fileOpenError := os.OpenFile(
			combinedExecutionLogPath,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0o644,
		)
		if fileOpenError != nil {
			return fileOpenError
		}
		defer frameworkRunnerOutputFile.Close()
		if _, writeError := frameworkRunnerOutputFile.WriteString(
			"framework-runner\n",
		); writeError != nil {
			return writeError
		}
		return nil
	}

	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{
					RunCombinedDevBuildHookCommands: true,
				},
			},
		},
	}

	actions, runConcurrentHookError := harness.engine.RunConcurrentHooksWithContext(
		context.Background(),
		eventWithHooks,
		harness.watcher,
	)
	if runConcurrentHookError != nil {
		t.Fatalf(
			"RunConcurrentHooksWithContext returned error: %v",
			runConcurrentHookError,
		)
	}
	if len(actions) != 0 {
		t.Fatalf(
			"expected no concurrent actions from combined dev hooks, got %#v",
			actions,
		)
	}

	if got := frameworkRunnerCallCount.Load(); got != 1 {
		t.Fatalf("framework build hook runner call count = %d, want 1", got)
	}

	combinedExecutionLog, readError := os.ReadFile(combinedExecutionLogPath)
	if readError != nil {
		t.Fatalf("failed reading combined execution log: %v", readError)
	}
	if string(combinedExecutionLog) != "user\nframework-runner\n" {
		t.Fatalf(
			"combined execution log = %q, want %q",
			string(combinedExecutionLog),
			"user\nframework-runner\n",
		)
	}

	if _, statError := os.Stat(frameworkCommandFallbackLogPath); !os.IsNotExist(
		statError,
	) {
		t.Fatalf(
			"expected framework command fallback log not to exist when framework runner is configured, stat err: %v",
			statError,
		)
	}
}

func TestRunConcurrentHooks_ReturnsActionsInHookOrder(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						time.Sleep(20 * time.Millisecond)
						return &wavewatch.RefreshAction{
							ReloadBrowser: true,
						}, nil
					},
				},
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						return &wavewatch.RefreshAction{WaitForApp: true}, nil
					},
				},
			},
		},
	}

	actions, runConcurrentHookError := harness.engine.RunConcurrentHooksWithContext(
		context.Background(),
		eventWithHooks,
		harness.watcher,
	)
	if runConcurrentHookError != nil {
		t.Fatalf(
			"RunConcurrentHooksWithContext returned error: %v",
			runConcurrentHookError,
		)
	}
	if len(actions) != 2 {
		t.Fatalf("action count=%d, want 2", len(actions))
	}
	if !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected first action to be ReloadBrowser=true, got %#v",
			actions[0],
		)
	}
	if !actions[1].WaitForApp {
		t.Fatalf(
			"expected second action to be WaitForApp=true, got %#v",
			actions[1],
		)
	}
}

func TestRunConcurrentHooks_AggregatesMultipleHookErrors(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "aggregate-errors.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						return nil, os.ErrPermission
					},
				},
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						return nil, os.ErrNotExist
					},
				},
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						return &wavewatch.RefreshAction{
							ReloadBrowser: true,
						}, nil
					},
				},
			},
		},
	}

	actions, runConcurrentHookError := harness.engine.RunConcurrentHooksWithContext(
		context.Background(),
		eventWithHooks,
		harness.watcher,
	)
	if runConcurrentHookError == nil {
		t.Fatal("expected aggregated concurrent hook error")
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected successful concurrent actions to be preserved, got %#v",
			actions,
		)
	}
	if !strings.Contains(
		runConcurrentHookError.Error(),
		os.ErrPermission.Error(),
	) {
		t.Fatalf(
			"expected aggregated error to include permission failure, got %q",
			runConcurrentHookError.Error(),
		)
	}
	if !strings.Contains(
		runConcurrentHookError.Error(),
		os.ErrNotExist.Error(),
	) {
		t.Fatalf(
			"expected aggregated error to include not-exist failure, got %q",
			runConcurrentHookError.Error(),
		)
	}
}

func TestRunConcurrentHooksWithContext_CanceledContextSkipsHookExecution(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	var callbackCalled atomic.Bool
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						callbackCalled.Store(true)
						return &wavewatch.RefreshAction{
							ReloadBrowser: true,
						}, nil
					},
				},
			},
		},
	}

	concurrentHookExecutionContext, cancelConcurrentHookExecutionContext := context.WithCancel(
		context.Background(),
	)
	cancelConcurrentHookExecutionContext()

	actions, runConcurrentHookError := harness.engine.RunConcurrentHooksWithContext(
		concurrentHookExecutionContext,
		eventWithHooks,
		harness.watcher,
	)
	if runConcurrentHookError != nil {
		t.Fatalf(
			"RunConcurrentHooksWithContext returned error: %v",
			runConcurrentHookError,
		)
	}
	if len(actions) != 0 {
		t.Fatalf(
			"expected no actions when context is canceled, got %#v",
			actions,
		)
	}
	if callbackCalled.Load() {
		t.Fatal(
			"did not expect concurrent hook callback to run when context is canceled",
		)
	}
}

func TestRunConcurrentHooks_CallbackReceivesIndependentHookContexts(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "context-clone.txt")
	receivedHookContexts := make(chan *wavewatch.HookContext, 2)
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{
			FilePath:         changedPath,
			ChangedFilePaths: []string{changedPath},
		},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{
					Callback: func(hookContext *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						receivedHookContexts <- hookContext
						return nil, nil
					},
				},
				{
					Callback: func(hookContext *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						receivedHookContexts <- hookContext
						return nil, nil
					},
				},
			},
		},
	}

	if _, runConcurrentHookError := harness.engine.RunConcurrentHooksWithContext(
		context.Background(),
		eventWithHooks,
		harness.watcher,
	); runConcurrentHookError != nil {
		t.Fatalf(
			"RunConcurrentHooksWithContext returned error: %v",
			runConcurrentHookError,
		)
	}

	firstHookContext := <-receivedHookContexts
	secondHookContext := <-receivedHookContexts
	if firstHookContext == secondHookContext {
		t.Fatal(
			"expected concurrent callbacks to receive independent hook context instances",
		)
	}
	if firstHookContext.ExecutionContext == nil ||
		secondHookContext.ExecutionContext == nil {
		t.Fatal("expected callback hook contexts to include execution context")
	}
}

func TestRunConcurrentHooksWithContext_CallbackCanObserveCancellation(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "callback-cancellation.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{
					Callback: func(hookContext *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						select {
						case <-hookContext.ExecutionContext.Done():
							return &wavewatch.RefreshAction{
								ReloadBrowser: true,
							}, nil
						case <-time.After(2 * time.Second):
							return nil, os.ErrDeadlineExceeded
						}
					},
				},
			},
		},
	}

	concurrentHookExecutionContext, cancelConcurrentHookExecutionContext := context.WithTimeout(
		context.Background(),
		100*time.Millisecond,
	)
	defer cancelConcurrentHookExecutionContext()

	callbackStartTime := time.Now()
	actions, runConcurrentHookError := harness.engine.RunConcurrentHooksWithContext(
		concurrentHookExecutionContext,
		eventWithHooks,
		harness.watcher,
	)
	callbackElapsedTime := time.Since(callbackStartTime)
	if runConcurrentHookError != nil {
		t.Fatalf(
			"RunConcurrentHooksWithContext returned error: %v",
			runConcurrentHookError,
		)
	}
	if callbackElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected callback cancellation observation to return quickly, elapsed=%s",
			callbackElapsedTime,
		)
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected callback action after cancellation observation, got %#v",
			actions,
		)
	}
}

func TestRunConcurrentHooksForEvents_ReturnsActionsInEventOrder(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	firstChangedPath := filepath.Join(root, "first.txt")
	secondChangedPath := filepath.Join(root, "second.txt")
	if err := os.WriteFile(firstChangedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing first changed file: %v", err)
	}
	if err := os.WriteFile(secondChangedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing second changed file: %v", err)
	}

	eventsWithHooks := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEvent(firstChangedPath),
			},
			HookCtx: &wavewatch.HookContext{FilePath: firstChangedPath},
			Hooks: &wavewatch.SortedHooks{
				Concurrent: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							time.Sleep(20 * time.Millisecond)
							return &wavewatch.RefreshAction{
								ReloadBrowser: true,
							}, nil
						},
					},
				},
			},
		},
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEvent(secondChangedPath),
			},
			HookCtx: &wavewatch.HookContext{FilePath: secondChangedPath},
			Hooks: &wavewatch.SortedHooks{
				Concurrent: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							return &wavewatch.RefreshAction{
								TriggerRestart: true,
								RecompileGo:    false,
							}, nil
						},
					},
				},
			},
		},
	}

	actions, executionErrors := harness.engine.RunConcurrentHooksForEventsWithContextAndErrors(
		context.Background(),
		eventsWithHooks,
		harness.watcher,
	)
	if len(executionErrors) != 0 {
		t.Fatalf(
			"expected no concurrent hook execution errors, got %#v",
			executionErrors,
		)
	}
	if len(actions) != 2 {
		t.Fatalf("action count=%d, want 2", len(actions))
	}
	if !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected first action to come from first event, got %#v",
			actions[0],
		)
	}
	if !actions[1].TriggerRestart {
		t.Fatalf(
			"expected second action to come from second event, got %#v",
			actions[1],
		)
	}
}

func TestRunConcurrentHooksForEventsWithContext_CanceledContextSkipsExecution(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	firstChangedPath := filepath.Join(root, "first.txt")
	secondChangedPath := filepath.Join(root, "second.txt")
	if err := os.WriteFile(firstChangedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing first changed file: %v", err)
	}
	if err := os.WriteFile(secondChangedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing second changed file: %v", err)
	}

	var callbackCount atomic.Int32
	eventsWithHooks := []eventpipeline.EventWithHooks{
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEvent(firstChangedPath),
			},
			HookCtx: &wavewatch.HookContext{FilePath: firstChangedPath},
			Hooks: &wavewatch.SortedHooks{
				Concurrent: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							callbackCount.Add(1)
							return &wavewatch.RefreshAction{
								ReloadBrowser: true,
							}, nil
						},
					},
				},
			},
		},
		{
			Classified: eventpipeline.ClassifiedEvent{
				Event: waveEvent(secondChangedPath),
			},
			HookCtx: &wavewatch.HookContext{FilePath: secondChangedPath},
			Hooks: &wavewatch.SortedHooks{
				Concurrent: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							callbackCount.Add(1)
							return &wavewatch.RefreshAction{
								WaitForApp: true,
							}, nil
						},
					},
				},
			},
		},
	}

	concurrentHookExecutionContext, cancelConcurrentHookExecutionContext := context.WithCancel(
		context.Background(),
	)
	cancelConcurrentHookExecutionContext()

	actions, executionErrors := harness.engine.RunConcurrentHooksForEventsWithContextAndErrors(
		concurrentHookExecutionContext,
		eventsWithHooks,
		harness.watcher,
	)
	if len(executionErrors) != 0 {
		t.Fatalf(
			"expected no execution errors when context is pre-canceled, got %#v",
			executionErrors,
		)
	}
	if len(actions) != 0 {
		t.Fatalf(
			"expected no actions when context is canceled, got %#v",
			actions,
		)
	}
	if callbackCount.Load() != 0 {
		t.Fatalf(
			"expected callback count=0 when context is canceled, got %d",
			callbackCount.Load(),
		)
	}
}

func TestRunConcurrentHooksWithContext_CanceledContextStopsRunningCommand(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "concurrent-command.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{
					Cmd: "sleep 2",
				},
			},
		},
	}

	concurrentHookExecutionContext, cancelConcurrentHookExecutionContext := context.WithTimeout(
		context.Background(),
		100*time.Millisecond,
	)
	defer cancelConcurrentHookExecutionContext()

	commandStartTime := time.Now()
	_, runConcurrentHookError := harness.engine.RunConcurrentHooksWithContext(
		concurrentHookExecutionContext,
		eventWithHooks,
		harness.watcher,
	)
	commandElapsedTime := time.Since(commandStartTime)
	if runConcurrentHookError == nil {
		t.Fatal("expected concurrent command to return cancellation error")
	}
	if commandElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected canceled concurrent command to stop quickly, elapsed=%s",
			commandElapsedTime,
		)
	}
}

func TestRunPreHooks_CallbackTimeoutUsesPreStageSetting(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	wavetest.SetWatchPreCallbackTimeoutMilliseconds(harness.server.Cfg, 100)

	changedPath := filepath.Join(t.TempDir(), "pre-callback-timeout.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Pre: []wavewatch.OnChangeHook{
				{
					Callback: func(hookContext *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						select {
						case <-hookContext.ExecutionContext.Done():
							return &wavewatch.RefreshAction{
								ReloadBrowser: true,
							}, nil
						case <-time.After(2 * time.Second):
							return nil, os.ErrDeadlineExceeded
						}
					},
				},
			},
		},
	}

	callbackStartTime := time.Now()
	actions, runPreHooksError := harness.engine.RunPreHooks(
		eventWithHooks,
		harness.watcher,
	)
	callbackElapsedTime := time.Since(callbackStartTime)
	if runPreHooksError != nil {
		t.Fatalf(
			"expected pre callback timeout path to succeed cooperatively, got %v",
			runPreHooksError,
		)
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected pre callback action after timeout cancellation, got %#v",
			actions,
		)
	}
	if callbackElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected pre callback timeout to return quickly, elapsed=%s",
			callbackElapsedTime,
		)
	}
}

func TestRunPreHooks_UsesRunCycleContextForCallbackExecution(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	canceledRunCycleContext, cancelRunCycleContext := context.WithCancel(
		context.Background(),
	)
	cancelRunCycleContext()
	harness.server.SetCurrentRunCycleContext(canceledRunCycleContext)

	changedPath := filepath.Join(t.TempDir(), "pre-run-cycle-context.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Pre: []wavewatch.OnChangeHook{
				{
					Callback: func(
						hookContext *wavewatch.HookContext,
					) (*wavewatch.RefreshAction, error) {
						if hookContext == nil ||
							hookContext.ExecutionContext == nil {
							return nil, errors.New(
								"expected callback execution context",
							)
						}
						if hookContext.ExecutionContext.Err() == nil {
							return nil, errors.New(
								"expected canceled callback execution context from run cycle context",
							)
						}
						return nil, hookContext.ExecutionContext.Err()
					},
				},
			},
		},
	}

	_, runPreHooksError := harness.engine.RunPreHooks(
		eventWithHooks,
		harness.watcher,
	)
	if runPreHooksError == nil {
		t.Fatal(
			"expected pre hook callback to observe canceled run cycle context",
		)
	}
	if !errors.Is(runPreHooksError, context.Canceled) {
		t.Fatalf(
			"expected canceled run cycle context error, got %v",
			runPreHooksError,
		)
	}
	if !strings.Contains(
		runPreHooksError.Error(),
		"pre hook failed for "+changedPath,
	) {
		t.Fatalf(
			"expected pre hook stage/path attribution, got %q",
			runPreHooksError.Error(),
		)
	}
}

func TestRunPreHooks_CommandTimeoutUsesPreStageSetting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	wavetest.SetWatchPreCommandTimeoutMilliseconds(harness.server.Cfg, 100)

	changedPath := filepath.Join(t.TempDir(), "pre-timeout.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Pre: []wavewatch.OnChangeHook{
				{Cmd: "sleep 2"},
			},
		},
	}

	commandStartTime := time.Now()
	_, runPreHooksError := harness.engine.RunPreHooks(
		eventWithHooks,
		harness.watcher,
	)
	commandElapsedTime := time.Since(commandStartTime)
	if runPreHooksError == nil {
		t.Fatal("expected pre hook command to time out")
	}
	if !errors.Is(runPreHooksError, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf(
			"expected timed-out command classification, got %v",
			runPreHooksError,
		)
	}
	if !strings.Contains(
		runPreHooksError.Error(),
		"pre hook failed for "+changedPath,
	) {
		t.Fatalf(
			"expected pre hook stage/path attribution, got %q",
			runPreHooksError.Error(),
		)
	}
	if commandElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected pre hook timeout to stop quickly, elapsed=%s",
			commandElapsedTime,
		)
	}
}

func TestRunConcurrentHooks_CommandTimeoutUsesConcurrentStageSetting(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	wavetest.SetWatchConcurrentCommandTimeoutMilliseconds(harness.server.Cfg, 100)

	changedPath := filepath.Join(t.TempDir(), "concurrent-timeout.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{Cmd: "sleep 2"},
			},
		},
	}

	commandStartTime := time.Now()
	_, runConcurrentHooksError := harness.engine.RunConcurrentHooksWithContext(
		context.Background(),
		eventWithHooks,
		harness.watcher,
	)
	commandElapsedTime := time.Since(commandStartTime)
	if runConcurrentHooksError == nil {
		t.Fatal("expected concurrent hook command to time out")
	}
	if !errors.Is(
		runConcurrentHooksError,
		executil.ErrCommandExecutionTimedOut,
	) {
		t.Fatalf(
			"expected timed-out command classification, got %v",
			runConcurrentHooksError,
		)
	}
	if !strings.Contains(
		runConcurrentHooksError.Error(),
		"concurrent hook failed for "+changedPath,
	) {
		t.Fatalf(
			"expected concurrent hook stage/path attribution, got %q",
			runConcurrentHooksError.Error(),
		)
	}
	if commandElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected concurrent hook timeout to stop quickly, elapsed=%s",
			commandElapsedTime,
		)
	}
}

func TestRunPostHooks_CommandTimeoutUsesPostStageSetting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	wavetest.SetWatchPostCommandTimeoutMilliseconds(harness.server.Cfg, 100)

	changedPath := filepath.Join(t.TempDir(), "post-timeout.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Post: []wavewatch.OnChangeHook{
				{Cmd: "sleep 2"},
			},
		},
	}

	commandStartTime := time.Now()
	_, runPostHooksError := harness.engine.RunPostHooks(
		eventWithHooks,
		harness.watcher,
	)
	commandElapsedTime := time.Since(commandStartTime)
	if runPostHooksError == nil {
		t.Fatal("expected post hook command to time out")
	}
	if !errors.Is(runPostHooksError, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf(
			"expected timed-out command classification, got %v",
			runPostHooksError,
		)
	}
	if !strings.Contains(
		runPostHooksError.Error(),
		"post hook failed for "+changedPath,
	) {
		t.Fatalf(
			"expected post hook stage/path attribution, got %q",
			runPostHooksError.Error(),
		)
	}
	if commandElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected post hook timeout to stop quickly, elapsed=%s",
			commandElapsedTime,
		)
	}
}

func TestRunPostHooks_UsesRunCycleContextForCallbackExecution(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	canceledRunCycleContext, cancelRunCycleContext := context.WithCancel(
		context.Background(),
	)
	cancelRunCycleContext()
	harness.server.SetCurrentRunCycleContext(canceledRunCycleContext)

	changedPath := filepath.Join(t.TempDir(), "post-run-cycle-context.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Post: []wavewatch.OnChangeHook{
				{
					Callback: func(
						hookContext *wavewatch.HookContext,
					) (*wavewatch.RefreshAction, error) {
						if hookContext == nil ||
							hookContext.ExecutionContext == nil {
							return nil, errors.New(
								"expected callback execution context",
							)
						}
						if hookContext.ExecutionContext.Err() == nil {
							return nil, errors.New(
								"expected canceled callback execution context from run cycle context",
							)
						}
						return nil, hookContext.ExecutionContext.Err()
					},
				},
			},
		},
	}

	_, runPostHooksError := harness.engine.RunPostHooks(
		eventWithHooks,
		harness.watcher,
	)
	if runPostHooksError == nil {
		t.Fatal(
			"expected post hook callback to observe canceled run cycle context",
		)
	}
	if !errors.Is(runPostHooksError, context.Canceled) {
		t.Fatalf(
			"expected canceled run cycle context error, got %v",
			runPostHooksError,
		)
	}
	if !strings.Contains(
		runPostHooksError.Error(),
		"post hook failed for "+changedPath,
	) {
		t.Fatalf(
			"expected post hook stage/path attribution, got %q",
			runPostHooksError.Error(),
		)
	}
}

func TestRunPreHooks_PerHookCommandTimeoutOverridesStageTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	wavetest.SetWatchPreCommandTimeoutMilliseconds(harness.server.Cfg, 2000)

	changedPath := filepath.Join(t.TempDir(), "pre-timeout-override.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Pre: []wavewatch.OnChangeHook{
				{
					Cmd:                        "sleep 2",
					CommandTimeoutMilliseconds: 100,
				},
			},
		},
	}

	commandStartTime := time.Now()
	_, runPreHooksError := harness.engine.RunPreHooks(
		eventWithHooks,
		harness.watcher,
	)
	commandElapsedTime := time.Since(commandStartTime)
	if runPreHooksError == nil {
		t.Fatal("expected pre hook command timeout override to trigger")
	}
	if !errors.Is(runPreHooksError, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf(
			"expected timed-out command classification, got %v",
			runPreHooksError,
		)
	}
	if !strings.Contains(
		runPreHooksError.Error(),
		"pre hook failed for "+changedPath,
	) {
		t.Fatalf(
			"expected pre hook stage/path attribution, got %q",
			runPreHooksError.Error(),
		)
	}
	if commandElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected per-hook timeout override to stop quickly, elapsed=%s",
			commandElapsedTime,
		)
	}
}

func TestRunPreHooks_DisableStageCommandTimeoutBypassesStageTimeout(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	wavetest.SetWatchPreCommandTimeoutMilliseconds(harness.server.Cfg, 100)

	changedPath := filepath.Join(t.TempDir(), "pre-timeout-disable.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Pre: []wavewatch.OnChangeHook{
				{
					Cmd:                        "sleep 1",
					DisableStageCommandTimeout: true,
				},
			},
		},
	}

	commandStartTime := time.Now()
	_, runPreHooksError := harness.engine.RunPreHooks(
		eventWithHooks,
		harness.watcher,
	)
	commandElapsedTime := time.Since(commandStartTime)
	if runPreHooksError != nil {
		t.Fatalf(
			"expected stage-timeout-disabled pre hook to succeed, got %v",
			runPreHooksError,
		)
	}
	if commandElapsedTime < 800*time.Millisecond {
		t.Fatalf(
			"expected disabled stage timeout to allow hook command runtime, elapsed=%s",
			commandElapsedTime,
		)
	}
}

func TestRunPreHooks_PerHookCallbackTimeoutOverridesStageTimeout(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	wavetest.SetWatchPreCallbackTimeoutMilliseconds(harness.server.Cfg, 2000)

	changedPath := filepath.Join(
		t.TempDir(),
		"pre-callback-timeout-override.txt",
	)
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Pre: []wavewatch.OnChangeHook{
				{
					CallbackTimeoutMilliseconds: 100,
					Callback: func(hookContext *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						select {
						case <-hookContext.ExecutionContext.Done():
							return &wavewatch.RefreshAction{
								ReloadBrowser: true,
							}, nil
						case <-time.After(2 * time.Second):
							return nil, os.ErrDeadlineExceeded
						}
					},
				},
			},
		},
	}

	callbackStartTime := time.Now()
	actions, runPreHooksError := harness.engine.RunPreHooks(
		eventWithHooks,
		harness.watcher,
	)
	callbackElapsedTime := time.Since(callbackStartTime)
	if runPreHooksError != nil {
		t.Fatalf(
			"expected per-hook callback timeout override to succeed cooperatively, got %v",
			runPreHooksError,
		)
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected callback action after per-hook timeout override, got %#v",
			actions,
		)
	}
	if callbackElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected per-hook callback timeout override to return quickly, elapsed=%s",
			callbackElapsedTime,
		)
	}
}

func TestRunPreHooks_DisableStageCallbackTimeoutBypassesStageTimeout(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	wavetest.SetWatchPreCallbackTimeoutMilliseconds(harness.server.Cfg, 100)

	changedPath := filepath.Join(
		t.TempDir(),
		"pre-callback-timeout-disable.txt",
	)
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Pre: []wavewatch.OnChangeHook{
				{
					DisableStageCallbackTimeout: true,
					Callback: func(hookContext *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						select {
						case <-hookContext.ExecutionContext.Done():
							return nil, os.ErrDeadlineExceeded
						case <-time.After(300 * time.Millisecond):
							return &wavewatch.RefreshAction{
								ReloadBrowser: true,
							}, nil
						}
					},
				},
			},
		},
	}

	callbackStartTime := time.Now()
	actions, runPreHooksError := harness.engine.RunPreHooks(
		eventWithHooks,
		harness.watcher,
	)
	callbackElapsedTime := time.Since(callbackStartTime)
	if runPreHooksError != nil {
		t.Fatalf(
			"expected disabled stage callback timeout to allow callback runtime, got %v",
			runPreHooksError,
		)
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected callback action when stage callback timeout is disabled, got %#v",
			actions,
		)
	}
	if callbackElapsedTime < 250*time.Millisecond {
		t.Fatalf(
			"expected disabled stage callback timeout to allow callback runtime, elapsed=%s",
			callbackElapsedTime,
		)
	}
}

func TestRunPostHooks_StopsOnCommandError(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	var lateCallbackCalled atomic.Bool
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(filepath.Join(t.TempDir(), "changed.txt")),
		},
		HookCtx: &wavewatch.HookContext{},
		Hooks: &wavewatch.SortedHooks{
			Post: []wavewatch.OnChangeHook{
				{Cmd: "false"},
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						lateCallbackCalled.Store(true)
						return nil, nil
					},
				},
			},
		},
	}

	actions, runPostHooksError := harness.engine.RunPostHooks(
		eventWithHooks,
		harness.watcher,
	)
	if runPostHooksError == nil {
		t.Fatal("expected RunPostHooks to fail on command error")
	}
	if len(actions) != 0 {
		t.Fatalf("expected no actions before failure, got %#v", actions)
	}
	if lateCallbackCalled.Load() {
		t.Fatal("did not expect hooks after failing command to run")
	}
}

func TestRunPostHooks_RunOnChangeOnlySkipsCommandAndKeepsCallback(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	commandOut := filepath.Join(root, "post-command.log")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	var callbackCalled atomic.Bool
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx:         &wavewatch.HookContext{FilePath: changedPath},
		RunOnChangeOnly: true,
		Hooks: &wavewatch.SortedHooks{
			Post: []wavewatch.OnChangeHook{
				{
					Cmd: "printf 'should-not-run\\n' >> " + strconv.Quote(
						commandOut,
					),
				},
				{
					Cmd: "printf 'should-not-run\\n' >> " + strconv.Quote(
						commandOut,
					),
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						callbackCalled.Store(true)
						return &wavewatch.RefreshAction{
							ReloadBrowser: true,
						}, nil
					},
				},
			},
		},
	}

	actions, runPostHooksError := harness.engine.RunPostHooks(
		eventWithHooks,
		harness.watcher,
	)
	if runPostHooksError != nil {
		t.Fatalf("RunPostHooks returned error: %v", runPostHooksError)
	}
	if !callbackCalled.Load() {
		t.Fatal(
			"expected callback hook to run for run-on-change-only post hooks",
		)
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf("unexpected post hook actions: %#v", actions)
	}
	if _, statError := os.Stat(commandOut); !os.IsNotExist(statError) {
		t.Fatalf(
			"expected run-on-change-only post commands to be skipped, stat error: %v",
			statError,
		)
	}
}

func TestRunPreHooks_ErrorIncludesStageAndChangedPath(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "pre-error.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Pre: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						return nil, os.ErrInvalid
					},
				},
			},
		},
	}

	_, runPreHooksError := harness.engine.RunPreHooks(
		eventWithHooks,
		harness.watcher,
	)
	if runPreHooksError == nil {
		t.Fatal("expected pre-hook error")
	}
	if !strings.Contains(
		runPreHooksError.Error(),
		"pre hook failed for "+changedPath,
	) {
		t.Fatalf(
			"expected pre-hook error to include stage/path attribution, got %q",
			runPreHooksError.Error(),
		)
	}
}

func TestRunConcurrentHooks_ErrorIncludesStageAndChangedPath(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "concurrent-error.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						return nil, os.ErrPermission
					},
				},
			},
		},
	}

	_, runConcurrentHooksError := harness.engine.RunConcurrentHooksWithContext(
		context.Background(),
		eventWithHooks,
		harness.watcher,
	)
	if runConcurrentHooksError == nil {
		t.Fatal("expected concurrent-hook error")
	}
	if !strings.Contains(
		runConcurrentHooksError.Error(),
		"concurrent hook failed for "+changedPath,
	) {
		t.Fatalf(
			"expected concurrent-hook error to include stage/path attribution, got %q",
			runConcurrentHooksError.Error(),
		)
	}
}

func TestRunPostHooks_ErrorIncludesStageAndChangedPath(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "post-error.txt")
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			Post: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						return nil, os.ErrNotExist
					},
				},
			},
		},
	}

	_, runPostHooksError := harness.engine.RunPostHooks(
		eventWithHooks,
		harness.watcher,
	)
	if runPostHooksError == nil {
		t.Fatal("expected post-hook error")
	}
	if !strings.Contains(
		runPostHooksError.Error(),
		"post hook failed for "+changedPath,
	) {
		t.Fatalf(
			"expected post-hook error to include stage/path attribution, got %q",
			runPostHooksError.Error(),
		)
	}
}

func TestFireNoWaitHooks_RunsAsyncCallbackAndCommand(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	commandOut := filepath.Join(root, "nowait.log")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	callbackDone := make(chan struct{}, 1)
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			ConcurrentNoWait: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
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

	harness.engine.FireNoWaitHooks(eventWithHooks, harness.watcher)

	select {
	case <-callbackDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for no-wait callback")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		data, readError := os.ReadFile(commandOut)
		if readError == nil && string(data) == "async\n" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"timed out waiting for no-wait command output (last read err=%v)",
				readError,
			)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestFireNoWaitHooks_CallbacksReceiveIndependentHookContexts(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "no-wait-context-clone.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	harness.server.ConcurrentNoWaitHookExecutionLimiter = make(chan struct{}, 2)

	receivedHookContexts := make(chan *wavewatch.HookContext, 2)
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{
			FilePath:         changedPath,
			ChangedFilePaths: []string{changedPath},
		},
		Hooks: &wavewatch.SortedHooks{
			ConcurrentNoWait: []wavewatch.OnChangeHook{
				{
					Callback: func(hookContext *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						receivedHookContexts <- hookContext
						return nil, nil
					},
				},
				{
					Callback: func(hookContext *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						receivedHookContexts <- hookContext
						return nil, nil
					},
				},
			},
		},
	}

	harness.engine.FireNoWaitHooks(eventWithHooks, harness.watcher)

	var firstHookContext *wavewatch.HookContext
	select {
	case firstHookContext = <-receivedHookContexts:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for first no-wait callback context")
	}

	var secondHookContext *wavewatch.HookContext
	select {
	case secondHookContext = <-receivedHookContexts:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for second no-wait callback context")
	}

	if firstHookContext == secondHookContext {
		t.Fatal(
			"expected no-wait callbacks to receive independent hook context instances",
		)
	}
	if firstHookContext.ExecutionContext == nil ||
		secondHookContext.ExecutionContext == nil {
		t.Fatal(
			"expected no-wait callback hook contexts to include execution context",
		)
	}
}

func TestFireNoWaitHooks_CallbackCanObserveExecutionContextCancellation(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	changedPath := filepath.Join(
		t.TempDir(),
		"no-wait-callback-cancellation.txt",
	)
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	wavetest.SetWatchConcurrentNoWaitCommandTimeoutMilliseconds(harness.server.Cfg, 100)
	wavetest.SetWatchConcurrentNoWaitCallbackTimeoutMilliseconds(harness.server.Cfg, 100)

	callbackDone := make(chan struct{}, 1)
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			ConcurrentNoWait: []wavewatch.OnChangeHook{
				{
					Callback: func(hookContext *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						select {
						case <-hookContext.ExecutionContext.Done():
							callbackDone <- struct{}{}
							return nil, nil
						case <-time.After(2 * time.Second):
							return nil, os.ErrDeadlineExceeded
						}
					},
				},
			},
		},
	}

	harness.engine.FireNoWaitHooks(eventWithHooks, harness.watcher)

	select {
	case <-callbackDone:
	case <-time.After(1 * time.Second):
		t.Fatal(
			"timed out waiting for no-wait callback to observe execution context cancellation",
		)
	}
}

func TestFireNoWaitHooks_CleanupForRebuildCancelsInFlightHooks(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	harness.server.Watcher = harness.watcher

	changedPath := filepath.Join(t.TempDir(), "no-wait-cleanup-cancel.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	hookStarted := make(chan struct{}, 1)
	hookCanceled := make(chan struct{}, 1)
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			ConcurrentNoWait: []wavewatch.OnChangeHook{
				{
					Callback: func(hookContext *wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						hookStarted <- struct{}{}
						select {
						case <-hookContext.ExecutionContext.Done():
							hookCanceled <- struct{}{}
						case <-time.After(600 * time.Millisecond):
						}
						return nil, nil
					},
				},
			},
		},
	}

	harness.engine.FireNoWaitHooks(eventWithHooks, harness.watcher)

	select {
	case <-hookStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for no-wait hook to start")
	}

	harness.server.CleanupForRebuild()

	select {
	case <-hookCanceled:
	case <-time.After(300 * time.Millisecond):
		t.Fatal(
			"expected CleanupForRebuild to cancel in-flight no-wait hook execution",
		)
	}
}

func TestFireNoWaitHooks_ExcludesMatchingHooksAndToleratesFailures(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	var excludedHookRan atomic.Bool
	failingHookCalled := make(chan struct{}, 1)
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			ConcurrentNoWait: []wavewatch.OnChangeHook{
				{
					Exclude: []string{changedPath},
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						excludedHookRan.Store(true)
						return nil, nil
					},
				},
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						failingHookCalled <- struct{}{}
						return nil, os.ErrInvalid
					},
					Cmd: "false",
				},
			},
		},
	}

	harness.engine.FireNoWaitHooks(eventWithHooks, harness.watcher)

	select {
	case <-failingHookCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for failing no-wait callback to be invoked")
	}

	if excludedHookRan.Load() {
		t.Fatal("did not expect excluded no-wait hook callback to run")
	}
}

func TestFireNoWaitHooks_CommandTimeoutUsesConcurrentNoWaitStageSetting(
	t *testing.T,
) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	harness.server.ConcurrentNoWaitHookExecutionLimiter = make(chan struct{}, 1)
	wavetest.SetWatchConcurrentNoWaitCommandTimeoutMilliseconds(harness.server.Cfg, 100)

	changedPath := filepath.Join(t.TempDir(), "concurrent-no-wait-timeout.txt")
	secondHookRanMarkerPath := filepath.Join(
		t.TempDir(),
		"concurrent-no-wait-second-hook-ran.txt",
	)
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			ConcurrentNoWait: []wavewatch.OnChangeHook{
				{Cmd: "sleep 2"},
				{
					Cmd: "printf 'ran\\n' >> " + strconv.Quote(
						secondHookRanMarkerPath,
					),
				},
			},
		},
	}

	stageExecutionStartTime := time.Now()
	harness.engine.FireNoWaitHooks(eventWithHooks, harness.watcher)

	for {
		if _, statError := os.Stat(secondHookRanMarkerPath); statError == nil {
			break
		}

		stageExecutionElapsedTime := time.Since(stageExecutionStartTime)
		if stageExecutionElapsedTime > 1500*time.Millisecond {
			t.Fatalf(
				"expected second no-wait hook command to run within timeout window, elapsed=%s",
				stageExecutionElapsedTime,
			)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestFireNoWaitHooks_CallbackPanicDoesNotStopOtherHooks(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	panicHookCalled := make(chan struct{}, 1)
	followUpHookCalled := make(chan struct{}, 1)
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event: waveEvent(changedPath),
		},
		HookCtx: &wavewatch.HookContext{FilePath: changedPath},
		Hooks: &wavewatch.SortedHooks{
			ConcurrentNoWait: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						panicHookCalled <- struct{}{}
						panic("expected test panic in no-wait callback")
					},
				},
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						followUpHookCalled <- struct{}{}
						return nil, nil
					},
				},
			},
		},
	}

	harness.engine.FireNoWaitHooks(eventWithHooks, harness.watcher)

	select {
	case <-panicHookCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for panicking no-wait callback")
	}

	select {
	case <-followUpHookCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for follow-up no-wait callback after panic")
	}
}

func TestFireNoWaitHooksForEvents_SkipsDuplicateHooks(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	firstPath := filepath.Join(root, "first.txt")
	secondPath := filepath.Join(root, "second.txt")
	commandOut := filepath.Join(root, "nowait-dedupe.log")
	if err := os.WriteFile(firstPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing first changed file: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing second changed file: %v", err)
	}

	var firstCallbackCount int32
	var duplicateCallbackCount int32
	eventsWithHooks := []eventpipeline.EventWithHooks{
		newEventWithHooksForStageExecutionTest(
			firstPath,
			&wavewatch.SortedHooks{
				ConcurrentNoWait: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&firstCallbackCount, 1)
							return nil, nil
						},
					},
					{
						Cmd: "printf 'first\\n' >> " + strconv.Quote(
							commandOut,
						),
					},
				},
			},
			false,
		),
		newEventWithHooksForStageExecutionTest(
			secondPath,
			&wavewatch.SortedHooks{
				ConcurrentNoWait: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&duplicateCallbackCount, 1)
							return nil, nil
						},
					},
					{
						Cmd: "printf 'duplicate\\n' >> " + strconv.Quote(
							commandOut,
						),
					},
				},
			},
			true,
		),
	}

	harness.engine.FireNoWaitHooksForEvents(eventsWithHooks, harness.watcher)

	waitDeadline := time.Now().Add(700 * time.Millisecond)
	for atomic.LoadInt32(&firstCallbackCount) != 1 {
		if time.Now().After(waitDeadline) {
			t.Fatalf(
				"timed out waiting for no-wait callback count (first=%d duplicate=%d)",
				atomic.LoadInt32(&firstCallbackCount),
				atomic.LoadInt32(&duplicateCallbackCount),
			)
		}
		time.Sleep(10 * time.Millisecond)
	}

	commandOutputDeadline := time.Now().Add(700 * time.Millisecond)
	for {
		data, readError := os.ReadFile(commandOut)
		if readError == nil && string(data) == "first\n" {
			break
		}
		if time.Now().After(commandOutputDeadline) {
			t.Fatalf(
				"timed out waiting for no-wait command output (err=%v, first=%d duplicate=%d)",
				readError,
				atomic.LoadInt32(&firstCallbackCount),
				atomic.LoadInt32(&duplicateCallbackCount),
			)
		}
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(75 * time.Millisecond)
	if atomic.LoadInt32(&duplicateCallbackCount) != 0 {
		t.Fatalf(
			"expected duplicate no-wait callback not to run, got %d calls",
			atomic.LoadInt32(&duplicateCallbackCount),
		)
	}

	finalCommandOutput, readError := os.ReadFile(commandOut)
	if readError != nil {
		t.Fatalf("failed reading no-wait command output: %v", readError)
	}
	if string(finalCommandOutput) != "first\n" {
		t.Fatalf(
			"expected duplicate no-wait command not to run, got output %q",
			string(finalCommandOutput),
		)
	}
}

func TestRunPreHooksForEvents_SkipsDuplicateHooksAndKeepsActionOrder(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	firstPath := filepath.Join(root, "first.txt")
	secondPath := filepath.Join(root, "second.txt")
	thirdPath := filepath.Join(root, "third.txt")

	var firstCallbackCount int32
	var duplicateCallbackCount int32
	var thirdCallbackCount int32
	eventsWithHooks := []eventpipeline.EventWithHooks{
		newEventWithHooksForStageExecutionTest(
			firstPath,
			&wavewatch.SortedHooks{
				Pre: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&firstCallbackCount, 1)
							return &wavewatch.RefreshAction{
								ReloadBrowser: true,
							}, nil
						},
					},
				},
			},
			false,
		),
		newEventWithHooksForStageExecutionTest(
			secondPath,
			&wavewatch.SortedHooks{
				Pre: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&duplicateCallbackCount, 1)
							return &wavewatch.RefreshAction{
								WaitForApp: true,
							}, nil
						},
					},
				},
			},
			true,
		),
		newEventWithHooksForStageExecutionTest(
			thirdPath,
			&wavewatch.SortedHooks{
				Pre: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&thirdCallbackCount, 1)
							return &wavewatch.RefreshAction{
								WaitForVite: true,
							}, nil
						},
					},
				},
			},
			false,
		),
	}

	actions, executionErrors := harness.engine.RunPreHooksForEventsWithErrors(
		eventsWithHooks,
		&eventpipeline.WorkSet{},
		harness.watcher,
	)
	if len(executionErrors) != 0 {
		t.Fatalf("expected no pre-hook errors, got %#v", executionErrors)
	}
	if len(actions) != 2 {
		t.Fatalf("pre action count=%d, want 2", len(actions))
	}
	if !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected first pre action from first event, got %#v",
			actions[0],
		)
	}
	if !actions[1].WaitForVite {
		t.Fatalf(
			"expected second pre action from third event, got %#v",
			actions[1],
		)
	}
	if atomic.LoadInt32(&firstCallbackCount) != 1 {
		t.Fatalf(
			"expected first pre callback count=1, got %d",
			atomic.LoadInt32(&firstCallbackCount),
		)
	}
	if atomic.LoadInt32(&duplicateCallbackCount) != 0 {
		t.Fatalf(
			"expected duplicate pre callback count=0, got %d",
			atomic.LoadInt32(&duplicateCallbackCount),
		)
	}
	if atomic.LoadInt32(&thirdCallbackCount) != 1 {
		t.Fatalf(
			"expected third pre callback count=1, got %d",
			atomic.LoadInt32(&thirdCallbackCount),
		)
	}
}

func TestRunConcurrentHooksForEvents_SkipsDuplicateHooksAndKeepsActionOrder(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	firstPath := filepath.Join(root, "first.txt")
	secondPath := filepath.Join(root, "second.txt")
	thirdPath := filepath.Join(root, "third.txt")

	var firstCallbackCount int32
	var duplicateCallbackCount int32
	var thirdCallbackCount int32
	eventsWithHooks := []eventpipeline.EventWithHooks{
		newEventWithHooksForStageExecutionTest(
			firstPath,
			&wavewatch.SortedHooks{
				Concurrent: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&firstCallbackCount, 1)
							time.Sleep(20 * time.Millisecond)
							return &wavewatch.RefreshAction{
								ReloadBrowser: true,
							}, nil
						},
					},
				},
			},
			false,
		),
		newEventWithHooksForStageExecutionTest(
			secondPath,
			&wavewatch.SortedHooks{
				Concurrent: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&duplicateCallbackCount, 1)
							return &wavewatch.RefreshAction{
								WaitForApp: true,
							}, nil
						},
					},
				},
			},
			true,
		),
		newEventWithHooksForStageExecutionTest(
			thirdPath,
			&wavewatch.SortedHooks{
				Concurrent: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&thirdCallbackCount, 1)
							return &wavewatch.RefreshAction{
								WaitForVite: true,
							}, nil
						},
					},
				},
			},
			false,
		),
	}

	actions, executionErrors := harness.engine.RunConcurrentHooksForEventsWithContextAndErrors(
		context.Background(),
		eventsWithHooks,
		harness.watcher,
	)
	if len(executionErrors) != 0 {
		t.Fatalf("expected no concurrent-hook errors, got %#v", executionErrors)
	}
	if len(actions) != 2 {
		t.Fatalf("concurrent action count=%d, want 2", len(actions))
	}
	if !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected first concurrent action from first event, got %#v",
			actions[0],
		)
	}
	if !actions[1].WaitForVite {
		t.Fatalf(
			"expected second concurrent action from third event, got %#v",
			actions[1],
		)
	}
	if atomic.LoadInt32(&firstCallbackCount) != 1 {
		t.Fatalf(
			"expected first concurrent callback count=1, got %d",
			atomic.LoadInt32(&firstCallbackCount),
		)
	}
	if atomic.LoadInt32(&duplicateCallbackCount) != 0 {
		t.Fatalf(
			"expected duplicate concurrent callback count=0, got %d",
			atomic.LoadInt32(&duplicateCallbackCount),
		)
	}
	if atomic.LoadInt32(&thirdCallbackCount) != 1 {
		t.Fatalf(
			"expected third concurrent callback count=1, got %d",
			atomic.LoadInt32(&thirdCallbackCount),
		)
	}
}

func TestRunPostHooksForEvents_SkipsDuplicateHooksAndKeepsActionOrder(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	firstPath := filepath.Join(root, "first.txt")
	secondPath := filepath.Join(root, "second.txt")
	thirdPath := filepath.Join(root, "third.txt")

	var firstCallbackCount int32
	var duplicateCallbackCount int32
	var thirdCallbackCount int32
	eventsWithHooks := []eventpipeline.EventWithHooks{
		newEventWithHooksForStageExecutionTest(
			firstPath,
			&wavewatch.SortedHooks{
				Post: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&firstCallbackCount, 1)
							return &wavewatch.RefreshAction{
								ReloadBrowser: true,
							}, nil
						},
					},
				},
			},
			false,
		),
		newEventWithHooksForStageExecutionTest(
			secondPath,
			&wavewatch.SortedHooks{
				Post: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&duplicateCallbackCount, 1)
							return &wavewatch.RefreshAction{
								WaitForApp: true,
							}, nil
						},
					},
				},
			},
			true,
		),
		newEventWithHooksForStageExecutionTest(
			thirdPath,
			&wavewatch.SortedHooks{
				Post: []wavewatch.OnChangeHook{
					{
						Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
							atomic.AddInt32(&thirdCallbackCount, 1)
							return &wavewatch.RefreshAction{
								WaitForVite: true,
							}, nil
						},
					},
				},
			},
			false,
		),
	}

	actions, executionErrors := harness.engine.RunPostHooksForEventsWithErrors(
		eventsWithHooks,
		harness.watcher,
	)
	if len(executionErrors) != 0 {
		t.Fatalf("expected no post-hook errors, got %#v", executionErrors)
	}
	if len(actions) != 2 {
		t.Fatalf("post action count=%d, want 2", len(actions))
	}
	if !actions[0].ReloadBrowser {
		t.Fatalf(
			"expected first post action from first event, got %#v",
			actions[0],
		)
	}
	if !actions[1].WaitForVite {
		t.Fatalf(
			"expected second post action from third event, got %#v",
			actions[1],
		)
	}
	if atomic.LoadInt32(&firstCallbackCount) != 1 {
		t.Fatalf(
			"expected first post callback count=1, got %d",
			atomic.LoadInt32(&firstCallbackCount),
		)
	}
	if atomic.LoadInt32(&duplicateCallbackCount) != 0 {
		t.Fatalf(
			"expected duplicate post callback count=0, got %d",
			atomic.LoadInt32(&duplicateCallbackCount),
		)
	}
	if atomic.LoadInt32(&thirdCallbackCount) != 1 {
		t.Fatalf(
			"expected third post callback count=1, got %d",
			atomic.LoadInt32(&thirdCallbackCount),
		)
	}
}

func newEventWithHooksForStageExecutionTest(
	filePath string,
	sortedHooks *wavewatch.SortedHooks,
	skipDuplicateHooks bool,
) eventpipeline.EventWithHooks {
	return eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event:    waveEvent(filePath),
			FileType: eventpipeline.FileTypeOther,
		},
		HookCtx: &wavewatch.HookContext{
			FilePath: filePath,
		},
		Hooks:              sortedHooks,
		SkipDuplicateHooks: skipDuplicateHooks,
	}
}

func TestProcessSingleEvent_PrehookRestartShortCircuitsPipeline(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event:       waveEvent(filepath.Join(t.TempDir(), "file.txt")),
			FileType:    eventpipeline.FileTypeOther,
			WatchedFile: &wavewatch.WatchedFile{},
		},
		HookCtx: &wavewatch.HookContext{},
		Hooks: &wavewatch.SortedHooks{
			Pre: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						return &wavewatch.RefreshAction{
							TriggerRestart: true,
						}, nil
					},
				},
			},
		},
		RunOnChangeOnly: false,
		NeedsHardReload: false,
	}

	runEventsWithDerivedExecutionPlan(
		harness.engine,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
		harness.watcher,
	)

	pendingRestartRequest := waitForPendingRestartRequestForRunloopTests(
		t,
		harness.restartAccumulator,
		200*time.Millisecond,
	)
	if pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected no-go restart from prehook action, got %#v",
			pendingRestartRequest,
		)
	}
}

func TestProcessSingleEvent_ConcurrentRestartCanRequestGoRecompile(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	work := &eventpipeline.WorkSet{}
	eventWithHooks := eventpipeline.EventWithHooks{
		Classified: eventpipeline.ClassifiedEvent{
			Event:    waveEvent(filepath.Join(t.TempDir(), "file.txt")),
			FileType: eventpipeline.FileTypeOther,
		},
		HookCtx: &wavewatch.HookContext{},
		Hooks: &wavewatch.SortedHooks{
			Concurrent: []wavewatch.OnChangeHook{
				{
					Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
						return &wavewatch.RefreshAction{
							TriggerRestart: true,
							RecompileGo:    true,
						}, nil
					},
				},
			},
		},
		RunOnChangeOnly: false,
		NeedsHardReload: false,
	}

	runEventsWithDerivedExecutionPlan(
		harness.engine,
		[]eventpipeline.EventWithHooks{eventWithHooks},
		work,
		harness.watcher,
	)

	pendingRestartRequest := waitForPendingRestartRequestForRunloopTests(
		t,
		harness.restartAccumulator,
		200*time.Millisecond,
	)
	if !pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected Go recompilation restart, got %#v",
			pendingRestartRequest,
		)
	}
}

func TestExecuteBuildPhase_ProcessesStaticFilesAndWritesCanonicalPublicFileMapOutputs(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)

	publicFile := filepath.Join(
		cfg.Core().StaticAssetDirsPublic(),
		waveartifacts.AssetsDirname,
		"logo.png",
	)
	privateFile := filepath.Join(
		cfg.Core().StaticAssetDirsPrivate(),
		"templates",
		"home.html",
	)

	if err := os.MkdirAll(filepath.Dir(publicFile), 0o755); err != nil {
		t.Fatalf("failed creating public file parent dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(privateFile), 0o755); err != nil {
		t.Fatalf("failed creating private file parent dir: %v", err)
	}
	if err := os.WriteFile(publicFile, []byte("logo"), 0o644); err != nil {
		t.Fatalf("failed writing public file: %v", err)
	}
	if err := os.WriteFile(privateFile, []byte("<h1>home</h1>"), 0o644); err != nil {
		t.Fatalf("failed writing private file: %v", err)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)
	serverForTest.Builder = builderForTest

	work := &eventpipeline.WorkSet{
		Build: eventpipeline.BuildPhaseDecision{
			ProcessPublicFiles:  true,
			ProcessPrivateFiles: true,
			BuildCriticalCSS:    true,
			BuildNormalCSS:      true,
		},
	}
	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(
		work,
	); executeBuildPhaseError != nil {
		t.Fatalf(
			"ExecuteBuildPhase returned error: %v",
			executeBuildPhaseError,
		)
	}

	requiredOutputs := []string{
		cfg.Dist().PublicFileMapGob(),
		cfg.Dist().PrivateFileMapGob(),
		cfg.Dist().PublicFileMapRef(),
	}
	for _, output := range requiredOutputs {
		if _, statError := os.Stat(output); statError != nil {
			t.Fatalf(
				"expected build phase output to exist: %s (error: %v)",
				output,
				statError,
			)
		}
	}

	refBytes, readRefError := os.ReadFile(cfg.Dist().PublicFileMapRef())
	if readRefError != nil {
		t.Fatalf("failed reading public filemap ref: %v", readRefError)
	}
	referencedPublicFileMapPath := filepath.Join(
		cfg.Dist().StaticPublic(),
		filepath.FromSlash(strings.TrimSpace(string(refBytes))),
	)
	if _, statError := os.Stat(referencedPublicFileMapPath); statError != nil {
		t.Fatalf(
			"expected referenced canonical public filemap output to exist: %s (error: %v)",
			referencedPublicFileMapPath,
			statError,
		)
	}
}

func TestExecuteBuildPhase_UsesChangedPathStaticProcessingWhenPathsProvided(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)

	publicTrackedFilePath := filepath.Join(
		cfg.Core().StaticAssetDirsPublic(),
		waveartifacts.AssetsDirname,
		"tracked-logo.png",
	)
	privateTrackedFilePath := filepath.Join(
		cfg.Core().StaticAssetDirsPrivate(),
		"templates",
		"tracked-home.html",
	)
	publicUnrelatedFilePath := filepath.Join(
		cfg.Core().StaticAssetDirsPublic(),
		waveartifacts.AssetsDirname,
		"unrelated-logo.png",
	)
	privateUnrelatedFilePath := filepath.Join(
		cfg.Core().StaticAssetDirsPrivate(),
		"templates",
		"unrelated-home.html",
	)

	for _, directoryPath := range []string{
		filepath.Dir(publicTrackedFilePath),
		filepath.Dir(privateTrackedFilePath),
	} {
		if mkdirError := os.MkdirAll(directoryPath, 0o755); mkdirError != nil {
			t.Fatalf(
				"create static parent directory %q: %v",
				directoryPath,
				mkdirError,
			)
		}
	}

	if writeError := os.WriteFile(publicTrackedFilePath, []byte("tracked"), 0o644); writeError != nil {
		t.Fatalf("write tracked public static file: %v", writeError)
	}
	if writeError := os.WriteFile(privateTrackedFilePath, []byte("<h1>tracked</h1>"), 0o644); writeError != nil {
		t.Fatalf("write tracked private static file: %v", writeError)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()

	// Seed file maps so changed-path processing can mutate incrementally.
	if processError := builderForTest.ProcessPublicFilesOnly(); processError != nil {
		t.Fatalf("seed public file map: %v", processError)
	}
	if processError := builderForTest.ProcessPrivateFilesOnly(); processError != nil {
		t.Fatalf("seed private file map: %v", processError)
	}

	if writeError := os.WriteFile(publicUnrelatedFilePath, []byte("unrelated"), 0o644); writeError != nil {
		t.Fatalf("write unrelated public static file: %v", writeError)
	}
	if writeError := os.WriteFile(privateUnrelatedFilePath, []byte("<h1>unrelated</h1>"), 0o644); writeError != nil {
		t.Fatalf("write unrelated private static file: %v", writeError)
	}

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)
	serverForTest.Builder = builderForTest

	work := &eventpipeline.WorkSet{
		Build: eventpipeline.BuildPhaseDecision{
			ProcessPublicFiles:            true,
			ProcessPrivateFiles:           true,
			PublicStaticChangedFilePaths:  []string{publicTrackedFilePath},
			PrivateStaticChangedFilePaths: []string{privateTrackedFilePath},
		},
	}
	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(work); executeBuildPhaseError != nil {
		t.Fatalf(
			"ExecuteBuildPhase with changed-path static processing returned error: %v",
			executeBuildPhaseError,
		)
	}

	publicMap, loadPublicMapError := builderForTest.LoadPublicFileMap()
	if loadPublicMapError != nil {
		t.Fatalf(
			"load public file map after changed-path processing: %v",
			loadPublicMapError,
		)
	}
	privateMap := loadStaticFileMapFromGobPathForRunloopProcessTests(
		t,
		cfg.Dist().PrivateFileMapGob(),
	)

	publicTrackedMapKey, publicTrackedRelError := filepath.Rel(
		cfg.Core().StaticAssetDirsPublic(),
		publicTrackedFilePath,
	)
	if publicTrackedRelError != nil {
		t.Fatalf(
			"derive tracked public static map key: %v",
			publicTrackedRelError,
		)
	}
	publicUnrelatedMapKey, publicUnrelatedRelError := filepath.Rel(
		cfg.Core().StaticAssetDirsPublic(),
		publicUnrelatedFilePath,
	)
	if publicUnrelatedRelError != nil {
		t.Fatalf(
			"derive unrelated public static map key: %v",
			publicUnrelatedRelError,
		)
	}
	privateTrackedMapKey, privateTrackedRelError := filepath.Rel(
		cfg.Core().StaticAssetDirsPrivate(),
		privateTrackedFilePath,
	)
	if privateTrackedRelError != nil {
		t.Fatalf(
			"derive tracked private static map key: %v",
			privateTrackedRelError,
		)
	}
	privateUnrelatedMapKey, privateUnrelatedRelError := filepath.Rel(
		cfg.Core().StaticAssetDirsPrivate(),
		privateUnrelatedFilePath,
	)
	if privateUnrelatedRelError != nil {
		t.Fatalf(
			"derive unrelated private static map key: %v",
			privateUnrelatedRelError,
		)
	}

	publicTrackedMapKey = filepath.ToSlash(publicTrackedMapKey)
	publicUnrelatedMapKey = filepath.ToSlash(publicUnrelatedMapKey)
	privateTrackedMapKey = filepath.ToSlash(privateTrackedMapKey)
	privateUnrelatedMapKey = filepath.ToSlash(privateUnrelatedMapKey)

	if _, exists := publicMap[publicTrackedMapKey]; !exists {
		t.Fatalf(
			"expected tracked public static file map key %q after changed-path processing, got %#v",
			publicTrackedMapKey,
			publicMap,
		)
	}
	if _, exists := privateMap[privateTrackedMapKey]; !exists {
		t.Fatalf(
			"expected tracked private static file map key %q after changed-path processing, got %#v",
			privateTrackedMapKey,
			privateMap,
		)
	}
	if _, exists := publicMap[publicUnrelatedMapKey]; exists {
		t.Fatalf(
			"did not expect unrelated public static file map key %q in changed-path processing result, got %#v",
			publicUnrelatedMapKey,
			publicMap,
		)
	}
	if _, exists := privateMap[privateUnrelatedMapKey]; exists {
		t.Fatalf(
			"did not expect unrelated private static file map key %q in changed-path processing result, got %#v",
			privateUnrelatedMapKey,
			privateMap,
		)
	}
}

func TestExecuteBuildPhase_NoPublicFileMapArtifactChangeClearsInvalidateViteAction(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)

	publicDir := cfg.Core().StaticAssetDirsPublic()
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "logo.png"), []byte("logo"), 0o644); err != nil {
		t.Fatalf("failed writing public source file: %v", err)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()
	if processError := builderForTest.ProcessPublicFilesOnly(); processError != nil {
		t.Fatalf("seed public file map: %v", processError)
	}

	ignoredPath := filepath.Join(publicDir, ".DS_Store")
	if err := os.WriteFile(ignoredPath, []byte("ignored"), 0o644); err != nil {
		t.Fatalf("failed writing ignored file: %v", err)
	}

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)
	serverForTest.Builder = builderForTest

	work := &eventpipeline.WorkSet{
		Build: eventpipeline.BuildPhaseDecision{
			ProcessPublicFiles:           true,
			PublicStaticChangedFilePaths: []string{ignoredPath},
		},
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionInvalidateVite,
		},
	}
	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(work); executeBuildPhaseError != nil {
		t.Fatalf("ExecuteBuildPhase returned error: %v", executeBuildPhaseError)
	}

	if work.Browser.Action != eventpipeline.BrowserPhaseActionNone {
		t.Fatalf(
			"expected browser action to downgrade to none when public filemap artifacts are unchanged, got %v",
			work.Browser.Action,
		)
	}
}

func TestExecuteBuildPhase_PublicFileMapArtifactChangeCallsFrameworkRuntimeReloadEndpoint(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)

	publicDir := cfg.Core().StaticAssetDirsPublic()
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}
	changedPublicFilePath := filepath.Join(publicDir, "logo.png")
	if err := os.WriteFile(changedPublicFilePath, []byte("logo-before"), 0o644); err != nil {
		t.Fatalf("failed writing public source file: %v", err)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()
	if processError := builderForTest.ProcessPublicFilesOnly(); processError != nil {
		t.Fatalf("seed public file map: %v", processError)
	}

	var frameworkReloadCallCount int32
	waveframework.StateForConfig(cfg).PublicFileMapReloadEndpointPath = "/__framework/reload-public-filemap"

	if err := os.WriteFile(changedPublicFilePath, []byte("logo-after"), 0o644); err != nil {
		t.Fatalf("failed updating public source file: %v", err)
	}

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)
	serverForTest.Builder = builderForTest
	serverForTest.CallFrameworkRuntimeReloadEndpointWithContextFunc = func(
		commandExecutionContext context.Context,
		reloadRequest wavewatch.FrameworkRuntimeReloadRequest,
	) error {
		if commandExecutionContext == nil {
			return errors.New("framework runtime reload execution context is nil")
		}
		if got, want := reloadRequest.EndpointPath, "/__framework/reload-public-filemap"; got != want {
			return fmt.Errorf("reload endpoint path = %q, want %q", got, want)
		}
		if got, want := reloadRequest.ReloadTrigger, "public-filemap-artifacts-changed"; got != want {
			return fmt.Errorf("reload trigger = %q, want %q", got, want)
		}
		atomic.AddInt32(&frameworkReloadCallCount, 1)
		return nil
	}

	work := &eventpipeline.WorkSet{
		Build: eventpipeline.BuildPhaseDecision{
			ProcessPublicFiles:           true,
			PublicStaticChangedFilePaths: []string{changedPublicFilePath},
		},
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionInvalidateVite,
		},
	}
	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(work); executeBuildPhaseError != nil {
		t.Fatalf("ExecuteBuildPhase returned error: %v", executeBuildPhaseError)
	}

	if got := atomic.LoadInt32(&frameworkReloadCallCount); got != 1 {
		t.Fatalf("framework runtime reload call count = %d, want 1", got)
	}
	if work.Browser.Action != eventpipeline.BrowserPhaseActionInvalidateVite {
		t.Fatalf(
			"expected browser action to keep invalidate-vite when public filemap artifacts change, got %v",
			work.Browser.Action,
		)
	}
}

func TestExecuteBuildPhase_PublicFileMapArtifactRepairKeepsInvalidateViteAction(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)

	publicDir := cfg.Core().StaticAssetDirsPublic()
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("failed creating public dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "logo.png"), []byte("logo"), 0o644); err != nil {
		t.Fatalf("failed writing public source file: %v", err)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()
	if processError := builderForTest.ProcessPublicFilesOnly(); processError != nil {
		t.Fatalf("seed public file map: %v", processError)
	}

	refBytes, readRefError := os.ReadFile(cfg.Dist().PublicFileMapRef())
	if readRefError != nil {
		t.Fatalf("read public filemap ref before repair test: %v", readRefError)
	}
	refTarget := strings.TrimSpace(string(refBytes))
	if refTarget == "" {
		t.Fatal("expected non-empty public filemap ref before repair test")
	}
	missingCanonicalPath := filepath.Join(cfg.Dist().StaticPublic(), refTarget)
	if removeError := os.Remove(missingCanonicalPath); removeError != nil {
		t.Fatalf(
			"remove canonical public filemap artifact before repair test: %v",
			removeError,
		)
	}

	ignoredPath := filepath.Join(publicDir, ".DS_Store")
	if err := os.WriteFile(ignoredPath, []byte("ignored"), 0o644); err != nil {
		t.Fatalf("failed writing ignored file: %v", err)
	}

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)
	serverForTest.Builder = builderForTest

	work := &eventpipeline.WorkSet{
		Build: eventpipeline.BuildPhaseDecision{
			ProcessPublicFiles:           true,
			PublicStaticChangedFilePaths: []string{ignoredPath},
		},
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionInvalidateVite,
		},
	}
	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(work); executeBuildPhaseError != nil {
		t.Fatalf("ExecuteBuildPhase returned error: %v", executeBuildPhaseError)
	}

	if work.Browser.Action != eventpipeline.BrowserPhaseActionInvalidateVite {
		t.Fatalf(
			"expected browser action to keep invalidate-vite when public filemap artifacts are repaired, got %v",
			work.Browser.Action,
		)
	}

	refBytesAfterRepair, readRefAfterRepairError := os.ReadFile(
		cfg.Dist().PublicFileMapRef(),
	)
	if readRefAfterRepairError != nil {
		t.Fatalf(
			"read public filemap ref after repair test: %v",
			readRefAfterRepairError,
		)
	}
	refTargetAfterRepair := strings.TrimSpace(string(refBytesAfterRepair))
	if refTargetAfterRepair == "" {
		t.Fatal("expected non-empty public filemap ref after repair test")
	}
	canonicalPathAfterRepair := filepath.Join(
		cfg.Dist().StaticPublic(),
		refTargetAfterRepair,
	)
	if _, statError := os.Stat(canonicalPathAfterRepair); statError != nil {
		t.Fatalf(
			"expected canonical public filemap artifact after repair path, stat error: %v",
			statError,
		)
	}
}

func TestExecuteBuildPhase_UsesFullScanStaticProcessingWhenChangedPathsAbsent(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)

	publicTrackedFilePath := filepath.Join(
		cfg.Core().StaticAssetDirsPublic(),
		waveartifacts.AssetsDirname,
		"tracked-logo.png",
	)
	privateTrackedFilePath := filepath.Join(
		cfg.Core().StaticAssetDirsPrivate(),
		"templates",
		"tracked-home.html",
	)
	publicNewFilePath := filepath.Join(
		cfg.Core().StaticAssetDirsPublic(),
		waveartifacts.AssetsDirname,
		"new-logo.png",
	)
	privateNewFilePath := filepath.Join(
		cfg.Core().StaticAssetDirsPrivate(),
		"templates",
		"new-home.html",
	)

	for _, directoryPath := range []string{
		filepath.Dir(publicTrackedFilePath),
		filepath.Dir(privateTrackedFilePath),
	} {
		if mkdirError := os.MkdirAll(directoryPath, 0o755); mkdirError != nil {
			t.Fatalf(
				"create static parent directory %q: %v",
				directoryPath,
				mkdirError,
			)
		}
	}

	if writeError := os.WriteFile(publicTrackedFilePath, []byte("tracked"), 0o644); writeError != nil {
		t.Fatalf("write tracked public static file: %v", writeError)
	}
	if writeError := os.WriteFile(privateTrackedFilePath, []byte("<h1>tracked</h1>"), 0o644); writeError != nil {
		t.Fatalf("write tracked private static file: %v", writeError)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()

	if processError := builderForTest.ProcessPublicFilesOnly(); processError != nil {
		t.Fatalf("seed public file map: %v", processError)
	}
	if processError := builderForTest.ProcessPrivateFilesOnly(); processError != nil {
		t.Fatalf("seed private file map: %v", processError)
	}

	if writeError := os.WriteFile(publicNewFilePath, []byte("new"), 0o644); writeError != nil {
		t.Fatalf("write new public static file: %v", writeError)
	}
	if writeError := os.WriteFile(privateNewFilePath, []byte("<h1>new</h1>"), 0o644); writeError != nil {
		t.Fatalf("write new private static file: %v", writeError)
	}

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)
	serverForTest.Builder = builderForTest

	work := &eventpipeline.WorkSet{
		Build: eventpipeline.BuildPhaseDecision{
			ProcessPublicFiles:  true,
			ProcessPrivateFiles: true,
		},
	}
	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(work); executeBuildPhaseError != nil {
		t.Fatalf(
			"ExecuteBuildPhase with full-scan static processing returned error: %v",
			executeBuildPhaseError,
		)
	}

	publicMap, loadPublicMapError := builderForTest.LoadPublicFileMap()
	if loadPublicMapError != nil {
		t.Fatalf("load public file map after full scan: %v", loadPublicMapError)
	}
	privateMap := loadStaticFileMapFromGobPathForRunloopProcessTests(
		t,
		cfg.Dist().PrivateFileMapGob(),
	)

	publicNewMapKey, publicNewRelError := filepath.Rel(
		cfg.Core().StaticAssetDirsPublic(),
		publicNewFilePath,
	)
	if publicNewRelError != nil {
		t.Fatalf("derive new public static map key: %v", publicNewRelError)
	}
	privateNewMapKey, privateNewRelError := filepath.Rel(
		cfg.Core().StaticAssetDirsPrivate(),
		privateNewFilePath,
	)
	if privateNewRelError != nil {
		t.Fatalf("derive new private static map key: %v", privateNewRelError)
	}

	publicNewMapKey = filepath.ToSlash(publicNewMapKey)
	privateNewMapKey = filepath.ToSlash(privateNewMapKey)

	if _, exists := publicMap[publicNewMapKey]; !exists {
		t.Fatalf(
			"expected full-scan public static processing to include new key %q, got %#v",
			publicNewMapKey,
			publicMap,
		)
	}
	if _, exists := privateMap[privateNewMapKey]; !exists {
		t.Fatalf(
			"expected full-scan private static processing to include new key %q, got %#v",
			privateNewMapKey,
			privateMap,
		)
	}
}

func TestExecuteBuildPhase_CompileGoErrorIsReturned(t *testing.T) {
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, t.TempDir())
	wavetest.SetCoreServerOnlyMode(cfg, true)
	wavetest.SetCoreMainAppEntry(cfg, "missing/package/for/compile")

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)
	serverForTest.Builder = builderForTest

	work := &eventpipeline.WorkSet{
		Build: eventpipeline.BuildPhaseDecision{
			CompileGo: true,
		},
	}
	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(
		work,
	); executeBuildPhaseError == nil {
		t.Fatal("expected compile-go build phase error")
	}
}

func TestExecuteBuildPhase_WithNilBuilderReturnsError(t *testing.T) {
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, t.TempDir())
	wavetest.SetCoreServerOnlyMode(cfg, true)

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)

	work := &eventpipeline.WorkSet{
		Build: eventpipeline.BuildPhaseDecision{
			CompileGo:           true,
			ProcessPublicFiles:  true,
			ProcessPrivateFiles: true,
			BuildCriticalCSS:    true,
			BuildNormalCSS:      true,
		},
	}
	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(
		work,
	); executeBuildPhaseError == nil {
		t.Fatal("expected nil-builder build phase error")
	}
}

func TestExecuteHookExecutionPlan_CallbackPanicReturnsErrorAndSkipsCommand(
	t *testing.T,
) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	commandOutputPath := filepath.Join(t.TempDir(), "hook-command-output.log")
	hookPlan := hooks.HookExecutionPlan{
		Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
			panic("expected panic from hook callback")
		},
		Command: "printf 'command should not run\\n' >> " + strconv.Quote(
			commandOutputPath,
		),
	}

	action, executeHookExecutionPlanError := harness.engine.ExecuteHookExecutionPlanWithContext(
		context.Background(),
		hooks.HookStageTypePre,
		hookPlan,
		&wavewatch.HookContext{},
	)
	if executeHookExecutionPlanError == nil {
		t.Fatal("expected callback panic to be surfaced as an error")
	}
	if action != nil {
		t.Fatalf("expected nil action when callback panics, got %#v", action)
	}

	if _, statError := os.Stat(commandOutputPath); !os.IsNotExist(statError) {
		t.Fatalf(
			"expected command to be skipped after callback panic, stat error: %v",
			statError,
		)
	}
}

func TestExecuteBuildPhase_WithNilBuilderAndNoBuildWorkReturnsNil(
	t *testing.T,
) {
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, t.TempDir())
	wavetest.SetCoreServerOnlyMode(cfg, true)

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)

	work := &eventpipeline.WorkSet{}
	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(
		work,
	); executeBuildPhaseError != nil {
		t.Fatalf(
			"expected no-op build phase to return nil, got %v",
			executeBuildPhaseError,
		)
	}
}

func TestExecuteBuildPhase_WithNilWorkSetReturnsNil(t *testing.T) {
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, t.TempDir())
	wavetest.SetCoreServerOnlyMode(cfg, true)

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)

	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(
		nil,
	); executeBuildPhaseError != nil {
		t.Fatalf(
			"expected nil workset to return nil, got %v",
			executeBuildPhaseError,
		)
	}
}

func TestExecuteBuildPhase_SetupDistDirErrorIsReturned(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForRunloopBatchedWatcherTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, false)

	if err := os.WriteFile(
		cfg.Dist().Internal(),
		[]byte("not-a-directory"),
		0o644,
	); err != nil {
		if mkdirError := os.MkdirAll(filepath.Dir(cfg.Dist().Internal()), 0o755); mkdirError != nil {
			t.Fatalf("failed creating dist-internal parent dir: %v", mkdirError)
		}
		if retryWriteError := os.WriteFile(
			cfg.Dist().Internal(),
			[]byte("not-a-directory"),
			0o644,
		); retryWriteError != nil {
			t.Fatalf(
				"failed writing dist-internal blocker file: %v",
				retryWriteError,
			)
		}
	} else {
		// no-op: blocker file created on first attempt.
	}

	publicFile := filepath.Join(
		cfg.Core().StaticAssetDirsPublic(),
		waveartifacts.AssetsDirname,
		"logo.png",
	)
	if err := os.MkdirAll(filepath.Dir(publicFile), 0o755); err != nil {
		t.Fatalf("failed creating public static dir: %v", err)
	}
	if err := os.WriteFile(publicFile, []byte("logo"), 0o644); err != nil {
		t.Fatalf("failed writing public static file: %v", err)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
	)
	defer builderForTest.Close()

	serverForTest := newRunloopTestServer(
		cfg,
		newDiscardLoggerForRunloopBatchedWatcherTests(),
		"",
	)
	serverForTest.Builder = builderForTest

	work := &eventpipeline.WorkSet{
		Build: eventpipeline.BuildPhaseDecision{
			ProcessPublicFiles: true,
		},
	}
	if executeBuildPhaseError := serverForTest.ExecuteBuildPhase(
		work,
	); executeBuildPhaseError == nil {
		t.Fatal("expected setup-dist error from build phase")
	}

	statInfo, statError := os.Stat(cfg.Dist().Internal())
	if statError != nil {
		t.Fatalf("failed stating dist-internal blocker: %v", statError)
	}
	if statInfo.IsDir() {
		t.Fatal("expected dist-internal blocker to remain a file")
	}
}

func TestResolveHookForStageExecution(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	classifiedEvent := eventpipeline.ClassifiedEvent{
		Event: waveEvent(changedPath),
	}
	callbackHook := wavewatch.OnChangeHook{
		Cmd: "echo should-not-run",
		Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
			return nil, nil
		},
	}

	excludedHook, excludedHookShouldRun := hooks.ResolveHookForStageExecutionWithRunOnChangeOnlyPolicy(
		harness.watcher,
		classifiedEvent,
		false,
		false,
		wavewatch.OnChangeHook{Exclude: []string{changedPath}},
	)
	if excludedHookShouldRun {
		t.Fatalf(
			"expected excluded hook to be skipped, got hook %#v",
			excludedHook,
		)
	}

	stageHookWithoutRunOnChangeRules, stageHookWithoutRunOnChangeRulesShouldRun := hooks.ResolveHookForStageExecutionWithRunOnChangeOnlyPolicy(
		harness.watcher,
		classifiedEvent,
		true,
		false,
		callbackHook,
	)
	if !stageHookWithoutRunOnChangeRulesShouldRun {
		t.Fatal(
			"expected hook to run when stage does not apply run-on-change-only rules",
		)
	}
	if stageHookWithoutRunOnChangeRules.Cmd == "" {
		t.Fatalf(
			"expected command to remain for stage without run-on-change-only rules, got %#v",
			stageHookWithoutRunOnChangeRules,
		)
	}

	commandOnlyHook, commandOnlyHookShouldRun := hooks.ResolveHookForStageExecutionWithRunOnChangeOnlyPolicy(
		harness.watcher,
		classifiedEvent,
		true,
		true,
		wavewatch.OnChangeHook{Cmd: "echo skip-command-only"},
	)
	if commandOnlyHookShouldRun {
		t.Fatalf(
			"expected command-only hook to be skipped for run-on-change-only stage, got %#v",
			commandOnlyHook,
		)
	}

	callbackAndCommandHook, callbackAndCommandHookShouldRun := hooks.ResolveHookForStageExecutionWithRunOnChangeOnlyPolicy(
		harness.watcher,
		classifiedEvent,
		true,
		true,
		callbackHook,
	)
	if !callbackAndCommandHookShouldRun {
		t.Fatal(
			"expected callback hook to be retained for run-on-change-only stage",
		)
	}
	if callbackAndCommandHook.Cmd != "" ||
		callbackAndCommandHook.RunCombinedDevBuildHookCommands {
		t.Fatalf(
			"expected run-on-change-only rules to strip command execution fields, got %#v",
			callbackAndCommandHook,
		)
	}
	if callbackAndCommandHook.Callback == nil {
		t.Fatalf(
			"expected callback to remain after run-on-change-only filtering, got %#v",
			callbackAndCommandHook,
		)
	}

	normalStageHook, normalStageHookShouldRun := hooks.ResolveHookForStageExecutionWithRunOnChangeOnlyPolicy(
		harness.watcher,
		classifiedEvent,
		false,
		true,
		callbackHook,
	)
	if !normalStageHookShouldRun {
		t.Fatal("expected hook to run when run-on-change-only mode is disabled")
	}
	if normalStageHook.Cmd == "" {
		t.Fatalf(
			"expected command to remain when run-on-change-only mode is disabled, got %#v",
			normalStageHook,
		)
	}
}

func TestDeriveExecutableHooksForStage(t *testing.T) {
	harness := newEngineAndWatcherForHookExecutionTest(t)
	defer harness.watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	callbackHook := wavewatch.OnChangeHook{
		Cmd: "echo callback-and-command",
		Callback: func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
			return nil, nil
		},
	}
	commandOnlyHook := wavewatch.OnChangeHook{
		Cmd: "echo command-only",
	}
	excludedHook := wavewatch.OnChangeHook{
		Cmd:     "echo excluded",
		Exclude: []string{changedPath},
	}

	t.Run(
		"stage without run-on-change-only rules keeps command hooks",
		func(t *testing.T) {
			eventWithHooks := eventpipeline.EventWithHooks{
				Classified: eventpipeline.ClassifiedEvent{
					Event: waveEvent(changedPath),
				},
				RunOnChangeOnly: true,
				Hooks: &wavewatch.SortedHooks{
					Pre: []wavewatch.OnChangeHook{
						excludedHook,
						commandOnlyHook,
						callbackHook,
					},
				},
			}
			hooksForExecution := hooks.DeriveExecutableHooksForStage(
				harness.watcher,
				eventWithHooks,
				hooks.HookStageTypePre,
			)
			if len(hooksForExecution) != 2 {
				t.Fatalf(
					"expected 2 hooks for execution, got %#v",
					hooksForExecution,
				)
			}
			if hooksForExecution[0].Cmd == "" {
				t.Fatalf(
					"expected command-only hook command to be preserved, got %#v",
					hooksForExecution[0],
				)
			}
			if hooksForExecution[1].Cmd == "" {
				t.Fatalf(
					"expected callback hook command to be preserved, got %#v",
					hooksForExecution[1],
				)
			}
			if hooksForExecution[1].Callback == nil {
				t.Fatalf(
					"expected callback hook callback to be preserved, got %#v",
					hooksForExecution[1],
				)
			}
		},
	)

	t.Run(
		"run-on-change-only stage strips command fields and drops command-only hooks",
		func(t *testing.T) {
			eventWithHooks := eventpipeline.EventWithHooks{
				Classified: eventpipeline.ClassifiedEvent{
					Event: waveEvent(changedPath),
				},
				RunOnChangeOnly: true,
				Hooks: &wavewatch.SortedHooks{
					Post: []wavewatch.OnChangeHook{
						excludedHook,
						commandOnlyHook,
						callbackHook,
					},
				},
			}
			hooksForExecution := hooks.DeriveExecutableHooksForStage(
				harness.watcher,
				eventWithHooks,
				hooks.HookStageTypePost,
			)
			if len(hooksForExecution) != 1 {
				t.Fatalf(
					"expected 1 callback-only hook for execution, got %#v",
					hooksForExecution,
				)
			}
			if hooksForExecution[0].Cmd != "" ||
				hooksForExecution[0].RunCombinedDevBuildHookCommands {
				t.Fatalf(
					"expected run-on-change-only stage to strip command fields, got %#v",
					hooksForExecution[0],
				)
			}
			if hooksForExecution[0].Callback == nil {
				t.Fatalf(
					"expected callback hook callback to remain, got %#v",
					hooksForExecution[0],
				)
			}
		},
	)

	t.Run(
		"run-on-change-only disabled keeps command-only hooks even when stage applies rules",
		func(t *testing.T) {
			eventWithHooks := eventpipeline.EventWithHooks{
				Classified: eventpipeline.ClassifiedEvent{
					Event: waveEvent(changedPath),
				},
				RunOnChangeOnly: false,
				Hooks: &wavewatch.SortedHooks{
					Post: []wavewatch.OnChangeHook{
						excludedHook,
						commandOnlyHook,
						callbackHook,
					},
				},
			}
			hooksForExecution := hooks.DeriveExecutableHooksForStage(
				harness.watcher,
				eventWithHooks,
				hooks.HookStageTypePost,
			)
			if len(hooksForExecution) != 2 {
				t.Fatalf(
					"expected 2 hooks for execution, got %#v",
					hooksForExecution,
				)
			}
			if hooksForExecution[0].Cmd == "" {
				t.Fatalf(
					"expected command-only hook command to remain, got %#v",
					hooksForExecution[0],
				)
			}
			if hooksForExecution[1].Cmd == "" {
				t.Fatalf(
					"expected callback hook command to remain, got %#v",
					hooksForExecution[1],
				)
			}
		},
	)
}
