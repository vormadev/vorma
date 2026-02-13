package wave

// RawConfigJSON returns the raw bytes of the configuration file.
func (w *Wave) RawConfigJSON() []byte {
	return append([]byte(nil), w.rawCfg...)
}

// AddFrameworkWatchPatterns adds watch patterns for use during development.
func (w *Wave) AddFrameworkWatchPatterns(patterns []WatchedFile) {
	w.cfg.FrameworkWatchPatterns = append(w.cfg.FrameworkWatchPatterns, patterns...)
}

// AddIgnoredPatterns adds glob patterns for files/directories to ignore during watching.
func (w *Wave) AddIgnoredPatterns(patterns []string) {
	w.cfg.FrameworkIgnoredPatterns = append(w.cfg.FrameworkIgnoredPatterns, patterns...)
}

// SetPublicFileMapOutDir sets the directory where Wave should write the public filemap TypeScript file.
func (w *Wave) SetPublicFileMapOutDir(dir string) {
	w.cfg.FrameworkPublicFileMapOutDir = dir
}

// GetIsDev returns true if running in development mode.
func (w *Wave) GetIsDev() bool {
	return GetIsDev()
}

// MustGetPort returns the application port.
func (w *Wave) MustGetPort() int {
	return MustGetPort()
}

// SetModeToDev sets the environment to development mode.
func (w *Wave) SetModeToDev() {
	SetModeToDev()
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
func (w *Wave) GetParsedConfig() *ParsedConfig {
	return w.cfg
}

func (w *Wave) GetConfigFile() string {
	return w.cfg.Core.ConfigLocation
}
