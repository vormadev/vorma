package vormabuild

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/lab/stringsutil"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
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

// TSGenInput contains all data needed for TypeScript generation.
// This makes the function pure - it takes inputs and returns output.
type TSGenInput struct {
	LoadersRouter *mux.NestedRouter
	ActionsRouter *mux.Router
	Paths         map[string]*vormaruntime.Path
	Config        *vormaruntime.VormaConfig
	AdHocTypes    []*tsgen.AdHocType
	ExtraTSCode   string
}

// generateTypeScript generates the route type definitions using live reflection.
func generateTypeScript(input TSGenInput) (string, error) {
	var collection []tsgen.CollectionItem

	allLoaders := input.LoadersRouter.AllRoutes()
	allActions := input.ActionsRouter.AllRoutes()

	loadersDynamicRune := input.LoadersRouter.GetDynamicParamPrefixRune()
	loadersSplatRune := input.LoadersRouter.GetSplatSegmentRune()
	actionsDynamicRune := input.ActionsRouter.GetDynamicParamPrefixRune()
	actionsSplatRune := input.ActionsRouter.GetSplatSegmentRune()

	expectedRootDataPattern := ""
	if input.LoadersRouter.GetExplicitIndexSegment() != "" {
		expectedRootDataPattern = "/"
	}

	var foundRootData bool
	seen := map[string]struct{}{}

	// Sort loader patterns for deterministic output
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
			item.PhantomTypes = map[string]tsgen.AdHocType{
				"phantomOutputType": {TypeInstance: loader.O()},
			}
		}
		if pattern == expectedRootDataPattern {
			if input.LoadersRouter.HasTaskHandler(pattern) {
				foundRootData = true
				item.ArbitraryProperties["isRootData"] = true
			}
		}
		collection = append(collection, item)
		seen[pattern] = struct{}{}
	}

	// Add client-defined paths without Go loaders
	extraPathPatterns := make([]string, 0, len(input.Paths))
	for pattern := range input.Paths {
		if _, ok := seen[pattern]; !ok {
			extraPathPatterns = append(extraPathPatterns, pattern)
		}
	}
	slices.Sort(extraPathPatterns)

	for _, pattern := range extraPathPatterns {
		p := input.Paths[pattern]
		item := tsgen.CollectionItem{
			ArbitraryProperties: map[string]any{
				base.DiscriminatorStr:     p.OriginalPattern,
				base.CategoryPropertyName: "loader",
			},
			PhantomTypes: map[string]tsgen.AdHocType{
				"phantomOutputType": {TypeInstance: mux.None{}},
			},
		}
		params := extractDynamicParamsFromPattern(p.OriginalPattern, actionsDynamicRune)
		if len(params) > 0 {
			item.ArbitraryProperties["params"] = params
		}
		if isSplat(p.OriginalPattern, actionsSplatRune) {
			item.ArbitraryProperties["isSplat"] = true
		}
		collection = append(collection, item)
		seen[p.OriginalPattern] = struct{}{}
	}

	// Sort actions for deterministic output
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
			item.PhantomTypes = map[string]tsgen.AdHocType{
				"phantomInputType":  {TypeInstance: action.I()},
				"phantomOutputType": {TypeInstance: action.O()},
			}
		}
		collection = append(collection, item)
	}

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
export type MutationProps<P extends MutationPattern> = VormaMutationProps<VormaApp, P>;
export type MutationInput<P extends MutationPattern> = VormaMutationInput<VormaApp, P>;
export type MutationOutput<P extends MutationPattern> = VormaMutationOutput<VormaApp, P>;

export type RouteProps<P extends VormaLoaderPattern<VormaApp>> = VormaRouteProps<VormaApp, P>;
`,
		input.ActionsRouter.MountRoot(),
		string(actionsDynamicRune),
		string(actionsSplatRune),
		string(loadersDynamicRune),
		string(loadersSplatRune),
		input.LoadersRouter.GetExplicitIndexSegment(),
		input.Config.UIVariant,
	))

	if input.ExtraTSCode != "" {
		sb.WriteString("\n")
		sb.WriteString(input.ExtraTSCode)
	}

	return tsgen.GenerateTSContent(tsgen.Opts{
		Collection:        collection,
		CollectionVarName: base.CollectionVarName,
		AdHocTypes:        input.AdHocTypes,
		ExtraTSCode:       sb.String(),
	})
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

// --- Vite Config Generation ---

var (
	reactDedupeList  = []string{"react", "react-dom"}
	preactDedupeList = []string{"preact", "preact/hooks", "@preact/signals", "preact/jsx-runtime", "preact/compat", "preact/test-utils"}
	solidDedupeList  = []string{"solid-js", "solid-js/web"}
)

const vitePluginTemplateStr = `
import { staticPublicAssetMap } from "./filemap";
export { staticPublicAssetMap };
export type StaticPublicAsset = keyof typeof staticPublicAssetMap;

declare global {
	function {{.FuncName}}(staticPublicAsset: StaticPublicAsset): string;
}

export const publicPathPrefix = "{{.PublicPathPrefix}}";

export function waveRuntimeURL(originalPublicURL: StaticPublicAsset) {
	const url = staticPublicAssetMap[originalPublicURL] ?? originalPublicURL;
	return publicPathPrefix + url;
}

export const vormaViteConfig = {
	rollupInput: [{{range $i, $e := .Entrypoints}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
	publicPathPrefix,
	staticPublicAssetMap,
	buildtimePublicURLFuncName: "{{.FuncName}}",
	ignoredPatterns: [{{range $i, $e := .IgnoredPatterns}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
	dedupeList: [{{range $i, $e := .DedupeList}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
} as const;
`

var vitePluginTemplate = template.Must(template.New("vitePlugin").Parse(vitePluginTemplateStr))

func generateRollupOptions(v *vormaruntime.Vorma, entrypoints []string) (string, error) {
	var sb stringsutil.Builder
	sb.Return()
	sb.Write(tsgen.Comment("Vorma Vite Config:"))
	sb.Return()

	var dedupeList []string
	switch vormaruntime.UIVariant(v.Config.UIVariant) {
	case vormaruntime.UIVariants.React:
		dedupeList = reactDedupeList
	case vormaruntime.UIVariants.Preact:
		dedupeList = preactDedupeList
	case vormaruntime.UIVariants.Solid:
		dedupeList = solidDedupeList
	}

	ignoredList := []string{
		"**/*.go",
		path.Join("**", v.Wave.GetDistDir()+"/**/*"),
		path.Join("**", v.Wave.GetPrivateStaticDir()+"/**/*"),
		path.Join("**", v.Wave.GetConfigFile()),
		path.Join("**", v.Config.TSGenOutDir+"/**/*"),
		path.Join("**", v.Config.ClientRouteDefsFile),
	}

	var buf bytes.Buffer
	err := vitePluginTemplate.Execute(&buf, map[string]any{
		"Entrypoints":      entrypoints,
		"PublicPathPrefix": v.Wave.GetPublicPathPrefix(),
		"FuncName":         v.Config.BuildtimePublicURLFuncName,
		"IgnoredPatterns":  ignoredList,
		"DedupeList":       dedupeList,
	})
	if err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}
	sb.Write(buf.String())
	return sb.String(), nil
}

func getEntrypoints(v *vormaruntime.Vorma) []string {
	paths := v.UnsafeGetPaths()
	entryPoints := make(map[string]struct{}, len(paths)+1)
	entryPoints[path.Clean(v.Config.ClientEntry)] = struct{}{}
	for _, p := range paths {
		if p.SrcPath != "" {
			entryPoints[p.SrcPath] = struct{}{}
		}
	}
	keys := make([]string, 0, len(entryPoints))
	for key := range entryPoints {
		keys = append(keys, key)
	}
	slices.SortStableFunc(keys, strings.Compare)
	return keys
}

// WriteGeneratedTS generates and writes the complete TypeScript output file.
// IMPORTANT: Caller must hold v.mu.Lock() OR ensure exclusive access.
func WriteGeneratedTS(v *vormaruntime.Vorma) error {
	input := TSGenInput{
		LoadersRouter: v.LoadersRouter().NestedRouter,
		ActionsRouter: v.ActionsRouter().Router,
		Paths:         v.UnsafeGetPaths(),
		Config:        v.Config,
		AdHocTypes:    v.GetAdHocTypes(),
		ExtraTSCode:   v.GetExtraTSCode(),
	}

	tsOutput, err := generateTypeScript(input)
	if err != nil {
		return fmt.Errorf("generate TypeScript: %w", err)
	}

	rollupOptions, err := generateRollupOptions(v, getEntrypoints(v))
	if err != nil {
		return fmt.Errorf("generate rollup options: %w", err)
	}

	content := tsOutput + rollupOptions
	target := filepath.Join(".", v.Config.TSGenOutDir, wave.GeneratedTSFileName)

	// Skip write if content unchanged (prevents infinite file watcher loop)
	if existingBytes, err := os.ReadFile(target); err == nil {
		if bytes.Equal(existingBytes, []byte(content)) {
			vormaruntime.Log.Info("Generated config unchanged, skipping write")
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(target, []byte(content), os.ModePerm); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}
