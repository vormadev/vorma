package wave

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type resolvedConfigDependencyMatcher struct {
	configDependencyFileSet      map[string]struct{}
	configDependencyGlobPatterns []string
}

func newResolvedConfigDependencyMatcher(
	configDependencies ConfigProviderDependencies,
) (*resolvedConfigDependencyMatcher, error) {
	matcher := &resolvedConfigDependencyMatcher{
		configDependencyFileSet: make(map[string]struct{}),
	}

	for _, configDependencyFilePath := range normalizeProviderDependencyValues(configDependencies.Files) {
		normalizedConfigDependencyFilePath := normalizeConfigDependencyPathForMatch(configDependencyFilePath)
		if normalizedConfigDependencyFilePath == "" {
			continue
		}
		matcher.configDependencyFileSet[normalizedConfigDependencyFilePath] = struct{}{}
	}

	for _, configDependencyGlobPattern := range normalizeProviderDependencyValues(configDependencies.Globs) {
		normalizedConfigDependencyGlobPattern := normalizeConfigDependencyPathForMatch(configDependencyGlobPattern)
		if normalizedConfigDependencyGlobPattern == "" {
			continue
		}
		if !doublestar.ValidatePattern(normalizedConfigDependencyGlobPattern) {
			return nil, fmt.Errorf(
				"invalid config dependency glob pattern %q",
				configDependencyGlobPattern,
			)
		}
		matcher.configDependencyGlobPatterns = append(
			matcher.configDependencyGlobPatterns,
			normalizedConfigDependencyGlobPattern,
		)
	}

	return matcher, nil
}

func (matcher *resolvedConfigDependencyMatcher) matchesPath(
	path string,
) bool {
	if matcher == nil {
		return false
	}

	normalizedPath := normalizeConfigDependencyPathForMatch(path)
	if normalizedPath == "" {
		return false
	}

	if _, found := matcher.configDependencyFileSet[normalizedPath]; found {
		return true
	}

	for _, configDependencyGlobPattern := range matcher.configDependencyGlobPatterns {
		if doublestar.MatchUnvalidated(configDependencyGlobPattern, normalizedPath) {
			return true
		}
	}

	return false
}

func normalizeConfigDependencyPathForMatch(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}

	absolutePath, err := filepath.Abs(trimmedPath)
	if err == nil {
		trimmedPath = absolutePath
	}

	return filepath.ToSlash(filepath.Clean(trimmedPath))
}
