package runtimehttp

import (
	"path"
	"strings"

	"github.com/vormadev/vorma/wave/waveartifacts"
)

func testWaveOutPath(relativePath string) string {
	return path.Join(
		waveartifacts.HashedOutputDirname,
		strings.TrimPrefix(relativePath, "/"),
	)
}
