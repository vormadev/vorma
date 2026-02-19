package tooling

import (
	"flag"
	"fmt"
	"log/slog"

	"github.com/vormadev/vorma/wave"
	toolingbuilder "github.com/vormadev/vorma/wave/tooling_2/builder"
	"github.com/vormadev/vorma/wave/tooling_2/devserver"
)

// -----------------------------------------------------------------------------
// Top-level build and dev execution.
// -----------------------------------------------------------------------------

// BuildOpts controls top-level Wave build execution.
type BuildOpts struct {
	CompileGo    bool
	IsDev        bool
	IsRebuild    bool
	FileOnlyMode bool
}

// RunDev runs Wave's development server orchestration.
func RunDev(cfg *wave.ParsedConfig, log *slog.Logger) error {
	return devserver.RunDev(cfg, log)
}

// RunBuild runs Wave's build orchestration.
func RunBuild(
	cfg *wave.ParsedConfig,
	log *slog.Logger,
	buildOpts BuildOpts,
) error {
	if err := toolingbuilder.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	builderForBuild := toolingbuilder.NewBuilder(cfg, log)
	defer builderForBuild.Close()

	return builderForBuild.Build(toolingbuilder.BuildOpts{
		CompileGo:    buildOpts.CompileGo,
		IsDev:        buildOpts.IsDev,
		IsRebuild:    buildOpts.IsRebuild,
		FileOnlyMode: buildOpts.FileOnlyMode,
	})
}

// -----------------------------------------------------------------------------
// CLI parsing and orchestration entrypoints.
// -----------------------------------------------------------------------------

// CLIOptions controls Wave CLI execution behavior.
type CLIOptions struct {
	IsDev    bool
	HookOnly bool
	NoBinary bool
}

// ExecutionDependencies contains callbacks required for CLI-driven execution.
type ExecutionDependencies struct {
	RunDev   func(log *slog.Logger) error
	RunBuild func(log *slog.Logger, compileGo bool) error
}

func (executionDependencies ExecutionDependencies) validate() error {
	if executionDependencies.RunDev == nil {
		return fmt.Errorf("cli execution dependency RunDev must not be nil")
	}
	if executionDependencies.RunBuild == nil {
		return fmt.Errorf("cli execution dependency RunBuild must not be nil")
	}
	return nil
}

// DefaultExecutionDependencies wires top-level CLI execution dependencies for cfg.
func DefaultExecutionDependencies(
	cfg *wave.ParsedConfig,
) ExecutionDependencies {
	return ExecutionDependencies{
		RunDev: func(log *slog.Logger) error {
			return RunDev(cfg, log)
		},
		RunBuild: func(log *slog.Logger, compileGo bool) error {
			return RunBuild(cfg, log, BuildOpts{
				CompileGo: compileGo,
				IsDev:     false,
			})
		},
	}
}

// ParseCLIOptions parses Wave CLI flags.
func ParseCLIOptions(
	commandLineArgs []string,
) (CLIOptions, error) {
	flagSet := flag.NewFlagSet("wave", flag.ContinueOnError)

	isDev := flagSet.Bool("dev", false, "run in dev mode")
	hookOnly := flagSet.Bool("hook", false, "run custom hook only")
	noBinary := flagSet.Bool("no-binary", false, "skip go binary compilation")

	if err := flagSet.Parse(commandLineArgs); err != nil {
		return CLIOptions{}, fmt.Errorf("parse CLI options: %w", err)
	}

	return CLIOptions{
		IsDev:    *isDev,
		HookOnly: *hookOnly,
		NoBinary: *noBinary,
	}, nil
}

// BuildWaveWithHookFromArgs runs Wave from args and an optional hook callback.
func BuildWaveWithHookFromArgs(
	log *slog.Logger,
	commandLineArgs []string,
	hook func(isDev bool) error,
	executionDependencies ExecutionDependencies,
) error {
	cliOptions, parseCLIOptionsError := ParseCLIOptions(commandLineArgs)
	if parseCLIOptionsError != nil {
		return parseCLIOptionsError
	}

	return BuildWaveWithHookOptions(
		log,
		cliOptions,
		hook,
		executionDependencies,
	)
}

// BuildWaveWithHookOptions runs Wave from parsed options and an optional hook callback.
func BuildWaveWithHookOptions(
	log *slog.Logger,
	cliOptions CLIOptions,
	hook func(isDev bool) error,
	executionDependencies ExecutionDependencies,
) error {
	if err := executionDependencies.validate(); err != nil {
		return err
	}

	if cliOptions.HookOnly && hook != nil {
		if hookExecutionError := hook(cliOptions.IsDev); hookExecutionError != nil {
			return fmt.Errorf("run hook: %w", hookExecutionError)
		}
		return nil
	}

	if cliOptions.IsDev {
		if runDevError := executionDependencies.RunDev(log); runDevError != nil {
			return fmt.Errorf("run dev: %w", runDevError)
		}
		return nil
	}

	if runBuildError := executionDependencies.RunBuild(log, !cliOptions.NoBinary); runBuildError != nil {
		return fmt.Errorf("run build: %w", runBuildError)
	}

	return nil
}

// RunFromArgs executes Wave using cfg-backed default dependencies.
func RunFromArgs(
	cfg *wave.ParsedConfig,
	log *slog.Logger,
	commandLineArgs []string,
	hook func(isDev bool) error,
) error {
	return BuildWaveWithHookFromArgs(
		log,
		commandLineArgs,
		hook,
		DefaultExecutionDependencies(cfg),
	)
}

// RunFromCLIOptions executes Wave using cfg-backed default dependencies and
// pre-parsed options.
func RunFromCLIOptions(
	cfg *wave.ParsedConfig,
	log *slog.Logger,
	cliOptions CLIOptions,
	hook func(isDev bool) error,
) error {
	return BuildWaveWithHookOptions(
		log,
		cliOptions,
		hook,
		DefaultExecutionDependencies(cfg),
	)
}

// ShouldSkipGoBinaryBuild reports whether the current CLI options disable
// binary compilation.
func ShouldSkipGoBinaryBuild(
	cliOptions CLIOptions,
) bool {
	return cliOptions.NoBinary
}
