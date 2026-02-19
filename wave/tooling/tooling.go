package tooling

import (
	"log/slog"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/devserver"
)

// RunDev runs Wave's development server orchestration.
func RunDev(cfg *wave.ParsedConfig, log *slog.Logger) error {
	return devserver.RunDev(cfg, log)
}
