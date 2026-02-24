package vormabuild

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
)

func newBuildArtifactFileSystemExecutorForTest(
	mutateDependencies func(*buildArtifactFileSystemExecutorDependencies),
) buildArtifactFileSystemExecutor {
	dependencies := buildArtifactFileSystemExecutorDependencies{}
	if mutateDependencies != nil {
		mutateDependencies(&dependencies)
	}
	return newBuildArtifactFileSystemExecutor(dependencies)
}

func TestCleanStaticPublicOutDir_IgnoresMissingDirectory(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}

	if err := cleanStaticPublicOutDir(app); err != nil {
		t.Fatalf(
			"cleanStaticPublicOutDir should ignore missing directory, got: %v",
			err,
		)
	}
}

func TestCleanStaticPublicOutDir_ReturnsErrorWhenPathIsNotDirectory(
	t *testing.T,
) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if err := os.RemoveAll(fixture.publicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}
	mustWriteFile(t, fixture.publicDir, []byte("not a directory"))

	err := cleanStaticPublicOutDir(app)
	if err == nil {
		t.Fatal("expected cleanStaticPublicOutDir to return error")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("error = %q, expected not-a-directory context", err)
	}
}

func TestCleanStaticPublicOutDir_ReturnsUnexpectedStatError(t *testing.T) {
	expectedErr := errors.New("stat failed")
	executor := newBuildArtifactFileSystemExecutorForTest(
		func(dependencies *buildArtifactFileSystemExecutorDependencies) {
			dependencies.buildArtifactCleanupDependencies.statStaticPublicOutDir = func(
				path string,
			) (fs.FileInfo, error) {
				return nil, expectedErr
			}
		},
	)

	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	err := executor.cleanStaticPublicOutDir(app)
	if err == nil {
		t.Fatal("expected cleanStaticPublicOutDir to return stat error")
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("error = %v, expected wrapped stat error", err)
	}
}

func TestRemoveMatchingEntriesRecursively_RemovesOnlyMatchingEntries(
	t *testing.T,
) {
	rootDir := t.TempDir()
	matchingTopLevel := filepath.Join(
		rootDir,
		vormaruntime.VormaVitePrehashedFilePrefix+"top.js",
	)
	nonMatchingTopLevel := filepath.Join(rootDir, "keep-top.js")
	matchingNested := filepath.Join(
		rootDir,
		"nested",
		vormaruntime.VormaRouteManifestPrefix+"nested.json",
	)
	nonMatchingNested := filepath.Join(rootDir, "nested", "keep-nested.js")

	mustWriteFile(t, matchingTopLevel, []byte("remove top-level match"))
	mustWriteFile(t, nonMatchingTopLevel, []byte("keep top-level non-match"))
	mustWriteFile(t, matchingNested, []byte("remove nested match"))
	mustWriteFile(t, nonMatchingNested, []byte("keep nested non-match"))

	if err := removeMatchingEntriesRecursively(rootDir, shouldRemoveGeneratedStaticPublicFile); err != nil {
		t.Fatalf("removeMatchingEntriesRecursively returned error: %v", err)
	}

	if _, err := os.Stat(matchingTopLevel); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf(
			"expected matching top-level file to be removed, stat err=%v",
			err,
		)
	}
	if _, err := os.Stat(matchingNested); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf(
			"expected matching nested file to be removed, stat err=%v",
			err,
		)
	}
	if _, err := os.Stat(nonMatchingTopLevel); err != nil {
		t.Fatalf(
			"expected non-matching top-level file to remain, stat err=%v",
			err,
		)
	}
	if _, err := os.Stat(nonMatchingNested); err != nil {
		t.Fatalf(
			"expected non-matching nested file to remain, stat err=%v",
			err,
		)
	}
}

func TestRemoveMatchingEntriesRecursively_DoesNotRemoveMatchingDirectories(
	t *testing.T,
) {
	rootDir := t.TempDir()
	matchingDirectory := filepath.Join(
		rootDir,
		vormaruntime.VormaRouteManifestPrefix+"dir",
	)
	matchingDirectoryFile := filepath.Join(
		matchingDirectory,
		vormaruntime.VormaRouteManifestPrefix+"nested.json",
	)
	nonMatchingFile := filepath.Join(matchingDirectory, "keep.txt")

	mustWriteFile(
		t,
		matchingDirectoryFile,
		[]byte("remove nested matching file"),
	)
	mustWriteFile(t, nonMatchingFile, []byte("keep nested non-matching file"))

	if err := removeMatchingEntriesRecursively(rootDir, shouldRemoveGeneratedStaticPublicFile); err != nil {
		t.Fatalf("removeMatchingEntriesRecursively returned error: %v", err)
	}

	if _, err := os.Stat(matchingDirectory); err != nil {
		t.Fatalf("expected matching directory to remain, stat err=%v", err)
	}
	if _, err := os.Stat(matchingDirectoryFile); !errors.Is(
		err,
		fs.ErrNotExist,
	) {
		t.Fatalf(
			"expected matching nested file to be removed, stat err=%v",
			err,
		)
	}
	if _, err := os.Stat(nonMatchingFile); err != nil {
		t.Fatalf(
			"expected non-matching nested file to remain, stat err=%v",
			err,
		)
	}
}

func TestRemoveMatchingEntriesRecursively_PropagatesWalkAndRemoveErrors(
	t *testing.T,
) {
	t.Run("returns walk error", func(t *testing.T) {
		expectedErr := errors.New("walk failed")
		executor := newBuildArtifactFileSystemExecutorForTest(
			func(dependencies *buildArtifactFileSystemExecutorDependencies) {
				dependencies.buildArtifactCleanupDependencies.walkDirForRemoval = func(
					root string,
					walkFn filepath.WalkFunc,
				) error {
					return walkFn(root, nil, expectedErr)
				}
			},
		)

		err := executor.removeMatchingEntriesRecursively(
			t.TempDir(),
			func(string) bool { return true },
		)
		if err == nil {
			t.Fatal(
				"expected removeMatchingEntriesRecursively to return walk error",
			)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped walk error", err)
		}
	})

	t.Run("returns remove error", func(t *testing.T) {
		expectedErr := errors.New("remove failed")
		rootDir := t.TempDir()
		targetPath := filepath.Join(
			rootDir,
			vormaruntime.VormaVitePrehashedFilePrefix+"target.js",
		)
		mustWriteFile(t, targetPath, []byte("remove me"))

		executor := newBuildArtifactFileSystemExecutorForTest(
			func(dependencies *buildArtifactFileSystemExecutorDependencies) {
				dependencies.buildArtifactCleanupDependencies.removePathForCleanup = func(path string) error {
					if path != targetPath {
						t.Fatalf("remove path = %q, want %q", path, targetPath)
					}
					return expectedErr
				}
			},
		)

		err := executor.removeMatchingEntriesRecursively(
			rootDir,
			shouldRemoveGeneratedStaticPublicFile,
		)
		if err == nil {
			t.Fatal(
				"expected removeMatchingEntriesRecursively to return remove error",
			)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped remove error", err)
		}
	})
}

func TestRemoveMatchingTopLevelFiles_OnlyRemovesTopLevelMatches(t *testing.T) {
	rootDir := t.TempDir()
	matchingTopLevel := filepath.Join(
		rootDir,
		vormaruntime.VormaRouteManifestPrefix+"top.json",
	)
	nonMatchingTopLevel := filepath.Join(rootDir, "keep-top.js")
	nestedMatching := filepath.Join(
		rootDir,
		"nested",
		vormaruntime.VormaRouteManifestPrefix+"nested.json",
	)

	mustWriteFile(t, matchingTopLevel, []byte("remove top-level match"))
	mustWriteFile(t, nonMatchingTopLevel, []byte("keep top-level non-match"))
	mustWriteFile(
		t,
		nestedMatching,
		[]byte("keep nested match because only top level is scanned"),
	)

	if err := removeMatchingTopLevelFiles(rootDir, shouldRemoveGeneratedStaticPublicFile); err != nil {
		t.Fatalf("removeMatchingTopLevelFiles returned error: %v", err)
	}

	if _, err := os.Stat(matchingTopLevel); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf(
			"expected matching top-level file to be removed, stat err=%v",
			err,
		)
	}
	if _, err := os.Stat(nonMatchingTopLevel); err != nil {
		t.Fatalf(
			"expected non-matching top-level file to remain, stat err=%v",
			err,
		)
	}
	if _, err := os.Stat(nestedMatching); err != nil {
		t.Fatalf("expected nested file to remain, stat err=%v", err)
	}
}

func TestRemoveMatchingTopLevelFiles_PropagatesReadAndRemoveErrors(
	t *testing.T,
) {
	t.Run("returns read-dir error", func(t *testing.T) {
		expectedErr := errors.New("read dir failed")
		executor := newBuildArtifactFileSystemExecutorForTest(
			func(dependencies *buildArtifactFileSystemExecutorDependencies) {
				dependencies.buildArtifactCleanupDependencies.readDirEntriesForCleanup = func(
					string,
				) ([]os.DirEntry, error) {
					return nil, expectedErr
				}
			},
		)

		err := executor.removeMatchingTopLevelFiles(
			t.TempDir(),
			shouldRemoveGeneratedStaticPublicFile,
		)
		if err == nil {
			t.Fatal(
				"expected removeMatchingTopLevelFiles to return read-dir error",
			)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped read-dir error", err)
		}
	})

	t.Run("wraps remove error with file name", func(t *testing.T) {
		expectedErr := errors.New("remove failed")
		rootDir := t.TempDir()
		fileName := vormaruntime.VormaRouteManifestPrefix + "target.json"
		mustWriteFile(t, filepath.Join(rootDir, fileName), []byte("remove me"))

		executor := newBuildArtifactFileSystemExecutorForTest(
			func(dependencies *buildArtifactFileSystemExecutorDependencies) {
				dependencies.buildArtifactCleanupDependencies.removeTopLevelFileForCleanup = func(
					path string,
				) error {
					return expectedErr
				}
			},
		)

		err := executor.removeMatchingTopLevelFiles(
			rootDir,
			shouldRemoveGeneratedStaticPublicFile,
		)
		if err == nil {
			t.Fatal(
				"expected removeMatchingTopLevelFiles to return remove error",
			)
		}
		if !strings.Contains(err.Error(), "remove "+fileName) {
			t.Fatalf("error = %q, expected file-name context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped remove error", err)
		}
	})
}

func TestWritePathsToDiskStageOne(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
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
		executor := newBuildArtifactFileSystemExecutorForTest(
			func(dependencies *buildArtifactFileSystemExecutorDependencies) {
				dependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile = func(
					*runtimepaths.PathsFile,
				) ([]byte, error) {
					return nil, expectedErr
				}
				dependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory = func(
					string,
					fs.FileMode,
				) error {
					t.Fatal(
						"did not expect directory creation after marshal failure",
					)
					return nil
				}
				dependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON = func(
					string,
					[]byte,
					fs.FileMode,
				) error {
					t.Fatal("did not expect file write after marshal failure")
					return nil
				}
			},
		)

		var err error
		withLockedVorma(t, func(l *vormaruntime.LockedVorma) {
			err = executor.writePathsToDiskStageOne(l)
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
		executor := newBuildArtifactFileSystemExecutorForTest(
			func(dependencies *buildArtifactFileSystemExecutorDependencies) {
				dependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile = func(
					*runtimepaths.PathsFile,
				) ([]byte, error) {
					return []byte(`{}`), nil
				}
				dependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory = func(
					string,
					fs.FileMode,
				) error {
					return expectedErr
				}
				dependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON = func(
					string,
					[]byte,
					fs.FileMode,
				) error {
					t.Fatal(
						"did not expect file write after directory creation failure",
					)
					return nil
				}
			},
		)

		var err error
		withLockedVorma(t, func(l *vormaruntime.LockedVorma) {
			err = executor.writePathsToDiskStageOne(l)
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
		executor := newBuildArtifactFileSystemExecutorForTest(
			func(dependencies *buildArtifactFileSystemExecutorDependencies) {
				dependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile = func(
					*runtimepaths.PathsFile,
				) ([]byte, error) {
					return []byte(`{}`), nil
				}
				dependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory = func(
					string,
					fs.FileMode,
				) error {
					return nil
				}
				dependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON = func(
					string,
					[]byte,
					fs.FileMode,
				) error {
					return expectedErr
				}
			},
		)

		var err error
		withLockedVorma(t, func(l *vormaruntime.LockedVorma) {
			err = executor.writePathsToDiskStageOne(l)
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
			var capturedMode fs.FileMode
			executor := newBuildArtifactFileSystemExecutorForTest(
				func(dependencies *buildArtifactFileSystemExecutorDependencies) {
					dependencies.stageOnePathsWriteDependencies.marshalStageOnePathsFile = func(
						*runtimepaths.PathsFile,
					) ([]byte, error) {
						return []byte(`{}`), nil
					}
					dependencies.stageOnePathsWriteDependencies.makeStageOnePathsOutputDirectory = func(
						string,
						fs.FileMode,
					) error {
						return nil
					}
					dependencies.stageOnePathsWriteDependencies.writeStageOnePathsJSON = func(
						_ string,
						_ []byte,
						fileMode fs.FileMode,
					) error {
						capturedMode = fileMode
						return nil
					}
				},
			)

			withLockedVorma(t, func(l *vormaruntime.LockedVorma) {
				if err := executor.writePathsToDiskStageOne(l); err != nil {
					t.Fatalf("writePathsToDiskStageOne returned error: %v", err)
				}
			})

			if capturedMode != buildArtifactFileMode {
				t.Fatalf(
					"file mode = %v, want %v",
					capturedMode,
					buildArtifactFileMode,
				)
			}
		},
	)
}

func TestWritePathsToDiskStageOneFromRuntimeState(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	runtimeStateSnapshot := routeBuildRuntimeStateSnapshot{
		paths: map[string]*vormaruntime.Path{
			"/": {
				OriginalPattern: "/",
				SrcPath:         "frontend/src/routes/home.tsx",
				ExportKey:       "default",
			},
		},
		buildID:           "runtime-snapshot-build-id",
		routeManifestFile: "manifest-from-snapshot-state.json",
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
