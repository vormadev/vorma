// Package testpath provides shared test-only path helpers.
package testpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PathRelativeToCurrentWorkingDirectory returns configuredPath as a
// current-working-directory-relative path when configuredPath is
// machine-absolute; otherwise it returns configuredPath unchanged.
func PathRelativeToCurrentWorkingDirectory(
	tb testing.TB,
	configuredPath string,
) string {
	tb.Helper()

	currentWorkingDirectory, currentWorkingDirectoryError := os.Getwd()
	if currentWorkingDirectoryError != nil {
		tb.Fatalf(
			"resolve current working directory: %v",
			currentWorkingDirectoryError,
		)
	}

	relativePath, relativePathError := PathRelativeToConfiguredWorkingDirectory(
		currentWorkingDirectory,
		configuredPath,
	)
	if relativePathError != nil {
		tb.Fatalf(
			"convert %q to current-working-directory-relative path from %q: %v",
			configuredPath,
			currentWorkingDirectory,
			relativePathError,
		)
	}
	return relativePath
}

// PathRelativeToConfiguredWorkingDirectory returns configuredPath as
// currentWorkingDirectory-relative when configuredPath is machine-absolute;
// otherwise it returns configuredPath unchanged.
func PathRelativeToConfiguredWorkingDirectory(
	currentWorkingDirectory string,
	configuredPath string,
) (string, error) {
	trimmedConfiguredPath := strings.TrimSpace(configuredPath)
	if trimmedConfiguredPath == "" || !filepath.IsAbs(trimmedConfiguredPath) {
		return configuredPath, nil
	}

	relativePath, relativePathError := filepath.Rel(
		currentWorkingDirectory,
		trimmedConfiguredPath,
	)
	if relativePathError != nil {
		return "", fmt.Errorf(
			"convert %q to current-working-directory-relative path from %q: %w",
			trimmedConfiguredPath,
			currentWorkingDirectory,
			relativePathError,
		)
	}
	return filepath.ToSlash(filepath.Clean(relativePath)), nil
}

// MustPathRelativeToConfiguredWorkingDirectory returns configuredPath as a path
// relative to currentWorkingDirectory and fails the test on conversion errors.
func MustPathRelativeToConfiguredWorkingDirectory(
	tb testing.TB,
	currentWorkingDirectory string,
	configuredPath string,
) string {
	tb.Helper()

	relativePath, relativePathError := PathRelativeToConfiguredWorkingDirectory(
		currentWorkingDirectory,
		configuredPath,
	)
	if relativePathError != nil {
		tb.Fatalf(
			"convert %q to path relative to %q: %v",
			configuredPath,
			currentWorkingDirectory,
			relativePathError,
		)
	}
	return relativePath
}

// PathRelativeToRootForJSON normalizes configuredPath for JSON fixture payloads
// that are rooted at root.
func PathRelativeToRootForJSON(
	tb testing.TB,
	root string,
	configuredPath string,
) string {
	tb.Helper()

	trimmedConfiguredPath := strings.TrimSpace(configuredPath)
	if trimmedConfiguredPath == "" {
		return ""
	}
	trimmedConfiguredPath = filepath.Clean(trimmedConfiguredPath)
	if trimmedConfiguredPath == "." {
		return "."
	}
	if !filepath.IsAbs(trimmedConfiguredPath) {
		return filepath.ToSlash(trimmedConfiguredPath)
	}

	relativePathFromRoot, relativePathError := filepath.Rel(
		root,
		trimmedConfiguredPath,
	)
	if relativePathError != nil {
		tb.Fatalf(
			"resolve path %q relative to root %q: %v",
			trimmedConfiguredPath,
			root,
			relativePathError,
		)
	}
	normalizedRelativePathFromRoot := filepath.ToSlash(
		filepath.Clean(relativePathFromRoot),
	)
	if normalizedRelativePathFromRoot == ".." ||
		strings.HasPrefix(normalizedRelativePathFromRoot, "../") {
		tb.Fatalf(
			"path %q escapes root %q after normalization: %q",
			trimmedConfiguredPath,
			root,
			normalizedRelativePathFromRoot,
		)
	}
	return normalizedRelativePathFromRoot
}
