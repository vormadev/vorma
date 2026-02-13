package pathnorm

import (
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
