package vormabuild

import (
	"flag"
	"fmt"
	"io"

	"github.com/vormadev/vorma/vormaruntime"
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
		configureBuildEnvironment: configureBuildEnvironment,
		runBuildHook: func(v *vormaruntime.Vorma, isDev bool) error {
			return buildInner(v, &buildInnerOptions{isDev: isDev})
		},
		runProdHookPostProcessing: runProdHookPostProcessing,
		runFullBuild:              build,
	}
}

func parseBuildCommandOptions(commandLineArgs []string) (buildCommandOptions, error) {
	var options buildCommandOptions

	flagSet := flag.NewFlagSet("vormabuild", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	flagSet.BoolVar(&options.runInDevelopmentMode, "dev", false, "run in development mode")
	flagSet.BoolVar(&options.runHookOnly, "hook", false, "run build hook only (internal use)")
	flagSet.BoolVar(&options.skipGoBinaryBuildStep, "no-binary", false, "skip go binary compilation")
	if err := flagSet.Parse(commandLineArgs); err != nil {
		return buildCommandOptions{}, err
	}

	return options, nil
}

func runBuildCommand(
	v *vormaruntime.Vorma,
	commandLineArgs []string,
	hooks buildCommandHooks,
) error {
	options, err := parseBuildCommandOptions(commandLineArgs)
	if err != nil {
		return fmt.Errorf("parse build flags: %w", err)
	}

	commandExecutor := newBuildCommandExecutor(v, hooks)
	return commandExecutor.run(options)
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

func (commandExecutor buildCommandExecutor) run(options buildCommandOptions) error {
	if options.runHookOnly {
		return commandExecutor.runHookOnly(options.runInDevelopmentMode)
	}

	return commandExecutor.runFullBuild(options.runInDevelopmentMode, options.skipGoBinaryBuildStep)
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
	return commandExecutor.hooks.runProdHookPostProcessing(commandExecutor.vorma)
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
