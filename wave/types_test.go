package wave

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestRelPathsAreStable(t *testing.T) {
	if RelPaths.Internal() != "internal" {
		t.Fatalf("unexpected internal rel path: %q", RelPaths.Internal())
	}
	if RelPaths.AssetsPublic() != "assets/public" {
		t.Fatalf("unexpected public assets rel path: %q", RelPaths.AssetsPublic())
	}
	if RelPaths.AssetsPrivate() != "assets/private" {
		t.Fatalf("unexpected private assets rel path: %q", RelPaths.AssetsPrivate())
	}
	if RelPaths.CriticalCSS() != "internal/critical.css" {
		t.Fatalf("unexpected critical css rel path: %q", RelPaths.CriticalCSS())
	}
	if RelPaths.NormalCSSRef() != "internal/normal_css_file_ref.txt" {
		t.Fatalf("unexpected normal css ref rel path: %q", RelPaths.NormalCSSRef())
	}
	if RelPaths.PublicFileMapRef() != "internal/public_file_map_file_ref.txt" {
		t.Fatalf("unexpected public file map ref rel path: %q", RelPaths.PublicFileMapRef())
	}
	if RelPaths.PublicFileMapGob() != "internal/public_filemap.gob" {
		t.Fatalf("unexpected public file map gob rel path: %q", RelPaths.PublicFileMapGob())
	}
	if RelPaths.PublicFileMapGobName() != "public_filemap.gob" {
		t.Fatalf("unexpected public file map gob name: %q", RelPaths.PublicFileMapGobName())
	}
	if RelPaths.PrivateFileMapGobName() != "private_filemap.gob" {
		t.Fatalf("unexpected private file map gob name: %q", RelPaths.PrivateFileMapGobName())
	}
	if RelPaths.PublicFileMapJSName() != "vorma_internal_public_filemap.js" {
		t.Fatalf("unexpected public file map js name: %q", RelPaths.PublicFileMapJSName())
	}
	if RelPaths.PublicFileMapTSName() != "filemap.ts" {
		t.Fatalf("unexpected public file map ts name: %q", RelPaths.PublicFileMapTSName())
	}
	if RelPaths.PublicFileMapJSONName() != "filemap.json" {
		t.Fatalf("unexpected public file map json name: %q", RelPaths.PublicFileMapJSONName())
	}
}

func TestDistLayoutBuildsExpectedPaths(t *testing.T) {
	d := DistLayout{Root: filepath.Join("tmp", "dist")}

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
	if d.StaticPrivate() != filepath.Join(d.Root, "static", "assets", "private") {
		t.Fatalf("unexpected static private path: %q", d.StaticPrivate())
	}
	if d.PublicFileMapGob() != filepath.Join(d.Root, "static", "internal", "public_filemap.gob") {
		t.Fatalf("unexpected public filemap gob path: %q", d.PublicFileMapGob())
	}
	if d.PrivateFileMapGob() != filepath.Join(d.Root, "static", "internal", "private_filemap.gob") {
		t.Fatalf("unexpected private filemap gob path: %q", d.PrivateFileMapGob())
	}
	if d.KeepFile() != filepath.Join(d.Root, "static", ".keep") {
		t.Fatalf("unexpected keep file path: %q", d.KeepFile())
	}
}

func TestFileMapLookupMappedAndFallback(t *testing.T) {
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

	fallback, fallbackFound := fm.Lookup("missing.txt", "/assets/")
	if fallbackFound {
		t.Fatal("expected missing asset lookup to report not found")
	}
	if fallback != "/assets/missing.txt" {
		t.Fatalf("unexpected fallback URL: %q", fallback)
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
		t.Fatal("expected traversal input to be treated as fallback (not found)")
	}
	if escapedURL != "/assets/secret.txt" {
		t.Fatalf("expected traversal fallback to stay under prefix, got %q", escapedURL)
	}

	mappedURL, mappedFound := fm.Lookup("/../nested/logo.txt", "/assets/")
	if !mappedFound {
		t.Fatal("expected normalized traversal input to match mapped entry")
	}
	if mappedURL != "/assets/vorma_out/nested.logo.hash.txt" {
		t.Fatalf("unexpected mapped URL for normalized traversal input: %q", mappedURL)
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
		Vite: &ViteConfig{},
		Watch: &WatchConfig{
			WatchRoot:           "./tmp/../tmp/watch",
			HealthcheckEndpoint: "/ok",
		},
		Dist: DistLayout{Root: filepath.Join("tmp", "dist")},
	}

	if cfg.WatchRoot() != filepath.Clean("./tmp/../tmp/watch") {
		t.Fatalf("unexpected watch root: %q", cfg.WatchRoot())
	}
	if cfg.HealthcheckEndpoint() != "/ok" {
		t.Fatalf("unexpected healthcheck endpoint: %q", cfg.HealthcheckEndpoint())
	}
	if cfg.UsingBrowser() {
		t.Fatal("expected server-only mode to disable browser usage")
	}
	if !cfg.UsingVite() {
		t.Fatal("expected non-nil Vite config to report UsingVite=true")
	}

	expectedManifest := filepath.Join(cfg.Dist.StaticPrivate(), "vorma_out", "vorma_vite_manifest.json")
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
		t.Fatalf("expected default healthcheck endpoint '/', got %q", cfg.HealthcheckEndpoint())
	}
}

func TestParsedConfigCSSEntryCleaning(t *testing.T) {
	cfg := &ParsedConfig{Core: &CoreConfig{}}
	if cfg.CriticalCSSEntry() != "" {
		t.Fatalf("expected empty critical css entry by default, got %q", cfg.CriticalCSSEntry())
	}
	if cfg.NonCriticalCSSEntry() != "" {
		t.Fatalf("expected empty non-critical css entry by default, got %q", cfg.NonCriticalCSSEntry())
	}

	cfg.Core.CSSEntryFiles.Critical = "./styles/../critical.css"
	cfg.Core.CSSEntryFiles.NonCritical = "./styles/./app.css"

	if cfg.CriticalCSSEntry() != filepath.Clean("./styles/../critical.css") {
		t.Fatalf("unexpected cleaned critical css entry: %q", cfg.CriticalCSSEntry())
	}
	if cfg.NonCriticalCSSEntry() != filepath.Clean("./styles/./app.css") {
		t.Fatalf("unexpected cleaned non-critical css entry: %q", cfg.NonCriticalCSSEntry())
	}
}

func TestWatchedFileSortGroupsHooksAndIsIdempotent(t *testing.T) {
	wf := &WatchedFile{
		OnChangeHooks: []OnChangeHook{
			{Cmd: "default"},
			{Cmd: "post", Timing: OnChangeStrategyPost},
			{Cmd: "concurrent", Timing: OnChangeStrategyConcurrent},
			{Cmd: "fire-and-forget", Timing: OnChangeStrategyConcurrentNoWait},
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
	if len(wf.SortedHooks.Concurrent) != 1 || wf.SortedHooks.Concurrent[0].Cmd != "concurrent" {
		t.Fatalf("unexpected concurrent hooks: %+v", wf.SortedHooks.Concurrent)
	}
	if len(wf.SortedHooks.ConcurrentNoWait) != 1 || wf.SortedHooks.ConcurrentNoWait[0].Cmd != "fire-and-forget" {
		t.Fatalf("unexpected concurrent-no-wait hooks: %+v", wf.SortedHooks.ConcurrentNoWait)
	}

	wf.Sort()
	if len(wf.SortedHooks.Pre) != 1 || len(wf.SortedHooks.Post) != 1 || len(wf.SortedHooks.Concurrent) != 1 || len(wf.SortedHooks.ConcurrentNoWait) != 1 {
		t.Fatal("expected second Sort call to be a no-op")
	}
}

func TestRefreshActionMergeAndIsZero(t *testing.T) {
	var empty RefreshAction
	if !empty.IsZero() {
		t.Fatal("expected zero RefreshAction to report IsZero=true")
	}

	a := RefreshAction{ReloadBrowser: true, WaitForVite: true}
	b := RefreshAction{WaitForApp: true, TriggerRestart: true, RecompileGo: true}
	merged := a.Merge(b)

	if !merged.ReloadBrowser || !merged.WaitForVite || !merged.WaitForApp || !merged.TriggerRestart || !merged.RecompileGo {
		t.Fatalf("unexpected merged refresh action: %+v", merged)
	}
	if merged.IsZero() {
		t.Fatal("expected merged RefreshAction to be non-zero")
	}
}
