package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestToHandlerMiddleware_UsesEndpointHandlerWhenRequestMatches(t *testing.T) {
	mw := ToHandlerMiddleware(
		"/healthz",
		[]string{http.MethodGet, http.MethodHead},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusTeapot)
	})

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	mw(next).ServeHTTP(recorder, request)

	if nextCalled {
		t.Fatal("expected middleware to bypass next handler for matching request")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

func TestToHandlerMiddleware_DelegatesToNextWhenRequestDoesNotMatch(t *testing.T) {
	mw := ToHandlerMiddleware(
		"/healthz",
		[]string{http.MethodGet},
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		},
	)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusTeapot)
	})

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "method mismatch", method: http.MethodPost, path: "/healthz"},
		{name: "path mismatch", method: http.MethodGet, path: "/other"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			nextCalled = false
			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			recorder := httptest.NewRecorder()

			mw(next).ServeHTTP(recorder, request)

			if !nextCalled {
				t.Fatal("expected next handler to be called for non-matching request")
			}
			if recorder.Code != http.StatusTeapot {
				t.Fatalf("expected status %d, got %d", http.StatusTeapot, recorder.Code)
			}
		})
	}
}
