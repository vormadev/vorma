package vormabuild

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestCleanRouteManifestsOnly_RemovesOnlyManifestFiles(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	manifestFile := filepath.Join(fixture.publicDir, vormaruntime.VormaRouteManifestPrefix+"a.json")
	manifestFile2 := filepath.Join(fixture.publicDir, vormaruntime.VormaRouteManifestPrefix+"b.json")
	keepFile := filepath.Join(fixture.publicDir, "keep.txt")
	manifestDir := filepath.Join(fixture.publicDir, vormaruntime.VormaRouteManifestPrefix+"dir")
	mustWriteFile(t, manifestFile, []byte("a"))
	mustWriteFile(t, manifestFile2, []byte("b"))
	mustWriteFile(t, keepFile, []byte("keep"))
	mustWriteFile(t, filepath.Join(manifestDir, "nested.txt"), []byte("nested"))

	if err := cleanRouteManifestsOnly(app); err != nil {
		t.Fatalf("cleanRouteManifestsOnly returned error: %v", err)
	}

	if _, err := os.Stat(manifestFile); !os.IsNotExist(err) {
		t.Fatalf("expected manifest file to be removed, stat error: %v", err)
	}
	if _, err := os.Stat(manifestFile2); !os.IsNotExist(err) {
		t.Fatalf("expected second manifest file to be removed, stat error: %v", err)
	}
	if _, err := os.Stat(keepFile); err != nil {
		t.Fatalf("expected non-manifest file to remain, stat error: %v", err)
	}
	if _, err := os.Stat(manifestDir); err != nil {
		t.Fatalf("expected directory with prefix to remain, stat error: %v", err)
	}
}

func TestCleanStaticPublicOutDir_RemovesGeneratedPrefixedFiles(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	routeManifest := filepath.Join(fixture.publicDir, vormaruntime.VormaRouteManifestPrefix+"x.json")
	viteOutput := filepath.Join(fixture.publicDir, vormaruntime.VormaVitePrehashedFilePrefix+"bundle.js")
	keepRoot := filepath.Join(fixture.publicDir, "logo.svg")
	keepNested := filepath.Join(fixture.publicDir, "nested", "keep.txt")
	mustWriteFile(t, routeManifest, []byte("{}"))
	mustWriteFile(t, viteOutput, []byte("console.log('x')"))
	mustWriteFile(t, keepRoot, []byte("<svg/>"))
	mustWriteFile(t, keepNested, []byte("ok"))

	if err := cleanStaticPublicOutDir(app); err != nil {
		t.Fatalf("cleanStaticPublicOutDir returned error: %v", err)
	}

	if _, err := os.Stat(routeManifest); !os.IsNotExist(err) {
		t.Fatalf("expected route manifest to be removed, stat error: %v", err)
	}
	if _, err := os.Stat(viteOutput); !os.IsNotExist(err) {
		t.Fatalf("expected vite prefixed file to be removed, stat error: %v", err)
	}
	if _, err := os.Stat(keepRoot); err != nil {
		t.Fatalf("expected non-prefixed root file to remain, stat error: %v", err)
	}
	if _, err := os.Stat(keepNested); err != nil {
		t.Fatalf("expected nested non-prefixed file to remain, stat error: %v", err)
	}
}

func TestGenerateAndWriteRouteManifest(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/has-loader",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)
	mux.RegisterNestedPatternWithoutHandler(app.LoadersRouter().NestedRouter, "/client-only")

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/has-loader": {
				OriginalPattern: "/has-loader",
				SrcPath:         "frontend/src/routes/has-loader.tsx",
				ExportKey:       "default",
			},
			"/client-only": {
				OriginalPattern: "/client-only",
				SrcPath:         "frontend/src/routes/client-only.tsx",
				ExportKey:       "default",
			},
		})
	})

	var manifest map[string]int
	app.WithRLock(func(l *vormaruntime.LockedVorma) {
		manifest = generateRouteManifest(l, app.LoadersRouter().NestedRouter)
	})

	if got := manifest["/has-loader"]; got != 1 {
		t.Fatalf("manifest[/has-loader] = %d, want 1", got)
	}
	if got := manifest["/client-only"]; got != 0 {
		t.Fatalf("manifest[/client-only] = %d, want 0", got)
	}

	filename, err := writeRouteManifestToDisk(app, manifest)
	if err != nil {
		t.Fatalf("writeRouteManifestToDisk returned error: %v", err)
	}
	if !strings.HasPrefix(filename, vormaruntime.VormaRouteManifestPrefix) {
		t.Fatalf("filename = %q, expected prefix %q", filename, vormaruntime.VormaRouteManifestPrefix)
	}
	if filepath.Ext(filename) != ".json" {
		t.Fatalf("filename = %q, expected .json extension", filename)
	}

	writtenPath := filepath.Join(fixture.publicDir, filename)
	bytes, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("read written manifest: %v", err)
	}

	var decoded map[string]int
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("unmarshal manifest JSON: %v", err)
	}
	if decoded["/has-loader"] != 1 || decoded["/client-only"] != 0 {
		t.Fatalf("decoded manifest = %#v, expected loader/client flags", decoded)
	}
}

func TestWritePathsToDiskStageOne_WritesExpectedFields(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-123")
		l.SetRouteManifestFile("vorma_out_manifest_file.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/items/:id": {
				OriginalPattern: "/items/:id",
				SrcPath:         "frontend/src/routes/items.$id.tsx",
				ExportKey:       "default",
			},
		})

		if err := writePathsToDisk_StageOne(l); err != nil {
			t.Fatalf("writePathsToDisk_StageOne returned error: %v", err)
		}
	})

	stageOnePath := filepath.Join(
		fixture.privateDir,
		vormaruntime.VormaOutDirname,
		vormaruntime.VormaPathsStageOneJSONFileName,
	)
	bytes, err := os.ReadFile(stageOnePath)
	if err != nil {
		t.Fatalf("read stage one file: %v", err)
	}

	var parsed vormaruntime.PathsFile
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("unmarshal stage one file: %v", err)
	}
	if parsed.Stage != "one" {
		t.Fatalf("stage = %q, want %q", parsed.Stage, "one")
	}
	if parsed.BuildID != "build-123" {
		t.Fatalf("build ID = %q, want %q", parsed.BuildID, "build-123")
	}
	if parsed.RouteManifestFile != "vorma_out_manifest_file.json" {
		t.Fatalf("route manifest file = %q, want %q", parsed.RouteManifestFile, "vorma_out_manifest_file.json")
	}
	if parsed.Paths["/items/:id"] == nil {
		t.Fatalf("expected /items/:id path in stage one output")
	}
}

func TestGetDefaultWatchPatterns_IncludesRouteTemplateAndGoPatterns(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	patterns := getDefaultWatchPatterns(app)
	if len(patterns) != 3 {
		t.Fatalf("expected 3 default patterns, got %d", len(patterns))
	}

	var foundRoutesPattern bool
	var foundTemplatePattern bool
	var foundGoPattern bool

	for _, pattern := range patterns {
		if pattern.Pattern == app.Config.ClientRouteDefsFile {
			foundRoutesPattern = true
			if !pattern.RunOnChangeOnly {
				t.Fatalf("routes pattern should use RunOnChangeOnly")
			}
			if len(pattern.OnChangeHooks) != 1 || pattern.OnChangeHooks[0].Callback == nil {
				t.Fatalf("routes pattern should include one callback hook")
			}
			if !pattern.SkipRebuildingNotification {
				t.Fatalf("routes pattern should skip rebuilding notification")
			}
		}
		templatePath := filepath.Join(app.Wave.GetPrivateStaticDir(), app.Config.HTMLTemplateLocation)
		if pattern.Pattern == templatePath {
			foundTemplatePattern = true
			if !pattern.RunOnChangeOnly {
				t.Fatalf("template pattern should use RunOnChangeOnly")
			}
			if len(pattern.OnChangeHooks) != 1 || pattern.OnChangeHooks[0].Callback == nil {
				t.Fatalf("template pattern should include one callback hook")
			}
		}
		if pattern.Pattern == "**/*.go" {
			foundGoPattern = true
			if len(pattern.OnChangeHooks) != 1 {
				t.Fatalf("go pattern should include one hook")
			}
			hook := pattern.OnChangeHooks[0]
			if hook.Cmd != "DevBuildHook" {
				t.Fatalf("go hook cmd = %q, want %q", hook.Cmd, "DevBuildHook")
			}
			if hook.Timing != "concurrent" {
				t.Fatalf("go hook timing = %q, want %q", hook.Timing, "concurrent")
			}
		}
	}

	if !foundRoutesPattern {
		t.Fatalf("did not find routes watch pattern %q", app.Config.ClientRouteDefsFile)
	}
	if !foundTemplatePattern {
		t.Fatalf("did not find template watch pattern")
	}
	if !foundGoPattern {
		t.Fatalf("did not find Go watch pattern")
	}
}

func TestInjectDefaultWatchPatterns_SkipsWhenIncludeDefaultsDisabled(t *testing.T) {
	includeDefaults := false
	cfg := vormaruntime.VormaConfig{
		IncludeDefaults:            &includeDefaults,
		MainBuildEntry:             "backend/cmd/build",
		UIVariant:                  string(vormaruntime.UIVariants.React),
		HTMLTemplateLocation:       "entry.go.html",
		ClientEntry:                "frontend/src/vorma.entry.tsx",
		ClientRouteDefsFile:        "frontend/src/vorma.routes.ts",
		TSGenOutDir:                "frontend/src/vorma.gen",
		BuildtimePublicURLFuncName: "waveBuildtimeURL",
	}
	fixture := newBuildTestFixture(t, &buildTestFixtureOptions{
		config: &cfg,
	})

	injectDefaultWatchPatterns(fixture.app)

	parsedCfg := fixture.app.Wave.GetParsedConfig()
	if len(parsedCfg.FrameworkWatchPatterns) != 0 {
		t.Fatalf("expected no framework watch patterns, got %d", len(parsedCfg.FrameworkWatchPatterns))
	}
	if parsedCfg.FrameworkPublicFileMapOutDir != "" {
		t.Fatalf("expected FrameworkPublicFileMapOutDir to stay empty, got %q", parsedCfg.FrameworkPublicFileMapOutDir)
	}
}

func TestRebuildRoutesOnly_ReturnsErrorOutsideDevMode(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	err := rebuildRoutesOnly(fixture.app)
	if err == nil {
		t.Fatal("expected rebuildRoutesOnly to error outside dev mode")
	}
	if !strings.Contains(err.Error(), "only be called in dev mode") {
		t.Fatalf("error = %q, expected dev-mode message", err)
	}
}

func TestShouldRemoveGeneratedStaticPublicFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		{
			name:     "vite prehashed output",
			fileName: vormaruntime.VormaVitePrehashedFilePrefix + "bundle.js",
			want:     true,
		},
		{
			name:     "route manifest output",
			fileName: vormaruntime.VormaRouteManifestPrefix + "routes.json",
			want:     true,
		},
		{
			name:     "non-generated asset",
			fileName: "logo.svg",
			want:     false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := shouldRemoveGeneratedStaticPublicFile(testCase.fileName)
			if got != testCase.want {
				t.Fatalf("shouldRemoveGeneratedStaticPublicFile(%q) = %v, want %v", testCase.fileName, got, testCase.want)
			}
		})
	}
}

func TestIsGeneratedRouteManifestFilename(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		{
			name:     "hashed route manifest name",
			fileName: vormaruntime.VormaRouteManifestPrefix + "abc123.json",
			want:     true,
		},
		{
			name:     "exact raw prefix only",
			fileName: vormaruntime.VormaRouteManifestPrefix,
			want:     false,
		},
		{
			name:     "non-manifest name",
			fileName: "manifest.json",
			want:     false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := isGeneratedRouteManifestFilename(testCase.fileName)
			if got != testCase.want {
				t.Fatalf("isGeneratedRouteManifestFilename(%q) = %v, want %v", testCase.fileName, got, testCase.want)
			}
		})
	}
}

func TestInitializeBuildInnerState_ProdMode(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("existing-build-id")
	})
	app.SetIsDev(true)

	if err := initializeBuildInnerState(app, &buildInnerOptions{isDev: false}); err != nil {
		t.Fatalf("initializeBuildInnerState returned error: %v", err)
	}
	if app.GetIsDevMode() {
		t.Fatal("expected production mode to set isDev to false")
	}
	if app.GetBuildID() != "existing-build-id" {
		t.Fatalf("build ID = %q, want %q", app.GetBuildID(), "existing-build-id")
	}
}

func TestInitializeBuildInnerState_DevMode(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := initializeBuildInnerState(app, &buildInnerOptions{isDev: true}); err != nil {
		t.Fatalf("initializeBuildInnerState returned error: %v", err)
	}
	if !app.GetIsDevMode() {
		t.Fatal("expected dev mode to set isDev to true")
	}
	if !strings.HasPrefix(app.GetBuildID(), "dev_") {
		t.Fatalf("build ID = %q, expected dev_ prefix", app.GetBuildID())
	}
	if len(app.GetBuildID()) <= len("dev_") {
		t.Fatalf("build ID = %q, expected non-empty suffix", app.GetBuildID())
	}
}

func TestParseAndSyncClientRoutes_MergesClientAndServerRoutes(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	t.Chdir(fixture.rootDir)

	mustWriteFile(t, "frontend/src/components/client.tsx", []byte("export const Client = () => null;"))
	mustWriteFile(t, app.Config.ClientRouteDefsFile, []byte(`
import { route } from "vorma/buildtime";
route("/client", import("./components/client.tsx"), "Client");
`))

	mux.RegisterNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/server-only",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)

	if err := parseAndSyncClientRoutes(app); err != nil {
		t.Fatalf("parseAndSyncClientRoutes returned error: %v", err)
	}

	paths := app.GetPathsSnapshot()
	clientPath := paths["/client"]
	if clientPath == nil {
		t.Fatal("missing parsed client route /client")
	}
	if clientPath.SrcPath != "frontend/src/components/client.tsx" {
		t.Fatalf("client src path = %q, want %q", clientPath.SrcPath, "frontend/src/components/client.tsx")
	}
	if clientPath.ExportKey != "Client" {
		t.Fatalf("client export key = %q, want %q", clientPath.ExportKey, "Client")
	}

	serverOnlyPath := paths["/server-only"]
	if serverOnlyPath == nil {
		t.Fatal("missing merged server-only route")
	}
	if serverOnlyPath.SrcPath != "" {
		t.Fatalf("server-only src path = %q, want empty", serverOnlyPath.SrcPath)
	}
	if serverOnlyPath.ExportKey != "default" {
		t.Fatalf("server-only export key = %q, want %q", serverOnlyPath.ExportKey, "default")
	}
}

func TestWritePublicFileMapTypeScript_WritesTSAndJSON(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	t.Chdir(fixture.rootDir)
	mustWriteFile(t, filepath.Join(fixture.publicDir, "logo.svg"), []byte("<svg/>"))

	if err := writePublicFileMapTypeScript(app); err != nil {
		t.Fatalf("writePublicFileMapTypeScript returned error: %v", err)
	}

	fileMapTSPath := filepath.Join(app.Config.TSGenOutDir, "filemap.ts")
	fileMapJSONPath := filepath.Join(app.Config.TSGenOutDir, "filemap.json")

	fileMapTSBytes, err := os.ReadFile(fileMapTSPath)
	if err != nil {
		t.Fatalf("read filemap.ts: %v", err)
	}
	if len(fileMapTSBytes) == 0 {
		t.Fatal("expected filemap.ts to be non-empty")
	}

	fileMapJSONBytes, err := os.ReadFile(fileMapJSONPath)
	if err != nil {
		t.Fatalf("read filemap.json: %v", err)
	}
	if len(fileMapJSONBytes) == 0 {
		t.Fatal("expected filemap.json to be non-empty")
	}
}

func TestConfigureBuildEnvironment_WiresSchemaHooksAndDefaults(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	configureBuildEnvironment(app)

	parsedCfg := app.Wave.GetParsedConfig()
	if _, ok := parsedCfg.FrameworkSchemaExtensions["Vorma"]; !ok {
		t.Fatal("expected Vorma schema extension to be registered")
	}

	expectedDevHook := fmt.Sprintf("go run ./%s --dev --hook", app.Config.MainBuildEntry)
	if parsedCfg.FrameworkDevBuildHook != expectedDevHook {
		t.Fatalf("FrameworkDevBuildHook = %q, want %q", parsedCfg.FrameworkDevBuildHook, expectedDevHook)
	}

	expectedProdHook := fmt.Sprintf("go run ./%s --hook", app.Config.MainBuildEntry)
	if parsedCfg.FrameworkProdBuildHook != expectedProdHook {
		t.Fatalf("FrameworkProdBuildHook = %q, want %q", parsedCfg.FrameworkProdBuildHook, expectedProdHook)
	}

	if len(parsedCfg.FrameworkWatchPatterns) != 3 {
		t.Fatalf("expected 3 default framework watch patterns, got %d", len(parsedCfg.FrameworkWatchPatterns))
	}

	if parsedCfg.FrameworkPublicFileMapOutDir != app.Config.TSGenOutDir {
		t.Fatalf("FrameworkPublicFileMapOutDir = %q, want %q", parsedCfg.FrameworkPublicFileMapOutDir, app.Config.TSGenOutDir)
	}
}

func TestRunBuildHook_DevBuildInnerFlow(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	t.Chdir(fixture.rootDir)
	writeBootstrapStyleRoutesFixtureFiles(t)

	if err := runBuildHook(app, true); err != nil {
		t.Fatalf("runBuildHook returned error: %v", err)
	}

	if !app.GetIsDevMode() {
		t.Fatal("expected dev hook to set app to dev mode")
	}
	if !strings.HasPrefix(app.GetBuildID(), "dev_") {
		t.Fatalf("build ID = %q, expected dev_ prefix", app.GetBuildID())
	}

	stageOnePath := filepath.Join(
		fixture.privateDir,
		vormaruntime.VormaOutDirname,
		vormaruntime.VormaPathsStageOneJSONFileName,
	)
	if _, err := os.Stat(stageOnePath); err != nil {
		t.Fatalf("expected stage one paths file to exist: %v", err)
	}

	generatedTSPath := filepath.Join(app.Config.TSGenOutDir, "index.ts")
	if _, err := os.Stat(generatedTSPath); err != nil {
		t.Fatalf("expected generated TS output to exist: %v", err)
	}

	paths := app.GetPathsSnapshot()
	if len(paths) != 3 {
		t.Fatalf("expected 3 routes from bootstrap-style defs, got %d", len(paths))
	}
}
