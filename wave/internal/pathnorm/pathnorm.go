package pathnorm

import (
	"os"
	"path/filepath"
	"strings"
)

func Absolute(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}

	absolutePath, err := filepath.Abs(trimmedPath)
	if err != nil {
		return filepath.Clean(trimmedPath)
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
	normalizedPathA := Absolute(pathA)
	normalizedPathB := Absolute(pathB)
	return normalizedPathA != "" &&
		normalizedPathB != "" &&
		normalizedPathA == normalizedPathB
}
