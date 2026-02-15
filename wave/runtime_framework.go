package wave

import (
	"context"

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
	w.cfg.FrameworkIgnoredPatterns = append(w.cfg.FrameworkIgnoredPatterns, patterns...)
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

func (w *Wave) SetBrowserRuntimeNamespace(namespace string) {
	w.cfg.FrameworkBrowserRuntimeNamespace = namespace
}

func (w *Wave) SetBrowserPublicURLResolverFunctionName(functionName string) {
	w.cfg.FrameworkBrowserPublicURLResolverFunctionName = functionName
}

func (w *Wave) SetBrowserRevalidateFunctionName(functionName string) {
	w.cfg.FrameworkBrowserRevalidateFunctionName = functionName
}

func (w *Wave) SetRefreshRebuildingOverlayElementID(elementID string) {
	w.cfg.FrameworkRefreshRebuildingOverlayElementID = elementID
}

func (w *Wave) SetCriticalCSSStyleElementID(elementID string) {
	w.cfg.FrameworkCriticalCSSStyleElementID = elementID
}

func (w *Wave) SetNonCriticalCSSLinkElementID(elementID string) {
	w.cfg.FrameworkNonCriticalCSSLinkElementID = elementID
}

// GetIsDev returns true if running in development mode.
func (w *Wave) GetIsDev() bool {
	return GetIsDev()
}

// MustGetPort returns the application port.
func (w *Wave) MustGetPort() int {
	if w == nil || w.portResolver == nil {
		return MustGetPort()
	}
	return w.portResolver.MustGetPort()
}

// SetModeToDev sets the environment to development mode.
func (w *Wave) SetModeToDev() {
	SetModeToDev()
}

func (w *Wave) SetPortResolver(portResolver *PortResolver) {
	if w == nil {
		return
	}
	if portResolver == nil {
		w.portResolver = NewPortResolver()
		return
	}
	w.portResolver = portResolver
}

func (w *Wave) GetPublicPathPrefix() string {
	return w.cfg.PublicPathPrefix()
}

func (w *Wave) GetDistDir() string {
	return w.cfg.Core.DistDir
}

func (w *Wave) GetPublicStaticDir() string {
	return w.cfg.Core.StaticAssetDirs.Public
}

func (w *Wave) GetPrivateStaticDir() string {
	return w.cfg.Core.StaticAssetDirs.Private
}

func (w *Wave) GetViteManifestLocation() string {
	return w.cfg.ViteManifestPath()
}

func (w *Wave) GetViteOutDir() string {
	return w.cfg.Dist.StaticPublic()
}

func (w *Wave) GetStaticPrivateOutDir() string {
	return w.cfg.Dist.StaticPrivate()
}

func (w *Wave) GetStaticPublicOutDir() string {
	return w.cfg.Dist.StaticPublic()
}

// GetParsedConfig returns the parsed configuration for use by tooling.
// This should only be used by build-time tooling, not at runtime.
// The returned value is a defensive snapshot and omits unstable internal-only
// runtime callback/schema fields.
func (w *Wave) GetParsedConfig() *ParsedConfig {
	return w.cfg.Clone()
}

// GetBuildtimeParsedConfig returns a defensive parsed-config snapshot for
// build/dev tooling. Unlike GetParsedConfig, this includes framework build
// callbacks and schema extensions.
func (w *Wave) GetBuildtimeParsedConfig() *ParsedConfig {
	return w.cfg.cloneForBuildtime()
}

func (w *Wave) GetConfigFile() string {
	return w.cfg.Core.ConfigLocation
}
