package tooling

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/vormadev/vorma/lab/vitecmd"
)

func TestCallViteFilemapInvalidate_ReturnsErrorWhenViteNotRunning(t *testing.T) {
	s := &server{log: newDiscardLogger()}
	err := s.callViteFilemapInvalidate()
	if err == nil {
		t.Fatal("expected error when Vite is not running")
	}
	if !strings.Contains(err.Error(), "vite not running") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallViteFilemapInvalidate_ReturnsSuccessOn200(t *testing.T) {
	var requestedPath string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	parsedURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("failed parsing test server URL: %v", err)
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		t.Fatalf("failed parsing test server port: %v", err)
	}

	s := &server{
		log:     newDiscardLogger(),
		viteCtx: vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{DefaultPort: port}),
	}

	if err := s.callViteFilemapInvalidate(); err != nil {
		t.Fatalf("expected successful invalidate call, got error: %v", err)
	}
	if requestedPath != "/__vorma_invalidate_filemap" {
		t.Fatalf("unexpected invalidate path: %s", requestedPath)
	}
}

func TestCallViteFilemapInvalidate_ReturnsErrorOnNon200(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()

	parsedURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("failed parsing test server URL: %v", err)
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		t.Fatalf("failed parsing test server port: %v", err)
	}

	s := &server{
		log:     newDiscardLogger(),
		viteCtx: vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{DefaultPort: port}),
	}

	err = s.callViteFilemapInvalidate()
	if err == nil {
		t.Fatal("expected non-200 invalidate response to return error")
	}
	if !strings.Contains(err.Error(), "endpoint returned 500") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForVite_ReturnsTrueWhenViteContextIsNil(t *testing.T) {
	s := &server{log: newDiscardLogger()}
	if !s.waitForVite() {
		t.Fatal("expected waitForVite to return true when no vite context exists")
	}
}

func TestGetBuilderAndSetBuilder(t *testing.T) {
	s := &server{log: newDiscardLogger()}
	if s.getBuilder() != nil {
		t.Fatal("expected initial builder to be nil")
	}

	builder := &Builder{}
	s.setBuilder(builder)
	if s.getBuilder() != builder {
		t.Fatal("expected getBuilder to return the builder set by setBuilder")
	}
}
