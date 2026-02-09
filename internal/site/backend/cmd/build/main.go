package main

import (
	"site/backend/internal/docsync"
	"site/backend/src/router"

	"github.com/vormadev/vorma/vormabuild"
)

func main() {
	if _, err := docsync.SyncAndResolvePublicURLs(router.App.GetParsedConfig(), router.App.Logger()); err != nil {
		panic(err)
	}
	vormabuild.Build(router.App)
}
