package vormaruntime

import (
	"encoding/json"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
)

type staticAssetDirsForTests = struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

type testFixture struct {
	app        *Vorma
	rootDir    string
	staticDir  string
	privateDir string
}

type testFixtureOptions struct {
	stageOne             *runtimepaths.PathsFile
	stageTwo             *runtimepaths.PathsFile
	template             string
	publicPathPrefix     string
	enableCriticalCSS    bool
	enableNonCriticalCSS bool
	getDefaultHeadEls    GetDefaultHeadElsFunc
	getHeadDedupeKeys    GetHeadDedupeKeysFunc
	getRootTemplateData  GetRootTemplateDataFunc
	actionsRouterOpts    ActionsRouterOptions
	loadersRouterOpts    LoadersRouterOptions
	adHocTypes           []*tsgen.AdHocType
	extraTSCode          string
	configureVormaConfig func(*VormaConfig)
}

func newTestFixture(tb testing.TB, o testFixtureOptions) *testFixture {
	tb.Helper()

	tb.Setenv("__WAVE_MODE", "")
	tb.Setenv("PORT", "8080")
	tb.Setenv("__WAVE_PORT_HAS_BEEN_SET", "true")
	tb.Setenv("__VITE_PORT", "5173")

	rootDir := wavetest.NewWorkspaceTempDir(tb, "vormaruntime-fixture-")
	staticDir := filepath.Join(rootDir, "dist", "static")
	privateDir := filepath.Join(
		staticDir,
		waveartifacts.AssetsDirname,
		waveartifacts.PrivateDirname,
	)
	publicDir := filepath.Join(
		staticDir,
		waveartifacts.AssetsDirname,
		waveartifacts.PublicDirname,
	)
	internalDir := filepath.Join(staticDir, waveartifacts.InternalDirname)

	mustMkdirAll(tb, filepath.Join(privateDir, runtimepaths.VormaInternalDirname))
	mustMkdirAll(tb, publicDir)
	mustMkdirAll(tb, internalDir)

	tmpl := o.template
	if tmpl == "" {
		tmpl = "<!doctype html><html><head>{{.VormaHeadEls}}</head><body><div id=\"{{.VormaRootID}}\"></div>{{.VormaSSRScript}}{{.VormaBodyScripts}}</body></html>"
	}
	mustWriteFile(tb, filepath.Join(privateDir, "entry.go.html"), []byte(tmpl))

	stageOne := o.stageOne
	if stageOne == nil {
		stageOne = defaultPathsFile("build-stage1", nil)
	}
	stageTwo := o.stageTwo
	if stageTwo == nil {
		stageTwo = defaultPathsFile("build-stage2", nil)
	}
	mustWriteJSONFile(
		tb,
		filepath.Join(
			privateDir,
			runtimepaths.VormaInternalDirname,
			runtimepaths.VormaPathsStageOneJSONFileName,
		),
		stageOne,
	)
	mustWriteJSONFile(
		tb,
		filepath.Join(
			privateDir,
			runtimepaths.VormaInternalDirname,
			runtimepaths.VormaPathsStageTwoJSONFileName,
		),
		stageTwo,
	)

	coreCfg := waveconfig.CoreConfig{
		MainAppEntry:     "backend/cmd/serve",
		DistDir:          wavetest.MustCWDRelativePath(filepath.Join(rootDir, "dist")),
		PublicPathPrefix: o.publicPathPrefix,
	}
	wavetest.SetCoreStaticAssetDirectories(&coreCfg, publicDir, privateDir)
	if o.enableCriticalCSS || o.enableNonCriticalCSS {
		wavetest.SetCoreCSSEntryFiles(
			&coreCfg,
			"frontend/src/styles/main.critical.css",
			"frontend/src/styles/main.css",
		)
	}

	if o.enableCriticalCSS {
		mustWriteFile(
			tb,
			filepath.Join(internalDir, waveartifacts.CriticalCSSFileName),
			[]byte("body{color:black;}"),
		)
	}
	if o.enableNonCriticalCSS {
		mustWriteFile(
			tb,
			filepath.Join(internalDir, waveartifacts.NormalCSSRefFileName),
			[]byte(testWaveOutPrefixedFileName("styles.css")),
		)
	}

	rawCfg := struct {
		Core  waveconfig.CoreConfig `json:"Core"`
		Vorma VormaConfig           `json:"Vorma"`
	}{
		Core: coreCfg,
		Vorma: VormaConfig{
			MainBuildEntry:       "backend/cmd/build",
			UIVariant:            string(UIVariantReact),
			HTMLTemplateLocation: "entry.go.html",
			ClientEntry:          "frontend/src/vorma.entry.tsx",
			ClientRouteDefinitionPatterns: []string{
				"frontend/src/**/*vorma.routes.ts",
			},
			TSGenOutDir:                "frontend/src/vorma.gen",
			BuildtimePublicURLFuncName: "",
		},
	}
	if o.configureVormaConfig != nil {
		o.configureVormaConfig(&rawCfg.Vorma)
	}

	cfgJSON, err := json.Marshal(rawCfg)
	if err != nil {
		tb.Fatalf("marshal config: %v", err)
	}

	w := wave.New(wave.Config{
		WaveConfigJSON: cfgJSON,
		DistStaticFS:   os.DirFS(staticDir),
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	app := NewVormaApp(VormaAppConfig{
		Wave:                 w,
		DefaultHeadElsFunc:   o.getDefaultHeadEls,
		HeadDedupeKeysFunc:   o.getHeadDedupeKeys,
		RootTemplateDataFunc: o.getRootTemplateData,
		LoadersRouterOptions: o.loadersRouterOpts,
		ActionsRouterOptions: o.actionsRouterOpts,
		AdHocTypes:           o.adHocTypes,
		ExtraTSCode:          o.extraTSCode,
		Logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	app.MustInit()

	return &testFixture{
		app:        app,
		rootDir:    rootDir,
		staticDir:  staticDir,
		privateDir: privateDir,
	}
}

func defaultPathsFile(
	buildID string,
	paths map[string]*Path,
) *runtimepaths.PathsFile {
	if paths == nil {
		paths = map[string]*Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/root.tsx",
				OutPath:         testWaveOutPath("root.js"),
				ExportKey:       "default",
			},
		}
	}
	return &runtimepaths.PathsFile{
		Stage:             "stage-two",
		BuildID:           buildID,
		ClientEntrySrc:    "frontend/src/vorma.entry.tsx",
		ClientEntryOut:    testWaveOutPath("client-entry.js"),
		ClientEntryDeps:   []string{testWaveOutPath("client-shared.js")},
		Paths:             toRuntimePathsRouteMap(paths),
		RouteManifestFile: testWaveOutPath("route-manifest.js"),
		DepToCSSBundleMap: map[string][]string{
			testWaveOutPath("client-entry.js"):  {testWaveOutPath("client-entry.css")},
			testWaveOutPath("client-shared.js"): {testWaveOutPath("client-shared.css")},
		},
	}
}

func toRuntimePathsRouteMap(
	paths map[string]*Path,
) map[string]*runtimepaths.RoutePath {
	if paths == nil {
		return nil
	}

	runtimePaths := make(map[string]*runtimepaths.RoutePath, len(paths))
	for pattern, routePath := range paths {
		if routePath == nil {
			runtimePaths[pattern] = nil
			continue
		}
		runtimePaths[pattern] = &runtimepaths.RoutePath{
			OriginalPattern: routePath.OriginalPattern,
			SrcPath:         routePath.SrcPath,
			ExportKey:       routePath.ExportKey,
			ErrorExportKey:  routePath.ErrorExportKey,
			OutPath:         routePath.OutPath,
			Deps:            append([]string(nil), routePath.Deps...),
		}
	}
	return runtimePaths
}

func mustWriteJSONFile(tb testing.TB, file string, v any) {
	tb.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		tb.Fatalf("marshal %s: %v", file, err)
	}
	mustWriteFile(tb, file, data)
}

func mustWriteFile(tb testing.TB, file string, data []byte) {
	tb.Helper()
	if err := os.WriteFile(file, data, 0o644); err != nil {
		tb.Fatalf("write %s: %v", file, err)
	}
}

func mustMkdirAll(tb testing.TB, dir string) {
	tb.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		tb.Fatalf("mkdir %s: %v", dir, err)
	}
}

func routeDataSnapshotVersionForTest(app *Vorma) uint64 {
	if app == nil {
		return 0
	}

	var snapshotVersion uint64
	app.WithRLock(func(lv *ReadLockedVorma) {
		snapshotVersion = lv.v._routeDataSnapshotVersion
	})
	return snapshotVersion
}
