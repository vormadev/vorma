// Package waveglob owns path normalization and glob matching semantics used by
// Wave build/dev watchers.
//
// This centralizes glob behavior so builder validation and watcher matching use
// one consistent interpretation.
package waveglob

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// ValidateNamedGlobPatternInput validates one labeled glob field.
func ValidateNamedGlobPatternInput(
	validationContextLabel string,
	fieldPath string,
	globPattern string,
) error {
	if strings.TrimSpace(globPattern) == "" {
		return fmt.Errorf(
			"%s: %s is required",
			validationContextLabel,
			fieldPath,
		)
	}
	if strings.TrimSpace(globPattern) != globPattern {
		return fmt.Errorf(
			"%s: %s must not include surrounding whitespace",
			validationContextLabel,
			fieldPath,
		)
	}

	normalizedGlobPattern := strings.ReplaceAll(globPattern, "\\", "/")
	if !doublestar.ValidatePattern(normalizedGlobPattern) {
		return fmt.Errorf(
			"%s: %s must be a valid glob pattern; got %q",
			validationContextLabel,
			fieldPath,
			globPattern,
		)
	}
	return nil
}

// NormalizePathForMatching normalizes filesystem paths for glob matching.
func NormalizePathForMatching(pathForNormalization string) string {
	if strings.TrimSpace(pathForNormalization) == "" {
		return ""
	}
	normalizedPath := filepath.Clean(pathForNormalization)
	return strings.ReplaceAll(normalizedPath, "\\", "/")
}

// NormalizeGlobPatternForMatching normalizes glob patterns for doublestar
// matching.
func NormalizeGlobPatternForMatching(patternForNormalization string) string {
	trimmedPattern := strings.TrimSpace(patternForNormalization)
	if trimmedPattern == "" {
		return ""
	}

	var normalizedPatternBuilder strings.Builder
	normalizedPatternBuilder.Grow(len(trimmedPattern))

	for index := 0; index < len(trimmedPattern); index++ {
		currentByte := trimmedPattern[index]
		if currentByte != '\\' {
			normalizedPatternBuilder.WriteByte(currentByte)
			continue
		}

		if index+1 < len(trimmedPattern) {
			nextByte := trimmedPattern[index+1]
			switch nextByte {
			case '\\', '*', '?', '[', ']', '{', '}':
				normalizedPatternBuilder.WriteByte('\\')
				normalizedPatternBuilder.WriteByte(nextByte)
				index++
				continue
			}
		}

		normalizedPatternBuilder.WriteByte('/')
	}

	return normalizedPatternBuilder.String()
}

// MatchPathAgainstGlob matches a normalized path against a normalized pattern.
func MatchPathAgainstGlob(pathForMatching string, globPattern string) bool {
	normalizedPath := NormalizePathForMatching(pathForMatching)
	normalizedPattern := NormalizeGlobPatternForMatching(globPattern)
	if normalizedPath == "" || normalizedPattern == "" {
		return false
	}
	matched, matchError := doublestar.PathMatch(
		normalizedPattern,
		normalizedPath,
	)
	if matchError != nil {
		return false
	}
	return matched
}

// IsPathWithinDirectory reports whether path is equal to or nested under
// directory.
func IsPathWithinDirectory(pathForCheck string, directoryPath string) bool {
	absolutePath, pathError := filepath.Abs(pathForCheck)
	if pathError != nil {
		return false
	}
	absoluteDirectory, directoryError := filepath.Abs(directoryPath)
	if directoryError != nil {
		return false
	}

	relativePath, relativeError := filepath.Rel(absoluteDirectory, absolutePath)
	if relativeError != nil {
		return false
	}
	if relativePath == "." {
		return true
	}
	return !strings.HasPrefix(relativePath, "..")
}
