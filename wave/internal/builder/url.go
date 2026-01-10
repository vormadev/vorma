package builder

import (
	"fmt"
	"path"
	"sort"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/tsgen"
	"github.com/vormadev/vorma/wave/internal/config"
)

// MustGetPublicURLBuildtime resolves a public URL at build time.
// This function reads the file map from disk on each call and panics on error.
// For hot paths (like CSS URL resolution), use getPublicURLBuildtimeCached instead.
func (b *Builder) MustGetPublicURLBuildtime(original string) string {
	fm, err := b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
	if err != nil {
		b.log.Error("failed to load file map", "error", err)
		panic(err)
	}

	url, found := fm.Lookup(original, b.cfg.PublicPathPrefix())
	if !found {
		b.log.Warn("no hashed URL found", "url", original)
	}
	return url
}

// GetPublicURLBuildtime resolves a public URL at build time without panicking.
// Returns the resolved URL and any error that occurred.
func (b *Builder) GetPublicURLBuildtime(original string) (string, error) {
	fm, err := b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
	if err != nil {
		return matcher.EnsureLeadingSlash(path.Join(b.cfg.PublicPathPrefix(), original)), err
	}

	url, found := fm.Lookup(original, b.cfg.PublicPathPrefix())
	if !found {
		b.log.Warn("no hashed URL found", "url", original)
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

func (b *Builder) loadOrBuildFileMap() (config.FileMap, error) {
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
func (b *Builder) LoadPublicFileMap() (config.FileMap, error) {
	return b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
}

// AddPublicAssetKeys adds public asset keys for TypeScript generation
func (b *Builder) AddPublicAssetKeys(statements *tsgen.Statements) *tsgen.Statements {
	s := statements
	if s == nil {
		s = &tsgen.Statements{}
	}

	keys, err := b.PublicFileMapKeys()
	if err != nil {
		panic(err)
	}

	s.Serialize("const WAVE_PUBLIC_ASSETS", keys)
	s.Raw("export type WavePublicAsset", "`${\"/\" | \"\"}${(typeof WAVE_PUBLIC_ASSETS)[number]}`")

	return s
}
