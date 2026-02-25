// Package vormabuild contains Vorma build and dev entrypoints.
//
// This package is build-time only and is intended for build commands (for
// example, ./cmd/build). Do not import it from runtime request-serving code.
// Keeping build-time orchestration out of production binaries avoids pulling in
// unnecessary tooling dependencies and keeps app binaries smaller.
//
// Build and fast-rebuild flows follow a deterministic plan/stage/commit model:
// 1. plan: capture immutable runtime snapshots and compute build decisions.
// 2. stage: run filesystem/codegen side effects outside runtime write locks.
// 3. commit: apply bounded runtime-state commits under lock.
//
// Runtime invariants:
// - runtime state writes commit through the shared runtime-state commit path so
// readers never observe mixed-field updates.
// - rollback of captured runtime snapshots is attempt-scoped and guarded by
// build-ID token checks so stale attempts cannot overwrite newer commits.
// - artifact writers treat superseded runtime snapshots as no-op completion
// rather than mutating newer runtime state.
package vormabuild

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/buildenv"
	"github.com/vormadev/vorma/vormabuild/buildflow"
	"github.com/vormadev/vorma/vormabuild/buildinner"
)

type buildCommandOptions struct {
	runInDevelopmentMode  bool
	runHookOnly           bool
	skipGoBinaryBuildStep bool
}

type buildCommandHooks struct {
	configureBuildEnvironment func(*vormaruntime.Vorma)
	runBuildHook              func(*vormaruntime.Vorma, bool) error
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
		runBuildHook: func(v *vormaruntime.Vorma, isDev bool) error {
			return buildinner.Run(v, &buildinner.RunOptions{IsDev: isDev})
		},
		runProdHookPostProcessing: buildflow.RunProdHookPostProcessing,
		runFullBuild:              buildflow.RunFullRuntimeBuild,
	}
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

var defaultBuildEntrypointExecutor = newBuildEntrypointExecutor(
	buildEntrypointDependencies{},
)

// Build parses flags and runs the build or dev server.
func Build(v *vorma.Vorma) {
	defaultBuildEntrypointExecutor.runBuildCommand(
		v.UnsafeRuntimeForFrameworkInternals(),
		os.Args[1:],
		defaultBuildCommandHooks(),
	)
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
	if err := entrypointExecutor.dependencies.runBuildCommandFromCLI(v, commandLineArgs, hooks); err != nil {
		entrypointExecutor.dependencies.fatalfForBuildCommand("%v", err)
	}
}
