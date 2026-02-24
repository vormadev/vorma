package vormaruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/vormadev/vorma/internal/vormaruntime/rendering"
	"github.com/vormadev/vorma/internal/vormaruntime/routepipeline"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimeconfig"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimecore"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimehttp"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
)

// VormaBuildIDHeaderKey carries the runtime build id in responses.
const VormaBuildIDHeaderKey = "X-Vorma-Build-Id"

// VormaJSONQueryKey toggles JSON route-data response mode for loaders.
const VormaJSONQueryKey = "vorma_json"

// LoadersHandler returns the main GET handler for loader and document
// rendering requests.
func (v *Vorma) LoadersHandler() mux.TasksCtxRequirerFunc {
	v.loadersHandlerOnce.Do(func() {
		v.ensureLoaderPatternsRegisteredForHandler()
		nestedRouter := v.LoadersRouter().NestedRouter
		v.loadersHandler = mux.TasksCtxRequirerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				res := response.New(w)
				requestedBuildID := r.URL.Query().Get(VormaJSONQueryKey)
				isJSON := requestedBuildID != ""

				routeResult := v.getUIRouteData(
					w,
					r,
					nestedRouter,
					isJSON,
					requestedBuildID,
				)
				if routeResult.TerminalState ==
					routepipeline.RouteTerminalStateStaleBuild {
					routepipeline.EnsureLoadersCacheControlHeader(w, res)
					res.SetHeader(VormaBuildIDHeaderKey, routeResult.BuildID)
					res.SetHeader(
						"X-Vorma-Reload",
						routepipeline.BuildLoadersReloadURL(r),
					)
					res.OK()
					return
				}
				if routepipeline.WriteTerminalLoadersResponse(
					res,
					routeResult,
				) {
					return
				}

				routeData := routepipeline.BuildRouteDataFinal(routeResult)

				routepipeline.EnsureLoadersCacheControlHeader(w, res)

				if isJSON {
					if err := routepipeline.WriteLoadersJSONResponse(res, routeData); err != nil {
						v.Log.Error(
							fmt.Sprintf("Error marshalling JSON: %v", err),
						)
						res.InternalServerError()
						return
					}
					return
				}

				htmlBytes, errorPrefix, err := v.buildLoadersHTMLResponseBytes(
					r,
					routeResult,
					routeData,
				)
				if err != nil {
					v.Log.Error(fmt.Sprintf("%s: %v", errorPrefix, err))
					res.InternalServerError()
					return
				}
				res.HTMLBytes(htmlBytes)
			},
		)
	})
	return v.loadersHandler
}

func (v *Vorma) handleDevReloadActionEndpoints(
	w http.ResponseWriter,
	r *http.Request,
	isDevMode bool,
) bool {
	if !isDevMode {
		return false
	}

	switch r.URL.Path {
	case v.DevReloadRoutesEndpointPath():
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return true
		}
		if !v.validateDevReloadExpectedBuildIDOrWriteConflict(w, r) {
			return true
		}
		if err := v.devReloadRoutesFromDisk(); err != nil {
			v.Log.Error(fmt.Sprintf("route reload failed: %s", err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return true
		}
		w.Write([]byte("ok"))
		return true
	case v.DevReloadTemplateEndpointPath():
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return true
		}
		if !v.validateDevReloadExpectedBuildIDOrWriteConflict(w, r) {
			return true
		}
		if err := v.devReloadTemplateFromDisk(); err != nil {
			v.Log.Error(fmt.Sprintf("template reload failed: %s", err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return true
		}
		w.Write([]byte("ok"))
		return true
	default:
		return false
	}
}

func (v *Vorma) validateDevReloadExpectedBuildIDOrWriteConflict(
	w http.ResponseWriter,
	r *http.Request,
) bool {
	if v == nil || r == nil {
		return true
	}

	requestExpectedBuildID := strings.TrimSpace(
		r.Header.Get(wave.FrameworkRuntimeReloadExpectedBuildIDHeaderName),
	)
	if requestExpectedBuildID == "" {
		return true
	}

	currentBuildID := strings.TrimSpace(v.BuildID())
	if requestExpectedBuildID == currentBuildID {
		return true
	}

	if v.Log != nil {
		v.Log.Warn(
			"dev reload endpoint rejected request due to expected build id mismatch",
			"request_expected_build_id",
			requestExpectedBuildID,
			"current_build_id",
			currentBuildID,
		)
	}
	http.Error(
		w,
		"expected build id does not match current build id",
		http.StatusConflict,
	)
	return false
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
	rootTemplateData, rootTemplateDataError := v.getRootTemplateDataOrEmpty(r)
	if rootTemplateDataError != nil {
		return nil, "Error getting root template data", rootTemplateDataError
	}

	htmlRenderSnapshot := routeResult.HTMLRenderSnapshot
	htmlBytes, htmlRenderError := rendering.BuildLoadersHTMLResponseBytes(
		rendering.BuildLoadersHTMLResponseInput{
			RenderHeadElements: func() (template.HTML, error) {
				return v.headElsInst.Render(
					routeResult.Assets.SortedAndPreEscapedHeadEls,
				)
			},
			CriticalCSSStyleElement: v.Wave.CriticalCSSStyleElement(),
			StyleSheetLinkElement:   v.Wave.StyleSheetLinkElement(),
			SSRRuntimeState: rendering.SSRRuntimeState{
				VormaSymbolStr:    VormaSymbolStr,
				IsDev:             routeResult.HTMLRenderSnapshot.IsDevMode,
				BuildID:           routeResult.BuildID,
				RootElementID:     v.ClientRootElementID(),
				PublicPathPrefix:  v.Wave.PublicPathPrefix(),
				RouteManifestFile: routeResult.RouteManifestFileSnapshot,
			},
			SSRRouteData: rendering.SSRRouteData{
				ViteDevURL: routeData.ViteDevURL,
				CSSBundles: routeData.CSSBundles,

				OutermostServerError:    routeData.RouteDataCore.OutermostServerError,
				OutermostServerErrorIdx: routeData.RouteDataCore.OutermostServerErrorIdx,
				ErrorExportKeys:         routeData.RouteDataCore.ErrorExportKeys,
				MatchedPatterns:         routeData.RouteDataCore.MatchedPatterns,
				LoadersData:             routeData.RouteDataCore.LoadersData,
				ImportURLs:              routeData.RouteDataCore.ImportURLs,
				ExportKeys:              routeData.RouteDataCore.ExportKeys,
				HasRootData:             routeData.RouteDataCore.HasRootData,
				Params:                  routeData.RouteDataCore.Params,
				SplatValues:             routeData.RouteDataCore.SplatValues,
				Deps:                    routeData.RouteDataCore.Deps,
			},
			RootTemplateData: rootTemplateData,

			TemplateDataKeyHeadElements:  v.TemplateDataKeyHeadElements(),
			TemplateDataKeyBodyScripts:   v.TemplateDataKeyBodyScripts(),
			TemplateDataKeySSRScript:     v.TemplateDataKeySSRScript(),
			TemplateDataKeySSRScriptHash: v.TemplateDataKeySSRScriptHash(),
			TemplateDataKeyRootElementID: v.TemplateDataKeyRootElementID(),
			ClientRootElementID:          v.ClientRootElementID(),

			BodyScriptsInput: rendering.BodyScriptsInput{
				RenderSnapshot:   htmlRenderSnapshot,
				PublicPathPrefix: v.Wave.PublicPathPrefix(),
				RefreshScript:    v.Wave.RefreshScript(),
				ClientEntry:      v.Config.ClientEntry,
				UseReactVariant: UIVariant(
					v.Config.UIVariant,
				) == UIVariantReact,
			},
		},
	)
	if htmlRenderError != nil {
		return nil, "Error rendering template", htmlRenderError
	}

	return htmlBytes, "", nil
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
		v.actionsHandler = mux.TasksCtxRequirerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				res := response.New(w)
				res.SetHeader(VormaBuildIDHeaderKey, v.BuildID())
				if v.handleDevReloadActionEndpoints(w, r, v.IsDevMode()) {
					return
				}
				router.ServeHTTP(w, r)
			},
		)
	})
	return v.actionsHandler
}

// LoadersRouter wraps nestedmux.Router for loader route registration.
type LoadersRouter struct {
	NestedRouter *nestedmux.Router
}

// ActionsRouter wraps mux.Router for action route registration.
type ActionsRouter struct {
	*mux.Router
	supportedMethods map[string]bool
}

// LoaderReqData is loader task request data.
type LoaderReqData = nestedmux.ReqData

// ActionReqData is action task request data.
type ActionReqData[I any] = mux.ReqData[I]

// LoadersRouterOptions configures loader route pattern semantics.
type LoadersRouterOptions struct {
	DynamicParamPrefix             rune
	SplatSegmentIdentifier         rune
	ExplicitIndexSegmentIdentifier string
}

// ActionsRouterOptions configures action route matching and accepted methods.
type ActionsRouterOptions struct {
	DynamicParamPrefix     rune
	SplatSegmentIdentifier rune
	MountRoot              string
	SupportedMethods       []string
}

func newLoadersRouter(options ...LoadersRouterOptions) *LoadersRouter {
	var o LoadersRouterOptions
	if len(options) > 0 {
		o = options[0]
	}
	return &LoadersRouter{
		NestedRouter: runtimehttp.BuildLoadersNestedRouter(
			runtimehttp.LoadersRouterSpec{
				DynamicParamPrefix:             o.DynamicParamPrefix,
				SplatSegmentIdentifier:         o.SplatSegmentIdentifier,
				ExplicitIndexSegmentIdentifier: o.ExplicitIndexSegmentIdentifier,
			},
		),
	}
}

func newActionsRouter(options ...ActionsRouterOptions) *ActionsRouter {
	var o ActionsRouterOptions
	if len(options) > 0 {
		o = options[0]
	}
	router, supportedMethods := runtimehttp.BuildActionsRouter(
		runtimehttp.ActionsRouterSpec{
			DynamicParamPrefix:     o.DynamicParamPrefix,
			SplatSegmentIdentifier: o.SplatSegmentIdentifier,
			MountRoot:              o.MountRoot,
			SupportedMethods:       o.SupportedMethods,
			IsFormDataInput: func(inputPtr any) bool {
				_, isFormData := inputPtr.(*FormData)
				return isFormData
			},
		},
	)
	return &ActionsRouter{
		Router:           router,
		supportedMethods: supportedMethods,
	}
}

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

type configWrapper struct {
	Vorma *VormaConfig `json:"Vorma,omitempty"`
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

	var wrapper configWrapper
	if err := json.Unmarshal(v.Wave.RawConfigJSON(), &wrapper); err != nil {
		panic(fmt.Sprintf("failed to parse Vorma config: %v", err))
	}
	if wrapper.Vorma == nil {
		wrapper.Vorma = &VormaConfig{}
	}
	v.Config = wrapper.Vorma
	v.validateConfig()

	v.getDefaultHeadEls = o.DefaultHeadElsFunc
	if v.getDefaultHeadEls == nil {
		v.getDefaultHeadEls = func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
			return nil
		}
	}
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
	v.loadersRouter = newLoadersRouter(o.LoadersRouterOptions)
	v.actionsRouter = newActionsRouter(o.ActionsRouterOptions)
	v.headElsInst = headels.NewInstance("vorma")
	v._routeDataCache = &sync.Map{}
	v._lifecycleState = runtimecore.LifecycleStateUninitialized

	return &v
}

func (v *Vorma) validateConfig() {
	err := runtimeconfig.NormalizeAndValidateMutableConfig(
		runtimeconfig.MutableValidationConfig{
			MainBuildEntry:                &v.Config.MainBuildEntry,
			UIVariant:                     &v.Config.UIVariant,
			HTMLTemplateLocation:          &v.Config.HTMLTemplateLocation,
			ClientEntry:                   &v.Config.ClientEntry,
			ClientRouteDefinitionPatterns: &v.Config.ClientRouteDefinitionPatterns,
			TSGenOutDir:                   &v.Config.TSGenOutDir,
			BuildtimePublicURLFuncName:    &v.Config.BuildtimePublicURLFuncName,
			UnresolvedRoutePolicy:         &v.Config.UnresolvedRoutePolicy,
			DevReloadRoutesEndpointPath:   &v.Config.DevReloadRoutesEndpointPath,
			DevReloadTemplateEndpointPath: &v.Config.DevReloadTemplateEndpointPath,
			TemplateDataKeyHeadElements:   &v.Config.TemplateDataKeyHeadElements,
			TemplateDataKeyBodyScripts:    &v.Config.TemplateDataKeyBodyScripts,
			TemplateDataKeySSRScript:      &v.Config.TemplateDataKeySSRScript,
			TemplateDataKeySSRScriptHash:  &v.Config.TemplateDataKeySSRScriptHash,
			TemplateDataKeyRootElementID:  &v.Config.TemplateDataKeyRootElementID,
			ClientRootElementID:           &v.Config.ClientRootElementID,
		},
	)
	if err != nil {
		panic(err)
	}
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
	original := h.vorma.ActionsRouter().supportedMethods
	if original == nil {
		return nil
	}
	clone := make(map[string]bool, len(original))
	for method, supported := range original {
		clone[method] = supported
	}
	return clone
}

// Route aliases mux.Route for public runtime surface continuity.
type Route[I any, O any] = mux.Route[I, O]

// TaskHandler aliases mux.TaskHandler for public runtime surface continuity.
type TaskHandler[I any, O any] = mux.TaskHandler[I, O]

func (v *Vorma) getRouteDataStage1(
	w http.ResponseWriter,
	r *http.Request,
	nestedRouter *nestedmux.Router,
	requestedBuildID string,
) *routepipeline.RouteResult {
	inputs, found := v.prepareRouteDataExecutionInputs(r, nestedRouter)
	if inputs.RuntimeSnapshot.BuildID != "" {
		w.Header().Set(VormaBuildIDHeaderKey, inputs.RuntimeSnapshot.BuildID)
	}
	if requestedBuildID != "" &&
		requestedBuildID != inputs.RuntimeSnapshot.BuildID {
		v.Log.Debug(
			"Stale build loaders request",
			"path",
			r.URL.Path,
			"requested_build_id",
			requestedBuildID,
			"current_build_id",
			inputs.RuntimeSnapshot.BuildID,
			"route_data_snapshot_version",
			inputs.RuntimeSnapshot.RouteDataSnapshotVersion,
		)
		return &routepipeline.RouteResult{
			TerminalState: routepipeline.RouteTerminalStateStaleBuild,
			BuildID:       inputs.RuntimeSnapshot.BuildID,
		}
	}
	if !found {
		v.Log.Debug(
			"No route match for loaders request",
			"path",
			r.URL.Path,
			"build_id",
			inputs.RuntimeSnapshot.BuildID,
			"route_data_snapshot_version",
			inputs.RuntimeSnapshot.RouteDataSnapshotVersion,
		)
		return &routepipeline.RouteResult{
			TerminalState: routepipeline.RouteTerminalStateNotFound,
			BuildID:       inputs.RuntimeSnapshot.BuildID,
		}
	}

	tasksResults := nestedmux.RunTasks(
		nestedRouter,
		r,
		inputs.MatchResults,
	)
	if tasksResults == nil {
		v.Log.Error(
			"Missing TasksCtx for loaders request. Use a mux.Router stack or wrap with mux.InjectTasksCtxMiddleware.",
		)
		res := response.New(w)
		res.InternalServerError()
		return &routepipeline.RouteResult{
			TerminalState: routepipeline.RouteTerminalStateError,
			BuildID:       inputs.RuntimeSnapshot.BuildID,
		}
	}

	return v.planRouteResultFromTaskResults(inputs, tasksResults)
}

func (v *Vorma) planRouteResultFromTaskResults(
	inputs routepipeline.RouteDataExecutionInputs,
	tasksResults *nestedmux.TasksResults,
) *routepipeline.RouteResult {
	return runtimehttp.PlanRouteResultFromTaskResults(
		runtimehttp.PlanRouteResultFromTaskResultsInput{
			ExecutionInputs: inputs,
			TasksResults:    tasksResults,
			WarnNilLoaderData: func(pattern string) {
				v.Log.Warn(
					"Do not return nil values from loaders unless the underlying type is an empty struct or you are returning an error.",
					"pattern",
					pattern,
				)
			},
			ResolveClientLoaderErrorMessage: v.resolveClientLoaderErrorMessage,
		},
	)
}

func (v *Vorma) prepareRouteDataExecutionInputs(
	r *http.Request,
	nestedRouter *nestedmux.Router,
) (routepipeline.RouteDataExecutionInputs, bool) {
	v.mu.RLock()
	runtimeSnapshot := v.captureRuntimeSnapshotLocked().
		ToRoutePipelineSnapshot()
	matchResults, found := nestedmux.FindMatches(nestedRouter, r)
	v.mu.RUnlock()

	if !found {
		return routepipeline.RouteDataExecutionInputs{
			RuntimeSnapshot: runtimeSnapshot,
		}, false
	}

	return runtimehttp.BuildExecutionInputsFromMatchResults(
		runtimehttp.BuildExecutionInputsFromMatchResultsInput{
			MatchResults:             matchResults,
			RuntimeSnapshot:          runtimeSnapshot,
			IsSnapshotVersionCurrent: v.isRouteDataSnapshotVersionCurrent,
		},
	), true
}

func (v *Vorma) isRouteDataSnapshotVersionCurrent(
	expectedSnapshotVersion uint64,
) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._routeDataSnapshotVersion == expectedSnapshotVersion
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

func (v *Vorma) getUIRouteData(
	w http.ResponseWriter,
	r *http.Request,
	nestedRouter *nestedmux.Router,
	isJSON bool,
	requestedBuildID string,
) *routepipeline.RouteResult {
	res := response.New(w)
	routeResult := v.getRouteDataStage1(w, r, nestedRouter, requestedBuildID)
	if routeResult.MergedResponseProxy != nil {
		routeResult.MergedResponseProxy.ApplyToResponseWriter(w, r)
	}
	if routeResult.TerminalState != routepipeline.RouteTerminalStateNone {
		return routeResult
	}

	defaultHeadElsRaw, err := v.getDefaultHeadElsRaw(r)
	if err != nil {
		v.Log.Error("Error in getUIRouteData", "error", err.Error())
		res.InternalServerError()
		return &routepipeline.RouteResult{
			TerminalState: routepipeline.RouteTerminalStateError,
			BuildID:       routeResult.BuildID,
		}
	}

	assets, err := routepipeline.BuildRouteAssets(
		routepipeline.BuildRouteAssetsInput{
			RouteResult:         routeResult,
			DefaultHeadElements: defaultHeadElsRaw,
			IsJSON:              isJSON,
			PublicPathPrefix:    v.Wave.PublicPathPrefix(),
			ToSortedAndPreEscapedHeadElsFn: v.headElsInst.
				ToSortedAndPreEscapedHeadEls,
		},
	)
	if err != nil {
		v.Log.Error(
			"Error in getUIRouteData asset resolution",
			"error",
			err.Error(),
		)
		res.InternalServerError()
		return &routepipeline.RouteResult{
			TerminalState: routepipeline.RouteTerminalStateError,
			BuildID:       routeResult.BuildID,
		}
	}

	return &routepipeline.RouteResult{
		BuildID:                   routeResult.BuildID,
		Core:                      routeResult.Core,
		Assets:                    assets,
		HTMLRenderSnapshot:        routeResult.HTMLRenderSnapshot,
		RouteManifestFileSnapshot: routeResult.RouteManifestFileSnapshot,
	}
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

func (v *Vorma) getSSRInnerHTML(
	routeData *routepipeline.RouteDataFinal,
) (*rendering.BuildSSRInnerHTMLOutput, error) {
	if routeData == nil {
		return nil, fmt.Errorf("routeData cannot be nil")
	}
	if routeData.RouteDataCore == nil {
		return nil, fmt.Errorf("routeData.RouteDataCore cannot be nil")
	}

	v.mu.RLock()
	isDev := v._isDev
	buildID := v._buildID
	routeManifestFile := v._routeManifestFile
	v.mu.RUnlock()

	return rendering.BuildSSRInnerHTMLFromRuntimeState(
		rendering.SSRRuntimeState{
			VormaSymbolStr:    VormaSymbolStr,
			IsDev:             isDev,
			BuildID:           buildID,
			RootElementID:     v.ClientRootElementID(),
			PublicPathPrefix:  v.Wave.PublicPathPrefix(),
			RouteManifestFile: routeManifestFile,
		},
		rendering.SSRRouteData{
			ViteDevURL: routeData.ViteDevURL,
			CSSBundles: routeData.CSSBundles,

			OutermostServerError:    routeData.RouteDataCore.OutermostServerError,
			OutermostServerErrorIdx: routeData.RouteDataCore.OutermostServerErrorIdx,
			ErrorExportKeys:         routeData.RouteDataCore.ErrorExportKeys,
			MatchedPatterns:         routeData.RouteDataCore.MatchedPatterns,
			LoadersData:             routeData.RouteDataCore.LoadersData,
			ImportURLs:              routeData.RouteDataCore.ImportURLs,
			ExportKeys:              routeData.RouteDataCore.ExportKeys,
			HasRootData:             routeData.RouteDataCore.HasRootData,
			Params:                  routeData.RouteDataCore.Params,
			SplatValues:             routeData.RouteDataCore.SplatValues,
			Deps:                    routeData.RouteDataCore.Deps,
		},
	)
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
	return cloneRuntimeCorePathsMapAsPublicOrNil(v._paths)
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
	return cloneRuntimeCorePathsMapAsPublicOrNil(l.v._paths)
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
	return cloneRuntimeCorePathsMapAsPublicOrNil(l.v._paths)
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

// Path represents a route path with its associated metadata.
type Path struct {
	NestedRoute nestedmux.AnyRoute `json:"-"`

	// Both stages one and two
	OriginalPattern string `json:"originalPattern"`
	SrcPath         string `json:"srcPath"`
	ExportKey       string `json:"exportKey"`
	ErrorExportKey  string `json:"errorExportKey,omitempty"`

	// Stage two only
	OutPath string   `json:"outPath,omitempty"`
	Deps    []string `json:"deps,omitempty"`
}

// UIVariant represents the UI framework variant.
type UIVariant string

const (
	UIVariantReact  UIVariant = "react"
	UIVariantPreact UIVariant = "preact"
	UIVariantSolid  UIVariant = "solid"
)

const (
	UnresolvedRoutePolicyWarn  = "warn"
	UnresolvedRoutePolicyError = "error"
)

// VormaConfig holds Vorma-specific configuration.
type VormaConfig struct {
	IncludeDefaults               *bool    `json:"IncludeDefaults,omitempty"`
	MainBuildEntry                string   `json:"MainBuildEntry"`
	UIVariant                     string   `json:"UIVariant"`
	HTMLTemplateLocation          string   `json:"HTMLTemplateLocation"`
	ClientEntry                   string   `json:"ClientEntry"`
	ClientRouteDefinitionPatterns []string `json:"ClientRouteDefinitionPatterns"`
	ServerRouteDefinitionPatterns []string `json:"ServerRouteDefinitionPatterns,omitempty"`
	TSGenOutDir                   string   `json:"TSGenOutDir"`
	BuildtimePublicURLFuncName    string   `json:"BuildtimePublicURLFuncName,omitempty"`
	UnresolvedRoutePolicy         string   `json:"UnresolvedRoutePolicy,omitempty"`
	DevReloadRoutesEndpointPath   string   `json:"DevReloadRoutesEndpointPath,omitempty"`
	DevReloadTemplateEndpointPath string   `json:"DevReloadTemplateEndpointPath,omitempty"`
	TemplateDataKeyHeadElements   string   `json:"TemplateDataKeyHeadElements,omitempty"`
	TemplateDataKeyBodyScripts    string   `json:"TemplateDataKeyBodyScripts,omitempty"`
	TemplateDataKeySSRScript      string   `json:"TemplateDataKeySSRScript,omitempty"`
	TemplateDataKeySSRScriptHash  string   `json:"TemplateDataKeySSRScriptHash,omitempty"`
	TemplateDataKeyRootElementID  string   `json:"TemplateDataKeyRootElementID,omitempty"`
	ClientRootElementID           string   `json:"ClientRootElementID,omitempty"`
}

// Output prefix constants.
const (
	VormaOutPrefix               = "vorma_out_"
	VormaVitePrehashedFilePrefix = VormaOutPrefix + "vite_"
	VormaRouteManifestPrefix     = VormaOutPrefix + "vorma_internal_route_manifest_"
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
		v.routes().syncFromDevReloadCore(
			runtimecore.CloneRoutePathsOrNil(
				artifacts.ParsedClientPaths,
			),
		)
	default:
		v.routes().replaceParsedPathsForInitCore(
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

type runtimeServingSnapshotInput struct {
	BuildID                  string
	IsDev                    bool
	Paths                    map[string]*runtimecore.RoutePath
	ClientEntryDeps          []string
	ClientEntryOut           string
	DepToCSSBundleMap        map[string][]string
	RootTemplate             *template.Template
	RouteManifestFile        string
	RouteDataSnapshotVersion uint64
	RouteDataCache           *sync.Map
}

type runtimeServingSnapshot struct {
	BuildID                  string
	IsDev                    bool
	Paths                    map[string]*runtimecore.RoutePath
	ClientEntryDeps          []string
	ClientEntryOut           string
	DepToCSSBundleMap        map[string][]string
	RootTemplate             *template.Template
	RouteManifestFile        string
	RouteDataSnapshotVersion uint64
	RouteDataCache           *sync.Map
}

func captureRuntimeServingSnapshot(
	input runtimeServingSnapshotInput,
) runtimeServingSnapshot {
	return runtimeServingSnapshot(input)
}

func (snapshot runtimeServingSnapshot) ToLoadersHTMLRenderSnapshot() rendering.LoadersHTMLRenderSnapshot {
	return rendering.LoadersHTMLRenderSnapshot{
		IsDevMode:      snapshot.IsDev,
		ClientEntryOut: snapshot.ClientEntryOut,
		RootTemplate:   snapshot.RootTemplate,
	}
}

func (snapshot runtimeServingSnapshot) ToRoutePipelineSnapshot() routepipeline.RuntimeSnapshot {
	return routepipeline.RuntimeSnapshot{
		BuildID: snapshot.BuildID,
		IsDev:   snapshot.IsDev,
		Paths: convertRuntimeCorePathsToRoutePipelinePaths(
			snapshot.Paths,
		),
		ClientEntryDeps:          snapshot.ClientEntryDeps,
		ClientEntryOut:           snapshot.ClientEntryOut,
		DepToCSSBundleMap:        snapshot.DepToCSSBundleMap,
		HTMLRenderSnapshot:       snapshot.ToLoadersHTMLRenderSnapshot(),
		RouteManifestFile:        snapshot.RouteManifestFile,
		RouteDataSnapshotVersion: snapshot.RouteDataSnapshotVersion,
		RouteDataCache:           snapshot.RouteDataCache,
	}
}

func convertRuntimeCorePathsToRoutePipelinePaths(
	paths map[string]*runtimecore.RoutePath,
) map[string]*routepipeline.PathData {
	if paths == nil {
		return nil
	}

	out := make(map[string]*routepipeline.PathData, len(paths))
	for pattern, pathValue := range paths {
		if pathValue == nil {
			out[pattern] = nil
			continue
		}
		out[pattern] = &routepipeline.PathData{
			OriginalPattern: pathValue.OriginalPattern,
			SrcPath:         pathValue.SrcPath,
			OutPath:         pathValue.OutPath,
			ExportKey:       pathValue.ExportKey,
			ErrorExportKey:  pathValue.ErrorExportKey,
			Deps:            pathValue.Deps,
		}
	}
	return out
}

func (v *Vorma) captureRuntimeSnapshotLocked() runtimeServingSnapshot {
	return captureRuntimeServingSnapshot(runtimeServingSnapshotInput{
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
	})
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

func toRuntimeCoreRoutePath(pathValue *Path) *runtimecore.RoutePath {
	if pathValue == nil {
		return nil
	}
	return &runtimecore.RoutePath{
		OriginalPattern: pathValue.OriginalPattern,
		SrcPath:         pathValue.SrcPath,
		OutPath:         pathValue.OutPath,
		ExportKey:       pathValue.ExportKey,
		ErrorExportKey:  pathValue.ErrorExportKey,
		Deps:            runtimecore.CloneStringSliceOrNil(pathValue.Deps),
	}
}

func fromRuntimeCoreRoutePath(pathValue *runtimecore.RoutePath) *Path {
	if pathValue == nil {
		return nil
	}
	return &Path{
		OriginalPattern: pathValue.OriginalPattern,
		SrcPath:         pathValue.SrcPath,
		OutPath:         pathValue.OutPath,
		ExportKey:       pathValue.ExportKey,
		ErrorExportKey:  pathValue.ErrorExportKey,
		Deps:            runtimecore.CloneStringSliceOrNil(pathValue.Deps),
	}
}

func toRuntimeCoreRoutePaths(
	paths map[string]*Path,
) map[string]*runtimecore.RoutePath {
	if paths == nil {
		return nil
	}

	cloned := make(map[string]*runtimecore.RoutePath, len(paths))
	for pattern, pathValue := range paths {
		cloned[pattern] = toRuntimeCoreRoutePath(pathValue)
	}
	return cloned
}

func fromRuntimeCoreRoutePaths(
	paths map[string]*runtimecore.RoutePath,
) map[string]*Path {
	if paths == nil {
		return make(map[string]*Path)
	}

	cloned := make(map[string]*Path, len(paths))
	for pattern, pathValue := range paths {
		cloned[pattern] = fromRuntimeCoreRoutePath(pathValue)
	}
	return cloned
}

func clonePathsMap(paths map[string]*Path) map[string]*Path {
	return fromRuntimeCoreRoutePaths(
		runtimecore.CloneRoutePaths(toRuntimeCoreRoutePaths(paths)),
	)
}

func clonePathsMapOrNil(paths map[string]*Path) map[string]*Path {
	if paths == nil {
		return nil
	}
	return clonePathsMap(paths)
}

func cloneRuntimeCorePathsMapAsPublicOrNil(
	paths map[string]*runtimecore.RoutePath,
) map[string]*Path {
	if paths == nil {
		return nil
	}
	return fromRuntimeCoreRoutePaths(
		runtimecore.CloneRoutePaths(paths),
	)
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

// RouteRegistry consolidates route state management.
// All methods require the Vorma mutex to be held.
type RouteRegistry struct {
	vorma *Vorma
}

// routes returns a RouteRegistry for route management operations.
// This is unexported to enforce that external packages use
// LockedVorma.Routes() via WithLock for compile-time safety.
func (v *Vorma) routes() *RouteRegistry {
	return &RouteRegistry{vorma: v}
}

// SyncFromDevReload updates the route state from parsed client routes for
// dev-time reload, preserving server-only handlers in parsed-path state and
// rebuilding nested-router registrations.
// Caller must hold v.mu.Lock().
func (r *RouteRegistry) SyncFromDevReload(paths map[string]*Path) {
	r.syncFromDevReloadCore(
		toRuntimeCoreRoutePaths(paths),
	)
}

func (r *RouteRegistry) syncFromDevReloadCore(
	paths map[string]*runtimecore.RoutePath,
) {
	v := r.vorma
	mutableState := runtimecore.RouteMutableState{
		Paths:                    v._paths,
		RouteDataSnapshotVersion: v._routeDataSnapshotVersion,
		RouteDataCache:           v._routeDataCache,
	}
	runtimecore.SyncRouteStateFromDevReload(
		runtimecore.SyncRouteStateFromDevReloadInput{
			State:                               &mutableState,
			ParsedClientPaths:                   paths,
			ServerRoutePatternsWithTaskHandlers: r.serverRoutePatternsWithTaskHandlers(),
			RebuildNestedRouterFromCurrentPaths: r.rebuildNestedRouterFromPaths,
		},
	)
	v._paths = mutableState.Paths
	v._routeDataSnapshotVersion = mutableState.RouteDataSnapshotVersion
	v._routeDataCache = mutableState.RouteDataCache
}

// ReplaceParsedPathsForInit updates route state from parsed client routes for
// init/re-init flows. This does not merge server-only handlers into parsed-path
// state, but can rebuild nested-router registrations when requested.
// Caller must hold v.mu.Lock().
func (r *RouteRegistry) ReplaceParsedPathsForInit(
	paths map[string]*Path,
	rebuildNestedRouter bool,
) {
	r.replaceParsedPathsForInitCore(
		toRuntimeCoreRoutePaths(paths),
		rebuildNestedRouter,
	)
}

func (r *RouteRegistry) replaceParsedPathsForInitCore(
	paths map[string]*runtimecore.RoutePath,
	rebuildNestedRouter bool,
) {
	v := r.vorma
	mutableState := runtimecore.RouteMutableState{
		Paths:                    v._paths,
		RouteDataSnapshotVersion: v._routeDataSnapshotVersion,
		RouteDataCache:           v._routeDataCache,
	}
	runtimecore.ReplaceRouteStateForInit(
		runtimecore.ReplaceRouteStateForInitInput{
			State:                               &mutableState,
			ParsedClientPaths:                   paths,
			RebuildNestedRouter:                 rebuildNestedRouter,
			RebuildNestedRouterFromCurrentPaths: r.rebuildNestedRouterFromPaths,
		},
	)
	v._paths = mutableState.Paths
	v._routeDataSnapshotVersion = mutableState.RouteDataSnapshotVersion
	v._routeDataCache = mutableState.RouteDataCache
}

func (r *RouteRegistry) serverRoutePatternsWithTaskHandlers() []string {
	v := r.vorma
	allServerRoutes := v.LoadersRouter().NestedRouter.AllRoutes()
	patterns := make([]string, 0, len(allServerRoutes))
	for pattern := range allServerRoutes {
		if !v.LoadersRouter().NestedRouter.HasTaskHandler(pattern) {
			continue
		}
		patterns = append(patterns, pattern)
	}
	return patterns
}

func (r *RouteRegistry) rebuildNestedRouterFromPaths(
	paths map[string]*runtimecore.RoutePath,
) {
	v := r.vorma
	patterns := runtimecore.BuildNestedRouterPatternList(paths)
	v.LoadersRouter().NestedRouter.RebuildPreservingHandlers(patterns)
}

// RegisterPatternIfNeeded registers a pattern if not already registered.
// This method is safe to call without holding the lock as the router handles
// its own synchronization.
func (v *Vorma) RegisterPatternIfNeeded(pattern string) {
	v.LoadersRouter().NestedRouter.AddPatternWithoutHandlerIfMissing(
		pattern,
	)
}

func (v *Vorma) guardDevOnlyReload(op string) error {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if !v._isDev {
		return fmt.Errorf("%s is dev-only and cannot run outside dev mode", op)
	}
	return nil
}

func (v *Vorma) guardDevOnlyReloadLocked(op string) error {
	if !v._isDev {
		return fmt.Errorf("%s is dev-only and cannot run outside dev mode", op)
	}
	return nil
}

// devReloadRoutesFromDisk reloads route configuration from JSON files on disk.
// Called by Process B when Process A has regenerated route artifacts.
// This does NOT regenerate TypeScript; Process A does that work.
func (v *Vorma) devReloadRoutesFromDisk() error {
	if err := v.guardDevOnlyReload("route reload"); err != nil {
		return err
	}

	v.mu.RLock()
	privateFS := v._privateFS
	v.mu.RUnlock()

	routeArtifacts, err := runtimepaths.LoadRouteArtifactsFromFS(
		privateFS,
		true,
	)
	if err != nil {
		return fmt.Errorf("load paths from disk: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.guardDevOnlyReloadLocked("route reload"); err != nil {
		return err
	}

	v.transitionLifecycleStateLocked(
		runtimecore.LifecycleStateReloadingRoutes,
		"dev route artifacts commit start",
		"",
	)
	v.commitRouteArtifactsLocked(
		routeArtifacts.RuntimeArtifacts,
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
	if err := v.guardDevOnlyReload("template reload"); err != nil {
		return err
	}

	v.mu.RLock()
	privateFS := v._privateFS
	rootTemplateLocation := v.Config.HTMLTemplateLocation
	v.mu.RUnlock()
	tmpl, err := runtimepaths.ParseRootTemplateFromFS(
		privateFS,
		rootTemplateLocation,
	)
	if err != nil {
		return err
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.guardDevOnlyReloadLocked("template reload"); err != nil {
		return err
	}
	v.transitionLifecycleStateLocked(
		runtimecore.LifecycleStateReloadingHTML,
		"dev html template commit start",
		"",
	)
	v.commitRootTemplateLocked(tmpl)
	v.transitionLifecycleStateLocked(
		runtimecore.LifecycleStateReady,
		"dev html template commit complete",
		"",
	)

	v.Log.Info("HTML template reloaded")
	return nil
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

func (v *Vorma) validateAndDecorateNestedRouter(
	nestedRouter *nestedmux.Router,
) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if nestedRouter == nil {
		panic("nestedRouter is nil")
	}
	for mapKeyPattern, pathEntry := range v._paths {
		if pathEntry == nil {
			panic(
				fmt.Sprintf(
					"paths entry for pattern %q is nil",
					mapKeyPattern,
				),
			)
		}
		nestedRouter.AddPatternWithoutHandlerIfMissing(
			pathEntry.OriginalPattern,
		)
	}
}

func (v *Vorma) ensureLoaderPatternsRegisteredForHandler() {
	v.validateAndDecorateNestedRouter(v.LoadersRouter().NestedRouter)
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
