package wave

import (
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

func TestParseConfigFileSetsConfigLocationToAbsolutePath(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "backend", "wave.config.json")

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create config parent directory: %v", err)
	}
	if err := os.WriteFile(
		configPath,
		[]byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`),
		0o644,
	); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := ParseConfigFile(configPath)
	if err != nil {
		t.Fatalf("ParseConfigFile returned error: %v", err)
	}

	expectedConfigLocation := filepath.Clean(configPath)
	if cfg.Core.ConfigLocation != expectedConfigLocation {
		t.Fatalf(
			"expected config location %q, got %q",
			expectedConfigLocation,
			cfg.Core.ConfigLocation,
		)
	}
}

func TestParseConfigFileRejectsEmptyPath(t *testing.T) {
	_, err := ParseConfigFile("   ")
	if err == nil {
		t.Fatal("expected ParseConfigFile to fail for empty config path")
	}
	if !strings.Contains(err.Error(), "config file path is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
