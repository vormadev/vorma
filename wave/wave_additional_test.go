package wave

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
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
	if w.IsDev() {
		t.Fatal("expected w.GetIsDev to reflect non-dev mode")
	}

	w.SetModeToDev()
	if !w.IsDev() {
		t.Fatal("expected w.SetModeToDev to switch to dev mode")
	}

	resetPortCacheForTest()
	t.Setenv(envPortSet, "true")
	t.Setenv(envPort, "43210")
	if got := w.MustGetPort(); got != 43210 {
		t.Fatalf("expected w.Port to return configured port, got %d", got)
	}
}

func TestMustGetHelpersSucceedWithValidSetup(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	publicFS, publicFSError := w.getPublicFS()
	if publicFSError != nil {
		t.Fatalf("unexpected getPublicFS error: %v", publicFSError)
	}
	if got := mustReadFileFromFS(t, publicFS, "logo.txt"); got != "logo" {
		t.Fatalf("unexpected public FS content from getPublicFS: %q", got)
	}

	privateFS := w.MustPrivateFS()
	if got := mustReadFileFromFS(t, privateFS, "template.html"); got != "private" {
		t.Fatalf("unexpected private FS content from MustPrivateFS: %q", got)
	}

	handler, handlerError := w.staticHandler(false)
	if handlerError != nil {
		t.Fatalf("unexpected staticHandler error: %v", handlerError)
	}
	req := httptest.NewRequest(http.MethodGet, "/assets/logo.txt", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf(
			"expected staticHandler result to serve file, got status %d",
			rec.Code,
		)
	}
}

func TestPublicFileMapMissingOrInvalidReturnsEmptyPublicURLWithoutPanic(
	t *testing.T,
) {
	t.Run("missing gob", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		if err := os.Remove(fixture.cfg.Dist.PublicFileMapGob()); err != nil {
			t.Fatalf("failed to remove public file map gob: %v", err)
		}

		w := newWaveForTest(t, fixture, true, nil)
		_, err := w.publicFileMap()
		if err == nil {
			t.Fatal("expected publicFileMap to fail when gob file is missing")
		}
		if got := w.PublicURL("logo.txt"); got != "" {
			t.Fatalf(
				"expected empty public URL when map is unavailable, got %q",
				got,
			)
		}
	})

	t.Run("invalid gob", func(t *testing.T) {
		fixture := newWaveTestFixture(t)
		mustWriteFile(t, fixture.cfg.Dist.PublicFileMapGob(), "not-a-gob")

		w := newWaveForTest(t, fixture, true, nil)
		_, err := w.publicFileMap()
		if err == nil {
			t.Fatal("expected publicFileMap to fail for invalid gob")
		}
		if got := w.PublicURL("logo.txt"); got != "" {
			t.Fatalf(
				"expected empty public URL when map decode fails, got %q",
				got,
			)
		}
	})
}

func TestGettersHandleUnavailableBaseFSGracefully(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, nil)

	if got := w.publicFileMapURL(); got != "" {
		t.Fatalf(
			"expected empty file map URL when base FS is unavailable, got %q",
			got,
		)
	}
	fileMapDetails := readFileMapDetailsFromCacheForTest(w)
	if fileMapDetails == nil {
		t.Fatal("expected public file map details cache value")
	}
	if got := fileMapDetails.elements; got != "" {
		t.Fatalf(
			"expected empty file map elements when base FS is unavailable, got %q",
			got,
		)
	}
	if got := w.styleSheetURL(); got != "" {
		t.Fatalf(
			"expected empty stylesheet URL when base FS is unavailable, got %q",
			got,
		)
	}
	if got := w.StyleSheetLinkElement(); got != "" {
		t.Fatalf(
			"expected empty stylesheet link when base FS is unavailable, got %q",
			got,
		)
	}
	if got := w.CriticalCSS(); got != "" {
		t.Fatalf(
			"expected empty critical CSS when base FS is unavailable, got %q",
			got,
		)
	}
	if got := w.CriticalCSSStyleElement(); got != "" {
		t.Fatalf(
			"expected empty critical CSS style element when base FS is unavailable, got %q",
			got,
		)
	}
	if data := w.getCriticalCSSData(); data != nil {
		t.Fatalf(
			"expected nil critical CSS data when base FS is unavailable, got %#v",
			data,
		)
	}

	if got := w.PublicURL("logo.txt"); got != "" {
		t.Fatalf(
			"expected empty public URL when base FS is unavailable, got %q",
			got,
		)
	}
}

func TestStylesheetReferenceMissingReturnsEmpty(t *testing.T) {
	fixture := newWaveTestFixture(t)
	if err := os.Remove(fixture.cfg.Dist.NormalCSSRef()); err != nil {
		t.Fatalf("failed removing normal css ref file: %v", err)
	}
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.styleSheetURL(); got != "" {
		t.Fatalf(
			"expected empty stylesheet URL when ref file is missing, got %q",
			got,
		)
	}
	if got := w.StyleSheetLinkElement(); got != "" {
		t.Fatalf(
			"expected empty stylesheet link when ref file is missing, got %q",
			got,
		)
	}
}

func TestStylesheetReferenceTrimsWhitespace(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(
		t,
		fixture.cfg.Dist.NormalCSSRef(),
		" vorma_out/vorma_internal_normal_hash.css \n",
	)

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.styleSheetURL(); got != "/assets/vorma_out/vorma_internal_normal_hash.css" {
		t.Fatalf("expected trimmed stylesheet URL, got %q", got)
	}
}

func TestStylesheetReferenceWithOnlyWhitespaceReturnsEmpty(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.cfg.Dist.NormalCSSRef(), " \n\t ")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.styleSheetURL(); got != "" {
		t.Fatalf(
			"expected empty stylesheet URL for whitespace-only ref, got %q",
			got,
		)
	}
	if got := w.StyleSheetLinkElement(); got != "" {
		t.Fatalf(
			"expected empty stylesheet link for whitespace-only ref, got %q",
			got,
		)
	}
}

func TestStylesheetReferenceTraversalStaysUnderPublicPrefix(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.cfg.Dist.NormalCSSRef(), "../outside.css")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.styleSheetURL(); got != "/assets/outside.css" {
		t.Fatalf(
			"expected stylesheet URL to stay under public prefix, got %q",
			got,
		)
	}
}

func TestServeStaticWithRootPrefix(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.PublicPathPrefix = "/"
	w := newWaveForTest(t, fixture, true, nil)

	handler, err := w.staticHandler(false)
	if err != nil {
		t.Fatalf("staticHandler returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/logo.txt", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"expected root-prefix static handler status 200, got %d",
			rec.Code,
		)
	}
	if rec.Body.String() != "logo" {
		t.Fatalf(
			"unexpected root-prefix static handler body: %q",
			rec.Body.String(),
		)
	}
}

func TestServeStaticMiddlewareFallsThroughForMissingPrefixedAsset(
	t *testing.T,
) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
		rw.WriteHeader(http.StatusAccepted)
	})
	h := w.MustStaticMiddleware(false)(next)

	req := httptest.NewRequest(http.MethodGet, "/assets/not-found.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected missing prefixed asset path to fall through to next handler, got %d",
			rec.Code,
		)
	}
	if !nextCalled {
		t.Fatal("expected missing prefixed asset path to call next handler")
	}
}

func TestServeStaticMiddlewareFallsThroughForRootAndDirectoryPathsWithRootPrefix(
	t *testing.T,
) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.PublicPathPrefix = "/"
	w := newWaveForTest(t, fixture, true, nil)

	nextCalls := 0
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalls++
		rw.WriteHeader(http.StatusAccepted)
	})
	h := w.MustStaticMiddleware(false)(next)

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
		t.Fatalf(
			"expected next handler to be called for both root and directory paths, calls=%d",
			nextCalls,
		)
	}
}

func TestIsPublicAssetReturnsFalseWhenRootPrefixPublicFSUnavailable(
	t *testing.T,
) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.PublicPathPrefix = "/"
	w := newWaveForTest(t, fixture, false, nil)

	if w.isPublicAsset("/logo.txt") {
		t.Fatal(
			"expected isPublicAsset to return false when public FS is unavailable",
		)
	}
}

func TestPublicFileMapGettersFailClosedWhenCacheReturnsNilData(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	w.fileMapDetails = newCache(func() (*fileMapDetails, error) {
		return nil, fmt.Errorf("forced failure")
	})

	fileMapDetails := readFileMapDetailsFromCacheForTest(w)
	if fileMapDetails != nil {
		t.Fatalf(
			"expected nil file map details on cache failure, got %#v",
			fileMapDetails,
		)
	}
}

func TestPublicFileMapReferenceTrimsWhitespace(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(
		t,
		fixture.cfg.Dist.PublicFileMapRef(),
		" vorma_out/vorma_internal_public_filemap_hash.js \n",
	)

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.publicFileMapURL(); got != "/assets/vorma_out/vorma_internal_public_filemap_hash.js" {
		t.Fatalf("expected trimmed public file map URL, got %q", got)
	}
}

func TestPublicFileMapReferenceWithOnlyWhitespaceReturnsEmpty(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.cfg.Dist.PublicFileMapRef(), " \n\t ")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.publicFileMapURL(); got != "" {
		t.Fatalf(
			"expected empty public file map URL for whitespace-only ref, got %q",
			got,
		)
	}
	fileMapDetails := readFileMapDetailsFromCacheForTest(w)
	if fileMapDetails == nil {
		t.Fatal("expected public file map details cache value")
	}
	if got := fileMapDetails.elements; got != "" {
		t.Fatalf(
			"expected empty public file map elements for whitespace-only ref, got %q",
			got,
		)
	}
}

func TestPublicFileMapReferenceTraversalStaysUnderPublicPrefix(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.cfg.Dist.PublicFileMapRef(), "../outside.js")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.publicFileMapURL(); got != "/assets/outside.js" {
		t.Fatalf(
			"expected public file map URL to stay under public prefix, got %q",
			got,
		)
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
	if got := w.CriticalCSS(); got != "" {
		t.Fatalf(
			"expected empty critical CSS when read returns non-not-exist error, got %q",
			got,
		)
	}
	if got := w.CriticalCSSStyleElement(); got != "" {
		t.Fatalf(
			"expected empty critical CSS style element when read fails, got %q",
			got,
		)
	}
}
