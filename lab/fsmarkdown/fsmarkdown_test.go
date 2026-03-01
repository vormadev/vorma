package fsmarkdown

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPlainTextMiddleware_MatchesAcceptHeaderCaseInsensitively(t *testing.T) {
	instance := newFSMarkdownInstanceForMiddlewareTest(
		t,
		fstest.MapFS{
			"markdown/_index.md": {Data: []byte("hello from markdown")},
		},
	)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := instance.PlainTextMiddleware("/*")(next)

	request := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	request.Header.Set("Accept", "TEXT/PLAIN")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "text/plain; charset=utf-8")
	}
	if !strings.Contains(recorder.Body.String(), "hello from markdown") {
		t.Fatalf("response body = %q, expected markdown content", recorder.Body.String())
	}
}

func TestPageDetails_MissingDotWellKnownPathReturnsNotFoundWithoutError(
	t *testing.T,
) {
	instance := newFSMarkdownInstanceForMiddlewareTest(
		t,
		fstest.MapFS{
			"markdown/_index.md": {Data: []byte("home")},
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"http://example.com/.well-known/appspecific/com.chrome.devtools.json",
		nil,
	)
	pageDetails, pageDetailsError := instance.PageDetails(request)
	if pageDetailsError != nil {
		t.Fatalf("PageDetails() returned error: %v", pageDetailsError)
	}
	if pageDetails == nil || pageDetails.Page == nil {
		t.Fatal("expected PageDetails() to return a not-found page")
	}
	if pageDetails.Title != "Error" {
		t.Fatalf("expected not-found page title \"Error\", got %q", pageDetails.Title)
	}
	if len(pageDetails.Sitemap) != 0 {
		t.Fatalf("expected empty sitemap for missing path, got %#v", pageDetails.Sitemap)
	}
}

func newFSMarkdownInstanceForMiddlewareTest(
	t *testing.T,
	fileSystem fs.FS,
) *Instance {
	t.Helper()

	return New(Options{
		FS: fileSystem,
		FrontmatterParser: func(reader io.Reader, destination any) ([]byte, error) {
			return io.ReadAll(reader)
		},
		MarkdownParser: func(markdown []byte, writer io.Writer) error {
			_, writeErr := writer.Write(markdown)
			return writeErr
		},
	})
}
