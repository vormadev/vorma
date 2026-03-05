package wave

import (
	"fmt"
	"github.com/vormadev/vorma/internal/wavetest"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/vormadev/vorma/wave/internal/wavecache"
	"github.com/vormadev/vorma/wave/internal/waveruntimecore"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveenv"
)

func TestNewUsesProvidedOrDefaultLogger(t *testing.T) {
	fixture := newWaveTestFixture(t)
	t.Chdir(fixture.root)
	providedLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	configPath := fixture.mustWriteConfigFile(t)
	configFS := os.DirFS(fixture.root)

	withProvided := New(Config{
		FS:         configFS,
		ConfigPath: configPath,
		Logger:     providedLogger,
	})
	if withProvided.Logger() != providedLogger {
		t.Fatal("expected New to use provided logger instance")
	}

	withDefault := New(Config{
		FS:         configFS,
		ConfigPath: configPath,
	})
	if withDefault.Logger() == nil {
		t.Fatal("expected New to create a default logger when none is provided")
	}
}

func TestWaveEnvWrapperMethods(t *testing.T) {
	fixture := newWaveTestFixture(t)
	t.Chdir(fixture.root)
	configPath := fixture.mustWriteConfigFile(t)
	w := New(Config{
		FS:         os.DirFS(fixture.root),
		ConfigPath: configPath,
		Logger:     newDiscardLoggerForWaveTests(),
	})

	t.Setenv(waveenv.EnvMode, "production")
	if w.IsDev() {
		t.Fatal("expected w.GetIsDev to reflect non-dev mode")
	}

	w.SetModeToDev()
	if !w.IsDev() {
		t.Fatal("expected w.SetModeToDev to switch to dev mode")
	}

	resetPortCacheForTest()
	t.Setenv(waveenv.EnvPortSet, "true")
	t.Setenv(waveenv.EnvPort, "43210")
	if got := w.MustGetPort(); got != 43210 {
		t.Fatalf("expected w.Port to return configured port, got %d", got)
	}
}

func TestMustGetHelpersSucceedWithValidSetup(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	publicFS, publicFSError := w.runtime.GetPublicFS()
	if publicFSError != nil {
		t.Fatalf("unexpected getPublicFS error: %v", publicFSError)
	}
	if got := mustReadFileFromFS(t, publicFS, "logo.txt"); got != "logo" {
		t.Fatalf("unexpected public FS content from getPublicFS: %q", got)
	}

	privateFS := w.MustPrivateFS()
	if got := mustReadFileFromFS(
		t,
		privateFS,
		"template.html",
	); got != waveartifacts.PrivateDirname {
		t.Fatalf("unexpected private FS content from MustPrivateFS: %q", got)
	}

	handler, handlerError := w.runtime.StaticHandler(false)
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
		if err := os.Remove(fixture.pathInRoot(fixture.cfg.Dist().PublicFileMapGob())); err != nil {
			t.Fatalf("failed to remove public file map gob: %v", err)
		}

		w := newWaveForTest(t, fixture, true, nil)
		_, err := w.runtime.PublicFileMap()
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
		mustWriteFile(t, fixture.pathInRoot(fixture.cfg.Dist().PublicFileMapGob()), "not-a-gob")

		w := newWaveForTest(t, fixture, true, nil)
		_, err := w.runtime.PublicFileMap()
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
	if removeErr := os.RemoveAll(fixture.pathInRoot(fixture.cfg.Dist().Static())); removeErr != nil {
		t.Fatalf("remove dist static directory: %v", removeErr)
	}

	if got := w.runtime.PublicFileMapURL(); got != "" {
		t.Fatalf(
			"expected empty file map URL when base FS is unavailable, got %q",
			got,
		)
	}
	fileMapDetails := readFileMapDetailsFromCacheForTest(w)
	if fileMapDetails == nil {
		t.Fatal("expected public file map details cache value")
	}
	if got := fileMapDetails.Elements; got != "" {
		t.Fatalf(
			"expected empty file map elements when base FS is unavailable, got %q",
			got,
		)
	}
	if got := w.runtime.StyleSheetURL(); got != "" {
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
	if data := w.runtime.GetCriticalCSSData(); data != nil {
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
	if err := os.Remove(fixture.pathInRoot(fixture.cfg.Dist().NormalCSSRef())); err != nil {
		t.Fatalf("failed removing normal css ref file: %v", err)
	}
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.runtime.StyleSheetURL(); got != "" {
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
		fixture.pathInRoot(fixture.cfg.Dist().NormalCSSRef()),
		" "+testOwnedOutputRelativePath("normal_hash.css")+" \n",
	)

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.runtime.StyleSheetURL(); got != testOwnedOutputPublicURL("normal_hash.css") {
		t.Fatalf("expected trimmed stylesheet URL, got %q", got)
	}
}

func TestStylesheetReferenceWithOnlyWhitespaceReturnsEmpty(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.pathInRoot(fixture.cfg.Dist().NormalCSSRef()), " \n\t ")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.runtime.StyleSheetURL(); got != "" {
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
	mustWriteFile(t, fixture.pathInRoot(fixture.cfg.Dist().NormalCSSRef()), "../outside.css")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.runtime.StyleSheetURL(); got != "/assets/outside.css" {
		t.Fatalf(
			"expected stylesheet URL to stay under public prefix, got %q",
			got,
		)
	}
}

func TestServeStaticWithRootPrefix(t *testing.T) {
	fixture := newWaveTestFixture(t)
	wavetest.SetCorePublicPathPrefix(fixture.cfg, "/")
	w := newWaveForTest(t, fixture, true, nil)

	handler, err := w.runtime.StaticHandler(false)
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
	wavetest.SetCorePublicPathPrefix(fixture.cfg, "/")
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

	dirReq := httptest.NewRequest(
		http.MethodGet,
		"/"+waveartifacts.HashedOutputDirname,
		nil,
	)
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
	wavetest.SetCorePublicPathPrefix(fixture.cfg, "/")
	w := newWaveForTest(t, fixture, false, nil)
	if removeErr := os.RemoveAll(fixture.pathInRoot(fixture.cfg.Dist().Static())); removeErr != nil {
		t.Fatalf("remove dist static directory: %v", removeErr)
	}

	if w.runtime.IsPublicAsset("/logo.txt") {
		t.Fatal(
			"expected isPublicAsset to return false when public FS is unavailable",
		)
	}
}

func TestPublicFileMapGettersFailClosedWhenCacheReturnsNilData(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	w.runtime.SetFileMapDetailsCache(
		wavecache.NewValueCacheWithModeResolver(
			func() (*waveruntimecore.FileMapDetails, error) {
				return nil, fmt.Errorf("forced failure")
			},
			w.IsDev,
		),
	)

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
		fixture.pathInRoot(fixture.cfg.Dist().PublicFileMapRef()),
		" "+testOwnedOutputRelativePath("public_filemap_hash.js")+" \n",
	)

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.runtime.PublicFileMapURL(); got != testOwnedOutputPublicURL("public_filemap_hash.js") {
		t.Fatalf("expected trimmed public file map URL, got %q", got)
	}
}

func TestPublicFileMapReferenceWithOnlyWhitespaceReturnsEmpty(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.pathInRoot(fixture.cfg.Dist().PublicFileMapRef()), " \n\t ")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.runtime.PublicFileMapURL(); got != "" {
		t.Fatalf(
			"expected empty public file map URL for whitespace-only ref, got %q",
			got,
		)
	}
	fileMapDetails := readFileMapDetailsFromCacheForTest(w)
	if fileMapDetails == nil {
		t.Fatal("expected public file map details cache value")
	}
	if got := fileMapDetails.Elements; got != "" {
		t.Fatalf(
			"expected empty public file map elements for whitespace-only ref, got %q",
			got,
		)
	}
}

func TestPublicFileMapReferenceTraversalStaysUnderPublicPrefix(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteFile(t, fixture.pathInRoot(fixture.cfg.Dist().PublicFileMapRef()), "../outside.js")

	w := newWaveForTest(t, fixture, true, nil)
	if got := w.runtime.PublicFileMapURL(); got != "/assets/outside.js" {
		t.Fatalf(
			"expected public file map URL to stay under public prefix, got %q",
			got,
		)
	}
}

func TestCriticalCSSReadErrorsFailClosed(t *testing.T) {
	fixture := newWaveTestFixture(t)
	if err := os.Remove(fixture.pathInRoot(fixture.cfg.Dist().CriticalCSS())); err != nil {
		t.Fatalf("failed removing critical css file: %v", err)
	}
	if err := os.Mkdir(fixture.pathInRoot(fixture.cfg.Dist().CriticalCSS()), 0o755); err != nil {
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
