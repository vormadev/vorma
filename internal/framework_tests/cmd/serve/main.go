package main

import (
	"frameworktests/scenario"

	"github.com/vormadev/vorma"
)

func main() {
	if vorma.IsDev() {
		scenario.SelectedVariant().ServeSelectedFromDisk()
		return
	}
	scenario.SelectedVariant().ServeFromDisk()
}
