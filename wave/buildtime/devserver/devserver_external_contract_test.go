package devserver_test

import (
	"github.com/vormadev/vorma/wave/waveconfig"
	"log/slog"

	"github.com/vormadev/vorma/wave/buildtime/devserver"
)

// Compile-time contract for non-tooling in-repo consumers (e.g. vormabuild).
var _ func(*waveconfig.ParsedConfig, *slog.Logger) error = devserver.RunDev
