package waveshared

import (
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
)

// ResolveFromReferencedPath resolves a referenced asset path under publicPathPrefix.
// It trims, normalizes, and rejects empty/effectively-empty referenced paths.
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
