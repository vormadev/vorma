package tooling

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
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
