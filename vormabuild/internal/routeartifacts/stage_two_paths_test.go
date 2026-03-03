package routeartifacts

import (
	"encoding/json"
	"github.com/vormadev/vorma/internal/testhelpers/waveoutputtest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func TestToPathsFileStageTwo_TransformsManifestWithoutMutatingBuildID(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-before-stage-two")
		l.SetRouteManifestFile(waveoutputtest.TestWaveOutputPrefixedFileName("route_manifest.json"))
		l.SetPaths(map[string]*vormaruntime.Path{
			"/users/:id": {
				OriginalPattern: "/users/:id",
				SrcPath:         "frontend/src/routes/users.tsx",
				ExportKey:       "default",
			},
		})
	})

	testkit.MustWriteFile(
		t,
		filepath.Join(fixture.PublicDir, "public-asset.txt"),
		[]byte("public asset"),
	)

	manifest := viteutil.Manifest{
		"frontend/src/vorma.entry.tsx": {
			Src:     "frontend/src/vorma.entry.tsx",
			File:    waveoutputtest.TestWaveOutputAssetPath("entry-abc.js"),
			CSS:     []string{waveoutputtest.TestWaveOutputAssetPath("entry.css")},
			IsEntry: true,
			Imports: []string{"shared-chunk.js"},
		},
		"shared-chunk.js": {
			File: waveoutputtest.TestWaveOutputAssetPath("shared.js"),
			CSS:  []string{waveoutputtest.TestWaveOutputAssetPath("shared.css")},
		},
		"frontend/src/routes/users.tsx": {
			Src:     "frontend/src/routes/users.tsx",
			File:    waveoutputtest.TestWaveOutputAssetPath("users.js"),
			CSS:     []string{waveoutputtest.TestWaveOutputAssetPath("users.css")},
			Imports: []string{"shared-chunk.js"},
		},
	}
	testkit.MustWriteJSONFile(t, app.Wave.ViteManifestLocation(), manifest)

	pathsFile, err := toPathsFileStageTwo(app)
	if err != nil {
		t.Fatalf("toPathsFileStageTwo returned error: %v", err)
	}

	if pathsFile.Stage != "two" {
		t.Fatalf("stage = %q, want %q", pathsFile.Stage, "two")
	}
	if pathsFile.ClientEntryOut != "entry-abc.js" {
		t.Fatalf(
			"client entry out = %q, want %q",
			pathsFile.ClientEntryOut,
			"entry-abc.js",
		)
	}
	if slices.Contains(pathsFile.ClientEntryDeps, "entry-abc.js") {
		t.Fatalf(
			"client entry deps should exclude client entry output itself, deps = %#v",
			pathsFile.ClientEntryDeps,
		)
	}
	if !slices.Contains(pathsFile.ClientEntryDeps, "shared.js") {
		t.Fatalf(
			"client entry deps = %#v, expected shared dependency",
			pathsFile.ClientEntryDeps,
		)
	}

	routePath := pathsFile.Paths["/users/:id"]
	if routePath == nil {
		t.Fatalf("missing /users/:id route in paths file")
	}
	if routePath.OutPath != "users.js" {
		t.Fatalf("route out path = %q, want %q", routePath.OutPath, "users.js")
	}
	if !slices.Contains(routePath.Deps, "users.js") ||
		!slices.Contains(routePath.Deps, "shared.js") {
		t.Fatalf(
			"route deps = %#v, expected users.js and shared.js",
			routePath.Deps,
		)
	}

	entryCSS := pathsFile.DepToCSSBundleMap["entry-abc.js"]
	if !slices.Equal(entryCSS, []string{"entry.css"}) {
		t.Fatalf("entry css = %#v, want %#v", entryCSS, []string{"entry.css"})
	}
	usersCSS := pathsFile.DepToCSSBundleMap["users.js"]
	if !slices.Equal(usersCSS, []string{"users.css"}) {
		t.Fatalf("users css = %#v, want %#v", usersCSS, []string{"users.css"})
	}

	if pathsFile.RouteManifestFile != waveoutputtest.TestWaveOutputPrefixedFileName("route_manifest.json") {
		t.Fatalf(
			"route manifest file = %q, want %q",
			pathsFile.RouteManifestFile,
			waveoutputtest.TestWaveOutputPrefixedFileName("route_manifest.json"),
		)
	}
	if pathsFile.BuildID == "" {
		t.Fatal("expected non-empty build ID")
	}
	if app.BuildID() != "build-before-stage-two" {
		t.Fatalf(
			"app build ID = %q, want %q",
			app.BuildID(),
			"build-before-stage-two",
		)
	}
}

func TestRemoveDependency_RetainsOrderAndExcludesMatches(t *testing.T) {
	dependencies := []string{"entry.js", "shared.js", "entry.js", "feature.js"}
	got := removeDependency(dependencies, "entry.js")
	want := []string{"shared.js", "feature.js"}
	if !slices.Equal(got, want) {
		t.Fatalf(
			"removeDependency(%#v, %q) = %#v, want %#v",
			dependencies,
			"entry.js",
			got,
			want,
		)
	}
}

func TestApplyViteManifestToPaths_UpdatesClientEntryAndRoutePaths(
	t *testing.T,
) {
	manifest := viteutil.Manifest{
		"frontend/src/vorma.entry.tsx": {
			Src:     "frontend/src/vorma.entry.tsx",
			File:    waveoutputtest.TestWaveOutputAssetPath("entry.js"),
			IsEntry: true,
			Imports: []string{"shared.js"},
			CSS:     []string{waveoutputtest.TestWaveOutputAssetPath("entry.css")},
		},
		"shared.js": {
			File: waveoutputtest.TestWaveOutputAssetPath("shared.js"),
		},
		"frontend/src/routes/home.tsx": {
			Src:     "frontend/src/routes/home.tsx",
			File:    waveoutputtest.TestWaveOutputAssetPath("home.js"),
			Imports: []string{"shared.js"},
			CSS:     []string{waveoutputtest.TestWaveOutputAssetPath("home.css")},
		},
	}
	paths := map[string]*vormaruntime.Path{
		"/home": {
			OriginalPattern: "/home",
			SrcPath:         "frontend/src/routes/home.tsx",
			ExportKey:       "default",
		},
	}

	result := applyViteManifestToPaths(
		manifest,
		paths,
		"frontend/src/vorma.entry.tsx",
	)

	if result.clientEntryOut != "entry.js" {
		t.Fatalf(
			"client entry out = %q, want %q",
			result.clientEntryOut,
			"entry.js",
		)
	}
	if !slices.Equal(result.clientEntryDeps, []string{"shared.js"}) {
		t.Fatalf(
			"client entry deps = %#v, want %#v",
			result.clientEntryDeps,
			[]string{"shared.js"},
		)
	}
	if !slices.Equal(
		result.depToCSSBundleMap["entry.js"],
		[]string{"entry.css"},
	) {
		t.Fatalf(
			"entry css bundles = %#v, want %#v",
			result.depToCSSBundleMap["entry.js"],
			[]string{"entry.css"},
		)
	}
	if !slices.Equal(
		result.depToCSSBundleMap["home.js"],
		[]string{"home.css"},
	) {
		t.Fatalf(
			"home css bundles = %#v, want %#v",
			result.depToCSSBundleMap["home.js"],
			[]string{"home.css"},
		)
	}

	home := paths["/home"]
	if home == nil {
		t.Fatal("missing /home path")
	}
	if home.OutPath != "home.js" {
		t.Fatalf("home out path = %q, want %q", home.OutPath, "home.js")
	}
	if !slices.Equal(home.Deps, []string{"home.js", "shared.js"}) {
		t.Fatalf(
			"home deps = %#v, want %#v",
			home.Deps,
			[]string{"home.js", "shared.js"},
		)
	}
}

func TestApplyViteManifestToPaths_UpdatesAllRoutesSharingSameSourcePath(
	t *testing.T,
) {
	manifest := viteutil.Manifest{
		"frontend/src/routes/shared.tsx": {
			Src:  "frontend/src/routes/shared.tsx",
			File: waveoutputtest.TestWaveOutputAssetPath("shared-route.js"),
		},
	}
	paths := map[string]*vormaruntime.Path{
		"/a": {
			OriginalPattern: "/a",
			SrcPath:         "frontend/src/routes/shared.tsx",
			ExportKey:       "default",
		},
		"/b": {
			OriginalPattern: "/b",
			SrcPath:         "frontend/src/routes/shared.tsx",
			ExportKey:       "default",
		},
	}

	_ = applyViteManifestToPaths(
		manifest,
		paths,
		"frontend/src/vorma.entry.tsx",
	)

	pathA := paths["/a"]
	pathB := paths["/b"]
	if pathA.OutPath != "shared-route.js" ||
		pathB.OutPath != "shared-route.js" {
		t.Fatalf(
			"expected both routes to receive same out path, got /a=%q /b=%q",
			pathA.OutPath,
			pathB.OutPath,
		)
	}
	if !slices.Equal(pathA.Deps, []string{"shared-route.js"}) ||
		!slices.Equal(pathB.Deps, []string{"shared-route.js"}) {
		t.Fatalf(
			"expected both routes to receive same deps, got /a=%#v /b=%#v",
			pathA.Deps,
			pathB.Deps,
		)
	}
}

func TestIndexPathsBySourcePath(t *testing.T) {
	paths := map[string]*vormaruntime.Path{
		"/a": {SrcPath: "one.tsx"},
		"/b": {SrcPath: "two.tsx"},
		"/c": {SrcPath: "one.tsx"},
	}
	indexed := indexPathsBySourcePath(paths)

	if len(indexed["one.tsx"]) != 2 {
		t.Fatalf("one.tsx index length = %d, want 2", len(indexed["one.tsx"]))
	}
	if len(indexed["two.tsx"]) != 1 {
		t.Fatalf("two.tsx index length = %d, want 1", len(indexed["two.tsx"]))
	}
}

func TestPathsOutputPath_StageTwoFile(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	got := pathsOutputPath(app, runtimepaths.VormaPathsStageTwoJSONFileName)
	want := filepath.Join(
		app.Wave.StaticPrivateOutDir(),
		runtimepaths.VormaInternalDirname,
		runtimepaths.VormaPathsStageTwoJSONFileName,
	)
	if got != want {
		t.Fatalf("pathsOutputPath(stage-two) = %q, want %q", got, want)
	}
}

func TestPathsOutputPath(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	got := pathsOutputPath(app, "custom.json")
	want := filepath.Join(
		app.Wave.StaticPrivateOutDir(),
		runtimepaths.VormaInternalDirname,
		"custom.json",
	)
	if got != want {
		t.Fatalf("pathsOutputPath() = %q, want %q", got, want)
	}
}

func TestWritePathsToDiskStageTwo_ReturnsErrorWhenParentIsNotDirectory(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	stageTwoDir := filepath.Join(
		app.Wave.StaticPrivateOutDir(),
		runtimepaths.VormaInternalDirname,
	)
	if err := os.RemoveAll(stageTwoDir); err != nil {
		t.Fatalf("remove stage two dir: %v", err)
	}
	testkit.MustWriteFile(t, stageTwoDir, []byte("not-a-directory"))

	err := writePathsToDiskStageTwo(app, &runtimepaths.PathsFile{Stage: "two"})
	if err == nil {
		t.Fatal(
			"expected writePathsToDiskStageTwo to fail when parent path is not a directory",
		)
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("error = %q, expected not-a-directory message", err)
	}
}

func TestWritePathsToDiskStageTwo_CreatesOutputDirectoryWhenMissing(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	stageTwoDir := filepath.Join(
		app.Wave.StaticPrivateOutDir(),
		runtimepaths.VormaInternalDirname,
	)
	if err := os.RemoveAll(stageTwoDir); err != nil {
		t.Fatalf("remove stage two dir: %v", err)
	}

	if err := writePathsToDiskStageTwo(app, &runtimepaths.PathsFile{Stage: "two"}); err != nil {
		t.Fatalf("writePathsToDiskStageTwo returned error: %v", err)
	}

	outputPath := pathsOutputPath(
		app,
		runtimepaths.VormaPathsStageTwoJSONFileName,
	)
	outputBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read stage-two output failed: %v", err)
	}
	var parsedPathsFile runtimepaths.PathsFile
	if err := json.Unmarshal(outputBytes, &parsedPathsFile); err != nil {
		t.Fatalf("unmarshal stage-two output failed: %v", err)
	}
	if parsedPathsFile.Stage != "two" {
		t.Fatalf(
			"stage-two output stage = %q, want %q",
			parsedPathsFile.Stage,
			"two",
		)
	}
}
