package tooling

import (
	"fmt"
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
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Watch.WatchRoot = filepath.Join(root, "does-not-exist")

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		restartCh: make(chan restartRequest, 1),
	}

	err := s.run()
	if err == nil {
		t.Fatal("expected run to fail when watch root does not exist")
	}
	if !strings.Contains(err.Error(), "init watcher") {
		t.Fatalf("unexpected run error: %v", err)
	}
}

func TestServerRun_BuildFailureThenRetryThenInitWatcherFailure(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "missing/package/for/devserver/run"

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		restartCh: make(chan restartRequest, 1),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			watcherReady := s.watcher != nil
			s.mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		cfg.Watch.WatchRoot = filepath.Join(root, "missing-after-retry")
		s.restartCh <- restartRequest{recompileGo: false}
	}()

	err := s.run()
	if err == nil {
		t.Fatal("expected run to exit with watcher init error after retry cycle")
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

func TestServerRun_SequentialCompileFailureThenRetryThenInitWatcherFailure(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.SequentialGoBuild = true
	cfg.Core.MainAppEntry = "missing/package/for/devserver/sequential"

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		restartCh: make(chan restartRequest, 1),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			watcherReady := s.watcher != nil
			s.mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		cfg.Watch.WatchRoot = filepath.Join(root, "missing-after-sequential-retry")
		s.restartCh <- restartRequest{recompileGo: false}
	}()

	err := s.run()
	if err == nil {
		t.Fatal("expected run to exit with watcher init error after sequential compile retry")
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
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Core.MainAppEntry = "../../internal/scripts/sum"
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: "command_that_does_not_exist_for_wave_run_vite_test",
		DefaultPort:             5199,
	}

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		restartCh: make(chan restartRequest, 1),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			watcherReady := s.watcher != nil
			s.mu.Unlock()
			if watcherReady {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		cfg.Watch.WatchRoot = filepath.Join(root, "missing-after-vite-start-failure")
		s.restartCh <- restartRequest{recompileGo: false}
	}()

	err := s.run()
	if err == nil {
		t.Fatal("expected run to exit with watcher init error after restart")
	}
	if !strings.Contains(err.Error(), "init watcher") {
		t.Fatalf("unexpected run error: %v", err)
	}
	if _, statErr := os.Stat(cfg.Dist.Binary()); statErr != nil {
		t.Fatalf("expected first pass to compile binary before Vite start attempt, stat error: %v", statErr)
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for run helper goroutine")
	}
}

func TestServerRun_ConfigRestartWaitsForAppBeforeReloadAndContinues(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.MainAppEntry = "../../internal/scripts/sum"
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	appPort := wave.MustGetPort()
	appListener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", appPort))
	if err != nil {
		t.Skipf("unable to bind app port %d for config-restart test: %v", appPort, err)
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

	s := &server{
		cfg:       cfg,
		log:       newDiscardLogger(),
		restartCh: make(chan restartRequest, 1),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		firstWatcher := waitForWatcherPointer(s, nil, 3*time.Second)
		if firstWatcher == nil {
			cfg.Watch.WatchRoot = filepath.Join(root, "missing-watch-root-fallback")
			sendRestartRequestWithTimeout(s.restartCh, restartRequest{recompileGo: false}, 250*time.Millisecond)
			return
		}

		if !sendRestartRequestWithTimeout(
			s.restartCh,
			restartRequest{recompileGo: false, isConfigRestart: true},
			2*time.Second,
		) {
			cfg.Watch.WatchRoot = filepath.Join(root, "missing-watch-root-send-timeout")
			sendRestartRequestWithTimeout(s.restartCh, restartRequest{recompileGo: false}, 250*time.Millisecond)
			return
		}

		secondWatcher := waitForWatcherPointer(s, firstWatcher, 4*time.Second)
		if secondWatcher == nil {
			cfg.Watch.WatchRoot = filepath.Join(root, "missing-watch-root-second-iteration-timeout")
			sendRestartRequestWithTimeout(s.restartCh, restartRequest{recompileGo: false}, 250*time.Millisecond)
			return
		}

		// Allow the config-restart iteration to execute broadcastReload(waitApp=true).
		time.Sleep(150 * time.Millisecond)

		cfg.Watch.WatchRoot = filepath.Join(root, "missing-watch-root-after-config-restart")
		sendRestartRequestWithTimeout(s.restartCh, restartRequest{recompileGo: false}, 2*time.Second)
	}()

	err = s.run()
	if err == nil {
		t.Fatal("expected run to exit with watcher init error after orchestration path")
	}
	if !strings.Contains(err.Error(), "init watcher") {
		t.Fatalf("unexpected run error: %v", err)
	}
	if appHealthHits.Load() == 0 {
		t.Fatal("expected config restart path to wait for app health before reload")
	}

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for orchestration helper goroutine")
	}
}

func waitForWatcherPointer(
	s *server,
	previousWatcher *Watcher,
	timeout time.Duration,
) *Watcher {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		currentWatcher := s.watcher
		s.mu.Unlock()

		if currentWatcher != nil && currentWatcher != previousWatcher {
			return currentWatcher
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func sendRestartRequestWithTimeout(
	restartChannel chan restartRequest,
	request restartRequest,
	timeout time.Duration,
) bool {
	select {
	case restartChannel <- request:
		return true
	case <-time.After(timeout):
		return false
	}
}
