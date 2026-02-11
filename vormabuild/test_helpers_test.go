package vormabuild

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

type buildTestFixture struct {
	app        *vormaruntime.Vorma
	rootDir    string
	distDir    string
	staticDir  string
	privateDir string
	publicDir  string
}

type buildTestFixtureOptions struct {
	config              *vormaruntime.VormaConfig
	loadersRouterConfig vormaruntime.LoadersRouterOptions
	actionsRouterConfig vormaruntime.ActionsRouterOptions
}

func newBuildTestFixture(t *testing.T, options *buildTestFixtureOptions) *buildTestFixture {
	t.Helper()

	rootDir := t.TempDir()
	distDir := filepath.Join(rootDir, "dist")
	staticDir := filepath.Join(distDir, "static")
	privateDir := filepath.Join(staticDir, "assets", "private")
	publicDir := filepath.Join(staticDir, "assets", "public")

	mustMkdirAll(t, privateDir)
	mustMkdirAll(t, publicDir)
	mustMkdirAll(t, filepath.Join(privateDir, vormaruntime.VormaOutDirname))

	cfg := vormaruntime.VormaConfig{
		MainBuildEntry:             "backend/cmd/build",
		UIVariant:                  string(vormaruntime.UIVariants.React),
		HTMLTemplateLocation:       "entry.go.html",
		ClientEntry:                "frontend/src/vorma.entry.tsx",
		ClientRouteDefsFile:        "frontend/src/vorma.routes.ts",
		TSGenOutDir:                "frontend/src/vorma.gen",
		BuildtimePublicURLFuncName: "waveBuildtimeURL",
	}
	if options != nil && options.config != nil {
		cfg = *options.config
	}

	mustWriteFile(
		t,
		filepath.Join(privateDir, cfg.HTMLTemplateLocation),
		[]byte("<!doctype html><html><body></body></html>"),
	)

	rawConfig := struct {
		Core  wave.CoreConfig          `json:"Core"`
		Vorma vormaruntime.VormaConfig `json:"Vorma"`
	}{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      distDir,
			StaticAssetDirs: wave.StaticAssetDirs{
				Private: privateDir,
				Public:  publicDir,
			},
			PublicPathPrefix: "/",
		},
		Vorma: cfg,
	}

	cfgJSON, err := json.Marshal(rawConfig)
	if err != nil {
		t.Fatalf("marshal test config: %v", err)
	}

	w := wave.New(wave.Config{
		WaveConfigJSON: cfgJSON,
		DistStaticFS:   os.DirFS(staticDir),
		Logger:         testLogger(),
	})

	var loadersOpts vormaruntime.LoadersRouterOptions
	var actionsOpts vormaruntime.ActionsRouterOptions
	if options != nil {
		loadersOpts = options.loadersRouterConfig
		actionsOpts = options.actionsRouterConfig
	}

	app := vormaruntime.NewVormaApp(vormaruntime.VormaAppConfig{
		Wave:                 w,
		LoadersRouterOptions: loadersOpts,
		ActionsRouterOptions: actionsOpts,
		Logger:               testLogger(),
	})

	return &buildTestFixture{
		app:        app,
		rootDir:    rootDir,
		distDir:    distDir,
		staticDir:  staticDir,
		privateDir: privateDir,
		publicDir:  publicDir,
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustWriteJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	bytes, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	mustWriteFile(t, path, bytes)
}
