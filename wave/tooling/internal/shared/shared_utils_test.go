package shared_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/tooling/internal/shared"
)

func TestValidateNamedGlobPatternInput(t *testing.T) {
	if validationError := shared.ValidateNamedGlobPatternInput(
		"watcher setup",
		"Watch.Include[0].Pattern",
		"",
	); validationError == nil {
		t.Fatal("expected empty glob pattern to fail validation")
	}

	if validationError := shared.ValidateNamedGlobPatternInput(
		"watcher setup",
		"Watch.Include[0].Pattern",
		" **/*.go",
	); validationError == nil {
		t.Fatal("expected surrounding whitespace in glob pattern to fail validation")
	}

	if validationError := shared.ValidateNamedGlobPatternInput(
		"watcher setup",
		"Watch.Include[0].Pattern",
		"[",
	); validationError == nil {
		t.Fatal("expected invalid glob pattern to fail validation")
	}

	if validationError := shared.ValidateNamedGlobPatternInput(
		"watcher setup",
		"Watch.Include[0].Pattern",
		"**/*.go",
	); validationError != nil {
		t.Fatalf("expected valid glob pattern to pass validation: %v", validationError)
	}
}

func TestNormalizeGlobPatternForMatching_PreservesEscapedMetaCharacters(t *testing.T) {
	normalizedPattern := shared.NormalizeGlobPatternForMatching(
		"/tmp/\\[watch-root\\]/**/*.txt",
	)
	if !strings.Contains(normalizedPattern, "\\[watch-root\\]") {
		t.Fatalf(
			"expected escaped metacharacters to be preserved, got pattern %q",
			normalizedPattern,
		)
	}
}

func TestMatchPathAgainstGlob_WithEscapedWatchRootMetacharacters(t *testing.T) {
	matched := shared.MatchPathAgainstGlob(
		"/tmp/[watch-root]/notes.txt",
		"/tmp/\\[watch-root\\]/**/*.txt",
	)
	if !matched {
		t.Fatal("expected escaped literal watch-root glob to match target path")
	}
}

func TestPathAndDirectoryHelpers(t *testing.T) {
	root := t.TempDir()
	childPath := filepath.Join(root, "nested", "file.txt")
	if !shared.IsPathWithinDirectory(childPath, root) {
		t.Fatalf("expected %q to be within %q", childPath, root)
	}
	if shared.IsPathWithinDirectory(root, childPath) {
		t.Fatalf("did not expect %q to be within %q", root, childPath)
	}

	if ensureError := shared.EnsureDirectoryForFile(childPath); ensureError != nil {
		t.Fatalf("EnsureDirectoryForFile returned error: %v", ensureError)
	}
	if _, statError := os.Stat(filepath.Dir(childPath)); statError != nil {
		t.Fatalf("expected directory to exist after EnsureDirectoryForFile: %v", statError)
	}
}

func TestWriteAndCopyFileAtomically(t *testing.T) {
	root := t.TempDir()
	destinationPath := filepath.Join(root, "out", "payload.txt")
	content := []byte("atomic-content")
	if writeError := shared.WriteFileAtomically(destinationPath, content, 0o644); writeError != nil {
		t.Fatalf("WriteFileAtomically returned error: %v", writeError)
	}

	destinationBytes, readError := os.ReadFile(destinationPath)
	if readError != nil {
		t.Fatalf("read destination file: %v", readError)
	}
	if string(destinationBytes) != string(content) {
		t.Fatalf("destination content = %q, want %q", string(destinationBytes), string(content))
	}

	copyPath := filepath.Join(root, "out", "copy.txt")
	if copyError := shared.CopyFileAtomically(destinationPath, copyPath); copyError != nil {
		t.Fatalf("CopyFileAtomically returned error: %v", copyError)
	}
	copyBytes, readCopyError := os.ReadFile(copyPath)
	if readCopyError != nil {
		t.Fatalf("read copied file: %v", readCopyError)
	}
	if string(copyBytes) != string(content) {
		t.Fatalf("copied file content = %q, want %q", string(copyBytes), string(content))
	}
}
