package tooling

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
)

func TestInitWatcher_SetsWatcherOnServer(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	if err := s.initWatcher(); err != nil {
		t.Fatalf("initWatcher returned error: %v", err)
	}
	defer s.watcher.Close()

	if s.watcher == nil {
		t.Fatal("expected initWatcher to set s.watcher")
	}

	rootKey := s.watcher.norm(cfg.WatchRoot())
	if _, ok := s.watcher.watchedDirs.Load(rootKey); !ok {
		t.Fatalf("expected watch root to be in watched dirs: %s", rootKey)
	}
}

func TestReloadConfig_NoConfigPathIsNoOp(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ConfigLocation = ""

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	original := s.cfg
	if err := s.reloadConfig(); err != nil {
		t.Fatalf("reloadConfig returned error: %v", err)
	}
	if s.cfg != original {
		t.Fatal("expected reloadConfig with empty path to leave config pointer unchanged")
	}
}

func TestReloadConfig_PreservesFrameworkInjectedFields(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "wave.config.json")

	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ConfigLocation = configPath
	cfg.FrameworkWatchPatterns = []wave.WatchedFile{{Pattern: "**/*.route"}}
	cfg.FrameworkIgnoredPatterns = []string{"generated/**"}
	cfg.FrameworkPublicFileMapOutDir = filepath.Join(root, "generated")

	newConfig := map[string]any{
		"Core": map[string]any{
			"MainAppEntry":   "cmd/new",
			"DistDir":        filepath.Join(root, "new-dist"),
			"ServerOnlyMode": true,
		},
	}
	newConfigJSON, err := json.Marshal(newConfig)
	if err != nil {
		t.Fatalf("failed marshaling config JSON: %v", err)
	}
	if err := os.WriteFile(configPath, newConfigJSON, 0644); err != nil {
		t.Fatalf("failed writing config file: %v", err)
	}

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	if err := s.reloadConfig(); err != nil {
		t.Fatalf("reloadConfig returned error: %v", err)
	}

	if s.cfg.Core.MainAppEntry != "cmd/new" {
		t.Fatalf("expected updated MainAppEntry, got %q", s.cfg.Core.MainAppEntry)
	}
	if len(s.cfg.FrameworkWatchPatterns) != 1 || s.cfg.FrameworkWatchPatterns[0].Pattern != "**/*.route" {
		t.Fatalf("framework watch patterns were not preserved: %#v", s.cfg.FrameworkWatchPatterns)
	}
	if len(s.cfg.FrameworkIgnoredPatterns) != 1 || s.cfg.FrameworkIgnoredPatterns[0] != "generated/**" {
		t.Fatalf("framework ignored patterns were not preserved: %#v", s.cfg.FrameworkIgnoredPatterns)
	}
	if s.cfg.FrameworkPublicFileMapOutDir != filepath.Join(root, "generated") {
		t.Fatalf("framework filemap out dir was not preserved: %q", s.cfg.FrameworkPublicFileMapOutDir)
	}
}

func TestReloadConfig_ValidationFailureKeepsPreviousConfig(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "wave.config.json")

	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ConfigLocation = configPath

	invalidConfig := map[string]any{
		"Core": map[string]any{
			"DistDir": filepath.Join(root, "dist-only-missing-main-entry"),
		},
	}
	invalidJSON, err := json.Marshal(invalidConfig)
	if err != nil {
		t.Fatalf("failed marshaling invalid config JSON: %v", err)
	}
	if err := os.WriteFile(configPath, invalidJSON, 0644); err != nil {
		t.Fatalf("failed writing invalid config file: %v", err)
	}

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	original := s.cfg
	err = s.reloadConfig()
	if err == nil {
		t.Fatal("expected reloadConfig to fail on invalid config")
	}
	if s.cfg != original {
		t.Fatal("expected reloadConfig failure to keep previous config pointer")
	}
}

func TestCleanupForRebuild_ClearsWatcherAndBuilder(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	watcher, err := NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("NewWatcher returned error: %v", err)
	}
	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		watcher: watcher,
		builder: builder,
	}

	s.cleanupForRebuild()

	if s.watcher != nil {
		t.Fatal("expected watcher to be cleared by cleanupForRebuild")
	}
	if s.builder != nil {
		t.Fatal("expected builder to be cleared by cleanupForRebuild")
	}
}

func TestCleanupRefreshServer_CancelsManagerAndWaits(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())

	manager := newClientManager()
	ctx, cancel := context.WithCancel(context.Background())
	go manager.start(ctx)

	s := &server{
		cfg:              cfg,
		log:              newDiscardLogger(),
		refreshMgr:       manager,
		refreshMgrCtx:    ctx,
		refreshMgrCancel: cancel,
	}

	s.cleanupRefreshServer()

	if s.refreshMgrCancel != nil {
		t.Fatal("expected refreshMgrCancel to be cleared after cleanupRefreshServer")
	}

	select {
	case <-manager.done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for refresh manager shutdown")
	}
}

func TestStartAndStopRefreshServer(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	manager := newClientManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go manager.start(ctx)

	s := &server{
		cfg:           cfg,
		log:           newDiscardLogger(),
		refreshMgr:    manager,
		refreshMgrCtx: ctx,
	}

	port, err := s.startRefreshServer(0)
	if err != nil {
		t.Fatalf("startRefreshServer returned error: %v", err)
	}
	defer s.stopRefreshServer()

	url := "http://localhost:" + strconv.Itoa(port) + "/get-refresh-script-inner"
	ready := s.waitForReady(url)
	if !ready {
		t.Fatalf("refresh server endpoint did not become ready: %s", url)
	}

	if err := s.stopRefreshServer(); err != nil {
		t.Fatalf("stopRefreshServer returned error: %v", err)
	}
	if s.refreshServer != nil {
		t.Fatal("expected refreshServer to be nil after stopRefreshServer")
	}
}

func TestStartRefreshServer_NoOpInServerOnlyMode(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	port, err := s.startRefreshServer(9999)
	if err != nil {
		t.Fatalf("startRefreshServer returned error in server-only mode: %v", err)
	}
	if port != 0 {
		t.Fatalf("expected no port in server-only mode, got %d", port)
	}
	if s.refreshServer != nil {
		t.Fatal("expected no refresh server in server-only mode")
	}
}

func TestWaitForVite_UsesViteClientEndpoint(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/@vite/client" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer testServer.Close()

	parsedURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("failed parsing test server URL: %v", err)
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		t.Fatalf("failed parsing test server port: %v", err)
	}

	s := &server{
		log:     newDiscardLogger(),
		viteCtx: vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{DefaultPort: port}),
	}

	if !s.waitForVite() {
		t.Fatal("expected waitForVite to return true when /@vite/client is ready")
	}
}

func TestStartAppAndStopApp_WithExecutableBinary(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	if err := SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}

	binPath := cfg.Dist.Binary()
	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(binPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed writing executable test binary: %v", err)
	}

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	s.startApp()
	if s.appCmd == nil || s.appCmd.Process == nil {
		t.Fatal("expected startApp to launch process")
	}

	if err := s.stopApp(); err != nil {
		t.Fatalf("stopApp returned error: %v", err)
	}
}

func TestStartApp_FailureLeavesAppCmdNil(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	s.startApp()
	if s.appCmd != nil {
		t.Fatal("expected startApp failure to leave appCmd nil")
	}
}

func TestStartViteAndStopVite_NoOpWhenViteDisabled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Vite = nil

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
	}

	if err := s.startVite(); err != nil {
		t.Fatalf("startVite returned error with Vite disabled: %v", err)
	}
	if s.viteCtx != nil {
		t.Fatalf("expected no vite context when Vite is disabled, got %#v", s.viteCtx)
	}

	if err := s.stopVite(); err != nil {
		t.Fatalf("stopVite returned error with Vite disabled: %v", err)
	}
}

func TestStartViteAndStopVite_WithViteEnabled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: "echo",
		DefaultPort:             5201,
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
	}

	if err := s.startVite(); err != nil {
		t.Fatalf("startVite returned error: %v", err)
	}
	if s.viteCtx == nil {
		t.Fatal("expected startVite to set viteCtx when Vite is enabled")
	}

	if err := s.stopVite(); err != nil {
		t.Fatalf("stopVite returned error: %v", err)
	}
	if s.viteCtx != nil {
		t.Fatal("expected stopVite to clear viteCtx")
	}
}

func TestCycleVite_NoOpWhenViteDisabled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Vite = nil

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	s.cycleVite()
}

func TestCycleVite_NoOpWhenViteEnabledButNotStarted(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: "echo",
		DefaultPort:             5202,
	}

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	s.cycleVite()
}

func TestCycleVite_StartFailureAfterStopLeavesViteContextCleared(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: "command_that_does_not_exist_for_wave_cycle_test",
		DefaultPort:             5203,
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
		viteCtx: vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{DefaultPort: cfg.Vite.DefaultPort}),
	}

	s.cycleVite()

	if s.viteCtx != nil {
		t.Fatal("expected cycleVite start failure to leave vite context cleared")
	}
}

func TestStartRefreshServer_ReturnsErrorForInvalidPort(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	manager := newClientManager()
	ctx, cancel := context.WithCancel(context.Background())
	go manager.start(ctx)
	defer func() {
		cancel()
		manager.wait()
	}()

	s := &server{
		cfg:           cfg,
		log:           newDiscardLogger(),
		refreshMgr:    manager,
		refreshMgrCtx: ctx,
	}

	if _, err := s.startRefreshServer(-1); err == nil {
		t.Fatal("expected startRefreshServer to return an error for invalid negative port")
	}
}

func TestStartRefreshServer_EventsEndpointSetsCORSAndRejectsNonWebSocket(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	manager := newClientManager()
	ctx, cancel := context.WithCancel(context.Background())
	go manager.start(ctx)
	defer func() {
		cancel()
		manager.wait()
	}()

	s := &server{
		cfg:           cfg,
		log:           newDiscardLogger(),
		refreshMgr:    manager,
		refreshMgrCtx: ctx,
	}

	port, err := s.startRefreshServer(0)
	if err != nil {
		t.Fatalf("startRefreshServer returned error: %v", err)
	}
	defer s.stopRefreshServer()

	resp, err := http.Get("http://localhost:" + strconv.Itoa(port) + "/events")
	if err != nil {
		t.Fatalf("events endpoint request failed: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin header '*', got %q", got)
	}
	if resp.StatusCode < http.StatusBadRequest {
		t.Fatalf("expected non-websocket request to be rejected, got status %d", resp.StatusCode)
	}
}
