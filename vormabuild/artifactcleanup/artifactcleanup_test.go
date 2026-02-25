package artifactcleanup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

func newCleanupExecutorForTest(
	mutateDependencies func(*cleanupDependencies),
) cleanupExecutor {
	dependencies := cleanupDependencies{}
	if mutateDependencies != nil {
		mutateDependencies(&dependencies)
	}
	return newCleanupExecutor(dependencies)
}

func TestCleanStaticPublicOutDir_RemovesGeneratedPrefixedFiles(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	routeManifest := filepath.Join(
		fixture.PublicDir,
		vormaruntime.VormaRouteManifestPrefix+"x.json",
	)
	viteOutput := filepath.Join(
		fixture.PublicDir,
		vormaruntime.VormaVitePrehashedFilePrefix+"bundle.js",
	)
	keepRoot := filepath.Join(fixture.PublicDir, "logo.svg")
	keepNested := filepath.Join(fixture.PublicDir, "nested", "keep.txt")
	testkit.MustWriteFile(t, routeManifest, []byte("{}"))
	testkit.MustWriteFile(t, viteOutput, []byte("console.log('x')"))
	testkit.MustWriteFile(t, keepRoot, []byte("<svg/>"))
	testkit.MustWriteFile(t, keepNested, []byte("ok"))

	if err := CleanStaticPublicOutDir(app); err != nil {
		t.Fatalf("CleanStaticPublicOutDir returned error: %v", err)
	}

	if _, err := os.Stat(routeManifest); !os.IsNotExist(err) {
		t.Fatalf("expected route manifest to be removed, stat error: %v", err)
	}
	if _, err := os.Stat(viteOutput); !os.IsNotExist(err) {
		t.Fatalf(
			"expected vite prefixed file to be removed, stat error: %v",
			err,
		)
	}
	if _, err := os.Stat(keepRoot); err != nil {
		t.Fatalf(
			"expected non-prefixed root file to remain, stat error: %v",
			err,
		)
	}
	if _, err := os.Stat(keepNested); err != nil {
		t.Fatalf(
			"expected nested non-prefixed file to remain, stat error: %v",
			err,
		)
	}
}

func TestCleanStaticPublicOutDir_IgnoresMissingDirectory(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	if err := os.RemoveAll(fixture.PublicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}

	if err := CleanStaticPublicOutDir(app); err != nil {
		t.Fatalf(
			"CleanStaticPublicOutDir should ignore missing directory, got: %v",
			err,
		)
	}
}

func TestCleanStaticPublicOutDir_ReturnsErrorWhenPathIsNotDirectory(
	t *testing.T,
) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	if err := os.RemoveAll(fixture.PublicDir); err != nil {
		t.Fatalf("remove public dir: %v", err)
	}
	testkit.MustWriteFile(t, fixture.PublicDir, []byte("not a directory"))

	err := CleanStaticPublicOutDir(app)
	if err == nil {
		t.Fatal("expected CleanStaticPublicOutDir to return error")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("error = %q, expected not-a-directory context", err)
	}
}

func TestCleanStaticPublicOutDir_ReturnsUnexpectedStatError(t *testing.T) {
	expectedErr := errors.New("stat failed")
	executor := newCleanupExecutorForTest(
		func(dependencies *cleanupDependencies) {
			dependencies.statStaticPublicOutDir = func(path string) (fs.FileInfo, error) {
				return nil, expectedErr
			}
		},
	)

	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App

	err := executor.cleanStaticPublicOutDir(app)
	if err == nil {
		t.Fatal("expected CleanStaticPublicOutDir to return stat error")
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

	testkit.MustWriteFile(t, matchingTopLevel, []byte("remove top-level match"))
	testkit.MustWriteFile(
		t,
		nonMatchingTopLevel,
		[]byte("keep top-level non-match"),
	)
	testkit.MustWriteFile(t, matchingNested, []byte("remove nested match"))
	testkit.MustWriteFile(t, nonMatchingNested, []byte("keep nested non-match"))

	if err := RemoveMatchingEntriesRecursively(rootDir, ShouldRemoveGeneratedStaticPublicFile); err != nil {
		t.Fatalf("RemoveMatchingEntriesRecursively returned error: %v", err)
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

	testkit.MustWriteFile(
		t,
		matchingDirectoryFile,
		[]byte("remove nested matching file"),
	)
	testkit.MustWriteFile(
		t,
		nonMatchingFile,
		[]byte("keep nested non-matching file"),
	)

	if err := RemoveMatchingEntriesRecursively(rootDir, ShouldRemoveGeneratedStaticPublicFile); err != nil {
		t.Fatalf("RemoveMatchingEntriesRecursively returned error: %v", err)
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
		executor := newCleanupExecutorForTest(
			func(dependencies *cleanupDependencies) {
				dependencies.walkDirForRemoval = func(
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
				"expected RemoveMatchingEntriesRecursively to return walk error",
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
		testkit.MustWriteFile(t, targetPath, []byte("remove me"))

		executor := newCleanupExecutorForTest(
			func(dependencies *cleanupDependencies) {
				dependencies.removePathForCleanup = func(path string) error {
					if path != targetPath {
						t.Fatalf("remove path = %q, want %q", path, targetPath)
					}
					return expectedErr
				}
			},
		)

		err := executor.removeMatchingEntriesRecursively(
			rootDir,
			ShouldRemoveGeneratedStaticPublicFile,
		)
		if err == nil {
			t.Fatal(
				"expected RemoveMatchingEntriesRecursively to return remove error",
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

	testkit.MustWriteFile(t, matchingTopLevel, []byte("remove top-level match"))
	testkit.MustWriteFile(
		t,
		nonMatchingTopLevel,
		[]byte("keep top-level non-match"),
	)
	testkit.MustWriteFile(
		t,
		nestedMatching,
		[]byte("keep nested match because only top level is scanned"),
	)

	if err := RemoveMatchingTopLevelFiles(rootDir, ShouldRemoveGeneratedStaticPublicFile); err != nil {
		t.Fatalf("RemoveMatchingTopLevelFiles returned error: %v", err)
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
		executor := newCleanupExecutorForTest(
			func(dependencies *cleanupDependencies) {
				dependencies.readDirEntriesForCleanup = func(
					string,
				) ([]os.DirEntry, error) {
					return nil, expectedErr
				}
			},
		)

		err := executor.removeMatchingTopLevelFiles(
			t.TempDir(),
			ShouldRemoveGeneratedStaticPublicFile,
		)
		if err == nil {
			t.Fatal(
				"expected RemoveMatchingTopLevelFiles to return read-dir error",
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
		testkit.MustWriteFile(
			t,
			filepath.Join(rootDir, fileName),
			[]byte("remove me"),
		)

		executor := newCleanupExecutorForTest(
			func(dependencies *cleanupDependencies) {
				dependencies.removeTopLevelFileForCleanup = func(
					path string,
				) error {
					return expectedErr
				}
			},
		)

		err := executor.removeMatchingTopLevelFiles(
			rootDir,
			ShouldRemoveGeneratedStaticPublicFile,
		)
		if err == nil {
			t.Fatal(
				"expected RemoveMatchingTopLevelFiles to return remove error",
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
