package devserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavebuild/builder"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/restartengine"
	"github.com/vormadev/vorma/wave/wavedev/internal/broadcast"
)

const testViteHelperProcessEnv = "WAVE_TOOLING_3_TEST_VITE_HELPER_PROCESS"

// TestWaveToolingViteHelperProcess runs in a subprocess to emulate a Vite dev server.
func TestWaveToolingViteHelperProcess(t *testing.T) {
	if os.Getenv(testViteHelperProcessEnv) != "1" {
		return
	}

	if err := runWaveToolingViteHelperProcess(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestCycleVite_RestartsRunningViteProcess(t *testing.T) {
	t.Setenv(testViteHelperProcessEnv, "1")

	cfg := newParsedConfigForRunloopOrchestrationTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = helperViteBaseCommand(t)
	cfg.Vite.DefaultPort = 5199

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopOrchestrationTests(),
	)
	defer builderForTest.Close()

	serverForTest := &Server{
		Cfg:     cfg,
		Log:     newDiscardLoggerForRunloopOrchestrationTests(),
		Builder: builderForTest,
	}

	if startViteError := serverForTest.StartVite(); startViteError != nil {
		t.Fatalf("StartVite returned error: %v", startViteError)
	}
	t.Cleanup(func() {
		_ = serverForTest.StopVite()
	})

	if !serverForTest.WaitForVite() {
		t.Fatal("expected first vite process to become ready")
	}
	vitePort := serverForTest.ViteContext.Port()
	firstProcessID := fetchViteProcessID(t, vitePort)

	serverForTest.CycleVite()

	if serverForTest.ViteContext == nil {
		t.Fatal("expected vite context to remain available after cycle")
	}
	if !serverForTest.WaitForVite() {
		t.Fatal("expected cycled vite process to become ready")
	}
	secondProcessID := fetchViteProcessID(t, serverForTest.ViteContext.Port())

	if firstProcessID == secondProcessID {
		t.Fatalf(
			"expected CycleVite to restart process, got same pid %q",
			firstProcessID,
		)
	}
}

func TestBroadcastReload_WithCycleVite_RestartsThenBroadcastsOnce(
	t *testing.T,
) {
	t.Setenv(testViteHelperProcessEnv, "1")

	cfg := newParsedConfigForRunloopOrchestrationTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = helperViteBaseCommand(t)
	cfg.Vite.DefaultPort = 5199

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForRunloopOrchestrationTests(),
	)
	defer builderForTest.Close()

	serverForTest := &Server{
		Cfg:     cfg,
		Log:     newDiscardLoggerForRunloopOrchestrationTests(),
		Builder: builderForTest,
	}
	refreshManager, connection, cleanup := setupRefreshWebsocketForRunloopOrchestrationTests(
		t,
	)
	defer cleanup()
	serverForTest.RefreshManager = refreshManager

	if startViteError := serverForTest.StartVite(); startViteError != nil {
		t.Fatalf("StartVite returned error: %v", startViteError)
	}
	t.Cleanup(func() {
		_ = serverForTest.StopVite()
	})

	if !serverForTest.WaitForVite() {
		t.Fatal("expected first vite process to become ready")
	}
	vitePort := serverForTest.ViteContext.Port()
	firstProcessID := fetchViteProcessID(t, vitePort)

	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
		CycleVite: true,
	})

	connection.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var firstMessage broadcast.Payload
	if readError := connection.ReadJSON(&firstMessage); readError == nil {
		t.Fatalf(
			"expected cycleVite reload to skip direct broadcast payload, got %#v",
			firstMessage,
		)
	}

	secondProcessID := waitForChangedViteProcessID(
		t,
		vitePort,
		firstProcessID,
	)
	if firstProcessID == secondProcessID {
		t.Fatalf(
			"expected cycleVite reload to restart vite process, got same pid %q",
			firstProcessID,
		)
	}

	connection.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var secondMessage broadcast.Payload
	if readError := connection.ReadJSON(&secondMessage); readError == nil {
		t.Fatalf(
			"expected no direct broadcast payload after cycleVite reload, got %#v",
			secondMessage,
		)
	}
}

func TestBroadcastReload_WaitsForAppAndViteBeforeBroadcast(t *testing.T) {
	cfg := newParsedConfigForRunloopOrchestrationTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	appPort := mustConfigureAndGetWaveAppPortForRunloopTests(t)
	appListener, listenError := net.Listen(
		"tcp",
		fmt.Sprintf("127.0.0.1:%d", appPort),
	)
	if listenError != nil {
		t.Skipf(
			"unable to bind app port %d for wait-for-app test: %v",
			appPort,
			listenError,
		)
	}
	defer appListener.Close()

	var appHits atomic.Int32
	appServer := &http.Server{
		Handler: http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/healthz" {
					appHits.Add(1)
					responseWriter.WriteHeader(http.StatusOK)
					return
				}
				responseWriter.WriteHeader(http.StatusNotFound)
			},
		),
	}
	defer appServer.Close()
	go func() {
		_ = appServer.Serve(appListener)
	}()

	var viteHits atomic.Int32
	viteServer := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/@vite/client" {
					viteHits.Add(1)
					responseWriter.WriteHeader(http.StatusOK)
					return
				}
				responseWriter.WriteHeader(http.StatusNotFound)
			},
		),
	)
	defer viteServer.Close()

	vitePort := mustPortFromURL(t, viteServer.URL)

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLoggerForRunloopOrchestrationTests(),
		ViteContext: vitecmd.NewBuildCtx(
			&vitecmd.BuildCtxOptions{DefaultPort: vitePort},
		),
	}
	refreshManager, connection, cleanup := setupRefreshWebsocketForRunloopOrchestrationTests(
		t,
	)
	defer cleanup()
	serverForTest.RefreshManager = refreshManager

	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload:  broadcast.Payload{ChangeType: broadcast.ChangeTypeRevalidate},
		WaitApp:  true,
		WaitVite: true,
	})

	connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	var message broadcast.Payload
	if readError := connection.ReadJSON(&message); readError != nil {
		t.Fatal("timed out waiting for waitApp+waitVite broadcast")
	}
	if message.ChangeType != broadcast.ChangeTypeRevalidate {
		t.Fatalf("unexpected broadcast payload: %#v", message)
	}

	if appHits.Load() == 0 {
		t.Fatal(
			"expected broadcastReload(waitApp=true) to probe app health endpoint",
		)
	}
	if viteHits.Load() == 0 {
		t.Fatal(
			"expected broadcastReload(waitVite=true) to probe vite endpoint",
		)
	}
}

func TestStartRefreshServer_FallsBackWhenPreferredPortIsUnavailable(
	t *testing.T,
) {
	cfg := newParsedConfigForRunloopOrchestrationTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	occupiedListener, listenError := net.Listen("tcp", "127.0.0.1:0")
	if listenError != nil {
		t.Skipf(
			"unable to reserve preferred port for fallback test: %v",
			listenError,
		)
	}
	defer occupiedListener.Close()
	occupiedPort := occupiedListener.Addr().(*net.TCPAddr).Port

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLoggerForRunloopOrchestrationTests(),
	}

	actualPort, startRefreshServerError := serverForTest.StartRefreshServer(
		occupiedPort,
	)
	if startRefreshServerError != nil {
		t.Fatalf(
			"StartRefreshServer returned error: %v",
			startRefreshServerError,
		)
	}
	defer func() {
		_ = serverForTest.StopRefreshServer()
	}()

	if actualPort <= 0 {
		t.Fatalf("expected positive fallback port, got %d", actualPort)
	}
	if actualPort == occupiedPort {
		t.Fatalf(
			"expected fallback port to differ from occupied port %d",
			occupiedPort,
		)
	}

	refreshScriptURL := fmt.Sprintf(
		"http://localhost:%d/get-refresh-script-inner",
		actualPort,
	)
	if !serverForTest.WaitForAnyReady([]string{refreshScriptURL}) {
		t.Fatalf(
			"refresh server did not become ready on fallback port: %s",
			refreshScriptURL,
		)
	}
}

func TestStartRefreshServer_NoOpWhenServerOnlyMode(t *testing.T) {
	cfg := newParsedConfigForRunloopOrchestrationTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	t.Setenv("__WAVE_REFRESH_SERVER_PORT", "")

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLoggerForRunloopOrchestrationTests(),
	}

	actualPort, startRefreshServerError := serverForTest.StartRefreshServer(
		5173,
	)
	if startRefreshServerError != nil {
		t.Fatalf(
			"StartRefreshServer returned error in server-only mode: %v",
			startRefreshServerError,
		)
	}
	if actualPort != 0 {
		t.Fatalf(
			"expected no refresh server port in server-only mode, got %d",
			actualPort,
		)
	}
	if serverForTest.RefreshServer != nil {
		t.Fatalf(
			"expected no refresh server instance in server-only mode, got %#v",
			serverForTest.RefreshServer,
		)
	}
	if refreshServerPort := os.Getenv("__WAVE_REFRESH_SERVER_PORT"); refreshServerPort != "" {
		t.Fatalf(
			"expected refresh server env port to remain unset in server-only mode, got %q",
			refreshServerPort,
		)
	}
}

func TestServerRun_BrowserModeInitWatcherFailureCleansRefreshResources(
	t *testing.T,
) {
	mustConfigureAndGetWaveAppPortForRunloopTests(t)

	root := t.TempDir()
	cfg := newParsedConfigForRunloopOrchestrationTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.WatchRoot = filepath.Join(root, "missing-watch-root")

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLoggerForRunloopOrchestrationTests(),
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

	if serverForTest.RefreshServer != nil {
		t.Fatal("expected refresh server to be cleaned up on Run() exit")
	}
	if serverForTest.RefreshMgrCancel != nil {
		t.Fatal(
			"expected refresh manager cancel func to be cleared on Run() exit",
		)
	}
}

func runWaveToolingViteHelperProcess() error {
	port, parsePortError := parseVitePortFromArgs(os.Args)
	if parsePortError != nil {
		return parsePortError
	}

	mux := http.NewServeMux()
	mux.HandleFunc(
		"/@vite/client",
		func(responseWriter http.ResponseWriter, _ *http.Request) {
			responseWriter.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(responseWriter, strconv.Itoa(os.Getpid()))
		},
	)

	listener, listenError := net.Listen(
		"tcp",
		fmt.Sprintf("127.0.0.1:%d", port),
	)
	if listenError != nil {
		return listenError
	}

	serverForTest := &http.Server{Handler: mux}
	serveErrorChannel := make(chan error, 1)
	go func() {
		serveErrorChannel <- serverForTest.Serve(listener)
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, helperTerminationSignals()...)
	defer signal.Stop(signals)

	select {
	case serveError := <-serveErrorChannel:
		if serveError != nil && !errors.Is(serveError, http.ErrServerClosed) {
			return serveError
		}
		return nil
	case <-signals:
		shutdownContext, cancelShutdown := context.WithTimeout(
			context.Background(),
			2*time.Second,
		)
		defer cancelShutdown()
		_ = serverForTest.Shutdown(shutdownContext)
		serveError := <-serveErrorChannel
		if serveError != nil && !errors.Is(serveError, http.ErrServerClosed) {
			return serveError
		}
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf(
			"vite helper process timed out without termination signal",
		)
	}
}

func parseVitePortFromArgs(args []string) (int, error) {
	for argumentIndex := 0; argumentIndex < len(args)-1; argumentIndex++ {
		if args[argumentIndex] == "--port" {
			port, atoiError := strconv.Atoi(args[argumentIndex+1])
			if atoiError != nil {
				return 0, fmt.Errorf(
					"invalid --port value %q: %w",
					args[argumentIndex+1],
					atoiError,
				)
			}
			return port, nil
		}
	}
	return 0, fmt.Errorf("missing --port argument")
}

func helperTerminationSignals() []os.Signal {
	if runtime.GOOS == "windows" {
		return []os.Signal{os.Interrupt}
	}
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

func helperViteBaseCommand(t *testing.T) string {
	t.Helper()

	executablePath, executableError := os.Executable()
	if executableError != nil {
		t.Fatalf("os.Executable() error: %v", executableError)
	}

	if strings.ContainsAny(executablePath, " \t\n") {
		t.Skip(
			"test binary path contains whitespace; strings.Fields parser cannot represent this safely",
		)
	}

	return executablePath + " -test.run=^TestWaveToolingViteHelperProcess$ --"
}

func fetchViteProcessID(t *testing.T, port int) string {
	t.Helper()

	viteClientURL := fmt.Sprintf("http://localhost:%d/@vite/client", port)
	client := &http.Client{Timeout: 300 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		response, getError := client.Get(viteClientURL)
		if getError == nil {
			bodyBytes, readError := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if readError == nil && response.StatusCode == http.StatusOK {
				processID := strings.TrimSpace(string(bodyBytes))
				if processID != "" {
					return processID
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("timed out reading vite helper pid from %s", viteClientURL)
	return ""
}

func waitForChangedViteProcessID(
	t *testing.T,
	port int,
	previousProcessID string,
) string {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		currentProcessID := fetchViteProcessID(t, port)
		if currentProcessID != previousProcessID {
			return currentProcessID
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf(
		"timed out waiting for vite helper pid change from %q on port %d",
		previousProcessID,
		port,
	)
	return ""
}

func mustPortFromURL(t *testing.T, rawURL string) int {
	t.Helper()

	parsedURL, parseError := url.Parse(rawURL)
	if parseError != nil {
		t.Fatalf("failed parsing URL %q: %v", rawURL, parseError)
	}

	port, atoiError := strconv.Atoi(parsedURL.Port())
	if atoiError != nil {
		t.Fatalf("failed parsing URL port %q: %v", parsedURL.Port(), atoiError)
	}
	return port
}

func newDiscardLoggerForRunloopOrchestrationTests() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTCP4HTTPTestServerForRunloopOrchestrationTests(
	t *testing.T,
	handler http.Handler,
) *httptest.Server {
	t.Helper()

	listener, listenError := net.Listen("tcp4", "127.0.0.1:0")
	if listenError != nil {
		t.Fatalf("create tcp4 listener: %v", listenError)
	}

	serverForTest := httptest.NewUnstartedServer(handler)
	serverForTest.Listener = listener
	serverForTest.Start()
	return serverForTest
}

func setupRefreshWebsocketForRunloopOrchestrationTests(
	t *testing.T,
) (
	*broadcast.Manager,
	*websocket.Conn,
	func(),
) {
	t.Helper()

	refreshManager := broadcast.NewManager(
		newDiscardLoggerForRunloopOrchestrationTests(),
		broadcast.ManagerConfig{},
	)
	refreshManagerContext, cancelRefreshManager := context.WithCancel(
		context.Background(),
	)
	go refreshManager.Run(refreshManagerContext)

	refreshHTTPServer := newTCP4HTTPTestServerForRunloopOrchestrationTests(
		t,
		refreshManager,
	)
	websocketURL := "ws" + strings.TrimPrefix(refreshHTTPServer.URL, "http")
	connection, _, dialError := websocket.DefaultDialer.Dial(websocketURL, nil)
	if dialError != nil {
		cancelRefreshManager()
		refreshHTTPServer.Close()
		t.Fatalf("dial websocket handler: %v", dialError)
	}

	cleanup := func() {
		_ = connection.Close()
		refreshHTTPServer.Close()
		cancelRefreshManager()
	}
	return refreshManager, connection, cleanup
}

func newParsedConfigForRunloopOrchestrationTestsAtRoot(
	root string,
) *wave.ParsedConfig {
	return wavetest.NewParsedConfigAtRoot(root)
}

func mustConfigureAndGetWaveAppPortForRunloopTests(t *testing.T) int {
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
	t.Setenv("PORT", strconv.Itoa(port))

	return port
}
