package ki

import (
	"html/template"
	"io/fs"
	"log/slog"
	"os/exec"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/vormadev/vorma/kit/dirs"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/safecache"
	"github.com/vormadev/vorma/kit/viteutil"
	"golang.org/x/sync/semaphore"
)

/////////////////////////////////////////////////////////////////////
/////// DEV CACHE
/////////////////////////////////////////////////////////////////////

type dev struct {
	mu sync.Mutex

	watcher                *fsnotify.Watcher
	lastBuildCmd           *exec.Cmd
	browserTabManager      *clientManager
	fileSemaphore          *semaphore.Weighted
	ignoredDirPatterns     []string
	ignoredFilePatterns    []string
	naiveIgnoreDirPatterns []string
	defaultWatchedFiles    []WatchedFile
	matchResults           *safecache.CacheMap[potentialMatch, string, bool]
	watchedDirs            sync.Map
}

/////////////////////////////////////////////////////////////////////
/////// RUNTIME CACHE
/////////////////////////////////////////////////////////////////////

type _runtime struct {
	runtime_cache runtimeCache
}

type runtimeCache struct {
	// FS
	base_fs     *safecache.Cache[fs.FS]
	base_dir_fs *safecache.Cache[fs.FS]
	public_fs   *safecache.Cache[fs.FS]
	private_fs  *safecache.Cache[fs.FS]

	// CSS
	stylesheet_link_el *safecache.Cache[*template.HTML]
	stylesheet_url     *safecache.Cache[string]
	critical_css       *safecache.Cache[*criticalCSSStatus]

	// Public URLs
	public_filemap_from_gob *safecache.Cache[FileMap]
	public_filemap_url      *safecache.Cache[string]
	public_filemap_details  *safecache.Cache[*publicFileMapDetails]
	public_urls             *safecache.CacheMap[string, string, string]
	is_public_asset         *safecache.CacheMap[string, string, bool]
}

func (c *Config) InitRuntimeCache() {
	c.runtime_cache = runtimeCache{
		// FS
		base_fs:     safecache.New(c.get_initial_base_fs, GetIsDev),
		base_dir_fs: safecache.New(c.get_initial_base_dir_fs, GetIsDev),
		public_fs:   safecache.New(func() (fs.FS, error) { return c.getSubFSPublic() }, GetIsDev),
		private_fs:  safecache.New(func() (fs.FS, error) { return c.getSubFSPrivate() }, GetIsDev),

		// CSS
		stylesheet_link_el: safecache.New(c.getInitialStyleSheetLinkElement, GetIsDev),
		stylesheet_url:     safecache.New(c.getInitialStyleSheetURL, GetIsDev),
		critical_css:       safecache.New(c.getInitialCriticalCSSStatus, GetIsDev),

		// Public URLs
		public_filemap_from_gob: safecache.New(c.getInitialPublicFileMapFromGobRuntime, GetIsDev),
		public_filemap_url:      safecache.New(c.getInitialPublicFileMapURL, GetIsDev),
		public_filemap_details:  safecache.New(c.getInitialPublicFileMapDetails, GetIsDev),
		public_urls: safecache.NewMap(c.getInitialPublicURL, publicURLsKeyMaker, func(string) bool {
			return GetIsDev()
		}),
		is_public_asset: safecache.NewMap(c.getInitialIsPublicAsset, publicURLsKeyMaker, func(string) bool {
			return GetIsDev()
		}),
	}
}

/////////////////////////////////////////////////////////////////////
/////// WAVE CONFIG
/////////////////////////////////////////////////////////////////////

type Config struct {
	// Required -- the bytes of your wave.config.json file. You can
	// use go:embed or just read the file in yourself. Using go:embed
	// is recommended for simpler deployments and improved performance.
	WaveConfigJSON []byte

	// Required -- be sure to pass in a file system that has your
	// <distDir>/static directory as its ROOT. If you are using an
	// embedded filesystem, you may need to use fs.Sub to get the
	// correct subdirectory. Using go:embed is recommended for simpler
	// deployments and improved performance.
	DistStaticFS fs.FS

	// Optional -- a Logger instance. If not provided, a default Logger
	// will be created that writes to standard out.
	Logger *slog.Logger

	dev
	_runtime
	cleanSources   CleanSources
	cleanWatchRoot string
	_dist          *dirs.Dir[Dist]
	_uc            *UserConfig

	_rebuild_cleanup_chan chan struct{}
	_vite_dev_ctx         *viteutil.BuildCtx
}

type CleanSources struct {
	Dist                string
	PrivateStatic       string
	PublicStatic        string
	CriticalCSSEntry    string
	NonCriticalCSSEntry string
}

func (c *Config) GetPrivateStaticDir() string {
	return c._uc.Core.StaticAssetDirs.Private
}
func (c *Config) GetPublicStaticDir() string {
	return c._uc.Core.StaticAssetDirs.Public
}
func (c *Config) GetDistDir() string {
	return c._uc.Core.DistDir
}
func (c *Config) GetPublicPathPrefix() string {
	p := c._uc.Core.PublicPathPrefix
	if p == "" || p == "/" {
		return "/"
	}
	return matcher.EnsureLeadingSlash(matcher.EnsureTrailingSlash(p))
}

/////////////////////////////////////////////////////////////////////
/////// USER CONFIG
/////////////////////////////////////////////////////////////////////

type Timing string

var TimingEnum = struct {
	Pre              Timing
	Post             Timing
	Concurrent       Timing
	ConcurrentNoWait Timing
}{
	Pre:              "pre",
	Post:             "post",
	Concurrent:       "concurrent",
	ConcurrentNoWait: "concurrent-no-wait",
}

type UserConfig struct {
	Core  *UserConfigCore
	Vorma *UserConfigVorma
	Vite  *UserConfigVite
	Watch *UserConfigWatch
}

type UserConfigCore struct {
	ConfigLocation   string
	DevBuildHook     string
	ProdBuildHook    string
	MainAppEntry     string
	DistDir          string
	StaticAssetDirs  StaticAssetDirs
	CSSEntryFiles    CSSEntryFiles
	PublicPathPrefix string
	ServerOnlyMode   bool
}

func (c *Config) GetConfigFile() string {
	return c._uc.Core.ConfigLocation
}

type StaticAssetDirs struct {
	Private string
	Public  string
}

type CSSEntryFiles struct {
	Critical    string
	NonCritical string
}

type UserConfigVite struct {
	JSPackageManagerBaseCmd string
	JSPackageManagerCmdDir  string
	DefaultPort             int
	ViteConfigFile          string
}

type UserConfigVorma struct {
	IncludeDefaults            *bool
	UIVariant                  string
	HTMLTemplateLocation       string // Relative to your static private dir
	ClientEntry                string
	ClientRouteDefsFile        string
	TSGenOutPath               string // e.g., "frontend/src/vorma.gen.ts"
	BuildtimePublicURLFuncName string // e.g., "waveURL", "withHash", etc.
}

func (c *Config) GetVormaUIVariant() string {
	return c._uc.Vorma.UIVariant
}
func (c *Config) GetVormaHTMLTemplateLocation() string {
	return c._uc.Vorma.HTMLTemplateLocation
}
func (c *Config) GetVormaClientEntry() string {
	return c._uc.Vorma.ClientEntry
}
func (c *Config) GetVormaClientRouteDefsFile() string {
	return c._uc.Vorma.ClientRouteDefsFile
}
func (c *Config) GetVormaTSGenOutPath() string {
	return c._uc.Vorma.TSGenOutPath
}
func (c *Config) GetVormaBuildtimePublicURLFuncName() string {
	funcName := c._uc.Vorma.BuildtimePublicURLFuncName
	if funcName == "" {
		funcName = "waveBuildtimeURL"
	}
	return funcName
}

type UserConfigWatch struct {
	WatchRoot           string
	HealthcheckEndpoint string
	Include             []WatchedFile
	Exclude             struct {
		Dirs  []string
		Files []string
	}
}

type OnChangeHook struct {
	Cmd     string
	Timing  Timing
	Exclude []string
}

type WatchedFile struct {
	Pattern                            string
	OnChangeHooks                      []OnChangeHook
	RecompileGoBinary                  bool
	RestartApp                         bool
	OnlyRunClientDefinedRevalidateFunc bool
	RunOnChangeOnly                    bool
	SkipRebuildingNotification         bool
	TreatAsNonGo                       bool
}
