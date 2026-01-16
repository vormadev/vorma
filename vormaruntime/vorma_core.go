package vormaruntime

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"github.com/vormadev/vorma/kit/headels"
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

	getDefaultHeadEls   GetDefaultHeadElsFunc
	getHeadDedupeKeys   GetHeadDedupeKeysFunc
	getRootTemplateData GetRootTemplateDataFunc

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

	// Config for TS Generation
	_adHocTypes  []*tsgen.AdHocType
	_extraTSCode string
}

// --- Public Getters (thread-safe, acquire read lock) ---

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

func (v *Vorma) GetDepToCSSBundleMap() map[string][]string {
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

// --- LockedVorma Pattern ---
// LockedVorma provides compile-time safe access to fields that require the lock.
// Use WithLock to obtain a LockedVorma instance.

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

// WithRLock acquires the read lock and calls fn with a LockedVorma.
// The lock is released when fn returns. Use this for read-only operations.
func (v *Vorma) WithRLock(fn func(*LockedVorma)) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	fn(&LockedVorma{v: v})
}

// --- LockedVorma Getters ---

// Vorma returns the underlying Vorma instance for accessing non-lock-protected fields.
func (l *LockedVorma) Vorma() *Vorma                       { return l.v }
func (l *LockedVorma) GetPaths() map[string]*Path          { return l.v._paths }
func (l *LockedVorma) GetBuildID() string                  { return l.v._buildID }
func (l *LockedVorma) GetRouteManifestFile() string        { return l.v._routeManifestFile }
func (l *LockedVorma) GetRootTemplate() *template.Template { return l.v._rootTemplate }
func (l *LockedVorma) GetIsDev() bool                      { return l.v._isDev }

// --- LockedVorma Setters ---

func (l *LockedVorma) SetPaths(paths map[string]*Path)      { l.v._paths = paths }
func (l *LockedVorma) SetBuildID(id string)                 { l.v._buildID = id }
func (l *LockedVorma) SetRouteManifestFile(f string)        { l.v._routeManifestFile = f }
func (l *LockedVorma) SetRootTemplate(t *template.Template) { l.v._rootTemplate = t }

// Routes returns the RouteRegistry for route management operations.
func (l *LockedVorma) Routes() *RouteRegistry {
	return &RouteRegistry{vorma: l.v}
}

// --- Thread-safe Setters (acquire lock internally) ---

func (v *Vorma) SetIsDev(isDev bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v._isDev = isDev
}
