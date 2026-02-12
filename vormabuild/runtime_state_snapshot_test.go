package vormabuild

import (
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
)

func TestCaptureRouteBuildRuntimeStateSnapshot(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	var runtimeStateSnapshot routeBuildRuntimeStateSnapshot
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-before")
		l.SetRouteManifestFile("route-manifest-before.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/before": {
				OriginalPattern: "/before",
				SrcPath:         "frontend/src/routes/before.tsx",
				ExportKey:       "default",
				Deps:            []string{"vorma_out/chunk-before.js"},
			},
		})

		runtimeStateSnapshot = captureRouteBuildRuntimeStateSnapshot(l)

		l.SetBuildID("build-after")
		l.SetRouteManifestFile("route-manifest-after.json")
		paths := l.GetPaths()
		paths["/before"].SrcPath = "frontend/src/routes/after.tsx"
		paths["/before"].Deps[0] = "vorma_out/chunk-after.js"
	})

	if runtimeStateSnapshot.buildID != "build-before" {
		t.Fatalf("snapshot build ID = %q, want %q", runtimeStateSnapshot.buildID, "build-before")
	}
	if runtimeStateSnapshot.routeManifestFile != "route-manifest-before.json" {
		t.Fatalf(
			"snapshot route manifest = %q, want %q",
			runtimeStateSnapshot.routeManifestFile,
			"route-manifest-before.json",
		)
	}

	beforePath := runtimeStateSnapshot.paths["/before"]
	if beforePath == nil {
		t.Fatalf("snapshot paths = %#v, expected /before path", runtimeStateSnapshot.paths)
	}
	if beforePath.SrcPath != "frontend/src/routes/before.tsx" {
		t.Fatalf(
			"snapshot /before src path = %q, want %q",
			beforePath.SrcPath,
			"frontend/src/routes/before.tsx",
		)
	}
	if len(beforePath.Deps) != 1 || beforePath.Deps[0] != "vorma_out/chunk-before.js" {
		t.Fatalf(
			"snapshot /before deps = %#v, want %#v",
			beforePath.Deps,
			[]string{"vorma_out/chunk-before.js"},
		)
	}
}

func TestRestoreRouteBuildRuntimeStateSnapshot(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	var runtimeStateSnapshot routeBuildRuntimeStateSnapshot
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

		runtimeStateSnapshot = captureRouteBuildRuntimeStateSnapshot(l)

		l.Routes().ReplaceParsedPathsForInit(map[string]*vormaruntime.Path{
			"/after": {
				OriginalPattern: "/after",
				SrcPath:         "frontend/src/routes/after.tsx",
				ExportKey:       "default",
			},
		}, true)
		l.SetBuildID("build-after")
		l.SetRouteManifestFile("route-manifest-after.json")

		restoreRouteBuildRuntimeStateSnapshot(l, runtimeStateSnapshot)
	})

	if app.GetBuildID() != "build-before" {
		t.Fatalf("build ID after restore = %q, want %q", app.GetBuildID(), "build-before")
	}
	if app.GetRouteManifestFile() != "route-manifest-before.json" {
		t.Fatalf(
			"route manifest after restore = %q, want %q",
			app.GetRouteManifestFile(),
			"route-manifest-before.json",
		)
	}

	restoredPaths := app.GetPathsSnapshot()
	if len(restoredPaths) != 1 {
		t.Fatalf("restored paths length = %d, want 1 (%#v)", len(restoredPaths), restoredPaths)
	}
	if restoredPaths["/before"] == nil {
		t.Fatalf("expected /before path after restore, got %#v", restoredPaths)
	}
	if restoredPaths["/after"] != nil {
		t.Fatalf("did not expect /after path after restore, got %#v", restoredPaths["/after"])
	}

	if !app.LoadersRouter().NestedRouter.IsRegistered("/before") {
		t.Fatal("expected /before to be registered in nested router after restore")
	}
	if app.LoadersRouter().NestedRouter.IsRegistered("/after") {
		t.Fatal("did not expect /after to remain registered in nested router after restore")
	}
}

func TestCloneRouteBuildRuntimePath(t *testing.T) {
	if got := cloneRouteBuildRuntimePath(nil); got != nil {
		t.Fatalf("cloneRouteBuildRuntimePath(nil) = %#v, want nil", got)
	}
}
