package runtimepaths

import (
	"encoding/json"
	"github.com/vormadev/vorma/internal/testhelpers/waveoutputtest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestVormaPathsStageFiles(t *testing.T) {
	if got, want := GetVormaPathsStageOneJSONPath(), "vorma_owned/vorma_paths_stage_1.json"; got != want {
		t.Fatalf("GetVormaPathsStageOneJSONPath() = %q, want %q", got, want)
	}
	if got, want := GetVormaPathsStageTwoJSONPath(), "vorma_owned/vorma_paths_stage_2.json"; got != want {
		t.Fatalf("GetVormaPathsStageTwoJSONPath() = %q, want %q", got, want)
	}
}

func TestValidatePathsFileStructuralIntegrity(t *testing.T) {
	t.Run("rejects nil route entries", func(t *testing.T) {
		err := ValidatePathsFileStructuralIntegrity(&PathsFile{
			Paths: map[string]*RoutePath{
				"/": nil,
			},
		})
		if err == nil {
			t.Fatal("expected nil route entry to be rejected")
		}
	})

	t.Run("rejects key and original pattern mismatch", func(t *testing.T) {
		err := ValidatePathsFileStructuralIntegrity(&PathsFile{
			Paths: map[string]*RoutePath{
				"/a": {OriginalPattern: "/b"},
			},
		})
		if err == nil {
			t.Fatal("expected key/pattern mismatch to be rejected")
		}
	})

	t.Run("accepts structurally valid entries", func(t *testing.T) {
		err := ValidatePathsFileStructuralIntegrity(&PathsFile{
			Paths: map[string]*RoutePath{
				"/a": {
					OriginalPattern: "/a",
					SrcPath:         "routes/a.tsx",
					ExportKey:       "default",
				},
			},
		})
		if err != nil {
			t.Fatalf(
				"ValidatePathsFileStructuralIntegrity returned error: %v",
				err,
			)
		}
	})
}

func TestValidatePathsFileSemanticIntegrity(t *testing.T) {
	t.Run("requires route manifest file", func(t *testing.T) {
		err := ValidatePathsFileSemanticIntegrity(&PathsFile{}, true)
		if err == nil {
			t.Fatal("expected missing routeManifestFile to be rejected")
		}
	})

	t.Run("requires client entry out in production", func(t *testing.T) {
		err := ValidatePathsFileSemanticIntegrity(
			&PathsFile{
				RouteManifestFile: "manifest.json",
				Paths:             map[string]*RoutePath{},
			},
			false,
		)
		if err == nil {
			t.Fatal(
				"expected missing clientEntryOut to be rejected in production mode",
			)
		}
	})

	t.Run("requires export key when src path is set", func(t *testing.T) {
		err := ValidatePathsFileSemanticIntegrity(
			&PathsFile{
				RouteManifestFile: "manifest.json",
				ClientEntryOut:    "entry.js",
				Paths: map[string]*RoutePath{
					"/a": {
						OriginalPattern: "/a",
						SrcPath:         "routes/a.tsx",
					},
				},
			},
			false,
		)
		if err == nil {
			t.Fatal("expected missing exportKey to be rejected")
		}
	})

	t.Run(
		"requires out path in production when src path is set",
		func(t *testing.T) {
			err := ValidatePathsFileSemanticIntegrity(
				&PathsFile{
					RouteManifestFile: "manifest.json",
					ClientEntryOut:    "entry.js",
					Paths: map[string]*RoutePath{
						"/a": {
							OriginalPattern: "/a",
							SrcPath:         "routes/a.tsx",
							ExportKey:       "default",
						},
					},
				},
				false,
			)
			if err == nil {
				t.Fatal(
					"expected missing outPath to be rejected in production mode",
				)
			}
		},
	)

	t.Run("accepts valid stage-one payload in dev", func(t *testing.T) {
		err := ValidatePathsFileSemanticIntegrity(
			&PathsFile{
				RouteManifestFile: "manifest.json",
				Paths: map[string]*RoutePath{
					"/a": {
						OriginalPattern: "/a",
						SrcPath:         "routes/a.tsx",
						ExportKey:       "default",
					},
				},
			},
			true,
		)
		if err != nil {
			t.Fatalf(
				"ValidatePathsFileSemanticIntegrity returned error: %v",
				err,
			)
		}
	})
}

func TestLoadPathsFileFromFS(t *testing.T) {
	t.Run("loads stage-one file in dev mode", func(t *testing.T) {
		stageOne := PathsFile{
			Stage:             "one",
			RouteManifestFile: "manifest.json",
			Paths: map[string]*RoutePath{
				"/": {
					OriginalPattern: "/",
					SrcPath:         "routes/home.tsx",
					ExportKey:       "default",
				},
			},
		}
		stageOneJSON, marshalError := json.Marshal(stageOne)
		if marshalError != nil {
			t.Fatalf("marshal stage one fixture: %v", marshalError)
		}

		loaded, err := LoadPathsFileFromFS(
			fstest.MapFS{
				GetVormaPathsStageOneJSONPath(): &fstest.MapFile{
					Data: stageOneJSON,
				},
			},
			true,
		)
		if err != nil {
			t.Fatalf("LoadPathsFileFromFS returned error: %v", err)
		}
		if loaded.Stage != "one" {
			t.Fatalf("loaded stage = %q, want %q", loaded.Stage, "one")
		}
	})

	t.Run("loads stage-two file in production mode", func(t *testing.T) {
		stageTwo := PathsFile{
			Stage:             "two",
			ClientEntryOut:    "entry.js",
			RouteManifestFile: "manifest.json",
			Paths: map[string]*RoutePath{
				"/": {
					OriginalPattern: "/",
					SrcPath:         "routes/home.tsx",
					ExportKey:       "default",
					OutPath:         "home.js",
				},
			},
		}
		stageTwoJSON, marshalError := json.Marshal(stageTwo)
		if marshalError != nil {
			t.Fatalf("marshal stage two fixture: %v", marshalError)
		}

		loaded, err := LoadPathsFileFromFS(
			fstest.MapFS{
				GetVormaPathsStageTwoJSONPath(): &fstest.MapFile{
					Data: stageTwoJSON,
				},
			},
			false,
		)
		if err != nil {
			t.Fatalf("LoadPathsFileFromFS returned error: %v", err)
		}
		if loaded.Stage != "two" {
			t.Fatalf("loaded stage = %q, want %q", loaded.Stage, "two")
		}
	})

	t.Run(
		"returns semantic validation error for invalid production payload",
		func(t *testing.T) {
			invalidStageTwo := PathsFile{
				Stage:             "two",
				RouteManifestFile: "manifest.json",
				Paths: map[string]*RoutePath{
					"/": {
						OriginalPattern: "/",
						SrcPath:         "routes/home.tsx",
						ExportKey:       "default",
						OutPath:         "home.js",
					},
				},
			}
			invalidStageTwoJSON, marshalError := json.Marshal(invalidStageTwo)
			if marshalError != nil {
				t.Fatalf("marshal invalid stage two fixture: %v", marshalError)
			}

			_, err := LoadPathsFileFromFS(
				fstest.MapFS{
					GetVormaPathsStageTwoJSONPath(): &fstest.MapFile{
						Data: invalidStageTwoJSON,
					},
				},
				false,
			)
			if err == nil {
				t.Fatal(
					"expected LoadPathsFileFromFS to return production semantic validation error",
				)
			}
			if !strings.Contains(err.Error(), "clientEntryOut is required") {
				t.Fatalf(
					"error = %q, expected missing-clientEntryOut context",
					err,
				)
			}
		},
	)
}

func TestPrettyPrintFS_NoErrorOnBasicFS(t *testing.T) {
	fsys := fstest.MapFS{
		"root.txt":  {Data: []byte("ok")},
		"dir/a.txt": {Data: []byte("a")},
	}
	if err := PrettyPrintFS(fsys); err != nil {
		t.Fatalf("PrettyPrintFS returned error: %v", err)
	}
}

func TestBuildRuntimePathsFileSnapshot(t *testing.T) {
	t.Run("nil_paths_file_returns_nil_snapshot", func(t *testing.T) {
		if BuildRuntimePathsFileSnapshot(nil) != nil {
			t.Fatal("expected nil snapshot for nil paths file input")
		}
	})

	t.Run("maps_paths_file_fields_to_runtimecore_snapshot", func(t *testing.T) {
		pathsFile := &PathsFile{
			BuildID:        "build-artifacts",
			ClientEntrySrc: "frontend/src/vorma.entry.tsx",
			ClientEntryOut: waveoutputtest.TestWaveOutputPath("client-entry.js"),
			ClientEntryDeps: []string{
				waveoutputtest.TestWaveOutputPath("chunk-client.js"),
			},
			DepToCSSBundleMap: map[string][]string{
				waveoutputtest.TestWaveOutputPath("chunk-client.js"): {waveoutputtest.TestWaveOutputPath("chunk-client.css")},
			},
			RouteManifestFile: waveoutputtest.TestWaveOutputPath("route-manifest.js"),
			Paths: map[string]*RoutePath{
				"/products/:id": {
					OriginalPattern: "/products/:id",
					SrcPath:         "frontend/src/routes/products.$id.tsx",
					OutPath:         waveoutputtest.TestWaveOutputPath("routes/products.$id.js"),
					ExportKey:       "default",
					Deps:            []string{waveoutputtest.TestWaveOutputPath("products.js")},
				},
			},
		}

		snapshot := BuildRuntimePathsFileSnapshot(pathsFile)
		if snapshot == nil {
			t.Fatal("expected non-nil runtime paths snapshot")
		}
		if got, want := snapshot.BuildID, pathsFile.BuildID; got != want {
			t.Fatalf("BuildID = %q, want %q", got, want)
		}
		if got, want := snapshot.ClientEntrySrc, pathsFile.ClientEntrySrc; got != want {
			t.Fatalf("ClientEntrySrc = %q, want %q", got, want)
		}
		if got, want := snapshot.ClientEntryOut, pathsFile.ClientEntryOut; got != want {
			t.Fatalf("ClientEntryOut = %q, want %q", got, want)
		}
		if got, want := snapshot.RouteManifestFile, pathsFile.RouteManifestFile; got != want {
			t.Fatalf("RouteManifestFile = %q, want %q", got, want)
		}
		if got, want := snapshot.ClientEntryDeps, pathsFile.ClientEntryDeps; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("ClientEntryDeps = %#v, want %#v", got, want)
		}
		if got, want := snapshot.DepToCSSBundleMap, pathsFile.DepToCSSBundleMap; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("DepToCSSBundleMap = %#v, want %#v", got, want)
		}
		if got := snapshot.Paths["/products/:id"].Deps[0]; got != waveoutputtest.TestWaveOutputPath("products.js") {
			t.Fatalf(
				"snapshot route deps[0] = %q, want %q",
				got,
				waveoutputtest.TestWaveOutputPath("products.js"),
			)
		}
		pathsFile.Paths["/products/:id"].Deps[0] = "mutated"
		if got := snapshot.Paths["/products/:id"].Deps[0]; got != waveoutputtest.TestWaveOutputPath("products.js") {
			t.Fatalf(
				"snapshot route deps[0] after source mutation = %q, want %q",
				got,
				waveoutputtest.TestWaveOutputPath("products.js"),
			)
		}
	})
}

func TestParseRootTemplateFromFS(t *testing.T) {
	t.Run("returns_error_for_nil_fs", func(t *testing.T) {
		_, err := ParseRootTemplateFromFS(nil, "index.html")
		if err == nil {
			t.Fatal("expected error for nil private fs")
		}
		if !strings.Contains(err.Error(), "private fs is nil") {
			t.Fatalf("error = %q, want private fs is nil", err)
		}
	})

	t.Run("parses_template", func(t *testing.T) {
		fsys := fstest.MapFS{
			"index.html": &fstest.MapFile{
				Data: []byte("<!doctype html><html><body>{{.}}</body></html>"),
			},
		}
		rootTemplate, err := ParseRootTemplateFromFS(fsys, "index.html")
		if err != nil {
			t.Fatalf("ParseRootTemplateFromFS: %v", err)
		}
		if rootTemplate == nil {
			t.Fatal("expected parsed template")
		}
	})
}

func TestLoadRouteArtifactsFromFS(t *testing.T) {
	makeStageFileBytes := func(
		buildID string,
		stage string,
		includeOutPath bool,
	) []byte {
		pathsFile := &PathsFile{
			Stage:             stage,
			BuildID:           buildID,
			ClientEntrySrc:    "frontend/src/vorma.entry.tsx",
			ClientEntryOut:    waveoutputtest.TestWaveOutputPath("client-entry.js"),
			ClientEntryDeps:   []string{waveoutputtest.TestWaveOutputPath("chunk-client.js")},
			RouteManifestFile: waveoutputtest.TestWaveOutputPath("route-manifest.js"),
			Paths: map[string]*RoutePath{
				"/products/:id": {
					OriginalPattern: "/products/:id",
					SrcPath:         "frontend/src/routes/products.$id.tsx",
					ExportKey:       "default",
					Deps:            []string{waveoutputtest.TestWaveOutputPath("chunk-products.js")},
				},
			},
		}
		if includeOutPath {
			pathsFile.Paths["/products/:id"].OutPath = waveoutputtest.TestWaveOutputPath("routes/products.$id.js")
		}
		b, err := json.Marshal(pathsFile)
		if err != nil {
			t.Fatalf("marshal paths file: %v", err)
		}
		return b
	}

	t.Run("loads_stage_one_in_dev_mode", func(t *testing.T) {
		fsys := fstest.MapFS{
			GetVormaPathsStageOneJSONPath(): &fstest.MapFile{
				Data: makeStageFileBytes("build-dev", "stage-one", false),
			},
			GetVormaPathsStageTwoJSONPath(): &fstest.MapFile{
				Data: makeStageFileBytes("build-prod", "stage-two", true),
			},
		}

		output, err := LoadRouteArtifactsFromFS(fsys, true)
		if err != nil {
			t.Fatalf("LoadRouteArtifactsFromFS(dev): %v", err)
		}
		if got, want := output.PathsFile.BuildID, "build-dev"; got != want {
			t.Fatalf("dev BuildID = %q, want %q", got, want)
		}
		if got, want := output.RuntimeArtifacts.BuildID, "build-dev"; got != want {
			t.Fatalf("dev RuntimeArtifacts.BuildID = %q, want %q", got, want)
		}
	})

	t.Run("loads_stage_two_in_production_mode", func(t *testing.T) {
		fsys := fstest.MapFS{
			GetVormaPathsStageOneJSONPath(): &fstest.MapFile{
				Data: makeStageFileBytes("build-dev", "stage-one", false),
			},
			GetVormaPathsStageTwoJSONPath(): &fstest.MapFile{
				Data: makeStageFileBytes("build-prod", "stage-two", true),
			},
		}

		output, err := LoadRouteArtifactsFromFS(fsys, false)
		if err != nil {
			t.Fatalf("LoadRouteArtifactsFromFS(prod): %v", err)
		}
		if got, want := output.PathsFile.BuildID, "build-prod"; got != want {
			t.Fatalf("prod BuildID = %q, want %q", got, want)
		}
		if got, want := output.RuntimeArtifacts.BuildID, "build-prod"; got != want {
			t.Fatalf("prod RuntimeArtifacts.BuildID = %q, want %q", got, want)
		}
	})

	t.Run("returns_decode_error_for_malformed_file", func(t *testing.T) {
		fsys := fstest.MapFS{
			GetVormaPathsStageOneJSONPath(): &fstest.MapFile{
				Data: []byte("{"),
			},
		}
		_, err := LoadRouteArtifactsFromFS(fsys, true)
		if err == nil {
			t.Fatal("expected decode error for malformed stage file")
		}
		if !strings.Contains(err.Error(), "could not decode") {
			t.Fatalf("error = %q, expected decode failure", err)
		}
	})
}
