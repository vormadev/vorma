//go:build !prod

package backend

import (
	"log/slog"
	"os"

	"github.com/vormadev/vorma/wave"
)

var l = func() *slog.Logger {
	// Configure the handler options to include the Debug level.
	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelDebug, // Explicitly set the minimum log level to Debug
	}

	// Create a new TextHandler with the specified options, writing to stderr.
	handler := slog.NewTextHandler(os.Stderr, handlerOptions)

	// Create a new logger with the configured handler and set it as the default.
	logger := slog.New(handler)

	return logger
}()

var Wave = wave.New(wave.Config{
	FS:         os.DirFS("backend"),
	ConfigPath: "wave.config.json",
	Logger:     l,
})
