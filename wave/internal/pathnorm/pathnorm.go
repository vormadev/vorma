package pathnorm

import (
	"os"
	"path/filepath"
	"strings"
)

func TrimAndCleanPath(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}
	return filepath.Clean(trimmedPath)
}

func Absolute(path string) string {
	cleanedPath := TrimAndCleanPath(path)
	if cleanedPath == "" {
		return ""
	}

	absolutePath, err := filepath.Abs(cleanedPath)
	if err != nil {
		return cleanedPath
	}

	return filepath.Clean(absolutePath)
}

func AbsoluteSlash(path string) string {
	normalizedPath := Absolute(path)
	if normalizedPath == "" {
		return ""
	}

	return filepath.ToSlash(normalizedPath)
}

func AbsoluteDirectory(path string) string {
	normalizedPath := Absolute(path)
	if normalizedPath == "" {
		return ""
	}

	fileInfo, statError := os.Stat(normalizedPath)
	if statError == nil && fileInfo.IsDir() {
		return normalizedPath
	}

	return filepath.Dir(normalizedPath)
}

func PathsReferToSameLocation(pathA string, pathB string) bool {
	normalizedPathA := CanonicalizePathForLocationComparison(pathA)
	normalizedPathB := CanonicalizePathForLocationComparison(pathB)
	return normalizedPathA != "" &&
		normalizedPathB != "" &&
		normalizedPathA == normalizedPathB
}

func CanonicalizePathForLocationComparison(path string) string {
	normalizedPath := Absolute(path)
	if normalizedPath == "" {
		return ""
	}

	return canonicalizeNormalizedPathForLocationComparison(normalizedPath)
}

func canonicalizeNormalizedPathForLocationComparison(normalizedPath string) string {
	resolvedPath, resolveError := filepath.EvalSymlinks(normalizedPath)
	if resolveError == nil && resolvedPath != "" {
		return filepath.Clean(resolvedPath)
	}

	parentPath := filepath.Dir(normalizedPath)
	if parentPath != "" && parentPath != normalizedPath {
		resolvedParentPath, parentResolveError := filepath.EvalSymlinks(parentPath)
		if parentResolveError == nil && resolvedParentPath != "" {
			return filepath.Clean(filepath.Join(resolvedParentPath, filepath.Base(normalizedPath)))
		}
	}

	return normalizedPath
}
