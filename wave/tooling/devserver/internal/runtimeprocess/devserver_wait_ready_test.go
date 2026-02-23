package runtimeprocess_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/wave/tooling/devserver/internal/runtimeprocess"
)

func TestWaitForReady_RetriesUntilServerIsHealthy(t *testing.T) {
	var attempts atomic.Int32

	testServer := httptest.NewServer(
		http.HandlerFunc(
			func(responseWriter http.ResponseWriter, _ *http.Request) {
				currentAttempt := attempts.Add(1)
				if currentAttempt < 3 {
					responseWriter.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				responseWriter.WriteHeader(http.StatusOK)
			},
		),
	)
	defer testServer.Close()

	policy := runtimeprocess.ReadinessWaitPolicy{
		HTTPClientTimeout: 150 * time.Millisecond,
		InitialDelay:      10 * time.Millisecond,
		MaximumDelay:      30 * time.Millisecond,
		MaximumTotalWait:  2 * time.Second,
		TreatHTTPStatusCodeAsReady: func(statusCode int) bool {
			return statusCode >= 200 && statusCode < 400
		},
	}

	if !runtimeprocess.WaitForAnyReady([]string{testServer.URL}, policy) {
		t.Fatal("expected WaitForAnyReady to return true after retries")
	}

	if attempts.Load() < 3 {
		t.Fatalf("expected at least 3 attempts, got %d", attempts.Load())
	}
}

func TestWaitForAnyReadyWithContext_CancellationStopsWaitEarly(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	requestCanceled := make(chan struct{}, 1)

	testServer := httptest.NewServer(
		http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			select {
			case requestStarted <- struct{}{}:
			default:
			}
			<-request.Context().Done()
			select {
			case requestCanceled <- struct{}{}:
			default:
			}
		}),
	)
	defer testServer.Close()

	readinessContext, cancelReadiness := context.WithCancel(
		context.Background(),
	)
	defer cancelReadiness()

	waitResultCh := make(chan bool, 1)
	go func() {
		waitResultCh <- runtimeprocess.WaitForAnyReadyWithContext(
			readinessContext,
			[]string{testServer.URL},
			runtimeprocess.ReadinessWaitPolicy{
				HTTPClientTimeout: 5 * time.Second,
				InitialDelay:      200 * time.Millisecond,
				MaximumDelay:      200 * time.Millisecond,
				MaximumTotalWait:  10 * time.Second,
				TreatHTTPStatusCodeAsReady: func(statusCode int) bool {
					return statusCode >= 200 && statusCode < 400
				},
			},
		)
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for initial readiness probe request")
	}

	cancelStart := time.Now()
	cancelReadiness()

	select {
	case waitResult := <-waitResultCh:
		if waitResult {
			t.Fatal("expected canceled readiness wait to return false")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for canceled readiness wait to return")
	}

	select {
	case <-requestCanceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for readiness probe request cancellation")
	}

	if cancelElapsed := time.Since(cancelStart); cancelElapsed > 450*time.Millisecond {
		t.Fatalf(
			"expected canceled readiness wait to return promptly, took %s",
			cancelElapsed,
		)
	}
}
