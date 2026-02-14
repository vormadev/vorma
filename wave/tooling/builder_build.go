package tooling

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/wave"
	"golang.org/x/sync/errgroup"
)

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
	var group errgroup.Group
	group.Go(func() error {
		return b.processPrivateFiles(granular)
	})
	group.Go(func() error {
		return b.css.buildAll(isDev)
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
