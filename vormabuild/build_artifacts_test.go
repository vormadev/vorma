package vormabuild

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/wave"
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

	mux.AddNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/has-loader",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)
	mux.AddNestedPatternWithoutHandler(app.LoadersRouter().NestedRouter, "/client-only")

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
	app.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
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

		if err := writePathsToDiskStageOne(l); err != nil {
			t.Fatalf("writePathsToDiskStageOne returned error: %v", err)
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

func TestPathsOutputPath_StageOneFile(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	got := pathsOutputPath(app, vormaruntime.VormaPathsStageOneJSONFileName)
	want := filepath.Join(
		app.Wave.StaticPrivateOutDir(),
		vormaruntime.VormaOutDirname,
		vormaruntime.VormaPathsStageOneJSONFileName,
	)
	if got != want {
		t.Fatalf("pathsOutputPath(stage-one) = %q, want %q", got, want)
	}
}

func TestStageOnePathsFile(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	var pathsFile *vormaruntime.PathsFile
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-stage-one")
		l.SetRouteManifestFile("manifest-stage-one.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/a": {
				OriginalPattern: "/a",
				SrcPath:         "frontend/src/routes/a.tsx",
				ExportKey:       "default",
			},
		})
		pathsFile = stageOnePathsFile(l, "manifest-stage-one.json")
	})

	if pathsFile == nil {
		t.Fatal("expected non-nil stage one paths file")
	}
	if pathsFile.Stage != "one" {
		t.Fatalf("stage = %q, want %q", pathsFile.Stage, "one")
	}
	if pathsFile.BuildID != "build-stage-one" {
		t.Fatalf("build ID = %q, want %q", pathsFile.BuildID, "build-stage-one")
	}
	if pathsFile.RouteManifestFile != "manifest-stage-one.json" {
		t.Fatalf("route manifest file = %q, want %q", pathsFile.RouteManifestFile, "manifest-stage-one.json")
	}
	if pathsFile.Paths["/a"] == nil {
		t.Fatal("expected /a path in stage one paths file")
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
	normalizedRoutePatterns := make(
		map[string]struct{},
		len(app.Config.ClientRouteDefinitionPatterns),
	)
	for _, configuredRoutePattern := range app.Config.ClientRouteDefinitionPatterns {
		normalizedRoutePatterns[normalizeFrameworkWatchPatternPath(configuredRoutePattern)] = struct{}{}
	}

	for _, pattern := range patterns {
		for normalizedRoutePattern := range normalizedRoutePatterns {
			if pattern.Pattern != normalizedRoutePattern {
				continue
			}
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
		templatePath := normalizeFrameworkWatchPatternPath(
			filepath.Join(app.Wave.PrivateStaticDir(), app.Config.HTMLTemplateLocation),
		)
		if pattern.Pattern == templatePath {
			foundTemplatePattern = true
			if pattern.RunOnChangeOnly {
				t.Fatalf("template pattern should not use RunOnChangeOnly")
			}
			if len(pattern.OnChangeHooks) != 1 || pattern.OnChangeHooks[0].Callback == nil {
				t.Fatalf("template pattern should include one callback hook")
			}
			if pattern.OnChangeHooks[0].Timing != wave.OnChangeStrategyPost {
				t.Fatalf(
					"template hook timing = %q, want %q",
					pattern.OnChangeHooks[0].Timing,
					wave.OnChangeStrategyPost,
				)
			}
		}
		if pattern.Pattern == "**/*.go" {
			foundGoPattern = true
			if len(pattern.OnChangeHooks) != 1 {
				t.Fatalf("go pattern should include one hook")
			}
			hook := pattern.OnChangeHooks[0]
			if hook.Cmd != "" {
				t.Fatalf("go hook cmd = %q, want empty", hook.Cmd)
			}
			if !hook.RunCombinedDevBuildHookCommands {
				t.Fatalf("go hook should set RunCombinedDevBuildHookCommands=true")
			}
			if hook.Timing != "concurrent" {
				t.Fatalf("go hook timing = %q, want %q", hook.Timing, "concurrent")
			}
		}
	}

	if !foundRoutesPattern {
		t.Fatalf("did not find configured routes watch patterns %#v", app.Config.ClientRouteDefinitionPatterns)
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
		IncludeDefaults:      &includeDefaults,
		MainBuildEntry:       "backend/cmd/build",
		UIVariant:            string(vormaruntime.UIVariantReact),
		HTMLTemplateLocation: "entry.go.html",
		ClientEntry:          "frontend/src/vorma.entry.tsx",
		ClientRouteDefinitionPatterns: []string{
			"frontend/src/**/*vorma.routes.ts",
		},
		TSGenOutDir:                "frontend/src/vorma.gen",
		BuildtimePublicURLFuncName: "waveBuildtimeURL",
	}
	fixture := newBuildTestFixture(t, &buildTestFixtureOptions{
		config: &cfg,
	})

	parsedCfg := injectDefaultWatchPatterns(fixture.app)
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

func TestRemoveMatchingTopLevelFiles_RemovesOnlyMatchingFiles(t *testing.T) {
	rootDir := t.TempDir()
	manifestFile := filepath.Join(rootDir, vormaruntime.VormaRouteManifestPrefix+"one.json")
	keepFile := filepath.Join(rootDir, "keep.txt")
	matchingDir := filepath.Join(rootDir, vormaruntime.VormaRouteManifestPrefix+"dir")

	mustWriteFile(t, manifestFile, []byte("{}"))
	mustWriteFile(t, keepFile, []byte("keep"))
	mustWriteFile(t, filepath.Join(matchingDir, "nested.txt"), []byte("nested"))

	if err := removeMatchingTopLevelFiles(rootDir, isGeneratedRouteManifestFilename); err != nil {
		t.Fatalf("removeMatchingTopLevelFiles returned error: %v", err)
	}

	if _, err := os.Stat(manifestFile); !os.IsNotExist(err) {
		t.Fatalf("expected matching top-level file removed, stat error: %v", err)
	}
	if _, err := os.Stat(keepFile); err != nil {
		t.Fatalf("expected non-matching file to remain, stat error: %v", err)
	}
	if _, err := os.Stat(matchingDir); err != nil {
		t.Fatalf("expected matching directory to remain untouched, stat error: %v", err)
	}
}

func TestRemoveMatchingEntriesRecursively_RemovesNestedMatchingFiles(t *testing.T) {
	rootDir := t.TempDir()
	nestedDir := filepath.Join(rootDir, "nested", "deeper")
	removeFile := filepath.Join(nestedDir, vormaruntime.VormaVitePrehashedFilePrefix+"bundle.js")
	keepFile := filepath.Join(nestedDir, "keep.txt")

	mustWriteFile(t, removeFile, []byte("remove"))
	mustWriteFile(t, keepFile, []byte("keep"))

	if err := removeMatchingEntriesRecursively(rootDir, shouldRemoveGeneratedStaticPublicFile); err != nil {
		t.Fatalf("removeMatchingEntriesRecursively returned error: %v", err)
	}

	if _, err := os.Stat(removeFile); !os.IsNotExist(err) {
		t.Fatalf("expected matching nested file removed, stat error: %v", err)
	}
	if _, err := os.Stat(keepFile); err != nil {
		t.Fatalf("expected non-matching nested file to remain, stat error: %v", err)
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
	if app.IsDevMode() {
		t.Fatal("expected production mode to set isDev to false")
	}
	if app.BuildID() != "existing-build-id" {
		t.Fatalf("build ID = %q, want %q", app.BuildID(), "existing-build-id")
	}
}

func TestInitializeBuildInnerState_DevMode(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := initializeBuildInnerState(app, &buildInnerOptions{isDev: true}); err != nil {
		t.Fatalf("initializeBuildInnerState returned error: %v", err)
	}
	if !app.IsDevMode() {
		t.Fatal("expected dev mode to set isDev to true")
	}
	if !strings.HasPrefix(app.BuildID(), "dev_") {
		t.Fatalf("build ID = %q, expected dev_ prefix", app.BuildID())
	}
	if len(app.BuildID()) <= len("dev_") {
		t.Fatalf("build ID = %q, expected non-empty suffix", app.BuildID())
	}
}

func TestParseAndSyncClientRoutes_MergesClientAndServerRoutes(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	t.Chdir(fixture.rootDir)

	mustWriteFile(t, "frontend/src/components/client.tsx", []byte("export const Client = () => null;"))
	mustWriteFile(t, "frontend/src/vorma.routes.ts", []byte(`
import { route } from "vorma/buildtime";
route("/client", import("./components/client.tsx"), "Client");
`))

	mux.AddNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/server-only",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)

	if err := parseAndSyncClientRoutes(app); err != nil {
		t.Fatalf("parseAndSyncClientRoutes returned error: %v", err)
	}

	paths := app.Paths()
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

func TestConfigureBuildEnvironment_WiresHooksAndDefaults(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	parsedCfg := configureBuildEnvironment(app)

	if _, hasVormaSchema := parsedCfg.FrameworkSchemaExtensions["Vorma"]; !hasVormaSchema {
		t.Fatal("expected configureBuildEnvironment to register Vorma schema extension")
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

	if parsedCfg.FrameworkRunBuildHook == nil {
		t.Fatal("expected configureBuildEnvironment to wire framework build hook runner")
	}

	if parsedCfg.FrameworkPrepareGoBuildOverlay == nil {
		t.Fatal("expected configureBuildEnvironment to wire framework go-build overlay preparation")
	}
}

func TestConfigureBuildEnvironment_PreservesExistingFrameworkBuildHooks(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	parsedCfg := app.Wave.BuildtimeParsedConfig()

	parsedCfg.FrameworkDevBuildHook = "go run ./custom/devhook"
	parsedCfg.FrameworkProdBuildHook = "go run ./custom/prodhook"

	configureBuildEnvironmentInConfig(app, parsedCfg)

	if parsedCfg.FrameworkDevBuildHook != "go run ./custom/devhook" {
		t.Fatalf("FrameworkDevBuildHook = %q, want preserved custom hook", parsedCfg.FrameworkDevBuildHook)
	}
	if parsedCfg.FrameworkProdBuildHook != "go run ./custom/prodhook" {
		t.Fatalf("FrameworkProdBuildHook = %q, want preserved custom hook", parsedCfg.FrameworkProdBuildHook)
	}
}

func TestConfigureBuildEnvironment_PreservesExistingFrameworkBuildHookRunner(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	parsedCfg := app.Wave.BuildtimeParsedConfig()

	frameworkBuildHookRunnerCalled := false
	parsedCfg.FrameworkRunBuildHook = func(context.Context, bool) error {
		frameworkBuildHookRunnerCalled = true
		return nil
	}

	configureBuildEnvironmentInConfig(app, parsedCfg)

	if parsedCfg.FrameworkRunBuildHook == nil {
		t.Fatal("expected existing framework build hook runner to remain configured")
	}
	if err := parsedCfg.FrameworkRunBuildHook(context.Background(), true); err != nil {
		t.Fatalf("existing framework build hook runner returned error: %v", err)
	}
	if !frameworkBuildHookRunnerCalled {
		t.Fatal("expected configureBuildEnvironment to preserve existing framework build hook runner")
	}
}

func TestConfigureBuildEnvironment_PreservesExistingFrameworkGoBuildOverlayPreparation(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	parsedCfg := app.Wave.BuildtimeParsedConfig()

	overlayPreparationCalled := false
	parsedCfg.FrameworkPrepareGoBuildOverlay = func() (*wave.GoBuildOverlay, error) {
		overlayPreparationCalled = true
		return nil, nil
	}

	configureBuildEnvironmentInConfig(app, parsedCfg)

	if parsedCfg.FrameworkPrepareGoBuildOverlay == nil {
		t.Fatal("expected existing framework go-build overlay preparation to remain configured")
	}
	if _, err := parsedCfg.FrameworkPrepareGoBuildOverlay(); err != nil {
		t.Fatalf("existing framework go-build overlay preparation returned error: %v", err)
	}
	if !overlayPreparationCalled {
		t.Fatal("expected configureBuildEnvironment to preserve existing framework overlay callback")
	}
}

func TestConfigureBuildEnvironment_FrameworkBuildHookRunner_ExecutesHookCommand(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	parsedCfg := app.Wave.BuildtimeParsedConfig()

	overlayCleanupCalled := false
	var capturedGoRunArgs []string
	frameworkBuildHookExecutor := newFrameworkBuildHookExecutor(
		frameworkBuildHookExecutionDependencies{
			prepareDiscoveredRouteRegistrarOverlayWithArtifactCache: func(
				*vormaruntime.Vorma,
				*discoveredRouteRegistrarArtifactCache,
			) (*discoveredRouteRegistrarOverlay, error) {
				return &discoveredRouteRegistrarOverlay{
					goOverlayConfigPath: "/tmp/vorma-test-overlay.json",
					cleanupTemporaryFiles: func() error {
						overlayCleanupCalled = true
						return nil
					},
				}, nil
			},
			runGoCommandWithContext: func(
				commandExecutionContext context.Context,
				goRunArgs []string,
			) error {
				if commandExecutionContext == nil {
					t.Fatal("expected non-nil command execution context")
				}
				capturedGoRunArgs = append([]string{}, goRunArgs...)
				return nil
			},
		},
	)

	configureBuildEnvironmentInConfigWithFrameworkBuildHookExecutor(
		app,
		parsedCfg,
		frameworkBuildHookExecutor,
	)
	if parsedCfg.FrameworkRunBuildHook == nil {
		t.Fatal("expected framework build hook runner to be configured")
	}

	if err := parsedCfg.FrameworkRunBuildHook(context.Background(), true); err != nil {
		t.Fatalf("framework build hook runner returned error: %v", err)
	}

	expectedGoRunArgs := []string{
		"run",
		"-overlay=/tmp/vorma-test-overlay.json",
		"./backend/cmd/build",
		"--dev",
		"--hook",
	}
	if len(capturedGoRunArgs) != len(expectedGoRunArgs) {
		t.Fatalf("go run args len = %d, want %d; got %#v", len(capturedGoRunArgs), len(expectedGoRunArgs), capturedGoRunArgs)
	}
	for argumentIndex, expectedArgument := range expectedGoRunArgs {
		if capturedGoRunArgs[argumentIndex] != expectedArgument {
			t.Fatalf(
				"go run arg[%d] = %q, want %q (args=%#v)",
				argumentIndex,
				capturedGoRunArgs[argumentIndex],
				expectedArgument,
				capturedGoRunArgs,
			)
		}
	}
	if !overlayCleanupCalled {
		t.Fatal("expected framework build hook runner to cleanup overlay")
	}
}

func TestConfigureBuildEnvironment_UsesLifecycleOwnedDiscoveredRegistrarCache(t *testing.T) {
	var capturedCaches []*discoveredRouteRegistrarArtifactCache
	frameworkBuildHookExecutor := newFrameworkBuildHookExecutor(
		frameworkBuildHookExecutionDependencies{
			prepareDiscoveredRouteRegistrarOverlayWithArtifactCache: func(
				_ *vormaruntime.Vorma,
				discoveredRegistrarArtifactsCache *discoveredRouteRegistrarArtifactCache,
			) (*discoveredRouteRegistrarOverlay, error) {
				if discoveredRegistrarArtifactsCache == nil {
					t.Fatal("expected non-nil discovered registrar artifacts cache")
				}
				capturedCaches = append(capturedCaches, discoveredRegistrarArtifactsCache)
				return nil, nil
			},
			runGoCommandWithContext: func(
				commandExecutionContext context.Context,
				_ []string,
			) error {
				if commandExecutionContext == nil {
					t.Fatal("expected non-nil command execution context")
				}
				return nil
			},
		},
	)

	fixtureOne := newBuildTestFixture(t, nil)
	parsedCfgOne := fixtureOne.app.Wave.BuildtimeParsedConfig()
	configureBuildEnvironmentInConfigWithFrameworkBuildHookExecutor(
		fixtureOne.app,
		parsedCfgOne,
		frameworkBuildHookExecutor,
	)

	if err := parsedCfgOne.FrameworkRunBuildHook(context.Background(), true); err != nil {
		t.Fatalf("first framework build hook run returned error: %v", err)
	}
	if _, err := parsedCfgOne.FrameworkPrepareGoBuildOverlay(); err != nil {
		t.Fatalf("first framework go build overlay preparation returned error: %v", err)
	}
	if err := parsedCfgOne.FrameworkRunBuildHook(context.Background(), false); err != nil {
		t.Fatalf("second framework build hook run returned error: %v", err)
	}

	if len(capturedCaches) != 3 {
		t.Fatalf("captured cache count = %d, want 3", len(capturedCaches))
	}
	firstLifecycleCache := capturedCaches[0]
	if capturedCaches[1] != firstLifecycleCache || capturedCaches[2] != firstLifecycleCache {
		t.Fatalf("expected same cache instance within lifecycle, got %#v", capturedCaches)
	}

	fixtureTwo := newBuildTestFixture(t, nil)
	parsedCfgTwo := fixtureTwo.app.Wave.BuildtimeParsedConfig()
	configureBuildEnvironmentInConfigWithFrameworkBuildHookExecutor(
		fixtureTwo.app,
		parsedCfgTwo,
		frameworkBuildHookExecutor,
	)

	if err := parsedCfgTwo.FrameworkRunBuildHook(context.Background(), true); err != nil {
		t.Fatalf("third framework build hook run returned error: %v", err)
	}
	if len(capturedCaches) != 4 {
		t.Fatalf("captured cache count = %d, want 4", len(capturedCaches))
	}
	secondLifecycleCache := capturedCaches[3]
	if secondLifecycleCache == firstLifecycleCache {
		t.Fatal("expected distinct lifecycle cache for second configured runtime")
	}
}

func TestBuildInner_DevBuildInnerFlow(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	t.Chdir(fixture.rootDir)
	writeBootstrapStyleRoutesFixtureFiles(t)

	if err := buildInner(app, &buildInnerOptions{isDev: true}); err != nil {
		t.Fatalf("buildInner returned error: %v", err)
	}

	if !app.IsDevMode() {
		t.Fatal("expected dev hook to set app to dev mode")
	}
	if !strings.HasPrefix(app.BuildID(), "dev_") {
		t.Fatalf("build ID = %q, expected dev_ prefix", app.BuildID())
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

	paths := app.Paths()
	if len(paths) != 3 {
		t.Fatalf("expected 3 routes from bootstrap-style defs, got %d", len(paths))
	}
}

func TestNewFastRebuildID(t *testing.T) {
	buildID, err := newFastRebuildID()
	if err != nil {
		t.Fatalf("newFastRebuildID returned error: %v", err)
	}
	if !strings.HasPrefix(buildID, "dev_fast_") {
		t.Fatalf("build ID = %q, expected dev_fast_ prefix", buildID)
	}
	if len(buildID) <= len("dev_fast_") {
		t.Fatalf("build ID = %q, expected non-empty suffix", buildID)
	}
}

func TestWriteAndSetRouteManifest(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	mux.AddNestedTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/server",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/server": {
				OriginalPattern: "/server",
				SrcPath:         "frontend/src/routes/server.tsx",
				ExportKey:       "default",
			},
		})

		if err := writeAndSetRouteManifest(l); err != nil {
			t.Fatalf("writeAndSetRouteManifest returned error: %v", err)
		}
	})

	manifestFile := app.RouteManifestFile()
	if manifestFile == "" {
		t.Fatal("expected route manifest filename to be set")
	}
	if !strings.HasPrefix(manifestFile, vormaruntime.VormaRouteManifestPrefix) {
		t.Fatalf("route manifest file = %q, expected prefix %q", manifestFile, vormaruntime.VormaRouteManifestPrefix)
	}
	if _, err := os.Stat(filepath.Join(fixture.publicDir, manifestFile)); err != nil {
		t.Fatalf("expected manifest file to exist on disk: %v", err)
	}
}

func TestRouteManifestFilename(t *testing.T) {
	manifestJSON := []byte(`{"a":1}`)
	filename := routeManifestFilename(manifestJSON)
	if !strings.HasPrefix(filename, vormaruntime.VormaRouteManifestPrefix) {
		t.Fatalf("filename = %q, expected prefix %q", filename, vormaruntime.VormaRouteManifestPrefix)
	}
	if filepath.Ext(filename) != ".json" {
		t.Fatalf("filename = %q, expected .json extension", filename)
	}

	filenameAgain := routeManifestFilename(manifestJSON)
	if filenameAgain != filename {
		t.Fatalf("routeManifestFilename should be deterministic: %q vs %q", filename, filenameAgain)
	}
}

func TestRouteManifestServerLoaderFlag(t *testing.T) {
	nestedRouter := mux.NewNestedRouter(nil)
	mux.AddNestedPatternWithoutHandler(nestedRouter, "/client-only")
	mux.AddNestedTaskHandler(
		nestedRouter,
		"/with-loader",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)

	if got := routeManifestServerLoaderFlag(nestedRouter, "/client-only"); got != 0 {
		t.Fatalf("routeManifestServerLoaderFlag(/client-only) = %d, want 0", got)
	}
	if got := routeManifestServerLoaderFlag(nestedRouter, "/with-loader"); got != 1 {
		t.Fatalf("routeManifestServerLoaderFlag(/with-loader) = %d, want 1", got)
	}
}
