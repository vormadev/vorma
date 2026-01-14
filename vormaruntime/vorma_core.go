package vormaruntime

import (
	"html/template"
	"io/fs"
	"net/http"
	"sync"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
)

const VormaSymbolStr = "__vorma_internal__"

var Log = colorlog.New("vorma")

type RouteType = string

var RouteTypes = struct {
	Loader   RouteType
	Query    RouteType
	Mutation RouteType
	NotFound RouteType
}{
	Loader:   "loader",
	Query:    "query",
	Mutation: "mutation",
	NotFound: "not-found",
}

type (
	GetDefaultHeadElsFunc    func(r *http.Request, app *Vorma, head *headels.HeadEls) error
	GetHeadElUniqueRulesFunc func(head *headels.HeadEls)
	GetRootTemplateDataFunc  func(r *http.Request) (map[string]any, error)
)

// Vorma is the main runtime struct for a Vorma application.
type Vorma struct {
	*wave.Wave

	Config *VormaConfig

	actionsRouter *ActionsRouter
	loadersRouter *LoadersRouter

	getDefaultHeadEls    GetDefaultHeadElsFunc
	getHeadElUniqueRules GetHeadElUniqueRulesFunc
	getRootTemplateData  GetRootTemplateDataFunc

	// mu protects mutable state that can be modified during dev rebuilds.
	mu                 sync.RWMutex
	_isDev             bool
	_paths             map[string]*Path
	_clientEntrySrc    string
	_clientEntryOut    string
	_clientEntryDeps   []string
	_buildID           string
	_depToCSSBundleMap map[string]string
	_rootTemplate      *template.Template
	_privateFS         fs.FS
	_routeManifestFile string
	_serverAddr        string

	// Config for TS Generation
	_adHocTypes  []*tsgen.AdHocType
	_extraTSCode string
}

// --- Public Getters (thread-safe, acquire lock) ---

func (v *Vorma) ServerAddr() string            { return v._serverAddr }
func (v *Vorma) LoadersRouter() *LoadersRouter { return v.loadersRouter }
func (v *Vorma) ActionsRouter() *ActionsRouter { return v.actionsRouter }

func (v *Vorma) GetPathsSnapshot() map[string]*Path {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._paths
}

func (v *Vorma) GetIsDevMode() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._isDev
}

func (v *Vorma) GetBuildID() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._buildID
}

func (v *Vorma) GetClientEntryOut() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._clientEntryOut
}

func (v *Vorma) GetClientEntryDeps() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._clientEntryDeps
}

func (v *Vorma) GetDepToCSSBundleMap() map[string]string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._depToCSSBundleMap
}

func (v *Vorma) GetRootTemplate() *template.Template {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._rootTemplate
}

func (v *Vorma) GetRouteManifestFile() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._routeManifestFile
}

// GetAdHocTypes returns the ad-hoc types for TS generation. (Immutable after init)
func (v *Vorma) GetAdHocTypes() []*tsgen.AdHocType {
	return v._adHocTypes
}

// GetExtraTSCode returns the extra TS code. (Immutable after init)
func (v *Vorma) GetExtraTSCode() string {
	return v._extraTSCode
}

// --- Lock Management ---
// Exposed for build package to acquire lock when making multiple updates.

func (v *Vorma) Lock()   { v.mu.Lock() }
func (v *Vorma) Unlock() { v.mu.Unlock() }

// --- Unsafe Getters/Setters (caller MUST hold lock) ---

func (v *Vorma) UnsafeGetPaths() map[string]*Path           { return v._paths }
func (v *Vorma) UnsafeGetBuildID() string                   { return v._buildID }
func (v *Vorma) UnsafeGetRouteManifestFile() string         { return v._routeManifestFile }
func (v *Vorma) UnsafeSetPaths(paths map[string]*Path)      { v._paths = paths }
func (v *Vorma) UnsafeSetBuildID(id string)                 { v._buildID = id }
func (v *Vorma) UnsafeSetRouteManifestFile(f string)        { v._routeManifestFile = f }
func (v *Vorma) UnsafeSetRootTemplate(t *template.Template) { v._rootTemplate = t }

// --- Thread-safe Setters (acquire lock internally) ---

func (v *Vorma) SetIsDev(isDev bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v._isDev = isDev
}
