package vormabuild

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/vorma2"
	"github.com/vormadev/vorma/vorma2/internal/constants"
	"github.com/vormadev/vorma/vorma2/internal/tsgen"
	"github.com/vormadev/vorma/wave/wavebuild"
)

var (
	query_methods = map[string]struct{}{
		http.MethodGet:  {},
		http.MethodHead: {},
	}
	mutation_methods = map[string]struct{}{
		http.MethodPost: {}, http.MethodPut: {},
		http.MethodPatch: {}, http.MethodDelete: {},
	}
)

/////////////////////////////////////////////////////////////////////
/////// Shared rendering types and functions
/////////////////////////////////////////////////////////////////////

type app_config_params struct {
	ActionsRouterMountRoot string
	ActionsDynamicRune     string
	ActionsSplatRune       string
	LoadersDynamicRune     string
	LoadersSplatRune       string
	LoadersExplicitIndex   string
	IncludePhantom         bool
}

type vite_config_params struct {
	RollupInput      []string
	PublicPathPrefix string
	FuncName         string
	IgnoredPatterns  []string
	DedupeList       []string
}

// render_app_config_const produces the `export const vormaAppConfig = { ... } as const;`
// block. When IncludePhantom is true, the __phantom field referencing VormaApp is included
// (requires VormaApp type to be defined earlier in the file).
func render_app_config_const(p app_config_params) string {
	var sb strings.Builder
	sb.WriteString("export const vormaAppConfig = {\n")
	fmt.Fprintf(
		&sb,
		"\tactionsRouterMountRoot: %q,\n",
		p.ActionsRouterMountRoot,
	)
	fmt.Fprintf(&sb, "\tactionsDynamicRune: %q,\n", p.ActionsDynamicRune)
	fmt.Fprintf(&sb, "\tactionsSplatRune: %q,\n", p.ActionsSplatRune)
	fmt.Fprintf(&sb, "\tloadersDynamicRune: %q,\n", p.LoadersDynamicRune)
	fmt.Fprintf(&sb, "\tloadersSplatRune: %q,\n", p.LoadersSplatRune)
	fmt.Fprintf(
		&sb,
		"\tloadersExplicitIndexSegmentIdentifier: %q,\n",
		p.LoadersExplicitIndex,
	)
	if p.IncludePhantom {
		sb.WriteString("\t__phantom: null as unknown as VormaApp,\n")
	}
	sb.WriteString("} as const;\n")
	return sb.String()
}

// render_vite_config_block produces the full Vorma Vite Config section:
// filemap re-export, global function declaration, publicPathPrefix,
// waveRuntimeURL, and the vormaViteConfig const.
func render_vite_config_block(p vite_config_params) string {
	var sb strings.Builder

	sb.WriteString("\n// Vorma Vite Config\n\n")
	sb.WriteString(`import { staticPublicAssetMap } from "./filemap";
export { staticPublicAssetMap };
export type StaticPublicAsset = keyof typeof staticPublicAssetMap;

`)
	fmt.Fprintf(
		&sb,
		"declare global {\n\tfunction %s(staticPublicAsset: StaticPublicAsset): string;\n}\n\n",
		p.FuncName,
	)
	fmt.Fprintf(&sb,
		"export const publicPathPrefix = %q;\n\n",
		p.PublicPathPrefix,
	)
	sb.WriteString(
		`export function waveRuntimeURL(originalPublicURL: StaticPublicAsset) {
	const hashedPublicURL = staticPublicAssetMap[originalPublicURL];
	if (!hashedPublicURL) {
		throw new Error("Wave public URL lookup miss: " + originalPublicURL);
	}
	return publicPathPrefix + hashedPublicURL;
}

`,
	)

	sb.WriteString("export const vormaViteConfig = {\n")
	sb.WriteString("\trollupInput: [\n")
	for _, e := range p.RollupInput {
		fmt.Fprintf(&sb, "\t\t%q,\n", e)
	}
	sb.WriteString("\t],\n")
	sb.WriteString("\tpublicPathPrefix,\n")
	fmt.Fprintf(&sb, "\tbuildtimePublicURLFuncName: %q,\n", p.FuncName)
	sb.WriteString("\timportMetaURL: import.meta.url,\n")

	sb.WriteString("\tignoredPatterns: [\n")
	for _, ip := range p.IgnoredPatterns {
		fmt.Fprintf(&sb, "\t\t%q,\n", ip)
	}
	sb.WriteString("\t],\n")

	sb.WriteString("\tdedupeList: [\n")
	for _, d := range p.DedupeList {
		fmt.Fprintf(&sb, "\t\t%q,\n", d)
	}
	sb.WriteString("\t],\n")
	sb.WriteString("} as const;\n")

	return sb.String()
}

func dedupe_for_variant(variant string) []string {
	switch variant {
	case "react":
		return []string{"react", "react-dom"}
	case "preact":
		return []string{
			"preact",
			"preact/hooks",
			"@preact/signals",
			"preact/jsx-runtime",
			"preact/compat",
			"preact/test-utils",
		}
	case "solid":
		return []string{"solid-js", "solid-js/web"}
	default:
		return nil
	}
}

func compute_ignored_patterns(wave_out_dir, gen_dir string) []string {
	return []string{
		"**/*.go",
		"**/" + wave_out_dir + "/**/*",
		"**/" + gen_dir + "/**/*",
	}
}

// extract_app_config_params reads the router config values from
// the app's build internals.
func extract_app_config_params(
	app *vorma2.Vorma,
	include_phantom bool,
) app_config_params {
	build := app.ForBuild()
	loaders := build.LoadersRouter()
	actions := build.ActionsRouter()
	return app_config_params{
		ActionsRouterMountRoot: actions.MountRoot(),
		ActionsDynamicRune:     string(actions.DynamicParamPrefix()),
		ActionsSplatRune:       string(actions.SplatSegmentIdentifier()),
		LoadersDynamicRune:     string(loaders.DynamicParamPrefix()),
		LoadersSplatRune:       string(loaders.SplatSegmentIdentifier()),
		LoadersExplicitIndex:   loaders.ExplicitIndexSegmentIdentifier(),
		IncludePhantom:         include_phantom,
	}
}

/////////////////////////////////////////////////////////////////////
/////// Full index.ts generation
/////////////////////////////////////////////////////////////////////

type tsgen_input struct {
	cfg             *parsed_vorma_config
	app             *vorma2.Vorma
	frontend_routes []discovered_route
	ctx             *wavebuild.PluginCtx
}

func generate_index_ts(input tsgen_input) ([]byte, error) {
	build := input.app.ForBuild()
	loaders_router := build.LoadersRouter()
	actions_router := build.ActionsRouter()

	var collection []tsgen.CollectionItem
	seen := map[string]struct{}{}

	// loaders from router
	all_loaders := loaders_router.AllRoutes()
	loader_patterns := make([]string, 0, len(all_loaders))
	for p := range all_loaders {
		loader_patterns = append(loader_patterns, p)
	}
	slices.Sort(loader_patterns)

	has_root_data := false
	for _, pattern := range loader_patterns {
		loader := all_loaders[pattern]
		item := tsgen.CollectionItem{
			ArbitraryProperties: map[string]any{
				"pattern": pattern,
				"_type":   "loader",
			},
		}
		set_pattern_metadata(
			item.ArbitraryProperties, pattern,
			loaders_router.DynamicParamPrefix(),
			loaders_router.SplatSegmentIdentifier(),
		)
		if loader != nil {
			item.PhantomTypes = map[string]tsgen.AdHocType{
				"phantomOutputType": {TypeInstance: loader.O()},
			}
		}
		if pattern == "/" && loaders_router.HasTaskHandler(pattern) {
			has_root_data = true
			item.ArbitraryProperties["isRootData"] = true
		}
		collection = append(collection, item)
		seen[pattern] = struct{}{}
	}

	// client-only routes
	for _, r := range input.frontend_routes {
		if _, ok := seen[r.pattern]; ok {
			continue
		}
		item := tsgen.CollectionItem{
			ArbitraryProperties: map[string]any{
				"pattern": r.pattern,
				"_type":   "loader",
			},
			PhantomTypes: map[string]tsgen.AdHocType{
				"phantomOutputType": {TypeInstance: mux.None{}},
			},
		}
		set_pattern_metadata(
			item.ArbitraryProperties, r.pattern,
			loaders_router.DynamicParamPrefix(),
			loaders_router.SplatSegmentIdentifier(),
		)
		collection = append(collection, item)
		seen[r.pattern] = struct{}{}
	}

	// actions
	all_actions := actions_router.AllRoutes()
	type action_sort_key struct {
		pattern string
		method  string
		index   int
	}
	action_keys := make([]action_sort_key, 0, len(all_actions))
	for i, a := range all_actions {
		action_keys = append(action_keys, action_sort_key{
			pattern: a.OriginalPattern(),
			method:  a.Method(),
			index:   i,
		})
	}
	slices.SortFunc(action_keys, func(a, b action_sort_key) int {
		if cmp := strings.Compare(a.pattern, b.pattern); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.method, b.method)
	})

	for _, ak := range action_keys {
		action := all_actions[ak.index]
		method := action.Method()
		category := ""
		is_mutation := false
		if _, ok := query_methods[method]; ok {
			category = "query"
		} else if _, ok := mutation_methods[method]; ok {
			category = "mutation"
			is_mutation = true
		} else {
			continue
		}

		item := tsgen.CollectionItem{
			ArbitraryProperties: map[string]any{
				"pattern": action.OriginalPattern(),
				"_type":   category,
			},
		}
		if is_mutation && method != http.MethodPost {
			item.ArbitraryProperties["method"] = method
		}
		set_pattern_metadata(
			item.ArbitraryProperties, action.OriginalPattern(),
			actions_router.DynamicParamPrefix(),
			actions_router.SplatSegmentIdentifier(),
		)
		if action != nil {
			item.PhantomTypes = map[string]tsgen.AdHocType{
				"phantomInputType":  {TypeInstance: action.I()},
				"phantomOutputType": {TypeInstance: action.O()},
			}
		}
		collection = append(collection, item)
	}

	extra_ts := build_extra_ts(input, has_root_data)

	ts_content, err := tsgen.GenerateTSContent(tsgen.Opts{
		Collection:        collection,
		CollectionVarName: "routes",
		AdHocTypes:        build.AdHocTypes(),
		ExtraTSCode:       extra_ts,
	})
	if err != nil {
		return nil, err
	}

	return []byte(ts_content), nil
}

func build_extra_ts(input tsgen_input, has_root_data bool) string {
	cfg := input.cfg

	var sb strings.Builder

	// root data type
	if has_root_data {
		sb.WriteString(`type VormaRootData = Extract<
	(typeof routes)[number],
	{ isRootData: true }
>["phantomOutputType"];`)
	} else {
		sb.WriteString("type VormaRootData = null;")
	}
	sb.WriteString("\n\n")

	// VormaApp type (must precede vormaAppConfig because __phantom references it)
	sb.WriteString(`export type VormaApp = {
	routes: typeof routes;
	appConfig: typeof vormaAppConfig;
	rootData: VormaRootData;
};

`)

	// vormaAppConfig const — uses shared renderer with real router values
	sb.WriteString(render_app_config_const(
		extract_app_config_params(input.app, true),
	))
	sb.WriteString("\n")

	// client type imports and exports
	fmt.Fprintf(&sb, `import type {
	VormaLoaderPattern,
	VormaMutationInput,
	VormaMutationOutput,
	VormaMutationPattern,
	VormaMutationProps,
	VormaQueryInput,
	VormaQueryOutput,
	VormaQueryPattern,
	VormaQueryProps,
} from "vorma/client";
import type { VormaRouteProps } from "vorma/%s";

export type QueryPattern = VormaQueryPattern<VormaApp>;
export type QueryProps<P extends QueryPattern> = VormaQueryProps<VormaApp, P>;
export type QueryInput<P extends QueryPattern> = VormaQueryInput<VormaApp, P>;
export type QueryOutput<P extends QueryPattern> = VormaQueryOutput<VormaApp, P>;

export type MutationPattern = VormaMutationPattern<VormaApp>;
export type MutationProps<P extends MutationPattern> = VormaMutationProps<VormaApp, P>;
export type MutationInput<P extends MutationPattern> = VormaMutationInput<VormaApp, P>;
export type MutationOutput<P extends MutationPattern> = VormaMutationOutput<VormaApp, P>;

export type RouteProps<P extends VormaLoaderPattern<VormaApp>> = VormaRouteProps<VormaApp, P>;

`, cfg.ui_variant)

	// user extra TS code
	build := input.app.ForBuild()
	if extra := build.ExtraTSCode(); extra != "" {
		sb.WriteString(extra)
		sb.WriteString("\n")
	}

	// vite config block — uses shared renderer
	entrypoints := []string{filepath.ToSlash(cfg.client_entry_path.Str())}
	for _, r := range input.frontend_routes {
		entrypoints = append(entrypoints, filepath.ToSlash(r.module_path.Str()))
	}

	wave_out_dir := filepath.ToSlash(input.ctx.WaveOutDir().Str())
	gen_dir := filepath.ToSlash(cfg.gen_out_dir.Str())

	sb.WriteString(render_vite_config_block(vite_config_params{
		RollupInput:      entrypoints,
		PublicPathPrefix: input.ctx.PublicPathPrefix(),
		FuncName:         cfg.buildtime_public_url_func_name,
		IgnoredPatterns:  compute_ignored_patterns(wave_out_dir, gen_dir),
		DedupeList:       dedupe_for_variant(cfg.ui_variant),
	}))

	return sb.String()
}

/////////////////////////////////////////////////////////////////////
/////// Pattern metadata helpers
/////////////////////////////////////////////////////////////////////

func set_pattern_metadata(
	props map[string]any,
	pattern string,
	dynamic_rune rune,
	splat_rune rune,
) {
	params := extract_dynamic_params(pattern, dynamic_rune)
	if len(params) > 0 {
		props["params"] = params
	}
	if strings.HasSuffix(pattern, "/"+string(splat_rune)) {
		props["isSplat"] = true
	}
}

func extract_dynamic_params(pattern string, dynamic_rune rune) []string {
	segments := matcher.ParseSegments(pattern)
	var params []string
	for _, seg := range segments {
		if len(seg) > 0 && seg[0] == byte(dynamic_rune) {
			params = append(params, seg[1:])
		}
	}
	return params
}

func write_index_ts(
	cfg *parsed_vorma_config,
	routes []discovered_route,
	app *vorma2.Vorma,
	ctx *wavebuild.PluginCtx,
) error {
	content, err := generate_index_ts(tsgen_input{
		cfg:             cfg,
		app:             app,
		frontend_routes: routes,
		ctx:             ctx,
	})
	if err != nil {
		return err
	}
	out_path := filepath.Join(
		cfg.gen_out_dir.Str(), constants.GENERATED_TS_INDEX_FILENAME,
	)
	return os.WriteFile(out_path, content, 0644)
}
