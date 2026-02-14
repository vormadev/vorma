package wave

import (
	"bytes"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func newDiscardLoggerForWaveTests() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newWaveForTest(
	t *testing.T,
	fixture *waveTestFixture,
	isDev bool,
	distStaticFS fs.FS,
) *Wave {
	t.Helper()
	setWaveDevModeForTest(t, isDev)
	return New(Config{
		WaveConfigJSON: fixture.configJSON(t),
		DistStaticFS:   distStaticFS,
		Logger:         newDiscardLoggerForWaveTests(),
	})
}

func TestNewPanicsWhenConfigJSONIsMissing(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic when no config input is provided")
		}
		if !strings.Contains(recovered.(string), "WaveConfigJSON is required") {
			t.Fatalf("unexpected panic value: %v", recovered)
		}
	}()
	_ = New(Config{})
}

func TestNewFromWaveConfigJSON(t *testing.T) {
	fixture := newWaveTestFixture(t)
	setWaveDevModeForTest(t, false)

	w := New(Config{
		WaveConfigJSON: fixture.configJSON(t),
		DistStaticFS:   os.DirFS(fixture.cfg.Dist.Static()),
		Logger:         newDiscardLoggerForWaveTests(),
	})

	if w == nil {
		t.Fatal("expected non-nil Wave instance")
	}
	if !bytes.Equal(w.RawConfigJSON(), fixture.configJSON(t)) {
		t.Fatal("expected RawConfigJSON to match input WaveConfigJSON")
	}
}

func TestNewPanicsWhenConfigJSONIsInvalid(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic for invalid config JSON")
		}
		if !strings.Contains(recovered.(string), "parse config") {
			t.Fatalf("unexpected panic value: %v", recovered)
		}
	}()

	_ = New(Config{
		WaveConfigJSON: []byte("{"),
	})
}

func TestNewCreatesWaveAndExposesConfigurationMutators(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

	if w.Logger() == nil {
		t.Fatal("expected Wave logger to be initialized")
	}
	if !bytes.Equal(w.RawConfigJSON(), fixture.configJSON(t)) {
		t.Fatal("expected RawConfigJSON to return original configuration bytes")
	}

	w.AddFrameworkWatchPatterns([]WatchedFile{{Pattern: "**/*.txt"}})
	if len(w.cfg.FrameworkWatchPatterns) != 1 {
		t.Fatalf("expected FrameworkWatchPatterns to append, got %+v", w.cfg.FrameworkWatchPatterns)
	}

	w.AddIgnoredPatterns([]string{"**/*.tmp"})
	if len(w.cfg.FrameworkIgnoredPatterns) != 1 {
		t.Fatalf("expected FrameworkIgnoredPatterns to append, got %+v", w.cfg.FrameworkIgnoredPatterns)
	}

	w.SetPublicFileMapOutDir("generated/public")
	if w.cfg.FrameworkPublicFileMapOutDir != "generated/public" {
		t.Fatalf("unexpected public filemap out dir: %q", w.cfg.FrameworkPublicFileMapOutDir)
	}

	w.SetBrowserRuntimeNamespace("__vorma_runtime")
	if got := w.cfg.FrameworkBrowserRuntimeNamespace; got != "__vorma_runtime" {
		t.Fatalf("unexpected browser runtime namespace: %q", got)
	}

	w.SetBrowserPublicURLResolverFunctionName("resolvePublicURL")
	if got := w.cfg.FrameworkBrowserPublicURLResolverFunctionName; got != "resolvePublicURL" {
		t.Fatalf("unexpected public URL resolver function name: %q", got)
	}

	w.SetBrowserRevalidateFunctionName("__vorma_revalidate")
	if got := w.cfg.FrameworkBrowserRevalidateFunctionName; got != "__vorma_revalidate" {
		t.Fatalf("unexpected browser revalidate function name: %q", got)
	}

	w.SetRefreshRebuildingOverlayElementID("vorma-refresh-overlay")
	if got := w.cfg.FrameworkRefreshRebuildingOverlayElementID; got != "vorma-refresh-overlay" {
		t.Fatalf("unexpected refresh rebuilding overlay element ID: %q", got)
	}

	w.SetCriticalCSSStyleElementID("vorma-critical-css")
	if got := w.cfg.FrameworkCriticalCSSStyleElementID; got != "vorma-critical-css" {
		t.Fatalf("unexpected critical CSS style element ID: %q", got)
	}

	w.SetNonCriticalCSSLinkElementID("vorma-noncritical-css")
	if got := w.cfg.FrameworkNonCriticalCSSLinkElementID; got != "vorma-noncritical-css" {
		t.Fatalf("unexpected non-critical CSS link element ID: %q", got)
	}

	resetPortCacheForTest()
	t.Setenv(envMode, "production")
	t.Setenv(envPortSet, "true")
	t.Setenv(envPort, "4500")
	w.SetPortResolver(NewPortResolver())
	if got := w.MustGetPort(); got != 4500 {
		t.Fatalf("expected Wave instance resolver port 4500, got %d", got)
	}

	t.Setenv(envPort, "4501")
	if got := w.MustGetPort(); got != 4500 {
		t.Fatalf("expected Wave instance resolver to cache first port 4500, got %d", got)
	}

	w.SetPortResolver(NewPortResolver())
	if got := w.MustGetPort(); got != 4501 {
		t.Fatalf("expected Wave instance resolver reset to pick updated port 4501, got %d", got)
	}
}

func TestAddFrameworkWatchPatternsClonesInput(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

	frameworkWatchPatterns := []WatchedFile{
		{
			Pattern: "**/*.txt",
			OnChangeHooks: []OnChangeHook{
				{
					Exclude: []string{"generated/**"},
				},
			},
		},
	}

	w.AddFrameworkWatchPatterns(frameworkWatchPatterns)

	frameworkWatchPatterns[0].Pattern = "**/*.changed"
	frameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0] = "changed/**"

	if got := w.cfg.FrameworkWatchPatterns[0].Pattern; got != "**/*.txt" {
		t.Fatalf("framework watch pattern = %q, want **/*.txt", got)
	}
	if got := w.cfg.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0]; got != "generated/**" {
		t.Fatalf("framework watch hook exclude = %q, want generated/**", got)
	}
}

func TestGetBaseFSUsesDiskInDevMode(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, fstest.MapFS{})

	baseFS, err := w.GetBaseFS()
	if err != nil {
		t.Fatalf("GetBaseFS returned error: %v", err)
	}

	criticalCSS := mustReadFileFromFS(t, baseFS, RelPaths.CriticalCSS())
	if criticalCSS != "body{color:red;}" {
		t.Fatalf("unexpected critical css content from base FS: %q", criticalCSS)
	}
}

func TestGetBaseFSProductionRequiresDistStaticFS(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, nil)

	_, err := w.GetBaseFS()
	if err == nil {
		t.Fatal("expected production mode base FS initialization to fail without DistStaticFS")
	}
	if !strings.Contains(err.Error(), "distStaticFS is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetBaseFSProductionUsesProvidedFS(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mapFS := fstest.MapFS{
		"internal/probe.txt": {Data: []byte("ok")},
	}
	w := newWaveForTest(t, fixture, false, mapFS)

	baseFS, err := w.GetBaseFS()
	if err != nil {
		t.Fatalf("GetBaseFS returned error: %v", err)
	}

	if got := mustReadFileFromFS(t, baseFS, "internal/probe.txt"); got != "ok" {
		t.Fatalf("expected provided FS content, got %q", got)
	}
}

func TestGetPublicAndPrivateFS(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	publicFS, err := w.GetPublicFS()
	if err != nil {
		t.Fatalf("GetPublicFS returned error: %v", err)
	}
	if got := mustReadFileFromFS(t, publicFS, "logo.txt"); got != "logo" {
		t.Fatalf("unexpected public file content: %q", got)
	}

	privateFS, err := w.GetPrivateFS()
	if err != nil {
		t.Fatalf("GetPrivateFS returned error: %v", err)
	}
	if got := mustReadFileFromFS(t, privateFS, "template.html"); got != "private" {
		t.Fatalf("unexpected private file content: %q", got)
	}
}

func TestPublicFileMapAndURLResolution(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	fm, err := w.GetPublicFileMap()
	if err != nil {
		t.Fatalf("GetPublicFileMap returned error: %v", err)
	}
	if _, ok := fm["logo.txt"]; !ok {
		t.Fatalf("expected logo.txt in public file map, got %+v", fm)
	}

	if got := w.GetPublicURL("logo.txt"); got != "/assets/vorma_out/logo.hash.txt" {
		t.Fatalf("unexpected mapped public URL: %q", got)
	}
	if got := w.GetPublicURL("missing.txt"); got != "/assets/missing.txt" {
		t.Fatalf("unexpected fallback public URL: %q", got)
	}
	dataURL := "data:image/svg+xml;base64,AAAA"
	if got := w.GetPublicURL(dataURL); got != dataURL {
		t.Fatalf("expected data URL passthrough, got %q", got)
	}
	upperDataURL := "DATA:image/svg+xml;base64,AAAA"
	if got := w.GetPublicURL(upperDataURL); got != upperDataURL {
		t.Fatalf("expected case-insensitive data URL passthrough, got %q", got)
	}
	externalURL := "https://cdn.example.com/logo.svg"
	if got := w.GetPublicURL(externalURL); got != externalURL {
		t.Fatalf("expected external URL passthrough, got %q", got)
	}
	protocolRelativeURL := "//cdn.example.com/logo.svg"
	if got := w.GetPublicURL(protocolRelativeURL); got != protocolRelativeURL {
		t.Fatalf("expected protocol-relative URL passthrough, got %q", got)
	}
	blobURL := "blob:https://example.com/uuid"
	if got := w.GetPublicURL(blobURL); got != blobURL {
		t.Fatalf("expected blob URL passthrough, got %q", got)
	}
}

func TestGetPublicFileMapReturnsDefensiveCopy(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

	firstMap, err := w.GetPublicFileMap()
	if err != nil {
		t.Fatalf("GetPublicFileMap returned error: %v", err)
	}

	firstMap["logo.txt"] = FileVal{
		DistName: "changed/logo.txt",
	}

	secondMap, err := w.GetPublicFileMap()
	if err != nil {
		t.Fatalf("GetPublicFileMap returned error: %v", err)
	}

	if got := secondMap["logo.txt"].DistName; got != "vorma_out/logo.hash.txt" {
		t.Fatalf("public file map entry persisted caller mutation: got %q", got)
	}
}

func TestPublicFileMapElementsAndHash(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.GetPublicFileMapURL(); got != "/assets/vorma_out/vorma_internal_public_filemap_hash.js" {
		t.Fatalf("unexpected public file map URL: %q", got)
	}

	elements := string(w.GetPublicFileMapElements())
	if !strings.Contains(elements, `rel="modulepreload"`) {
		t.Fatalf("expected modulepreload link in filemap elements, got %q", elements)
	}
	if !strings.Contains(elements, `const browserRuntimeNamespace = "__wave";`) {
		t.Fatalf("expected default browser runtime namespace in filemap script, got %q", elements)
	}
	if !strings.Contains(elements, `window[browserRuntimeNamespace][publicURLResolverFunctionName] = getPublicURL;`) {
		t.Fatalf("expected public URL resolver registration in filemap elements, got %q", elements)
	}

	hash := w.GetPublicFileMapScriptSha256Hash()
	if hash == "" {
		t.Fatal("expected non-empty public filemap script hash")
	}
}

func TestPublicFileMapElementsUseConfiguredBrowserRuntimeSettings(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	w.SetBrowserRuntimeNamespace("__vorma_runtime")
	w.SetBrowserPublicURLResolverFunctionName("resolvePublicURL")

	elements := string(w.GetPublicFileMapElements())
	if !strings.Contains(elements, `const browserRuntimeNamespace = "__vorma_runtime";`) {
		t.Fatalf("expected configured browser runtime namespace in filemap elements, got %q", elements)
	}
	if !strings.Contains(elements, `const publicURLResolverFunctionName = "resolvePublicURL";`) {
		t.Fatalf("expected configured resolver function name in filemap elements, got %q", elements)
	}
}

func TestPublicFileMapElementsEmptyWhenRefMissing(t *testing.T) {
	fixture := newWaveTestFixture(t)
	if err := os.Remove(fixture.cfg.Dist.PublicFileMapRef()); err != nil {
		t.Fatalf("failed to remove public file map ref: %v", err)
	}
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.GetPublicFileMapURL(); got != "" {
		t.Fatalf("expected empty file map URL when ref file is missing, got %q", got)
	}
	if got := w.GetPublicFileMapElements(); got != "" {
		t.Fatalf("expected empty file map elements when ref file is missing, got %q", got)
	}
	if got := w.GetPublicFileMapScriptSha256Hash(); got != "" {
		t.Fatalf("expected empty file map script hash when ref file is missing, got %q", got)
	}
}

func TestCriticalCSSMethods(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if got := string(w.GetCriticalCSS()); got != "body{color:red;}" {
		t.Fatalf("unexpected critical css: %q", got)
	}
	el := string(w.GetCriticalCSSStyleElement())
	if !strings.Contains(el, `id="wave-critical-css"`) {
		t.Fatalf("expected critical css style element id, got %q", el)
	}
	if !strings.Contains(el, "body{color:red;}") {
		t.Fatalf("expected critical css content in style element, got %q", el)
	}
	if w.GetCriticalCSSStyleElementSha256Hash() == "" {
		t.Fatal("expected critical css SHA-256 hash to be non-empty")
	}
	if w.GetCriticalCSSElementID() != CriticalCSSElementID {
		t.Fatalf("unexpected critical css element id: %q", w.GetCriticalCSSElementID())
	}
}

func TestCriticalCSSUsesConfiguredElementID(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	w.SetCriticalCSSStyleElementID("vorma-critical-css")

	el := string(w.GetCriticalCSSStyleElement())
	if !strings.Contains(el, `id="vorma-critical-css"`) {
		t.Fatalf("expected configured critical css style element id, got %q", el)
	}
	if w.GetCriticalCSSElementID() != "vorma-critical-css" {
		t.Fatalf("unexpected configured critical css element id getter value: %q", w.GetCriticalCSSElementID())
	}
}

func TestCriticalCSSReturnsEmptyWhenEntryUnsetOrMissingFile(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.CSSEntryFiles.Critical = ""
	wNoEntry := newWaveForTest(t, fixture, true, nil)
	if got := wNoEntry.GetCriticalCSS(); got != "" {
		t.Fatalf("expected empty critical css when entry is unset, got %q", got)
	}
	if got := wNoEntry.GetCriticalCSSStyleElement(); got != "" {
		t.Fatalf("expected empty critical css style element when entry is unset, got %q", got)
	}
	if got := wNoEntry.GetCriticalCSSStyleElementSha256Hash(); got != "" {
		t.Fatalf("expected empty critical css hash when entry is unset, got %q", got)
	}

	fixture = newWaveTestFixture(t)
	if err := os.Remove(fixture.cfg.Dist.CriticalCSS()); err != nil {
		t.Fatalf("failed to remove critical css file: %v", err)
	}
	wMissingFile := newWaveForTest(t, fixture, true, nil)
	if got := wMissingFile.GetCriticalCSS(); got != "" {
		t.Fatalf("expected empty critical css when file is missing, got %q", got)
	}
	if got := wMissingFile.GetCriticalCSSStyleElement(); got != "" {
		t.Fatalf("expected empty critical css style element when file is missing, got %q", got)
	}
	if got := wMissingFile.GetCriticalCSSStyleElementSha256Hash(); got != "" {
		t.Fatalf("expected empty critical css hash when file is missing, got %q", got)
	}
}

func TestStylesheetURLAndLink(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.GetStyleSheetURL(); got != "/assets/vorma_out/vorma_internal_normal_hash.css" {
		t.Fatalf("unexpected stylesheet URL: %q", got)
	}
	link := string(w.GetStyleSheetLinkElement())
	if !strings.Contains(link, `href="/assets/vorma_out/vorma_internal_normal_hash.css"`) {
		t.Fatalf("expected stylesheet href in link element, got %q", link)
	}
	if !strings.Contains(link, `id="wave-normal-css"`) {
		t.Fatalf("expected stylesheet element id in link element, got %q", link)
	}
	if w.GetStyleSheetElementID() != StyleSheetElementID {
		t.Fatalf("unexpected stylesheet element id: %q", w.GetStyleSheetElementID())
	}
}

func TestStylesheetLinkUsesConfiguredElementID(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	w.SetNonCriticalCSSLinkElementID("vorma-normal-css")

	link := string(w.GetStyleSheetLinkElement())
	if !strings.Contains(link, `id="vorma-normal-css"`) {
		t.Fatalf("expected configured stylesheet element id in link element, got %q", link)
	}
	if w.GetStyleSheetElementID() != "vorma-normal-css" {
		t.Fatalf("unexpected configured stylesheet element id getter value: %q", w.GetStyleSheetElementID())
	}
}

func TestStylesheetReturnsEmptyWhenEntryUnset(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.CSSEntryFiles.NonCritical = ""
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.GetStyleSheetURL(); got != "" {
		t.Fatalf("expected empty stylesheet URL when non-critical entry is unset, got %q", got)
	}
	if got := w.GetStyleSheetLinkElement(); got != "" {
		t.Fatalf("expected empty stylesheet link when non-critical entry is unset, got %q", got)
	}
}

func TestIsPublicAssetWithConfiguredPrefixUsesFileExistence(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if !w.IsPublicAsset("/assets/logo.txt") {
		t.Fatal("expected existing prefixed file path to be treated as a public asset")
	}
	if w.IsPublicAsset("/assets/anything.txt") {
		t.Fatal("expected missing prefixed file path to not be treated as a public asset")
	}
	if w.IsPublicAsset("/other/path") {
		t.Fatal("expected non-prefixed path to not be treated as a public asset")
	}
	if w.IsPublicAsset("/assets/vorma_out") {
		t.Fatal("expected prefixed directory path to not be treated as a public asset")
	}
}

func TestIsPublicAssetRootPrefixUsesFileExistence(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.PublicPathPrefix = "/"
	w := newWaveForTest(t, fixture, true, nil)

	if !w.IsPublicAsset("/logo.txt") {
		t.Fatal("expected existing file under root prefix to be a public asset")
	}
	if w.IsPublicAsset("/missing.txt") {
		t.Fatal("expected missing file under root prefix to not be a public asset")
	}
	if w.IsPublicAsset("/") {
		t.Fatal("expected root path to not be treated as an asset")
	}
	if w.IsPublicAsset("/vorma_out") {
		t.Fatal("expected directory path to not be treated as an asset")
	}
}

func TestGetServeStaticHandlerAndServeStaticMiddleware(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	immutableHandler, err := w.GetServeStaticHandler(true)
	if err != nil {
		t.Fatalf("GetServeStaticHandler returned error: %v", err)
	}

	immutableReq := httptest.NewRequest(http.MethodGet, "/assets/logo.txt", nil)
	immutableRec := httptest.NewRecorder()
	immutableHandler.ServeHTTP(immutableRec, immutableReq)

	if immutableRec.Code != http.StatusOK {
		t.Fatalf("expected immutable handler status 200, got %d", immutableRec.Code)
	}
	if got := immutableRec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("unexpected immutable cache-control header: %q", got)
	}
	if body := immutableRec.Body.String(); body != "logo" {
		t.Fatalf("unexpected immutable handler body: %q", body)
	}

	mutableHandler, err := w.GetServeStaticHandler(false)
	if err != nil {
		t.Fatalf("GetServeStaticHandler returned error: %v", err)
	}

	mutableReq := httptest.NewRequest(http.MethodGet, "/assets/logo.txt", nil)
	mutableRec := httptest.NewRecorder()
	mutableHandler.ServeHTTP(mutableRec, mutableReq)
	if got := mutableRec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("expected mutable static handler to skip cache-control header, got %q", got)
	}

	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
		rw.WriteHeader(http.StatusAccepted)
		_, _ = rw.Write([]byte("next"))
	})

	middlewareHandler := w.ServeStatic(false)(next)
	assetReq := httptest.NewRequest(http.MethodGet, "/assets/logo.txt", nil)
	assetRec := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(assetRec, assetReq)

	if assetRec.Code != http.StatusOK {
		t.Fatalf("expected asset request to be served by static handler, got %d", assetRec.Code)
	}
	if assetRec.Body.String() != "logo" {
		t.Fatalf("unexpected static middleware body: %q", assetRec.Body.String())
	}
	if nextCalled {
		t.Fatal("expected static middleware to bypass next handler for asset request")
	}

	nonAssetReq := httptest.NewRequest(http.MethodGet, "/application/route", nil)
	nonAssetRec := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(nonAssetRec, nonAssetReq)

	if nonAssetRec.Code != http.StatusAccepted {
		t.Fatalf("expected non-asset request to reach next handler, got %d", nonAssetRec.Code)
	}
	if nonAssetRec.Body.String() != "next" {
		t.Fatalf("unexpected next-handler response body: %q", nonAssetRec.Body.String())
	}

	prefixedNonAssetReq := httptest.NewRequest(http.MethodGet, "/assets/application/route", nil)
	prefixedNonAssetRec := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(prefixedNonAssetRec, prefixedNonAssetReq)

	if prefixedNonAssetRec.Code != http.StatusAccepted {
		t.Fatalf("expected missing prefixed path to reach next handler, got %d", prefixedNonAssetRec.Code)
	}
	if prefixedNonAssetRec.Body.String() != "next" {
		t.Fatalf("unexpected next-handler response body for missing prefixed path: %q", prefixedNonAssetRec.Body.String())
	}
}

func TestFaviconRedirectMiddleware(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
	})
	h := w.FaviconRedirect()(next)

	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect status 302 for mapped favicon, got %d", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/assets/vorma_out/favicon.hash.ico" {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestFaviconRedirectReturns404WhenUnmapped(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mustWriteGob(t, fixture.cfg.Dist.PublicFileMapGob(), FileMap{
		"logo.txt": {
			DistName: "vorma_out/logo.hash.txt",
		},
	})

	w := newWaveForTest(t, fixture, true, nil)
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
	})
	h := w.FaviconRedirect()(next)

	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when favicon is unmapped, got %d", rec.Code)
	}
}

func TestConfigAccessorMethods(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if w.GetPublicPathPrefix() != "/assets/" {
		t.Fatalf("unexpected public path prefix: %q", w.GetPublicPathPrefix())
	}
	if w.GetDistDir() != fixture.cfg.Core.DistDir {
		t.Fatalf("unexpected dist dir: %q", w.GetDistDir())
	}
	if w.GetPublicStaticDir() != fixture.cfg.Core.StaticAssetDirs.Public {
		t.Fatalf("unexpected public static dir: %q", w.GetPublicStaticDir())
	}
	if w.GetPrivateStaticDir() != fixture.cfg.Core.StaticAssetDirs.Private {
		t.Fatalf("unexpected private static dir: %q", w.GetPrivateStaticDir())
	}
	if w.GetViteManifestLocation() != fixture.cfg.ViteManifestPath() {
		t.Fatalf("unexpected Vite manifest location: %q", w.GetViteManifestLocation())
	}
	if w.GetViteOutDir() != fixture.cfg.Dist.StaticPublic() {
		t.Fatalf("unexpected Vite out dir: %q", w.GetViteOutDir())
	}
	if w.GetStaticPrivateOutDir() != fixture.cfg.Dist.StaticPrivate() {
		t.Fatalf("unexpected static private out dir: %q", w.GetStaticPrivateOutDir())
	}
	if w.GetStaticPublicOutDir() != fixture.cfg.Dist.StaticPublic() {
		t.Fatalf("unexpected static public out dir: %q", w.GetStaticPublicOutDir())
	}
	if w.GetParsedConfig() != w.cfg {
		t.Fatal("expected GetParsedConfig to return Wave parsed config reference")
	}
}

func TestMustGetFSAndStaticHandlerPanicsOnFailure(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, nil)

	assertPanicContains(t, "distStaticFS is nil", func() {
		_ = w.MustGetPublicFS()
	})
	assertPanicContains(t, "distStaticFS is nil", func() {
		_ = w.MustGetPrivateFS()
	})
	assertPanicContains(t, "distStaticFS is nil", func() {
		_ = w.MustGetServeStaticHandler(true)
	})
	assertPanicContains(t, "distStaticFS is nil", func() {
		_ = w.ServeStatic(true)
	})
}

func assertPanicContains(t *testing.T, expectedSubstr string, fn func()) {
	t.Helper()
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("expected panic containing %q", expectedSubstr)
		}

		if str, isString := recovered.(string); isString {
			if !strings.Contains(str, expectedSubstr) {
				t.Fatalf("unexpected panic string: %q", str)
			}
			return
		}
		if err, isErr := recovered.(error); isErr {
			if !strings.Contains(err.Error(), expectedSubstr) {
				t.Fatalf("unexpected panic error: %v", err)
			}
			return
		}
		t.Fatalf("panic value type %T did not contain %q", recovered, expectedSubstr)
	}()
	fn()
}
