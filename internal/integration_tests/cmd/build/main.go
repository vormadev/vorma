package main

import (
	"bombadilfixture/scenario"
	"runtime"
)

func main() {
	scenario.SelectedVariant().Build(runtime.Caller(0))
}
