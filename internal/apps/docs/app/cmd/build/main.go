package main

import (
	"docs/app/router"
	"runtime"

	"github.com/vormadev/vorma/build"
)

func main() {
	build.Run(router.Router, build.Caller(runtime.Caller(0)))
}
