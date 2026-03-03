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

	"github.com/vormadev/vorma/internal/testpath"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave/internal/wavefilemap"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveenv"
	"github.com/vormadev/vorma/wave/waveframework"
	"github.com/vormadev/vorma/wave/wavewatch"
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

func appendFrameworkWatchPatternsForTest(
	w *Wave,
	watchPatterns []wavewatch.WatchedFile,
) {
	waveframework.StateForConfig(w.cfg).WatchPatterns = append(
		waveframework.StateForConfig(w.cfg).WatchPatterns,
		watchPatterns...,
	)
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

	appendFrameworkWatchPatternsForTest(
		w,
		[]wavewatch.WatchedFile{{Pattern: "**/*.txt"}},
	)
	if len(waveframework.StateForConfig(w.cfg).WatchPatterns) != 1 {
		t.Fatalf(
			"expected FrameworkWatchPatterns to append, got %+v",
			waveframework.StateForConfig(w.cfg).WatchPatterns,
		)
	}

	waveframework.StateForConfig(w.cfg).IgnoredPatterns = append(
		waveframework.StateForConfig(w.cfg).IgnoredPatterns,
		"**/*.tmp",
	)
	if len(waveframework.StateForConfig(w.cfg).IgnoredPatterns) != 1 {
		t.Fatalf(
			"expected FrameworkIgnoredPatterns to append, got %+v",
			waveframework.StateForConfig(w.cfg).IgnoredPatterns,
		)
	}

	waveframework.StateForConfig(w.cfg).BrowserRuntimeNamespace = "__vorma_runtime"
	if got := waveframework.StateForConfig(w.cfg).BrowserRuntimeNamespace; got != "__vorma_runtime" {
		t.Fatalf("unexpected browser runtime namespace: %q", got)
	}

	waveframework.StateForConfig(w.cfg).BrowserPublicURLResolverFunctionName = "resolvePublicURL"
	if got := waveframework.StateForConfig(w.cfg).BrowserPublicURLResolverFunctionName; got != "resolvePublicURL" {
		t.Fatalf("unexpected public URL resolver function name: %q", got)
	}

	waveframework.StateForConfig(w.cfg).BrowserRevalidateFunctionName = "__vorma_revalidate"
	if got := waveframework.StateForConfig(w.cfg).BrowserRevalidateFunctionName; got != "__vorma_revalidate" {
		t.Fatalf("unexpected browser revalidate function name: %q", got)
	}

	waveframework.StateForConfig(w.cfg).RefreshRebuildingOverlayElementID = "vorma-refresh-overlay"
	if got := waveframework.StateForConfig(w.cfg).RefreshRebuildingOverlayElementID; got != "vorma-refresh-overlay" {
		t.Fatalf("unexpected refresh rebuilding overlay element ID: %q", got)
	}

	waveframework.StateForConfig(w.cfg).CriticalCSSStyleElementID = "vorma-critical-css"
	if got := waveframework.StateForConfig(w.cfg).CriticalCSSStyleElementID; got != "vorma-critical-css" {
		t.Fatalf("unexpected critical CSS style element ID: %q", got)
	}

	waveframework.StateForConfig(w.cfg).NonCriticalCSSLinkElementID = "vorma-noncritical-css"
	if got := waveframework.StateForConfig(w.cfg).NonCriticalCSSLinkElementID; got != "vorma-noncritical-css" {
		t.Fatalf("unexpected non-critical CSS link element ID: %q", got)
	}

	resetPortCacheForTest()
	t.Setenv(waveenv.EnvMode, "production")
	t.Setenv(waveenv.EnvPortSet, "true")
	t.Setenv(waveenv.EnvPort, "4500")
	w.runtime.SetPortResolver(waveenv.NewResolverForMode(w.IsDev()))
	if got := w.MustGetPort(); got != 4500 {
		t.Fatalf("expected Wave instance resolver port 4500, got %d", got)
	}

	t.Setenv(waveenv.EnvPort, "4501")
	if got := w.MustGetPort(); got != 4500 {
		t.Fatalf(
			"expected Wave instance resolver to cache first port 4500, got %d",
			got,
		)
	}

	w.runtime.SetPortResolver(waveenv.NewResolverForMode(w.IsDev()))
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

	appendFrameworkWatchPatternsForTest(
		w,
		[]wavewatch.WatchedFile{{Pattern: "**/*.route"}},
	)
	waveframework.StateForConfig(w.cfg).IgnoredPatterns = append(
		waveframework.StateForConfig(w.cfg).IgnoredPatterns,
		"generated/**",
	)
	waveframework.StateForConfig(w.cfg).DevBuildHook = "go run ./backend/cmd/build --dev"
	waveframework.StateForConfig(w.cfg).ProdBuildHook = "go run ./backend/cmd/build --prod"
	waveframework.StateForConfig(w.cfg).SchemaExtensions = map[string]jsonschema.Entry{
		"Vorma": {Type: jsonschema.TypeObject},
	}
	waveframework.StateForConfig(w.cfg).RunBuildHook = func(context.Context, bool) error {
		return nil
	}
	waveframework.StateForConfig(w.cfg).PrepareGoBuildOverlay = func() (*waveframework.GoBuildOverlay, error) {
		return &waveframework.GoBuildOverlay{
			OverlayConfigPath: "/tmp/vorma-overlay.json",
		}, nil
	}
	waveframework.StateForConfig(w.cfg).BrowserRuntimeNamespace = "__vorma_runtime"
	waveframework.StateForConfig(w.cfg).BrowserPublicURLResolverFunctionName = "resolvePublicURL"
	waveframework.StateForConfig(w.cfg).BrowserRevalidateFunctionName = "__vorma_revalidate"
	waveframework.StateForConfig(w.cfg).RefreshRebuildingOverlayElementID = "vorma-refresh-overlay"
	waveframework.StateForConfig(w.cfg).CriticalCSSStyleElementID = "vorma-critical-css"
	waveframework.StateForConfig(w.cfg).NonCriticalCSSLinkElementID = "vorma-normal-css"

	if got := *w.cfg.Core; got != originalCoreConfig {
		t.Fatalf(
			"framework setters mutated core config: got %#v want %#v",
			got,
			originalCoreConfig,
		)
	}
}

func TestAddFrameworkWatchPatternsAppendsInput(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, os.DirFS(fixture.cfg.Dist.Static()))

	frameworkWatchPatterns := []wavewatch.WatchedFile{
		{
			Pattern: "**/*.txt",
			OnChangeHooks: []wavewatch.OnChangeHook{
				{
					Exclude: []string{"generated/**"},
				},
			},
		},
	}

	appendFrameworkWatchPatternsForTest(w, frameworkWatchPatterns)

	if got := waveframework.StateForConfig(w.cfg).WatchPatterns[0].Pattern; got != "**/*.txt" {
		t.Fatalf("framework watch pattern = %q, want **/*.txt", got)
	}
	if got := waveframework.StateForConfig(w.cfg).WatchPatterns[0].OnChangeHooks[0].Exclude[0]; got != "generated/**" {
		t.Fatalf("framework watch hook exclude = %q, want generated/**", got)
	}
}

func TestBaseFSUsesDiskInDevMode(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, fstest.MapFS{})

	baseFS, err := w.runtime.GetBaseFS()
	if err != nil {
		t.Fatalf("baseFS returned error: %v", err)
	}

	criticalCSS := mustReadFileFromFS(t, baseFS, "internal/critical.css")
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

	_, err := w.runtime.GetBaseFS()
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

	baseFS, err := w.runtime.GetBaseFS()
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

	publicFS, err := w.runtime.GetPublicFS()
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
	if got := mustReadFileFromFS(
		t,
		privateFS,
		"template.html",
	); got != waveartifacts.PrivateDirname {
		t.Fatalf("unexpected private file content: %q", got)
	}
}

func TestPublicFileMapAndURLResolution(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	fm, err := w.runtime.PublicFileMap()
	if err != nil {
		t.Fatalf("publicFileMap returned error: %v", err)
	}
	if _, ok := fm["logo.txt"]; !ok {
		t.Fatalf("expected logo.txt in public file map, got %+v", fm)
	}

	if got := w.PublicURL("logo.txt"); got != testHashedOutputPublicURL(
		"logo.hash.txt",
	) {
		t.Fatalf("unexpected mapped public URL: %q", got)
	}
	if got := w.PublicURL("missing.txt"); got != "" {
		t.Fatalf(
			"expected missing public URL lookup to return empty string, got %q",
			got,
		)
	}
	if got := w.PublicURL("/assets/logo.txt"); got != testHashedOutputPublicURL(
		"logo.hash.txt",
	) {
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

	firstMap, err := w.runtime.PublicFileMap()
	if err != nil {
		t.Fatalf("publicFileMap returned error: %v", err)
	}

	firstMap["logo.txt"] = wavefilemap.FileVal{
		DistName: "changed/logo.txt",
	}

	secondMap, err := w.runtime.PublicFileMap()
	if err != nil {
		t.Fatalf("publicFileMap returned error: %v", err)
	}

	if got := secondMap["logo.txt"].DistName; got != testHashedOutputRelativePath(
		"logo.hash.txt",
	) {
		t.Fatalf("public file map entry persisted caller mutation: got %q", got)
	}
}

func TestPublicFileMapElementsAndHash(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.runtime.PublicFileMapURL(); got != testOwnedOutputPublicURL(
		"public_filemap_hash.js",
	) {
		t.Fatalf("unexpected public file map URL: %q", got)
	}

	fileMapDetails := readFileMapDetailsFromCacheForTest(w)
	if fileMapDetails == nil {
		t.Fatal("expected public file map details to be initialized")
	}
	elements := fileMapDetails.Elements
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
}

func TestPublicFileMapElementsUseConfiguredBrowserRuntimeSettings(
	t *testing.T,
) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	waveframework.StateForConfig(w.cfg).BrowserRuntimeNamespace = "__vorma_runtime"
	waveframework.StateForConfig(w.cfg).BrowserPublicURLResolverFunctionName = "resolvePublicURL"

	fileMapDetails := readFileMapDetailsFromCacheForTest(w)
	if fileMapDetails == nil {
		t.Fatal("expected public file map details to be initialized")
	}
	elements := fileMapDetails.Elements
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

	if got := w.runtime.PublicFileMapURL(); got != "" {
		t.Fatalf(
			"expected empty file map URL when ref file is missing, got %q",
			got,
		)
	}
	fileMapDetails := readFileMapDetailsFromCacheForTest(w)
	if fileMapDetails == nil {
		t.Fatal("expected public file map details cache value")
	}
	if got := fileMapDetails.Elements; got != "" {
		t.Fatalf(
			"expected empty file map elements when ref file is missing, got %q",
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
	if waveframework.CriticalCSSStyleElementID(w.cfg) != "wave-critical-css" {
		t.Fatalf(
			"unexpected critical css element id: %q",
			waveframework.CriticalCSSStyleElementID(w.cfg),
		)
	}
}

func TestCriticalCSSUsesConfiguredElementID(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	waveframework.StateForConfig(w.cfg).CriticalCSSStyleElementID = "vorma-critical-css"

	el := string(w.CriticalCSSStyleElement())
	if !strings.Contains(el, `id="vorma-critical-css"`) {
		t.Fatalf(
			"expected configured critical css style element id, got %q",
			el,
		)
	}
	if waveframework.CriticalCSSStyleElementID(w.cfg) != "vorma-critical-css" {
		t.Fatalf(
			"unexpected configured critical css element id getter value: %q",
			waveframework.CriticalCSSStyleElementID(w.cfg),
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
}

func TestStylesheetURLAndLink(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.runtime.StyleSheetURL(); got != testOwnedOutputPublicURL(
		"normal_hash.css",
	) {
		t.Fatalf("unexpected stylesheet URL: %q", got)
	}
	link := string(w.StyleSheetLinkElement())
	if !strings.Contains(
		link,
		`href="`+testOwnedOutputPublicURL("normal_hash.css")+`"`,
	) {
		t.Fatalf("expected stylesheet href in link element, got %q", link)
	}
	if !strings.Contains(link, `id="wave-normal-css"`) {
		t.Fatalf("expected stylesheet element id in link element, got %q", link)
	}
	if waveframework.NonCriticalCSSLinkElementID(w.cfg) != "wave-normal-css" {
		t.Fatalf(
			"unexpected stylesheet element id: %q",
			waveframework.NonCriticalCSSLinkElementID(w.cfg),
		)
	}
}

func TestStylesheetLinkUsesConfiguredElementID(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)
	waveframework.StateForConfig(w.cfg).NonCriticalCSSLinkElementID = "vorma-normal-css"

	link := string(w.StyleSheetLinkElement())
	if !strings.Contains(link, `id="vorma-normal-css"`) {
		t.Fatalf(
			"expected configured stylesheet element id in link element, got %q",
			link,
		)
	}
	if waveframework.NonCriticalCSSLinkElementID(w.cfg) != "vorma-normal-css" {
		t.Fatalf(
			"unexpected configured stylesheet element id getter value: %q",
			waveframework.NonCriticalCSSLinkElementID(w.cfg),
		)
	}
}

func TestStylesheetReturnsEmptyWhenEntryUnset(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.CSSEntryFiles.NonCritical = ""
	w := newWaveForTest(t, fixture, true, nil)

	if got := w.runtime.StyleSheetURL(); got != "" {
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

	if !w.runtime.IsPublicAsset("/assets/logo.txt") {
		t.Fatal(
			"expected existing prefixed file path to be treated as a public asset",
		)
	}
	if w.runtime.IsPublicAsset("/assets/anything.txt") {
		t.Fatal(
			"expected missing prefixed file path to not be treated as a public asset",
		)
	}
	if w.runtime.IsPublicAsset("/other/path") {
		t.Fatal(
			"expected non-prefixed path to not be treated as a public asset",
		)
	}
	if w.runtime.IsPublicAsset("/assets/" + waveartifacts.HashedOutputDirname) {
		t.Fatal(
			"expected prefixed directory path to not be treated as a public asset",
		)
	}
}

func TestIsPublicAssetRootPrefixUsesFileExistence(t *testing.T) {
	fixture := newWaveTestFixture(t)
	fixture.cfg.Core.PublicPathPrefix = "/"
	w := newWaveForTest(t, fixture, true, nil)

	if !w.runtime.IsPublicAsset("/logo.txt") {
		t.Fatal("expected existing file under root prefix to be a public asset")
	}
	if w.runtime.IsPublicAsset("/missing.txt") {
		t.Fatal(
			"expected missing file under root prefix to not be a public asset",
		)
	}
	if w.runtime.IsPublicAsset("/") {
		t.Fatal("expected root path to not be treated as an asset")
	}
	if w.runtime.IsPublicAsset("/" + waveartifacts.HashedOutputDirname) {
		t.Fatal("expected directory path to not be treated as an asset")
	}
}

func TestStaticHandlerAndMustStaticMiddleware(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	immutableHandler, err := w.runtime.StaticHandler(true)
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

	mutableHandler, err := w.runtime.StaticHandler(false)
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

func TestConfigAccessorMethods(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	if w.PublicPathPrefix() != "/assets/" {
		t.Fatalf("unexpected public path prefix: %q", w.PublicPathPrefix())
	}
	expectedDistDir := testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		fixture.cfg.Core.DistDir,
	)
	if w.DistDir() != expectedDistDir {
		t.Fatalf("unexpected dist dir: %q", w.DistDir())
	}
	expectedPublicStaticDir := testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		fixture.cfg.Core.StaticAssetDirs.Public,
	)
	if w.runtime.PublicStaticDir() != expectedPublicStaticDir {
		t.Fatalf(
			"unexpected public static dir: %q",
			w.runtime.PublicStaticDir(),
		)
	}
	expectedPrivateStaticDir := testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		fixture.cfg.Core.StaticAssetDirs.Private,
	)
	if w.PrivateStaticDir() != expectedPrivateStaticDir {
		t.Fatalf("unexpected private static dir: %q", w.PrivateStaticDir())
	}
	expectedViteManifestLocation := testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		fixture.cfg.ViteManifestPath(),
	)
	if w.ViteManifestLocation() != expectedViteManifestLocation {
		t.Fatalf(
			"unexpected Vite manifest location: %q",
			w.ViteManifestLocation(),
		)
	}
	expectedViteOutDir := testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		fixture.cfg.Dist.StaticPublic(),
	)
	if w.runtime.ViteOutDir() != expectedViteOutDir {
		t.Fatalf("unexpected Vite out dir: %q", w.runtime.ViteOutDir())
	}
	expectedStaticPrivateOutDir := testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		fixture.cfg.Dist.StaticPrivate(),
	)
	if w.StaticPrivateOutDir() != expectedStaticPrivateOutDir {
		t.Fatalf(
			"unexpected static private out dir: %q",
			w.StaticPrivateOutDir(),
		)
	}
	expectedStaticPublicOutDir := testpath.PathRelativeToCurrentWorkingDirectory(
		t,
		fixture.cfg.Dist.StaticPublic(),
	)
	if w.StaticPublicOutDir() != expectedStaticPublicOutDir {
		t.Fatalf("unexpected static public out dir: %q", w.StaticPublicOutDir())
	}

	parsedConfig := waveframework.ParsedConfig(w.RawConfigJSON())
	if parsedConfig == nil {
		t.Fatal("expected ParsedConfig() to return non-nil parsed config")
	}
	parsedConfigAgain := waveframework.ParsedConfig(w.RawConfigJSON())
	if parsedConfigAgain != parsedConfig {
		t.Fatal(
			"expected ParsedConfig() to return stable canonical parsed config across repeated calls",
		)
	}

	appendFrameworkWatchPatternsForTest(
		&Wave{cfg: parsedConfig},
		[]wavewatch.WatchedFile{
			{
				Pattern: "**/*.txt",
				OnChangeHooks: []wavewatch.OnChangeHook{
					{
						Exclude: []string{"generated/**"},
					},
				},
			},
		},
	)
	waveframework.StateForConfig(parsedConfig).IgnoredPatterns = append(
		waveframework.StateForConfig(parsedConfig).IgnoredPatterns,
		"ignored/**",
	)
	waveframework.StateForConfig(parsedConfig).DevBuildHook = "go run ./backend/cmd/build --dev"
	waveframework.StateForConfig(parsedConfig).RunBuildHook = func(context.Context, bool) error {
		return nil
	}
	waveframework.StateForConfig(parsedConfig).SchemaExtensions = map[string]jsonschema.Entry{
		"Custom": {Type: jsonschema.TypeObject},
	}

	buildtimeParsedConfig := waveframework.BuildtimeParsedConfig(
		w.RawConfigJSON(),
	)
	if buildtimeParsedConfig != parsedConfig {
		t.Fatal(
			"expected BuildtimeParsedConfig() to resolve to canonical parsed config instance",
		)
	}

	if got := waveframework.StateForConfig(buildtimeParsedConfig).WatchPatterns[0].Pattern; got != "**/*.txt" {
		t.Fatalf(
			"FrameworkWatchPatterns[0].Pattern = %q, want %q",
			got,
			"**/*.txt",
		)
	}
	if got := waveframework.StateForConfig(buildtimeParsedConfig).WatchPatterns[0].OnChangeHooks[0].Exclude[0]; got != "generated/**" {
		t.Fatalf(
			"FrameworkWatchPatterns[0].OnChangeHooks[0].Exclude[0] = %q, want %q",
			got,
			"generated/**",
		)
	}
	if got := waveframework.StateForConfig(buildtimeParsedConfig).IgnoredPatterns[0]; got != "ignored/**" {
		t.Fatalf("FrameworkIgnoredPatterns[0] = %q, want %q", got, "ignored/**")
	}
	if got := waveframework.StateForConfig(buildtimeParsedConfig).DevBuildHook; got != "go run ./backend/cmd/build --dev" {
		t.Fatalf(
			"FrameworkDevBuildHook = %q, want %q",
			got,
			"go run ./backend/cmd/build --dev",
		)
	}
	if _, ok := waveframework.StateForConfig(buildtimeParsedConfig).SchemaExtensions["Custom"]; !ok {
		t.Fatal(
			"expected BuildtimeParsedConfig() to preserve schema extensions",
		)
	}
	if waveframework.StateForConfig(buildtimeParsedConfig).RunBuildHook == nil {
		t.Fatal(
			"expected BuildtimeParsedConfig() to preserve run build hook callback",
		)
	}
}

func TestBuildtimeParsedConfigIncludesFrameworkBuildCallbacks(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	parsedConfig := waveframework.BuildtimeParsedConfig(w.RawConfigJSON())
	if parsedConfig == nil {
		t.Fatal(
			"expected BuildtimeParsedConfig() to return non-nil parsed config",
		)
	}
	waveframework.StateForConfig(parsedConfig).DevBuildHook = "go run ./backend/cmd/build --dev"
	waveframework.StateForConfig(parsedConfig).SchemaExtensions = map[string]jsonschema.Entry{
		"Custom": {Type: jsonschema.TypeObject},
	}
	waveframework.StateForConfig(parsedConfig).RunBuildHook = func(context.Context, bool) error {
		return nil
	}

	buildtimeParsedConfig := waveframework.BuildtimeParsedConfig(
		w.RawConfigJSON(),
	)
	if buildtimeParsedConfig != parsedConfig {
		t.Fatal(
			"expected BuildtimeParsedConfig() to return stable canonical parsed config",
		)
	}
	if got := waveframework.StateForConfig(buildtimeParsedConfig).DevBuildHook; got != "go run ./backend/cmd/build --dev" {
		t.Fatalf("FrameworkDevBuildHook = %q, want configured value", got)
	}
	if _, ok := waveframework.StateForConfig(buildtimeParsedConfig).SchemaExtensions["Custom"]; !ok {
		t.Fatal(
			"expected BuildtimeParsedConfig snapshot to include framework schema extensions",
		)
	}
	if waveframework.StateForConfig(buildtimeParsedConfig).RunBuildHook == nil {
		t.Fatal(
			"expected BuildtimeParsedConfig snapshot to include framework run build hook",
		)
	}
}

func TestMustGetFSAndStaticHandlerPanicsOnFailure(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, nil)

	publicFS, publicFSError := w.runtime.GetPublicFS()
	if publicFSError == nil {
		t.Fatalf(
			"expected getPublicFS to fail when distStaticFS is nil, got %#v",
			publicFS,
		)
	}
	if !strings.Contains(publicFSError.Error(), "distStaticFS is nil") {
		t.Fatalf("unexpected getPublicFS error: %v", publicFSError)
	}
	assertPanicContains(t, "distStaticFS is nil", func() {
		_ = w.MustPrivateFS()
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
