// Package wave2 is the app-facing runtime API for Wave2 applications.
//
// This package intentionally excludes build/dev orchestration concerns so
// production applications keep a lean runtime dependency surface.
package wave2

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// RuntimeSurface defines the runtime-only operational contract that backs the
// app-facing Wave API.
//
// This keeps wave2 runtime-facing while allowing runtime internals to evolve
// behind an explicit interface boundary.
type RuntimeSurface interface {
	Logger() *slog.Logger
	IsDev() bool
	MustGetPort() int
	SetModeToDev()
	PublicPathPrefix() string
	DistDir() string
	PrivateStaticDir() string
	ViteManifestLocation() string
	StaticPrivateOutDir() string
	StaticPublicOutDir() string
	PrivateFS() (fs.FS, error)
	MustPrivateFS() fs.FS
	PublicURL(original string) string
	CriticalCSS() template.CSS
	CriticalCSSStyleElement() template.HTML
	StyleSheetLinkElement() template.HTML
	RefreshScript() template.HTML
	MustStaticMiddleware(immutable bool) func(http.Handler) http.Handler
}

// Config configures Wave2 runtime construction.
type Config struct {
	// Required filesystem root used for config discovery/read operations.
	FS fs.FS

	// Required config path relative to Config.FS root.
	ConfigPath string

	// Optional logger used by runtime consumers.
	Logger *slog.Logger

	// Optional runtime operational adapter used for runtime-backed behavior.
	RuntimeSurface RuntimeSurface

	// Optional raw config JSON for runtime consumers.
	RawConfigJSON []byte
}

// Wave is the app-facing runtime surface.
type Wave struct {
	config_fs       fs.FS
	config_path     string
	logger          *slog.Logger
	runtime_surface RuntimeSurface
	raw_config_json []byte
}

// New constructs one Wave2 runtime instance.
func New(cfg Config) *Wave {
	if cfg.FS == nil {
		panic("wave2.New: FS is required")
	}
	normalized_config_path, err := normalize_config_path_for_fs(
		cfg.ConfigPath,
	)
	if err != nil {
		panic("wave2.New: " + err.Error())
	}
	return &Wave{
		config_fs:       cfg.FS,
		config_path:     normalized_config_path,
		logger:          cfg.Logger,
		runtime_surface: cfg.RuntimeSurface,
		raw_config_json: cfg.RawConfigJSON,
	}
}

// ConfigFS returns the configured filesystem root used by this runtime.
func (w *Wave) ConfigFS() fs.FS {
	if w == nil {
		return nil
	}
	return w.config_fs
}

// ConfigFile returns the normalized config path relative to ConfigFS root.
func (w *Wave) ConfigFile() string {
	if w == nil {
		return ""
	}
	return w.config_path
}

// Logger returns the configured logger.
func (w *Wave) Logger() *slog.Logger {
	if w == nil {
		return nil
	}
	if w.logger == nil && w.runtime_surface != nil {
		return w.runtime_surface.Logger()
	}
	return w.logger
}

// SetLogger replaces the runtime logger.
func (w *Wave) SetLogger(logger *slog.Logger) {
	if w == nil {
		return
	}
	w.logger = logger
}

// RawConfigJSON returns the raw config JSON.
func (w *Wave) RawConfigJSON() []byte {
	if w == nil {
		return nil
	}
	return w.raw_config_json
}

// RuntimeSurface returns the configured runtime adapter.
func (w *Wave) RuntimeSurface() RuntimeSurface {
	if w == nil {
		return nil
	}
	return w.runtime_surface
}

// IsDev returns whether this runtime is in development mode.
func (w *Wave) IsDev() bool {
	return w.must_runtime_surface("IsDev").IsDev()
}

// MustGetPort returns the resolved application runtime port.
func (w *Wave) MustGetPort() int {
	return w.must_runtime_surface("MustGetPort").MustGetPort()
}

// SetModeToDev sets runtime mode to development.
func (w *Wave) SetModeToDev() {
	w.must_runtime_surface("SetModeToDev").SetModeToDev()
}

// PublicPathPrefix returns the normalized configured public path prefix.
func (w *Wave) PublicPathPrefix() string {
	return w.must_runtime_surface("PublicPathPrefix").PublicPathPrefix()
}

// DistDir returns the configured build output directory root.
func (w *Wave) DistDir() string {
	return w.must_runtime_surface("DistDir").DistDir()
}

// PrivateStaticDir returns the source directory for private static assets.
func (w *Wave) PrivateStaticDir() string {
	return w.must_runtime_surface("PrivateStaticDir").PrivateStaticDir()
}

// ViteManifestLocation returns the expected path of the Vite manifest in build output.
func (w *Wave) ViteManifestLocation() string {
	return w.must_runtime_surface("ViteManifestLocation").
		ViteManifestLocation()
}

// StaticPrivateOutDir returns the private static output directory.
func (w *Wave) StaticPrivateOutDir() string {
	return w.must_runtime_surface("StaticPrivateOutDir").
		StaticPrivateOutDir()
}

// StaticPublicOutDir returns the public static output directory.
func (w *Wave) StaticPublicOutDir() string {
	return w.must_runtime_surface("StaticPublicOutDir").
		StaticPublicOutDir()
}

// PrivateFS returns the runtime private-assets filesystem.
func (w *Wave) PrivateFS() (fs.FS, error) {
	return w.must_runtime_surface("PrivateFS").PrivateFS()
}

// MustPrivateFS returns the private filesystem or panics if unavailable.
func (w *Wave) MustPrivateFS() fs.FS {
	return w.must_runtime_surface("MustPrivateFS").MustPrivateFS()
}

// PublicURL resolves one source public asset path to its built URL.
func (w *Wave) PublicURL(original string) string {
	return w.must_runtime_surface("PublicURL").PublicURL(original)
}

// CriticalCSS returns critical CSS content when available.
func (w *Wave) CriticalCSS() template.CSS {
	return w.must_runtime_surface("CriticalCSS").CriticalCSS()
}

// CriticalCSSStyleElement returns one rendered critical-css <style> element.
func (w *Wave) CriticalCSSStyleElement() template.HTML {
	return w.must_runtime_surface("CriticalCSSStyleElement").
		CriticalCSSStyleElement()
}

// StyleSheetLinkElement returns one rendered non-critical stylesheet <link>.
func (w *Wave) StyleSheetLinkElement() template.HTML {
	return w.must_runtime_surface("StyleSheetLinkElement").
		StyleSheetLinkElement()
}

// RefreshScript returns one rendered dev refresh script.
func (w *Wave) RefreshScript() template.HTML {
	return w.must_runtime_surface("RefreshScript").RefreshScript()
}

// MustStaticMiddleware returns middleware that serves static public assets and
// delegates all other requests to next.
func (w *Wave) MustStaticMiddleware(
	immutable bool,
) func(http.Handler) http.Handler {
	return w.must_runtime_surface("MustStaticMiddleware").
		MustStaticMiddleware(immutable)
}

func (w *Wave) must_runtime_surface(method_name string) RuntimeSurface {
	if w == nil {
		panic("wave2." + method_name + ": Wave is required")
	}
	if w.runtime_surface == nil {
		panic(
			"wave2." + method_name +
				": Config.RuntimeSurface is required for runtime-backed behavior",
		)
	}
	return w.runtime_surface
}

func normalize_config_path_for_fs(config_path string) (string, error) {
	trimmed_config_path := strings.TrimSpace(config_path)
	if trimmed_config_path == "" {
		return "", fmt.Errorf("ConfigPath is required")
	}
	normalized_config_path := path.Clean(filepath.ToSlash(trimmed_config_path))
	if normalized_config_path == "." {
		return "", fmt.Errorf("ConfigPath is required")
	}
	if strings.HasPrefix(normalized_config_path, "/") {
		return "", fmt.Errorf(
			"ConfigPath must be relative to FS root: %q",
			config_path,
		)
	}
	if normalized_config_path == ".." ||
		strings.HasPrefix(normalized_config_path, "../") {
		return "", fmt.Errorf(
			"ConfigPath must not escape FS root: %q",
			config_path,
		)
	}
	return normalized_config_path, nil
}
