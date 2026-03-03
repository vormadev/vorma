// Package waveoutputtest centralizes Wave output path helpers shared by tests.
package waveoutputtest

import (
	"path"
	"strings"

	"github.com/vormadev/vorma/wave/waveartifacts"
)

// TestWaveOutputPath builds a hashed output-relative path used in test fixtures.
func TestWaveOutputPath(relativePath string) string {
	return path.Join(
		waveartifacts.HashedOutputDirname,
		strings.TrimPrefix(relativePath, "/"),
	)
}

// TestWaveOutputURLPath builds a rooted URL path for a hashed output file.
func TestWaveOutputURLPath(relativePath string) string {
	return "/" + TestWaveOutputPath(relativePath)
}

// TestWaveOutputAssetPath builds an assets-directory path for hashed output.
func TestWaveOutputAssetPath(relativePath string) string {
	return path.Join(waveartifacts.AssetsDirname, TestWaveOutputPath(relativePath))
}

// TestPublicWaveOutputURLPath builds a public URL path for a hashed output file.
func TestPublicWaveOutputURLPath(relativePath string) string {
	return path.Join(
		"/"+waveartifacts.PublicDirname,
		TestWaveOutputPath(relativePath),
	)
}

// TestWaveOutputPrefixedFileName adds the standard Wave output filename prefix.
func TestWaveOutputPrefixedFileName(fileNameSuffix string) string {
	return waveartifacts.ApplyWaveFileOutputPrefix(fileNameSuffix)
}
