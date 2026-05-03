package main

import (
	"docs/app"
	"docs/dist"
	"fmt"
	"net/http"

	"github.com/vormadev/vorma/kit/envutil"
)

func main() {
	port := envutil.GetInt("PORT", 0)
	if port == 0 {
		panic("PORT env var must be set to a valid integer")
	}

	r, err := app.Router(dist.FS)()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize router: %v", err))
	}

	addr := fmt.Sprintf(":%d", port)
	url := "http://localhost" + addr

	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	fmt.Printf("Starting application server at %s\n", url)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(fmt.Sprintf("Application server failed: %v", err))
	}
}
