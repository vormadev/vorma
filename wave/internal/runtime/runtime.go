// Package runtime provides filesystem access, URL resolution, and static serving
// for Wave applications at runtime. This is the only internal package used at
// runtime by the application itself (as opposed to build/dev time).
package runtime

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/safecache"
	"github.com/vormadev/vorma/wave/internal/config"
)

const (
	CriticalCSSElementID = "wave-critical-css"
	StyleSheetElementID  = "wave-normal-css"
)

// Runtime provides runtime services for Wave applications.
// It uses caching with dev-mode bypass for optimal performance.
type Runtime struct {
	cfg    *config.Config
	distFS fs.FS // embedded or disk-based FS rooted at dist/static
	log    *slog.Logger

	// Cached values with dev-mode bypass
	baseFS    *safecache.Cache[fs.FS]
	publicFS  *safecache.Cache[fs.FS]
	privateFS *safecache.Cache[fs.FS]
	fileMap   *safecache.Cache[config.FileMap]

	// CSS caches
	criticalCSS    *safecache.Cache[*criticalCSSData]
	stylesheetURL  *safecache.Cache[string]
	stylesheetLink *safecache.Cache[string]

	// File map caches
	fileMapURL     *safecache.Cache[string]
	fileMapDetails *safecache.Cache[*fileMapDetails]

	// URL resolution cache
	publicURLs *safecache.CacheMap[string, string, string]
	isAsset    *safecache.CacheMap[string, string, bool]
}

// criticalCSSData holds pre-computed critical CSS data
type criticalCSSData struct {
	content    string
	noSuchFile bool
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

	// Dev mode bypass function - always re-read in dev mode
	bypass := config.GetIsDev
	bypassKey := func(string) bool { return config.GetIsDev() }
	identity := func(s string) string { return s }

	// Initialize caches
	r.baseFS = safecache.New(r.initBaseFS, bypass)
	r.publicFS = safecache.New(r.initPublicFS, bypass)
	r.privateFS = safecache.New(r.initPrivateFS, bypass)
	r.fileMap = safecache.New(r.initFileMap, bypass)
	r.criticalCSS = safecache.New(r.initCriticalCSS, bypass)
	r.stylesheetURL = safecache.New(r.initStylesheetURL, bypass)
	r.stylesheetLink = safecache.New(r.initStylesheetLink, bypass)
	r.fileMapURL = safecache.New(r.initFileMapURL, bypass)
	r.fileMapDetails = safecache.New(r.initFileMapDetails, bypass)
	r.publicURLs = safecache.NewMap(r.resolvePublicURL, identity, bypassKey)
	r.isAsset = safecache.NewMap(r.checkIsAsset, identity, bypassKey)

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
	return fs.Sub(base, "assets/public")
}

func (r *Runtime) initPrivateFS() (fs.FS, error) {
	base, err := r.BaseFS()
	if err != nil {
		return nil, err
	}
	return fs.Sub(base, "assets/private")
}

// BaseFS returns the base filesystem (dist/static)
func (r *Runtime) BaseFS() (fs.FS, error) {
	return r.baseFS.Get()
}

// PublicFS returns the public assets filesystem
func (r *Runtime) PublicFS() (fs.FS, error) {
	return r.publicFS.Get()
}

// PrivateFS returns the private assets filesystem
func (r *Runtime) PrivateFS() (fs.FS, error) {
	return r.privateFS.Get()
}

// === File Map ===

func (r *Runtime) initFileMap() (config.FileMap, error) {
	base, err := r.BaseFS()
	if err != nil {
		return nil, err
	}

	f, err := base.Open("internal/" + config.PublicFileMapGobName)
	if err != nil {
		return nil, fmt.Errorf("open file map: %w", err)
	}
	defer f.Close()

	fm, err := config.DecodeFileMap(f)
	if err != nil {
		return nil, fmt.Errorf("decode file map: %w", err)
	}
	return fm, nil
}

// PublicFileMap returns the public file map
func (r *Runtime) PublicFileMap() (config.FileMap, error) {
	return r.fileMap.Get()
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
	url, _ := r.publicURLs.Get(original)
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
		isAsset, _ := r.isAsset.Get(urlPath)
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

func sha256Base64(data []byte) string {
	return bytesutil.ToBase64(cryptoutil.Sha256Hash(data))
}
