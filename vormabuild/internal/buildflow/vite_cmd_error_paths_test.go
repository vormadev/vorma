package buildflow

import (
	"errors"
	"github.com/vormadev/vorma/internal/testhelpers/waveoutputtest"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestPostViteProdBuild_ErrorWrapping(t *testing.T) {
	t.Run("wraps conversion error", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		err := postViteProdBuild(app)
		if err == nil {
			t.Fatal("expected postViteProdBuild to return conversion error")
		}
		if !strings.Contains(err.Error(), "convert paths to stage two") {
			t.Fatalf("error = %q, expected conversion context", err)
		}
		if !strings.Contains(err.Error(), "read vite manifest") {
			t.Fatalf("error = %q, expected manifest-read context", err)
		}
	})

	t.Run("wraps stage-two write error", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App
		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetBuildID("build-before-stage-two-write-failure")
			l.SetPaths(map[string]*vormaruntime.Path{
				"/": {
					OriginalPattern: "/",
					SrcPath:         "frontend/src/routes/root.tsx",
					ExportKey:       "default",
				},
			})
		})
		testkit.MustWriteJSONFile(
			t,
			app.Wave.ViteManifestLocation(),
			viteutil.Manifest{
				"frontend/src/vorma.entry.tsx": {
					Src:     "frontend/src/vorma.entry.tsx",
					File:    waveoutputtest.TestWaveOutputAssetPath("entry.js"),
					IsEntry: true,
				},
				"frontend/src/routes/root.tsx": {
					Src:  "frontend/src/routes/root.tsx",
					File: waveoutputtest.TestWaveOutputAssetPath("root.js"),
				},
			},
		)

		expectedErr := errors.New("write stage-two failed")
		err := postViteProdBuildWithDependencies(
			app,
			postViteProdBuildDependencies{
				writePathsToDiskStageTwo: func(
					*vormaruntime.Vorma,
					*runtimepaths.PathsFile,
				) error {
					return expectedErr
				},
			},
		)
		if err == nil {
			t.Fatal(
				"expected postViteProdBuild to return stage-two write error",
			)
		}
		if !strings.Contains(err.Error(), "write stage-two paths") {
			t.Fatalf("error = %q, expected stage-two write context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped stage-two write error", err)
		}
		if app.BuildID() != "build-before-stage-two-write-failure" {
			t.Fatalf(
				"app build ID = %q, want unchanged build ID %q",
				app.BuildID(),
				"build-before-stage-two-write-failure",
			)
		}
	})
}
