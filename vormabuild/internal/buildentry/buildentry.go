// Package buildentry owns vormabuild command parsing and execution orchestration.
//
// It keeps CLI-mode behavior and hook/build dispatch outside the end-user
// `vormabuild` facade so app-facing API surface stays minimal.
package buildentry

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/buildenv"
	"github.com/vormadev/vorma/vormabuild/internal/buildflow"
	"github.com/vormadev/vorma/vormabuild/internal/buildinner"
	"github.com/vormadev/vorma/wave/waveframework"
)

// Options configures one vormabuild execution.
type Options struct {
	Dev               bool
	HookOnly          bool
	HookExecutionOnly bool
	SkipGoBinaryBuild bool
}

type buildCommandOptions struct {
	runInDevelopmentMode  bool
	runHookOnly           bool
	runHookExecutionOnly  bool
	skipGoBinaryBuildStep bool
}

type buildCommandHooks struct {
	configureBuildEnvironment func(*vormaruntime.Vorma)
	// runBuildHook executes canonical hook orchestration (outer hook mode).
	runBuildHook func(*vormaruntime.Vorma, bool) error
	// runHookExecutionOnly executes hook build logic directly (inner hook mode).
	runHookExecutionOnly      func(*vormaruntime.Vorma, bool) error
	runProdHookPostProcessing func(*vormaruntime.Vorma) error
	runFullBuild              func(*vormaruntime.Vorma, bool, bool) error
}

type buildCommandExecutor struct {
	vorma *vormaruntime.Vorma
	hooks buildCommandHooks
}

func defaultBuildCommandHooks() buildCommandHooks {
	return buildCommandHooks{
		configureBuildEnvironment: func(v *vormaruntime.Vorma) {
			_ = buildenv.Configure(v)
		},
		runBuildHook: runConfiguredFrameworkBuildHookRunner,
		runHookExecutionOnly: func(v *vormaruntime.Vorma, isDev bool) error {
			return buildinner.Run(v, &buildinner.RunOptions{IsDev: isDev})
		},
		runProdHookPostProcessing: buildflow.RunProdHookPostProcessing,
		runFullBuild:              buildflow.RunFullRuntimeBuild,
	}
}

func runConfiguredFrameworkBuildHookRunner(
	v *vormaruntime.Vorma,
	runInDevelopmentMode bool,
) error {
	if v == nil || v.Wave == nil || v.Wave.ParsedConfig() == nil {
		return errors.New("vorma runtime/config is required")
	}
	runBuildHook := waveframework.StateForConfig(v.Wave.ParsedConfig()).RunBuildHook
	if runBuildHook == nil {
		return errors.New("framework build hook runner is not configured")
	}
	return runBuildHook(context.Background(), runInDevelopmentMode)
}

// ParseCommandOptions parses CLI flags into build options.
func ParseCommandOptions(
	commandLineArgs []string,
) (Options, error) {
	parsedOptions, parseError := parseBuildCommandOptions(commandLineArgs)
	if parseError != nil {
		return Options{}, fmt.Errorf("parse build flags: %w", parseError)
	}
	return Options{
		Dev:               parsedOptions.runInDevelopmentMode,
		HookOnly:          parsedOptions.runHookOnly,
		HookExecutionOnly: parsedOptions.runHookExecutionOnly,
		SkipGoBinaryBuild: parsedOptions.skipGoBinaryBuildStep,
	}, nil
}

// RunWithOptions executes vormabuild with explicit options.
func RunWithOptions(v *vormaruntime.Vorma, options Options) error {
	if v == nil {
		return errors.New("vorma runtime is required")
	}
	if err := validateBuildCommandHooks(defaultBuildCommandHooks()); err != nil {
		return err
	}
	commandExecutor := newBuildCommandExecutor(v, defaultBuildCommandHooks())
	return commandExecutor.run(buildCommandOptions{
		runInDevelopmentMode:  options.Dev,
		runHookOnly:           options.HookOnly,
		runHookExecutionOnly:  options.HookExecutionOnly,
		skipGoBinaryBuildStep: options.SkipGoBinaryBuild,
	})
}

func parseBuildCommandOptions(
	commandLineArgs []string,
) (buildCommandOptions, error) {
	var options buildCommandOptions

	flagSet := flag.NewFlagSet("vormabuild", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	flagSet.BoolVar(
		&options.runInDevelopmentMode,
		"dev",
		false,
		"run in development mode",
	)
	flagSet.BoolVar(
		&options.runHookOnly,
		"hook",
		false,
		"run build hook only (internal use)",
	)
	flagSet.BoolVar(
		&options.runHookExecutionOnly,
		"hook-inner",
		false,
		"run build hook execution only (internal use)",
	)
	flagSet.BoolVar(
		&options.skipGoBinaryBuildStep,
		"no-binary",
		false,
		"skip go binary compilation",
	)
	if err := flagSet.Parse(commandLineArgs); err != nil {
		return buildCommandOptions{}, err
	}
	if remainingArgs := flagSet.Args(); len(remainingArgs) > 0 {
		return buildCommandOptions{}, fmt.Errorf(
			"unexpected positional arguments: %s",
			strings.Join(remainingArgs, ", "),
		)
	}
	if options.runHookOnly && options.runHookExecutionOnly {
		return buildCommandOptions{}, errors.New(
			"build flags --hook and --hook-inner cannot be combined",
		)
	}

	return options, nil
}

func runBuildCommand(
	v *vormaruntime.Vorma,
	commandLineArgs []string,
	hooks buildCommandHooks,
) error {
	if v == nil {
		return errors.New("vorma runtime is required")
	}
	if err := validateBuildCommandHooks(hooks); err != nil {
		return err
	}

	options, err := parseBuildCommandOptions(commandLineArgs)
	if err != nil {
		return fmt.Errorf("parse build flags: %w", err)
	}

	commandExecutor := newBuildCommandExecutor(v, hooks)
	return commandExecutor.run(options)
}

func validateBuildCommandHooks(hooks buildCommandHooks) error {
	if hooks.configureBuildEnvironment == nil {
		return errors.New(
			"build command hook configureBuildEnvironment is required",
		)
	}
	if hooks.runBuildHook == nil {
		return errors.New("build command hook runBuildHook is required")
	}
	if hooks.runHookExecutionOnly == nil {
		return errors.New("build command hook runHookExecutionOnly is required")
	}
	if hooks.runProdHookPostProcessing == nil {
		return errors.New(
			"build command hook runProdHookPostProcessing is required",
		)
	}
	if hooks.runFullBuild == nil {
		return errors.New("build command hook runFullBuild is required")
	}
	return nil
}

func newBuildCommandExecutor(
	v *vormaruntime.Vorma,
	hooks buildCommandHooks,
) buildCommandExecutor {
	return buildCommandExecutor{
		vorma: v,
		hooks: hooks,
	}
}

func (commandExecutor buildCommandExecutor) run(
	options buildCommandOptions,
) error {
	if options.runHookOnly && options.runHookExecutionOnly {
		return errors.New("build flags --hook and --hook-inner cannot be combined")
	}
	if options.runHookExecutionOnly {
		return commandExecutor.runHookExecutionOnly(options.runInDevelopmentMode)
	}
	if options.runHookOnly {
		return commandExecutor.runHookOnly(options.runInDevelopmentMode)
	}

	return commandExecutor.runFullBuild(
		options.runInDevelopmentMode,
		options.skipGoBinaryBuildStep,
	)
}

func (commandExecutor buildCommandExecutor) runHookOnly(
	runInDevelopmentMode bool,
) error {
	commandExecutor.hooks.configureBuildEnvironment(commandExecutor.vorma)
	if err := commandExecutor.hooks.runBuildHook(commandExecutor.vorma, runInDevelopmentMode); err != nil {
		return fmt.Errorf("build hook failed: %w", err)
	}
	return nil
}

func (commandExecutor buildCommandExecutor) runHookExecutionOnly(
	runInDevelopmentMode bool,
) error {
	commandExecutor.hooks.configureBuildEnvironment(commandExecutor.vorma)
	if err := commandExecutor.hooks.runHookExecutionOnly(
		commandExecutor.vorma,
		runInDevelopmentMode,
	); err != nil {
		return fmt.Errorf("build hook failed: %w", err)
	}
	if runInDevelopmentMode {
		return nil
	}
	return commandExecutor.hooks.runProdHookPostProcessing(
		commandExecutor.vorma,
	)
}

func (commandExecutor buildCommandExecutor) runFullBuild(
	runInDevelopmentMode bool,
	skipGoBinaryBuildStep bool,
) error {
	if err := commandExecutor.hooks.runFullBuild(
		commandExecutor.vorma,
		runInDevelopmentMode,
		skipGoBinaryBuildStep,
	); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return nil
}

type buildEntrypointDependencies struct {
	runBuildCommandFromCLI func(*vormaruntime.Vorma, []string, buildCommandHooks) error
	fatalfForBuildCommand  func(string, ...any)
}

type buildEntrypointExecutor struct {
	dependencies buildEntrypointDependencies
}

func defaultBuildEntrypointDependencies() buildEntrypointDependencies {
	return buildEntrypointDependencies{
		runBuildCommandFromCLI: runBuildCommand,
		fatalfForBuildCommand:  log.Fatalf,
	}
}

func normalizeBuildEntrypointDependencies(
	dependencies buildEntrypointDependencies,
) buildEntrypointDependencies {
	defaultDependencies := defaultBuildEntrypointDependencies()
	if dependencies.runBuildCommandFromCLI == nil {
		dependencies.runBuildCommandFromCLI = defaultDependencies.runBuildCommandFromCLI
	}
	if dependencies.fatalfForBuildCommand == nil {
		dependencies.fatalfForBuildCommand = defaultDependencies.fatalfForBuildCommand
	}
	return dependencies
}

func newBuildEntrypointExecutor(
	dependencies buildEntrypointDependencies,
) buildEntrypointExecutor {
	return buildEntrypointExecutor{
		dependencies: normalizeBuildEntrypointDependencies(dependencies),
	}
}

func runBuildEntrypointWithDependencies(
	v *vormaruntime.Vorma,
	commandLineArgs []string,
	hooks buildCommandHooks,
	dependencies buildEntrypointDependencies,
) {
	newBuildEntrypointExecutor(
		dependencies,
	).runBuildCommand(v, commandLineArgs, hooks)
}

func (entrypointExecutor buildEntrypointExecutor) runBuildCommand(
	v *vormaruntime.Vorma,
	commandLineArgs []string,
	hooks buildCommandHooks,
) {
	if err := entrypointExecutor.dependencies.runBuildCommandFromCLI(
		v,
		commandLineArgs,
		hooks,
	); err != nil {
		entrypointExecutor.dependencies.fatalfForBuildCommand("%v", err)
	}
}
