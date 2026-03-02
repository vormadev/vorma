package devserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveframework"
	"github.com/vormadev/vorma/wave/wavewatch"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/buildtime/internal/broadcast"
)

type siteRegressionPathsForToolingTests struct {
	AppRoot                string
	WorkspaceRoot          string
	ConfigFilePath         string
	CriticalCSSEntryPath   string
	NormalCSSEntryPath     string
	CriticalCSSImportPath  string
	NormalCSSImportPath    string
	TailwindCSSPath        string
	RouteRegistryPath      string
	TemplatePath           string
	MarkdownPath           string
	AdditionalMarkdownPath string
	RandomFrontendTSPath   string
	PublicStaticPath       string
	PrivateStaticPath      string
	GoSourcePath           string
}

type siteRegressionHarnessForToolingTests struct {
	Paths      siteRegressionPathsForToolingTests
	Server     *Server
	Connection *websocket.Conn
	Cleanup    func()
	LogBuffer  *bytes.Buffer
}

func setupSiteRegressionHarnessForToolingTests(
	t *testing.T,
	captureLogs bool,
) siteRegressionHarnessForToolingTests {
	return setupSiteRegressionHarnessForToolingTestsWithOptions(
		t,
		captureLogs,
		true,
	)
}

func setupSiteRegressionHarnessWithoutMarkdownIncludeForToolingTests(
	t *testing.T,
	captureLogs bool,
) siteRegressionHarnessForToolingTests {
	return setupSiteRegressionHarnessForToolingTestsWithOptions(
		t,
		captureLogs,
		false,
	)
}

func setupSiteRegressionHarnessForToolingTestsWithOptions(
	t *testing.T,
	captureLogs bool,
	includeMarkdownRevalidateWatchOverride bool,
) siteRegressionHarnessForToolingTests {
	return setupSiteRegressionHarnessForToolingTestsWithRuntimeOptions(
		t,
		captureLogs,
		includeMarkdownRevalidateWatchOverride,
		true,
	)
}

func setupSiteRegressionHarnessWithoutAppHealthForToolingTests(
	t *testing.T,
	captureLogs bool,
) siteRegressionHarnessForToolingTests {
	return setupSiteRegressionHarnessForToolingTestsWithRuntimeOptions(
		t,
		captureLogs,
		true,
		false,
	)
}

func setupSiteRegressionHarnessForToolingTestsWithRuntimeOptions(
	t *testing.T,
	captureLogs bool,
	includeMarkdownRevalidateWatchOverride bool,
	startAppHealthServer bool,
) siteRegressionHarnessForToolingTests {
	t.Helper()

	appPort := mustConfigureAndGetWaveAppPortForDevserverRunTests(t)
	if startAppHealthServer {
		appListener, listenError := net.Listen(
			"tcp",
			fmt.Sprintf("127.0.0.1:%d", appPort),
		)
		if listenError != nil {
			t.Fatalf("bind app health listener: %v", listenError)
		}
		appServer := &http.Server{
			Handler: http.HandlerFunc(func(
				responseWriter http.ResponseWriter,
				request *http.Request,
			) {
				if request.URL.Path == "/healthz" {
					responseWriter.WriteHeader(http.StatusOK)
					_, _ = responseWriter.Write([]byte("ok"))
					return
				}
				responseWriter.WriteHeader(http.StatusNotFound)
			}),
		}
		go func() {
			_ = appServer.Serve(appListener)
		}()
		t.Cleanup(func() {
			_ = appServer.Close()
			_ = appListener.Close()
		})
	}

	workspaceRoot := t.TempDir()
	appRoot := filepath.Join(workspaceRoot, "internal", "site")
	if mkdirError := os.MkdirAll(appRoot, 0o755); mkdirError != nil {
		t.Fatalf("create app root: %v", mkdirError)
	}

	originalWorkingDirectory, workingDirectoryError := os.Getwd()
	if workingDirectoryError != nil {
		t.Fatalf("read current working directory: %v", workingDirectoryError)
	}
	if chdirError := os.Chdir(appRoot); chdirError != nil {
		t.Fatalf("chdir into app root %q: %v", appRoot, chdirError)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})

	paths := siteRegressionPathsForToolingTests{
		AppRoot:        appRoot,
		WorkspaceRoot:  workspaceRoot,
		ConfigFilePath: filepath.Join(appRoot, "backend", "wave.config.json"),
		CriticalCSSEntryPath: filepath.Join(
			appRoot,
			"frontend",
			"src",
			"styles",
			"main.critical.css",
		),
		NormalCSSEntryPath: filepath.Join(
			appRoot,
			"frontend",
			"src",
			"styles",
			"main.css",
		),
		CriticalCSSImportPath: filepath.Join(
			appRoot,
			"frontend",
			"src",
			"styles",
			"critical_import.css",
		),
		NormalCSSImportPath: filepath.Join(
			appRoot,
			"frontend",
			"src",
			"styles",
			"fonts.css",
		),
		TailwindCSSPath: filepath.Join(
			appRoot,
			"frontend",
			"src",
			"styles",
			"tailwind.css",
		),
		RouteRegistryPath: filepath.Join(
			appRoot,
			"frontend",
			"src",
			"routes",
			"core.vorma.routes.ts",
		),
		TemplatePath: filepath.Join(
			appRoot,
			"backend",
			waveartifacts.AssetsDirname,
			"entry.go.html",
		),
		MarkdownPath: filepath.Join(
			appRoot,
			"backend",
			waveartifacts.AssetsDirname,
			"markdown",
			"blog",
			"post.md",
		),
		AdditionalMarkdownPath: filepath.Join(
			appRoot,
			"backend",
			waveartifacts.AssetsDirname,
			"markdown",
			"blog",
			"new-post.md",
		),
		RandomFrontendTSPath: filepath.Join(
			appRoot,
			"frontend",
			"src",
			"lib",
			"client_util.ts",
		),
		PublicStaticPath: filepath.Join(
			appRoot,
			"frontend",
			waveartifacts.AssetsDirname,
			"logo.svg",
		),
		PrivateStaticPath: filepath.Join(
			appRoot,
			"backend",
			waveartifacts.AssetsDirname,
			"notes.txt",
		),
		GoSourcePath: filepath.Join(
			appRoot,
			"backend",
			"handlers",
			"health.go",
		),
	}

	directoriesToCreate := []string{
		filepath.Dir(paths.ConfigFilePath),
		filepath.Dir(paths.CriticalCSSEntryPath),
		filepath.Dir(paths.NormalCSSEntryPath),
		filepath.Dir(paths.CriticalCSSImportPath),
		filepath.Dir(paths.NormalCSSImportPath),
		filepath.Dir(paths.TailwindCSSPath),
		filepath.Dir(paths.RouteRegistryPath),
		filepath.Dir(paths.TemplatePath),
		filepath.Dir(paths.MarkdownPath),
		filepath.Dir(paths.AdditionalMarkdownPath),
		filepath.Dir(paths.RandomFrontendTSPath),
		filepath.Dir(paths.PublicStaticPath),
		filepath.Dir(paths.PrivateStaticPath),
		filepath.Dir(paths.GoSourcePath),
	}
	for _, directoryPath := range directoriesToCreate {
		if mkdirError := os.MkdirAll(directoryPath, 0o755); mkdirError != nil {
			t.Fatalf("create directory %q: %v", directoryPath, mkdirError)
		}
	}

	filesToWrite := map[string]string{
		paths.CriticalCSSEntryPath:  "@import \"./critical_import.css\";\nbody { color: red; }",
		paths.NormalCSSEntryPath:    "@import \"./fonts.css\";\nbody { color: black; }",
		paths.CriticalCSSImportPath: ".critical-import { display: block; }",
		paths.NormalCSSImportPath:   ".font-face { font-family: Test; }",
		paths.TailwindCSSPath:       "@tailwind utilities;",
		paths.RouteRegistryPath:     "export const routes = [];",
		paths.TemplatePath:          "<!doctype html><html><body>{{.VormaBodyScripts}}</body></html>",
		paths.MarkdownPath:          "# Post\n\nHello",
		paths.RandomFrontendTSPath:  "export const clientUtil = () => 'ok';",
		paths.PublicStaticPath:      "<svg></svg>",
		paths.PrivateStaticPath:     "private note",
		paths.GoSourcePath:          "package handlers\n\nfunc Health() string { return \"ok\" }\n",
	}
	for filePath, fileContents := range filesToWrite {
		if writeError := os.WriteFile(filePath, []byte(fileContents), 0o644); writeError != nil {
			t.Fatalf("write file %q: %v", filePath, writeError)
		}
	}

	watchConfigPayload := map[string]any{
		"WatchRoot":           pathRelativeToCurrentWorkingDirectoryForToolingConfigJSON(t, workspaceRoot),
		"HealthcheckEndpoint": "/healthz",
	}
	if includeMarkdownRevalidateWatchOverride {
		watchConfigPayload["Include"] = []map[string]any{
			{
				"Pattern":                            "internal/site/backend/assets/markdown/**/*.md",
				"OnlyRunClientDefinedRevalidateFunc": true,
				"SkipRebuildingNotification":         true,
			},
		}
	}

	configPayload := map[string]any{
		"Core": map[string]any{
			"MainAppEntry": "backend/cmd/serve",
			"DistDir":      "backend/dist",
			"StaticAssetDirs": map[string]any{
				"Public":  "frontend/assets",
				"Private": "backend/assets",
			},
			"CSSEntryFiles": map[string]any{
				"Critical":    "frontend/src/styles/main.critical.css",
				"NonCritical": "frontend/src/styles/main.css",
			},
			"PublicPathPrefix": "/",
		},
		"Watch": watchConfigPayload,
	}
	configPayloadBytes, marshalError := json.Marshal(configPayload)
	if marshalError != nil {
		t.Fatalf("marshal config payload: %v", marshalError)
	}
	if writeError := os.WriteFile(
		paths.ConfigFilePath,
		configPayloadBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write config payload: %v", writeError)
	}

	parsedConfig, parseError := waveconfig.ParseConfigFile(paths.ConfigFilePath)
	if parseError != nil {
		t.Fatalf("parse config payload: %v", parseError)
	}
	frameworkReloadActionCallback := func(*wavewatch.HookContext) (*wavewatch.RefreshAction, error) {
		return &wavewatch.RefreshAction{
			ReloadBrowser: true,
			WaitForApp:    true,
			WaitForVite:   true,
		}, nil
	}
	waveframework.StateForConfig(parsedConfig).WatchPatterns = []wavewatch.WatchedFile{
		{
			Pattern:                    "internal/site/frontend/src/**/*vorma.routes.ts",
			RunOnChangeOnly:            true,
			SkipRebuildingNotification: true,
			OnChangeHooks: []wavewatch.OnChangeHook{
				{
					Timing:   wavewatch.OnChangeStrategyPost,
					Callback: frameworkReloadActionCallback,
				},
			},
		},
		{
			Pattern:                    "internal/site/backend/assets/entry.go.html",
			SkipRebuildingNotification: true,
			OnChangeHooks: []wavewatch.OnChangeHook{
				{
					Timing:   wavewatch.OnChangeStrategyPost,
					Callback: frameworkReloadActionCallback,
				},
			},
		},
	}
	for watchedFileIndex := range waveframework.StateForConfig(parsedConfig).WatchPatterns {
		waveframework.StateForConfig(parsedConfig).WatchPatterns[watchedFileIndex].Sort()
	}
	for watchedFileIndex := range parsedConfig.Watch.Include {
		parsedConfig.Watch.Include[watchedFileIndex].Sort()
	}

	serverForTest := setupProcessEventsServerForToolingTests(t, parsedConfig)
	if buildCSSError := serverForTest.Builder.BuildCSS(
		builder.CSSBuildOptions{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	); buildCSSError != nil {
		t.Fatalf("initial BuildCSS returned error: %v", buildCSSError)
	}

	var logBuffer *bytes.Buffer
	if captureLogs {
		buffer := &bytes.Buffer{}
		serverForTest.Log = slog.New(slog.NewTextHandler(buffer, nil))
		logBuffer = buffer
	}

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	serverForTest.RefreshManager = refreshManager

	return siteRegressionHarnessForToolingTests{
		Paths:      paths,
		Server:     serverForTest,
		Connection: connection,
		Cleanup:    cleanup,
		LogBuffer:  logBuffer,
	}
}

func assertNoRestartRequestForSiteRegressionHarness(
	t *testing.T,
	serverForTest *Server,
	contextMessage string,
) {
	t.Helper()

	if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
		serverForTest,
	); hasPendingRestartRequest {
		t.Fatalf(
			"%s: expected no restart request, got %#v",
			contextMessage,
			pendingRestartRequest,
		)
	}
}

func assertExpectedBroadcastPayloadForSiteRegressionHarness(
	t *testing.T,
	connection *websocket.Conn,
	expectedChangeType broadcast.ChangeType,
	contextMessage string,
) broadcast.Payload {
	t.Helper()

	connection.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"%s: expected broadcast payload: %v",
			contextMessage,
			readError,
		)
	}
	if receivedPayload.ChangeType != expectedChangeType {
		t.Fatalf(
			"%s: expected payload type %q, got %#v",
			contextMessage,
			expectedChangeType,
			receivedPayload,
		)
	}
	return receivedPayload
}

func assertNoBroadcastPayloadForSiteRegressionHarness(
	t *testing.T,
	connection *websocket.Conn,
	contextMessage string,
) {
	t.Helper()

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"%s: expected no broadcast payload, got %#v",
			contextMessage,
			receivedPayload,
		)
	}
}

func assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
	t *testing.T,
	connection *websocket.Conn,
	contextMessage string,
) {
	t.Helper()

	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		connection,
		broadcast.ChangeTypeRebuilding,
		contextMessage,
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		connection,
		broadcast.ChangeTypeOther,
		contextMessage,
	)
}

func TestSiteRegression_CriticalCSSEditHotReloadsWhenWatchRootIsAncestor(
	t *testing.T,
) {
	workspaceRoot := t.TempDir()
	appRoot := filepath.Join(workspaceRoot, "internal", "site")
	if mkdirError := os.MkdirAll(appRoot, 0o755); mkdirError != nil {
		t.Fatalf("create app root: %v", mkdirError)
	}

	originalWorkingDirectory, workingDirectoryError := os.Getwd()
	if workingDirectoryError != nil {
		t.Fatalf("read current working directory: %v", workingDirectoryError)
	}
	if chdirError := os.Chdir(appRoot); chdirError != nil {
		t.Fatalf("chdir into app root %q: %v", appRoot, chdirError)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})

	criticalCSSEntryPath := filepath.Join(
		appRoot,
		"frontend",
		"src",
		"styles",
		"main.critical.css",
	)
	if mkdirError := os.MkdirAll(filepath.Dir(criticalCSSEntryPath), 0o755); mkdirError != nil {
		t.Fatalf("create critical css directory: %v", mkdirError)
	}
	if writeError := os.WriteFile(
		criticalCSSEntryPath,
		[]byte("body { color: red; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("write initial critical css entry: %v", writeError)
	}

	configFilePath := filepath.Join(appRoot, "backend", "wave.config.json")
	if mkdirError := os.MkdirAll(filepath.Dir(configFilePath), 0o755); mkdirError != nil {
		t.Fatalf("create config directory: %v", mkdirError)
	}
	configPayload := map[string]any{
		"Core": map[string]any{
			"MainAppEntry": "backend/cmd/serve",
			"DistDir":      "backend/dist",
			"StaticAssetDirs": map[string]any{
				"Public":  "frontend/assets",
				"Private": "backend/assets",
			},
			"CSSEntryFiles": map[string]any{
				"Critical": "frontend/src/styles/main.critical.css",
			},
			"PublicPathPrefix": "/",
		},
		"Watch": map[string]any{
			"WatchRoot": pathRelativeToCurrentWorkingDirectoryForToolingConfigJSON(
				t,
				workspaceRoot,
			),
		},
	}
	configPayloadBytes, marshalError := json.Marshal(configPayload)
	if marshalError != nil {
		t.Fatalf("marshal config payload: %v", marshalError)
	}
	if writeError := os.WriteFile(
		configFilePath,
		configPayloadBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write config payload: %v", writeError)
	}

	parsedConfig, parseError := waveconfig.ParseConfigFile(configFilePath)
	if parseError != nil {
		t.Fatalf("parse config payload: %v", parseError)
	}

	serverForTest := setupProcessEventsServerForToolingTests(t, parsedConfig)
	if buildCSSError := serverForTest.Builder.BuildCSS(
		builder.CSSBuildOptions{
			BuildCriticalCSS: true,
		},
	); buildCSSError != nil {
		t.Fatalf("initial BuildCSS returned error: %v", buildCSSError)
	}

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()
	serverForTest.RefreshManager = refreshManager

	if writeError := os.WriteFile(
		criticalCSSEntryPath,
		[]byte("body { color: blue; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated critical css entry: %v", writeError)
	}

	serverForTest.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: criticalCSSEntryPath,
		Op:   fsnotify.Write,
	}})

	if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
		serverForTest,
	); hasPendingRestartRequest {
		t.Fatalf(
			"expected no restart request for critical css edit, got %#v",
			pendingRestartRequest,
		)
	}

	connection.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf("expected critical css broadcast payload: %v", readError)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeCriticalCSS {
		t.Fatalf("expected critical css payload, got %#v", receivedPayload)
	}
}

func TestSiteRegression_NormalCSSEditHotReloadsWhenWatchRootIsAncestor(
	t *testing.T,
) {
	workspaceRoot := t.TempDir()
	appRoot := filepath.Join(workspaceRoot, "internal", "site")
	if mkdirError := os.MkdirAll(appRoot, 0o755); mkdirError != nil {
		t.Fatalf("create app root: %v", mkdirError)
	}

	originalWorkingDirectory, workingDirectoryError := os.Getwd()
	if workingDirectoryError != nil {
		t.Fatalf("read current working directory: %v", workingDirectoryError)
	}
	if chdirError := os.Chdir(appRoot); chdirError != nil {
		t.Fatalf("chdir into app root %q: %v", appRoot, chdirError)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})

	normalCSSEntryPath := filepath.Join(
		appRoot,
		"frontend",
		"src",
		"styles",
		"main.css",
	)
	if mkdirError := os.MkdirAll(filepath.Dir(normalCSSEntryPath), 0o755); mkdirError != nil {
		t.Fatalf("create normal css directory: %v", mkdirError)
	}
	if writeError := os.WriteFile(
		normalCSSEntryPath,
		[]byte("body { color: red; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("write initial normal css entry: %v", writeError)
	}

	configFilePath := filepath.Join(appRoot, "backend", "wave.config.json")
	if mkdirError := os.MkdirAll(filepath.Dir(configFilePath), 0o755); mkdirError != nil {
		t.Fatalf("create config directory: %v", mkdirError)
	}
	configPayload := map[string]any{
		"Core": map[string]any{
			"MainAppEntry": "backend/cmd/serve",
			"DistDir":      "backend/dist",
			"StaticAssetDirs": map[string]any{
				"Public":  "frontend/assets",
				"Private": "backend/assets",
			},
			"CSSEntryFiles": map[string]any{
				"NonCritical": "frontend/src/styles/main.css",
			},
			"PublicPathPrefix": "/",
		},
		"Watch": map[string]any{
			"WatchRoot": pathRelativeToCurrentWorkingDirectoryForToolingConfigJSON(
				t,
				workspaceRoot,
			),
		},
	}
	configPayloadBytes, marshalError := json.Marshal(configPayload)
	if marshalError != nil {
		t.Fatalf("marshal config payload: %v", marshalError)
	}
	if writeError := os.WriteFile(
		configFilePath,
		configPayloadBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write config payload: %v", writeError)
	}

	parsedConfig, parseError := waveconfig.ParseConfigFile(configFilePath)
	if parseError != nil {
		t.Fatalf("parse config payload: %v", parseError)
	}

	serverForTest := setupProcessEventsServerForToolingTests(t, parsedConfig)
	if buildCSSError := serverForTest.Builder.BuildCSS(
		builder.CSSBuildOptions{
			BuildNormalCSS: true,
		},
	); buildCSSError != nil {
		t.Fatalf("initial BuildCSS returned error: %v", buildCSSError)
	}

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()
	serverForTest.RefreshManager = refreshManager

	if writeError := os.WriteFile(
		normalCSSEntryPath,
		[]byte("body { color: blue; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated normal css entry: %v", writeError)
	}

	serverForTest.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: normalCSSEntryPath,
		Op:   fsnotify.Write,
	}})

	if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
		serverForTest,
	); hasPendingRestartRequest {
		t.Fatalf(
			"expected no restart request for normal css edit, got %#v",
			pendingRestartRequest,
		)
	}

	connection.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf("expected normal css broadcast payload: %v", readError)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeNormalCSS {
		t.Fatalf("expected normal css payload, got %#v", receivedPayload)
	}
	if receivedPayload.NormalCSSURL == "" {
		t.Fatalf(
			"expected non-empty normal css URL payload, got %#v",
			receivedPayload,
		)
	}
}

func TestSiteRegression_CriticalCSSImportedFileEditHotReloadsWhenWatchRootIsAncestor(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.CriticalCSSImportPath,
		[]byte(".critical-import { display: inline-block; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated critical css import file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.CriticalCSSImportPath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"critical css import write",
	)
	receivedPayload := assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeCriticalCSS,
		"critical css import write",
	)
	if strings.TrimSpace(receivedPayload.CriticalCSS) == "" {
		t.Fatalf(
			"expected critical css hot reload payload with base64 data, got %#v",
			receivedPayload,
		)
	}
}

func TestSiteRegression_NormalCSSImportedFileEditHotReloadsWhenWatchRootIsAncestor(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.NormalCSSImportPath,
		[]byte(".font-face { font-family: Updated; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated normal css import file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.NormalCSSImportPath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"normal css import write",
	)
	receivedPayload := assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeNormalCSS,
		"normal css import write",
	)
	if strings.TrimSpace(receivedPayload.NormalCSSURL) == "" {
		t.Fatalf(
			"expected normal css hot reload payload with URL, got %#v",
			receivedPayload,
		)
	}
}

func TestSiteRegression_PublicStaticWriteTriggersHardReloadWithoutRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.PublicStaticPath,
		[]byte("<svg><!--updated--></svg>"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated public static file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.PublicStaticPath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"public static write",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRebuilding,
		"public static write",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"public static write",
	)
}

func TestSiteRegression_PrivateStaticWriteTriggersHardReloadWithoutRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.PrivateStaticPath,
		[]byte("private note updated"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated private static file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.PrivateStaticPath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"private static write",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRebuilding,
		"private static write",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"private static write",
	)
}

func TestSiteRegression_PublicStaticCreateTriggersHardReloadWithoutRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	addedPublicStaticPath := filepath.Join(
		filepath.Dir(harness.Paths.PublicStaticPath),
		"added-logo.svg",
	)
	if writeError := os.WriteFile(
		addedPublicStaticPath,
		[]byte("<svg><!--added--></svg>"),
		0o644,
	); writeError != nil {
		t.Fatalf("write added public static file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: addedPublicStaticPath,
		Op:   fsnotify.Create,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"public static create",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"public static create",
	)
}

func TestSiteRegression_PublicStaticRemoveTriggersHardReloadWithoutRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if removeError := os.Remove(harness.Paths.PublicStaticPath); removeError != nil {
		t.Fatalf("remove public static file before event: %v", removeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.PublicStaticPath,
		Op:   fsnotify.Remove,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"public static remove",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"public static remove",
	)
}

func TestSiteRegression_PublicStaticRenameTriggersHardReloadWithoutRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	renamedPublicStaticPath := filepath.Join(
		filepath.Dir(harness.Paths.PublicStaticPath),
		"renamed-logo.svg",
	)
	if renameError := os.Rename(
		harness.Paths.PublicStaticPath,
		renamedPublicStaticPath,
	); renameError != nil {
		t.Fatalf("rename public static file before event: %v", renameError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.PublicStaticPath,
		Op:   fsnotify.Rename,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"public static rename",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"public static rename",
	)
}

func TestSiteRegression_PrivateStaticCreateTriggersHardReloadWithoutRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	addedPrivateStaticPath := filepath.Join(
		filepath.Dir(harness.Paths.PrivateStaticPath),
		"added-notes.txt",
	)
	if writeError := os.WriteFile(
		addedPrivateStaticPath,
		[]byte("private note added"),
		0o644,
	); writeError != nil {
		t.Fatalf("write added private static file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: addedPrivateStaticPath,
		Op:   fsnotify.Create,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"private static create",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"private static create",
	)
}

func TestSiteRegression_PrivateStaticRemoveTriggersHardReloadWithoutRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if removeError := os.Remove(harness.Paths.PrivateStaticPath); removeError != nil {
		t.Fatalf("remove private static file before event: %v", removeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.PrivateStaticPath,
		Op:   fsnotify.Remove,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"private static remove",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"private static remove",
	)
}

func TestSiteRegression_PrivateStaticRenameTriggersHardReloadWithoutRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	renamedPrivateStaticPath := filepath.Join(
		filepath.Dir(harness.Paths.PrivateStaticPath),
		"renamed-notes.txt",
	)
	if renameError := os.Rename(
		harness.Paths.PrivateStaticPath,
		renamedPrivateStaticPath,
	); renameError != nil {
		t.Fatalf("rename private static file before event: %v", renameError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.PrivateStaticPath,
		Op:   fsnotify.Rename,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"private static rename",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"private static rename",
	)
}

func TestSiteRegression_UnmanagedTailwindCSSEditIsNoOpWhenWatchRootIsAncestor(
	t *testing.T,
) {
	workspaceRoot := t.TempDir()
	appRoot := filepath.Join(workspaceRoot, "internal", "site")
	if mkdirError := os.MkdirAll(appRoot, 0o755); mkdirError != nil {
		t.Fatalf("create app root: %v", mkdirError)
	}

	originalWorkingDirectory, workingDirectoryError := os.Getwd()
	if workingDirectoryError != nil {
		t.Fatalf("read current working directory: %v", workingDirectoryError)
	}
	if chdirError := os.Chdir(appRoot); chdirError != nil {
		t.Fatalf("chdir into app root %q: %v", appRoot, chdirError)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})

	criticalCSSEntryPath := filepath.Join(
		appRoot,
		"frontend",
		"src",
		"styles",
		"main.critical.css",
	)
	normalCSSEntryPath := filepath.Join(
		appRoot,
		"frontend",
		"src",
		"styles",
		"main.css",
	)
	tailwindCSSPath := filepath.Join(
		appRoot,
		"frontend",
		"src",
		"styles",
		"tailwind.css",
	)
	for _, path := range []string{
		criticalCSSEntryPath,
		normalCSSEntryPath,
		tailwindCSSPath,
	} {
		if mkdirError := os.MkdirAll(filepath.Dir(path), 0o755); mkdirError != nil {
			t.Fatalf("create css directory for %q: %v", path, mkdirError)
		}
	}
	if writeError := os.WriteFile(
		criticalCSSEntryPath,
		[]byte("body { color: red; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("write critical css entry: %v", writeError)
	}
	if writeError := os.WriteFile(
		normalCSSEntryPath,
		[]byte("body { color: black; }"),
		0o644,
	); writeError != nil {
		t.Fatalf("write normal css entry: %v", writeError)
	}
	if writeError := os.WriteFile(
		tailwindCSSPath,
		[]byte("@tailwind utilities;"),
		0o644,
	); writeError != nil {
		t.Fatalf("write unmanaged tailwind css file: %v", writeError)
	}

	configFilePath := filepath.Join(appRoot, "backend", "wave.config.json")
	if mkdirError := os.MkdirAll(filepath.Dir(configFilePath), 0o755); mkdirError != nil {
		t.Fatalf("create config directory: %v", mkdirError)
	}
	configPayload := map[string]any{
		"Core": map[string]any{
			"MainAppEntry": "backend/cmd/serve",
			"DistDir":      "backend/dist",
			"StaticAssetDirs": map[string]any{
				"Public":  "frontend/assets",
				"Private": "backend/assets",
			},
			"CSSEntryFiles": map[string]any{
				"Critical":    "frontend/src/styles/main.critical.css",
				"NonCritical": "frontend/src/styles/main.css",
			},
			"PublicPathPrefix": "/",
		},
		"Watch": map[string]any{
			"WatchRoot": pathRelativeToCurrentWorkingDirectoryForToolingConfigJSON(
				t,
				workspaceRoot,
			),
		},
	}
	configPayloadBytes, marshalError := json.Marshal(configPayload)
	if marshalError != nil {
		t.Fatalf("marshal config payload: %v", marshalError)
	}
	if writeError := os.WriteFile(
		configFilePath,
		configPayloadBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write config payload: %v", writeError)
	}

	parsedConfig, parseError := waveconfig.ParseConfigFile(configFilePath)
	if parseError != nil {
		t.Fatalf("parse config payload: %v", parseError)
	}

	serverForTest := setupProcessEventsServerForToolingTests(t, parsedConfig)
	if buildCSSError := serverForTest.Builder.BuildCSS(
		builder.CSSBuildOptions{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	); buildCSSError != nil {
		t.Fatalf("initial BuildCSS returned error: %v", buildCSSError)
	}

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()
	serverForTest.RefreshManager = refreshManager

	if writeError := os.WriteFile(
		tailwindCSSPath,
		[]byte("@tailwind base;\n@tailwind components;\n@tailwind utilities;"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated unmanaged tailwind css file: %v", writeError)
	}

	serverForTest.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: tailwindCSSPath,
		Op:   fsnotify.Write,
	}})

	if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
		serverForTest,
	); hasPendingRestartRequest {
		t.Fatalf(
			"expected no restart request for unmanaged tailwind css edit, got %#v",
			pendingRestartRequest,
		)
	}

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"expected no browser payload for unmanaged tailwind css edit, got %#v",
			receivedPayload,
		)
	}
}

func TestSiteRegression_NoOpConfigWriteLogsAndSkipsRestartWhenWatchRootIsAncestor(
	t *testing.T,
) {
	workspaceRoot := t.TempDir()
	appRoot := filepath.Join(workspaceRoot, "internal", "site")
	if mkdirError := os.MkdirAll(appRoot, 0o755); mkdirError != nil {
		t.Fatalf("create app root: %v", mkdirError)
	}

	originalWorkingDirectory, workingDirectoryError := os.Getwd()
	if workingDirectoryError != nil {
		t.Fatalf("read current working directory: %v", workingDirectoryError)
	}
	if chdirError := os.Chdir(appRoot); chdirError != nil {
		t.Fatalf("chdir into app root %q: %v", appRoot, chdirError)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWorkingDirectory)
	})

	configFilePath := filepath.Join(appRoot, "backend", "wave.config.json")
	if mkdirError := os.MkdirAll(filepath.Dir(configFilePath), 0o755); mkdirError != nil {
		t.Fatalf("create config directory: %v", mkdirError)
	}
	configPayload := map[string]any{
		"Core": map[string]any{
			"MainAppEntry": "backend/cmd/serve",
			"DistDir":      "backend/dist",
			"StaticAssetDirs": map[string]any{
				"Public":  "frontend/assets",
				"Private": "backend/assets",
			},
		},
		"Watch": map[string]any{
			"WatchRoot": pathRelativeToCurrentWorkingDirectoryForToolingConfigJSON(
				t,
				workspaceRoot,
			),
		},
	}
	configPayloadBytes, marshalError := json.Marshal(configPayload)
	if marshalError != nil {
		t.Fatalf("marshal config payload: %v", marshalError)
	}
	if writeError := os.WriteFile(
		configFilePath,
		configPayloadBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write config payload: %v", writeError)
	}

	parsedConfig, parseError := waveconfig.ParseConfigFile(configFilePath)
	if parseError != nil {
		t.Fatalf("parse config payload: %v", parseError)
	}

	serverForTest := setupProcessEventsServerForToolingTests(t, parsedConfig)
	var logOutputBuffer bytes.Buffer
	serverForTest.Log = slog.New(slog.NewTextHandler(&logOutputBuffer, nil))

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()
	serverForTest.RefreshManager = refreshManager

	if writeError := os.WriteFile(
		configFilePath,
		configPayloadBytes,
		0o644,
	); writeError != nil {
		t.Fatalf("write unchanged config payload: %v", writeError)
	}

	serverForTest.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: filepath.Join(
			filepath.Dir(configFilePath),
			".",
			filepath.Base(configFilePath),
		),
		Op: fsnotify.Write,
	}})

	if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(
		serverForTest,
	); hasPendingRestartRequest {
		t.Fatalf(
			"expected no restart request for no-op config write, got %#v",
			pendingRestartRequest,
		)
	}

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"expected no browser payload for no-op config write, got %#v",
			receivedPayload,
		)
	}

	if !strings.Contains(
		logOutputBuffer.String(),
		"no changes to wave.config.json; skipping restart",
	) {
		t.Fatalf(
			"expected no-op config write log message, got logs: %s",
			logOutputBuffer.String(),
		)
	}
}

func TestSiteRegression_FrameworkRouteRegistryWriteTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.RouteRegistryPath,
		[]byte("export const routes = [{ path: '/docs' }];"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated route registry file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.RouteRegistryPath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"framework route-registry write",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"framework route-registry write",
	)
}

func TestSiteRegression_FrameworkRouteRegistryCreateTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	additionalRouteRegistryPath := filepath.Join(
		filepath.Dir(harness.Paths.RouteRegistryPath),
		"content.vorma.routes.ts",
	)
	if writeError := os.WriteFile(
		additionalRouteRegistryPath,
		[]byte("export const contentRoutes = [];"),
		0o644,
	); writeError != nil {
		t.Fatalf("write additional route registry file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: additionalRouteRegistryPath,
		Op:   fsnotify.Create,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"framework route-registry create",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"framework route-registry create",
	)
}

func TestSiteRegression_FrameworkRouteRegistryRenameTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	renamedRouteRegistryPath := filepath.Join(
		filepath.Dir(harness.Paths.RouteRegistryPath),
		"renamed.vorma.routes.ts",
	)
	if renameError := os.Rename(
		harness.Paths.RouteRegistryPath,
		renamedRouteRegistryPath,
	); renameError != nil {
		t.Fatalf("rename route registry file before event: %v", renameError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.RouteRegistryPath,
		Op:   fsnotify.Rename,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"framework route-registry rename",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"framework route-registry rename",
	)
}

func TestSiteRegression_FrameworkRouteRegistryRemoveTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if removeError := os.Remove(harness.Paths.RouteRegistryPath); removeError != nil {
		t.Fatalf("remove route registry file before event: %v", removeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.RouteRegistryPath,
		Op:   fsnotify.Remove,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"framework route-registry remove",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"framework route-registry remove",
	)
}

func TestSiteRegression_FrameworkTemplateWriteTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.TemplatePath,
		[]byte("<!doctype html><html><body><main>updated</main>{{.VormaBodyScripts}}</body></html>"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated template file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.TemplatePath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"framework template write",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"framework template write",
	)
}

func TestSiteRegression_FrameworkTemplateRenameTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	renamedTemplatePath := filepath.Join(
		filepath.Dir(harness.Paths.TemplatePath),
		"entry-renamed.go.html",
	)
	if renameError := os.Rename(
		harness.Paths.TemplatePath,
		renamedTemplatePath,
	); renameError != nil {
		t.Fatalf("rename template file before event: %v", renameError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.TemplatePath,
		Op:   fsnotify.Rename,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"framework template rename",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"framework template rename",
	)
}

func TestSiteRegression_FrameworkTemplateRemoveTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if removeError := os.Remove(harness.Paths.TemplatePath); removeError != nil {
		t.Fatalf("remove template file before event: %v", removeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.TemplatePath,
		Op:   fsnotify.Remove,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"framework template remove",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"framework template remove",
	)
}

func TestSiteRegression_MarkdownWriteWithoutWatchOverrideTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessWithoutMarkdownIncludeForToolingTests(
		t,
		false,
	)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.MarkdownPath,
		[]byte("# Post\n\nUpdated"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated markdown file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.MarkdownPath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"markdown write without watch override",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRebuilding,
		"markdown write without watch override",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeOther,
		"markdown write without watch override",
	)
}

func TestSiteRegression_MarkdownCreateWithoutWatchOverrideTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessWithoutMarkdownIncludeForToolingTests(
		t,
		false,
	)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.AdditionalMarkdownPath,
		[]byte("# New Post\n\nCreated"),
		0o644,
	); writeError != nil {
		t.Fatalf("write added markdown file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.AdditionalMarkdownPath,
		Op:   fsnotify.Create,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"markdown create without watch override",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"markdown create without watch override",
	)
}

func TestSiteRegression_MarkdownRemoveWithoutWatchOverrideTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessWithoutMarkdownIncludeForToolingTests(
		t,
		false,
	)
	defer harness.Cleanup()

	if removeError := os.Remove(harness.Paths.MarkdownPath); removeError != nil {
		t.Fatalf("remove markdown file before event: %v", removeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.MarkdownPath,
		Op:   fsnotify.Remove,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"markdown remove without watch override",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"markdown remove without watch override",
	)
}

func TestSiteRegression_MarkdownRenameWithoutWatchOverrideTriggersHardReload(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessWithoutMarkdownIncludeForToolingTests(
		t,
		false,
	)
	defer harness.Cleanup()

	renamedMarkdownPath := filepath.Join(
		filepath.Dir(harness.Paths.MarkdownPath),
		"renamed-post.md",
	)
	if renameError := os.Rename(
		harness.Paths.MarkdownPath,
		renamedMarkdownPath,
	); renameError != nil {
		t.Fatalf("rename markdown file before event: %v", renameError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.MarkdownPath,
		Op:   fsnotify.Rename,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"markdown rename without watch override",
	)
	assertRebuildingThenHardReloadBroadcastPayloadsForSiteRegressionHarness(
		t,
		harness.Connection,
		"markdown rename without watch override",
	)
}

func TestSiteRegression_MarkdownWriteTriggersRevalidate(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.MarkdownPath,
		[]byte("# Post\n\nUpdated"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated markdown file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.MarkdownPath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"markdown write",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRevalidate,
		"markdown write",
	)
}

func TestSiteRegression_MarkdownCreateTriggersRevalidate(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.AdditionalMarkdownPath,
		[]byte("# New Post\n\nCreated"),
		0o644,
	); writeError != nil {
		t.Fatalf("write new markdown file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.AdditionalMarkdownPath,
		Op:   fsnotify.Create,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"markdown create",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRevalidate,
		"markdown create",
	)
}

func TestSiteRegression_MarkdownRemoveTriggersRevalidate(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if removeError := os.Remove(harness.Paths.MarkdownPath); removeError != nil {
		t.Fatalf("remove markdown file before event: %v", removeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.MarkdownPath,
		Op:   fsnotify.Remove,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"markdown remove",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRevalidate,
		"markdown remove",
	)
}

func TestSiteRegression_MarkdownRenameTriggersRevalidate(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	renamedMarkdownPath := filepath.Join(
		filepath.Dir(harness.Paths.MarkdownPath),
		"renamed-post.md",
	)
	if renameError := os.Rename(
		harness.Paths.MarkdownPath,
		renamedMarkdownPath,
	); renameError != nil {
		t.Fatalf("rename markdown file before event: %v", renameError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.MarkdownPath,
		Op:   fsnotify.Rename,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"markdown rename",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRevalidate,
		"markdown rename",
	)
}

func TestSiteRegression_MarkdownPathAliasWriteTriggersRevalidate(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.MarkdownPath,
		[]byte("# Post\n\nAlias Updated"),
		0o644,
	); writeError != nil {
		t.Fatalf("write updated markdown file: %v", writeError)
	}

	aliasMarkdownPath := filepath.Join(
		filepath.Dir(harness.Paths.MarkdownPath),
		".",
		filepath.Base(harness.Paths.MarkdownPath),
	)
	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: aliasMarkdownPath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"markdown alias write",
	)
	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRevalidate,
		"markdown alias write",
	)
}

func TestSiteRegression_RandomFrontendTSWriteIsNoOpWhenUnwatched(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, false)
	defer harness.Cleanup()

	if writeError := os.WriteFile(
		harness.Paths.RandomFrontendTSPath,
		[]byte("export const clientUtil = () => 'updated';"),
		0o644,
	); writeError != nil {
		t.Fatalf("write random frontend TS file: %v", writeError)
	}

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.RandomFrontendTSPath,
		Op:   fsnotify.Write,
	}})

	assertNoRestartRequestForSiteRegressionHarness(
		t,
		harness.Server,
		"random frontend ts write",
	)
	assertNoBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		"random frontend ts write",
	)
}

func TestSiteRegression_ConfigSemanticWriteBroadcastsRebuildingAndQueuesConfigRestart(
	t *testing.T,
) {
	harness := setupSiteRegressionHarnessForToolingTests(t, true)
	defer harness.Cleanup()

	writeSemanticallyChangedConfigForToolingTests(
		t,
		harness.Paths.ConfigFilePath,
	)

	harness.Server.BuildRunloopEngine().ProcessEvents([]fsnotify.Event{{
		Name: harness.Paths.ConfigFilePath,
		Op:   fsnotify.Write,
	}})

	pendingRestartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		harness.Server,
		500*time.Millisecond,
	)
	if !pendingRestartRequest.IsConfigRestart ||
		!pendingRestartRequest.RecompileGo {
		t.Fatalf(
			"expected config restart with Go recompile, got %#v",
			pendingRestartRequest,
		)
	}

	assertExpectedBroadcastPayloadForSiteRegressionHarness(
		t,
		harness.Connection,
		broadcast.ChangeTypeRebuilding,
		"semantic config write",
	)
}
