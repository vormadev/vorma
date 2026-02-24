package parseutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMustPackageJSONFromString_ReturnsLinesVersionLineAndVersionValue(
	t *testing.T,
) {
	content := "{\n  \"name\": \"demo\",\n  \"version\": \"1.2.3\",\n  \"private\": true\n}\n"

	lines, versionLineIndex, currentVersion := MustPackageJSONFromString(
		content,
	)

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

	_, versionLineIndex, currentVersion := MustPackageJSONFromFile(
		packageJSONPath,
	)
	if versionLineIndex != 1 {
		t.Fatalf("version line index = %d, want %d", versionLineIndex, 1)
	}
	if currentVersion != "4.5.6" {
		t.Fatalf("current version = %q, want %q", currentVersion, "4.5.6")
	}
}

func TestMustPackageJSONFromString_PanicsWhenVersionLineMissing(t *testing.T) {
	expectParseutilPanicContaining(
		t,
		"version line not found",
		func() {
			_, _, _ = MustPackageJSONFromString("{\n  \"name\": \"demo\"\n}\n")
		},
	)
}

func TestMustPackageJSONFromString_PanicsWhenVersionIsNotString(t *testing.T) {
	expectParseutilPanicContaining(
		t,
		"version must be a string",
		func() {
			_, _, _ = MustPackageJSONFromString("{\n  \"version\": 123\n}\n")
		},
	)
}

func expectParseutilPanicContaining(
	t *testing.T,
	expectedMessageSubstring string,
	run func(),
) {
	t.Helper()

	defer func() {
		recoveredValue := recover()
		if recoveredValue == nil {
			t.Fatalf(
				"expected panic containing %q, but function did not panic",
				expectedMessageSubstring,
			)
		}
		panicMessage := recoveredValueToParseutilPanicMessage(recoveredValue)
		if !strings.Contains(panicMessage, expectedMessageSubstring) {
			t.Fatalf(
				"panic message = %q, expected to contain %q",
				panicMessage,
				expectedMessageSubstring,
			)
		}
	}()

	run()
}

func recoveredValueToParseutilPanicMessage(recoveredValue any) string {
	switch recoveredValueTyped := recoveredValue.(type) {
	case string:
		return recoveredValueTyped
	case error:
		return recoveredValueTyped.Error()
	default:
		return "non-string panic value"
	}
}
