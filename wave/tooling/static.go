package tooling

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave"
)

var staticIgnoreList = map[string]struct{}{
	".DS_Store": {},
}

func (b *Builder) processPublicFiles(granular bool) error {
	return b.processStaticFiles(staticOpts{
		srcDir:     filepath.Clean(b.cfg.Core.StaticAssetDirs.Public),
		distDir:    b.cfg.Dist.StaticPublic(),
		gobPath:    b.cfg.Dist.PublicFileMapGob(),
		granular:   granular,
		isPublic:   true,
		hashOutput: true,
	})
}

func (b *Builder) processPrivateFiles(granular bool) error {
	return b.processStaticFiles(staticOpts{
		srcDir:     filepath.Clean(b.cfg.Core.StaticAssetDirs.Private),
		distDir:    b.cfg.Dist.StaticPrivate(),
		gobPath:    b.cfg.Dist.PrivateFileMapGob(),
		granular:   granular,
		isPublic:   false,
		hashOutput: false,
	})
}

type staticOpts struct {
	srcDir     string
	distDir    string
	gobPath    string
	granular   bool
	isPublic   bool
	hashOutput bool
}

type fileInfo struct {
	srcPath string
	relPath string
	prehash bool
}

func (b *Builder) processStaticFiles(opts staticOpts) error {
	if _, err := os.Stat(opts.srcDir); os.IsNotExist(err) {
		if err := b.saveFileMap(wave.FileMap{}, opts.gobPath); err != nil {
			return err
		}
		if opts.isPublic {
			return b.savePublicFileMapJS(wave.FileMap{})
		}
		return nil
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

	// Use context for cancellation on error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Discover files
	files := make(chan fileInfo, 100)

	// Track walk errors
	var walkErr error
	var walkErrOnce sync.Once

	go func() {
		defer close(files)
		err := filepath.WalkDir(opts.srcDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Check for cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if d.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(opts.srcDir, path)
			if err != nil {
				return fmt.Errorf("failed to get relative path for %s: %w", path, err)
			}
			relPath = filepath.ToSlash(relPath)

			prehash := false
			prehashedPrefix := wave.PrehashedDirname + "/"
			nohashPrefix := wave.NohashDirname + "/"
			if strings.HasPrefix(relPath, prehashedPrefix) {
				prehash = true
				relPath = strings.TrimPrefix(relPath, prehashedPrefix)
			} else if strings.HasPrefix(relPath, nohashPrefix) {
				prehash = true
				relPath = strings.TrimPrefix(relPath, nohashPrefix)
			}

			if _, ignore := staticIgnoreList[filepath.Base(relPath)]; ignore {
				return nil
			}

			select {
			case files <- fileInfo{srcPath: path, relPath: relPath, prehash: prehash}:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
		if err != nil {
			walkErrOnce.Do(func() {
				// Only record walk error if it's not just a cancellation
				// caused by a worker error (which is stored in firstErr)
				if !errors.Is(err, context.Canceled) {
					walkErr = fmt.Errorf("walk %s: %w", opts.srcDir, err)
				}
				cancel()
			})
		}
	}()

	// Process files with worker pool
	const numWorkers = 32
	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fi := range files {
				select {
				case <-ctx.Done():
					return
				default:
				}

				err := b.processFile(fi, opts, newMap, oldMap)
				if err != nil {
					errOnce.Do(func() {
						firstErr = err
						cancel() // Cancel other workers
					})
					return
				}
			}
		}()
	}

	wg.Wait()

	// Check for errors - prioritize worker errors over walk errors
	// since walk errors may just be "context canceled" from our cancellation
	if firstErr != nil {
		return firstErr
	}
	if walkErr != nil {
		return walkErr
	}

	// Convert to regular map
	finalMap := make(wave.FileMap)
	newMap.Range(func(k, v any) bool {
		finalMap[k.(string)] = v.(wave.FileVal)
		return true
	})

	// Cleanup old files
	if opts.granular && oldMapLoaded {
		for key, oldVal := range oldMap {
			newVal, exists := finalMap[key]
			if !exists || newVal.DistName != oldVal.DistName {
				_ = os.Remove(filepath.Join(opts.distDir, oldVal.DistName))
			}
		}
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

func (b *Builder) processFile(
	fi fileInfo,
	opts staticOpts,
	newMap *sync.Map,
	oldMap wave.FileMap,
) error {
	underscorePath := strings.ReplaceAll(fi.relPath, "/", "_")
	contentHash, err := hashFile(fi.srcPath, underscorePath)
	if err != nil {
		return fmt.Errorf("hash %s: %w", fi.srcPath, err)
	}

	var distName string
	if fi.prehash {
		distName = fi.relPath
	} else if !opts.hashOutput {
		distName = fi.relPath
	} else {
		distName = contentHash
	}

	val := wave.FileVal{
		DistName:    distName,
		ContentHash: contentHash,
		IsPrehashed: fi.prehash,
	}
	newMap.Store(fi.relPath, val)

	var distPath string
	if opts.hashOutput {
		distPath = filepath.Join(opts.distDir, distName)
	} else {
		distPath = filepath.Join(opts.distDir, fi.relPath)
	}

	// Skip if unchanged
	if oldMap != nil {
		if oldVal, ok := oldMap[fi.relPath]; ok {
			if oldVal.ContentHash == contentHash {
				if _, statErr := os.Stat(distPath); statErr == nil {
					return nil
				} else if !os.IsNotExist(statErr) {
					return statErr
				}
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(distPath), 0755); err != nil {
		return err
	}

	return fsutil.CopyFile(fi.srcPath, distPath)
}
func (b *Builder) loadFileMapFromPath(gobPath string) (wave.FileMap, error) {
	f, err := os.Open(gobPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return fsutil.FromGob[wave.FileMap](f)
}

func (b *Builder) saveFileMap(fm wave.FileMap, gobPath string) error {
	return writeFileAtomic(gobPath, func(f *os.File) error {
		return gob.NewEncoder(f).Encode(fm)
	})
}

func (b *Builder) savePublicFileMapJS(fm wave.FileMap) error {
	simpleMap := make(map[string]string, len(fm))
	for k, v := range fm {
		simpleMap[k] = v.DistName
	}

	jsonBytes, err := json.Marshal(simpleMap)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("export const wavePublicFileMap = %s;", string(jsonBytes))
	hashedName := hashBytes([]byte(content), wave.RelPaths.PublicFileMapJSName())
	publicDir := b.cfg.Dist.StaticPublic()
	refPath := b.cfg.Dist.PublicFileMapRef()

	if existingRefBytes, readErr := os.ReadFile(refPath); readErr == nil {
		existingHashedName := strings.TrimSpace(string(existingRefBytes))
		if existingHashedName == hashedName {
			existingHashedPath := filepath.Join(publicDir, existingHashedName)
			if _, statErr := os.Stat(existingHashedPath); statErr == nil {
				return nil
			}
		}
	}

	// Cleanup old files
	oldFiles, err := filepath.Glob(filepath.Join(publicDir, wave.FileMapJSGlobPattern))
	if err != nil {
		b.log.Warn("failed to glob old filemap files", "error", err)
	}
	for _, old := range oldFiles {
		if err := os.Remove(old); err != nil {
			b.log.Warn("failed to remove old filemap file", "file", old, "error", err)
		}
	}

	// Write ref file atomically
	if err := writeFileAtomicBytes(refPath, []byte(hashedName)); err != nil {
		return err
	}

	// Write JS file atomically
	return writeFileAtomicBytes(filepath.Join(publicDir, hashedName), []byte(content))
}

// WritePublicFileMapTS writes the public file map as a TypeScript file to the specified directory.
// This enables Vite HMR to pick up public static file changes without running Go.
func (b *Builder) WritePublicFileMapTS(outDir string) error {
	fm, err := b.loadOrBuildFileMap()
	if err != nil {
		return fmt.Errorf("load file map: %w", err)
	}

	// Collect and sort keys for deterministic output
	keys := make([]string, 0, len(fm))
	for k := range fm {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("/////// Auto-generated by Wave. Do not edit.\n\n")
	sb.WriteString("export const staticPublicAssetMap = {\n")

	for _, k := range keys {
		v := fm[k]
		sb.WriteString(fmt.Sprintf("\t%q: %q,\n", k, v.DistName))
	}

	sb.WriteString("} as const;\n")

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(outDir, wave.RelPaths.PublicFileMapTSName())
	if _, err := writeFileAtomicBytesIfChanged(outPath, []byte(sb.String())); err != nil {
		return fmt.Errorf("write TS file: %w", err)
	}

	// Also write JSON version for Vite plugin dev mode cache invalidation
	if err := b.writePublicFileMapJSON(outDir, fm); err != nil {
		return fmt.Errorf("write JSON file: %w", err)
	}

	return nil
}

// writePublicFileMapJSON writes the public file map as a JSON file for the Vite plugin
// to read in dev mode. This allows the plugin to dynamically reload the filemap
// without restarting Vite.
func (b *Builder) writePublicFileMapJSON(outDir string, fm wave.FileMap) error {
	simpleMap := make(map[string]string, len(fm))
	for k, v := range fm {
		simpleMap[k] = v.DistName
	}

	jsonBytes, err := json.MarshalIndent(simpleMap, "", "\t")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	outPath := filepath.Join(outDir, wave.RelPaths.PublicFileMapJSONName())
	_, err = writeFileAtomicBytesIfChanged(outPath, jsonBytes)
	return err
}

// writeFileAtomic writes data to a file atomically using a randomized temp file
// and rename. The write function is called with the temp file to write content.
func writeFileAtomic(path string, write func(*os.File) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create temp file with randomized name to avoid race conditions
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on any error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Write content
	if err := write(tmpFile); err != nil {
		tmpFile.Close()
		return err
	}

	// Close before rename
	if err := tmpFile.Close(); err != nil {
		return err
	}

	// Rename to final destination
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	success = true
	return nil
}

// writeFileAtomicBytes is a convenience wrapper for writing byte slices atomically
func writeFileAtomicBytes(path string, data []byte) error {
	return writeFileAtomic(path, func(f *os.File) error {
		_, err := f.Write(data)
		return err
	})
}

func writeFileAtomicBytesIfChanged(path string, data []byte) (bool, error) {
	existingData, readErr := os.ReadFile(path)
	if readErr == nil {
		if bytes.Equal(existingData, data) {
			return false, nil
		}
	} else if !os.IsNotExist(readErr) {
		return false, readErr
	}

	if err := writeFileAtomicBytes(path, data); err != nil {
		return false, err
	}

	return true, nil
}
