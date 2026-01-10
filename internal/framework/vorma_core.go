package vorma

import (
	"html/template"
	"io/fs"
	"net/http"
	"sync"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/wave"
)

const (
	VormaSymbolStr = "__vorma_internal__"
)

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

type Path struct {
	NestedRoute mux.AnyNestedRoute `json:"-"`

	// both stages one and two
	OriginalPattern string `json:"originalPattern"`
	SrcPath         string `json:"srcPath"`
	ExportKey       string `json:"exportKey"`
	ErrorExportKey  string `json:"errorExportKey,omitempty"`

	// stage two only
	OutPath string   `json:"outPath,omitempty"`
	Deps    []string `json:"deps,omitempty"`
}

type UIVariant string

var UIVariants = struct {
	React  UIVariant
	Preact UIVariant
	Solid  UIVariant
}{
	React:  "react",
	Preact: "preact",
	Solid:  "solid",
}

type VormaConfig struct {
	IncludeDefaults            *bool  `json:"IncludeDefaults,omitempty"`
	UIVariant                  string `json:"UIVariant"`
	HTMLTemplateLocation       string `json:"HTMLTemplateLocation"`
	ClientEntry                string `json:"ClientEntry"`
	ClientRouteDefsFile        string `json:"ClientRouteDefsFile"`
	TSGenOutDir                string `json:"TSGenOutDir"`
	BuildtimePublicURLFuncName string `json:"BuildtimePublicURLFuncName,omitempty"`
}

type (
	GetDefaultHeadElsFunc    func(r *http.Request, app *Vorma, head *headels.HeadEls) error
	GetHeadElUniqueRulesFunc func(head *headels.HeadEls)
	GetRootTemplateDataFunc  func(r *http.Request) (map[string]any, error)
)

type Vorma struct {
	*wave.Wave

	config *VormaConfig

	actionsRouter *ActionsRouter
	loadersRouter *LoadersRouter

	getDefaultHeadEls    GetDefaultHeadElsFunc
	getHeadElUniqueRules GetHeadElUniqueRulesFunc
	getRootTemplateData  GetRootTemplateDataFunc

	// mu protects mutable state that can be modified during dev rebuilds.
	// Use RLock for read access, Lock for write access.
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
	_lastConfigHash    [32]byte // Cache for config hashing to skip redundant writes

	// Config for TS Generation (persisted in app struct)
	_adHocTypes  []*AdHocType
	_extraTSCode string
}

func (v *Vorma) ServerAddr() string            { return v._serverAddr }
func (v *Vorma) LoadersRouter() *LoadersRouter { return v.loadersRouter }
func (v *Vorma) ActionsRouter() *ActionsRouter { return v.actionsRouter }

// getPathsSnapshot returns the paths map for safe concurrent access.
// The map is replaced atomically during rebuilds (not mutated in place),
// so callers can safely read from it without holding a lock for the duration.
func (v *Vorma) getPathsSnapshot() map[string]*Path {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._paths
}

// getIsDev returns whether we're in dev mode. Thread-safe.
func (v *Vorma) getIsDev() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._isDev
}

// getBuildID returns the current build ID. Thread-safe.
func (v *Vorma) getBuildID() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._buildID
}

// getClientEntryOut returns the client entry output path. Thread-safe.
func (v *Vorma) getClientEntryOut() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._clientEntryOut
}

// getClientEntryDeps returns the client entry dependencies. Thread-safe.
func (v *Vorma) getClientEntryDeps() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._clientEntryDeps
}

// getDepToCSSBundleMap returns the dep to CSS bundle mapping. Thread-safe.
func (v *Vorma) getDepToCSSBundleMap() map[string]string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._depToCSSBundleMap
}

// getRootTemplate returns the root HTML template. Thread-safe.
func (v *Vorma) getRootTemplate() *template.Template {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._rootTemplate
}
