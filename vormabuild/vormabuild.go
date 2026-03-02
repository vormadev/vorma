// Package vormabuild provides the end-user build entrypoint for Vorma apps.
//
// The public surface stays intentionally tiny: `Build` for CLI-driven usage,
// and `BuildWithOptions` for explicit programmatic invocation.
package vormabuild

import (
	"errors"
	"log"
	"os"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormabuild/internal/buildentry"
)

// BuildOptions configures one vormabuild execution.
type BuildOptions struct {
	Dev               bool
	HookOnly          bool
	SkipGoBinaryBuild bool
}

// Build parses CLI flags from os.Args and executes vormabuild.
//
// Supported flags:
// --dev, --hook, --no-binary.
func Build(v *vorma.Vorma) {
	parsedOptions, parseError := buildentry.ParseCommandOptions(os.Args[1:])
	if parseError != nil {
		log.Fatalf("%v", parseError)
	}
	if runError := BuildWithOptions(
		v,
		BuildOptions{
			Dev:               parsedOptions.Dev,
			HookOnly:          parsedOptions.HookOnly,
			SkipGoBinaryBuild: parsedOptions.SkipGoBinaryBuild,
		},
	); runError != nil {
		log.Fatalf("%v", runError)
	}
}

// BuildWithOptions executes vormabuild with explicit options.
func BuildWithOptions(v *vorma.Vorma, options BuildOptions) error {
	if v == nil {
		return errors.New("vorma app is required")
	}
	return buildentry.RunWithOptions(
		v.UnsafeRuntimeForFrameworkInternals(),
		buildentry.Options{
			Dev:               options.Dev,
			HookOnly:          options.HookOnly,
			SkipGoBinaryBuild: options.SkipGoBinaryBuild,
		},
	)
}
