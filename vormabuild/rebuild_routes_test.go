package vormabuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestCleanRouteManifestsOnly_IgnoresMissingPublicOutDir(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}

	if err := cleanRouteManifestsOnly(app); err != nil {
		t.Fatalf("cleanRouteManifestsOnly should ignore missing dir, got: %v", err)
	}
}

func TestSyncRoutesAndWriteFastRebuildArtifacts_Success(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

	clientPaths := map[string]*vormaruntime.Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.tsx",
			ExportKey:       "default",
		},
	}

	if err := syncRoutesAndWriteFastRebuildArtifacts(app, clientPaths, "dev_fast_test"); err != nil {
		t.Fatalf("syncRoutesAndWriteFastRebuildArtifacts returned error: %v", err)
	}

	if got := app.GetBuildID(); got != "dev_fast_test" {
		t.Fatalf("build ID = %q, want %q", got, "dev_fast_test")
	}
	if app.GetPathsSnapshot()["/products/:id"] == nil {
		t.Fatal("expected synced route to be present in app paths")
	}

	stageOnePath := filepath.Join(
		fixture.privateDir,
		vormaruntime.VormaOutDirname,
		vormaruntime.VormaPathsStageOneJSONFileName,
	)
	if _, err := os.Stat(stageOnePath); err != nil {
		t.Fatalf("expected stage one paths file to exist: %v", err)
	}

	manifestFile := app.GetRouteManifestFile()
	if manifestFile == "" {
		t.Fatal("expected route manifest file name to be set")
	}
	if _, err := os.Stat(filepath.Join(fixture.publicDir, manifestFile)); err != nil {
		t.Fatalf("expected route manifest file to exist: %v", err)
	}

	generatedTSPath := filepath.Join(app.Config.TSGenOutDir, "index.ts")
	if _, err := os.Stat(generatedTSPath); err != nil {
		t.Fatalf("expected generated TypeScript to exist: %v", err)
	}
}

func TestSyncRoutesAndWriteFastRebuildArtifacts_ReturnsCleanError(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}
	mustWriteFile(t, fixture.publicDir, []byte("not a directory"))

	err := syncRoutesAndWriteFastRebuildArtifacts(
		app,
		map[string]*vormaruntime.Path{},
		"dev_fast_test",
	)
	if err == nil {
		t.Fatal("expected syncRoutesAndWriteFastRebuildArtifacts to return error")
	}
	if !strings.Contains(err.Error(), "clean route manifests") {
		t.Fatalf("error = %q, expected clean-route-manifests context", err)
	}
}
