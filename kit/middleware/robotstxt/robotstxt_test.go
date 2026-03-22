package robotstxt

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllowMiddleware(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusTeapot)
	})

	mw := Allow(next)
	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "User-agent: *\nAllow: /" {
		t.Fatalf("unexpected robots content: %q", w.Body.String())
	}
	if nextCalled {
		t.Fatal("expected robots middleware to short-circuit next handler")
	}
}

func TestRobotsMiddlewarePassThrough(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusAccepted)
	})

	mw := Disallow(next)
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
