// Package vormaruntime owns Vorma runtime bootstrap, state, and public runtime
// APIs.
//
// This package exists so application code can initialize one coherent runtime
// engine without coupling directly to lower-level routing/template internals.
package vormaruntime

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"

	"github.com/vormadev/vorma/internal/outputprefix"
	"github.com/vormadev/vorma/internal/vormaruntime/rendering"
	"github.com/vormadev/vorma/internal/vormaruntime/routepipeline"
	"github.com/vormadev/vorma/internal/vormaruntime/routepublic"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimeconfig"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimecore"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimehttp"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/waveframework"
)

// VormaBuildIDHeaderKey carries the runtime build id in responses.
const VormaBuildIDHeaderKey = "X-Wave-Framework-Build-Id"

// VormaJSONQueryKey toggles JSON route-data response mode for loaders.
const VormaJSONQueryKey = "vorma_json"

// LoadersHandler returns the main GET handler for loader and document
// rendering requests.
func (v *Vorma) LoadersHandler() mux.TasksCtxRequirerFunc {
	v.loadersHandlerOnce.Do(func() {
		v.ensureLoaderPatternsRegisteredForHandler()
		nestedRouter := v.LoadersRouter().NestedRouter
		executeRouteDataStageOne := func(
			responseWriter http.ResponseWriter,
			request *http.Request,
			nestedRouterForRequest *nestedmux.Router,
			requestedBuildID string,
		) *routepipeline.RouteResult {
			return runtimehttp.ExecuteRouteDataStageOne(
				runtimehttp.RouteDataStageOneInput{
					ResponseWriter:   responseWriter,
					Request:          request,
					NestedRouter:     nestedRouterForRequest,
					RequestedBuildID: requestedBuildID,
					BuildIDHeaderKey: VormaBuildIDHeaderKey,
					PrepareExecutionInputs: func(
						request *http.Request,
						nestedRouter *nestedmux.Router,
					) (routepipeline.RouteDataExecutionInputs, bool) {
						return v.prepareRouteDataExecutionInputs(
							request,
							nestedRouter,
						)
					},
					ResolveClientLoaderMessage: v.resolveClientLoaderErrorMessage,
					Log:                        v.Log,
				},
			)
		}
		var getDefaultHeadElementsForRouteData func(*http.Request) ([]*htmlutil.Element, error)
		if v.getDefaultHeadEls != nil {
			getDefaultHeadElementsForRouteData = v.getDefaultHeadElsRaw
		}
		buildResolvedRouteData := func(
			responseWriter http.ResponseWriter,
			routeResult *routepipeline.RouteResult,
			isJSON bool,
			defaultHeadElements []*htmlutil.Element,
		) *routepipeline.RouteResult {
			return runtimehttp.BuildUIRouteDataFromResolvedRoute(
				runtimehttp.BuildUIRouteDataFromResolvedRouteInput{
					ResponseWriter:      responseWriter,
					RouteResult:         routeResult,
					IsJSON:              isJSON,
					DefaultHeadElements: defaultHeadElements,
					PublicPathPrefix:    v.Wave.PublicPathPrefix(),
					ToSortedAndPreEscapedHeadEls: v.headElsInst.
						ToSortedAndPreEscapedHeadEls,
					Log: v.Log,
				},
			)
		}
		resolveUIRouteData := func(
			responseWriter http.ResponseWriter,
			request *http.Request,
			nestedRouterForRequest *nestedmux.Router,
			isJSON bool,
			requestedBuildID string,
		) *routepipeline.RouteResult {
			return runtimehttp.ResolveUIRouteData(
				runtimehttp.ResolveUIRouteDataInput{
					ResponseWriter:           responseWriter,
					Request:                  request,
					NestedRouter:             nestedRouterForRequest,
					IsJSON:                   isJSON,
					RequestedBuildID:         requestedBuildID,
					ExecuteRouteDataStageOne: executeRouteDataStageOne,
					GetDefaultHeadElements:   getDefaultHeadElementsForRouteData,
					BuildResolvedRouteData:   buildResolvedRouteData,
					Log:                      v.Log,
				},
			)
		}
		v.loadersHandler = runtimehttp.BuildLoadersHandler(
			runtimehttp.BuildLoadersHandlerInput{
				NestedRouter:       nestedRouter,
				JSONQueryKey:       VormaJSONQueryKey,
				BuildIDHeaderKey:   VormaBuildIDHeaderKey,
				ResolveUIRouteData: resolveUIRouteData,
				PublicPathPrefix: func() string {
					return v.Wave.PublicPathPrefix()
				},
				BuildLoadersHTMLResponseBytes: v.buildLoadersHTMLResponseBytes,
				Log:                           v.Log,
			},
		)
	})
	return v.loadersHandler
}

// DevReloadRoutesEndpointPath returns the configured (or default) dev routes
// reload endpoint path.
func (v *Vorma) DevReloadRoutesEndpointPath() string {
	if v == nil || v.Config == nil {
		return runtimeconfig.DefaultDevReloadRoutesEndpointPath
	}
	return runtimeconfig.ResolveDevReloadRoutesEndpointPath(
		v.Config.DevReloadRoutesEndpointPath,
	)
}

// DevReloadTemplateEndpointPath returns the configured (or default) dev
// template reload endpoint path.
func (v *Vorma) DevReloadTemplateEndpointPath() string {
	if v == nil || v.Config == nil {
		return runtimeconfig.DefaultDevReloadTemplateEndpointPath
	}
	return runtimeconfig.ResolveDevReloadTemplateEndpointPath(
		v.Config.DevReloadTemplateEndpointPath,
	)
}

// TemplateDataKeyHeadElements returns the template data key used for rendered
// head elements.
func (v *Vorma) TemplateDataKeyHeadElements() string {
	if v == nil || v.Config == nil {
		return runtimeconfig.DefaultTemplateDataKeyHeadElements
	}
	return runtimeconfig.ResolveTemplateDataKeyHeadElements(
		v.Config.TemplateDataKeyHeadElements,
	)
}

// TemplateDataKeyBodyScripts returns the template data key used for rendered
// body scripts.
func (v *Vorma) TemplateDataKeyBodyScripts() string {
	if v == nil || v.Config == nil {
		return runtimeconfig.DefaultTemplateDataKeyBodyScripts
	}
	return runtimeconfig.ResolveTemplateDataKeyBodyScripts(
		v.Config.TemplateDataKeyBodyScripts,
	)
}

// TemplateDataKeySSRScript returns the template data key used for SSR inner
// HTML.
func (v *Vorma) TemplateDataKeySSRScript() string {
	if v == nil || v.Config == nil {
		return runtimeconfig.DefaultTemplateDataKeySSRScript
	}
	return runtimeconfig.ResolveTemplateDataKeySSRScript(
		v.Config.TemplateDataKeySSRScript,
	)
}

// TemplateDataKeySSRScriptHash returns the template data key used for the SSR
// script hash.
func (v *Vorma) TemplateDataKeySSRScriptHash() string {
	if v == nil || v.Config == nil {
		return runtimeconfig.DefaultTemplateDataKeySSRScriptHash
	}
	return runtimeconfig.ResolveTemplateDataKeySSRScriptHash(
		v.Config.TemplateDataKeySSRScriptHash,
	)
}

// TemplateDataKeyRootElementID returns the template data key used for the
// client root element id.
func (v *Vorma) TemplateDataKeyRootElementID() string {
	if v == nil || v.Config == nil {
		return runtimeconfig.DefaultTemplateDataKeyRootElementID
	}
	return runtimeconfig.ResolveTemplateDataKeyRootElementID(
		v.Config.TemplateDataKeyRootElementID,
	)
}

// ClientRootElementID returns the configured (or default) client mount root id.
func (v *Vorma) ClientRootElementID() string {
	if v == nil || v.Config == nil {
		return runtimeconfig.DefaultClientRootElementID
	}
	return runtimeconfig.ResolveClientRootElementID(
		v.Config.ClientRootElementID,
	)
}

func (v *Vorma) buildLoadersHTMLResponseBytes(
	r *http.Request,
	routeResult *routepipeline.RouteResult,
	routeData *routepipeline.RouteDataFinal,
) ([]byte, string, error) {
	return runtimehttp.BuildLoadersHTMLResponseBytesForRuntime(
		runtimehttp.BuildLoadersHTMLResponseBytesForRuntimeInput{
			Request:                      r,
			RouteResult:                  routeResult,
			RouteData:                    routeData,
			GetRootTemplateDataOrEmpty:   v.getRootTemplateDataOrEmpty,
			RenderHeadElements:           v.headElsInst.Render,
			CriticalCSSStyleElement:      v.Wave.CriticalCSSStyleElement(),
			StyleSheetLinkElement:        v.Wave.StyleSheetLinkElement(),
			VormaSymbolStr:               VormaSymbolStr,
			TemplateDataKeyHeadElements:  v.TemplateDataKeyHeadElements(),
			TemplateDataKeyBodyScripts:   v.TemplateDataKeyBodyScripts(),
			TemplateDataKeySSRScript:     v.TemplateDataKeySSRScript(),
			TemplateDataKeySSRScriptHash: v.TemplateDataKeySSRScriptHash(),
			TemplateDataKeyRootElementID: v.TemplateDataKeyRootElementID(),
			ClientRootElementID:          v.ClientRootElementID(),
			PublicPathPrefix:             v.Wave.PublicPathPrefix(),
			RefreshScript:                v.Wave.RefreshScript(),
			ClientEntry:                  v.Config.ClientEntry,
			UseReactVariant: UIVariant(
				v.Config.UIVariant,
			) == UIVariantReact,
		},
	)
}

func (v *Vorma) getRootTemplateDataOrEmpty(
	r *http.Request,
) (map[string]any, error) {
	if v.getRootTemplateData == nil {
		return make(map[string]any), nil
	}

	rootTemplateData, err := v.getRootTemplateData(r)
	if err != nil {
		return nil, err
	}
	if rootTemplateData == nil {
		return make(map[string]any), nil
	}

	return rendering.CloneTemplateDataMap(rootTemplateData), nil
}

// IsJSONRequest reports whether a request asks for JSON loader data response.
func IsJSONRequest(r *http.Request) bool {
	return r.URL.Query().Get(VormaJSONQueryKey) != ""
}

// IsCurrentBuildJSONRequest reports whether a JSON request explicitly targets
// the current runtime build id.
func (v *Vorma) IsCurrentBuildJSONRequest(r *http.Request) bool {
	return r.URL.Query().Get(VormaJSONQueryKey) == v.BuildID()
}

// ActionsHandler returns the task-backed HTTP handler for action routes.
func (v *Vorma) ActionsHandler() mux.TasksCtxRequirerFunc {
	v.actionsHandlerOnce.Do(func() {
		router := v.ActionsRouter().Router
		v.actionsHandler = runtimehttp.BuildActionsHandler(
			runtimehttp.BuildActionsHandlerInput{
				Router:         router,
				CurrentBuildID: v.BuildID,
				IsDevMode:      v.IsDevMode,
				RoutesEndpointPath: func() string {
					return v.DevReloadRoutesEndpointPath()
				},
				TemplateEndpointPath: func() string {
					return v.DevReloadTemplateEndpointPath()
				},
				ValidateExpectedBuildIDOrWriteConflict: func(
					responseWriter http.ResponseWriter,
					request *http.Request,
				) bool {
					return runtimehttp.ValidateExpectedBuildIDOrWriteConflict(
						runtimehttp.ValidateExpectedBuildIDOrWriteConflictInput{
							ResponseWriter: responseWriter,
							Request:        request,
							CurrentBuildID: v.BuildID(),
							ExpectedBuildIDHeaderName: waveframework.
								FrameworkRuntimeReloadExpectedBuildIDHeaderName,
							Log: v.Log,
						},
					)
				},
				ReloadRoutesFromDisk:   v.devReloadRoutesFromDisk,
				ReloadTemplateFromDisk: v.devReloadTemplateFromDisk,
				Log:                    v.Log,
			},
		)
	})
	return v.actionsHandler
}

// LoadersRouter wraps nestedmux.Router for loader route registration.
type LoadersRouter = runtimehttp.LoadersRouter

// ActionsRouter wraps mux.Router for action route registration.
type ActionsRouter = runtimehttp.ActionsRouter

// LoaderReqData is loader task request data.
type LoaderReqData = nestedmux.ReqData

// ActionReqData is action task request data.
type ActionReqData[I any] = mux.ReqData[I]

// LoadersRouterOptions configures loader route pattern semantics.
type LoadersRouterOptions = runtimehttp.LoadersRouterSpec

// ActionsRouterOptions configures action route matching and accepted methods.
type ActionsRouterOptions = runtimehttp.ActionsRouterSpec

// FormData is used as the input type for actions that accept form data.
type FormData struct{}

// TSTypeRaw returns the TypeScript-side type name for generated declarations.
func (m FormData) TSTypeRaw() string { return "FormData" }

// VormaAppConfig configures a Vorma runtime app instance.
type VormaAppConfig struct {
	Wave                 *wave.Wave
	DefaultHeadElsFunc   GetDefaultHeadElsFunc
	HeadDedupeKeysFunc   GetHeadDedupeKeysFunc
	RootTemplateDataFunc GetRootTemplateDataFunc
	LoadersRouterOptions LoadersRouterOptions
	ActionsRouterOptions ActionsRouterOptions
	AdHocTypes           []*tsgen.AdHocType
	ExtraTSCode          string
	Logger               *slog.Logger
}

// NewVormaApp constructs a Vorma runtime from Wave config and runtime hooks.
func NewVormaApp(o VormaAppConfig) *Vorma {
	var v Vorma

	v.Wave = o.Wave
	if v.Wave == nil {
		panic("Wave instance is required")
	}

	if o.Logger != nil {
		v.Log = o.Logger
	} else {
		v.Log = colorlog.New("vorma")
	}

	config, configErr := runtimeconfig.ParseAndValidateVormaConfig(
		v.Wave.RawConfigJSON(),
	)
	if configErr != nil {
		panic(configErr)
	}
	v.Config = config

	v.getDefaultHeadEls = o.DefaultHeadElsFunc
	v.getHeadDedupeKeys = o.HeadDedupeKeysFunc
	if v.getHeadDedupeKeys == nil {
		v.getHeadDedupeKeys = func(h *headels.HeadEls) {}
	}
	v.getRootTemplateData = o.RootTemplateDataFunc
	if v.getRootTemplateData == nil {
		v.getRootTemplateData = func(r *http.Request) (map[string]any, error) {
			return map[string]any{}, nil
		}
	}

	v._adHocTypes = cloneAdHocTypesOrNil(o.AdHocTypes)
	v._extraTSCode = o.ExtraTSCode
	v.loadersRouter = runtimehttp.NewLoadersRouter(
		runtimehttp.LoadersRouterSpec(o.LoadersRouterOptions),
	)
	actionsRouterSpec := runtimehttp.ActionsRouterSpec(o.ActionsRouterOptions)
	actionsRouterSpec.IsFormDataInput = func(inputPtr any) bool {
		_, isFormData := inputPtr.(*FormData)
		return isFormData
	}
	v.actionsRouter = runtimehttp.NewActionsRouter(actionsRouterSpec)
	v.headElsInst = headels.NewInstance("vorma")
	v._routeDataCache = &sync.Map{}
	v._lifecycleState = runtimecore.LifecycleStateUninitialized

	return &v
}

// Loaders exposes convenience helpers for mounting loader handlers.
type Loaders struct{ vorma *Vorma }

// Actions exposes convenience helpers for mounting action handlers.
type Actions struct{ vorma *Vorma }

// MustStaticMiddleware returns static asset middleware configured for Vorma runtime.
func (v *Vorma) MustStaticMiddleware() func(http.Handler) http.Handler {
	return v.Wave.MustStaticMiddleware(true)
}

// Loaders returns loader mounting helpers.
func (v *Vorma) Loaders() *Loaders { return &Loaders{vorma: v} }

// Actions returns action mounting helpers.
func (v *Vorma) Actions() *Actions { return &Actions{vorma: v} }

// HandlerMountPattern returns the mount pattern for loader handlers.
func (h *Loaders) HandlerMountPattern() string { return "/*" }

// Handler returns the loader HTTP handler.
func (h *Loaders) Handler() http.Handler {
	return h.vorma.LoadersHandler()
}

// HandlerMountPattern returns the mount pattern for action handlers.
func (h *Actions) HandlerMountPattern() string {
	return h.vorma.ActionsRouter().MountRoot("*")
}

// Handler returns the action HTTP handler.
func (h *Actions) Handler() http.Handler {
	return h.vorma.ActionsHandler()
}

// SupportedMethods returns a copy of methods accepted by the actions router.
func (h *Actions) SupportedMethods() map[string]bool {
	return h.vorma.ActionsRouter().SupportedMethodsClone()
}

// Route aliases mux.Route for public runtime surface continuity.
type Route[I any, O any] = mux.Route[I, O]

// TaskHandler aliases mux.TaskHandler for public runtime surface continuity.
type TaskHandler[I any, O any] = mux.TaskHandler[I, O]

func (v *Vorma) prepareRouteDataExecutionInputs(
	r *http.Request,
	nestedRouter *nestedmux.Router,
) (routepipeline.RouteDataExecutionInputs, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	runtimeSnapshot := routepipeline.BuildRuntimeSnapshotFromCore(
		routepipeline.RuntimeSnapshotFromCoreInput{
			BuildID:                  v._buildID,
			IsDev:                    v._isDev,
			Paths:                    v._paths,
			ClientEntryDeps:          v._clientEntryDeps,
			ClientEntryOut:           v._clientEntryOut,
			DepToCSSBundleMap:        v._depToCSSBundleMap,
			RootTemplate:             v._rootTemplate,
			RouteManifestFile:        v._routeManifestFile,
			RouteDataSnapshotVersion: v._routeDataSnapshotVersion,
			RouteDataCache:           v._routeDataCache,
		},
	)
	currentSnapshotVersion := runtimeSnapshot.RouteDataSnapshotVersion
	return runtimehttp.PrepareExecutionInputs(
		runtimehttp.PrepareExecutionInputsInput{
			Request:         r,
			NestedRouter:    nestedRouter,
			RuntimeSnapshot: runtimeSnapshot,
			IsSnapshotVersionCurrent: func(expectedSnapshotVersion uint64) bool {
				return expectedSnapshotVersion == currentSnapshotVersion
			},
		},
	)
}

func (v *Vorma) resolveClientLoaderErrorMessage(
	err error,
	pattern string,
) string {
	var clientMsg string
	var errToLog error

	var loaderErr LoaderErrorMarker
	if errors.As(err, &loaderErr) {
		clientMsg = loaderErr.ClientMessage()
		errToLog = loaderErr.ServerError()
		if clientMsg == "" {
			clientMsg = "An error occurred"
			v.Log.Warn(
				"LoaderError has empty ClientMessage(); sending generic error to client.",
			)
		}
	} else {
		clientMsg = "An error occurred"
		errToLog = err
		v.Log.Warn("Sending generic error to client. Use vorma.LoaderError for custom client messages.")
	}

	if errToLog != nil {
		v.Log.Error("loader error", "pattern", pattern, "error", errToLog)
	}
	return clientMsg
}

func (v *Vorma) getDefaultHeadElsRaw(
	r *http.Request,
) ([]*htmlutil.Element, error) {
	defaultHeadEls := headels.New()
	if v.getDefaultHeadEls == nil {
		return defaultHeadEls.Collect(), nil
	}
	if err := v.getDefaultHeadEls(r, v, defaultHeadEls); err != nil {
		return nil, err
	}
	return defaultHeadEls.Collect(), nil
}

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
	_paths             map[string]*runtimecore.RoutePath
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

	_lifecycleState         runtimecore.LifecycleState
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
	return routepublic.CloneRuntimeCorePathsMapAsPublicOrNil(v._paths)
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
	return runtimecore.CloneStringSliceOrNil(v._clientEntryDeps)
}

// DepToCSSBundleMap returns a defensive copy of dep-to-css-bundle mappings.
func (v *Vorma) DepToCSSBundleMap() map[string][]string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return runtimecore.CloneDepToCSSBundleMapOrNil(v._depToCSSBundleMap)
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
	return routepublic.CloneRuntimeCorePathsMapAsPublicOrNil(l.v._paths)
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
	return routepublic.CloneRuntimeCorePathsMapAsPublicOrNil(l.v._paths)
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
	return l.v.routes()
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

// LoaderErrorMarker is an interface for loader errors that can provide
// separate client and server error messages.
type LoaderErrorMarker interface {
	__isLoaderError()
	ClientMessage() string
	ServerError() error
}

// LoaderError represents an error from a loader with separate client
// and server error messages.
type LoaderError struct {
	Client string
	Server error
}

func (e *LoaderError) Error() string {
	if e == nil {
		return "loader error"
	}
	if e.Server != nil {
		return e.Server.Error()
	}
	if e.Client != "" {
		return e.Client
	}
	return "loader error"
}

func (e *LoaderError) __isLoaderError()      {}
func (e *LoaderError) ClientMessage() string { return e.Client }
func (e *LoaderError) ServerError() error    { return e.Server }

// Path aliases the routepublic public route-path shape.
type Path = routepublic.Path

// UIVariant aliases runtimeconfig UI variant constants.
type UIVariant = runtimeconfig.UIVariant

const (
	UIVariantReact  = runtimeconfig.UIVariantReact
	UIVariantPreact = runtimeconfig.UIVariantPreact
	UIVariantSolid  = runtimeconfig.UIVariantSolid
)

const (
	UnresolvedRoutePolicyWarn  = runtimeconfig.UnresolvedRoutePolicyWarn
	UnresolvedRoutePolicyError = runtimeconfig.UnresolvedRoutePolicyError
)

// VormaConfig aliases runtimeconfig Vorma config schema.
type VormaConfig = runtimeconfig.VormaConfig

// Output prefix constants.
const (
	VormaOutPrefix               = outputprefix.HashedOutputPrefix
	VormaVitePrehashedFilePrefix = outputprefix.VitePrehashedFilePrefix
	VormaRouteManifestPrefix     = outputprefix.VormaRouteManifestPrefix
)

func (v *Vorma) transitionLifecycleStateLocked(
	nextState runtimecore.LifecycleState,
	reason string,
	lastError string,
) {
	stateTracker := runtimecore.LifecycleStateTracker{
		CurrentState:      v._lifecycleState,
		TransitionSeq:     v._lifecycleTransitionSeq,
		LastError:         v._lifecycleLastError,
		RoutePathsPresent: v._paths != nil,
	}
	transitionResult := runtimecore.TransitionLifecycleState(
		&stateTracker,
		nextState,
		reason,
		lastError,
	)
	v._lifecycleState = stateTracker.CurrentState
	v._lifecycleTransitionSeq = stateTracker.TransitionSeq
	v._lifecycleLastError = stateTracker.LastError

	v.Log.Debug(
		"Vorma lifecycle transition",
		"seq",
		transitionResult.TransitionSeq,
		"from",
		transitionResult.PreviousState,
		"to",
		transitionResult.CurrentState,
		"reason",
		reason,
		"error",
		transitionResult.CurrentError,
		"build_id",
		v._buildID,
		"route_data_snapshot_version",
		v._routeDataSnapshotVersion,
		"is_dev",
		v._isDev,
		"route_count",
		len(v._paths),
		"route_manifest_file",
		v._routeManifestFile,
	)
}

func (v *Vorma) lifecycleStateForRouteCommitLocked() runtimecore.LifecycleState {
	return runtimecore.LifecycleStateForRouteCommit(v._paths)
}

func (v *Vorma) commitRouteArtifactsLocked(
	artifacts *runtimecore.RuntimeRouteArtifacts,
	rebuildNestedRouter bool,
	commitMode runtimecore.RouteArtifactCommitMode,
) {
	if artifacts == nil {
		panic("runtime route artifacts cannot be nil")
	}

	v.applyRuntimeRouteArtifactsMetadataLocked(artifacts)

	switch commitMode {
	case runtimecore.RouteArtifactCommitModeDevReload:
		v.routes().SyncFromDevReloadRuntimeCore(
			runtimecore.CloneRoutePathsOrNil(
				artifacts.ParsedClientPaths,
			),
		)
	default:
		v.routes().ReplaceParsedPathsForInitRuntimeCore(
			runtimecore.CloneRoutePathsOrNil(
				artifacts.ParsedClientPaths,
			),
			rebuildNestedRouter,
		)
	}
}

func (v *Vorma) commitRootTemplateLocked(rootTemplate *template.Template) {
	v._rootTemplate = rootTemplate
}

// invalidateRouteDataCacheLocked invalidates route-data cache entries for
// requests that still reference an older runtime snapshot.
//
// Caller must hold v.mu.Lock().
func (v *Vorma) invalidateRouteDataCacheLocked() {
	cacheState := runtimecore.RouteCacheState{
		RouteDataSnapshotVersion: v._routeDataSnapshotVersion,
		RouteDataCache:           v._routeDataCache,
	}
	runtimecore.InvalidateRouteDataCache(&cacheState)
	v._routeDataSnapshotVersion = cacheState.RouteDataSnapshotVersion
	v._routeDataCache = cacheState.RouteDataCache
}

func (v *Vorma) applyRuntimeRouteArtifactsMetadataLocked(
	artifacts *runtimecore.RuntimeRouteArtifacts,
) {
	state := runtimecore.RuntimeRouteMetadataState{
		BuildID:           v._buildID,
		ClientEntrySrc:    v._clientEntrySrc,
		ClientEntryOut:    v._clientEntryOut,
		ClientEntryDeps:   v._clientEntryDeps,
		DepToCSSBundleMap: v._depToCSSBundleMap,
		RouteManifestFile: v._routeManifestFile,
	}
	runtimecore.ApplyRuntimeRouteArtifactsMetadata(&state, artifacts)

	v._buildID = state.BuildID
	v._clientEntrySrc = state.ClientEntrySrc
	v._clientEntryOut = state.ClientEntryOut
	v._clientEntryDeps = state.ClientEntryDeps
	v._depToCSSBundleMap = state.DepToCSSBundleMap
	v._routeManifestFile = state.RouteManifestFile
}

// routeMutationControllerDependencies configures how route mutation operations
// read/write runtime state and update nested-router patterns.
type routeMutationControllerDependencies struct {
	ReadMutableState                    func() runtimecore.RouteMutableState
	WriteMutableState                   func(runtimecore.RouteMutableState)
	ServerRoutePatternsWithTaskHandlers func() []string
	RebuildNestedRouterFromCurrentPaths func(map[string]*runtimecore.RoutePath)
}

// routeMutationController executes route-state mutation operations.
type routeMutationController struct {
	dependencies routeMutationControllerDependencies
}

func newRouteMutationControllerWithDependencies(
	dependencies routeMutationControllerDependencies,
) *routeMutationController {
	if dependencies.ReadMutableState == nil {
		panic(
			"vormaruntime.newRouteMutationControllerWithDependencies: ReadMutableState is required",
		)
	}
	if dependencies.WriteMutableState == nil {
		panic(
			"vormaruntime.newRouteMutationControllerWithDependencies: WriteMutableState is required",
		)
	}
	if dependencies.ServerRoutePatternsWithTaskHandlers == nil {
		panic(
			"vormaruntime.newRouteMutationControllerWithDependencies: ServerRoutePatternsWithTaskHandlers is required",
		)
	}
	if dependencies.RebuildNestedRouterFromCurrentPaths == nil {
		panic(
			"vormaruntime.newRouteMutationControllerWithDependencies: RebuildNestedRouterFromCurrentPaths is required",
		)
	}
	return &routeMutationController{dependencies: dependencies}
}

// SyncFromDevReload merges parsed client paths while preserving server-only
// handler routes and rebuilding nested-router patterns.
func (controller *routeMutationController) SyncFromDevReload(
	paths map[string]*runtimecore.RoutePath,
) {
	if controller == nil {
		panic(
			"vormaruntime.routeMutationController.SyncFromDevReload: controller is nil",
		)
	}
	mutableState := controller.dependencies.ReadMutableState()
	runtimecore.SyncRouteStateFromDevReload(
		runtimecore.SyncRouteStateFromDevReloadInput{
			State:                               &mutableState,
			ParsedClientPaths:                   paths,
			ServerRoutePatternsWithTaskHandlers: controller.dependencies.ServerRoutePatternsWithTaskHandlers(),
			RebuildNestedRouterFromCurrentPaths: controller.dependencies.RebuildNestedRouterFromCurrentPaths,
		},
	)
	controller.dependencies.WriteMutableState(mutableState)
}

// ReplaceParsedPathsForInit replaces parsed client paths for init/re-init
// flows, optionally rebuilding nested-router patterns.
func (controller *routeMutationController) ReplaceParsedPathsForInit(
	paths map[string]*runtimecore.RoutePath,
	rebuildNestedRouter bool,
) {
	if controller == nil {
		panic(
			"vormaruntime.routeMutationController.ReplaceParsedPathsForInit: controller is nil",
		)
	}
	mutableState := controller.dependencies.ReadMutableState()
	runtimecore.ReplaceRouteStateForInit(
		runtimecore.ReplaceRouteStateForInitInput{
			State:                               &mutableState,
			ParsedClientPaths:                   paths,
			RebuildNestedRouter:                 rebuildNestedRouter,
			RebuildNestedRouterFromCurrentPaths: controller.dependencies.RebuildNestedRouterFromCurrentPaths,
		},
	)
	controller.dependencies.WriteMutableState(mutableState)
}

// RouteRegistry consolidates route-state mutation operations.
type RouteRegistry struct {
	controller *routeMutationController
}

func newRouteRegistry(controller *routeMutationController) *RouteRegistry {
	if controller == nil {
		panic("vormaruntime.newRouteRegistry: controller is required")
	}
	return &RouteRegistry{controller: controller}
}

// SyncFromDevReload merges parsed client paths while preserving server-only
// handlers.
func (registry *RouteRegistry) SyncFromDevReload(
	paths map[string]*routepublic.Path,
) {
	registry.SyncFromDevReloadRuntimeCore(
		routepublic.ToRuntimeCoreRoutePaths(paths),
	)
}

// SyncFromDevReloadRuntimeCore merges parsed client paths in runtimecore shape.
func (registry *RouteRegistry) SyncFromDevReloadRuntimeCore(
	paths map[string]*runtimecore.RoutePath,
) {
	if registry == nil || registry.controller == nil {
		panic(
			"vormaruntime.RouteRegistry.SyncFromDevReloadRuntimeCore: registry is nil",
		)
	}
	registry.controller.SyncFromDevReload(paths)
}

// ReplaceParsedPathsForInit replaces parsed paths for init/re-init flows.
func (registry *RouteRegistry) ReplaceParsedPathsForInit(
	paths map[string]*routepublic.Path,
	rebuildNestedRouter bool,
) {
	registry.ReplaceParsedPathsForInitRuntimeCore(
		routepublic.ToRuntimeCoreRoutePaths(paths),
		rebuildNestedRouter,
	)
}

// ReplaceParsedPathsForInitRuntimeCore replaces parsed paths in runtimecore
// shape for init/re-init flows.
func (registry *RouteRegistry) ReplaceParsedPathsForInitRuntimeCore(
	paths map[string]*runtimecore.RoutePath,
	rebuildNestedRouter bool,
) {
	if registry == nil || registry.controller == nil {
		panic(
			"vormaruntime.RouteRegistry.ReplaceParsedPathsForInitRuntimeCore: registry is nil",
		)
	}
	registry.controller.ReplaceParsedPathsForInit(
		paths,
		rebuildNestedRouter,
	)
}

// routes returns a RouteRegistry for route management operations.
// This is unexported to enforce that external packages use
// LockedVorma.Routes() via WithLock for compile-time safety.
func (v *Vorma) routes() *RouteRegistry {
	return newRouteRegistry(newRouteMutationController(v))
}

func newRouteMutationController(v *Vorma) *routeMutationController {
	return newRouteMutationControllerWithDependencies(
		routeMutationControllerDependencies{
			ReadMutableState: func() runtimecore.RouteMutableState {
				return runtimecore.RouteMutableState{
					Paths:                    v._paths,
					RouteDataSnapshotVersion: v._routeDataSnapshotVersion,
					RouteDataCache:           v._routeDataCache,
				}
			},
			WriteMutableState: func(mutableState runtimecore.RouteMutableState) {
				v._paths = mutableState.Paths
				v._routeDataSnapshotVersion = mutableState.RouteDataSnapshotVersion
				v._routeDataCache = mutableState.RouteDataCache
			},
			ServerRoutePatternsWithTaskHandlers: func() []string {
				allServerRoutes := v.LoadersRouter().NestedRouter.AllRoutes()
				patterns := make([]string, 0, len(allServerRoutes))
				for pattern := range allServerRoutes {
					if !v.LoadersRouter().NestedRouter.HasTaskHandler(pattern) {
						continue
					}
					patterns = append(patterns, pattern)
				}
				return patterns
			},
			RebuildNestedRouterFromCurrentPaths: func(
				paths map[string]*runtimecore.RoutePath,
			) {
				patterns := runtimecore.BuildNestedRouterPatternList(paths)
				v.LoadersRouter().NestedRouter.RebuildPreservingHandlers(
					patterns,
				)
			},
		},
	)
}

// RegisterPatternIfNeeded registers a pattern if not already registered.
// This method is safe to call without holding the lock as the router handles
// its own synchronization.
func (v *Vorma) RegisterPatternIfNeeded(pattern string) {
	v.LoadersRouter().NestedRouter.AddPatternWithoutHandlerIfMissing(
		pattern,
	)
}

// devReloadRoutesFromDisk reloads route configuration from JSON files on disk.
// Called by Process B when Process A has regenerated route artifacts.
// This does NOT regenerate TypeScript; Process A does that work.
func (v *Vorma) devReloadRoutesFromDisk() error {
	v.mu.RLock()
	isDev := v._isDev
	privateFS := v._privateFS
	v.mu.RUnlock()

	if err := guardDevOnlyReload(isDev, "route reload"); err != nil {
		return err
	}

	routeArtifactsLoadOutput, routeArtifactsLoadError := runtimepaths.LoadRouteArtifactsFromFS(
		privateFS,
		true,
	)
	if routeArtifactsLoadError != nil {
		return fmt.Errorf("load paths from disk: %w", routeArtifactsLoadError)
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if err := guardDevOnlyReload(v._isDev, "route reload"); err != nil {
		return err
	}
	v.transitionLifecycleStateLocked(
		runtimecore.LifecycleStateReloadingRoutes,
		"dev route artifacts commit start",
		"",
	)
	v.commitRouteArtifactsLocked(
		routeArtifactsLoadOutput.RuntimeArtifacts,
		true,
		runtimecore.RouteArtifactCommitModeDevReload,
	)
	v.transitionLifecycleStateLocked(
		runtimecore.LifecycleStateReady,
		"dev route artifacts commit complete",
		"",
	)
	v.Log.Info("Routes reloaded from disk", "buildID", v._buildID)
	return nil
}

// devReloadTemplateFromDisk re-parses the HTML template from disk.
func (v *Vorma) devReloadTemplateFromDisk() error {
	v.mu.RLock()
	isDev := v._isDev
	privateFS := v._privateFS
	rootTemplateLocation := v.Config.HTMLTemplateLocation
	v.mu.RUnlock()

	if err := guardDevOnlyReload(isDev, "template reload"); err != nil {
		return err
	}

	rootTemplate, rootTemplateError := runtimepaths.ParseRootTemplateFromFS(
		privateFS,
		rootTemplateLocation,
	)
	if rootTemplateError != nil {
		return rootTemplateError
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if err := guardDevOnlyReload(v._isDev, "template reload"); err != nil {
		return err
	}
	v.transitionLifecycleStateLocked(
		runtimecore.LifecycleStateReloadingHTML,
		"dev html template commit start",
		"",
	)
	v.commitRootTemplateLocked(rootTemplate)
	v.transitionLifecycleStateLocked(
		runtimecore.LifecycleStateReady,
		"dev html template commit complete",
		"",
	)
	v.Log.Info("HTML template reloaded")
	return nil
}

func guardDevOnlyReload(
	isDevMode bool,
	operation string,
) error {
	if isDevMode {
		return nil
	}
	return fmt.Errorf(
		"%s is dev-only and cannot run outside dev mode",
		operation,
	)
}

// MustInit initializes Vorma. Panics on error.
func (v *Vorma) MustInit() {
	isDev := wave.GetIsDev()
	if err := v.initInner(isDev); err != nil {
		panic(fmt.Errorf("error initializing Vorma: %w", err))
	}
	v.Log.Info("Vorma initialized", "build_id", v._buildID)
}

// MustInitWithDefaultRouter initializes Vorma and returns a configured mux.Router.
func (v *Vorma) MustInitWithDefaultRouter() *mux.Router {
	v.MustInit()
	r := mux.NewRouter()
	loaders, actions := v.Loaders(), v.Actions()
	r.AddHTTPHandler("GET", loaders.HandlerMountPattern(), loaders.Handler())
	for m := range actions.SupportedMethods() {
		r.AddHTTPHandler(m, actions.HandlerMountPattern(), actions.Handler())
	}
	if v.IsDevMode() {
		r.AddHTTPHandler(
			http.MethodPost,
			v.DevReloadRoutesEndpointPath(),
			actions.Handler(),
		)
		r.AddHTTPHandler(
			http.MethodPost,
			v.DevReloadTemplateEndpointPath(),
			actions.Handler(),
		)
	}
	return r
}

func (v *Vorma) ensureLoaderPatternsRegisteredForHandler() {
	v.validateAndDecorateNestedRouter(v.LoadersRouter().NestedRouter)
}

func (v *Vorma) validateAndDecorateNestedRouter(
	nestedRouter *nestedmux.Router,
) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	runtimehttp.EnsureLoaderPatternsRegistered(
		runtimehttp.EnsureLoaderPatternsRegisteredInput{
			NestedRouter: nestedRouter,
			Paths:        v._paths,
		},
	)
}

func (v *Vorma) initInner(isDev bool) error {
	privateFS, err := v.Wave.PrivateFS()
	if err != nil {
		return fmt.Errorf("could not get private fs: %w", err)
	}

	routeArtifacts, err := runtimepaths.LoadRouteArtifactsFromFS(
		privateFS,
		isDev,
	)
	if err != nil {
		return fmt.Errorf("could not get base paths: %w", err)
	}
	tmpl, err := runtimepaths.ParseRootTemplateFromFS(
		privateFS,
		v.Config.HTMLTemplateLocation,
	)
	if err != nil {
		return fmt.Errorf("error parsing root template: %w", err)
	}

	var headElsUniqueRules *headels.HeadEls
	if v.getHeadDedupeKeys != nil {
		headEls := headels.New()
		v.getHeadDedupeKeys(headEls)
		headElsUniqueRules = headEls
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	wasInitialized := v._paths != nil

	v.setIsDevModeLocked(isDev)
	v._privateFS = privateFS
	v.transitionLifecycleStateLocked(
		v.lifecycleStateForRouteCommitLocked(),
		"init route artifacts commit start",
		"",
	)
	v.commitRouteArtifactsLocked(
		routeArtifacts.RuntimeArtifacts,
		wasInitialized,
		runtimecore.RouteArtifactCommitModeInit,
	)

	v.commitRootTemplateLocked(tmpl)
	if v.headElsInst == nil {
		v.headElsInst = headels.NewInstance("vorma")
	}

	if headElsUniqueRules != nil {
		v.headElsInst.InitUniqueRules(headElsUniqueRules)
	} else {
		v.headElsInst.InitUniqueRules(nil)
	}

	v._serverAddr = fmt.Sprintf(":%d", v.MustGetPort())
	v.transitionLifecycleStateLocked(
		runtimecore.LifecycleStateReady,
		"init commit complete",
		"",
	)
	return nil
}
