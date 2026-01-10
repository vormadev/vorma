// Package config provides shared configuration types and parsing for Wave.
// This package has no dependencies on other internal packages.
package config

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/lab/jsonschema"
)

// Path segment constants - private, used only by DistLayout and RelPaths methods
const (
	segStatic   = "static"
	segAssets   = "assets"
	segPublic   = "public"
	segPrivate  = "private"
	segInternal = "internal"
)

// File name constants - private, used only by DistLayout and RelPaths methods
const (
	fileBinary        = "main"
	fileBinaryWindows = "main.exe"
	fileKeep          = ".keep"
	fileCriticalCSS   = "critical.css"
	fileNormalCSSRef  = "normal_css_file_ref.txt"
	filePublicMapRef  = "public_file_map_file_ref.txt"
	filePublicMapGob  = "public_filemap.gob"
	filePrivateMapGob = "private_filemap.gob"
	filePublicMapJS   = "vorma_internal_public_filemap.js"
)

// Public constants - referenced by external packages
const (
	PrehashedDirname = "prehashed"
	NohashDirname    = "__nohash"

	// HashedOutputPrefix is the prefix for all Wave-generated hashed files
	HashedOutputPrefix           = "vorma_out_"
	HashedOutputPrefixNoTrailing = "vorma_out"

	// CSS output
	NormalCSSBaseName    = "vorma_internal_normal.css"
	NormalCSSGlobPattern = HashedOutputPrefix + "vorma_internal_normal_*.css"

	// Files
	FilePublicMapTS      = "filemap.ts"
	FileMapJSGlobPattern = HashedOutputPrefix + "vorma_internal_public_filemap_*.js"
)

// RelPaths provides fs.FS-relative paths (no leading slash, forward slashes).
// Used by runtime package when reading from embedded/disk fs.FS.
var RelPaths = relPaths{}

type relPaths struct{}

func (relPaths) Internal() string              { return segInternal }
func (relPaths) AssetsPublic() string          { return segAssets + "/" + segPublic }
func (relPaths) AssetsPrivate() string         { return segAssets + "/" + segPrivate }
func (relPaths) CriticalCSS() string           { return segInternal + "/" + fileCriticalCSS }
func (relPaths) NormalCSSRef() string          { return segInternal + "/" + fileNormalCSSRef }
func (relPaths) PublicFileMapRef() string      { return segInternal + "/" + filePublicMapRef }
func (relPaths) PublicFileMapGob() string      { return segInternal + "/" + filePublicMapGob }
func (relPaths) PublicFileMapGobName() string  { return filePublicMapGob }
func (relPaths) PrivateFileMapGobName() string { return filePrivateMapGob }
func (relPaths) PublicFileMapJSName() string   { return filePublicMapJS }
func (relPaths) PublicFileMapTSName() string   { return FilePublicMapTS }

// DistLayout provides computed paths for the dist directory structure.
// All methods return absolute paths using filepath (OS-native separators).
type DistLayout struct {
	Root string
}

func (d DistLayout) Binary() string {
	name := fileBinary
	if runtime.GOOS == "windows" {
		name = fileBinaryWindows
	}
	return filepath.Join(d.Root, name)
}

func (d DistLayout) Static() string            { return filepath.Join(d.Root, segStatic) }
func (d DistLayout) StaticAssets() string      { return filepath.Join(d.Static(), segAssets) }
func (d DistLayout) StaticPublic() string      { return filepath.Join(d.StaticAssets(), segPublic) }
func (d DistLayout) StaticPrivate() string     { return filepath.Join(d.StaticAssets(), segPrivate) }
func (d DistLayout) Internal() string          { return filepath.Join(d.Static(), segInternal) }
func (d DistLayout) CriticalCSS() string       { return filepath.Join(d.Internal(), fileCriticalCSS) }
func (d DistLayout) NormalCSSRef() string      { return filepath.Join(d.Internal(), fileNormalCSSRef) }
func (d DistLayout) PublicFileMapRef() string  { return filepath.Join(d.Internal(), filePublicMapRef) }
func (d DistLayout) PublicFileMapGob() string  { return filepath.Join(d.Internal(), filePublicMapGob) }
func (d DistLayout) PrivateFileMapGob() string { return filepath.Join(d.Internal(), filePrivateMapGob) }
func (d DistLayout) KeepFile() string          { return filepath.Join(d.Static(), fileKeep) }

// Config is the parsed and validated wave.config.json
type Config struct {
	Core  *CoreConfig  `json:"Core"`
	Vite  *ViteConfig  `json:"Vite,omitempty"`
	Watch *WatchConfig `json:"Watch,omitempty"`

	// Computed paths
	Dist DistLayout `json:"-"`

	// Framework hooks (Runtime only, not serialized)
	// These allow the framework (Driver) to configure the Engine without
	// the Engine knowing about the Framework's specific config structs.
	FrameworkWatchPatterns       []WatchedFile               `json:"-"`
	FrameworkIgnoredPatterns     []string                    `json:"-"`
	FrameworkPublicFileMapOutDir string                      `json:"-"`
	FrameworkSchemaExtensions    map[string]jsonschema.Entry `json:"-"`
}

type CoreConfig struct {
	ConfigLocation   string          `json:"ConfigLocation,omitempty"`
	DevBuildHook     string          `json:"DevBuildHook,omitempty"`
	ProdBuildHook    string          `json:"ProdBuildHook,omitempty"`
	MainAppEntry     string          `json:"MainAppEntry"`
	DistDir          string          `json:"DistDir"`
	StaticAssetDirs  StaticAssetDirs `json:"StaticAssetDirs"`
	CSSEntryFiles    CSSEntryFiles   `json:"CSSEntryFiles,omitempty"`
	PublicPathPrefix string          `json:"PublicPathPrefix,omitempty"`
	ServerOnlyMode   bool            `json:"ServerOnlyMode,omitempty"`
}

type StaticAssetDirs struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

type CSSEntryFiles struct {
	Critical    string `json:"Critical,omitempty"`
	NonCritical string `json:"NonCritical,omitempty"`
}

type ViteConfig struct {
	JSPackageManagerBaseCmd string `json:"JSPackageManagerBaseCmd"`
	JSPackageManagerCmdDir  string `json:"JSPackageManagerCmdDir,omitempty"`
	DefaultPort             int    `json:"DefaultPort,omitempty"`
	ViteConfigFile          string `json:"ViteConfigFile,omitempty"`
}

type WatchConfig struct {
	WatchRoot           string        `json:"WatchRoot,omitempty"`
	HealthcheckEndpoint string        `json:"HealthcheckEndpoint,omitempty"`
	Include             []WatchedFile `json:"Include,omitempty"`
	Exclude             struct {
		Dirs  []string `json:"Dirs,omitempty"`
		Files []string `json:"Files,omitempty"`
	} `json:"Exclude,omitempty"`
}

type WatchedFile struct {
	Pattern                            string         `json:"Pattern"`
	OnChangeHooks                      []OnChangeHook `json:"OnChangeHooks,omitempty"`
	RecompileGoBinary                  bool           `json:"RecompileGoBinary,omitempty"`
	RestartApp                         bool           `json:"RestartApp,omitempty"`
	OnlyRunClientDefinedRevalidateFunc bool           `json:"OnlyRunClientDefinedRevalidateFunc,omitempty"`
	RunOnChangeOnly                    bool           `json:"RunOnChangeOnly,omitempty"`
	SkipRebuildingNotification         bool           `json:"SkipRebuildingNotification,omitempty"`
	TreatAsNonGo                       bool           `json:"TreatAsNonGo,omitempty"`

	// SortedHooks holds pre-sorted hooks by timing.
	// Populated during watcher setup.
	SortedHooks *SortedHooks `json:"-"`
}

// SortedHooks holds hooks organized by timing for efficient access.
type SortedHooks struct {
	Pre              []OnChangeHook
	Concurrent       []OnChangeHook
	ConcurrentNoWait []OnChangeHook
	Post             []OnChangeHook
}

// Sort categorizes hooks by timing and stores the result in SortedHooks.
func (wf *WatchedFile) Sort() {
	if wf.SortedHooks != nil {
		return
	}
	wf.SortedHooks = &SortedHooks{}
	for _, h := range wf.OnChangeHooks {
		switch h.Timing {
		case TimingPost:
			wf.SortedHooks.Post = append(wf.SortedHooks.Post, h)
		case TimingConcurrent:
			wf.SortedHooks.Concurrent = append(wf.SortedHooks.Concurrent, h)
		case TimingConcurrentNoWait:
			wf.SortedHooks.ConcurrentNoWait = append(wf.SortedHooks.ConcurrentNoWait, h)
		default:
			wf.SortedHooks.Pre = append(wf.SortedHooks.Pre, h)
		}
	}
}

// OnChangeStrategy defines a declarative strategy for handling file changes.
// This allows frameworks to specify complex behaviors without Wave needing
// framework-specific knowledge.
type OnChangeStrategy struct {
	// HttpEndpoint is the URL to call on the running app (e.g., "/__vorma/rebuild-routes").
	// If the call fails, FallbackAction is executed.
	HttpEndpoint string `json:"HttpEndpoint,omitempty"`

	// SkipDevHook skips running the DevBuildHook for this change.
	SkipDevHook bool `json:"SkipDevHook,omitempty"`

	// SkipGoCompile skips Go binary recompilation for this change.
	SkipGoCompile bool `json:"SkipGoCompile,omitempty"`

	// WaitForApp waits for the app's healthcheck before reloading the browser.
	WaitForApp bool `json:"WaitForApp,omitempty"`

	// WaitForVite waits for Vite dev server before reloading the browser.
	WaitForVite bool `json:"WaitForVite,omitempty"`

	// ReloadBrowser triggers a browser reload after successful execution.
	ReloadBrowser bool `json:"ReloadBrowser,omitempty"`

	// FallbackAction specifies what to do if HttpEndpoint fails.
	// Valid values: "restart" (full restart with Go), "restart-no-go" (restart without Go recompile), "none"
	FallbackAction string `json:"FallbackAction,omitempty"`
}

// FallbackAction constants
const (
	FallbackRestart     = "restart"
	FallbackRestartNoGo = "restart-no-go"
	FallbackNone        = "none"
)

type OnChangeHook struct {
	// Cmd is a shell command to run.
	// Use "DevBuildHook" to run the configured dev build hook.
	Cmd string `json:"Cmd,omitempty"`

	// Strategy is an alternative to Cmd that specifies a declarative behavior.
	// If Strategy is set, Cmd is ignored.
	Strategy *OnChangeStrategy `json:"Strategy,omitempty"`

	Timing  Timing   `json:"Timing,omitempty"`
	Exclude []string `json:"Exclude,omitempty"`
}

// HasStrategy returns true if this hook uses a strategy rather than a command.
func (h *OnChangeHook) HasStrategy() bool {
	return h.Strategy != nil
}

type Timing string

const (
	TimingPre              Timing = "pre"
	TimingPost             Timing = "post"
	TimingConcurrent       Timing = "concurrent"
	TimingConcurrentNoWait Timing = "concurrent-no-wait"
)

// FileMap types
type FileMap map[string]FileVal

type FileVal struct {
	DistName    string
	ContentHash string
	IsPrehashed bool
}

// Lookup returns the hashed URL for an original path and whether it was found.
// If not found, returns the fallback URL.
func (fm FileMap) Lookup(original, prefix string) (url string, found bool) {
	clean := strings.TrimPrefix(path.Clean(original), "/")
	if entry, ok := fm[clean]; ok {
		return matcher.EnsureLeadingSlash(path.Join(prefix, entry.DistName)), true
	}
	return matcher.EnsureLeadingSlash(path.Join(prefix, original)), false
}

// Config methods

func (c *Config) PublicPathPrefix() string {
	p := c.Core.PublicPathPrefix
	if p == "" || p == "/" {
		return "/"
	}
	return matcher.EnsureLeadingAndTrailingSlash(p)
}

func (c *Config) ViteManifestPath() string {
	return filepath.Join(c.Dist.StaticPrivate(), HashedOutputPrefixNoTrailing, "vorma_vite_manifest.json")
}

func (c *Config) WatchRoot() string {
	if c.Watch != nil && c.Watch.WatchRoot != "" {
		return filepath.Clean(c.Watch.WatchRoot)
	}
	return "."
}

func (c *Config) HealthcheckEndpoint() string {
	if c.Watch != nil && c.Watch.HealthcheckEndpoint != "" {
		return c.Watch.HealthcheckEndpoint
	}
	return "/"
}

func (c *Config) UsingBrowser() bool { return !c.Core.ServerOnlyMode }
func (c *Config) UsingVite() bool    { return c.Vite != nil }

func (c *Config) CriticalCSSEntry() string {
	if c.Core.CSSEntryFiles.Critical == "" {
		return ""
	}
	return filepath.Clean(c.Core.CSSEntryFiles.Critical)
}

func (c *Config) NonCriticalCSSEntry() string {
	if c.Core.CSSEntryFiles.NonCritical == "" {
		return ""
	}
	return filepath.Clean(c.Core.CSSEntryFiles.NonCritical)
}
