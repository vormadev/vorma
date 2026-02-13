package wave

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseConfigFromLoadedConfig(t *testing.T) {
	parsedConfig, err := ParseConfigFromLoadedConfig(
		&LoadedConfig{
			ConfigJSON: []byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`),
			Dependencies: ConfigProviderDependencies{
				Files: []string{"backend/wave.config.go"},
				Globs: []string{"backend/**/*.go"},
				Env:   []string{"WAVE_MODE"},
			},
			Fingerprint: "fingerprint-value",
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ParseConfigFromLoadedConfig returned error: %v", err)
	}

	if got, want := parsedConfig.GetResolvedConfigDependencies().Files, []string{"backend/wave.config.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("resolved config dependency files = %v, want %v", got, want)
	}
	if got := parsedConfig.GetResolvedConfigFingerprint(); got != "fingerprint-value" {
		t.Fatalf("resolved config fingerprint = %q, want fingerprint-value", got)
	}
}

func TestParseConfigFromLoadedConfigFailsForInvalidDependencyGlobPattern(
	t *testing.T,
) {
	_, err := ParseConfigFromLoadedConfig(
		&LoadedConfig{
			ConfigJSON: []byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`),
			Dependencies: ConfigProviderDependencies{
				Globs: []string{"["},
			},
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected ParseConfigFromLoadedConfig to fail for invalid dependency glob pattern")
	}
	if !strings.Contains(err.Error(), "invalid config dependency glob pattern") {
		t.Fatalf("unexpected error for invalid dependency glob pattern: %v", err)
	}
}
