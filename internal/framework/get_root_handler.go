package vorma

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/viteutil"
	"golang.org/x/sync/errgroup"
)

const VormaBuildIDHeaderKey = "X-Vorma-Build-Id"
const VormaJSONQueryKey = "vorma_json"

var headElsInstance = headels.NewInstance("vorma")

// Deprecated: use GetLoadersHandler instead.
func (v *Vorma) GetUIHandler(nestedRouter *mux.NestedRouter) mux.TasksCtxRequirerFunc {
	return v.GetLoadersHandler(nestedRouter)
}

func (v *Vorma) GetLoadersHandler(nestedRouter *mux.NestedRouter) mux.TasksCtxRequirerFunc {
	v.validateAndDecorateNestedRouter(nestedRouter)

	handler := mux.TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)
		res.SetHeader(VormaBuildIDHeaderKey, v._buildID)

		isJSON := IsJSONRequest(r)
		if isJSON && !v.IsCurrentBuildJSONRequest(r) {
			newURL, err := url.Parse(r.URL.Path)
			if err != nil {
				Log.Error(fmt.Sprintf("Error parsing URL: %v\n", err))
				res.InternalServerError()
				return
			}
			q := newURL.Query()
			q.Del(VormaJSONQueryKey)
			newURL.RawQuery = q.Encode()
			res.SetHeader("X-Vorma-Reload", newURL.String())
			res.OK()
			return
		}

		uiRouteData := v.getUIRouteData(w, r, nestedRouter, isJSON)

		if uiRouteData.notFound {
			res.NotFound()
			return
		}

		if uiRouteData.didErr || uiRouteData.didRedirect {
			return
		}

		routeData := &final_ui_data{
			ui_data_core: uiRouteData.ui_data_core,
			Title:        uiRouteData.state_2_final.SortedAndPreEscapedHeadEls.Title,
			Meta:         uiRouteData.state_2_final.SortedAndPreEscapedHeadEls.Meta,
			Rest:         uiRouteData.state_2_final.SortedAndPreEscapedHeadEls.Rest,
			CSSBundles:   uiRouteData.state_2_final.CSSBundles,
			ViteDevURL:   uiRouteData.state_2_final.ViteDevURL,
		}

		currentCacheControlHeader := w.Header().Get("Cache-Control")

		if currentCacheControlHeader == "" {
			// Set a conservative default cache control header
			res.SetHeader("Cache-Control", "private, max-age=0, must-revalidate, no-cache")
		}

		if isJSON {
			jsonBytes, err := json.Marshal(routeData)
			if err != nil {
				Log.Error(fmt.Sprintf("Error marshalling JSON: %v\n", err))
				res.InternalServerError()
				return
			}

			res.JSONBytes(jsonBytes)
			return
		}

		var eg errgroup.Group
		var ssrScript *template.HTML
		var ssrScriptSha256Hash string
		var headElements template.HTML

		eg.Go(func() error {
			he, err := headElsInstance.Render(uiRouteData.state_2_final.SortedAndPreEscapedHeadEls)
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
			Log.Error(fmt.Sprintf("Error getting route data: %v\n", err))
			res.InternalServerError()
			return
		}

		var rootTemplateData map[string]any
		var err error
		if v.getRootTemplateData != nil {
			rootTemplateData, err = v.getRootTemplateData(r)
		} else {
			rootTemplateData = make(map[string]any)
		}
		if err != nil {
			Log.Error(fmt.Sprintf("Error getting root template data: %v\n", err))
			res.InternalServerError()
			return
		}

		rootTemplateData["VormaHeadEls"] = headElements
		rootTemplateData["VormaSSRScript"] = ssrScript
		rootTemplateData["VormaSSRScriptSha256Hash"] = ssrScriptSha256Hash
		rootTemplateData["VormaRootID"] = "vorma-root"

		if !v._isDev {
			rootTemplateData["VormaBodyScripts"] = template.HTML(
				fmt.Sprintf(
					`<script type="module" src="%s%s"></script>`,
					v.Wave.GetPublicPathPrefix(), v._clientEntryOut,
				),
			)
		} else {
			opts := viteutil.ToDevScriptsOptions{ClientEntry: v._clientEntrySrc}
			if UIVariant(v.Wave.GetVormaUIVariant()) == UIVariants.React {
				opts.Variant = viteutil.Variants.React
			} else {
				opts.Variant = viteutil.Variants.Other
			}

			devScripts, err := viteutil.ToDevScripts(opts)
			if err != nil {
				Log.Error(fmt.Sprintf("Error getting dev scripts: %v\n", err))
				res.InternalServerError()
				return
			}

			rootTemplateData["VormaBodyScripts"] = devScripts + "\n" + v.Wave.GetRefreshScript()
		}

		var buf bytes.Buffer

		err = v._rootTemplate.Execute(&buf, rootTemplateData)
		if err != nil {
			Log.Error(fmt.Sprintf("Error executing template: %v\n", err))
			res.InternalServerError()
		}

		res.HTMLBytes(buf.Bytes())
	})

	return handler
}

// If true, is JSON, but may or may not be from an up-to-date client.
func IsJSONRequest(r *http.Request) bool {
	return r.URL.Query().Get(VormaJSONQueryKey) != ""
}

// If true, is both (1) JSON and (2) guaranteed to be from a client
// that has knowledge of the latest build ID.
func (v *Vorma) IsCurrentBuildJSONRequest(r *http.Request) bool {
	return r.URL.Query().Get(VormaJSONQueryKey) == v._buildID
}

// GetCurrentBuildID returns the current build ID of the Vorma instance.
func (v *Vorma) GetCurrentBuildID() string {
	return v._buildID
}

func (v *Vorma) GetActionsHandler(router *mux.Router) mux.TasksCtxRequirerFunc {
	return mux.TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)
		res.SetHeader(VormaBuildIDHeaderKey, v._buildID)
		router.ServeHTTP(w, r)
	})
}
