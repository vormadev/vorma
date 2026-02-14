package wave

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewUsesProvidedOrDefaultLogger(t *testing.T) {
	fixture := newWaveTestFixture(t)
	providedLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	withProvided := New(Config{
		WaveConfigJSON: fixture.configJSON(t),
		Logger:         providedLogger,
	})
	if withProvided.Logger() != providedLogger {
		t.Fatal("expected New to use provided logger instance")
	}

	withDefault := New(Config{
		WaveConfigJSON: fixture.configJSON(t),
	})
	if withDefault.Logger() == nil {
		t.Fatal("expected New to create a default logger when none is provided")
	}
}

func TestWaveEnvWrapperMethods(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := New(Config{
		WaveConfigJSON: fixture.configJSON(t),
		Logger:         newDiscardLoggerForWaveTests(),
	})

	t.Setenv(envMode, "production")
	if w.GetIsDev() {
		t.Fatal("expected w.GetIsDev to reflect non-dev mode")
	}

	w.SetModeToDev()
	if !w.GetIsDev() {
		t.Fatal("expected w.SetModeToDev to switch to dev mode")
	}

	resetPortCacheForTest()
	t.Setenv(envPortSet, "true")
	t.Setenv(envPort, "43210")
	if got := w.MustGetPort(); got != 43210 {
		t.Fatalf("expected w.MustGetPort to return configured port, got %d", got)
	}
}

func TestMustGetHelpersSucceedWithValidSetup(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	publicFS := w.MustGetPublicFS()
	if got := mustReadFileFromFS(t, publicFS, "logo.txt"); got != "logo" {
		t.Fatalf("unexpected public FS content from MustGetPublicFS: %q", got)
	}

	privateFS := w.MustGetPrivateFS()
	if got := mustReadFileFromFS(t, privateFS, "template.html"); got != "private" {
		t.Fatalf("unexpected private FS content from MustGetPrivateFS: %q", got)
	}

	handler := w.MustGetServeStaticHandler(false)
	req := httptest.NewRequest(http.MethodGet, "/assets/logo.txt", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected MustGetServeStaticHandler result to serve file, got status %d", rec.Code)
	}
}

func TestPublicFileMapMissingOrInvalidFallsBackWithoutPanic(t *testing.T) {
	t.Run("missing gob", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		if err := os.Remove(fixture.cfg.Dist.PublicFileMapGob()); err != nil {
			t.Fatalf("failed to remove public file map gob: %v", err)
		}

		w := newWaveForTest(t, fixture, true, nil)
		_, err := w.GetPublicFileMap()
		if err == nil {
			t.Fatal("expected GetPublicFileMap to fail when gob file is missing")
		}
		if got := w.GetPublicURL("logo.txt"); got != "/assets/logo.txt" {
			t.Fatalf("expected fallback public URL when map is unavailable, got %q", got)
		}
	})

	t.Run("invalid gob", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		mustWriteFile(t, fixture.cfg.Dist.PublicFileMapGob(), "not-a-gob")

		w := newWaveForTest(t, fixture, true, nil)
		_, err := w.GetPublicFileMap()
		if err == nil {
			t.Fatal("expected GetPublicFileMap to fail for invalid gob")
		}
		if got := w.GetPublicURL("logo.txt"); got != "/assets/logo.txt" {
			t.Fatalf("expected fallback public URL when map decode fails, got %q", got)
		}
	})
}

func TestGettersHandleUnavailableBaseFSGracefully(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, nil)

	if got := w.GetPublicFileMapURL(); got != "" {
		t.Fatalf("expected empty file map URL when base FS is unavailable, got %q", got)
	}
	if got := w.GetPublicFileMapElements(); got != "" {
		t.Fatalf("expected empty file map elements when base FS is unavailable, got %q", got)
	}
	if got := w.GetPublicFileMapScriptSha256Hash(); got != "" {
		t.Fatalf("expected empty file map script hash when base FS is unavailable, got %q", got)
	}
	if got := w.GetStyleSheetURL(); got != "" {
		t.Fatalf("expected empty stylesheet URL when base FS is unavailable, got %q", got)
	}
	if got := w.GetStyleSheetLinkElement(); got != "" {
		t.Fatalf("expected empty stylesheet link when base FS is unavailable, got %q", got)
	}
	if got := w.GetCriticalCSS(); got != "" {
		t.Fatalf("expected empty critical CSS when base FS is unavailable, got %q", got)
	}
	if got := w.GetCriticalCSSStyleElement(); got != "" {
		t.Fatalf("expected empty critical CSS style element when base FS is unavailable, got %q", got)
	}
	if got := w.GetCriticalCSSStyleElementSha256Hash(); got != "" {
		t.Fatalf("expected empty critical CSS hash when base FS is unavailable, got %q", got)
	}

	if got := w.GetPublicURL("logo.txt"); got != "/assets/logo.txt" {
		t.Fatalf("expected fallback URL when base FS is unavailable, got %q", got)
	}
}

func TestStylesheetReferenceMissingReturnsEmpty(t *testing.T) {
	fixture := newWaveTestFixture(t)
	if err := os.Remove(fixture.cfg.Dist.NormalCSSRef()); err != nil {
		t.Fatalf("failed removing normal css ref file: %v", err)
	}
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.GetStyleSheetURL(); got != "" {
		t.Fatalf("expected empty stylesheet URL when ref file is missing, got %q", got)
	}
	if got := w.GetStyleSheetLinkElement(); got != "" {
		t.Fatalf("expected empty stylesheet link when ref file is missing, got %q", got)
	}
}

func TestStylesheetReferenceTrimsWhitespace(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.cfg.Dist.NormalCSSRef(), " vorma_out/vorma_internal_normal_hash.css \n")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.GetStyleSheetURL(); got != "/assets/vorma_out/vorma_internal_normal_hash.css" {
		t.Fatalf("expected trimmed stylesheet URL, got %q", got)
	}
}

func TestStylesheetReferenceWithOnlyWhitespaceReturnsEmpty(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.cfg.Dist.NormalCSSRef(), " \n\t ")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.GetStyleSheetURL(); got != "" {
		t.Fatalf("expected empty stylesheet URL for whitespace-only ref, got %q", got)
	}
	if got := w.GetStyleSheetLinkElement(); got != "" {
		t.Fatalf("expected empty stylesheet link for whitespace-only ref, got %q", got)
	}
}

func TestStylesheetReferenceTraversalStaysUnderPublicPrefix(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.cfg.Dist.NormalCSSRef(), "../outside.css")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.GetStyleSheetURL(); got != "/assets/outside.css" {
		t.Fatalf("expected stylesheet URL to stay under public prefix, got %q", got)
	}
}

func TestServeStaticWithRootPrefix(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.PublicPathPrefix = "/"
	w := newWaveForTest(t, fixture, true, nil)

	handler, err := w.GetServeStaticHandler(false)
	if err != nil {
		t.Fatalf("GetServeStaticHandler returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/logo.txt", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected root-prefix static handler status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "logo" {
		t.Fatalf("unexpected root-prefix static handler body: %q", rec.Body.String())
	}
}

func TestServeStaticMiddlewareFallsThroughForMissingPrefixedAsset(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
		rw.WriteHeader(http.StatusAccepted)
	})
	h := w.ServeStatic(false)(next)

	req := httptest.NewRequest(http.MethodGet, "/assets/not-found.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected missing prefixed asset path to fall through to next handler, got %d", rec.Code)
	}
	if !nextCalled {
		t.Fatal("expected missing prefixed asset path to call next handler")
	}
}

func TestServeStaticMiddlewareFallsThroughForRootAndDirectoryPathsWithRootPrefix(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.PublicPathPrefix = "/"
	w := newWaveForTest(t, fixture, true, nil)

	nextCalls := 0
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalls++
		rw.WriteHeader(http.StatusAccepted)
	})
	h := w.ServeStatic(false)(next)

	rootReq := httptest.NewRequest(http.MethodGet, "/", nil)
	rootRec := httptest.NewRecorder()
	h.ServeHTTP(rootRec, rootReq)
	if rootRec.Code != http.StatusAccepted {
		t.Fatalf("expected root path to fall through, got %d", rootRec.Code)
	}

	dirReq := httptest.NewRequest(http.MethodGet, "/vorma_out", nil)
	dirRec := httptest.NewRecorder()
	h.ServeHTTP(dirRec, dirReq)
	if dirRec.Code != http.StatusAccepted {
		t.Fatalf("expected directory path to fall through, got %d", dirRec.Code)
	}

	if nextCalls != 2 {
		t.Fatalf("expected next handler to be called for both root and directory paths, calls=%d", nextCalls)
	}
}

func TestIsPublicAssetReturnsFalseWhenRootPrefixPublicFSUnavailable(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.PublicPathPrefix = "/"
	w := newWaveForTest(t, fixture, false, nil)

	if w.IsPublicAsset("/logo.txt") {
		t.Fatal("expected IsPublicAsset to return false when public FS is unavailable")
	}
}

func TestPublicFileMapGettersFailClosedWhenCacheReturnsNilData(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	w.fileMapDetails = newCache(func() (*fileMapDetails, error) {
		return nil, fmt.Errorf("forced failure")
	})

	if got := w.GetPublicFileMapElements(); got != "" {
		t.Fatalf("expected empty file map elements on cache failure, got %q", got)
	}
	if got := w.GetPublicFileMapScriptSha256Hash(); got != "" {
		t.Fatalf("expected empty file map script hash on cache failure, got %q", got)
	}
}

func TestPublicFileMapReferenceTrimsWhitespace(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.cfg.Dist.PublicFileMapRef(), " vorma_out/vorma_internal_public_filemap_hash.js \n")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.GetPublicFileMapURL(); got != "/assets/vorma_out/vorma_internal_public_filemap_hash.js" {
		t.Fatalf("expected trimmed public file map URL, got %q", got)
	}
}

func TestPublicFileMapReferenceWithOnlyWhitespaceReturnsEmpty(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.cfg.Dist.PublicFileMapRef(), " \n\t ")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.GetPublicFileMapURL(); got != "" {
		t.Fatalf("expected empty public file map URL for whitespace-only ref, got %q", got)
	}
	if got := w.GetPublicFileMapElements(); got != "" {
		t.Fatalf("expected empty public file map elements for whitespace-only ref, got %q", got)
	}
	if got := w.GetPublicFileMapScriptSha256Hash(); got != "" {
		t.Fatalf("expected empty public file map hash for whitespace-only ref, got %q", got)
	}
}

func TestPublicFileMapReferenceTraversalStaysUnderPublicPrefix(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.cfg.Dist.PublicFileMapRef(), "../outside.js")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.GetPublicFileMapURL(); got != "/assets/outside.js" {
		t.Fatalf("expected public file map URL to stay under public prefix, got %q", got)
	}
}

func TestCriticalCSSReadErrorsFailClosed(t *testing.T) {
	fixture := newWaveTestFixture(t)
	if err := os.Remove(fixture.cfg.Dist.CriticalCSS()); err != nil {
		t.Fatalf("failed removing critical css file: %v", err)
	}
	if err := os.Mkdir(fixture.cfg.Dist.CriticalCSS(), 0o755); err != nil {
		t.Fatalf("failed creating directory at critical css path: %v", err)
	}

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.GetCriticalCSS(); got != "" {
		t.Fatalf("expected empty critical CSS when read returns non-not-exist error, got %q", got)
	}
	if got := w.GetCriticalCSSStyleElement(); got != "" {
		t.Fatalf("expected empty critical CSS style element when read fails, got %q", got)
	}
	if got := w.GetCriticalCSSStyleElementSha256Hash(); got != "" {
		t.Fatalf("expected empty critical CSS hash when read fails, got %q", got)
	}
}

func TestFaviconRedirectMiddlewareGuardsMethodAndPath(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
		rw.WriteHeader(http.StatusTeapot)
	})
	h := w.FaviconRedirect()(next)

	postReq := httptest.NewRequest(http.MethodPost, "/favicon.ico", nil)
	postRec := httptest.NewRecorder()
	h.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusTeapot || !nextCalled {
		t.Fatalf("expected POST /favicon.ico to fall through to next, status=%d nextCalled=%v", postRec.Code, nextCalled)
	}

	nextCalled = false
	otherReq := httptest.NewRequest(http.MethodGet, "/not-favicon.ico", nil)
	otherRec := httptest.NewRecorder()
	h.ServeHTTP(otherRec, otherReq)
	if otherRec.Code != http.StatusTeapot || !nextCalled {
		t.Fatalf("expected GET non-favicon path to fall through to next, status=%d nextCalled=%v", otherRec.Code, nextCalled)
	}

	headReq := httptest.NewRequest(http.MethodHead, "/favicon.ico", nil)
	headRec := httptest.NewRecorder()
	h.ServeHTTP(headRec, headReq)
	if headRec.Code != http.StatusFound {
		t.Fatalf("expected HEAD /favicon.ico to redirect, got status %d", headRec.Code)
	}
	if !strings.Contains(headRec.Header().Get("Location"), "favicon") {
		t.Fatalf("expected favicon location header on HEAD redirect, got %q", headRec.Header().Get("Location"))
	}
}

func TestFaviconRedirectMiddlewareSupportsIdentityMappedFavicon(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteGob(t, fixture.cfg.Dist.PublicFileMapGob(), FileMap{
		"favicon.ico": {
			DistName: "favicon.ico",
		},
	})

	w := newWaveForTest(t, fixture, true, nil)
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusTeapot)
	})
	h := w.FaviconRedirect()(next)

	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect status 302 for identity-mapped favicon, got %d", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/assets/favicon.ico" {
		t.Fatalf("unexpected redirect location for identity-mapped favicon: %q", location)
	}
}
