package parseutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMustPackageJSONFromString_ReturnsLinesVersionLineAndVersionValue(t *testing.T) {
	content := "{\n  \"name\": \"demo\",\n  \"version\": \"1.2.3\",\n  \"private\": true\n}\n"

	lines, versionLineIndex, currentVersion := MustPackageJSONFromString(content)

	if len(lines) == 0 {
		t.Fatal("expected non-empty lines slice")
	}
	if versionLineIndex != 2 {
		t.Fatalf("version line index = %d, want %d", versionLineIndex, 2)
	}
	if currentVersion != "1.2.3" {
		t.Fatalf("current version = %q, want %q", currentVersion, "1.2.3")
	}
}

func TestMustPackageJSONFromFile_ReadsVersionDataFromDisk(t *testing.T) {
	tempDir := t.TempDir()
	packageJSONPath := filepath.Join(tempDir, "package.json")
	content := "{\n  \"version\": \"4.5.6\"\n}\n"

	if writeErr := os.WriteFile(packageJSONPath, []byte(content), 0o644); writeErr != nil {
		t.Fatalf("os.WriteFile() error = %v", writeErr)
	}

	_, versionLineIndex, currentVersion := MustPackageJSONFromFile(packageJSONPath)
	if versionLineIndex != 1 {
		t.Fatalf("version line index = %d, want %d", versionLineIndex, 1)
	}
	if currentVersion != "4.5.6" {
		t.Fatalf("current version = %q, want %q", currentVersion, "4.5.6")
	}
}
