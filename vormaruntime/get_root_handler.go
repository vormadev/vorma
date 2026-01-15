package vormaruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/lab/viteutil"
	"golang.org/x/sync/errgroup"
)

const VormaBuildIDHeaderKey = "X-Vorma-Build-Id"
const VormaJSONQueryKey = "vorma_json"

const (
	// DevReloadRoutesPath is the endpoint for reloading routes from disk.
	// Called by Wave after Process A has regenerated route artifacts.
	DevReloadRoutesPath = "/__vorma/reload-routes"
	// DevReloadTemplatePath is the endpoint for reloading the HTML template.
	DevReloadTemplatePath = "/__vorma/reload-template"
)

var headElsInstance = headels.NewInstance("vorma")

func (v *Vorma) GetLoadersHandler(nestedRouter *mux.NestedRouter) mux.TasksCtxRequirerFunc {
	v.validateAndDecorateNestedRouter(nestedRouter)

	return mux.TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Dev-only fast reload endpoints
		if v.GetIsDevMode() {
			if r.URL.Path == DevReloadRoutesPath {
				if err := v.ReloadRoutesFromDisk(); err != nil {
					v.Log.Error(fmt.Sprintf("route reload failed: %s", err))
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Write([]byte("ok"))
				return
			}
			if r.URL.Path == DevReloadTemplatePath {
				if err := v.ReloadTemplateFromDisk(); err != nil {
					v.Log.Error(fmt.Sprintf("template reload failed: %s", err))
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Write([]byte("ok"))
				return
			}
		}

		buildID := v.GetBuildID()
		res := response.New(w)
		res.SetHeader(VormaBuildIDHeaderKey, buildID)

		isJSON := IsJSONRequest(r)
		if isJSON && !v.IsCurrentBuildJSONRequest(r) {
			newURL := *r.URL
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

		if w.Header().Get("Cache-Control") == "" {
			res.SetHeader("Cache-Control", "private, max-age=0, must-revalidate, no-cache")
		}

		if isJSON {
			jsonBytes, err := json.Marshal(routeData)
			if err != nil {
				v.Log.Error(fmt.Sprintf("Error marshalling JSON: %v", err))
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
			v.Log.Error(fmt.Sprintf("Error getting route data: %v", err))
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
			v.Log.Error(fmt.Sprintf("Error getting root template data: %v", err))
			res.InternalServerError()
			return
		}

		rootTemplateData["VormaHeadEls"] = headElements
		rootTemplateData["VormaSSRScript"] = ssrScript
		rootTemplateData["VormaSSRScriptSha256Hash"] = ssrScriptSha256Hash
		rootTemplateData["VormaRootID"] = "vorma-root"

		isDev := v.GetIsDevMode()
		clientEntryOut := v.GetClientEntryOut()

		if !isDev {
			rootTemplateData["VormaBodyScripts"] = template.HTML(
				fmt.Sprintf(`<script type="module" src="%s%s"></script>`,
					v.Wave.GetPublicPathPrefix(), clientEntryOut),
			)
		} else {
			opts := viteutil.ToDevScriptsOptions{ClientEntry: v.Config.ClientEntry}
			if UIVariant(v.Config.UIVariant) == UIVariants.React {
				opts.Variant = viteutil.Variants.React
			} else {
				opts.Variant = viteutil.Variants.Other
			}
			devScripts, err := viteutil.ToDevScripts(opts)
			if err != nil {
				v.Log.Error(fmt.Sprintf("Error getting dev scripts: %v", err))
				res.InternalServerError()
				return
			}
			rootTemplateData["VormaBodyScripts"] = devScripts + "\n" + v.Wave.GetRefreshScript()
		}

		var buf bytes.Buffer
		rootTemplate := v.GetRootTemplate()
		if err := rootTemplate.Execute(&buf, rootTemplateData); err != nil {
			v.Log.Error(fmt.Sprintf("Error executing template: %v", err))
			res.InternalServerError()
			return
		}
		res.HTMLBytes(buf.Bytes())
	})
}

func IsJSONRequest(r *http.Request) bool {
	return r.URL.Query().Get(VormaJSONQueryKey) != ""
}

func (v *Vorma) IsCurrentBuildJSONRequest(r *http.Request) bool {
	return r.URL.Query().Get(VormaJSONQueryKey) == v.GetBuildID()
}

func (v *Vorma) GetCurrentBuildID() string {
	return v.GetBuildID()
}

func (v *Vorma) GetActionsHandler(router *mux.Router) mux.TasksCtxRequirerFunc {
	return mux.TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)
		res.SetHeader(VormaBuildIDHeaderKey, v.GetBuildID())
		router.ServeHTTP(w, r)
	})
}

func GetHeadElsInstance() *headels.Instance {
	return headElsInstance
}
