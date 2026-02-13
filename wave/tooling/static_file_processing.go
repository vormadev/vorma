package tooling

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave"
)

func (b *Builder) processStaticFileAndReturnValue(
	staticFileInfo fileInfo,
	opts staticOpts,
	oldMap wave.FileMap,
) (wave.FileVal, error) {
	underscorePath := strings.ReplaceAll(staticFileInfo.relPath, "/", "_")
	contentHash, hashError := hashFile(staticFileInfo.srcPath, underscorePath)
	if hashError != nil {
		return wave.FileVal{}, fmt.Errorf("hash %s: %w", staticFileInfo.srcPath, hashError)
	}

	distName := deriveStaticDistName(staticFileInfo, opts.hashOutput, contentHash)
	processedValue := wave.FileVal{
		DistName:    distName,
		ContentHash: contentHash,
		IsPrehashed: staticFileInfo.prehash,
	}
	distPath := deriveStaticDistPath(staticFileInfo, opts, distName)

	if oldMap != nil {
		if oldVal, hasOldValue := oldMap[staticFileInfo.relPath]; hasOldValue && oldVal.ContentHash == contentHash {
			if _, statErr := os.Stat(distPath); statErr == nil {
				return processedValue, nil
			} else if !os.IsNotExist(statErr) {
				return wave.FileVal{}, statErr
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(distPath), 0o755); err != nil {
		return wave.FileVal{}, err
	}

	if err := fsutil.CopyFile(staticFileInfo.srcPath, distPath); err != nil {
		return wave.FileVal{}, err
	}

	return processedValue, nil
}

func deriveStaticDistName(
	staticFileInfo fileInfo,
	hashOutput bool,
	contentHash string,
) string {
	if staticFileInfo.prehash {
		return staticFileInfo.relPath
	}
	if !hashOutput {
		return staticFileInfo.relPath
	}
	return contentHash
}

func deriveStaticDistPath(
	staticFileInfo fileInfo,
	opts staticOpts,
	distName string,
) string {
	if opts.hashOutput {
		return filepath.Join(opts.distDir, distName)
	}
	return filepath.Join(opts.distDir, staticFileInfo.relPath)
}

func (b *Builder) processFile(
	fi fileInfo,
	opts staticOpts,
	newMap *sync.Map,
	oldMap wave.FileMap,
) error {
	processedValue, processError := b.processStaticFileAndReturnValue(
		fi,
		opts,
		oldMap,
	)
	if processError != nil {
		return processError
	}

	newMap.Store(fi.relPath, processedValue)
	return nil
}
