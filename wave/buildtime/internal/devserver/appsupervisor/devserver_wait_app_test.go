package appsupervisor_test

import (
	"fmt"
	"github.com/vormadev/vorma/wave/waveconfig"
	"net"
	"net/http"
	"strconv"
	"testing"

	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/appsupervisor"
)

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
) *waveconfig.ParsedConfig {
	return wavetest.NewParsedConfigAtRoot(root)
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
