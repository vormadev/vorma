package builder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/lab/vitecmd"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/builder/internal/css"
	"github.com/vormadev/vorma/wave/tooling/builder/internal/schema"
	"github.com/vormadev/vorma/wave/tooling/builder/internal/static"
	"github.com/vormadev/vorma/wave/tooling/internal/shared"
	"golang.org/x/sync/errgroup"
)

// BuildOpts controls top-level build execution behavior.
type BuildOpts struct {
	CompileGo    bool
	IsDev        bool
	IsRebuild    bool
	FileOnlyMode bool
}

// CSSBuildOptions controls CSS pipeline build behavior.
type CSSBuildOptions struct {
	BuildCriticalCSS bool
	BuildNormalCSS   bool
}

// Builder owns build pipelines for go, CSS, static files, and schema output.
type Builder struct {
	cfg *wave.ParsedConfig
	log *slog.Logger

	cssProcessor    *css.Processor
	staticProcessor *static.Processor
	schemaProcessor *schema.Processor

	criticalCSSBuildContextIdentity  *cssBuildContextIdentity
	criticalCSSBuildContextModeIsDev bool
	criticalCSSBuildContextEntryPath string

	normalCSSBuildContextIdentity  *cssBuildContextIdentity
	normalCSSBuildContextModeIsDev bool
	normalCSSBuildContextEntryPath string

	cssBuildContextIdentityHistory []*cssBuildContextIdentity
	nextCSSBuildContextSerial      uint64
}

// NewBuilder creates a build orchestrator for one parsed config.
func NewBuilder(cfg *wave.ParsedConfig, log *slog.Logger) *Builder {
	if log == nil {
		log = slog.Default()
	}

	staticProcessor := static.NewProcessor(cfg, log)
	return &Builder{
		cfg:             cfg,
		log:             log,
		staticProcessor: staticProcessor,
		cssProcessor: css.NewProcessor(
			cfg,
			log,
			func(originalPath string) (string, bool, error) {
				return staticProcessor.PublicURLBuildtime(originalPath)
			},
		),
		schemaProcessor: schema.NewProcessor(cfg, log),
	}
}

// Close closes builder resources.
func (builder *Builder) Close() error {
	if builder == nil {
		return nil
	}
	return nil
}

func buildGoBuildCommand(
	destinationPath string,
	entryPath string,
	isDev bool,
	goBuildOverlayPath string,
) *exec.Cmd {
	resolvedEntryPath := resolveGoBuildEntryPath(entryPath)
	commandArguments := []string{"build"}

	if strings.TrimSpace(goBuildOverlayPath) != "" {
		commandArguments = append(
			commandArguments,
			"-overlay="+goBuildOverlayPath,
		)
	}

	if !isDev {
		commandArguments = append(commandArguments, "-tags=prod")
	}

	commandArguments = append(
		commandArguments,
		"-o",
		destinationPath,
		resolvedEntryPath,
	)

	return exec.Command("go", commandArguments...)
}

func resolveGoBuildEntryPath(entryPath string) string {
	resolvedEntryPath := strings.TrimSpace(entryPath)
	if resolvedEntryPath == "" {
		return entryPath
	}

	if strings.HasPrefix(resolvedEntryPath, "./") ||
		strings.HasPrefix(resolvedEntryPath, "../") ||
		filepath.IsAbs(resolvedEntryPath) {
		return resolvedEntryPath
	}

	if directoryInfo, statError := os.Stat(resolvedEntryPath); statError == nil &&
		directoryInfo.IsDir() {
		return "./" + resolvedEntryPath
	}

	return resolvedEntryPath
}

func (builder *Builder) viteBuildContext() *vitecmd.BuildCtx {
	if builder == nil || builder.cfg == nil || builder.cfg.Vite == nil {
		return nil
	}
	return vitecmd.NewBuildCtx(&vitecmd.BuildCtxOptions{
		JSPackageManagerBaseCmd: builder.cfg.Vite.JSPackageManagerBaseCmd,
		JSPackageManagerCmdDir:  builder.cfg.Vite.JSPackageManagerCmdDir,
		OutDir:                  builder.cfg.Dist.StaticPublic(),
		ManifestOut:             builder.cfg.ViteManifestPath(),
		DefaultPort:             builder.cfg.Vite.DefaultPort,
		ViteConfigFile:          builder.cfg.Vite.ViteConfigFile,
	})
}

// ViteProdBuild runs a Vite production build.
func (builder *Builder) ViteProdBuild() error {
	if builder == nil || builder.cfg == nil || !builder.cfg.UsingVite() {
		return nil
	}
	viteBuildContext := builder.viteBuildContext()
	if viteBuildContext == nil {
		return nil
	}
	return viteBuildContext.ProdBuild()
}

// NewViteDevContext creates and starts a new Vite development build context.
func (builder *Builder) NewViteDevContext() (*vitecmd.BuildCtx, error) {
	if builder == nil || builder.cfg == nil || !builder.cfg.UsingVite() {
		return nil, nil
	}
	viteBuildContext := builder.viteBuildContext()
	if viteBuildContext == nil {
		return nil, nil
	}
	if viteBuildError := viteBuildContext.DevBuild(); viteBuildError != nil {
		return nil, viteBuildError
	}
	return viteBuildContext, nil
}

// config returns a defensive read-only config snapshot.
// Internal framework-only mutable fields are intentionally omitted.
func (builder *Builder) config() *wave.ParsedConfig {
	if builder == nil || builder.cfg == nil {
		return nil
	}
	return builder.cfg.Clone()
}

// registerSchemaSection adds one framework-defined root section to schema output.
func (builder *Builder) registerSchemaSection(
	name string,
	schemaSection jsonschema.Entry,
) {
	if builder == nil || builder.cfg == nil {
		return
	}
	if strings.TrimSpace(name) == "" {
		return
	}
	if builder.cfg.FrameworkSchemaExtensions == nil {
		builder.cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	builder.cfg.FrameworkSchemaExtensions[name] = schemaSection
}

// processFilesOnly runs static and CSS processing without hooks or go compilation.
func (builder *Builder) processFilesOnly(isRebuild bool, isDev bool) error {
	return builder.processFiles(isRebuild, isDev)
}

func (builder *Builder) processFiles(granular bool, isDev bool) error {
	_ = isDev
	if builder == nil || builder.cfg == nil {
		return errors.New("builder config is nil")
	}

	if !granular {
		staticDirectoryPath := builder.cfg.Dist.Static()
		directoryEntries, readDirectoryError := os.ReadDir(staticDirectoryPath)
		if readDirectoryError != nil &&
			!errors.Is(readDirectoryError, os.ErrNotExist) {
			return fmt.Errorf(
				"read dist static directory: %w",
				readDirectoryError,
			)
		}
		for _, directoryEntry := range directoryEntries {
			if shared.IsLockFileName(directoryEntry.Name()) {
				continue
			}
			entryPath := filepath.Join(
				staticDirectoryPath,
				directoryEntry.Name(),
			)
			if removeEntryError := os.RemoveAll(entryPath); removeEntryError != nil {
				return fmt.Errorf(
					"remove dist static entry %q: %w",
					entryPath,
					removeEntryError,
				)
			}
		}
	}

	if ensureDirectoriesError := builder.ensureOutputDirectories(); ensureDirectoriesError != nil {
		return ensureDirectoriesError
	}

	if !builder.cfg.UsingBrowser() {
		return nil
	}

	var processGroup errgroup.Group
	processGroup.Go(func() error {
		return builder.ProcessPublicFilesOnly()
	})
	processGroup.Go(func() error {
		return builder.ProcessPrivateFilesOnly()
	})
	processGroup.Go(func() error {
		return builder.BuildCSS(
			CSSBuildOptions{
				BuildCriticalCSS: true,
				BuildNormalCSS:   true,
			},
		)
	})
	return processGroup.Wait()
}

// Build executes one full build with selected options.
func (builder *Builder) Build(options BuildOpts) error {
	if builder == nil || builder.cfg == nil {
		return errors.New("builder config is nil")
	}
	if validationError := ValidateConfig(builder.cfg); validationError != nil {
		return validationError
	}
	if ensureError := builder.ensureOutputDirectories(); ensureError != nil {
		return ensureError
	}
	if options.FileOnlyMode {
		return builder.processFilesOnly(options.IsRebuild, options.IsDev)
	}

	if hookError := builder.runBuildHooks(options.IsDev); hookError != nil {
		return hookError
	}

	var buildGroup errgroup.Group
	if options.CompileGo {
		buildGroup.Go(func() error {
			if compileError := builder.CompileGo(); compileError != nil {
				return fmt.Errorf("go compilation failed: %w", compileError)
			}
			return nil
		})
	}

	if builder.cfg.UsingBrowser() {
		buildGroup.Go(func() error {
			if processPublicError := builder.ProcessPublicFilesOnly(); processPublicError != nil {
				return processPublicError
			}
			if processPrivateError := builder.ProcessPrivateFilesOnly(); processPrivateError != nil {
				return processPrivateError
			}
			return nil
		})

		buildGroup.Go(func() error {
			return builder.BuildCSS(
				CSSBuildOptions{BuildCriticalCSS: true, BuildNormalCSS: true},
			)
		})
	}

	buildGroup.Go(func() error {
		return builder.schemaProcessor.WriteSchema()
	})

	if buildError := buildGroup.Wait(); buildError != nil {
		return buildError
	}

	if builder.cfg.FrameworkPublicFileMapOutDir != "" {
		if writeMapError := builder.writeFrameworkPublicFileMapTS(); writeMapError != nil {
			return writeMapError
		}
	}

	builder.log.Info(
		"build completed",
		"is_dev",
		options.IsDev,
		"is_rebuild",
		options.IsRebuild,
	)
	return nil
}

// CompileGo compiles the configured go binary output.
func (builder *Builder) CompileGo() error {
	if builder == nil || builder.cfg == nil {
		return errors.New("builder config is nil")
	}

	commandExecutionContext := context.Background()
	overlayCleanup := func() error { return nil }
	if builder.cfg.FrameworkPrepareGoBuildOverlay != nil {
		overlay, overlayError := builder.cfg.FrameworkPrepareGoBuildOverlay()
		if overlayError != nil {
			return overlayError
		}
		if overlay != nil {
			if strings.TrimSpace(overlay.OverlayConfigPath) != "" {
				commandExecutionContext = context.WithValue(
					commandExecutionContext,
					overlayContextKey{},
					overlay.OverlayConfigPath,
				)
			}
			if overlay.Cleanup != nil {
				overlayCleanup = overlay.Cleanup
			}
		}
	}
	defer func() {
		_ = overlayCleanup()
	}()

	binaryOutputPath := builder.cfg.Dist.Binary()
	if ensureDirectoryError := shared.EnsureDirectoryForFile(binaryOutputPath); ensureDirectoryError != nil {
		return ensureDirectoryError
	}

	goBuildArguments := []string{
		"build",
		"-o",
		binaryOutputPath,
		resolveGoBuildEntryPath(builder.cfg.Core.MainAppEntry),
	}
	if overlayConfigPath, found := commandExecutionContext.Value(overlayContextKey{}).(string); found &&
		strings.TrimSpace(overlayConfigPath) != "" {
		goBuildArguments = append(
			[]string{
				"build",
				"-overlay",
				overlayConfigPath,
				"-o",
				binaryOutputPath,
				resolveGoBuildEntryPath(builder.cfg.Core.MainAppEntry),
			},
			[]string{}...)
	}

	goBuildCommand := exec.Command("go", goBuildArguments...)
	goBuildCommand.Stdout = os.Stdout
	goBuildCommand.Stderr = os.Stderr
	goBuildCommand.Env = os.Environ()
	if runError := goBuildCommand.Run(); runError != nil {
		return fmt.Errorf("compile go binary: %w", runError)
	}

	builder.log.Info("compiled go binary", "out", binaryOutputPath)
	return nil
}

// compileGoOnly compiles the Go binary without running full build orchestration.
func (builder *Builder) compileGoOnly(isDev bool) error {
	_ = isDev
	if compileError := builder.CompileGo(); compileError != nil {
		return fmt.Errorf("go build failed: %w", compileError)
	}
	return nil
}

// BuildCSS executes CSS build pipelines.
func (builder *Builder) BuildCSS(options CSSBuildOptions) error {
	if builder == nil || builder.cssProcessor == nil {
		return errors.New("css processor is unavailable")
	}
	return builder.cssProcessor.Build(css.BuildOptions{
		BuildCriticalCSS: options.BuildCriticalCSS,
		BuildNormalCSS:   options.BuildNormalCSS,
	})
}

// buildAllCSS builds critical and non-critical CSS outputs.
func (builder *Builder) buildAllCSS(isDev bool) error {
	if buildCriticalCSSError := builder.buildCriticalCSS(isDev); buildCriticalCSSError != nil {
		return buildCriticalCSSError
	}
	if buildNormalCSSError := builder.buildNormalCSS(isDev); buildNormalCSSError != nil {
		return buildNormalCSSError
	}
	return nil
}

// buildCriticalCSS builds only the critical CSS pipeline.
func (builder *Builder) buildCriticalCSS(isDev bool) error {
	if builder == nil || builder.cssProcessor == nil {
		return errors.New("css processor is unavailable")
	}
	builder.refreshCriticalCSSBuildContextIdentity(isDev)
	if buildCriticalCSSError := builder.cssProcessor.BuildCriticalCSSOnly(); buildCriticalCSSError != nil {
		return fmt.Errorf("build critical CSS: %w", buildCriticalCSSError)
	}
	return nil
}

// buildNormalCSS builds only the non-critical CSS pipeline.
func (builder *Builder) buildNormalCSS(isDev bool) error {
	if builder == nil || builder.cssProcessor == nil {
		return errors.New("css processor is unavailable")
	}
	builder.refreshNormalCSSBuildContextIdentity(isDev)
	if buildNormalCSSError := builder.cssProcessor.BuildNormalCSSOnly(); buildNormalCSSError != nil {
		return fmt.Errorf("build normal CSS: %w", buildNormalCSSError)
	}
	return nil
}

// readCriticalCSS reads critical CSS output with stale-cache fallback.
func (builder *Builder) readCriticalCSS() (string, error) {
	return builder.ReadCriticalCSSForHotReload(false)
}

// readNormalCSSURL reads normal CSS URL with stale-cache fallback.
func (builder *Builder) readNormalCSSURL() (string, error) {
	return builder.ReadNormalCSSURLForHotReload(false)
}

// ReadCriticalCSSForHotReload reads critical CSS for browser hot reload.
func (builder *Builder) ReadCriticalCSSForHotReload(
	requireFreshBuildOutput bool,
) (string, error) {
	if builder == nil || builder.cssProcessor == nil {
		return "", errors.New("css processor is unavailable")
	}
	return builder.cssProcessor.ReadCriticalCSSHotReloadOutput(
		requireFreshBuildOutput,
	)
}

// ReadNormalCSSURLForHotReload reads normal CSS URL for browser hot reload.
func (builder *Builder) ReadNormalCSSURLForHotReload(
	requireFreshBuildOutput bool,
) (string, error) {
	if builder == nil || builder.cssProcessor == nil {
		return "", errors.New("css processor is unavailable")
	}
	return builder.cssProcessor.ReadNormalCSSHotReloadURL(
		requireFreshBuildOutput,
	)
}

// getPublicURLBuildtimeCached resolves one public path and panics on miss/error.
func (builder *Builder) getPublicURLBuildtimeCached(originalPath string) string {
	return builder.staticProcessor.MustPublicURLBuildtime(originalPath)
}

// loadFileMapFromPath loads one gob-encoded file map from disk.
func (builder *Builder) loadFileMapFromPath(path string) (wave.FileMap, error) {
	if builder == nil || builder.staticProcessor == nil {
		return nil, errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.LoadFileMapFromPath(path)
}

// saveFileMap saves one gob-encoded file map to disk.
func (builder *Builder) saveFileMap(
	fileMap wave.FileMap,
	path string,
) error {
	if builder == nil || builder.staticProcessor == nil {
		return errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.SaveFileMap(fileMap, path)
}

// savePublicFileMapJS writes hashed JS + ref artifacts for the given public map.
func (builder *Builder) savePublicFileMapJS(fileMap wave.FileMap) error {
	if builder == nil || builder.staticProcessor == nil {
		return errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.SavePublicFileMapJS(fileMap)
}

// LoadPublicFileMap loads the current public static file map.
func (builder *Builder) LoadPublicFileMap() (wave.FileMap, error) {
	if builder == nil || builder.staticProcessor == nil {
		return nil, errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.LoadFileMapFromPath(
		builder.cfg.Dist.PublicFileMapGob(),
	)
}

// loadPrivateFileMap loads the current private static file map.
func (builder *Builder) loadPrivateFileMap() (wave.FileMap, error) {
	if builder == nil || builder.staticProcessor == nil {
		return nil, errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.LoadFileMapFromPath(
		builder.cfg.Dist.PrivateFileMapGob(),
	)
}

// publicFileMapKeys returns sorted public file-map keys excluding prehashed entries.
func (builder *Builder) publicFileMapKeys() ([]string, error) {
	if builder == nil || builder.staticProcessor == nil {
		return nil, errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.PublicFileMapKeys()
}

// simplePublicFileMap returns path->distName map excluding prehashed entries.
func (builder *Builder) simplePublicFileMap() (map[string]string, error) {
	if builder == nil || builder.staticProcessor == nil {
		return nil, errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.SimplePublicFileMap()
}

// addPublicAssetKeys appends typed public asset key declarations.
func (builder *Builder) addPublicAssetKeys(
	statements *tsgen.Statements,
) (*tsgen.Statements, error) {
	if builder == nil || builder.staticProcessor == nil {
		return nil, errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.AddPublicAssetKeys(statements)
}

// ProcessPublicFilesOnly performs full-scan public static processing.
func (builder *Builder) ProcessPublicFilesOnly() error {
	if builder == nil || builder.staticProcessor == nil {
		return errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.ProcessPublicFilesOnly()
}

// ProcessPrivateFilesOnly performs full-scan private static processing.
func (builder *Builder) ProcessPrivateFilesOnly() error {
	if builder == nil || builder.staticProcessor == nil {
		return errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.ProcessPrivateFilesOnly()
}

// ProcessPublicFilesOnlyForChangedPaths performs incremental public static processing.
func (builder *Builder) ProcessPublicFilesOnlyForChangedPaths(
	changedPaths []string,
) error {
	if builder == nil || builder.staticProcessor == nil {
		return errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.ProcessPublicFilesOnlyForChangedPaths(
		changedPaths,
	)
}

// ProcessPrivateFilesOnlyForChangedPaths performs incremental private static processing.
func (builder *Builder) ProcessPrivateFilesOnlyForChangedPaths(
	changedPaths []string,
) error {
	if builder == nil || builder.staticProcessor == nil {
		return errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.ProcessPrivateFilesOnlyForChangedPaths(
		changedPaths,
	)
}

// writeFrameworkPublicFileMapTS writes framework-facing public file map module.
func (builder *Builder) writeFrameworkPublicFileMapTS() error {
	if builder == nil || builder.staticProcessor == nil {
		return errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.WriteFrameworkPublicFileMapTS()
}

// WritePublicFileMapTS writes framework TypeScript and JSON file map outputs.
func (builder *Builder) WritePublicFileMapTS(outDir string) error {
	if builder == nil || builder.staticProcessor == nil {
		return errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.WritePublicFileMapTS(outDir)
}

// publicURLBuildtime resolves one original public path at build time.
func (builder *Builder) publicURLBuildtime(
	originalPath string,
) (string, error) {
	if builder == nil || builder.staticProcessor == nil {
		return "", errors.New("static processor is unavailable")
	}
	resolvedURL, found, resolutionError := builder.staticProcessor.PublicURLBuildtime(
		originalPath,
	)
	if resolutionError != nil {
		return "", resolutionError
	}
	if !found {
		return "", fmt.Errorf("no hashed URL found for %q", originalPath)
	}
	return resolvedURL, nil
}

// mustPublicURLBuildtime resolves one original path and panics on miss/error.
func (builder *Builder) mustPublicURLBuildtime(originalPath string) string {
	return builder.staticProcessor.MustPublicURLBuildtime(originalPath)
}

// IsCriticalCSSFile reports whether path is configured critical CSS entry.
func (builder *Builder) IsCriticalCSSFile(path string) bool {
	if builder == nil || builder.cssProcessor == nil {
		return false
	}
	return builder.cssProcessor.IsCriticalCSSFile(path)
}

// IsNormalCSSFile reports whether path is configured non-critical CSS entry.
func (builder *Builder) IsNormalCSSFile(path string) bool {
	if builder == nil || builder.cssProcessor == nil {
		return false
	}
	return builder.cssProcessor.IsNormalCSSFile(path)
}

// isCSSFile reports whether path is any configured/tracked CSS input.
func (builder *Builder) isCSSFile(path string) bool {
	if builder == nil || builder.cssProcessor == nil {
		return false
	}
	return builder.cssProcessor.IsCSSFile(path)
}

// setTrackedCriticalCSSImportPaths replaces tracked critical CSS import paths.
func (builder *Builder) setTrackedCriticalCSSImportPaths(importPaths []string) {
	if builder == nil || builder.cssProcessor == nil {
		return
	}
	builder.cssProcessor.SetTrackedCriticalCSSImportPaths(importPaths)
}

// setTrackedNormalCSSImportPaths replaces tracked non-critical CSS import paths.
func (builder *Builder) setTrackedNormalCSSImportPaths(importPaths []string) {
	if builder == nil || builder.cssProcessor == nil {
		return
	}
	builder.cssProcessor.SetTrackedNormalCSSImportPaths(importPaths)
}

// countTrackedCriticalCSSImportPaths returns tracked critical import path count.
func (builder *Builder) countTrackedCriticalCSSImportPaths() int {
	if builder == nil || builder.cssProcessor == nil {
		return 0
	}
	return builder.cssProcessor.CountTrackedCriticalCSSImportPaths()
}

// listTrackedCriticalCSSImportPaths returns tracked critical import paths.
func (builder *Builder) listTrackedCriticalCSSImportPaths() []string {
	if builder == nil || builder.cssProcessor == nil {
		return nil
	}
	return builder.cssProcessor.ListTrackedCriticalCSSImportPaths()
}

// getCriticalCSS returns cached critical CSS content.
func (builder *Builder) getCriticalCSS() (string, bool) {
	if builder == nil || builder.cssProcessor == nil {
		return "", false
	}
	return builder.cssProcessor.CriticalCSS()
}

// getNormalCSSURL returns cached normal CSS URL.
func (builder *Builder) getNormalCSSURL() (string, bool) {
	if builder == nil || builder.cssProcessor == nil {
		return "", false
	}
	return builder.cssProcessor.NormalCSSURL()
}

// runHooks executes configured user/framework build hooks for one mode.
func (builder *Builder) runHooks(isDev bool) error {
	if hookError := builder.runBuildHooks(isDev); hookError != nil {
		return hookError
	}
	return nil
}

// runBuildHooks executes configured user/framework build hooks.
func (builder *Builder) runBuildHooks(isDev bool) error {
	if builder == nil || builder.cfg == nil || builder.cfg.Core == nil {
		return nil
	}

	userCommand := ""
	frameworkCommand := ""
	timeout := deriveBuildHookCommandTimeoutDuration(
		builder.cfg.Core,
		isDev,
	)

	if isDev {
		userCommand = builder.cfg.Core.DevBuildHook
		frameworkCommand = builder.cfg.FrameworkDevBuildHook
	} else {
		userCommand = builder.cfg.Core.ProdBuildHook
		frameworkCommand = builder.cfg.FrameworkProdBuildHook
	}

	if runError := runBuildHookCommandWithTimeout(userCommand, timeout); runError != nil {
		return fmt.Errorf("user build hook failed: %w", runError)
	}

	if builder.cfg.FrameworkRunBuildHook != nil {
		runnerContext, cancelRunnerContext := deriveExecutionContextWithOptionalTimeout(
			context.Background(),
			timeout,
		)
		if cancelRunnerContext != nil {
			defer cancelRunnerContext()
		}
		if runError := builder.cfg.FrameworkRunBuildHook(runnerContext, isDev); runError != nil {
			return fmt.Errorf("framework build hook failed: %w", runError)
		}
		return nil
	}

	if runError := runBuildHookCommandWithTimeout(frameworkCommand, timeout); runError != nil {
		return fmt.Errorf("framework build hook failed: %w", runError)
	}

	return nil
}

func runBuildHookCommandWithTimeout(
	command string,
	timeout time.Duration,
) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}

	commandExecutionContext, cancelCommandExecutionContext := deriveExecutionContextWithOptionalTimeout(
		context.Background(),
		timeout,
	)
	if cancelCommandExecutionContext != nil {
		defer cancelCommandExecutionContext()
	}

	return executil.RunShellWithContext(
		commandExecutionContext,
		command,
	)
}

// deriveBuildHookCommandTimeoutDuration resolves build-hook timeout for mode.
func deriveBuildHookCommandTimeoutDuration(
	coreConfig *wave.CoreConfig,
	isDev bool,
) time.Duration {
	if coreConfig == nil {
		return 0
	}
	if isDev {
		if coreConfig.DevBuildHookTimeoutMilliseconds <= 0 {
			return 0
		}
		return time.Duration(
			coreConfig.DevBuildHookTimeoutMilliseconds,
		) * time.Millisecond
	}
	if coreConfig.ProdBuildHookTimeoutMilliseconds <= 0 {
		return 0
	}
	return time.Duration(
		coreConfig.ProdBuildHookTimeoutMilliseconds,
	) * time.Millisecond
}

// resolveSequentialShellCommands joins non-empty commands into one shell command.
func resolveSequentialShellCommands(commands ...string) string {
	resolvedCommands := make([]string, 0, len(commands))
	for _, command := range commands {
		trimmedCommand := strings.TrimSpace(command)
		if trimmedCommand == "" {
			continue
		}
		resolvedCommands = append(resolvedCommands, trimmedCommand)
	}
	if len(resolvedCommands) == 0 {
		return ""
	}
	return strings.Join(resolvedCommands, " && ")
}

// deriveExecutionContextWithOptionalTimeout applies timeout only when set.
func deriveExecutionContextWithOptionalTimeout(
	parentContext context.Context,
	timeoutDuration time.Duration,
) (context.Context, context.CancelFunc) {
	if parentContext == nil {
		parentContext = context.Background()
	}
	if timeoutDuration <= 0 {
		return parentContext, nil
	}
	return context.WithTimeout(parentContext, timeoutDuration)
}

// executeShellCommandWithContext executes one shell command with inherited stdio.
func executeShellCommandWithContext(
	commandExecutionContext context.Context,
	command string,
) error {
	trimmedCommand := strings.TrimSpace(command)
	if trimmedCommand == "" {
		return nil
	}
	if commandExecutionContext == nil {
		commandExecutionContext = context.Background()
	}

	executionCommand := exec.CommandContext(
		commandExecutionContext,
		"sh",
		"-c",
		trimmedCommand,
	)
	executionCommand.Stdout = os.Stdout
	executionCommand.Stderr = os.Stderr
	executionCommand.Env = os.Environ()
	if runError := executionCommand.Run(); runError != nil {
		return runError
	}
	return nil
}

// SetupDistDir creates required dist directory structure and keep-file.
func SetupDistDir(cfg *wave.ParsedConfig) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	directories := []string{
		cfg.Dist.Internal(),
		cfg.Dist.StaticPublic(),
		cfg.Dist.StaticPrivate(),
	}
	for _, directoryPath := range directories {
		if mkdirError := os.MkdirAll(directoryPath, 0o755); mkdirError != nil {
			return fmt.Errorf(
				"create output directory %q: %w",
				directoryPath,
				mkdirError,
			)
		}
	}

	keepPath := cfg.Dist.KeepFile()
	if _, statError := os.Stat(keepPath); statError != nil {
		if !errors.Is(statError, os.ErrNotExist) {
			return statError
		}
		if writeKeepError := os.WriteFile(keepPath, []byte("keep\n"), 0o644); writeKeepError != nil {
			return writeKeepError
		}
	}
	return nil
}

// ensureOutputDirectories creates output directories required by build pipelines.
func (builder *Builder) ensureOutputDirectories() error {
	if builder == nil || builder.cfg == nil {
		return errors.New("builder config is nil")
	}
	return SetupDistDir(builder.cfg)
}

// overlayContextKey is context key for go build overlay configuration path.
type overlayContextKey struct{}

// cssBuildContextIdentity is a pointer-identity token for CSS build context state.
type cssBuildContextIdentity struct {
	serial uint64
}

func (builder *Builder) newCSSBuildContextIdentity() *cssBuildContextIdentity {
	if builder == nil {
		return nil
	}
	builder.nextCSSBuildContextSerial++
	return &cssBuildContextIdentity{
		serial: builder.nextCSSBuildContextSerial,
	}
}

func (builder *Builder) refreshCriticalCSSBuildContextIdentity(isDev bool) {
	if builder == nil || builder.cfg == nil {
		return
	}
	entryPath := normalizeCSSContextEntryPath(builder.cfg.CriticalCSSEntry())
	if builder.criticalCSSBuildContextIdentity == nil ||
		builder.criticalCSSBuildContextModeIsDev != isDev ||
		builder.criticalCSSBuildContextEntryPath != entryPath {
		builder.criticalCSSBuildContextIdentity = builder.newCSSBuildContextIdentity()
		builder.cssBuildContextIdentityHistory = append(
			builder.cssBuildContextIdentityHistory,
			builder.criticalCSSBuildContextIdentity,
		)
		builder.criticalCSSBuildContextModeIsDev = isDev
		builder.criticalCSSBuildContextEntryPath = entryPath
	}
}

func (builder *Builder) refreshNormalCSSBuildContextIdentity(isDev bool) {
	if builder == nil || builder.cfg == nil {
		return
	}
	entryPath := normalizeCSSContextEntryPath(builder.cfg.NonCriticalCSSEntry())
	if builder.normalCSSBuildContextIdentity == nil ||
		builder.normalCSSBuildContextModeIsDev != isDev ||
		builder.normalCSSBuildContextEntryPath != entryPath {
		builder.normalCSSBuildContextIdentity = builder.newCSSBuildContextIdentity()
		builder.cssBuildContextIdentityHistory = append(
			builder.cssBuildContextIdentityHistory,
			builder.normalCSSBuildContextIdentity,
		)
		builder.normalCSSBuildContextModeIsDev = isDev
		builder.normalCSSBuildContextEntryPath = entryPath
	}
}

func normalizeCSSContextEntryPath(entryPath string) string {
	if strings.TrimSpace(entryPath) == "" {
		return ""
	}
	absoluteEntryPath, absoluteEntryPathError := filepath.Abs(entryPath)
	if absoluteEntryPathError == nil {
		return filepath.Clean(absoluteEntryPath)
	}
	return filepath.Clean(entryPath)
}

// ValidateConfig validates configuration semantics needed by tooling build workflows.
func ValidateConfig(cfg *wave.ParsedConfig) error {
	if cfg == nil {
		return errors.New("config: parsed config is required")
	}
	if cfg.Core == nil {
		return errors.New("config: Core section is required")
	}
	if strings.TrimSpace(cfg.Core.MainAppEntry) == "" {
		return errors.New("config: Core.MainAppEntry is required")
	}
	if strings.TrimSpace(cfg.Core.DistDir) == "" {
		return errors.New("config: Core.DistDir is required")
	}

	if validateError := validateNonNegativeTimeoutFields(
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
	); validateError != nil {
		return validateError
	}

	if !cfg.Core.ServerOnlyMode {
		if strings.TrimSpace(cfg.Core.StaticAssetDirs.Private) == "" {
			return errors.New("config: Core.StaticAssetDirs.Private is required")
		}
		if strings.TrimSpace(cfg.Core.StaticAssetDirs.Public) == "" {
			return errors.New("config: Core.StaticAssetDirs.Public is required")
		}
	}

	if cfg.Vite != nil &&
		strings.TrimSpace(cfg.Vite.JSPackageManagerBaseCmd) == "" {
		return errors.New("config: Vite.JSPackageManagerBaseCmd is required")
	}

	if validateError := validatePublicPathPrefix(cfg.Core.PublicPathPrefix); validateError != nil {
		return validateError
	}
	if validateError := css.ValidateCSSConfig(cfg); validateError != nil {
		return validateError
	}

	if cfg.Watch == nil {
		return nil
	}

	if validateError := validateHealthcheckEndpoint(cfg.Watch.HealthcheckEndpoint); validateError != nil {
		return validateError
	}
	if validateError := validateHookStageFailurePolicy(cfg.Watch.HookStageFailurePolicy); validateError != nil {
		return validateError
	}
	if validateError := validateHookCommandTimeoutConfig(cfg.Watch.HookCommandTimeouts); validateError != nil {
		return validateError
	}
	if validateError := validateHookCallbackTimeoutConfig(cfg.Watch.HookCallbackTimeouts); validateError != nil {
		return validateError
	}

	for excludeDirectoryPatternIndex, excludeDirectoryPattern := range cfg.Watch.Exclude.Dirs {
		if validateError := validateWatchGlobPattern(
			fmt.Sprintf("Watch.Exclude.Dirs[%d]", excludeDirectoryPatternIndex),
			excludeDirectoryPattern,
		); validateError != nil {
			return validateError
		}
	}
	for excludeFilePatternIndex, excludeFilePattern := range cfg.Watch.Exclude.Files {
		if validateError := validateWatchGlobPattern(
			fmt.Sprintf("Watch.Exclude.Files[%d]", excludeFilePatternIndex),
			excludeFilePattern,
		); validateError != nil {
			return validateError
		}
	}

	for includeWatchPatternIndex, watchedFile := range cfg.Watch.Include {
		if validateError := validateWatchedFile(
			&watchedFile,
			includeWatchPatternIndex,
		); validateError != nil {
			return validateError
		}
	}

	return nil
}

// validatePublicPathPrefix validates configured public path prefix.
func validatePublicPathPrefix(publicPathPrefix string) error {
	if strings.TrimSpace(publicPathPrefix) == "" {
		return nil
	}
	if strings.TrimSpace(publicPathPrefix) != publicPathPrefix {
		return errors.New(
			"config: Core.PublicPathPrefix must not include surrounding whitespace",
		)
	}
	if !strings.HasPrefix(publicPathPrefix, "/") {
		return errors.New("config: Core.PublicPathPrefix must start with '/'")
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

func validateHookCommandTimeoutConfig(
	hookCommandTimeouts wave.HookCommandTimeoutConfig,
) error {
	return validateNonNegativeTimeoutFields(
		[]timeoutFieldValidation{
			{
				fieldPath:           "Watch.HookCommandTimeouts.PreCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeouts.PreCommandTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCommandTimeouts.PostCommandTimeoutMilliseconds",
				timeoutMilliseconds: hookCommandTimeouts.PostCommandTimeoutMilliseconds,
			},
		},
	)
}

func validateHookCallbackTimeoutConfig(
	hookCallbackTimeouts wave.HookCallbackTimeoutConfig,
) error {
	return validateNonNegativeTimeoutFields(
		[]timeoutFieldValidation{
			{
				fieldPath:           "Watch.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeouts.PreCallbackTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds,
			},
			{
				fieldPath:           "Watch.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds",
				timeoutMilliseconds: hookCallbackTimeouts.PostCallbackTimeoutMilliseconds,
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

func validateWatchedFile(
	watchedFile *wave.WatchedFile,
	watchedFileIndex int,
) error {
	if watchedFile == nil {
		return nil
	}

	if validateError := validateWatchGlobPattern(
		fmt.Sprintf("Watch.Include[%d].Pattern", watchedFileIndex),
		watchedFile.Pattern,
	); validateError != nil {
		return validateError
	}

	for hookIndex, onChangeHook := range watchedFile.OnChangeHooks {
		for excludedPatternIndex, excludedPattern := range onChangeHook.Exclude {
			if validateError := validateWatchGlobPattern(
				fmt.Sprintf(
					"Watch.Include[%d].OnChangeHooks[%d].Exclude[%d]",
					watchedFileIndex,
					hookIndex,
					excludedPatternIndex,
				),
				excludedPattern,
			); validateError != nil {
				return validateError
			}
		}

		if onChangeHook.CommandTimeoutMilliseconds < 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d].CommandTimeoutMilliseconds must be >= 0",
				watchedFileIndex,
				hookIndex,
			)
		}
		if onChangeHook.DisableStageCommandTimeout &&
			onChangeHook.CommandTimeoutMilliseconds > 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] cannot set both DisableStageCommandTimeout and CommandTimeoutMilliseconds",
				watchedFileIndex,
				hookIndex,
			)
		}

		if onChangeHook.CallbackTimeoutMilliseconds < 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d].CallbackTimeoutMilliseconds must be >= 0",
				watchedFileIndex,
				hookIndex,
			)
		}
		if onChangeHook.DisableStageCallbackTimeout &&
			onChangeHook.CallbackTimeoutMilliseconds > 0 {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] cannot set both DisableStageCallbackTimeout and CallbackTimeoutMilliseconds",
				watchedFileIndex,
				hookIndex,
			)
		}

		if strings.TrimSpace(onChangeHook.Cmd) != "" &&
			onChangeHook.RunCombinedDevBuildHookCommands {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] cannot set both Cmd and RunCombinedDevBuildHookCommands",
				watchedFileIndex,
				hookIndex,
			)
		}

		if !watchedFile.RunOnChangeOnly {
			continue
		}
		if strings.TrimSpace(onChangeHook.Cmd) == "" &&
			!onChangeHook.RunCombinedDevBuildHookCommands {
			continue
		}
		if onChangeHook.Timing != "" &&
			onChangeHook.Timing != wave.OnChangeStrategyPre {
			return fmt.Errorf(
				"config: Watch.Include[%d].OnChangeHooks[%d] has Timing %q but RunOnChangeOnly requires all command hooks to use \"pre\" timing (the default)",
				watchedFileIndex,
				hookIndex,
				onChangeHook.Timing,
			)
		}
	}

	return nil
}

func validateHealthcheckEndpoint(healthcheckEndpoint string) error {
	if strings.TrimSpace(healthcheckEndpoint) == "" {
		return nil
	}

	if strings.TrimSpace(healthcheckEndpoint) != healthcheckEndpoint {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must not include surrounding whitespace",
		)
	}

	if strings.ContainsAny(healthcheckEndpoint, "\t\r\n ") {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must not contain whitespace",
		)
	}

	if strings.Contains(healthcheckEndpoint, "://") {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must be a path, not a URL",
		)
	}

	if !strings.HasPrefix(healthcheckEndpoint, "/") {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must start with '/'",
		)
	}

	if strings.HasPrefix(healthcheckEndpoint, "//") {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must be a single absolute path",
		)
	}

	if strings.Contains(healthcheckEndpoint, "?") ||
		strings.Contains(healthcheckEndpoint, "#") {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must not include query or fragment segments",
		)
	}

	return nil
}

func validateWatchGlobPattern(
	fieldPath string,
	globPattern string,
) error {
	return shared.ValidateNamedGlobPatternInput("config", fieldPath, globPattern)
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
