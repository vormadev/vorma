package wave

import (
	"html/template"
	"io/fs"
	"log/slog"

	"github.com/vormadev/vorma/kit/colorlog"
)

const (
	// CriticalCSSElementID is the default DOM id used for the injected critical
	// CSS style element.
	CriticalCSSElementID = DefaultCriticalCSSStyleElementID
	// StyleSheetElementID is the default DOM id used for the injected
	// non-critical stylesheet link element.
	StyleSheetElementID = DefaultNonCriticalCSSLinkElementID
)

// Wave provides runtime services for Wave applications.
type Wave struct {
	cfg    *ParsedConfig
	rawCfg []byte
	log    *slog.Logger

	isDevMode bool

	portResolver *portResolver

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

// Logger returns the Wave logger instance.
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

// Config configures Wave initialization.
type Config struct {
	// Required -- Raw Wave configuration JSON.
	// This is the single app-facing configuration input path.
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

// New constructs a Wave runtime instance from raw config JSON and a dist/static
// file system root.
func New(c Config) *Wave {
	if len(c.WaveConfigJSON) == 0 {
		panic("wave.New: WaveConfigJSON is required")
	}

	configJSON := cloneBytes(c.WaveConfigJSON)
	cfg, parseError := ParseConfig(configJSON)
	if parseError != nil {
		panic("wave.New: " + parseError.Error())
	}

	w := &Wave{
		cfg:          cfg,
		rawCfg:       configJSON,
		log:          resolveWaveLogger(c.Logger),
		isDevMode:    GetIsDev(),
		distStaticFS: c.DistStaticFS,
		portResolver: newPortResolver(),
	}

	w.initRuntimeCaches()

	return w
}

func resolveWaveLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}

	return colorlog.New("wave")
}

func cloneBytes(data []byte) []byte {
	return append([]byte(nil), data...)
}

// initRuntimeCaches wires all runtime caches against the current mode resolver.
// When mode changes (for example via SetModeToDev), caches are rebuilt so mode-
// sensitive values do not leak across environments.
func (w *Wave) initRuntimeCaches() {
	w.baseFS = newCacheWithModeResolver(w.initBaseFS, w.IsDev)
	w.publicFS = newCacheWithModeResolver(w.initPublicFS, w.IsDev)
	w.privateFS = newCacheWithModeResolver(w.initPrivateFS, w.IsDev)
	w.fileMap = newCacheWithModeResolver(w.initFileMap, w.IsDev)
	w.criticalCSS = newCacheWithModeResolver(w.initCriticalCSS, w.IsDev)
	w.stylesheetURL = newCacheWithModeResolver(w.initStylesheetURL, w.IsDev)
	w.stylesheetLink = newCacheWithModeResolver(w.initStylesheetLink, w.IsDev)
	w.fileMapURL = newCacheWithModeResolver(w.initFileMapURL, w.IsDev)
	w.fileMapDetails = newCacheWithModeResolver(w.initFileMapDetails, w.IsDev)
	w.publicURLs = newCacheMapWithPolicyAndModeResolver(
		w.resolvePublicURL,
		nil,
		w.IsDev,
	)
	w.isAsset = newCacheMapWithPolicyAndModeResolver(
		w.checkIsAsset,
		func(isAsset bool, err error) bool {
			return err == nil && isAsset
		},
		w.IsDev,
	)
}
