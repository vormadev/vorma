package wave

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/vormadev/vorma/wave/internal/wavecore"
)

func TestRelPathsAreStable(t *testing.T) {
	if RelPaths.Internal() != "internal" {
		t.Fatalf("unexpected internal rel path: %q", RelPaths.Internal())
	}
	if RelPaths.assetsPublic() != "assets/public" {
		t.Fatalf(
			"unexpected public assets rel path: %q",
			RelPaths.assetsPublic(),
		)
	}
	if RelPaths.assetsPrivate() != "assets/private" {
		t.Fatalf(
			"unexpected private assets rel path: %q",
			RelPaths.assetsPrivate(),
		)
	}
	if RelPaths.CriticalCSS() != "internal/critical.css" {
		t.Fatalf("unexpected critical css rel path: %q", RelPaths.CriticalCSS())
	}
	if RelPaths.NormalCSSRef() != "internal/normal_css_file_ref.txt" {
		t.Fatalf(
			"unexpected normal css ref rel path: %q",
			RelPaths.NormalCSSRef(),
		)
	}
	if RelPaths.PublicFileMapRef() != "internal/public_file_map_file_ref.txt" {
		t.Fatalf(
			"unexpected public file map ref rel path: %q",
			RelPaths.PublicFileMapRef(),
		)
	}
	if RelPaths.PublicFileMapGob() != "internal/public_filemap.gob" {
		t.Fatalf(
			"unexpected public file map gob rel path: %q",
			RelPaths.PublicFileMapGob(),
		)
	}
	if RelPaths.publicFileMapGobName() != "public_filemap.gob" {
		t.Fatalf(
			"unexpected public file map gob name: %q",
			RelPaths.publicFileMapGobName(),
		)
	}
	if RelPaths.privateFileMapGobName() != "private_filemap.gob" {
		t.Fatalf(
			"unexpected private file map gob name: %q",
			RelPaths.privateFileMapGobName(),
		)
	}
	if RelPaths.PublicFileMapJSName() != "vorma_internal_public_filemap.js" {
		t.Fatalf(
			"unexpected public file map js name: %q",
			RelPaths.PublicFileMapJSName(),
		)
	}
	if RelPaths.PublicFileMapTSName() != "filemap.ts" {
		t.Fatalf(
			"unexpected public file map ts name: %q",
			RelPaths.PublicFileMapTSName(),
		)
	}
	if RelPaths.PublicFileMapJSONName() != "filemap.json" {
		t.Fatalf(
			"unexpected public file map json name: %q",
			RelPaths.PublicFileMapJSONName(),
		)
	}
}

func TestDistLayoutBuildsExpectedPaths(t *testing.T) {
	d := distLayout{Root: filepath.Join("tmp", "dist")}

	expectedBinaryName := "main"
	if runtime.GOOS == "windows" {
		expectedBinaryName = "main.exe"
	}
	if d.Binary() != filepath.Join(d.Root, expectedBinaryName) {
		t.Fatalf("unexpected binary path: %q", d.Binary())
	}
	if d.Static() != filepath.Join(d.Root, "static") {
		t.Fatalf("unexpected static path: %q", d.Static())
	}
	if d.StaticPublic() != filepath.Join(d.Root, "static", "assets", "public") {
		t.Fatalf("unexpected static public path: %q", d.StaticPublic())
	}
	if d.StaticPrivate() != filepath.Join(
		d.Root,
		"static",
		"assets",
		"private",
	) {
		t.Fatalf("unexpected static private path: %q", d.StaticPrivate())
	}
	if d.PublicFileMapGob() != filepath.Join(
		d.Root,
		"static",
		"internal",
		"public_filemap.gob",
	) {
		t.Fatalf("unexpected public filemap gob path: %q", d.PublicFileMapGob())
	}
	if d.PrivateFileMapGob() != filepath.Join(
		d.Root,
		"static",
		"internal",
		"private_filemap.gob",
	) {
		t.Fatalf(
			"unexpected private filemap gob path: %q",
			d.PrivateFileMapGob(),
		)
	}
	if d.KeepFile() != filepath.Join(d.Root, "static", ".keep") {
		t.Fatalf("unexpected keep file path: %q", d.KeepFile())
	}
}

func TestFileMapLookupMappedAndMiss(t *testing.T) {
	fm := FileMap{
		"logo.txt": {
			DistName: "vorma_out/logo.hash.txt",
		},
	}

	url, found := fm.Lookup("/logo.txt", "/assets/")
	if !found {
		t.Fatal("expected mapped asset to be found")
	}
	if url != "/assets/vorma_out/logo.hash.txt" {
		t.Fatalf("unexpected mapped URL: %q", url)
	}

	missing, missingFound := fm.Lookup("missing.txt", "/assets/")
	if missingFound {
		t.Fatal("expected missing asset lookup to report not found")
	}
	if missing != "" {
		t.Fatalf("expected empty URL for missing lookup, got %q", missing)
	}
}

func TestResolvePublicURLFromReferencedPath(t *testing.T) {
	testCases := []struct {
		name                string
		publicPathPrefix    string
		referencedPath      string
		expectedResolvedURL string
	}{
		{
			name:                "joins normalized referenced path under prefix",
			publicPathPrefix:    "/assets/",
			referencedPath:      "vorma_out/styles.hash.css",
			expectedResolvedURL: "/assets/vorma_out/styles.hash.css",
		},
		{
			name:                "trims and normalizes traversal path under prefix",
			publicPathPrefix:    "/assets/",
			referencedPath:      " ../outside.css \n",
			expectedResolvedURL: "/assets/outside.css",
		},
		{
			name:                "root prefix remains rooted",
			publicPathPrefix:    "/",
			referencedPath:      "/styles/site.css",
			expectedResolvedURL: "/styles/site.css",
		},
		{
			name:                "empty path resolves to empty URL",
			publicPathPrefix:    "/assets/",
			referencedPath:      "",
			expectedResolvedURL: "",
		},
		{
			name:                "whitespace-only path resolves to empty URL",
			publicPathPrefix:    "/assets/",
			referencedPath:      " \n\t ",
			expectedResolvedURL: "",
		},
		{
			name:                "slash-only path resolves to empty URL",
			publicPathPrefix:    "/assets/",
			referencedPath:      "/",
			expectedResolvedURL: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resolvedURL := wavecore.ResolveFromReferencedPath(
				testCase.publicPathPrefix,
				testCase.referencedPath,
			)
			if resolvedURL != testCase.expectedResolvedURL {
				t.Fatalf(
					"resolvePublicURLFromReferencedPath(%q, %q) = %q, want %q",
					testCase.publicPathPrefix,
					testCase.referencedPath,
					resolvedURL,
					testCase.expectedResolvedURL,
				)
			}
		})
	}
}

func TestFileMapLookupPreventsPrefixEscapeFromTraversalInput(t *testing.T) {
	fm := FileMap{
		"nested/logo.txt": {
			DistName: "vorma_out/nested.logo.hash.txt",
		},
	}

	escapedURL, found := fm.Lookup("../secret.txt", "/assets/")
	if found {
		t.Fatal("expected traversal input to be reported as not found")
	}
	if escapedURL != "" {
		t.Fatalf(
			"expected empty URL for missing traversal lookup, got %q",
			escapedURL,
		)
	}

	mappedURL, mappedFound := fm.Lookup("/../nested/logo.txt", "/assets/")
	if !mappedFound {
		t.Fatal("expected normalized traversal input to match mapped entry")
	}
	if mappedURL != "/assets/vorma_out/nested.logo.hash.txt" {
		t.Fatalf(
			"unexpected mapped URL for normalized traversal input: %q",
			mappedURL,
		)
	}
}

func TestFileMapLookupWithAlreadyPrefixedInputDoesNotDuplicatePrefix(
	t *testing.T,
) {
	fm := FileMap{
		"logo.txt": {
			DistName: "vorma_out/logo.hash.txt",
		},
	}

	mappedURL, mappedFound := fm.Lookup("/assets/logo.txt", "/assets/")
	if !mappedFound {
		t.Fatal("expected already-prefixed mapped input to be found")
	}
	if mappedURL != "/assets/vorma_out/logo.hash.txt" {
		t.Fatalf(
			"unexpected mapped URL for already-prefixed input: %q",
			mappedURL,
		)
	}

	fallbackURL, fallbackFound := fm.Lookup("/assets/missing.txt", "/assets/")
	if fallbackFound {
		t.Fatal("expected missing already-prefixed input to report not found")
	}
	if fallbackURL != "" {
		t.Fatalf(
			"expected empty URL for missing already-prefixed input, got %q",
			fallbackURL,
		)
	}

	idempotentURL, idempotentFound := fm.Lookup(
		"/assets/vorma_out/logo.hash.txt",
		"/assets/",
	)
	if idempotentFound {
		t.Fatal("expected already-resolved hashed URL to report not found")
	}
	if idempotentURL != "" {
		t.Fatalf(
			"expected empty URL for already-resolved hashed lookup, got %q",
			idempotentURL,
		)
	}
}

func TestParsedConfigPublicPathPrefixNormalization(t *testing.T) {
	cfg := &ParsedConfig{Core: &CoreConfig{}}

	if got := cfg.PublicPathPrefix(); got != "/" {
		t.Fatalf("expected default prefix '/', got %q", got)
	}

	cfg.Core.PublicPathPrefix = "/"
	if got := cfg.PublicPathPrefix(); got != "/" {
		t.Fatalf("expected '/' to remain '/', got %q", got)
	}

	cfg.Core.PublicPathPrefix = "assets"
	if got := cfg.PublicPathPrefix(); got != "/assets/" {
		t.Fatalf("expected normalized prefix '/assets/', got %q", got)
	}

	cfg.Core.PublicPathPrefix = "/assets"
	if got := cfg.PublicPathPrefix(); got != "/assets/" {
		t.Fatalf("expected normalized prefix '/assets/', got %q", got)
	}
}

func TestParsedConfigAccessors(t *testing.T) {
	cfg := &ParsedConfig{
		Core: &CoreConfig{ServerOnlyMode: true},
		Vite: &viteConfig{},
		Watch: &WatchConfig{
			WatchRoot:           "./tmp/../tmp/watch",
			HealthcheckEndpoint: "/ok",
		},
		Dist: distLayout{Root: filepath.Join("tmp", "dist")},
	}

	if cfg.WatchRoot() != filepath.Clean("./tmp/../tmp/watch") {
		t.Fatalf("unexpected watch root: %q", cfg.WatchRoot())
	}
	if cfg.HealthcheckEndpoint() != "/ok" {
		t.Fatalf(
			"unexpected healthcheck endpoint: %q",
			cfg.HealthcheckEndpoint(),
		)
	}
	if cfg.UsingBrowser() {
		t.Fatal("expected server-only mode to disable browser usage")
	}
	if !cfg.UsingVite() {
		t.Fatal("expected non-nil Vite config to report UsingVite=true")
	}

	expectedManifest := filepath.Join(
		cfg.Dist.StaticPrivate(),
		"vorma_out",
		"vorma_vite_manifest.json",
	)
	if cfg.ViteManifestPath() != expectedManifest {
		t.Fatalf("unexpected manifest path: %q", cfg.ViteManifestPath())
	}
}

func TestParsedConfigDefaultsWhenWatchConfigMissing(t *testing.T) {
	cfg := &ParsedConfig{Core: &CoreConfig{}}

	if cfg.WatchRoot() != "." {
		t.Fatalf("expected default watch root '.', got %q", cfg.WatchRoot())
	}
	if cfg.HealthcheckEndpoint() != "/" {
		t.Fatalf(
			"expected default healthcheck endpoint '/', got %q",
			cfg.HealthcheckEndpoint(),
		)
	}
}

func TestParsedConfigCSSEntryCleaning(t *testing.T) {
	cfg := &ParsedConfig{Core: &CoreConfig{}}
	if cfg.CriticalCSSEntry() != "" {
		t.Fatalf(
			"expected empty critical css entry by default, got %q",
			cfg.CriticalCSSEntry(),
		)
	}
	if cfg.NonCriticalCSSEntry() != "" {
		t.Fatalf(
			"expected empty non-critical css entry by default, got %q",
			cfg.NonCriticalCSSEntry(),
		)
	}

	cfg.Core.CSSEntryFiles.Critical = "./styles/../critical.css"
	cfg.Core.CSSEntryFiles.NonCritical = "./styles/./app.css"

	if cfg.CriticalCSSEntry() != filepath.Clean("./styles/../critical.css") {
		t.Fatalf(
			"unexpected cleaned critical css entry: %q",
			cfg.CriticalCSSEntry(),
		)
	}
	if cfg.NonCriticalCSSEntry() != filepath.Clean("./styles/./app.css") {
		t.Fatalf(
			"unexpected cleaned non-critical css entry: %q",
			cfg.NonCriticalCSSEntry(),
		)
	}
}

func TestParsedConfigBrowserRuntimeDefaultsAndOverrides(t *testing.T) {
	var nilCfg *ParsedConfig
	if got := nilCfg.browserRuntimeNamespace(); got != defaultBrowserRuntimeNamespace {
		t.Fatalf(
			"nil config browser runtime namespace = %q, want %q",
			got,
			defaultBrowserRuntimeNamespace,
		)
	}

	cfg := &ParsedConfig{Core: &CoreConfig{}}
	if got := cfg.browserRuntimeNamespace(); got != defaultBrowserRuntimeNamespace {
		t.Fatalf(
			"default browser runtime namespace = %q, want %q",
			got,
			defaultBrowserRuntimeNamespace,
		)
	}
	if got := cfg.browserPublicURLResolverFunctionName(); got != defaultBrowserPublicURLResolverFunctionName {
		t.Fatalf(
			"default public URL resolver function name = %q, want %q",
			got,
			defaultBrowserPublicURLResolverFunctionName,
		)
	}
	if got := cfg.browserRevalidateFunctionName(); got != defaultBrowserRevalidateFunctionName {
		t.Fatalf(
			"default browser revalidate function name = %q, want %q",
			got,
			defaultBrowserRevalidateFunctionName,
		)
	}
	if got := cfg.refreshRebuildingOverlayElementID(); got != defaultRefreshRebuildingOverlayElementID {
		t.Fatalf(
			"default refresh rebuilding overlay element ID = %q, want %q",
			got,
			defaultRefreshRebuildingOverlayElementID,
		)
	}
	if got := cfg.criticalCSSStyleElementID(); got != defaultCriticalCSSStyleElementID {
		t.Fatalf(
			"default critical CSS style element ID = %q, want %q",
			got,
			defaultCriticalCSSStyleElementID,
		)
	}
	if got := cfg.nonCriticalCSSLinkElementID(); got != defaultNonCriticalCSSLinkElementID {
		t.Fatalf(
			"default non-critical CSS link element ID = %q, want %q",
			got,
			defaultNonCriticalCSSLinkElementID,
		)
	}

	cfg.FrameworkBrowserRuntimeNamespace = "__vorma_runtime"
	cfg.FrameworkBrowserPublicURLResolverFunctionName = "resolvePublicURL"
	cfg.FrameworkBrowserRevalidateFunctionName = "__vorma_revalidate"
	cfg.FrameworkRefreshRebuildingOverlayElementID = "vorma-refresh-overlay"
	cfg.FrameworkCriticalCSSStyleElementID = "vorma-critical-css"
	cfg.FrameworkNonCriticalCSSLinkElementID = "vorma-noncritical-css"

	if got := cfg.browserRuntimeNamespace(); got != "__vorma_runtime" {
		t.Fatalf(
			"configured browser runtime namespace = %q, want __vorma_runtime",
			got,
		)
	}
	if got := cfg.browserPublicURLResolverFunctionName(); got != "resolvePublicURL" {
		t.Fatalf(
			"configured public URL resolver function name = %q, want resolvePublicURL",
			got,
		)
	}
	if got := cfg.browserRevalidateFunctionName(); got != "__vorma_revalidate" {
		t.Fatalf(
			"configured browser revalidate function name = %q, want __vorma_revalidate",
			got,
		)
	}
	if got := cfg.refreshRebuildingOverlayElementID(); got != "vorma-refresh-overlay" {
		t.Fatalf(
			"configured refresh rebuilding overlay element ID = %q, want vorma-refresh-overlay",
			got,
		)
	}
	if got := cfg.criticalCSSStyleElementID(); got != "vorma-critical-css" {
		t.Fatalf(
			"configured critical CSS style element ID = %q, want vorma-critical-css",
			got,
		)
	}
	if got := cfg.nonCriticalCSSLinkElementID(); got != "vorma-noncritical-css" {
		t.Fatalf(
			"configured non-critical CSS link element ID = %q, want vorma-noncritical-css",
			got,
		)
	}
}

func TestWatchedFileSortGroupsHooksAndIsIdempotent(t *testing.T) {
	wf := &WatchedFile{
		OnChangeHooks: []OnChangeHook{
			{Cmd: "default"},
			{Cmd: "post", Timing: OnChangeStrategyPost},
			{Cmd: "concurrent", Timing: OnChangeStrategyConcurrent},
			{Cmd: "fire-and-forget", Timing: onChangeStrategyConcurrentNoWait},
		},
	}

	wf.Sort()
	if wf.SortedHooks == nil {
		t.Fatal("expected SortedHooks to be initialized")
	}
	if len(wf.SortedHooks.Pre) != 1 || wf.SortedHooks.Pre[0].Cmd != "default" {
		t.Fatalf("unexpected pre hooks: %+v", wf.SortedHooks.Pre)
	}
	if len(wf.SortedHooks.Post) != 1 || wf.SortedHooks.Post[0].Cmd != "post" {
		t.Fatalf("unexpected post hooks: %+v", wf.SortedHooks.Post)
	}
	if len(wf.SortedHooks.Concurrent) != 1 ||
		wf.SortedHooks.Concurrent[0].Cmd != "concurrent" {
		t.Fatalf("unexpected concurrent hooks: %+v", wf.SortedHooks.Concurrent)
	}
	if len(wf.SortedHooks.ConcurrentNoWait) != 1 ||
		wf.SortedHooks.ConcurrentNoWait[0].Cmd != "fire-and-forget" {
		t.Fatalf(
			"unexpected concurrent-no-wait hooks: %+v",
			wf.SortedHooks.ConcurrentNoWait,
		)
	}

	wf.Sort()
	if len(wf.SortedHooks.Pre) != 1 || len(wf.SortedHooks.Post) != 1 ||
		len(wf.SortedHooks.Concurrent) != 1 ||
		len(wf.SortedHooks.ConcurrentNoWait) != 1 {
		t.Fatal("expected second Sort call to be a no-op")
	}
}

func TestRefreshActionMergeAndIsZero(t *testing.T) {
	var empty RefreshAction
	if !empty.IsZero() {
		t.Fatal("expected zero RefreshAction to report IsZero=true")
	}

	a := RefreshAction{ReloadBrowser: true, WaitForVite: true}
	b := RefreshAction{
		WaitForApp:     true,
		TriggerRestart: true,
		RecompileGo:    true,
	}
	merged := a.merge(b)

	if !merged.ReloadBrowser || !merged.WaitForVite || !merged.WaitForApp ||
		!merged.TriggerRestart ||
		!merged.RecompileGo {
		t.Fatalf("unexpected merged refresh action: %+v", merged)
	}
	if merged.IsZero() {
		t.Fatal("expected merged RefreshAction to be non-zero")
	}

	firstRequest := &FrameworkRuntimeReloadRequest{
		EndpointPath: "/reload-routes",
	}
	secondRequest := &FrameworkRuntimeReloadRequest{
		EndpointPath: "/reload-template",
	}
	mergedWithFrameworkRequest := RefreshAction{
		FrameworkRuntimeReloadRequest: firstRequest,
	}.merge(RefreshAction{
		FrameworkRuntimeReloadRequest: secondRequest,
	})
	if mergedWithFrameworkRequest.FrameworkRuntimeReloadRequest != firstRequest {
		t.Fatalf(
			"expected first non-nil framework runtime reload request to win merge, got %#v",
			mergedWithFrameworkRequest.FrameworkRuntimeReloadRequest,
		)
	}
}
