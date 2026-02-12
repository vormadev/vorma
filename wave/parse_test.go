package wave

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfigRejectsInvalidJSON(t *testing.T) {
	_, err := ParseConfig([]byte("{"))
	if err == nil {
		t.Fatal("expected ParseConfig to fail for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("expected parse config error prefix, got %q", err)
	}
}

func TestParseConfigRequiresCoreSection(t *testing.T) {
	_, err := ParseConfig([]byte(`{"Vite":{"DefaultPort":5173}}`))
	if err == nil {
		t.Fatal("expected ParseConfig to fail when Core section is missing")
	}
	if !strings.Contains(err.Error(), "Core section is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseConfigSetsCleanDistRoot(t *testing.T) {
	fixture := newWaveTestFixture(t)
	raw := []byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"` + fixture.root + `/dist/../dist/."}}`)

	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}

	expectedDist := filepath.Clean(filepath.Join(fixture.root, "dist"))
	if cfg.Dist.Root != expectedDist {
		t.Fatalf("expected cleaned Dist root %q, got %q", expectedDist, cfg.Dist.Root)
	}
}

func TestParseConfigFileReadsAndParses(t *testing.T) {
	fixture := newWaveTestFixture(t)
	configPath := filepath.Join(fixture.root, "wave.config.json")
	if err := os.WriteFile(configPath, fixture.configJSON(t), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := ParseConfigFile(configPath)
	if err != nil {
		t.Fatalf("ParseConfigFile returned error: %v", err)
	}

	if cfg.Core.MainAppEntry != "cmd/app" {
		t.Fatalf("expected MainAppEntry to be parsed, got %q", cfg.Core.MainAppEntry)
	}
}

func TestParseConfigFileReturnsReadError(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing-wave-config.json")
	_, err := ParseConfigFile(missingPath)
	if err == nil {
		t.Fatal("expected ParseConfigFile to fail for missing file")
	}
	if !strings.Contains(err.Error(), "read config file") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}
