package runtimeprocess_test

import (
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
		http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
			currentAttempt := attempts.Add(1)
			if currentAttempt < 3 {
				responseWriter.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			responseWriter.WriteHeader(http.StatusOK)
		}),
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
