package vorma

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/lab/tsgen"
)

type AdHocType = tsgen.AdHocType

type tsGenOptions struct {
	LoadersRouter *mux.NestedRouter
	ActionsRouter *mux.Router
	AdHocTypes    []*AdHocType
	ExtraTSCode   string
}

var base = tsgen.BaseOptions{
	CollectionVarName:    "routes",
	DiscriminatorStr:     "pattern",
	CategoryPropertyName: "_type",
}

var queryMethods = map[string]struct{}{
	http.MethodGet: {}, http.MethodHead: {},
}
var mutationMethods = map[string]struct{}{
	http.MethodPost: {}, http.MethodPut: {}, http.MethodPatch: {}, http.MethodDelete: {},
}

func (v *Vorma) generateTypeScript(opts *tsGenOptions) (string, error) {
	var collection []tsgen.CollectionItem

	allLoaders := opts.LoadersRouter.AllRoutes()
	allActions := opts.ActionsRouter.AllRoutes()

	loadersDynamicRune := opts.LoadersRouter.GetDynamicParamPrefixRune()
	loadersSplatRune := opts.LoadersRouter.GetSplatSegmentRune()
	actionsDynamicRune := opts.ActionsRouter.GetDynamicParamPrefixRune()
	actionsSplatRune := opts.ActionsRouter.GetSplatSegmentRune()

	expectedRootDataPattern := ""
	if opts.LoadersRouter.GetExplicitIndexSegment() != "" {
		expectedRootDataPattern = "/"
	}

	var foundRootData bool

	var seen = map[string]struct{}{}

	// Sort loader patterns for deterministic output.
	// Map iteration order in Go is random, which would cause the generated
	// TypeScript to differ between runs even with identical input.
	loaderPatterns := make([]string, 0, len(allLoaders))
	for pattern := range allLoaders {
		loaderPatterns = append(loaderPatterns, pattern)
	}
	slices.Sort(loaderPatterns)

	for _, pattern := range loaderPatterns {
		loader := allLoaders[pattern]
		item := tsgen.CollectionItem{
			ArbitraryProperties: map[string]any{
				base.DiscriminatorStr:     pattern,
				base.CategoryPropertyName: "loader",
			},
		}
		params := extractDynamicParamsFromPattern(pattern, loadersDynamicRune)
		if len(params) > 0 {
			item.ArbitraryProperties["params"] = params
		}
		if isSplat(pattern, loadersSplatRune) {
			item.ArbitraryProperties["isSplat"] = true
		}
		if loader != nil {
			item.PhantomTypes = map[string]AdHocType{
				"phantomOutputType": {TypeInstance: loader.O()},
			}
		}
		if pattern == expectedRootDataPattern {
			// Only mark as root data if there's an actual Go loader (task handler),
			// not just a client-only route registered without a handler.
			if opts.LoadersRouter.HasTaskHandler(pattern) {
				foundRootData = true
				item.ArbitraryProperties["isRootData"] = true
			}
		}
		collection = append(collection, item)
		seen[pattern] = struct{}{}
	}

	// Sort client-defined path patterns for deterministic output
	maybeExtraLoaderPaths := v._paths
	extraPathPatterns := make([]string, 0, len(maybeExtraLoaderPaths))
	for pattern := range maybeExtraLoaderPaths {
		if _, ok := seen[pattern]; !ok {
			extraPathPatterns = append(extraPathPatterns, pattern)
		}
	}
	slices.Sort(extraPathPatterns)

	// add any client-defined paths that don't have loaders
	for _, pattern := range extraPathPatterns {
		path := maybeExtraLoaderPaths[pattern]
		item := tsgen.CollectionItem{
			ArbitraryProperties: map[string]any{
				base.DiscriminatorStr:     path.OriginalPattern,
				base.CategoryPropertyName: "loader",
			},
			PhantomTypes: map[string]AdHocType{
				"phantomOutputType": {TypeInstance: mux.None{}},
			},
		}
		params := extractDynamicParamsFromPattern(path.OriginalPattern, actionsDynamicRune)
		if len(params) > 0 {
			item.ArbitraryProperties["params"] = params
		}
		if isSplat(path.OriginalPattern, actionsSplatRune) {
			item.ArbitraryProperties["isSplat"] = true
		}
		collection = append(collection, item)
		seen[path.OriginalPattern] = struct{}{}
	}

	// Sort actions for deterministic output.
	// Create a sortable key combining pattern and method since multiple
	// actions can exist for the same pattern with different HTTP methods.
	type actionKey struct {
		pattern string
		method  string
		index   int
	}
	actionKeys := make([]actionKey, 0, len(allActions))
	for i, action := range allActions {
		actionKeys = append(actionKeys, actionKey{
			pattern: action.OriginalPattern(),
			method:  action.Method(),
			index:   i,
		})
	}
	slices.SortFunc(actionKeys, func(a, b actionKey) int {
		if cmp := strings.Compare(a.pattern, b.pattern); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.method, b.method)
	})

	for _, ak := range actionKeys {
		action := allActions[ak.index]
		method, pattern := action.Method(), action.OriginalPattern()
		_, isQuery := queryMethods[method]
		_, isMutation := mutationMethods[method]
		if !isQuery && !isMutation {
			continue
		}
		categoryPropertyName := "query"
		if isMutation {
			categoryPropertyName = "mutation"
		}
		item := tsgen.CollectionItem{
			ArbitraryProperties: map[string]any{
				base.DiscriminatorStr:     pattern,
				base.CategoryPropertyName: categoryPropertyName,
			},
		}
		if isMutation && method != http.MethodPost {
			item.ArbitraryProperties["method"] = method
		}
		params := extractDynamicParamsFromPattern(pattern, actionsDynamicRune)
		if len(params) > 0 {
			item.ArbitraryProperties["params"] = params
		}
		if isSplat(pattern, actionsSplatRune) {
			item.ArbitraryProperties["isSplat"] = true
		}
		if action != nil {
			item.PhantomTypes = map[string]AdHocType{
				"phantomInputType":  {TypeInstance: action.I()},
				"phantomOutputType": {TypeInstance: action.O()},
			}
		}
		collection = append(collection, item)
	}

	uiVariant := v.config.UIVariant

	var sb strings.Builder

	if foundRootData {
		sb.WriteString(`type VormaRootData = Extract<
	(typeof routes)[number],
	{ isRootData: true }
>["phantomOutputType"];`)
	} else {
		sb.WriteString("type VormaRootData = null;")
	}

	sb.WriteString("\n\n")

	sb.WriteString(fmt.Sprintf(`export type VormaApp = {
	routes: typeof routes;
	appConfig: typeof vormaAppConfig;
	rootData: VormaRootData;
};

export const vormaAppConfig = {
	actionsRouterMountRoot: "%s",
	actionsDynamicRune: "%s",
	actionsSplatRune: "%s",
	loadersDynamicRune: "%s",
	loadersSplatRune: "%s",
	loadersExplicitIndexSegment: "%s",
	__phantom: null as unknown as VormaApp,
} as const;

import type {
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
export type MutationProps<P extends MutationPattern> = VormaMutationProps<
	VormaApp,
	P
>;
export type MutationInput<P extends MutationPattern> = VormaMutationInput<
	VormaApp,
	P
>;
export type MutationOutput<P extends MutationPattern> = VormaMutationOutput<
	VormaApp,
	P
>;

export type RouteProps<P extends VormaLoaderPattern<VormaApp>> =
	VormaRouteProps<VormaApp, P>;
`,
		opts.ActionsRouter.MountRoot(),
		string(actionsDynamicRune),
		string(actionsSplatRune),
		string(loadersDynamicRune),
		string(loadersSplatRune),
		opts.LoadersRouter.GetExplicitIndexSegment(),
		uiVariant,
	))

	if opts.ExtraTSCode != "" {
		sb.WriteString("\n")
		sb.WriteString(opts.ExtraTSCode)
	}

	return tsgen.GenerateTSContent(tsgen.Opts{
		Collection:        collection,
		CollectionVarName: base.CollectionVarName,
		AdHocTypes:        opts.AdHocTypes,
		ExtraTSCode:       sb.String(),
	})
}

func extractDynamicParamsFromPattern(pattern string, dynamicRune rune) []string {
	dynamicParams := []string{}
	segments := matcher.ParseSegments(pattern)
	for _, segment := range segments {
		if len(segment) > 0 && segment[0] == byte(dynamicRune) {
			dynamicParams = append(dynamicParams, segment[1:])
		}
	}
	return dynamicParams
}

func isSplat(pattern string, splatRune rune) bool {
	return strings.HasSuffix(pattern, "/"+string(splatRune))
}
