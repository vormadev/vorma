package pathnorm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAbsoluteDirectory(t *testing.T) {
	root := t.TempDir()
	configDirectoryPath := filepath.Join(root, "backend")
	configFilePath := filepath.Join(configDirectoryPath, "wave.config.json")
	missingConfigPath := filepath.Join(configDirectoryPath, "missing.config.json")

	if err := os.MkdirAll(configDirectoryPath, 0o755); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	if err := os.WriteFile(configFilePath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	if got := AbsoluteDirectory(configDirectoryPath); got != configDirectoryPath {
		t.Fatalf("AbsoluteDirectory(existing dir) = %q, want %q", got, configDirectoryPath)
	}

	if got := AbsoluteDirectory(configFilePath); got != configDirectoryPath {
		t.Fatalf("AbsoluteDirectory(existing file) = %q, want %q", got, configDirectoryPath)
	}

	if got := AbsoluteDirectory(missingConfigPath); got != configDirectoryPath {
		t.Fatalf("AbsoluteDirectory(missing path) = %q, want %q", got, configDirectoryPath)
	}
}

func TestPathsReferToSameLocation(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "backend", "wave.config.json")
	equivalentConfigPath := filepath.Join(root, "backend", ".", "wave.config.json")
	otherConfigPath := filepath.Join(root, "backend", "other.config.json")

	if !PathsReferToSameLocation(configPath, equivalentConfigPath) {
		t.Fatalf("expected %q and %q to refer to same location", configPath, equivalentConfigPath)
	}

	if PathsReferToSameLocation(configPath, otherConfigPath) {
		t.Fatalf("did not expect %q and %q to refer to same location", configPath, otherConfigPath)
	}

	if PathsReferToSameLocation("", equivalentConfigPath) {
		t.Fatal("expected empty path to fail same-location comparison")
	}
}
