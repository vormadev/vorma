package tooling

import (
	"io"
	"log/slog"
	"path/filepath"

	"github.com/vormadev/vorma/wave"
)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newParsedConfigForToolingTestsAtRoot(root string) *wave.ParsedConfig {
	cfg := &wave.ParsedConfig{
		Core: &wave.CoreConfig{
			MainAppEntry: "cmd/app",
			DistDir:      filepath.Join(root, "dist"),
			StaticAssetDirs: wave.StaticAssetDirs{
				Public:  filepath.Join(root, "static", "public"),
				Private: filepath.Join(root, "static", "private"),
			},
		},
		Watch: &wave.WatchConfig{
			WatchRoot: root,
		},
	}
	cfg.Dist = wave.DistLayout{Root: cfg.Core.DistDir}
	return cfg
}
