package appsupervisor_test

import (
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavedev/devserver/internal/appsupervisor"
)

type staticAssetDirsForTests = struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

func TestWaitForApp_UsesConfiguredHealthcheckEndpoint(t *testing.T) {
	cfg := newParsedConfigForRuntimeprocessWaitTestsAtRoot(t.TempDir())
	cfg.Watch.HealthcheckEndpoint = "/healthz"

	port := mustConfigureAndGetWaveAppPortForRuntimeprocessWaitTests(t)
	listener, listenError := net.Listen(
		"tcp",
		fmt.Sprintf("127.0.0.1:%d", port),
	)
	if listenError != nil {
		t.Skipf(
			"unable to bind app port %d for WaitForApp test: %v",
			port,
			listenError,
		)
	}
	defer listener.Close()

	httpServer := &http.Server{
		Handler: http.HandlerFunc(
			func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/healthz" {
					responseWriter.WriteHeader(http.StatusOK)
					return
				}
				responseWriter.WriteHeader(http.StatusNotFound)
			},
		),
	}
	defer httpServer.Close()
	go func() {
		_ = httpServer.Serve(listener)
	}()

	appReadyURL := appsupervisor.ResolveAppReadyURL(
		port,
		cfg.HealthcheckEndpoint(),
	)
	if !appsupervisor.WaitForAnyReady(
		[]string{appReadyURL},
		appsupervisor.DefaultReadinessWaitPolicy(),
	) {
		t.Fatal(
			"expected appsupervisor.WaitForAnyReady to return true when healthcheck endpoint is ready",
		)
	}
}

func TestResolveAppReadyURL_UsesIPv4LoopbackHost(t *testing.T) {
	resolvedURL := appsupervisor.ResolveAppReadyURL(4242, "/healthz")
	const expectedURL = "http://127.0.0.1:4242/healthz"
	if resolvedURL != expectedURL {
		t.Fatalf("ResolveAppReadyURL()=%q, want %q", resolvedURL, expectedURL)
	}
}

func newParsedConfigForRuntimeprocessWaitTestsAtRoot(
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
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	cfg.Dist.Root = cfg.Core.DistDir
	return cfg
}

func mustConfigureAndGetWaveAppPortForRuntimeprocessWaitTests(
	t *testing.T,
) int {
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
