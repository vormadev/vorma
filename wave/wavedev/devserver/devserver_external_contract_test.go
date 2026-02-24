package devserver_test

import (
	"log/slog"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/wavedev/devserver"
)

// Compile-time contract for non-tooling in-repo consumers (e.g. vormabuild).
var _ func(*wave.ParsedConfig, *slog.Logger) error = devserver.RunDev
