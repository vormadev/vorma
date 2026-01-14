package main

import (
	"log"

	"site/backend/src/router"

	"github.com/vormadev/vorma/fw/build"
)

func main() {
	if err := build.RunBuildCLI(router.App); err != nil {
		log.Fatal(err)
	}
}
