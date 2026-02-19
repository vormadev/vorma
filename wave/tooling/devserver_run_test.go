package tooling

import (
	"encoding/json"
	"fmt"
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"github.com/vormadev/vorma/wave/tooling/devserver/devserverengine"
	"github.com/vormadev/vorma/wave/tooling/watch"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
)

func TestServerRun_ReturnsInitWatcherErrorWhenWatchRootMissing(t *testing.T) {
	mustConfigureAndGetWaveAppPortForToolingTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Watch.WatchRoot = filepath.Join(root, "does-not-exist")

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: devserverengine.NewRestartIntentAccumulator(
			make(chan devserverengine.RestartRequest, 1),
		),
	}

	err := s.Run()
	if err == nil {
		t.Fatal("expected run to fail when watch root does not exist")
	}
	if !strings.Contains(err.Error(), "init watcher") {
		t.Fatalf("unexpected run error: %v", err)
	}
}

func TestServerRun_BuildFailureThenRetryThenInitWatcherFailure(t *testing.T) {
	mustConfigureAndGetWaveAppPortForToolingTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "missing/package/for/devserver/run"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if err := writeToolingConfigForWatchRoot(cfg.Core.ConfigLocation, cfg, cfg.Watch.WatchRoot); err != nil {
		t.Fatalf("failed writing initial tooling config: %v", err)
	}

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: devserverengine.NewRestartIntentAccumulator(
			make(chan devserverengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			s.Mu.Lock()
			watcherReady := s.Watcher != nil
			s.Mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if err := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-after-retry"),
		); err != nil {
			t.Error(err)
			return
		}
		queueRestartRequestForToolingTests(
			s,
			devserverengine.RestartRequest{RecompileGo: false},
		)
	}()

	err := s.Run()
	if err == nil {
		t.Fatal(
			"expected run to exit with watcher init error after retry cycle",
		)
	}
	if !strings.Contains(err.Error(), "init watcher") {
		t.Fatalf("unexpected run error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for run helper goroutine")
	}
}

func TestServerRun_SequentialCompileFailureThenRetryThenInitWatcherFailure(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForToolingTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.SequentialGoBuild = true
	cfg.Core.MainAppEntry = "missing/package/for/devserver/sequential"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if err := writeToolingConfigForWatchRoot(cfg.Core.ConfigLocation, cfg, cfg.Watch.WatchRoot); err != nil {
		t.Fatalf("failed writing initial tooling config: %v", err)
	}

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: devserverengine.NewRestartIntentAccumulator(
			make(chan devserverengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			s.Mu.Lock()
			watcherReady := s.Watcher != nil
			s.Mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if err := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-after-sequential-retry"),
		); err != nil {
			t.Error(err)
			return
		}
		queueRestartRequestForToolingTests(
			s,
			devserverengine.RestartRequest{RecompileGo: false},
		)
	}()

	err := s.Run()
	if err == nil {
		t.Fatal(
			"expected run to exit with watcher init error after sequential compile retry",
		)
	}
	if !strings.Contains(err.Error(), "init watcher") {
		t.Fatalf("unexpected run error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for run helper goroutine")
	}
}

func TestServerRun_ViteStartFailureStillEntersRestartLoop(t *testing.T) {
	mustConfigureAndGetWaveAppPortForToolingTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "../../internal/cmd/sum"
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: "command_that_does_not_exist_for_wave_run_vite_test",
		DefaultPort:             5199,
	}
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if err := writeToolingConfigForWatchRoot(cfg.Core.ConfigLocation, cfg, cfg.Watch.WatchRoot); err != nil {
		t.Fatalf("failed writing initial tooling config: %v", err)
	}

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: devserverengine.NewRestartIntentAccumulator(
			make(chan devserverengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			s.Mu.Lock()
			watcherReady := s.Watcher != nil
			s.Mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		if err := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-after-vite-start-failure"),
		); err != nil {
			t.Error(err)
			return
		}
		queueRestartRequestForToolingTests(
			s,
			devserverengine.RestartRequest{RecompileGo: false},
		)
	}()

	err := s.Run()
	if err == nil {
		t.Fatal("expected run to exit with watcher init error after restart")
	}
	if !strings.Contains(err.Error(), "init watcher") {
		t.Fatalf("unexpected run error: %v", err)
	}
	if _, statErr := os.Stat(cfg.Dist.Binary()); statErr != nil {
		t.Fatalf(
			"expected first pass to compile binary before Vite start attempt, stat error: %v",
			statErr,
		)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for run helper goroutine")
	}
}

func TestServerRun_ConfigRestartWaitsForAppBeforeReloadAndContinues(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.MainAppEntry = "../../internal/cmd/sum"
	cfg.Watch.HealthcheckEndpoint = "/healthz"
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")

	if err := writeToolingConfigForWatchRoot(cfg.Core.ConfigLocation, cfg, cfg.Watch.WatchRoot); err != nil {
		t.Fatalf("failed writing initial tooling config: %v", err)
	}

	appPort := mustConfigureAndGetWaveAppPortForToolingTests(t)
	appListener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", appPort))
	if err != nil {
		t.Skipf(
			"unable to bind app port %d for config-restart test: %v",
			appPort,
			err,
		)
	}
	defer appListener.Close()

	var appHealthHits atomic.Int32
	appServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				appHealthHits.Add(1)
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}),
	}
	defer appServer.Close()
	go appServer.Serve(appListener)

	s := &devserver.Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: devserverengine.NewRestartIntentAccumulator(
			make(chan devserverengine.RestartRequest, 1),
		),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		firstWatcher := waitForWatcherPointer(s, nil, 3*time.Second)
		if firstWatcher == nil {
			if err := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-fallback"),
			); err != nil {
				t.Error(err)
				return
			}
			sendRestartRequestWithTimeout(
				s,
				devserverengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		if !sendRestartRequestWithTimeout(
			s,
			devserverengine.RestartRequest{RecompileGo: false, IsConfigRestart: true},
			2*time.Second,
		) {
			if err := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-send-timeout"),
			); err != nil {
				t.Error(err)
				return
			}
			sendRestartRequestWithTimeout(
				s,
				devserverengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		secondWatcher := waitForWatcherPointer(s, firstWatcher, 4*time.Second)
		if secondWatcher == nil {
			if err := writeToolingConfigForWatchRoot(
				cfg.Core.ConfigLocation,
				cfg,
				filepath.Join(root, "missing-watch-root-second-iteration-timeout"),
			); err != nil {
				t.Error(err)
				return
			}
			sendRestartRequestWithTimeout(
				s,
				devserverengine.RestartRequest{RecompileGo: false},
				250*time.Millisecond,
			)
			return
		}

		// Allow the config-restart iteration to execute broadcastReload(waitApp=true).
		time.Sleep(150 * time.Millisecond)

		if err := writeToolingConfigForWatchRoot(
			cfg.Core.ConfigLocation,
			cfg,
			filepath.Join(root, "missing-watch-root-after-config-restart"),
		); err != nil {
			t.Error(err)
			return
		}
		sendRestartRequestWithTimeout(
			s,
			devserverengine.RestartRequest{RecompileGo: false},
			2*time.Second,
		)
	}()

	err = s.Run()
	if err == nil {
		t.Fatal(
			"expected run to exit with watcher init error after orchestration path",
		)
	}
	if !strings.Contains(err.Error(), "init watcher") {
		t.Fatalf("unexpected run error: %v", err)
	}
	if appHealthHits.Load() == 0 {
		t.Fatal(
			"expected config restart path to wait for app health before reload",
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
	s := &devserver.Server{
		Log: newDiscardLogger(),
		RestartIntents: devserverengine.NewRestartIntentAccumulator(
			make(chan devserverengine.RestartRequest, 1),
		),
	}
	queueRestartRequestForToolingTests(s, devserverengine.RestartRequest{RecompileGo: false})

	restartRequestForRetry := s.WaitForBuildRetry()
	nextRunIntent := devserverengine.DeriveRunIntentFromRestartRequest(restartRequestForRetry)

	if nextRunIntent.RecompileGo {
		t.Fatalf(
			"expected queued retry restart to preserve recompileGo=false, got %#v",
			nextRunIntent,
		)
	}
	if nextRunIntent.IsConfigRestart {
		t.Fatalf(
			"expected queued retry restart to preserve isConfigRestart=false, got %#v",
			nextRunIntent,
		)
	}
	s.RestartIntents.Mu.Lock()
	waitingForBuildRetry := s.RestartIntents.WaitingForBuildRetry
	s.RestartIntents.Mu.Unlock()
	if waitingForBuildRetry {
		t.Fatal("expected waitForBuildRetry to clear waiting-for-retry guard")
	}
}

func TestWriteToolingConfigForWatchRoot_UpdatesConfigFileOnly(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ConfigLocation = filepath.Join(root, "wave.config.json")
	originalWatchRoot := cfg.Watch.WatchRoot
	updatedWatchRoot := filepath.Join(root, "updated-watch-root")

	if err := writeToolingConfigForWatchRoot(cfg.Core.ConfigLocation, cfg, updatedWatchRoot); err != nil {
		t.Fatalf("failed to write updated tooling config: %v", err)
	}

	reloadedConfig, err := wave.ParseConfigFile(cfg.Core.ConfigLocation)
	if err != nil {
		t.Fatalf("failed to parse updated config: %v", err)
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
	s *devserver.Server,
	previousWatcher *watch.Watcher,
	timeout time.Duration,
) *watch.Watcher {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.Mu.Lock()
		currentWatcher := s.Watcher
		s.Mu.Unlock()

		if currentWatcher != nil && currentWatcher != previousWatcher {
			return currentWatcher
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func sendRestartRequestWithTimeout(
	s *devserver.Server,
	request devserverengine.RestartRequest,
	timeout time.Duration,
) bool {
	if s == nil {
		return false
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.QueueRestartRequest(request)
		return true
	}
	return false
}

func writeToolingConfigForWatchRoot(
	configFilePath string,
	baseConfig *wave.ParsedConfig,
	watchRoot string,
) error {
	if baseConfig == nil || baseConfig.Core == nil {
		return fmt.Errorf("base config missing required core section")
	}

	configForDisk := *baseConfig
	configCoreForDisk := *baseConfig.Core
	configCoreForDisk.ConfigLocation = configFilePath
	configForDisk.Core = &configCoreForDisk

	configWatchForDisk := &wave.WatchConfig{}
	if baseConfig.Watch != nil {
		*configWatchForDisk = *baseConfig.Watch
	}
	configWatchForDisk.WatchRoot = watchRoot
	configForDisk.Watch = configWatchForDisk

	configForDiskJSON, err := json.Marshal(configForDisk)
	if err != nil {
		return fmt.Errorf("marshal updated tooling config: %w", err)
	}

	if err := os.WriteFile(configFilePath, configForDiskJSON, 0o644); err != nil {
		return fmt.Errorf("write tooling config file: %w", err)
	}

	return nil
}
