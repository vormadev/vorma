package watch_test

import (
	"github.com/vormadev/vorma/wave/waveconfig"
	"log/slog"
	"testing"

	"github.com/vormadev/vorma/internal/wavetest"
)

func newDiscardLoggerForWatchTests() *slog.Logger {
	return wavetest.NewDiscardLogger()
}

func newParsedConfigForWatchTestsAtRoot(
	tb testing.TB,
	root string,
) waveconfig.ParsedConfig {
	return wavetest.NewParsedConfigAtRoot(tb, root)
}
