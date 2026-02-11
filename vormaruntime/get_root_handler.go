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
	// Dev_ReloadRoutesPath is the endpoint for reloading routes from disk.
	// Called by Wave after Process A has regenerated route artifacts.
	Dev_ReloadRoutesPath = "/__vorma/reload-routes"
	// Dev_ReloadTemplatePath is the endpoint for reloading the HTML template.
	Dev_ReloadTemplatePath = "/__vorma/reload-template"

	// Backward-compatibility aliases. Prefer Dev_*-prefixed names.
	DevReloadRoutesPath   = Dev_ReloadRoutesPath
	DevReloadTemplatePath = Dev_ReloadTemplatePath
)

// legacyHeadElsInstance preserves the package-level accessor behavior.
// Runtime internals use app-scoped instances to avoid cross-app rule leakage.
var legacyHeadElsInstance = headels.NewInstance("vorma")

func (v *Vorma) GetLoadersHandler(nestedRouter *mux.NestedRouter) mux.TasksCtxRequirerFunc {
	v.validateAndDecorateNestedRouter(nestedRouter)

	return mux.TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Dev-only fast reload endpoints
		if v.GetIsDevMode() {
			if r.URL.Path == Dev_ReloadRoutesPath {
				if err := v.devReloadRoutesFromDisk(); err != nil {
					v.Log.Error(fmt.Sprintf("route reload failed: %s", err))
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Write([]byte("ok"))
				return
			}
			if r.URL.Path == Dev_ReloadTemplatePath {
				if err := v.devReloadTemplateFromDisk(); err != nil {
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

		query := r.URL.Query()
		requestedBuildID := query.Get(VormaJSONQueryKey)
		isJSON := requestedBuildID != ""
		if isJSON && requestedBuildID != buildID {
			newURL := *r.URL
			q := query
			q.Del(VormaJSONQueryKey)
			newURL.RawQuery = q.Encode()
			res.SetHeader("X-Vorma-Reload", newURL.String())
			res.OK()
			return
		}

		routeResult := v.getUIRouteData(w, r, nestedRouter, isJSON)

		if routeResult.notFound {
			res.NotFound()
			return
		}
		if routeResult.didErr || routeResult.didRedirect {
			return
		}

		routeData := &RouteDataFinal{
			RouteDataCore: routeResult.core,
			Title:         routeResult.assets.SortedAndPreEscapedHeadEls.Title,
			Meta:          routeResult.assets.SortedAndPreEscapedHeadEls.Meta,
			Rest:          routeResult.assets.SortedAndPreEscapedHeadEls.Rest,
			CSSBundles:    routeResult.assets.CSSBundles,
			ViteDevURL:    routeResult.assets.ViteDevURL,
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
			he, err := v.headElsInst.Render(routeResult.assets.SortedAndPreEscapedHeadEls)
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
	return legacyHeadElsInstance
}
