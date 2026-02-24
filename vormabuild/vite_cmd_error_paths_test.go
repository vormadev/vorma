package vormabuild

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/lab/viteutil"
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

func TestPostViteProdBuild_ErrorWrapping(t *testing.T) {
	t.Run("wraps conversion error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		err := postViteProdBuild(app)
		if err == nil {
			t.Fatal("expected postViteProdBuild to return conversion error")
		}
		if !strings.Contains(err.Error(), "convert paths to stage two") {
			t.Fatalf("error = %q, expected conversion context", err)
		}
		if !strings.Contains(err.Error(), "read vite manifest") {
			t.Fatalf("error = %q, expected manifest-read context", err)
		}
	})

	t.Run("wraps stage-two write error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app
		app.WithLock(func(l *vormaruntime.LockedVorma) {
			l.SetBuildID("build-before-stage-two-write-failure")
			l.SetPaths(map[string]*vormaruntime.Path{
				"/": {
					OriginalPattern: "/",
					SrcPath:         "frontend/src/routes/root.tsx",
					ExportKey:       "default",
				},
			})
		})
		mustWriteJSONFile(t, app.Wave.ViteManifestLocation(), viteutil.Manifest{
			"frontend/src/vorma.entry.tsx": {
				Src:     "frontend/src/vorma.entry.tsx",
				File:    "assets/vorma_out/entry.js",
				IsEntry: true,
			},
			"frontend/src/routes/root.tsx": {
				Src:  "frontend/src/routes/root.tsx",
				File: "assets/vorma_out/root.js",
			},
		})

		expectedErr := errors.New("write stage-two failed")
		stageTwoWriteExecutor := newStageTwoPathsWriteExecutorForTest(
			func(dependencies *stageTwoPathsWriteDependencies) {
				dependencies.marshalStageTwoPathsFile = func(*runtimepaths.PathsFile) ([]byte, error) {
					return []byte(`{"stage":"two"}`), nil
				}
				dependencies.writeStageTwoPathsJSON = func(*vormaruntime.Vorma, []byte) error {
					return expectedErr
				}
			},
		)

		err := postViteProdBuildWithDependencies(
			app,
			postViteProdBuildDependencies{
				writePathsToDiskStageTwo: stageTwoWriteExecutor.writePathsToDiskStageTwo,
			},
		)
		if err == nil {
			t.Fatal(
				"expected postViteProdBuild to return stage-two write error",
			)
		}
		if !strings.Contains(err.Error(), "write stage-two paths") {
			t.Fatalf("error = %q, expected stage-two write context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped stage-two write error", err)
		}
		if app.BuildID() != "build-before-stage-two-write-failure" {
			t.Fatalf(
				"app build ID = %q, want unchanged build ID %q",
				app.BuildID(),
				"build-before-stage-two-write-failure",
			)
		}
	})
}

func TestWritePathsToDiskStageTwo_ErrorWrappingAndStepFlow(t *testing.T) {
	t.Run("wraps marshal error", func(t *testing.T) {
		expectedErr := errors.New("marshal failed")
		executor := newStageTwoPathsWriteExecutorForTest(
			func(dependencies *stageTwoPathsWriteDependencies) {
				dependencies.marshalStageTwoPathsFile = func(*runtimepaths.PathsFile) ([]byte, error) {
					return nil, expectedErr
				}
				dependencies.writeStageTwoPathsJSON = func(*vormaruntime.Vorma, []byte) error {
					t.Fatal("did not expect write stage after marshal error")
					return nil
				}
			},
		)

		err := executor.writePathsToDiskStageTwo(
			&vormaruntime.Vorma{},
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
		expectedErr := errors.New("write failed")
		executor := newStageTwoPathsWriteExecutorForTest(
			func(dependencies *stageTwoPathsWriteDependencies) {
				dependencies.marshalStageTwoPathsFile = func(*runtimepaths.PathsFile) ([]byte, error) {
					return []byte(`{"stage":"two"}`), nil
				}
				dependencies.writeStageTwoPathsJSON = func(*vormaruntime.Vorma, []byte) error {
					return expectedErr
				}
			},
		)

		err := executor.writePathsToDiskStageTwo(
			&vormaruntime.Vorma{},
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
				dependencies.writeStageTwoPathsJSON = func(_ *vormaruntime.Vorma, pathsJSON []byte) error {
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

		if err := executor.writePathsToDiskStageTwo(&vormaruntime.Vorma{}, pathsFile); err != nil {
			t.Fatalf("writePathsToDiskStageTwo returned error: %v", err)
		}
		if strings.Join(observedSteps, ",") != "marshal,write" {
			t.Fatalf(
				"observed step order = %#v, want [marshal write]",
				observedSteps,
			)
		}
	})
}

func TestToPathsFileStageTwo_ReturnsErrorWhenPublicOutDirMissing(t *testing.T) {
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
	mustWriteJSONFile(t, app.Wave.ViteManifestLocation(), manifest)

	if err := os.RemoveAll(fixture.publicDir); err != nil {
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
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

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

	mustWriteJSONFile(t, app.Wave.ViteManifestLocation(), viteutil.Manifest{
		"frontend/src/routes/root.tsx": {
			Src:  "frontend/src/routes/root.tsx",
			File: "assets/vorma_out/root.js",
		},
	})

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
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/orders/:id": {
				OriginalPattern: "/orders/:id",
				SrcPath:         "frontend/src/routes/orders.$id.tsx",
				ExportKey:       "default",
			},
		})
	})

	mustWriteJSONFile(t, app.Wave.ViteManifestLocation(), viteutil.Manifest{
		"frontend/src/vorma.entry.tsx": {
			Src:     "frontend/src/vorma.entry.tsx",
			File:    "assets/vorma_out/entry.js",
			IsEntry: true,
		},
	})

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
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

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
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

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
