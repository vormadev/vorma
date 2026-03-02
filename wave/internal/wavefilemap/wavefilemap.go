// Package wavefilemap models mappings from source public assets to built output
// artifacts.
package wavefilemap

import (
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
)

// FileMap maps source public asset paths to built artifact metadata.
type FileMap map[string]FileVal

// FileVal describes a single built public asset mapping entry.
type FileVal struct {
	DistName    string
	ContentHash string
	IsPrehashed bool
}

// Lookup resolves a source public asset path to its built public URL.
//
// It supports both raw lookup keys and keys that already include the configured
// public path prefix.
func (fileMap FileMap) Lookup(
	original string,
	prefix string,
) (url string, found bool) {
	normalizedOriginal := normalizePublicAssetPathForLookup(original)
	if entry, ok := fileMap[normalizedOriginal]; ok {
		return joinPublicURLPrefixAndPath(prefix, entry.DistName), true
	}

	deprefixedOriginal := trimConfiguredPublicPathPrefixFromLookupPath(
		normalizedOriginal,
		prefix,
	)
	if deprefixedOriginal != normalizedOriginal {
		if entry, ok := fileMap[deprefixedOriginal]; ok {
			return joinPublicURLPrefixAndPath(prefix, entry.DistName), true
		}
	}

	return "", false
}

// Clone returns a defensive copy of one file map.
func Clone(fileMap FileMap) FileMap {
	if len(fileMap) == 0 {
		return nil
	}

	clonedPublicFileMap := make(FileMap, len(fileMap))
	for key, value := range fileMap {
		clonedPublicFileMap[key] = value
	}

	return clonedPublicFileMap
}

func normalizePublicAssetPathForLookup(original string) string {
	return strings.TrimPrefix(path.Clean("/"+original), "/")
}

func trimConfiguredPublicPathPrefixFromLookupPath(
	normalizedLookupPath string,
	publicPathPrefix string,
) string {
	normalizedPublicPathPrefix := normalizeConfiguredPublicPathPrefixForLookup(
		publicPathPrefix,
	)
	if normalizedPublicPathPrefix == "" {
		return normalizedLookupPath
	}

	if normalizedLookupPath == normalizedPublicPathPrefix {
		return ""
	}

	normalizedPublicPathPrefixWithTrailingSlash := normalizedPublicPathPrefix + "/"
	if after, ok := strings.CutPrefix(
		normalizedLookupPath,
		normalizedPublicPathPrefixWithTrailingSlash,
	); ok {
		return after
	}

	return normalizedLookupPath
}

func normalizeConfiguredPublicPathPrefixForLookup(
	publicPathPrefix string,
) string {
	return strings.Trim(path.Clean("/"+publicPathPrefix), "/")
}

func joinPublicURLPrefixAndPath(
	publicPathPrefix string,
	publicPath string,
) string {
	return matcher.EnsureLeadingSlash(path.Join(publicPathPrefix, publicPath))
}
