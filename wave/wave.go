package wave

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/vormadev/vorma/kit/middleware"
	"github.com/vormadev/vorma/wave/internal/ki"
)

type (
	Wave        struct{ c *ki.Config }
	FileMap     = ki.FileMap
	WatchedFile = ki.WatchedFile
	OnChangeCmd = ki.OnChangeHook
)

const (
	OnChangeStrategyPre              = ki.OnChangeStrategyPre
	OnChangeStrategyConcurrent       = ki.OnChangeStrategyConcurrent
	OnChangeStrategyConcurrentNoWait = ki.OnChangeStrategyConcurrentNoWait
	OnChangeStrategyPost             = ki.OnChangeStrategyPost
	PrehashedDirname                 = ki.PrehashedDirname
)

var (
	MustGetPort  = ki.MustGetAppPort
	GetIsDev     = ki.GetIsDev
	SetModeToDev = ki.SetModeToDev
)

// Also add top-level funcs to Wave struct for convenience.
func (k Wave) GetIsDev() bool   { return GetIsDev() }
func (k Wave) MustGetPort() int { return MustGetPort() }
func (k Wave) SetModeToDev()    { SetModeToDev() }

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

	// Optional -- a logger instance. If not provided, a default logger
	// will be created that writes to standard out.
	Logger *slog.Logger
}

func New(config Config) *Wave {
	if config.WaveConfigJSON == nil {
		panic("wave.New: config.WaveConfigJSON cannot be nil")
	}
	cfg := &ki.Config{
		WaveConfigJSON: config.WaveConfigJSON,
		DistStaticFS:   config.DistStaticFS,
		Logger:         config.Logger,
	}
	cfg.MainInit(ki.MainInitOptions{}, "wave.New")
	return &Wave{cfg}
}

// If you want to do a custom build command, just use
// Wave.BuildWaveWithoutCompilingGo() instead of Wave.BuildWave(),
// and then you can control your build yourself afterwards.

func (k Wave) BuildWave() error {
	return k.c.BuildWave(ki.BuildOptions{RecompileGoBinary: true})
}
func (k Wave) BuildWaveWithoutCompilingGo() error {
	return k.c.BuildWave(ki.BuildOptions{})
}

func (k Wave) GetPublicFS() (fs.FS, error) {
	return k.c.GetPublicFS()
}
func (k Wave) GetPrivateFS() (fs.FS, error) {
	return k.c.GetPrivateFS()
}
func (k Wave) MustGetPublicFS() fs.FS {
	fs, err := k.c.GetPublicFS()
	if err != nil {
		panic(err)
	}
	return fs
}
func (k Wave) MustGetPrivateFS() fs.FS {
	fs, err := k.c.GetPrivateFS()
	if err != nil {
		panic(err)
	}
	return fs
}
func (k Wave) GetPublicURL(originalPublicURL string) string {
	return k.c.GetPublicURL(originalPublicURL)
}
func (k Wave) MustGetPublicURLBuildtime(originalPublicURL string) string {
	return k.c.MustGetPublicURLBuildtime(originalPublicURL)
}
func (k Wave) MustStartDev() {
	k.c.MustStartDev()
}
func (k Wave) GetCriticalCSS() template.CSS {
	return template.CSS(k.c.GetCriticalCSS())
}
func (k Wave) GetStyleSheetURL() string {
	return k.c.GetStyleSheetURL()
}
func (k Wave) GetRefreshScript() template.HTML {
	return template.HTML(k.c.GetRefreshScript())
}
func (k Wave) GetRefreshScriptSha256Hash() string {
	return k.c.GetRefreshScriptSha256Hash()
}
func (k Wave) GetCriticalCSSElementID() string {
	return ki.CriticalCSSElementID
}
func (k Wave) GetStyleSheetElementID() string {
	return ki.StyleSheetElementID
}
func (k Wave) GetBaseFS() (fs.FS, error) {
	return k.c.GetBaseFS()
}
func (k Wave) GetCriticalCSSStyleElement() template.HTML {
	return k.c.GetCriticalCSSStyleElement()
}
func (k Wave) GetCriticalCSSStyleElementSha256Hash() string {
	return k.c.GetCriticalCSSStyleElementSha256Hash()
}
func (k Wave) GetStyleSheetLinkElement() template.HTML {
	return k.c.GetStyleSheetLinkElement()
}
func (k Wave) GetServeStaticHandler(addImmutableCacheHeaders bool) (http.Handler, error) {
	return k.c.GetServeStaticHandler(addImmutableCacheHeaders)
}
func (k Wave) MustGetServeStaticHandler(addImmutableCacheHeaders bool) http.Handler {
	handler, err := k.c.GetServeStaticHandler(addImmutableCacheHeaders)
	if err != nil {
		panic(err)
	}
	return handler
}

func (k Wave) ServeStatic(addImmutableCacheHeaders bool) func(http.Handler) http.Handler {
	return k.c.ServeStaticPublicAssets(addImmutableCacheHeaders)
}
func (k Wave) GetPublicFileMap() (FileMap, error) {
	return k.c.GetPublicFileMap()
}
func (k Wave) GetPublicFileMapKeysBuildtime() ([]string, error) {
	return k.c.GetPublicFileMapKeysBuildtime()
}
func (k Wave) GetPublicFileMapElements() template.HTML {
	return k.c.GetPublicFileMapElements()
}
func (k Wave) GetPublicFileMapScriptSha256Hash() string {
	return k.c.GetPublicFileMapScriptSha256Hash()
}
func (k Wave) GetPublicFileMapURL() string {
	return k.c.GetPublicFileMapURL()
}
func (k Wave) SetupDistDir() {
	k.c.SetupDistDir()
}
func (k Wave) GetSimplePublicFileMapBuildtime() (map[string]string, error) {
	return k.c.GetSimplePublicFileMapBuildtime()
}
func (k Wave) GetPrivateStaticDir() string {
	return k.c.GetPrivateStaticDir()
}
func (k Wave) GetPublicStaticDir() string {
	return k.c.GetPublicStaticDir()
}
func (k Wave) GetPublicPathPrefix() string {
	return k.c.GetPublicPathPrefix()
}
func (k Wave) ViteProdBuildWave() error {
	return k.c.ViteProdBuildWave()
}
func (k Wave) GetViteManifestLocation() string {
	return k.c.GetViteManifestLocation()
}
func (k Wave) GetViteOutDir() string {
	return k.c.GetViteOutDir()
}
func (k Wave) BuildWaveWithHook(hook func(isDev bool) error) {
	k.c.BuildWaveWithHook(hook)
}
func (k Wave) GetVormaUIVariant() string {
	return k.c.GetVormaUIVariant()
}
func (k Wave) GetVormaHTMLTemplateLocation() string {
	return k.c.GetVormaHTMLTemplateLocation()
}
func (k Wave) GetVormaClientEntry() string {
	return k.c.GetVormaClientEntry()
}
func (k Wave) GetVormaClientRouteDefsFile() string {
	return k.c.GetVormaClientRouteDefsFile()
}
func (k Wave) GetVormaTSGenOutPath() string {
	return k.c.GetVormaTSGenOutPath()
}
func (k Wave) GetVormaBuildtimePublicURLFuncName() string {
	return k.c.GetVormaBuildtimePublicURLFuncName()
}
func (k Wave) GetConfigFile() string {
	return k.c.GetConfigFile()
}
func (k Wave) GetDistDir() string {
	return k.c.GetDistDir()
}
func (k Wave) GetStaticPrivateOutDir() string {
	return k.c.GetStaticPrivateOutDir()
}
func (k Wave) GetStaticPublicOutDir() string {
	return k.c.GetStaticPublicOutDir()
}

// Forwards requests for "/favicon.ico" to "/{your-public-prefix}/favicon.ico".
// Not necessary if you're explicitly defining your favicon anywhere.
// Only comes into play if your preference is to drop a "favicon.ico" file into
// your public static directory and call it a day.
func (k Wave) FaviconRedirect() middleware.Middleware {
	return k.c.FaviconRedirect()
}
