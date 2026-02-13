package tooling

import (
	"context"
	"errors"
	"fmt"
	"io"
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

	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
)

const testViteHelperProcessEnv = "WAVE_TOOLING_TEST_VITE_HELPER_PROCESS"

// TestWaveToolingViteHelperProcess is executed in a subprocess to emulate a vite dev server.
func TestWaveToolingViteHelperProcess(t *testing.T) {
	if os.Getenv(testViteHelperProcessEnv) != "1" {
		return
	}

	if err := runWaveToolingViteHelperProcess(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestCycleVite_RestartsRunningViteProcess(t *testing.T) {
	t.Setenv(testViteHelperProcessEnv, "1")

	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: helperViteBaseCommand(t),
		DefaultPort:             5199,
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
	t.Cleanup(func() {
		_ = s.stopVite()
	})

	if !s.waitForVite() {
		t.Fatal("expected first vite process to become ready")
	}
	firstPID := fetchViteProcessID(t, s.viteCtx.GetPort())

	s.cycleVite()

	if s.viteCtx == nil {
		t.Fatal("expected vite context to remain available after cycle")
	}
	if !s.waitForVite() {
		t.Fatal("expected cycled vite process to become ready")
	}
	secondPID := fetchViteProcessID(t, s.viteCtx.GetPort())

	if firstPID == secondPID {
		t.Fatalf("expected cycleVite to restart process, got same pid %q", firstPID)
	}
}

func TestBroadcastReload_WithCycleVite_RestartsThenBroadcastsOnce(t *testing.T) {
	t.Setenv(testViteHelperProcessEnv, "1")

	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Vite = &wave.ViteConfig{
		JSPackageManagerBaseCmd: helperViteBaseCommand(t),
		DefaultPort:             5199,
	}

	builder := NewBuilder(cfg, newDiscardLogger())
	defer builder.Close()

	s := &server{
		cfg:     cfg,
		log:     newDiscardLogger(),
		builder: builder,
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 2),
		},
		refreshMgrCtx: context.Background(),
	}

	if err := s.startVite(); err != nil {
		t.Fatalf("startVite returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = s.stopVite()
	})

	if !s.waitForVite() {
		t.Fatal("expected first vite process to become ready")
	}
	firstPID := fetchViteProcessID(t, s.viteCtx.GetPort())

	s.broadcastReload(reloadOpts{
		payload:   refreshPayload{ChangeType: changeTypeOther},
		cycleVite: true,
	})

	select {
	case msg := <-s.refreshMgr.broadcast:
		t.Fatalf("expected cycleVite reload to skip direct broadcast payload, got %#v", msg)
	case <-time.After(100 * time.Millisecond):
	}

	secondPID := fetchViteProcessID(t, s.viteCtx.GetPort())
	if firstPID == secondPID {
		t.Fatalf("expected cycleVite reload to restart vite process, got same pid %q", firstPID)
	}

	select {
	case extra := <-s.refreshMgr.broadcast:
		t.Fatalf("expected no direct broadcast payload after cycleVite reload, got %#v", extra)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBroadcastReload_WaitsForAppAndViteBeforeBroadcast(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	appPort := wave.MustGetPort()
	appListener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", appPort))
	if err != nil {
		t.Skipf("unable to bind app port %d for wait-for-app test: %v", appPort, err)
	}
	defer appListener.Close()

	var appHits atomic.Int32
	appServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				appHits.Add(1)
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}),
	}
	defer appServer.Close()
	go appServer.Serve(appListener)

	var viteHits atomic.Int32
	viteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/@vite/client" {
			viteHits.Add(1)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer viteServer.Close()

	vitePort := mustPortFromURL(t, viteServer.URL)

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
		refreshMgr: &clientManager{
			broadcast: make(chan refreshPayload, 1),
		},
		refreshMgrCtx: context.Background(),
		viteCtx:       vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{DefaultPort: vitePort}),
	}

	s.broadcastReload(reloadOpts{
		payload:  refreshPayload{ChangeType: changeTypeRevalidate},
		waitApp:  true,
		waitVite: true,
	})

	select {
	case msg := <-s.refreshMgr.broadcast:
		if msg.ChangeType != changeTypeRevalidate {
			t.Fatalf("unexpected broadcast payload: %#v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for waitApp+waitVite broadcast")
	}

	if appHits.Load() == 0 {
		t.Fatal("expected broadcastReload(waitApp=true) to probe app health endpoint")
	}
	if viteHits.Load() == 0 {
		t.Fatal("expected broadcastReload(waitVite=true) to probe vite endpoint")
	}
}

func TestStartRefreshServer_FallsBackWhenPreferredPortIsUnavailable(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = false

	occupiedListener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Skipf("unable to reserve preferred port for fallback test: %v", err)
	}
	defer occupiedListener.Close()
	occupiedPort := occupiedListener.Addr().(*net.TCPAddr).Port

	s := &server{
		cfg: cfg,
		log: newDiscardLogger(),
	}

	actualPort, err := s.startRefreshServer(occupiedPort)
	if err != nil {
		t.Fatalf("startRefreshServer returned error: %v", err)
	}
	defer s.stopRefreshServer()

	if actualPort <= 0 {
		t.Fatalf("expected positive fallback port, got %d", actualPort)
	}
	if actualPort == occupiedPort {
		t.Fatalf("expected fallback port to differ from occupied port %d", occupiedPort)
	}

	url := fmt.Sprintf("http://localhost:%d/get-refresh-script-inner", actualPort)
	if !s.waitForReady(url) {
		t.Fatalf("refresh server did not become ready on fallback port: %s", url)
	}
}

func TestServerRun_BrowserModeInitWatcherFailureCleansRefreshResources(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = false
	cfg.Watch.WatchRoot = filepath.Join(root, "missing-watch-root")

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

	if s.refreshServer != nil {
		t.Fatal("expected refresh server to be cleaned up on run() exit")
	}
	if s.refreshMgrCancel != nil {
		t.Fatal("expected refresh manager cancel func to be cleared on run() exit")
	}
}

func runWaveToolingViteHelperProcess() error {
	port, err := parseVitePortFromArgs(os.Args)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/@vite/client", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, strconv.Itoa(os.Getpid()))
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}

	server := &http.Server{Handler: mux}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.Serve(listener)
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, helperTerminationSignals()...)
	defer signal.Stop(signals)

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-signals:
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		err := <-serveErr
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("vite helper process timed out without termination signal")
	}
}

func parseVitePortFromArgs(args []string) (int, error) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--port" {
			port, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, fmt.Errorf("invalid --port value %q: %w", args[i+1], err)
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

	executablePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error: %v", err)
	}

	if strings.ContainsAny(executablePath, " \t\n") {
		t.Skip("test binary path contains whitespace; strings.Fields parser cannot represent this safely")
	}

	return executablePath + " -test.run=^TestWaveToolingViteHelperProcess$ --"
}

func fetchViteProcessID(t *testing.T, port int) string {
	t.Helper()

	url := fmt.Sprintf("http://localhost:%d/@vite/client", port)
	client := &http.Client{Timeout: 300 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr == nil && resp.StatusCode == http.StatusOK {
				pid := strings.TrimSpace(string(bodyBytes))
				if pid != "" {
					return pid
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("timed out reading vite helper pid from %s", url)
	return ""
}

func mustPortFromURL(t *testing.T, rawURL string) int {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("failed parsing URL %q: %v", rawURL, err)
	}

	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		t.Fatalf("failed parsing URL port %q: %v", parsedURL.Port(), err)
	}
	return port
}
