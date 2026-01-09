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

// DevRebuildRoutesPath is the endpoint path for fast route rebuilds.
// Wave calls this instead of running the full build hook when only
// vorma.routes.ts changes.
// This reduces rebuild time from ~1.5s to ~50ms.
const DevRebuildRoutesPath = "/__vorma/rebuild-routes"

// DevReloadTemplatePath is the endpoint path for fast template reloads.
// Wave calls this instead of a full restart when only the HTML template changes.
const DevReloadTemplatePath = "/__vorma/reload-template"

// DevRebuildFileMapPath is the endpoint path for regenerating filemap.ts.
// Used when public static files change.
const DevRebuildFileMapPath = "/__vorma/rebuild-filemap"

var headElsInstance = headels.NewInstance("vorma")

// Deprecated: use GetLoadersHandler instead.
func (v *Vorma) GetUIHandler(nestedRouter *mux.NestedRouter) mux.TasksCtxRequirerFunc {
	return v.GetLoadersHandler(nestedRouter)
}

func (v *Vorma) GetLoadersHandler(nestedRouter *mux.NestedRouter) mux.TasksCtxRequirerFunc {
	v.validateAndDecorateNestedRouter(nestedRouter)

	handler := mux.TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Dev-only fast rebuild endpoints
		// Use getter method for thread-safe access to _isDev
		if v.getIsDev() {
			if r.URL.Path == DevRebuildRoutesPath {
				if err := v.rebuildRoutesOnly(); err != nil {
					Log.Error(fmt.Sprintf("fast route rebuild failed: %s", err))
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}
			if r.URL.Path == DevReloadTemplatePath {
				if err := v.reloadTemplate(); err != nil {
					Log.Error(fmt.Sprintf("fast template reload failed: %s", err))
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}
			if r.URL.Path == DevRebuildFileMapPath {
				if err := v.Wave.WritePublicFileMapTS(v.config.TSGenOutDir); err != nil {
					Log.Error(fmt.Sprintf("fast filemap rebuild failed: %s", err))
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
				return
			}
		}

		// Get buildID once for this request using thread-safe getter
		buildID := v.getBuildID()

		res := response.New(w)
		res.SetHeader(VormaBuildIDHeaderKey, buildID)

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

		// Use thread-safe getters
		isDev := v.getIsDev()
		clientEntryOut := v.getClientEntryOut()

		if !isDev {
			rootTemplateData["VormaBodyScripts"] = template.HTML(
				fmt.Sprintf(
					`<script type="module" src="%s%s"></script>`,
					v.Wave.GetPublicPathPrefix(), clientEntryOut,
				),
			)
		} else {
			opts := viteutil.ToDevScriptsOptions{ClientEntry: v.config.ClientEntry}
			if UIVariant(v.config.UIVariant) == UIVariants.React {
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

		// Use thread-safe getter for template
		rootTemplate := v.getRootTemplate()
		err = rootTemplate.Execute(&buf, rootTemplateData)
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

// IsCurrentBuildJSONRequest checks if the request is from a client with the current build ID.
// If true, is both (1) JSON and (2) guaranteed to be from a client
// that has knowledge of the latest build ID.
func (v *Vorma) IsCurrentBuildJSONRequest(r *http.Request) bool {
	return r.URL.Query().Get(VormaJSONQueryKey) == v.getBuildID()
}

// GetCurrentBuildID returns the current build ID of the Vorma instance.
func (v *Vorma) GetCurrentBuildID() string {
	return v.getBuildID()
}

func (v *Vorma) GetActionsHandler(router *mux.Router) mux.TasksCtxRequirerFunc {
	return mux.TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)
		res.SetHeader(VormaBuildIDHeaderKey, v.getBuildID())
		router.ServeHTTP(w, r)
	})
}
