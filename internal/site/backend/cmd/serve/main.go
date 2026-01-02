package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"site/backend/src/router"
	"time"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/grace"
)

var Log = colorlog.New("site")

func main() {
	addr, handler := router.Init()
	url := "http://localhost" + addr

	server := &http.Server{
		Addr:                         addr,
		Handler:                      http.TimeoutHandler(handler, 60*time.Second, "Request timed out"),
		ReadTimeout:                  15 * time.Second,
		WriteTimeout:                 30 * time.Second,
		IdleTimeout:                  60 * time.Second,
		ReadHeaderTimeout:            10 * time.Second,
		MaxHeaderBytes:               1 << 20, // 1 MB
		DisableGeneralOptionsHandler: true,
		ErrorLog:                     log.New(os.Stderr, "HTTP: ", log.Ldate|log.Ltime|log.Lshortfile),
	}

	grace.Orchestrate(grace.OrchestrateOptions{
		StartupCallback: func() error {
			Log.Info("Starting server", "url", url)

			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Server listen and serve error: %v\n", err)
			}

			return nil
		},

		ShutdownCallback: func(shutdownCtx context.Context) error {
			Log.Info("Shutting down server", "url", url)

			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Fatalf("Server shutdown error: %v\n", err)
			}

			return nil
		},
	})
}
