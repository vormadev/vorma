package wave

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/vormadev/vorma/lab/jsonschema"
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

	w.addFrameworkWatchPatterns([]WatchedFile{{Pattern: "**/*.txt"}})
	if len(w.cfg.FrameworkWatchPatterns) != 1 {
		t.Fatalf(
			"expected FrameworkWatchPatterns to append, got %+v",
			w.cfg.FrameworkWatchPatterns,
		)
	}

	w.addIgnoredPatterns([]string{"**/*.tmp"})
	if len(w.cfg.FrameworkIgnoredPatterns) != 1 {
		t.Fatalf(
			"expected FrameworkIgnoredPatterns to append, got %+v",
			w.cfg.FrameworkIgnoredPatterns,
		)
	}

	w.setPublicFileMapOutDir("generated/public")
	if w.cfg.FrameworkPublicFileMapOutDir != "generated/public" {
		t.Fatalf(
			"unexpected public filemap out dir: %q",
			w.cfg.FrameworkPublicFileMapOutDir,
		)
	}

	w.setBrowserRuntimeNamespace("__vorma_runtime")
	if got := w.cfg.FrameworkBrowserRuntimeNamespace; got != "__vorma_runtime" {
		t.Fatalf("unexpected browser runtime namespace: %q", got)
	}

	w.setBrowserPublicURLResolverFunctionName("resolvePublicURL")
	if got := w.cfg.FrameworkBrowserPublicURLResolverFunctionName; got != "resolvePublicURL" {
		t.Fatalf("unexpected public URL resolver function name: %q", got)
	}

	w.setBrowserRevalidateFunctionName("__vorma_revalidate")
	if got := w.cfg.FrameworkBrowserRevalidateFunctionName; got != "__vorma_revalidate" {
		t.Fatalf("unexpected browser revalidate function name: %q", got)
	}

	w.setRefreshRebuildingOverlayElementID("vorma-refresh-overlay")
	if got := w.cfg.FrameworkRefreshRebuildingOverlayElementID; got != "vorma-refresh-overlay" {
		t.Fatalf("unexpected refresh rebuilding overlay element ID: %q", got)
	}

	w.setCriticalCSSStyleElementID("vorma-critical-css")
	if got := w.cfg.FrameworkCriticalCSSStyleElementID; got != "vorma-critical-css" {
		t.Fatalf("unexpected critical CSS style element ID: %q", got)
	}

	w.setNonCriticalCSSLinkElementID("vorma-noncritical-css")
	if got := w.cfg.FrameworkNonCriticalCSSLinkElementID; got != "vorma-noncritical-css" {
		t.Fatalf("unexpected non-critical CSS link element ID: %q", got)
	}

	resetPortCacheForTest()
	t.Setenv(envMode, "production")
	t.Setenv(envPortSet, "true")
	t.Setenv(envPort, "4500")
	w.portResolver = newPortResolver()
	if got := w.MustGetPort(); got != 4500 {
		t.Fatalf("expected Wave instance resolver port 4500, got %d", got)
	}

	t.Setenv(envPort, "4501")
	if got := w.MustGetPort(); got != 4500 {
		t.Fatalf(
			"expected Wave instance resolver to cache first port 4500, got %d",
			got,
		)
	}

	w.portResolver = newPortResolver()
	if got := w.MustGetPort(); got != 4501 {
		t.Fatalf(
			"expected Wave instance resolver reset to pick updated port 4501, got %d",
			got,
		)
	}
}

func TestConfigFileReturnsParsedConfigLocation(t *testing.T) {
	fixture := newWaveTestFixture(t)
	expectedConfigLocation := filepath.Join(
		fixture.root,
		"configs",
		"wave.config.json",
	)
	fixture.cfg.Core.ConfigLocation = expectedConfigLocation

	w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))
	if got := w.ConfigFile(); got != expectedConfigLocation {
		t.Fatalf("ConfigFile() = %q, want %q", got, expectedConfigLocation)
	}
}

func TestFrameworkSettersDoNotMutateCoreConfigFields(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

	originalCoreConfig := *w.cfg.Core

	w.addFrameworkWatchPatterns([]WatchedFile{{Pattern: "**/*.route"}})
	w.addIgnoredPatterns([]string{"generated/**"})
	w.setPublicFileMapOutDir("generated/public")
	w.setFrameworkDevBuildHookCommand("go run ./backend/cmd/build --dev")
	w.setFrameworkProdBuildHookCommand("go run ./backend/cmd/build --prod")
	w.registerFrameworkSchemaSection(
		"Vorma",
		jsonschema.Entry{Type: jsonschema.TypeObject},
	)
	w.setFrameworkRunBuildHookRunner(func(context.Context, bool) error {
		return nil
	})
	w.setFrameworkPrepareGoBuildOverlay(func() (*GoBuildOverlay, error) {
		return &GoBuildOverlay{
			OverlayConfigPath: "/tmp/vorma-overlay.json",
		}, nil
	})
	w.setBrowserRuntimeNamespace("__vorma_runtime")
	w.setBrowserPublicURLResolverFunctionName("resolvePublicURL")
	w.setBrowserRevalidateFunctionName("__vorma_revalidate")
	w.setRefreshRebuildingOverlayElementID("vorma-refresh-overlay")
	w.setCriticalCSSStyleElementID("vorma-critical-css")
	w.setNonCriticalCSSLinkElementID("vorma-normal-css")

	if got := *w.cfg.Core; got != originalCoreConfig {
		t.Fatalf(
			"framework setters mutated core config: got %#v want %#v",
			got,
			originalCoreConfig,
		)
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

	w.addFrameworkWatchPatterns(frameworkWatchPatterns)

	frameworkWatchPatterns[0].Pattern = "**/*.changed"
	frameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0] = "changed/**"

	if got := w.cfg.FrameworkWatchPatterns[0].Pattern; got != "**/*.txt" {
		t.Fatalf("framework watch pattern = %q, want **/*.txt", got)
	}
	if got := w.cfg.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0]; got != "generated/**" {
		t.Fatalf("framework watch hook exclude = %q, want generated/**", got)
	}
}

func TestBaseFSUsesDiskInDevMode(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, fstest.MapFS{})

	baseFS, err := w.getBaseFS()
	if err != nil {
		t.Fatalf("baseFS returned error: %v", err)
	}

	criticalCSS := mustReadFileFromFS(t, baseFS, RelPaths.CriticalCSS())
	if criticalCSS != "body{color:red;}" {
		t.Fatalf(
			"unexpected critical css content from base FS: %q",
			criticalCSS,
		)
	}
}

func TestBaseFSProductionRequiresDistStaticFS(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, nil)

	_, err := w.getBaseFS()
	if err == nil {
		t.Fatal(
			"expected production mode base FS initialization to fail without DistStaticFS",
		)
	}
	if !strings.Contains(err.Error(), "distStaticFS is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBaseFSProductionUsesProvidedFS(t *testing.T) {
	fixture := newWaveTestFixture(t)
	mapFS := fstest.MapFS{
		"internal/probe.txt": {Data: []byte("ok")},
	}
	w := newWaveForTest(t, fixture, false, mapFS)

	baseFS, err := w.getBaseFS()
	if err != nil {
		t.Fatalf("baseFS returned error: %v", err)
	}

	if got := mustReadFileFromFS(t, baseFS, "internal/probe.txt"); got != "ok" {
		t.Fatalf("expected provided FS content, got %q", got)
	}
}

func TestGetPublicAndPrivateFS(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	publicFS, err := w.getPublicFS()
	if err != nil {
		t.Fatalf("publicFS returned error: %v", err)
	}
	if got := mustReadFileFromFS(t, publicFS, "logo.txt"); got != "logo" {
		t.Fatalf("unexpected public file content: %q", got)
	}

	privateFS, err := w.PrivateFS()
	if err != nil {
		t.Fatalf("PrivateFS returned error: %v", err)
	}
	if got := mustReadFileFromFS(t, privateFS, "template.html"); got != "private" {
		t.Fatalf("unexpected private file content: %q", got)
	}
}

func TestPublicFileMapAndURLResolution(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	fm, err := w.publicFileMap()
	if err != nil {
		t.Fatalf("publicFileMap returned error: %v", err)
	}
	if _, ok := fm["logo.txt"]; !ok {
		t.Fatalf("expected logo.txt in public file map, got %+v", fm)
	}

	if got := w.PublicURL("logo.txt"); got != "/assets/vorma_out/logo.hash.txt" {
		t.Fatalf("unexpected mapped public URL: %q", got)
	}
	if got := w.PublicURL("missing.txt"); got != "" {
		t.Fatalf(
			"expected missing public URL lookup to return empty string, got %q",
			got,
		)
	}
	if got := w.PublicURL("/assets/logo.txt"); got != "/assets/vorma_out/logo.hash.txt" {
		t.Fatalf("unexpected already-prefixed mapped public URL: %q", got)
	}
	if got := w.PublicURL("/assets/missing.txt"); got != "" {
		t.Fatalf(
			"expected missing prefixed public URL lookup to return empty string, got %q",
			got,
		)
	}
	if mappedOnce, mappedTwice := w.PublicURL("logo.txt"), w.PublicURL(w.PublicURL("logo.txt")); mappedTwice != "" {
		t.Fatalf(
			"expected second lookup of already-hashed public URL to miss, got once=%q twice=%q",
			mappedOnce,
			mappedTwice,
		)
	}
	if missingOnce, missingTwice := w.PublicURL("missing.txt"), w.PublicURL(w.PublicURL("missing.txt")); missingTwice != missingOnce {
		t.Fatalf(
			"expected missing public URL lookup to be idempotent, got once=%q twice=%q",
			missingOnce,
			missingTwice,
		)
	}
	dataURL := "data:image/svg+xml;base64,AAAA"
	if got := w.PublicURL(dataURL); got != "" {
		t.Fatalf(
			"expected strict public URL lookup miss for data URL, got %q",
			got,
		)
	}
	upperDataURL := "DATA:image/svg+xml;base64,AAAA"
	if got := w.PublicURL(upperDataURL); got != "" {
		t.Fatalf(
			"expected strict public URL lookup miss for uppercase data URL, got %q",
			got,
		)
	}
	externalURL := "https://cdn.example.com/logo.svg"
	if got := w.PublicURL(externalURL); got != "" {
		t.Fatalf(
			"expected strict public URL lookup miss for external URL, got %q",
			got,
		)
	}
	protocolRelativeURL := "//cdn.example.com/logo.svg"
	if got := w.PublicURL(protocolRelativeURL); got != "" {
		t.Fatalf(
			"expected strict public URL lookup miss for protocol-relative URL, got %q",
			got,
		)
	}
	blobURL := "blob:https://example.com/uuid"
	if got := w.PublicURL(blobURL); got != "" {
		t.Fatalf(
			"expected strict public URL lookup miss for blob URL, got %q",
			got,
		)
	}
}

func TestPublicFileMapReturnsDefensiveCopy(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

	firstMap, err := w.publicFileMap()
	if err != nil {
		t.Fatalf("publicFileMap returned error: %v", err)
	}

	firstMap["logo.txt"] = FileVal{
		DistName: "changed/logo.txt",
	}

	secondMap, err := w.publicFileMap()
	if err != nil {
		t.Fatalf("publicFileMap returned error: %v", err)
	}

	if got := secondMap["logo.txt"].DistName; got != "vorma_out/logo.hash.txt" {
		t.Fatalf("public file map entry persisted caller mutation: got %q", got)
	}
}

func TestPublicFileMapElementsAndHash(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.publicFileMapURL(); got != "/assets/vorma_out/vorma_internal_public_filemap_hash.js" {
		t.Fatalf("unexpected public file map URL: %q", got)
	}

	elements := string(w.publicFileMapElements())
	if !strings.Contains(elements, `rel="modulepreload"`) {
		t.Fatalf(
			"expected modulepreload link in filemap elements, got %q",
			elements,
		)
	}
	if !strings.Contains(
		elements,
		`const browserRuntimeNamespace = "__wave";`,
	) {
		t.Fatalf(
			"expected default browser runtime namespace in filemap script, got %q",
			elements,
		)
	}
	if !strings.Contains(
		elements,
		`window[browserRuntimeNamespace].publicFileMap = wavePublicFileMap;`,
	) {
		t.Fatalf(
			"expected public file map registration in filemap elements, got %q",
			elements,
		)
	}

	hash := w.publicFileMapScriptSha256Hash()
	if hash == "" {
		t.Fatal("expected non-empty public filemap script hash")
	}
}

func TestPublicFileMapElementsUseConfiguredBrowserRuntimeSettings(
	t *testing.T,
) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	w.setBrowserRuntimeNamespace("__vorma_runtime")
	w.setBrowserPublicURLResolverFunctionName("resolvePublicURL")

	elements := string(w.publicFileMapElements())
	if !strings.Contains(
		elements,
		`const browserRuntimeNamespace = "__vorma_runtime";`,
	) {
		t.Fatalf(
			"expected configured browser runtime namespace in filemap elements, got %q",
			elements,
		)
	}
}

func TestPublicFileMapElementsEmptyWhenRefMissing(t *testing.T) {
	fixture := newWaveTestFixture(t)
	if err := os.Remove(fixture.cfg.Dist.PublicFileMapRef()); err != nil {
		t.Fatalf("failed to remove public file map ref: %v", err)
	}
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.publicFileMapURL(); got != "" {
		t.Fatalf(
			"expected empty file map URL when ref file is missing, got %q",
			got,
		)
	}
	if got := w.publicFileMapElements(); got != "" {
		t.Fatalf(
			"expected empty file map elements when ref file is missing, got %q",
			got,
		)
	}
	if got := w.publicFileMapScriptSha256Hash(); got != "" {
		t.Fatalf(
			"expected empty file map script hash when ref file is missing, got %q",
			got,
		)
	}
}

func TestCriticalCSSMethods(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if got := string(w.CriticalCSS()); got != "body{color:red;}" {
		t.Fatalf("unexpected critical css: %q", got)
	}
	el := string(w.CriticalCSSStyleElement())
	if !strings.Contains(el, `id="wave-critical-css"`) {
		t.Fatalf("expected critical css style element id, got %q", el)
	}
	if !strings.Contains(el, "body{color:red;}") {
		t.Fatalf("expected critical css content in style element, got %q", el)
	}
	if w.criticalCSSStyleElementSha256Hash() == "" {
		t.Fatal("expected critical css SHA-256 hash to be non-empty")
	}
	if w.criticalCSSElementID() != criticalCSSElementID {
		t.Fatalf(
			"unexpected critical css element id: %q",
			w.criticalCSSElementID(),
		)
	}
}

func TestCriticalCSSUsesConfiguredElementID(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	w.setCriticalCSSStyleElementID("vorma-critical-css")

	el := string(w.CriticalCSSStyleElement())
	if !strings.Contains(el, `id="vorma-critical-css"`) {
		t.Fatalf(
			"expected configured critical css style element id, got %q",
			el,
		)
	}
	if w.criticalCSSElementID() != "vorma-critical-css" {
		t.Fatalf(
			"unexpected configured critical css element id getter value: %q",
			w.criticalCSSElementID(),
		)
	}
}

func TestCriticalCSSReturnsEmptyWhenEntryUnsetOrMissingFile(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.CSSEntryFiles.Critical = ""
	wNoEntry := newWaveForTest(t, fixture, true, nil)
	if got := wNoEntry.CriticalCSS(); got != "" {
		t.Fatalf("expected empty critical css when entry is unset, got %q", got)
	}
	if got := wNoEntry.CriticalCSSStyleElement(); got != "" {
		t.Fatalf(
			"expected empty critical css style element when entry is unset, got %q",
			got,
		)
	}
	if got := wNoEntry.criticalCSSStyleElementSha256Hash(); got != "" {
		t.Fatalf(
			"expected empty critical css hash when entry is unset, got %q",
			got,
		)
	}

	fixture = newWaveTestFixture(t)
	if err := os.Remove(fixture.cfg.Dist.CriticalCSS()); err != nil {
		t.Fatalf("failed to remove critical css file: %v", err)
	}
	wMissingFile := newWaveForTest(t, fixture, true, nil)
	if got := wMissingFile.CriticalCSS(); got != "" {
		t.Fatalf(
			"expected empty critical css when file is missing, got %q",
			got,
		)
	}
	if got := wMissingFile.CriticalCSSStyleElement(); got != "" {
		t.Fatalf(
			"expected empty critical css style element when file is missing, got %q",
			got,
		)
	}
	if got := wMissingFile.criticalCSSStyleElementSha256Hash(); got != "" {
		t.Fatalf(
			"expected empty critical css hash when file is missing, got %q",
			got,
		)
	}
}

func TestStylesheetURLAndLink(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.styleSheetURL(); got != "/assets/vorma_out/vorma_internal_normal_hash.css" {
		t.Fatalf("unexpected stylesheet URL: %q", got)
	}
	link := string(w.StyleSheetLinkElement())
	if !strings.Contains(
		link,
		`href="/assets/vorma_out/vorma_internal_normal_hash.css"`,
	) {
		t.Fatalf("expected stylesheet href in link element, got %q", link)
	}
	if !strings.Contains(link, `id="wave-normal-css"`) {
		t.Fatalf("expected stylesheet element id in link element, got %q", link)
	}
	if w.styleSheetElementID() != styleSheetElementID {
		t.Fatalf(
			"unexpected stylesheet element id: %q",
			w.styleSheetElementID(),
		)
	}
}

func TestStylesheetLinkUsesConfiguredElementID(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	w.setNonCriticalCSSLinkElementID("vorma-normal-css")

	link := string(w.StyleSheetLinkElement())
	if !strings.Contains(link, `id="vorma-normal-css"`) {
		t.Fatalf(
			"expected configured stylesheet element id in link element, got %q",
			link,
		)
	}
	if w.styleSheetElementID() != "vorma-normal-css" {
		t.Fatalf(
			"unexpected configured stylesheet element id getter value: %q",
			w.styleSheetElementID(),
		)
	}
}

func TestStylesheetReturnsEmptyWhenEntryUnset(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.CSSEntryFiles.NonCritical = ""
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.styleSheetURL(); got != "" {
		t.Fatalf(
			"expected empty stylesheet URL when non-critical entry is unset, got %q",
			got,
		)
	}
	if got := w.StyleSheetLinkElement(); got != "" {
		t.Fatalf(
			"expected empty stylesheet link when non-critical entry is unset, got %q",
			got,
		)
	}
}

func TestIsPublicAssetWithConfiguredPrefixUsesFileExistence(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if !w.isPublicAsset("/assets/logo.txt") {
		t.Fatal(
			"expected existing prefixed file path to be treated as a public asset",
		)
	}
	if w.isPublicAsset("/assets/anything.txt") {
		t.Fatal(
			"expected missing prefixed file path to not be treated as a public asset",
		)
	}
	if w.isPublicAsset("/other/path") {
		t.Fatal(
			"expected non-prefixed path to not be treated as a public asset",
		)
	}
	if w.isPublicAsset("/assets/vorma_out") {
		t.Fatal(
			"expected prefixed directory path to not be treated as a public asset",
		)
	}
}

func TestIsPublicAssetRootPrefixUsesFileExistence(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.PublicPathPrefix = "/"
	w := newWaveForTest(t, fixture, true, nil)

	if !w.isPublicAsset("/logo.txt") {
		t.Fatal("expected existing file under root prefix to be a public asset")
	}
	if w.isPublicAsset("/missing.txt") {
		t.Fatal(
			"expected missing file under root prefix to not be a public asset",
		)
	}
	if w.isPublicAsset("/") {
		t.Fatal("expected root path to not be treated as an asset")
	}
	if w.isPublicAsset("/vorma_out") {
		t.Fatal("expected directory path to not be treated as an asset")
	}
}

func TestStaticHandlerAndMustStaticMiddleware(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	immutableHandler, err := w.staticHandler(true)
	if err != nil {
		t.Fatalf("staticHandler returned error: %v", err)
	}

	immutableReq := httptest.NewRequest(http.MethodGet, "/assets/logo.txt", nil)
	immutableRec := httptest.NewRecorder()
	immutableHandler.ServeHTTP(immutableRec, immutableReq)

	if immutableRec.Code != http.StatusOK {
		t.Fatalf(
			"expected immutable handler status 200, got %d",
			immutableRec.Code,
		)
	}
	if got := immutableRec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("unexpected immutable cache-control header: %q", got)
	}
	if body := immutableRec.Body.String(); body != "logo" {
		t.Fatalf("unexpected immutable handler body: %q", body)
	}

	mutableHandler, err := w.staticHandler(false)
	if err != nil {
		t.Fatalf("staticHandler returned error: %v", err)
	}

	mutableReq := httptest.NewRequest(http.MethodGet, "/assets/logo.txt", nil)
	mutableRec := httptest.NewRecorder()
	mutableHandler.ServeHTTP(mutableRec, mutableReq)
	if got := mutableRec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf(
			"expected mutable static handler to skip cache-control header, got %q",
			got,
		)
	}

	nextCalled := false
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		nextCalled = true
		rw.WriteHeader(http.StatusAccepted)
		_, _ = rw.Write([]byte("next"))
	})

	middlewareHandler := w.MustStaticMiddleware(false)(next)
	assetReq := httptest.NewRequest(http.MethodGet, "/assets/logo.txt", nil)
	assetRec := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(assetRec, assetReq)

	if assetRec.Code != http.StatusOK {
		t.Fatalf(
			"expected asset request to be served by static handler, got %d",
			assetRec.Code,
		)
	}
	if assetRec.Body.String() != "logo" {
		t.Fatalf(
			"unexpected static middleware body: %q",
			assetRec.Body.String(),
		)
	}
	if nextCalled {
		t.Fatal(
			"expected static middleware to bypass next handler for asset request",
		)
	}

	nonAssetReq := httptest.NewRequest(
		http.MethodGet,
		"/application/route",
		nil,
	)
	nonAssetRec := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(nonAssetRec, nonAssetReq)

	if nonAssetRec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected non-asset request to reach next handler, got %d",
			nonAssetRec.Code,
		)
	}
	if nonAssetRec.Body.String() != "next" {
		t.Fatalf(
			"unexpected next-handler response body: %q",
			nonAssetRec.Body.String(),
		)
	}

	prefixedNonAssetReq := httptest.NewRequest(
		http.MethodGet,
		"/assets/application/route",
		nil,
	)
	prefixedNonAssetRec := httptest.NewRecorder()
	middlewareHandler.ServeHTTP(prefixedNonAssetRec, prefixedNonAssetReq)

	if prefixedNonAssetRec.Code != http.StatusAccepted {
		t.Fatalf(
			"expected missing prefixed path to reach next handler, got %d",
			prefixedNonAssetRec.Code,
		)
	}
	if prefixedNonAssetRec.Body.String() != "next" {
		t.Fatalf(
			"unexpected next-handler response body for missing prefixed path: %q",
			prefixedNonAssetRec.Body.String(),
		)
	}
}

func TestFaviconRedirectMiddleware(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusNoContent)
	})
	h := w.faviconRedirect()(next)

	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf(
			"expected redirect status 302 for mapped favicon, got %d",
			rec.Code,
		)
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
	h := w.faviconRedirect()(next)

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

	if w.PublicPathPrefix() != "/assets/" {
		t.Fatalf("unexpected public path prefix: %q", w.PublicPathPrefix())
	}
	if w.DistDir() != fixture.cfg.Core.DistDir {
		t.Fatalf("unexpected dist dir: %q", w.DistDir())
	}
	if w.publicStaticDir() != fixture.cfg.Core.StaticAssetDirs.Public {
		t.Fatalf("unexpected public static dir: %q", w.publicStaticDir())
	}
	if w.PrivateStaticDir() != fixture.cfg.Core.StaticAssetDirs.Private {
		t.Fatalf("unexpected private static dir: %q", w.PrivateStaticDir())
	}
	if w.ViteManifestLocation() != fixture.cfg.ViteManifestPath() {
		t.Fatalf(
			"unexpected Vite manifest location: %q",
			w.ViteManifestLocation(),
		)
	}
	if w.viteOutDir() != fixture.cfg.Dist.StaticPublic() {
		t.Fatalf("unexpected Vite out dir: %q", w.viteOutDir())
	}
	if w.StaticPrivateOutDir() != fixture.cfg.Dist.StaticPrivate() {
		t.Fatalf(
			"unexpected static private out dir: %q",
			w.StaticPrivateOutDir(),
		)
	}
	if w.StaticPublicOutDir() != fixture.cfg.Dist.StaticPublic() {
		t.Fatalf("unexpected static public out dir: %q", w.StaticPublicOutDir())
	}

	w.addFrameworkWatchPatterns([]WatchedFile{
		{
			Pattern: "**/*.txt",
			OnChangeHooks: []OnChangeHook{
				{
					Exclude: []string{"generated/**"},
				},
			},
		},
	})
	w.addIgnoredPatterns([]string{"ignored/**"})
	w.setFrameworkDevBuildHookCommand("go run ./backend/cmd/build --dev")
	w.setFrameworkRunBuildHookRunner(func(context.Context, bool) error {
		return nil
	})

	parsedConfigSnapshot := w.ParsedConfig()
	if parsedConfigSnapshot == w.cfg {
		t.Fatal("expected ParsedConfig to return a defensive copy")
	}
	if parsedConfigSnapshot.Core == w.cfg.Core {
		t.Fatal("expected ParsedConfig to deep-clone Core config")
	}

	parsedConfigSnapshot.Core.MainAppEntry = "cmd/changed"
	parsedConfigSnapshot.FrameworkWatchPatterns[0].Pattern = "**/*.changed"
	parsedConfigSnapshot.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0] = "changed/**"
	parsedConfigSnapshot.FrameworkIgnoredPatterns[0] = "changed-ignored/**"
	parsedConfigSnapshot.FrameworkDevBuildHook = "go run ./backend/cmd/build --changed-dev"

	if got := w.cfg.Core.MainAppEntry; got != fixture.cfg.Core.MainAppEntry {
		t.Fatalf("ParsedConfig snapshot mutated core main app entry: %q", got)
	}
	if got := w.cfg.FrameworkWatchPatterns[0].Pattern; got != "**/*.txt" {
		t.Fatalf(
			"ParsedConfig snapshot mutated framework watch pattern: %q",
			got,
		)
	}
	if got := w.cfg.FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0]; got != "generated/**" {
		t.Fatalf(
			"ParsedConfig snapshot mutated framework watch hook exclude: %q",
			got,
		)
	}
	if got := w.cfg.FrameworkIgnoredPatterns[0]; got != "ignored/**" {
		t.Fatalf(
			"ParsedConfig snapshot mutated framework ignored pattern: %q",
			got,
		)
	}
	if got := w.cfg.FrameworkDevBuildHook; got != "go run ./backend/cmd/build --dev" {
		t.Fatalf(
			"ParsedConfig snapshot mutated framework dev build hook: %q",
			got,
		)
	}
	if parsedConfigSnapshot.FrameworkSchemaExtensions != nil {
		t.Fatal(
			"expected ParsedConfig snapshot to omit framework schema extensions",
		)
	}
	if parsedConfigSnapshot.FrameworkRunBuildHook != nil {
		t.Fatal(
			"expected ParsedConfig snapshot to omit framework run build hook callback",
		)
	}
}

func TestBuildtimeParsedConfigIncludesFrameworkBuildCallbacks(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	w.setFrameworkDevBuildHookCommand("go run ./backend/cmd/build --dev")
	w.registerFrameworkSchemaSection(
		"Custom",
		jsonschema.Entry{Type: jsonschema.TypeObject},
	)
	w.setFrameworkRunBuildHookRunner(func(context.Context, bool) error {
		return nil
	})

	buildtimeParsedConfig := w.BuildtimeParsedConfig()
	buildtimeParsedConfig.Core.MainAppEntry = "cmd/internal-mutated"

	if got := w.cfg.Core.MainAppEntry; got != fixture.cfg.Core.MainAppEntry {
		t.Fatalf(
			"BuildtimeParsedConfig snapshot mutated core main app entry: %q",
			got,
		)
	}
	if got := buildtimeParsedConfig.FrameworkDevBuildHook; got != "go run ./backend/cmd/build --dev" {
		t.Fatalf("FrameworkDevBuildHook = %q, want configured value", got)
	}
	if _, ok := buildtimeParsedConfig.FrameworkSchemaExtensions["Custom"]; !ok {
		t.Fatal(
			"expected BuildtimeParsedConfig snapshot to include framework schema extensions",
		)
	}
	if buildtimeParsedConfig.FrameworkRunBuildHook == nil {
		t.Fatal(
			"expected BuildtimeParsedConfig snapshot to include framework run build hook",
		)
	}
}

func TestMustGetFSAndStaticHandlerPanicsOnFailure(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, nil)

	assertPanicContains(t, "distStaticFS is nil", func() {
		_ = w.mustPublicFS()
	})
	assertPanicContains(t, "distStaticFS is nil", func() {
		_ = w.MustPrivateFS()
	})
	assertPanicContains(t, "distStaticFS is nil", func() {
		_ = w.mustStaticHandler(true)
	})
	assertPanicContains(t, "distStaticFS is nil", func() {
		_ = w.MustStaticMiddleware(true)
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
		t.Fatalf(
			"panic value type %T did not contain %q",
			recovered,
			expectedSubstr,
		)
	}()
	fn()
}
