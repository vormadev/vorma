// Package tooling provides build-time and dev-time functionality for Wave.
// This package has heavy dependencies and should not be imported at runtime.
package tooling

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
	"golang.org/x/sync/errgroup"
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

// Config returns the builder's config (read-only access)
func (b *Builder) Config() *wave.ParsedConfig {
	return b.cfg
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

// ValidateConfig performs full validation of the Wave configuration.
// This should be called at build time before any build operations.
func ValidateConfig(cfg *wave.ParsedConfig) error {
	if cfg.Core == nil {
		return fmt.Errorf("config: Core section is required")
	}
	if cfg.Core.MainAppEntry == "" {
		return fmt.Errorf("config: Core.MainAppEntry is required")
	}
	if cfg.Core.DistDir == "" {
		return fmt.Errorf("config: Core.DistDir is required")
	}

	if !cfg.Core.ServerOnlyMode {
		if cfg.Core.StaticAssetDirs.Private == "" {
			return fmt.Errorf("config: Core.StaticAssetDirs.Private is required")
		}
		if cfg.Core.StaticAssetDirs.Public == "" {
			return fmt.Errorf("config: Core.StaticAssetDirs.Public is required")
		}
	}

	if cfg.Vite != nil {
		if cfg.Vite.JSPackageManagerBaseCmd == "" {
			return fmt.Errorf("config: Vite.JSPackageManagerBaseCmd is required")
		}
	}

	if cfg.Watch != nil {
		for i, wf := range cfg.Watch.Include {
			if err := validateWatchedFile(&wf, i); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateWatchedFile(wf *wave.WatchedFile, index int) error {
	for j, hook := range wf.OnChangeHooks {
		if strings.TrimSpace(hook.Cmd) != "" && hook.RunCombinedDevBuildHookCommands {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] cannot set both Cmd and RunCombinedDevBuildHookCommands",
				index,
				j,
			)
		}

		if !wf.RunOnChangeOnly {
			continue
		}

		// Callbacks can use any timing - they return RefreshAction to control behavior.
		// This validation only applies to command-like hooks.
		if strings.TrimSpace(hook.Cmd) == "" && !hook.RunCombinedDevBuildHookCommands {
			continue
		}

		if hook.Timing != "" && hook.Timing != wave.OnChangeStrategyPre {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] has Timing %q but RunOnChangeOnly requires all command hooks to use \"pre\" timing (the default)",
				index, j, hook.Timing,
			)
		}
	}

	return nil
}

// Build performs a full build
func (b *Builder) Build(opts BuildOpts) error {
	start := time.Now()

	// Validate config before building
	if err := ValidateConfig(b.cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

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
		// Selective cleanup: remove contents except lock files
		staticDir := b.cfg.Dist.Static()
		entries, err := os.ReadDir(staticDir)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read dist/static: %w", err)
		}
		for _, entry := range entries {
			// Preserve Wave dev lock files
			if isLockFile(entry.Name()) {
				continue
			}
			if err := os.RemoveAll(filepath.Join(staticDir, entry.Name())); err != nil {
				return fmt.Errorf("remove %s: %w", entry.Name(), err)
			}
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
	var userHook, frameworkHook string
	if isDev {
		userHook = b.cfg.Core.DevBuildHook
		frameworkHook = b.cfg.FrameworkDevBuildHook
	} else {
		userHook = b.cfg.Core.ProdBuildHook
		frameworkHook = b.cfg.FrameworkProdBuildHook
	}

	// User hooks first -- they may generate Go types used in loaders/actions
	if userHook != "" {
		if err := executil.RunShell(userHook); err != nil {
			return fmt.Errorf("user build hook failed: %w", err)
		}
	}

	// Framework hooks second -- Vorma reflects on the final Go types
	if frameworkHook != "" {
		if err := executil.RunShell(frameworkHook); err != nil {
			return fmt.Errorf("framework build hook failed: %w", err)
		}
	}

	return nil
}

func (b *Builder) compileGo(isDev bool) error {
	start := time.Now()
	b.log.Info("Compiling Go binary...")

	dest := b.cfg.Dist.Binary()
	entry := fmt.Sprintf(".%c%s", filepath.Separator, filepath.Clean(b.cfg.Core.MainAppEntry))
	cmd := buildGoBuildCommand(dest, entry, isDev)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}

	b.log.Info("DONE compiling Go", "duration", time.Since(start))
	return nil
}

func buildGoBuildCommand(
	dest string,
	entry string,
	isDev bool,
) *exec.Cmd {
	commandArguments := []string{"build"}

	if !isDev {
		commandArguments = append(commandArguments, "-tags=prod")
	}

	commandArguments = append(commandArguments, "-o", dest, entry)

	return exec.Command("go", commandArguments...)
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

func (b *Builder) processPublicFilesOnlyForChangedPaths(changedSourcePaths []string) error {
	return b.processPublicFilesForChangedPaths(changedSourcePaths)
}

// ProcessPrivateFilesOnly reprocesses just the private static files (for dev hot reload)
func (b *Builder) ProcessPrivateFilesOnly() error {
	return b.processPrivateFiles(true)
}

func (b *Builder) processPrivateFilesOnlyForChangedPaths(changedSourcePaths []string) error {
	return b.processPrivateFilesForChangedPaths(changedSourcePaths)
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
	b.css.cachedFileMapMu.Lock()
	defer b.css.cachedFileMapMu.Unlock()

	if b.css.cachedFileMap == nil {
		fm, err := b.loadFileMapFromPath(b.cfg.Dist.PublicFileMapGob())
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
