package buildlifecycle

import (
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestCaptureRouteBuildRuntimeState(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	var runtimeStateSnapshot RouteBuildRuntimeStateSnapshot
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-before")
		l.SetRouteManifestFile("route-manifest-before.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/before": {
				OriginalPattern: "/before",
				SrcPath:         "frontend/src/routes/before.tsx",
				ExportKey:       "default",
				Deps:            []string{testWaveOutPath("chunk-before.js")},
			},
		})

		runtimeStateSnapshot = CaptureRouteBuildRuntimeState(l)

		l.SetBuildID("build-after")
		l.SetRouteManifestFile("route-manifest-after.json")
		paths := l.Paths()
		paths["/before"].SrcPath = "frontend/src/routes/after.tsx"
		paths["/before"].Deps[0] = testWaveOutPath("chunk-after.js")
	})

	if runtimeStateSnapshot.BuildID != "build-before" {
		t.Fatalf(
			"snapshot build ID = %q, want %q",
			runtimeStateSnapshot.BuildID,
			"build-before",
		)
	}
	if runtimeStateSnapshot.RouteManifestFile != "route-manifest-before.json" {
		t.Fatalf(
			"snapshot route manifest = %q, want %q",
			runtimeStateSnapshot.RouteManifestFile,
			"route-manifest-before.json",
		)
	}

	beforePath := runtimeStateSnapshot.Paths["/before"]
	if beforePath == nil {
		t.Fatalf(
			"snapshot paths = %#v, expected /before path",
			runtimeStateSnapshot.Paths,
		)
	}
	if beforePath.SrcPath != "frontend/src/routes/before.tsx" {
		t.Fatalf(
			"snapshot /before src path = %q, want %q",
			beforePath.SrcPath,
			"frontend/src/routes/before.tsx",
		)
	}
	if len(beforePath.Deps) != 1 ||
		beforePath.Deps[0] != testWaveOutPath("chunk-before.js") {
		t.Fatalf(
			"snapshot /before deps = %#v, want %#v",
			beforePath.Deps,
			[]string{testWaveOutPath("chunk-before.js")},
		)
	}
}

func TestRestoreRouteBuildRuntimeState(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	var runtimeStateSnapshot RouteBuildRuntimeStateSnapshot
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.Routes().ReplaceParsedPathsForInit(map[string]*vormaruntime.Path{
			"/before": {
				OriginalPattern: "/before",
				SrcPath:         "frontend/src/routes/before.tsx",
				ExportKey:       "default",
			},
		}, true)
		l.SetBuildID("build-before")
		l.SetRouteManifestFile("route-manifest-before.json")

		runtimeStateSnapshot = CaptureRouteBuildRuntimeState(l)

		l.Routes().ReplaceParsedPathsForInit(map[string]*vormaruntime.Path{
			"/after": {
				OriginalPattern: "/after",
				SrcPath:         "frontend/src/routes/after.tsx",
				ExportKey:       "default",
			},
		}, true)
		l.SetBuildID("build-after")
		l.SetRouteManifestFile("route-manifest-after.json")

		RestoreRouteBuildRuntimeState(l, runtimeStateSnapshot)
	})

	if app.BuildID() != "build-before" {
		t.Fatalf(
			"build ID after restore = %q, want %q",
			app.BuildID(),
			"build-before",
		)
	}
	if app.RouteManifestFile() != "route-manifest-before.json" {
		t.Fatalf(
			"route manifest after restore = %q, want %q",
			app.RouteManifestFile(),
			"route-manifest-before.json",
		)
	}

	restoredPaths := app.Paths()
	if len(restoredPaths) != 1 {
		t.Fatalf(
			"restored paths length = %d, want 1 (%#v)",
			len(restoredPaths),
			restoredPaths,
		)
	}
	if restoredPaths["/before"] == nil {
		t.Fatalf("expected /before path after restore, got %#v", restoredPaths)
	}
	if restoredPaths["/after"] != nil {
		t.Fatalf(
			"did not expect /after path after restore, got %#v",
			restoredPaths["/after"],
		)
	}

	if !app.LoadersRouter().NestedRouter.IsRegistered("/before") {
		t.Fatal(
			"expected /before to be registered in nested router after restore",
		)
	}
	if app.LoadersRouter().NestedRouter.IsRegistered("/after") {
		t.Fatal(
			"did not expect /after to remain registered in nested router after restore",
		)
	}
}

func TestCloneRouteBuildRuntimePath(t *testing.T) {
	if got := CloneRouteBuildRuntimePath(nil); got != nil {
		t.Fatalf(
			"CloneRouteBuildRuntimePath(nil) = %#v, want nil",
			got,
		)
	}
}

func TestBuildRuntimeState(t *testing.T) {
	t.Run("captures and restores full build runtime state", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		var runtimeStateSnapshot BuildRuntimeStateSnapshot
		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetIsDev(false)
			l.Routes().ReplaceParsedPathsForInit(map[string]*vormaruntime.Path{
				"/before": {
					OriginalPattern: "/before",
					SrcPath:         "frontend/src/routes/before.tsx",
					ExportKey:       "default",
				},
			}, true)
			l.SetBuildID("build-before")
			l.SetRouteManifestFile("route-manifest-before.json")

			runtimeStateSnapshot = CaptureBuildRuntimeState(l)

			l.SetIsDev(true)
			l.Routes().ReplaceParsedPathsForInit(map[string]*vormaruntime.Path{
				"/after": {
					OriginalPattern: "/after",
					SrcPath:         "frontend/src/routes/after.tsx",
					ExportKey:       "default",
				},
			}, true)
			l.SetBuildID("build-after")
			l.SetRouteManifestFile("route-manifest-after.json")

			RestoreBuildRuntimeState(l, runtimeStateSnapshot)

			if !BuildRuntimeStateSnapshotMatches(
				l,
				runtimeStateSnapshot,
			) {
				t.Fatalf(
					"restored runtime state does not match captured snapshot: %#v",
					runtimeStateSnapshot,
				)
			}
		})

		if app.IsDevMode() {
			t.Fatal("expected isDev=false after restore")
		}
		if got := app.BuildID(); got != "build-before" {
			t.Fatalf(
				"build ID after restore = %q, want %q",
				got,
				"build-before",
			)
		}
		if got := app.RouteManifestFile(); got != "route-manifest-before.json" {
			t.Fatalf(
				"route manifest after restore = %q, want %q",
				got,
				"route-manifest-before.json",
			)
		}

		restoredPaths := app.Paths()
		if len(restoredPaths) != 1 {
			t.Fatalf(
				"restored paths length = %d, want 1 (%#v)",
				len(restoredPaths),
				restoredPaths,
			)
		}
		if restoredPaths["/before"] == nil {
			t.Fatalf(
				"expected /before path after restore, got %#v",
				restoredPaths,
			)
		}
		if restoredPaths["/after"] != nil {
			t.Fatalf(
				"did not expect /after path after restore, got %#v",
				restoredPaths["/after"],
			)
		}
	})

	t.Run(
		"snapshot match reports false when build runtime state changed",
		func(t *testing.T) {
			fixture := testkit.NewBuildTestFixture(t, nil)
			app := fixture.App

			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetIsDev(false)
				l.SetBuildID("build-before")
				l.SetRouteManifestFile("route-manifest-before.json")
				l.Routes().
					ReplaceParsedPathsForInit(map[string]*vormaruntime.Path{
						"/before": {
							OriginalPattern: "/before",
							SrcPath:         "frontend/src/routes/before.tsx",
							ExportKey:       "default",
						},
					}, true)

				runtimeStateSnapshot := CaptureBuildRuntimeState(
					l,
				)

				l.SetBuildID("build-after")
				if BuildRuntimeStateSnapshotMatches(
					l,
					runtimeStateSnapshot,
				) {
					t.Fatalf(
						"expected changed build state not to match snapshot: %#v",
						runtimeStateSnapshot,
					)
				}
			})
		},
	)
}
