// Package runtime provides filesystem access, URL resolution, and static serving
// for Wave applications at runtime. This is the only internal package used at
// runtime by the application itself (as opposed to build/dev time).
package runtime

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

	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave/internal/config"
)

const (
	CriticalCSSElementID = "wave-critical-css"
	StyleSheetElementID  = "wave-normal-css"
)

// cacheInProd holds a lazily-initialized value that is cached in prod but
// recomputed on every access in dev mode.
type cacheInProd[T any] struct {
	val      T
	err      error
	once     sync.Once
	initFunc func() (T, error)
}

func newCacheInProd[T any](initFunc func() (T, error)) *cacheInProd[T] {
	return &cacheInProd[T]{initFunc: initFunc}
}

func (c *cacheInProd[T]) get() (T, error) {
	if config.GetIsDev() {
		return c.initFunc()
	}
	c.once.Do(func() { c.val, c.err = c.initFunc() })
	return c.val, c.err
}

// cacheInProdMap holds lazily-initialized keyed values that are cached in prod
// but recomputed on every access in dev mode.
type cacheInProdMap[K comparable, V any] struct {
	cache    sync.Map
	initFunc func(K) (V, error)
}

type cacheInProdEntry[V any] struct {
	val V
	err error
}

func newCacheInProdMap[K comparable, V any](initFunc func(K) (V, error)) *cacheInProdMap[K, V] {
	return &cacheInProdMap[K, V]{initFunc: initFunc}
}

func (m *cacheInProdMap[K, V]) get(key K) (V, error) {
	if config.GetIsDev() {
		return m.initFunc(key)
	}
	if entry, ok := m.cache.Load(key); ok {
		e := entry.(*cacheInProdEntry[V])
		return e.val, e.err
	}
	val, err := m.initFunc(key)
	actual, _ := m.cache.LoadOrStore(key, &cacheInProdEntry[V]{val: val, err: err})
	e := actual.(*cacheInProdEntry[V])
	return e.val, e.err
}

// Runtime provides runtime services for Wave applications.
type Runtime struct {
	cfg    *config.Config
	distFS fs.FS
	log    *slog.Logger

	baseFS    *cacheInProd[fs.FS]
	publicFS  *cacheInProd[fs.FS]
	privateFS *cacheInProd[fs.FS]
	fileMap   *cacheInProd[config.FileMap]

	criticalCSS    *cacheInProd[*criticalCSSData]
	stylesheetURL  *cacheInProd[string]
	stylesheetLink *cacheInProd[string]

	fileMapURL     *cacheInProd[string]
	fileMapDetails *cacheInProd[*fileMapDetails]

	publicURLs *cacheInProdMap[string, string]
	isAsset    *cacheInProdMap[string, bool]
}

// criticalCSSData holds pre-computed critical CSS data
type criticalCSSData struct {
	content    string
	noSuchFile bool
	styleEl    template.HTML
	sha256Hash string
}

// fileMapDetails holds pre-computed file map HTML
type fileMapDetails struct {
	elements   string
	sha256Hash string
}

// New creates a new Runtime instance with lazy-initialized caches
func New(cfg *config.Config, distStaticFS fs.FS, log *slog.Logger) *Runtime {
	r := &Runtime{
		cfg:    cfg,
		distFS: distStaticFS,
		log:    log,
	}

	r.baseFS = newCacheInProd(r.initBaseFS)
	r.publicFS = newCacheInProd(r.initPublicFS)
	r.privateFS = newCacheInProd(r.initPrivateFS)
	r.fileMap = newCacheInProd(r.initFileMap)
	r.criticalCSS = newCacheInProd(r.initCriticalCSS)
	r.stylesheetURL = newCacheInProd(r.initStylesheetURL)
	r.stylesheetLink = newCacheInProd(r.initStylesheetLink)
	r.fileMapURL = newCacheInProd(r.initFileMapURL)
	r.fileMapDetails = newCacheInProd(r.initFileMapDetails)
	r.publicURLs = newCacheInProdMap(r.resolvePublicURL)
	r.isAsset = newCacheInProdMap(r.checkIsAsset)

	return r
}

// Config returns the runtime's config
func (r *Runtime) Config() *config.Config {
	return r.cfg
}

// === Filesystem ===

func (r *Runtime) initBaseFS() (fs.FS, error) {
	if config.GetIsDev() {
		return os.DirFS(r.cfg.Dist.Static()), nil
	}
	if r.distFS == nil {
		return nil, fmt.Errorf("distStaticFS is nil in production mode")
	}
	return r.distFS, nil
}

func (r *Runtime) initPublicFS() (fs.FS, error) {
	base, err := r.BaseFS()
	if err != nil {
		return nil, err
	}
	return fs.Sub(base, config.RelPaths.AssetsPublic())
}

func (r *Runtime) initPrivateFS() (fs.FS, error) {
	base, err := r.BaseFS()
	if err != nil {
		return nil, err
	}
	return fs.Sub(base, config.RelPaths.AssetsPrivate())
}

// BaseFS returns the base filesystem (dist/static)
func (r *Runtime) BaseFS() (fs.FS, error) {
	return r.baseFS.get()
}

// PublicFS returns the public assets filesystem
func (r *Runtime) PublicFS() (fs.FS, error) {
	return r.publicFS.get()
}

// PrivateFS returns the private assets filesystem
func (r *Runtime) PrivateFS() (fs.FS, error) {
	return r.privateFS.get()
}

// === File Map ===

func (r *Runtime) initFileMap() (config.FileMap, error) {
	base, err := r.BaseFS()
	if err != nil {
		return nil, err
	}

	f, err := base.Open(config.RelPaths.PublicFileMapGob())
	if err != nil {
		return nil, fmt.Errorf("open file map: %w", err)
	}
	defer f.Close()

	fm, err := fsutil.FromGob[config.FileMap](f)
	if err != nil {
		return nil, fmt.Errorf("decode file map: %w", err)
	}
	return fm, nil
}

// PublicFileMap returns the public file map
func (r *Runtime) PublicFileMap() (config.FileMap, error) {
	return r.fileMap.get()
}

// === URL Resolution ===

func (r *Runtime) resolvePublicURL(original string) (string, error) {
	if strings.HasPrefix(original, "data:") {
		return original, nil
	}

	fm, err := r.PublicFileMap()
	if err != nil {
		r.log.Warn("failed to load file map", "error", err)
	}

	url, found := fm.Lookup(original, r.cfg.PublicPathPrefix())
	if !found {
		r.log.Warn("no hashed URL found", "url", original)
	}

	return url, nil
}

// PublicURL returns the hashed URL for a public asset
func (r *Runtime) PublicURL(original string) string {
	url, _ := r.publicURLs.get(original)
	return url
}

// === Asset Check ===

func (r *Runtime) checkIsAsset(urlPath string) (bool, error) {
	publicFS, err := r.PublicFS()
	if err != nil {
		return false, err
	}
	clean := strings.TrimPrefix(path.Clean(urlPath), "/")
	_, err = fs.Stat(publicFS, clean)
	return err == nil, nil
}

// IsPublicAsset checks if a path corresponds to a public asset
func (r *Runtime) IsPublicAsset(urlPath string) bool {
	prefix := r.cfg.PublicPathPrefix()
	if prefix == "" || prefix == "/" {
		isAsset, _ := r.isAsset.get(urlPath)
		return isAsset
	}
	return strings.HasPrefix(urlPath, prefix)
}

// === Static Serving ===

// StaticHandler returns an HTTP handler for serving static files
func (r *Runtime) StaticHandler(immutableCache bool) (http.Handler, error) {
	publicFS, err := r.PublicFS()
	if err != nil {
		return nil, err
	}

	fileServer := http.StripPrefix(
		r.cfg.PublicPathPrefix(),
		http.FileServer(http.FS(publicFS)),
	)

	if !immutableCache {
		return fileServer, nil
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(w, req)
	}), nil
}

// StaticMiddleware returns middleware for serving static public assets
func (r *Runtime) StaticMiddleware(immutableCache bool) func(http.Handler) http.Handler {
	handler, err := r.StaticHandler(immutableCache)
	if err != nil {
		r.log.Error("failed to create static handler", "error", err)
		panic(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if r.IsPublicAsset(req.URL.Path) {
				handler.ServeHTTP(w, req)
			} else {
				next.ServeHTTP(w, req)
			}
		})
	}
}
