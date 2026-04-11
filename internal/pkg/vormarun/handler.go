package vormarun

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"

	"github.com/vormadev/vorma/internal/pkg/viteutil"
	"github.com/vormadev/vorma/kit/envutil"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/jsonutil"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/set"
)

/////////////////////////////////////////////////////////////////////
/////// Loaders handler
/////////////////////////////////////////////////////////////////////

func (v *Vorma) loaders_handler() mux.TasksCtxRequirerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)

		expected_client_build_id, err := v.ClientBuildID()
		if err != nil {
			v.log.Error("error getting client build id", "err", err)
			res.InternalServerError()
			return
		}

		submitted_client_build_id := r.URL.Query().Get(Query_Key_Vorma_JSON)
		is_json := submitted_client_build_id != ""

		res.SetHeader(X_Vorma_Client_Build_Id, expected_client_build_id)

		if is_json &&
			submitted_client_build_id != expected_client_build_id &&
			// lets us get JSON responses manually by setting `vorma_json=_` in the query
			submitted_client_build_id != "_" {
			reload_url := *r.URL
			q := reload_url.Query()
			q.Del(Query_Key_Vorma_JSON)
			reload_url.RawQuery = q.Encode()
			res.SetHeader(X_Vorma_Reload, reload_url.String())
			res.SetHeader("Cache-Control", "no-store")
			res.OK()
			return
		}

		var root_template_data map[string]any
		var root_template_data_err error
		var root_template_data_wg sync.WaitGroup

		if !is_json {
			if v.HTMLConfig.TemplateData != nil {
				root_template_data_wg.Go(func() {
					root_template_data, root_template_data_err = v.HTMLConfig.TemplateData(r)
				})
			} else {
				root_template_data = make(map[string]any)
			}
		}

		var default_head_els []*htmlutil.Element
		var default_head_err error
		var default_head_wg sync.WaitGroup

		if v.HTMLConfig.DefaultHead != nil {
			default_head_wg.Go(func() {
				h := headels.New()
				default_head_err = v.HTMLConfig.DefaultHead(r, v, h)
				if default_head_err == nil {
					default_head_els = h.Collect()
				}
			})
		}

		match_results, found := mux.FindNestedMatches(v.loaders_mux, r)
		if !found {
			res.NotFound()
			return
		}

		tasks_results := mux.RunNestedTasks(v.loaders_mux, r, match_results)
		if tasks_results == nil {
			res.InternalServerError()
			return
		}

		default_head_wg.Wait()
		if default_head_err != nil {
			v.log.Error("error in DefaultHeadEls func", "err", default_head_err)
			res.InternalServerError()
			return
		}

		manifest, err := v.manifest()
		if err != nil {
			v.log.Error("error getting manifest", "err", err)
			res.InternalServerError()
			return
		}

		matches := match_results.Matches
		matched_patterns := make([]string, 0, len(matches))
		for _, m := range matches {
			matched_patterns = append(matched_patterns, m.OriginalPattern())
		}

		import_urls := make([]string, 0, len(matches))
		loaders_data := make([]any, 0, len(matches))

		deps := set.New[string]()
		css_bundles := set.New[string]()

		outermost_server_err := ""
		outermost_server_err_idx := -1

		for i, m := range matches {
			// Apply even if loader errored, up to and including
			// (but not after) the erroring loader.
			pattern := m.OriginalPattern()
			route_mod, ok := manifest.ClientRoutes[pattern]
			if !ok {
				v.log.Error("no route module found for matched pattern",
					"pattern", pattern,
				)
				res.InternalServerError()
				return
			}
			import_urls = append(import_urls, route_mod.URL)
			for _, dep := range route_mod.DepURLs {
				deps.Add(dep)
			}
			for _, css := range route_mod.CSSBundleURLs {
				css_bundles.Add(css)
			}

			// Apply only if no loader error
			result := tasks_results.Results[i]
			loader_err := result.Err()
			if loader_err != nil {
				outermost_server_err_idx = i
				if typed_loader_err, ok := errors.AsType[*LoaderError](loader_err); ok {
					outermost_server_err = typed_loader_err.ClientMsg
				} else {
					outermost_server_err = "An unexpected error occurred."
				}
				v.log.Error("loader error",
					"pattern", pattern,
					"path", r.URL.Path,
					"err", loader_err,
				)
				break
			} else if result.RanTask() {
				data := result.Data()
				loaders_data = append(loaders_data, data)
				is_nil := reflectutil.
					ExcludingNoneGetIsNilOrUltimatelyPointsToNil(data)
				if is_nil {
					v.log.Warn(
						"Do not return nil values from loaders unless "+
							"the referenced type is an empty struct "+
							"or you are returning an error.",
						"pattern", pattern,
					)
				}
			}
		}

		merged_proxy := response.MergeProxyResponses(tasks_results.ResponseProxies...)
		raw_head_els := append(default_head_els, merged_proxy.HeadEls().Collect()...)

		if !IsDev() && !is_json {
			for dep := range deps.Range() {
				raw_head_els = append(raw_head_els, &htmlutil.Element{
					Tag: "link",
					AttributesKnownSafe: map[string]string{
						"rel":  "modulepreload",
						"href": dep,
					},
					SelfClosing: true,
				})
			}
			for css := range css_bundles.Range() {
				raw_head_els = append(raw_head_els, &htmlutil.Element{
					Tag: "link",
					AttributesKnownSafe: map[string]string{
						"rel":  "stylesheet",
						"href": css,
					},
					SelfClosing: true,
				})
			}
		}

		sorted_head_els := v.headels_instance.ToSortedAndPreEscapedHeadEls(raw_head_els)

		payload := loader_payload{
			MatchedPatterns: matched_patterns,
			Params:          match_results.Params,
			SplatValues:     match_results.SplatValues,

			Title:       sorted_head_els.Title,
			MetaHeadEls: sorted_head_els.Meta,
			RestHeadEls: sorted_head_els.Rest,

			OutermostServerErr: outermost_server_err,
			OutermostServerErrIdx: func() *int {
				if outermost_server_err_idx != -1 {
					return &outermost_server_err_idx
				}
				return nil
			}(),

			ImportURLs: import_urls,
			Deps:       deps.Slice(),
			CSSBundles: css_bundles.Slice(),

			LoadersData: loaders_data,
		}

		merged_proxy.ApplyToResponseWriter(w, r)
		if merged_proxy.IsError() || merged_proxy.IsRedirect() {
			return
		}

		// If no cache-control set, set a conservative default
		// to avoid overzealous browser heuristics.
		if w.Header().Get("Cache-Control") == "" {
			res.SetHeader("Cache-Control", "private, max-age=0, must-revalidate, no-cache")
		}

		if is_json {
			res.JSON(payload)
			return
		}

		root_template_data_wg.Wait()
		if root_template_data_err != nil {
			v.log.Error("error in RootHTMLTemplateData func", "err", root_template_data_err)
			res.InternalServerError()
			return
		}

		vorma_head := &strings.Builder{}

		user_head, err := v.headels_instance.Render(sorted_head_els)
		if err != nil {
			v.log.Error("error rendering head elements", "err", err)
			res.InternalServerError()
			return
		}
		vorma_head.WriteString(string(user_head))
		vorma_head.WriteString("\n")

		critical_css_el, error := v.critical_css_el()
		if error != nil {
			v.log.Error("error generating critical CSS element", "err", error)
			res.InternalServerError()
			return
		}
		rendered_critical_css_el, err := htmlutil.RenderElement(critical_css_el)
		if err != nil {
			v.log.Error("error rendering critical CSS element", "err", err)
			res.InternalServerError()
			return
		}
		vorma_head.WriteString(string(rendered_critical_css_el))
		vorma_head.WriteString("\n")

		main_css_el, error := v.main_css_el()
		if error != nil {
			v.log.Error("error generating main CSS element", "err", error)
			res.InternalServerError()
			return
		}
		rendered_main_css_el, err := htmlutil.RenderElement(main_css_el)
		if err != nil {
			v.log.Error("error rendering main CSS element", "err", err)
			res.InternalServerError()
			return
		}
		vorma_head.WriteString(string(rendered_main_css_el))

		if root_template_data == nil {
			root_template_data = make(map[string]any)
		}
		root_template_data["VormaHead"] = template.HTML(vorma_head.String())

		vorma_body := &strings.Builder{}
		ssr_payload := ssr_payload{
			ClientBuildID:  expected_client_build_id,
			IsDev:          IsDev(),
			loader_payload: payload,
		}
		if envutil.GetBool("VERCEL_SKEW_PROTECTION_ENABLED", false) {
			ssr_payload.DeploymentID = envutil.GetStr("VERCEL_DEPLOYMENT_ID", "")
		}
		payload_json, err := jsonutil.Serialize(ssr_payload)
		if err != nil {
			v.log.Error("error serializing loader payload to JSON", "err", err)
			res.InternalServerError()
			return
		}
		data_json_el := &htmlutil.Element{
			Tag: "script",
			AttributesKnownSafe: map[string]string{
				"type": "application/json",
				"id":   Vorma_Data_JSON_Script_El_ID,
			},
			DangerousInnerHTML: string(payload_json),
		}
		rendered_data_json_el, err := htmlutil.RenderElement(data_json_el)
		if err != nil {
			v.log.Error("error rendering data JSON element", "err", err)
			res.InternalServerError()
			return
		}
		vorma_body.WriteString(string(rendered_data_json_el))
		vorma_body.WriteString("\n")
		fmt.Fprintf(vorma_body, `<div id="%s"></div>`, Vorma_Root_El_ID)
		vorma_body.WriteString("\n")
		if !IsDev() {
			fmt.Fprintf(
				vorma_body,
				`<script type="module" src="%s"></script>`,
				manifest.ClientEntry.URL,
			)
		} else {
			dev_scripts, err := viteutil.ToDevScripts(
				manifest.Dev_ViteServerPort,
				manifest.ClientEntry.URL,
				manifest.UIVariant == "react",
			)
			if err != nil {
				v.log.Error("error generating dev scripts for client entry module", "err", err)
				res.InternalServerError()
				return
			}
			vorma_body.WriteString(string(dev_scripts))
			vorma_body.WriteString("\n")
			refresh_script_inner_html, err := v.refresh_script_inner_html()
			if err != nil {
				v.log.Error("error generating refresh script inner HTML", "err", err)
				res.InternalServerError()
				return
			}
			fmt.Fprintf(vorma_body, "<script>%s</script>", refresh_script_inner_html)
		}

		root_template_data["VormaBody"] = template.HTML(vorma_body.String())

		var buf bytes.Buffer
		if err := v.parsed_tmpl.Execute(&buf, root_template_data); err != nil {
			v.log.Error("error executing template", "err", err)
			res.InternalServerError()
			return
		}

		res.HTMLBytes(buf.Bytes())
	}
}

/////////////////////////////////////////////////////////////////////
/////// Actions handler
/////////////////////////////////////////////////////////////////////

func (v *Vorma) actions_handler() mux.TasksCtxRequirerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := response.New(w)

		if !v.supported_methods.Has(r.Method) {
			res.SetHeader("Allow", v.supported_methods_allow_val)
			res.MethodNotAllowed()
			return
		}
		client_build_id, err := v.ClientBuildID()
		if err != nil {
			v.log.Error("error getting client build id", "err", err)
			res.InternalServerError()
			return
		}
		res.SetHeader(X_Vorma_Client_Build_Id, client_build_id)
		v.actions_mux.ServeHTTP(w, r)
	}
}
