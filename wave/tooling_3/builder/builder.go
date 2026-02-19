package builder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling_3/builder/css"
	"github.com/vormadev/vorma/wave/tooling_3/builder/schema"
	"github.com/vormadev/vorma/wave/tooling_3/builder/static"
	"github.com/vormadev/vorma/wave/tooling_3/toolingshared"
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
}

// NewBuilder creates a build orchestrator for one parsed config.
func NewBuilder(cfg *wave.ParsedConfig, log *slog.Logger) *Builder {
	if log == nil {
		log = slog.Default()
	}
	return &Builder{
		cfg:             cfg,
		log:             log,
		cssProcessor:    css.NewProcessor(cfg, log),
		staticProcessor: static.NewProcessor(cfg, log),
		schemaProcessor: schema.NewProcessor(cfg, log),
	}
}

// Close closes builder resources.
func (builder *Builder) Close() {
	if builder == nil {
		return
	}
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

	if hookError := builder.runBuildHooks(options.IsDev); hookError != nil {
		return hookError
	}

	var buildGroup errgroup.Group
	if !options.FileOnlyMode {
		if options.CompileGo {
			buildGroup.Go(func() error {
				return builder.CompileGo()
			})
		}
	}

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

	buildGroup.Go(func() error {
		return builder.schemaProcessor.WriteSchema()
	})

	if buildError := buildGroup.Wait(); buildError != nil {
		return buildError
	}

	if builder.cfg.FrameworkPublicFileMapOutDir != "" {
		if writeMapError := builder.WriteFrameworkPublicFileMapTS(); writeMapError != nil {
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
	if ensureDirectoryError := toolingshared.EnsureDirectoryForFile(binaryOutputPath); ensureDirectoryError != nil {
		return ensureDirectoryError
	}

	goBuildArguments := []string{
		"build",
		"-o",
		binaryOutputPath,
		builder.cfg.Core.MainAppEntry,
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
				builder.cfg.Core.MainAppEntry,
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

// WriteFrameworkPublicFileMapTS writes framework-facing public file map module.
func (builder *Builder) WriteFrameworkPublicFileMapTS() error {
	if builder == nil || builder.staticProcessor == nil {
		return errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.WriteFrameworkPublicFileMapTS()
}

// PublicURLBuildtime resolves one original public path at build time.
func (builder *Builder) PublicURLBuildtime(
	originalPath string,
) (string, bool, error) {
	if builder == nil || builder.staticProcessor == nil {
		return "", false, errors.New("static processor is unavailable")
	}
	return builder.staticProcessor.PublicURLBuildtime(originalPath)
}

// MustPublicURLBuildtime resolves one original path and panics on miss/error.
func (builder *Builder) MustPublicURLBuildtime(originalPath string) string {
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

// GetCriticalCSS returns cached critical CSS content.
func (builder *Builder) GetCriticalCSS() (string, bool) {
	if builder == nil || builder.cssProcessor == nil {
		return "", false
	}
	return builder.cssProcessor.CriticalCSS()
}

// GetNormalCSSURL returns cached normal CSS URL.
func (builder *Builder) GetNormalCSSURL() (string, bool) {
	if builder == nil || builder.cssProcessor == nil {
		return "", false
	}
	return builder.cssProcessor.NormalCSSURL()
}

// runBuildHooks executes configured user/framework build hooks.
func (builder *Builder) runBuildHooks(isDev bool) error {
	if builder == nil || builder.cfg == nil || builder.cfg.Core == nil {
		return nil
	}

	command := ""
	frameworkCommand := ""
	timeout := deriveBuildHookCommandTimeoutDuration(
		builder.cfg.Core,
		isDev,
	)

	if isDev {
		command = builder.cfg.Core.DevBuildHook
		frameworkCommand = builder.cfg.FrameworkDevBuildHook
	} else {
		command = builder.cfg.Core.ProdBuildHook
		frameworkCommand = builder.cfg.FrameworkProdBuildHook
	}

	resolvedCommand := resolveSequentialShellCommands(
		command,
		frameworkCommand,
	)
	if strings.TrimSpace(resolvedCommand) != "" {
		hookContext, cancelHookContext := deriveExecutionContextWithOptionalTimeout(
			context.Background(),
			timeout,
		)
		if cancelHookContext != nil {
			defer cancelHookContext()
		}
		if runError := executeShellCommandWithContext(
			hookContext,
			resolvedCommand,
		); runError != nil {
			return fmt.Errorf("run build hook command: %w", runError)
		}
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
			return fmt.Errorf("run framework build hook: %w", runError)
		}
	}

	return nil
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

// ensureOutputDirectories creates output directories required by build pipelines.
func (builder *Builder) ensureOutputDirectories() error {
	if builder == nil || builder.cfg == nil {
		return errors.New("builder config is nil")
	}

	directories := []string{
		builder.cfg.Dist.Internal(),
		builder.cfg.Dist.StaticPublic(),
		builder.cfg.Dist.StaticPrivate(),
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

	keepPath := builder.cfg.Dist.KeepFile()
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

// overlayContextKey is context key for go build overlay configuration path.
type overlayContextKey struct{}

// ValidateConfig validates configuration semantics needed by tooling build workflows.
func ValidateConfig(cfg *wave.ParsedConfig) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if cfg.Core == nil {
		return errors.New("config.Core is nil")
	}

	if strings.TrimSpace(cfg.Core.MainAppEntry) == "" {
		return errors.New("config: Core.MainAppEntry is required")
	}
	if strings.TrimSpace(cfg.Core.DistDir) == "" {
		return errors.New("config: Core.DistDir is required")
	}
	if strings.TrimSpace(cfg.Core.StaticAssetDirs.Public) == "" {
		return errors.New("config: Core.StaticAssetDirs.Public is required")
	}
	if strings.TrimSpace(cfg.Core.StaticAssetDirs.Private) == "" {
		return errors.New("config: Core.StaticAssetDirs.Private is required")
	}

	if validateError := validatePublicPathPrefix(cfg.Core.PublicPathPrefix); validateError != nil {
		return validateError
	}
	if validateError := validateHealthcheckEndpoint(cfg.HealthcheckEndpoint()); validateError != nil {
		return validateError
	}
	if validateError := css.ValidateCSSConfig(cfg); validateError != nil {
		return validateError
	}
	if validateError := validateWatchPatterns(cfg); validateError != nil {
		return validateError
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

// validateHealthcheckEndpoint validates healthcheck endpoint semantics.
func validateHealthcheckEndpoint(healthcheckEndpoint string) error {
	if strings.TrimSpace(healthcheckEndpoint) == "" {
		return nil
	}
	if strings.TrimSpace(healthcheckEndpoint) != healthcheckEndpoint {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must not include surrounding whitespace",
		)
	}
	if strings.ContainsAny(healthcheckEndpoint, " \t\n\r") {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must not contain whitespace",
		)
	}
	if parsedURL, parseError := url.Parse(healthcheckEndpoint); parseError == nil {
		if parsedURL.Scheme != "" || parsedURL.Host != "" {
			return fmt.Errorf(
				"config: Watch.HealthcheckEndpoint must be a path, not a URL",
			)
		}
	}
	if !strings.HasPrefix(healthcheckEndpoint, "/") {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must start with '/'",
		)
	}
	if strings.Contains(healthcheckEndpoint, "?") ||
		strings.Contains(healthcheckEndpoint, "#") {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must not include query or fragment segments",
		)
	}
	if filepath.Clean(healthcheckEndpoint) != healthcheckEndpoint {
		return fmt.Errorf(
			"config: Watch.HealthcheckEndpoint must be a single absolute path",
		)
	}
	return nil
}

// validateWatchPatterns validates all configured watch glob patterns.
func validateWatchPatterns(cfg *wave.ParsedConfig) error {
	if cfg == nil || cfg.Watch == nil {
		return nil
	}

	for includeIndex, watchedFile := range cfg.Watch.Include {
		if validationError := toolingshared.ValidateNamedGlobPatternInput(
			"config",
			fmt.Sprintf("Watch.Include[%d].Pattern", includeIndex),
			watchedFile.Pattern,
		); validationError != nil {
			return validationError
		}

		for hookIndex, hook := range watchedFile.OnChangeHooks {
			for excludeIndex, excludedPattern := range hook.Exclude {
				if validationError := toolingshared.ValidateNamedGlobPatternInput(
					"config",
					fmt.Sprintf("Watch.Include[%d].OnChangeHooks[%d].Exclude[%d]", includeIndex, hookIndex, excludeIndex),
					excludedPattern,
				); validationError != nil {
					return validationError
				}
			}
		}
	}

	for excludeDirIndex, excludePattern := range cfg.Watch.Exclude.Dirs {
		if validationError := toolingshared.ValidateNamedGlobPatternInput(
			"config",
			fmt.Sprintf("Watch.Exclude.Dirs[%d]", excludeDirIndex),
			excludePattern,
		); validationError != nil {
			return validationError
		}
	}

	for excludeFileIndex, excludePattern := range cfg.Watch.Exclude.Files {
		if validationError := toolingshared.ValidateNamedGlobPatternInput(
			"config",
			fmt.Sprintf("Watch.Exclude.Files[%d]", excludeFileIndex),
			excludePattern,
		); validationError != nil {
			return validationError
		}
	}

	return nil
}
