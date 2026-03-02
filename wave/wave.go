// Package wave is the end-user runtime API for Wave applications.
//
// Application code should only need this package to construct runtime behavior
// and attach middleware/templating helpers.
package wave

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/vormadev/vorma/wave/internal/waveruntimecore"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveenv"
)

var defaultPortResolver = waveenv.NewResolver()

// GetIsDev reports whether Wave is running in development mode.
func GetIsDev() bool {
	return waveenv.GetIsDev()
}

// SetModeToDev marks the current process as development mode.
func SetModeToDev() {
	waveenv.SetModeToDev()
}

// MustGetPort returns the application runtime port.
// In dev mode it resolves and caches a framework-controlled free port.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func MustGetPort() int {
	if defaultPortResolver == nil {
		defaultPortResolver = waveenv.NewResolver()
	}
	return defaultPortResolver.MustGetPort()
}

// Config configures Wave initialization.
type Config struct {
	// Required -- Raw Wave configuration JSON.
	WaveConfigJSON []byte

	// Required -- this must root at <distDir>/static for production.
	DistStaticFS fs.FS

	// Optional logger.
	Logger *slog.Logger
}

// Wave provides the app-facing runtime API surface.
type Wave struct {
	cfg     *waveconfig.ParsedConfig
	runtime *waveruntimecore.Runtime
}

// New constructs a Wave runtime instance from raw config JSON and a dist/static
// file system root.
func New(c Config) *Wave {
	if len(c.WaveConfigJSON) == 0 {
		panic("wave.New: WaveConfigJSON is required")
	}

	rawConfigJSON := append([]byte(nil), c.WaveConfigJSON...)
	parsedConfig, parseError := waveconfig.ParseConfigJSON(rawConfigJSON)
	if parseError != nil {
		panic("wave.New: " + parseError.Error())
	}

	waveRuntime := &Wave{cfg: parsedConfig}
	waveRuntime.runtime = waveruntimecore.New(
		waveruntimecore.Config{
			ParsedConfig:  parsedConfig,
			RawConfigJSON: rawConfigJSON,
			DistStaticFS:  c.DistStaticFS,
			Logger:        c.Logger,
			IsDevMode:     GetIsDev(),
			PortResolver:  waveenv.NewResolverForMode(GetIsDev()),
		},
	)

	return waveRuntime
}

// Logger returns the Wave logger instance.
func (w *Wave) Logger() *slog.Logger {
	return w.runtime.Logger()
}

// RawConfigJSON returns the raw bytes of the configuration file.
func (w *Wave) RawConfigJSON() []byte {
	if w == nil {
		return nil
	}
	return w.runtime.RawConfigJSON()
}

// IsDev returns true if running in development mode.
func (w *Wave) IsDev() bool {
	if w == nil {
		return GetIsDev()
	}
	return w.runtime.IsDev()
}

// MustGetPort returns the application runtime port.
func (w *Wave) MustGetPort() int {
	if w == nil {
		return MustGetPort()
	}
	return w.runtime.MustGetPort()
}

// SetModeToDev sets the environment to development mode.
func (w *Wave) SetModeToDev() {
	if w != nil {
		w.runtime.SetDevMode()
	}
	SetModeToDev()
}

// PublicPathPrefix returns the normalized configured public path prefix.
func (w *Wave) PublicPathPrefix() string {
	return w.runtime.PublicPathPrefix()
}

// DistDir returns the configured build output directory root.
func (w *Wave) DistDir() string {
	return w.runtime.DistDir()
}

// PrivateStaticDir returns the source directory for private static assets.
func (w *Wave) PrivateStaticDir() string {
	return w.runtime.PrivateStaticDir()
}

// ViteManifestLocation returns the expected path of the Vite manifest in build
// output.
func (w *Wave) ViteManifestLocation() string {
	return w.runtime.ViteManifestLocation()
}

// StaticPrivateOutDir returns the private static output directory.
func (w *Wave) StaticPrivateOutDir() string {
	return w.runtime.StaticPrivateOutDir()
}

// StaticPublicOutDir returns the public static output directory.
func (w *Wave) StaticPublicOutDir() string {
	return w.runtime.StaticPublicOutDir()
}

// ConfigFile returns the original configuration file location if known.
func (w *Wave) ConfigFile() string {
	return w.runtime.ConfigFile()
}

// PrivateFS returns the runtime private-assets filesystem.
func (w *Wave) PrivateFS() (fs.FS, error) {
	return w.runtime.PrivateFS()
}

// MustPrivateFS returns the private filesystem or panics if unavailable.
func (w *Wave) MustPrivateFS() fs.FS {
	return w.runtime.MustPrivateFS()
}

// PublicURL resolves one source public asset path to its built URL.
func (w *Wave) PublicURL(original string) string {
	return w.runtime.PublicURL(original)
}

// CriticalCSS returns critical CSS content when available.
func (w *Wave) CriticalCSS() template.CSS {
	return w.runtime.CriticalCSS()
}

// CriticalCSSStyleElement returns one rendered critical-css <style> element.
func (w *Wave) CriticalCSSStyleElement() template.HTML {
	return w.runtime.CriticalCSSStyleElement()
}

// StyleSheetLinkElement returns one rendered non-critical stylesheet <link>.
func (w *Wave) StyleSheetLinkElement() template.HTML {
	return w.runtime.StyleSheetLinkElement()
}

// RefreshScript returns one rendered dev refresh script.
func (w *Wave) RefreshScript() template.HTML {
	return w.runtime.RefreshScript()
}

// MustStaticMiddleware returns middleware that serves static public assets and
// delegates all other requests to next.
func (w *Wave) MustStaticMiddleware(
	immutable bool,
) func(http.Handler) http.Handler {
	return w.runtime.MustStaticMiddleware(immutable)
}
