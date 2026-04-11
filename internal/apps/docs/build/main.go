package main

import (
	"docs/app"
	"runtime"

	"github.com/vormadev/vorma/build"
)

func main() {
	build.Run(build.RunArgs{
		App:     app.App,
		Loaders: app.Loaders,
		Actions: app.Actions,
		Caller:  build.CaptureCaller(runtime.Caller(0)),
	})
}
