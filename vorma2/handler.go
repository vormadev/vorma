package vorma2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/htmlutil"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/reflectutil"
	"github.com/vormadev/vorma/kit/response"
	"github.com/vormadev/vorma/kit/set"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vorma2/internal/constants"
	"github.com/vormadev/vorma/vorma2/internal/types"
	"github.com/vormadev/vorma/wave"
)

const (
	json_query_key  = "vorma_json"
	root_element_id = "vorma-root"

	tmpl_key_head         = "VormaHeadEls"
	tmpl_key_ssr_script   = "VormaSSRScript"
	tmpl_key_ssr_hash     = "VormaSSRScriptSha256Hash"
	tmpl_key_body_scripts = "VormaBodyScripts"
	tmpl_key_root_id      = "VormaRootID"
)

/////////////////////////////////////////////////////////////////////
/////// JSON response shape
/////////////////////////////////////////////////////////////////////

type route_data_json struct {
	OutermostServerError    string              `json:"outermostServerError,omitempty"`
	OutermostServerErrorIdx *int                `json:"outermostServerErrorIdx,omitempty"`
	ErrorExportKeys         []string            `json:"errorExportKeys,omitempty"`
	MatchedPatterns         []string            `json:"matchedPatterns,omitempty"`
	LoadersData             []any               `json:"loadersData,omitempty"`
	ImportURLs              []string            `json:"importURLs,omitempty"`
	ExportKeys              []string            `json:"exportKeys,omitempty"`
	HasRootData             bool                `json:"hasRootData,omitempty"`
	Params                  mux.Params          `json:"params,omitempty"`
	SplatValues             []string            `json:"splatValues,omitempty"`
	Deps                    []string            `json:"deps,omitempty"`
	Title                   *htmlutil.Element   `json:"title,omitempty"`
	MetaHeadEls             []*htmlutil.Element `json:"metaHeadEls,omitempty"`
	RestHeadEls             []*htmlutil.Element `json:"restHeadEls,omitempty"`
	CSSBundles              []string            `json:"cssBundles,omitempty"`
}

/////////////////////////////////////////////////////////////////////
/////// Loader errors
/////////////////////////////////////////////////////////////////////

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

func (v *Vorma) resolve_client_error(err error, pattern string) string {
	var loader_err *LoaderError
	if errors.As(err, &loader_err) {
		if loader_err.Client != "" {
			if loader_err.Server != nil {
				v.logger.Error(
					"loader error",
					"pattern",
					pattern,
					"error",
					loader_err.Server,
				)
			}
			return loader_err.Client
		}
		v.logger.Error("LoaderError with empty Client", "pattern", pattern)
	} else {
		v.logger.Error("loader error", "pattern", pattern, "error", err)
	}
	return "An error occurred"
}

/////////////////////////////////////////////////////////////////////
/////// Route data cache
/////////////////////////////////////////////////////////////////////

type route_metadata_cache struct {
	matched_patterns  []string
	import_urls       []string
	export_keys       []string
	error_export_keys []string
	deps              []string
	css_bundles       []string
}

func static_head_cache_key(deps, css []string) string {
	var sb strings.Builder
	for _, d := range deps {
		sb.WriteString(d)
		sb.WriteByte(0)
	}
	sb.WriteByte('|')
	for _, c := range css {
		sb.WriteString(c)
		sb.WriteByte(0)
	}
	return sb.String()
}

func route_data_cache_key(
	build_id string,
	matches []*matcher.NestedMatch,
) string {
	var sb strings.Builder
	sb.Grow(len(build_id) + len(matches)*16 + 2)
	sb.WriteString(build_id)
	sb.WriteByte('|')
	for _, m := range matches {
		sb.WriteString(m.NormalizedPattern())
		sb.WriteByte(';')
	}
	return sb.String()
}

func (v *Vorma) get_or_build_route_metadata_cache(
	snapshot *types.RuntimeSnapshot,
	matches []*matcher.NestedMatch,
) *route_metadata_cache {
	// In dev, prune stale entries when the build ID changes
	if wave.IsDev() {
		last_bid, _ := v.route_data_cache_bid.Load().(string)
		if last_bid != snapshot.BuildID {
			v.route_data_cache.Range(func(key, _ any) bool {
				v.route_data_cache.Delete(key)
				return true
			})
			v.static_prod_head_cache.Range(func(key, _ any) bool {
				v.static_prod_head_cache.Delete(key)
				return true
			})
			v.route_data_cache_bid.Store(snapshot.BuildID)
		}
	}

	key := route_data_cache_key(snapshot.BuildID, matches)
	if val, ok := v.route_data_cache.Load(key); ok {
		return val.(*route_metadata_cache)
	}
	meta := build_route_metadata_cache(snapshot, matches)
	actual, _ := v.route_data_cache.LoadOrStore(key, meta)
	return actual.(*route_metadata_cache)
}

func build_route_metadata_cache(
	snapshot *types.RuntimeSnapshot,
	matches []*matcher.NestedMatch,
) *route_metadata_cache {
	n := len(matches)
	meta := &route_metadata_cache{
		matched_patterns:  make([]string, n),
		import_urls:       make([]string, n),
		export_keys:       make([]string, n),
		error_export_keys: make([]string, n),
	}

	seen_deps := &set.Set[string]{}
	var all_deps []string

	for _, d := range snapshot.ClientEntryDeps {
		ds := d.Str()
		if !seen_deps.Has(ds) {
			seen_deps.Add(ds)
			all_deps = append(all_deps, ds)
		}
	}

	for i, match := range matches {
		pattern := types.RoutePattern(match.OriginalPattern())
		meta.matched_patterns[i] = match.OriginalPattern()

		if rp := snapshot.Paths[pattern]; rp != nil {
			meta.import_urls[i] = rp.ImportPath.Str()
			meta.export_keys[i] = rp.ExportKey
			meta.error_export_keys[i] = rp.ErrorExportKey

			for _, d := range rp.Deps {
				ds := d.Str()
				if !seen_deps.Has(ds) {
					seen_deps.Add(ds)
					all_deps = append(all_deps, ds)
				}
			}
		}
	}

	meta.deps = all_deps

	// css bundles
	css_seen := &set.Set[string]{}
	var css_bundles []string
	add_css := func(paths []types.SitePublicPath) {
		for _, p := range paths {
			s := p.Str()
			if !css_seen.Has(s) {
				css_seen.Add(s)
				css_bundles = append(css_bundles, s)
			}
		}
	}
	add_css(snapshot.DepToCSSBundleMap[snapshot.ClientEntryPath])
	for _, dep := range all_deps {
		add_css(snapshot.DepToCSSBundleMap[types.SitePublicPath(dep)])
	}
	meta.css_bundles = css_bundles

	return meta
}

/////////////////////////////////////////////////////////////////////
/////// LoadersHandler
/////////////////////////////////////////////////////////////////////

func (v *Vorma) LoadersHandler() mux.TasksCtxRequirerFunc {
	v.loaders_handler_once.Do(func() {
		v.ensure_patterns_registered()
		v.loaders_handler = mux.TasksCtxRequirerFunc(v.serve_loaders)
	})
	return v.loaders_handler
}

func (v *Vorma) ensure_patterns_registered() {
	snapshot, err := v.runtime_snapshot.Get()
	if err != nil {
		v.logger.Error(
			"failed to load snapshot for pattern registration",
			"error",
			err,
		)
		return
	}
	for pattern := range snapshot.Paths {
		v.loaders_router.AddPatternWithoutHandlerIfMissing(pattern.Str())
	}
}

func (v *Vorma) serve_loaders(w http.ResponseWriter, r *http.Request) {
	res := response.New(w)

	snapshot, err := v.runtime_snapshot.Get()
	if err != nil {
		v.logger.Error("failed to load runtime snapshot", "error", err)
		res.InternalServerError()
		return
	}

	v.maybe_reregister_patterns(snapshot)

	requested_build_id := r.URL.Query().Get(json_query_key)
	is_json := requested_build_id != ""

	w.Header().Set(BuildIDHeaderKey, snapshot.BuildID)

	// stale build check — tell the client to reload without the JSON param
	if is_json && requested_build_id != snapshot.BuildID {
		reload_url := *r.URL
		q := reload_url.Query()
		q.Del(json_query_key)
		reload_url.RawQuery = q.Encode()
		w.Header().Set("X-Vorma-Reload", reload_url.String())
		w.Header().
			Set("Cache-Control", "private, max-age=0, must-revalidate, no-cache")
		res.OK()
		return
	}

	// start default head elements in parallel with loader execution
	var default_head_els []*htmlutil.Element
	var default_head_err error
	var head_wg sync.WaitGroup
	var cancel_head context.CancelFunc

	if v.get_default_head_els != nil && !is_json {
		head_ctx, cancel := context.WithCancel(r.Context())
		cancel_head = cancel
		head_req := r.Clone(head_ctx)
		head_wg.Add(1)
		go func() {
			defer head_wg.Done()
			head := headels.New()
			default_head_err = v.get_default_head_els(head_req, v, head)
			if default_head_err == nil {
				default_head_els = head.Collect()
			}
		}()
	}

	defer func() {
		if cancel_head != nil {
			cancel_head()
		}
	}()

	// match routes
	match_results, found := mux.FindNestedMatches(v.loaders_router, r)
	if !found {
		res.NotFound()
		return
	}

	// run loader tasks
	tasks_results := mux.RunNestedTasks(
		v.loaders_router, r, match_results,
	)
	if tasks_results == nil {
		v.logger.Error(
			"missing TasksCtx — use mux.Router or mux.InjectTasksCtxMiddleware",
		)
		res.InternalServerError()
		return
	}

	// merge response proxies
	merged_proxy := response.MergeProxyResponses(
		tasks_results.ResponseProxies...)
	if merged_proxy != nil {
		merged_proxy.ApplyToResponseWriter(w, r)
		if merged_proxy.IsError() || merged_proxy.IsRedirect() {
			return
		}
	}

	// build route data
	rd := v.build_route_data(snapshot, match_results, tasks_results)

	// Set a conservative default cache control header only if the
	// loaders didn't already set one via response proxies.
	if w.Header().Get("Cache-Control") == "" {
		w.Header().
			Set("Cache-Control", "private, max-age=0, must-revalidate, no-cache")
	}

	if is_json {
		v.serve_loaders_json(res, rd)
		return
	}

	// wait for head elements
	head_wg.Wait()
	if default_head_err != nil {
		v.logger.Error("default head elements error", "error", default_head_err)
		res.InternalServerError()
		return
	}

	v.serve_loaders_html(r, res, rd, snapshot, default_head_els)
}

/////////////////////////////////////////////////////////////////////
/////// Route data assembly
/////////////////////////////////////////////////////////////////////

type route_data struct {
	metadata            *route_metadata_cache
	outermost_error     string
	outermost_error_idx *int
	matched_patterns    []string
	loaders_data        []any
	import_urls         []string
	export_keys         []string
	error_export_keys   []string
	has_root_data       bool
	params              mux.Params
	splat_values        []string
	deps                []string
	css_bundles         []string
	loader_head_els     []*htmlutil.Element
}

func (v *Vorma) build_route_data(
	snapshot *types.RuntimeSnapshot,
	match_results *matcher.FindNestedMatchesResults,
	tasks_results *mux.NestedTasksResults,
) *route_data {
	matches := match_results.Matches
	metadata := v.get_or_build_route_metadata_cache(snapshot, matches)

	rd := &route_data{
		metadata:          metadata,
		matched_patterns:  metadata.matched_patterns,
		import_urls:       metadata.import_urls,
		export_keys:       metadata.export_keys,
		error_export_keys: metadata.error_export_keys,
		deps:              metadata.deps,
		css_bundles:       metadata.css_bundles,
		loaders_data:      make([]any, len(matches)),
		params:            match_results.Params,
		splat_values:      match_results.SplatValues,
	}

	if len(matches) > 0 && matches[0].NormalizedPattern() == "" &&
		tasks_results.HasTaskHandlerAt(0) {
		rd.has_root_data = true
	}

	for i := range matches {
		result := tasks_results.Slice[i]
		rd.loaders_data[i] = result.Data()

		if result.Err() != nil && rd.outermost_error_idx == nil {
			idx := i
			rd.outermost_error_idx = &idx
			rd.outermost_error = v.resolve_client_error(
				result.Err(), rd.matched_patterns[i],
			)
		} else if result.RanTask() && result.Err() == nil {
			if reflectutil.ExcludingNoneGetIsNilOrUltimatelyPointsToNil(rd.loaders_data[i]) {
				v.logger.Warn(
					"Do not return nil values from loaders unless the underlying type is an empty struct or you are returning an error.",
					"pattern", rd.matched_patterns[i],
				)
			}
		}
	}

	if rd.outermost_error_idx != nil {
		cut := *rd.outermost_error_idx + 1
		rd.matched_patterns = rd.matched_patterns[:cut]
		rd.loaders_data = rd.loaders_data[:cut]
		rd.import_urls = rd.import_urls[:cut]
		rd.export_keys = rd.export_keys[:cut]
		rd.error_export_keys = rd.error_export_keys[:cut]
	}

	// collect head elements from loaders before error boundary
	head_cut := len(matches)
	if rd.outermost_error_idx != nil {
		head_cut = *rd.outermost_error_idx
	}
	for i := 0; i < head_cut && i < len(tasks_results.ResponseProxies); i++ {
		proxy := tasks_results.ResponseProxies[i]
		if proxy == nil {
			continue
		}
		els := proxy.HeadEls()
		if els != nil {
			rd.loader_head_els = append(rd.loader_head_els, els.Collect()...)
		}
	}

	return rd
}

/////////////////////////////////////////////////////////////////////
/////// JSON response
/////////////////////////////////////////////////////////////////////

func (v *Vorma) serve_loaders_json(
	res response.Response,
	rd *route_data,
) {
	sorted := v.head_els_inst.ToSortedAndPreEscapedHeadEls(rd.loader_head_els)

	params := rd.params
	if params == nil {
		params = mux.Params{}
	}
	splat := rd.splat_values
	if splat == nil {
		splat = []string{}
	}

	payload := &route_data_json{
		OutermostServerError:    rd.outermost_error,
		OutermostServerErrorIdx: rd.outermost_error_idx,
		ErrorExportKeys:         rd.error_export_keys,
		MatchedPatterns:         rd.matched_patterns,
		LoadersData:             rd.loaders_data,
		ImportURLs:              rd.import_urls,
		ExportKeys:              rd.export_keys,
		HasRootData:             rd.has_root_data,
		Params:                  params,
		SplatValues:             splat,
		Deps:                    rd.deps,
		Title:                   sorted.Title,
		MetaHeadEls:             sorted.Meta,
		RestHeadEls:             sorted.Rest,
		CSSBundles:              rd.css_bundles,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		v.logger.Error("failed to marshal JSON response", "error", err)
		res.InternalServerError()
		return
	}
	res.JSONBytes(bytes)
}

/////////////////////////////////////////////////////////////////////
/////// HTML response
/////////////////////////////////////////////////////////////////////

func (v *Vorma) serve_loaders_html(
	r *http.Request,
	res response.Response,
	rd *route_data,
	snapshot *types.RuntimeSnapshot,
	default_head_els []*htmlutil.Element,
) {
	tmpl, err := v.root_template.Get()
	if err != nil {
		v.logger.Error("failed to load root template", "error", err)
		res.InternalServerError()
		return
	}

	is_dev := wave.IsDev()

	// head elements — use static prod cache when possible
	var rendered_head template.HTML
	if !is_dev && len(default_head_els) == 0 && len(rd.loader_head_els) == 0 {
		cache_key := static_head_cache_key(rd.deps, rd.css_bundles)
		if cached, ok := v.static_prod_head_cache.Load(cache_key); ok {
			rendered_head = cached.(template.HTML)
		} else {
			var elements []*htmlutil.Element
			for _, dep := range rd.deps {
				elements = append(elements, &htmlutil.Element{
					Tag: "link",
					AttributesKnownSafe: map[string]string{
						"rel":  "modulepreload",
						"href": dep,
					},
					SelfClosing: true,
				})
			}
			for _, css := range rd.css_bundles {
				elements = append(elements, &htmlutil.Element{
					Tag: "link",
					AttributesKnownSafe: map[string]string{
						"rel":  "stylesheet",
						"href": css,
					},
					Attributes: map[string]string{
						"data-vorma-css-bundle": css,
					},
					SelfClosing: true,
				})
			}
			sorted := v.head_els_inst.ToSortedAndPreEscapedHeadEls(elements)
			rendered, render_err := v.head_els_inst.Render(sorted)
			if render_err != nil {
				v.logger.Error("failed to render static head", "error", render_err)
				res.InternalServerError()
				return
			}
			rendered_head = rendered + "\n" + template.HTML(v.Wave.MustCSSEls())
			v.static_prod_head_cache.Store(cache_key, rendered_head)
		}
	} else {
		var head_elements []*htmlutil.Element
		head_elements = append(head_elements, default_head_els...)
		head_elements = append(head_elements, rd.loader_head_els...)

		if !is_dev {
			for _, dep := range rd.deps {
				head_elements = append(head_elements, &htmlutil.Element{
					Tag: "link",
					AttributesKnownSafe: map[string]string{
						"rel":  "modulepreload",
						"href": dep,
					},
					SelfClosing: true,
				})
			}
			for _, css := range rd.css_bundles {
				head_elements = append(head_elements, &htmlutil.Element{
					Tag: "link",
					AttributesKnownSafe: map[string]string{
						"rel":  "stylesheet",
						"href": css,
					},
					Attributes: map[string]string{
						"data-vorma-css-bundle": css,
					},
					SelfClosing: true,
				})
			}
		}

		sorted_head := v.head_els_inst.ToSortedAndPreEscapedHeadEls(
			head_elements,
		)
		rendered_head, err = v.head_els_inst.Render(sorted_head)
		if err != nil {
			v.logger.Error("failed to render head elements", "error", err)
			res.InternalServerError()
			return
		}
		rendered_head += "\n" + template.HTML(v.Wave.MustCSSEls())
	}

	// SSR script
	route_manifest_url, err := v.Wave.PublicURL(
		constants.PUBLIC_ROUTE_MANIFEST_FILENAME,
	)
	if err != nil {
		v.logger.Error("route manifest not in public filemap", "error", err)
		res.InternalServerError()
		return
	}

	loaders_json, err := json.Marshal(rd.loaders_data)
	if err != nil {
		v.logger.Error("failed to marshal loaders data", "error", err)
		res.InternalServerError()
		return
	}

	ssr, err := build_ssr_script(ssr_input{
		VormaSymbol:             vorma_symbol_str,
		IsDev:                   is_dev,
		PublicPathPrefix:        v.Wave.MustPublicPathPrefix(),
		RouteManifestURL:        route_manifest_url,
		BuildID:                 snapshot.BuildID,
		RootElementID:           root_element_id,
		OutermostServerError:    rd.outermost_error,
		OutermostServerErrorIdx: rd.outermost_error_idx,
		ErrorExportKeys:         rd.error_export_keys,
		MatchedPatterns:         rd.matched_patterns,
		LoadersDataJSON:         template.JS(loaders_json),
		ImportURLs:              rd.import_urls,
		ExportKeys:              rd.export_keys,
		HasRootData:             rd.has_root_data,
		Params:                  rd.params,
		SplatValues:             rd.splat_values,
	})
	if err != nil {
		v.logger.Error("failed to build SSR script", "error", err)
		res.InternalServerError()
		return
	}

	// body scripts
	var body_scripts template.HTML
	if is_dev {
		client_path := strings.TrimPrefix(snapshot.ClientEntryPath.Str(), "/")
		dev_opts := viteutil.ToDevScriptsOptions{
			ClientEntry: client_path,
			Port:        v.Wave.DevVitePort(),
		}
		if snapshot.UIVariant == "react" {
			dev_opts.Variant = viteutil.VariantReact
		} else {
			dev_opts.Variant = viteutil.VariantOther
		}
		dev_scripts, err := viteutil.ToDevScripts(dev_opts)
		if err != nil {
			v.logger.Error("failed to build dev scripts", "error", err)
			res.InternalServerError()
			return
		}
		body_scripts = dev_scripts + "\n" + v.Wave.DevRefreshScriptEl()
	} else {
		body_scripts = template.HTML(fmt.Sprintf(
			`<script type="module" src="%s"></script>`,
			snapshot.ClientEntryPath.Str(),
		))
	}

	// assemble template data
	tmpl_data := map[string]any{}
	if v.get_root_tmpl_data != nil {
		user_data, err := v.get_root_tmpl_data(r)
		if err != nil {
			v.logger.Error("root template data error", "error", err)
			res.InternalServerError()
			return
		}
		for k, val := range user_data {
			tmpl_data[k] = val
		}
	}

	tmpl_data[tmpl_key_head] = rendered_head
	tmpl_data[tmpl_key_ssr_script] = ssr.script_html
	tmpl_data[tmpl_key_ssr_hash] = ssr.csp_hash
	tmpl_data[tmpl_key_body_scripts] = body_scripts
	tmpl_data[tmpl_key_root_id] = root_element_id

	var buf strings.Builder
	if err := tmpl.Execute(&buf, tmpl_data); err != nil {
		v.logger.Error("template execution error", "error", err)
		res.InternalServerError()
		return
	}

	res.HTMLBytes([]byte(buf.String()))
}

/////////////////////////////////////////////////////////////////////
/////// ActionsHandler
/////////////////////////////////////////////////////////////////////

func (v *Vorma) ActionsHandler() mux.TasksCtxRequirerFunc {
	v.actions_handler_once.Do(func() {
		v.actions_handler = mux.TasksCtxRequirerFunc(v.serve_actions)
	})
	return v.actions_handler
}

func (v *Vorma) serve_actions(w http.ResponseWriter, r *http.Request) {
	snapshot, err := v.runtime_snapshot.Get()
	if err != nil {
		v.logger.Error("failed to load runtime snapshot", "error", err)
		res := response.New(w)
		res.InternalServerError()
		return
	}
	w.Header().Set(BuildIDHeaderKey, snapshot.BuildID)
	v.actions_router.ServeHTTP(w, r)
}
