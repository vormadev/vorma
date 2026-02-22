package watch_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/wave/tooling/internal/watch"
)

func TestWatcherAddDir_AddsNonIgnoredDirectoriesAndSkipsIgnoredOnes(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

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
		t.Fatalf("did not expect ignored .git directory to be watched: %s", gitDir)
	}
}

func TestWatcherRemoveStale_RemovesDeletedDirectoriesFromWatchSet(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForWatchTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true
	cfg.Dist.Root = cfg.Core.DistDir

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
		t.Fatalf("expected watched dir to be tracked before deletion: %s", watchedDir)
	}

	if err := os.RemoveAll(watchedDir); err != nil {
		t.Fatalf("failed deleting watched dir: %v", err)
	}

	watcher.RemoveStale()

	if watcher.IsWatchingDir(watchedDir) {
		t.Fatalf("expected deleted watched dir to be removed from tracked set: %s", watchedDir)
	}
}
