// Package vormaruntime contains Vorma runtime internals used by package vorma.
//
// Application code should import package vorma for public app-facing APIs.
// Build-time orchestration lives in package vormabuild to preserve a strict
// runtime/buildtime split and keep production binaries dependency-light.
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

// VormaSymbolStr is the global runtime symbol namespace used by browser
// integration code.
const VormaSymbolStr = "__vorma_internal__"

// RouteType represents a route classification identifier.
type RouteType = string

type (
	// GetDefaultHeadElsFunc fills default head elements for requests.
	GetDefaultHeadElsFunc func(r *http.Request, app *Vorma, head *headels.HeadEls) error
	// GetHeadDedupeKeysFunc configures head element dedupe keys.
	GetHeadDedupeKeysFunc func(head *headels.HeadEls)
	// GetRootTemplateDataFunc provides per-request root template data.
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

// --- Public Getters ---

// ServerAddr returns the configured server address.
func (v *Vorma) ServerAddr() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._serverAddr
}

// LoadersRouter and ActionsRouter return long-lived router instances.
// These pointers are initialized once and then reused for the app lifetime.
func (v *Vorma) LoadersRouter() *LoadersRouter { return v.loadersRouter }

// ActionsRouter returns the long-lived actions router instance.
func (v *Vorma) ActionsRouter() *ActionsRouter { return v.actionsRouter }

// Paths returns a defensive copy of registered route path metadata.
func (v *Vorma) Paths() map[string]*Path {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return clonePathsMapOrNil(v._paths)
}

// IsDevMode reports whether runtime is currently in development mode.
func (v *Vorma) IsDevMode() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._isDev
}

// BuildID returns the current build identifier.
func (v *Vorma) BuildID() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._buildID
}

// ClientEntryOut returns the built client entry output path.
func (v *Vorma) ClientEntryOut() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._clientEntryOut
}

// ClientEntryDeps returns a defensive copy of client entry dependency paths.
func (v *Vorma) ClientEntryDeps() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return cloneStringSliceOrNil(v._clientEntryDeps)
}

// DepToCSSBundleMap returns a defensive copy of dep-to-css-bundle mappings.
func (v *Vorma) DepToCSSBundleMap() map[string][]string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return cloneDepToCSSBundleMapOrNil(v._depToCSSBundleMap)
}

// RootTemplate returns the current compiled root template.
func (v *Vorma) RootTemplate() *template.Template {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._rootTemplate
}

// RouteManifestFile returns the route manifest asset path.
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

// Paths returns a defensive copy of route path metadata.
func (l *ReadLockedVorma) Paths() map[string]*Path {
	return clonePathsMapOrNil(l.v._paths)
}

// BuildID returns the current build identifier.
func (l *ReadLockedVorma) BuildID() string { return l.v._buildID }

// RouteManifestFile returns the current route manifest file path.
func (l *ReadLockedVorma) RouteManifestFile() string { return l.v._routeManifestFile }

// RootTemplate returns the currently active root template.
func (l *ReadLockedVorma) RootTemplate() *template.Template { return l.v._rootTemplate }

// IsDev reports whether runtime is in development mode.
func (l *ReadLockedVorma) IsDev() bool { return l.v._isDev }

// Vorma returns the underlying mutable runtime instance.
func (l *LockedVorma) Vorma() *Vorma { return l.v }

// Paths returns a defensive copy of route path metadata.
func (l *LockedVorma) Paths() map[string]*Path {
	return clonePathsMapOrNil(l.v._paths)
}

// BuildID returns the current build identifier.
func (l *LockedVorma) BuildID() string { return l.v._buildID }

// RouteManifestFile returns the current route manifest file path.
func (l *LockedVorma) RouteManifestFile() string { return l.v._routeManifestFile }

// RootTemplate returns the currently active root template.
func (l *LockedVorma) RootTemplate() *template.Template { return l.v._rootTemplate }

// IsDev reports whether runtime is in development mode.
func (l *LockedVorma) IsDev() bool { return l.v._isDev }

// --- LockedVorma Setters ---

// SetPaths replaces parsed paths and runs route-registry synchronization.
func (l *LockedVorma) SetPaths(paths map[string]*Path) {
	l.v.routes().ReplaceParsedPathsForInit(paths, false)
}

// SetIsDev updates runtime mode and invalidates route-data cache if changed.
func (l *LockedVorma) SetIsDev(isDev bool) { l.v.setIsDevModeLocked(isDev) }

// SetBuildID updates build id and invalidates route-data cache when changed.
func (l *LockedVorma) SetBuildID(buildID string) {
	if l.v._buildID == buildID {
		return
	}
	l.v._buildID = buildID
	l.v.invalidateRouteDataCacheLocked()
}

// SetRouteManifestFile updates route manifest reference and invalidates
// route-data cache when changed.
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

// SetIsDev updates runtime dev-mode flag with internal locking.
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
