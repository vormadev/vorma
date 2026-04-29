package main

import (
	"frameworktests/scenario"
	"runtime"
)

func main() {
	scenario.SelectedVariant().Build(runtime.Caller(0))
}
