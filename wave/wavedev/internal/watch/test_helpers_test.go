package watch_test

import (
	"log/slog"

	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave"
)

func newDiscardLoggerForWatchTests() *slog.Logger {
	return wavetest.NewDiscardLogger()
}

func newParsedConfigForWatchTestsAtRoot(root string) *wave.ParsedConfig {
	return wavetest.NewParsedConfigAtRoot(root)
}
