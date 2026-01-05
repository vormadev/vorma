package vorma

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/rpc"
	"github.com/vormadev/vorma/kit/tsgen"
)

type AdHocType = rpc.AdHocType

type tsGenOptions struct {
	LoadersRouter *mux.NestedRouter
	ActionsRouter *mux.Router
	AdHocTypes    []*AdHocType
	ExtraTSCode   string
}

var base = rpc.BaseOptions{
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

	for pattern, loader := range allLoaders {
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
			foundRootData = true
			item.ArbitraryProperties["isRootData"] = true
		}
		collection = append(collection, item)
		seen[pattern] = struct{}{}
	}

	// add any client-defined paths that don't have loaders
	maybeExtraLoaderPaths := v._paths
	for _, path := range maybeExtraLoaderPaths {
		if _, ok := seen[path.OriginalPattern]; ok {
			continue
		}
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

	for _, action := range allActions {
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

	uiVariant := v.Wave.GetVormaUIVariant()

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
