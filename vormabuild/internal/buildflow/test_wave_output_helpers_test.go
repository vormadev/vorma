package buildflow

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

func testWaveOutAssetPath(relativePath string) string {
	return path.Join(waveartifacts.AssetsDirname, testWaveOutPath(relativePath))
}

func testWaveOutPrefixedFileName(fileNameSuffix string) string {
	return waveartifacts.ApplyWaveFileOutputPrefix(fileNameSuffix)
}
