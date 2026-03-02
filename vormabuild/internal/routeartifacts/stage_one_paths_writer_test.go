package routeartifacts

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/vormabuild/internal/buildlifecycle"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func newStageOnePathsWriterExecutorForTest(
	mutateDependencies func(*stageOnePathsWriterDependencies),
) stageOnePathsWriterExecutor {
	dependencies := stageOnePathsWriterDependencies{}
	if mutateDependencies != nil {
		mutateDependencies(&dependencies)
	}
	return newStageOnePathsWriterExecutor(dependencies)
}

func TestWritePathsToDiskStageOne(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("stage-one-build-id")
		l.SetRouteManifestFile("vorma_route_manifest_test.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
	})

	withLockedVorma := func(t *testing.T, fn func(*vormaruntime.LockedVorma)) {
		t.Helper()
		app.WithLock(fn)
	}

	t.Run(
		"writes stage-one paths file with expected metadata",
		func(t *testing.T) {
			withLockedVorma(t, func(l *vormaruntime.LockedVorma) {
				if err := writePathsToDiskStageOne(l); err != nil {
					t.Fatalf("writePathsToDiskStageOne returned error: %v", err)
				}
			})

			outputPath := pathsOutputPath(
				app,
				runtimepaths.VormaPathsStageOneJSONFileName,
			)
			rawJSON, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("read stage-one output failed: %v", err)
			}

			var parsed runtimepaths.PathsFile
			if err := json.Unmarshal(rawJSON, &parsed); err != nil {
				t.Fatalf("unmarshal stage-one output failed: %v", err)
			}
			if parsed.Stage != "one" {
				t.Fatalf("stage = %q, want %q", parsed.Stage, "one")
			}
			if parsed.BuildID != "stage-one-build-id" {
				t.Fatalf(
					"buildID = %q, want %q",
					parsed.BuildID,
					"stage-one-build-id",
				)
			}
			if parsed.RouteManifestFile != "vorma_route_manifest_test.json" {
				t.Fatalf(
					"routeManifestFile = %q, want %q",
					parsed.RouteManifestFile,
					"vorma_route_manifest_test.json",
				)
			}
			if _, ok := parsed.Paths["/"]; !ok {
				t.Fatalf(
					"expected root path in stage-one output, got %#v",
					parsed.Paths,
				)
			}
		},
	)

	t.Run("wraps marshal error", func(t *testing.T) {
		expectedErr := errors.New("marshal failed")
		executor := newStageOnePathsWriterExecutorForTest(
			func(dependencies *stageOnePathsWriterDependencies) {
				dependencies.marshalStageOnePathsFile = func(*runtimepaths.PathsFile) ([]byte, error) {
					return nil, expectedErr
				}
				dependencies.makeStageOnePathsOutputDirectory = func(string, os.FileMode) error {
					t.Fatal(
						"did not expect directory creation after marshal failure",
					)
					return nil
				}
				dependencies.writeStageOnePathsJSON = func(string, []byte, os.FileMode) error {
					t.Fatal("did not expect file write after marshal failure")
					return nil
				}
			},
		)

		var err error
		withLockedVorma(t, func(l *vormaruntime.LockedVorma) {
			err = executor.writePathsToDiskStageOneWithRouteManifest(
				l,
				l.RouteManifestFile(),
			)
		})
		if err == nil {
			t.Fatal("expected writePathsToDiskStageOne to return marshal error")
		}
		if !strings.Contains(err.Error(), "marshal stage-one paths file") {
			t.Fatalf("error = %q, expected marshal context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped marshal error", err)
		}
	})

	t.Run("wraps output-directory creation error", func(t *testing.T) {
		expectedErr := errors.New("mkdir failed")
		executor := newStageOnePathsWriterExecutorForTest(
			func(dependencies *stageOnePathsWriterDependencies) {
				dependencies.marshalStageOnePathsFile = func(*runtimepaths.PathsFile) ([]byte, error) {
					return []byte(`{}`), nil
				}
				dependencies.makeStageOnePathsOutputDirectory = func(string, os.FileMode) error {
					return expectedErr
				}
				dependencies.writeStageOnePathsJSON = func(string, []byte, os.FileMode) error {
					t.Fatal(
						"did not expect file write after directory creation failure",
					)
					return nil
				}
			},
		)

		var err error
		withLockedVorma(t, func(l *vormaruntime.LockedVorma) {
			err = executor.writePathsToDiskStageOneWithRouteManifest(
				l,
				l.RouteManifestFile(),
			)
		})
		if err == nil {
			t.Fatal(
				"expected writePathsToDiskStageOne to return directory-creation error",
			)
		}
		if !strings.Contains(
			err.Error(),
			"create stage-one paths output directory",
		) {
			t.Fatalf("error = %q, expected directory-creation context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf(
				"error = %v, expected wrapped directory-creation error",
				err,
			)
		}
	})

	t.Run("wraps write-file error", func(t *testing.T) {
		expectedErr := errors.New("write failed")
		executor := newStageOnePathsWriterExecutorForTest(
			func(dependencies *stageOnePathsWriterDependencies) {
				dependencies.marshalStageOnePathsFile = func(*runtimepaths.PathsFile) ([]byte, error) {
					return []byte(`{}`), nil
				}
				dependencies.makeStageOnePathsOutputDirectory = func(string, os.FileMode) error {
					return nil
				}
				dependencies.writeStageOnePathsJSON = func(string, []byte, os.FileMode) error {
					return expectedErr
				}
			},
		)

		var err error
		withLockedVorma(t, func(l *vormaruntime.LockedVorma) {
			err = executor.writePathsToDiskStageOneWithRouteManifest(
				l,
				l.RouteManifestFile(),
			)
		})
		if err == nil {
			t.Fatal(
				"expected writePathsToDiskStageOne to return write-file error",
			)
		}
		if !strings.Contains(err.Error(), "write stage-one paths JSON") {
			t.Fatalf("error = %q, expected write-file context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped write-file error", err)
		}
	})

	t.Run(
		"writes stage-one paths JSON with build artifact file mode",
		func(t *testing.T) {
			var capturedDirectoryMode os.FileMode
			var capturedFileMode os.FileMode
			executor := newStageOnePathsWriterExecutorForTest(
				func(dependencies *stageOnePathsWriterDependencies) {
					dependencies.marshalStageOnePathsFile = func(*runtimepaths.PathsFile) ([]byte, error) {
						return []byte(`{}`), nil
					}
					dependencies.makeStageOnePathsOutputDirectory = func(
						_ string,
						fileMode os.FileMode,
					) error {
						capturedDirectoryMode = fileMode
						return nil
					}
					dependencies.writeStageOnePathsJSON = func(_ string, _ []byte, fileMode os.FileMode) error {
						capturedFileMode = fileMode
						return nil
					}
				},
			)

			withLockedVorma(t, func(l *vormaruntime.LockedVorma) {
				if err := executor.writePathsToDiskStageOneWithRouteManifest(
					l,
					l.RouteManifestFile(),
				); err != nil {
					t.Fatalf(
						"writePathsToDiskStageOneWithRouteManifest returned error: %v",
						err,
					)
				}
			})

			if capturedDirectoryMode != buildArtifactDirectoryMode {
				t.Fatalf(
					"directory mode = %v, want %v",
					capturedDirectoryMode,
					buildArtifactDirectoryMode,
				)
			}
			if capturedFileMode != buildArtifactFileMode {
				t.Fatalf(
					"file mode = %v, want %v",
					capturedFileMode,
					buildArtifactFileMode,
				)
			}
		},
	)
}

func TestPathsOutputPath_StageOneFile(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	got := pathsOutputPath(app, runtimepaths.VormaPathsStageOneJSONFileName)
	want := filepath.Join(
		app.Wave.StaticPrivateOutDir(),
		runtimepaths.VormaInternalDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)
	if got != want {
		t.Fatalf("pathsOutputPath(stage-one) = %q, want %q", got, want)
	}
}

func TestStageOnePathsFile(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	var pathsFile *runtimepaths.PathsFile
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-stage-one")
		l.SetRouteManifestFile("manifest-stage-one.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/a": {
				OriginalPattern: "/a",
				SrcPath:         "frontend/src/routes/a.tsx",
				ExportKey:       "default",
			},
		})
		pathsFile = stageOnePathsFile(l, "manifest-stage-one.json")
	})

	if pathsFile == nil {
		t.Fatal("expected non-nil stage one paths file")
	}
	if pathsFile.Stage != "one" {
		t.Fatalf("stage = %q, want %q", pathsFile.Stage, "one")
	}
	if pathsFile.BuildID != "build-stage-one" {
		t.Fatalf("build ID = %q, want %q", pathsFile.BuildID, "build-stage-one")
	}
	if pathsFile.RouteManifestFile != "manifest-stage-one.json" {
		t.Fatalf(
			"route manifest file = %q, want %q",
			pathsFile.RouteManifestFile,
			"manifest-stage-one.json",
		)
	}
	if pathsFile.Paths["/a"] == nil {
		t.Fatal("expected /a path in stage one paths file")
	}
}

func TestWritePathsToDiskStageOneFromRuntimeState(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	runtimeStateSnapshot := buildlifecycle.RouteBuildRuntimeStateSnapshot{
		Paths: map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		},
		BuildID:           "runtime-snapshot-build-id",
		RouteManifestFile: "manifest-from-snapshot-state.json",
	}

	if err := writePathsToDiskStageOneFromRuntimeState(
		app,
		runtimeStateSnapshot,
		"manifest-written-this-build.json",
	); err != nil {
		t.Fatalf(
			"writePathsToDiskStageOneFromRuntimeState returned error: %v",
			err,
		)
	}

	outputPath := pathsOutputPath(
		app,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)
	rawJSON, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read stage-one output failed: %v", err)
	}

	var parsed runtimepaths.PathsFile
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("unmarshal stage-one output failed: %v", err)
	}

	if parsed.Stage != "one" {
		t.Fatalf("stage = %q, want %q", parsed.Stage, "one")
	}
	if parsed.BuildID != "runtime-snapshot-build-id" {
		t.Fatalf(
			"buildID = %q, want %q",
			parsed.BuildID,
			"runtime-snapshot-build-id",
		)
	}
	if parsed.RouteManifestFile != "manifest-written-this-build.json" {
		t.Fatalf(
			"routeManifestFile = %q, want %q",
			parsed.RouteManifestFile,
			"manifest-written-this-build.json",
		)
	}
	if _, ok := parsed.Paths["/"]; !ok {
		t.Fatalf(
			"expected root path in stage-one output, got %#v",
			parsed.Paths,
		)
	}
}
