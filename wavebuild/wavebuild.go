// Package wavebuild provides the end-user build entrypoint for Wave apps.
//
// The public surface stays intentionally tiny: `Build` for CLI-driven usage,
// and `BuildWithOptions` for explicit programmatic invocation.
package wavebuild

import (
	"log"
	"os"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/buildtime/buildentry"
)

// BuildOptions configures one wavebuild execution.
type BuildOptions struct {
	Dev               bool
	HookOnly          bool
	SkipGoBinaryBuild bool
}

// Build parses CLI flags from os.Args and executes wavebuild.
//
// Supported flags:
// --dev, --hook, --no-binary.
func Build(w *wave.Wave) {
	parsedOptions, parseError := buildentry.ParseCommandOptions(os.Args[1:])
	if parseError != nil {
		log.Fatalf("%v", parseError)
	}
	if runError := BuildWithOptions(
		w,
		BuildOptions{
			Dev:               parsedOptions.Dev,
			HookOnly:          parsedOptions.HookOnly,
			SkipGoBinaryBuild: parsedOptions.SkipGoBinaryBuild,
		},
	); runError != nil {
		log.Fatalf("%v", runError)
	}
}

// BuildWithOptions executes wavebuild with explicit options.
func BuildWithOptions(w *wave.Wave, options BuildOptions) error {
	return buildentry.RunWithOptions(
		w,
		buildentry.Options{
			Dev:               options.Dev,
			HookOnly:          options.HookOnly,
			SkipGoBinaryBuild: options.SkipGoBinaryBuild,
		},
	)
}
