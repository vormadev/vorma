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
	configFS       fs.FS
	configPath     string
	logger         *slog.Logger
	runtimeSurface RuntimeSurface
	rawConfigJSON  []byte
}

// New constructs one Wave2 runtime instance.
func New(config Config) *Wave {
	if config.FS == nil {
		panic("wave2.New: FS is required")
	}
	normalizedConfigPath, normalizeConfigPathError := normalizeConfigPathForFS(
		config.ConfigPath,
	)
	if normalizeConfigPathError != nil {
		panic("wave2.New: " + normalizeConfigPathError.Error())
	}
	return &Wave{
		configFS:       config.FS,
		configPath:     normalizedConfigPath,
		logger:         config.Logger,
		runtimeSurface: config.RuntimeSurface,
		rawConfigJSON:  config.RawConfigJSON,
	}
}

// ConfigFS returns the configured filesystem root used by this runtime.
func (waveRuntime *Wave) ConfigFS() fs.FS {
	if waveRuntime == nil {
		return nil
	}
	return waveRuntime.configFS
}

// ConfigFile returns the normalized config path relative to ConfigFS root.
func (waveRuntime *Wave) ConfigFile() string {
	if waveRuntime == nil {
		return ""
	}
	return waveRuntime.configPath
}

// Logger returns the configured logger.
func (waveRuntime *Wave) Logger() *slog.Logger {
	if waveRuntime == nil {
		return nil
	}
	if waveRuntime.logger == nil && waveRuntime.runtimeSurface != nil {
		return waveRuntime.runtimeSurface.Logger()
	}
	return waveRuntime.logger
}

// SetLogger replaces the runtime logger.
func (waveRuntime *Wave) SetLogger(logger *slog.Logger) {
	if waveRuntime == nil {
		return
	}
	waveRuntime.logger = logger
}

// RawConfigJSON returns the raw config JSON.
func (waveRuntime *Wave) RawConfigJSON() []byte {
	if waveRuntime == nil {
		return nil
	}
	return waveRuntime.rawConfigJSON
}

// RuntimeSurface returns the configured runtime adapter.
func (waveRuntime *Wave) RuntimeSurface() RuntimeSurface {
	if waveRuntime == nil {
		return nil
	}
	return waveRuntime.runtimeSurface
}

// IsDev returns whether this runtime is in development mode.
func (waveRuntime *Wave) IsDev() bool {
	return waveRuntime.mustRuntimeSurface("IsDev").IsDev()
}

// MustGetPort returns the resolved application runtime port.
func (waveRuntime *Wave) MustGetPort() int {
	return waveRuntime.mustRuntimeSurface("MustGetPort").MustGetPort()
}

// SetModeToDev sets runtime mode to development.
func (waveRuntime *Wave) SetModeToDev() {
	waveRuntime.mustRuntimeSurface("SetModeToDev").SetModeToDev()
}

// PublicPathPrefix returns the normalized configured public path prefix.
func (waveRuntime *Wave) PublicPathPrefix() string {
	return waveRuntime.mustRuntimeSurface("PublicPathPrefix").PublicPathPrefix()
}

// DistDir returns the configured build output directory root.
func (waveRuntime *Wave) DistDir() string {
	return waveRuntime.mustRuntimeSurface("DistDir").DistDir()
}

// PrivateStaticDir returns the source directory for private static assets.
func (waveRuntime *Wave) PrivateStaticDir() string {
	return waveRuntime.mustRuntimeSurface("PrivateStaticDir").PrivateStaticDir()
}

// ViteManifestLocation returns the expected path of the Vite manifest in build output.
func (waveRuntime *Wave) ViteManifestLocation() string {
	return waveRuntime.mustRuntimeSurface("ViteManifestLocation").
		ViteManifestLocation()
}

// StaticPrivateOutDir returns the private static output directory.
func (waveRuntime *Wave) StaticPrivateOutDir() string {
	return waveRuntime.mustRuntimeSurface("StaticPrivateOutDir").
		StaticPrivateOutDir()
}

// StaticPublicOutDir returns the public static output directory.
func (waveRuntime *Wave) StaticPublicOutDir() string {
	return waveRuntime.mustRuntimeSurface("StaticPublicOutDir").
		StaticPublicOutDir()
}

// PrivateFS returns the runtime private-assets filesystem.
func (waveRuntime *Wave) PrivateFS() (fs.FS, error) {
	return waveRuntime.mustRuntimeSurface("PrivateFS").PrivateFS()
}

// MustPrivateFS returns the private filesystem or panics if unavailable.
func (waveRuntime *Wave) MustPrivateFS() fs.FS {
	return waveRuntime.mustRuntimeSurface("MustPrivateFS").MustPrivateFS()
}

// PublicURL resolves one source public asset path to its built URL.
func (waveRuntime *Wave) PublicURL(original string) string {
	return waveRuntime.mustRuntimeSurface("PublicURL").PublicURL(original)
}

// CriticalCSS returns critical CSS content when available.
func (waveRuntime *Wave) CriticalCSS() template.CSS {
	return waveRuntime.mustRuntimeSurface("CriticalCSS").CriticalCSS()
}

// CriticalCSSStyleElement returns one rendered critical-css <style> element.
func (waveRuntime *Wave) CriticalCSSStyleElement() template.HTML {
	return waveRuntime.mustRuntimeSurface("CriticalCSSStyleElement").
		CriticalCSSStyleElement()
}

// StyleSheetLinkElement returns one rendered non-critical stylesheet <link>.
func (waveRuntime *Wave) StyleSheetLinkElement() template.HTML {
	return waveRuntime.mustRuntimeSurface("StyleSheetLinkElement").
		StyleSheetLinkElement()
}

// RefreshScript returns one rendered dev refresh script.
func (waveRuntime *Wave) RefreshScript() template.HTML {
	return waveRuntime.mustRuntimeSurface("RefreshScript").RefreshScript()
}

// MustStaticMiddleware returns middleware that serves static public assets and
// delegates all other requests to next.
func (waveRuntime *Wave) MustStaticMiddleware(
	immutable bool,
) func(http.Handler) http.Handler {
	return waveRuntime.mustRuntimeSurface("MustStaticMiddleware").
		MustStaticMiddleware(immutable)
}

func (waveRuntime *Wave) mustRuntimeSurface(methodName string) RuntimeSurface {
	if waveRuntime == nil {
		panic("wave2." + methodName + ": Wave is required")
	}
	if waveRuntime.runtimeSurface == nil {
		panic(
			"wave2." + methodName +
				": Config.RuntimeSurface is required for runtime-backed behavior",
		)
	}
	return waveRuntime.runtimeSurface
}

func normalizeConfigPathForFS(configPath string) (string, error) {
	trimmedConfigPath := strings.TrimSpace(configPath)
	if trimmedConfigPath == "" {
		return "", fmt.Errorf("ConfigPath is required")
	}
	normalizedConfigPath := path.Clean(filepath.ToSlash(trimmedConfigPath))
	if normalizedConfigPath == "." {
		return "", fmt.Errorf("ConfigPath is required")
	}
	if strings.HasPrefix(normalizedConfigPath, "/") {
		return "", fmt.Errorf(
			"ConfigPath must be relative to FS root: %q",
			configPath,
		)
	}
	if normalizedConfigPath == ".." ||
		strings.HasPrefix(normalizedConfigPath, "../") {
		return "", fmt.Errorf(
			"ConfigPath must not escape FS root: %q",
			configPath,
		)
	}
	return normalizedConfigPath, nil
}
