package routeartifacts

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func newStageTwoPathsWriteExecutorForTest(
	mutateDependencies func(*stageTwoPathsWriteDependencies),
) stageTwoPathsWriteExecutor {
	dependencies := stageTwoPathsWriteDependencies{}
	if mutateDependencies != nil {
		mutateDependencies(&dependencies)
	}
	return newStageTwoPathsWriteExecutor(dependencies)
}

func newStageTwoBuildIDExecutorForTest(
	mutateDependencies func(*stageTwoBuildIDDependencies),
) stageTwoBuildIDExecutor {
	dependencies := stageTwoBuildIDDependencies{}
	if mutateDependencies != nil {
		mutateDependencies(&dependencies)
	}
	return newStageTwoBuildIDExecutor(dependencies)
}

func TestWritePathsToDiskStageTwo_ErrorWrappingAndStepFlow(t *testing.T) {
	t.Run("wraps marshal error", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		expectedErr := errors.New("marshal failed")
		executor := newStageTwoPathsWriteExecutorForTest(
			func(dependencies *stageTwoPathsWriteDependencies) {
				dependencies.marshalStageTwoPathsFile = func(*runtimepaths.PathsFile) ([]byte, error) {
					return nil, expectedErr
				}
				dependencies.writeStageTwoPathsJSON = func(string, []byte, os.FileMode) error {
					t.Fatal("did not expect write stage after marshal error")
					return nil
				}
			},
		)

		err := executor.writePathsToDiskStageTwo(
			app,
			&runtimepaths.PathsFile{Stage: "two"},
		)
		if err == nil {
			t.Fatal("expected writePathsToDiskStageTwo to return marshal error")
		}
		if !strings.Contains(err.Error(), "marshal stage-two paths file") {
			t.Fatalf("error = %q, expected marshal context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped marshal error", err)
		}
	})

	t.Run("wraps write error", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		expectedErr := errors.New("write failed")
		executor := newStageTwoPathsWriteExecutorForTest(
			func(dependencies *stageTwoPathsWriteDependencies) {
				dependencies.marshalStageTwoPathsFile = func(*runtimepaths.PathsFile) ([]byte, error) {
					return []byte(`{"stage":"two"}`), nil
				}
				dependencies.writeStageTwoPathsJSON = func(string, []byte, os.FileMode) error {
					return expectedErr
				}
			},
		)

		err := executor.writePathsToDiskStageTwo(
			app,
			&runtimepaths.PathsFile{Stage: "two"},
		)
		if err == nil {
			t.Fatal("expected writePathsToDiskStageTwo to return write error")
		}
		if !strings.Contains(err.Error(), "write stage-two paths JSON") {
			t.Fatalf("error = %q, expected write context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped write error", err)
		}
	})

	t.Run("runs marshal and write in order", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		var observedSteps []string
		pathsFile := &runtimepaths.PathsFile{Stage: "two"}
		executor := newStageTwoPathsWriteExecutorForTest(
			func(dependencies *stageTwoPathsWriteDependencies) {
				dependencies.marshalStageTwoPathsFile = func(gotPathsFile *runtimepaths.PathsFile) ([]byte, error) {
					observedSteps = append(observedSteps, "marshal")
					if gotPathsFile != pathsFile {
						t.Fatalf(
							"marshal received unexpected paths file pointer: %p vs %p",
							gotPathsFile,
							pathsFile,
						)
					}
					return []byte(`{"stage":"two"}`), nil
				}
				dependencies.writeStageTwoPathsJSON = func(
					_ string,
					pathsJSON []byte,
					_ os.FileMode,
				) error {
					observedSteps = append(observedSteps, "write")
					if string(pathsJSON) != `{"stage":"two"}` {
						t.Fatalf(
							"write received unexpected marshaled JSON: %q",
							string(pathsJSON),
						)
					}
					return nil
				}
			},
		)

		if err := executor.writePathsToDiskStageTwo(app, pathsFile); err != nil {
			t.Fatalf("writePathsToDiskStageTwo returned error: %v", err)
		}
		if strings.Join(observedSteps, ",") != "marshal,write" {
			t.Fatalf(
				"observed step order = %#v, want [marshal write]",
				observedSteps,
			)
		}
	})

	t.Run(
		"writes stage-two paths JSON with explicit directory and file modes",
		func(t *testing.T) {
			fixture := testkit.NewBuildTestFixture(t, nil)
			app := fixture.App

			var capturedDirectoryMode os.FileMode
			var capturedFileMode os.FileMode
			pathsFile := &runtimepaths.PathsFile{Stage: "two"}
			executor := newStageTwoPathsWriteExecutorForTest(
				func(dependencies *stageTwoPathsWriteDependencies) {
					dependencies.marshalStageTwoPathsFile = func(*runtimepaths.PathsFile) ([]byte, error) {
						return []byte(`{"stage":"two"}`), nil
					}
					dependencies.makeStageTwoPathsOutputDirectory = func(
						_ string,
						fileMode os.FileMode,
					) error {
						capturedDirectoryMode = fileMode
						return nil
					}
					dependencies.writeStageTwoPathsJSON = func(
						_ string,
						_ []byte,
						fileMode os.FileMode,
					) error {
						capturedFileMode = fileMode
						return nil
					}
				},
			)

			if err := executor.writePathsToDiskStageTwo(app, pathsFile); err != nil {
				t.Fatalf("writePathsToDiskStageTwo returned error: %v", err)
			}
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

func TestToPathsFileStageTwo_ReturnsErrorWhenPublicOutDirMissing(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

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
	testkit.MustWriteJSONFile(t, app.Wave.ViteManifestLocation(), manifest)

	if err := os.RemoveAll(fixture.PublicDir); err != nil {
		t.Fatalf("remove public out dir: %v", err)
	}

	_, err := toPathsFileStageTwo(app)
	if err == nil {
		t.Fatal(
			"expected toPathsFileStageTwo to fail when static public out dir is missing",
		)
	}
	if !strings.Contains(err.Error(), "get FS summary hash") {
		t.Fatalf("error = %q, expected FS summary hash context", err)
	}
}

func TestToPathsFileStageTwo_ReturnsManifestReadError(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	_, err := toPathsFileStageTwo(app)
	if err == nil {
		t.Fatal(
			"expected toPathsFileStageTwo to fail when Vite manifest is missing",
		)
	}
	if !strings.Contains(err.Error(), "read vite manifest") {
		t.Fatalf("error = %q, expected read-vite-manifest context", err)
	}
}

func TestToPathsFileStageTwo_ReturnsErrorWhenClientEntryChunkIsMissing(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/root.tsx",
				ExportKey:       "default",
			},
		})
	})

	testkit.MustWriteJSONFile(
		t,
		app.Wave.ViteManifestLocation(),
		viteutil.Manifest{
			"frontend/src/routes/root.tsx": {
				Src:  "frontend/src/routes/root.tsx",
				File: "assets/vorma_out/root.js",
			},
		},
	)

	_, err := toPathsFileStageTwo(app)
	if err == nil {
		t.Fatal(
			"expected toPathsFileStageTwo to fail when client entry chunk is missing",
		)
	}
	if !strings.Contains(err.Error(), "client entry") {
		t.Fatalf("error = %q, expected missing-client-entry context", err)
	}
}

func TestToPathsFileStageTwo_ReturnsErrorWhenRouteChunkIsMissing(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/orders/:id": {
				OriginalPattern: "/orders/:id",
				SrcPath:         "frontend/src/routes/orders.$id.tsx",
				ExportKey:       "default",
			},
		})
	})

	testkit.MustWriteJSONFile(
		t,
		app.Wave.ViteManifestLocation(),
		viteutil.Manifest{
			"frontend/src/vorma.entry.tsx": {
				Src:     "frontend/src/vorma.entry.tsx",
				File:    "assets/vorma_out/entry.js",
				IsEntry: true,
			},
		},
	)

	_, err := toPathsFileStageTwo(app)
	if err == nil {
		t.Fatal(
			"expected toPathsFileStageTwo to fail when a route chunk is missing from manifest",
		)
	}
	if !strings.Contains(err.Error(), "route chunk") {
		t.Fatalf("error = %q, expected missing-route-chunk context", err)
	}
}

func TestComputeStageTwoBuildID_ErrorWrapping(t *testing.T) {
	t.Run("wraps marshal paths file error", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		expectedErr := errors.New("marshal failed")
		stageTwoBuildIDExecutor := newStageTwoBuildIDExecutorForTest(
			func(dependencies *stageTwoBuildIDDependencies) {
				dependencies.readHTMLTemplate = func(path string) ([]byte, error) {
					return []byte("<html></html>"), nil
				}
				dependencies.marshalPathsFile = func(any) ([]byte, error) {
					return nil, expectedErr
				}
				dependencies.summarizePublicFS = func(fs.FS) ([]byte, error) {
					t.Fatal(
						"did not expect FS summary step after marshal error",
					)
					return nil, nil
				}
			},
		)

		_, err := stageTwoBuildIDExecutor.computeStageTwoBuildID(
			app,
			&runtimepaths.PathsFile{},
		)
		if err == nil {
			t.Fatal("expected computeStageTwoBuildID to return marshal error")
		}
		if !strings.Contains(err.Error(), "marshal paths file") {
			t.Fatalf("error = %q, expected marshal-paths-file context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped marshal error", err)
		}
	})

	t.Run("wraps FS summary error", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		expectedErr := errors.New("summary failed")
		stageTwoBuildIDExecutor := newStageTwoBuildIDExecutorForTest(
			func(dependencies *stageTwoBuildIDDependencies) {
				dependencies.readHTMLTemplate = func(path string) ([]byte, error) {
					return []byte("<html></html>"), nil
				}
				dependencies.marshalPathsFile = func(any) ([]byte, error) {
					return []byte(`{"stage":"two"}`), nil
				}
				dependencies.summarizePublicFS = func(fs.FS) ([]byte, error) {
					return nil, expectedErr
				}
			},
		)

		_, err := stageTwoBuildIDExecutor.computeStageTwoBuildID(
			app,
			&runtimepaths.PathsFile{},
		)
		if err == nil {
			t.Fatal(
				"expected computeStageTwoBuildID to return FS summary error",
			)
		}
		if !strings.Contains(err.Error(), "get FS summary hash") {
			t.Fatalf("error = %q, expected FS-summary context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped FS summary error", err)
		}
	})
}
