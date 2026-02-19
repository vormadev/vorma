package tooling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/builder/static"
)

func TestHashBytes_DifferentOriginalNamesProduceDifferentHashedNames(t *testing.T) {
	content := []byte("same content")

	first := static.HashBytes(content, "a.css")
	second := static.HashBytes(content, "b.css")

	if first == second {
		t.Fatalf("expected different hashed names for different original names, got %q", first)
	}
	if !strings.HasPrefix(first, wave.HashedOutputPrefix+"a_") {
		t.Fatalf("unexpected hashBytes output prefix: %q", first)
	}
	if !strings.HasSuffix(first, ".css") {
		t.Fatalf("expected .css suffix, got %q", first)
	}
}

func TestHashFile_MatchesHashBytesForSameInput(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "asset.js")
	content := []byte("console.log('hello')")

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	fromFile, err := static.HashFile(filePath, "asset.js")
	if err != nil {
		t.Fatalf("hashFile returned error: %v", err)
	}
	fromBytes := static.HashBytes(content, "asset.js")

	if fromFile != fromBytes {
		t.Fatalf("hashFile=%q, hashBytes=%q; expected equal output", fromFile, fromBytes)
	}
}
