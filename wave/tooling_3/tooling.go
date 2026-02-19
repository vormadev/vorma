package tooling

import (
	"flag"
	"fmt"
	"log/slog"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling_3/builder"
	"github.com/vormadev/vorma/wave/tooling_3/devserver"
)

// BuildOpts controls top-level Wave build execution.
type BuildOpts struct {
	CompileGo    bool
	IsDev        bool
	IsRebuild    bool
	FileOnlyMode bool
}

// RunDev runs Wave development server orchestration.
func RunDev(cfg *wave.ParsedConfig, log *slog.Logger) error {
	return devserver.RunDev(cfg, log)
}

// RunBuild runs Wave build orchestration.
func RunBuild(
	cfg *wave.ParsedConfig,
	log *slog.Logger,
	buildOpts BuildOpts,
) error {
	if validationError := builder.ValidateConfig(cfg); validationError != nil {
		return fmt.Errorf("config validation failed: %w", validationError)
	}

	builderInstance := builder.NewBuilder(cfg, log)
	defer builderInstance.Close()

	return builderInstance.Build(builder.BuildOpts{
		CompileGo:    buildOpts.CompileGo,
		IsDev:        buildOpts.IsDev,
		IsRebuild:    buildOpts.IsRebuild,
		FileOnlyMode: buildOpts.FileOnlyMode,
	})
}

// CLIOptions controls Wave CLI execution behavior.
type CLIOptions struct {
	IsDev    bool
	HookOnly bool
	NoBinary bool
}

// ExecutionPath identifies which top-level workflow should execute.
type ExecutionPath int

const (
	// ExecutionPathHookOnly executes only user-provided hook callback.
	ExecutionPathHookOnly ExecutionPath = iota
	// ExecutionPathDev runs devserver workflow.
	ExecutionPathDev
	// ExecutionPathBuild runs build workflow.
	ExecutionPathBuild
)

// ExecutionDependencies contains callbacks required for CLI-driven execution.
type ExecutionDependencies struct {
	RunDev   func(log *slog.Logger) error
	RunBuild func(log *slog.Logger, compileGo bool) error
}

// validate validates dependency callbacks.
func (executionDependencies ExecutionDependencies) validate() error {
	if executionDependencies.RunDev == nil {
		return fmt.Errorf("execution dependency RunDev must not be nil")
	}
	if executionDependencies.RunBuild == nil {
		return fmt.Errorf("execution dependency RunBuild must not be nil")
	}
	return nil
}

// ValidateCLIOptions validates internal CLI option relationships.
func ValidateCLIOptions(cliOptions CLIOptions) error {
	if cliOptions.HookOnly && !cliOptions.IsDev {
		// Hook-only mode is valid for both dev and build, so this currently has no
		// conflicting combinations. This function exists as the single gate for
		// future option interactions.
		return nil
	}
	return nil
}

// DeriveExecutionPath resolves the execution path implied by CLI options.
func DeriveExecutionPath(cliOptions CLIOptions) ExecutionPath {
	if cliOptions.HookOnly {
		return ExecutionPathHookOnly
	}
	if cliOptions.IsDev {
		return ExecutionPathDev
	}
	return ExecutionPathBuild
}

// DeriveBuildOptsFromCLIOptions derives top-level build options from CLI flags.
func DeriveBuildOptsFromCLIOptions(cliOptions CLIOptions) BuildOpts {
	return BuildOpts{
		CompileGo:    !cliOptions.NoBinary,
		IsDev:        false,
		IsRebuild:    false,
		FileOnlyMode: false,
	}
}

// DefaultExecutionDependencies returns cfg-bound default execution callbacks.
func DefaultExecutionDependencies(
	cfg *wave.ParsedConfig,
) ExecutionDependencies {
	return ExecutionDependencies{
		RunDev: func(log *slog.Logger) error {
			return RunDev(cfg, log)
		},
		RunBuild: func(log *slog.Logger, compileGo bool) error {
			return RunBuild(
				cfg,
				log,
				BuildOpts{CompileGo: compileGo, IsDev: false},
			)
		},
	}
}

// ParseCLIOptions parses Wave CLI flags.
func ParseCLIOptions(commandLineArgs []string) (CLIOptions, error) {
	flagSet := flag.NewFlagSet("wave", flag.ContinueOnError)

	isDev := flagSet.Bool("dev", false, "run in dev mode")
	hookOnly := flagSet.Bool("hook", false, "run custom hook only")
	noBinary := flagSet.Bool("no-binary", false, "skip go binary compilation")

	if parseError := flagSet.Parse(commandLineArgs); parseError != nil {
		return CLIOptions{}, fmt.Errorf("parse CLI options: %w", parseError)
	}

	return CLIOptions{
		IsDev:    *isDev,
		HookOnly: *hookOnly,
		NoBinary: *noBinary,
	}, nil
}

// BuildWaveWithHookFromArgs executes Wave using raw CLI args and optional hook callback.
func BuildWaveWithHookFromArgs(
	log *slog.Logger,
	commandLineArgs []string,
	hook func(isDev bool) error,
	executionDependencies ExecutionDependencies,
) error {
	cliOptions, parseError := ParseCLIOptions(commandLineArgs)
	if parseError != nil {
		return parseError
	}
	return BuildWaveWithHookOptions(
		log,
		cliOptions,
		hook,
		executionDependencies,
	)
}

// BuildWaveWithHookOptions executes Wave using parsed CLI options and optional hook callback.
func BuildWaveWithHookOptions(
	log *slog.Logger,
	cliOptions CLIOptions,
	hook func(isDev bool) error,
	executionDependencies ExecutionDependencies,
) error {
	if validationError := executionDependencies.validate(); validationError != nil {
		return validationError
	}
	if validationError := ValidateCLIOptions(cliOptions); validationError != nil {
		return validationError
	}

	switch DeriveExecutionPath(cliOptions) {
	case ExecutionPathHookOnly:
		if hook == nil {
			return nil
		}
		if hookError := hook(cliOptions.IsDev); hookError != nil {
			return fmt.Errorf("run hook: %w", hookError)
		}
		return nil

	case ExecutionPathDev:
		if runDevError := executionDependencies.RunDev(log); runDevError != nil {
			return fmt.Errorf("run dev: %w", runDevError)
		}
		return nil

	case ExecutionPathBuild:
		if runBuildError := executionDependencies.RunBuild(log, !cliOptions.NoBinary); runBuildError != nil {
			return fmt.Errorf("run build: %w", runBuildError)
		}
		return nil

	default:
		return fmt.Errorf("unsupported execution path")
	}
}

// RunFromArgs executes Wave using cfg-bound default dependencies and raw args.
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

// RunFromCLIOptions executes Wave using cfg-bound default dependencies and parsed options.
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

// ShouldSkipGoBinaryBuild reports whether CLI options disable go binary compilation.
func ShouldSkipGoBinaryBuild(cliOptions CLIOptions) bool {
	return cliOptions.NoBinary
}
