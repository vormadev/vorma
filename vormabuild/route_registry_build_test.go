package vormabuild

import (
	"os"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestWriteRouteManifestToDisk_ReturnsWriteErrorWhenPublicOutDirIsFile(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}
	mustWriteFile(t, fixture.publicDir, []byte("not a directory"))

	_, err := writeRouteManifestToDisk(app, map[string]int{
		"/": 1,
	})
	if err == nil {
		t.Fatal("expected writeRouteManifestToDisk to return an error")
	}
	if !strings.Contains(err.Error(), "write route manifest") {
		t.Fatalf("error = %q, expected write-route-manifest context", err)
	}
}

func TestWriteRouteArtifacts_ReturnsWrappedErrorForEachArtifactStep(t *testing.T) {
	run := func(
		t *testing.T,
		name string,
		prepareFixtureForError func(*buildTestFixture),
		expectedErrContext string,
	) {
		t.Run(name, func(t *testing.T) {
			fixture := newBuildTestFixture(t, nil)
			app := fixture.app
			t.Chdir(fixture.rootDir)

			prepareFixtureForError(fixture)

			var gotErr error
			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetPaths(map[string]*vormaruntime.Path{
					"/": {
						OriginalPattern: "/",
						SrcPath:         "frontend/src/routes/home.tsx",
						ExportKey:       "default",
					},
				})
				gotErr = writeRouteArtifacts(l)
			})

			if gotErr == nil {
				t.Fatal("expected writeRouteArtifacts to return an error")
			}
			if !strings.Contains(gotErr.Error(), expectedErrContext) {
				t.Fatalf("error = %q, expected context %q", gotErr, expectedErrContext)
			}
		})
	}

	run(
		t,
		"manifest write failure",
		func(fixture *buildTestFixture) {
			if err := os.RemoveAll(fixture.publicDir); err != nil {
				t.Fatalf("remove public dir: %v", err)
			}
			mustWriteFile(t, fixture.publicDir, []byte("not a directory"))
		},
		"write route manifest",
	)

	run(
		t,
		"paths json write failure",
		func(fixture *buildTestFixture) {
			if err := os.RemoveAll(fixture.privateDir); err != nil {
				t.Fatalf("remove private dir: %v", err)
			}
			mustWriteFile(t, fixture.privateDir, []byte("not a directory"))
		},
		"write paths JSON",
	)

	run(
		t,
		"generated typescript write failure",
		func(fixture *buildTestFixture) {
			mustWriteFile(t, fixture.app.Config.TSGenOutDir, []byte("file blocks tsgen out dir"))
		},
		"write generated TypeScript",
	)
}
