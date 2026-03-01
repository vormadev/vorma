package devserver

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

	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavebuild/builder"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/restartengine"
	"github.com/vormadev/vorma/wave/wavedev/internal/broadcast"
	"github.com/vormadev/vorma/wave/wavedev/internal/watch"
)

func TestInitWatcher_SetsWatcherOnServer(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	if err := s.InitWatcher(); err != nil {
		t.Fatalf("initWatcher returned error: %v", err)
	}
	defer s.Watcher.Close()

	if s.Watcher == nil {
		t.Fatal("expected initWatcher to set s.watcher")
	}

	if !s.Watcher.IsWatchingDir(cfg.WatchRoot()) {
		t.Fatalf(
			"expected watch root to be in watched dirs: %s",
			s.Watcher.NormalizePath(cfg.WatchRoot()),
		)
	}
}

func TestInitWatcher_AddsConfigFileDirectoryOutsideWatchRoot(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

	outsideConfigDirectory := t.TempDir()
	outsideConfigFilePath := filepath.Join(
		outsideConfigDirectory,
		"wave.config.json",
	)
	if err := os.WriteFile(outsideConfigFilePath, []byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`), 0644); err != nil {
		t.Fatalf("failed writing outside config file: %v", err)
	}

	cfg.Core.ConfigLocation = outsideConfigFilePath

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	if err := s.InitWatcher(); err != nil {
		t.Fatalf("initWatcher returned error: %v", err)
	}
	defer s.Watcher.Close()

	if !s.Watcher.IsWatchingDir(outsideConfigDirectory) {
		t.Fatalf(
			"expected config file directory to be watched: %s",
			outsideConfigDirectory,
		)
	}
}

func TestAddConfigFileDirectory_IsIdempotent(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

	outsideConfigDirectory := t.TempDir()
	cfg.Core.ConfigLocation = filepath.Join(
		outsideConfigDirectory,
		"wave.config.json",
	)

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	if err := s.InitWatcher(); err != nil {
		t.Fatalf("initWatcher returned error: %v", err)
	}
	defer s.Watcher.Close()

	normalizedOutsideConfigDirectory := s.Watcher.NormalizePath(
		outsideConfigDirectory,
	)
	watchedDirectoryPaths := s.Watcher.WatchedDirectoryPaths()
	occurrenceCountBefore := 0
	for _, watchedDirectoryPath := range watchedDirectoryPaths {
		if watchedDirectoryPath == normalizedOutsideConfigDirectory {
			occurrenceCountBefore++
		}
	}
	if occurrenceCountBefore != 1 {
		t.Fatalf(
			"expected one watched entry for config directory before explicit re-add, got %d",
			occurrenceCountBefore,
		)
	}

	if err := s.addConfigFileDirectory(); err != nil {
		t.Fatalf("addConfigFileDirectory returned error: %v", err)
	}

	watchedDirectoryPaths = s.Watcher.WatchedDirectoryPaths()
	occurrenceCountAfter := 0
	for _, watchedDirectoryPath := range watchedDirectoryPaths {
		if watchedDirectoryPath == normalizedOutsideConfigDirectory {
			occurrenceCountAfter++
		}
	}
	if occurrenceCountAfter != 1 {
		t.Fatalf(
			"expected idempotent config directory watch registration, got %d entries",
			occurrenceCountAfter,
		)
	}
}

func TestReloadConfig_NoConfigFilePathIsNoOp(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	original := s.Cfg
	_, err := s.ReloadConfig()
	if err != nil {
		t.Fatalf(
			"reloadConfig returned error with empty config location: %v",
			err,
		)
	}
	if s.Cfg != original {
		t.Fatal(
			"expected reloadConfig with empty config location to keep previous config pointer",
		)
	}
}

func TestReloadConfig_PreservesFrameworkInjectedFields(t *testing.T) {
	root := t.TempDir()

	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.FrameworkWatchPatterns = []wave.WatchedFile{{Pattern: "**/*.route"}}
	cfg.FrameworkIgnoredPatterns = []string{"generated/**"}
	cfg.FrameworkPublicFileMapOutDir = filepath.Join(root, "generated")
	cfg.FrameworkDevBuildHook = "go run ./backend/cmd/build --dev --hook"
	cfg.FrameworkProdBuildHook = "go run ./backend/cmd/build --hook"
	cfg.FrameworkSchemaExtensions = map[string]jsonschema.Entry{
		"Vorma": {
			Type: jsonschema.TypeObject,
		},
	}
	cfg.FrameworkRunBuildHook = func(context.Context, bool) error { return nil }
	cfg.FrameworkPrepareGoBuildOverlay = func() (*wave.GoBuildOverlay, error) {
		return &wave.GoBuildOverlay{
			OverlayConfigPath: filepath.Join(root, "go-overlay.json"),
		}, nil
	}

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
	configPath := filepath.Join(root, "backend", "wave.config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("failed creating config parent directory: %v", err)
	}
	if err := os.WriteFile(configPath, newConfigJSON, 0644); err != nil {
		t.Fatalf("failed writing config JSON: %v", err)
	}
	cfg.Core.ConfigLocation = configPath

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	if _, err := s.ReloadConfig(); err != nil {
		t.Fatalf("reloadConfig returned error: %v", err)
	}

	if s.Cfg.Core.MainAppEntry != "cmd/new" {
		t.Fatalf(
			"expected updated MainAppEntry, got %q",
			s.Cfg.Core.MainAppEntry,
		)
	}
	if len(s.Cfg.FrameworkWatchPatterns) != 1 ||
		s.Cfg.FrameworkWatchPatterns[0].Pattern != "**/*.route" {
		t.Fatalf(
			"framework watch patterns were not preserved: %#v",
			s.Cfg.FrameworkWatchPatterns,
		)
	}
	if len(s.Cfg.FrameworkIgnoredPatterns) != 1 ||
		s.Cfg.FrameworkIgnoredPatterns[0] != "generated/**" {
		t.Fatalf(
			"framework ignored patterns were not preserved: %#v",
			s.Cfg.FrameworkIgnoredPatterns,
		)
	}
	if s.Cfg.FrameworkPublicFileMapOutDir != filepath.Join(root, "generated") {
		t.Fatalf(
			"framework filemap out dir was not preserved: %q",
			s.Cfg.FrameworkPublicFileMapOutDir,
		)
	}
	if s.Cfg.FrameworkDevBuildHook != "go run ./backend/cmd/build --dev --hook" {
		t.Fatalf(
			"framework dev build hook was not preserved: %q",
			s.Cfg.FrameworkDevBuildHook,
		)
	}
	if s.Cfg.FrameworkProdBuildHook != "go run ./backend/cmd/build --hook" {
		t.Fatalf(
			"framework prod build hook was not preserved: %q",
			s.Cfg.FrameworkProdBuildHook,
		)
	}
	if got := s.Cfg.FrameworkSchemaExtensions["Vorma"].Type; got != jsonschema.TypeObject {
		t.Fatalf(
			"framework schema extension was not preserved: %#v",
			s.Cfg.FrameworkSchemaExtensions["Vorma"],
		)
	}
	if s.Cfg.FrameworkRunBuildHook == nil {
		t.Fatal("framework run build hook was not preserved")
	}
	if err := s.Cfg.FrameworkRunBuildHook(context.Background(), true); err != nil {
		t.Fatalf("framework run build hook returned error: %v", err)
	}
	if s.Cfg.FrameworkPrepareGoBuildOverlay == nil {
		t.Fatal("framework go build overlay preparer was not preserved")
	}
	overlay, overlayErr := s.Cfg.FrameworkPrepareGoBuildOverlay()
	if overlayErr != nil {
		t.Fatalf(
			"framework go build overlay preparer returned error: %v",
			overlayErr,
		)
	}
	if overlay == nil ||
		overlay.OverlayConfigPath != filepath.Join(root, "go-overlay.json") {
		t.Fatalf("framework go build overlay was not preserved: %#v", overlay)
	}
}

func TestReloadConfig_UsesConfigLocationWhenAvailable(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "backend", "wave.config.json")
	newConfigJSON := []byte(`{
		"Core": {
			"MainAppEntry": "cmd/new",
			"DistDir": "` + filepath.ToSlash(filepath.Join(root, "dist")) + `",
			"StaticAssetDirs": {"Public":"` + filepath.ToSlash(filepath.Join(root, "public")) + `","Private":"` + filepath.ToSlash(filepath.Join(root, "private")) + `"}
		}
	}`)
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("failed creating config parent directory: %v", err)
	}
	if err := os.WriteFile(configPath, newConfigJSON, 0644); err != nil {
		t.Fatalf("failed writing config JSON: %v", err)
	}

	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ConfigLocation = configPath
	cfg.FrameworkWatchPatterns = []wave.WatchedFile{{Pattern: "**/*.route"}}

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	if _, err := s.ReloadConfig(); err != nil {
		t.Fatalf("reloadConfig returned error: %v", err)
	}

	if s.Cfg.Core.MainAppEntry != "cmd/new" {
		t.Fatalf(
			"expected updated MainAppEntry, got %q",
			s.Cfg.Core.MainAppEntry,
		)
	}
	if got := s.Cfg.Core.ConfigLocation; got != configPath {
		t.Fatalf("config location = %q, want %q", got, configPath)
	}
	if len(s.Cfg.FrameworkWatchPatterns) != 1 {
		t.Fatalf(
			"framework watch patterns were not preserved: %#v",
			s.Cfg.FrameworkWatchPatterns,
		)
	}
}

func TestDidConfigReloadChange_IgnoresEquivalentConfigLocationPathShapes(
	t *testing.T,
) {
	root := t.TempDir()
	t.Chdir(root)

	relativeConfigPath := filepath.Join("backend", "wave.config.json")
	absoluteConfigPath := filepath.Join(root, "backend", "wave.config.json")

	currentConfig := newParsedConfigForToolingTestsAtRoot(root)
	currentConfig.Core.ConfigLocation = relativeConfigPath
	nextConfig := currentConfig.Clone()
	nextConfig.Core.ConfigLocation = absoluteConfigPath

	if didConfigReloadChange(currentConfig, nextConfig) {
		t.Fatalf(
			"expected equivalent relative/absolute config paths to be treated as unchanged: current=%q next=%q",
			currentConfig.Core.ConfigLocation,
			nextConfig.Core.ConfigLocation,
		)
	}
}

func TestReloadConfigIfChanged_FirstNoOpWriteWithRelativeConfigPathIsNoOp(
	t *testing.T,
) {
	root := t.TempDir()
	t.Chdir(root)

	configRelativePath := filepath.Join("backend", "wave.config.json")
	configAbsolutePath := filepath.Join(root, "backend", "wave.config.json")

	configPayload := map[string]any{
		"Core": map[string]any{
			"MainAppEntry":   "cmd/app",
			"DistDir":        filepath.Join(root, "dist"),
			"ConfigLocation": configRelativePath,
			"StaticAssetDirs": map[string]any{
				"Public":  filepath.Join(root, "static", "public"),
				"Private": filepath.Join(root, "static", "private"),
			},
		},
		"Watch": map[string]any{
			"WatchRoot": root,
		},
	}
	configPayloadBytes, marshalError := json.Marshal(configPayload)
	if marshalError != nil {
		t.Fatalf("marshal config payload: %v", marshalError)
	}
	if mkdirError := os.MkdirAll(filepath.Dir(configAbsolutePath), 0o755); mkdirError != nil {
		t.Fatalf("create config directory: %v", mkdirError)
	}
	if writeError := os.WriteFile(
		configAbsolutePath,
		configPayloadBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write config payload: %v", writeError)
	}

	currentConfig := newParsedConfigForToolingTestsAtRoot(root)
	currentConfig.Core.ConfigLocation = configRelativePath

	serverForTest := &Server{
		Cfg: currentConfig,
		Log: newDiscardLogger(),
	}

	configChanged, reloadError := serverForTest.ReloadConfigIfChanged()
	if reloadError != nil {
		t.Fatalf("ReloadConfigIfChanged returned error: %v", reloadError)
	}
	if configChanged {
		t.Fatal(
			"expected first no-op config write path-shape mismatch to report unchanged",
		)
	}
	if serverForTest.Cfg.Core.ConfigLocation != configRelativePath {
		t.Fatalf(
			"expected unchanged config pointer to preserve relative config location %q, got %q",
			configRelativePath,
			serverForTest.Cfg.Core.ConfigLocation,
		)
	}
}

func TestReloadConfig_ValidationFailureKeepsPreviousConfig(t *testing.T) {
	root := t.TempDir()

	cfg := newParsedConfigForToolingTestsAtRoot(root)

	invalidConfig := map[string]any{
		"Core": map[string]any{
			"DistDir": filepath.Join(root, "dist-only-missing-main-entry"),
		},
	}
	invalidJSON, err := json.Marshal(invalidConfig)
	if err != nil {
		t.Fatalf("failed marshaling invalid config JSON: %v", err)
	}
	configPath := filepath.Join(root, "backend", "wave.config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("failed creating config parent directory: %v", err)
	}
	if err := os.WriteFile(configPath, invalidJSON, 0644); err != nil {
		t.Fatalf("failed writing invalid config JSON: %v", err)
	}
	cfg.Core.ConfigLocation = configPath

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	original := s.Cfg
	_, err = s.ReloadConfig()
	if err == nil {
		t.Fatal("expected reloadConfig to fail on invalid config")
	}
	if s.Cfg != original {
		t.Fatal("expected reloadConfig failure to keep previous config pointer")
	}
}

func TestReloadConfig_ConfigReadFailureKeepsPreviousConfig(t *testing.T) {
	root := t.TempDir()

	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ConfigLocation = filepath.Join(
		root,
		"backend",
		"missing-wave.config.json",
	)

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	original := s.Cfg
	_, err := s.ReloadConfig()
	if err == nil {
		t.Fatal("expected reloadConfig to fail when config read fails")
	}
	if s.Cfg != original {
		t.Fatal("expected reloadConfig failure to keep previous config pointer")
	}
}

func TestCleanupForRebuild_ClearsWatcherAndBuilder(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	watcher, err := watch.NewWatcher(cfg, newDiscardLogger())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	builder := builder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &Server{
		Cfg:     cfg,
		Log:     newDiscardLogger(),
		Watcher: watcher,
		Builder: builder,
	}

	s.CleanupForRebuild()

	if s.Watcher != nil {
		t.Fatal("expected watcher to be cleared by cleanupForRebuild")
	}
	if s.Builder != nil {
		t.Fatal("expected builder to be cleared by cleanupForRebuild")
	}
}

func TestStopRefreshServer_CancelsManagerAndWaits(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())

	manager := broadcast.NewManager(
		newDiscardLogger(),
		broadcast.ManagerConfig{},
	)
	ctx, cancel := context.WithCancel(context.Background())
	managerRunDone := make(chan struct{})
	go func() {
		manager.Run(ctx)
		close(managerRunDone)
	}()

	s := &Server{
		Cfg:              cfg,
		Log:              newDiscardLogger(),
		RefreshManager:   manager,
		RefreshMgrCancel: cancel,
	}

	if stopError := s.StopRefreshServer(); stopError != nil {
		t.Fatalf("StopRefreshServer returned error: %v", stopError)
	}

	if s.RefreshMgrCancel != nil {
		t.Fatal(
			"expected refreshMgrCancel to be cleared after StopRefreshServer",
		)
	}

	select {
	case <-managerRunDone:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for refresh manager shutdown")
	}
}

func TestStartAndStopRefreshServer(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	port, err := s.StartRefreshServer(0)
	if err != nil {
		t.Fatalf("startRefreshServer returned error: %v", err)
	}
	defer s.StopRefreshServer()

	url := "http://localhost:" + strconv.Itoa(
		port,
	) + "/get-refresh-script-inner"
	ready := s.WaitForAnyReady([]string{url})
	if !ready {
		t.Fatalf("refresh server endpoint did not become ready: %s", url)
	}

	if err := s.StopRefreshServer(); err != nil {
		t.Fatalf("stopRefreshServer returned error: %v", err)
	}
	if s.RefreshServer != nil {
		t.Fatal("expected refreshServer to be nil after stopRefreshServer")
	}
}

func TestStartRefreshServer_RefreshEndpointIsReachableOnLocalhostAndIPv4Loopback(
	t *testing.T,
) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	port, err := s.StartRefreshServer(0)
	if err != nil {
		t.Fatalf("startRefreshServer returned error: %v", err)
	}
	defer s.StopRefreshServer()

	if port <= 0 {
		t.Fatalf("expected refresh server port > 0, got %d", port)
	}
	if s.RefreshServer == nil {
		t.Fatal("expected refresh server to be initialized")
	}

	refreshScriptURLOnLocalhost := "http://localhost:" + strconv.Itoa(
		port,
	) + "/get-refresh-script-inner"
	if !s.WaitForAnyReady([]string{refreshScriptURLOnLocalhost}) {
		t.Fatalf(
			"expected refresh endpoint to be reachable via localhost: %s",
			refreshScriptURLOnLocalhost,
		)
	}
	refreshScriptURLOnIPv4Loopback := "http://127.0.0.1:" + strconv.Itoa(
		port,
	) + "/get-refresh-script-inner"
	if !s.WaitForAnyReady([]string{refreshScriptURLOnIPv4Loopback}) {
		t.Fatalf(
			"expected refresh endpoint to be reachable via IPv4 loopback: %s",
			refreshScriptURLOnIPv4Loopback,
		)
	}
}

func TestStartRefreshServer_NoOpInServerOnlyMode(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	port, err := s.StartRefreshServer(9999)
	if err != nil {
		t.Fatalf(
			"startRefreshServer returned error in server-only Mode: %v",
			err,
		)
	}
	if port != 0 {
		t.Fatalf("expected no port in server-only mode, got %d", port)
	}
	if s.RefreshServer != nil {
		t.Fatal("expected no refresh server in server-only mode")
	}
}

func TestWaitForVite_UsesViteClientEndpoint(t *testing.T) {
	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/@vite/client" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}),
	)
	defer testServer.Close()

	parsedURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("failed parsing test server URL: %v", err)
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		t.Fatalf("failed parsing test server port: %v", err)
	}

	s := &Server{
		Log: newDiscardLogger(),
		ViteContext: vitecmd.NewBuildCtx(
			&vitecmd.BuildCtxOptions{DefaultPort: port},
		),
	}

	if !s.WaitForVite() {
		t.Fatal(
			"expected waitForVite to return true when /@vite/client is ready",
		)
	}
}

func TestStartAppAndStopApp_WithExecutableBinary(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	if err := builder.SetupDistDir(cfg); err != nil {
		t.Fatalf("SetupDistDir returned error: %v", err)
	}

	binPath := cfg.Dist.Binary()
	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(binPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed writing executable test binary: %v", err)
	}

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	if startAppError := s.StartApp(); startAppError != nil {
		t.Fatalf("startApp returned error: %v", startAppError)
	}
	if s.AppCommand == nil || s.AppCommand.Process == nil {
		t.Fatal("expected startApp to launch process")
	}

	if err := s.StopApp(); err != nil {
		t.Fatalf("stopApp returned error: %v", err)
	}
}

func TestStartApp_FailureLeavesAppCmdNil(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	startAppError := s.StartApp()
	if startAppError == nil {
		t.Fatal("expected startApp to fail for missing binary")
	}
	if s.AppCommand != nil {
		t.Fatal("expected startApp failure to leave appCmd nil")
	}
}

func TestStartAppOrQueueNoGoRestart_QueuesRestartOnStartFailure(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
		RestartIntents: restartengine.NewRestartIntentAccumulator(
			make(chan restartengine.RestartRequest, 1),
		),
	}

	s.startAppOrQueueNoGoRestart()

	pendingRestartRequest, hasPendingRestartRequest := s.ConsumePendingRestartRequest()
	if !hasPendingRestartRequest {
		t.Fatal("expected no-go restart request after start-app failure")
	}
	if pendingRestartRequest.RecompileGo || pendingRestartRequest.IsConfigRestart {
		t.Fatalf(
			"expected no-go non-config restart request, got %#v",
			pendingRestartRequest,
		)
	}
	if s.AppCommand != nil {
		t.Fatal("expected start-app failure helper to leave app command nil")
	}
}

func TestStartRunCycleRuntime_ReturnsErrorWhenStartAppFails(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true
	cfg.Vite = nil

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	startRuntimeError := s.startRunCycleRuntime()
	if startRuntimeError == nil {
		t.Fatal("expected startRunCycleRuntime to fail when app binary is missing")
	}
}

func TestStartViteAndStopVite_NoOpWhenViteDisabled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Vite = nil

	builder := builder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &Server{
		Cfg:     cfg,
		Log:     newDiscardLogger(),
		Builder: builder,
	}

	if err := s.StartVite(); err != nil {
		t.Fatalf("startVite returned error with Vite disabled: %v", err)
	}
	if s.ViteContext != nil {
		t.Fatalf(
			"expected no vite context when Vite is disabled, got %#v",
			s.ViteContext,
		)
	}

	if err := s.StopVite(); err != nil {
		t.Fatalf("stopVite returned error with Vite disabled: %v", err)
	}
}

func TestStartViteAndStopVite_WithViteEnabled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = "echo"
	cfg.Vite.DefaultPort = 5201

	builder := builder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &Server{
		Cfg:     cfg,
		Log:     newDiscardLogger(),
		Builder: builder,
	}

	if err := s.StartVite(); err != nil {
		t.Fatalf("startVite returned error: %v", err)
	}
	if s.ViteContext == nil {
		t.Fatal("expected startVite to set viteCtx when Vite is enabled")
	}

	if err := s.StopVite(); err != nil {
		t.Fatalf("stopVite returned error: %v", err)
	}
	if s.ViteContext != nil {
		t.Fatal("expected stopVite to clear viteCtx")
	}
}

func TestCycleVite_NoOpWhenViteDisabled(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Vite = nil

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	s.CycleVite()
}

func TestCycleVite_NoOpWhenViteEnabledButNotStarted(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = "echo"
	cfg.Vite.DefaultPort = 5202

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	s.CycleVite()
}

func TestCycleVite_StartFailureAfterStopLeavesViteContextCleared(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = "command_that_does_not_exist_for_wave_cycle_test"
	cfg.Vite.DefaultPort = 5203

	builder := builder.NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &Server{
		Cfg:     cfg,
		Log:     newDiscardLogger(),
		Builder: builder,
		ViteContext: vitecmd.NewBuildCtx(
			&vitecmd.BuildCtxOptions{DefaultPort: cfg.Vite.DefaultPort},
		),
	}

	s.CycleVite()

	if s.ViteContext != nil {
		t.Fatal(
			"expected cycleVite start failure to leave vite context cleared",
		)
	}
}

func TestStartRefreshServer_ReturnsErrorForInvalidPort(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	if _, err := s.StartRefreshServer(-1); err == nil {
		t.Fatal(
			"expected startRefreshServer to return an error for invalid negative port",
		)
	}
}

func TestStartRefreshServer_EventsEndpointSetsCORSAndRejectsNonWebSocket(
	t *testing.T,
) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	s := &Server{
		Cfg: cfg,
		Log: newDiscardLogger(),
	}

	port, err := s.StartRefreshServer(0)
	if err != nil {
		t.Fatalf("startRefreshServer returned error: %v", err)
	}
	defer s.StopRefreshServer()

	resp, err := http.Get("http://localhost:" + strconv.Itoa(port) + "/events")
	if err != nil {
		t.Fatalf("events endpoint request failed: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin header '*', got %q", got)
	}
	if resp.StatusCode < http.StatusBadRequest {
		t.Fatalf(
			"expected non-websocket request to be rejected, got status %d",
			resp.StatusCode,
		)
	}
}
