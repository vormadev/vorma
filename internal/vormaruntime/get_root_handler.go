package vormaruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/lab/viteutil"
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
