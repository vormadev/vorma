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

const VormaBuildIDHeaderKey = "X-Vorma-Build-Id"
const VormaJSONQueryKey = "vorma_json"

const (
	DefaultDevReloadRoutesEndpointPath   = "/__vorma_internal/reload-routes"
	DefaultDevReloadTemplateEndpointPath = "/__vorma_internal/reload-template"
	DefaultTemplateDataKeyHeadElements   = "VormaHeadEls"
	DefaultTemplateDataKeyBodyScripts    = "VormaBodyScripts"
	DefaultTemplateDataKeySSRScript      = "VormaSSRScript"
	DefaultTemplateDataKeySSRScriptHash  = "VormaSSRScriptSha256Hash"
	DefaultTemplateDataKeyRootElementID  = "VormaRootID"
	DefaultClientRootElementID           = "vorma-root"
)

type loadersHTMLRenderSnapshot struct {
	isDevMode      bool
	clientEntryOut string
	rootTemplate   *template.Template
}

func (v *Vorma) GetLoadersHandler(nestedRouter *mux.NestedRouter) mux.TasksCtxRequirerFunc {
	v.validateAndDecorateNestedRouter(nestedRouter)

	return mux.TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v.handleDevReloadEndpoints(w, r, v.GetIsDevMode()) {
			return
		}

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
				v.Log.Error(fmt.Sprintf("Error marshalling JSON: %v", err))
				res.InternalServerError()
				return
			}
			return
		}

		htmlBytes, errorPrefix, err := v.buildLoadersHTMLResponseBytes(r, routeResult, routeData)
		if err != nil {
			v.Log.Error(fmt.Sprintf("%s: %v", errorPrefix, err))
			res.InternalServerError()
			return
		}
		res.HTMLBytes(htmlBytes)
	})
}

func (v *Vorma) captureLoadersHTMLRenderSnapshot() loadersHTMLRenderSnapshot {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return loadersHTMLRenderSnapshot{
		isDevMode:      v._isDev,
		clientEntryOut: v._clientEntryOut,
		rootTemplate:   v._rootTemplate,
	}
}

func (v *Vorma) handleDevReloadEndpoints(
	w http.ResponseWriter,
	r *http.Request,
	isDevMode bool,
) bool {
	if !isDevMode {
		return false
	}

	switch r.URL.Path {
	case v.DevReloadRoutesEndpointPath():
		if err := v.devReloadRoutesFromDisk(); err != nil {
			v.Log.Error(fmt.Sprintf("route reload failed: %s", err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return true
		}
		w.Write([]byte("ok"))
		return true
	case v.DevReloadTemplateEndpointPath():
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
		res.SetHeader("Cache-Control", "private, max-age=0, must-revalidate, no-cache")
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
	assets *RouteAssets,
	routeData *RouteDataFinal,
) (template.HTML, *template.HTML, string, error) {
	var eg errgroup.Group
	var ssrScript *template.HTML
	var ssrScriptSha256Hash string
	var headElements template.HTML

	eg.Go(func() error {
		he, err := v.headElsInst.Render(assets.SortedAndPreEscapedHeadEls)
		if err != nil {
			return fmt.Errorf("error getting head elements: %w", err)
		}
		headElements = he
		headElements += "\n" + v.Wave.GetCriticalCSSStyleElement()
		headElements += "\n" + v.Wave.GetStyleSheetLinkElement()
		return nil
	})

	eg.Go(func() error {
		sih, err := v.getSSRInnerHTML(routeData)
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
		routeResult.assets,
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

	htmlRenderSnapshot := v.captureLoadersHTMLRenderSnapshot()
	bodyScripts, err := v.getBodyScriptsForTemplate(htmlRenderSnapshot)
	if err != nil {
		return nil, "Error getting dev scripts", err
	}
	rootTemplateData[v.TemplateDataKeyBodyScripts()] = bodyScripts

	htmlBytes, err := executeRootTemplate(htmlRenderSnapshot.rootTemplate, rootTemplateData)
	if err != nil {
		return nil, "Error executing template", err
	}

	return htmlBytes, "", nil
}

func (v *Vorma) getRootTemplateDataOrEmpty(r *http.Request) (map[string]any, error) {
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

func (v *Vorma) getBodyScriptsForTemplate(htmlRenderSnapshot loadersHTMLRenderSnapshot) (template.HTML, error) {
	if !htmlRenderSnapshot.isDevMode {
		return template.HTML(
			fmt.Sprintf(
				`<script type="module" src="%s%s"></script>`,
				v.Wave.GetPublicPathPrefix(),
				htmlRenderSnapshot.clientEntryOut,
			),
		), nil
	}

	opts := viteutil.ToDevScriptsOptions{ClientEntry: v.Config.ClientEntry}
	if UIVariant(v.Config.UIVariant) == UIVariants.React {
		opts.Variant = viteutil.Variants.React
	} else {
		opts.Variant = viteutil.Variants.Other
	}

	devScripts, err := viteutil.ToDevScripts(opts)
	if err != nil {
		return "", err
	}

	return devScripts + "\n" + v.Wave.GetRefreshScript(), nil
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

func IsJSONRequest(r *http.Request) bool {
	return r.URL.Query().Get(VormaJSONQueryKey) != ""
}

func (v *Vorma) IsCurrentBuildJSONRequest(r *http.Request) bool {
	return r.URL.Query().Get(VormaJSONQueryKey) == v.GetBuildID()
}

func (v *Vorma) GetActionsHandler(router *mux.Router) mux.TasksCtxRequirerFunc {
	return mux.TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)
		res.SetHeader(VormaBuildIDHeaderKey, v.GetBuildID())
		router.ServeHTTP(w, r)
	})
}
