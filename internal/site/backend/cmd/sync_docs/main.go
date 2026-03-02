package main

import (
	"flag"
	"github.com/vormadev/vorma/wave/waveframework"
	"log"

	"site/backend"
	"site/backend/internal/docsync"
)

func main() {
	rewriteOnly := flag.Bool(
		"rewrite-only",
		false,
		"only rewrite generated docs with current public file map",
	)
	flag.Parse()

	if *rewriteOnly {
		rewritten, err := docsync.ResolveGeneratedDocsPublicURLs(
			waveframework.ParsedConfig(backend.Wave.RawConfigJSON()),
			backend.Wave.Logger(),
		)
		if err != nil {
			log.Fatalf("rewrite docs URLs failed: %v", err)
		}
		log.Printf("rewrote generated docs public URLs: files=%d", rewritten)
		return
	}

	res, err := docsync.SyncAndResolvePublicURLs(
		waveframework.ParsedConfig(backend.Wave.RawConfigJSON()),
		backend.Wave.Logger(),
	)
	if err != nil {
		log.Fatalf("sync docs failed: %v", err)
	}
	log.Printf(
		"synced reference docs from README files: sources=%d written=%d deleted=%d assets_written=%d assets_deleted=%d urls_rewritten=%d",
		res.Sources,
		res.Written,
		res.Deleted,
		res.AssetsWritten,
		res.AssetsDeleted,
		res.URLsRewritten,
	)
}
