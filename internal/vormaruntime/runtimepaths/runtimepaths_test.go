package runtimepaths

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func TestVormaPathsStageFiles(t *testing.T) {
	if got, want := GetVormaPathsStageOneJSONPath(), "vorma_out/vorma_paths_stage_1.json"; got != want {
		t.Fatalf("GetVormaPathsStageOneJSONPath() = %q, want %q", got, want)
	}
	if got, want := GetVormaPathsStageTwoJSONPath(), "vorma_out/vorma_paths_stage_2.json"; got != want {
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
