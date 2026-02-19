package cli

import (
	"flag"
	"fmt"
	"log/slog"
)

// CLIOptions controls Wave CLI execution behavior.
type CLIOptions struct {
	IsDev    bool
	HookOnly bool
	NoBinary bool
}

// ExecutionDependencies contains the callbacks required to execute parsed CLI
// intent without coupling this package to the root tooling package.
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

// ParseCLIOptions parses Wave build/dev CLI flags.
func ParseCLIOptions(commandLineArgs []string) (CLIOptions, error) {
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

// BuildWaveWithHookFromArgs runs Wave build/dev flow from explicit args and an
// optional custom hook.
func BuildWaveWithHookFromArgs(
	log *slog.Logger,
	commandLineArgs []string,
	hook func(isDev bool) error,
	executionDependencies ExecutionDependencies,
) error {
	cliOptions, err := ParseCLIOptions(commandLineArgs)
	if err != nil {
		return err
	}

	return BuildWaveWithHookOptions(log, cliOptions, hook, executionDependencies)
}

// BuildWaveWithHookOptions runs Wave build/dev flow using already-parsed
// options.
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
		if err := hook(cliOptions.IsDev); err != nil {
			return fmt.Errorf("run hook: %w", err)
		}
		return nil
	}

	if cliOptions.IsDev {
		if err := executionDependencies.RunDev(log); err != nil {
			return fmt.Errorf("run dev: %w", err)
		}
		return nil
	}

	if err := executionDependencies.RunBuild(log, !cliOptions.NoBinary); err != nil {
		return fmt.Errorf("run build: %w", err)
	}

	return nil
}
