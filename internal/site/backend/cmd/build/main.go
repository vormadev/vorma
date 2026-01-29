package main

import (
	"site/backend/src/router"

	"github.com/vormadev/vorma/vormabuild"
)

func main() {
	vormabuild.Build(router.App)
}
