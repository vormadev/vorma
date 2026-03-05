package buildinner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
	"github.com/vormadev/vorma/wave/waveartifacts"
)

func TestInitializeBuildInnerState_ProdMode(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

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
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

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
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	t.Chdir(fixture.RootDir)

	testkit.MustWriteFile(
		t,
		"frontend/src/components/client.tsx",
		[]byte("export const Client = () => null;"),
	)
	testkit.MustWriteFile(t, "frontend/src/vorma.routes.ts", []byte(`
import { route } from "vorma/buildtime";
route("/client", import("./components/client.tsx"), "Client");
`))

	nestedmux.AddTaskHandler(
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
		t.Fatalf(
			"client src path = %q, want %q",
			clientPath.SrcPath,
			"frontend/src/components/client.tsx",
		)
	}
	if clientPath.ExportKey != "Client" {
		t.Fatalf(
			"client export key = %q, want %q",
			clientPath.ExportKey,
			"Client",
		)
	}

	serverOnlyPath := paths["/server-only"]
	if serverOnlyPath == nil {
		t.Fatal("missing merged server-only route")
	}
	if serverOnlyPath.SrcPath != "" {
		t.Fatalf(
			"server-only src path = %q, want empty",
			serverOnlyPath.SrcPath,
		)
	}
	if serverOnlyPath.ExportKey != "default" {
		t.Fatalf(
			"server-only export key = %q, want %q",
			serverOnlyPath.ExportKey,
			"default",
		)
	}
}

func TestWritePublicFileMapTypeScript_WritesTypeScriptFromCanonicalJSON(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	t.Chdir(fixture.RootDir)
	testkit.MustWriteFile(
		t,
		filepath.Join(fixture.PublicDir, "logo.svg"),
		[]byte("<svg/>"),
	)

	if err := writePublicFileMapTypeScript(app); err != nil {
		t.Fatalf("writePublicFileMapTypeScript returned error: %v", err)
	}

	fileMapTSPath := filepath.Join(app.Config.TSGenOutDir(), "filemap.ts")

	fileMapTSBytes, err := os.ReadFile(fileMapTSPath)
	if err != nil {
		t.Fatalf("read filemap.ts: %v", err)
	}
	if len(fileMapTSBytes) == 0 {
		t.Fatal("expected filemap.ts to be non-empty")
	}
	if _, err := os.Stat(
		filepath.Join(app.Config.TSGenOutDir(), waveartifacts.PublicFileMapJSONName),
	); !os.IsNotExist(err) {
		t.Fatalf(
			"expected %s to be absent, stat err=%v",
			waveartifacts.PublicFileMapJSONName,
			err,
		)
	}
}

func TestBuildInner_DevBuildInnerFlow(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	t.Chdir(fixture.RootDir)
	testkit.WriteBootstrapStyleRoutesFixtureFiles(t)

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
		fixture.PrivateDir,
		runtimepaths.VormaInternalDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)
	if _, err := os.Stat(stageOnePath); err != nil {
		t.Fatalf("expected stage one paths file to exist: %v", err)
	}

	generatedTSPath := filepath.Join(app.Config.TSGenOutDir(), "index.ts")
	if _, err := os.Stat(generatedTSPath); err != nil {
		t.Fatalf("expected generated TS output to exist: %v", err)
	}

	paths := app.Paths()
	if len(paths) != 3 {
		t.Fatalf(
			"expected 3 routes from bootstrap-style defs, got %d",
			len(paths),
		)
	}
}
