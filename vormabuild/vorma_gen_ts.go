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

type actionKey struct {
	pattern string
	method  string
	index   int
}

// generateTypeScript generates the route type definitions using live reflection.
func generateTypeScript(input TSGenInput) (string, error) {
	collection := make([]tsgen.CollectionItem, 0)
	allLoaders := input.LoadersRouter.AllRoutes()
	loadersDynamicRune := input.LoadersRouter.GetDynamicParamPrefixRune()
	loadersSplatRune := input.LoadersRouter.GetSplatSegmentRune()
	actionsDynamicRune := input.ActionsRouter.GetDynamicParamPrefixRune()
	actionsSplatRune := input.ActionsRouter.GetSplatSegmentRune()
	expectedRootDataPattern := ""
	if input.LoadersRouter.GetExplicitIndexSegment() != "" {
		expectedRootDataPattern = "/"
	}

	seen := map[string]struct{}{}
	foundRootData := appendLoaderCollectionItems(
		input,
		allLoaders,
		&collection,
		seen,
		loadersDynamicRune,
		loadersSplatRune,
		expectedRootDataPattern,
	)
	appendClientOnlyLoaderCollectionItems(
		input,
		&collection,
		seen,
		actionsDynamicRune,
		actionsSplatRune,
	)
	appendActionCollectionItems(
		input.ActionsRouter.AllRoutes(),
		&collection,
		actionsDynamicRune,
		actionsSplatRune,
	)

	extraCode := buildGeneratedTypeScriptBlock(
		input,
		foundRootData,
		actionsDynamicRune,
		actionsSplatRune,
		loadersDynamicRune,
		loadersSplatRune,
	)

	return tsgen.GenerateTSContent(tsgen.Opts{
		Collection:        collection,
		CollectionVarName: base.CollectionVarName,
		AdHocTypes:        input.AdHocTypes,
		ExtraTSCode:       extraCode,
	})
}

func appendLoaderCollectionItems(
	input TSGenInput,
	allLoaders map[string]mux.AnyNestedRoute,
	collection *[]tsgen.CollectionItem,
	seen map[string]struct{},
	loadersDynamicRune rune,
	loadersSplatRune rune,
	expectedRootDataPattern string,
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
		setPatternMetadata(item.ArbitraryProperties, pattern, loadersDynamicRune, loadersSplatRune)
		if loader != nil {
			item.PhantomTypes = map[string]tsgen.AdHocType{
				"phantomOutputType": {TypeInstance: loader.O()},
			}
		}
		if pattern == expectedRootDataPattern && input.LoadersRouter.HasTaskHandler(pattern) {
			foundRootData = true
			item.ArbitraryProperties["isRootData"] = true
		}
		*collection = append(*collection, item)
		seen[pattern] = struct{}{}
	}
	return foundRootData
}

func appendClientOnlyLoaderCollectionItems(
	input TSGenInput,
	collection *[]tsgen.CollectionItem,
	seen map[string]struct{},
	actionsDynamicRune rune,
	actionsSplatRune rune,
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
		setPatternMetadata(item.ArbitraryProperties, path.OriginalPattern, actionsDynamicRune, actionsSplatRune)
		*collection = append(*collection, item)
		seen[path.OriginalPattern] = struct{}{}
	}
}

func appendActionCollectionItems(
	allActions []mux.AnyRoute,
	collection *[]tsgen.CollectionItem,
	actionsDynamicRune rune,
	actionsSplatRune rune,
) {
	for _, currentActionKey := range sortedActionKeys(allActions) {
		action := allActions[currentActionKey.index]
		method := action.Method()
		pattern := action.OriginalPattern()
		item, ok := buildActionCollectionItem(action, method, pattern, actionsDynamicRune, actionsSplatRune)
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
	actionsDynamicRune rune,
	actionsSplatRune rune,
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
	setPatternMetadata(item.ArbitraryProperties, pattern, actionsDynamicRune, actionsSplatRune)
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
	actionsDynamicRune rune,
	actionsSplatRune rune,
	loadersDynamicRune rune,
	loadersSplatRune rune,
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
	filemapJSONPath: "{{.FilemapJSONPath}}",
	ignoredPatterns: [{{range $i, $e := .IgnoredPatterns}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
	dedupeList: [{{range $i, $e := .DedupeList}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
} as const;
`

var vitePluginTemplate = template.Must(template.New("vitePlugin").Parse(vitePluginTemplateStr))

type vitePluginTemplateData struct {
	Entrypoints      []string
	PublicPathPrefix string
	FuncName         string
	FilemapJSONPath  string
	IgnoredPatterns  []string
	DedupeList       []string
}

func generateRollupOptions(l *vormaruntime.LockedVorma, entrypoints []string) (string, error) {
	v := l.Vorma()

	var sb stringsutil.Builder
	sb.Return()
	sb.Write(tsgen.Comment("Vorma Vite Config:"))
	sb.Return()

	renderedViteConfig, err := renderVitePluginConfig(buildVitePluginTemplateData(v, entrypoints))
	if err != nil {
		return "", err
	}
	sb.Write(renderedViteConfig)
	return sb.String(), nil
}

func buildVitePluginTemplateData(v *vormaruntime.Vorma, entrypoints []string) vitePluginTemplateData {
	return vitePluginTemplateData{
		Entrypoints:      entrypoints,
		PublicPathPrefix: v.Wave.GetPublicPathPrefix(),
		FuncName:         v.Config.BuildtimePublicURLFuncName,
		FilemapJSONPath:  buildFileMapJSONPath(v.Config.TSGenOutDir),
		IgnoredPatterns:  buildViteIgnoredPatterns(v),
		DedupeList:       dedupeListForUIVariant(v.Config.UIVariant),
	}
}

func dedupeListForUIVariant(uiVariant string) []string {
	switch vormaruntime.UIVariant(uiVariant) {
	case vormaruntime.UIVariants.React:
		return reactDedupeList
	case vormaruntime.UIVariants.Preact:
		return preactDedupeList
	case vormaruntime.UIVariants.Solid:
		return solidDedupeList
	default:
		return nil
	}
}

func buildViteIgnoredPatterns(v *vormaruntime.Vorma) []string {
	return []string{
		"**/*.go",
		path.Join("**", v.Wave.GetDistDir()+"/**/*"),
		path.Join("**", v.Wave.GetPrivateStaticDir()+"/**/*"),
		path.Join("**", v.Wave.GetConfigFile()),
		path.Join("**", v.Config.TSGenOutDir+"/**/*"),
		path.Join("**", v.Config.ClientRouteDefsFile),
	}
}

func buildFileMapJSONPath(tsGenOutDir string) string {
	return path.Join(tsGenOutDir, wave.RelPaths.PublicFileMapJSONName())
}

func renderVitePluginConfig(templateData vitePluginTemplateData) (string, error) {
	var buffer bytes.Buffer
	if err := vitePluginTemplate.Execute(&buffer, templateData); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}
	return buffer.String(), nil
}

func getEntrypoints(l *vormaruntime.LockedVorma) []string {
	v := l.Vorma()
	paths := l.GetPaths()
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
func WriteGeneratedTS(l *vormaruntime.LockedVorma) error {
	v := l.Vorma()

	contentBytes, err := generateAndAssembleTSContent(v, l)
	if err != nil {
		return err
	}

	targetPath := generatedTSTargetPath(v.Config.TSGenOutDir)
	return writeGeneratedTSContentIfChanged(v, targetPath, contentBytes)
}

func generateAndAssembleTSContent(v *vormaruntime.Vorma, l *vormaruntime.LockedVorma) ([]byte, error) {
	tsOutput, err := generateTypeScript(tsGenInputForLockedVorma(v, l))
	if err != nil {
		return nil, fmt.Errorf("generate TypeScript: %w", err)
	}

	rollupOptions, err := generateRollupOptions(l, getEntrypoints(l))
	if err != nil {
		return nil, fmt.Errorf("generate rollup options: %w", err)
	}

	return []byte(tsOutput + rollupOptions), nil
}

func writeGeneratedTSContentIfChanged(
	v *vormaruntime.Vorma,
	targetPath string,
	contentBytes []byte,
) error {
	unchanged, err := generatedTSUnchanged(targetPath, contentBytes)
	if err != nil {
		return fmt.Errorf("check existing generated file: %w", err)
	}
	if unchanged {
		v.Log.Info("Generated config unchanged, skipping write")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(targetPath, contentBytes, os.ModePerm); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func tsGenInputForLockedVorma(v *vormaruntime.Vorma, l *vormaruntime.LockedVorma) TSGenInput {
	return TSGenInput{
		LoadersRouter: v.LoadersRouter().NestedRouter,
		ActionsRouter: v.ActionsRouter().Router,
		Paths:         l.GetPaths(),
		Config:        v.Config,
		AdHocTypes:    v.GetAdHocTypes(),
		ExtraTSCode:   v.GetExtraTSCode(),
	}
}

func generatedTSTargetPath(tsGenOutDir string) string {
	return filepath.Join(".", tsGenOutDir, wave.GeneratedTSFileName)
}

func generatedTSUnchanged(targetPath string, newContent []byte) (bool, error) {
	existingBytes, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return bytes.Equal(existingBytes, newContent), nil
}
