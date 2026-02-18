package wave

import (
	"context"

	"github.com/vormadev/vorma/internal/waveport"
	"github.com/vormadev/vorma/lab/jsonschema"
)

// RawConfigJSON returns the raw bytes of the configuration file.
func (w *Wave) RawConfigJSON() []byte {
	return append([]byte(nil), w.rawCfg...)
}

// AddFrameworkWatchPatterns adds watch patterns for use during development.
func (w *Wave) AddFrameworkWatchPatterns(patterns []WatchedFile) {
	w.cfg.FrameworkWatchPatterns = append(
		w.cfg.FrameworkWatchPatterns,
		cloneFrameworkWatchPatterns(patterns)...,
	)
}

// AddIgnoredPatterns adds glob patterns for files/directories to ignore during watching.
func (w *Wave) AddIgnoredPatterns(patterns []string) {
	w.cfg.FrameworkIgnoredPatterns = append(
		w.cfg.FrameworkIgnoredPatterns,
		patterns...)
}

// SetPublicFileMapOutDir sets the directory where Wave should write the public filemap TypeScript file.
func (w *Wave) SetPublicFileMapOutDir(dir string) {
	w.cfg.FrameworkPublicFileMapOutDir = dir
}

// RegisterFrameworkSchemaSection registers a framework-owned schema extension
// for wave.config.json generation at build time.
func (w *Wave) RegisterFrameworkSchemaSection(
	name string,
	schema jsonschema.Entry,
) {
	if w.cfg.FrameworkSchemaExtensions == nil {
		w.cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	w.cfg.FrameworkSchemaExtensions[name] = schema
}

// SetFrameworkDevBuildHookCommand sets the framework dev build hook command.
func (w *Wave) SetFrameworkDevBuildHookCommand(command string) {
	w.cfg.FrameworkDevBuildHook = command
}

// SetFrameworkProdBuildHookCommand sets the framework production build hook command.
func (w *Wave) SetFrameworkProdBuildHookCommand(command string) {
	w.cfg.FrameworkProdBuildHook = command
}

// SetFrameworkRunBuildHookRunner sets the framework build hook runner callback.
func (w *Wave) SetFrameworkRunBuildHookRunner(
	runner func(context.Context, bool) error,
) {
	w.cfg.FrameworkRunBuildHook = runner
}

// SetFrameworkPrepareGoBuildOverlay sets the framework Go build overlay callback.
func (w *Wave) SetFrameworkPrepareGoBuildOverlay(
	preparer func() (*GoBuildOverlay, error),
) {
	w.cfg.FrameworkPrepareGoBuildOverlay = preparer
}

// SetBrowserRuntimeNamespace overrides the browser runtime namespace used by
// generated client integration scripts.
func (w *Wave) SetBrowserRuntimeNamespace(namespace string) {
	w.cfg.FrameworkBrowserRuntimeNamespace = namespace
}

// SetBrowserPublicURLResolverFunctionName overrides the generated browser
// helper name used to resolve public asset URLs.
func (w *Wave) SetBrowserPublicURLResolverFunctionName(functionName string) {
	w.cfg.FrameworkBrowserPublicURLResolverFunctionName = functionName
}

// SetBrowserRevalidateFunctionName overrides the generated browser helper name
// used to trigger revalidation.
func (w *Wave) SetBrowserRevalidateFunctionName(functionName string) {
	w.cfg.FrameworkBrowserRevalidateFunctionName = functionName
}

// SetRefreshRebuildingOverlayElementID overrides the DOM element id used for
// the rebuild overlay in dev refresh flows.
func (w *Wave) SetRefreshRebuildingOverlayElementID(elementID string) {
	w.cfg.FrameworkRefreshRebuildingOverlayElementID = elementID
}

// SetCriticalCSSStyleElementID overrides the DOM element id used for injected
// critical CSS style elements.
func (w *Wave) SetCriticalCSSStyleElementID(elementID string) {
	w.cfg.FrameworkCriticalCSSStyleElementID = elementID
}

// SetNonCriticalCSSLinkElementID overrides the DOM element id used for
// injected non-critical stylesheet link elements.
func (w *Wave) SetNonCriticalCSSLinkElementID(elementID string) {
	w.cfg.FrameworkNonCriticalCSSLinkElementID = elementID
}

// IsDev returns true if running in development mode.
func (w *Wave) IsDev() bool {
	if w == nil {
		return GetIsDev()
	}
	return w.isDevMode
}

// MustGetPort returns the application runtime port.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func (w *Wave) MustGetPort() int {
	if w == nil || w.portResolver == nil {
		return MustGetPort()
	}
	return w.portResolver.MustGetPort()
}

func (w *Wave) setDevModeForInstance(isDevMode bool) {
	w.isDevMode = isDevMode
	w.portResolver = waveport.NewResolverForMode(isDevMode)
	w.initRuntimeCaches()
}

// SetModeToDev sets the environment to development mode.
func (w *Wave) SetModeToDev() {
	if w != nil {
		w.setDevModeForInstance(true)
	}
	SetModeToDev()
}

// PublicPathPrefix returns the normalized configured public path prefix.
func (w *Wave) PublicPathPrefix() string {
	return w.cfg.PublicPathPrefix()
}

// DistDir returns the configured build output directory root.
func (w *Wave) DistDir() string {
	return w.cfg.Core.DistDir
}

// PublicStaticDir returns the source directory for public static assets.
func (w *Wave) PublicStaticDir() string {
	return w.cfg.Core.StaticAssetDirs.Public
}

// PrivateStaticDir returns the source directory for private static assets.
func (w *Wave) PrivateStaticDir() string {
	return w.cfg.Core.StaticAssetDirs.Private
}

// ViteManifestLocation returns the expected path of the Vite manifest in build
// output.
func (w *Wave) ViteManifestLocation() string {
	return w.cfg.ViteManifestPath()
}

// ViteOutDir returns the directory where Vite emits public artifacts.
func (w *Wave) ViteOutDir() string {
	return w.cfg.Dist.StaticPublic()
}

// StaticPrivateOutDir returns the private static output directory.
func (w *Wave) StaticPrivateOutDir() string {
	return w.cfg.Dist.StaticPrivate()
}

// StaticPublicOutDir returns the public static output directory.
func (w *Wave) StaticPublicOutDir() string {
	return w.cfg.Dist.StaticPublic()
}

// ParsedConfig returns the parsed configuration for use by tooling.
// This should only be used by build-time tooling, not at runtime.
// The returned value is a defensive snapshot and omits unstable internal-only
// runtime callback/schema fields.
func (w *Wave) ParsedConfig() *ParsedConfig {
	return w.cfg.Clone()
}

// BuildtimeParsedConfig returns a defensive parsed-config snapshot for
// build/dev tooling. Unlike ParsedConfig, this includes framework build
// callbacks and schema extensions.
func (w *Wave) BuildtimeParsedConfig() *ParsedConfig {
	return w.cfg.cloneForBuildtime()
}

// ConfigFile returns the original configuration file location if known.
func (w *Wave) ConfigFile() string {
	return w.cfg.Core.ConfigLocation
}
