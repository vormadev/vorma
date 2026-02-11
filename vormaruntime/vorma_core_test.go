package vormaruntime

import (
	"html/template"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/lab/tsgen"
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

func TestLockedVormaGettersAndSetters(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app
	app.SetIsDev(true)

	tmpl := template.Must(template.New("root").Parse("<html>{{.VormaBodyScripts}}</html>"))

	app.WithLock(func(lv *LockedVorma) {
		lv.SetBuildID("locked-build")
		lv.SetRouteManifestFile("vorma_out/locked-route-manifest.js")
		lv.SetRootTemplate(tmpl)
		lv.SetPaths(map[string]*Path{
			"/locked": {
				OriginalPattern: "/locked",
				SrcPath:         "frontend/src/routes/locked.tsx",
				OutPath:         "vorma_out/routes/locked.js",
				ExportKey:       "default",
			},
		})
	})

	if got := app.GetBuildID(); got != "locked-build" {
		t.Fatalf("GetBuildID() = %q, want %q", got, "locked-build")
	}
	if got := app.GetRouteManifestFile(); got != "vorma_out/locked-route-manifest.js" {
		t.Fatalf("GetRouteManifestFile() = %q, want %q", got, "vorma_out/locked-route-manifest.js")
	}
	if got := app.GetRootTemplate(); got == nil {
		t.Fatal("GetRootTemplate() returned nil after SetRootTemplate")
	}
	if got := app.GetPathsSnapshot(); got["/locked"] == nil {
		t.Fatal("GetPathsSnapshot() missing /locked after SetPaths")
	}

	app.WithRLock(func(lv *LockedVorma) {
		if got := lv.Vorma(); got != app {
			t.Fatal("LockedVorma.Vorma() did not return underlying app instance")
		}
		if got := lv.GetBuildID(); got != "locked-build" {
			t.Fatalf("LockedVorma.GetBuildID() = %q, want %q", got, "locked-build")
		}
		if got := lv.GetRouteManifestFile(); got != "vorma_out/locked-route-manifest.js" {
			t.Fatalf("LockedVorma.GetRouteManifestFile() = %q, want %q", got, "vorma_out/locked-route-manifest.js")
		}
		if got := lv.GetRootTemplate(); got == nil {
			t.Fatal("LockedVorma.GetRootTemplate() returned nil")
		}
		if got := lv.GetPaths(); got["/locked"] == nil {
			t.Fatal("LockedVorma.GetPaths() missing /locked")
		}
		if !lv.GetIsDev() {
			t.Fatal("LockedVorma.GetIsDev() = false, want true")
		}
	})
}

func TestCoreAccessorsAndServerAddr(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})
	app := fixture.app

	if got := app.ServerAddr(); got != ":8080" {
		t.Fatalf("ServerAddr() = %q, want %q", got, ":8080")
	}
	if app.LoadersRouter() == nil {
		t.Fatal("LoadersRouter() should not return nil")
	}
	if app.ActionsRouter() == nil {
		t.Fatal("ActionsRouter() should not return nil")
	}
}

func TestTSGenerationGettersReflectConfiguredValues(t *testing.T) {
	adHoc := []*tsgen.AdHocType{
		{
			TypeInstance: struct {
				Name string
			}{},
			TSTypeName: "UserDTO",
		},
	}

	fixture := newTestFixture(t, testFixtureOptions{
		adHocTypes:  adHoc,
		extraTSCode: "export type Extra = string;",
	})
	app := fixture.app

	gotTypes := app.GetAdHocTypes()
	if len(gotTypes) != 1 {
		t.Fatalf("GetAdHocTypes length = %d, want %d", len(gotTypes), 1)
	}
	if gotTypes[0].TSTypeName != "UserDTO" {
		t.Fatalf("GetAdHocTypes[0].TSTypeName = %q, want %q", gotTypes[0].TSTypeName, "UserDTO")
	}
	if got := app.GetExtraTSCode(); got != "export type Extra = string;" {
		t.Fatalf("GetExtraTSCode() = %q, want %q", got, "export type Extra = string;")
	}
}
