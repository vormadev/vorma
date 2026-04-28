package main

import (
	"bombadilfixture/scenario"

	"github.com/vormadev/vorma"
)

func main() {
	if vorma.IsDev() {
		scenario.SelectedVariant().ServeSelectedFromDisk()
		return
	}
	scenario.SelectedVariant().ServeFromDisk()
}
