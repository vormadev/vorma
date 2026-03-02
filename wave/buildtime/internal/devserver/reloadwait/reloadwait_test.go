package reloadwait

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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave/buildtime/internal/broadcast"
	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/eventpipeline"
	"github.com/vormadev/vorma/wave/waveframework"
	"github.com/vormadev/vorma/wave/wavewatch"
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

func TestNotifyViteFileMapChangedWithContext(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	port := parsePortFromRawURLForReloadWaitTests(t, server.URL)
	if callError := NotifyViteFileMapChangedWithContext(context.Background(), port); callError != nil {
		t.Fatalf("expected success, got error: %v", callError)
	}
	if requestedPath != waveframework.ViteFileMapChangedNotifyEndpointPath {
		t.Fatalf(
			"requestedPath=%q, want %q",
			requestedPath,
			waveframework.ViteFileMapChangedNotifyEndpointPath,
		)
	}
}

func TestNotifyViteFileMapChangedWithContext_ReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
	)
	defer server.Close()

	port := parsePortFromRawURLForReloadWaitTests(t, server.URL)
	callError := NotifyViteFileMapChangedWithContext(
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
		wavewatch.FrameworkRuntimeReloadRequest{
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
	if got, want := requestHeaders.Get(waveframework.FrameworkRuntimeReloadAttemptIDHeaderName), "attempt-7"; got != want {
		t.Fatalf("reload attempt header=%q, want %q", got, want)
	}
	if got, want := requestHeaders.Get(waveframework.FrameworkRuntimeReloadExpectedBuildIDHeaderName), "build-7"; got != want {
		t.Fatalf("expected build ID header=%q, want %q", got, want)
	}
	if got, want := requestHeaders.Get(waveframework.FrameworkRuntimeReloadTriggerHeaderName), "dev-reload"; got != want {
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
		wavewatch.FrameworkRuntimeReloadRequest{
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

func TestStartRefreshRuntime_HealthzReachableOnLocalhostAndIPv4Loopback(
	t *testing.T,
) {
	startResult, startError := StartRefreshRuntime(
		0,
		newDiscardLoggerForReloadWaitTests(),
	)
	if startError != nil {
		t.Fatalf("StartRefreshRuntime returned error: %v", startError)
	}
	defer StopRefreshRuntime(startResult.State)

	go startResult.RunManager()
	go startResult.RunServer()

	client := &http.Client{Timeout: 500 * time.Millisecond}
	for _, healthzURL := range []string{
		fmt.Sprintf("http://localhost:%d/healthz", startResult.State.Port),
		fmt.Sprintf("http://127.0.0.1:%d/healthz", startResult.State.Port),
	} {
		var lastError error
		ready := false
		for range 40 {
			response, requestError := client.Get(healthzURL)
			if requestError == nil {
				_ = response.Body.Close()
				if response.StatusCode == http.StatusOK {
					ready = true
					break
				}
				lastError = fmt.Errorf(
					"status=%d",
					response.StatusCode,
				)
			} else {
				lastError = requestError
			}
			time.Sleep(25 * time.Millisecond)
		}
		if !ready {
			t.Fatalf("healthz never became ready for %s: %v", healthzURL, lastError)
		}
	}
}

func TestStartRefreshRuntime_FallsBackFromUnavailablePreferredPort(
	t *testing.T,
) {
	preferredListener, listenError := net.Listen("tcp", ":0")
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

func TestBroadcastReloadAfterReadinessWithGeneration_ReadinessFailureTriggersNoGoRestart(
	t *testing.T,
) {
	var triggerRestartNoGoCallCount int
	var broadcastCallCount int

	BroadcastReloadAfterReadinessWithGeneration(
		BroadcastReloadAfterReadinessWithGenerationOptions{
			ReadinessContext:          context.Background(),
			ReloadBroadcastGeneration: 1,
			ReloadOptions: eventpipeline.ReloadOpts{
				Payload: broadcast.Payload{
					ChangeType: broadcast.ChangeTypeOther,
				},
				WaitApp: true,
			},
			IsGenerationCurrent: func(uint64) bool { return true },
			WaitForReloadReadiness: func(
				context.Context,
				eventpipeline.ReloadOpts,
			) bool {
				return false
			},
			TriggerRestartNoGo: func() {
				triggerRestartNoGoCallCount++
			},
			BroadcastReloadPayloadIfGenerationCurrent: func(
				uint64,
				broadcast.Payload,
			) {
				broadcastCallCount++
			},
			Log: newDiscardLoggerForReloadWaitTests(),
		},
	)

	if triggerRestartNoGoCallCount != 1 {
		t.Fatalf(
			"expected one TriggerRestartNoGo call, got %d",
			triggerRestartNoGoCallCount,
		)
	}
	if broadcastCallCount != 0 {
		t.Fatalf(
			"expected no broadcast on readiness failure, got %d calls",
			broadcastCallCount,
		)
	}
}

func TestWaitForReloadReadiness_CycleViteContract(t *testing.T) {
	testCases := []struct {
		name      string
		options   WaitForReloadReadinessOptions
		shouldRun bool
	}{
		{
			name: "cycle fails when vite is disabled",
			options: WaitForReloadReadinessOptions{
				ReloadOptions: eventpipeline.ReloadOpts{CycleVite: true},
				UsingVite:     false,
			},
			shouldRun: false,
		},
		{
			name: "cycle fails when vite runtime is unavailable",
			options: WaitForReloadReadinessOptions{
				ReloadOptions: eventpipeline.ReloadOpts{CycleVite: true},
				UsingVite:     true,
				IsViteRunning: func() bool { return false },
			},
			shouldRun: false,
		},
		{
			name: "cycle fails when callback is missing",
			options: WaitForReloadReadinessOptions{
				ReloadOptions: eventpipeline.ReloadOpts{CycleVite: true},
				UsingVite:     true,
				IsViteRunning: func() bool { return true },
			},
			shouldRun: false,
		},
		{
			name: "cycle failure returns false",
			options: WaitForReloadReadinessOptions{
				ReloadOptions: eventpipeline.ReloadOpts{CycleVite: true},
				UsingVite:     true,
				IsViteRunning: func() bool { return true },
				CycleViteAndWaitForReadinessWithContext: func(
					context.Context,
				) bool {
					return false
				},
			},
			shouldRun: false,
		},
		{
			name: "cycle success continues to wait gates",
			options: WaitForReloadReadinessOptions{
				ReloadOptions: eventpipeline.ReloadOpts{
					CycleVite: true,
					WaitApp:   true,
					WaitVite:  true,
				},
				UsingVite: true,
				IsViteRunning: func() bool {
					return true
				},
				CycleViteAndWaitForReadinessWithContext: func(
					context.Context,
				) bool {
					return true
				},
				WaitForAppWithContext: func(context.Context) bool {
					return true
				},
				WaitForViteWithContext: func(context.Context) bool {
					return true
				},
			},
			shouldRun: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := WaitForReloadReadiness(testCase.options)
			if result != testCase.shouldRun {
				t.Fatalf("result=%v, want %v", result, testCase.shouldRun)
			}
		})
	}
}

func TestShouldBroadcastReloadPayloadAfterReadiness(t *testing.T) {
	if !ShouldBroadcastReloadPayloadAfterReadiness(
		eventpipeline.ReloadOpts{CycleVite: false},
	) {
		t.Fatal("expected non-cycle reload to broadcast payload")
	}

	if ShouldBroadcastReloadPayloadAfterReadiness(
		eventpipeline.ReloadOpts{CycleVite: true},
	) {
		t.Fatal("expected cycle-vite reload to skip payload broadcast")
	}
}

func TestNotifyViteFileMapChangedWithContext_UsesClientTimeout(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(localReloadHTTPRequestTimeout + 300*time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	port := parsePortFromRawURLForReloadWaitTests(t, server.URL)
	start := time.Now()
	callError := NotifyViteFileMapChangedWithContext(context.Background(), port)
	elapsed := time.Since(start)

	if callError == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(callError, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded error, got %v", callError)
	}
	if elapsed > localReloadHTTPRequestTimeout+time.Second {
		t.Fatalf(
			"expected request to fail near timeout budget; elapsed=%s timeout=%s",
			elapsed,
			localReloadHTTPRequestTimeout,
		)
	}
}

func TestCallFrameworkRuntimeReloadEndpointWithContext_UsesClientTimeout(
	t *testing.T,
) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(localReloadHTTPRequestTimeout + 300*time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	port := parsePortFromRawURLForReloadWaitTests(t, server.URL)
	start := time.Now()
	callError := CallFrameworkRuntimeReloadEndpointWithContext(
		context.Background(),
		port,
		wavewatch.FrameworkRuntimeReloadRequest{
			EndpointPath: "/__vorma_internal/reload-template",
		},
	)
	elapsed := time.Since(start)

	if callError == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(callError, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded error, got %v", callError)
	}
	if elapsed > localReloadHTTPRequestTimeout+time.Second {
		t.Fatalf(
			"expected request to fail near timeout budget; elapsed=%s timeout=%s",
			elapsed,
			localReloadHTTPRequestTimeout,
		)
	}
}
