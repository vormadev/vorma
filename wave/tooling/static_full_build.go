package tooling

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sync"

	"github.com/vormadev/vorma/wave"
)

func (b *Builder) processStaticFiles(opts staticOpts) error {
	if _, err := os.Stat(opts.srcDir); os.IsNotExist(err) {
		return b.clearStaticOutputsWhenSourceDirectoryMissing(opts)
	}

	newMap := &sync.Map{}
	var oldMap wave.FileMap
	oldMapLoaded := false

	if opts.granular {
		old, err := b.loadFileMapFromPath(opts.gobPath)
		if err == nil {
			oldMap = old
			oldMapLoaded = true
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	files := make(chan fileInfo, 100)
	discoveredFileInfosByRelativePath := make(map[string]fileInfo)

	var walkErr error
	var walkErrOnce sync.Once

	go func() {
		defer close(files)
		err := filepath.WalkDir(opts.srcDir, func(path string, directoryEntry fs.DirEntry, walkEntryErr error) error {
			if walkEntryErr != nil {
				return walkEntryErr
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if directoryEntry.IsDir() {
				return nil
			}

			staticFileInfo, shouldProcessFile, resolveError := resolveStaticFileInfoFromSourcePath(
				opts.srcDir,
				path,
			)
			if resolveError != nil {
				return fmt.Errorf("resolve static file info for %s: %w", path, resolveError)
			}
			if !shouldProcessFile {
				return nil
			}

			existingFileInfo, alreadyDiscovered := discoveredFileInfosByRelativePath[staticFileInfo.relPath]
			if alreadyDiscovered {
				if collisionError := ensureNoStaticLogicalPathCollision(existingFileInfo, staticFileInfo); collisionError != nil {
					return collisionError
				}
				return nil
			}
			discoveredFileInfosByRelativePath[staticFileInfo.relPath] = staticFileInfo

			select {
			case files <- staticFileInfo:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
		if err != nil {
			walkErrOnce.Do(func() {
				if !errors.Is(err, context.Canceled) {
					walkErr = fmt.Errorf("walk %s: %w", opts.srcDir, err)
				}
				cancel()
			})
		}
	}()

	numWorkers := determineStaticProcessingWorkerCountFromRuntime()
	var waitGroup sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for range numWorkers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for staticFileInfo := range files {
				select {
				case <-ctx.Done():
					return
				default:
				}

				processError := b.processFile(staticFileInfo, opts, newMap, oldMap)
				if processError != nil {
					errOnce.Do(func() {
						firstErr = processError
						cancel()
					})
					return
				}
			}
		}()
	}

	waitGroup.Wait()

	if firstErr != nil {
		return firstErr
	}
	if walkErr != nil {
		return walkErr
	}

	finalMap := make(wave.FileMap)
	newMap.Range(func(key, val any) bool {
		finalMap[key.(string)] = val.(wave.FileVal)
		return true
	})

	if opts.granular && oldMapLoaded {
		cleanupStaleStaticDistFiles(opts.distDir, oldMap, finalMap)
	}

	shouldSaveFileMap := !oldMapLoaded || !maps.Equal(oldMap, finalMap)
	if shouldSaveFileMap {
		if err := b.saveFileMap(finalMap, opts.gobPath); err != nil {
			return err
		}
	}

	if opts.isPublic {
		return b.savePublicFileMapJS(finalMap)
	}

	return nil
}

func (b *Builder) clearStaticOutputsWhenSourceDirectoryMissing(
	opts staticOpts,
) error {
	emptyMap := wave.FileMap{}

	previousMap, loadError := b.loadFileMapFromPath(opts.gobPath)
	if loadError == nil {
		cleanupStaleStaticDistFiles(opts.distDir, previousMap, emptyMap)
	}

	if err := b.saveFileMap(emptyMap, opts.gobPath); err != nil {
		return err
	}
	if opts.isPublic {
		return b.savePublicFileMapJS(emptyMap)
	}
	return nil
}
