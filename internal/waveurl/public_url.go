package waveurl

import (
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
)

func ResolveFromReferencedPath(
	publicPathPrefix string,
	referencedPath string,
) string {
	trimmedReferencedPath := strings.TrimSpace(referencedPath)
	if trimmedReferencedPath == "" {
		return ""
	}

	normalizedReferencedPath := strings.TrimPrefix(
		path.Clean("/"+trimmedReferencedPath),
		"/",
	)
	if normalizedReferencedPath == "" || normalizedReferencedPath == "." {
		return ""
	}

	return matcher.EnsureLeadingSlash(
		path.Join(publicPathPrefix, normalizedReferencedPath),
	)
}
