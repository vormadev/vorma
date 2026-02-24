package devserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/internal/wavecore"
	"github.com/vormadev/vorma/wave/wavebuild/builder"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/eventpipeline"
	"github.com/vormadev/vorma/wave/wavedev/internal/broadcast"
)

const positiveBroadcastReadTimeoutForBehaviorTests = time.Second

func newDiscardLoggerForBroadcastBehaviorTests() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newParsedConfigForBroadcastBehaviorTestsAtRoot(
	root string,
) *wave.ParsedConfig {
	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: staticAssetDirsForTests{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		Watch: &wave.WatchConfig{WatchRoot: root},
	}
	cfg.Dist.Root = cfg.Core.DistDir
	return cfg
}

func newTCP4HTTPTestServerForBroadcastBehaviorTests(
	t *testing.T,
	handler http.Handler,
) *httptest.Server {
	t.Helper()

	listener, listenError := net.Listen("tcp4", "127.0.0.1:0")
	if listenError != nil {
		t.Fatalf("create tcp4 listener: %v", listenError)
	}

	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}

func startBlockingHealthServerForBroadcastBehaviorTests(
	t *testing.T,
) (
	int,
	chan struct{},
	chan struct{},
	func(),
) {
	t.Helper()

	requestStarted := make(chan struct{}, 1)
	requestCanceled := make(chan struct{}, 1)

	listener, listenError := net.Listen("tcp4", "127.0.0.1:0")
	if listenError != nil {
		t.Fatalf("create health server listener: %v", listenError)
	}

	healthServer := &http.Server{
		Handler: http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/healthz" {
					responseWriter.WriteHeader(http.StatusNotFound)
					return
				}
				select {
				case requestStarted <- struct{}{}:
				default:
				}
				<-request.Context().Done()
				select {
				case requestCanceled <- struct{}{}:
				default:
				}
			},
		),
	}

	go func() {
		_ = healthServer.Serve(listener)
	}()

	healthServerPort := listener.Addr().(*net.TCPAddr).Port
	cleanup := func() {
		shutdownContext, shutdownCancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer shutdownCancel()
		_ = healthServer.Shutdown(shutdownContext)
	}
	return healthServerPort, requestStarted, requestCanceled, cleanup
}

func startBlockingViteInvalidateServerForBroadcastBehaviorTests(
	t *testing.T,
) (
	int,
	chan struct{},
	chan struct{},
	func(),
) {
	t.Helper()

	requestStarted := make(chan struct{}, 1)
	requestCanceled := make(chan struct{}, 1)

	listener, listenError := net.Listen("tcp4", "127.0.0.1:0")
	if listenError != nil {
		t.Fatalf("create invalidate server listener: %v", listenError)
	}

	invalidateServer := &http.Server{
		Handler: http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/__vorma_invalidate_filemap" {
					responseWriter.WriteHeader(http.StatusNotFound)
					return
				}
				if request.Method != http.MethodPost {
					responseWriter.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				select {
				case requestStarted <- struct{}{}:
				default:
				}
				<-request.Context().Done()
				select {
				case requestCanceled <- struct{}{}:
				default:
				}
			},
		),
	}

	go func() {
		_ = invalidateServer.Serve(listener)
	}()

	invalidateServerPort := listener.Addr().(*net.TCPAddr).Port
	cleanup := func() {
		shutdownContext, shutdownCancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer shutdownCancel()
		_ = invalidateServer.Shutdown(shutdownContext)
	}
	return invalidateServerPort, requestStarted, requestCanceled, cleanup
}

func startBlockingFrameworkRuntimeReloadServerForBroadcastBehaviorTests(
	t *testing.T,
	endpointPath string,
) (
	int,
	chan struct{},
	chan struct{},
	func(),
) {
	t.Helper()

	requestStarted := make(chan struct{}, 1)
	requestCanceled := make(chan struct{}, 1)

	listener, listenError := net.Listen("tcp4", "127.0.0.1:0")
	if listenError != nil {
		t.Fatalf("create framework reload server listener: %v", listenError)
	}

	frameworkReloadServer := &http.Server{
		Handler: http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/healthz" {
					responseWriter.WriteHeader(http.StatusOK)
					return
				}
				if request.URL.Path != endpointPath {
					responseWriter.WriteHeader(http.StatusNotFound)
					return
				}
				if request.Method != http.MethodPost {
					responseWriter.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				select {
				case requestStarted <- struct{}{}:
				default:
				}
				<-request.Context().Done()
				select {
				case requestCanceled <- struct{}{}:
				default:
				}
			},
		),
	}

	go func() {
		_ = frameworkReloadServer.Serve(listener)
	}()

	frameworkReloadServerPort := listener.Addr().(*net.TCPAddr).Port
	cleanup := func() {
		shutdownContext, shutdownCancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer shutdownCancel()
		_ = frameworkReloadServer.Shutdown(shutdownContext)
	}
	return frameworkReloadServerPort, requestStarted, requestCanceled, cleanup
}

func TestBroadcastRebuilding_SendsPayloadWhenEnabled(t *testing.T) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	refreshManager := broadcast.NewManager(
		newDiscardLoggerForBroadcastBehaviorTests(),
		broadcast.ManagerConfig{},
	)
	refreshManagerContext, cancelRefreshManager := context.WithCancel(
		context.Background(),
	)
	defer cancelRefreshManager()
	go refreshManager.Run(refreshManagerContext)

	refreshHTTPServer := newTCP4HTTPTestServerForBroadcastBehaviorTests(
		t,
		refreshManager,
	)
	defer refreshHTTPServer.Close()

	websocketURL := "ws" + strings.TrimPrefix(refreshHTTPServer.URL, "http")
	connection, _, dialError := websocket.DefaultDialer.Dial(websocketURL, nil)
	if dialError != nil {
		t.Fatalf("dial websocket handler: %v", dialError)
	}
	defer connection.Close()
	waitForRefreshManagerConnectionCountForBroadcastBehaviorTests(
		t,
		refreshManager,
		1,
	)

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	var receivedPayload broadcast.Payload
	payloadDelivered := false
	for attemptIndex := 0; attemptIndex < 10; attemptIndex++ {
		serverForTest.BroadcastRebuilding()
		connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		if readError := connection.ReadJSON(&receivedPayload); readError == nil {
			payloadDelivered = true
			break
		}
	}

	if !payloadDelivered {
		t.Fatal("timed out waiting for rebuilding broadcast payload")
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeRebuilding {
		t.Fatalf("expected rebuilding payload, got %#v", receivedPayload)
	}
}

func setupRefreshWebsocketForBroadcastBehaviorTests(
	t *testing.T,
) (
	*broadcast.Manager,
	*websocket.Conn,
	context.CancelFunc,
	func(),
) {
	t.Helper()

	refreshManager := broadcast.NewManager(
		newDiscardLoggerForBroadcastBehaviorTests(),
		broadcast.ManagerConfig{},
	)
	refreshManagerContext, cancelRefreshManager := context.WithCancel(
		context.Background(),
	)
	go refreshManager.Run(refreshManagerContext)

	refreshHTTPServer := newTCP4HTTPTestServerForBroadcastBehaviorTests(
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
	waitForRefreshManagerConnectionCountForBroadcastBehaviorTests(
		t,
		refreshManager,
		1,
	)

	cleanup := func() {
		_ = connection.Close()
		refreshHTTPServer.Close()
		cancelRefreshManager()
	}
	return refreshManager, connection, cancelRefreshManager, cleanup
}

func waitForRefreshManagerConnectionCountForBroadcastBehaviorTests(
	t *testing.T,
	refreshManager *broadcast.Manager,
	expectedCount int,
) {
	t.Helper()

	waitDeadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(waitDeadline) {
		if refreshManager.ConnectionCount() >= expectedCount {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf(
		"timed out waiting for refresh manager connection count >= %d (got %d)",
		expectedCount,
		refreshManager.ConnectionCount(),
	)
}

func TestBroadcastRebuilding_NoOpInServerOnlyMode(t *testing.T) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	serverForTest.BroadcastRebuilding()
	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"did not expect rebuilding payload in server-only mode, got %#v",
			receivedPayload,
		)
	}
}

func TestBroadcastRebuilding_NoOpWhenContextCanceled(t *testing.T) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	refreshManager, connection, cancelRefreshManager, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()
	cancelRefreshManager()
	time.Sleep(20 * time.Millisecond)

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	serverForTest.BroadcastRebuilding()
	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"did not expect rebuilding payload after context cancellation, got %#v",
			receivedPayload,
		)
	}
}

func TestBroadcastReload_StopsWhenContextCanceled(t *testing.T) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	refreshManager, connection, cancelRefreshManager, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()
	cancelRefreshManager()
	time.Sleep(20 * time.Millisecond)

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload: broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
	})

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"did not expect payload after context cancellation, got %#v",
			receivedPayload,
		)
	}
}

func TestBroadcastReload_WaitingForBuildRetrySkipsPayloadBroadcast(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}
	serverForTest.SetWaitingForBuildRetry(true)

	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload: broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
	})

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"did not expect payload while waiting for build retry, got %#v",
			receivedPayload,
		)
	}
}

func TestBroadcastReload_WaitAppDoesNotBlockSubsequentReloads(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	healthServerPort, requestStarted, requestCanceled, cleanupHealthServer := startBlockingHealthServerForBroadcastBehaviorTests(
		t,
	)
	defer cleanupHealthServer()
	t.Setenv(wavecore.EnvMode, wavecore.EnvModeDev)
	t.Setenv(wavecore.EnvPort, strconv.Itoa(healthServerPort))
	t.Setenv(wavecore.EnvPortSet, "true")

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}
	defer serverForTest.cancelReloadReadinessWait()

	firstCallStartedAt := time.Now()
	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload: broadcast.Payload{
			ChangeType: broadcast.ChangeTypeOther,
		},
		WaitApp: true,
	})
	if firstCallDuration := time.Since(firstCallStartedAt); firstCallDuration > 250*time.Millisecond {
		t.Fatalf(
			"expected first wait-app reload call to return promptly, took %s",
			firstCallDuration,
		)
	}

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for blocking healthcheck probe request")
	}

	secondCallStartedAt := time.Now()
	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload: broadcast.Payload{
			ChangeType: broadcast.ChangeTypeRevalidate,
		},
	})
	if secondCallDuration := time.Since(secondCallStartedAt); secondCallDuration > 250*time.Millisecond {
		t.Fatalf(
			"expected second reload call to return promptly, took %s",
			secondCallDuration,
		)
	}

	select {
	case <-requestCanceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for first wait-app probe cancellation")
	}

	connection.SetReadDeadline(
		time.Now().Add(positiveBroadcastReadTimeoutForBehaviorTests),
	)
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"expected immediate second reload payload broadcast, got read error: %v",
			readError,
		)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeRevalidate {
		t.Fatalf("expected second reload payload, got %#v", receivedPayload)
	}
}

func TestBroadcastReload_CleanupCancelsOutstandingReadinessWait(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	healthServerPort, requestStarted, requestCanceled, cleanupHealthServer := startBlockingHealthServerForBroadcastBehaviorTests(
		t,
	)
	defer cleanupHealthServer()
	t.Setenv(wavecore.EnvMode, wavecore.EnvModeDev)
	t.Setenv(wavecore.EnvPort, strconv.Itoa(healthServerPort))
	t.Setenv(wavecore.EnvPortSet, "true")

	refreshManager, _, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload: broadcast.Payload{
			ChangeType: broadcast.ChangeTypeOther,
		},
		WaitApp: true,
	})

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for outstanding readiness wait request")
	}

	serverForTest.CleanupForRebuild()

	select {
	case <-requestCanceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal(
			"timed out waiting for readiness wait cancellation during cleanup",
		)
	}
}

func TestBroadcastReload_CycleViteReadinessWaitIsAsyncAndCleanupCancelable(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	healthServerPort, requestStarted, requestCanceled, cleanupHealthServer := startBlockingHealthServerForBroadcastBehaviorTests(
		t,
	)
	defer cleanupHealthServer()
	t.Setenv(wavecore.EnvMode, wavecore.EnvModeDev)
	t.Setenv(wavecore.EnvPort, strconv.Itoa(healthServerPort))
	t.Setenv(wavecore.EnvPortSet, "true")

	refreshManager, _, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	broadcastReturn := make(chan struct{})
	go func() {
		serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
			Payload: broadcast.Payload{
				ChangeType: broadcast.ChangeTypeOther,
			},
			CycleVite: true,
			WaitApp:   true,
		})
		close(broadcastReturn)
	}()

	select {
	case <-broadcastReturn:
	case <-time.After(500 * time.Millisecond):
		t.Fatal(
			"expected cycle-vite reload scheduling to return without waiting for readiness probes",
		)
	}

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cycle-vite readiness wait request")
	}

	serverForTest.CleanupForRebuild()

	select {
	case <-requestCanceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal(
			"timed out waiting for cycle-vite readiness wait cancellation during cleanup",
		)
	}
}

func TestShouldBroadcastReloadPayloadAfterReadiness(t *testing.T) {
	serverForTest := &Server{
		Cfg: newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir()),
		Log: newDiscardLoggerForBroadcastBehaviorTests(),
	}

	testCases := []struct {
		name          string
		reloadOptions eventpipeline.ReloadOpts
		expected      bool
	}{
		{
			name:          "non-cycle reload broadcasts payload",
			reloadOptions: eventpipeline.ReloadOpts{CycleVite: false},
			expected:      true,
		},
		{
			name:          "cycle requested broadcasts payload",
			reloadOptions: eventpipeline.ReloadOpts{CycleVite: true},
			expected:      true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			shouldBroadcastPayload := serverForTest.shouldBroadcastReloadPayloadAfterReadiness(
				testCase.reloadOptions,
			)
			if shouldBroadcastPayload != testCase.expected {
				t.Fatalf(
					"ShouldBroadcastReloadPayloadAfterReadiness(%#v)=%t, want %t",
					testCase.reloadOptions,
					shouldBroadcastPayload,
					testCase.expected,
				)
			}
		})
	}
}

func TestBroadcastReload_CycleViteWithoutActiveContextFallsBackToPayloadBroadcast(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
		CycleVite: true,
	})

	connection.SetReadDeadline(
		time.Now().Add(positiveBroadcastReadTimeoutForBehaviorTests),
	)
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"expected fallback payload broadcast when cycleVite cannot be applied, got read error: %v",
			readError,
		)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeOther {
		t.Fatalf(
			"expected fallback hard-reload payload, got %#v",
			receivedPayload,
		)
	}
}

func TestBroadcastReload_FrameworkRuntimeReloadRequestsAreAsyncAndCleanupCancelable(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	reloadEndpointPath := "/__vorma_internal/reload-routes"
	reloadServerPort, requestStarted, requestCanceled, cleanupReloadServer := startBlockingFrameworkRuntimeReloadServerForBroadcastBehaviorTests(
		t,
		reloadEndpointPath,
	)
	defer cleanupReloadServer()
	t.Setenv(wavecore.EnvMode, wavecore.EnvModeDev)
	t.Setenv(wavecore.EnvPort, strconv.Itoa(reloadServerPort))
	t.Setenv(wavecore.EnvPortSet, "true")

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	broadcastReturn := make(chan struct{})
	go func() {
		serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
			Payload: broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
			FrameworkRuntimeReloadRequests: []wave.FrameworkRuntimeReloadRequest{
				{
					EndpointPath:    reloadEndpointPath,
					ReloadAttemptID: "attempt-1",
					ExpectedBuildID: "build-1",
					ReloadTrigger:   "routes-watch",
				},
			},
		})
		close(broadcastReturn)
	}()

	select {
	case <-broadcastReturn:
	case <-time.After(500 * time.Millisecond):
		t.Fatal(
			"expected framework reload scheduling to return without waiting for endpoint response",
		)
	}

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for framework reload request start")
	}

	serverForTest.CleanupForRebuild()

	select {
	case <-requestCanceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal(
			"timed out waiting for framework reload request cancellation during cleanup",
		)
	}

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"did not expect browser payload while framework reload request is blocked, got %#v",
			receivedPayload,
		)
	}
}

func TestBroadcastReload_FrameworkRuntimeReloadRequestsWaitForAppReadiness(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	var appReady atomic.Bool
	requestObservedBeforeReady := make(chan struct{}, 1)
	requestObservedWhenReady := make(chan struct{}, 1)
	reloadServer := newTCP4HTTPTestServerForBroadcastBehaviorTests(
		t,
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/healthz":
					if appReady.Load() {
						responseWriter.WriteHeader(http.StatusOK)
						return
					}
					responseWriter.WriteHeader(http.StatusServiceUnavailable)
					return
				case "/__vorma_internal/reload-routes":
					if !appReady.Load() {
						select {
						case requestObservedBeforeReady <- struct{}{}:
						default:
						}
						responseWriter.WriteHeader(
							http.StatusInternalServerError,
						)
						return
					}
					select {
					case requestObservedWhenReady <- struct{}{}:
					default:
					}
					responseWriter.WriteHeader(http.StatusOK)
					return
				default:
					responseWriter.WriteHeader(http.StatusNotFound)
					return
				}
			},
		),
	)
	defer reloadServer.Close()

	reloadServerURL, parseURLError := url.Parse(reloadServer.URL)
	if parseURLError != nil {
		t.Fatalf("parse reload test server URL: %v", parseURLError)
	}
	reloadServerPort, parsePortError := strconv.Atoi(reloadServerURL.Port())
	if parsePortError != nil {
		t.Fatalf("parse reload test server port: %v", parsePortError)
	}
	t.Setenv(wavecore.EnvMode, wavecore.EnvModeDev)
	t.Setenv(wavecore.EnvPort, strconv.Itoa(reloadServerPort))
	t.Setenv(wavecore.EnvPortSet, "true")

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	go func() {
		time.Sleep(200 * time.Millisecond)
		appReady.Store(true)
	}()

	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload: broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
		WaitApp: false,
		FrameworkRuntimeReloadRequests: []wave.FrameworkRuntimeReloadRequest{
			{
				EndpointPath: "/__vorma_internal/reload-routes",
			},
		},
	})

	connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"expected browser payload after app readiness and framework reload, got read error: %v",
			readError,
		)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeOther {
		t.Fatalf(
			"expected hard-reload payload after readiness-gated framework reload, got %#v",
			receivedPayload,
		)
	}

	select {
	case <-requestObservedBeforeReady:
		t.Fatal(
			"framework reload request was sent before app readiness was satisfied",
		)
	default:
	}
	select {
	case <-requestObservedWhenReady:
	case <-time.After(time.Second):
		t.Fatal(
			"timed out waiting for framework reload request after app became ready",
		)
	}

	if pendingRestartRequest, hasPendingRestartRequest := consumePendingRestartRequestForToolingTests(serverForTest); hasPendingRestartRequest {
		t.Fatalf(
			"did not expect restart request on readiness-gated framework reload success, got %#v",
			pendingRestartRequest,
		)
	}
}

func TestBroadcastReload_FrameworkRuntimeReloadFailureQueuesRestartAndSkipsPayload(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	reloadServer := newTCP4HTTPTestServerForBroadcastBehaviorTests(
		t,
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/healthz" {
					responseWriter.WriteHeader(http.StatusOK)
					return
				}
				if request.URL.Path != "/__vorma_internal/reload-routes" {
					responseWriter.WriteHeader(http.StatusNotFound)
					return
				}
				responseWriter.WriteHeader(http.StatusInternalServerError)
			},
		),
	)
	defer reloadServer.Close()

	reloadServerURL, parseURLError := url.Parse(reloadServer.URL)
	if parseURLError != nil {
		t.Fatalf("parse reload test server URL: %v", parseURLError)
	}
	reloadServerPort, parsePortError := strconv.Atoi(reloadServerURL.Port())
	if parsePortError != nil {
		t.Fatalf("parse reload test server port: %v", parsePortError)
	}
	t.Setenv(wavecore.EnvMode, wavecore.EnvModeDev)
	t.Setenv(wavecore.EnvPort, strconv.Itoa(reloadServerPort))
	t.Setenv(wavecore.EnvPortSet, "true")

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload: broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
		FrameworkRuntimeReloadRequests: []wave.FrameworkRuntimeReloadRequest{
			{
				EndpointPath:    "/__vorma_internal/reload-routes",
				ReloadAttemptID: "attempt-failing",
				ExpectedBuildID: "build-failing",
				ReloadTrigger:   "routes-watch",
			},
		},
	})

	restartRequest := waitForPendingRestartRequestForToolingTests(
		t,
		serverForTest,
		time.Second,
	)
	if restartRequest.IsConfigRestart || restartRequest.RecompileGo {
		t.Fatalf(
			"expected restart without go recompilation after framework reload failure, got %#v",
			restartRequest,
		)
	}

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"did not expect browser payload after framework reload failure, got %#v",
			receivedPayload,
		)
	}
}

func TestBroadcastReload_FrameworkRuntimeReloadSuccessBroadcastsPayloadAndHeaders(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	requestHeaders := make(chan http.Header, 1)
	reloadServer := newTCP4HTTPTestServerForBroadcastBehaviorTests(
		t,
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/healthz" {
					responseWriter.WriteHeader(http.StatusOK)
					return
				}
				if request.URL.Path != "/__vorma_internal/reload-template" {
					responseWriter.WriteHeader(http.StatusNotFound)
					return
				}
				select {
				case requestHeaders <- request.Header.Clone():
				default:
				}
				responseWriter.WriteHeader(http.StatusOK)
			},
		),
	)
	defer reloadServer.Close()

	reloadServerURL, parseURLError := url.Parse(reloadServer.URL)
	if parseURLError != nil {
		t.Fatalf("parse reload test server URL: %v", parseURLError)
	}
	reloadServerPort, parsePortError := strconv.Atoi(reloadServerURL.Port())
	if parsePortError != nil {
		t.Fatalf("parse reload test server port: %v", parsePortError)
	}
	t.Setenv(wavecore.EnvMode, wavecore.EnvModeDev)
	t.Setenv(wavecore.EnvPort, strconv.Itoa(reloadServerPort))
	t.Setenv(wavecore.EnvPortSet, "true")

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload: broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
		FrameworkRuntimeReloadRequests: []wave.FrameworkRuntimeReloadRequest{
			{
				EndpointPath:    "/__vorma_internal/reload-template",
				ReloadAttemptID: "attempt-42",
				ExpectedBuildID: "build-42",
				ReloadTrigger:   "template-watch",
			},
		},
	})

	connection.SetReadDeadline(
		time.Now().Add(positiveBroadcastReadTimeoutForBehaviorTests),
	)
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"expected browser payload after successful framework reload request, got read error: %v",
			readError,
		)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeOther {
		t.Fatalf(
			"expected hard-reload payload after framework reload success, got %#v",
			receivedPayload,
		)
	}

	select {
	case capturedHeaders := <-requestHeaders:
		if got, want := capturedHeaders.Get(wave.FrameworkRuntimeReloadAttemptIDHeaderName), "attempt-42"; got != want {
			t.Fatalf("reload attempt header=%q, want %q", got, want)
		}
		if got, want := capturedHeaders.Get(wave.FrameworkRuntimeReloadExpectedBuildIDHeaderName), "build-42"; got != want {
			t.Fatalf("expected build ID header=%q, want %q", got, want)
		}
		if got, want := capturedHeaders.Get(wave.FrameworkRuntimeReloadTriggerHeaderName), "template-watch"; got != want {
			t.Fatalf("reload trigger header=%q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for framework reload request headers")
	}
}

func TestBroadcastReload_CycleViteFailureFallsBackToPayloadBroadcast(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = "command_that_does_not_exist_for_cycle_vite_failure_test"
	cfg.Vite.DefaultPort = 5211

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForBroadcastBehaviorTests(),
	)
	defer builderForTest.Close()

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:     cfg,
		Log:     newDiscardLoggerForBroadcastBehaviorTests(),
		Builder: builderForTest,
		ViteContext: vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{
			JSPackageManagerBaseCmd: cfg.Vite.JSPackageManagerBaseCmd,
			DefaultPort:             cfg.Vite.DefaultPort,
		}),
		RefreshManager: refreshManager,
	}

	serverForTest.BroadcastReload(eventpipeline.ReloadOpts{
		Payload:   broadcast.Payload{ChangeType: broadcast.ChangeTypeOther},
		CycleVite: true,
	})

	connection.SetReadDeadline(
		time.Now().Add(positiveBroadcastReadTimeoutForBehaviorTests),
	)
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"expected fallback payload broadcast after cycleVite failure, got read error: %v",
			readError,
		)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeOther {
		t.Fatalf(
			"expected fallback hard-reload payload, got %#v",
			receivedPayload,
		)
	}
}

func TestExecuteBrowserPhase_InvalidateViteFallbackWithoutViteSetsHardReload(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Vite = nil

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLoggerForBroadcastBehaviorTests(),
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionInvalidateVite,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
		!work.Browser.WaitForApp {
		t.Fatalf(
			"expected invalidate fallback to set hard-reload+wait-app, got action=%v waitApp=%v",
			work.Browser.Action,
			work.Browser.WaitForApp,
		)
	}
	if work.Browser.WaitForVite {
		t.Fatal("did not expect wait-for-vite when Vite is disabled")
	}
}

func TestExecuteBrowserPhase_WaitingForBuildRetrySkipsBrowserBroadcast(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}
	serverForTest.SetWaitingForBuildRetry(true)

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionRevalidate,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"did not expect browser payload while waiting for build retry, got %#v",
			receivedPayload,
		)
	}
}

func TestExecuteBrowserPhase_InvalidateViteFailureFallsBackToHardReload(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = "pnpm"

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLoggerForBroadcastBehaviorTests(),
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionInvalidateVite,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	if work.Browser.Action != eventpipeline.BrowserPhaseActionHardReload ||
		!work.Browser.WaitForApp ||
		!work.Browser.WaitForVite {
		t.Fatalf(
			"expected fallback hard-reload flags, got action=%v waitApp=%v waitVite=%v",
			work.Browser.Action,
			work.Browser.WaitForApp,
			work.Browser.WaitForVite,
		)
	}
}

func TestExecuteBrowserPhase_InvalidateViteFailureLogsFallbackMessage(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = "pnpm"

	var logBuffer bytes.Buffer
	serverForTest := &Server{
		Cfg: cfg,
		Log: slog.New(slog.NewTextHandler(&logBuffer, nil)),
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionInvalidateVite,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	if !strings.Contains(
		logBuffer.String(),
		"vite invalidate endpoint failed; falling back to hard reload",
	) {
		t.Fatalf(
			"expected invalidate fallback log message, got logs: %s",
			logBuffer.String(),
		)
	}
}

func TestExecuteBrowserPhase_InvalidateViteIsAsyncAndCleanupCancelable(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = "pnpm"

	invalidateServerPort, requestStarted, requestCanceled, cleanupInvalidateServer := startBlockingViteInvalidateServerForBroadcastBehaviorTests(
		t,
	)
	defer cleanupInvalidateServer()

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLoggerForBroadcastBehaviorTests(),
		ViteContext: vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{
			DefaultPort: invalidateServerPort,
		}),
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionInvalidateVite,
		},
	}

	executeDone := make(chan struct{})
	go func() {
		serverForTest.ExecuteBrowserPhase(work)
		close(executeDone)
	}()

	select {
	case <-executeDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal(
			"expected invalidate browser phase to schedule async work and return quickly",
		)
	}

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async invalidate request start")
	}

	serverForTest.CleanupForRebuild()

	select {
	case <-requestCanceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal(
			"timed out waiting for async invalidate request cancellation during cleanup",
		)
	}
}

func TestExecuteBrowserPhase_HotReloadCSSBroadcastsCriticalAndNormalPayloads(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical:    filepath.Join(root, "styles", "critical.css"),
		NonCritical: filepath.Join(root, "styles", "normal.css"),
	}

	if mkdirCriticalError := os.MkdirAll(
		filepath.Dir(cfg.Core.CSSEntryFiles.Critical),
		0o755,
	); mkdirCriticalError != nil {
		t.Fatalf(
			"failed creating critical css entry parent dir: %v",
			mkdirCriticalError,
		)
	}
	if mkdirNormalError := os.MkdirAll(
		filepath.Dir(cfg.Core.CSSEntryFiles.NonCritical),
		0o755,
	); mkdirNormalError != nil {
		t.Fatalf(
			"failed creating normal css entry parent dir: %v",
			mkdirNormalError,
		)
	}
	if writeCriticalEntryError := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte("body{color:red;}"),
		0o644,
	); writeCriticalEntryError != nil {
		t.Fatalf(
			"failed writing critical css entry file: %v",
			writeCriticalEntryError,
		)
	}
	if writeNormalEntryError := os.WriteFile(
		cfg.Core.CSSEntryFiles.NonCritical,
		[]byte("body{color:blue;}"),
		0o644,
	); writeNormalEntryError != nil {
		t.Fatalf(
			"failed writing normal css entry file: %v",
			writeNormalEntryError,
		)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForBroadcastBehaviorTests(),
	)
	defer builderForTest.Close()

	if buildCSSError := builderForTest.BuildCSS(builder.CSSBuildOptions{
		BuildCriticalCSS: true,
		BuildNormalCSS:   true,
	}); buildCSSError != nil {
		t.Fatalf("BuildCSS returned error: %v", buildCSSError)
	}

	criticalCSSBytes, readCriticalCSSError := os.ReadFile(
		cfg.Dist.CriticalCSS(),
	)
	if readCriticalCSSError != nil {
		t.Fatalf(
			"failed reading built critical css output: %v",
			readCriticalCSSError,
		)
	}
	normalCSSRefBytes, readNormalCSSRefError := os.ReadFile(
		cfg.Dist.NormalCSSRef(),
	)
	if readNormalCSSRefError != nil {
		t.Fatalf(
			"failed reading built normal css ref output: %v",
			readNormalCSSRefError,
		)
	}

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		Builder:        builderForTest,
		RefreshManager: refreshManager,
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionHotReloadCSS,
		},
		Build: eventpipeline.BuildPhaseDecision{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	connection.SetReadDeadline(
		time.Now().Add(positiveBroadcastReadTimeoutForBehaviorTests),
	)
	var firstPayload broadcast.Payload
	if readFirstPayloadError := connection.ReadJSON(&firstPayload); readFirstPayloadError != nil {
		t.Fatalf(
			"expected first css payload broadcast, got read error: %v",
			readFirstPayloadError,
		)
	}
	var secondPayload broadcast.Payload
	if readSecondPayloadError := connection.ReadJSON(&secondPayload); readSecondPayloadError != nil {
		t.Fatalf(
			"expected second css payload broadcast, got read error: %v",
			readSecondPayloadError,
		)
	}

	if firstPayload.ChangeType != broadcast.ChangeTypeCriticalCSS {
		t.Fatalf(
			"expected first payload to be critical CSS reload, got %#v",
			firstPayload,
		)
	}
	if secondPayload.ChangeType != broadcast.ChangeTypeNormalCSS {
		t.Fatalf(
			"expected second payload to be normal CSS reload, got %#v",
			secondPayload,
		)
	}

	expectedCriticalCSSPayload := base64.StdEncoding.EncodeToString(
		criticalCSSBytes,
	)
	if firstPayload.CriticalCSS != expectedCriticalCSSPayload {
		t.Fatalf(
			"unexpected critical CSS payload: %q",
			firstPayload.CriticalCSS,
		)
	}

	expectedNormalCSSURL := filepath.ToSlash(
		cfg.PublicPathPrefix() + strings.TrimSpace(string(normalCSSRefBytes)),
	)
	if secondPayload.NormalCSSURL != expectedNormalCSSURL {
		t.Fatalf(
			"unexpected normal CSS URL payload: %q",
			secondPayload.NormalCSSURL,
		)
	}
}

func TestExecuteBrowserPhase_HotReloadCSSSkipsPayloadsWhenFreshBuildOutputsAreUnavailable(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical:    filepath.Join(root, "styles", "critical.css"),
		NonCritical: filepath.Join(root, "styles", "normal.css"),
	}

	if mkdirCriticalError := os.MkdirAll(
		filepath.Dir(cfg.Core.CSSEntryFiles.Critical),
		0o755,
	); mkdirCriticalError != nil {
		t.Fatalf(
			"failed creating critical css entry parent dir: %v",
			mkdirCriticalError,
		)
	}
	if mkdirNormalError := os.MkdirAll(
		filepath.Dir(cfg.Core.CSSEntryFiles.NonCritical),
		0o755,
	); mkdirNormalError != nil {
		t.Fatalf(
			"failed creating normal css entry parent dir: %v",
			mkdirNormalError,
		)
	}
	if writeCriticalEntryError := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte("body{color:red;}"),
		0o644,
	); writeCriticalEntryError != nil {
		t.Fatalf(
			"failed writing critical css entry file: %v",
			writeCriticalEntryError,
		)
	}
	if writeNormalEntryError := os.WriteFile(
		cfg.Core.CSSEntryFiles.NonCritical,
		[]byte("body{color:blue;}"),
		0o644,
	); writeNormalEntryError != nil {
		t.Fatalf(
			"failed writing normal css entry file: %v",
			writeNormalEntryError,
		)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForBroadcastBehaviorTests(),
	)
	defer builderForTest.Close()

	if initialBuildError := builderForTest.BuildCSS(builder.CSSBuildOptions{
		BuildCriticalCSS: true,
		BuildNormalCSS:   true,
	}); initialBuildError != nil {
		t.Fatalf("initial BuildCSS returned error: %v", initialBuildError)
	}

	cfg.Core.CSSEntryFiles.Critical = filepath.Join(
		root,
		"styles",
		"missing-critical.css",
	)
	cfg.Core.CSSEntryFiles.NonCritical = filepath.Join(
		root,
		"styles",
		"missing-normal.css",
	)
	if buildError := builderForTest.BuildCSS(builder.CSSBuildOptions{
		BuildCriticalCSS: true,
		BuildNormalCSS:   false,
	}); buildError == nil {
		t.Fatal(
			"expected BuildCSS critical-only rebuild to fail for missing entry",
		)
	}
	if buildError := builderForTest.BuildCSS(builder.CSSBuildOptions{
		BuildCriticalCSS: false,
		BuildNormalCSS:   true,
	}); buildError == nil {
		t.Fatal(
			"expected BuildCSS normal-only rebuild to fail for missing entry",
		)
	}

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		Builder:        builderForTest,
		RefreshManager: refreshManager,
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionHotReloadCSS,
		},
		Build: eventpipeline.BuildPhaseDecision{
			BuildCriticalCSS: true,
			BuildNormalCSS:   true,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"did not expect css hot-reload payload after failed rebuilds, got %#v",
			receivedPayload,
		)
	}
}

func TestExecuteBrowserPhase_RevalidateBroadcastsRevalidatePayload(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionRevalidate,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	connection.SetReadDeadline(
		time.Now().Add(positiveBroadcastReadTimeoutForBehaviorTests),
	)
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"expected revalidate broadcast payload, got read error: %v",
			readError,
		)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeRevalidate {
		t.Fatalf("expected revalidate payload, got %#v", receivedPayload)
	}
}

func TestExecuteBrowserPhase_RevalidateExecutesDeferredFrameworkReloadRequests(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	requestPaths := make(chan string, 1)
	reloadServer := newTCP4HTTPTestServerForBroadcastBehaviorTests(
		t,
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/healthz" {
					responseWriter.WriteHeader(http.StatusOK)
					return
				}
				if request.URL.Path == "/__vorma_internal/reload-routes" {
					select {
					case requestPaths <- request.URL.Path:
					default:
					}
				}
				responseWriter.WriteHeader(http.StatusOK)
			},
		),
	)
	defer reloadServer.Close()

	reloadServerURL, parseURLError := url.Parse(reloadServer.URL)
	if parseURLError != nil {
		t.Fatalf("parse reload test server URL: %v", parseURLError)
	}
	reloadServerPort, parsePortError := strconv.Atoi(reloadServerURL.Port())
	if parsePortError != nil {
		t.Fatalf("parse reload test server port: %v", parsePortError)
	}
	t.Setenv(wavecore.EnvMode, wavecore.EnvModeDev)
	t.Setenv(wavecore.EnvPort, strconv.Itoa(reloadServerPort))
	t.Setenv(wavecore.EnvPortSet, "true")

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionRevalidate,
		},
		FrameworkRuntimeReloadRequests: []wave.FrameworkRuntimeReloadRequest{
			{
				EndpointPath: "/__vorma_internal/reload-routes",
			},
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	connection.SetReadDeadline(
		time.Now().Add(positiveBroadcastReadTimeoutForBehaviorTests),
	)
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"expected revalidate payload after deferred framework reload request, got read error: %v",
			readError,
		)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeRevalidate {
		t.Fatalf("expected revalidate payload, got %#v", receivedPayload)
	}

	select {
	case requestPath := <-requestPaths:
		if requestPath != "/__vorma_internal/reload-routes" {
			t.Fatalf(
				"framework reload request path=%q, want %q",
				requestPath,
				"/__vorma_internal/reload-routes",
			)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for deferred framework reload request")
	}
}

func TestExecuteBrowserPhase_HotReloadCSSBroadcastsCriticalOnlyPayload(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: filepath.Join(root, "styles", "critical.css"),
	}

	if mkdirCriticalError := os.MkdirAll(
		filepath.Dir(cfg.Core.CSSEntryFiles.Critical),
		0o755,
	); mkdirCriticalError != nil {
		t.Fatalf(
			"failed creating critical css entry parent dir: %v",
			mkdirCriticalError,
		)
	}
	if writeCriticalEntryError := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte("body{background:black;}"),
		0o644,
	); writeCriticalEntryError != nil {
		t.Fatalf(
			"failed writing critical css entry file: %v",
			writeCriticalEntryError,
		)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForBroadcastBehaviorTests(),
	)
	defer builderForTest.Close()

	if buildCSSError := builderForTest.BuildCSS(builder.CSSBuildOptions{
		BuildCriticalCSS: true,
	}); buildCSSError != nil {
		t.Fatalf("BuildCSS critical-only returned error: %v", buildCSSError)
	}

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		Builder:        builderForTest,
		RefreshManager: refreshManager,
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionHotReloadCSS,
		},
		Build: eventpipeline.BuildPhaseDecision{
			BuildCriticalCSS: true,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	connection.SetReadDeadline(
		time.Now().Add(positiveBroadcastReadTimeoutForBehaviorTests),
	)
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"expected critical css payload broadcast, got read error: %v",
			readError,
		)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeCriticalCSS {
		t.Fatalf("expected critical css payload, got %#v", receivedPayload)
	}
}

func TestExecuteBrowserPhase_HotReloadCSSBroadcastsCriticalPayloadWithEmptyCSSField(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		Critical: filepath.Join(root, "styles", "critical.css"),
	}

	if mkdirCriticalError := os.MkdirAll(
		filepath.Dir(cfg.Core.CSSEntryFiles.Critical),
		0o755,
	); mkdirCriticalError != nil {
		t.Fatalf(
			"failed creating critical css entry parent dir: %v",
			mkdirCriticalError,
		)
	}
	if writeCriticalEntryError := os.WriteFile(
		cfg.Core.CSSEntryFiles.Critical,
		[]byte(""),
		0o644,
	); writeCriticalEntryError != nil {
		t.Fatalf(
			"failed writing empty critical css entry file: %v",
			writeCriticalEntryError,
		)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForBroadcastBehaviorTests(),
	)
	defer builderForTest.Close()

	if buildCSSError := builderForTest.BuildCSS(builder.CSSBuildOptions{
		BuildCriticalCSS: true,
	}); buildCSSError != nil {
		t.Fatalf("BuildCSS critical-only returned error: %v", buildCSSError)
	}

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		Builder:        builderForTest,
		RefreshManager: refreshManager,
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionHotReloadCSS,
		},
		Build: eventpipeline.BuildPhaseDecision{
			BuildCriticalCSS: true,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	connection.SetReadDeadline(
		time.Now().Add(positiveBroadcastReadTimeoutForBehaviorTests),
	)
	_, payloadBytes, readError := connection.ReadMessage()
	if readError != nil {
		t.Fatalf(
			"expected critical css payload broadcast, got read error: %v",
			readError,
		)
	}
	payloadString := string(payloadBytes)
	if !strings.Contains(payloadString, `"changeType":"critical"`) {
		t.Fatalf("expected critical changeType payload, got %s", payloadString)
	}
	if !strings.Contains(payloadString, `"criticalCSS":""`) {
		t.Fatalf(
			"expected criticalCSS field with empty payload to be serialized, got %s",
			payloadString,
		)
	}
}

func TestExecuteBrowserPhase_HotReloadCSSBroadcastsNormalOnlyPayload(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Core.CSSEntryFiles = cssEntryFilesForTests{
		NonCritical: filepath.Join(root, "styles", "normal.css"),
	}

	if mkdirNormalError := os.MkdirAll(
		filepath.Dir(cfg.Core.CSSEntryFiles.NonCritical),
		0o755,
	); mkdirNormalError != nil {
		t.Fatalf(
			"failed creating normal css entry parent dir: %v",
			mkdirNormalError,
		)
	}
	if writeNormalEntryError := os.WriteFile(
		cfg.Core.CSSEntryFiles.NonCritical,
		[]byte("body{background:white;}"),
		0o644,
	); writeNormalEntryError != nil {
		t.Fatalf(
			"failed writing normal css entry file: %v",
			writeNormalEntryError,
		)
	}

	builderForTest := builder.NewBuilder(
		cfg,
		newDiscardLoggerForBroadcastBehaviorTests(),
	)
	defer builderForTest.Close()

	if buildCSSError := builderForTest.BuildCSS(builder.CSSBuildOptions{
		BuildNormalCSS: true,
	}); buildCSSError != nil {
		t.Fatalf("BuildCSS normal-only returned error: %v", buildCSSError)
	}

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		Builder:        builderForTest,
		RefreshManager: refreshManager,
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionHotReloadCSS,
		},
		Build: eventpipeline.BuildPhaseDecision{
			BuildNormalCSS: true,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	connection.SetReadDeadline(
		time.Now().Add(positiveBroadcastReadTimeoutForBehaviorTests),
	)
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError != nil {
		t.Fatalf(
			"expected normal css payload broadcast, got read error: %v",
			readError,
		)
	}
	if receivedPayload.ChangeType != broadcast.ChangeTypeNormalCSS {
		t.Fatalf("expected normal css payload, got %#v", receivedPayload)
	}
}

func TestExecuteBrowserPhase_NoOpWhenServerOnlyMode(t *testing.T) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg:            cfg,
		Log:            newDiscardLoggerForBroadcastBehaviorTests(),
		RefreshManager: refreshManager,
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionHardReload,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"did not expect browser broadcast in server-only mode, got %#v",
			receivedPayload,
		)
	}
}

func TestExecuteBrowserPhase_InvalidateViteSuccessReturnsWithoutReloadFallback(
	t *testing.T,
) {
	cfg := newParsedConfigForBroadcastBehaviorTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	ensureViteConfigForToolingTests(t, cfg)
	cfg.Vite.JSPackageManagerBaseCmd = "pnpm"

	invalidateServer := httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter,
		request *http.Request,
	) {
		if request.URL.Path != "/__vorma_invalidate_filemap" {
			responseWriter.WriteHeader(http.StatusNotFound)
			return
		}
		if request.Method != http.MethodPost {
			responseWriter.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer invalidateServer.Close()

	parsedInvalidateServerURL, parseURLError := url.Parse(invalidateServer.URL)
	if parseURLError != nil {
		t.Fatalf("failed parsing invalidate test server URL: %v", parseURLError)
	}
	invalidatePort, parsePortError := strconv.Atoi(
		parsedInvalidateServerURL.Port(),
	)
	if parsePortError != nil {
		t.Fatalf(
			"failed parsing invalidate test server port: %v",
			parsePortError,
		)
	}

	refreshManager, connection, _, cleanup := setupRefreshWebsocketForBroadcastBehaviorTests(
		t,
	)
	defer cleanup()

	serverForTest := &Server{
		Cfg: cfg,
		Log: newDiscardLoggerForBroadcastBehaviorTests(),
		ViteContext: vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{
			DefaultPort: invalidatePort,
		}),
		RefreshManager: refreshManager,
	}

	work := &eventpipeline.WorkSet{
		Browser: eventpipeline.BrowserPhaseDecision{
			Action: eventpipeline.BrowserPhaseActionInvalidateVite,
		},
	}
	serverForTest.ExecuteBrowserPhase(work)

	if work.Browser.Action != eventpipeline.BrowserPhaseActionInvalidateVite ||
		work.Browser.WaitForApp ||
		work.Browser.WaitForVite {
		t.Fatalf(
			"expected no fallback flags on successful vite invalidation, got action=%v waitApp=%v waitVite=%v",
			work.Browser.Action,
			work.Browser.WaitForApp,
			work.Browser.WaitForVite,
		)
	}

	connection.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var receivedPayload broadcast.Payload
	if readError := connection.ReadJSON(&receivedPayload); readError == nil {
		t.Fatalf(
			"did not expect fallback reload broadcast on successful invalidation, got %#v",
			receivedPayload,
		)
	}
}
