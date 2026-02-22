package devserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/restartengine"
	"github.com/vormadev/vorma/wave/tooling/internal/shared"
	"github.com/vormadev/vorma/wave/tooling/internal/watch"
)

func TestRunDev_ReturnsValidationErrorForInvalidConfig(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.MainAppEntry = ""

	runError := RunDev(cfg, newDiscardLogger())
	if runError == nil {
		t.Fatal("expected RunDev to fail validation for missing MainAppEntry")
	}
	if !strings.Contains(runError.Error(), "config validation failed") {
		t.Fatalf("unexpected RunDev error: %v", runError)
	}
}

func TestRunDev_ReturnsErrorForNilConfig(t *testing.T) {
	runError := RunDev(nil, newDiscardLogger())
	if runError == nil {
		t.Fatal("expected RunDev to fail for nil config")
	}
	if !strings.Contains(runError.Error(), "config is nil") {
		t.Fatalf("unexpected RunDev error: %v", runError)
	}
}

func TestRunDev_ReturnsLockHeldErrorWhenProjectIsAlreadyLocked(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	lock := shared.NewDevLock(cfg.Dist.Static())
	if lockAcquireError := lock.Acquire(); lockAcquireError != nil {
		t.Fatalf("failed to acquire initial lock: %v", lockAcquireError)
	}
	defer func() {
		_ = lock.Release()
	}()

	runError := RunDev(cfg, newDiscardLogger())
	if runError == nil {
		t.Fatal("expected RunDev to fail when lock is already held")
	}
	if !errors.Is(runError, shared.ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got: %v", runError)
	}
}

func TestRunDev_WithNilLoggerReleasesLockWhenRunReturnsError(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Watch.WatchRoot = filepath.Join(root, "missing-watch-root")

	runError := RunDev(cfg, nil)
	if runError == nil {
		t.Fatal("expected RunDev to fail when watch root does not exist")
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected RunDev error: %v", runError)
	}

	lock := shared.NewDevLock(cfg.Dist.Static())
	if lockAcquireError := lock.Acquire(); lockAcquireError != nil {
		t.Fatalf(
			"expected lock to be released after RunDev error, acquire failed: %v",
			lockAcquireError,
		)
	}
	defer func() {
		_ = lock.Release()
	}()
}

func TestServerRun_ReturnsInitWatcherErrorWhenWatchRootMissing(t *testing.T) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Watch.WatchRoot = filepath.Join(root, "does-not-exist")

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal("expected Run to fail when watch root does not exist")
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}
}

func TestServerRun_BuildFailureThenRetryThenInitWatcherFailure(t *testing.T) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "missing/package/for/devserver/run"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			serverForTest.Mu.Lock()
			watcherReady := serverForTest.Watcher != nil
			serverForTest.Mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-after-retry"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		serverForTest.QueueRestartRequest(
			restartengine.RestartRequest{RecompileGo: false},
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after retry cycle",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for Run helper goroutine")
	}
}

func TestServerRun_SequentialCompileFailureThenRetryThenInitWatcherFailure(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.SequentialGoBuild = true
	cfg.Core.MainAppEntry = "missing/package/for/devserver/sequential"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			serverForTest.Mu.Lock()
			watcherReady := serverForTest.Watcher != nil
			serverForTest.Mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-after-sequential-retry"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		serverForTest.QueueRestartRequest(
			restartengine.RestartRequest{RecompileGo: false},
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after sequential compile retry",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for Run helper goroutine")
	}
}

func TestServerRun_BuildFailureThenConfigFixRecoversAutomatically(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "missing/package/for/devserver/recovery"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForMainEntryAndWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Core.MainAppEntry,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg: cfg,
		Log: slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		firstWatcher := waitForWatcherPointer(
			serverForTest,
			nil,
			3*time.Second,
		)
		if firstWatcher == nil {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-initial-timeout"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		if !waitForWaitingForBuildRetryFlag(
			serverForTest,
			true,
			3*time.Second,
		) {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-retry-timeout"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		if writeError := writeToolingConfigForMainEntryAndWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			"../../../internal/cmd/sum",
			cfg.Watch.WatchRoot,
		); writeError != nil {
			t.Error(writeError)
			return
		}

		secondWatcher := waitForWatcherPointer(
			serverForTest,
			firstWatcher,
			4*time.Second,
		)
		if secondWatcher == nil {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-recovery-timeout"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		time.Sleep(200 * time.Millisecond)

		if writeError := writeToolingConfigForMainEntryAndWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			"../../../internal/cmd/sum",
			filepath.Join(root, "missing-watch-root-after-recovery"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after recovery validation path",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for recovery helper goroutine")
	}

	runLogOutput := runLogBuffer.String()
	if !strings.Contains(runLogOutput, "build failed") {
		t.Fatalf("expected initial build failure log, got logs: %s", runLogOutput)
	}
	if !strings.Contains(runLogOutput, "build completed") {
		t.Fatalf(
			"expected recovered build success log after config fix, got logs: %s",
			runLogOutput,
		)
	}
	if strings.Contains(runLogOutput, "app did not become ready before timeout") {
		t.Fatalf(
			"expected config-fix recovery path not to log app readiness timeout, got logs: %s",
			runLogOutput,
		)
	}
}

func TestServerRun_NoOpConfigWriteFirstSaveLogsNoopWithoutWatcherRestart(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "../../../internal/cmd/sum"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	var runLogBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg: cfg,
		Log: slog.New(slog.NewTextHandler(&runLogBuffer, nil)),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	var observedUnexpectedWatcherRestart atomic.Bool
	var observedNoopConfigWrite atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)

		firstWatcher := waitForWatcherPointer(
			serverForTest,
			nil,
			3*time.Second,
		)
		if firstWatcher == nil {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-noop-initial-timeout"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		unchangedConfigBytes, readError := os.ReadFile(cfg.Core.ConfigLocation)
		if readError != nil {
			t.Error(readError)
			return
		}
		if writeError := os.WriteFile(
			cfg.Core.ConfigLocation,
			unchangedConfigBytes,
			0o644,
		); writeError != nil {
			t.Error(writeError)
			return
		}
		observedNoopConfigWrite.Store(true)

		if restartedWatcher := waitForWatcherPointer(
			serverForTest,
			firstWatcher,
			700*time.Millisecond,
		); restartedWatcher != nil {
			observedUnexpectedWatcherRestart.Store(true)
		}

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-watch-root-after-noop"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after no-op validation path",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for no-op validation helper goroutine")
	}

	if !observedNoopConfigWrite.Load() {
		t.Fatal("expected helper to execute no-op config write")
	}
	if observedUnexpectedWatcherRestart.Load() {
		t.Fatal("expected no-op config first save not to restart watcher")
	}

	runLogOutput := runLogBuffer.String()
	if !strings.Contains(runLogOutput, "no changes to wave.config.json; skipping restart") {
		t.Fatalf(
			"expected no-op config save to log explicit no-op message, got logs: %s",
			runLogOutput,
		)
	}
}

func TestServerRun_ViteStartFailureStillEntersRestartLoop(t *testing.T) {
	mustConfigureAndGetWaveAppPortForDevserverRunTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "../../../internal/cmd/sum"
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = "command_that_does_not_exist_for_wave_run_vite_test"
	cfg.Vite.DefaultPort = 5199
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			serverForTest.Mu.Lock()
			watcherReady := serverForTest.Watcher != nil
			serverForTest.Mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-after-vite-start-failure"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		serverForTest.QueueRestartRequest(
			restartengine.RestartRequest{RecompileGo: false},
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal("expected Run to exit with watcher init error after restart")
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}
	if _, statError := os.Stat(cfg.Dist.Binary()); statError != nil {
		t.Fatalf(
			"expected first pass to compile binary before Vite start attempt, stat error: %v",
			statError,
		)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for Run helper goroutine")
	}
}

func TestServerRun_ConfigRestartReloadsConfigWithoutWaitingForStaleAppReadiness(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.MainAppEntry = "../../../internal/cmd/sum"
	cfg.Watch.HealthcheckEndpoint = "/healthz"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		cfg.Watch.WatchRoot,
	); writeError != nil {
		t.Fatalf("failed writing initial tooling config: %v", writeError)
	}

	appPort := mustConfigureAndGetWaveAppPortForDevserverRunTests(t)
	appListener, listenError := net.Listen(
		"tcp",
		fmt.Sprintf("127.0.0.1:%d", appPort),
	)
	if listenError != nil {
		t.Skipf(
			"unable to bind app port %d for config-restart test: %v",
			appPort,
			listenError,
		)
	}
	defer appListener.Close()

	var appHealthHits atomic.Int32
	appServer := &http.Server{
		Handler: http.HandlerFunc(func(
			responseWriter http.ResponseWriter,
			request *http.Request,
		) {
			if request.URL.Path == "/healthz" {
				appHealthHits.Add(1)
				responseWriter.WriteHeader(http.StatusOK)
				return
			}
			responseWriter.WriteHeader(http.StatusNotFound)
		}),
	}
	defer appServer.Close()
	go func() {
		_ = appServer.Serve(appListener)
	}()

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		firstWatcher := waitForWatcherPointer(
			serverForTest,
			nil,
			3*time.Second,
		)
		if firstWatcher == nil {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-fallback"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		if !sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{
				RecompileGo:     false,
				IsConfigRestart: true,
			},
			2*time.Second,
		) {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-send-timeout"),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		secondWatcher := waitForWatcherPointer(
			serverForTest,
			firstWatcher,
			4*time.Second,
		)
		if secondWatcher == nil {
			if writeError := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(
					root,
					"missing-watch-root-second-iteration-timeout",
				),
			); writeError != nil {
				t.Error(writeError)
				return
			}
			sendRestartRequestWithTimeout(
				serverForTest,
				restartengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		// Allow the config-restart iteration to execute BroadcastReload(waitApp=true).
		time.Sleep(150 * time.Millisecond)

		if writeError := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-watch-root-after-config-restart"),
		); writeError != nil {
			t.Error(writeError)
			return
		}
		sendRestartRequestWithTimeout(
			serverForTest,
			restartengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	runError := serverForTest.Run()
	if runError == nil {
		t.Fatal(
			"expected Run to exit with watcher init error after orchestration path",
		)
	}
	if !strings.Contains(runError.Error(), "init watcher") {
		t.Fatalf("unexpected Run error: %v", runError)
	}
	if appHealthHits.Load() != 0 {
		t.Fatalf(
			"expected config restart path to reload config without waiting on stale app health probes, got %d probe(s)",
			appHealthHits.Load(),
		)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for orchestration helper goroutine")
	}
}

func TestWaitForBuildRetry_QueuedNoGoRestartIntentIsPreservedForNextPass(
	t *testing.T,
) {
	serverForTest := &Server{
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}
	serverForTest.QueueRestartRequest(
		restartengine.RestartRequest{RecompileGo: false},
	)

	restartRequestForRetry := serverForTest.WaitForBuildRetry()
	nextRunIntent := restartengine.DeriveRunIntentFromRestartRequest(
		restartRequestForRetry,
	)

	if nextRunIntent.RecompileGo {
		t.Fatalf(
			"expected queued retry restart to preserve recompileGo=false, got %#v",
			nextRunIntent,
		)
	}
	if restartRequestForRetry.IsConfigRestart {
		t.Fatalf(
			"expected queued retry restart to preserve isConfigRestart=false, got %#v",
			restartRequestForRetry,
		)
	}

	serverForTest.Mu.Lock()
	waitingForBuildRetry := serverForTest.WaitingForBuildRetry
	serverForTest.Mu.Unlock()
	if waitingForBuildRetry {
		t.Fatal("expected WaitForBuildRetry to clear waiting-for-retry guard")
	}
}

func TestWriteToolingConfigForWatchRoot_UpdatesConfigFileOnly(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")
	originalWatchRoot := cfg.Watch.WatchRoot
	updatedWatchRoot := filepath.Join(root, "updated-watch-root")

	if writeError := writeToolingConfigForWatchRoot(
		cfg.Core.ConfigLocation,
		cfg,
		updatedWatchRoot,
	); writeError != nil {
		t.Fatalf("failed to write updated tooling config: %v", writeError)
	}

	reloadedConfig, parseError := wave.ParseConfigFile(cfg.Core.ConfigLocation)
	if parseError != nil {
		t.Fatalf("failed to parse updated config: %v", parseError)
	}

	if cfg.Watch.WatchRoot != originalWatchRoot {
		t.Fatalf(
			"expected base config watch root to remain %q, got %q",
			originalWatchRoot,
			cfg.Watch.WatchRoot,
		)
	}
	if reloadedConfig.Watch == nil ||
		reloadedConfig.Watch.WatchRoot != updatedWatchRoot {
		t.Fatalf(
			"expected reloaded config watch root to be %q, got %#v",
			updatedWatchRoot,
			reloadedConfig.Watch,
		)
	}
}

func waitForWatcherPointer(
	serverForTest *Server,
	previousWatcher *watch.Watcher,
	timeout time.Duration,
) *watch.Watcher {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		serverForTest.Mu.Lock()
		currentWatcher := serverForTest.Watcher
		serverForTest.Mu.Unlock()

		if currentWatcher != nil && currentWatcher != previousWatcher {
			return currentWatcher
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func sendRestartRequestWithTimeout(
	serverForTest *Server,
	request restartengine.RestartRequest,
	timeout time.Duration,
) bool {
	if serverForTest == nil {
		return false
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		serverForTest.QueueRestartRequest(request)
		return true
	}
	return false
}

func waitForWaitingForBuildRetryFlag(
	serverForTest *Server,
	expectedFlag bool,
	timeout time.Duration,
) bool {
	if serverForTest == nil {
		return false
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		serverForTest.Mu.Lock()
		waitingForBuildRetry := serverForTest.WaitingForBuildRetry
		serverForTest.Mu.Unlock()

		if waitingForBuildRetry == expectedFlag {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func writeToolingConfigForWatchRoot(
	configFilePath string,
	baseConfig *wave.ParsedConfig,
	watchRoot string,
) error {
	return writeToolingConfigForMainEntryAndWatchRoot(
		configFilePath,
		baseConfig,
		baseConfig.Core.MainAppEntry,
		watchRoot,
	)
}

func writeToolingConfigForMainEntryAndWatchRoot(
	configFilePath string,
	baseConfig *wave.ParsedConfig,
	mainAppEntry string,
	watchRoot string,
) error {
	if baseConfig == nil || baseConfig.Core == nil {
		return fmt.Errorf("base config missing required core section")
	}

	configForDisk := *baseConfig
	configCoreForDisk := *baseConfig.Core
	configCoreForDisk.ConfigLocation = configFilePath
	configCoreForDisk.MainAppEntry = mainAppEntry
	configForDisk.Core = &configCoreForDisk

	configWatchForDisk := &wave.WatchConfig{}
	if baseConfig.Watch != nil {
		*configWatchForDisk = *baseConfig.Watch
	}
	configWatchForDisk.WatchRoot = watchRoot
	configForDisk.Watch = configWatchForDisk

	configForDiskJSON, marshalError := json.Marshal(configForDisk)
	if marshalError != nil {
		return fmt.Errorf("marshal updated tooling config: %w", marshalError)
	}

	if writeError := os.WriteFile(
		configFilePath,
		configForDiskJSON,
		0o644,
	); writeError != nil {
		return fmt.Errorf("write tooling config file: %w", writeError)
	}
	return nil
}

func mustConfigureAndGetWaveAppPortForDevserverRunTests(t *testing.T) int {
	t.Helper()

	listener, listenError := net.Listen("tcp", "127.0.0.1:0")
	if listenError != nil {
		t.Fatalf("failed to reserve app port for test: %v", listenError)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if closeError := listener.Close(); closeError != nil {
		t.Fatalf(
			"failed to release reserved app port %d for test: %v",
			port,
			closeError,
		)
	}

	t.Setenv("__WAVE_MODE", "production")
	t.Setenv("__WAVE_PORT_HAS_BEEN_SET", "true")
	t.Setenv("PORT", fmt.Sprintf("%d", port))

	return port
}
