package tooling

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/wave"
)

type CLIOptions struct {
	IsDev    bool
	HookOnly bool
	NoBinary bool
}

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

func BuildWaveWithHookFromArgs(
	cfg *wave.ParsedConfig,
	log *slog.Logger,
	commandLineArgs []string,
	hook func(isDev bool) error,
) error {
	cliOptions, err := ParseCLIOptions(commandLineArgs)
	if err != nil {
		return err
	}

	return BuildWaveWithHookOptions(cfg, log, cliOptions, hook)
}

func BuildWaveWithHookOptions(
	cfg *wave.ParsedConfig,
	log *slog.Logger,
	cliOptions CLIOptions,
	hook func(isDev bool) error,
) error {
	if log == nil {
		log = colorlog.New("wave")
	}

	if cliOptions.HookOnly && hook != nil {
		if err := hook(cliOptions.IsDev); err != nil {
			return fmt.Errorf("run hook: %w", err)
		}
		return nil
	}

	if cliOptions.IsDev {
		if err := RunDev(cfg, log); err != nil {
			return fmt.Errorf("run dev: %w", err)
		}
		return nil
	}

	builder := NewBuilder(cfg, log)
	defer builder.Close()

	if err := builder.Build(BuildOpts{CompileGo: !cliOptions.NoBinary}); err != nil {
		return fmt.Errorf("run build: %w", err)
	}

	return nil
}

// BuildWaveWithHook provides CLI integration for build commands with a custom hook.
func BuildWaveWithHook(cfg *wave.ParsedConfig, log *slog.Logger, hook func(isDev bool) error) {
	if err := BuildWaveWithHookFromArgs(cfg, log, os.Args[1:], hook); err != nil {
		panic(err)
	}
}

// BuildWave is a simplified entry point without a custom hook.
func BuildWave(cfg *wave.ParsedConfig, log *slog.Logger) {
	BuildWaveWithHook(cfg, log, nil)
}
