package watch_test

import (
	"github.com/vormadev/vorma/internal/wavetest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/buildtime/internal/watch"
)

func TestWatcherAddDir_AddsNonIgnoredDirectoriesAndSkipsIgnoredOnes(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)

	projectRoot := filepath.Join(root, "project")
	srcDir := filepath.Join(projectRoot, "src")
	gitDir := filepath.Join(projectRoot, ".git")
	if err := os.MkdirAll(filepath.Join(srcDir, "nested"), 0755); err != nil {
		t.Fatalf("failed creating src dirs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "objects"), 0755); err != nil {
		t.Fatalf("failed creating .git dirs: %v", err)
	}

	watcher, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	if err := watcher.AddDir(projectRoot); err != nil {
		t.Fatalf("AddDir returned error: %v", err)
	}

	if !watcher.IsWatchingDir(srcDir) {
		t.Fatalf("expected src directory to be watched: %s", srcDir)
	}
	if watcher.IsWatchingDir(gitDir) {
		t.Fatalf(
			"did not expect ignored .git directory to be watched: %s",
			gitDir,
		)
	}
}

func TestWatcherRemoveStale_RemovesDeletedDirectoriesFromWatchSet(
	t *testing.T,
) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)

	watchedDir := filepath.Join(root, "watched")
	if err := os.MkdirAll(watchedDir, 0755); err != nil {
		t.Fatalf("failed creating watched dir: %v", err)
	}

	watcher, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err != nil {
		t.Fatalf("newWatcher returned error: %v", err)
	}
	defer watcher.Close()

	if err := watcher.AddDir(watchedDir); err != nil {
		t.Fatalf("AddDir returned error: %v", err)
	}

	if !watcher.IsWatchingDir(watchedDir) {
		t.Fatalf(
			"expected watched dir to be tracked before deletion: %s",
			watchedDir,
		)
	}

	if err := os.RemoveAll(watchedDir); err != nil {
		t.Fatalf("failed deleting watched dir: %v", err)
	}

	watcher.RemoveStale()

	if watcher.IsWatchingDir(watchedDir) {
		t.Fatalf(
			"expected deleted watched dir to be removed from tracked set: %s",
			watchedDir,
		)
	}
}

func TestWatcherAddDir_FailsWhenNestedDirectoryIsNotReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based unreadable directory test is unix-specific")
	}
	if os.Geteuid() == 0 {
		t.Skip(
			"permission-denied behavior is not reliable when running as root",
		)
	}

	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(t, root)
	wavetest.SetCoreServerOnlyMode(cfg, true)

	projectRoot := filepath.Join(root, "project")
	unreadableDirectory := filepath.Join(projectRoot, "unreadable")
	if err := os.MkdirAll(unreadableDirectory, 0o755); err != nil {
		t.Fatalf("failed creating unreadable dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(unreadableDirectory, "file.txt"),
		[]byte("hello"),
		0o644,
	); err != nil {
		t.Fatalf("failed creating file inside unreadable dir: %v", err)
	}
	if err := os.Chmod(unreadableDirectory, 0o000); err != nil {
		t.Fatalf("chmod unreadable directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(unreadableDirectory, 0o755)
	})

	watcher, err := watch.NewWatcher(cfg, newDiscardLoggerForWatchTests())
	if err == nil {
		defer watcher.Close()
		t.Fatal(
			"expected NewWatcher to fail when walking unreadable nested directory",
		)
	}
	errorText := strings.ToLower(err.Error())
	if !strings.Contains(errorText, "permission denied") &&
		!strings.Contains(errorText, "operation not permitted") {
		t.Fatalf(
			"expected permission error from NewWatcher, got %v",
			err,
		)
	}
}
