// Package waveruntimecore hosts low-level Wave runtime mechanics.
//
// The top-level `wave` package intentionally remains a thin, app-facing API
// surface and delegates implementation details here.
package waveruntimecore

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/wave/internal/wavecache"
	"github.com/vormadev/vorma/wave/internal/wavefilemap"
	"github.com/vormadev/vorma/wave/internal/waveruntime"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveenv"
	"github.com/vormadev/vorma/wave/waveframework"
)

const (
	segAssets   = waveartifacts.AssetsDirname
	segPublic   = waveartifacts.PublicDirname
	segPrivate  = waveartifacts.PrivateDirname
	segInternal = waveartifacts.InternalDirname
)

const (
	fileCriticalCSS  = waveartifacts.CriticalCSSFileName
	fileNormalCSSRef = waveartifacts.NormalCSSRefFileName
	filePublicMapRef = waveartifacts.PublicFileMapRefFileName
	filePublicMapGob = waveartifacts.PublicFileMapGobFileName
)

const defaultRefreshPort = 10000

// Config configures low-level runtime behavior for one Wave instance.
type Config struct {
	ParsedConfig  waveconfig.ParsedConfig
	RawConfigJSON []byte
	DistStaticFS  fs.FS
	Logger        *slog.Logger
	IsDevMode     bool
	PortResolver  *waveenv.Resolver
}

// Runtime owns runtime caches and filesystem-backed resolution behavior.
type Runtime struct {
	cfg    waveconfig.ParsedConfig
	rawCfg []byte
	log    *slog.Logger

	isDevMode bool

	portResolver *waveenv.Resolver

	distStaticFS fs.FS

	baseFS         *wavecache.ValueCache[fs.FS]
	publicFS       *wavecache.ValueCache[fs.FS]
	privateFS      *wavecache.ValueCache[fs.FS]
	fileMap        *wavecache.ValueCache[wavefilemap.FileMap]
	criticalCSS    *wavecache.ValueCache[*criticalCSSData]
	stylesheetURL  *wavecache.ValueCache[string]
	stylesheetLink *wavecache.ValueCache[string]
	fileMapURL     *wavecache.ValueCache[string]
	fileMapDetails *wavecache.ValueCache[*FileMapDetails]
	publicURLs     *wavecache.KeyedCache[string, string]
	isAsset        *wavecache.KeyedCache[string, bool]
}

type criticalCSSData struct {
	content    string
	noSuchFile bool
	styleEl    template.HTML
	sha256Hash string
}

// FileMapDetails stores rendered browser file-map elements and CSP hash.
type FileMapDetails struct {
	Elements   string
	SHA256Hash string
}

// New constructs one low-level runtime instance.
func New(c Config) *Runtime {
	logger := c.Logger
	if logger == nil {
		logger = colorlog.New("wave")
	}

	resolver := c.PortResolver
	if resolver == nil {
		resolver = waveenv.NewResolverForMode(c.IsDevMode)
	}

	runtime := &Runtime{
		cfg:          c.ParsedConfig,
		rawCfg:       cloneBytes(c.RawConfigJSON),
		log:          logger,
		isDevMode:    c.IsDevMode,
		portResolver: resolver,
		distStaticFS: c.DistStaticFS,
	}

	runtime.initRuntimeCaches()

	return runtime
}

// Logger returns the runtime logger.
func (runtime *Runtime) Logger() *slog.Logger { return runtime.log }

// RawConfigJSON returns a defensive copy of raw config JSON.
func (runtime *Runtime) RawConfigJSON() []byte {
	return cloneBytes(runtime.rawCfg)
}

// IsDev reports whether this runtime currently runs in development mode.
func (runtime *Runtime) IsDev() bool {
	if runtime == nil {
		return waveenv.GetIsDev()
	}
	return runtime.isDevMode
}

// MustGetPort resolves and returns the runtime port.
func (runtime *Runtime) MustGetPort() int {
	if runtime == nil || runtime.portResolver == nil {
		resolver := waveenv.NewResolver()
		return resolver.MustGetPort()
	}
	return runtime.portResolver.MustGetPort()
}

// SetPortResolver installs one resolver for this runtime instance.
func (runtime *Runtime) SetPortResolver(resolver *waveenv.Resolver) {
	if runtime == nil {
		return
	}
	runtime.portResolver = resolver
}

// SetDevMode switches this runtime instance into development mode and resets
// mode-sensitive caches.
func (runtime *Runtime) SetDevMode() {
	if runtime == nil {
		return
	}
	runtime.isDevMode = true
	runtime.portResolver = waveenv.NewResolverForMode(true)
	runtime.initRuntimeCaches()
}

func cloneBytes(data []byte) []byte {
	return append([]byte(nil), data...)
}

// PublicPathPrefix returns the normalized configured public path prefix.
func (runtime *Runtime) PublicPathPrefix() string {
	return runtime.cfg.PublicPathPrefix()
}

// DistDir returns the configured build output directory root.
func (runtime *Runtime) DistDir() string {
	return runtime.cfg.Dist().Root()
}

// PublicStaticDir returns the source directory for public static assets.
func (runtime *Runtime) PublicStaticDir() string {
	return runtime.cfg.Core().StaticAssetDirsPublic()
}

// PrivateStaticDir returns the source directory for private static assets.
func (runtime *Runtime) PrivateStaticDir() string {
	return runtime.cfg.Core().StaticAssetDirsPrivate()
}

// ViteManifestLocation returns the expected Vite manifest path in build output.
func (runtime *Runtime) ViteManifestLocation() string {
	return runtime.cfg.ViteManifestPath()
}

// ViteOutDir returns the directory where Vite emits public artifacts.
func (runtime *Runtime) ViteOutDir() string {
	return runtime.cfg.Dist().StaticPublic()
}

// StaticPrivateOutDir returns the private static output directory.
func (runtime *Runtime) StaticPrivateOutDir() string {
	return runtime.cfg.Dist().StaticPrivate()
}

// StaticPublicOutDir returns the public static output directory.
func (runtime *Runtime) StaticPublicOutDir() string {
	return runtime.cfg.Dist().StaticPublic()
}

func (runtime *Runtime) initRuntimeCaches() {
	runtime.baseFS = wavecache.NewValueCacheWithModeResolver(
		runtime.initBaseFS,
		runtime.IsDev,
	)
	runtime.publicFS = wavecache.NewValueCacheWithModeResolver(
		runtime.initPublicFS,
		runtime.IsDev,
	)
	runtime.privateFS = wavecache.NewValueCacheWithModeResolver(
		runtime.initPrivateFS,
		runtime.IsDev,
	)
	runtime.fileMap = wavecache.NewValueCacheWithModeResolver(
		runtime.initFileMap,
		runtime.IsDev,
	)
	runtime.criticalCSS = wavecache.NewValueCacheWithModeResolver(
		runtime.initCriticalCSS,
		runtime.IsDev,
	)
	runtime.stylesheetURL = wavecache.NewValueCacheWithModeResolver(
		runtime.initStylesheetURL,
		runtime.IsDev,
	)
	runtime.stylesheetLink = wavecache.NewValueCacheWithModeResolver(
		runtime.initStylesheetLink,
		runtime.IsDev,
	)
	runtime.fileMapURL = wavecache.NewValueCacheWithModeResolver(
		runtime.initFileMapURL,
		runtime.IsDev,
	)
	runtime.fileMapDetails = wavecache.NewValueCacheWithModeResolver(
		runtime.initFileMapDetails,
		runtime.IsDev,
	)
	runtime.publicURLs = wavecache.NewKeyedCacheWithPolicyAndModeResolver(
		runtime.resolvePublicURL,
		nil,
		runtime.IsDev,
	)
	runtime.isAsset = wavecache.NewKeyedCacheWithPolicyAndModeResolver(
		runtime.checkIsAsset,
		func(isAsset bool, err error) bool {
			return err == nil && isAsset
		},
		runtime.IsDev,
	)
}

func (runtime *Runtime) initBaseFS() (fs.FS, error) {
	if runtime.distStaticFS == nil {
		if runtime.IsDev() {
			return os.DirFS(runtime.cfg.Dist().Static()), nil
		}
		return nil, fmt.Errorf("distStaticFS is nil in production mode")
	}
	return runtime.distStaticFS, nil
}

func (runtime *Runtime) initPublicFS() (fs.FS, error) {
	return runtime.initSubFS(publicAssetsRelativePath())
}

func (runtime *Runtime) initPrivateFS() (fs.FS, error) {
	return runtime.initSubFS(privateAssetsRelativePath())
}

func (runtime *Runtime) initSubFS(
	relativeSubdirectoryPath string,
) (fs.FS, error) {
	base, err := runtime.GetBaseFS()
	if err != nil {
		return nil, err
	}
	return fs.Sub(base, relativeSubdirectoryPath)
}

// GetBaseFS returns the runtime base filesystem.
func (runtime *Runtime) GetBaseFS() (fs.FS, error) {
	return runtime.baseFS.Get()
}

// GetPublicFS returns the runtime public-assets filesystem.
func (runtime *Runtime) GetPublicFS() (fs.FS, error) {
	return runtime.publicFS.Get()
}

// PrivateFS returns the runtime private-assets filesystem.
func (runtime *Runtime) PrivateFS() (fs.FS, error) {
	return runtime.privateFS.Get()
}

// MustPrivateFS returns the private filesystem or panics if unavailable.
func (runtime *Runtime) MustPrivateFS() fs.FS {
	fileSystem, err := runtime.privateFS.Get()
	if err != nil {
		panic(err)
	}
	return fileSystem
}

func (runtime *Runtime) readTrimmedInternalRefFile(
	relativePath string,
) (string, error) {
	baseFS, err := runtime.GetBaseFS()
	if err != nil {
		return "", err
	}

	content, err := fs.ReadFile(baseFS, relativePath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}

func (runtime *Runtime) initPublicURLFromInternalRefFile(
	relativePath string,
) (string, error) {
	refPath, err := runtime.readTrimmedInternalRefFile(relativePath)
	if err != nil {
		return "", err
	}

	return waveenv.ResolveFromReferencedPath(
		runtime.cfg.PublicPathPrefix(),
		refPath,
	), nil
}

func (runtime *Runtime) initFileMap() (wavefilemap.FileMap, error) {
	base, err := runtime.GetBaseFS()
	if err != nil {
		return nil, err
	}

	f, err := base.Open(publicFileMapGobRelativePath())
	if err != nil {
		return nil, fmt.Errorf("open file map: %w", err)
	}
	defer f.Close()

	fileMap, err := fsutil.FromGob[wavefilemap.FileMap](f)
	if err != nil {
		return nil, fmt.Errorf("decode file map: %w", err)
	}

	return fileMap, nil
}

// PublicFileMap returns a defensive snapshot of the public file map.
func (runtime *Runtime) PublicFileMap() (wavefilemap.FileMap, error) {
	fileMap, err := runtime.fileMap.Get()
	if err != nil {
		return nil, err
	}

	return wavefilemap.Clone(fileMap), nil
}

func (runtime *Runtime) resolvePublicURL(original string) (string, error) {
	fileMap, err := runtime.fileMap.Get()
	if err != nil {
		runtime.log.Warn("failed to load file map", "error", err)
		return "", err
	}

	url, found := fileMap.Lookup(original, runtime.cfg.PublicPathPrefix())
	if !found {
		runtime.log.Warn("no hashed URL found", "url", original)
		return "", fmt.Errorf("no hashed URL found for %q", original)
	}

	return url, nil
}

// PublicURL resolves one source public asset path to its built URL.
func (runtime *Runtime) PublicURL(original string) string {
	url, _ := runtime.publicURLs.Get(original)
	return url
}

func (runtime *Runtime) checkIsAsset(urlPath string) (bool, error) {
	publicAssetPath, isPublicAssetPath := runtime.publicAssetPath(urlPath)
	if !isPublicAssetPath {
		return false, nil
	}

	publicFS, err := runtime.GetPublicFS()
	if err != nil {
		return false, err
	}

	info, err := fs.Stat(publicFS, publicAssetPath)
	if err != nil {
		return false, nil
	}

	return !info.IsDir(), nil
}

func (runtime *Runtime) publicAssetPath(urlPath string) (string, bool) {
	cleanURLPath := path.Clean("/" + urlPath)
	if cleanURLPath == "/" {
		return "", false
	}

	prefix := runtime.cfg.PublicPathPrefix()
	if prefix != "/" {
		if !strings.HasPrefix(cleanURLPath, prefix) {
			return "", false
		}
		cleanURLPath = strings.TrimPrefix(cleanURLPath, prefix)
	}

	assetPath := strings.TrimPrefix(cleanURLPath, "/")
	if assetPath == "" || assetPath == "." {
		return "", false
	}

	return assetPath, true
}

// IsPublicAsset reports whether the path resolves to a concrete public asset.
func (runtime *Runtime) IsPublicAsset(urlPath string) bool {
	isAsset, _ := runtime.isAsset.Get(urlPath)
	return isAsset
}

func (runtime *Runtime) initCriticalCSS() (*criticalCSSData, error) {
	if runtime.cfg.CriticalCSSEntry() == "" {
		return &criticalCSSData{noSuchFile: true}, nil
	}

	content, noSuchFile, err := runtime.readCriticalCSSContent()
	if err != nil {
		return nil, err
	}
	if noSuchFile {
		return &criticalCSSData{noSuchFile: true}, nil
	}

	return runtime.buildCriticalCSSData(content)
}

func (runtime *Runtime) readCriticalCSSContent() (string, bool, error) {
	baseFS, err := runtime.GetBaseFS()
	if err != nil {
		return "", false, err
	}

	content, err := fs.ReadFile(baseFS, criticalCSSRelativePath())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", true, nil
		}
		return "", false, err
	}

	return string(content), false, nil
}

func (runtime *Runtime) buildCriticalCSSData(
	content string,
) (*criticalCSSData, error) {
	styleElement, sha256Hash, err := waveruntime.BuildCriticalCSSStyleElement(
		content,
		waveframework.CriticalCSSStyleElementID(runtime.cfg),
	)
	if err != nil {
		runtime.log.Error(
			fmt.Sprintf("error building critical css style element: %v", err),
		)
		return nil, err
	}

	return &criticalCSSData{
		content:    content,
		styleEl:    styleElement,
		sha256Hash: sha256Hash,
	}, nil
}

// GetCriticalCSSData returns cached critical-css data when available.
func (runtime *Runtime) GetCriticalCSSData() *criticalCSSData {
	data, err := runtime.criticalCSS.Get()
	if err != nil || data == nil || data.noSuchFile {
		return nil
	}
	return data
}

// CriticalCSS returns critical CSS content when available.
func (runtime *Runtime) CriticalCSS() template.CSS {
	data := runtime.GetCriticalCSSData()
	if data == nil {
		return ""
	}
	return template.CSS(data.content)
}

// CriticalCSSStyleElement returns one rendered critical-css <style> element.
func (runtime *Runtime) CriticalCSSStyleElement() template.HTML {
	data := runtime.GetCriticalCSSData()
	if data == nil {
		return ""
	}
	return data.styleEl
}

func (runtime *Runtime) initStylesheetURL() (string, error) {
	if runtime.cfg.NonCriticalCSSEntry() == "" {
		return "", nil
	}

	return runtime.initPublicURLFromInternalRefFile(normalCSSRefRelativePath())
}

// StyleSheetURL returns the non-critical stylesheet URL.
func (runtime *Runtime) StyleSheetURL() string {
	url, _ := runtime.stylesheetURL.Get()
	return url
}

func (runtime *Runtime) initStylesheetLink() (string, error) {
	url := runtime.StyleSheetURL()
	if url == "" {
		return "", nil
	}

	return waveruntime.BuildStylesheetLink(
		url,
		waveframework.NonCriticalCSSLinkElementID(runtime.cfg),
	), nil
}

// StyleSheetLinkElement returns one rendered non-critical stylesheet <link>.
func (runtime *Runtime) StyleSheetLinkElement() template.HTML {
	link, _ := runtime.stylesheetLink.Get()
	return template.HTML(link)
}

func (runtime *Runtime) initFileMapURL() (string, error) {
	return runtime.initPublicURLFromInternalRefFile(
		publicFileMapRefRelativePath(),
	)
}

// PublicFileMapURL returns the generated public file-map URL.
func (runtime *Runtime) PublicFileMapURL() string {
	url, _ := runtime.fileMapURL.Get()
	return url
}

func (runtime *Runtime) initFileMapDetails() (*FileMapDetails, error) {
	fileMapURL := runtime.PublicFileMapURL()
	if fileMapURL == "" {
		return &FileMapDetails{}, nil
	}

	elements, sha256Hash, err := waveruntime.BuildPublicFileMapElements(
		fileMapURL,
		waveframework.BrowserRuntimeNamespace(runtime.cfg),
	)
	if err != nil {
		return nil, err
	}

	return &FileMapDetails{
		Elements:   elements,
		SHA256Hash: sha256Hash,
	}, nil
}

// FileMapDetailsFromCache returns cached file-map details when available.
func (runtime *Runtime) FileMapDetailsFromCache() *FileMapDetails {
	if runtime == nil || runtime.fileMapDetails == nil {
		return nil
	}
	fileMapDetails, err := runtime.fileMapDetails.Get()
	if err != nil {
		return nil
	}
	return fileMapDetails
}

// SetFileMapDetailsCache replaces the file-map details cache instance.
func (runtime *Runtime) SetFileMapDetailsCache(
	fileMapDetailsCache *wavecache.ValueCache[*FileMapDetails],
) {
	if runtime == nil {
		return
	}
	runtime.fileMapDetails = fileMapDetailsCache
}

// RefreshScript returns one rendered dev refresh script.
func (runtime *Runtime) RefreshScript() template.HTML {
	if !runtime.IsDev() {
		return ""
	}

	port := waveenv.ParseRefreshServerPort()
	if port == 0 {
		port = defaultRefreshPort
	}

	return template.HTML(
		fmt.Sprintf(
			"<script>%s</script>",
			waveruntime.BuildRefreshScript(
				port,
				waveruntime.RefreshScriptConfig{
					BrowserRevalidateFunctionName: waveframework.BrowserRevalidateFunctionName(
						runtime.cfg,
					),
					RefreshRebuildingOverlayElementID: waveframework.RefreshRebuildingOverlayElementID(
						runtime.cfg,
					),
					NonCriticalCSSLinkElementID: waveframework.NonCriticalCSSLinkElementID(
						runtime.cfg,
					),
					CriticalCSSStyleElementID: waveframework.CriticalCSSStyleElementID(
						runtime.cfg,
					),
				},
			),
		),
	)
}

// StaticHandler returns an asset-serving handler for static public files.
func (runtime *Runtime) StaticHandler(immutable bool) (http.Handler, error) {
	publicFS, err := runtime.GetPublicFS()
	if err != nil {
		return nil, err
	}

	fileServer := http.StripPrefix(
		runtime.cfg.PublicPathPrefix(),
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

// MustStaticMiddleware returns middleware that serves static public assets and
// delegates all other requests to next.
func (runtime *Runtime) MustStaticMiddleware(
	immutable bool,
) func(http.Handler) http.Handler {
	handler, err := runtime.StaticHandler(immutable)
	if err != nil {
		runtime.log.Error("failed to create static handler", "error", err)
		panic(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(rw http.ResponseWriter, req *http.Request) {
				if runtime.IsPublicAsset(req.URL.Path) {
					handler.ServeHTTP(rw, req)
					return
				}
				next.ServeHTTP(rw, req)
			},
		)
	}
}

func publicAssetsRelativePath() string {
	return segAssets + "/" + segPublic
}

func privateAssetsRelativePath() string {
	return segAssets + "/" + segPrivate
}

func criticalCSSRelativePath() string {
	return segInternal + "/" + fileCriticalCSS
}

func normalCSSRefRelativePath() string {
	return segInternal + "/" + fileNormalCSSRef
}

func publicFileMapRefRelativePath() string {
	return segInternal + "/" + filePublicMapRef
}

func publicFileMapGobRelativePath() string {
	return segInternal + "/" + filePublicMapGob
}
