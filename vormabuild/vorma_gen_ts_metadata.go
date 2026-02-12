package vormabuild

import (
	"fmt"
	"slices"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/vormaruntime"
)

func setPatternMetadata(properties map[string]any, pattern string, dynamicRune rune, splatRune rune) {
	params := extractDynamicParamsFromPattern(pattern, dynamicRune)
	if len(params) > 0 {
		properties["params"] = params
	}
	if isSplat(pattern, splatRune) {
		properties["isSplat"] = true
	}
}

func sortedLoaderPatterns(allLoaders map[string]mux.AnyNestedRoute) []string {
	loaderPatterns := make([]string, 0, len(allLoaders))
	for pattern := range allLoaders {
		loaderPatterns = append(loaderPatterns, pattern)
	}
	slices.Sort(loaderPatterns)
	return loaderPatterns
}

func sortedClientOnlyPathPatterns(
	paths map[string]*vormaruntime.Path,
	seen map[string]struct{},
) []string {
	extraPathPatterns := make([]string, 0, len(paths))
	for pattern := range paths {
		if _, ok := seen[pattern]; ok {
			continue
		}
		extraPathPatterns = append(extraPathPatterns, pattern)
	}
	slices.Sort(extraPathPatterns)
	return extraPathPatterns
}

func sortedActionKeys(allActions []mux.AnyRoute) []actionKey {
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
	return actionKeys
}

func buildGeneratedTypeScriptBlock(
	input TSGenInput,
	foundRootData bool,
	metadataConfig routePatternMetadataConfig,
) string {
	var sb strings.Builder
	sb.WriteString(rootDataTypeAlias(foundRootData))
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
export type MutationProps<P extends MutationPattern> = VormaMutationProps<VormaApp, P>;
export type MutationInput<P extends MutationPattern> = VormaMutationInput<VormaApp, P>;
export type MutationOutput<P extends MutationPattern> = VormaMutationOutput<VormaApp, P>;

export type RouteProps<P extends VormaLoaderPattern<VormaApp>> = VormaRouteProps<VormaApp, P>;
`,
		input.ActionsRouter.MountRoot(),
		string(metadataConfig.actionsDynamicRune),
		string(metadataConfig.actionsSplatRune),
		string(metadataConfig.loadersDynamicRune),
		string(metadataConfig.loadersSplatRune),
		input.LoadersRouter.GetExplicitIndexSegment(),
		input.Config.UIVariant,
	))

	if input.ExtraTSCode != "" {
		sb.WriteString("\n")
		sb.WriteString(input.ExtraTSCode)
	}
	return sb.String()
}

func rootDataTypeAlias(foundRootData bool) string {
	if foundRootData {
		return `type VormaRootData = Extract<
	(typeof routes)[number],
	{ isRootData: true }
>["phantomOutputType"];`
	}
	return "type VormaRootData = null;"
}

func extractDynamicParamsFromPattern(pattern string, dynamicRune rune) []string {
	var dynamicParams []string
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
