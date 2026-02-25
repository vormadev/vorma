package routeartifacts

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/vormabuild/artifactio"
	"github.com/vormadev/vorma/vormabuild/buildlifecycle"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func newRouteRegistryBuildExecutorForTest(
	mutateDependencies func(*routeRegistryBuildDependencies),
) routeRegistryBuildExecutor {
	dependencies := routeRegistryBuildDependencies{}
	if mutateDependencies != nil {
		mutateDependencies(&dependencies)
	}
	return newRouteRegistryBuildExecutor(dependencies)
}

func TestWriteRouteManifestToDisk_WrapsMarshalError(t *testing.T) {
	expectedErr := errors.New("marshal failed")
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.marshalRouteManifestJSON = func(any) ([]byte, error) {
				return nil, expectedErr
			}
			dependencies.writeRouteManifestJSON = func(string, []byte, os.FileMode) error {
				t.Fatal(
					"did not expect route manifest file write when marshal fails",
				)
				return nil
			}
		},
	)

	fixture := testkit.NewBuildTestFixture(t, nil)
	_, err := routeRegistryBuildExecutor.writeRouteManifestToDisk(
		fixture.App,
		map[string]int{
			"/": 1,
		},
	)
	if err == nil {
		t.Fatal("expected writeRouteManifestToDisk to return marshal error")
	}
	if !strings.Contains(err.Error(), "marshal route manifest") {
		t.Fatalf("error = %q, expected marshal context", err)
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("error = %v, expected wrapped marshal error", err)
	}
}

func TestWriteRouteManifestToDisk_ReturnsWriteErrorWhenPublicOutDirIsFile(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	if err := os.RemoveAll(fixture.PublicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}
	testkit.MustWriteFile(t, fixture.PublicDir, []byte("not a directory"))

	_, err := writeRouteManifestToDisk(app, map[string]int{
		"/": 1,
	})
	if err == nil {
		t.Fatal("expected writeRouteManifestToDisk to return an error")
	}
	if !strings.Contains(err.Error(), "write route manifest") {
		t.Fatalf("error = %q, expected write-route-manifest context", err)
	}
}

func TestWriteRouteManifestToDisk_UsesBuildArtifactFileMode(t *testing.T) {
	var capturedFileMode os.FileMode
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.marshalRouteManifestJSON = func(any) ([]byte, error) {
				return []byte(`{"/":1}`), nil
			}
			dependencies.writeRouteManifestJSON = func(
				_ string,
				_ []byte,
				fileMode os.FileMode,
			) error {
				capturedFileMode = fileMode
				return nil
			}
		},
	)

	fixture := testkit.NewBuildTestFixture(t, nil)
	if _, err := routeRegistryBuildExecutor.writeRouteManifestToDisk(
		fixture.App,
		map[string]int{"/": 1},
	); err != nil {
		t.Fatalf("writeRouteManifestToDisk returned error: %v", err)
	}

	if capturedFileMode != os.FileMode(buildArtifactFileMode) {
		t.Fatalf(
			"file mode = %v, want %v",
			capturedFileMode,
			buildArtifactFileMode,
		)
	}
}

func TestGenerateAndWriteRouteManifest(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/has-loader",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)
	nestedmux.AddPatternWithoutHandler(
		app.LoadersRouter().NestedRouter,
		"/client-only",
	)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/has-loader": {
				OriginalPattern: "/has-loader",
				SrcPath:         "frontend/src/routes/has-loader.tsx",
				ExportKey:       "default",
			},
			"/client-only": {
				OriginalPattern: "/client-only",
				SrcPath:         "frontend/src/routes/client-only.tsx",
				ExportKey:       "default",
			},
		})
	})

	var manifest map[string]int
	app.WithRLock(func(l *vormaruntime.ReadLockedVorma) {
		manifest = generateRouteManifest(l, app.LoadersRouter().NestedRouter)
	})

	if got := manifest["/has-loader"]; got != 1 {
		t.Fatalf("manifest[/has-loader] = %d, want 1", got)
	}
	if got := manifest["/client-only"]; got != 0 {
		t.Fatalf("manifest[/client-only] = %d, want 0", got)
	}

	filename, err := writeRouteManifestToDisk(app, manifest)
	if err != nil {
		t.Fatalf("writeRouteManifestToDisk returned error: %v", err)
	}
	if !strings.HasPrefix(filename, vormaruntime.VormaRouteManifestPrefix) {
		t.Fatalf(
			"filename = %q, expected prefix %q",
			filename,
			vormaruntime.VormaRouteManifestPrefix,
		)
	}
	if filepath.Ext(filename) != ".json" {
		t.Fatalf("filename = %q, expected .json extension", filename)
	}

	writtenPath := filepath.Join(fixture.PublicDir, filename)
	bytes, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("read written manifest: %v", err)
	}

	var decoded map[string]int
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("unmarshal manifest JSON: %v", err)
	}
	if decoded["/has-loader"] != 1 || decoded["/client-only"] != 0 {
		t.Fatalf(
			"decoded manifest = %#v, expected loader/client flags",
			decoded,
		)
	}
}

func TestWriteAndSetRouteManifest(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	nestedmux.AddTaskHandler(
		app.LoadersRouter().NestedRouter,
		"/server",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/server": {
				OriginalPattern: "/server",
				SrcPath:         "frontend/src/routes/server.tsx",
				ExportKey:       "default",
			},
		})

		if err := writeAndSetRouteManifest(l); err != nil {
			t.Fatalf("writeAndSetRouteManifest returned error: %v", err)
		}
	})

	manifestFile := app.RouteManifestFile()
	if manifestFile == "" {
		t.Fatal("expected route manifest filename to be set")
	}
	if !strings.HasPrefix(manifestFile, vormaruntime.VormaRouteManifestPrefix) {
		t.Fatalf(
			"route manifest file = %q, expected prefix %q",
			manifestFile,
			vormaruntime.VormaRouteManifestPrefix,
		)
	}
	if _, err := os.Stat(filepath.Join(fixture.PublicDir, manifestFile)); err != nil {
		t.Fatalf("expected manifest file to exist on disk: %v", err)
	}
}

func TestRouteManifestFilename(t *testing.T) {
	manifestJSON := []byte(`{"a":1}`)
	filename := routeManifestFilename(manifestJSON)
	if !strings.HasPrefix(filename, vormaruntime.VormaRouteManifestPrefix) {
		t.Fatalf(
			"filename = %q, expected prefix %q",
			filename,
			vormaruntime.VormaRouteManifestPrefix,
		)
	}
	if filepath.Ext(filename) != ".json" {
		t.Fatalf("filename = %q, expected .json extension", filename)
	}

	filenameAgain := routeManifestFilename(manifestJSON)
	if filenameAgain != filename {
		t.Fatalf(
			"routeManifestFilename should be deterministic: %q vs %q",
			filename,
			filenameAgain,
		)
	}
}

func TestRouteManifestServerLoaderFlag(t *testing.T) {
	nestedRouter := nestedmux.NewRouter(nil)
	nestedmux.AddPatternWithoutHandler(nestedRouter, "/client-only")
	nestedmux.AddTaskHandler(
		nestedRouter,
		"/with-loader",
		mux.TaskHandlerFromFunc(func(_ *mux.ReqData[mux.None]) (string, error) {
			return "ok", nil
		}),
	)

	if got := routeManifestServerLoaderFlag(nestedRouter, "/client-only"); got != 0 {
		t.Fatalf(
			"routeManifestServerLoaderFlag(/client-only) = %d, want 0",
			got,
		)
	}
	if got := routeManifestServerLoaderFlag(nestedRouter, "/with-loader"); got != 1 {
		t.Fatalf(
			"routeManifestServerLoaderFlag(/with-loader) = %d, want 1",
			got,
		)
	}
}

func TestWriteRouteArtifacts_ReturnsWrappedErrorForEachArtifactStep(
	t *testing.T,
) {
	run := func(
		t *testing.T,
		name string,
		prepareFixtureForError func(*testkit.BuildTestFixture),
		expectedErrContext string,
	) {
		t.Run(name, func(t *testing.T) {
			fixture := testkit.NewBuildTestFixture(t, nil)
			app := fixture.App
			t.Chdir(fixture.RootDir)

			prepareFixtureForError(fixture)

			var gotErr error
			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetPaths(map[string]*vormaruntime.Path{
					"/": {
						OriginalPattern: "/",
						SrcPath:         "frontend/src/routes/home.tsx",
						ExportKey:       "default",
					},
				})
				gotErr = writeRouteArtifacts(l)
			})

			if gotErr == nil {
				t.Fatal("expected writeRouteArtifacts to return an error")
			}
			if !strings.Contains(gotErr.Error(), expectedErrContext) {
				t.Fatalf(
					"error = %q, expected context %q",
					gotErr,
					expectedErrContext,
				)
			}
		})
	}

	run(
		t,
		"manifest write failure",
		func(fixture *testkit.BuildTestFixture) {
			if err := os.RemoveAll(fixture.PublicDir); err != nil {
				t.Fatalf("remove public dir: %v", err)
			}
			testkit.MustWriteFile(
				t,
				fixture.PublicDir,
				[]byte("not a directory"),
			)
		},
		"write route manifest",
	)

	run(
		t,
		"paths json write failure",
		func(fixture *testkit.BuildTestFixture) {
			if err := os.RemoveAll(fixture.PrivateDir); err != nil {
				t.Fatalf("remove private dir: %v", err)
			}
			testkit.MustWriteFile(
				t,
				fixture.PrivateDir,
				[]byte("not a directory"),
			)
		},
		"write paths JSON",
	)

	run(
		t,
		"generated typescript write failure",
		func(fixture *testkit.BuildTestFixture) {
			testkit.MustWriteFile(
				t,
				fixture.App.Config.TSGenOutDir,
				[]byte("file blocks tsgen out dir"),
			)
		},
		"write generated TypeScript",
	)
}

func TestWriteRouteArtifacts_RestoresArtifactsThenRePanicsWhenGeneratedTypeScriptPanics(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	previousStageOnePathsPath := stageOnePathsArtifactOutputPath(app)
	previousStageOnePathsContent := []byte("previous-stage-one-paths-content")
	testkit.MustWriteFile(
		t,
		previousStageOnePathsPath,
		previousStageOnePathsContent,
	)

	expectedPanic := errors.New("generated ts panic")
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.writeGeneratedTypeScript = func(*vormaruntime.LockedVorma) error {
				panic(expectedPanic)
			}
		},
	)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetRouteManifestFile("manifest-before-generated-ts-panic.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
	})

	defer func() {
		recoveredPanicValue := recover()
		if recoveredPanicValue == nil {
			t.Fatal("expected writeRouteArtifacts to panic")
		}
		recoveredPanicErr, ok := recoveredPanicValue.(error)
		if !ok {
			t.Fatalf(
				"recovered panic type = %T, want error",
				recoveredPanicValue,
			)
		}
		if !errors.Is(recoveredPanicErr, expectedPanic) {
			t.Fatalf(
				"recovered panic = %v, want %v",
				recoveredPanicErr,
				expectedPanic,
			)
		}

		manifestFiles := listGeneratedRouteManifestFiles(t, fixture.PublicDir)
		if len(manifestFiles) != 0 {
			t.Fatalf(
				"expected no generated route manifest artifacts after panic cleanup, got %#v",
				manifestFiles,
			)
		}

		restoredStageOnePathsContent, err := os.ReadFile(
			previousStageOnePathsPath,
		)
		if err != nil {
			t.Fatalf("read restored stage-one paths artifact: %v", err)
		}
		if !bytes.Equal(
			restoredStageOnePathsContent,
			previousStageOnePathsContent,
		) {
			t.Fatalf(
				"restored stage-one paths artifact = %q, want previous content %q",
				string(restoredStageOnePathsContent),
				string(previousStageOnePathsContent),
			)
		}

		if got := app.RouteManifestFile(); got != "manifest-before-generated-ts-panic.json" {
			t.Fatalf(
				"route manifest file after panic = %q, want unchanged %q",
				got,
				"manifest-before-generated-ts-panic.json",
			)
		}
	}()

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		_ = routeRegistryBuildExecutor.writeRouteArtifacts(l)
	})
}

func TestWriteRouteArtifacts_RePanicsWhenCleanupAfterPanicFails(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	// Force the panic-time cleanup path to attempt removal (non-existent snapshot).
	stageOnePathsPath := stageOnePathsArtifactOutputPath(app)
	_ = os.Remove(stageOnePathsPath)

	expectedPanic := errors.New("generated ts panic")
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.writeGeneratedTypeScript = func(*vormaruntime.LockedVorma) error {
				panic(expectedPanic)
			}
			dependencies.removeStageOnePathsArtifact = func(string) error {
				return errors.New("cleanup stage-one remove failed")
			}
		},
	)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
	})

	defer func() {
		recoveredPanicValue := recover()
		if recoveredPanicValue == nil {
			t.Fatal("expected writeRouteArtifacts to panic")
		}
		recoveredPanicErr, ok := recoveredPanicValue.(error)
		if !ok {
			t.Fatalf(
				"recovered panic type = %T, want error",
				recoveredPanicValue,
			)
		}
		if !errors.Is(recoveredPanicErr, expectedPanic) {
			t.Fatalf(
				"recovered panic = %v, want %v",
				recoveredPanicErr,
				expectedPanic,
			)
		}
	}()

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		_ = routeRegistryBuildExecutor.writeRouteArtifacts(l)
	})
}

func TestWriteRouteArtifacts_OnlyCommitsRouteManifestFileOnSuccess(
	t *testing.T,
) {
	runFailure := func(
		t *testing.T,
		name string,
		prepareFixtureForError func(*testkit.BuildTestFixture),
		expectedErrContext string,
	) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			fixture := testkit.NewBuildTestFixture(t, nil)
			app := fixture.App
			t.Chdir(fixture.RootDir)

			prepareFixtureForError(fixture)

			var gotErr error
			var routeManifestFileAfterWriteAttempt string
			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetRouteManifestFile("manifest-before-write-attempt.json")
				l.SetPaths(map[string]*vormaruntime.Path{
					"/": {
						OriginalPattern: "/",
						SrcPath:         "frontend/src/routes/home.tsx",
						ExportKey:       "default",
					},
				})
				gotErr = writeRouteArtifacts(l)
				routeManifestFileAfterWriteAttempt = l.RouteManifestFile()
			})

			if gotErr == nil {
				t.Fatal("expected writeRouteArtifacts to return an error")
			}
			if !strings.Contains(gotErr.Error(), expectedErrContext) {
				t.Fatalf(
					"error = %q, expected context %q",
					gotErr,
					expectedErrContext,
				)
			}
			if routeManifestFileAfterWriteAttempt != "manifest-before-write-attempt.json" {
				t.Fatalf(
					"routeManifestFile = %q, want unchanged %q after failed write",
					routeManifestFileAfterWriteAttempt,
					"manifest-before-write-attempt.json",
				)
			}
		})
	}

	runFailure(
		t,
		"does not commit route manifest file when stage-one paths write fails",
		func(fixture *testkit.BuildTestFixture) {
			if err := os.RemoveAll(fixture.PrivateDir); err != nil {
				t.Fatalf("remove private dir: %v", err)
			}
			testkit.MustWriteFile(
				t,
				fixture.PrivateDir,
				[]byte("not a directory"),
			)
		},
		"write paths JSON",
	)

	runFailure(
		t,
		"does not commit route manifest file when generated TypeScript write fails",
		func(fixture *testkit.BuildTestFixture) {
			testkit.MustWriteFile(
				t,
				fixture.App.Config.TSGenOutDir,
				[]byte("file blocks tsgen out dir"),
			)
		},
		"write generated TypeScript",
	)

	t.Run(
		"commits new route manifest file after successful artifact writes",
		func(t *testing.T) {
			fixture := testkit.NewBuildTestFixture(t, nil)
			app := fixture.App
			t.Chdir(fixture.RootDir)

			var routeManifestFileAfterSuccess string
			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetRouteManifestFile("manifest-before-success.json")
				l.SetPaths(map[string]*vormaruntime.Path{
					"/": {
						OriginalPattern: "/",
						SrcPath:         "frontend/src/routes/home.tsx",
						ExportKey:       "default",
					},
				})

				if err := writeRouteArtifacts(l); err != nil {
					t.Fatalf("writeRouteArtifacts returned error: %v", err)
				}

				routeManifestFileAfterSuccess = l.RouteManifestFile()
			})

			if routeManifestFileAfterSuccess == "" {
				t.Fatal(
					"expected non-empty route manifest file after successful write",
				)
			}
			if routeManifestFileAfterSuccess == "manifest-before-success.json" {
				t.Fatalf(
					"expected route manifest file to change after successful write, got %q",
					routeManifestFileAfterSuccess,
				)
			}
			if _, err := os.Stat(filepath.Join(fixture.PublicDir, routeManifestFileAfterSuccess)); err != nil {
				t.Fatalf("expected written route manifest on disk: %v", err)
			}
		},
	)
}

func TestWriteAndSetRouteManifest_DoesNotMutateStateWhenWriteFails(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	if err := os.RemoveAll(fixture.PublicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}
	testkit.MustWriteFile(t, fixture.PublicDir, []byte("not a directory"))

	var gotErr error
	var routeManifestFileAfterFailure string
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetRouteManifestFile("manifest-before-failure.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})

		gotErr = writeAndSetRouteManifest(l)
		routeManifestFileAfterFailure = l.RouteManifestFile()
	})

	if gotErr == nil {
		t.Fatal("expected writeAndSetRouteManifest to return an error")
	}
	if routeManifestFileAfterFailure != "manifest-before-failure.json" {
		t.Fatalf(
			"routeManifestFile = %q, want unchanged %q",
			routeManifestFileAfterFailure,
			"manifest-before-failure.json",
		)
	}
}

func TestWriteRouteArtifacts_CleansUpRouteManifestArtifactAfterDownstreamFailure(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	if err := os.RemoveAll(fixture.PrivateDir); err != nil {
		t.Fatalf("remove private dir: %v", err)
	}
	testkit.MustWriteFile(t, fixture.PrivateDir, []byte("not a directory"))

	var gotErr error
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetRouteManifestFile("manifest-before-failure.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
		gotErr = writeRouteArtifacts(l)
	})
	if gotErr == nil {
		t.Fatal("expected writeRouteArtifacts to return an error")
	}
	if !strings.Contains(gotErr.Error(), "write paths JSON") {
		t.Fatalf("error = %q, expected stage-one write context", gotErr)
	}

	manifestFiles := listGeneratedRouteManifestFiles(t, fixture.PublicDir)
	if len(manifestFiles) != 0 {
		t.Fatalf(
			"expected no generated route manifest artifacts after failed downstream write, got %#v",
			manifestFiles,
		)
	}
}

func TestWriteRouteArtifacts_PreservesCommittedRouteManifestWhenFilenameIsUnchanged(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	manifestJSON, err := json.Marshal(map[string]int{
		"/": 0,
	})
	if err != nil {
		t.Fatalf("marshal expected manifest JSON: %v", err)
	}
	committedManifestFile := routeManifestFilename(manifestJSON)
	committedManifestPath := filepath.Join(
		fixture.PublicDir,
		committedManifestFile,
	)
	testkit.MustWriteFile(t, committedManifestPath, manifestJSON)

	if err := os.RemoveAll(fixture.PrivateDir); err != nil {
		t.Fatalf("remove private dir: %v", err)
	}
	testkit.MustWriteFile(t, fixture.PrivateDir, []byte("not a directory"))

	var gotErr error
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetRouteManifestFile(committedManifestFile)
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
		gotErr = writeRouteArtifacts(l)
	})
	if gotErr == nil {
		t.Fatal("expected writeRouteArtifacts to return an error")
	}
	if _, statErr := os.Stat(committedManifestPath); statErr != nil {
		t.Fatalf(
			"expected committed manifest file to remain after failed downstream write, stat err=%v",
			statErr,
		)
	}
}

func TestWriteRouteArtifacts_JoinsManifestCleanupErrorWithDownstreamWriteError(
	t *testing.T,
) {
	cleanupErr := errors.New("cleanup failed")
	cleanupCalled := false
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.removeRouteManifestJSON = func(string) error {
				cleanupCalled = true
				return cleanupErr
			}
		},
	)

	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	if err := os.RemoveAll(fixture.PrivateDir); err != nil {
		t.Fatalf("remove private dir: %v", err)
	}
	testkit.MustWriteFile(t, fixture.PrivateDir, []byte("not a directory"))

	var gotErr error
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetRouteManifestFile("manifest-before-failure.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
		gotErr = routeRegistryBuildExecutor.writeRouteArtifacts(l)
	})

	if gotErr == nil {
		t.Fatal("expected writeRouteArtifacts to return an error")
	}
	if !cleanupCalled {
		t.Fatal("expected route manifest cleanup step to be called")
	}
	if !strings.Contains(gotErr.Error(), "write paths JSON") {
		t.Fatalf("error = %q, expected stage-one write context", gotErr)
	}
	if !strings.Contains(gotErr.Error(), "cleanup route manifest artifact") {
		t.Fatalf("error = %q, expected cleanup context", gotErr)
	}
	if !errors.Is(gotErr, cleanupErr) {
		t.Fatalf(
			"error = %v, expected cleanup error in joined error chain",
			gotErr,
		)
	}
}

func TestWriteRouteArtifacts_RestoresStageOnePathsArtifactAfterGeneratedTypeScriptFailure(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	stageOnePathsPath := stageOnePathsArtifactOutputPath(app)
	previousStageOnePathsContent := []byte(
		`{"stage":"one","buildID":"previous"}`,
	)
	testkit.MustWriteFile(t, stageOnePathsPath, previousStageOnePathsContent)

	testkit.MustWriteFile(
		t,
		app.Config.TSGenOutDir,
		[]byte("file blocks tsgen out dir"),
	)

	var gotErr error
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetRouteManifestFile("manifest-before-failure.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
		gotErr = writeRouteArtifacts(l)
	})
	if gotErr == nil {
		t.Fatal("expected writeRouteArtifacts to return an error")
	}
	if !strings.Contains(gotErr.Error(), "write generated TypeScript") {
		t.Fatalf(
			"error = %q, expected generated TypeScript write context",
			gotErr,
		)
	}

	restoredStageOnePathsContent, err := os.ReadFile(stageOnePathsPath)
	if err != nil {
		t.Fatalf("read restored stage-one paths artifact: %v", err)
	}
	if !bytes.Equal(
		restoredStageOnePathsContent,
		previousStageOnePathsContent,
	) {
		t.Fatalf(
			"restored stage-one paths artifact = %q, want previous content %q",
			string(restoredStageOnePathsContent),
			string(previousStageOnePathsContent),
		)
	}
}

func TestWriteRouteArtifacts_RemovesStageOnePathsArtifactWhenNoPreviousState(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	stageOnePathsPath := stageOnePathsArtifactOutputPath(app)
	if err := os.Remove(stageOnePathsPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove stage-one paths artifact: %v", err)
	}

	testkit.MustWriteFile(
		t,
		app.Config.TSGenOutDir,
		[]byte("file blocks tsgen out dir"),
	)

	var gotErr error
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetRouteManifestFile("manifest-before-failure.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
		gotErr = writeRouteArtifacts(l)
	})
	if gotErr == nil {
		t.Fatal("expected writeRouteArtifacts to return an error")
	}

	if _, err := os.Stat(stageOnePathsPath); !os.IsNotExist(err) {
		t.Fatalf(
			"expected no stage-one paths artifact after cleanup, stat err=%v",
			err,
		)
	}
}

func TestWriteRouteArtifacts_JoinsStageOnePathsCleanupErrorWithDownstreamWriteError(
	t *testing.T,
) {
	cleanupErr := errors.New("restore stage-one paths failed")
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.writeStageOnePathsArtifact = func(string, []byte, os.FileMode) error {
				return cleanupErr
			}
		},
	)

	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	stageOnePathsPath := stageOnePathsArtifactOutputPath(app)
	testkit.MustWriteFile(
		t,
		stageOnePathsPath,
		[]byte(`{"stage":"one","buildID":"previous"}`),
	)

	testkit.MustWriteFile(
		t,
		app.Config.TSGenOutDir,
		[]byte("file blocks tsgen out dir"),
	)

	var gotErr error
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetRouteManifestFile("manifest-before-failure.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
		gotErr = routeRegistryBuildExecutor.writeRouteArtifacts(l)
	})

	if gotErr == nil {
		t.Fatal("expected writeRouteArtifacts to return an error")
	}
	if !strings.Contains(gotErr.Error(), "write generated TypeScript") {
		t.Fatalf(
			"error = %q, expected generated TypeScript write context",
			gotErr,
		)
	}
	if !strings.Contains(gotErr.Error(), "cleanup stage-one paths artifact") {
		t.Fatalf("error = %q, expected stage-one cleanup context", gotErr)
	}
	if !errors.Is(gotErr, cleanupErr) {
		t.Fatalf(
			"error = %v, expected cleanup error in joined error chain",
			gotErr,
		)
	}
}

func TestShouldRemoveRouteManifestArtifactAfterWriteFailure(t *testing.T) {
	tests := []struct {
		name                      string
		manifestFile              string
		previousRouteManifestFile string
		want                      bool
	}{
		{
			name:                      "removes new manifest artifact",
			manifestFile:              "vorma_route_manifest_new.json",
			previousRouteManifestFile: "vorma_route_manifest_old.json",
			want:                      true,
		},
		{
			name:                      "skips removal when manifest file is unchanged",
			manifestFile:              "vorma_route_manifest_same.json",
			previousRouteManifestFile: "vorma_route_manifest_same.json",
			want:                      false,
		},
		{
			name:                      "skips removal when manifest file is empty",
			manifestFile:              "",
			previousRouteManifestFile: "vorma_route_manifest_old.json",
			want:                      false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := shouldRemoveRouteManifestArtifactAfterWriteFailure(
				testCase.manifestFile,
				testCase.previousRouteManifestFile,
			)
			if got != testCase.want {
				t.Fatalf(
					"shouldRemoveRouteManifestArtifactAfterWriteFailure(%q, %q) = %v, want %v",
					testCase.manifestFile,
					testCase.previousRouteManifestFile,
					got,
					testCase.want,
				)
			}
		})
	}
}

func listGeneratedRouteManifestFiles(
	t *testing.T,
	staticPublicOutDir string,
) []string {
	t.Helper()

	directoryEntries, err := os.ReadDir(staticPublicOutDir)
	if err != nil {
		t.Fatalf("read generated route manifest directory: %v", err)
	}

	manifestFiles := make([]string, 0, len(directoryEntries))
	for _, directoryEntry := range directoryEntries {
		if directoryEntry.IsDir() {
			continue
		}
		fileName := directoryEntry.Name()
		if strings.HasPrefix(fileName, vormaruntime.VormaRouteManifestPrefix) {
			manifestFiles = append(manifestFiles, fileName)
		}
	}
	return manifestFiles
}

func TestWriteRouteArtifacts_ReturnsSnapshotErrorWhenStageOneSnapshotReadFails(
	t *testing.T,
) {
	snapshotErr := errors.New("snapshot read failed")
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.readStageOnePathsArtifact = func(string) ([]byte, error) {
				return nil, snapshotErr
			}
		},
	)

	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	var gotErr error
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
		gotErr = routeRegistryBuildExecutor.writeRouteArtifacts(l)
	})

	if gotErr == nil {
		t.Fatal(
			"expected writeRouteArtifacts to return stage-one snapshot error",
		)
	}
	if !strings.Contains(gotErr.Error(), "snapshot stage-one paths artifact") {
		t.Fatalf("error = %q, expected stage-one snapshot context", gotErr)
	}
	if !errors.Is(gotErr, snapshotErr) {
		t.Fatalf(
			"error = %v, expected wrapped stage-one snapshot error",
			gotErr,
		)
	}
}

func TestCaptureStageOnePathsArtifact(t *testing.T) {
	t.Run(
		"returns non-existent snapshot for ENOTDIR errors",
		func(t *testing.T) {
			routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
				func(dependencies *routeRegistryBuildDependencies) {
					dependencies.readStageOnePathsArtifact = func(string) ([]byte, error) {
						return nil, syscall.ENOTDIR
					}
				},
			)

			snapshot, err := routeRegistryBuildExecutor.captureStageOnePathsArtifact(
				"ignored",
			)
			if err != nil {
				t.Fatalf("captureStageOnePathsArtifact returned error: %v", err)
			}
			if snapshot.Existed() {
				t.Fatalf(
					"snapshot.Existed() = %v, want false for ENOTDIR",
					snapshot.Existed(),
				)
			}
			if snapshot.Content() != nil {
				t.Fatalf(
					"snapshot.Content() = %#v, want nil for ENOTDIR",
					snapshot.Content(),
				)
			}
		},
	)

	t.Run("returns error for non-ENOENT/ENOTDIR failures", func(t *testing.T) {
		readErr := errors.New("read failed")
		routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
			func(dependencies *routeRegistryBuildDependencies) {
				dependencies.readStageOnePathsArtifact = func(string) ([]byte, error) {
					return nil, readErr
				}
			},
		)

		_, err := routeRegistryBuildExecutor.captureStageOnePathsArtifact(
			"ignored",
		)
		if err == nil {
			t.Fatal("expected captureStageOnePathsArtifact to return error")
		}
		if !errors.Is(err, readErr) {
			t.Fatalf("error = %v, expected wrapped read error", err)
		}
	})
}

func TestRestoreStageOnePathsArtifactFromState(t *testing.T) {
	t.Run(
		"restores existing snapshot using stage-one writer and build artifact mode",
		func(t *testing.T) {
			var writeCalled bool
			routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
				func(dependencies *routeRegistryBuildDependencies) {
					dependencies.writeStageOnePathsArtifact = func(
						path string,
						content []byte,
						fileMode os.FileMode,
					) error {
						writeCalled = true
						if path != "stage-one-path.json" {
							t.Fatalf(
								"restore write path = %q, want %q",
								path,
								"stage-one-path.json",
							)
						}
						if string(content) != "snapshot-content" {
							t.Fatalf(
								"restore write content = %q, want %q",
								string(content),
								"snapshot-content",
							)
						}
						if fileMode != os.FileMode(buildArtifactFileMode) {
							t.Fatalf(
								"restore write mode = %v, want %v",
								fileMode,
								buildArtifactFileMode,
							)
						}
						return nil
					}
					dependencies.removeStageOnePathsArtifact = func(string) error {
						t.Fatal("did not expect removal when snapshot exists")
						return nil
					}
				},
			)

			snapshot, err := artifactio.CaptureBuildArtifactFile(
				"stage-one-path.json",
				func(string) ([]byte, error) {
					return []byte("snapshot-content"), nil
				},
			)
			if err != nil {
				t.Fatalf("capture snapshot: %v", err)
			}

			err = routeRegistryBuildExecutor.restoreStageOnePathsArtifactFromState(
				"stage-one-path.json",
				snapshot,
			)
			if err != nil {
				t.Fatalf(
					"restoreStageOnePathsArtifactFromState returned error: %v",
					err,
				)
			}
			if !writeCalled {
				t.Fatal(
					"expected stage-one snapshot restore write to be called",
				)
			}
		},
	)

	t.Run(
		"ignores ENOTDIR when removing non-existent snapshot artifact",
		func(t *testing.T) {
			routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
				func(dependencies *routeRegistryBuildDependencies) {
					dependencies.removeStageOnePathsArtifact = func(string) error {
						return syscall.ENOTDIR
					}
				},
			)

			if err := routeRegistryBuildExecutor.restoreStageOnePathsArtifactFromState(
				"ignored",
				stageOnePathsArtifactSnapshot{},
			); err != nil {
				t.Fatalf(
					"restoreStageOnePathsArtifactFromState returned error: %v",
					err,
				)
			}
		},
	)

	t.Run(
		"returns removal error for non-ENOENT/ENOTDIR failures",
		func(t *testing.T) {
			removeErr := errors.New("remove failed")
			routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
				func(dependencies *routeRegistryBuildDependencies) {
					dependencies.removeStageOnePathsArtifact = func(string) error {
						return removeErr
					}
				},
			)

			err := routeRegistryBuildExecutor.restoreStageOnePathsArtifactFromState(
				"ignored",
				stageOnePathsArtifactSnapshot{},
			)
			if err == nil {
				t.Fatal(
					"expected restoreStageOnePathsArtifactFromState to return remove error",
				)
			}
			if !errors.Is(err, removeErr) {
				t.Fatalf("error = %v, expected wrapped remove error", err)
			}
		},
	)
}

func TestWriteRouteArtifactsWithoutHoldingRuntimeLock_RunsHeavyArtifactStepsOutsideRuntimeWriteLock(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("captured-build-id")
		l.SetRouteManifestFile("manifest-before-success.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
	})

	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.writeStageOnePathsJSONForRuntimeState = func(
				v *vormaruntime.Vorma,
				runtimeStateSnapshot buildlifecycle.RouteBuildRuntimeStateSnapshot,
				routeManifestFile string,
			) error {
				if runtimeStateSnapshot.BuildID != "captured-build-id" {
					t.Fatalf(
						"runtime snapshot build ID = %q, want %q",
						runtimeStateSnapshot.BuildID,
						"captured-build-id",
					)
				}
				if routeManifestFile == "" {
					t.Fatal(
						"expected non-empty route manifest file during stage-one write",
					)
				}
				testkit.AssertRuntimeWriteLockCanBeAcquiredPromptly(
					t,
					v,
					"writeStageOnePathsJSONForRuntimeState",
				)
				return nil
			}
			dependencies.writeGeneratedTypeScriptForRuntimeState = func(
				v *vormaruntime.Vorma,
				runtimeStateSnapshot buildlifecycle.RouteBuildRuntimeStateSnapshot,
			) error {
				if runtimeStateSnapshot.BuildID != "captured-build-id" {
					t.Fatalf(
						"runtime snapshot build ID = %q, want %q",
						runtimeStateSnapshot.BuildID,
						"captured-build-id",
					)
				}
				testkit.AssertRuntimeWriteLockCanBeAcquiredPromptly(
					t,
					v,
					"writeGeneratedTypeScriptForRuntimeState",
				)
				return nil
			}
		},
	)

	if err := routeRegistryBuildExecutor.writeRouteArtifactsWithoutHoldingRuntimeLock(app); err != nil {
		t.Fatalf(
			"writeRouteArtifactsWithoutHoldingRuntimeLock returned error: %v",
			err,
		)
	}

	if got := app.RouteManifestFile(); got == "" {
		t.Fatal("expected committed route manifest file after successful write")
	}
}

func TestWriteRouteArtifactsWithoutHoldingRuntimeLock_GeneratesManifestOutsideRuntimeWriteLock(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("manifest-planning-build-id")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
	})

	defaultManifestGenerator := generateRouteManifestFromPaths
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.generateRouteManifestFromPaths = func(
				paths map[string]*vormaruntime.Path,
				nestedRouter *nestedmux.Router,
			) map[string]int {
				testkit.AssertRuntimeWriteLockCanBeAcquiredPromptly(
					t,
					app,
					"generateRouteManifestFromPaths",
				)
				return defaultManifestGenerator(paths, nestedRouter)
			}
			dependencies.writeStageOnePathsJSONForRuntimeState = func(
				*vormaruntime.Vorma,
				buildlifecycle.RouteBuildRuntimeStateSnapshot,
				string,
			) error {
				return nil
			}
			dependencies.writeGeneratedTypeScriptForRuntimeState = func(
				*vormaruntime.Vorma,
				buildlifecycle.RouteBuildRuntimeStateSnapshot,
			) error {
				return nil
			}
		},
	)

	if err := routeRegistryBuildExecutor.writeRouteArtifactsWithoutHoldingRuntimeLock(app); err != nil {
		t.Fatalf(
			"writeRouteArtifactsWithoutHoldingRuntimeLock returned error: %v",
			err,
		)
	}
}

func TestWriteRouteArtifactsWithoutHoldingRuntimeLock_DoesNotCommitRouteManifestFileWhenGeneratedTypeScriptWriteFails(
	t *testing.T,
) {
	expectedErr := errors.New("generated TS failed")
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.writeStageOnePathsJSONForRuntimeState = func(
				*vormaruntime.Vorma,
				buildlifecycle.RouteBuildRuntimeStateSnapshot,
				string,
			) error {
				return nil
			}
			dependencies.writeGeneratedTypeScriptForRuntimeState = func(
				*vormaruntime.Vorma,
				buildlifecycle.RouteBuildRuntimeStateSnapshot,
			) error {
				return expectedErr
			}
		},
	)

	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetRouteManifestFile("manifest-before-failure.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
	})

	gotErr := routeRegistryBuildExecutor.writeRouteArtifactsWithoutHoldingRuntimeLock(
		app,
	)
	if gotErr == nil {
		t.Fatal(
			"expected writeRouteArtifactsWithoutHoldingRuntimeLock to return an error",
		)
	}
	if !strings.Contains(gotErr.Error(), "write generated TypeScript") {
		t.Fatalf(
			"error = %q, expected generated TypeScript write context",
			gotErr,
		)
	}
	if !errors.Is(gotErr, expectedErr) {
		t.Fatalf(
			"error = %v, expected wrapped generated TypeScript error",
			gotErr,
		)
	}
	if got := app.RouteManifestFile(); got != "manifest-before-failure.json" {
		t.Fatalf(
			"routeManifestFile = %q, want unchanged %q",
			got,
			"manifest-before-failure.json",
		)
	}

	manifestFiles := listGeneratedRouteManifestFiles(t, fixture.PublicDir)
	if len(manifestFiles) != 0 {
		t.Fatalf(
			"expected no generated route manifest artifacts after failed downstream write, got %#v",
			manifestFiles,
		)
	}
}

func TestWriteRouteArtifactsWithoutHoldingRuntimeLock_UsesSingleCapturedRuntimeSnapshotAcrossArtifactSteps(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	initialPaths := map[string]*vormaruntime.Path{
		"/stable": {
			OriginalPattern: "/stable",
			SrcPath:         "frontend/src/routes/stable.tsx",
			ExportKey:       "default",
		},
	}

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-before-mutation")
		l.SetPaths(initialPaths)
	})

	var snapshotSeenByStageOne buildlifecycle.RouteBuildRuntimeStateSnapshot
	var snapshotSeenByGeneratedTS buildlifecycle.RouteBuildRuntimeStateSnapshot
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.writeStageOnePathsJSONForRuntimeState = func(
				_ *vormaruntime.Vorma,
				runtimeStateSnapshot buildlifecycle.RouteBuildRuntimeStateSnapshot,
				_ string,
			) error {
				snapshotSeenByStageOne = runtimeStateSnapshot
				return nil
			}
			dependencies.writeGeneratedTypeScriptForRuntimeState = func(
				v *vormaruntime.Vorma,
				runtimeStateSnapshot buildlifecycle.RouteBuildRuntimeStateSnapshot,
			) error {
				snapshotSeenByGeneratedTS = runtimeStateSnapshot
				v.WithLock(func(l *vormaruntime.LockedVorma) {
					l.SetBuildID("build-after-mutation")
					l.SetPaths(map[string]*vormaruntime.Path{
						"/mutated": {
							OriginalPattern: "/mutated",
							SrcPath:         "frontend/src/routes/mutated.tsx",
							ExportKey:       "default",
						},
					})
				})
				return nil
			}
		},
	)

	if err := routeRegistryBuildExecutor.writeRouteArtifactsWithoutHoldingRuntimeLock(app); err != nil {
		t.Fatalf(
			"writeRouteArtifactsWithoutHoldingRuntimeLock returned error: %v",
			err,
		)
	}

	if snapshotSeenByStageOne.BuildID != "build-before-mutation" {
		t.Fatalf(
			"stage-one snapshot build ID = %q, want %q",
			snapshotSeenByStageOne.BuildID,
			"build-before-mutation",
		)
	}
	if snapshotSeenByGeneratedTS.BuildID != "build-before-mutation" {
		t.Fatalf(
			"generated TS snapshot build ID = %q, want %q",
			snapshotSeenByGeneratedTS.BuildID,
			"build-before-mutation",
		)
	}
	if !buildlifecycle.RouteBuildRuntimePathsMapMatches(
		snapshotSeenByGeneratedTS.Paths,
		initialPaths,
	) {
		t.Fatalf(
			"generated TS snapshot paths = %#v, want stable initial paths %#v",
			snapshotSeenByGeneratedTS.Paths,
			initialPaths,
		)
	}
}

func TestWriteRouteArtifactsWithoutHoldingRuntimeLock_StopsBeforeWritingArtifactsWhenSnapshotIsStale(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("stale-build-id")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
	})

	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.isRouteBuildRuntimeStateSnapshotCurrent = func(
				*vormaruntime.Vorma,
				buildlifecycle.RouteBuildRuntimeStateSnapshot,
			) bool {
				return false
			}
			dependencies.writeStageOnePathsJSONForRuntimeState = func(
				*vormaruntime.Vorma,
				buildlifecycle.RouteBuildRuntimeStateSnapshot,
				string,
			) error {
				t.Fatal(
					"did not expect stage-one write when runtime snapshot is stale",
				)
				return nil
			}
			dependencies.writeGeneratedTypeScriptForRuntimeState = func(
				*vormaruntime.Vorma,
				buildlifecycle.RouteBuildRuntimeStateSnapshot,
			) error {
				t.Fatal(
					"did not expect TypeScript write when runtime snapshot is stale",
				)
				return nil
			}
			dependencies.commitRouteManifestFileWithRuntimeLock = func(
				*vormaruntime.Vorma,
				routeManifestCommitInput,
			) bool {
				t.Fatal(
					"did not expect route manifest commit when runtime snapshot is stale",
				)
				return false
			}
		},
	)

	if err := routeRegistryBuildExecutor.writeRouteArtifactsWithoutHoldingRuntimeLock(app); err != nil {
		t.Fatalf(
			"writeRouteArtifactsWithoutHoldingRuntimeLock returned error: %v",
			err,
		)
	}
}

func TestWriteRouteArtifactsWithoutHoldingRuntimeLock_SkipsRollbackCleanupWhenRuntimeStateBecomesStaleAfterStageOneWrite(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	t.Chdir(fixture.RootDir)

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-before-staleness")
		l.SetRouteManifestFile("manifest-before-staleness.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		})
	})

	stalenessCheckCount := 0
	routeRegistryBuildExecutor := newRouteRegistryBuildExecutorForTest(
		func(dependencies *routeRegistryBuildDependencies) {
			dependencies.isRouteBuildRuntimeStateSnapshotCurrent = func(
				*vormaruntime.Vorma,
				buildlifecycle.RouteBuildRuntimeStateSnapshot,
			) bool {
				stalenessCheckCount++
				return stalenessCheckCount == 1
			}
			dependencies.writeStageOnePathsJSONForRuntimeState = func(
				*vormaruntime.Vorma,
				buildlifecycle.RouteBuildRuntimeStateSnapshot,
				string,
			) error {
				return nil
			}
			dependencies.writeGeneratedTypeScriptForRuntimeState = func(
				*vormaruntime.Vorma,
				buildlifecycle.RouteBuildRuntimeStateSnapshot,
			) error {
				t.Fatal(
					"did not expect TypeScript write after runtime snapshot became stale",
				)
				return nil
			}
			dependencies.removeRouteManifestJSON = func(string) error {
				t.Fatal(
					"did not expect manifest cleanup when stale guard skips rollback",
				)
				return nil
			}
			dependencies.writeStageOnePathsArtifact = func(string, []byte, os.FileMode) error {
				t.Fatal(
					"did not expect stage-one restore when stale guard skips rollback",
				)
				return nil
			}
			dependencies.removeStageOnePathsArtifact = func(string) error {
				t.Fatal(
					"did not expect stage-one remove when stale guard skips rollback",
				)
				return nil
			}
			dependencies.commitRouteManifestFileWithRuntimeLock = func(
				*vormaruntime.Vorma,
				routeManifestCommitInput,
			) bool {
				t.Fatal(
					"did not expect route manifest commit after stale guard",
				)
				return false
			}
		},
	)

	if err := routeRegistryBuildExecutor.writeRouteArtifactsWithoutHoldingRuntimeLock(app); err != nil {
		t.Fatalf(
			"writeRouteArtifactsWithoutHoldingRuntimeLock returned error: %v",
			err,
		)
	}
}
