package vormabuild

import (
	"net/http"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/lab/tsgen"
)

// TypeScript generation lives entirely in the build package.
// Runtime never generates TypeScript - it only reads pre-built artifacts.

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

// tsGenInput contains all data needed for TypeScript generation.
// This makes the function pure - it takes inputs and returns output.
type tsGenInput struct {
	LoadersRouter *mux.NestedRouter
	ActionsRouter *mux.Router
	Paths         map[string]*vormaruntime.Path
	Config        *vormaruntime.VormaConfig
	AdHocTypes    []*tsgen.AdHocType
	ExtraTSCode   string
}

type actionKey struct {
	pattern string
	method  string
	index   int
}

type routePatternMetadataConfig struct {
	expectedRootDataPattern string
	loadersDynamicRune      rune
	loadersSplatRune        rune
	actionsDynamicRune      rune
	actionsSplatRune        rune
}

func deriveRoutePatternMetadataConfig(
	input tsGenInput,
) routePatternMetadataConfig {
	config := routePatternMetadataConfig{
		loadersDynamicRune: input.LoadersRouter.DynamicParamPrefix(),
		loadersSplatRune:   input.LoadersRouter.SplatSegmentIdentifier(),
		actionsDynamicRune: input.ActionsRouter.DynamicParamPrefix(),
		actionsSplatRune:   input.ActionsRouter.SplatSegmentIdentifier(),
	}

	if input.LoadersRouter.ExplicitIndexSegmentIdentifier() != "" {
		config.expectedRootDataPattern = "/"
	}

	return config
}

// generateTypeScript generates the route type definitions using live reflection.
func generateTypeScript(input tsGenInput) (string, error) {
	collection := make([]tsgen.CollectionItem, 0)
	allLoaders := input.LoadersRouter.AllRoutes()
	metadataConfig := deriveRoutePatternMetadataConfig(input)

	seen := map[string]struct{}{}
	foundRootData := appendLoaderCollectionItems(
		input,
		allLoaders,
		&collection,
		seen,
		metadataConfig,
	)
	appendClientOnlyLoaderCollectionItems(
		input,
		&collection,
		seen,
		metadataConfig,
	)
	appendActionCollectionItems(
		input.ActionsRouter.AllRoutes(),
		&collection,
		metadataConfig,
	)

	extraCode := buildGeneratedTypeScriptBlock(
		input,
		foundRootData,
		metadataConfig,
	)

	return tsgen.GenerateTSContent(tsgen.Opts{
		Collection:        collection,
		CollectionVarName: base.CollectionVarName,
		AdHocTypes:        input.AdHocTypes,
		ExtraTSCode:       extraCode,
	})
}

func appendLoaderCollectionItems(
	input tsGenInput,
	allLoaders map[string]mux.AnyNestedRoute,
	collection *[]tsgen.CollectionItem,
	seen map[string]struct{},
	metadataConfig routePatternMetadataConfig,
) bool {
	foundRootData := false
	for _, pattern := range sortedLoaderPatterns(allLoaders) {
		loader := allLoaders[pattern]
		item := tsgen.CollectionItem{
			ArbitraryProperties: map[string]any{
				base.DiscriminatorStr:     pattern,
				base.CategoryPropertyName: "loader",
			},
		}
		setPatternMetadata(
			item.ArbitraryProperties,
			pattern,
			metadataConfig.loadersDynamicRune,
			metadataConfig.loadersSplatRune,
		)
		if loader != nil {
			item.PhantomTypes = map[string]tsgen.AdHocType{
				"phantomOutputType": {TypeInstance: loader.O()},
			}
		}
		if pattern == metadataConfig.expectedRootDataPattern &&
			input.LoadersRouter.HasTaskHandler(pattern) {
			foundRootData = true
			item.ArbitraryProperties["isRootData"] = true
		}
		*collection = append(*collection, item)
		seen[pattern] = struct{}{}
	}
	return foundRootData
}

func appendClientOnlyLoaderCollectionItems(
	input tsGenInput,
	collection *[]tsgen.CollectionItem,
	seen map[string]struct{},
	metadataConfig routePatternMetadataConfig,
) {
	for _, pattern := range sortedClientOnlyPathPatterns(input.Paths, seen) {
		path := input.Paths[pattern]
		item := tsgen.CollectionItem{
			ArbitraryProperties: map[string]any{
				base.DiscriminatorStr:     path.OriginalPattern,
				base.CategoryPropertyName: "loader",
			},
			PhantomTypes: map[string]tsgen.AdHocType{
				"phantomOutputType": {TypeInstance: mux.None{}},
			},
		}
		setPatternMetadata(
			item.ArbitraryProperties,
			path.OriginalPattern,
			metadataConfig.loadersDynamicRune,
			metadataConfig.loadersSplatRune,
		)
		*collection = append(*collection, item)
		seen[path.OriginalPattern] = struct{}{}
	}
}

func appendActionCollectionItems(
	allActions []mux.AnyRoute,
	collection *[]tsgen.CollectionItem,
	metadataConfig routePatternMetadataConfig,
) {
	for _, currentActionKey := range sortedActionKeys(allActions) {
		action := allActions[currentActionKey.index]
		method := action.Method()
		pattern := action.OriginalPattern()
		item, ok := buildActionCollectionItem(
			action,
			method,
			pattern,
			metadataConfig,
		)
		if !ok {
			continue
		}
		*collection = append(*collection, item)
	}
}

func buildActionCollectionItem(
	action mux.AnyRoute,
	method string,
	pattern string,
	metadataConfig routePatternMetadataConfig,
) (tsgen.CollectionItem, bool) {
	categoryPropertyName, isMutation, ok := actionCategoryForMethod(method)
	if !ok {
		return tsgen.CollectionItem{}, false
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
	setPatternMetadata(
		item.ArbitraryProperties,
		pattern,
		metadataConfig.actionsDynamicRune,
		metadataConfig.actionsSplatRune,
	)
	if action != nil {
		item.PhantomTypes = map[string]tsgen.AdHocType{
			"phantomInputType":  {TypeInstance: action.I()},
			"phantomOutputType": {TypeInstance: action.O()},
		}
	}
	return item, true
}

func actionCategoryForMethod(method string) (string, bool, bool) {
	if _, isQuery := queryMethods[method]; isQuery {
		return "query", false, true
	}
	if _, isMutation := mutationMethods[method]; isMutation {
		return "mutation", true, true
	}
	return "", false, false
}
