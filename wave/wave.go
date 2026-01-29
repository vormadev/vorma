package wave

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/middleware"
)

const (
	CriticalCSSElementID = "wave-critical-css"
	StyleSheetElementID  = "wave-normal-css"
)

// Wave provides runtime services for Wave applications.
type Wave struct {
	cfg    *ParsedConfig
	rawCfg []byte
	log    *slog.Logger

	distStaticFS fs.FS

	baseFS         *cache[fs.FS]
	publicFS       *cache[fs.FS]
	privateFS      *cache[fs.FS]
	fileMap        *cache[FileMap]
	criticalCSS    *cache[*criticalCSSData]
	stylesheetURL  *cache[string]
	stylesheetLink *cache[string]
	fileMapURL     *cache[string]
	fileMapDetails *cache[*fileMapDetails]
	publicURLs     *cacheMap[string, string]
	isAsset        *cacheMap[string, bool]
}

func (w *Wave) Logger() *slog.Logger { return w.log }

type criticalCSSData struct {
	content    string
	noSuchFile bool
	styleEl    template.HTML
	sha256Hash string
}

type fileMapDetails struct {
	elements   string
	sha256Hash string
}

// cache holds a lazily-initialized value that is cached in prod but
// recomputed on every access in dev mode.
type cache[T any] struct {
	val  T
	err  error
	once sync.Once
	fn   func() (T, error)
}

func newCache[T any](fn func() (T, error)) *cache[T] {
	return &cache[T]{fn: fn}
}

func (c *cache[T]) get() (T, error) {
	if GetIsDev() {
		return c.fn()
	}
	c.once.Do(func() { c.val, c.err = c.fn() })
	return c.val, c.err
}

// cacheMap holds lazily-initialized keyed values that are cached in prod
// but recomputed on every access in dev mode.
type cacheMap[K comparable, V any] struct {
	m  sync.Map
	fn func(K) (V, error)
}

func newCacheMap[K comparable, V any](fn func(K) (V, error)) *cacheMap[K, V] {
	return &cacheMap[K, V]{fn: fn}
}

func (c *cacheMap[K, V]) get(key K) (V, error) {
	if GetIsDev() {
		return c.fn(key)
	}
	if v, ok := c.m.Load(key); ok {
		return v.(V), nil
	}
	val, err := c.fn(key)
	if err == nil {
		c.m.Store(key, val)
	}
	return val, err
}

// Config configures Wave initialization.
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
	cfg, err := ParseConfig(c.WaveConfigJSON)
	if err != nil {
		panic("wave.New: " + err.Error())
	}
	log := c.Logger
	if log == nil {
		log = colorlog.New("wave")
	}

	w := &Wave{
		cfg:          cfg,
		rawCfg:       c.WaveConfigJSON,
		log:          log,
		distStaticFS: c.DistStaticFS,
	}

	w.baseFS = newCache(w.initBaseFS)
	w.publicFS = newCache(w.initPublicFS)
	w.privateFS = newCache(w.initPrivateFS)
	w.fileMap = newCache(w.initFileMap)
	w.criticalCSS = newCache(w.initCriticalCSS)
	w.stylesheetURL = newCache(w.initStylesheetURL)
	w.stylesheetLink = newCache(w.initStylesheetLink)
	w.fileMapURL = newCache(w.initFileMapURL)
	w.fileMapDetails = newCache(w.initFileMapDetails)
	w.publicURLs = newCacheMap(w.resolvePublicURL)
	w.isAsset = newCacheMap(w.checkIsAsset)

	return w
}

// RawConfigJSON returns the raw bytes of the configuration file.
func (w *Wave) RawConfigJSON() []byte {
	return w.rawCfg
}

// AddFrameworkWatchPatterns adds watch patterns for use during development.
func (w *Wave) AddFrameworkWatchPatterns(patterns []WatchedFile) {
	w.cfg.FrameworkWatchPatterns = append(w.cfg.FrameworkWatchPatterns, patterns...)
}

// AddIgnoredPatterns adds glob patterns for files/directories to ignore during watching.
func (w *Wave) AddIgnoredPatterns(patterns []string) {
	w.cfg.FrameworkIgnoredPatterns = append(w.cfg.FrameworkIgnoredPatterns, patterns...)
}

// SetPublicFileMapOutDir sets the directory where Wave should write the public filemap TypeScript file.
func (w *Wave) SetPublicFileMapOutDir(dir string) {
	w.cfg.FrameworkPublicFileMapOutDir = dir
}

// GetIsDev returns true if running in development mode.
func (w *Wave) GetIsDev() bool {
	return GetIsDev()
}

// MustGetPort returns the application port.
func (w *Wave) MustGetPort() int {
	return MustGetPort()
}

// SetModeToDev sets the environment to development mode.
func (w *Wave) SetModeToDev() {
	SetModeToDev()
}

// === Filesystem ===

func (w *Wave) initBaseFS() (fs.FS, error) {
	if GetIsDev() {
		return os.DirFS(w.cfg.Dist.Static()), nil
	}
	if w.distStaticFS == nil {
		return nil, fmt.Errorf("distStaticFS is nil in production mode")
	}
	return w.distStaticFS, nil
}

func (w *Wave) initPublicFS() (fs.FS, error) {
	base, err := w.GetBaseFS()
	if err != nil {
		return nil, err
	}
	return fs.Sub(base, RelPaths.AssetsPublic())
}

func (w *Wave) initPrivateFS() (fs.FS, error) {
	base, err := w.GetBaseFS()
	if err != nil {
		return nil, err
	}
	return fs.Sub(base, RelPaths.AssetsPrivate())
}

func (w *Wave) GetBaseFS() (fs.FS, error) {
	return w.baseFS.get()
}

func (w *Wave) GetPublicFS() (fs.FS, error) {
	return w.publicFS.get()
}

func (w *Wave) GetPrivateFS() (fs.FS, error) {
	return w.privateFS.get()
}

func (w *Wave) MustGetPublicFS() fs.FS {
	f, err := w.publicFS.get()
	if err != nil {
		panic(err)
	}
	return f
}

func (w *Wave) MustGetPrivateFS() fs.FS {
	f, err := w.privateFS.get()
	if err != nil {
		panic(err)
	}
	return f
}

// === File Map ===

func (w *Wave) initFileMap() (FileMap, error) {
	base, err := w.GetBaseFS()
	if err != nil {
		return nil, err
	}
	f, err := base.Open(RelPaths.PublicFileMapGob())
	if err != nil {
		return nil, fmt.Errorf("open file map: %w", err)
	}
	defer f.Close()
	fm, err := fsutil.FromGob[FileMap](f)
	if err != nil {
		return nil, fmt.Errorf("decode file map: %w", err)
	}
	return fm, nil
}

func (w *Wave) GetPublicFileMap() (FileMap, error) {
	return w.fileMap.get()
}

// === URL Resolution ===

func (w *Wave) resolvePublicURL(original string) (string, error) {
	if strings.HasPrefix(original, "data:") {
		return original, nil
	}

	fm, err := w.GetPublicFileMap()
	if err != nil {
		w.log.Warn("failed to load file map", "error", err)
	}

	url, found := fm.Lookup(original, w.cfg.PublicPathPrefix())
	if !found {
		w.log.Warn("no hashed URL found", "url", original)
	}

	return url, nil
}

func (w *Wave) GetPublicURL(original string) string {
	url, _ := w.publicURLs.get(original)
	return url
}

// === Asset Check ===

func (w *Wave) checkIsAsset(urlPath string) (bool, error) {
	publicFS, err := w.GetPublicFS()
	if err != nil {
		return false, err
	}
	clean := strings.TrimPrefix(path.Clean(urlPath), "/")
	_, err = fs.Stat(publicFS, clean)
	return err == nil, nil
}

func (w *Wave) IsPublicAsset(urlPath string) bool {
	prefix := w.cfg.PublicPathPrefix()
	if prefix == "" || prefix == "/" {
		isAsset, _ := w.isAsset.get(urlPath)
		return isAsset
	}
	return strings.HasPrefix(urlPath, prefix)
}

// === Static Serving ===

func (w *Wave) GetServeStaticHandler(immutable bool) (http.Handler, error) {
	publicFS, err := w.GetPublicFS()
	if err != nil {
		return nil, err
	}

	fileServer := http.StripPrefix(
		w.cfg.PublicPathPrefix(),
		http.FileServer(http.FS(publicFS)),
	)

	if !immutable {
		return fileServer, nil
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(rw, req)
	}), nil
}

func (w *Wave) MustGetServeStaticHandler(immutable bool) http.Handler {
	h, err := w.GetServeStaticHandler(immutable)
	if err != nil {
		panic(err)
	}
	return h
}

func (w *Wave) ServeStatic(immutable bool) func(http.Handler) http.Handler {
	handler, err := w.GetServeStaticHandler(immutable)
	if err != nil {
		w.log.Error("failed to create static handler", "error", err)
		panic(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if w.IsPublicAsset(req.URL.Path) {
				handler.ServeHTTP(rw, req)
			} else {
				next.ServeHTTP(rw, req)
			}
		})
	}
}

func (w *Wave) FaviconRedirect() middleware.Middleware {
	return middleware.ToHandlerMiddleware(
		"/favicon.ico",
		[]string{http.MethodGet, http.MethodHead},
		func(rw http.ResponseWriter, req *http.Request) {
			url := w.GetPublicURL("favicon.ico")
			fallback := w.cfg.PublicPathPrefix() + "favicon.ico"
			if url == fallback {
				rw.WriteHeader(http.StatusNotFound)
				return
			}
			http.Redirect(rw, req, url, http.StatusFound)
		},
	)
}

// === Config Accessors ===

func (w *Wave) GetPublicPathPrefix() string     { return w.cfg.PublicPathPrefix() }
func (w *Wave) GetDistDir() string              { return w.cfg.Core.DistDir }
func (w *Wave) GetPublicStaticDir() string      { return w.cfg.Core.StaticAssetDirs.Public }
func (w *Wave) GetPrivateStaticDir() string     { return w.cfg.Core.StaticAssetDirs.Private }
func (w *Wave) GetConfigFile() string           { return w.cfg.Core.ConfigLocation }
func (w *Wave) GetViteManifestLocation() string { return w.cfg.ViteManifestPath() }
func (w *Wave) GetViteOutDir() string           { return w.cfg.Dist.StaticPublic() }
func (w *Wave) GetStaticPrivateOutDir() string  { return w.cfg.Dist.StaticPrivate() }
func (w *Wave) GetStaticPublicOutDir() string   { return w.cfg.Dist.StaticPublic() }

// GetParsedConfig returns the parsed configuration for use by tooling.
// This should only be used by build-time tooling, not at runtime.
func (w *Wave) GetParsedConfig() *ParsedConfig {
	return w.cfg
}
