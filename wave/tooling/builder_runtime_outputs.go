package tooling

import (
	"os"

	"github.com/vormadev/vorma/wave"
)

// SetupDistDir creates the required dist directory structure
func SetupDistDir(cfg *wave.ParsedConfig) error {
	dirs := []string{
		cfg.Dist.Internal(),
		cfg.Dist.StaticPublic(),
		cfg.Dist.StaticPrivate(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	// Create .keep file for go:embed
	keepPath := cfg.Dist.KeepFile()
	return os.WriteFile(keepPath, []byte("//go:embed directives require at least one file to compile\n"), 0o644)
}

// ReadCriticalCSS reads the critical CSS content from dist
func (b *Builder) ReadCriticalCSS() (string, error) {
	return b.ReadCriticalCSSForHotReload(false)
}

// ReadNormalCSSURL reads the normal CSS URL from the ref file
func (b *Builder) ReadNormalCSSURL() (string, error) {
	return b.ReadNormalCSSURLForHotReload(false)
}

// ReadCriticalCSSForHotReload reads critical CSS for browser hot reload.
// When requireFreshBuildOutput is true, stale fallback reads from dist are disabled.
func (b *Builder) ReadCriticalCSSForHotReload(requireFreshBuildOutput bool) (string, error) {
	return b.css.readCriticalCSSHotReloadOutput(requireFreshBuildOutput)
}

// ReadNormalCSSURLForHotReload reads the normal CSS URL for browser hot reload.
// When requireFreshBuildOutput is true, stale fallback reads from dist are disabled.
func (b *Builder) ReadNormalCSSURLForHotReload(requireFreshBuildOutput bool) (string, error) {
	return b.css.readNormalCSSHotReloadURL(requireFreshBuildOutput)
}

// getPublicURLBuildtimeCached resolves a public URL using cached file map (for CSS builds).
// Panics if the file map cannot be loaded (this is build-time, not runtime).
func (b *Builder) getPublicURLBuildtimeCached(original string) string {
	if wave.IsPassthroughPublicURL(original) {
		return original
	}

	b.css.cachedFileMapMu.Lock()
	defer b.css.cachedFileMapMu.Unlock()

	if b.css.cachedFileMap == nil {
		fm, err := b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
		if err != nil {
			b.log.Warn(
				"failed to load file map for CSS URL resolution; using fallback URL",
				"error",
				err,
				"url",
				original,
			)
			return resolvePublicURLFallback(original, b.cfg.PublicPathPrefix())
		}
		b.css.cachedFileMap = fm
	}

	url, found := b.css.cachedFileMap.Lookup(original, b.cfg.PublicPathPrefix())
	if !found {
		b.log.Warn("no hashed URL found", "url", original)
	}

	return url
}
