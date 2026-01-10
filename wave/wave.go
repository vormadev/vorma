package wave

import (
	"flag"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/middleware"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave/internal/builder"
	"github.com/vormadev/vorma/wave/internal/config"
	"github.com/vormadev/vorma/wave/internal/devserver"
	"github.com/vormadev/vorma/wave/internal/runtime"
)

type (
	Wave struct {
		cfg    *config.Config
		rawCfg []byte
		rt     *runtime.Runtime
		log    *slog.Logger
	}
	FileMap          = config.FileMap
	WatchedFile      = config.WatchedFile
	OnChangeHook     = config.OnChangeHook
	OnChangeStrategy = config.OnChangeStrategy
)

const (
	OnChangeStrategyPre              = config.TimingPre
	OnChangeStrategyConcurrent       = config.TimingConcurrent
	OnChangeStrategyConcurrentNoWait = config.TimingConcurrentNoWait
	OnChangeStrategyPost             = config.TimingPost
	PrehashedDirname                 = config.PrehashedDirname
	// Expose constant so frameworks can reference the expected filename
	PublicFileMapTSName = config.FilePublicMapTS

	// Fallback action constants for OnChangeStrategy
	FallbackRestart     = config.FallbackRestart
	FallbackRestartNoGo = config.FallbackRestartNoGo
	FallbackNone        = config.FallbackNone
)

var (
	MustGetPort  = config.MustGetAppPort
	GetIsDev     = config.GetIsDev
	SetModeToDev = config.SetModeToDev
)

// Also add top-level funcs to Wave struct for convenience.
func (w *Wave) GetIsDev() bool   { return GetIsDev() }
func (w *Wave) MustGetPort() int { return MustGetPort() }
func (w *Wave) SetModeToDev()    { SetModeToDev() }

type Config struct {
	// Required -- the bytes of your wave.config.json file.
	// You can use go:embed or just read the file in yourself.
	// Using go:embed is recommended for simpler deployments and improved performance.
	WaveConfigJSON []byte

	// Required -- be sure to pass in a file system that has your
	// <distDir>/static directory as its ROOT.
	// If you are using an embedded filesystem, you may need to use fs.Sub to get the
	// correct subdirectory.
	// Using go:embed is recommended for simpler deployments and improved performance.
	DistStaticFS fs.FS

	// Optional -- a logger instance.
	// If not provided, a default logger will be created that writes to standard out.
	Logger *slog.Logger
}

func New(c Config) *Wave {
	if c.WaveConfigJSON == nil {
		panic("wave.New: WaveConfigJSON cannot be nil")
	}

	cfg, err := config.Parse(c.WaveConfigJSON)
	if err != nil {
		panic("wave.New: " + err.Error())
	}

	log := c.Logger
	if log == nil {
		log = colorlog.New("wave")
	}

	rt := runtime.New(cfg, c.DistStaticFS, log)

	return &Wave{cfg: cfg, rawCfg: c.WaveConfigJSON, rt: rt, log: log}
}

// RawConfigJSON returns the raw bytes of the configuration file.
// Frameworks can use this to parse their own sections.
func (w *Wave) RawConfigJSON() []byte {
	return w.rawCfg
}

// AddFrameworkWatchPatterns adds watch patterns for use during development.
// This is intended for frameworks (like Vorma) to inject their own file watching
// behavior without Wave needing framework-specific knowledge.
//
// These patterns are only used during dev mode (wave dev) and are ignored in production.
// Call this method before calling MustStartDev().
func (w *Wave) AddFrameworkWatchPatterns(patterns []WatchedFile) {
	w.cfg.FrameworkWatchPatterns = append(w.cfg.FrameworkWatchPatterns, patterns...)
}

// AddIgnoredPatterns adds glob patterns for files/directories to ignore during watching.
// This is useful for frameworks to ignore their generated files to prevent infinite rebuild loops.
func (w *Wave) AddIgnoredPatterns(patterns []string) {
	w.cfg.FrameworkIgnoredPatterns = append(w.cfg.FrameworkIgnoredPatterns, patterns...)
}

// SetPublicFileMapOutDir sets the directory where Wave should write the public filemap TypeScript file.
// If set, Wave will automatically regenerate this file when public assets change.
func (w *Wave) SetPublicFileMapOutDir(dir string) {
	w.cfg.FrameworkPublicFileMapOutDir = dir
}

// RegisterConfigSchemaSection adds a custom section to the generated JSON schema.
// This allows frameworks to extend wave.config.json with their own configuration
// while maintaining IDE autocomplete support.
func (w *Wave) RegisterConfigSchemaSection(name string, schema jsonschema.Entry) {
	if w.cfg.FrameworkSchemaExtensions == nil {
		w.cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	w.cfg.FrameworkSchemaExtensions[name] = schema
}

// If you want to do a custom build command, just use
// Wave.BuildWaveWithoutCompilingGo() instead of Wave.BuildWave(),
// and then you can control your build yourself afterwards.
func (w *Wave) BuildWave() error {
	b := builder.New(w.cfg, w.log)
	defer b.Close()
	return b.Build(builder.Opts{CompileGo: true})
}
func (w *Wave) BuildWaveWithoutCompilingGo() error {
	b := builder.New(w.cfg, w.log)
	defer b.Close()
	return b.Build(builder.Opts{CompileGo: false})
}

func (w *Wave) GetPublicFS() (fs.FS, error)  { return w.rt.PublicFS() }
func (w *Wave) GetPrivateFS() (fs.FS, error) { return w.rt.PrivateFS() }
func (w *Wave) MustGetPublicFS() fs.FS {
	f, err := w.rt.PublicFS()
	if err != nil {
		panic(err)
	}
	return f
}
func (w *Wave) MustGetPrivateFS() fs.FS {
	f, err := w.rt.PrivateFS()
	if err != nil {
		panic(err)
	}
	return f
}
func (w *Wave) GetPublicURL(original string) string {
	return w.rt.PublicURL(original)
}
func (w *Wave) MustGetPublicURLBuildtime(original string) string {
	b := builder.New(w.cfg, w.log)
	defer b.Close()
	return b.MustGetPublicURLBuildtime(original)
}
func (w *Wave) MustStartDev() {
	if err := devserver.Run(w.cfg, w.log); err != nil {
		panic(err)
	}
}
func (w *Wave) GetCriticalCSS() template.CSS {
	return template.CSS(w.rt.CriticalCSS())
}
func (w *Wave) GetStyleSheetURL() string {
	return w.rt.StyleSheetURL()
}
func (w *Wave) GetRefreshScript() template.HTML {
	return w.rt.RefreshScript()
}
func (w *Wave) GetRefreshScriptSha256Hash() string {
	return w.rt.RefreshScriptHash()
}
func (w *Wave) GetCriticalCSSElementID() string {
	return runtime.CriticalCSSElementID
}
func (w *Wave) GetStyleSheetElementID() string {
	return runtime.StyleSheetElementID
}
func (w *Wave) GetBaseFS() (fs.FS, error) {
	return w.rt.BaseFS()
}
func (w *Wave) GetCriticalCSSStyleElement() template.HTML {
	return w.rt.CriticalCSSStyleElement()
}
func (w *Wave) GetCriticalCSSStyleElementSha256Hash() string {
	return w.rt.CriticalCSSStyleElementHash()
}
func (w *Wave) GetStyleSheetLinkElement() template.HTML {
	return w.rt.StyleSheetLinkElement()
}
func (w *Wave) GetServeStaticHandler(immutable bool) (http.Handler, error) {
	return w.rt.StaticHandler(immutable)
}
func (w *Wave) MustGetServeStaticHandler(immutable bool) http.Handler {
	h, err := w.rt.StaticHandler(immutable)
	if err != nil {
		panic(err)
	}
	return h
}

func (w *Wave) ServeStatic(immutable bool) func(http.Handler) http.Handler {
	return w.rt.StaticMiddleware(immutable)
}
func (w *Wave) GetPublicFileMap() (FileMap, error) {
	return w.rt.PublicFileMap()
}
func (w *Wave) GetPublicFileMapKeysBuildtime() ([]string, error) {
	b := builder.New(w.cfg, w.log)
	defer b.Close()
	return b.PublicFileMapKeys()
}
func (w *Wave) GetPublicFileMapElements() template.HTML {
	return w.rt.PublicFileMapElements()
}
func (w *Wave) GetPublicFileMapScriptSha256Hash() string {
	return w.rt.PublicFileMapScriptHash()
}
func (w *Wave) GetPublicFileMapURL() string {
	return w.rt.PublicFileMapURL()
}
func (w *Wave) SetupDistDir() {
	if err := builder.SetupDistDir(w.cfg); err != nil {
		panic(err)
	}
}
func (w *Wave) GetSimplePublicFileMapBuildtime() (map[string]string, error) {
	b := builder.New(w.cfg, w.log)
	defer b.Close()
	return b.SimplePublicFileMap()
}
func (w *Wave) GetPrivateStaticDir() string {
	return w.cfg.Core.StaticAssetDirs.Private
}
func (w *Wave) GetPublicStaticDir() string {
	return w.cfg.Core.StaticAssetDirs.Public
}
func (w *Wave) GetPublicPathPrefix() string {
	return w.cfg.PublicPathPrefix()
}
func (w *Wave) ViteProdBuildWave() error {
	b := builder.New(w.cfg, w.log)
	defer b.Close()
	return b.ViteProdBuild()
}
func (w *Wave) GetViteManifestLocation() string {
	return w.cfg.ViteManifestPath()
}
func (w *Wave) GetViteOutDir() string {
	return w.cfg.Dist.StaticPublic()
}

// BuildWaveWithHook provides CLI integration for build commands with a custom hook.
// It parses -dev, -hook, and -no-binary flags and orchestrates the build workflow.
func (w *Wave) BuildWaveWithHook(hook func(isDev bool) error) {
	devModeFlag := flag.Bool("dev", false, "set dev mode")
	hookModeFlag := flag.Bool("hook", false, "set hook mode")
	noBinaryFlag := flag.Bool("no-binary", false, "skip go binary compilation")

	flag.Parse()

	isDev := *devModeFlag
	isHook := *hookModeFlag
	noBinary := *noBinaryFlag

	if isHook {
		if err := hook(isDev); err != nil {
			panic(err)
		}
		return
	}

	if isDev {
		config.SetModeToDev()
		if err := devserver.Run(w.cfg, w.log); err != nil {
			panic(err)
		}
		return
	}

	b := builder.New(w.cfg, w.log)
	defer b.Close()
	if err := b.Build(builder.Opts{CompileGo: !noBinary}); err != nil {
		panic(err)
	}
}

func (w *Wave) GetConfigFile() string {
	return w.cfg.Core.ConfigLocation
}
func (w *Wave) GetDistDir() string {
	return w.cfg.Core.DistDir
}
func (w *Wave) GetStaticPrivateOutDir() string {
	return w.cfg.Dist.StaticPrivate()
}
func (w *Wave) GetStaticPublicOutDir() string {
	return w.cfg.Dist.StaticPublic()
}

// WritePublicFileMapTS writes the public file map as a TypeScript file.
// This is useful for build-time generation when the client needs the map.
func (w *Wave) WritePublicFileMapTS(outDir string) error {
	if outDir == "" {
		return nil
	}
	b := builder.New(w.cfg, w.log)
	defer b.Close()
	return b.WritePublicFileMapTS(outDir)
}

// Forwards requests for "/favicon.ico" to "/{your-public-prefix}/favicon.ico".
// Not necessary if you're explicitly defining your favicon anywhere.
// Only comes into play if your preference is to drop a "favicon.ico" file into
// your public static directory and call it a day.
func (w *Wave) FaviconRedirect() middleware.Middleware {
	return w.rt.FaviconRedirect()
}
