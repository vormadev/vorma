// Package buildentry owns wavebuild command parsing and execution orchestration.
//
// This package keeps build-mode dispatch details out of the end-user
// `wavebuild` facade.
package buildentry

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/buildtime/devserver"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveframework"
)

// Options configures one wavebuild execution.
type Options struct {
	Dev               bool
	HookOnly          bool
	SkipGoBinaryBuild bool
}

type commandOptions struct {
	runInDevelopmentMode  bool
	runHookOnly           bool
	skipGoBinaryBuildStep bool
}

// ParseCommandOptions parses CLI flags into build options.
func ParseCommandOptions(commandLineArgs []string) (Options, error) {
	parsedOptions, parseError := parseCommandOptions(commandLineArgs)
	if parseError != nil {
		return Options{}, fmt.Errorf("parse build flags: %w", parseError)
	}
	return Options{
		Dev:               parsedOptions.runInDevelopmentMode,
		HookOnly:          parsedOptions.runHookOnly,
		SkipGoBinaryBuild: parsedOptions.skipGoBinaryBuildStep,
	}, nil
}

// RunWithOptions executes wavebuild with explicit options.
func RunWithOptions(w *wave.Wave, options Options) error {
	if w == nil {
		return errors.New("wave runtime is required")
	}
	if options.Dev {
		w.SetModeToDev()
	}

	buildtimeConfig := waveframework.BuildtimeParsedConfig(w.RawConfigJSON())
	if buildtimeConfig == nil {
		return errors.New("wave build config is required")
	}

	if options.HookOnly {
		if hookOnlyRunError := runHookOnlyBuild(
			buildtimeConfig,
			w.Logger(),
			options.Dev,
		); hookOnlyRunError != nil {
			return fmt.Errorf("run hook-only build: %w", hookOnlyRunError)
		}
		return nil
	}

	if options.Dev {
		if runDevelopmentError := devserver.RunDev(
			buildtimeConfig,
			w.Logger(),
		); runDevelopmentError != nil {
			return fmt.Errorf(
				"run development server: %w",
				runDevelopmentError,
			)
		}
		return nil
	}

	if productionBuildError := runProductionBuild(
		buildtimeConfig,
		w.Logger(),
		options.SkipGoBinaryBuild,
	); productionBuildError != nil {
		return fmt.Errorf("run production build: %w", productionBuildError)
	}
	return nil
}

func parseCommandOptions(commandLineArgs []string) (commandOptions, error) {
	var options commandOptions

	flagSet := flag.NewFlagSet("wavebuild", flag.ContinueOnError)
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
		"run build hook only",
	)
	flagSet.BoolVar(
		&options.skipGoBinaryBuildStep,
		"no-binary",
		false,
		"skip go binary compilation",
	)
	if parseError := flagSet.Parse(commandLineArgs); parseError != nil {
		return commandOptions{}, parseError
	}
	if remainingArgs := flagSet.Args(); len(remainingArgs) > 0 {
		return commandOptions{}, fmt.Errorf(
			"unexpected positional arguments: %s",
			strings.Join(remainingArgs, ", "),
		)
	}

	return options, nil
}

func runHookOnlyBuild(
	cfg *waveconfig.ParsedConfig,
	log *slog.Logger,
	runInDevelopmentMode bool,
) error {
	builderInstance := builder.NewBuilder(cfg, log)
	defer func() { _ = builderInstance.Close() }()
	return builderInstance.RunBuildHooksOnly(runInDevelopmentMode)
}

func runProductionBuild(
	cfg *waveconfig.ParsedConfig,
	log *slog.Logger,
	skipGoBinaryBuildStep bool,
) error {
	builderInstance := builder.NewBuilder(cfg, log)
	defer func() { _ = builderInstance.Close() }()
	return builderInstance.Build(
		builder.BuildOpts{
			CompileGo:                          !skipGoBinaryBuildStep,
			IsDev:                              false,
			IsRebuild:                          false,
			FileOnlyMode:                       false,
			SkipPostHookPublicStaticProcessing: false,
		},
	)
}
