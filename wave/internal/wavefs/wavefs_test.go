package wavefs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vormadev/vorma/wave/internal/wavefs"
)

func TestEnsureDirectoryForFile(t *testing.T) {
	root := t.TempDir()
	childPath := filepath.Join(root, "nested", "file.txt")

	if ensureError := wavefs.EnsureDirectoryForFile(childPath); ensureError != nil {
		t.Fatalf("EnsureDirectoryForFile returned error: %v", ensureError)
	}
	if _, statError := os.Stat(filepath.Dir(childPath)); statError != nil {
		t.Fatalf(
			"expected directory to exist after EnsureDirectoryForFile: %v",
			statError,
		)
	}
}
