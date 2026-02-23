package wave

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/fsutil"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave/internal/wavecore"
	"github.com/vormadev/vorma/wave/internal/waveruntime"
)

// Cache Primitives

// cache holds a lazily-initialized value that is cached in prod but
// recomputed on every access in dev mode.
type cache[T any] struct {
	val       T
	err       error
	once      sync.Once
	fn        func() (T, error)
	isDevMode func() bool
}

func newCache[T any](fn func() (T, error)) *cache[T] {
	return newCacheWithModeResolver(fn, GetIsDev)
}

func newCacheWithModeResolver[T any](
	fn func() (T, error),
	isDevMode func() bool,
) *cache[T] {
	if isDevMode == nil {
		isDevMode = GetIsDev
	}
	return &cache[T]{
		fn:        fn,
		isDevMode: isDevMode,
	}
}

func (c *cache[T]) get() (T, error) {
	if c.isDevMode() {
		return c.fn()
	}
	c.once.Do(func() { c.val, c.err = c.fn() })
	return c.val, c.err
}

// cacheMap holds lazily-initialized keyed values that are cached in prod
// but recomputed on every access in dev mode.
type cacheMap[K comparable, V any] struct {
	m           sync.Map
	fn          func(K) (V, error)
	shouldCache func(V, error) bool
	isDevMode   func() bool
}

type cacheMapEntry[V any] struct {
	once sync.Once
	val  V
	err  error
}

func newCacheMap[K comparable, V any](fn func(K) (V, error)) *cacheMap[K, V] {
	return newCacheMapWithPolicyAndModeResolver(fn, nil, GetIsDev)
}

func newCacheMapWithPolicyAndModeResolver[K comparable, V any](
	fn func(K) (V, error),
	shouldCache func(V, error) bool,
	isDevMode func() bool,
) *cacheMap[K, V] {
	if shouldCache == nil {
		shouldCache = func(_ V, err error) bool {
			return err == nil
		}
	}
	if isDevMode == nil {
		isDevMode = GetIsDev
	}

	return &cacheMap[K, V]{
		fn:          fn,
		shouldCache: shouldCache,
		isDevMode:   isDevMode,
	}
}

func (c *cacheMap[K, V]) get(key K) (V, error) {
	if c.isDevMode() {
		return c.fn(key)
	}

	entryAny, _ := c.m.LoadOrStore(key, &cacheMapEntry[V]{})
	entry := entryAny.(*cacheMapEntry[V])

	entry.once.Do(func() {
		entry.val, entry.err = c.fn(key)
		if c.shouldCache != nil && !c.shouldCache(entry.val, entry.err) {
			c.m.Delete(key)
		}
	})

	if entry.err != nil {
		var zero V
		return zero, entry.err
	}

	return entry.val, nil
}

// Environment Mode and Port Resolution

const (
	envMode              = wavecore.EnvMode
	envModeDev           = wavecore.EnvModeDev
	envPort              = wavecore.EnvPort
	envPortSet           = wavecore.EnvPortSet
	envRefreshServerPort = "__WAVE_REFRESH_SERVER_PORT"
)

// GetIsDev reports whether Wave is running in development mode.
func GetIsDev() bool {
	return wavecore.GetIsDev()
}

// SetModeToDev marks the current process as development mode.
func SetModeToDev() {
	wavecore.SetModeToDev()
}

type portResolver = wavecore.Resolver

var defaultPortResolver = wavecore.NewResolver()

func newPortResolver() *portResolver {
	return wavecore.NewResolverForMode(GetIsDev())
}

// MustGetPort returns the application runtime port.
// In dev mode it resolves and caches a framework-controlled free port.
// It panics in dev mode if a free port cannot be resolved.
// It panics in non-dev mode when PORT is missing or invalid.
func MustGetPort() int {
	if defaultPortResolver == nil {
		defaultPortResolver = wavecore.NewResolver()
	}
	return defaultPortResolver.MustGetPort()
}

// getRefreshServerPort returns the active refresh-server port from environment.
// It returns 0 when unset or invalid.
func getRefreshServerPort() int {
	p, err := strconv.Atoi(os.Getenv(envRefreshServerPort))
	if err != nil || p <= 0 || p > 65535 {
		return 0
	}
	return p
}

// setRefreshServerPort writes the refresh-server port into environment.
func setRefreshServerPort(port int) {
	os.Setenv(envRefreshServerPort, strconv.Itoa(port))
}

// Public Types and Dist Layout

// Package wave provides runtime services that may be linked into production binaries.
//
// Build-time and dev-time functionality is in the wave/tooling subpackage.
// Keeping this split explicit lets applications use runtime-only code in
// production while avoiding build-tool dependencies in shipped binaries.

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
	// PrehashedDirname is the static directory segment for prehashed assets.
	PrehashedDirname = "prehashed"
	// NohashDirname is the static directory segment for assets that should not
	// be hashed.
	NohashDirname = "__nohash"

	// HashedOutputPrefix is the generated filename prefix for hashed artifacts.
	HashedOutputPrefix = "vorma_out_"
	// hashedOutputPrefixNoTrailing is the unhashed directory prefix used in
	// internal generated output paths.
	hashedOutputPrefixNoTrailing = "vorma_out"

	// GeneratedTSFileName is the default generated TypeScript file name.
	GeneratedTSFileName = "index.ts"
	// PublicFileMapTSName is the generated TypeScript public filemap name.
	PublicFileMapTSName = "filemap.ts"
	// PublicFileMapJSONName is the generated JSON public filemap name.
	PublicFileMapJSONName = "filemap.json"
	// FileMapJSGlobPattern matches hashed public filemap runtime script files.
	FileMapJSGlobPattern = HashedOutputPrefix + "vorma_internal_public_filemap_*.js"
	// FrameworkRuntimeReloadAttemptIDHeaderName is the request header carrying
	// framework runtime reload attempt identifier.
	FrameworkRuntimeReloadAttemptIDHeaderName = "X-Vorma-Reload-Attempt-Id"
	// FrameworkRuntimeReloadExpectedBuildIDHeaderName is the request header
	// carrying expected framework runtime build identifier.
	FrameworkRuntimeReloadExpectedBuildIDHeaderName = "X-Vorma-Reload-Expected-Build-Id"
	// FrameworkRuntimeReloadTriggerHeaderName is the request header carrying
	// framework runtime reload trigger identifier.
	FrameworkRuntimeReloadTriggerHeaderName = "X-Vorma-Reload-Trigger"
)

// Runtime browser integration defaults.
const (
	// defaultBrowserRuntimeNamespace is the default browser global namespace.
	defaultBrowserRuntimeNamespace = "__wave"
	// defaultBrowserPublicURLResolverFunctionName is the default browser helper
	// name for public URL resolution.
	defaultBrowserPublicURLResolverFunctionName = "getPublicURL"
	// defaultBrowserRevalidateFunctionName is the default browser helper name for
	// route revalidation.
	defaultBrowserRevalidateFunctionName = "__waveRevalidate"
	// defaultRefreshRebuildingOverlayElementID is the default DOM element id for
	// rebuild overlays.
	defaultRefreshRebuildingOverlayElementID = "wave-refreshscript-rebuilding"
	// defaultCriticalCSSStyleElementID is the default DOM id for critical CSS
	// style injection.
	defaultCriticalCSSStyleElementID = "wave-critical-css"
	// defaultNonCriticalCSSLinkElementID is the default DOM id for non-critical
	// stylesheet link injection.
	defaultNonCriticalCSSLinkElementID = "wave-normal-css"
)

// onChangeTiming represents when an OnChangeHook runs relative to Wave's
// rebuild process.
type onChangeTiming string

const (
	// Blocks build, use when build depends on hook output (e.g., code generation)
	OnChangeStrategyPre onChangeTiming = "pre"
	// Blocks reload, use when hook depends on build output (e.g., something that reads compiled artifacts)
	OnChangeStrategyPost onChangeTiming = "post"
	// Runs during build, blocks reload, saves time when hook and build are independent
	OnChangeStrategyConcurrent onChangeTiming = "concurrent"
	// Fire-and-forget, blocks nothing
	onChangeStrategyConcurrentNoWait onChangeTiming = "concurrent-no-wait"
)

// RelPaths provides fs.FS-relative paths (no leading slash, forward slashes).
var RelPaths = relPaths{}

type relPaths struct{}

// Internal returns the internal metadata path segment.
func (relPaths) Internal() string { return segInternal }

// assetsPublic returns the relative public assets path.
func (relPaths) assetsPublic() string { return segAssets + "/" + segPublic }

// assetsPrivate returns the relative private assets path.
func (relPaths) assetsPrivate() string { return segAssets + "/" + segPrivate }

// CriticalCSS returns the relative critical CSS metadata file path.
func (relPaths) CriticalCSS() string { return segInternal + "/" + fileCriticalCSS }

// NormalCSSRef returns the relative normal CSS reference metadata path.
func (relPaths) NormalCSSRef() string { return segInternal + "/" + fileNormalCSSRef }

// PublicFileMapRef returns the relative public filemap reference metadata path.
func (relPaths) PublicFileMapRef() string { return segInternal + "/" + filePublicMapRef }

// PublicFileMapGob returns the relative gob-encoded public filemap path.
func (relPaths) PublicFileMapGob() string { return segInternal + "/" + filePublicMapGob }

// publicFileMapGobName returns the public filemap gob filename.
func (relPaths) publicFileMapGobName() string { return filePublicMapGob }

// privateFileMapGobName returns the private filemap gob filename.
func (relPaths) privateFileMapGobName() string { return filePrivateMapGob }

// PublicFileMapJSName returns the runtime public filemap JavaScript filename.
func (relPaths) PublicFileMapJSName() string { return filePublicMapJS }

// PublicFileMapTSName returns the generated TypeScript public filemap filename.
func (relPaths) PublicFileMapTSName() string { return PublicFileMapTSName }

// PublicFileMapJSONName returns the generated JSON public filemap filename.
func (relPaths) PublicFileMapJSONName() string { return PublicFileMapJSONName }

// distLayout provides computed paths for the dist directory structure.
type distLayout struct {
	Root string
}

// Binary returns the platform-specific app binary path.
func (d distLayout) Binary() string {
	name := fileBinary
	if runtime.GOOS == "windows" {
		name = fileBinaryWindows
	}
	return filepath.Join(d.Root, name)
}

// Static returns the dist static directory path.
func (d distLayout) Static() string { return filepath.Join(d.Root, segStatic) }

// StaticPublic returns the dist public assets directory path.
func (d distLayout) StaticPublic() string {
	return filepath.Join(d.Static(), segAssets, segPublic)
}

// StaticPrivate returns the dist private assets directory path.
func (d distLayout) StaticPrivate() string {
	return filepath.Join(d.Static(), segAssets, segPrivate)
}

// Internal returns the dist internal metadata directory path.
func (d distLayout) Internal() string { return filepath.Join(d.Static(), segInternal) }

// CriticalCSS returns the dist critical CSS metadata file path.
func (d distLayout) CriticalCSS() string { return filepath.Join(d.Internal(), fileCriticalCSS) }

// NormalCSSRef returns the dist normal CSS reference metadata file path.
func (d distLayout) NormalCSSRef() string { return filepath.Join(d.Internal(), fileNormalCSSRef) }

// PublicFileMapRef returns the dist public filemap reference metadata path.
func (d distLayout) PublicFileMapRef() string { return filepath.Join(d.Internal(), filePublicMapRef) }

// PublicFileMapGob returns the dist gob-encoded public filemap path.
func (d distLayout) PublicFileMapGob() string { return filepath.Join(d.Internal(), filePublicMapGob) }

// PrivateFileMapGob returns the dist gob-encoded private filemap path.
func (d distLayout) PrivateFileMapGob() string {
	return filepath.Join(d.Internal(), filePrivateMapGob)
}

// KeepFile returns the dist keep-file path used to retain directories.
func (d distLayout) KeepFile() string { return filepath.Join(d.Static(), fileKeep) }

// ParsedConfig is the parsed and validated Wave configuration payload.
type ParsedConfig struct {
	Core  *CoreConfig  `json:"Core"`
	Vite  *viteConfig  `json:"Vite,omitempty"`
	Watch *WatchConfig `json:"Watch,omitempty"`

	Dist distLayout `json:"-"`

	FrameworkWatchPatterns         []WatchedFile                     `json:"-"`
	FrameworkIgnoredPatterns       []string                          `json:"-"`
	FrameworkPublicFileMapOutDir   string                            `json:"-"`
	FrameworkSchemaExtensions      map[string]jsonschema.Entry       `json:"-"`
	FrameworkDevBuildHook          string                            `json:"-"`
	FrameworkProdBuildHook         string                            `json:"-"`
	FrameworkRunBuildHook          func(context.Context, bool) error `json:"-"`
	FrameworkPrepareGoBuildOverlay func() (*GoBuildOverlay, error)   `json:"-"`

	FrameworkBrowserRuntimeNamespace              string `json:"-"`
	FrameworkBrowserPublicURLResolverFunctionName string `json:"-"`
	FrameworkBrowserRevalidateFunctionName        string `json:"-"`
	FrameworkRefreshRebuildingOverlayElementID    string `json:"-"`
	FrameworkCriticalCSSStyleElementID            string `json:"-"`
	FrameworkNonCriticalCSSLinkElementID          string `json:"-"`
}

// GoBuildOverlay describes a temporary overlay used for Go build execution.
type GoBuildOverlay struct {
	OverlayConfigPath string
	Cleanup           func() error
}

// CoreConfig contains framework/runtime core configuration.
type CoreConfig struct {
	ConfigLocation                   string          `json:"ConfigLocation,omitempty"`
	DevBuildHook                     string          `json:"DevBuildHook,omitempty"`
	DevBuildHookTimeoutMilliseconds  int             `json:"DevBuildHookTimeoutMilliseconds,omitempty"`
	ProdBuildHook                    string          `json:"ProdBuildHook,omitempty"`
	ProdBuildHookTimeoutMilliseconds int             `json:"ProdBuildHookTimeoutMilliseconds,omitempty"`
	MainAppEntry                     string          `json:"MainAppEntry"`
	DistDir                          string          `json:"DistDir"`
	StaticAssetDirs                  staticAssetDirs `json:"StaticAssetDirs"`
	CSSEntryFiles                    cssEntryFiles   `json:"CSSEntryFiles,omitempty"`
	PublicPathPrefix                 string          `json:"PublicPathPrefix,omitempty"`
	ServerOnlyMode                   bool            `json:"ServerOnlyMode,omitempty"`
	SequentialGoBuild                bool            `json:"SequentialGoBuild,omitempty"`
}

// staticAssetDirs configures public/private static asset source directories.
type staticAssetDirs struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

// cssEntryFiles configures optional critical and non-critical CSS entry files.
type cssEntryFiles struct {
	Critical    string `json:"Critical,omitempty"`
	NonCritical string `json:"NonCritical,omitempty"`
}

// viteConfig configures optional Vite integration.
type viteConfig struct {
	JSPackageManagerBaseCmd string `json:"JSPackageManagerBaseCmd"`
	JSPackageManagerCmdDir  string `json:"JSPackageManagerCmdDir,omitempty"`
	DefaultPort             int    `json:"DefaultPort,omitempty"`
	ViteConfigFile          string `json:"ViteConfigFile,omitempty"`
}

// WatchConfig configures dev watch behavior and hooks.
type WatchConfig struct {
	WatchRoot              string                    `json:"WatchRoot,omitempty"`
	HealthcheckEndpoint    string                    `json:"HealthcheckEndpoint,omitempty"`
	HookStageFailurePolicy string                    `json:"HookStageFailurePolicy,omitempty"`
	HookCommandTimeouts    HookCommandTimeoutConfig  `json:"HookCommandTimeouts"`
	HookCallbackTimeouts   HookCallbackTimeoutConfig `json:"HookCallbackTimeouts"`
	Include                []WatchedFile             `json:"Include,omitempty"`
	Exclude                struct {
		Dirs  []string `json:"Dirs,omitempty"`
		Files []string `json:"Files,omitempty"`
	} `json:"Exclude,omitempty"`
}

// HookCommandTimeoutConfig configures command timeout overrides per hook stage.
type HookCommandTimeoutConfig struct {
	PreCommandTimeoutMilliseconds              int `json:"PreCommandTimeoutMilliseconds,omitempty"`
	ConcurrentCommandTimeoutMilliseconds       int `json:"ConcurrentCommandTimeoutMilliseconds,omitempty"`
	ConcurrentNoWaitCommandTimeoutMilliseconds int `json:"ConcurrentNoWaitCommandTimeoutMilliseconds,omitempty"`
	PostCommandTimeoutMilliseconds             int `json:"PostCommandTimeoutMilliseconds,omitempty"`
}

// HookCallbackTimeoutConfig configures callback timeout overrides per hook
// stage.
type HookCallbackTimeoutConfig struct {
	PreCallbackTimeoutMilliseconds              int `json:"PreCallbackTimeoutMilliseconds,omitempty"`
	ConcurrentCallbackTimeoutMilliseconds       int `json:"ConcurrentCallbackTimeoutMilliseconds,omitempty"`
	ConcurrentNoWaitCallbackTimeoutMilliseconds int `json:"ConcurrentNoWaitCallbackTimeoutMilliseconds,omitempty"`
	PostCallbackTimeoutMilliseconds             int `json:"PostCallbackTimeoutMilliseconds,omitempty"`
}

// WatchedFile configures file-pattern watch behavior and hooks.
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

// SortedHooks stores hooks partitioned by execution timing stage.
type SortedHooks struct {
	Pre              []OnChangeHook
	Concurrent       []OnChangeHook
	ConcurrentNoWait []OnChangeHook
	Post             []OnChangeHook
}

// HookContext provides context to callbacks during file change handling.
type HookContext struct {
	// ExecutionContext is canceled when the surrounding execution pipeline is
	// canceled (for example, concurrent stage cancellation after build failure).
	// Callback hooks can watch this context for cooperative cancellation.
	ExecutionContext context.Context
	// FilePath is the absolute path of the changed file.
	FilePath string
	// ChangedFilePaths contains all changed file paths associated with the hook
	// execution context. For deduped pattern hook execution, this includes all
	// files in the matched batch.
	ChangedFilePaths []string
	// AppStoppedForBatch is true when the app has been stopped as part of batch
	// processing (e.g., a Go file changed in the same batch). When true, HTTP
	// endpoints on the running app cannot be called.
	AppStoppedForBatch bool
}

// FrameworkRuntimeReloadRequest describes one framework runtime reload request
// that must be executed before browser reload payload broadcast.
type FrameworkRuntimeReloadRequest struct {
	// EndpointPath is the framework runtime endpoint path to call.
	EndpointPath string
	// ReloadAttemptID is the attempt identifier propagated as request header.
	ReloadAttemptID string
	// ExpectedBuildID is the expected framework build identifier propagated as
	// request header.
	ExpectedBuildID string
	// ReloadTrigger is the reload trigger identifier propagated as request
	// header.
	ReloadTrigger string
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
	// FrameworkRuntimeReloadRequest requests one framework runtime reload
	// endpoint call before browser reload payload broadcast.
	FrameworkRuntimeReloadRequest *FrameworkRuntimeReloadRequest
}

// OnChangeHook defines an action to run when a watched file changes.
type OnChangeHook struct {
	// Cmd is a shell command to run.
	Cmd string `json:"Cmd,omitempty"`
	// CommandTimeoutMilliseconds overrides stage-level command timeout for this hook when > 0.
	CommandTimeoutMilliseconds int `json:"CommandTimeoutMilliseconds,omitempty"`
	// DisableStageCommandTimeout disables stage-level command timeout for this hook.
	DisableStageCommandTimeout bool `json:"DisableStageCommandTimeout,omitempty"`
	// CallbackTimeoutMilliseconds overrides stage-level callback timeout for this hook when > 0.
	CallbackTimeoutMilliseconds int `json:"CallbackTimeoutMilliseconds,omitempty"`
	// DisableStageCallbackTimeout disables stage-level callback timeout for this hook.
	DisableStageCallbackTimeout bool `json:"DisableStageCallbackTimeout,omitempty"`
	// RunCombinedDevBuildHookCommands executes the configured development build
	// hooks in order (Core.DevBuildHook then framework dev build hook).
	RunCombinedDevBuildHookCommands bool `json:"RunCombinedDevBuildHookCommands,omitempty"`
	// Timing controls when the hook runs relative to Wave's rebuild process.
	Timing onChangeTiming `json:"Timing,omitempty"`
	// Exclude contains glob patterns for files to exclude from triggering this hook.
	Exclude []string `json:"Exclude,omitempty"`
	// Callback is a Go function to run. Framework use only (not JSON-configurable).
	// If the callback returns a non-nil RefreshAction, it controls what Wave does
	// after all hooks complete. Multiple RefreshActions are merged with OR semantics.
	Callback func(*HookContext) (*RefreshAction, error) `json:"-"`
}

// FileMap maps source public asset paths to built artifact metadata.
type FileMap map[string]FileVal

// FileVal describes a single built public asset mapping entry.
type FileVal struct {
	DistName    string
	ContentHash string
	IsPrehashed bool
}

// Public Type Methods and Path Normalization

// Sort partitions hook entries by execution timing for efficient watch
// pipeline planning.
func (wf *WatchedFile) Sort() {
	if wf.SortedHooks != nil {
		return
	}

	wf.SortedHooks = &SortedHooks{}
	for _, onChangeHook := range wf.OnChangeHooks {
		switch onChangeHook.Timing {
		case OnChangeStrategyPost:
			wf.SortedHooks.Post = append(wf.SortedHooks.Post, onChangeHook)
		case OnChangeStrategyConcurrent:
			wf.SortedHooks.Concurrent = append(
				wf.SortedHooks.Concurrent,
				onChangeHook,
			)
		case onChangeStrategyConcurrentNoWait:
			wf.SortedHooks.ConcurrentNoWait = append(
				wf.SortedHooks.ConcurrentNoWait,
				onChangeHook,
			)
		default:
			wf.SortedHooks.Pre = append(wf.SortedHooks.Pre, onChangeHook)
		}
	}
}

// Merge combines two RefreshActions with OR semantics.
// TriggerRestart takes precedence over browser reload.
func (refreshAction RefreshAction) merge(other RefreshAction) RefreshAction {
	mergedFrameworkRuntimeReloadRequest := refreshAction.FrameworkRuntimeReloadRequest
	if mergedFrameworkRuntimeReloadRequest == nil {
		mergedFrameworkRuntimeReloadRequest = other.FrameworkRuntimeReloadRequest
	}
	return RefreshAction{
		ReloadBrowser: refreshAction.ReloadBrowser ||
			other.ReloadBrowser,
		WaitForApp: refreshAction.WaitForApp ||
			other.WaitForApp,
		WaitForVite: refreshAction.WaitForVite ||
			other.WaitForVite,
		TriggerRestart: refreshAction.TriggerRestart ||
			other.TriggerRestart,
		RecompileGo: refreshAction.RecompileGo ||
			other.RecompileGo,
		FrameworkRuntimeReloadRequest: mergedFrameworkRuntimeReloadRequest,
	}
}

// IsZero returns true if this RefreshAction specifies no action.
func (refreshAction RefreshAction) IsZero() bool {
	return !refreshAction.ReloadBrowser &&
		!refreshAction.WaitForApp &&
		!refreshAction.WaitForVite &&
		!refreshAction.TriggerRestart &&
		!refreshAction.RecompileGo &&
		refreshAction.FrameworkRuntimeReloadRequest == nil
}

// Lookup resolves a source public asset path to its built public URL.
//
// It supports both raw lookup keys and keys that already include the configured
// public path prefix.
func (fileMap FileMap) Lookup(
	original string,
	prefix string,
) (url string, found bool) {
	normalizedOriginal := normalizePublicAssetPathForLookup(original)
	if entry, ok := fileMap[normalizedOriginal]; ok {
		return joinPublicURLPrefixAndPath(prefix, entry.DistName), true
	}

	deprefixedOriginal := trimConfiguredPublicPathPrefixFromLookupPath(
		normalizedOriginal,
		prefix,
	)
	if deprefixedOriginal != normalizedOriginal {
		if entry, ok := fileMap[deprefixedOriginal]; ok {
			return joinPublicURLPrefixAndPath(prefix, entry.DistName), true
		}
	}

	return "", false
}

func normalizePublicAssetPathForLookup(original string) string {
	return strings.TrimPrefix(path.Clean("/"+original), "/")
}

func trimConfiguredPublicPathPrefixFromLookupPath(
	normalizedLookupPath string,
	publicPathPrefix string,
) string {
	normalizedPublicPathPrefix := normalizeConfiguredPublicPathPrefixForLookup(
		publicPathPrefix,
	)
	if normalizedPublicPathPrefix == "" {
		return normalizedLookupPath
	}

	if normalizedLookupPath == normalizedPublicPathPrefix {
		return ""
	}

	normalizedPublicPathPrefixWithTrailingSlash := normalizedPublicPathPrefix + "/"
	if strings.HasPrefix(
		normalizedLookupPath,
		normalizedPublicPathPrefixWithTrailingSlash,
	) {
		return strings.TrimPrefix(
			normalizedLookupPath,
			normalizedPublicPathPrefixWithTrailingSlash,
		)
	}

	return normalizedLookupPath
}

func normalizeConfiguredPublicPathPrefixForLookup(
	publicPathPrefix string,
) string {
	return strings.Trim(path.Clean("/"+publicPathPrefix), "/")
}

func joinPublicURLPrefixAndPath(
	publicPathPrefix string,
	publicPath string,
) string {
	return matcher.EnsureLeadingSlash(path.Join(publicPathPrefix, publicPath))
}

// PublicPathPrefix returns the normalized configured public path prefix.
// The root prefix is represented as "/".
func (parsedConfig *ParsedConfig) PublicPathPrefix() string {
	prefix := parsedConfig.Core.PublicPathPrefix
	if prefix == "" || prefix == "/" {
		return "/"
	}
	return matcher.EnsureLeadingAndTrailingSlash(prefix)
}

// ViteManifestPath returns the expected private output path for the Vite
// manifest produced during builds.
func (parsedConfig *ParsedConfig) ViteManifestPath() string {
	return filepath.Join(
		parsedConfig.Dist.StaticPrivate(),
		hashedOutputPrefixNoTrailing,
		"vorma_vite_manifest.json",
	)
}

// WatchRoot returns the normalized watch root path used by dev tooling.
func (parsedConfig *ParsedConfig) WatchRoot() string {
	if parsedConfig.Watch != nil && parsedConfig.Watch.WatchRoot != "" {
		return filepath.Clean(parsedConfig.Watch.WatchRoot)
	}
	return "."
}

// HealthcheckEndpoint returns the app healthcheck endpoint used by dev
// readiness probes.
func (parsedConfig *ParsedConfig) HealthcheckEndpoint() string {
	if parsedConfig.Watch != nil &&
		parsedConfig.Watch.HealthcheckEndpoint != "" {
		return parsedConfig.Watch.HealthcheckEndpoint
	}
	return "/"
}

// UsingBrowser reports whether browser runtime integration is enabled.
func (parsedConfig *ParsedConfig) UsingBrowser() bool {
	return !parsedConfig.Core.ServerOnlyMode
}

// UsingVite reports whether Vite integration is configured.
func (parsedConfig *ParsedConfig) UsingVite() bool {
	return parsedConfig.Vite != nil
}

// CriticalCSSEntry returns the normalized critical CSS entry path, if set.
func (parsedConfig *ParsedConfig) CriticalCSSEntry() string {
	if parsedConfig.Core.CSSEntryFiles.Critical == "" {
		return ""
	}
	return filepath.Clean(parsedConfig.Core.CSSEntryFiles.Critical)
}

// NonCriticalCSSEntry returns the normalized non-critical CSS entry path, if
// set.
func (parsedConfig *ParsedConfig) NonCriticalCSSEntry() string {
	if parsedConfig.Core.CSSEntryFiles.NonCritical == "" {
		return ""
	}
	return filepath.Clean(parsedConfig.Core.CSSEntryFiles.NonCritical)
}

// browserRuntimeNamespace returns the browser global namespace used by runtime
// integration scripts.
func (parsedConfig *ParsedConfig) browserRuntimeNamespace() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkBrowserRuntimeNamespace == "" {
		return defaultBrowserRuntimeNamespace
	}
	return parsedConfig.FrameworkBrowserRuntimeNamespace
}

// browserPublicURLResolverFunctionName returns the browser helper function name
// used for resolving public asset URLs.
func (parsedConfig *ParsedConfig) browserPublicURLResolverFunctionName() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkBrowserPublicURLResolverFunctionName == "" {
		return defaultBrowserPublicURLResolverFunctionName
	}
	return parsedConfig.FrameworkBrowserPublicURLResolverFunctionName
}

// browserRevalidateFunctionName returns the browser helper function name used
// for route revalidation.
func (parsedConfig *ParsedConfig) browserRevalidateFunctionName() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkBrowserRevalidateFunctionName == "" {
		return defaultBrowserRevalidateFunctionName
	}
	return parsedConfig.FrameworkBrowserRevalidateFunctionName
}

// refreshRebuildingOverlayElementID returns the DOM element id used for the
// rebuild overlay during dev refresh cycles.
func (parsedConfig *ParsedConfig) refreshRebuildingOverlayElementID() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkRefreshRebuildingOverlayElementID == "" {
		return defaultRefreshRebuildingOverlayElementID
	}
	return parsedConfig.FrameworkRefreshRebuildingOverlayElementID
}

// criticalCSSStyleElementID returns the DOM element id used for injected
// critical CSS.
func (parsedConfig *ParsedConfig) criticalCSSStyleElementID() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkCriticalCSSStyleElementID == "" {
		return defaultCriticalCSSStyleElementID
	}
	return parsedConfig.FrameworkCriticalCSSStyleElementID
}

// nonCriticalCSSLinkElementID returns the DOM element id used for injected
// non-critical stylesheet links.
func (parsedConfig *ParsedConfig) nonCriticalCSSLinkElementID() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkNonCriticalCSSLinkElementID == "" {
		return defaultNonCriticalCSSLinkElementID
	}
	return parsedConfig.FrameworkNonCriticalCSSLinkElementID
}

// Config Parsing

// parseConfig parses Wave config JSON bytes into a ParsedConfig.
// This performs minimal validation to prevent nil pointer panics during parsing.
// Full validation of required fields should be done at build time via
// wave/tooling/builder.ValidateConfig.
func parseConfig(data []byte) (*ParsedConfig, error) {
	var cfg ParsedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Minimal safety check: Core must exist to access Core.DistDir below
	if cfg.Core == nil {
		return nil, fmt.Errorf("config: Core section is required")
	}

	cfg.Dist = distLayout{Root: filepath.Clean(cfg.Core.DistDir)}

	return &cfg, nil
}

// ParseConfigFile reads and parses a config file.
func ParseConfigFile(path string) (*ParsedConfig, error) {
	configFilePath := strings.TrimSpace(path)
	if configFilePath == "" {
		return nil, fmt.Errorf("config file path is required")
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg, err := parseConfig(data)
	if err != nil {
		return nil, err
	}

	cfg.Core.ConfigLocation = wavecore.Absolute(configFilePath)

	return cfg, nil
}

// Config Cloning

// Clone returns a defensive public snapshot of parsed config values.
// Internal framework-only mutable fields are intentionally omitted.
func (parsedConfig *ParsedConfig) Clone() *ParsedConfig {
	return parsedConfig.cloneParsedConfig(false)
}

func (parsedConfig *ParsedConfig) cloneForBuildtime() *ParsedConfig {
	return parsedConfig.cloneParsedConfig(true)
}

func (parsedConfig *ParsedConfig) cloneParsedConfig(
	includeBuildtimeFrameworkInternals bool,
) *ParsedConfig {
	if parsedConfig == nil {
		return nil
	}

	clonedParsedConfig := &ParsedConfig{
		Core:  cloneCoreConfig(parsedConfig.Core),
		Vite:  cloneViteConfig(parsedConfig.Vite),
		Watch: cloneWatchConfig(parsedConfig.Watch),

		Dist: parsedConfig.Dist,
	}
	copyFrameworkRuntimeFields(
		clonedParsedConfig,
		parsedConfig,
		includeBuildtimeFrameworkInternals,
	)

	return clonedParsedConfig
}

// CopyFrameworkRuntimeFieldsForToolingReload copies framework-owned runtime
// fields from one parsed config into another for config-reload flows.
func CopyFrameworkRuntimeFieldsForToolingReload(
	target *ParsedConfig,
	source *ParsedConfig,
) {
	copyFrameworkRuntimeFields(target, source, true)
}

func copyFrameworkRuntimeFields(
	target *ParsedConfig,
	source *ParsedConfig,
	includeBuildtimeFrameworkInternals bool,
) {
	if target == nil || source == nil {
		return
	}

	target.FrameworkWatchPatterns = cloneFrameworkWatchPatterns(
		source.FrameworkWatchPatterns,
	)
	target.FrameworkIgnoredPatterns = append(
		[]string(nil),
		source.FrameworkIgnoredPatterns...,
	)
	target.FrameworkPublicFileMapOutDir = source.FrameworkPublicFileMapOutDir
	target.FrameworkSchemaExtensions = nil
	target.FrameworkDevBuildHook = source.FrameworkDevBuildHook
	target.FrameworkProdBuildHook = source.FrameworkProdBuildHook
	target.FrameworkRunBuildHook = nil
	target.FrameworkPrepareGoBuildOverlay = nil
	target.FrameworkBrowserRuntimeNamespace = source.FrameworkBrowserRuntimeNamespace
	target.FrameworkBrowserPublicURLResolverFunctionName = source.FrameworkBrowserPublicURLResolverFunctionName
	target.FrameworkBrowserRevalidateFunctionName = source.FrameworkBrowserRevalidateFunctionName
	target.FrameworkRefreshRebuildingOverlayElementID = source.FrameworkRefreshRebuildingOverlayElementID
	target.FrameworkCriticalCSSStyleElementID = source.FrameworkCriticalCSSStyleElementID
	target.FrameworkNonCriticalCSSLinkElementID = source.FrameworkNonCriticalCSSLinkElementID

	if includeBuildtimeFrameworkInternals {
		target.FrameworkSchemaExtensions = cloneFrameworkSchemaExtensions(
			source.FrameworkSchemaExtensions,
		)
		target.FrameworkRunBuildHook = source.FrameworkRunBuildHook
		target.FrameworkPrepareGoBuildOverlay = source.FrameworkPrepareGoBuildOverlay
	}
}

func cloneCoreConfig(
	coreConfig *CoreConfig,
) *CoreConfig {
	if coreConfig == nil {
		return nil
	}

	clonedCoreConfig := *coreConfig
	return &clonedCoreConfig
}

func cloneViteConfig(
	viteConfig *viteConfig,
) *viteConfig {
	if viteConfig == nil {
		return nil
	}

	clonedViteConfig := *viteConfig
	return &clonedViteConfig
}

func cloneWatchConfig(
	watchConfig *WatchConfig,
) *WatchConfig {
	if watchConfig == nil {
		return nil
	}

	clonedWatchConfig := *watchConfig
	clonedWatchConfig.Include = cloneFrameworkWatchPatterns(watchConfig.Include)
	clonedWatchConfig.Exclude.Dirs = append(
		[]string(nil),
		watchConfig.Exclude.Dirs...)
	clonedWatchConfig.Exclude.Files = append(
		[]string(nil),
		watchConfig.Exclude.Files...)

	return &clonedWatchConfig
}

func cloneFrameworkSchemaExtensions(
	schemaExtensions map[string]jsonschema.Entry,
) map[string]jsonschema.Entry {
	if len(schemaExtensions) == 0 {
		return nil
	}

	clonedSchemaExtensions := make(
		map[string]jsonschema.Entry,
		len(schemaExtensions),
	)
	for key, value := range schemaExtensions {
		clonedSchemaExtensions[key] = value
	}
	return clonedSchemaExtensions
}

func cloneFrameworkWatchPatterns(
	watchedFiles []WatchedFile,
) []WatchedFile {
	if len(watchedFiles) == 0 {
		return nil
	}

	clonedWatchedFiles := make([]WatchedFile, 0, len(watchedFiles))
	for _, watchedFile := range watchedFiles {
		clonedWatchedFiles = append(
			clonedWatchedFiles,
			cloneWatchedFileForFrameworkRuntimeState(watchedFile),
		)
	}

	return clonedWatchedFiles
}

func cloneWatchedFileForFrameworkRuntimeState(
	watchedFile WatchedFile,
) WatchedFile {
	clonedWatchedFile := watchedFile
	clonedWatchedFile.OnChangeHooks = cloneOnChangeHooksForFrameworkRuntimeState(
		watchedFile.OnChangeHooks,
	)
	clonedWatchedFile.SortedHooks = cloneSortedHooksForFrameworkRuntimeState(
		watchedFile.SortedHooks,
	)

	return clonedWatchedFile
}

func cloneSortedHooksForFrameworkRuntimeState(
	sortedHooks *SortedHooks,
) *SortedHooks {
	if sortedHooks == nil {
		return nil
	}

	return &SortedHooks{
		Pre: cloneOnChangeHooksForFrameworkRuntimeState(sortedHooks.Pre),
		Concurrent: cloneOnChangeHooksForFrameworkRuntimeState(
			sortedHooks.Concurrent,
		),
		ConcurrentNoWait: cloneOnChangeHooksForFrameworkRuntimeState(
			sortedHooks.ConcurrentNoWait,
		),
		Post: cloneOnChangeHooksForFrameworkRuntimeState(sortedHooks.Post),
	}
}

func cloneOnChangeHooksForFrameworkRuntimeState(
	onChangeHooks []OnChangeHook,
) []OnChangeHook {
	if len(onChangeHooks) == 0 {
		return nil
	}

	clonedOnChangeHooks := make([]OnChangeHook, 0, len(onChangeHooks))
	for _, onChangeHook := range onChangeHooks {
		clonedOnChangeHook := onChangeHook
		clonedOnChangeHook.Exclude = append(
			[]string(nil),
			onChangeHook.Exclude...,
		)
		clonedOnChangeHooks = append(clonedOnChangeHooks, clonedOnChangeHook)
	}

	return clonedOnChangeHooks
}

// Wave Construction and Runtime Cache Wiring

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
	cfg, parseError := parseConfig(configJSON)
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

// Runtime Framework Mutators and Accessors

// RawConfigJSON returns the raw bytes of the configuration file.
func (w *Wave) RawConfigJSON() []byte {
	return append([]byte(nil), w.rawCfg...)
}

// addFrameworkWatchPatterns adds watch patterns for use during development.
func (w *Wave) addFrameworkWatchPatterns(patterns []WatchedFile) {
	w.cfg.FrameworkWatchPatterns = append(
		w.cfg.FrameworkWatchPatterns,
		cloneFrameworkWatchPatterns(patterns)...,
	)
}

// addIgnoredPatterns adds glob patterns for files/directories to ignore during watching.
func (w *Wave) addIgnoredPatterns(patterns []string) {
	w.cfg.FrameworkIgnoredPatterns = append(
		w.cfg.FrameworkIgnoredPatterns,
		patterns...)
}

// setPublicFileMapOutDir sets the directory where Wave should write the public filemap TypeScript file.
func (w *Wave) setPublicFileMapOutDir(dir string) {
	w.cfg.FrameworkPublicFileMapOutDir = dir
}

// registerFrameworkSchemaSection registers a framework-owned schema extension
// for wave.config.json generation at build time.
func (w *Wave) registerFrameworkSchemaSection(
	name string,
	schema jsonschema.Entry,
) {
	if w.cfg.FrameworkSchemaExtensions == nil {
		w.cfg.FrameworkSchemaExtensions = make(map[string]jsonschema.Entry)
	}
	w.cfg.FrameworkSchemaExtensions[name] = schema
}

// setFrameworkDevBuildHookCommand sets the framework dev build hook command.
func (w *Wave) setFrameworkDevBuildHookCommand(command string) {
	w.cfg.FrameworkDevBuildHook = command
}

// setFrameworkProdBuildHookCommand sets the framework production build hook command.
func (w *Wave) setFrameworkProdBuildHookCommand(command string) {
	w.cfg.FrameworkProdBuildHook = command
}

// setFrameworkRunBuildHookRunner sets the framework build hook runner callback.
func (w *Wave) setFrameworkRunBuildHookRunner(
	runner func(context.Context, bool) error,
) {
	w.cfg.FrameworkRunBuildHook = runner
}

// setFrameworkPrepareGoBuildOverlay sets the framework Go build overlay callback.
func (w *Wave) setFrameworkPrepareGoBuildOverlay(
	preparer func() (*GoBuildOverlay, error),
) {
	w.cfg.FrameworkPrepareGoBuildOverlay = preparer
}

// setBrowserRuntimeNamespace overrides the browser runtime namespace used by
// generated client integration scripts.
func (w *Wave) setBrowserRuntimeNamespace(namespace string) {
	w.cfg.FrameworkBrowserRuntimeNamespace = namespace
}

// setBrowserPublicURLResolverFunctionName overrides the generated browser
// helper name used to resolve public asset URLs.
func (w *Wave) setBrowserPublicURLResolverFunctionName(functionName string) {
	w.cfg.FrameworkBrowserPublicURLResolverFunctionName = functionName
}

// setBrowserRevalidateFunctionName overrides the generated browser helper name
// used to trigger revalidation.
func (w *Wave) setBrowserRevalidateFunctionName(functionName string) {
	w.cfg.FrameworkBrowserRevalidateFunctionName = functionName
}

// setRefreshRebuildingOverlayElementID overrides the DOM element id used for
// the rebuild overlay in dev refresh flows.
func (w *Wave) setRefreshRebuildingOverlayElementID(elementID string) {
	w.cfg.FrameworkRefreshRebuildingOverlayElementID = elementID
}

// setCriticalCSSStyleElementID overrides the DOM element id used for injected
// critical CSS style elements.
func (w *Wave) setCriticalCSSStyleElementID(elementID string) {
	w.cfg.FrameworkCriticalCSSStyleElementID = elementID
}

// setNonCriticalCSSLinkElementID overrides the DOM element id used for
// injected non-critical stylesheet link elements.
func (w *Wave) setNonCriticalCSSLinkElementID(elementID string) {
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
	w.portResolver = wavecore.NewResolverForMode(isDevMode)
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

// publicStaticDir returns the source directory for public static assets.
func (w *Wave) publicStaticDir() string {
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

// viteOutDir returns the directory where Vite emits public artifacts.
func (w *Wave) viteOutDir() string {
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

// Runtime File Systems

func (w *Wave) initBaseFS() (fs.FS, error) {
	if w.IsDev() {
		return os.DirFS(w.cfg.Dist.Static()), nil
	}
	if w.distStaticFS == nil {
		return nil, fmt.Errorf("distStaticFS is nil in production mode")
	}
	return w.distStaticFS, nil
}

func (w *Wave) initPublicFS() (fs.FS, error) {
	return w.initSubFS(RelPaths.assetsPublic())
}

func (w *Wave) initPrivateFS() (fs.FS, error) {
	return w.initSubFS(RelPaths.assetsPrivate())
}

func (w *Wave) initSubFS(
	relativeSubdirectoryPath string,
) (fs.FS, error) {
	base, err := w.getBaseFS()
	if err != nil {
		return nil, err
	}
	return fs.Sub(base, relativeSubdirectoryPath)
}

func (w *Wave) getBaseFS() (fs.FS, error) {
	return w.baseFS.get()
}

func (w *Wave) getPublicFS() (fs.FS, error) {
	return w.publicFS.get()
}

func (w *Wave) PrivateFS() (fs.FS, error) {
	return w.privateFS.get()
}

func (w *Wave) MustPrivateFS() fs.FS {
	return mustGetCachedFileSystem(w.privateFS)
}

func mustGetCachedFileSystem(
	cachedFileSystem *cache[fs.FS],
) fs.FS {
	fileSystem, err := cachedFileSystem.get()
	if err != nil {
		panic(err)
	}
	return fileSystem
}

// Runtime Internal Reference Files

func (w *Wave) readTrimmedInternalRefFile(relativePath string) (string, error) {
	baseFS, err := w.getBaseFS()
	if err != nil {
		return "", err
	}

	content, err := fs.ReadFile(baseFS, relativePath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}

func (w *Wave) initPublicURLFromInternalRefFile(
	relativePath string,
) (string, error) {
	refPath, err := w.readTrimmedInternalRefFile(relativePath)
	if err != nil {
		return "", err
	}

	return wavecore.ResolveFromReferencedPath(
		w.cfg.PublicPathPrefix(),
		refPath,
	), nil
}

// Runtime Asset Map and URL Resolution

func (w *Wave) initFileMap() (FileMap, error) {
	base, err := w.getBaseFS()
	if err != nil {
		return nil, err
	}

	f, err := base.Open(RelPaths.PublicFileMapGob())
	if err != nil {
		return nil, fmt.Errorf("open file map: %w", err)
	}
	defer f.Close()

	fm, err := fsutil.FromGob[FileMap](f)
	if err != nil {
		return nil, fmt.Errorf("decode file map: %w", err)
	}

	return fm, nil
}

func (w *Wave) publicFileMap() (FileMap, error) {
	fileMap, err := w.fileMap.get()
	if err != nil {
		return nil, err
	}

	return clonePublicFileMap(fileMap), nil
}

func (w *Wave) resolvePublicURL(original string) (string, error) {
	fm, err := w.fileMap.get()
	if err != nil {
		w.log.Warn("failed to load file map", "error", err)
		return "", err
	}

	url, found := fm.Lookup(original, w.cfg.PublicPathPrefix())
	if !found {
		w.log.Warn("no hashed URL found", "url", original)
		return "", fmt.Errorf("no hashed URL found for %q", original)
	}

	return url, nil
}

func (w *Wave) PublicURL(original string) string {
	url, _ := w.publicURLs.get(original)
	return url
}

func (w *Wave) checkIsAsset(urlPath string) (bool, error) {
	publicAssetPath, isPublicAssetPath := w.publicAssetPath(urlPath)
	if !isPublicAssetPath {
		return false, nil
	}

	publicFS, err := w.getPublicFS()
	if err != nil {
		return false, err
	}

	info, err := fs.Stat(publicFS, publicAssetPath)
	if err != nil {
		return false, nil
	}

	return !info.IsDir(), nil
}

func (w *Wave) publicAssetPath(urlPath string) (string, bool) {
	cleanURLPath := path.Clean("/" + urlPath)
	if cleanURLPath == "/" {
		return "", false
	}

	prefix := w.cfg.PublicPathPrefix()
	if prefix != "/" {
		if !strings.HasPrefix(cleanURLPath, prefix) {
			return "", false
		}
		cleanURLPath = strings.TrimPrefix(cleanURLPath, prefix)
	}

	assetPath := strings.TrimPrefix(cleanURLPath, "/")
	if assetPath == "" || assetPath == "." {
		return "", false
	}

	return assetPath, true
}

func (w *Wave) isPublicAsset(urlPath string) bool {
	isAsset, _ := w.isAsset.get(urlPath)
	return isAsset
}

func clonePublicFileMap(
	fileMap FileMap,
) FileMap {
	if len(fileMap) == 0 {
		return nil
	}

	clonedPublicFileMap := make(FileMap, len(fileMap))
	for key, value := range fileMap {
		clonedPublicFileMap[key] = value
	}

	return clonedPublicFileMap
}

// Runtime CSS Integration

func (w *Wave) initCriticalCSS() (*criticalCSSData, error) {
	if w.cfg.CriticalCSSEntry() == "" {
		return &criticalCSSData{noSuchFile: true}, nil
	}

	content, noSuchFile, err := w.readCriticalCSSContent()
	if err != nil {
		return nil, err
	}
	if noSuchFile {
		return &criticalCSSData{noSuchFile: true}, nil
	}

	return w.buildCriticalCSSData(content)
}

func (w *Wave) readCriticalCSSContent() (string, bool, error) {
	baseFS, err := w.getBaseFS()
	if err != nil {
		return "", false, err
	}

	content, err := fs.ReadFile(baseFS, RelPaths.CriticalCSS())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", true, nil
		}
		return "", false, err
	}

	return string(content), false, nil
}

func (w *Wave) buildCriticalCSSData(content string) (*criticalCSSData, error) {
	styleElement, sha256Hash, err := waveruntime.BuildCriticalCSSStyleElement(
		content,
		w.cfg.criticalCSSStyleElementID(),
	)
	if err != nil {
		w.log.Error(
			fmt.Sprintf("error building critical css style element: %v", err),
		)
		return nil, err
	}

	return &criticalCSSData{
		content:    content,
		styleEl:    styleElement,
		sha256Hash: sha256Hash,
	}, nil
}

func (w *Wave) getCriticalCSSData() *criticalCSSData {
	data, err := w.criticalCSS.get()
	if err != nil || data == nil || data.noSuchFile {
		return nil
	}
	return data
}

func (w *Wave) CriticalCSS() template.CSS {
	data := w.getCriticalCSSData()
	if data == nil {
		return ""
	}
	return template.CSS(data.content)
}

func (w *Wave) CriticalCSSStyleElement() template.HTML {
	data := w.getCriticalCSSData()
	if data == nil {
		return ""
	}
	return data.styleEl
}

func (w *Wave) initStylesheetURL() (string, error) {
	if w.cfg.NonCriticalCSSEntry() == "" {
		return "", nil
	}

	return w.initPublicURLFromInternalRefFile(RelPaths.NormalCSSRef())
}

func (w *Wave) styleSheetURL() string {
	url, _ := w.stylesheetURL.get()
	return url
}

func (w *Wave) initStylesheetLink() (string, error) {
	url := w.styleSheetURL()
	if url == "" {
		return "", nil
	}

	return waveruntime.BuildStylesheetLink(
		url,
		w.cfg.nonCriticalCSSLinkElementID(),
	), nil
}

func (w *Wave) StyleSheetLinkElement() template.HTML {
	link, _ := w.stylesheetLink.get()
	return template.HTML(link)
}

// Runtime Public File-Map Markup

func (w *Wave) initFileMapURL() (string, error) {
	return w.initPublicURLFromInternalRefFile(RelPaths.PublicFileMapRef())
}

func (w *Wave) publicFileMapURL() string {
	url, _ := w.fileMapURL.get()
	return url
}

func (w *Wave) initFileMapDetails() (*fileMapDetails, error) {
	fileMapURL := w.publicFileMapURL()
	if fileMapURL == "" {
		return &fileMapDetails{}, nil
	}

	elements, sha256Hash, err := waveruntime.BuildPublicFileMapElements(
		fileMapURL,
		w.cfg.browserRuntimeNamespace(),
	)
	if err != nil {
		return nil, err
	}

	return &fileMapDetails{
		elements:   elements,
		sha256Hash: sha256Hash,
	}, nil
}

// Runtime Refresh Script APIs

const defaultRefreshPort = 10000

func (w *Wave) RefreshScript() template.HTML {
	if !w.IsDev() {
		return ""
	}

	port := getRefreshServerPort()
	if port == 0 {
		port = defaultRefreshPort
	}

	return template.HTML(
		fmt.Sprintf(
			"<script>%s</script>",
			waveruntime.BuildRefreshScript(
				port,
				waveruntime.RefreshScriptConfig{
					BrowserRevalidateFunctionName:     w.cfg.browserRevalidateFunctionName(),
					RefreshRebuildingOverlayElementID: w.cfg.refreshRebuildingOverlayElementID(),
					NonCriticalCSSLinkElementID:       w.cfg.nonCriticalCSSLinkElementID(),
					CriticalCSSStyleElementID:         w.cfg.criticalCSSStyleElementID(),
				},
			),
		),
	)
}

// Runtime Static Handlers

func (w *Wave) staticHandler(immutable bool) (http.Handler, error) {
	publicFS, err := w.getPublicFS()
	if err != nil {
		return nil, err
	}

	fileServer := http.StripPrefix(
		w.cfg.PublicPathPrefix(),
		http.FileServer(http.FS(publicFS)),
	)

	if !immutable {
		return fileServer, nil
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(rw, req)
	}), nil
}

func (w *Wave) MustStaticMiddleware(
	immutable bool,
) func(http.Handler) http.Handler {
	handler, err := w.staticHandler(immutable)
	if err != nil {
		w.log.Error("failed to create static handler", "error", err)
		panic(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(rw http.ResponseWriter, req *http.Request) {
				if w.isPublicAsset(req.URL.Path) {
					handler.ServeHTTP(rw, req)
					return
				}
				next.ServeHTTP(rw, req)
			},
		)
	}
}
