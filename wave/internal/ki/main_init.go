package ki

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/safecache"
	"golang.org/x/sync/semaphore"
)

const __internal_full_dev_reset_less_go_mrkr = "__internal_full_dev_reset_less_go_mrkr"

/////////////////////////////////////////////////////////////////////
/////// MAIN INIT
/////////////////////////////////////////////////////////////////////

type MainInitOptions struct {
	IsDev     bool
	IsRebuild bool
}

func (c *Config) MainInit(opts MainInitOptions, calledFrom string) {
	if opts.IsDev {
		SetModeToDev()
	}

	// LOGGER
	if c.Logger == nil {
		c.Logger = colorlog.New("wave")
	}

	if opts.IsDev && opts.IsRebuild {
		configFileLocation := c.GetConfigFile()
		if configFileLocation != "" {
			c.Logger.Info("Reloading config from disk", "path", configFileLocation)
			newConfigBytes, err := os.ReadFile(configFileLocation)
			if err != nil {
				c.panic("failed to re-read config file on rebuild", err)
			}
			c.WaveConfigJSON = newConfigBytes
		}
	}

	c.fileSemaphore = semaphore.NewWeighted(100)

	if len(c.WaveConfigJSON) == 0 {
		c.panic("Config Error: ConfigBytes cannot be nil or empty. A valid wave.config.json must be provided.", nil)
	}

	// USER CONFIG
	c._uc = new(UserConfig)
	if err := json.Unmarshal(c.WaveConfigJSON, c._uc); err != nil {
		c.panic("failed to unmarshal user config", err)
	}

	c.validateUserConfig()

	// CLEAN SOURCES
	c.cleanSources = CleanSources{
		Dist:          filepath.Clean(c._uc.Core.DistDir),
		PrivateStatic: filepath.Clean(c._uc.Core.StaticAssetDirs.Private),
		PublicStatic:  filepath.Clean(c._uc.Core.StaticAssetDirs.Public),
	}
	if c._uc.Core.CSSEntryFiles.Critical != "" {
		c.cleanSources.CriticalCSSEntry = filepath.Clean(c._uc.Core.CSSEntryFiles.Critical)
	}
	if c._uc.Core.CSSEntryFiles.NonCritical != "" {
		c.cleanSources.NonCriticalCSSEntry = filepath.Clean(c._uc.Core.CSSEntryFiles.NonCritical)
	}

	// DIST LAYOUT
	c._dist = toDistLayout(c.cleanSources.Dist)

	c.InitRuntimeCache()

	// AFTER HERE, ALL DEV-TIME STUFF
	if !opts.IsDev {
		return
	}

	c.dev.mu.Lock()
	defer c.dev.mu.Unlock()

	c.kill_browser_refresh_mux()

	c._rebuild_cleanup_chan = make(chan struct{})

	c.cleanWatchRoot = filepath.Clean(c._uc.Watch.WatchRoot)

	// HEALTH CHECK ENDPOINT
	if c._uc.Watch.HealthcheckEndpoint == "" {
		c._uc.Watch.HealthcheckEndpoint = "/"
	}

	if !opts.IsRebuild {
		c.browserTabManager = newClientManager()
		go c.browserTabManager.start()
	}

	c.ignoredFilePatterns = []string{
		c.get_binary_output_path(),
	}

	c.naiveIgnoreDirPatterns = []string{
		"**/.git",
		"**/node_modules",
		c._dist.S().Static.FullPath(),
		filepath.Join(c.cleanSources.PublicStatic, noHashPublicDirsByVersion[0]),
		filepath.Join(c.cleanSources.PublicStatic, noHashPublicDirsByVersion[1]),
	}

	for _, p := range c.naiveIgnoreDirPatterns {
		c.ignoredDirPatterns = append(c.ignoredDirPatterns, filepath.Join(c.cleanWatchRoot, p))
	}
	for _, p := range c._uc.Watch.Exclude.Dirs {
		c.ignoredDirPatterns = append(c.ignoredDirPatterns, filepath.Join(c.cleanWatchRoot, p))
	}
	for _, p := range c._uc.Watch.Exclude.Files {
		c.ignoredFilePatterns = append(c.ignoredFilePatterns, filepath.Join(c.cleanWatchRoot, p))
	}

	c.defaultWatchedFiles = []WatchedFile{
		{
			Pattern:       filepath.Join(c.cleanSources.PublicStatic, "**/*"),
			OnChangeHooks: []OnChangeHook{{Cmd: __internal_full_dev_reset_less_go_mrkr}},
		},
	}

	includeDefaults := c._uc.Vorma != nil
	if c._uc.Vorma != nil && c._uc.Vorma.IncludeDefaults != nil && !*c._uc.Vorma.IncludeDefaults {
		includeDefaults = false
	}

	if includeDefaults {
		relClientRouteDefsFile, err := filepath.Rel(c.cleanWatchRoot, c._uc.Vorma.ClientRouteDefsFile)
		if err != nil {
			c.panic("failed to get relative path for ClientRouteDefsFile", err)
		}

		c.defaultWatchedFiles = append(c.defaultWatchedFiles, WatchedFile{
			Pattern:       filepath.Join(c.cleanSources.PrivateStatic, c._uc.Vorma.HTMLTemplateLocation),
			OnChangeHooks: []OnChangeHook{{Cmd: __internal_full_dev_reset_less_go_mrkr}},
		})

		c.defaultWatchedFiles = append(c.defaultWatchedFiles, WatchedFile{
			Pattern:       filepath.ToSlash(relClientRouteDefsFile),
			OnChangeHooks: []OnChangeHook{{Cmd: __internal_full_dev_reset_less_go_mrkr}},
		})

		c.defaultWatchedFiles = append(c.defaultWatchedFiles, WatchedFile{
			Pattern:       "**/*.go",
			OnChangeHooks: []OnChangeHook{{Cmd: "DevBuildHook", Timing: "concurrent"}},
		})

		relHTMLTemplateLocation, err := filepath.Rel(c.cleanWatchRoot, c._uc.Vorma.HTMLTemplateLocation)
		if err != nil {
			c.panic("failed to get relative path for HTMLTemplateLocation", err)
		}

		c.defaultWatchedFiles = append(c.defaultWatchedFiles, WatchedFile{
			Pattern:    filepath.ToSlash(relHTMLTemplateLocation),
			RestartApp: true,
		})

		relTSGenOutPath, err := filepath.Rel(c.cleanWatchRoot, c._uc.Vorma.TSGenOutPath)
		if err != nil {
			c.panic("failed to get relative path for TSGenOutPath", err)
		}

		c.ignoredFilePatterns = append(
			c.ignoredFilePatterns,
			filepath.ToSlash(relTSGenOutPath),
		)
	}

	// Loop through all WatchedFiles...
	for i, wfc := range c._uc.Watch.Include {
		// and make each WatchedFile's Pattern relative to cleanWatchRoot...
		c._uc.Watch.Include[i].Pattern = filepath.Join(c.cleanWatchRoot, wfc.Pattern)
		// then loop through such WatchedFile's OnChangeHooks...
		for j, oc := range wfc.OnChangeHooks {
			// and make each such OnChangeCallback's ExcludedPatterns also relative to cleanWatchRoot
			for k, p := range oc.Exclude {
				c._uc.Watch.Include[i].OnChangeHooks[j].Exclude[k] = filepath.Join(c.cleanWatchRoot, p)
			}
		}
	}

	c.matchResults = safecache.NewMap(c.get_initial_match_results, c.match_results_key_maker, nil)

	if c.watcher != nil {
		if err := c.watcher.Close(); err != nil {
			c.panic("failed to close watcher", err)
		}
		c.watcher = nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		c.panic("failed to create watcher", err)
	}

	c.watcher = watcher

	if err := c.add_directory_to_watcher(c.cleanWatchRoot); err != nil {
		c.panic("failed to add directory to watcher", err)
	}
}

var ErrConfigValidation = errors.New("config validation error")

func (c *Config) validateUserConfig() {
	// Validate top-level required fields in [Core].
	if c._uc.Core.MainAppEntry == "" {
		c.panic("Config Error: Core.MainAppEntry is required and cannot be an empty string.", ErrConfigValidation)
	}
	if c._uc.Core.DistDir == "" {
		c.panic("Config Error: Core.DistDir is required and cannot be an empty string.", ErrConfigValidation)
	}

	// Validate conditionally required fields.
	if !c._uc.Core.ServerOnlyMode {
		if c._uc.Core.StaticAssetDirs.Private == "" {
			c.panic("Config Error: Core.StaticAssetDirs.Private is required and cannot be empty when not in ServerOnlyMode.", ErrConfigValidation)
		}
		if c._uc.Core.StaticAssetDirs.Public == "" {
			c.panic("Config Error: Core.StaticAssetDirs.Public is required and cannot be empty when not in ServerOnlyMode.", ErrConfigValidation)
		}
	}

	// Validate required fields within optional blocks.
	if c._uc.Vorma != nil {
		if c._uc.Vorma.UIVariant == "" {
			c.panic("Config Error: Vorma.UIVariant is required when the [Vorma] block is present.", ErrConfigValidation)
		}
		if c._uc.Vorma.HTMLTemplateLocation == "" {
			c.panic("Config Error: Vorma.HTMLTemplateLocation is required when the [Vorma] block is present.", ErrConfigValidation)
		}
		if c._uc.Vorma.ClientEntry == "" {
			c.panic("Config Error: Vorma.ClientEntry is required when the [Vorma] block is present.", ErrConfigValidation)
		}
		if c._uc.Vorma.ClientRouteDefsFile == "" {
			c.panic("Config Error: Vorma.ClientRouteDefsFile is required when the [Vorma] block is present.", ErrConfigValidation)
		}
		if c._uc.Vorma.TSGenOutPath == "" {
			c.panic("Config Error: Vorma.TSGenOutPath is required when the [Vorma] block is present.", ErrConfigValidation)
		}
	}

	if c._uc.Vite != nil {
		if c._uc.Vite.JSPackageManagerBaseCmd == "" {
			c.panic("Config Error: Vite.JSPackageManagerBaseCmd is required when the [Vite] block is present.", ErrConfigValidation)
		}
	}
}
