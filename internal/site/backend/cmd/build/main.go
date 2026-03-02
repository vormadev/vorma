package main

import (
	"github.com/vormadev/vorma/wave/waveframework"
	"site/backend/internal/docsync"
	"site/backend/src/router"

	"github.com/vormadev/vorma/vormabuild"
)

func main() {
	app := router.App

	if _, err := docsync.SyncAndResolvePublicURLs(
		waveframework.ParsedConfig(app.Wave.RawConfigJSON()),
		app.Logger(),
	); err != nil {
		panic(err)
	}
	vormabuild.Build(app)
}
