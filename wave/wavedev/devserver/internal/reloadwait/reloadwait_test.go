package reloadwait

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave"
)

func newDiscardLoggerForReloadWaitTests() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func parsePortFromRawURLForReloadWaitTests(
	t *testing.T,
	rawURL string,
) int {
	t.Helper()

	parsedURL, parseURLError := url.Parse(rawURL)
	if parseURLError != nil {
		t.Fatalf("parse URL: %v", parseURLError)
	}
	port, parsePortError := strconv.Atoi(parsedURL.Port())
	if parsePortError != nil {
		t.Fatalf("parse port: %v", parsePortError)
	}
	return port
}

func waitForHealthzForReloadWaitTests(
	t *testing.T,
	serverAddress string,
) {
	t.Helper()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	healthzURL := "http://" + serverAddress + "/healthz"
	var lastError error
	for range 40 {
		response, requestError := client.Get(healthzURL)
		if requestError == nil {
			bodyBytes, readBodyError := io.ReadAll(response.Body)
			response.Body.Close()
			if readBodyError == nil &&
				response.StatusCode == http.StatusOK &&
				string(bodyBytes) == "ok" {
				return
			}
			lastError = fmt.Errorf(
				"status=%d body=%q readError=%v",
				response.StatusCode,
				string(bodyBytes),
				readBodyError,
			)
		} else {
			lastError = requestError
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("healthz never became ready: %v", lastError)
}

func TestResolveViteReadyURLs(t *testing.T) {
	got := ResolveViteReadyURLs(5151)
	want := []string{
		"http://127.0.0.1:5151/@vite/client",
		"http://localhost:5151/@vite/client",
	}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("got[%d]=%q, want %q", index, got[index], want[index])
		}
	}
}

func TestResolveRefreshRuntimePortFromEnvironmentOrDefault(t *testing.T) {
	const defaultPort = 10000

	t.Setenv("WAVE_REFRESH_PORT", "")
	if got := ResolveRefreshRuntimePortFromEnvironmentOrDefault(defaultPort); got != defaultPort {
		t.Fatalf("empty env got %d, want %d", got, defaultPort)
	}

	t.Setenv("WAVE_REFRESH_PORT", "9876")
	if got := ResolveRefreshRuntimePortFromEnvironmentOrDefault(defaultPort); got != 9876 {
		t.Fatalf("valid env got %d, want %d", got, 9876)
	}

	t.Setenv("WAVE_REFRESH_PORT", "bad-port")
	if got := ResolveRefreshRuntimePortFromEnvironmentOrDefault(defaultPort); got != defaultPort {
		t.Fatalf("invalid env got %d, want %d", got, defaultPort)
	}

	t.Setenv("WAVE_REFRESH_PORT", "-12")
	if got := ResolveRefreshRuntimePortFromEnvironmentOrDefault(defaultPort); got != defaultPort {
		t.Fatalf("negative env got %d, want %d", got, defaultPort)
	}
}

func TestResolveDirectoryPathFromFilePath(t *testing.T) {
	if got := ResolveDirectoryPathFromFilePath(""); got != "" {
		t.Fatalf("empty path got %q, want empty", got)
	}

	got := ResolveDirectoryPathFromFilePath("/tmp/app/../app/config/wave.toml")
	const want = "/tmp/app/config"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCallViteFilemapInvalidateWithContext(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	port := parsePortFromRawURLForReloadWaitTests(t, server.URL)
	if callError := CallViteFilemapInvalidateWithContext(context.Background(), port); callError != nil {
		t.Fatalf("expected success, got error: %v", callError)
	}
	if requestedPath != "/__vorma_invalidate_filemap" {
		t.Fatalf(
			"requestedPath=%q, want %q",
			requestedPath,
			"/__vorma_invalidate_filemap",
		)
	}
}

func TestCallViteFilemapInvalidateWithContext_ReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
	)
	defer server.Close()

	port := parsePortFromRawURLForReloadWaitTests(t, server.URL)
	callError := CallViteFilemapInvalidateWithContext(
		context.Background(),
		port,
	)
	if callError == nil {
		t.Fatal("expected status error")
	}
	if !strings.Contains(callError.Error(), "returned 500") {
		t.Fatalf("unexpected error: %v", callError)
	}
}

func TestCallFrameworkRuntimeReloadEndpointWithContext(t *testing.T) {
	var requestedPath string
	var requestMethod string
	var requestHeaders http.Header
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.Path
			requestMethod = r.Method
			requestHeaders = r.Header.Clone()
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	port := parsePortFromRawURLForReloadWaitTests(t, server.URL)
	callError := CallFrameworkRuntimeReloadEndpointWithContext(
		context.Background(),
		port,
		wave.FrameworkRuntimeReloadRequest{
			EndpointPath:    "__vorma_internal/reload-template",
			ReloadAttemptID: "attempt-7",
			ExpectedBuildID: "build-7",
			ReloadTrigger:   "dev-reload",
		},
	)
	if callError != nil {
		t.Fatalf("expected success, got error: %v", callError)
	}
	if requestedPath != "/__vorma_internal/reload-template" {
		t.Fatalf(
			"requestedPath=%q, want %q",
			requestedPath,
			"/__vorma_internal/reload-template",
		)
	}
	if requestMethod != http.MethodPost {
		t.Fatalf("requestMethod=%q, want %q", requestMethod, http.MethodPost)
	}
	if got, want := requestHeaders.Get(wave.FrameworkRuntimeReloadAttemptIDHeaderName), "attempt-7"; got != want {
		t.Fatalf("reload attempt header=%q, want %q", got, want)
	}
	if got, want := requestHeaders.Get(wave.FrameworkRuntimeReloadExpectedBuildIDHeaderName), "build-7"; got != want {
		t.Fatalf("expected build ID header=%q, want %q", got, want)
	}
	if got, want := requestHeaders.Get(wave.FrameworkRuntimeReloadTriggerHeaderName), "dev-reload"; got != want {
		t.Fatalf("reload trigger header=%q, want %q", got, want)
	}
}

func TestCallFrameworkRuntimeReloadEndpointWithContext_ReturnsStatusError(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}),
	)
	defer server.Close()

	port := parsePortFromRawURLForReloadWaitTests(t, server.URL)
	callError := CallFrameworkRuntimeReloadEndpointWithContext(
		context.Background(),
		port,
		wave.FrameworkRuntimeReloadRequest{
			EndpointPath: "/__vorma_internal/reload-template",
		},
	)
	if callError == nil {
		t.Fatal("expected status error")
	}
	if !strings.Contains(callError.Error(), "endpoint returned 400") {
		t.Fatalf("unexpected error: %v", callError)
	}
}

func TestStartAndStopRefreshRuntime(t *testing.T) {
	startResult, startError := StartRefreshRuntime(
		0,
		newDiscardLoggerForReloadWaitTests(),
	)
	if startError != nil {
		t.Fatalf("StartRefreshRuntime returned error: %v", startError)
	}

	go startResult.RunManager()
	go startResult.RunServer()
	waitForHealthzForReloadWaitTests(t, startResult.State.Server.Addr)

	StopRefreshRuntime(startResult.State)
}

func TestStartRefreshRuntime_FallsBackFromUnavailablePreferredPort(
	t *testing.T,
) {
	preferredListener, listenError := net.Listen("tcp", "127.0.0.1:0")
	if listenError != nil {
		t.Fatalf("failed creating preferred listener: %v", listenError)
	}
	defer preferredListener.Close()
	preferredPort := preferredListener.Addr().(*net.TCPAddr).Port

	startResult, startError := StartRefreshRuntime(
		preferredPort,
		newDiscardLoggerForReloadWaitTests(),
	)
	if startError != nil {
		t.Fatalf("StartRefreshRuntime returned error: %v", startError)
	}
	defer StopRefreshRuntime(startResult.State)

	go startResult.RunManager()
	go startResult.RunServer()
	waitForHealthzForReloadWaitTests(t, startResult.State.Server.Addr)

	if startResult.State.Port == preferredPort {
		t.Fatalf(
			"expected fallback from preferred port %d, but runtime bound same port",
			preferredPort,
		)
	}
}
