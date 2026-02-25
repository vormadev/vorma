package vormagogen

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/wave"
)

type discoveredRegistrationRuntimeFixture struct {
	app *vorma.Vorma
}

type discoveredRegistrationStaticAssetDirs = struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

func newDiscoveredRegistrationRuntimeFixture(
	t *testing.T,
) *discoveredRegistrationRuntimeFixture {
	t.Helper()

	rootDir := t.TempDir()
	distDir := filepath.Join(rootDir, "dist")
	staticDir := filepath.Join(distDir, "static")
	privateDir := filepath.Join(staticDir, "assets", "private")
	publicDir := filepath.Join(staticDir, "assets", "public")
	if err := os.MkdirAll(privateDir, 0o755); err != nil {
		t.Fatalf("create private asset dir: %v", err)
	}
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatalf("create public asset dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(privateDir, "entry.go.html"),
		[]byte("<!doctype html><html><body></body></html>"),
		0o644,
	); err != nil {
		t.Fatalf("write test HTML template: %v", err)
	}

	rawConfig := struct {
		Core  wave.CoreConfig `json:"Core"`
		Vorma struct {
			MainBuildEntry                string   `json:"MainBuildEntry"`
			UIVariant                     string   `json:"UIVariant"`
			HTMLTemplateLocation          string   `json:"HTMLTemplateLocation"`
			ClientEntry                   string   `json:"ClientEntry"`
			ClientRouteDefinitionPatterns []string `json:"ClientRouteDefinitionPatterns"`
			TSGenOutDir                   string   `json:"TSGenOutDir"`
			BuildtimePublicURLFuncName    string   `json:"BuildtimePublicURLFuncName"`
		} `json:"Vorma"`
	}{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      distDir,
			StaticAssetDirs: discoveredRegistrationStaticAssetDirs{
				Private: privateDir,
				Public:  publicDir,
			},
			PublicPathPrefix: "/",
		},
	}
	rawConfig.Vorma.MainBuildEntry = "backend/cmd/build"
	rawConfig.Vorma.UIVariant = "react"
	rawConfig.Vorma.HTMLTemplateLocation = "entry.go.html"
	rawConfig.Vorma.ClientEntry = "frontend/src/vorma.entry.tsx"
	rawConfig.Vorma.ClientRouteDefinitionPatterns = []string{
		"frontend/src/**/*vorma.routes.ts",
	}
	rawConfig.Vorma.TSGenOutDir = "frontend/src/vorma.gen"
	rawConfig.Vorma.BuildtimePublicURLFuncName = "waveBuildtimeURL"

	rawConfigBytes, err := json.Marshal(rawConfig)
	if err != nil {
		t.Fatalf("marshal test config: %v", err)
	}

	w := wave.New(wave.Config{
		WaveConfigJSON: rawConfigBytes,
		DistStaticFS:   os.DirFS(staticDir),
		Logger:         discoveredRegistrationTestLogger(),
	})
	return &discoveredRegistrationRuntimeFixture{
		app: vorma.NewVormaApp(vorma.VormaAppConfig{
			Wave:   w,
			Logger: discoveredRegistrationTestLogger(),
		}),
	}
}

func discoveredRegistrationTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDiscoveredRegistrationRuntimeHelpers(t *testing.T) {
	fixture := newDiscoveredRegistrationRuntimeFixture(t)
	app := fixture.app

	if got := len(app.RegisteredActionRoutes()); got != 0 {
		t.Fatalf("expected no pre-registered actions, got %d", got)
	}

	loaderTask := vorma.DefineLoaderForRegistration(
		app,
		"/no-op",
		func(rd *vorma.LoaderReqData) (string, error) {
			return "ok", nil
		},
		func(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
			return rd
		},
	)
	if loaderTask == nil {
		t.Fatal("expected DefineLoaderForRegistration to return loader task")
	}
	if app.HasRegisteredLoaderTask("/no-op") {
		t.Fatal(
			"expected DefineLoaderForRegistration to avoid direct router registration",
		)
	}

	_ = RegisterLoaderDiscoveredByBuild(
		app,
		"/registered",
		func(rd *vorma.LoaderReqData) (string, error) {
			return "ok", nil
		},
		func(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
			return rd
		},
	)
	if !app.HasRegisteredLoaderTask("/registered") {
		t.Fatal(
			"expected RegisterLoaderDiscoveredByBuild to register nested handler",
		)
	}

	actionTask := vorma.DefineActionForRegistration(
		app,
		"POST",
		"/no-op-action",
		func(rd *vorma.ActionReqData[vorma.None]) (string, error) {
			return "ok", nil
		},
		func(rd *vorma.ActionReqData[vorma.None]) *vorma.ActionReqData[vorma.None] {
			return rd
		},
	)
	if actionTask == nil {
		t.Fatal("expected DefineActionForRegistration to return action task")
	}
	if got := len(app.RegisteredActionRoutes()); got != 0 {
		t.Fatalf(
			"expected DefineActionForRegistration to avoid direct action registration, got %d routes",
			got,
		)
	}

	_ = RegisterActionDiscoveredByBuild(
		app,
		"POST",
		"/registered-action",
		func(rd *vorma.ActionReqData[vorma.None]) (string, error) {
			return "ok", nil
		},
		func(rd *vorma.ActionReqData[vorma.None]) *vorma.ActionReqData[vorma.None] {
			return rd
		},
	)
	registeredActionRoutes := app.RegisteredActionRoutes()
	if got := len(registeredActionRoutes); got != 1 {
		t.Fatalf("expected one discovered action registration, got %d", got)
	}
	if registeredActionRoutes[0].Method != "POST" ||
		registeredActionRoutes[0].Pattern != "/registered-action" {
		t.Fatalf(
			"unexpected registered action route: method=%q pattern=%q",
			registeredActionRoutes[0].Method,
			registeredActionRoutes[0].Pattern,
		)
	}
}
