package rendering

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

func testWaveOutURLPath(relativePath string) string {
	return "/" + testWaveOutPath(relativePath)
}

func testWaveOutAssetPath(relativePath string) string {
	return path.Join(waveartifacts.AssetsDirname, testWaveOutPath(relativePath))
}
