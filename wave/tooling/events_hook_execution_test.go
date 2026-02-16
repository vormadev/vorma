package tooling

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/wave"
)

func newServerAndWatcherForHookExecutionTest(t *testing.T) (*server, *watcher) {
	t.Helper()

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	watcher, err := newWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}

	s := &server{
		cfg:            cfg,
		log:            newDiscardLogger(),
		restartIntents: newRestartIntentAccumulator(make(chan restartRequest, 1)),
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

func TestRunConcurrentHooks_RunCombinedDevBuildHookCommands_UsesFrameworkBuildHookRunner(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.go")
	combinedExecutionLogPath := filepath.Join(root, "combined-execution.log")
	frameworkCommandFallbackLogPath := filepath.Join(root, "framework-command-fallback.log")
	if err := os.WriteFile(changedPath, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	s.cfg.Core.DevBuildHook = "printf 'user\\n' >> " + strconv.Quote(combinedExecutionLogPath)
	s.cfg.FrameworkDevBuildHook = "printf 'framework-command\\n' >> " + strconv.Quote(frameworkCommandFallbackLogPath)

	var frameworkRunnerCallCount atomic.Int32
	s.cfg.FrameworkRunBuildHook = func(
		hookExecutionContext context.Context,
		runInDevelopmentMode bool,
	) error {
		frameworkRunnerCallCount.Add(1)
		if !runInDevelopmentMode {
			return errors.New("expected framework build hook runner to execute in development mode")
		}
		if hookExecutionContext == nil {
			return errors.New("expected non-nil framework build hook execution context")
		}

		frameworkRunnerOutputFile, err := os.OpenFile(
			combinedExecutionLogPath,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0o644,
		)
		if err != nil {
			return err
		}
		defer frameworkRunnerOutputFile.Close()
		if _, err := frameworkRunnerOutputFile.WriteString("framework-runner\n"); err != nil {
			return err
		}
		return nil
	}

	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{
					RunCombinedDevBuildHookCommands: true,
				},
			},
		},
	}

	actions, err := s.runConcurrentHooks(ewh, watcher)
	if err != nil {
		t.Fatalf("runConcurrentHooks returned error: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("expected no concurrent actions from combined dev hooks, got %#v", actions)
	}

	if got := frameworkRunnerCallCount.Load(); got != 1 {
		t.Fatalf("framework build hook runner call count = %d, want 1", got)
	}

	combinedExecutionLog, err := os.ReadFile(combinedExecutionLogPath)
	if err != nil {
		t.Fatalf("failed reading combined execution log: %v", err)
	}
	if string(combinedExecutionLog) != "user\nframework-runner\n" {
		t.Fatalf("combined execution log = %q, want %q", string(combinedExecutionLog), "user\nframework-runner\n")
	}

	if _, err := os.Stat(frameworkCommandFallbackLogPath); !os.IsNotExist(err) {
		t.Fatalf(
			"expected framework command fallback log not to exist when framework runner is configured, stat err: %v",
			err,
		)
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

func TestRunConcurrentHooks_AggregatesMultipleHookErrors(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "aggregate-errors.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return nil, os.ErrPermission
					},
				},
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return nil, os.ErrNotExist
					},
				},
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						return &wave.RefreshAction{ReloadBrowser: true}, nil
					},
				},
			},
		},
	}

	actions, err := s.runConcurrentHooks(ewh, watcher)
	if err == nil {
		t.Fatal("expected aggregated concurrent hook error")
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf("expected successful concurrent actions to be preserved, got %#v", actions)
	}
	if !strings.Contains(err.Error(), os.ErrPermission.Error()) {
		t.Fatalf("expected aggregated error to include permission failure, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), os.ErrNotExist.Error()) {
		t.Fatalf("expected aggregated error to include not-exist failure, got %q", err.Error())
	}
}

func TestRunConcurrentHooksWithContext_CanceledContextSkipsHookExecution(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	var callbackCalled atomic.Bool
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
			},
		},
	}

	concurrentHookExecutionContext, cancelConcurrentHookExecutionContext := context.WithCancel(
		context.Background(),
	)
	cancelConcurrentHookExecutionContext()

	actions, err := s.runConcurrentHooksWithContext(
		concurrentHookExecutionContext,
		ewh,
		watcher,
	)
	if err != nil {
		t.Fatalf("runConcurrentHooksWithContext returned error: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("expected no actions when context is canceled, got %#v", actions)
	}
	if callbackCalled.Load() {
		t.Fatal("did not expect concurrent hook callback to run when context is canceled")
	}
}

func TestRunConcurrentHooks_CallbackReceivesIndependentHookContexts(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "context-clone.txt")
	receivedHookContexts := make(chan *wave.HookContext, 2)
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx: &wave.HookContext{
			FilePath:         changedPath,
			ChangedFilePaths: []string{changedPath},
		},
		hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{
					Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
						receivedHookContexts <- hookContext
						return nil, nil
					},
				},
				{
					Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
						receivedHookContexts <- hookContext
						return nil, nil
					},
				},
			},
		},
	}

	if _, err := s.runConcurrentHooks(ewh, watcher); err != nil {
		t.Fatalf("runConcurrentHooks returned error: %v", err)
	}

	firstHookContext := <-receivedHookContexts
	secondHookContext := <-receivedHookContexts
	if firstHookContext == secondHookContext {
		t.Fatal("expected concurrent callbacks to receive independent hook context instances")
	}
	if firstHookContext.ExecutionContext == nil || secondHookContext.ExecutionContext == nil {
		t.Fatal("expected callback hook contexts to include execution context")
	}
}

func TestRunConcurrentHooksWithContext_CallbackCanObserveCancellation(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "callback-cancellation.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{
					Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
						select {
						case <-hookContext.ExecutionContext.Done():
							return &wave.RefreshAction{ReloadBrowser: true}, nil
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
	actions, err := s.runConcurrentHooksWithContext(
		concurrentHookExecutionContext,
		ewh,
		watcher,
	)
	callbackElapsedTime := time.Since(callbackStartTime)
	if err != nil {
		t.Fatalf("runConcurrentHooksWithContext returned error: %v", err)
	}
	if callbackElapsedTime > 1*time.Second {
		t.Fatalf(
			"expected callback cancellation observation to return quickly, elapsed=%s",
			callbackElapsedTime,
		)
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf("expected callback action after cancellation observation, got %#v", actions)
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

func TestRunConcurrentHooksForEventsWithContext_CanceledContextSkipsExecution(t *testing.T) {
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

	var callbackCount atomic.Int32
	eventsWithHooks := []eventWithHooks{
		{
			classified: classifiedEvent{event: waveEvent(firstChangedPath)},
			hookCtx:    &wave.HookContext{FilePath: firstChangedPath},
			hooks: &wave.SortedHooks{
				Concurrent: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							callbackCount.Add(1)
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
							callbackCount.Add(1)
							return &wave.RefreshAction{WaitForApp: true}, nil
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

	actions := s.runConcurrentHooksForEventsWithContext(
		concurrentHookExecutionContext,
		eventsWithHooks,
		watcher,
	)
	if len(actions) != 0 {
		t.Fatalf("expected no actions when context is canceled, got %#v", actions)
	}
	if callbackCount.Load() != 0 {
		t.Fatalf("expected callback count=0 when context is canceled, got %d", callbackCount.Load())
	}
}

func TestRunConcurrentHooksWithContext_CanceledContextStopsRunningCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "concurrent-command.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
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
	_, err := s.runConcurrentHooksWithContext(
		concurrentHookExecutionContext,
		ewh,
		watcher,
	)
	commandElapsedTime := time.Since(commandStartTime)
	if err == nil {
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
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	s.cfg.Watch.HookCallbackTimeouts = wave.HookCallbackTimeoutConfig{
		PreCallbackTimeoutMilliseconds: 100,
	}

	changedPath := filepath.Join(t.TempDir(), "pre-callback-timeout.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
						select {
						case <-hookContext.ExecutionContext.Done():
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						case <-time.After(2 * time.Second):
							return nil, os.ErrDeadlineExceeded
						}
					},
				},
			},
		},
	}

	callbackStartTime := time.Now()
	actions, err := s.runPreHooks(ewh, watcher)
	callbackElapsedTime := time.Since(callbackStartTime)
	if err != nil {
		t.Fatalf("expected pre callback timeout path to succeed cooperatively, got %v", err)
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf("expected pre callback action after timeout cancellation, got %#v", actions)
	}
	if callbackElapsedTime > 1*time.Second {
		t.Fatalf("expected pre callback timeout to return quickly, elapsed=%s", callbackElapsedTime)
	}
}

func TestRunPreHooks_CommandTimeoutUsesPreStageSetting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	s.cfg.Watch.HookCommandTimeouts = wave.HookCommandTimeoutConfig{
		PreCommandTimeoutMilliseconds: 100,
	}

	changedPath := filepath.Join(t.TempDir(), "pre-timeout.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{Cmd: "sleep 2"},
			},
		},
	}

	commandStartTime := time.Now()
	_, err := s.runPreHooks(ewh, watcher)
	commandElapsedTime := time.Since(commandStartTime)
	if err == nil {
		t.Fatal("expected pre hook command to time out")
	}
	if !errors.Is(err, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf("expected timed-out command classification, got %v", err)
	}
	if !strings.Contains(err.Error(), "pre hook failed for "+changedPath) {
		t.Fatalf("expected pre hook stage/path attribution, got %q", err.Error())
	}
	if commandElapsedTime > 1*time.Second {
		t.Fatalf("expected pre hook timeout to stop quickly, elapsed=%s", commandElapsedTime)
	}
}

func TestRunConcurrentHooks_CommandTimeoutUsesConcurrentStageSetting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	s.cfg.Watch.HookCommandTimeouts = wave.HookCommandTimeoutConfig{
		ConcurrentCommandTimeoutMilliseconds: 100,
	}

	changedPath := filepath.Join(t.TempDir(), "concurrent-timeout.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Concurrent: []wave.OnChangeHook{
				{Cmd: "sleep 2"},
			},
		},
	}

	commandStartTime := time.Now()
	_, err := s.runConcurrentHooks(ewh, watcher)
	commandElapsedTime := time.Since(commandStartTime)
	if err == nil {
		t.Fatal("expected concurrent hook command to time out")
	}
	if !errors.Is(err, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf("expected timed-out command classification, got %v", err)
	}
	if !strings.Contains(err.Error(), "concurrent hook failed for "+changedPath) {
		t.Fatalf("expected concurrent hook stage/path attribution, got %q", err.Error())
	}
	if commandElapsedTime > 1*time.Second {
		t.Fatalf("expected concurrent hook timeout to stop quickly, elapsed=%s", commandElapsedTime)
	}
}

func TestRunPostHooks_CommandTimeoutUsesPostStageSetting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	s.cfg.Watch.HookCommandTimeouts = wave.HookCommandTimeoutConfig{
		PostCommandTimeoutMilliseconds: 100,
	}

	changedPath := filepath.Join(t.TempDir(), "post-timeout.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Post: []wave.OnChangeHook{
				{Cmd: "sleep 2"},
			},
		},
	}

	commandStartTime := time.Now()
	_, err := s.runPostHooks(ewh, watcher)
	commandElapsedTime := time.Since(commandStartTime)
	if err == nil {
		t.Fatal("expected post hook command to time out")
	}
	if !errors.Is(err, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf("expected timed-out command classification, got %v", err)
	}
	if !strings.Contains(err.Error(), "post hook failed for "+changedPath) {
		t.Fatalf("expected post hook stage/path attribution, got %q", err.Error())
	}
	if commandElapsedTime > 1*time.Second {
		t.Fatalf("expected post hook timeout to stop quickly, elapsed=%s", commandElapsedTime)
	}
}

func TestRunPreHooks_PerHookCommandTimeoutOverridesStageTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	s.cfg.Watch.HookCommandTimeouts = wave.HookCommandTimeoutConfig{
		PreCommandTimeoutMilliseconds: 2000,
	}

	changedPath := filepath.Join(t.TempDir(), "pre-timeout-override.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					Cmd:                        "sleep 2",
					CommandTimeoutMilliseconds: 100,
				},
			},
		},
	}

	commandStartTime := time.Now()
	_, err := s.runPreHooks(ewh, watcher)
	commandElapsedTime := time.Since(commandStartTime)
	if err == nil {
		t.Fatal("expected pre hook command timeout override to trigger")
	}
	if !errors.Is(err, executil.ErrCommandExecutionTimedOut) {
		t.Fatalf("expected timed-out command classification, got %v", err)
	}
	if !strings.Contains(err.Error(), "pre hook failed for "+changedPath) {
		t.Fatalf("expected pre hook stage/path attribution, got %q", err.Error())
	}
	if commandElapsedTime > 1*time.Second {
		t.Fatalf("expected per-hook timeout override to stop quickly, elapsed=%s", commandElapsedTime)
	}
}

func TestRunPreHooks_DisableStageCommandTimeoutBypassesStageTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	s.cfg.Watch.HookCommandTimeouts = wave.HookCommandTimeoutConfig{
		PreCommandTimeoutMilliseconds: 100,
	}

	changedPath := filepath.Join(t.TempDir(), "pre-timeout-disable.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					Cmd:                        "sleep 1",
					DisableStageCommandTimeout: true,
				},
			},
		},
	}

	commandStartTime := time.Now()
	_, err := s.runPreHooks(ewh, watcher)
	commandElapsedTime := time.Since(commandStartTime)
	if err != nil {
		t.Fatalf("expected stage-timeout-disabled pre hook to succeed, got %v", err)
	}
	if commandElapsedTime < 800*time.Millisecond {
		t.Fatalf(
			"expected disabled stage timeout to allow hook command runtime, elapsed=%s",
			commandElapsedTime,
		)
	}
}

func TestRunPreHooks_PerHookCallbackTimeoutOverridesStageTimeout(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	s.cfg.Watch.HookCallbackTimeouts = wave.HookCallbackTimeoutConfig{
		PreCallbackTimeoutMilliseconds: 2000,
	}

	changedPath := filepath.Join(t.TempDir(), "pre-callback-timeout-override.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					CallbackTimeoutMilliseconds: 100,
					Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
						select {
						case <-hookContext.ExecutionContext.Done():
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						case <-time.After(2 * time.Second):
							return nil, os.ErrDeadlineExceeded
						}
					},
				},
			},
		},
	}

	callbackStartTime := time.Now()
	actions, err := s.runPreHooks(ewh, watcher)
	callbackElapsedTime := time.Since(callbackStartTime)
	if err != nil {
		t.Fatalf("expected per-hook callback timeout override to succeed cooperatively, got %v", err)
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf("expected callback action after per-hook timeout override, got %#v", actions)
	}
	if callbackElapsedTime > 1*time.Second {
		t.Fatalf("expected per-hook callback timeout override to return quickly, elapsed=%s", callbackElapsedTime)
	}
}

func TestRunPreHooks_DisableStageCallbackTimeoutBypassesStageTimeout(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	s.cfg.Watch.HookCallbackTimeouts = wave.HookCallbackTimeoutConfig{
		PreCallbackTimeoutMilliseconds: 100,
	}

	changedPath := filepath.Join(t.TempDir(), "pre-callback-timeout-disable.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			Pre: []wave.OnChangeHook{
				{
					DisableStageCallbackTimeout: true,
					Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
						select {
						case <-hookContext.ExecutionContext.Done():
							return nil, os.ErrDeadlineExceeded
						case <-time.After(300 * time.Millisecond):
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						}
					},
				},
			},
		},
	}

	callbackStartTime := time.Now()
	actions, err := s.runPreHooks(ewh, watcher)
	callbackElapsedTime := time.Since(callbackStartTime)
	if err != nil {
		t.Fatalf("expected disabled stage callback timeout to allow callback runtime, got %v", err)
	}
	if len(actions) != 1 || !actions[0].ReloadBrowser {
		t.Fatalf("expected callback action when stage callback timeout is disabled, got %#v", actions)
	}
	if callbackElapsedTime < 250*time.Millisecond {
		t.Fatalf(
			"expected disabled stage callback timeout to allow callback runtime, elapsed=%s",
			callbackElapsedTime,
		)
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

func TestRunPreHooks_ErrorIncludesStageAndChangedPath(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "pre-error.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
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

	_, err := s.runPreHooks(ewh, watcher)
	if err == nil {
		t.Fatal("expected pre-hook error")
	}
	if !strings.Contains(err.Error(), "pre hook failed for "+changedPath) {
		t.Fatalf("expected pre-hook error to include stage/path attribution, got %q", err.Error())
	}
}

func TestRunConcurrentHooks_ErrorIncludesStageAndChangedPath(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "concurrent-error.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
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

	_, err := s.runConcurrentHooks(ewh, watcher)
	if err == nil {
		t.Fatal("expected concurrent-hook error")
	}
	if !strings.Contains(err.Error(), "concurrent hook failed for "+changedPath) {
		t.Fatalf(
			"expected concurrent-hook error to include stage/path attribution, got %q",
			err.Error(),
		)
	}
}

func TestRunPostHooks_ErrorIncludesStageAndChangedPath(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "post-error.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
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

	_, err := s.runPostHooks(ewh, watcher)
	if err == nil {
		t.Fatal("expected post-hook error")
	}
	if !strings.Contains(err.Error(), "post hook failed for "+changedPath) {
		t.Fatalf("expected post-hook error to include stage/path attribution, got %q", err.Error())
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

func TestFireNoWaitHooks_CallbacksReceiveIndependentHookContexts(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "no-wait-context-clone.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	s.concurrentNoWaitHookExecutionLimiter = make(chan struct{}, 2)

	receivedHookContexts := make(chan *wave.HookContext, 2)
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx: &wave.HookContext{
			FilePath:         changedPath,
			ChangedFilePaths: []string{changedPath},
		},
		hooks: &wave.SortedHooks{
			ConcurrentNoWait: []wave.OnChangeHook{
				{
					Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
						receivedHookContexts <- hookContext
						return nil, nil
					},
				},
				{
					Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
						receivedHookContexts <- hookContext
						return nil, nil
					},
				},
			},
		},
	}

	s.fireNoWaitHooks(ewh, watcher)

	var firstHookContext *wave.HookContext
	select {
	case firstHookContext = <-receivedHookContexts:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for first no-wait callback context")
	}

	var secondHookContext *wave.HookContext
	select {
	case secondHookContext = <-receivedHookContexts:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for second no-wait callback context")
	}

	if firstHookContext == secondHookContext {
		t.Fatal("expected no-wait callbacks to receive independent hook context instances")
	}
	if firstHookContext.ExecutionContext == nil || secondHookContext.ExecutionContext == nil {
		t.Fatal("expected no-wait callback hook contexts to include execution context")
	}
}

func TestFireNoWaitHooks_CallbackCanObserveExecutionContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	changedPath := filepath.Join(t.TempDir(), "no-wait-callback-cancellation.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	s.cfg.Watch.HookCommandTimeouts = wave.HookCommandTimeoutConfig{
		ConcurrentNoWaitCommandTimeoutMilliseconds: 100,
	}
	s.cfg.Watch.HookCallbackTimeouts = wave.HookCallbackTimeoutConfig{
		ConcurrentNoWaitCallbackTimeoutMilliseconds: 100,
	}

	callbackDone := make(chan struct{}, 1)
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			ConcurrentNoWait: []wave.OnChangeHook{
				{
					Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
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

	s.fireNoWaitHooks(ewh, watcher)

	select {
	case <-callbackDone:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for no-wait callback to observe execution context cancellation")
	}
}

func TestFireNoWaitHooks_CleanupForRebuildCancelsInFlightHooks(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	s.watcher = watcher
	s.builder = NewBuilder(s.cfg, newDiscardLogger())
	defer s.builder.Close()

	changedPath := filepath.Join(t.TempDir(), "no-wait-cleanup-cancel.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	hookStarted := make(chan struct{}, 1)
	hookCanceled := make(chan struct{}, 1)
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			ConcurrentNoWait: []wave.OnChangeHook{
				{
					Callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
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

	s.fireNoWaitHooks(ewh, watcher)

	select {
	case <-hookStarted:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for no-wait hook to start")
	}

	s.cleanupForRebuild()

	select {
	case <-hookCanceled:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected cleanupForRebuild to cancel in-flight no-wait hook execution")
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

func TestFireNoWaitHooks_CommandTimeoutUsesConcurrentNoWaitStageSetting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command assertion is Unix-oriented")
	}

	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	s.concurrentNoWaitHookExecutionLimiter = make(chan struct{}, 1)
	s.cfg.Watch.HookCommandTimeouts = wave.HookCommandTimeoutConfig{
		ConcurrentNoWaitCommandTimeoutMilliseconds: 100,
	}

	changedPath := filepath.Join(t.TempDir(), "concurrent-no-wait-timeout.txt")
	secondHookRanMarkerPath := filepath.Join(t.TempDir(), "concurrent-no-wait-second-hook-ran.txt")
	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			ConcurrentNoWait: []wave.OnChangeHook{
				{Cmd: "sleep 2"},
				{Cmd: "printf 'ran\\n' >> " + strconv.Quote(secondHookRanMarkerPath)},
			},
		},
	}

	stageExecutionStartTime := time.Now()
	s.fireNoWaitHooks(ewh, watcher)

	for {
		if _, statErr := os.Stat(secondHookRanMarkerPath); statErr == nil {
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
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	panicHookCalled := make(chan struct{}, 1)
	followUpHookCalled := make(chan struct{}, 1)

	ewh := eventWithHooks{
		classified: classifiedEvent{event: waveEvent(changedPath)},
		hookCtx:    &wave.HookContext{FilePath: changedPath},
		hooks: &wave.SortedHooks{
			ConcurrentNoWait: []wave.OnChangeHook{
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						panicHookCalled <- struct{}{}
						panic("expected test panic in no-wait callback")
					},
				},
				{
					Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
						followUpHookCalled <- struct{}{}
						return nil, nil
					},
				},
			},
		},
	}

	s.fireNoWaitHooks(ewh, watcher)

	select {
	case <-panicHookCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for panicing no-wait callback")
	}

	select {
	case <-followUpHookCalled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for follow-up no-wait callback after panic")
	}
}

func TestFireNoWaitHooksForEvents_SkipsDuplicateHooks(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

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

	eventsWithHooks := []eventWithHooks{
		newEventWithHooksForStageExecutionTest(
			firstPath,
			&wave.SortedHooks{
				ConcurrentNoWait: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&firstCallbackCount, 1)
							return nil, nil
						},
					},
					{
						Cmd: "printf 'first\\n' >> " + strconv.Quote(commandOut),
					},
				},
			},
			false,
		),
		newEventWithHooksForStageExecutionTest(
			secondPath,
			&wave.SortedHooks{
				ConcurrentNoWait: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&duplicateCallbackCount, 1)
							return nil, nil
						},
					},
					{
						Cmd: "printf 'duplicate\\n' >> " + strconv.Quote(commandOut),
					},
				},
			},
			true,
		),
	}

	s.fireNoWaitHooksForEvents(eventsWithHooks, watcher)

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
		data, err := os.ReadFile(commandOut)
		if err == nil && string(data) == "first\n" {
			break
		}
		if time.Now().After(commandOutputDeadline) {
			t.Fatalf(
				"timed out waiting for no-wait command output (err=%v, first=%d duplicate=%d)",
				err,
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

	finalCommandOutput, err := os.ReadFile(commandOut)
	if err != nil {
		t.Fatalf("failed reading no-wait command output: %v", err)
	}
	if string(finalCommandOutput) != "first\n" {
		t.Fatalf(
			"expected duplicate no-wait command not to run, got output %q",
			string(finalCommandOutput),
		)
	}
}

func TestRunPreHooksForEvents_SkipsDuplicateHooksAndKeepsActionOrder(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	firstPath := filepath.Join(root, "first.txt")
	secondPath := filepath.Join(root, "second.txt")
	thirdPath := filepath.Join(root, "third.txt")

	var firstCallbackCount int32
	var duplicateCallbackCount int32
	var thirdCallbackCount int32

	eventsWithHooks := []eventWithHooks{
		newEventWithHooksForStageExecutionTest(
			firstPath,
			&wave.SortedHooks{
				Pre: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&firstCallbackCount, 1)
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						},
					},
				},
			},
			false,
		),
		newEventWithHooksForStageExecutionTest(
			secondPath,
			&wave.SortedHooks{
				Pre: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&duplicateCallbackCount, 1)
							return &wave.RefreshAction{WaitForApp: true}, nil
						},
					},
				},
			},
			true,
		),
		newEventWithHooksForStageExecutionTest(
			thirdPath,
			&wave.SortedHooks{
				Pre: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&thirdCallbackCount, 1)
							return &wave.RefreshAction{WaitForVite: true}, nil
						},
					},
				},
			},
			false,
		),
	}

	actions := s.runPreHooksForEvents(eventsWithHooks, &workSet{}, watcher)
	if len(actions) != 2 {
		t.Fatalf("pre action count=%d, want 2", len(actions))
	}
	if !actions[0].ReloadBrowser {
		t.Fatalf("expected first pre action from first event, got %#v", actions[0])
	}
	if !actions[1].WaitForVite {
		t.Fatalf("expected second pre action from third event, got %#v", actions[1])
	}
	if atomic.LoadInt32(&firstCallbackCount) != 1 {
		t.Fatalf("expected first pre callback count=1, got %d", atomic.LoadInt32(&firstCallbackCount))
	}
	if atomic.LoadInt32(&duplicateCallbackCount) != 0 {
		t.Fatalf(
			"expected duplicate pre callback count=0, got %d",
			atomic.LoadInt32(&duplicateCallbackCount),
		)
	}
	if atomic.LoadInt32(&thirdCallbackCount) != 1 {
		t.Fatalf("expected third pre callback count=1, got %d", atomic.LoadInt32(&thirdCallbackCount))
	}
}

func TestRunConcurrentHooksForEvents_SkipsDuplicateHooksAndKeepsActionOrder(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	firstPath := filepath.Join(root, "first.txt")
	secondPath := filepath.Join(root, "second.txt")
	thirdPath := filepath.Join(root, "third.txt")

	var firstCallbackCount int32
	var duplicateCallbackCount int32
	var thirdCallbackCount int32

	eventsWithHooks := []eventWithHooks{
		newEventWithHooksForStageExecutionTest(
			firstPath,
			&wave.SortedHooks{
				Concurrent: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&firstCallbackCount, 1)
							time.Sleep(20 * time.Millisecond)
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						},
					},
				},
			},
			false,
		),
		newEventWithHooksForStageExecutionTest(
			secondPath,
			&wave.SortedHooks{
				Concurrent: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&duplicateCallbackCount, 1)
							return &wave.RefreshAction{WaitForApp: true}, nil
						},
					},
				},
			},
			true,
		),
		newEventWithHooksForStageExecutionTest(
			thirdPath,
			&wave.SortedHooks{
				Concurrent: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&thirdCallbackCount, 1)
							return &wave.RefreshAction{WaitForVite: true}, nil
						},
					},
				},
			},
			false,
		),
	}

	actions := s.runConcurrentHooksForEvents(eventsWithHooks, watcher)
	if len(actions) != 2 {
		t.Fatalf("concurrent action count=%d, want 2", len(actions))
	}
	if !actions[0].ReloadBrowser {
		t.Fatalf("expected first concurrent action from first event, got %#v", actions[0])
	}
	if !actions[1].WaitForVite {
		t.Fatalf("expected second concurrent action from third event, got %#v", actions[1])
	}
	if atomic.LoadInt32(&firstCallbackCount) != 1 {
		t.Fatalf("expected first concurrent callback count=1, got %d", atomic.LoadInt32(&firstCallbackCount))
	}
	if atomic.LoadInt32(&duplicateCallbackCount) != 0 {
		t.Fatalf(
			"expected duplicate concurrent callback count=0, got %d",
			atomic.LoadInt32(&duplicateCallbackCount),
		)
	}
	if atomic.LoadInt32(&thirdCallbackCount) != 1 {
		t.Fatalf("expected third concurrent callback count=1, got %d", atomic.LoadInt32(&thirdCallbackCount))
	}
}

func TestRunPostHooksForEvents_SkipsDuplicateHooksAndKeepsActionOrder(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	firstPath := filepath.Join(root, "first.txt")
	secondPath := filepath.Join(root, "second.txt")
	thirdPath := filepath.Join(root, "third.txt")

	var firstCallbackCount int32
	var duplicateCallbackCount int32
	var thirdCallbackCount int32

	eventsWithHooks := []eventWithHooks{
		newEventWithHooksForStageExecutionTest(
			firstPath,
			&wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&firstCallbackCount, 1)
							return &wave.RefreshAction{ReloadBrowser: true}, nil
						},
					},
				},
			},
			false,
		),
		newEventWithHooksForStageExecutionTest(
			secondPath,
			&wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&duplicateCallbackCount, 1)
							return &wave.RefreshAction{WaitForApp: true}, nil
						},
					},
				},
			},
			true,
		),
		newEventWithHooksForStageExecutionTest(
			thirdPath,
			&wave.SortedHooks{
				Post: []wave.OnChangeHook{
					{
						Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
							atomic.AddInt32(&thirdCallbackCount, 1)
							return &wave.RefreshAction{WaitForVite: true}, nil
						},
					},
				},
			},
			false,
		),
	}

	actions := s.runPostHooksForEvents(eventsWithHooks, watcher)
	if len(actions) != 2 {
		t.Fatalf("post action count=%d, want 2", len(actions))
	}
	if !actions[0].ReloadBrowser {
		t.Fatalf("expected first post action from first event, got %#v", actions[0])
	}
	if !actions[1].WaitForVite {
		t.Fatalf("expected second post action from third event, got %#v", actions[1])
	}
	if atomic.LoadInt32(&firstCallbackCount) != 1 {
		t.Fatalf("expected first post callback count=1, got %d", atomic.LoadInt32(&firstCallbackCount))
	}
	if atomic.LoadInt32(&duplicateCallbackCount) != 0 {
		t.Fatalf(
			"expected duplicate post callback count=0, got %d",
			atomic.LoadInt32(&duplicateCallbackCount),
		)
	}
	if atomic.LoadInt32(&thirdCallbackCount) != 1 {
		t.Fatalf("expected third post callback count=1, got %d", atomic.LoadInt32(&thirdCallbackCount))
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

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		s,
		200*time.Millisecond,
	)
	if pendingRestartRequest.recompileGo {
		t.Fatalf(
			"expected no-go restart from prehook action, got %#v",
			pendingRestartRequest,
		)
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

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		s,
		200*time.Millisecond,
	)
	if !pendingRestartRequest.recompileGo {
		t.Fatalf(
			"expected Go recompilation restart, got %#v",
			pendingRestartRequest,
		)
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
	if err := s.executeBuildPhase(work); err != nil {
		t.Fatalf("executeBuildPhase returned error: %v", err)
	}

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

func TestExecuteBuildPhase_CompileGoErrorIsReturned(t *testing.T) {
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
	if err := s.executeBuildPhase(work); err == nil {
		t.Fatal("expected compile-go build phase error")
	}
}

func TestExecuteBuildPhase_WithNilBuilderReturnsError(t *testing.T) {
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
	if err := s.executeBuildPhase(work); err == nil {
		t.Fatal("expected nil-builder build phase error")
	}
}

func TestExecuteHookExecutionPlan_CallbackPanicReturnsErrorAndSkipsCommand(t *testing.T) {
	s, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	commandOutputPath := filepath.Join(t.TempDir(), "hook-command-output.log")
	hookPlan := hookExecutionPlan{
		callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
			panic("expected panic from hook callback")
		},
		command: "printf 'command should not run\\n' >> " + strconv.Quote(commandOutputPath),
	}

	action, err := s.executeHookExecutionPlan(
		hookStageTypePre,
		hookPlan,
		&wave.HookContext{},
	)
	if err == nil {
		t.Fatal("expected callback panic to be surfaced as an error")
	}
	if action != nil {
		t.Fatalf("expected nil action when callback panics, got %#v", action)
	}

	if _, statErr := os.Stat(commandOutputPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected command to be skipped after callback panic, stat error: %v", statErr)
	}
}

func TestExecuteBuildPhase_WithNilBuilderAndNoBuildWorkReturnsNil(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	work := &workSet{}
	if err := s.executeBuildPhase(work); err != nil {
		t.Fatalf("expected no-op build phase to return nil, got %v", err)
	}
}

func TestExecuteBuildPhase_WithNilWorkSetReturnsNil(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	if err := s.executeBuildPhase(nil); err != nil {
		t.Fatalf("expected nil workset to return nil, got %v", err)
	}
}

func TestExecuteBuildPhase_WritePublicFileMapTSErrorIsReturned(t *testing.T) {
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
	if err := s.executeBuildPhase(work); err == nil {
		t.Fatal("expected framework file map write error from build phase")
	}

	statInfo, err := os.Stat(cfg.FrameworkPublicFileMapOutDir)
	if err != nil {
		t.Fatalf("failed stating framework output blocker: %v", err)
	}
	if statInfo.IsDir() {
		t.Fatal("expected framework output blocker to remain a file")
	}
}

func TestResolveHookForStageExecution(t *testing.T) {
	_, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	callbackHook := wave.OnChangeHook{
		Cmd: "echo should-not-run",
		Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
			return nil, nil
		},
	}

	excludedHook, excludedHookShouldRun := resolveHookForStageExecution(
		watcher,
		changedPath,
		false,
		false,
		wave.OnChangeHook{Exclude: []string{changedPath}},
	)
	if excludedHookShouldRun {
		t.Fatalf("expected excluded hook to be skipped, got hook %#v", excludedHook)
	}

	stageHookWithoutRunOnChangeRules, stageHookWithoutRunOnChangeRulesShouldRun := resolveHookForStageExecution(
		watcher,
		changedPath,
		true,
		false,
		callbackHook,
	)
	if !stageHookWithoutRunOnChangeRulesShouldRun {
		t.Fatal("expected hook to run when stage does not apply run-on-change-only rules")
	}
	if stageHookWithoutRunOnChangeRules.Cmd == "" {
		t.Fatalf("expected command to remain for stage without run-on-change-only rules, got %#v", stageHookWithoutRunOnChangeRules)
	}

	commandOnlyHook, commandOnlyHookShouldRun := resolveHookForStageExecution(
		watcher,
		changedPath,
		true,
		true,
		wave.OnChangeHook{Cmd: "echo skip-command-only"},
	)
	if commandOnlyHookShouldRun {
		t.Fatalf("expected command-only hook to be skipped for run-on-change-only stage, got %#v", commandOnlyHook)
	}

	callbackAndCommandHook, callbackAndCommandHookShouldRun := resolveHookForStageExecution(
		watcher,
		changedPath,
		true,
		true,
		callbackHook,
	)
	if !callbackAndCommandHookShouldRun {
		t.Fatal("expected callback hook to be retained for run-on-change-only stage")
	}
	if callbackAndCommandHook.Cmd != "" || callbackAndCommandHook.RunCombinedDevBuildHookCommands {
		t.Fatalf(
			"expected run-on-change-only rules to strip command execution fields, got %#v",
			callbackAndCommandHook,
		)
	}
	if callbackAndCommandHook.Callback == nil {
		t.Fatalf("expected callback to remain after run-on-change-only filtering, got %#v", callbackAndCommandHook)
	}

	normalStageHook, normalStageHookShouldRun := resolveHookForStageExecution(
		watcher,
		changedPath,
		false,
		true,
		callbackHook,
	)
	if !normalStageHookShouldRun {
		t.Fatal("expected hook to run when run-on-change-only mode is disabled")
	}
	if normalStageHook.Cmd == "" {
		t.Fatalf("expected command to remain when run-on-change-only mode is disabled, got %#v", normalStageHook)
	}
}

func TestDeriveExecutableHooksForStage(t *testing.T) {
	_, watcher := newServerAndWatcherForHookExecutionTest(t)
	defer watcher.Close()

	root := t.TempDir()
	changedPath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(changedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed writing changed file: %v", err)
	}

	callbackHook := wave.OnChangeHook{
		Cmd: "echo callback-and-command",
		Callback: func(*wave.HookContext) (*wave.RefreshAction, error) {
			return nil, nil
		},
	}
	commandOnlyHook := wave.OnChangeHook{
		Cmd: "echo command-only",
	}
	excludedHook := wave.OnChangeHook{
		Cmd:     "echo excluded",
		Exclude: []string{changedPath},
	}
	stageHooks := []wave.OnChangeHook{
		excludedHook,
		commandOnlyHook,
		callbackHook,
	}

	t.Run("stage without run-on-change-only rules keeps command hooks", func(t *testing.T) {
		hooksForExecution := deriveExecutableHooksForStage(
			watcher,
			changedPath,
			true,
			false,
			stageHooks,
		)
		if len(hooksForExecution) != 2 {
			t.Fatalf("expected 2 hooks for execution, got %#v", hooksForExecution)
		}
		if hooksForExecution[0].Cmd == "" {
			t.Fatalf("expected command-only hook command to be preserved, got %#v", hooksForExecution[0])
		}
		if hooksForExecution[1].Cmd == "" {
			t.Fatalf("expected callback hook command to be preserved, got %#v", hooksForExecution[1])
		}
		if hooksForExecution[1].Callback == nil {
			t.Fatalf("expected callback hook callback to be preserved, got %#v", hooksForExecution[1])
		}
	})

	t.Run("run-on-change-only stage strips command fields and drops command-only hooks", func(t *testing.T) {
		hooksForExecution := deriveExecutableHooksForStage(
			watcher,
			changedPath,
			true,
			true,
			stageHooks,
		)
		if len(hooksForExecution) != 1 {
			t.Fatalf("expected 1 callback-only hook for execution, got %#v", hooksForExecution)
		}
		if hooksForExecution[0].Cmd != "" || hooksForExecution[0].RunCombinedDevBuildHookCommands {
			t.Fatalf("expected run-on-change-only stage to strip command fields, got %#v", hooksForExecution[0])
		}
		if hooksForExecution[0].Callback == nil {
			t.Fatalf("expected callback hook callback to remain, got %#v", hooksForExecution[0])
		}
	})

	t.Run("run-on-change-only disabled keeps command-only hooks even when stage applies rules", func(t *testing.T) {
		hooksForExecution := deriveExecutableHooksForStage(
			watcher,
			changedPath,
			false,
			true,
			stageHooks,
		)
		if len(hooksForExecution) != 2 {
			t.Fatalf("expected 2 hooks for execution, got %#v", hooksForExecution)
		}
		if hooksForExecution[0].Cmd == "" {
			t.Fatalf("expected command-only hook command to remain, got %#v", hooksForExecution[0])
		}
		if hooksForExecution[1].Cmd == "" {
			t.Fatalf("expected callback hook command to remain, got %#v", hooksForExecution[1])
		}
	})
}

func waveEvent(path string) fsnotify.Event {
	return fsnotify.Event{Name: path, Op: fsnotify.Write}
}

func newEventWithHooksForStageExecutionTest(
	filePath string,
	hooks *wave.SortedHooks,
	skipDuplicateHooks bool,
) eventWithHooks {
	return eventWithHooks{
		classified: classifiedEvent{
			event:    waveEvent(filePath),
			fileType: fileTypeOther,
		},
		hookCtx: &wave.HookContext{
			FilePath: filePath,
		},
		hooks:              hooks,
		skipDuplicateHooks: skipDuplicateHooks,
	}
}
