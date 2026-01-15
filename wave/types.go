// Package wave provides runtime services for Wave applications.
// Build-time and dev-time functionality is in the wave/tooling subpackage.
package wave

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/lab/jsonschema"
)

// Path segment constants
const (
	segStatic   = "static"
	segAssets   = "assets"
	segPublic   = "public"
	segPrivate  = "private"
	segInternal = "internal"
)

// File name constants
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

// Public constants
const (
	PrehashedDirname = "prehashed"
	NohashDirname    = "__nohash"

	HashedOutputPrefix           = "vorma_out_"
	HashedOutputPrefixNoTrailing = "vorma_out"

	NormalCSSBaseName    = "vorma_internal_normal.css"
	NormalCSSGlobPattern = HashedOutputPrefix + "vorma_internal_normal_*.css"

	GeneratedTSFileName   = "index.ts"
	PublicFileMapTSName   = "filemap.ts"
	PublicFileMapJSONName = "filemap.json"
	FileMapJSGlobPattern  = HashedOutputPrefix + "vorma_internal_public_filemap_*.js"
)

// Timing represents when an OnChangeHook runs relative to Wave's rebuild process
type Timing string

const (
	OnChangeStrategyPre              Timing = "pre"
	OnChangeStrategyPost             Timing = "post"
	OnChangeStrategyConcurrent       Timing = "concurrent"
	OnChangeStrategyConcurrentNoWait Timing = "concurrent-no-wait"
)

// RelPaths provides fs.FS-relative paths (no leading slash, forward slashes).
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
func (relPaths) PublicFileMapTSName() string   { return PublicFileMapTSName }
func (relPaths) PublicFileMapJSONName() string { return PublicFileMapJSONName }

// DistLayout provides computed paths for the dist directory structure.
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

// ParsedConfig is the parsed and validated wave.config.json
type ParsedConfig struct {
	Core  *CoreConfig  `json:"Core"`
	Vite  *ViteConfig  `json:"Vite,omitempty"`
	Watch *WatchConfig `json:"Watch,omitempty"`

	Dist DistLayout `json:"-"`

	FrameworkWatchPatterns       []WatchedFile               `json:"-"`
	FrameworkIgnoredPatterns     []string                    `json:"-"`
	FrameworkPublicFileMapOutDir string                      `json:"-"`
	FrameworkSchemaExtensions    map[string]jsonschema.Entry `json:"-"`
	FrameworkDevBuildHook        string                      `json:"-"`
	FrameworkProdBuildHook       string                      `json:"-"`
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
	SortedHooks                        *SortedHooks   `json:"-"`
}

type SortedHooks struct {
	Pre              []OnChangeHook
	Concurrent       []OnChangeHook
	ConcurrentNoWait []OnChangeHook
	Post             []OnChangeHook
}

func (wf *WatchedFile) Sort() {
	if wf.SortedHooks != nil {
		return
	}
	wf.SortedHooks = &SortedHooks{}
	for _, h := range wf.OnChangeHooks {
		switch h.Timing {
		case OnChangeStrategyPost:
			wf.SortedHooks.Post = append(wf.SortedHooks.Post, h)
		case OnChangeStrategyConcurrent:
			wf.SortedHooks.Concurrent = append(wf.SortedHooks.Concurrent, h)
		case OnChangeStrategyConcurrentNoWait:
			wf.SortedHooks.ConcurrentNoWait = append(wf.SortedHooks.ConcurrentNoWait, h)
		default:
			wf.SortedHooks.Pre = append(wf.SortedHooks.Pre, h)
		}
	}
}

// HookContext provides context to callbacks during file change handling.
type HookContext struct {
	// FilePath is the absolute path of the changed file.
	FilePath string
	// AppStoppedForBatch is true when the app has been stopped as part of batch
	// processing (e.g., a Go file changed in the same batch). When true, HTTP
	// endpoints on the running app cannot be called.
	AppStoppedForBatch bool
}

// RefreshAction specifies what Wave should do after a callback completes.
// Multiple RefreshActions from different hooks are merged with OR semantics.
type RefreshAction struct {
	// ReloadBrowser triggers a browser reload via WebSocket.
	// Ignored if TriggerRestart is true.
	ReloadBrowser bool
	// WaitForApp polls the app's healthcheck before reloading the browser.
	// Ignored if TriggerRestart is true.
	WaitForApp bool
	// WaitForVite waits for Vite dev server to be ready before reloading.
	// Ignored if TriggerRestart is true.
	WaitForVite bool
	// TriggerRestart causes Wave to restart the app process.
	// When true, ReloadBrowser/WaitForApp/WaitForVite are ignored.
	TriggerRestart bool
	// RecompileGo recompiles the Go binary before restart.
	// Only relevant when TriggerRestart is true.
	RecompileGo bool
}

// Merge combines two RefreshActions with OR semantics.
// TriggerRestart takes precedence over browser reload.
func (r RefreshAction) Merge(other RefreshAction) RefreshAction {
	return RefreshAction{
		ReloadBrowser:  r.ReloadBrowser || other.ReloadBrowser,
		WaitForApp:     r.WaitForApp || other.WaitForApp,
		WaitForVite:    r.WaitForVite || other.WaitForVite,
		TriggerRestart: r.TriggerRestart || other.TriggerRestart,
		RecompileGo:    r.RecompileGo || other.RecompileGo,
	}
}

// IsZero returns true if this RefreshAction specifies no action.
func (r RefreshAction) IsZero() bool {
	return !r.ReloadBrowser && !r.WaitForApp && !r.WaitForVite && !r.TriggerRestart && !r.RecompileGo
}

// OnChangeHook defines an action to run when a watched file changes.
type OnChangeHook struct {
	// Cmd is a shell command to run. Can be any shell command or "DevBuildHook"
	// to run the configured dev build hook.
	Cmd string `json:"Cmd,omitempty"`
	// Timing controls when the hook runs relative to Wave's rebuild process.
	Timing Timing `json:"Timing,omitempty"`
	// Exclude contains glob patterns for files to exclude from triggering this hook.
	Exclude []string `json:"Exclude,omitempty"`
	// Callback is a Go function to run. Framework use only (not JSON-configurable).
	// If the callback returns a non-nil RefreshAction, it controls what Wave does
	// after all hooks complete. Multiple RefreshActions are merged with OR semantics.
	Callback func(*HookContext) (*RefreshAction, error) `json:"-"`
}

type FileMap map[string]FileVal

type FileVal struct {
	DistName    string
	ContentHash string
	IsPrehashed bool
}

func (fm FileMap) Lookup(original, prefix string) (url string, found bool) {
	clean := strings.TrimPrefix(path.Clean(original), "/")
	if entry, ok := fm[clean]; ok {
		return matcher.EnsureLeadingSlash(path.Join(prefix, entry.DistName)), true
	}
	return matcher.EnsureLeadingSlash(path.Join(prefix, original)), false
}

func (c *ParsedConfig) PublicPathPrefix() string {
	p := c.Core.PublicPathPrefix
	if p == "" || p == "/" {
		return "/"
	}
	return matcher.EnsureLeadingAndTrailingSlash(p)
}

func (c *ParsedConfig) ViteManifestPath() string {
	return filepath.Join(c.Dist.StaticPrivate(), HashedOutputPrefixNoTrailing, "vorma_vite_manifest.json")
}

func (c *ParsedConfig) WatchRoot() string {
	if c.Watch != nil && c.Watch.WatchRoot != "" {
		return filepath.Clean(c.Watch.WatchRoot)
	}
	return "."
}

func (c *ParsedConfig) HealthcheckEndpoint() string {
	if c.Watch != nil && c.Watch.HealthcheckEndpoint != "" {
		return c.Watch.HealthcheckEndpoint
	}
	return "/"
}

func (c *ParsedConfig) UsingBrowser() bool { return !c.Core.ServerOnlyMode }
func (c *ParsedConfig) UsingVite() bool    { return c.Vite != nil }

func (c *ParsedConfig) CriticalCSSEntry() string {
	if c.Core.CSSEntryFiles.Critical == "" {
		return ""
	}
	return filepath.Clean(c.Core.CSSEntryFiles.Critical)
}

func (c *ParsedConfig) NonCriticalCSSEntry() string {
	if c.Core.CSSEntryFiles.NonCritical == "" {
		return ""
	}
	return filepath.Clean(c.Core.CSSEntryFiles.NonCritical)
}
