package vormaruntime

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
)

type testFixture struct {
	app        *Vorma
	rootDir    string
	staticDir  string
	privateDir string
}

type testFixtureOptions struct {
	stageOne             *PathsFile
	stageTwo             *PathsFile
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

	tb.Setenv("WAVE_MODE", "")
	tb.Setenv("PORT", "8080")
	tb.Setenv("WAVE_PORT_HAS_BEEN_SET", "true")

	rootDir := tb.TempDir()
	staticDir := filepath.Join(rootDir, "dist", "static")
	privateDir := filepath.Join(staticDir, "assets", "private")
	publicDir := filepath.Join(staticDir, "assets", "public")
	internalDir := filepath.Join(staticDir, "internal")

	mustMkdirAll(tb, filepath.Join(privateDir, VormaOutDirname))
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
	mustWriteJSONFile(tb, filepath.Join(privateDir, VormaOutDirname, VormaPathsStageOneJSONFileName), stageOne)
	mustWriteJSONFile(tb, filepath.Join(privateDir, VormaOutDirname, VormaPathsStageTwoJSONFileName), stageTwo)

	coreCfg := wave.CoreConfig{
		MainAppEntry: "backend/cmd/serve",
		DistDir:      filepath.Join(rootDir, "dist"),
		StaticAssetDirs: wave.StaticAssetDirs{
			Private: privateDir,
			Public:  publicDir,
		},
		PublicPathPrefix: o.publicPathPrefix,
	}
	if o.enableCriticalCSS || o.enableNonCriticalCSS {
		coreCfg.CSSEntryFiles = wave.CSSEntryFiles{
			Critical:    "frontend/src/styles/main.critical.css",
			NonCritical: "frontend/src/styles/main.css",
		}
	}

	if o.enableCriticalCSS {
		mustWriteFile(tb, filepath.Join(internalDir, "critical.css"), []byte("body{color:black;}"))
	}
	if o.enableNonCriticalCSS {
		mustWriteFile(tb, filepath.Join(internalDir, "normal_css_file_ref.txt"), []byte("vorma_out_styles.css"))
	}

	rawCfg := struct {
		Core  wave.CoreConfig `json:"Core"`
		Vorma VormaConfig     `json:"Vorma"`
	}{
		Core: coreCfg,
		Vorma: VormaConfig{
			MainBuildEntry:       "backend/cmd/build",
			UIVariant:            string(UIVariants.React),
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
		ConfigSource: wave.NewStaticConfigSource(cfgJSON),
		DistStaticFS: os.DirFS(staticDir),
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	app := NewVormaApp(VormaAppConfig{
		Wave:                 w,
		GetDefaultHeadEls:    o.getDefaultHeadEls,
		GetHeadDedupeKeys:    o.getHeadDedupeKeys,
		GetRootTemplateData:  o.getRootTemplateData,
		LoadersRouterOptions: o.loadersRouterOpts,
		ActionsRouterOptions: o.actionsRouterOpts,
		AdHocTypes:           o.adHocTypes,
		ExtraTSCode:          o.extraTSCode,
		Logger:               slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	app.Init()

	return &testFixture{
		app:        app,
		rootDir:    rootDir,
		staticDir:  staticDir,
		privateDir: privateDir,
	}
}

func defaultPathsFile(buildID string, paths map[string]*Path) *PathsFile {
	if paths == nil {
		paths = map[string]*Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/root.tsx",
				OutPath:         "vorma_out/root.js",
				ExportKey:       "default",
			},
		}
	}
	return &PathsFile{
		Stage:             "stage-two",
		BuildID:           buildID,
		ClientEntrySrc:    "frontend/src/vorma.entry.tsx",
		ClientEntryOut:    "vorma_out/client-entry.js",
		ClientEntryDeps:   []string{"vorma_out/client-shared.js"},
		Paths:             paths,
		RouteManifestFile: "vorma_out/route-manifest.js",
		DepToCSSBundleMap: map[string][]string{
			"vorma_out/client-entry.js":  {"vorma_out/client-entry.css"},
			"vorma_out/client-shared.js": {"vorma_out/client-shared.css"},
		},
	}
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
