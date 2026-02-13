package wave

import (
	"context"
	"html/template"
	"io/fs"
	"log/slog"

	"github.com/vormadev/vorma/kit/colorlog"
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

// Config configures Wave initialization.
type Config struct {
	// Optional -- Raw Wave configuration JSON.
	// This is the simplest app-facing path when configuration is authored in JSON.
	// Exactly one of WaveConfigJSON or ConfigSource must be provided.
	WaveConfigJSON []byte

	// Optional -- Wave loads configuration through this source.
	// The source can be provider-backed in dev, or static/in-memory in production.
	// Exactly one of WaveConfigJSON or ConfigSource must be provided.
	ConfigSource ConfigSource

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
	if c.ConfigSource != nil && len(c.WaveConfigJSON) > 0 {
		panic("wave.New: exactly one of WaveConfigJSON or ConfigSource must be provided")
	}

	resolvedConfigSource := c.ConfigSource
	if resolvedConfigSource == nil {
		if len(c.WaveConfigJSON) == 0 {
			panic("wave.New: WaveConfigJSON or ConfigSource is required")
		}
		resolvedConfigSource = NewStaticConfigSource(cloneBytes(c.WaveConfigJSON))
	}

	loadedCfg, parseError := resolvedConfigSource.LoadConfig(context.Background())
	if parseError != nil {
		panic("wave.New: load config source: " + parseError.Error())
	}
	if loadedCfg == nil {
		panic("wave.New: config source returned nil")
	}

	cfg, parseError := ParseConfigFromLoadedConfig(loadedCfg, resolvedConfigSource)
	if parseError != nil {
		panic("wave.New: " + parseError.Error())
	}

	w := &Wave{
		cfg:          cfg,
		rawCfg:       cloneBytes(loadedCfg.ConfigJSON),
		log:          resolveWaveLogger(c.Logger),
		distStaticFS: c.DistStaticFS,
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

func (w *Wave) initRuntimeCaches() {
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
}
