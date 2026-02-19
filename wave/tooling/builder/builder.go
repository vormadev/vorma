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
// internal/classification.
package toolingbuilder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/builder/css"
	"github.com/vormadev/vorma/wave/tooling/builder/schema"
	"github.com/vormadev/vorma/wave/tooling/builder/static"
	"github.com/vormadev/vorma/wave/tooling/toolingshared"
	"golang.org/x/sync/errgroup"
)

// Builder handles build operations. It is safe to reuse across multiple builds.
type Builder struct {
	cfg    *wave.ParsedConfig
	log    *slog.Logger
	css    *css.Processor
	static *static.Processor
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
	b.static = static.NewProcessor(cfg, log)
	b.css = css.NewProcessor(cfg, log, b.static.GetPublicURLBuildtimeCached)
	return b
}

// Close releases resources held by the builder (e.g., esbuild contexts).
// Should be called when the builder is no longer needed.
func (b *Builder) Close() error {
	if b.css != nil {
		return b.css.Close()
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
	return b.css.ReadCriticalCSSHotReloadOutput(requireFreshBuildOutput)
}

// ReadNormalCSSURLForHotReload reads the normal CSS URL for browser hot reload.
// When requireFreshBuildOutput is true, stale fallback reads from dist are disabled.
func (b *Builder) ReadNormalCSSURLForHotReload(
	requireFreshBuildOutput bool,
) (string, error) {
	return b.css.ReadNormalCSSHotReloadURL(requireFreshBuildOutput)
}

// GetPublicURLBuildtimeCached resolves a public URL using cached file map (for CSS builds).
// Panics if the file map cannot be loaded or if the lookup misses.
func (b *Builder) GetPublicURLBuildtimeCached(original string) string {
	return b.static.GetPublicURLBuildtimeCached(original)
}

func (b *Builder) LoadFileMapFromPath(gobPath string) (wave.FileMap, error) {
	return b.static.LoadFileMapFromPath(gobPath)
}

func (b *Builder) SaveFileMap(fm wave.FileMap, gobPath string) error {
	return b.static.SaveFileMap(fm, gobPath)
}

func (b *Builder) SavePublicFileMapJS(fm wave.FileMap) error {
	return b.static.SavePublicFileMapJS(fm)
}

func (b *Builder) WritePublicFileMapTS(outDir string) error {
	return b.static.WritePublicFileMapTS(outDir)
}

func (b *Builder) MustPublicURLBuildtime(original string) string {
	return b.static.MustPublicURLBuildtime(original)
}

func (b *Builder) PublicURLBuildtime(original string) (string, error) {
	return b.static.PublicURLBuildtime(original)
}

func (b *Builder) PublicFileMapKeys() ([]string, error) {
	return b.static.PublicFileMapKeys()
}

func (b *Builder) SimplePublicFileMap() (map[string]string, error) {
	return b.static.SimplePublicFileMap()
}

func (b *Builder) LoadPublicFileMap() (wave.FileMap, error) {
	return b.static.LoadPublicFileMap()
}

func (b *Builder) AddPublicAssetKeys(
	statements *tsgen.Statements,
) (*tsgen.Statements, error) {
	return b.static.AddPublicAssetKeys(statements)
}

func (b *Builder) IsCriticalCSSFile(path string) bool {
	return b.css.IsCriticalFile(path)
}

func (b *Builder) IsNormalCSSFile(path string) bool {
	return b.css.IsNormalFile(path)
}

func (b *Builder) IsCSSFile(path string) bool {
	return b.css.IsCSSFile(path)
}

func (b *Builder) SetTrackedCriticalCSSImportPaths(importPaths []string) {
	b.css.SetTrackedCriticalCSSImportPaths(importPaths)
}

func (b *Builder) SetTrackedNormalCSSImportPaths(importPaths []string) {
	b.css.SetTrackedNormalCSSImportPaths(importPaths)
}

func (b *Builder) ListTrackedCriticalCSSImportPaths() []string {
	return b.css.ListTrackedCriticalCSSImportPaths()
}

func (b *Builder) CountTrackedCriticalCSSImportPaths() int {
	return b.css.CountTrackedCriticalCSSImportPaths()
}

func (b *Builder) CriticalCSSBuildContext() esbuild.BuildContext {
	return b.css.CriticalCSSBuildContext()
}

func (b *Builder) NormalCSSBuildContext() esbuild.BuildContext {
	return b.css.NormalCSSBuildContext()
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
	if err := schema.WriteConfigSchema(b.cfg); err != nil {
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
			if toolingshared.IsLockFile(entry.Name()) {
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
	if err := b.static.ProcessPublicFiles(granular); err != nil {
		return fmt.Errorf("public files: %w", err)
	}

	// Private files and CSS in parallel
	var group errgroup.Group
	group.Go(func() error {
		return b.static.ProcessPrivateFiles(granular)
	})
	group.Go(func() error {
		return b.css.BuildAll(isDev)
	})
	return group.Wait()
}

func (b *Builder) runHooks(isDev bool) error {
	var userHook, frameworkHook string
	frameworkRunBuildHook := b.cfg.FrameworkRunBuildHook
	if isDev {
		userHook = b.cfg.Core.DevBuildHook
		frameworkHook = b.cfg.FrameworkDevBuildHook
	} else {
		userHook = b.cfg.Core.ProdBuildHook
		frameworkHook = b.cfg.FrameworkProdBuildHook
	}

	// User hooks first -- they may generate Go types used in loaders/actions
	buildHookCommandTimeout := b.deriveBuildHookCommandTimeout(isDev)
	if userHook != "" {
		if err := runBuildHookCommandWithTimeout(
			userHook,
			buildHookCommandTimeout,
		); err != nil {
			return fmt.Errorf("user build hook failed: %w", err)
		}
	}

	// Framework hooks second -- Vorma reflects on the final Go types
	if frameworkRunBuildHook != nil {
		frameworkHookExecutionContext, cancelFrameworkHookExecutionContext := deriveExecutionContextWithOptionalTimeout(
			context.Background(),
			buildHookCommandTimeout,
		)
		if cancelFrameworkHookExecutionContext != nil {
			defer cancelFrameworkHookExecutionContext()
		}
		if err := frameworkRunBuildHook(frameworkHookExecutionContext, isDev); err != nil {
			return fmt.Errorf("framework build hook failed: %w", err)
		}
		return nil
	}
	if frameworkHook != "" {
		if err := runBuildHookCommandWithTimeout(
			frameworkHook,
			buildHookCommandTimeout,
		); err != nil {
			return fmt.Errorf("framework build hook failed: %w", err)
		}
	}

	return nil
}

func (b *Builder) deriveBuildHookCommandTimeout(
	isDev bool,
) time.Duration {
	if b == nil || b.cfg == nil {
		return 0
	}
	return deriveBuildHookCommandTimeoutDuration(
		b.cfg.Core,
		isDev,
	)
}

func runBuildHookCommandWithTimeout(
	buildHookCommand string,
	buildHookCommandTimeout time.Duration,
) error {
	buildHookCommandExecutionContext, cancelBuildHookCommandExecutionContext := deriveExecutionContextWithOptionalTimeout(
		context.Background(),
		buildHookCommandTimeout,
	)
	if cancelBuildHookCommandExecutionContext != nil {
		defer cancelBuildHookCommandExecutionContext()
	}

	return executil.RunShellWithContext(
		buildHookCommandExecutionContext,
		buildHookCommand,
	)
}

func (b *Builder) compileGo(isDev bool) error {
	start := time.Now()
	b.log.Info("Compiling Go binary...")

	frameworkGoBuildOverlay, err := prepareFrameworkGoBuildOverlay(b.cfg)
	if err != nil {
		return fmt.Errorf("prepare framework go build overlay: %w", err)
	}

	dest := b.cfg.Dist.Binary()
	entry := fmt.Sprintf(".%c%s", filepath.Separator, filepath.Clean(b.cfg.Core.MainAppEntry))
	goBuildOverlayPath := ""
	if frameworkGoBuildOverlay != nil {
		goBuildOverlayPath = strings.TrimSpace(frameworkGoBuildOverlay.OverlayConfigPath)
	}
	cmd := buildGoBuildCommand(dest, entry, isDev, goBuildOverlayPath)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	goBuildErr := cmd.Run()
	cleanupOverlayErr := cleanupFrameworkGoBuildOverlay(frameworkGoBuildOverlay)
	if goBuildErr != nil {
		if cleanupOverlayErr != nil {
			return fmt.Errorf(
				"go build: %w (cleanup framework go build overlay failed: %v)",
				goBuildErr,
				cleanupOverlayErr,
			)
		}
		return fmt.Errorf("go build: %w", goBuildErr)
	}
	if cleanupOverlayErr != nil {
		return fmt.Errorf("cleanup framework go build overlay: %w", cleanupOverlayErr)
	}

	b.log.Info("DONE compiling Go", "duration", time.Since(start))
	return nil
}

func prepareFrameworkGoBuildOverlay(
	parsedCfg *wave.ParsedConfig,
) (*wave.GoBuildOverlay, error) {
	if parsedCfg == nil || parsedCfg.FrameworkPrepareGoBuildOverlay == nil {
		return nil, nil
	}

	frameworkGoBuildOverlay, err := parsedCfg.FrameworkPrepareGoBuildOverlay()
	if err != nil {
		return nil, err
	}
	return frameworkGoBuildOverlay, nil
}

func cleanupFrameworkGoBuildOverlay(
	frameworkGoBuildOverlay *wave.GoBuildOverlay,
) error {
	if frameworkGoBuildOverlay == nil || frameworkGoBuildOverlay.Cleanup == nil {
		return nil
	}
	return frameworkGoBuildOverlay.Cleanup()
}

func buildGoBuildCommand(
	dest string,
	entry string,
	isDev bool,
	goBuildOverlayPath string,
) *exec.Cmd {
	commandArguments := []string{"build"}
	if strings.TrimSpace(goBuildOverlayPath) != "" {
		commandArguments = append(commandArguments, "-overlay="+goBuildOverlayPath)
	}

	if !isDev {
		commandArguments = append(commandArguments, "-tags=prod")
	}

	commandArguments = append(commandArguments, "-o", dest, entry)

	return exec.Command("go", commandArguments...)
}

// BuildGoBuildCommand prepares a go build command for dev or prod compilation.
func BuildGoBuildCommand(
	dest string,
	entry string,
	isDev bool,
	goBuildOverlayPath string,
) *exec.Cmd {
	return buildGoBuildCommand(dest, entry, isDev, goBuildOverlayPath)
}

// CompileGoOnly compiles the Go binary without running the full build
func (b *Builder) CompileGoOnly(isDev bool) error {
	return b.compileGo(isDev)
}

// ProcessFiles runs static file processing with full or granular mode.
func (b *Builder) ProcessFiles(granular bool, isDev bool) error {
	return b.processFiles(granular, isDev)
}

// ProcessFilesOnly runs file processing without hooks or binary compilation
func (b *Builder) ProcessFilesOnly(isRebuild bool, isDev bool) error {
	return b.processFiles(isRebuild, isDev)
}

// ProcessPublicFilesOnly reprocesses just the public static files (for dev hot reload)
func (b *Builder) ProcessPublicFilesOnly() error {
	return b.static.ProcessPublicFiles(true)
}

func (b *Builder) ProcessPublicFilesOnlyForChangedPaths(changedSourcePaths []string) error {
	return b.static.ProcessPublicFilesForChangedPaths(changedSourcePaths)
}

// ProcessPrivateFilesOnly reprocesses just the private static files (for dev hot reload)
func (b *Builder) ProcessPrivateFilesOnly() error {
	return b.static.ProcessPrivateFiles(true)
}

func (b *Builder) ProcessPrivateFilesOnlyForChangedPaths(changedSourcePaths []string) error {
	return b.static.ProcessPrivateFilesForChangedPaths(changedSourcePaths)
}

// RunHooks executes user and framework build hooks for dev or prod mode.
func (b *Builder) RunHooks(isDev bool) error {
	return b.runHooks(isDev)
}

// BuildAllCSS builds critical and non-critical CSS outputs.
func (b *Builder) BuildAllCSS(isDev bool) error {
	return b.css.BuildAll(isDev)
}

// BuildCriticalCSS builds only critical CSS
func (b *Builder) BuildCriticalCSS(isDev bool) error {
	return b.css.BuildCritical(isDev)
}

// BuildNormalCSS builds only normal CSS
func (b *Builder) BuildNormalCSS(isDev bool) error {
	return b.css.BuildNormal(isDev)
}

// ValidateConfig performs full validation of the Wave configuration.
// This should be called at build time before any build operations.
func ValidateConfig(cfg *wave.ParsedConfig) error {
	if cfg == nil {
		return fmt.Errorf("config: parsed config is required")
	}
	if cfg.Core == nil {
		return fmt.Errorf("config: Core section is required")
	}
	if cfg.Core.MainAppEntry == "" {
		return fmt.Errorf("config: Core.MainAppEntry is required")
	}
	if cfg.Core.DistDir == "" {
		return fmt.Errorf("config: Core.DistDir is required")
	}
	if err := validateNonNegativeTimeoutFields(
		[]timeoutFieldValidation{
			{
				fieldPath:           "Core.DevBuildHookTimeoutMilliseconds",
				timeoutMilliseconds: cfg.Core.DevBuildHookTimeoutMilliseconds,
			},
			{
				fieldPath:           "Core.ProdBuildHookTimeoutMilliseconds",
				timeoutMilliseconds: cfg.Core.ProdBuildHookTimeoutMilliseconds,
			},
		},
	); err != nil {
		return err
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
		if err := validateHealthcheckEndpoint(cfg.Watch.HealthcheckEndpoint); err != nil {
			return err
		}
		if err := validateHookStageFailurePolicy(cfg.Watch.HookStageFailurePolicy); err != nil {
			return err
		}
		if err := validateHookCommandTimeoutConfig(cfg.Watch.HookCommandTimeouts); err != nil {
			return err
		}
		if err := validateHookCallbackTimeoutConfig(cfg.Watch.HookCallbackTimeouts); err != nil {
			return err
		}

		for excludedDirectoryPatternIndex, excludedDirectoryPattern := range cfg.Watch.Exclude.Dirs {
			if err := validateWatchGlobPattern(
				fmt.Sprintf("Watch.Exclude.Dirs[%d]", excludedDirectoryPatternIndex),
				excludedDirectoryPattern,
			); err != nil {
				return err
			}
		}
		for excludedFilePatternIndex, excludedFilePattern := range cfg.Watch.Exclude.Files {
			if err := validateWatchGlobPattern(
				fmt.Sprintf("Watch.Exclude.Files[%d]", excludedFilePatternIndex),
				excludedFilePattern,
			); err != nil {
				return err
			}
		}

		for i, watchedFile := range cfg.Watch.Include {
			if err := validateWatchedFile(&watchedFile, i); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateHookStageFailurePolicy(
	hookStageFailurePolicy string,
) error {
	normalizedHookStageFailurePolicy := normalizeConfiguredHookStageFailurePolicy(
		hookStageFailurePolicy,
	)
	switch normalizedHookStageFailurePolicy {
	case "",
		configuredHookStageFailurePolicyFailOpen,
		configuredHookStageFailurePolicyFailClosed:
		return nil
	default:
		return fmt.Errorf(
			"config: Watch.HookStageFailurePolicy must be one of %q or %q",
			configuredHookStageFailurePolicyFailOpen,
			configuredHookStageFailurePolicyFailClosed,
		)
	}
}

func validateHookCallbackTimeoutConfig(
	hookCallbackTimeoutConfig wave.HookCallbackTimeoutConfig,
) error {
	return validateNonNegativeTimeoutFields(
		[]timeoutFieldValidation{
			{
				fieldPath:           "Watch.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeoutConfig.PreCallbackTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeoutConfig.ConcurrentCallbackTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeoutConfig.ConcurrentNoWaitCallbackTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeoutConfig.PostCallbackTimeoutMilliseconds,
			},
		},
	)
}

func validateHookCommandTimeoutConfig(
	hookCommandTimeoutConfig wave.HookCommandTimeoutConfig,
) error {
	return validateNonNegativeTimeoutFields(
		[]timeoutFieldValidation{
			{
				fieldPath:           "Watch.HookCommandTimeouts.PreCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeoutConfig.PreCommandTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeoutConfig.ConcurrentCommandTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeoutConfig.ConcurrentNoWaitCommandTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCommandTimeouts.PostCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeoutConfig.PostCommandTimeoutMilliseconds,
			},
		},
	)
}

type timeoutFieldValidation struct {
	fieldPath           string
	timeoutMilliseconds int
}

func validateNonNegativeTimeoutFields(
	timeoutFieldValidations []timeoutFieldValidation,
) error {
	for _, timeoutFieldValidation := range timeoutFieldValidations {
		if timeoutFieldValidation.timeoutMilliseconds < 0 {
			return fmt.Errorf(
				"config: %s must be >= 0",
				timeoutFieldValidation.fieldPath,
			)
		}
	}

	return nil
}

func validateWatchedFile(wf *wave.WatchedFile, index int) error {
	if err := validateWatchGlobPattern(
		fmt.Sprintf("Watch.Include[%d].Pattern", index),
		wf.Pattern,
	); err != nil {
		return err
	}

	for hookIndex, hook := range wf.OnChangeHooks {
		for excludedPatternIndex, excludedPattern := range hook.Exclude {
			if err := validateWatchGlobPattern(
				fmt.Sprintf(
					"Watch.Include[%d].OnChangeHooks[%d].Exclude[%d]",
					index,
					hookIndex,
					excludedPatternIndex,
				),
				excludedPattern,
			); err != nil {
				return err
			}
		}

		if hook.CommandTimeoutMilliseconds < 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d].CommandTimeoutMilliseconds must be >= 0",
				index,
				hookIndex,
			)
		}
		if hook.DisableStageCommandTimeout && hook.CommandTimeoutMilliseconds > 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] cannot set both DisableStageCommandTimeout and CommandTimeoutMilliseconds",
				index,
				hookIndex,
			)
		}
		if hook.CallbackTimeoutMilliseconds < 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d].CallbackTimeoutMilliseconds must be >= 0",
				index,
				hookIndex,
			)
		}
		if hook.DisableStageCallbackTimeout && hook.CallbackTimeoutMilliseconds > 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] cannot set both DisableStageCallbackTimeout and CallbackTimeoutMilliseconds",
				index,
				hookIndex,
			)
		}

		if strings.TrimSpace(hook.Cmd) != "" && hook.RunCombinedDevBuildHookCommands {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] cannot set both Cmd and RunCombinedDevBuildHookCommands",
				index,
				hookIndex,
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
				index,
				hookIndex,
				hook.Timing,
			)
		}
	}

	return nil
}

// ValidateWatchedFile validates one Watch.Include entry with field index context.
func ValidateWatchedFile(wf *wave.WatchedFile, index int) error {
	return validateWatchedFile(wf, index)
}

func validateHealthcheckEndpoint(healthcheckEndpoint string) error {
	if healthcheckEndpoint == "" {
		return nil
	}

	if strings.TrimSpace(healthcheckEndpoint) != healthcheckEndpoint {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must not include surrounding whitespace")
	}

	if strings.ContainsAny(healthcheckEndpoint, "\t\r\n ") {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must not contain whitespace")
	}

	if strings.Contains(healthcheckEndpoint, "://") {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must be a path, not a URL")
	}

	if !strings.HasPrefix(healthcheckEndpoint, "/") {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must start with '/'")
	}

	if strings.HasPrefix(healthcheckEndpoint, "//") {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must be a single absolute path")
	}

	if strings.Contains(healthcheckEndpoint, "?") || strings.Contains(healthcheckEndpoint, "#") {
		return fmt.Errorf("config: Watch.HealthcheckEndpoint must not include query or fragment segments")
	}

	return nil
}

func validateWatchGlobPattern(
	fieldPath string,
	globPattern string,
) error {
	return toolingshared.ValidateNamedGlobPatternInput("config", fieldPath, globPattern)
}

const (
	configuredHookStageFailurePolicyFailOpen   = "fail-open"
	configuredHookStageFailurePolicyFailClosed = "fail-closed"
)

func normalizeConfiguredHookStageFailurePolicy(
	configuredHookStageFailurePolicy string,
) string {
	return strings.TrimSpace(strings.ToLower(configuredHookStageFailurePolicy))
}

func deriveBuildHookCommandTimeoutMilliseconds(
	coreConfig *wave.CoreConfig,
	isDev bool,
) int {
	if coreConfig == nil {
		return 0
	}

	if isDev {
		return coreConfig.DevBuildHookTimeoutMilliseconds
	}
	return coreConfig.ProdBuildHookTimeoutMilliseconds
}

func deriveTimeoutDurationFromMilliseconds(
	timeoutMilliseconds int,
) time.Duration {
	if timeoutMilliseconds <= 0 {
		return 0
	}
	return time.Duration(timeoutMilliseconds) * time.Millisecond
}

func deriveBuildHookCommandTimeoutDuration(
	coreConfig *wave.CoreConfig,
	isDev bool,
) time.Duration {
	return deriveTimeoutDurationFromMilliseconds(
		deriveBuildHookCommandTimeoutMilliseconds(coreConfig, isDev),
	)
}

func deriveExecutionContextWithOptionalTimeout(
	parentExecutionContext context.Context,
	executionTimeoutDuration time.Duration,
) (
	executionContext context.Context,
	cancelExecutionContext context.CancelFunc,
) {
	if executionTimeoutDuration <= 0 {
		if parentExecutionContext == nil {
			return context.Background(), nil
		}
		return parentExecutionContext, nil
	}

	if parentExecutionContext == nil {
		return context.WithTimeout(context.Background(), executionTimeoutDuration)
	}
	return context.WithTimeout(parentExecutionContext, executionTimeoutDuration)
}
