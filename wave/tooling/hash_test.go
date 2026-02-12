package tooling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestHashBytes_DifferentOriginalNamesProduceDifferentHashedNames(t *testing.T) {
	content := []byte("same content")

	first := hashBytes(content, "a.css")
	second := hashBytes(content, "b.css")

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

	fromFile, err := hashFile(filePath, "asset.js")
	if err != nil {
		t.Fatalf("hashFile returned error: %v", err)
	}
	fromBytes := hashBytes(content, "asset.js")

	if fromFile != fromBytes {
		t.Fatalf("hashFile=%q, hashBytes=%q; expected equal output", fromFile, fromBytes)
	}
}
