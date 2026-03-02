package buildflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestPostViteProdBuild_WritesStageTwoPathsFile(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-before-post-vite")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/root.tsx",
				ExportKey:       "default",
			},
		})
	})

	manifest := viteutil.Manifest{
		"frontend/src/vorma.entry.tsx": {
			Src:     "frontend/src/vorma.entry.tsx",
			File:    testWaveOutAssetPath("entry.js"),
			IsEntry: true,
		},
		"frontend/src/routes/root.tsx": {
			Src:  "frontend/src/routes/root.tsx",
			File: testWaveOutAssetPath("root.js"),
		},
	}
	testkit.MustWriteJSONFile(t, app.Wave.ViteManifestLocation(), manifest)

	if err := postViteProdBuild(app); err != nil {
		t.Fatalf("postViteProdBuild returned error: %v", err)
	}

	stageTwoPath := filepath.Join(
		fixture.PrivateDir,
		runtimepaths.VormaInternalDirname,
		runtimepaths.VormaPathsStageTwoJSONFileName,
	)
	bytes, err := os.ReadFile(stageTwoPath)
	if err != nil {
		t.Fatalf("read stage two file: %v", err)
	}

	var parsed runtimepaths.PathsFile
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("unmarshal stage two file: %v", err)
	}
	if parsed.Stage != "two" {
		t.Fatalf("stage = %q, want %q", parsed.Stage, "two")
	}
	if parsed.ClientEntryOut != "entry.js" {
		t.Fatalf(
			"client entry out = %q, want %q",
			parsed.ClientEntryOut,
			"entry.js",
		)
	}
	if parsed.Paths["/"] == nil || parsed.Paths["/"].OutPath != "root.js" {
		t.Fatalf(
			"root route output = %#v, expected out path root.js",
			parsed.Paths["/"],
		)
	}
	if parsed.BuildID == "" {
		t.Fatal("expected non-empty stage-two build ID")
	}
	if app.BuildID() != parsed.BuildID {
		t.Fatalf("app build ID = %q, want %q", app.BuildID(), parsed.BuildID)
	}
}

func TestPostViteProdBuild_ReturnsErrorWhenTemplateMissing(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	app.Config.HTMLTemplateLocation = "missing-template.html"
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/root.tsx",
				ExportKey:       "default",
			},
		})
	})

	manifest := viteutil.Manifest{
		"frontend/src/vorma.entry.tsx": {
			Src:     "frontend/src/vorma.entry.tsx",
			File:    testWaveOutAssetPath("entry.js"),
			IsEntry: true,
		},
		"frontend/src/routes/root.tsx": {
			Src:  "frontend/src/routes/root.tsx",
			File: testWaveOutAssetPath("root.js"),
		},
	}
	testkit.MustWriteJSONFile(t, app.Wave.ViteManifestLocation(), manifest)

	err := postViteProdBuild(app)
	if err == nil {
		t.Fatal("expected postViteProdBuild to fail when template is missing")
	}
	if !strings.Contains(err.Error(), "convert paths to stage two") {
		t.Fatalf("error = %q, expected stage-two conversion context", err)
	}
	if !strings.Contains(err.Error(), "read HTML template") {
		t.Fatalf("error = %q, expected template read context", err)
	}
}
