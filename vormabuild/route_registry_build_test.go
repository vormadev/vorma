package vormabuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

func TestWriteRouteManifestToDisk_WrapsMarshalError(t *testing.T) {
	originalMarshalRouteManifestJSONStep := routeRegistryBuildDeps.marshalRouteManifestJSON
	originalWriteRouteManifestJSONFileStep := routeRegistryBuildDeps.writeRouteManifestJSON
	t.Cleanup(func() {
		routeRegistryBuildDeps.marshalRouteManifestJSON = originalMarshalRouteManifestJSONStep
		routeRegistryBuildDeps.writeRouteManifestJSON = originalWriteRouteManifestJSONFileStep
	})

	expectedErr := errors.New("marshal failed")
	routeRegistryBuildDeps.marshalRouteManifestJSON = func(any) ([]byte, error) {
		return nil, expectedErr
	}
	routeRegistryBuildDeps.writeRouteManifestJSON = func(string, []byte, os.FileMode) error {
		t.Fatal("did not expect route manifest file write when marshal fails")
		return nil
	}

	fixture := newBuildTestFixture(t, nil)
	_, err := writeRouteManifestToDisk(fixture.app, map[string]int{
		"/": 1,
	})
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

func TestWriteRouteManifestToDisk_ReturnsWriteErrorWhenPublicOutDirIsFile(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}
	mustWriteFile(t, fixture.publicDir, []byte("not a directory"))

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
	originalMarshalRouteManifestJSONStep := routeRegistryBuildDeps.marshalRouteManifestJSON
	originalWriteRouteManifestJSONFileStep := routeRegistryBuildDeps.writeRouteManifestJSON
	t.Cleanup(func() {
		routeRegistryBuildDeps.marshalRouteManifestJSON = originalMarshalRouteManifestJSONStep
		routeRegistryBuildDeps.writeRouteManifestJSON = originalWriteRouteManifestJSONFileStep
	})

	routeRegistryBuildDeps.marshalRouteManifestJSON = func(any) ([]byte, error) {
		return []byte(`{"/":1}`), nil
	}

	var capturedFileMode os.FileMode
	routeRegistryBuildDeps.writeRouteManifestJSON = func(_ string, _ []byte, fileMode os.FileMode) error {
		capturedFileMode = fileMode
		return nil
	}

	fixture := newBuildTestFixture(t, nil)
	if _, err := writeRouteManifestToDisk(fixture.app, map[string]int{"/": 1}); err != nil {
		t.Fatalf("writeRouteManifestToDisk returned error: %v", err)
	}

	if capturedFileMode != os.FileMode(buildArtifactFileMode) {
		t.Fatalf("file mode = %v, want %v", capturedFileMode, buildArtifactFileMode)
	}
}

func TestWriteRouteArtifacts_ReturnsWrappedErrorForEachArtifactStep(t *testing.T) {
	run := func(
		t *testing.T,
		name string,
		prepareFixtureForError func(*buildTestFixture),
		expectedErrContext string,
	) {
		t.Run(name, func(t *testing.T) {
			fixture := newBuildTestFixture(t, nil)
			app := fixture.app
			t.Chdir(fixture.rootDir)

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
				t.Fatalf("error = %q, expected context %q", gotErr, expectedErrContext)
			}
		})
	}

	run(
		t,
		"manifest write failure",
		func(fixture *buildTestFixture) {
			if err := os.RemoveAll(fixture.publicDir); err != nil {
				t.Fatalf("remove public dir: %v", err)
			}
			mustWriteFile(t, fixture.publicDir, []byte("not a directory"))
		},
		"write route manifest",
	)

	run(
		t,
		"paths json write failure",
		func(fixture *buildTestFixture) {
			if err := os.RemoveAll(fixture.privateDir); err != nil {
				t.Fatalf("remove private dir: %v", err)
			}
			mustWriteFile(t, fixture.privateDir, []byte("not a directory"))
		},
		"write paths JSON",
	)

	run(
		t,
		"generated typescript write failure",
		func(fixture *buildTestFixture) {
			mustWriteFile(t, fixture.app.Config.TSGenOutDir, []byte("file blocks tsgen out dir"))
		},
		"write generated TypeScript",
	)
}

func TestWriteRouteArtifacts_RestoresArtifactsThenRePanicsWhenGeneratedTypeScriptPanics(t *testing.T) {
	originalWriteGeneratedTypeScriptStep := routeRegistryBuildDeps.writeGeneratedTypeScript
	t.Cleanup(func() {
		routeRegistryBuildDeps.writeGeneratedTypeScript = originalWriteGeneratedTypeScriptStep
	})

	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

	previousStageOnePathsPath := stageOnePathsArtifactOutputPath(app)
	previousStageOnePathsContent := []byte("previous-stage-one-paths-content")
	mustWriteFile(t, previousStageOnePathsPath, previousStageOnePathsContent)

	expectedPanic := errors.New("generated ts panic")
	routeRegistryBuildDeps.writeGeneratedTypeScript = func(*vormaruntime.LockedVorma) error {
		panic(expectedPanic)
	}

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
			t.Fatalf("recovered panic type = %T, want error", recoveredPanicValue)
		}
		if !errors.Is(recoveredPanicErr, expectedPanic) {
			t.Fatalf("recovered panic = %v, want %v", recoveredPanicErr, expectedPanic)
		}

		manifestFiles := listGeneratedRouteManifestFiles(t, fixture.publicDir)
		if len(manifestFiles) != 0 {
			t.Fatalf("expected no generated route manifest artifacts after panic cleanup, got %#v", manifestFiles)
		}

		restoredStageOnePathsContent, err := os.ReadFile(previousStageOnePathsPath)
		if err != nil {
			t.Fatalf("read restored stage-one paths artifact: %v", err)
		}
		if !bytes.Equal(restoredStageOnePathsContent, previousStageOnePathsContent) {
			t.Fatalf(
				"restored stage-one paths artifact = %q, want previous content %q",
				string(restoredStageOnePathsContent),
				string(previousStageOnePathsContent),
			)
		}

		if got := app.GetRouteManifestFile(); got != "manifest-before-generated-ts-panic.json" {
			t.Fatalf(
				"route manifest file after panic = %q, want unchanged %q",
				got,
				"manifest-before-generated-ts-panic.json",
			)
		}
	}()

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		_ = writeRouteArtifacts(l)
	})
}

func TestWriteRouteArtifacts_RePanicsWhenCleanupAfterPanicFails(t *testing.T) {
	originalWriteGeneratedTypeScriptStep := routeRegistryBuildDeps.writeGeneratedTypeScript
	originalRemoveStageOnePathsArtifactStep := routeRegistryBuildDeps.removeStageOnePathsArtifact
	t.Cleanup(func() {
		routeRegistryBuildDeps.writeGeneratedTypeScript = originalWriteGeneratedTypeScriptStep
		routeRegistryBuildDeps.removeStageOnePathsArtifact = originalRemoveStageOnePathsArtifactStep
	})

	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

	// Force the panic-time cleanup path to attempt removal (non-existent snapshot).
	stageOnePathsPath := stageOnePathsArtifactOutputPath(app)
	_ = os.Remove(stageOnePathsPath)

	expectedPanic := errors.New("generated ts panic")
	routeRegistryBuildDeps.writeGeneratedTypeScript = func(*vormaruntime.LockedVorma) error {
		panic(expectedPanic)
	}
	routeRegistryBuildDeps.removeStageOnePathsArtifact = func(string) error {
		return errors.New("cleanup stage-one remove failed")
	}

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
			t.Fatalf("recovered panic type = %T, want error", recoveredPanicValue)
		}
		if !errors.Is(recoveredPanicErr, expectedPanic) {
			t.Fatalf("recovered panic = %v, want %v", recoveredPanicErr, expectedPanic)
		}
	}()

	app.WithLock(func(l *vormaruntime.LockedVorma) {
		_ = writeRouteArtifacts(l)
	})
}

func TestWriteRouteArtifacts_OnlyCommitsRouteManifestFileOnSuccess(t *testing.T) {
	runFailure := func(
		t *testing.T,
		name string,
		prepareFixtureForError func(*buildTestFixture),
		expectedErrContext string,
	) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			fixture := newBuildTestFixture(t, nil)
			app := fixture.app
			t.Chdir(fixture.rootDir)

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
				routeManifestFileAfterWriteAttempt = l.GetRouteManifestFile()
			})

			if gotErr == nil {
				t.Fatal("expected writeRouteArtifacts to return an error")
			}
			if !strings.Contains(gotErr.Error(), expectedErrContext) {
				t.Fatalf("error = %q, expected context %q", gotErr, expectedErrContext)
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
		func(fixture *buildTestFixture) {
			if err := os.RemoveAll(fixture.privateDir); err != nil {
				t.Fatalf("remove private dir: %v", err)
			}
			mustWriteFile(t, fixture.privateDir, []byte("not a directory"))
		},
		"write paths JSON",
	)

	runFailure(
		t,
		"does not commit route manifest file when generated TypeScript write fails",
		func(fixture *buildTestFixture) {
			mustWriteFile(t, fixture.app.Config.TSGenOutDir, []byte("file blocks tsgen out dir"))
		},
		"write generated TypeScript",
	)

	t.Run("commits new route manifest file after successful artifact writes", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app
		t.Chdir(fixture.rootDir)

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

			routeManifestFileAfterSuccess = l.GetRouteManifestFile()
		})

		if routeManifestFileAfterSuccess == "" {
			t.Fatal("expected non-empty route manifest file after successful write")
		}
		if routeManifestFileAfterSuccess == "manifest-before-success.json" {
			t.Fatalf("expected route manifest file to change after successful write, got %q", routeManifestFileAfterSuccess)
		}
		if _, err := os.Stat(filepath.Join(fixture.publicDir, routeManifestFileAfterSuccess)); err != nil {
			t.Fatalf("expected written route manifest on disk: %v", err)
		}
	})
}

func TestWriteAndSetRouteManifest_DoesNotMutateStateWhenWriteFails(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}
	mustWriteFile(t, fixture.publicDir, []byte("not a directory"))

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
		routeManifestFileAfterFailure = l.GetRouteManifestFile()
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

func TestWriteRouteArtifacts_CleansUpRouteManifestArtifactAfterDownstreamFailure(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

	if err := os.RemoveAll(fixture.privateDir); err != nil {
		t.Fatalf("remove private dir: %v", err)
	}
	mustWriteFile(t, fixture.privateDir, []byte("not a directory"))

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

	manifestFiles := listGeneratedRouteManifestFiles(t, fixture.publicDir)
	if len(manifestFiles) != 0 {
		t.Fatalf("expected no generated route manifest artifacts after failed downstream write, got %#v", manifestFiles)
	}
}

func TestWriteRouteArtifacts_PreservesCommittedRouteManifestWhenFilenameIsUnchanged(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

	manifestJSON, err := routeRegistryBuildDeps.marshalRouteManifestJSON(map[string]int{
		"/": 0,
	})
	if err != nil {
		t.Fatalf("marshal expected manifest JSON: %v", err)
	}
	committedManifestFile := routeManifestFilename(manifestJSON)
	committedManifestPath := filepath.Join(fixture.publicDir, committedManifestFile)
	mustWriteFile(t, committedManifestPath, manifestJSON)

	if err := os.RemoveAll(fixture.privateDir); err != nil {
		t.Fatalf("remove private dir: %v", err)
	}
	mustWriteFile(t, fixture.privateDir, []byte("not a directory"))

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
		t.Fatalf("expected committed manifest file to remain after failed downstream write, stat err=%v", statErr)
	}
}

func TestWriteRouteArtifacts_JoinsManifestCleanupErrorWithDownstreamWriteError(t *testing.T) {
	originalRemoveRouteManifestJSONStep := routeRegistryBuildDeps.removeRouteManifestJSON
	t.Cleanup(func() {
		routeRegistryBuildDeps.removeRouteManifestJSON = originalRemoveRouteManifestJSONStep
	})

	cleanupErr := errors.New("cleanup failed")
	cleanupCalled := false
	routeRegistryBuildDeps.removeRouteManifestJSON = func(string) error {
		cleanupCalled = true
		return cleanupErr
	}

	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

	if err := os.RemoveAll(fixture.privateDir); err != nil {
		t.Fatalf("remove private dir: %v", err)
	}
	mustWriteFile(t, fixture.privateDir, []byte("not a directory"))

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
		t.Fatalf("error = %v, expected cleanup error in joined error chain", gotErr)
	}
}

func TestWriteRouteArtifacts_RestoresStageOnePathsArtifactAfterGeneratedTypeScriptFailure(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

	stageOnePathsPath := stageOnePathsArtifactOutputPath(app)
	previousStageOnePathsContent := []byte(`{"stage":"one","buildID":"previous"}`)
	mustWriteFile(t, stageOnePathsPath, previousStageOnePathsContent)

	mustWriteFile(t, app.Config.TSGenOutDir, []byte("file blocks tsgen out dir"))

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
		t.Fatalf("error = %q, expected generated TypeScript write context", gotErr)
	}

	restoredStageOnePathsContent, err := os.ReadFile(stageOnePathsPath)
	if err != nil {
		t.Fatalf("read restored stage-one paths artifact: %v", err)
	}
	if !bytes.Equal(restoredStageOnePathsContent, previousStageOnePathsContent) {
		t.Fatalf(
			"restored stage-one paths artifact = %q, want previous content %q",
			string(restoredStageOnePathsContent),
			string(previousStageOnePathsContent),
		)
	}
}

func TestWriteRouteArtifacts_RemovesStageOnePathsArtifactWhenNoPreviousSnapshot(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

	stageOnePathsPath := stageOnePathsArtifactOutputPath(app)
	if err := os.Remove(stageOnePathsPath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove stage-one paths artifact: %v", err)
	}

	mustWriteFile(t, app.Config.TSGenOutDir, []byte("file blocks tsgen out dir"))

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
		t.Fatalf("expected no stage-one paths artifact after cleanup, stat err=%v", err)
	}
}

func TestWriteRouteArtifacts_JoinsStageOnePathsCleanupErrorWithDownstreamWriteError(t *testing.T) {
	originalWriteStageOnePathsArtifactStep := routeRegistryBuildDeps.writeStageOnePathsArtifact
	t.Cleanup(func() {
		routeRegistryBuildDeps.writeStageOnePathsArtifact = originalWriteStageOnePathsArtifactStep
	})

	cleanupErr := errors.New("restore stage-one paths failed")
	routeRegistryBuildDeps.writeStageOnePathsArtifact = func(string, []byte, os.FileMode) error {
		return cleanupErr
	}

	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

	stageOnePathsPath := stageOnePathsArtifactOutputPath(app)
	mustWriteFile(t, stageOnePathsPath, []byte(`{"stage":"one","buildID":"previous"}`))

	mustWriteFile(t, app.Config.TSGenOutDir, []byte("file blocks tsgen out dir"))

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
		t.Fatalf("error = %q, expected generated TypeScript write context", gotErr)
	}
	if !strings.Contains(gotErr.Error(), "cleanup stage-one paths artifact") {
		t.Fatalf("error = %q, expected stage-one cleanup context", gotErr)
	}
	if !errors.Is(gotErr, cleanupErr) {
		t.Fatalf("error = %v, expected cleanup error in joined error chain", gotErr)
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

func listGeneratedRouteManifestFiles(t *testing.T, staticPublicOutDir string) []string {
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

func TestWriteRouteArtifacts_ReturnsSnapshotErrorWhenStageOneSnapshotReadFails(t *testing.T) {
	originalReadStageOnePathsArtifactStep := routeRegistryBuildDeps.readStageOnePathsArtifact
	t.Cleanup(func() {
		routeRegistryBuildDeps.readStageOnePathsArtifact = originalReadStageOnePathsArtifactStep
	})

	snapshotErr := errors.New("snapshot read failed")
	routeRegistryBuildDeps.readStageOnePathsArtifact = func(string) ([]byte, error) {
		return nil, snapshotErr
	}

	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

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
		t.Fatal("expected writeRouteArtifacts to return stage-one snapshot error")
	}
	if !strings.Contains(gotErr.Error(), "snapshot stage-one paths artifact") {
		t.Fatalf("error = %q, expected stage-one snapshot context", gotErr)
	}
	if !errors.Is(gotErr, snapshotErr) {
		t.Fatalf("error = %v, expected wrapped stage-one snapshot error", gotErr)
	}
}

func TestCaptureStageOnePathsArtifactSnapshot(t *testing.T) {
	t.Run("returns non-existent snapshot for ENOTDIR errors", func(t *testing.T) {
		originalReadStageOnePathsArtifactStep := routeRegistryBuildDeps.readStageOnePathsArtifact
		t.Cleanup(func() {
			routeRegistryBuildDeps.readStageOnePathsArtifact = originalReadStageOnePathsArtifactStep
		})

		routeRegistryBuildDeps.readStageOnePathsArtifact = func(string) ([]byte, error) {
			return nil, syscall.ENOTDIR
		}

		snapshot, err := captureStageOnePathsArtifactSnapshot("ignored")
		if err != nil {
			t.Fatalf("captureStageOnePathsArtifactSnapshot returned error: %v", err)
		}
		if snapshot.existed {
			t.Fatalf("snapshot.existed = %v, want false for ENOTDIR", snapshot.existed)
		}
		if snapshot.content != nil {
			t.Fatalf("snapshot.content = %#v, want nil for ENOTDIR", snapshot.content)
		}
	})

	t.Run("returns error for non-ENOENT/ENOTDIR failures", func(t *testing.T) {
		originalReadStageOnePathsArtifactStep := routeRegistryBuildDeps.readStageOnePathsArtifact
		t.Cleanup(func() {
			routeRegistryBuildDeps.readStageOnePathsArtifact = originalReadStageOnePathsArtifactStep
		})

		readErr := errors.New("read failed")
		routeRegistryBuildDeps.readStageOnePathsArtifact = func(string) ([]byte, error) {
			return nil, readErr
		}

		_, err := captureStageOnePathsArtifactSnapshot("ignored")
		if err == nil {
			t.Fatal("expected captureStageOnePathsArtifactSnapshot to return error")
		}
		if !errors.Is(err, readErr) {
			t.Fatalf("error = %v, expected wrapped read error", err)
		}
	})
}

func TestRestoreStageOnePathsArtifactFromSnapshot(t *testing.T) {
	t.Run("restores existing snapshot using stage-one writer and build artifact mode", func(t *testing.T) {
		originalWriteStageOnePathsArtifactStep := routeRegistryBuildDeps.writeStageOnePathsArtifact
		originalRemoveStageOnePathsArtifactStep := routeRegistryBuildDeps.removeStageOnePathsArtifact
		t.Cleanup(func() {
			routeRegistryBuildDeps.writeStageOnePathsArtifact = originalWriteStageOnePathsArtifactStep
			routeRegistryBuildDeps.removeStageOnePathsArtifact = originalRemoveStageOnePathsArtifactStep
		})

		var writeCalled bool
		routeRegistryBuildDeps.writeStageOnePathsArtifact = func(
			path string,
			content []byte,
			fileMode os.FileMode,
		) error {
			writeCalled = true
			if path != "stage-one-path.json" {
				t.Fatalf("restore write path = %q, want %q", path, "stage-one-path.json")
			}
			if string(content) != "snapshot-content" {
				t.Fatalf("restore write content = %q, want %q", string(content), "snapshot-content")
			}
			if fileMode != os.FileMode(buildArtifactFileMode) {
				t.Fatalf("restore write mode = %v, want %v", fileMode, buildArtifactFileMode)
			}
			return nil
		}
		routeRegistryBuildDeps.removeStageOnePathsArtifact = func(string) error {
			t.Fatal("did not expect removal when snapshot exists")
			return nil
		}

		err := restoreStageOnePathsArtifactFromSnapshot(
			"stage-one-path.json",
			stageOnePathsArtifactSnapshot{existed: true, content: []byte("snapshot-content")},
		)
		if err != nil {
			t.Fatalf("restoreStageOnePathsArtifactFromSnapshot returned error: %v", err)
		}
		if !writeCalled {
			t.Fatal("expected stage-one snapshot restore write to be called")
		}
	})

	t.Run("ignores ENOTDIR when removing non-existent snapshot artifact", func(t *testing.T) {
		originalRemoveStageOnePathsArtifactStep := routeRegistryBuildDeps.removeStageOnePathsArtifact
		t.Cleanup(func() {
			routeRegistryBuildDeps.removeStageOnePathsArtifact = originalRemoveStageOnePathsArtifactStep
		})

		routeRegistryBuildDeps.removeStageOnePathsArtifact = func(string) error {
			return syscall.ENOTDIR
		}

		if err := restoreStageOnePathsArtifactFromSnapshot("ignored", stageOnePathsArtifactSnapshot{}); err != nil {
			t.Fatalf("restoreStageOnePathsArtifactFromSnapshot returned error: %v", err)
		}
	})

	t.Run("returns removal error for non-ENOENT/ENOTDIR failures", func(t *testing.T) {
		originalRemoveStageOnePathsArtifactStep := routeRegistryBuildDeps.removeStageOnePathsArtifact
		t.Cleanup(func() {
			routeRegistryBuildDeps.removeStageOnePathsArtifact = originalRemoveStageOnePathsArtifactStep
		})

		removeErr := errors.New("remove failed")
		routeRegistryBuildDeps.removeStageOnePathsArtifact = func(string) error {
			return removeErr
		}

		err := restoreStageOnePathsArtifactFromSnapshot("ignored", stageOnePathsArtifactSnapshot{})
		if err == nil {
			t.Fatal("expected restoreStageOnePathsArtifactFromSnapshot to return remove error")
		}
		if !errors.Is(err, removeErr) {
			t.Fatalf("error = %v, expected wrapped remove error", err)
		}
	})
}
