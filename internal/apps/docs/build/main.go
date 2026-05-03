package main

import (
	"docs/app"
	"runtime"

	"github.com/vormadev/vorma/build"
)

func main() {
	build.Run(app.Router(nil), build.Caller(runtime.Caller(0)))
}
