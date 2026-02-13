package tooling

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/vormadev/vorma/kit/executil"
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
