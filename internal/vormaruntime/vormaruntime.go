package vormaruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/colorlog"
	"github.com/vormadev/vorma/kit/envutil"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmatcher"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/validate"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/wave"
	"golang.org/x/sync/errgroup"
)

// VormaBuildIDHeaderKey carries the runtime build id in responses.
const VormaBuildIDHeaderKey = "X-Vorma-Build-Id"

// VormaJSONQueryKey toggles JSON route-data response mode for loaders.
const VormaJSONQueryKey = "vorma_json"

const (
	// DefaultDevReloadRoutesEndpointPath is the default dev endpoint for route
	// reloading.
	DefaultDevReloadRoutesEndpointPath = "/__vorma_internal/reload-routes"
	// DefaultDevReloadTemplateEndpointPath is the default dev endpoint for root
	// template reloading.
	DefaultDevReloadTemplateEndpointPath = "/__vorma_internal/reload-template"
	// DefaultTemplateDataKeyHeadElements is the default key for rendered head
	// elements in template data.
	DefaultTemplateDataKeyHeadElements = "VormaHeadEls"
	// DefaultTemplateDataKeyBodyScripts is the default key for body script tags
	// in template data.
	DefaultTemplateDataKeyBodyScripts = "VormaBodyScripts"
	// DefaultTemplateDataKeySSRScript is the default key for SSR inner HTML in
	// template data.
	DefaultTemplateDataKeySSRScript = "VormaSSRScript"
	// DefaultTemplateDataKeySSRScriptHash is the default key for the SSR script
	// SHA-256 hash in template data.
	DefaultTemplateDataKeySSRScriptHash = "VormaSSRScriptSha256Hash"
	// DefaultTemplateDataKeyRootElementID is the default key for the client root
	// element id in template data.
	DefaultTemplateDataKeyRootElementID = "VormaRootID"
	// DefaultClientRootElementID is the default DOM id expected by client mount
	// logic.
	DefaultClientRootElementID = "vorma-root"
)

type loadersHTMLRenderSnapshot struct {
	isDevMode      bool
	clientEntryOut string
	rootTemplate   *template.Template
}

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
				if routeResult.terminalState == routeTerminalStateStaleBuild {
					ensureLoadersCacheControlHeader(w, res)
					res.SetHeader(VormaBuildIDHeaderKey, routeResult.buildID)
					res.SetHeader("X-Vorma-Reload", buildLoadersReloadURL(r))
					res.OK()
					return
				}
				if writeTerminalLoadersResponse(res, routeResult) {
					return
				}

				routeData := buildRouteDataFinal(routeResult)

				ensureLoadersCacheControlHeader(w, res)

				if isJSON {
					if err := writeLoadersJSONResponse(res, routeData); err != nil {
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
		return DefaultDevReloadRoutesEndpointPath
	}

	configuredPath := strings.TrimSpace(v.Config.DevReloadRoutesEndpointPath)
	if configuredPath == "" {
		return DefaultDevReloadRoutesEndpointPath
	}

	return configuredPath
}

// DevReloadTemplateEndpointPath returns the configured (or default) dev
// template reload endpoint path.
func (v *Vorma) DevReloadTemplateEndpointPath() string {
	if v == nil || v.Config == nil {
		return DefaultDevReloadTemplateEndpointPath
	}

	configuredPath := strings.TrimSpace(v.Config.DevReloadTemplateEndpointPath)
	if configuredPath == "" {
		return DefaultDevReloadTemplateEndpointPath
	}

	return configuredPath
}

// TemplateDataKeyHeadElements returns the template data key used for rendered
// head elements.
func (v *Vorma) TemplateDataKeyHeadElements() string {
	if v == nil || v.Config == nil {
		return DefaultTemplateDataKeyHeadElements
	}
	configuredKey := strings.TrimSpace(v.Config.TemplateDataKeyHeadElements)
	if configuredKey == "" {
		return DefaultTemplateDataKeyHeadElements
	}
	return configuredKey
}

// TemplateDataKeyBodyScripts returns the template data key used for rendered
// body scripts.
func (v *Vorma) TemplateDataKeyBodyScripts() string {
	if v == nil || v.Config == nil {
		return DefaultTemplateDataKeyBodyScripts
	}
	configuredKey := strings.TrimSpace(v.Config.TemplateDataKeyBodyScripts)
	if configuredKey == "" {
		return DefaultTemplateDataKeyBodyScripts
	}
	return configuredKey
}

// TemplateDataKeySSRScript returns the template data key used for SSR inner
// HTML.
func (v *Vorma) TemplateDataKeySSRScript() string {
	if v == nil || v.Config == nil {
		return DefaultTemplateDataKeySSRScript
	}
	configuredKey := strings.TrimSpace(v.Config.TemplateDataKeySSRScript)
	if configuredKey == "" {
		return DefaultTemplateDataKeySSRScript
	}
	return configuredKey
}

// TemplateDataKeySSRScriptHash returns the template data key used for the SSR
// script hash.
func (v *Vorma) TemplateDataKeySSRScriptHash() string {
	if v == nil || v.Config == nil {
		return DefaultTemplateDataKeySSRScriptHash
	}
	configuredKey := strings.TrimSpace(v.Config.TemplateDataKeySSRScriptHash)
	if configuredKey == "" {
		return DefaultTemplateDataKeySSRScriptHash
	}
	return configuredKey
}

// TemplateDataKeyRootElementID returns the template data key used for the
// client root element id.
func (v *Vorma) TemplateDataKeyRootElementID() string {
	if v == nil || v.Config == nil {
		return DefaultTemplateDataKeyRootElementID
	}
	configuredKey := strings.TrimSpace(v.Config.TemplateDataKeyRootElementID)
	if configuredKey == "" {
		return DefaultTemplateDataKeyRootElementID
	}
	return configuredKey
}

// ClientRootElementID returns the configured (or default) client mount root id.
func (v *Vorma) ClientRootElementID() string {
	if v == nil || v.Config == nil {
		return DefaultClientRootElementID
	}
	rootElementID := strings.TrimSpace(v.Config.ClientRootElementID)
	if rootElementID == "" {
		return DefaultClientRootElementID
	}
	return rootElementID
}

func buildLoadersReloadURL(r *http.Request) string {
	newURL := *r.URL
	q := r.URL.Query()
	q.Del(VormaJSONQueryKey)
	newURL.RawQuery = q.Encode()
	return newURL.String()
}

func writeTerminalLoadersResponse(
	res response.Response,
	routeResult *RouteResult,
) bool {
	switch routeResult.terminalState {
	case routeTerminalStateNotFound:
		res.NotFound()
		return true
	case routeTerminalStateRedirect, routeTerminalStateError:
		return true
	default:
		return false
	}
}

func buildRouteDataFinal(routeResult *RouteResult) *RouteDataFinal {
	return &RouteDataFinal{
		RouteDataCore: routeResult.core,
		Title:         routeResult.assets.SortedAndPreEscapedHeadEls.Title,
		Meta:          routeResult.assets.SortedAndPreEscapedHeadEls.Meta,
		Rest:          routeResult.assets.SortedAndPreEscapedHeadEls.Rest,
		CSSBundles:    routeResult.assets.CSSBundles,
		ViteDevURL:    routeResult.assets.ViteDevURL,
	}
}

func ensureLoadersCacheControlHeader(
	w http.ResponseWriter,
	res response.Response,
) {
	if w.Header().Get("Cache-Control") == "" {
		res.SetHeader(
			"Cache-Control",
			"private, max-age=0, must-revalidate, no-cache",
		)
	}
}

func writeLoadersJSONResponse(
	res response.Response,
	routeData *RouteDataFinal,
) error {
	jsonBytes, err := json.Marshal(routeData)
	if err != nil {
		return err
	}
	res.JSONBytes(jsonBytes)
	return nil
}

func (v *Vorma) renderHeadAndSSRForTemplate(
	routeResult *RouteResult,
	routeData *RouteDataFinal,
) (template.HTML, *template.HTML, string, error) {
	var eg errgroup.Group
	var ssrScript *template.HTML
	var ssrScriptSha256Hash string
	var headElements template.HTML

	eg.Go(func() error {
		he, err := v.headElsInst.Render(
			routeResult.assets.SortedAndPreEscapedHeadEls,
		)
		if err != nil {
			return fmt.Errorf("error getting head elements: %w", err)
		}
		headElements = he
		headElements += "\n" + v.Wave.CriticalCSSStyleElement()
		headElements += "\n" + v.Wave.StyleSheetLinkElement()
		return nil
	})

	eg.Go(func() error {
		sih, err := v.getSSRInnerHTMLWithRuntimeState(
			routeData,
			ssrRuntimeSnapshot{
				isDev:             routeResult.htmlRenderSnapshot.isDevMode,
				buildID:           routeResult.buildID,
				routeManifestFile: routeResult.routeManifestFileSnapshot,
			},
		)
		if err != nil {
			return fmt.Errorf("error getting SSR inner HTML: %w", err)
		}
		ssrScript = sih.Script
		ssrScriptSha256Hash = sih.Sha256Hash
		return nil
	})

	if err := eg.Wait(); err != nil {
		return "", nil, "", err
	}

	return headElements, ssrScript, ssrScriptSha256Hash, nil
}

func (v *Vorma) buildLoadersHTMLResponseBytes(
	r *http.Request,
	routeResult *RouteResult,
	routeData *RouteDataFinal,
) ([]byte, string, error) {
	headElements, ssrScript, ssrScriptSha256Hash, err := v.renderHeadAndSSRForTemplate(
		routeResult,
		routeData,
	)
	if err != nil {
		return nil, "Error getting route data", err
	}

	rootTemplateData, err := v.getRootTemplateDataOrEmpty(r)
	if err != nil {
		return nil, "Error getting root template data", err
	}
	v.injectVormaTemplateFields(
		rootTemplateData,
		headElements,
		ssrScript,
		ssrScriptSha256Hash,
	)

	htmlRenderSnapshot := routeResult.htmlRenderSnapshot
	bodyScripts, err := v.getBodyScriptsForTemplate(htmlRenderSnapshot)
	if err != nil {
		return nil, "Error getting dev scripts", err
	}
	rootTemplateData[v.TemplateDataKeyBodyScripts()] = bodyScripts

	htmlBytes, err := executeRootTemplate(
		htmlRenderSnapshot.rootTemplate,
		rootTemplateData,
	)
	if err != nil {
		return nil, "Error executing template", err
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

	return cloneTemplateDataMap(rootTemplateData), nil
}

func cloneTemplateDataMap(input map[string]any) map[string]any {
	cloned := make(map[string]any, len(input))
	for k, v := range input {
		cloned[k] = v
	}
	return cloned
}

func (v *Vorma) injectVormaTemplateFields(
	rootTemplateData map[string]any,
	headElements template.HTML,
	ssrScript *template.HTML,
	ssrScriptSha256Hash string,
) {
	rootTemplateData[v.TemplateDataKeyHeadElements()] = headElements
	rootTemplateData[v.TemplateDataKeySSRScript()] = ssrScript
	rootTemplateData[v.TemplateDataKeySSRScriptHash()] = ssrScriptSha256Hash
	rootTemplateData[v.TemplateDataKeyRootElementID()] = v.ClientRootElementID()
}

func (v *Vorma) getBodyScriptsForTemplate(
	htmlRenderSnapshot loadersHTMLRenderSnapshot,
) (template.HTML, error) {
	if !htmlRenderSnapshot.isDevMode {
		return template.HTML(
			fmt.Sprintf(
				`<script type="module" src="%s%s"></script>`,
				v.Wave.PublicPathPrefix(),
				htmlRenderSnapshot.clientEntryOut,
			),
		), nil
	}

	opts := viteutil.ToDevScriptsOptions{ClientEntry: v.Config.ClientEntry}
	if UIVariant(v.Config.UIVariant) == UIVariantReact {
		opts.Variant = viteutil.VariantReact
	} else {
		opts.Variant = viteutil.VariantOther
	}

	devScripts, err := viteutil.ToDevScripts(opts)
	if err != nil {
		return "", err
	}

	return devScripts + "\n" + v.Wave.RefreshScript(), nil
}

func executeRootTemplate(
	rootTemplate *template.Template,
	rootTemplateData map[string]any,
) ([]byte, error) {
	var buf bytes.Buffer
	if err := rootTemplate.Execute(&buf, rootTemplateData); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
	explicitIndexSegment := o.ExplicitIndexSegmentIdentifier
	if explicitIndexSegment == "" {
		explicitIndexSegment = "_index"
	}
	return &LoadersRouter{
		NestedRouter: nestedmux.NewRouter(&nestedmux.Options{
			DynamicParamPrefix:             o.DynamicParamPrefix,
			SplatSegmentIdentifier:         o.SplatSegmentIdentifier,
			ExplicitIndexSegmentIdentifier: explicitIndexSegment,
		}),
	}
}

func newActionsRouter(options ...ActionsRouterOptions) *ActionsRouter {
	var o ActionsRouterOptions
	if len(options) > 0 {
		o = options[0]
	}
	mountRoot := o.MountRoot
	if mountRoot == "" {
		mountRoot = "/api/"
	}
	supportedMethods := make(map[string]bool, len(o.SupportedMethods))
	if len(o.SupportedMethods) == 0 {
		supportedMethods["GET"] = true
		supportedMethods["POST"] = true
		supportedMethods["PUT"] = true
		supportedMethods["DELETE"] = true
		supportedMethods["PATCH"] = true
	} else {
		for _, m := range o.SupportedMethods {
			upperMethod := strings.ToUpper(strings.TrimSpace(m))
			if upperMethod == "" {
				continue
			}
			supportedMethods[upperMethod] = true
		}
	}
	return &ActionsRouter{
		Router: mux.NewRouter(&mux.Options{
			DynamicParamPrefix:     o.DynamicParamPrefix,
			SplatSegmentIdentifier: o.SplatSegmentIdentifier,
			MountRoot:              mountRoot,
			ParseInput: func(r *http.Request, iPtr any) error {
				if r.Method == http.MethodGet || r.Method == http.MethodHead {
					return validate.URLSearchParamsInto(r, iPtr)
				}
				if supportedMethods[r.Method] {
					contentType, _, _ := mime.ParseMediaType(
						r.Header.Get("Content-Type"),
					)
					if contentType == "application/x-www-form-urlencoded" ||
						contentType == "multipart/form-data" {
						if _, isFormData := iPtr.(*FormData); isFormData {
							return nil
						}
						return &validate.ValidationError{
							Err: errors.New(
								"form content type requires vormaruntime.FormData input",
							),
						}
					}
					return validate.JSONBodyInto(r, iPtr)
				}
				return &validate.ValidationError{
					Err: errors.New("unsupported method"),
				}
			},
		}),
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
	v._lifecycleState = runtimeLifecycleStateUninitialized

	return &v
}

func (v *Vorma) validateConfig() {
	if v.Config.MainBuildEntry == "" {
		panic("config: Vorma.MainBuildEntry is required")
	}
	if v.Config.UIVariant == "" {
		panic("config: Vorma.UIVariant is required")
	}
	if v.Config.HTMLTemplateLocation == "" {
		panic("config: Vorma.HTMLTemplateLocation is required")
	}
	if v.Config.ClientEntry == "" {
		panic("config: Vorma.ClientEntry is required")
	}
	if len(v.Config.ClientRouteDefinitionPatterns) == 0 {
		panic("config: Vorma.ClientRouteDefinitionPatterns is required")
	}
	for _, pattern := range v.Config.ClientRouteDefinitionPatterns {
		if strings.TrimSpace(pattern) == "" {
			panic(
				"config: Vorma.ClientRouteDefinitionPatterns cannot contain empty entries",
			)
		}
	}
	if v.Config.TSGenOutDir == "" {
		panic("config: Vorma.TSGenOutDir is required")
	}
	applyDefaultConfigStringValue(
		&v.Config.BuildtimePublicURLFuncName,
		"waveBuildtimeURL",
	)
	trimConfigStringValue(&v.Config.UnresolvedRoutePolicy)
	v.Config.UnresolvedRoutePolicy = strings.ToLower(
		v.Config.UnresolvedRoutePolicy,
	)
	if !isValidUnresolvedRoutePolicy(v.Config.UnresolvedRoutePolicy) {
		panic(
			`config: Vorma.UnresolvedRoutePolicy must be "warn" or "error" when set`,
		)
	}

	applyDefaultConfigStringValue(
		&v.Config.DevReloadRoutesEndpointPath,
		DefaultDevReloadRoutesEndpointPath,
	)
	applyDefaultConfigStringValue(
		&v.Config.DevReloadTemplateEndpointPath,
		DefaultDevReloadTemplateEndpointPath,
	)

	trimConfigStringValue(&v.Config.DevReloadRoutesEndpointPath)
	trimConfigStringValue(&v.Config.DevReloadTemplateEndpointPath)
	if !strings.HasPrefix(v.Config.DevReloadRoutesEndpointPath, "/") {
		panic("config: Vorma.DevReloadRoutesEndpointPath must start with '/'")
	}
	if !strings.HasPrefix(v.Config.DevReloadTemplateEndpointPath, "/") {
		panic("config: Vorma.DevReloadTemplateEndpointPath must start with '/'")
	}
	if v.Config.DevReloadRoutesEndpointPath == v.Config.DevReloadTemplateEndpointPath {
		panic(
			"config: Vorma.DevReloadRoutesEndpointPath and Vorma.DevReloadTemplateEndpointPath must differ",
		)
	}

	applyDefaultAndTrimConfigStringValue(
		&v.Config.TemplateDataKeyHeadElements,
		DefaultTemplateDataKeyHeadElements,
	)
	applyDefaultAndTrimConfigStringValue(
		&v.Config.TemplateDataKeyBodyScripts,
		DefaultTemplateDataKeyBodyScripts,
	)
	applyDefaultAndTrimConfigStringValue(
		&v.Config.TemplateDataKeySSRScript,
		DefaultTemplateDataKeySSRScript,
	)
	applyDefaultAndTrimConfigStringValue(
		&v.Config.TemplateDataKeySSRScriptHash,
		DefaultTemplateDataKeySSRScriptHash,
	)
	applyDefaultAndTrimConfigStringValue(
		&v.Config.TemplateDataKeyRootElementID,
		DefaultTemplateDataKeyRootElementID,
	)
	applyDefaultAndTrimConfigStringValue(
		&v.Config.ClientRootElementID,
		DefaultClientRootElementID,
	)

	templateDataKeys := []string{
		v.Config.TemplateDataKeyHeadElements,
		v.Config.TemplateDataKeyBodyScripts,
		v.Config.TemplateDataKeySSRScript,
		v.Config.TemplateDataKeySSRScriptHash,
		v.Config.TemplateDataKeyRootElementID,
	}
	for _, templateDataKey := range templateDataKeys {
		if templateDataKey == "" {
			panic("config: Vorma template data keys must be non-empty")
		}
	}
	seenTemplateDataKeys := make(map[string]struct{}, len(templateDataKeys))
	for _, templateDataKey := range templateDataKeys {
		if _, found := seenTemplateDataKeys[templateDataKey]; found {
			panic("config: Vorma template data keys must be unique")
		}
		seenTemplateDataKeys[templateDataKey] = struct{}{}
	}
	if v.Config.ClientRootElementID == "" {
		panic("config: Vorma.ClientRootElementID is required")
	}
}

func applyDefaultConfigStringValue(configField *string, defaultValue string) {
	if configField == nil {
		return
	}
	if *configField == "" {
		*configField = defaultValue
	}
}

func trimConfigStringValue(configField *string) {
	if configField == nil {
		return
	}
	*configField = strings.TrimSpace(*configField)
}

func applyDefaultAndTrimConfigStringValue(
	configField *string,
	defaultValue string,
) {
	applyDefaultConfigStringValue(configField, defaultValue)
	trimConfigStringValue(configField)
}

func isValidUnresolvedRoutePolicy(policy string) bool {
	if policy == "" {
		return true
	}

	return policy == UnresolvedRoutePolicyWarn ||
		policy == UnresolvedRoutePolicyError
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

type cachedItemSubset struct {
	ImportURLs      []string
	ExportKeys      []string
	ErrorExportKeys []string
	Deps            []string
}

type SplatValues []string

// RouteDataCore contains the core route data that is serialized to JSON for the client.
type RouteDataCore struct {
	OutermostServerError    string   `json:"outermostServerError,omitempty"`
	OutermostServerErrorIdx *int     `json:"outermostServerErrorIdx,omitempty"`
	ErrorExportKeys         []string `json:"errorExportKeys,omitempty"`

	MatchedPatterns []string `json:"matchedPatterns,omitempty"`
	LoadersData     []any    `json:"loadersData,omitempty"`
	ImportURLs      []string `json:"importURLs,omitempty"`
	ExportKeys      []string `json:"exportKeys,omitempty"`
	HasRootData     bool     `json:"hasRootData,omitempty"`

	Params      mux.Params  `json:"params,omitempty"`
	SplatValues SplatValues `json:"splatValues,omitempty"`
	Deps        []string    `json:"deps,omitempty"`
}

// RouteAssets contains resolved CSS bundles and head elements.
type RouteAssets struct {
	SortedAndPreEscapedHeadEls *headels.SortedAndPreEscapedHeadEls
	CSSBundles                 []string
	ViteDevURL                 string
}

// RouteResult is the full result of route resolution, including early-return signals.
type RouteResult struct {
	terminalState routeTerminalState
	buildID       string

	core                      *RouteDataCore
	headElements              []*htmlutil.Element
	cssBundles                []string
	assets                    *RouteAssets
	isDev                     bool
	htmlRenderSnapshot        loadersHTMLRenderSnapshot
	routeManifestFileSnapshot string
	mergedResponseProxy       *response.Proxy
}

type routeTerminalState uint8

const (
	routeTerminalStateNone routeTerminalState = iota
	routeTerminalStateNotFound
	routeTerminalStateRedirect
	routeTerminalStateError
	routeTerminalStateStaleBuild
)

// RouteDataFinal is the final structure serialized to JSON for the client.
type RouteDataFinal struct {
	*RouteDataCore
	Title      *htmlutil.Element   `json:"title,omitempty"`
	Meta       []*htmlutil.Element `json:"metaHeadEls,omitempty"`
	Rest       []*htmlutil.Element `json:"restHeadEls,omitempty"`
	CSSBundles []string            `json:"cssBundles,omitempty"`
	ViteDevURL string              `json:"viteDevURL,omitempty"`
}

type routeDataExecutionInputs struct {
	matchResults    *nestedmatcher.Results
	matches         []*nestedmatcher.Match
	matchedPatterns []string
	cached          *cachedItemSubset
	runtimeSnapshot RuntimeSnapshot
}

type routeErrorCutPlan struct {
	cutIdx           int
	headRouteCount   int
	clientMessage    string
	depsForRouteData []string
}

type routeStageOnePlannerInput struct {
	matchResults              *nestedmatcher.Results
	matches                   []*nestedmatcher.Match
	matchedPatterns           []string
	cached                    *cachedItemSubset
	runtimeSnapshot           RuntimeSnapshot
	hasRootData               bool
	loadersData               []any
	outermostLoaderErrorIndex *int
	clientLoaderErrorMessage  string
	responseProxies           []*response.Proxy
	mergedResponseProxy       *response.Proxy
}

func (v *Vorma) getRouteDataStage1(
	w http.ResponseWriter,
	r *http.Request,
	nestedRouter *nestedmux.Router,
	requestedBuildID string,
) *RouteResult {
	inputs, found := v.prepareRouteDataExecutionInputs(r, nestedRouter)
	if inputs.runtimeSnapshot.buildID != "" {
		w.Header().Set(VormaBuildIDHeaderKey, inputs.runtimeSnapshot.buildID)
	}
	if requestedBuildID != "" &&
		requestedBuildID != inputs.runtimeSnapshot.buildID {
		v.Log.Debug(
			"Stale build loaders request",
			"path",
			r.URL.Path,
			"requested_build_id",
			requestedBuildID,
			"current_build_id",
			inputs.runtimeSnapshot.buildID,
			"route_data_snapshot_version",
			inputs.runtimeSnapshot.routeDataSnapshotVersion,
		)
		return &RouteResult{
			terminalState: routeTerminalStateStaleBuild,
			buildID:       inputs.runtimeSnapshot.buildID,
		}
	}
	if !found {
		v.Log.Debug(
			"No route match for loaders request",
			"path",
			r.URL.Path,
			"build_id",
			inputs.runtimeSnapshot.buildID,
			"route_data_snapshot_version",
			inputs.runtimeSnapshot.routeDataSnapshotVersion,
		)
		return &RouteResult{
			terminalState: routeTerminalStateNotFound,
			buildID:       inputs.runtimeSnapshot.buildID,
		}
	}

	tasksResults := nestedmux.RunTasks(
		nestedRouter,
		r,
		inputs.matchResults,
	)
	if tasksResults == nil {
		v.Log.Error(
			"Missing TasksCtx for loaders request. Use a mux.Router stack or wrap with mux.InjectTasksCtxMiddleware.",
		)
		res := response.New(w)
		res.InternalServerError()
		return &RouteResult{
			terminalState: routeTerminalStateError,
			buildID:       inputs.runtimeSnapshot.buildID,
		}
	}

	return v.planRouteResultFromTaskResults(inputs, tasksResults)
}

func (v *Vorma) planRouteResultFromTaskResults(
	inputs routeDataExecutionInputs,
	tasksResults *nestedmux.TasksResults,
) *RouteResult {
	mergedResponseProxy := response.MergeProxyResponses(
		tasksResults.ResponseProxies...)
	hasRootData := computeHasRootData(inputs.matchResults, tasksResults)

	loadersData, loadersErrs := v.collectLoadersDataAndErrors(
		tasksResults,
		inputs.matchedPatterns,
	)
	outermostErrorIdx := findFirstErrorIndex(loadersErrs)
	clientLoaderErrorMessage := ""
	if outermostErrorIdx != nil {
		derefErrorIdx := *outermostErrorIdx
		clientLoaderErrorMessage = v.resolveClientLoaderErrorMessage(
			loadersErrs[derefErrorIdx],
			inputs.matchedPatterns[derefErrorIdx],
		)
	}

	return planRouteResultFromResolvedTaskOutcomes(routeStageOnePlannerInput{
		matchResults:              inputs.matchResults,
		matches:                   inputs.matches,
		matchedPatterns:           inputs.matchedPatterns,
		cached:                    inputs.cached,
		runtimeSnapshot:           inputs.runtimeSnapshot,
		hasRootData:               hasRootData,
		loadersData:               loadersData,
		outermostLoaderErrorIndex: outermostErrorIdx,
		clientLoaderErrorMessage:  clientLoaderErrorMessage,
		responseProxies:           tasksResults.ResponseProxies,
		mergedResponseProxy:       mergedResponseProxy,
	})
}

func planRouteResultFromResolvedTaskOutcomes(
	input routeStageOnePlannerInput,
) *RouteResult {
	terminalState := detectTerminalStateFromMergedResponseProxy(
		input.mergedResponseProxy,
	)
	if terminalState != routeTerminalStateNone {
		return &RouteResult{
			terminalState:       terminalState,
			buildID:             input.runtimeSnapshot.buildID,
			mergedResponseProxy: input.mergedResponseProxy,
		}
	}

	cached := input.cached
	if cached == nil {
		cached = buildEmptyCachedItemSubset(len(input.matches))
	}
	matchResults := input.matchResults
	if matchResults == nil {
		matchResults = &nestedmatcher.Results{}
	}

	normalizedInput := input
	normalizedInput.cached = cached
	normalizedInput.matchResults = matchResults

	cutPlan := buildRouteErrorCutPlan(normalizedInput)
	core := buildRouteDataCore(
		normalizedInput.matchResults,
		normalizedInput.matchedPatterns,
		normalizedInput.loadersData,
		normalizedInput.cached,
		normalizedInput.hasRootData,
		normalizedInput.outermostLoaderErrorIndex,
		cutPlan.clientMessage,
		cutPlan.cutIdx,
		cutPlan.depsForRouteData,
	)
	cssBundles := getCSSBundles(
		core.Deps,
		normalizedInput.runtimeSnapshot.clientEntryOut,
		normalizedInput.runtimeSnapshot.depToCSSBundleMap,
	)

	return &RouteResult{
		buildID: normalizedInput.runtimeSnapshot.buildID,
		core:    core,
		headElements: collectFlattenedHeadElementsForPrefix(
			normalizedInput.responseProxies,
			cutPlan.headRouteCount,
		),
		cssBundles:                cssBundles,
		isDev:                     normalizedInput.runtimeSnapshot.isDev,
		htmlRenderSnapshot:        normalizedInput.runtimeSnapshot.toLoadersHTMLRender(),
		routeManifestFileSnapshot: normalizedInput.runtimeSnapshot.routeManifestFile,
		mergedResponseProxy:       normalizedInput.mergedResponseProxy,
	}
}

func buildEmptyCachedItemSubset(routeCount int) *cachedItemSubset {
	return &cachedItemSubset{
		ImportURLs:      make([]string, routeCount),
		ExportKeys:      make([]string, routeCount),
		ErrorExportKeys: make([]string, routeCount),
	}
}

func buildRouteErrorCutPlan(input routeStageOnePlannerInput) routeErrorCutPlan {
	plan := routeErrorCutPlan{
		cutIdx:         len(input.matches),
		headRouteCount: len(input.matches),
	}
	if input.cached != nil {
		plan.depsForRouteData = input.cached.Deps
	}
	if input.outermostLoaderErrorIndex == nil {
		return plan
	}

	derefErrorIdx := *input.outermostLoaderErrorIndex
	plan.clientMessage = input.clientLoaderErrorMessage
	plan.cutIdx = derefErrorIdx + 1
	plan.headRouteCount = derefErrorIdx
	if plan.cutIdx < len(input.matches) {
		plan.depsForRouteData = getDepsFromData(
			input.matches[:plan.cutIdx],
			input.runtimeSnapshot.paths,
			input.runtimeSnapshot.clientEntryDeps,
		)
	}
	return plan
}

func (v *Vorma) prepareRouteDataExecutionInputs(
	r *http.Request,
	nestedRouter *nestedmux.Router,
) (routeDataExecutionInputs, bool) {
	v.mu.RLock()
	runtimeSnapshot := v.captureRuntimeSnapshotLocked()

	matchResults, found := nestedmux.FindMatches(nestedRouter, r)
	if !found {
		v.mu.RUnlock()
		return routeDataExecutionInputs{
			runtimeSnapshot: runtimeSnapshot,
		}, false
	}

	matches := matchResults.Matches
	v.mu.RUnlock()

	matchedPatterns := collectMatchedPatterns(matches)
	cacheKey := v.buildRouteDataCacheKey(
		matches,
		runtimeSnapshot.isDev,
		runtimeSnapshot.buildID,
		runtimeSnapshot.routeDataSnapshotVersion,
	)
	cached := loadOrBuildCachedItemSubset(
		v,
		cacheKey,
		matches,
		runtimeSnapshot.paths,
		runtimeSnapshot.clientEntryDeps,
		runtimeSnapshot.isDev,
		runtimeSnapshot.routeDataSnapshotVersion,
		runtimeSnapshot.routeDataCache,
	)

	return routeDataExecutionInputs{
		matchResults:    matchResults,
		matches:         matches,
		matchedPatterns: matchedPatterns,
		cached:          cached,
		runtimeSnapshot: runtimeSnapshot,
	}, true
}

func collectMatchedPatterns(matches []*nestedmatcher.Match) []string {
	matchedPatterns := make([]string, len(matches))
	for i, match := range matches {
		matchedPatterns[i] = match.OriginalPattern()
	}
	return matchedPatterns
}

func loadOrBuildCachedItemSubset(
	v *Vorma,
	cacheKey string,
	matches []*nestedmatcher.Match,
	pathsSnapshot map[string]*Path,
	clientEntryDepsSnapshot []string,
	isDev bool,
	expectedSnapshotVersion uint64,
	routeDataCacheSnapshot *sync.Map,
) *cachedItemSubset {
	if routeDataCacheSnapshot == nil {
		return buildCachedItemSubset(
			matches,
			pathsSnapshot,
			clientEntryDepsSnapshot,
			isDev,
		)
	}

	if cachedValue, isCached := routeDataCacheSnapshot.Load(cacheKey); isCached {
		return cachedValue.(*cachedItemSubset)
	}

	cached := buildCachedItemSubset(
		matches,
		pathsSnapshot,
		clientEntryDepsSnapshot,
		isDev,
	)
	if v.isRouteDataSnapshotVersionCurrent(expectedSnapshotVersion) {
		routeDataCacheSnapshot.Store(cacheKey, cached)
	}
	return cached
}

func (v *Vorma) isRouteDataSnapshotVersionCurrent(
	expectedSnapshotVersion uint64,
) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v._routeDataSnapshotVersion == expectedSnapshotVersion
}

func computeHasRootData(
	matchResults *nestedmatcher.Results,
	tasksResults *nestedmux.TasksResults,
) bool {
	return len(matchResults.Matches) > 0 &&
		matchResults.Matches[0].NormalizedPattern() == "" &&
		tasksResults.HasTaskHandlerAt(0)
}

func detectTerminalStateFromMergedResponseProxy(
	mergedResponseProxy *response.Proxy,
) routeTerminalState {
	if mergedResponseProxy == nil {
		return routeTerminalStateNone
	}

	if mergedResponseProxy.IsError() {
		return routeTerminalStateError
	}
	if mergedResponseProxy.IsRedirect() {
		return routeTerminalStateRedirect
	}

	return routeTerminalStateNone
}

func buildCachedItemSubset(
	matches []*nestedmatcher.Match,
	pathsSnapshot map[string]*Path,
	clientEntryDepsSnapshot []string,
	isDev bool,
) *cachedItemSubset {
	cached := &cachedItemSubset{
		ImportURLs:      make([]string, 0, len(matches)),
		ExportKeys:      make([]string, 0, len(matches)),
		ErrorExportKeys: make([]string, 0, len(matches)),
	}

	for _, match := range matches {
		foundPath := pathsSnapshot[match.OriginalPattern()]
		if foundPath == nil || foundPath.SrcPath == "" {
			cached.ImportURLs = append(cached.ImportURLs, "")
			cached.ExportKeys = append(cached.ExportKeys, "")
			cached.ErrorExportKeys = append(cached.ErrorExportKeys, "")
			continue
		}
		pathToUse := foundPath.OutPath
		if isDev {
			pathToUse = foundPath.SrcPath
		}
		cached.ImportURLs = append(cached.ImportURLs, "/"+pathToUse)
		cached.ExportKeys = append(cached.ExportKeys, foundPath.ExportKey)
		cached.ErrorExportKeys = append(
			cached.ErrorExportKeys,
			foundPath.ErrorExportKey,
		)
	}

	cached.Deps = getDepsFromData(
		matches,
		pathsSnapshot,
		clientEntryDepsSnapshot,
	)
	return cached
}

func (v *Vorma) collectLoadersDataAndErrors(
	tasksResults *nestedmux.TasksResults,
	matchedPatterns []string,
) ([]any, []error) {
	numberOfLoaders := len(matchedPatterns)
	loadersData := make([]any, numberOfLoaders)
	loadersErrs := make([]error, numberOfLoaders)
	if numberOfLoaders == 0 {
		return loadersData, loadersErrs
	}

	for i := 0; i < numberOfLoaders; i++ {
		result := tasksResults.Slice[i]
		loadersData[i] = result.Data()
		loadersErrs[i] = result.Err()

		if result.RanTask() && loadersErrs[i] == nil {
			shouldWarn := reflectutil.ExcludingNoneGetIsNilOrUltimatelyPointsToNil(
				loadersData[i],
			)
			if shouldWarn {
				v.Log.Warn(
					"Do not return nil values from loaders unless the underlying type is an empty struct or you are returning an error.",
					"pattern",
					matchedPatterns[i],
				)
			}
		}
	}
	return loadersData, loadersErrs
}

func findFirstErrorIndex(errs []error) *int {
	for i, err := range errs {
		if err != nil {
			out := i
			return &out
		}
	}
	return nil
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

func buildRouteDataCore(
	matchResults *nestedmatcher.Results,
	matchedPatterns []string,
	loadersData []any,
	cached *cachedItemSubset,
	hasRootData bool,
	outermostErrorIdx *int,
	clientMsg string,
	cutIdx int,
	deps []string,
) *RouteDataCore {
	return &RouteDataCore{
		OutermostServerError:    clientMsg,
		OutermostServerErrorIdx: outermostErrorIdx,
		ErrorExportKeys:         cached.ErrorExportKeys[:cutIdx],
		MatchedPatterns:         matchedPatterns[:cutIdx],
		LoadersData:             loadersData[:cutIdx],
		ImportURLs:              cached.ImportURLs[:cutIdx],
		ExportKeys:              cached.ExportKeys[:cutIdx],
		HasRootData:             hasRootData,
		Params:                  matchResults.Params,
		SplatValues:             matchResults.SplatValues,
		Deps:                    deps,
	}
}

func collectFlattenedHeadElementsForPrefix(
	responseProxies []*response.Proxy,
	routeCount int,
) []*htmlutil.Element {
	if routeCount <= 0 || len(responseProxies) == 0 {
		return nil
	}
	if routeCount > len(responseProxies) {
		routeCount = len(responseProxies)
	}

	headElsByRoute := make([][]*htmlutil.Element, 0, routeCount)
	total := 0
	for routeIdx := 0; routeIdx < routeCount; routeIdx++ {
		routeElements := responseProxies[routeIdx].HeadEls().Collect()
		headElsByRoute = append(headElsByRoute, routeElements)
		total += len(routeElements)
	}

	flattenedHeadEls := make([]*htmlutil.Element, 0, total)
	for _, routeElements := range headElsByRoute {
		flattenedHeadEls = append(flattenedHeadEls, routeElements...)
	}
	return flattenedHeadEls
}

func (v *Vorma) buildRouteDataCacheKey(
	matches []*nestedmatcher.Match,
	isDev bool,
	buildID string,
	routeDataSnapshotVersion uint64,
) string {
	snapshotVersionString := strconv.FormatUint(routeDataSnapshotVersion, 10)
	var sb strings.Builder
	sb.Grow(len(buildID) + len(snapshotVersionString) + (len(matches) * 16) + 3)
	if isDev {
		sb.WriteByte('1')
	} else {
		sb.WriteByte('0')
	}
	sb.WriteByte('|')
	sb.WriteString(snapshotVersionString)
	sb.WriteByte('|')
	sb.WriteString(buildID)
	sb.WriteByte('|')
	for _, match := range matches {
		sb.WriteString(match.NormalizedPattern())
		sb.WriteByte(';')
	}
	return sb.String()
}

func (v *Vorma) getUIRouteData(
	w http.ResponseWriter,
	r *http.Request,
	nestedRouter *nestedmux.Router,
	isJSON bool,
	requestedBuildID string,
) *RouteResult {
	res := response.New(w)
	routeResult := v.getRouteDataStage1(w, r, nestedRouter, requestedBuildID)
	if routeResult.mergedResponseProxy != nil {
		routeResult.mergedResponseProxy.ApplyToResponseWriter(w, r)
	}
	if routeResult.terminalState != routeTerminalStateNone {
		return routeResult
	}

	defaultHeadElsRaw, err := v.getDefaultHeadElsRaw(r)
	if err != nil {
		v.Log.Error("Error in getUIRouteData", "error", err.Error())
		res.InternalServerError()
		return &RouteResult{
			terminalState: routeTerminalStateError,
			buildID:       routeResult.buildID,
		}
	}

	assets := v.buildRouteAssets(routeResult, defaultHeadElsRaw, isJSON)
	return &RouteResult{
		buildID:                   routeResult.buildID,
		core:                      routeResult.core,
		assets:                    assets,
		htmlRenderSnapshot:        routeResult.htmlRenderSnapshot,
		routeManifestFileSnapshot: routeResult.routeManifestFileSnapshot,
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

func (v *Vorma) buildRouteAssets(
	routeResult *RouteResult,
	defaultHeadElsRaw []*htmlutil.Element,
	isJSON bool,
) *RouteAssets {
	cssBundles := routeResult.cssBundles
	combinedHeadEls := combineDefaultAndRouteHeadElements(
		defaultHeadElsRaw,
		routeResult.headElements,
	)

	if shouldAppendProductionAssetLinks(routeResult.isDev, isJSON) {
		combinedHeadEls = appendProductionAssetLinks(
			combinedHeadEls,
			v.Wave.PublicPathPrefix(),
			routeResult.core.Deps,
			cssBundles,
		)
	}

	headEls := v.headElsInst.ToSortedAndPreEscapedHeadEls(combinedHeadEls)
	return &RouteAssets{
		SortedAndPreEscapedHeadEls: headEls,
		CSSBundles:                 cssBundles,
		ViteDevURL:                 getViteDevURLForMode(routeResult.isDev),
	}
}

func combineDefaultAndRouteHeadElements(
	defaultHeadElsRaw []*htmlutil.Element,
	routeHeadEls []*htmlutil.Element,
) []*htmlutil.Element {
	out := make(
		[]*htmlutil.Element,
		0,
		len(defaultHeadElsRaw)+len(routeHeadEls),
	)
	out = append(out, defaultHeadElsRaw...)
	out = append(out, routeHeadEls...)
	return out
}

func shouldAppendProductionAssetLinks(isDev bool, isJSON bool) bool {
	return !isDev && !isJSON
}

func appendProductionAssetLinks(
	headElements []*htmlutil.Element,
	publicPathPrefix string,
	deps []string,
	cssBundles []string,
) []*htmlutil.Element {
	capacityToGrow := len(deps) + len(cssBundles)
	out := make([]*htmlutil.Element, 0, len(headElements)+capacityToGrow)
	out = append(out, headElements...)

	for _, dep := range deps {
		out = append(out, &htmlutil.Element{
			Tag: "link",
			AttributesKnownSafe: map[string]string{
				"rel":  "modulepreload",
				"href": publicPathPrefix + dep,
			},
			SelfClosing: true,
		})
	}

	for _, cssBundle := range cssBundles {
		out = append(out, &htmlutil.Element{
			Tag: "link",
			AttributesKnownSafe: map[string]string{
				"rel":  "stylesheet",
				"href": publicPathPrefix + cssBundle,
			},
			Attributes: map[string]string{
				"data-vorma-css-bundle": cssBundle,
			},
			SelfClosing: true,
		})
	}

	return out
}

type SSRInnerHTMLInput struct {
	VormaSymbolStr   string
	IsDev            bool
	ViteDevURL       string
	BuildID          string
	RootElementID    string
	PublicPathPrefix string
	DeploymentID     string
	RouteManifestURL string
	*RouteDataCore
	CSSBundles []string
}

const ssrInnerHTMLTmplStr = `<script>
globalThis[Symbol.for("{{.VormaSymbolStr}}")] = {};
const x = globalThis[Symbol.for("{{.VormaSymbolStr}}")];
x.patternToWaitFnMap = {};
x.clientLoadersData = [];
x.isDev = {{.IsDev}};
x.viteDevURL = {{.ViteDevURL}};
x.buildID = {{.BuildID}};
x.rootElementID = "{{.RootElementID}}";
x.publicPathPrefix = "{{.PublicPathPrefix}}";
x.outermostServerError = {{.OutermostServerError}};
x.outermostServerErrorIdx = {{.OutermostServerErrorIdx}};
x.errorExportKeys = {{.ErrorExportKeys}};
x.matchedPatterns = {{.MatchedPatterns}};
x.loadersData = {{.LoadersData}};
x.importURLs = {{.ImportURLs}};
x.exportKeys = {{.ExportKeys}};
x.hasRootData = {{.HasRootData}};
x.params = {{.Params}};
x.splatValues = {{.SplatValues}};
x.deps = {{.Deps}};
x.cssBundles = {{.CSSBundles}};
x.deploymentID = {{.DeploymentID}};
x.routeManifestURL = {{.RouteManifestURL}};
</script>`

var ssrInnerTmpl = template.Must(template.New("ssr").Parse(ssrInnerHTMLTmplStr))

type GetSSRInnerHTMLOutput struct {
	Script     *template.HTML
	Sha256Hash string
}

type ssrRuntimeSnapshot struct {
	isDev             bool
	buildID           string
	routeManifestFile string
}

func (v *Vorma) getSSRInnerHTML(
	routeData *RouteDataFinal,
) (*GetSSRInnerHTMLOutput, error) {
	v.mu.RLock()
	snapshot := ssrRuntimeSnapshot{
		isDev:             v._isDev,
		buildID:           v._buildID,
		routeManifestFile: v._routeManifestFile,
	}
	v.mu.RUnlock()

	return v.getSSRInnerHTMLWithRuntimeState(routeData, snapshot)
}

func (v *Vorma) getSSRInnerHTMLWithRuntimeState(
	routeData *RouteDataFinal,
	snapshot ssrRuntimeSnapshot,
) (*GetSSRInnerHTMLOutput, error) {
	if routeData == nil {
		return nil, fmt.Errorf("routeData cannot be nil")
	}
	if routeData.RouteDataCore == nil {
		return nil, fmt.Errorf("routeData.RouteDataCore cannot be nil")
	}
	for i, loaderData := range routeData.RouteDataCore.LoadersData {
		if err := json.NewEncoder(io.Discard).Encode(loaderData); err != nil {
			return nil, fmt.Errorf(
				"routeData.LoadersData[%d] must be JSON-serializable: %w",
				i,
				err,
			)
		}
	}

	var htmlBuilder strings.Builder
	publicPathPrefix := v.Wave.PublicPathPrefix()

	dto := SSRInnerHTMLInput{
		VormaSymbolStr:   VormaSymbolStr,
		IsDev:            snapshot.isDev,
		ViteDevURL:       routeData.ViteDevURL,
		BuildID:          snapshot.buildID,
		RootElementID:    v.ClientRootElementID(),
		PublicPathPrefix: publicPathPrefix,
		RouteManifestURL: path.Join(
			publicPathPrefix,
			snapshot.routeManifestFile,
		),
		RouteDataCore: routeData.RouteDataCore,
		CSSBundles:    routeData.CSSBundles,
	}

	if envutil.GetBool("VERCEL_SKEW_PROTECTION_ENABLED", false) {
		dto.DeploymentID = envutil.GetStr("VERCEL_DEPLOYMENT_ID", "")
	}

	if err := ssrInnerTmpl.Execute(&htmlBuilder, dto); err != nil {
		return nil, fmt.Errorf(
			"could not execute SSR inner HTML template: %w",
			err,
		)
	}

	innerHTML := htmlBuilder.String()
	innerHTML = strings.TrimPrefix(innerHTML, "<script>")
	innerHTML = strings.TrimSuffix(innerHTML, "</script>")

	el := htmlutil.Element{
		Tag:                 "script",
		AttributesKnownSafe: map[string]string{"type": "module"},
		DangerousInnerHTML:  innerHTML,
	}

	sha256Hash, err := htmlutil.ComputeContentSha256(&el)
	if err != nil {
		return nil, fmt.Errorf("could not compute CSP hash: %w", err)
	}

	renderedEl, err := htmlutil.RenderElement(&el)
	if err != nil {
		return nil, fmt.Errorf("could not render SSR inner HTML: %w", err)
	}

	return &GetSSRInnerHTMLOutput{
		Script:     &renderedEl,
		Sha256Hash: sha256Hash,
	}, nil
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

// PathsFile represents the serialized paths data written to disk.
type PathsFile struct {
	Stage             string           `json:"stage"`
	BuildID           string           `json:"buildID,omitempty"`
	ClientEntrySrc    string           `json:"clientEntrySrc"`
	Paths             map[string]*Path `json:"paths"`
	RouteManifestFile string           `json:"routeManifestFile"`

	// Stage two only
	ClientEntryOut    string              `json:"clientEntryOut,omitempty"`
	ClientEntryDeps   []string            `json:"clientEntryDeps,omitempty"`
	DepToCSSBundleMap map[string][]string `json:"depToCSSBundleMap,omitempty"`
}

// Directory name constants.
const (
	VormaOutDirname = "vorma_out"
)

// File name constants.
const (
	VormaPathsStageOneJSONFileName = "vorma_paths_stage_1.json"
	VormaPathsStageTwoJSONFileName = "vorma_paths_stage_2.json"
)

// Output prefix constants.
const (
	VormaOutPrefix               = "vorma_out_"
	VormaVitePrehashedFilePrefix = VormaOutPrefix + "vite_"
	VormaRouteManifestPrefix     = VormaOutPrefix + "vorma_internal_route_manifest_"
)

func GetVormaPathsStageOneJSONPath() string {
	return path.Join(VormaOutDirname, VormaPathsStageOneJSONFileName)
}

func GetVormaPathsStageTwoJSONPath() string {
	return path.Join(VormaOutDirname, VormaPathsStageTwoJSONFileName)
}

type runtimeLifecycleState string

const (
	runtimeLifecycleStateUninitialized   runtimeLifecycleState = "uninitialized"
	runtimeLifecycleStateInitializing    runtimeLifecycleState = "initializing"
	runtimeLifecycleStateReady           runtimeLifecycleState = "ready"
	runtimeLifecycleStateReloadingRoutes runtimeLifecycleState = "reloading-routes"
	runtimeLifecycleStateReloadingHTML   runtimeLifecycleState = "reloading-html-template"
)

func (v *Vorma) transitionLifecycleStateLocked(
	nextState runtimeLifecycleState,
	reason string,
	lastError string,
) {
	previousState := v._lifecycleState
	v._lifecycleTransitionSeq++
	v._lifecycleState = nextState
	v._lifecycleLastError = lastError

	v.Log.Debug(
		"Vorma lifecycle transition",
		"seq",
		v._lifecycleTransitionSeq,
		"from",
		previousState,
		"to",
		nextState,
		"reason",
		reason,
		"error",
		lastError,
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

func (v *Vorma) lifecycleStateForRouteCommitLocked() runtimeLifecycleState {
	if v._paths == nil {
		return runtimeLifecycleStateInitializing
	}
	return runtimeLifecycleStateReloadingRoutes
}

type runtimeRouteArtifacts struct {
	buildID           string
	clientEntrySrc    string
	clientEntryOut    string
	clientEntryDeps   []string
	depToCSSBundleMap map[string][]string
	routeManifestFile string
	parsedClientPaths map[string]*Path
}

func buildRuntimeRouteArtifacts(
	pathsFile *PathsFile,
) (*runtimeRouteArtifacts, error) {
	if pathsFile == nil {
		return nil, fmt.Errorf("paths file is nil")
	}

	return &runtimeRouteArtifacts{
		buildID:           pathsFile.BuildID,
		clientEntrySrc:    pathsFile.ClientEntrySrc,
		clientEntryOut:    pathsFile.ClientEntryOut,
		clientEntryDeps:   pathsFile.ClientEntryDeps,
		depToCSSBundleMap: pathsFile.DepToCSSBundleMap,
		routeManifestFile: pathsFile.RouteManifestFile,
		parsedClientPaths: pathsFile.Paths,
	}, nil
}

type routeArtifactCommitMode uint8

const (
	routeArtifactCommitModeInit routeArtifactCommitMode = iota
	routeArtifactCommitModeDevReload
)

func (v *Vorma) commitRouteArtifactsLocked(
	artifacts *runtimeRouteArtifacts,
	rebuildNestedRouter bool,
	commitMode routeArtifactCommitMode,
) {
	if artifacts == nil {
		panic("runtimeRouteArtifacts cannot be nil")
	}

	v.applyRuntimeRouteArtifactsMetadataLocked(artifacts)

	switch commitMode {
	case routeArtifactCommitModeDevReload:
		v.routes().SyncFromDevReload(artifacts.parsedClientPaths)
	default:
		v.routes().
			ReplaceParsedPathsForInit(artifacts.parsedClientPaths, rebuildNestedRouter)
	}
}

func (v *Vorma) commitRootTemplateLocked(rootTemplate *template.Template) {
	v._rootTemplate = rootTemplate
}

// RuntimeSnapshot captures request-serving runtime state for one coherent
// generation.
type RuntimeSnapshot struct {
	buildID                  string
	isDev                    bool
	paths                    map[string]*Path
	clientEntryDeps          []string
	clientEntryOut           string
	depToCSSBundleMap        map[string][]string
	rootTemplate             *template.Template
	routeManifestFile        string
	routeDataSnapshotVersion uint64
	routeDataCache           *sync.Map
}

func (v *Vorma) captureRuntimeSnapshotLocked() RuntimeSnapshot {
	return RuntimeSnapshot{
		buildID:                  v._buildID,
		isDev:                    v._isDev,
		paths:                    v._paths,
		clientEntryDeps:          v._clientEntryDeps,
		clientEntryOut:           v._clientEntryOut,
		depToCSSBundleMap:        v._depToCSSBundleMap,
		rootTemplate:             v._rootTemplate,
		routeManifestFile:        v._routeManifestFile,
		routeDataSnapshotVersion: v._routeDataSnapshotVersion,
		routeDataCache:           v._routeDataCache,
	}
}

func (snapshot RuntimeSnapshot) toLoadersHTMLRender() loadersHTMLRenderSnapshot {
	return loadersHTMLRenderSnapshot{
		isDevMode:      snapshot.isDev,
		clientEntryOut: snapshot.clientEntryOut,
		rootTemplate:   snapshot.rootTemplate,
	}
}

// invalidateRouteDataCacheLocked invalidates route-data cache entries for
// requests that still reference an older runtime snapshot.
//
// Caller must hold v.mu.Lock().
func (v *Vorma) invalidateRouteDataCacheLocked() {
	v._routeDataSnapshotVersion++
	v._routeDataCache = &sync.Map{}
}

func clonePathsMap(paths map[string]*Path) map[string]*Path {
	if paths == nil {
		return make(map[string]*Path)
	}

	cloned := make(map[string]*Path, len(paths))
	for pattern, p := range paths {
		cloned[pattern] = clonePath(p)
	}
	return cloned
}

func clonePathsMapOrNil(paths map[string]*Path) map[string]*Path {
	if paths == nil {
		return nil
	}
	return clonePathsMap(paths)
}

func cloneStringSliceOrNil(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}

func cloneDepToCSSBundleMapOrEmpty(
	depToBundles map[string][]string,
) map[string][]string {
	if depToBundles == nil {
		return make(map[string][]string)
	}

	cloned := make(map[string][]string, len(depToBundles))
	for dep, bundles := range depToBundles {
		cloned[dep] = append([]string(nil), bundles...)
	}
	return cloned
}

func cloneDepToCSSBundleMapOrNil(
	depToBundles map[string][]string,
) map[string][]string {
	if depToBundles == nil {
		return nil
	}
	return cloneDepToCSSBundleMapOrEmpty(depToBundles)
}

func (v *Vorma) applyRuntimeRouteArtifactsMetadataLocked(
	artifacts *runtimeRouteArtifacts,
) {
	if artifacts == nil {
		v._buildID = ""
		v._clientEntrySrc = ""
		v._clientEntryOut = ""
		v._clientEntryDeps = nil
		v._depToCSSBundleMap = make(map[string][]string)
		v._routeManifestFile = ""
		return
	}

	v._buildID = artifacts.buildID
	v._clientEntrySrc = artifacts.clientEntrySrc
	v._clientEntryOut = artifacts.clientEntryOut
	v._clientEntryDeps = cloneStringSliceOrNil(artifacts.clientEntryDeps)
	v._depToCSSBundleMap = cloneDepToCSSBundleMapOrEmpty(
		artifacts.depToCSSBundleMap,
	)
	v._routeManifestFile = artifacts.routeManifestFile
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
	v := r.vorma
	v._paths = clonePathsMap(paths)
	r.mergeServerRoutes()
	v.invalidateRouteDataCacheLocked()
	r.rebuildNestedRouterFromCurrentPaths()
}

// ReplaceParsedPathsForInit updates route state from parsed client routes for
// init/re-init flows. This does not merge server-only handlers into parsed-path
// state, but can rebuild nested-router registrations when requested.
// Caller must hold v.mu.Lock().
func (r *RouteRegistry) ReplaceParsedPathsForInit(
	paths map[string]*Path,
	rebuildNestedRouter bool,
) {
	v := r.vorma
	v._paths = clonePathsMap(paths)
	v.invalidateRouteDataCacheLocked()
	if rebuildNestedRouter {
		r.rebuildNestedRouterFromCurrentPaths()
	}
}

// mergeServerRoutes adds server-only routes to paths.
// Caller must hold v.mu.Lock().
func (r *RouteRegistry) mergeServerRoutes() {
	v := r.vorma
	allServerRoutes := v.LoadersRouter().NestedRouter.AllRoutes()
	for pattern := range allServerRoutes {
		if !v.LoadersRouter().NestedRouter.HasTaskHandler(pattern) {
			continue
		}
		if _, hasClientRoute := v._paths[pattern]; !hasClientRoute {
			v._paths[pattern] = &Path{
				OriginalPattern: pattern,
				SrcPath:         "",
				ExportKey:       "default",
				ErrorExportKey:  "",
			}
		}
	}
}

func (r *RouteRegistry) rebuildNestedRouterFromCurrentPaths() {
	v := r.vorma
	patterns := make([]string, 0, len(v._paths))
	for pattern := range v._paths {
		patterns = append(patterns, pattern)
	}
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

	pathsFile, err := v.getBasePathsFromFS(privateFS, true)
	if err != nil {
		return fmt.Errorf("load paths from disk: %w", err)
	}
	runtimeArtifacts, err := buildRuntimeRouteArtifacts(pathsFile)
	if err != nil {
		return fmt.Errorf("build runtime route artifacts: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if err := v.guardDevOnlyReloadLocked("route reload"); err != nil {
		return err
	}

	v.transitionLifecycleStateLocked(
		runtimeLifecycleStateReloadingRoutes,
		"dev route artifacts commit start",
		"",
	)
	v.commitRouteArtifactsLocked(
		runtimeArtifacts,
		true,
		routeArtifactCommitModeDevReload,
	)
	v.transitionLifecycleStateLocked(
		runtimeLifecycleStateReady,
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
	if privateFS == nil {
		return fmt.Errorf("private fs is nil")
	}

	tmpl, err := template.ParseFS(privateFS, rootTemplateLocation)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	if err := v.guardDevOnlyReloadLocked("template reload"); err != nil {
		return err
	}
	v.transitionLifecycleStateLocked(
		runtimeLifecycleStateReloadingHTML,
		"dev html template commit start",
		"",
	)
	v.commitRootTemplateLocked(tmpl)
	v.transitionLifecycleStateLocked(
		runtimeLifecycleStateReady,
		"dev html template commit complete",
		"",
	)

	v.Log.Info("HTML template reloaded")
	return nil
}

func getViteDevURLForMode(isDevMode bool) string {
	if !isDevMode {
		return ""
	}
	return fmt.Sprintf("http://localhost:%s", viteutil.GetVitePortStr())
}

func (v *Vorma) getDeps(
	matches []*nestedmatcher.Match,
	paths map[string]*Path,
) []string {
	v.mu.RLock()
	clientEntryDeps := v._clientEntryDeps
	v.mu.RUnlock()

	return getDepsFromData(matches, paths, clientEntryDeps)
}

func getDepsFromData(
	matches []*nestedmatcher.Match,
	paths map[string]*Path,
	clientEntryDeps []string,
) []string {
	var deps []string
	seen := make(map[string]struct{}, len(matches))
	handleDeps := func(src []string) {
		for _, d := range src {
			if _, ok := seen[d]; !ok {
				deps = append(deps, d)
				seen[d] = struct{}{}
			}
		}
	}
	if clientEntryDeps != nil {
		handleDeps(clientEntryDeps)
	}
	for _, match := range matches {
		path := paths[match.OriginalPattern()]
		if path == nil {
			continue
		}
		handleDeps(path.Deps)
	}
	return deps
}

func (v *Vorma) getCSSBundles(deps []string) []string {
	v.mu.RLock()
	clientEntryOut := v._clientEntryOut
	depToCSSBundleMap := v._depToCSSBundleMap
	v.mu.RUnlock()

	return getCSSBundles(deps, clientEntryOut, depToCSSBundleMap)
}

func getCSSBundles(
	deps []string,
	clientEntryOut string,
	depToCSSBundleMap map[string][]string,
) []string {
	clientEntryBundles := depToCSSBundleMap[clientEntryOut]
	seen := make(map[string]struct{})
	cssBundles := make([]string, 0, len(deps))

	addBundles := func(bundles []string) {
		for _, bundle := range bundles {
			if _, exists := seen[bundle]; !exists {
				seen[bundle] = struct{}{}
				cssBundles = append(cssBundles, bundle)
			}
		}
	}

	if len(clientEntryBundles) > 0 {
		addBundles(clientEntryBundles)
	}

	for _, dep := range deps {
		if bundles, exists := depToCSSBundleMap[dep]; exists {
			addBundles(bundles)
		}
	}

	return cssBundles
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

	pathsFile, err := v.getBasePathsFromFS(privateFS, isDev)
	if err != nil {
		return fmt.Errorf("could not get base paths: %w", err)
	}
	runtimeArtifacts, err := buildRuntimeRouteArtifacts(pathsFile)
	if err != nil {
		return fmt.Errorf("could not build runtime route artifacts: %w", err)
	}

	tmpl, err := template.ParseFS(privateFS, v.Config.HTMLTemplateLocation)
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
		runtimeArtifacts,
		wasInitialized,
		routeArtifactCommitModeInit,
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
		runtimeLifecycleStateReady,
		"init commit complete",
		"",
	)
	return nil
}

func (v *Vorma) getBasePaths_StageOneOrTwo(isDev bool) (*PathsFile, error) {
	return v.getBasePathsFromFS(v._privateFS, isDev)
}

func (v *Vorma) getBasePathsFromFS(
	privateFS fs.FS,
	isDev bool,
) (*PathsFile, error) {
	if privateFS == nil {
		return nil, fmt.Errorf("private fs is nil")
	}

	fileToUse := VormaPathsStageOneJSONFileName
	if !isDev {
		fileToUse = VormaPathsStageTwoJSONFileName
	}

	file, err := privateFS.Open(path.Join("vorma_out", fileToUse))
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", fileToUse, err)
	}
	defer file.Close()

	var pathsFile PathsFile
	if err := json.NewDecoder(file).Decode(&pathsFile); err != nil {
		return nil, fmt.Errorf("could not decode %s: %w", fileToUse, err)
	}
	if err := validatePathsFileStructuralIntegrity(&pathsFile); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", fileToUse, err)
	}
	if err := validatePathsFileSemanticIntegrity(&pathsFile, isDev); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", fileToUse, err)
	}
	return &pathsFile, nil
}

func validatePathsFileStructuralIntegrity(pathsFile *PathsFile) error {
	if pathsFile == nil {
		return fmt.Errorf("paths file is nil")
	}
	if pathsFile.Paths == nil {
		return nil
	}
	for mapKeyPattern, pathEntry := range pathsFile.Paths {
		if pathEntry == nil {
			return fmt.Errorf("paths[%q] cannot be null", mapKeyPattern)
		}
		if mapKeyPattern != "" && pathEntry.OriginalPattern == "" {
			return fmt.Errorf(
				"paths[%q].originalPattern is required",
				mapKeyPattern,
			)
		}
		if pathEntry.OriginalPattern != mapKeyPattern {
			return fmt.Errorf(
				"paths[%q].originalPattern=%q does not match key",
				mapKeyPattern,
				pathEntry.OriginalPattern,
			)
		}
	}
	return nil
}

func validatePathsFileSemanticIntegrity(
	pathsFile *PathsFile,
	isDev bool,
) error {
	if pathsFile == nil {
		return fmt.Errorf("paths file is nil")
	}
	if strings.TrimSpace(pathsFile.RouteManifestFile) == "" {
		return fmt.Errorf("routeManifestFile is required")
	}
	if !isDev && strings.TrimSpace(pathsFile.ClientEntryOut) == "" {
		return fmt.Errorf("clientEntryOut is required")
	}

	for mapKeyPattern, pathEntry := range pathsFile.Paths {
		if pathEntry == nil {
			continue
		}
		if pathEntry.SrcPath != "" &&
			strings.TrimSpace(pathEntry.ExportKey) == "" {
			return fmt.Errorf(
				"paths[%q].exportKey is required when srcPath is set",
				mapKeyPattern,
			)
		}
		if !isDev && pathEntry.SrcPath != "" &&
			strings.TrimSpace(pathEntry.OutPath) == "" {
			return fmt.Errorf(
				"paths[%q].outPath is required in production mode",
				mapKeyPattern,
			)
		}
	}

	return nil
}

// PrettyPrintFS is a debug utility for fs.FS instances.
func PrettyPrintFS(fsys fs.FS) error {
	return fs.WalkDir(
		fsys,
		".",
		func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				fmt.Println(p)
			} else {
				fmt.Printf("%s (%s)\n", p, d.Type())
			}
			return nil
		},
	)
}
