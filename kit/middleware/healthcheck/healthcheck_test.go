package healthcheck

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzMiddleware(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusAccepted)
	})

	mw := Healthz(next)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "OK" {
		t.Fatalf("expected body OK, got %q", w.Body.String())
	}
	if nextCalled {
		t.Fatal("expected healthcheck middleware to short-circuit next handler")
	}
}

func TestHealthzMiddlewarePassThrough(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusAccepted)
	})

	mw := Healthz(next)
	req := httptest.NewRequest(http.MethodGet, "/other", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if !nextCalled {
		t.Fatal("expected middleware to pass through unmatched endpoint")
	}
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected passthrough status 202, got %d", w.Code)
	}
}
