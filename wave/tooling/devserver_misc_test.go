package tooling

import (
	"github.com/vormadev/vorma/wave/tooling/devserver"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave/internal/waveshared"
	"github.com/vormadev/vorma/wave/tooling/builder"
)

func TestCallViteFilemapInvalidate_ReturnsErrorWhenViteNotRunning(
	t *testing.T,
) {
	s := &devserver.Server{Log: newDiscardLogger()}
	err := s.CallViteFilemapInvalidate()
	if err == nil {
		t.Fatal("expected error when Vite is not running")
	}
	if !strings.Contains(err.Error(), "vite not running") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCallViteFilemapInvalidate_ReturnsSuccessOn200(t *testing.T) {
	var requestedPath string
	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}),
	)
	defer testServer.Close()

	parsedURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("failed parsing test server URL: %v", err)
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		t.Fatalf("failed parsing test server port: %v", err)
	}

	s := &devserver.Server{
		Log: newDiscardLogger(),
		ViteCtx: vitecmd.NewBuildCtx(
			&vitecmd.BuildCtxOptions{DefaultPort: port},
		),
	}

	if err := s.CallViteFilemapInvalidate(); err != nil {
		t.Fatalf("expected successful invalidate call, got error: %v", err)
	}
	if requestedPath != "/__vorma_invalidate_filemap" {
		t.Fatalf("unexpected invalidate path: %s", requestedPath)
	}
}

func TestCallViteFilemapInvalidate_ReturnsErrorOnNon200(t *testing.T) {
	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}),
	)
	defer testServer.Close()

	parsedURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("failed parsing test server URL: %v", err)
	}
	port, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		t.Fatalf("failed parsing test server port: %v", err)
	}

	s := &devserver.Server{
		Log: newDiscardLogger(),
		ViteCtx: vitecmd.NewBuildCtx(
			&vitecmd.BuildCtxOptions{DefaultPort: port},
		),
	}

	err = s.CallViteFilemapInvalidate()
	if err == nil {
		t.Fatal("expected non-200 invalidate response to return error")
	}
	if !strings.Contains(err.Error(), "endpoint returned 500") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForVite_ReturnsTrueWhenViteContextIsNil(t *testing.T) {
	s := &devserver.Server{Log: newDiscardLogger()}
	if !s.WaitForVite() {
		t.Fatal(
			"expected waitForVite to return true when no vite context exists",
		)
	}
}

func TestResolveViteReadyURL_UsesIPv4LoopbackHost(t *testing.T) {
	got := devserver.ResolveViteReadyURL(5151)
	const want = "http://127.0.0.1:5151/@vite/client"
	if got != want {
		t.Fatalf("devserver.ResolveViteReadyURL()=%q, want %q", got, want)
	}
}

func TestResolveViteReadyURLs_UsesLoopbackHosts(t *testing.T) {
	got := devserver.ResolveViteReadyURLs(5151)
	want := []string{
		"http://127.0.0.1:5151/@vite/client",
		"http://localhost:5151/@vite/client",
	}
	if len(got) != len(want) {
		t.Fatalf("devserver.ResolveViteReadyURLs() len=%d, want %d", len(got), len(want))
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf(
				"devserver.ResolveViteReadyURLs()[%d]=%q, want %q",
				idx,
				got[idx],
				want[idx],
			)
		}
	}
}

func TestGetBuilderAndSetBuilder(t *testing.T) {
	s := &devserver.Server{Log: newDiscardLogger()}
	if s.GetBuilder() != nil {
		t.Fatal("expected initial builder to be nil")
	}

	builder := &toolingbuilder.Builder{}
	s.SetBuilder(builder)
	if s.GetBuilder() != builder {
		t.Fatal("expected getBuilder to return the builder set by setBuilder")
	}
}

func TestServerPortUsesOwnedResolverState(t *testing.T) {
	t.Setenv("__WAVE_MODE", "production")
	t.Setenv("__WAVE_PORT_HAS_BEEN_SET", "true")
	t.Setenv("PORT", "6001")

	s := &devserver.Server{
		Log:          newDiscardLogger(),
		PortResolver: waveshared.NewResolver(),
	}

	if got := s.MustGetPort(); got != 6001 {
		t.Fatalf("expected server resolver to return 6001, got %d", got)
	}

	t.Setenv("PORT", "6002")
	if got := s.MustGetPort(); got != 6001 {
		t.Fatalf(
			"expected server resolver to cache first value 6001, got %d",
			got,
		)
	}

	s.PortResolver = waveshared.NewResolver()
	if got := s.MustGetPort(); got != 6002 {
		t.Fatalf(
			"expected refreshed server resolver to return 6002, got %d",
			got,
		)
	}
}
