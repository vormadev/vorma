// Package builder handles Wave build operations: Go compilation,
// CSS processing, static file hashing, and Vite integration.
package builder

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/kit/viteutil"
	"github.com/vormadev/vorma/wave/internal/config"
	"golang.org/x/sync/errgroup"
)

// Builder handles build operations. It is safe to reuse across multiple builds.
type Builder struct {
	cfg *config.Config
	log *slog.Logger
	css *cssProcessor
}

// Opts configures a build
type Opts struct {
	CompileGo    bool
	IsDev        bool
	IsRebuild    bool
	FileOnlyMode bool // Skip hooks and binary
}

// New creates a new Builder
func New(cfg *config.Config, log *slog.Logger) *Builder {
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

// Config returns the builder's config (read-only access)
func (b *Builder) Config() *config.Config {
	return b.cfg
}

// Build performs a full build
func (b *Builder) Build(opts Opts) error {
	start := time.Now()

	if !opts.FileOnlyMode {
		b.log.Info("START build", "compile_go", opts.CompileGo, "is_dev", opts.IsDev)
	}

	// Process static files (before hooks)
	if err := b.processFiles(opts.IsRebuild, opts.IsDev); err != nil {
		return fmt.Errorf("file processing failed: %w", err)
	}

	if opts.FileOnlyMode {
		return nil
	}

	// Run build hooks
	hookStart := time.Now()
	if err := b.runHooks(opts.IsDev); err != nil {
		return fmt.Errorf("build hook failed: %w", err)
	}
	hookDur := time.Since(hookStart)

	// Process files again (hooks may have generated files)
	if err := b.processFiles(true, opts.IsDev); err != nil {
		return fmt.Errorf("post-hook file processing failed: %w", err)
	}

	// Write config schema
	if err := writeConfigSchema(b); err != nil {
		b.log.Warn("failed to write config schema (non-fatal)", "error", err)
	}

	// Compile Go binary
	var goDur time.Duration
	if opts.CompileGo {
		goStart := time.Now()
		if err := b.compileGo(opts.IsDev); err != nil {
			return fmt.Errorf("go compilation failed: %w", err)
		}
		goDur = time.Since(goStart)
	}

	total := time.Since(start)
	b.log.Info("DONE build",
		"total", total,
		"hooks", hookDur,
		"go", goDur,
		"wave", total-hookDur-goDur,
	)

	return nil
}

func (b *Builder) processFiles(granular bool, isDev bool) error {
	if !granular {
		if err := os.RemoveAll(b.cfg.Dist.Static()); err != nil {
			return fmt.Errorf("remove dist/static: %w", err)
		}
		if err := SetupDistDir(b.cfg); err != nil {
			return fmt.Errorf("setup dist dir: %w", err)
		}
	}

	if !b.cfg.UsingBrowser() {
		return nil
	}

	// Public files first (CSS may reference them)
	if err := b.processPublicFiles(granular); err != nil {
		return fmt.Errorf("public files: %w", err)
	}

	// Private files and CSS in parallel
	var g errgroup.Group
	g.Go(func() error {
		return b.processPrivateFiles(granular)
	})
	g.Go(func() error {
		return b.css.buildAll(isDev)
	})
	return g.Wait()
}

func (b *Builder) runHooks(isDev bool) error {
	var hook string
	if isDev {
		hook = b.cfg.Core.DevBuildHook
	} else {
		hook = b.cfg.Core.ProdBuildHook
	}

	if hook == "" {
		return nil
	}

	return executil.RunShell(hook)
}

func (b *Builder) compileGo(isDev bool) error {
	start := time.Now()
	b.log.Info("Compiling Go binary...")

	dest := b.cfg.Dist.Binary()
	entry := fmt.Sprintf(".%c%s", filepath.Separator, filepath.Clean(b.cfg.Core.MainAppEntry))

	var cmd *exec.Cmd
	if isDev {
		cmd = exec.Command("go", "build", "-o", dest, entry)
	} else {
		cmd = exec.Command("go", "build", "-tags=prod", "-o", dest, entry)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}

	b.log.Info("DONE compiling Go", "duration", time.Since(start))
	return nil
}

// CompileGoOnly compiles the Go binary without running the full build
func (b *Builder) CompileGoOnly(isDev bool) error {
	return b.compileGo(isDev)
}

// ProcessFilesOnly runs file processing without hooks or binary compilation
func (b *Builder) ProcessFilesOnly(isRebuild bool, isDev bool) error {
	return b.processFiles(isRebuild, isDev)
}

// ProcessPublicFilesOnly reprocesses just the public static files (for dev hot reload)
func (b *Builder) ProcessPublicFilesOnly() error {
	return b.processPublicFiles(true)
}

// BuildCriticalCSS builds only critical CSS
func (b *Builder) BuildCriticalCSS(isDev bool) error {
	return b.css.buildCritical(isDev)
}

// BuildNormalCSS builds only normal CSS
func (b *Builder) BuildNormalCSS(isDev bool) error {
	return b.css.buildNormal(isDev)
}

// ViteProdBuild runs a Vite production build
func (b *Builder) ViteProdBuild() error {
	if !b.cfg.UsingVite() {
		return nil
	}
	return b.viteCtx().ProdBuild()
}

func (b *Builder) viteCtx() *viteutil.BuildCtx {
	return viteutil.NewBuildCtx(&viteutil.BuildCtxOptions{
		JSPackageManagerBaseCmd: b.cfg.Vite.JSPackageManagerBaseCmd,
		JSPackageManagerCmdDir:  b.cfg.Vite.JSPackageManagerCmdDir,
		OutDir:                  b.cfg.Dist.StaticPublic(),
		ManifestOut:             b.cfg.ViteManifestPath(),
		ViteConfigFile:          b.cfg.Vite.ViteConfigFile,
		DefaultPort:             b.cfg.Vite.DefaultPort,
	})
}

// NewViteDevContext creates a new Vite dev context
func (b *Builder) NewViteDevContext() (*viteutil.BuildCtx, error) {
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
func SetupDistDir(cfg *config.Config) error {
	dirs := []string{
		cfg.Dist.Internal(),
		cfg.Dist.StaticPublic(),
		cfg.Dist.StaticPrivate(),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	// Create .keep file for go:embed
	keepPath := cfg.Dist.KeepFile()
	return os.WriteFile(keepPath, []byte("//go:embed directives require at least one file to compile\n"), 0644)
}

// ReadCriticalCSS reads the critical CSS content from dist
func (b *Builder) ReadCriticalCSS() (string, error) {
	data, err := os.ReadFile(b.cfg.Dist.CriticalCSS())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadNormalCSSURL reads the normal CSS URL from the ref file
func (b *Builder) ReadNormalCSSURL() (string, error) {
	data, err := os.ReadFile(b.cfg.Dist.NormalCSSRef())
	if err != nil {
		return "", err
	}
	return b.cfg.PublicPathPrefix() + string(data), nil
}

// getPublicURLBuildtimeCached resolves a public URL using cached file map (for CSS builds).
// Panics if the file map cannot be loaded (this is build-time, not runtime).
func (b *Builder) getPublicURLBuildtimeCached(original string) string {
	b.css.cachedFileMapMu.Lock()
	defer b.css.cachedFileMapMu.Unlock()

	if b.css.cachedFileMap == nil {
		fm, err := b.loadFileMap(config.PublicFileMapGobName)
		if err != nil {
			b.log.Error("failed to load file map for CSS URL resolution", "error", err, "url", original)
			panic(err)
		}
		b.css.cachedFileMap = fm
	}

	url, found := b.css.cachedFileMap.Lookup(original, b.cfg.PublicPathPrefix())
	if !found {
		b.log.Warn("no hashed URL found", "url", original)
	}

	return url
}
