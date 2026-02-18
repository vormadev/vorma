package main

import (
	"site/backend/internal/docsync"
	"site/backend/src/router"

	"github.com/vormadev/vorma/vormabuild"
)

func main() {
	app := router.App

	if _, err := docsync.SyncAndResolvePublicURLs(
		app.ParsedConfig(),
		app.Logger(),
	); err != nil {
		panic(err)
	}
	vormabuild.Build(app)
}
