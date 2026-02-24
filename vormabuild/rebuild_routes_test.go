package vormabuild

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
)

func newFastRouteRebuildIDExecutorForTest(
	mutateDependencies func(*fastRouteRebuildBuildIDDependencies),
) fastRouteRebuildIDExecutor {
	dependencies := fastRouteRebuildBuildIDDependencies{}
	if mutateDependencies != nil {
		mutateDependencies(&dependencies)
	}
	return newFastRouteRebuildIDExecutor(dependencies)
}

func TestCleanRouteManifestsOnly_IgnoresMissingPublicOutDir(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}

	if err := cleanRouteManifestsOnly(app); err != nil {
		t.Fatalf(
			"cleanRouteManifestsOnly should ignore missing dir, got: %v",
			err,
		)
	}
}

func TestRunRouteSyncExecution_ForFastRebuildSuccess(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)

	clientPaths := map[string]*vormaruntime.Path{
		"/products/:id": {
			OriginalPattern: "/products/:id",
			SrcPath:         "frontend/src/routes/products.$id.tsx",
			ExportKey:       "default",
		},
	}

	if err := runRouteSyncExecution(
		app,
		routeSyncExecutionOptions{
			parseClientRoutes: func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
				return clientPaths, nil
			},
			generateBuildID: func() (string, error) {
				return "dev_fast_test", nil
			},
			postSyncHook: func(*vormaruntime.Vorma) error {
				return writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
					app,
					fastRouteRebuildArtifactDependencies{},
				)
			},
		},
	); err != nil {
		t.Fatalf("runRouteSyncExecution returned error: %v", err)
	}

	if got := app.BuildID(); got != "dev_fast_test" {
		t.Fatalf("build ID = %q, want %q", got, "dev_fast_test")
	}
	if app.Paths()["/products/:id"] == nil {
		t.Fatal("expected synced route to be present in app paths")
	}

	stageOnePath := filepath.Join(
		fixture.privateDir,
		runtimepaths.VormaOutDirname,
		runtimepaths.VormaPathsStageOneJSONFileName,
	)
	if _, err := os.Stat(stageOnePath); err != nil {
		t.Fatalf("expected stage one paths file to exist: %v", err)
	}

	manifestFile := app.RouteManifestFile()
	if manifestFile == "" {
		t.Fatal("expected route manifest file name to be set")
	}
	if _, err := os.Stat(filepath.Join(fixture.publicDir, manifestFile)); err != nil {
		t.Fatalf("expected route manifest file to exist: %v", err)
	}

	generatedTSPath := filepath.Join(app.Config.TSGenOutDir, "index.ts")
	if _, err := os.Stat(generatedTSPath); err != nil {
		t.Fatalf("expected generated TypeScript to exist: %v", err)
	}
}

func TestRunRouteSyncExecution_ForFastRebuildReturnsCleanError(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-before-fast-rebuild-failure")
		l.SetRouteManifestFile("manifest-before-fast-rebuild-failure.json")
		l.SetPaths(map[string]*vormaruntime.Path{
			"/existing": {
				OriginalPattern: "/existing",
				SrcPath:         "frontend/src/routes/existing.tsx",
				ExportKey:       "default",
			},
		})
	})

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}
	mustWriteFile(t, fixture.publicDir, []byte("not a directory"))

	err := runRouteSyncExecution(
		app,
		routeSyncExecutionOptions{
			parseClientRoutes: func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
				return map[string]*vormaruntime.Path{
					"/products/:id": {
						OriginalPattern: "/products/:id",
						SrcPath:         "frontend/src/routes/products.$id.tsx",
						ExportKey:       "default",
					},
				}, nil
			},
			generateBuildID: func() (string, error) {
				return "dev_fast_test", nil
			},
			postSyncHook: func(*vormaruntime.Vorma) error {
				return writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
					app,
					fastRouteRebuildArtifactDependencies{},
				)
			},
		},
	)
	if err == nil {
		t.Fatal("expected runRouteSyncExecution to return error")
	}
	if !strings.Contains(err.Error(), "clean route manifests") {
		t.Fatalf("error = %q, expected clean-route-manifests context", err)
	}
	if got := app.BuildID(); got != "build-before-fast-rebuild-failure" {
		t.Fatalf(
			"build ID after failed fast rebuild = %q, want %q",
			got,
			"build-before-fast-rebuild-failure",
		)
	}
	if got := app.RouteManifestFile(); got != "manifest-before-fast-rebuild-failure.json" {
		t.Fatalf(
			"route manifest after failed fast rebuild = %q, want %q",
			got,
			"manifest-before-fast-rebuild-failure.json",
		)
	}
	paths := app.Paths()
	if paths["/existing"] == nil {
		t.Fatalf(
			"expected /existing path to remain after failed fast rebuild, got %#v",
			paths,
		)
	}
	if paths["/products/:id"] != nil {
		t.Fatalf(
			"did not expect /products/:id path after failed fast rebuild, got %#v",
			paths["/products/:id"],
		)
	}
}

func TestWriteFastRebuildArtifactsAfterRouteSync(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	t.Chdir(fixture.rootDir)
	defaultDependencies := defaultFastRouteRebuildArtifactDependencies()
	newDependencies := func() fastRouteRebuildArtifactDependencies {
		return defaultDependencies
	}

	t.Run("wraps clean route manifests error", func(t *testing.T) {
		expectedErr := errors.New("clean failed")
		dependencies := newDependencies()
		dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
			return expectedErr
		}
		dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
			t.Fatal("did not expect writeRouteArtifacts after clean failure")
			return nil
		}

		err := writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
			app,
			dependencies,
		)
		if err == nil {
			t.Fatal(
				"expected writeFastRebuildArtifactsAfterRouteSync to return clean error",
			)
		}
		if !strings.Contains(err.Error(), "clean route manifests") {
			t.Fatalf("error = %q, expected clean-route-manifests context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf(
				"error = %v, expected wrapped clean-route-manifests error",
				err,
			)
		}
	})

	t.Run(
		"restores previous route manifest artifact when clean step fails after removal",
		func(t *testing.T) {
			previousManifestFile := vormaruntime.VormaRouteManifestPrefix + "previous_clean.json"
			previousManifestPath := filepath.Join(
				fixture.publicDir,
				previousManifestFile,
			)
			previousManifestContent := []byte(`{"/":0}`)
			mustWriteFile(t, previousManifestPath, previousManifestContent)

			expectedErr := errors.New("clean failed")
			dependencies := newDependencies()
			dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
				if err := os.Remove(previousManifestPath); err != nil {
					t.Fatalf(
						"remove previous manifest during clean simulation: %v",
						err,
					)
				}
				return expectedErr
			}
			dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
				t.Fatal(
					"did not expect writeRouteArtifacts after clean failure",
				)
				return nil
			}

			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetRouteManifestFile(previousManifestFile)
			})
			err := writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
				app,
				dependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFastRebuildArtifactsAfterRouteSync to return clean error",
				)
			}
			if !strings.Contains(err.Error(), "clean route manifests") {
				t.Fatalf(
					"error = %q, expected clean-route-manifests context",
					err,
				)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf(
					"error = %v, expected wrapped clean-route-manifests error",
					err,
				)
			}

			restoredManifestContent, readErr := os.ReadFile(
				previousManifestPath,
			)
			if readErr != nil {
				t.Fatalf("read restored route manifest artifact: %v", readErr)
			}
			if !bytes.Equal(restoredManifestContent, previousManifestContent) {
				t.Fatalf(
					"restored route manifest artifact = %q, want %q",
					string(restoredManifestContent),
					string(previousManifestContent),
				)
			}
		},
	)

	t.Run(
		"returns snapshot error when reading current manifest artifact fails",
		func(t *testing.T) {
			snapshotErr := errors.New("snapshot failed")
			dependencies := newDependencies()
			dependencies.readRouteManifestArtifact = func(string) ([]byte, error) {
				return nil, snapshotErr
			}
			dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
				t.Fatal("did not expect clean step after snapshot failure")
				return nil
			}

			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetRouteManifestFile(
					vormaruntime.VormaRouteManifestPrefix + "current.json",
				)
			})
			err := writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
				app,
				dependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFastRebuildArtifactsAfterRouteSync to return snapshot error",
				)
			}
			if !strings.Contains(
				err.Error(),
				"snapshot current route manifest artifact",
			) {
				t.Fatalf("error = %q, expected snapshot context", err)
			}
			if !errors.Is(err, snapshotErr) {
				t.Fatalf("error = %v, expected wrapped snapshot error", err)
			}
		},
	)

	t.Run("returns write route artifacts error", func(t *testing.T) {
		expectedErr := errors.New("write artifacts failed")
		dependencies := newDependencies()
		dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
			return nil
		}
		dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
			return expectedErr
		}

		err := writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
			app,
			dependencies,
		)
		if err == nil {
			t.Fatal(
				"expected writeFastRebuildArtifactsAfterRouteSync to return write-artifacts error",
			)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected write-artifacts error", err)
		}
	})

	t.Run(
		"restores previous route manifest artifact when write step fails",
		func(t *testing.T) {
			previousManifestFile := vormaruntime.VormaRouteManifestPrefix + "previous.json"
			previousManifestPath := filepath.Join(
				fixture.publicDir,
				previousManifestFile,
			)
			previousManifestContent := []byte(`{"/":0}`)
			mustWriteFile(t, previousManifestPath, previousManifestContent)

			dependencies := newDependencies()
			dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
				return os.Remove(previousManifestPath)
			}
			expectedErr := errors.New("write artifacts failed")
			dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
				return expectedErr
			}

			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetRouteManifestFile(previousManifestFile)
			})
			err := writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
				app,
				dependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFastRebuildArtifactsAfterRouteSync to return write-artifacts error",
				)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected write-artifacts error", err)
			}

			restoredManifestContent, readErr := os.ReadFile(
				previousManifestPath,
			)
			if readErr != nil {
				t.Fatalf("read restored route manifest artifact: %v", readErr)
			}
			if !bytes.Equal(restoredManifestContent, previousManifestContent) {
				t.Fatalf(
					"restored route manifest artifact = %q, want %q",
					string(restoredManifestContent),
					string(previousManifestContent),
				)
			}
		},
	)

	t.Run(
		"joins restore error when write step fails and restore fails",
		func(t *testing.T) {
			previousManifestFile := vormaruntime.VormaRouteManifestPrefix + "previous_join.json"
			previousManifestPath := filepath.Join(
				fixture.publicDir,
				previousManifestFile,
			)
			mustWriteFile(t, previousManifestPath, []byte(`{"/":0}`))

			dependencies := newDependencies()
			dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
				return os.Remove(previousManifestPath)
			}
			expectedErr := errors.New("write artifacts failed")
			dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
				return expectedErr
			}
			restoreErr := errors.New("restore failed")
			dependencies.writeRouteManifestArtifact = func(string, []byte, os.FileMode) error {
				return restoreErr
			}

			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetRouteManifestFile(previousManifestFile)
			})
			err := writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
				app,
				dependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFastRebuildArtifactsAfterRouteSync to return joined error",
				)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf(
					"error = %v, expected write-artifacts error in joined chain",
					err,
				)
			}
			if !errors.Is(err, restoreErr) {
				t.Fatalf(
					"error = %v, expected restore error in joined chain",
					err,
				)
			}
			if !strings.Contains(
				err.Error(),
				"restore route manifest artifact",
			) {
				t.Fatalf("error = %q, expected restore context", err)
			}
		},
	)

	t.Run(
		"restores previous route manifest artifact then re-panics when write step panics",
		func(t *testing.T) {
			previousManifestFile := vormaruntime.VormaRouteManifestPrefix + "previous_panic.json"
			previousManifestPath := filepath.Join(
				fixture.publicDir,
				previousManifestFile,
			)
			previousManifestContent := []byte(`{"/":0}`)
			mustWriteFile(t, previousManifestPath, previousManifestContent)

			dependencies := newDependencies()
			dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
				return os.Remove(previousManifestPath)
			}

			expectedPanic := errors.New("write panic")
			dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
				panic(expectedPanic)
			}

			defer func() {
				recoveredPanicValue := recover()
				if recoveredPanicValue == nil {
					t.Fatal(
						"expected writeFastRebuildArtifactsAfterRouteSync to panic",
					)
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

				restoredManifestContent, readErr := os.ReadFile(
					previousManifestPath,
				)
				if readErr != nil {
					t.Fatalf(
						"read restored route manifest artifact: %v",
						readErr,
					)
				}
				if !bytes.Equal(
					restoredManifestContent,
					previousManifestContent,
				) {
					t.Fatalf(
						"restored route manifest artifact = %q, want %q",
						string(restoredManifestContent),
						string(previousManifestContent),
					)
				}
			}()

			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetRouteManifestFile(previousManifestFile)
			})
			_ = writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
				app,
				dependencies,
			)
		},
	)

	t.Run(
		"re-panics when manifest restore after panic fails",
		func(t *testing.T) {
			previousManifestFile := vormaruntime.VormaRouteManifestPrefix + "previous_panic_join.json"
			previousManifestPath := filepath.Join(
				fixture.publicDir,
				previousManifestFile,
			)
			mustWriteFile(t, previousManifestPath, []byte(`{"/":0}`))

			dependencies := newDependencies()
			dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
				return os.Remove(previousManifestPath)
			}

			expectedPanic := errors.New("write panic")
			dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
				panic(expectedPanic)
			}
			dependencies.writeRouteManifestArtifact = func(string, []byte, os.FileMode) error {
				return errors.New("restore failed")
			}

			defer func() {
				recoveredPanicValue := recover()
				if recoveredPanicValue == nil {
					t.Fatal(
						"expected writeFastRebuildArtifactsAfterRouteSync to panic",
					)
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
				l.SetRouteManifestFile(previousManifestFile)
			})
			_ = writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
				app,
				dependencies,
			)
		},
	)

	t.Run("runs clean then writes artifacts", func(t *testing.T) {
		var observedSteps []string
		dependencies := newDependencies()
		dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
			observedSteps = append(observedSteps, "clean")
			return nil
		}
		dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
			observedSteps = append(observedSteps, "write")
			return nil
		}

		err := writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
			app,
			dependencies,
		)
		if err != nil {
			t.Fatalf(
				"writeFastRebuildArtifactsAfterRouteSync returned error: %v",
				err,
			)
		}
		if len(observedSteps) != 2 || observedSteps[0] != "clean" ||
			observedSteps[1] != "write" {
			t.Fatalf("observed steps = %#v, want [clean write]", observedSteps)
		}
	})

	t.Run(
		"skips clean and write when build ID token is stale before artifact write",
		func(t *testing.T) {
			buildIDReadCount := 0
			dependencies := newDependencies()
			dependencies.getCurrentBuildIDWithReadLock = func(*vormaruntime.Vorma) string {
				buildIDReadCount++
				if buildIDReadCount == 1 {
					return "build-before"
				}
				return "build-after"
			}
			dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
				t.Fatal(
					"did not expect clean route manifests when build ID token is stale",
				)
				return nil
			}
			dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
				t.Fatal(
					"did not expect writeRouteArtifacts when build ID token is stale",
				)
				return nil
			}

			if err := writeFastRebuildArtifactsAfterRouteSyncWithDependencies(app, dependencies); err != nil {
				t.Fatalf(
					"writeFastRebuildArtifactsAfterRouteSync returned error: %v",
					err,
				)
			}
		},
	)

	t.Run(
		"skips manifest snapshot restore when build ID token becomes stale before rollback",
		func(t *testing.T) {
			previousManifestFile := vormaruntime.VormaRouteManifestPrefix + "previous_stale_skip_restore.json"
			previousManifestPath := filepath.Join(
				fixture.publicDir,
				previousManifestFile,
			)
			mustWriteFile(t, previousManifestPath, []byte(`{"/":0}`))

			buildIDReadCount := 0
			dependencies := newDependencies()
			dependencies.getCurrentBuildIDWithReadLock = func(*vormaruntime.Vorma) string {
				buildIDReadCount++
				if buildIDReadCount <= 3 {
					return "build-before"
				}
				return "build-after"
			}
			dependencies.cleanRouteManifestsOnly = func(*vormaruntime.Vorma) error {
				return os.Remove(previousManifestPath)
			}

			expectedErr := errors.New("write artifacts failed")
			dependencies.writeRouteArtifacts = func(*vormaruntime.Vorma) error {
				return expectedErr
			}
			dependencies.writeRouteManifestArtifact = func(string, []byte, os.FileMode) error {
				t.Fatal(
					"did not expect manifest restore write when build ID token is stale",
				)
				return nil
			}
			dependencies.removeRouteManifestArtifact = func(string) error {
				t.Fatal(
					"did not expect manifest restore removal when build ID token is stale",
				)
				return nil
			}

			app.WithLock(func(l *vormaruntime.LockedVorma) {
				l.SetBuildID("build-before")
				l.SetRouteManifestFile(previousManifestFile)
			})
			err := writeFastRebuildArtifactsAfterRouteSyncWithDependencies(
				app,
				dependencies,
			)
			if err == nil {
				t.Fatal(
					"expected writeFastRebuildArtifactsAfterRouteSync to return write-artifacts error",
				)
			}
			if !errors.Is(err, expectedErr) {
				t.Fatalf("error = %v, expected write-artifacts error", err)
			}
			if _, statErr := os.Stat(previousManifestPath); !os.IsNotExist(
				statErr,
			) {
				t.Fatalf(
					"expected previous manifest artifact to stay removed when stale rollback is skipped, stat err=%v",
					statErr,
				)
			}
		},
	)
}

func TestCaptureFastRebuildRouteManifestArtifact(t *testing.T) {
	t.Run(
		"returns zero snapshot when manifest file name is empty",
		func(t *testing.T) {
			fixture := newBuildTestFixture(t, nil)
			snapshot, err := captureFastRebuildRouteManifestArtifactSnapshotWithDependencies(
				fixture.app,
				"",
				fastRouteRebuildArtifactDependencies{},
			)
			if err != nil {
				t.Fatalf(
					"captureFastRebuildRouteManifestArtifact returned error: %v",
					err,
				)
			}
			if snapshot.existed {
				t.Fatalf("snapshot.existed = %v, want false", snapshot.existed)
			}
			if snapshot.content != nil {
				t.Fatalf("snapshot.content = %#v, want nil", snapshot.content)
			}
		},
	)

	t.Run(
		"returns non-existent snapshot for ENOTDIR errors",
		func(t *testing.T) {
			dependencies := fastRouteRebuildArtifactDependencies{
				readRouteManifestArtifact: func(string) ([]byte, error) {
					return nil, syscall.ENOTDIR
				},
			}

			fixture := newBuildTestFixture(t, nil)
			snapshot, err := captureFastRebuildRouteManifestArtifactSnapshotWithDependencies(
				fixture.app,
				"ignored.json",
				dependencies,
			)
			if err != nil {
				t.Fatalf(
					"captureFastRebuildRouteManifestArtifact returned error: %v",
					err,
				)
			}
			if snapshot.existed {
				t.Fatalf(
					"snapshot.existed = %v, want false for ENOTDIR",
					snapshot.existed,
				)
			}
			if snapshot.content != nil {
				t.Fatalf(
					"snapshot.content = %#v, want nil for ENOTDIR",
					snapshot.content,
				)
			}
		},
	)

	t.Run("returns error for non-ENOENT/ENOTDIR failures", func(t *testing.T) {
		readErr := errors.New("read failed")
		dependencies := fastRouteRebuildArtifactDependencies{
			readRouteManifestArtifact: func(string) ([]byte, error) {
				return nil, readErr
			},
		}

		fixture := newBuildTestFixture(t, nil)
		_, err := captureFastRebuildRouteManifestArtifactSnapshotWithDependencies(
			fixture.app,
			vormaruntime.VormaRouteManifestPrefix+"current.json",
			dependencies,
		)
		if err == nil {
			t.Fatal(
				"expected captureFastRebuildRouteManifestArtifact to return read error",
			)
		}
		if !errors.Is(err, readErr) {
			t.Fatalf("error = %v, expected wrapped read error", err)
		}
	})
}

func TestRestoreFastRebuildRouteManifestArtifact(t *testing.T) {
	t.Run("returns nil when manifest file name is empty", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		err := restoreFastRebuildRouteManifestArtifactSnapshotWithDependencies(
			fixture.app,
			"",
			fastRebuildRouteManifestArtifactSnapshot{},
			fastRouteRebuildArtifactDependencies{},
		)
		if err != nil {
			t.Fatalf(
				"restoreFastRebuildRouteManifestArtifact returned error: %v",
				err,
			)
		}
	})

	t.Run(
		"returns error when removing missing-snapshot manifest fails",
		func(t *testing.T) {
			removeErr := errors.New("remove failed")
			dependencies := fastRouteRebuildArtifactDependencies{
				removeRouteManifestArtifact: func(string) error {
					return removeErr
				},
			}

			fixture := newBuildTestFixture(t, nil)
			err := restoreFastRebuildRouteManifestArtifactSnapshotWithDependencies(
				fixture.app,
				vormaruntime.VormaRouteManifestPrefix+"current.json",
				fastRebuildRouteManifestArtifactSnapshot{},
				dependencies,
			)
			if err == nil {
				t.Fatal(
					"expected restoreFastRebuildRouteManifestArtifact to return remove error",
				)
			}
			if !errors.Is(err, removeErr) {
				t.Fatalf("error = %v, expected wrapped remove error", err)
			}
		},
	)

	t.Run(
		"ignores ENOTDIR when removing missing-snapshot manifest",
		func(t *testing.T) {
			dependencies := fastRouteRebuildArtifactDependencies{
				removeRouteManifestArtifact: func(string) error {
					return syscall.ENOTDIR
				},
			}

			fixture := newBuildTestFixture(t, nil)
			if err := restoreFastRebuildRouteManifestArtifactSnapshotWithDependencies(
				fixture.app,
				vormaruntime.VormaRouteManifestPrefix+"current.json",
				fastRebuildRouteManifestArtifactSnapshot{},
				dependencies,
			); err != nil {
				t.Fatalf(
					"restoreFastRebuildRouteManifestArtifact returned error: %v",
					err,
				)
			}
		},
	)
}

func TestRebuildRoutesOnly(t *testing.T) {
	defaultDependencies := defaultFastRouteRebuildDependencies()
	newDependencies := func() fastRouteRebuildDependencies {
		return defaultDependencies
	}

	t.Run("returns parse error with context", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app
		app.SetIsDev(true)

		dependencies := newDependencies()
		dependencies.parseClientRoutes = func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
			return nil, os.ErrInvalid
		}
		dependencies.newFastRebuildID = func() (string, error) {
			t.Fatal("did not expect ID generation after parse error")
			return "", nil
		}

		err := rebuildRoutesOnlyWithDependencies(
			app,
			dependencies,
			fastRouteRebuildArtifactDependencies{},
		)
		if err == nil {
			t.Fatal("expected rebuildRoutesOnly to return parse error")
		}
		if !strings.Contains(err.Error(), "parse client routes") {
			t.Fatalf("error = %q, expected parse context", err)
		}
	})

	t.Run("propagates build ID generation error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app
		app.SetIsDev(true)

		dependencies := newDependencies()
		dependencies.parseClientRoutes = func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
			return map[string]*vormaruntime.Path{}, nil
		}
		expectedErr := os.ErrPermission
		dependencies.newFastRebuildID = func() (string, error) {
			return "", expectedErr
		}

		err := rebuildRoutesOnlyWithDependencies(
			app,
			dependencies,
			fastRouteRebuildArtifactDependencies{},
		)
		if err == nil {
			t.Fatal(
				"expected rebuildRoutesOnly to return build ID generation error",
			)
		}
		if !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Fatalf(
				"error = %q, expected wrapped build ID generation error",
				err,
			)
		}
	})

	t.Run("propagates artifact sync error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app
		app.SetIsDev(true)

		dependencies := newDependencies()
		dependencies.parseClientRoutes = func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
			return map[string]*vormaruntime.Path{
				"/ok": {
					OriginalPattern: "/ok",
					SrcPath:         "frontend/src/routes/ok.tsx",
					ExportKey:       "default",
				},
			}, nil
		}
		dependencies.newFastRebuildID = func() (string, error) {
			return "dev_fast_stub", nil
		}
		expectedErr := os.ErrInvalid
		dependencies.runRouteSyncExecution = func(*vormaruntime.Vorma, routeSyncExecutionOptions) error {
			return expectedErr
		}

		err := rebuildRoutesOnlyWithDependencies(
			app,
			dependencies,
			fastRouteRebuildArtifactDependencies{},
		)
		if err == nil {
			t.Fatal("expected rebuildRoutesOnly to return artifact sync error")
		}
		if !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Fatalf("error = %q, expected wrapped artifact sync error", err)
		}
	})

	t.Run("success path logs completion", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app
		t.Chdir(fixture.rootDir)
		app.SetIsDev(true)

		dependencies := newDependencies()
		dependencies.parseClientRoutes = func(*vormaruntime.Vorma) (map[string]*vormaruntime.Path, error) {
			return map[string]*vormaruntime.Path{
				"/ok": {
					OriginalPattern: "/ok",
					SrcPath:         "frontend/src/routes/ok.tsx",
					ExportKey:       "default",
				},
			}, nil
		}
		dependencies.newFastRebuildID = func() (string, error) {
			return "dev_fast_stub", nil
		}

		var observedBuildID string
		var observedPathsCount int
		dependencies.runRouteSyncExecution = func(v *vormaruntime.Vorma, options routeSyncExecutionOptions) error {
			parsedPaths, parseErr := options.parseClientRoutes(v)
			if parseErr != nil {
				return parseErr
			}
			buildID, buildIDErr := options.generateBuildID()
			if buildIDErr != nil {
				return buildIDErr
			}
			observedBuildID = buildID
			observedPathsCount = len(parsedPaths)
			if options.postSyncHook != nil {
				if postSyncHookErr := options.postSyncHook(v); postSyncHookErr != nil {
					return postSyncHookErr
				}
			}
			return nil
		}

		var completionLogged bool
		dependencies.logFastRouteRebuildCompletion = func(*vormaruntime.Vorma, time.Time) {
			completionLogged = true
		}

		err := rebuildRoutesOnlyWithDependencies(
			app,
			dependencies,
			fastRouteRebuildArtifactDependencies{},
		)
		if err != nil {
			t.Fatalf("rebuildRoutesOnly returned error: %v", err)
		}
		if observedBuildID != "dev_fast_stub" {
			t.Fatalf(
				"observed build ID = %q, want %q",
				observedBuildID,
				"dev_fast_stub",
			)
		}
		if observedPathsCount != 1 {
			t.Fatalf("observed path count = %d, want 1", observedPathsCount)
		}
		if !completionLogged {
			t.Fatal("expected fast route rebuild completion to be logged")
		}
	})
}

func TestNewFastRebuildID_ReturnsErrorWhenIDGenerationFails(t *testing.T) {
	fastRouteRebuildIDExecutor := newFastRouteRebuildIDExecutorForTest(
		func(dependencies *fastRouteRebuildBuildIDDependencies) {
			dependencies.generateFastRebuildIDSuffix = func() (string, error) {
				return "", os.ErrPermission
			}
		},
	)

	_, err := fastRouteRebuildIDExecutor.newFastRebuildID()
	if err == nil {
		t.Fatal("expected newFastRebuildID to return an error")
	}
	if !strings.Contains(err.Error(), "generate build ID") {
		t.Fatalf("error = %q, expected generate-build-id context", err)
	}
}

func TestShouldRunFastRebuildArtifactWriteForBuildID(t *testing.T) {
	if !shouldRunFastRebuildArtifactWriteForBuildID("build", "build") {
		t.Fatal("expected artifact write to run when build IDs match")
	}
	if shouldRunFastRebuildArtifactWriteForBuildID(
		"build-current",
		"build-expected",
	) {
		t.Fatal("expected artifact write to skip when build IDs differ")
	}
}

func TestShouldRestoreFastRebuildManifestSnapshotForBuildID(t *testing.T) {
	if !shouldRestoreFastRebuildManifestSnapshotForBuildID("build", "build") {
		t.Fatal("expected manifest snapshot restore when build IDs match")
	}
	if shouldRestoreFastRebuildManifestSnapshotForBuildID(
		"build-current",
		"build-expected",
	) {
		t.Fatal(
			"expected manifest snapshot restore to skip when build IDs differ",
		)
	}
}
