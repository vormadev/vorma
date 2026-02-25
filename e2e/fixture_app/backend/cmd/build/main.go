package main

import (
	"e2eapp/backend/src/router"

	"github.com/vormadev/vorma/vormabuild"
)

func main() {
	vormabuild.Build(router.App)
}
