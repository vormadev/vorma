package server

import (
	"context"
	"docs/app/router"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/vormadev/vorma/kit/envutil"
	"github.com/vormadev/vorma/kit/grace"
)

func Start() {
	port := envutil.GetInt("PORT", 0)
	if port == 0 {
		panic("PORT env var must be set to a valid integer")
	}

	r, err := router.Router()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize router: %v", err))
	}

	addr := fmt.Sprintf(":%d", port)
	url := "http://localhost" + addr

	server := &http.Server{
		Addr: addr,
		Handler: http.TimeoutHandler(
			r,
			60*time.Second,
			"Request timed out",
		),
		ReadTimeout:                  15 * time.Second,
		WriteTimeout:                 30 * time.Second,
		IdleTimeout:                  60 * time.Second,
		ReadHeaderTimeout:            10 * time.Second,
		MaxHeaderBytes:               1 << 20, // 1 MB
		DisableGeneralOptionsHandler: true,
		ErrorLog: log.New(
			os.Stderr,
			"HTTP: ",
			log.Ldate|log.Ltime|log.Lshortfile,
		),
	}

	lifecycle := grace.Lifecycle{
		Startup: func() error {
			fmt.Printf("Starting server at %s\n", url)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("server error: %w", err)
			}
			return nil
		},
		Shutdown: func(shutdownCtx context.Context) error {
			fmt.Println("Shutting down server...")
			if err := server.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("server shutdown error: %w", err)
			}
			return nil
		},
	}

	lifecycle.Run()
}
