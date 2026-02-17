package wave

import (
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
)

// ResolvePublicURLFromReferencedPath normalizes a dist-relative referenced path
// and joins it under the provided public URL prefix.
func ResolvePublicURLFromReferencedPath(
	publicPathPrefix string,
	referencedPath string,
) string {
	normalizedReferencedPath := normalizeReferencedPathForPublicURLResolution(
		referencedPath,
	)
	if normalizedReferencedPath == "" {
		return ""
	}

	return matcher.EnsureLeadingSlash(
		path.Join(publicPathPrefix, normalizedReferencedPath),
	)
}

func IsPassthroughPublicURL(originalURL string) bool {
	lowerOriginalURL := strings.ToLower(originalURL)

	return strings.HasPrefix(lowerOriginalURL, "data:") ||
		strings.HasPrefix(lowerOriginalURL, "http://") ||
		strings.HasPrefix(lowerOriginalURL, "https://") ||
		strings.HasPrefix(lowerOriginalURL, "ws://") ||
		strings.HasPrefix(lowerOriginalURL, "wss://") ||
		strings.HasPrefix(lowerOriginalURL, "blob:") ||
		strings.HasPrefix(lowerOriginalURL, "file:") ||
		strings.HasPrefix(originalURL, "//")
}

func normalizeReferencedPathForPublicURLResolution(
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

	return normalizedReferencedPath
}
