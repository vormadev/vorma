package wave

import (
	"strings"
	"testing"
)

func TestParseConfigWithRuntimeMetadata(t *testing.T) {
	parsedConfig, err := ParseConfigWithRuntimeMetadata(
		[]byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`),
		"backend/wave.config.json",
	)
	if err != nil {
		t.Fatalf("ParseConfigWithRuntimeMetadata returned error: %v", err)
	}

	if got := parsedConfig.GetResolvedConfigFingerprint(); strings.TrimSpace(got) == "" {
		t.Fatalf("resolved config fingerprint is empty")
	}
	if got := parsedConfig.GetResolvedConfigFilePath(); got != "backend/wave.config.json" {
		t.Fatalf("resolved config file path = %q, want backend/wave.config.json", got)
	}
}

func TestParseConfigWithRuntimeMetadata_AllowsEmptyConfigFilePath(t *testing.T) {
	parsedConfig, err := ParseConfigWithRuntimeMetadata(
		[]byte(`{"Core":{"MainAppEntry":"cmd/app","DistDir":"dist"}}`),
		"",
	)
	if err != nil {
		t.Fatalf("ParseConfigWithRuntimeMetadata returned error: %v", err)
	}
	if got := parsedConfig.GetResolvedConfigFilePath(); got != "" {
		t.Fatalf("resolved config file path = %q, want empty", got)
	}
}
