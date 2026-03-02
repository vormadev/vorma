package waveglob_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/internal/waveglob"
)

func TestValidateNamedGlobPatternInput(t *testing.T) {
	if validationError := waveglob.ValidateNamedGlobPatternInput(
		"watcher setup",
		"Watch.Include[0].Pattern",
		"",
	); validationError == nil {
		t.Fatal("expected empty glob pattern to fail validation")
	}

	if validationError := waveglob.ValidateNamedGlobPatternInput(
		"watcher setup",
		"Watch.Include[0].Pattern",
		" **/*.go",
	); validationError == nil {
		t.Fatal(
			"expected surrounding whitespace in glob pattern to fail validation",
		)
	}

	if validationError := waveglob.ValidateNamedGlobPatternInput(
		"watcher setup",
		"Watch.Include[0].Pattern",
		"[",
	); validationError == nil {
		t.Fatal("expected invalid glob pattern to fail validation")
	}

	if validationError := waveglob.ValidateNamedGlobPatternInput(
		"watcher setup",
		"Watch.Include[0].Pattern",
		"**/*.go",
	); validationError != nil {
		t.Fatalf(
			"expected valid glob pattern to pass validation: %v",
			validationError,
		)
	}
}

func TestNormalizeGlobPatternForMatching_PreservesEscapedMetaCharacters(
	t *testing.T,
) {
	normalizedPattern := waveglob.NormalizeGlobPatternForMatching(
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
	matched := waveglob.MatchPathAgainstGlob(
		"/tmp/[watch-root]/notes.txt",
		"/tmp/\\[watch-root\\]/**/*.txt",
	)
	if !matched {
		t.Fatal("expected escaped literal watch-root glob to match target path")
	}
}

func TestIsPathWithinDirectory(t *testing.T) {
	root := t.TempDir()
	childPath := filepath.Join(root, "nested", "file.txt")
	if !waveglob.IsPathWithinDirectory(childPath, root) {
		t.Fatalf("expected %q to be within %q", childPath, root)
	}
	if waveglob.IsPathWithinDirectory(root, childPath) {
		t.Fatalf("did not expect %q to be within %q", root, childPath)
	}
}
