package build

import (
	"os"
	"slices"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/internal/pkg/vormabuild"
)

type RunArgs struct {
	App     *vorma.Vorma
	Loaders vorma.Loaders
	Actions vorma.Actions
	// Do this in your build entry's main func: `Caller: build.CaptureCaller(runtime.Caller(0))`
	Caller string
}

func Run(args RunArgs) {
	if args.App == nil {
		panic("[build.Run]: args.App is nil")
	}
	if args.Caller == "" {
		panic(
			"[build.Run]: args.Caller is empty. Do this in your build entry's main func: `Caller: build.CaptureCaller(runtime.Caller(0))`",
		)
	}
	vormabuild.Run(
		args.App,
		args.Loaders,
		args.Actions,
		slices.Contains(os.Args[1:], "--dev"),
		args.Caller,
	)
}

// Pass `runtime.Caller(0)` directly into this function.
func CaptureCaller(pc uintptr, file string, line int, ok bool) string {
	return file
}
