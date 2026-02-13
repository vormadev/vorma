package wave

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvedConfigDependencyMatcherMatchesFileAndGlob(
	t *testing.T,
) {
	root := t.TempDir()

	configDependencyFilePath := filepath.Join(root, "backend", "config", "config.go")
	configDependencyGlobPattern := filepath.Join(root, "backend", "config", "**/*.yaml")

	matcher, err := newResolvedConfigDependencyMatcher(
		ConfigProviderDependencies{
			Files: []string{configDependencyFilePath},
			Globs: []string{configDependencyGlobPattern},
		},
	)
	if err != nil {
		t.Fatalf("newResolvedConfigDependencyMatcher returned error: %v", err)
	}

	if !matcher.matchesPath(configDependencyFilePath) {
		t.Fatalf("expected matcher to match dependency file path: %q", configDependencyFilePath)
	}

	configDependencyGlobMatchPath := filepath.Join(root, "backend", "config", "environments", "dev.yaml")
	if !matcher.matchesPath(configDependencyGlobMatchPath) {
		t.Fatalf("expected matcher to match dependency glob path: %q", configDependencyGlobMatchPath)
	}

	nonDependencyPath := filepath.Join(root, "backend", "main.go")
	if matcher.matchesPath(nonDependencyPath) {
		t.Fatalf("expected matcher to skip non-dependency path: %q", nonDependencyPath)
	}
}

func TestResolvedConfigDependencyMatcherRejectsInvalidGlobPattern(
	t *testing.T,
) {
	_, err := newResolvedConfigDependencyMatcher(
		ConfigProviderDependencies{
			Globs: []string{"["},
		},
	)
	if err == nil {
		t.Fatal("expected invalid glob pattern error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid config dependency glob pattern") {
		t.Fatalf("unexpected error for invalid glob pattern: %v", err)
	}
}

func TestIsResolvedConfigDependencyPathWithManuallySetDependencies(
	t *testing.T,
) {
	root := t.TempDir()
	configDependencyPath := filepath.Join(root, "backend", "config", "config.go")

	parsedConfig := &ParsedConfig{
		ResolvedConfigDependencies: ConfigProviderDependencies{
			Files: []string{configDependencyPath},
		},
	}

	if !parsedConfig.IsResolvedConfigDependencyPath(configDependencyPath) {
		t.Fatalf("expected path to match manually set config dependencies: %q", configDependencyPath)
	}
}
