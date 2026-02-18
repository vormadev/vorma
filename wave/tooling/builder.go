// Package tooling contains Wave build-time and development-time orchestration.
//
// It is intentionally separate from package wave runtime APIs so production
// binaries can depend on runtime functionality without pulling in build/dev
// tool dependencies.
//
// Major responsibilities include:
// - static asset processing and file mapping
// - CSS/Vite build integration
// - devserver lifecycle, watch pipelines, and restart orchestration
// - config validation and schema generation
//
// Internal subpackages define explicit boundaries for isolated concerns. For
// example, watcher pre/post classification decisions live in
// internal/watchereventclassification.
package tooling

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
)

// Builder handles build operations. It is safe to reuse across multiple builds.
type Builder struct {
	cfg *wave.ParsedConfig
	log *slog.Logger
	css *cssProcessor
}

// BuildOpts configures a build
type BuildOpts struct {
	CompileGo    bool
	IsDev        bool
	IsRebuild    bool
	FileOnlyMode bool // Skip hooks and binary
}

// NewBuilder creates a new Builder
func NewBuilder(cfg *wave.ParsedConfig, log *slog.Logger) *Builder {
	if log == nil {
		log = colorlog.New("wave")
	}

	b := &Builder{
		cfg: cfg,
		log: log,
	}
	b.css = newCSSProcessor(cfg, log, b)
	return b
}

// Close releases resources held by the builder (e.g., esbuild contexts).
// Should be called when the builder is no longer needed.
func (b *Builder) Close() error {
	if b.css != nil {
		return b.css.close()
	}
	return nil
}

// Config returns a defensive read-only config snapshot.
// Unstable internal callback/schema fields are omitted.
func (b *Builder) Config() *wave.ParsedConfig {
	return b.cfg.Clone()
}

// RegisterSchemaSection adds a custom section to the generated JSON schema.
// This allows frameworks to extend wave.config.json with their own configuration
// while maintaining IDE autocomplete support.
func (b *Builder) RegisterSchemaSection(
	name string,
	schema jsonschema.Entry,
) {
	if b.cfg.FrameworkSchemaExtensions == nil {
		b.cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	b.cfg.FrameworkSchemaExtensions[name] = schema
}

// ViteProdBuild runs a Vite production build
func (b *Builder) ViteProdBuild() error {
	if !b.cfg.UsingVite() {
		return nil
	}
	return b.viteCtx().ProdBuild()
}

func (b *Builder) viteCtx() *vitecmd.BuildCtx {
	return vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{
		JSPackageManagerBaseCmd: b.cfg.Vite.JSPackageManagerBaseCmd,
		JSPackageManagerCmdDir:  b.cfg.Vite.JSPackageManagerCmdDir,
		OutDir:                  b.cfg.Dist.StaticPublic(),
		ManifestOut:             b.cfg.ViteManifestPath(),
		ViteConfigFile:          b.cfg.Vite.ViteConfigFile,
		DefaultPort:             b.cfg.Vite.DefaultPort,
	})
}

// NewViteDevContext creates a new Vite dev context
func (b *Builder) NewViteDevContext() (*vitecmd.BuildCtx, error) {
	if !b.cfg.UsingVite() {
		return nil, nil
	}
	ctx := b.viteCtx()
	if err := ctx.DevBuild(); err != nil {
		return nil, err
	}
	return ctx, nil
}

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
	return os.WriteFile(
		keepPath,
		[]byte("//go:embed directives require at least one file to compile\n"),
		0o644,
	)
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
func (b *Builder) ReadCriticalCSSForHotReload(
	requireFreshBuildOutput bool,
) (string, error) {
	return b.css.readCriticalCSSHotReloadOutput(requireFreshBuildOutput)
}

// ReadNormalCSSURLForHotReload reads the normal CSS URL for browser hot reload.
// When requireFreshBuildOutput is true, stale fallback reads from dist are disabled.
func (b *Builder) ReadNormalCSSURLForHotReload(
	requireFreshBuildOutput bool,
) (string, error) {
	return b.css.readNormalCSSHotReloadURL(requireFreshBuildOutput)
}

// getPublicURLBuildtimeCached resolves a public URL using cached file map (for CSS builds).
// Panics if the file map cannot be loaded or if the lookup misses.
func (b *Builder) getPublicURLBuildtimeCached(original string) string {
	b.css.cachedFileMapMu.Lock()
	defer b.css.cachedFileMapMu.Unlock()

	if b.css.cachedFileMap == nil {
		fm, err := b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
		if err != nil {
			panic(fmt.Errorf("load file map for CSS URL resolution: %w", err))
		}
		b.css.cachedFileMap = fm
	}

	url, found := b.css.cachedFileMap.Lookup(original, b.cfg.PublicPathPrefix())
	if !found {
		panic(fmt.Errorf("no hashed URL found for %q", original))
	}

	return url
}
