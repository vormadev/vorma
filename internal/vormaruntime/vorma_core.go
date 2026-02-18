package vormaruntime

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
)

const VormaSymbolStr = "__vorma_internal__"

type RouteType = string

type (
	GetDefaultHeadElsFunc   func(r *http.Request, app *Vorma, head *headels.HeadEls) error
	GetHeadDedupeKeysFunc   func(head *headels.HeadEls)
	GetRootTemplateDataFunc func(r *http.Request) (map[string]any, error)
)

// Vorma is the main runtime struct for a Vorma application.
type Vorma struct {
	*wave.Wave

	Config *VormaConfig
	Log    *slog.Logger

	actionsRouter *ActionsRouter
	loadersRouter *LoadersRouter
	headElsInst   *headels.Instance

	getDefaultHeadEls   GetDefaultHeadElsFunc
	getHeadDedupeKeys   GetHeadDedupeKeysFunc
	getRootTemplateData GetRootTemplateDataFunc

	loadersHandlerOnce sync.Once
	loadersHandler     mux.TasksCtxRequirerFunc
	actionsHandlerOnce sync.Once
	actionsHandler     mux.TasksCtxRequirerFunc

	// mu protects mutable state that can be modified during dev rebuilds.
	mu                 sync.RWMutex
	_isDev             bool
	_paths             map[string]*Path
	_clientEntrySrc    string
	_clientEntryOut    string
	_clientEntryDeps   []string
	_buildID           string
	_depToCSSBundleMap map[string][]string
	_rootTemplate      *template.Template
	_privateFS         fs.FS
	_routeManifestFile string
	_serverAddr        string
	// Monotonic version for route-data snapshot coherence. Increment whenever
	// route-data inputs that influence stage-1/cacheable outputs are mutated.
	_routeDataSnapshotVersion uint64
	_routeDataCache           *sync.Map

	_lifecycleState         runtimeLifecycleState
	_lifecycleTransitionSeq uint64
	_lifecycleLastError     string

	// Config for TS Generation
	_adHocTypes  []*tsgen.AdHocType
	_extraTSCode string
}

// --- Public Getters (thread-safe, acquire read lock) ---

func (v *Vorma) ServerAddr() string            { return v._serverAddr }
func (v *Vorma) LoadersRouter() *LoadersRouter { return v.loadersRouter }
func (v *Vorma) ActionsRouter() *ActionsRouter { return v.actionsRouter }

func (v *Vorma) Paths() map[string]*Path {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return clonePathsMapOrNil(v._paths)
}

func (v *Vorma) IsDevMode() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._isDev
}

func (v *Vorma) BuildID() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._buildID
}

func (v *Vorma) ClientEntryOut() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._clientEntryOut
}

func (v *Vorma) ClientEntryDeps() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return cloneStringSliceOrNil(v._clientEntryDeps)
}

func (v *Vorma) DepToCSSBundleMap() map[string][]string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return cloneDepToCSSBundleMapOrNil(v._depToCSSBundleMap)
}

func (v *Vorma) RootTemplate() *template.Template {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._rootTemplate
}

func (v *Vorma) RouteManifestFile() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._routeManifestFile
}

// AdHocTypes returns a defensive copy of ad-hoc TS types.
func (v *Vorma) AdHocTypes() []*tsgen.AdHocType {
	return cloneAdHocTypesOrNil(v._adHocTypes)
}

// ExtraTSCode returns the extra TS code. (Immutable after init)
func (v *Vorma) ExtraTSCode() string {
	return v._extraTSCode
}

// --- LockedVorma Pattern ---
// Use ReadLockedVorma for read-only access under WithRLock.
// Use LockedVorma for mutable access under WithLock.

// ReadLockedVorma wraps a Vorma instance for read-only access while holding
// the read lock. This type can only be obtained via Vorma.WithRLock.
type ReadLockedVorma struct {
	v *Vorma
}

// LockedVorma wraps a Vorma instance and provides access to lock-protected fields.
// This type can only be obtained via Vorma.WithLock, ensuring the lock is held.
type LockedVorma struct {
	v *Vorma
}

// WithLock acquires the write lock and calls fn with a LockedVorma.
// The lock is released when fn returns.
func (v *Vorma) WithLock(fn func(*LockedVorma)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	fn(&LockedVorma{v: v})
}

// WithRLock acquires the read lock and calls fn with a ReadLockedVorma.
// The lock is released when fn returns. Use this for read-only operations.
func (v *Vorma) WithRLock(fn func(*ReadLockedVorma)) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	fn(&ReadLockedVorma{v: v})
}

// --- LockedVorma Getters ---

// Vorma returns the underlying Vorma instance for accessing non-lock-protected fields.
func (l *ReadLockedVorma) Vorma() *Vorma { return l.v }
func (l *ReadLockedVorma) Paths() map[string]*Path {
	return clonePathsMapOrNil(l.v._paths)
}

func (l *ReadLockedVorma) BuildID() string { return l.v._buildID }

func (l *ReadLockedVorma) RouteManifestFile() string { return l.v._routeManifestFile }

func (l *ReadLockedVorma) RootTemplate() *template.Template { return l.v._rootTemplate }

func (l *ReadLockedVorma) IsDev() bool { return l.v._isDev }

func (l *LockedVorma) Vorma() *Vorma { return l.v }
func (l *LockedVorma) Paths() map[string]*Path {
	return clonePathsMapOrNil(l.v._paths)
}
func (l *LockedVorma) BuildID() string { return l.v._buildID }

func (l *LockedVorma) RouteManifestFile() string { return l.v._routeManifestFile }

func (l *LockedVorma) RootTemplate() *template.Template { return l.v._rootTemplate }
func (l *LockedVorma) IsDev() bool                      { return l.v._isDev }

// --- LockedVorma Setters ---

func (l *LockedVorma) SetPaths(paths map[string]*Path) {
	l.v.routes().ReplaceParsedPathsForInit(paths, false)
}
func (l *LockedVorma) SetIsDev(isDev bool) { l.v.setIsDevModeLocked(isDev) }
func (l *LockedVorma) SetBuildID(buildID string) {
	if l.v._buildID == buildID {
		return
	}
	l.v._buildID = buildID
	l.v.invalidateRouteDataCacheLocked()
}
func (l *LockedVorma) SetRouteManifestFile(routeManifestFile string) {
	if l.v._routeManifestFile == routeManifestFile {
		return
	}
	l.v._routeManifestFile = routeManifestFile
	l.v.invalidateRouteDataCacheLocked()
}

// Routes returns the RouteRegistry for route management operations.
func (l *LockedVorma) Routes() *RouteRegistry {
	return &RouteRegistry{vorma: l.v}
}

// --- Thread-safe Setters (acquire lock internally) ---

func (v *Vorma) SetIsDev(isDev bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.setIsDevModeLocked(isDev)
}

func (v *Vorma) setIsDevModeLocked(isDev bool) {
	if v._isDev == isDev {
		return
	}
	v._isDev = isDev
	v.invalidateRouteDataCacheLocked()
}

func clonePath(path *Path) *Path {
	if path == nil {
		return nil
	}
	out := *path
	if path.Deps != nil {
		out.Deps = append([]string(nil), path.Deps...)
	}
	return &out
}

func cloneAdHocTypesOrNil(adHocTypes []*tsgen.AdHocType) []*tsgen.AdHocType {
	if adHocTypes == nil {
		return nil
	}

	cloned := make([]*tsgen.AdHocType, 0, len(adHocTypes))
	for _, adHocType := range adHocTypes {
		if adHocType == nil {
			cloned = append(cloned, nil)
			continue
		}
		copiedAdHocType := *adHocType
		cloned = append(cloned, &copiedAdHocType)
	}
	return cloned
}
