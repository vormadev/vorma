package ki

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/vormadev/vorma/kit/errutil"
	"github.com/vormadev/vorma/kit/esbuildutil"
	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/typed"
	"github.com/vormadev/vorma/wave/internal/ki/configschema"
	"golang.org/x/sync/errgroup"
)

func (c *Config) GetStaticPrivateOutDir() string {
	return c._dist.S().Static.S().Assets.S().Private.FullPath()
}

func (c *Config) GetStaticPublicOutDir() string {
	return c._dist.S().Static.S().Assets.S().Public.FullPath()
}

const PrehashedDirname = "prehashed"

var noHashPublicDirsByVersion = map[uint8]string{0: "__nohash", 1: PrehashedDirname}

type BuildOptions struct {
	IsDev                      bool
	RecompileGoBinary          bool
	just_run_simple_file_build bool
	is_dev_rebuild             bool
}

func (c *Config) do_build_time_file_processing(shouldBeGranular bool) error {
	if !shouldBeGranular {
		// nuke the dist/static directory
		if err := os.RemoveAll(c._dist.S().Static.FullPath()); err != nil {
			return fmt.Errorf("error removing dist/static directory: %w", err)
		}

		// re-make required directories
		if err := c.SetupDistDir(); err != nil {
			return fmt.Errorf("error making requisite directories: %w", err)
		}
	}

	if c.is_using_browser() {
		// Must be complete before BuildCSS in case the CSS references any public files
		if err := c.handlePublicFiles(shouldBeGranular); err != nil {
			return fmt.Errorf("error handling public files: %w", err)
		}

		var eg errgroup.Group
		eg.Go(func() error {
			return errutil.Maybe("error during precompile task (copyPrivateFiles)", c.copyPrivateFiles(shouldBeGranular))
		})
		eg.Go(func() error {
			return errutil.Maybe("error during precompile task (buildCSS)", c.buildCSS())
		})
		if err := eg.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) BuildWave(opts BuildOptions) error {
	a := time.Now()

	if !opts.just_run_simple_file_build {
		c.Logger.Info("START building Wave",
			"recompile_go_binary", opts.RecompileGoBinary,
			"is_dev_rebuild", opts.is_dev_rebuild,
		)
	}

	err := c.do_build_time_file_processing(opts.is_dev_rebuild) // once before build hook
	if err != nil {
		return fmt.Errorf("error processing build time files: %w", err)
	}

	if opts.just_run_simple_file_build {
		return nil
	}

	hook_start := time.Now()

	with_dev_hook := opts.IsDev && c._uc.Core.DevBuildHook != ""
	if with_dev_hook {
		if err := executil.RunCmd(strings.Fields(c._uc.Core.DevBuildHook)...); err != nil {
			return fmt.Errorf("error running dev build command: %w", err)
		}
	}

	with_prod_hook := !opts.IsDev && c._uc.Core.ProdBuildHook != ""
	if with_prod_hook {
		if err := executil.RunCmd(strings.Fields(c._uc.Core.ProdBuildHook)...); err != nil {
			return fmt.Errorf("error running prod build command: %w", err)
		}
	}

	hook_duration := time.Since(hook_start)

	err = c.do_build_time_file_processing(true) // and once again after
	if err != nil {
		return fmt.Errorf("error processing build time files: %w", err)
	}

	err = configschema.Write(filepath.Join(
		c._dist.S().Static.S().Internal.FullPath(),
		"schema.json",
	))
	if err != nil {
		return fmt.Errorf("error writing config schema: %w", err)
	}

	go_compile_start := time.Now()

	if opts.RecompileGoBinary {
		if err := c.compile_go_binary(opts.IsDev); err != nil {
			return fmt.Errorf("error compiling binary: %w", err)
		}
	}

	go_compile_duration := time.Since(go_compile_start)

	total_duration := time.Since(a)

	c.Logger.Info("DONE building Wave",
		"total_duration", total_duration,
		"hook_duration", hook_duration,
		"go_compile_duration", go_compile_duration,
		"wave_build_duration", total_duration-hook_duration-go_compile_duration,
	)

	return nil
}

func (c *Config) buildCSS() error {
	err := c.processCSSCritical()
	if err != nil {
		return fmt.Errorf("error processing critical CSS: %w", err)
	}

	err = c.processCSSNormal()
	if err != nil {
		return fmt.Errorf("error processing normal CSS: %w", err)
	}

	return nil
}

type esbuildCtxSafe struct {
	ctx esbuild.BuildContext
	mu  sync.Mutex
}

var (
	cssImportURLsMu         *sync.RWMutex  = &sync.RWMutex{}
	criticalReliedUponFiles                = map[string]struct{}{}
	normalReliedUponFiles                  = map[string]struct{}{}
	esbuildCtxCritical      esbuildCtxSafe = esbuildCtxSafe{}
	esbuildCtxNormal        esbuildCtxSafe = esbuildCtxSafe{}
)

func (c *Config) processCSSCritical() error { return c.__processCSS("critical") }
func (c *Config) processCSSNormal() error   { return c.__processCSS("normal") }

// nature = "critical" or "normal"
func (c *Config) __processCSS(nature string) error {
	entryPoint := c.cleanSources.NonCriticalCSSEntry
	if nature == "critical" {
		entryPoint = c.cleanSources.CriticalCSSEntry
	}

	if entryPoint == "" {
		return nil
	}

	isDev := GetIsDev()

	ctx, ctxErr := esbuild.Context(esbuild.BuildOptions{
		EntryPoints:       []string{entryPoint},
		Bundle:            true,
		MinifyWhitespace:  !isDev,
		MinifyIdentifiers: !isDev,
		MinifySyntax:      !isDev,
		Write:             false,
		Metafile:          true,
		Plugins: []esbuild.Plugin{
			{
				Name: "url-resolver",
				Setup: func(build esbuild.PluginBuild) {
					build.OnResolve(esbuild.OnResolveOptions{Filter: ".*", Namespace: "file"},
						func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
							if args.Kind == esbuild.ResolveCSSURLToken {
								u, err := url.Parse(args.Path)
								if err == nil && u.Scheme != "" {
									// It's a valid URL with a scheme (http, https, data, etc.)
									return esbuild.OnResolveResult{}, nil
								}

								// Check for protocol-relative URLs
								if strings.HasPrefix(args.Path, "//") {
									return esbuild.OnResolveResult{}, nil
								}

								return esbuild.OnResolveResult{
									Path:     c.MustGetPublicURLBuildtime(args.Path),
									External: true,
								}, nil
							}
							return esbuild.OnResolveResult{}, nil
						},
					)
				},
			},
		},
	})
	if ctxErr != nil {
		return fmt.Errorf("error creating esbuild context: %v", ctxErr.Errors)
	}

	if nature == "critical" {
		esbuildCtxCritical.mu.Lock()
		esbuildCtxCritical.ctx = ctx
		esbuildCtxCritical.mu.Unlock()
	} else {
		esbuildCtxNormal.mu.Lock()
		esbuildCtxNormal.ctx = ctx
		esbuildCtxNormal.mu.Unlock()
	}

	result := ctx.Rebuild()
	if err := esbuildutil.CollectErrors(result); err != nil {
		return fmt.Errorf("error building CSS: %w", err)
	}

	var metafile esbuildutil.ESBuildMetafileSubset
	if err := json.Unmarshal([]byte(result.Metafile), &metafile); err != nil {
		return fmt.Errorf("error unmarshalling esbuild metafile: %w", err)
	}

	srcURL := c.cleanSources.NonCriticalCSSEntry
	if nature == "critical" {
		srcURL = c.cleanSources.CriticalCSSEntry
	}

	imports := metafile.Inputs[srcURL].Imports

	cssImportURLsMu.Lock()

	if nature == "critical" {
		criticalReliedUponFiles = map[string]struct{}{}
	} else {
		normalReliedUponFiles = map[string]struct{}{}
	}

	for _, imp := range imports {
		if imp.Kind != "import-rule" {
			continue
		}

		if nature == "critical" {
			criticalReliedUponFiles[imp.Path] = struct{}{}
		} else {
			normalReliedUponFiles[imp.Path] = struct{}{}
		}
	}

	cssImportURLsMu.Unlock()

	// Determine output path and filename
	var outputPath string

	switch nature {
	case "critical":
		outputPath = c._dist.S().Static.S().Internal.FullPath()
	case "normal":
		outputPath = c._dist.S().Static.S().Assets.S().Public.FullPath()
	}

	outputFileName := nature + ".css" // Default for 'critical'

	if nature == "normal" {
		// first, delete the old normal.css file(s)
		oldNormalPath := filepath.Join(outputPath, "vorma_out_vorma_internal_normal_*.css")
		oldNormalFiles, err := filepath.Glob(oldNormalPath)
		if err != nil {
			return fmt.Errorf("error finding old normal CSS files: %w", err)
		}
		for _, oldNormalFile := range oldNormalFiles {
			if err := os.Remove(oldNormalFile); err != nil {
				return fmt.Errorf("error removing old normal CSS file: %w", err)
			}
		}

		// Hash the css output
		outputFileName = getHashedFilename(
			result.OutputFiles[0].Contents,
			"vorma_internal_normal.css",
		)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	// Write css to file
	outputFile := filepath.Join(outputPath, outputFileName)

	// If normal, also write to a file called normal_css_ref.txt with the hash
	if nature == "normal" {
		hashFile := c._dist.S().Static.S().Internal.S().NormalCSSFileRefDotTXT.FullPath()
		if err := os.WriteFile(hashFile, []byte(outputFileName), 0644); err != nil {
			return fmt.Errorf("error writing to file: %w", err)
		}
	}

	return os.WriteFile(outputFile, result.OutputFiles[0].Contents, 0644)
}

type staticFileProcessorOpts struct {
	basename       string
	srcDir         string
	distDir        string
	mapName        string
	is_dev_rebuild bool
	getIsNoHashDir func(string) (bool, uint8)
	writeWithHash  bool
}

func (c *Config) handlePublicFiles(isDevRebuild bool) error {
	return c.processStaticFiles(&staticFileProcessorOpts{
		basename:       PUBLIC,
		srcDir:         c.cleanSources.PublicStatic,
		distDir:        c._dist.S().Static.S().Assets.S().Public.FullPath(),
		mapName:        PublicFileMapGobName,
		is_dev_rebuild: isDevRebuild,
		getIsNoHashDir: func(path string) (bool, uint8) {
			if strings.HasPrefix(path, noHashPublicDirsByVersion[1]) {
				return true, 1
			}
			if strings.HasPrefix(path, noHashPublicDirsByVersion[0]) {
				return true, 0
			}
			return false, 0
		},
		writeWithHash: true,
	})
}

func (c *Config) copyPrivateFiles(is_dev_rebuild bool) error {
	return c.processStaticFiles(&staticFileProcessorOpts{
		basename:       PRIVATE,
		srcDir:         c.cleanSources.PrivateStatic,
		distDir:        c._dist.S().Static.S().Assets.S().Private.FullPath(),
		mapName:        PrivateFileMapGobName,
		is_dev_rebuild: is_dev_rebuild,
		getIsNoHashDir: func(path string) (bool, uint8) {
			return false, 0
		},
		writeWithHash: false,
	})
}

type fileInfo struct {
	path         string
	relativePath string
	isNoHashDir  bool
}

// __TODO this should probably be a config option and use glob patterns
var STATIC_FILENAMES_IGNORE_LIST = map[string]struct{}{
	".DS_Store": {},
}

func (c *Config) processStaticFiles(opts *staticFileProcessorOpts) error {
	if _, err := os.Stat(opts.srcDir); os.IsNotExist(err) {
		// If source dir doesn't exist, just save empty maps and return.
		err := c.saveMapToGob(map[string]fileVal{}, opts.mapName)
		if err != nil {
			return fmt.Errorf("error saving empty file map: %w", err)
		}
		if opts.basename == PUBLIC {
			err = c.savePublicFileMapJSToInternalPublicDir(map[string]fileVal{})
			if err != nil {
				return fmt.Errorf("error saving empty public file map JSON: %w", err)
			}
		}
		return nil
	}

	newFileMap := typed.SyncMap[string, fileVal]{}
	oldFileMap := typed.SyncMap[string, fileVal]{}

	// Load old file map if granular updates are enabled
	if opts.is_dev_rebuild {
		var err error
		oldMap, err := c.loadMapFromGob(opts.mapName, true)
		if err != nil {
			return fmt.Errorf("error reading old file map: %w", err)
		}
		for k, v := range oldMap {
			oldFileMap.Store(k, v)
		}
	}

	fileChan := make(chan fileInfo, 100)
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	// File discovery goroutine
	go func() {
		defer close(fileChan)
		err := filepath.WalkDir(opts.srcDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				relativePath, err := filepath.Rel(opts.srcDir, path)
				if err != nil {
					return err
				}
				relativePath = filepath.ToSlash(relativePath)
				isNoHashDir, version := opts.getIsNoHashDir(relativePath)
				if isNoHashDir {
					relativePath = strings.TrimPrefix(relativePath, noHashPublicDirsByVersion[version]+"/")
				}
				if _, isIgnore := STATIC_FILENAMES_IGNORE_LIST[filepath.Base(relativePath)]; isIgnore {
					return nil
				}
				fileChan <- fileInfo{path: path, relativePath: relativePath, isNoHashDir: isNoHashDir}
			}
			return nil
		})
		if err != nil {
			errChan <- err
		}
	}()

	// File processing goroutines
	workerCount := 4
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fi := range fileChan {
				if err := c.processFile(fi, opts, &newFileMap, &oldFileMap, opts.distDir); err != nil {
					errChan <- err
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	if err := <-errChan; err != nil {
		return err
	}

	// Cleanup old moot files if granular updates are enabled
	if opts.is_dev_rebuild {
		var oldMapErr error
		oldFileMap.Range(func(k string, v fileVal) bool {
			newFile, exists := newFileMap.Load(k)

			// Only delete if:
			// 1. File was deleted from source (!exists), OR
			// 2. File was renamed (different DistName)
			shouldDelete := !exists || (exists && newFile.DistName != v.DistName)

			if shouldDelete {
				oldDistPath := filepath.Join(opts.distDir, v.DistName)
				err := os.Remove(oldDistPath)
				if err != nil && !os.IsNotExist(err) {
					oldMapErr = fmt.Errorf(
						"error removing old static file from dist (%s/%v): %v", opts.basename, v, err,
					)
					return false
				}
			}
			return true
		})
		if oldMapErr != nil {
			return oldMapErr
		}
	}

	// Save the updated file map
	err := c.saveMapToGob(to_std_map(&newFileMap), opts.mapName)
	if err != nil {
		return fmt.Errorf("error saving file map: %w", err)
	}

	if opts.basename == PUBLIC {
		err = c.savePublicFileMapJSToInternalPublicDir(to_std_map(&newFileMap))
		if err != nil {
			return fmt.Errorf("error saving public file map JSON: %w", err)
		}
	}

	return nil
}

func (c *Config) processFile(
	fi fileInfo,
	opts *staticFileProcessorOpts,
	newFileMap,
	oldFileMap *typed.SyncMap[string, fileVal],
	distDir string,
) error {
	if err := c.fileSemaphore.Acquire(context.Background(), 1); err != nil {
		return fmt.Errorf("error acquiring semaphore: %w", err)
	}
	defer c.fileSemaphore.Release(1)

	relativePathUnderscores := strings.ReplaceAll(fi.relativePath, "/", "_")

	contentHash, err := getHashedFilenameFromPath(fi.path, relativePathUnderscores)
	if err != nil {
		return fmt.Errorf("error getting hashed filename: %w", err)
	}

	var fileIdentifier fileVal
	fileIdentifier.ContentHash = contentHash

	if fi.isNoHashDir {
		fileIdentifier.DistName = fi.relativePath
		fileIdentifier.IsPrehashed = true
	} else if !opts.writeWithHash {
		// For private files, the on-disk name is the original relative path
		fileIdentifier.DistName = fi.relativePath
	} else {
		// For public files, the on-disk name is the hashed name
		fileIdentifier.DistName = contentHash
	}

	newFileMap.Store(fi.relativePath, fileIdentifier)

	// Skip unchanged files if granular updates are enabled
	if opts.is_dev_rebuild {
		if oldVal, exists := oldFileMap.Load(fi.relativePath); exists {
			if oldVal.ContentHash == fileIdentifier.ContentHash {
				return nil
			}
		}
	}

	var distPath string
	if opts.writeWithHash {
		distPath = filepath.Join(distDir, fileIdentifier.DistName)
	} else {
		distPath = filepath.Join(distDir, fi.relativePath)
	}

	err = os.MkdirAll(filepath.Dir(distPath), 0755)
	if err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	err = fsutil.CopyFile(fi.path, distPath)
	if err != nil {
		return fmt.Errorf("error copying file: %w", err)
	}

	return nil
}

func to_std_map(sm *typed.SyncMap[string, fileVal]) map[string]fileVal {
	m := make(map[string]fileVal)
	sm.Range(func(k string, v fileVal) bool {
		m[k] = v
		return true
	})
	return m
}
