package build

import (
	"os"
	"slices"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/internal/pkg/vormabuild"
	"github.com/vormadev/vorma/internal/pkg/vormarun"
)

// Usage: `build.Run(routerInitFunc, build.Caller(runtime.Caller(0)))`
func Run(routerInitFunc func() (*vorma.Router, error), caller string) {
	os.Setenv(vormarun.Env_Key_Is_Build, "1")
	if routerInitFunc == nil {
		panic("[build.Run]: *vorma.Router getter is nil")
	}
	router, err := routerInitFunc()
	if err != nil {
		panic("[build.Run]: failed to initialize router: " + err.Error())
	}
	if router == nil {
		panic("[build.Run]: *vorma.Router is nil")
	}
	if caller == "" {
		panic(
			"[build.Run]: caller is empty. Proper usage: `build.Run(routerInitFunc, build.Caller(runtime.Caller(0)))`",
		)
	}
	vormabuild.Run(
		router,
		caller,
		slices.Contains(os.Args[1:], "--dev"),
	)
}

// Pass `runtime.Caller(0)` directly into this function.
func Caller(pc uintptr, file string, line int, ok bool) string {
	return file
}
