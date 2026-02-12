package tooling

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestWaitForReady_RetriesUntilServerIsHealthy(t *testing.T) {
	var attempts atomic.Int32

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := attempts.Add(1)
		if current < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	s := &server{log: newDiscardLogger()}
	if !s.waitForReady(testServer.URL) {
		t.Fatal("expected waitForReady to return true after retries")
	}

	if attempts.Load() < 3 {
		t.Fatalf("expected at least 3 attempts, got %d", attempts.Load())
	}
}
