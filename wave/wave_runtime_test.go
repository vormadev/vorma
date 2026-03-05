package wave

import (
	"bytes"
	"context"
	"fmt"
	"github.com/vormadev/vorma/internal/wavetest"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

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
	t.Chdir(fixture.root)
	_ = distStaticFS
	configPath := fixture.mustWriteConfigFile(t)
	return New(Config{
		FS:         os.DirFS(fixture.root),
		ConfigPath: configPath,
		Logger:     newDiscardLoggerForWaveTests(),
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
		if !strings.Contains(recovered.(string), "FS is required") {
			t.Fatalf("unexpected panic value: %v", recovered)
		}
	}()
	_ = New(Config{})
}

func TestNewFromWaveConfigJSON(t *testing.T) {
	fixture := newWaveTestFixture(t)
	t.Chdir(fixture.root)
	setWaveDevModeForTest(t, false)
	configPath := fixture.mustWriteConfigFile(t)
	expectedRawConfigJSON, readConfigError := os.ReadFile(
		filepath.Join(fixture.root, configPath),
	)
	if readConfigError != nil {
		t.Fatalf("read fixture config: %v", readConfigError)
	}

	w := New(Config{
		FS:         os.DirFS(fixture.root),
		ConfigPath: configPath,
		Logger:     newDiscardLoggerForWaveTests(),
	})

	if w == nil {
		t.Fatal("expected non-nil Wave instance")
	}
	if !bytes.Equal(w.RawConfigJSON(), expectedRawConfigJSON) {
		t.Fatal("expected RawConfigJSON to match input WaveConfigJSON")
	}
}

func TestNewResolvesResolveRootRelativeToOSDirFSConfigLocation(
	t *testing.T,
) {
	root := t.TempDir()
	t.Chdir(root)

	mustWriteFile(
		t,
		filepath.Join(root, "backend", "wave.config.json"),
		`{
			"Core":{"ProjectID":"test-project",
				"ResolveRoot": "../",
				"ServerOnlyMode": true,
				"MainAppEntry": "backend/cmd/serve"
			}
		}`,
	)

	waveRuntime := New(Config{
		FS:         os.DirFS("backend"),
		ConfigPath: "wave.config.json",
		Logger:     newDiscardLoggerForWaveTests(),
	})

	expectedResolveRoot := filepath.Clean(root)
	if gotResolveRoot := waveenv.Absolute(
		waveRuntime.ParsedConfig().ResolveRoot(),
	); gotResolveRoot != expectedResolveRoot {
		t.Fatalf(
			"expected resolve root %q, got %q",
			expectedResolveRoot,
			gotResolveRoot,
		)
	}
}

func TestNewDiscoversNestedConfigPathByBasenameWhenUnique(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	configPath := filepath.Join("apps", "site", "backend", "wave.config.json")
	mustWriteFile(
		t,
		filepath.Join(root, configPath),
		`{
			"Core":{"ProjectID":"discovery-unique","MainAppEntry":"cmd/serve","ServerOnlyMode":true}
		}`,
	)
	if mkdirError := os.MkdirAll(
		filepath.Join(root, "apps", "site", "backend", ".wavedist", "static"),
		0o755,
	); mkdirError != nil {
		t.Fatalf("create discovered static directory: %v", mkdirError)
	}

	w := New(Config{
		FS:         os.DirFS(root),
		ConfigPath: "wave.config.json",
		Logger:     newDiscardLoggerForWaveTests(),
	})

	if got := w.ConfigFile(); got != filepath.ToSlash(configPath) {
		t.Fatalf("ConfigFile() = %q, want %q", got, filepath.ToSlash(configPath))
	}
}

func TestNewPanicsWhenConfigDiscoveryIsAmbiguousWithoutDirectMatch(
	t *testing.T,
) {
	root := t.TempDir()
	t.Chdir(root)

	mustWriteFile(
		t,
		filepath.Join(root, "app-a", "backend", "wave.config.json"),
		`{
			"Core":{"ProjectID":"project-a","MainAppEntry":"cmd/serve","ServerOnlyMode":true}
		}`,
	)
	mustWriteFile(
		t,
		filepath.Join(root, "app-b", "backend", "wave.config.json"),
		`{
			"Core":{"ProjectID":"project-b","MainAppEntry":"cmd/serve","ServerOnlyMode":true}
		}`,
	)
	for _, staticRoot := range []string{
		filepath.Join(root, "app-a", "backend", ".wavedist", "static"),
		filepath.Join(root, "app-b", "backend", ".wavedist", "static"),
	} {
		if mkdirError := os.MkdirAll(staticRoot, 0o755); mkdirError != nil {
			t.Fatalf("create static directory %q: %v", staticRoot, mkdirError)
		}
	}

	defer func() {
		recoveredPanic := recover()
		if recoveredPanic == nil {
			t.Fatal("expected panic for ambiguous config discovery")
		}
		panicMessage := fmt.Sprint(recoveredPanic)
		if !strings.Contains(panicMessage, "ambiguous config discovery") {
			t.Fatalf("unexpected panic message: %v", recoveredPanic)
		}
	}()

	_ = New(Config{
		FS:         os.DirFS(root),
		ConfigPath: "wave.config.json",
		Logger:     newDiscardLoggerForWaveTests(),
	})
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
		FS: fstest.MapFS{
			"wave.config.json": &fstest.MapFile{Data: []byte("{")},
		},
		ConfigPath: "wave.config.json",
	})
}

func TestNewCreatesMissingDistStaticDirectoryForOSDirFS(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	configPath := filepath.Join("backend", "wave.config.json")
	mustWriteFile(
		t,
		filepath.Join(root, configPath),
		`{
			"Core":{
				"ProjectID":"dist-auto-create",
				"MainAppEntry":"cmd/serve",
				"ServerOnlyMode":true
			}
		}`,
	)

	distStaticDirectoryPath := filepath.Join(
		root,
		"backend",
		".wavedist",
		"static",
	)
	if _, distStaticDirectoryPathError := os.Stat(distStaticDirectoryPath); !os.IsNotExist(distStaticDirectoryPathError) {
		t.Fatalf(
			"expected %q to be missing before New, stat error = %v",
			distStaticDirectoryPath,
			distStaticDirectoryPathError,
		)
	}

	w := New(Config{
		FS:         os.DirFS(root),
		ConfigPath: configPath,
		Logger:     newDiscardLoggerForWaveTests(),
	})
	if w == nil {
		t.Fatal("expected non-nil Wave instance")
	}

	distStaticDirectoryInfo, distStaticDirectoryPathError := os.Stat(
		distStaticDirectoryPath,
	)
	if distStaticDirectoryPathError != nil {
		t.Fatalf(
			"expected dist static directory to exist after New: %v",
			distStaticDirectoryPathError,
		)
	}
	if !distStaticDirectoryInfo.IsDir() {
		t.Fatalf(
			"expected %q to be a directory after New",
			distStaticDirectoryPath,
		)
	}
}

func TestNewPanicsWhenCWDConfigDiscoveryUnavailable(
	t *testing.T,
) {
	root := t.TempDir()
	configPath := filepath.Join("backend", "wave.config.json")
	mustWriteFile(
		t,
		filepath.Join(root, configPath),
		`{
			"Core":{
				"ProjectID":"dist-create-contract-panic",
				"MainAppEntry":"cmd/serve",
				"ServerOnlyMode":true
			}
		}`,
	)

	defer func() {
		recoveredPanic := recover()
		if recoveredPanic == nil {
			t.Fatal("expected New to panic when CWD config discovery is unavailable")
		}
		panicMessage := fmt.Sprint(recoveredPanic)
		if !strings.Contains(
			panicMessage,
			"resolve config path",
		) {
			t.Fatalf("unexpected panic message: %v", recoveredPanic)
		}
	}()

	_ = New(Config{
		FS:         os.DirFS(root),
		ConfigPath: configPath,
		Logger:     newDiscardLoggerForWaveTests(),
	})
}

func TestNewCreatesWaveAndExposesConfigurationMutators(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(
		t,
		fixture,
		false,
		os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
	)

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

func TestConfigFileReturnsConstructorConfigPath(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(
		t,
		fixture,
		false,
		os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
	)
	expectedConfigPath := fixture.pathInRoot("wave.config.json")
	if got := w.ConfigFile(); !waveenv.PathsReferToSameLocation(
		got,
		expectedConfigPath,
	) {
		t.Fatalf(
			"ConfigFile() = %q, want path equivalent to %q",
			got,
			expectedConfigPath,
		)
	}
}

func TestFrameworkSettersDoNotMutateCoreConfigFields(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(
		t,
		fixture,
		false,
		os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
	)

	originalCoreConfig := w.cfg.Core().Clone()

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

	if !reflect.DeepEqual(w.cfg.Core(), originalCoreConfig) {
		t.Fatalf(
			"framework setters mutated core config: got %#v want %#v",
			w.cfg.Core(),
			originalCoreConfig,
		)
	}
}

func TestAddFrameworkWatchPatternsAppendsInput(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(
		t,
		fixture,
		false,
		os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
	)

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

func TestBaseFSProductionCreatesMissingDistStaticFS(t *testing.T) {
	fixture := newWaveTestFixture(t)
	if removeError := os.RemoveAll(
		fixture.pathInRoot(fixture.cfg.Dist().Static()),
	); removeError != nil {
		t.Fatalf("remove dist static directory: %v", removeError)
	}
	w := newWaveForTest(t, fixture, false, nil)
	if w == nil {
		t.Fatal("expected New to return a non-nil Wave instance")
	}
	distStaticDirectoryInfo, distStaticDirectoryStatError := os.Stat(
		fixture.pathInRoot(fixture.cfg.Dist().Static()),
	)
	if distStaticDirectoryStatError != nil {
		t.Fatalf("expected dist static directory to be recreated: %v", distStaticDirectoryStatError)
	}
	if !distStaticDirectoryInfo.IsDir() {
		t.Fatalf(
			"expected dist static path %q to be a directory",
			fixture.pathInRoot(fixture.cfg.Dist().Static()),
		)
	}
}

func TestBaseFSProductionUsesProvidedFS(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, nil)

	baseFS, err := w.runtime.GetBaseFS()
	if err != nil {
		t.Fatalf("baseFS returned error: %v", err)
	}

	if got := mustReadFileFromFS(t, baseFS, "internal/critical.css"); got != "body{color:red;}" {
		t.Fatalf(
			"unexpected critical css content from base FS: %q",
			got,
		)
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
	w := newWaveForTest(
		t,
		fixture,
		false,
		os.DirFS(fixture.pathInRoot(fixture.cfg.Dist().Static())),
	)

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
	if err := os.Remove(fixture.pathInRoot(fixture.cfg.Dist().PublicFileMapRef())); err != nil {
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
	wavetest.SetCoreCriticalCSSEntryFile(fixture.cfg, "")
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
	if err := os.Remove(fixture.pathInRoot(fixture.cfg.Dist().CriticalCSS())); err != nil {
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
	wavetest.SetCoreNonCriticalCSSEntryFile(fixture.cfg, "")
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
	wavetest.SetCorePublicPathPrefix(fixture.cfg, "/")
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
	expectedDistDir := fixture.cfg.Dist().Root()
	if !waveenv.PathsReferToSameLocation(w.DistDir(), expectedDistDir) {
		t.Fatalf(
			"unexpected dist dir: got %q, want path equivalent to %q",
			w.DistDir(),
			expectedDistDir,
		)
	}
	expectedPublicStaticDir := fixture.cfg.Core().StaticAssetDirsPublic()
	if !waveenv.PathsReferToSameLocation(
		w.runtime.PublicStaticDir(),
		expectedPublicStaticDir,
	) {
		t.Fatalf(
			"unexpected public static dir: got %q, want path equivalent to %q",
			w.runtime.PublicStaticDir(),
			expectedPublicStaticDir,
		)
	}
	expectedPrivateStaticDir := fixture.cfg.Core().StaticAssetDirsPrivate()
	if !waveenv.PathsReferToSameLocation(
		w.PrivateStaticDir(),
		expectedPrivateStaticDir,
	) {
		t.Fatalf(
			"unexpected private static dir: got %q, want path equivalent to %q",
			w.PrivateStaticDir(),
			expectedPrivateStaticDir,
		)
	}
	expectedViteManifestLocation := fixture.cfg.ViteManifestPath()
	if !waveenv.PathsReferToSameLocation(
		w.ViteManifestLocation(),
		expectedViteManifestLocation,
	) {
		t.Fatalf(
			"unexpected Vite manifest location: got %q, want path equivalent to %q",
			w.ViteManifestLocation(),
			expectedViteManifestLocation,
		)
	}
	expectedViteOutDir := fixture.cfg.Dist().StaticPublic()
	if !waveenv.PathsReferToSameLocation(
		w.runtime.ViteOutDir(),
		expectedViteOutDir,
	) {
		t.Fatalf(
			"unexpected Vite out dir: got %q, want path equivalent to %q",
			w.runtime.ViteOutDir(),
			expectedViteOutDir,
		)
	}
	expectedStaticPrivateOutDir := fixture.cfg.Dist().StaticPrivate()
	if !waveenv.PathsReferToSameLocation(
		w.StaticPrivateOutDir(),
		expectedStaticPrivateOutDir,
	) {
		t.Fatalf(
			"unexpected static private out dir: got %q, want path equivalent to %q",
			w.StaticPrivateOutDir(),
			expectedStaticPrivateOutDir,
		)
	}
	expectedStaticPublicOutDir := fixture.cfg.Dist().StaticPublic()
	if !waveenv.PathsReferToSameLocation(
		w.StaticPublicOutDir(),
		expectedStaticPublicOutDir,
	) {
		t.Fatalf(
			"unexpected static public out dir: got %q, want path equivalent to %q",
			w.StaticPublicOutDir(),
			expectedStaticPublicOutDir,
		)
	}

	parsedConfig := w.ParsedConfig()
	if parsedConfig == nil {
		t.Fatal("expected Wave.ParsedConfig() to return non-nil parsed config")
	}
	parsedConfigAgain := w.ParsedConfig()
	if parsedConfigAgain != parsedConfig {
		t.Fatal(
			"expected Wave.ParsedConfig() to return stable canonical parsed config across repeated calls",
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

	buildtimeParsedConfig := w.ParsedConfig()
	if buildtimeParsedConfig != parsedConfig {
		t.Fatal(
			"expected Wave.ParsedConfig() to resolve to canonical parsed config instance",
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
			"expected Wave.ParsedConfig() to preserve schema extensions",
		)
	}
	if waveframework.StateForConfig(buildtimeParsedConfig).RunBuildHook == nil {
		t.Fatal(
			"expected Wave.ParsedConfig() to preserve run build hook callback",
		)
	}
}

func TestWaveParsedConfigIncludesFrameworkBuildCallbacks(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, true, nil)

	parsedConfig := w.ParsedConfig()
	if parsedConfig == nil {
		t.Fatal(
			"expected Wave.ParsedConfig() to return non-nil parsed config",
		)
	}
	waveframework.StateForConfig(parsedConfig).DevBuildHook = "go run ./backend/cmd/build --dev"
	waveframework.StateForConfig(parsedConfig).SchemaExtensions = map[string]jsonschema.Entry{
		"Custom": {Type: jsonschema.TypeObject},
	}
	waveframework.StateForConfig(parsedConfig).RunBuildHook = func(context.Context, bool) error {
		return nil
	}

	buildtimeParsedConfig := w.ParsedConfig()
	if buildtimeParsedConfig != parsedConfig {
		t.Fatal(
			"expected Wave.ParsedConfig() to return stable canonical parsed config",
		)
	}
	if got := waveframework.StateForConfig(buildtimeParsedConfig).DevBuildHook; got != "go run ./backend/cmd/build --dev" {
		t.Fatalf("FrameworkDevBuildHook = %q, want configured value", got)
	}
	if _, ok := waveframework.StateForConfig(buildtimeParsedConfig).SchemaExtensions["Custom"]; !ok {
		t.Fatal(
			"expected Wave.ParsedConfig() snapshot to include framework schema extensions",
		)
	}
	if waveframework.StateForConfig(buildtimeParsedConfig).RunBuildHook == nil {
		t.Fatal(
			"expected Wave.ParsedConfig() snapshot to include framework run build hook",
		)
	}
}

func TestMustGetFSAndStaticHandlerPanicsOnFailure(t *testing.T) {
	fixture := newWaveTestFixture(t)
	w := newWaveForTest(t, fixture, false, nil)
	if removeErr := os.RemoveAll(fixture.pathInRoot(fixture.cfg.Dist().Static())); removeErr != nil {
		t.Fatalf("remove dist static directory: %v", removeErr)
	}

	publicFS, publicFSError := w.runtime.GetPublicFS()
	if publicFSError != nil {
		t.Fatalf("unexpected getPublicFS error: %v", publicFSError)
	}
	if _, readErr := fs.ReadFile(publicFS, "logo.txt"); readErr == nil {
		t.Fatal(
			"expected public FS reads to fail when dist static directory is unavailable",
		)
	}

	privateFS := w.MustPrivateFS()
	if _, readErr := fs.ReadFile(privateFS, "template.html"); readErr == nil {
		t.Fatal(
			"expected private FS reads to fail when dist static directory is unavailable",
		)
	}
}
