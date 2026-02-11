package vormabuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormaruntime"
)

func TestToPathsFileStageTwo_TransformsManifestAndUpdatesBuildID(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetRouteManifestFile("vorma_out_route_manifest.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/users/:id": {
				OriginalPattern: "/users/:id",
				SrcPath:         "frontend/src/routes/users.tsx",
				ExportKey:       "default",
			},
		})
	})

	mustWriteFile(t, filepath.Join(fixture.publicDir, "public-asset.txt"), []byte("public asset"))

	manifest := viteutil.Manifest{
		"frontend/src/vorma.entry.tsx": {
			Src:     "frontend/src/vorma.entry.tsx",
			File:    "assets/vorma_out/entry-abc.js",
			CSS:     []string{"assets/vorma_out/entry.css"},
			IsEntry: true,
			Imports: []string{"shared-chunk.js"},
		},
		"shared-chunk.js": {
			File: "assets/vorma_out/shared.js",
			CSS:  []string{"assets/vorma_out/shared.css"},
		},
		"frontend/src/routes/users.tsx": {
			Src:     "frontend/src/routes/users.tsx",
			File:    "assets/vorma_out/users.js",
			CSS:     []string{"assets/vorma_out/users.css"},
			Imports: []string{"shared-chunk.js"},
		},
	}
	mustWriteJSONFile(t, app.Wave.GetViteManifestLocation(), manifest)

	pathsFile, err := toPathsFile_StageTwo(app)
	if err != nil {
		t.Fatalf("toPathsFile_StageTwo returned error: %v", err)
	}

	if pathsFile.Stage != "two" {
		t.Fatalf("stage = %q, want %q", pathsFile.Stage, "two")
	}
	if pathsFile.ClientEntryOut != "entry-abc.js" {
		t.Fatalf("client entry out = %q, want %q", pathsFile.ClientEntryOut, "entry-abc.js")
	}
	if slices.Contains(pathsFile.ClientEntryDeps, "entry-abc.js") {
		t.Fatalf("client entry deps should exclude client entry output itself, deps = %#v", pathsFile.ClientEntryDeps)
	}
	if !slices.Contains(pathsFile.ClientEntryDeps, "shared.js") {
		t.Fatalf("client entry deps = %#v, expected shared dependency", pathsFile.ClientEntryDeps)
	}

	routePath := pathsFile.Paths["/users/:id"]
	if routePath == nil {
		t.Fatalf("missing /users/:id route in paths file")
	}
	if routePath.OutPath != "users.js" {
		t.Fatalf("route out path = %q, want %q", routePath.OutPath, "users.js")
	}
	if !slices.Contains(routePath.Deps, "users.js") || !slices.Contains(routePath.Deps, "shared.js") {
		t.Fatalf("route deps = %#v, expected users.js and shared.js", routePath.Deps)
	}

	entryCSS := pathsFile.DepToCSSBundleMap["entry-abc.js"]
	if !slices.Equal(entryCSS, []string{"entry.css"}) {
		t.Fatalf("entry css = %#v, want %#v", entryCSS, []string{"entry.css"})
	}
	usersCSS := pathsFile.DepToCSSBundleMap["users.js"]
	if !slices.Equal(usersCSS, []string{"users.css"}) {
		t.Fatalf("users css = %#v, want %#v", usersCSS, []string{"users.css"})
	}

	if pathsFile.RouteManifestFile != "vorma_out_route_manifest.json" {
		t.Fatalf("route manifest file = %q, want %q", pathsFile.RouteManifestFile, "vorma_out_route_manifest.json")
	}
	if pathsFile.BuildID == "" {
		t.Fatal("expected non-empty build ID")
	}
	if app.GetBuildID() != pathsFile.BuildID {
		t.Fatalf("app build ID = %q, want %q", app.GetBuildID(), pathsFile.BuildID)
	}
}

func TestPostViteProdBuild_WritesStageTwoPathsFile(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

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
			File:    "assets/vorma_out/entry.js",
			IsEntry: true,
		},
		"frontend/src/routes/root.tsx": {
			Src:  "frontend/src/routes/root.tsx",
			File: "assets/vorma_out/root.js",
		},
	}
	mustWriteJSONFile(t, app.Wave.GetViteManifestLocation(), manifest)

	if err := postViteProdBuild(app); err != nil {
		t.Fatalf("postViteProdBuild returned error: %v", err)
	}

	stageTwoPath := filepath.Join(
		fixture.privateDir,
		"vorma_out",
		vormaruntime.VormaPathsStageTwoJSONFileName,
	)
	bytes, err := os.ReadFile(stageTwoPath)
	if err != nil {
		t.Fatalf("read stage two file: %v", err)
	}

	var parsed vormaruntime.PathsFile
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("unmarshal stage two file: %v", err)
	}
	if parsed.Stage != "two" {
		t.Fatalf("stage = %q, want %q", parsed.Stage, "two")
	}
	if parsed.ClientEntryOut != "entry.js" {
		t.Fatalf("client entry out = %q, want %q", parsed.ClientEntryOut, "entry.js")
	}
	if parsed.Paths["/"] == nil || parsed.Paths["/"].OutPath != "root.js" {
		t.Fatalf("root route output = %#v, expected out path root.js", parsed.Paths["/"])
	}
}

func TestRemoveDependency_RetainsOrderAndExcludesMatches(t *testing.T) {
	dependencies := []string{"entry.js", "shared.js", "entry.js", "feature.js"}
	got := removeDependency(dependencies, "entry.js")
	want := []string{"shared.js", "feature.js"}
	if !slices.Equal(got, want) {
		t.Fatalf("removeDependency(%#v, %q) = %#v, want %#v", dependencies, "entry.js", got, want)
	}
}

func TestApplyViteManifestToPaths_UpdatesClientEntryAndRoutePaths(t *testing.T) {
	manifest := viteutil.Manifest{
		"frontend/src/vorma.entry.tsx": {
			Src:     "frontend/src/vorma.entry.tsx",
			File:    "assets/vorma_out/entry.js",
			IsEntry: true,
			Imports: []string{"shared.js"},
			CSS:     []string{"assets/vorma_out/entry.css"},
		},
		"shared.js": {
			File: "assets/vorma_out/shared.js",
		},
		"frontend/src/routes/home.tsx": {
			Src:     "frontend/src/routes/home.tsx",
			File:    "assets/vorma_out/home.js",
			Imports: []string{"shared.js"},
			CSS:     []string{"assets/vorma_out/home.css"},
		},
	}
	paths := map[string]*vormaruntime.Path{
		"/home": {
			OriginalPattern: "/home",
			SrcPath:         "frontend/src/routes/home.tsx",
			ExportKey:       "default",
		},
	}

	clientEntryOut, clientEntryDeps, depToCSSBundleMap := applyViteManifestToPaths(
		manifest,
		paths,
		"frontend/src/vorma.entry.tsx",
	)

	if clientEntryOut != "entry.js" {
		t.Fatalf("client entry out = %q, want %q", clientEntryOut, "entry.js")
	}
	if !slices.Equal(clientEntryDeps, []string{"shared.js"}) {
		t.Fatalf("client entry deps = %#v, want %#v", clientEntryDeps, []string{"shared.js"})
	}
	if !slices.Equal(depToCSSBundleMap["entry.js"], []string{"entry.css"}) {
		t.Fatalf("entry css bundles = %#v, want %#v", depToCSSBundleMap["entry.js"], []string{"entry.css"})
	}
	if !slices.Equal(depToCSSBundleMap["home.js"], []string{"home.css"}) {
		t.Fatalf("home css bundles = %#v, want %#v", depToCSSBundleMap["home.js"], []string{"home.css"})
	}

	home := paths["/home"]
	if home == nil {
		t.Fatal("missing /home path")
	}
	if home.OutPath != "home.js" {
		t.Fatalf("home out path = %q, want %q", home.OutPath, "home.js")
	}
	if !slices.Equal(home.Deps, []string{"home.js", "shared.js"}) {
		t.Fatalf("home deps = %#v, want %#v", home.Deps, []string{"home.js", "shared.js"})
	}
}
