package classification_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/wave/wavedev/internal/watch/classification"
)

func TestIsNonEmptyChmodOnly(t *testing.T) {
	root := t.TempDir()

	nonEmpty := filepath.Join(root, "non-empty.txt")
	if err := os.WriteFile(nonEmpty, []byte("data"), 0644); err != nil {
		t.Fatalf("failed writing %s: %v", nonEmpty, err)
	}

	empty := filepath.Join(root, "empty.txt")
	if err := os.WriteFile(empty, nil, 0644); err != nil {
		t.Fatalf("failed writing %s: %v", empty, err)
	}

	if !classification.IsNonEmptyChmodOnly(fsnotify.Event{
		Name: nonEmpty,
		Op:   fsnotify.Chmod,
	}) {
		t.Fatal(
			"expected non-empty chmod-only event to be treated as chmod-only",
		)
	}

	if classification.IsNonEmptyChmodOnly(fsnotify.Event{
		Name: empty,
		Op:   fsnotify.Chmod,
	}) {
		t.Fatal(
			"expected empty-file chmod event to not be treated as chmod-only",
		)
	}

	if classification.IsNonEmptyChmodOnly(fsnotify.Event{
		Name: nonEmpty,
		Op:   fsnotify.Write,
	}) {
		t.Fatal("expected write event to not be treated as chmod-only")
	}
}
