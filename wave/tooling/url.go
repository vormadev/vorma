package tooling

import (
	"fmt"
	"sort"

	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
)

// MustPublicURLBuildtime resolves a public URL at build time.
// This function reads the file map from disk on each call and panics on error.
// For hot paths (like CSS URL resolution), use getPublicURLBuildtimeCached instead.
func (b *Builder) MustPublicURLBuildtime(original string) string {
	fm, err := b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
	if err != nil {
		b.log.Error("failed to load file map", "error", err)
		panic(err)
	}

	url, found := fm.Lookup(original, b.cfg.PublicPathPrefix())
	if !found {
		resolveError := fmt.Errorf("no hashed URL found for %q", original)
		b.log.Error("no hashed URL found", "url", original)
		panic(resolveError)
	}
	return url
}

// PublicURLBuildtime resolves a public URL at build time without panicking.
// Returns the resolved URL and any error that occurred.
func (b *Builder) PublicURLBuildtime(original string) (string, error) {
	fm, err := b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
	if err != nil {
		return "", err
	}

	url, found := fm.Lookup(original, b.cfg.PublicPathPrefix())
	if !found {
		return "", fmt.Errorf("no hashed URL found for %q", original)
	}
	return url, nil
}

// PublicFileMapKeys returns sorted keys of non-prehashed public files
func (b *Builder) PublicFileMapKeys() ([]string, error) {
	fm, err := b.loadOrBuildFileMap()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(fm))
	for k, v := range fm {
		if !v.IsPrehashed {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// SimplePublicFileMap returns a simple path->distName map
func (b *Builder) SimplePublicFileMap() (map[string]string, error) {
	fm, err := b.loadOrBuildFileMap()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(fm))
	for k, v := range fm {
		if !v.IsPrehashed {
			result[k] = v.DistName
		}
	}
	return result, nil
}

func (b *Builder) loadOrBuildFileMap() (wave.FileMap, error) {
	fm, err := b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
	if err != nil {
		// Try building first (use false for isDev as this is typically called at build time)
		if buildErr := b.processFiles(false, false); buildErr != nil {
			return nil, fmt.Errorf("build files: %w", buildErr)
		}
		fm, err = b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
	}
	return fm, err
}

// LoadPublicFileMap loads the public file map from disk (for build-time use)
func (b *Builder) LoadPublicFileMap() (wave.FileMap, error) {
	return b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
}

// AddPublicAssetKeys adds public asset keys for TypeScript generation.
func (b *Builder) AddPublicAssetKeys(
	statements *tsgen.Statements,
) (*tsgen.Statements, error) {
	resolvedStatements := statements
	if resolvedStatements == nil {
		resolvedStatements = &tsgen.Statements{}
	}

	keys, err := b.PublicFileMapKeys()
	if err != nil {
		return nil, fmt.Errorf("public file map keys: %w", err)
	}

	resolvedStatements.MustSerialize("const WAVE_PUBLIC_ASSETS", keys)
	resolvedStatements.Raw(
		"export type WavePublicAsset",
		"`${\"/\" | \"\"}${(typeof WAVE_PUBLIC_ASSETS)[number]}`",
	)

	return resolvedStatements, nil
}
