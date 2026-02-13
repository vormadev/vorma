package wave

import (
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
