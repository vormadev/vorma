package main

import (
	"site/backend/internal/docsync"
	"site/backend/src/app"

	"github.com/vormadev/vorma/vormabuild"
)

func main() {
	appRuntime := app.App

	if _, err := docsync.SyncAndResolvePublicURLs(
		appRuntime.Wave.ParsedConfig(),
		appRuntime.Logger(),
	); err != nil {
		panic(err)
	}
	vormabuild.Build(appRuntime)
}
