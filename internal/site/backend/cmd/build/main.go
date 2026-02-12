package main

import (
	"site/backend/internal/docsync"
	"site/backend/src/router"

	"github.com/vormadev/vorma/vormabuild"
)

func main() {
	app := router.GetApp()

	if _, err := docsync.SyncAndResolvePublicURLs(app.GetParsedConfig(), app.Logger()); err != nil {
		panic(err)
	}
	vormabuild.Build(app)
}
