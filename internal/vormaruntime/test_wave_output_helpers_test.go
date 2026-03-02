package vormaruntime

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

func testWaveOutPrefixedFileName(fileNameSuffix string) string {
	return waveartifacts.ApplyWaveFileOutputPrefix(fileNameSuffix)
}
