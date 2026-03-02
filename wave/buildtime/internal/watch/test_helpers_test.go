package watch_test

import (
	"github.com/vormadev/vorma/wave/waveconfig"
	"log/slog"

	"github.com/vormadev/vorma/internal/wavetest"
)

func newDiscardLoggerForWatchTests() *slog.Logger {
	return wavetest.NewDiscardLogger()
}

func newParsedConfigForWatchTestsAtRoot(root string) *waveconfig.ParsedConfig {
	return wavetest.NewParsedConfigAtRoot(root)
}
