package fswatcher

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDesiredWatchDirsForDotPatternWatchesRecursivelyWithExclusions(t *testing.T) {
	t.Chdir(t.TempDir())
	must_mkdir_all(t, filepath.Join("src", "nested"))
	must_mkdir_all(t, filepath.Join("node_modules", "pkg"))

	w := NewWatcher(WatcherOptions{
		WatchPatterns: []string{".", "!node_modules"},
	})

	assert_watch_dirs(t, w, []string{
		".",
		"src",
		filepath.Join("src", "nested"),
	})
}

func TestDesiredWatchDirsForFilePatternWatchesParentOnly(t *testing.T) {
	t.Chdir(t.TempDir())
	must_write_file(t, "server.go")
	must_mkdir_all(t, "nested")
	must_write_file(t, filepath.Join("nested", "server.go"))

	w := NewWatcher(WatcherOptions{
		WatchPatterns: []string{"./server.go"},
	})

	assert_watch_dirs(t, w, []string{"."})
	if !w.plan.should_emit("server.go", false) {
		t.Fatal("expected root server.go to emit")
	}
	if w.plan.should_emit(filepath.Join("nested", "server.go"), false) {
		t.Fatal("did not expect nested server.go to emit")
	}
}

func TestDesiredWatchDirsForRecursiveFilePatternWatchesMatchingSubtree(t *testing.T) {
	t.Chdir(t.TempDir())
	must_mkdir_all(t, filepath.Join("app", "content", "deep"))
	must_mkdir_all(t, "other")

	w := NewWatcher(WatcherOptions{
		WatchPatterns: []string{"app/**/*.md"},
	})

	assert_watch_dirs(t, w, []string{
		".",
		"app",
		filepath.Join("app", "content"),
		filepath.Join("app", "content", "deep"),
	})
	if !w.plan.should_emit(filepath.Join("app", "content", "post.md"), false) {
		t.Fatal("expected markdown file to emit")
	}
	if w.plan.should_emit(filepath.Join("app", "content", "post.txt"), false) {
		t.Fatal("did not expect non-markdown file to emit")
	}
}

func TestDesiredWatchDirsForSingleLevelPatternPrunesDeeperDirs(t *testing.T) {
	t.Chdir(t.TempDir())
	must_mkdir_all(t, filepath.Join("app", "nested"))

	w := NewWatcher(WatcherOptions{
		WatchPatterns: []string{"app/*.md"},
	})

	assert_watch_dirs(t, w, []string{".", "app"})
}

func TestDesiredWatchDirsKeepsReincludedPathUnderExcludedSubtree(t *testing.T) {
	t.Chdir(t.TempDir())
	must_mkdir_all(t, filepath.Join("tmp", "other", "deep"))
	must_write_file(t, filepath.Join("tmp", "keep.go"))

	w := NewWatcher(WatcherOptions{
		WatchPatterns: []string{".", "!tmp/**", "tmp/keep.go"},
	})

	assert_watch_dirs(t, w, []string{".", "tmp"})
	if !w.plan.should_emit(filepath.Join("tmp", "keep.go"), false) {
		t.Fatal("expected re-included file to emit")
	}
	if w.plan.should_emit(filepath.Join("tmp", "other", "drop.go"), false) {
		t.Fatal("did not expect excluded file to emit")
	}
}

func assert_watch_dirs(t *testing.T, w *Watcher, expected []string) {
	t.Helper()

	actual_set, err := w.desired_watch_dirs()
	if err != nil {
		t.Fatalf("error getting desired watch dirs: %v", err)
	}
	actual := actual_set.Slice()
	slices.Sort(actual)
	slices.Sort(expected)

	if !slices.Equal(actual, expected) {
		t.Fatalf("expected watch dirs %v, got %v", expected, actual)
	}
}

func must_mkdir_all(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("error creating directory %q: %v", path, err)
	}
}

func must_write_file(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("error creating parent directory for %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("error writing file %q: %v", path, err)
	}
}
