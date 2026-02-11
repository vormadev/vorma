package vormaruntime

import (
	"reflect"
	"testing"
)

func TestGetterSnapshotsAreDefensiveCopies(t *testing.T) {
	stage := defaultPathsFile("build-copy", map[string]*Path{
		"/": {
			OriginalPattern: "/",
			SrcPath:         "frontend/src/routes/root.tsx",
			OutPath:         "vorma_out/root.js",
			ExportKey:       "default",
			Deps:            []string{"vorma_out/root-dep.js"},
		},
	})
	stage.ClientEntryDeps = []string{"vorma_out/client-a.js", "vorma_out/client-b.js"}
	stage.DepToCSSBundleMap = map[string][]string{
		"vorma_out/client-entry.js": {"vorma_out/client.css"},
		"vorma_out/client-a.js":     {"vorma_out/client-a.css"},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		stageOne: stage,
		stageTwo: stage,
	})
	app := fixture.app

	pathsA := app.GetPathsSnapshot()
	pathsA["/new"] = &Path{OriginalPattern: "/new"}
	pathsA["/"].Deps[0] = "mutated-dep.js"

	pathsB := app.GetPathsSnapshot()
	if _, ok := pathsB["/new"]; ok {
		t.Fatal("GetPathsSnapshot leaked caller mutation into runtime state")
	}
	if got, want := pathsB["/"].Deps[0], "vorma_out/root-dep.js"; got != want {
		t.Fatalf("path deps mutated through snapshot: got %q want %q", got, want)
	}

	entryDepsA := app.GetClientEntryDeps()
	entryDepsA[0] = "mutated-client-dep.js"
	entryDepsB := app.GetClientEntryDeps()
	if got, want := entryDepsB[0], "vorma_out/client-a.js"; got != want {
		t.Fatalf("client entry deps mutated through getter: got %q want %q", got, want)
	}

	cssMapA := app.GetDepToCSSBundleMap()
	cssMapA["vorma_out/client-entry.js"][0] = "mutated.css"
	cssMapA["vorma_out/new.js"] = []string{"new.css"}

	cssMapB := app.GetDepToCSSBundleMap()
	if got, want := cssMapB["vorma_out/client-entry.js"][0], "vorma_out/client.css"; got != want {
		t.Fatalf("dep->css map mutated through getter: got %q want %q", got, want)
	}
	if _, ok := cssMapB["vorma_out/new.js"]; ok {
		t.Fatal("dep->css map leaked caller-added key into runtime state")
	}
}

func TestGettersPreserveNilShape(t *testing.T) {
	app := &Vorma{}

	if got := app.GetPathsSnapshot(); got != nil {
		t.Fatalf("GetPathsSnapshot with nil internal paths should return nil, got %#v", got)
	}
	if got := app.GetClientEntryDeps(); got != nil {
		t.Fatalf("GetClientEntryDeps with nil internal deps should return nil, got %#v", got)
	}
	if got := app.GetDepToCSSBundleMap(); got != nil {
		t.Fatalf("GetDepToCSSBundleMap with nil internal map should return nil, got %#v", got)
	}

	app.WithLock(func(lv *LockedVorma) {
		lv.SetPaths(map[string]*Path{"/": &Path{OriginalPattern: "/"}})
	})
	paths := app.GetPathsSnapshot()
	if !reflect.DeepEqual(paths, map[string]*Path{"/": &Path{OriginalPattern: "/"}}) {
		t.Fatalf("unexpected paths snapshot after set: %#v", paths)
	}
}
