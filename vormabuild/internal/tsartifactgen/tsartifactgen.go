// Package tsartifactgen owns TypeScript build artifact generation for Vorma.
//
// It turns runtime route metadata into deterministic generated TS artifacts and
// Vite config payloads. vormabuild orchestrates when generation runs, while this
// package owns how content is assembled and written.
package tsartifactgen

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimeconfig"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/lab/stringsutil"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/vormabuild/internal/routeparse"
)

// WriteDependencies configures filesystem writes for generated TS artifacts.
type WriteDependencies struct {
	WriteGeneratedTSFile func(string, []byte, fs.FileMode) error
}

// RuntimeStateSnapshot captures route state used to generate TS without
// requiring callers to pass the full vormabuild snapshot type.
type RuntimeStateSnapshot struct {
	Paths   map[string]*vormaruntime.Path
	BuildID string
}

// WriteGeneratedTS generates and writes the TS artifact for the current locked
// runtime state.
func WriteGeneratedTS(
	l *vormaruntime.LockedVorma,
	dependencies WriteDependencies,
) error {
	normalizedDependencies := normalizeWriteDependencies(dependencies)
	return writeGeneratedTSWithDependencies(
		l,
		generatedTSWriteDependencies{
			writeGeneratedTSContentIfChanged: func(
				v *vormaruntime.Vorma,
				targetPath string,
				contentBytes []byte,
			) error {
				return writeGeneratedTSContentIfChangedWithDependencies(
					v,
					targetPath,
					contentBytes,
					generatedTSWriteFileDependencies{
						writeGeneratedTSFile: normalizedDependencies.WriteGeneratedTSFile,
					},
				)
			},
		},
	)
}

// WriteGeneratedTSForRuntimeState generates and writes the TS artifact for a
// captured route-runtime snapshot.
func WriteGeneratedTSForRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot RuntimeStateSnapshot,
	dependencies WriteDependencies,
) error {
	normalizedDependencies := normalizeWriteDependencies(dependencies)
	executor := newGeneratedTSWriteExecutor(
		generatedTSWriteDependencies{
			writeGeneratedTSContentIfChanged: func(
				vv *vormaruntime.Vorma,
				targetPath string,
				contentBytes []byte,
			) error {
				return writeGeneratedTSContentIfChangedWithDependencies(
					vv,
					targetPath,
					contentBytes,
					generatedTSWriteFileDependencies{
						writeGeneratedTSFile: normalizedDependencies.WriteGeneratedTSFile,
					},
				)
			},
		},
	)
	return executor.writeGeneratedTSForRouteBuildRuntimeState(
		v,
		routeBuildRuntimeStateSnapshot{
			paths:   runtimeStateSnapshot.Paths,
			buildID: runtimeStateSnapshot.BuildID,
		},
	)
}

func normalizeWriteDependencies(
	dependencies WriteDependencies,
) WriteDependencies {
	if dependencies.WriteGeneratedTSFile == nil {
		dependencies.WriteGeneratedTSFile = writeFileAtomically
	}
	return dependencies
}

type routeBuildRuntimeStateSnapshot struct {
	paths   map[string]*vormaruntime.Path
	buildID string
}

const (
	buildArtifactDirectoryMode fs.FileMode = 0o755
	buildArtifactFileMode      fs.FileMode = 0o644
)

func writeFileAtomically(
	targetPath string,
	fileContents []byte,
	fileMode fs.FileMode,
) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), buildArtifactDirectoryMode); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	return os.WriteFile(targetPath, fileContents, fileMode)
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

// tsGenInput contains all data needed for TypeScript generation.
// This makes the function pure - it takes inputs and returns output.
type tsGenInput struct {
	LoadersRouter *nestedmux.Router
	ActionsRouter *mux.Router
	Paths         map[string]*vormaruntime.Path
	Config        vormaruntime.VormaConfig
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
	allLoaders map[string]nestedmux.AnyRoute,
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

func setPatternMetadata(
	properties map[string]any,
	pattern string,
	dynamicRune rune,
	splatRune rune,
) {
	params := extractDynamicParamsFromPattern(pattern, dynamicRune)
	if len(params) > 0 {
		properties["params"] = params
	}
	if isSplat(pattern, splatRune) {
		properties["isSplat"] = true
	}
}

func sortedLoaderPatterns(
	allLoaders map[string]nestedmux.AnyRoute,
) []string {
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
	input tsGenInput,
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
	loadersExplicitIndexSegmentIdentifier: "%s",
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
		input.LoadersRouter.ExplicitIndexSegmentIdentifier(),
		input.Config.UIVariant(),
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

func extractDynamicParamsFromPattern(
	pattern string,
	dynamicRune rune,
) []string {
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

type generatedTSAssemblyDependencies struct {
	generateTypeScript                func(tsGenInput) (string, error)
	generateRollupInput               func(*vormaruntime.LockedVorma, []string) (string, error)
	generateRollupInputForEntrypoints func(*vormaruntime.Vorma, []string) (string, error)
	getEntrypoints                    func(*vormaruntime.LockedVorma) ([]string, error)
	getEntrypointsForPaths            func(*vormaruntime.Vorma, map[string]*vormaruntime.Path) ([]string, error)
}

type generatedTSWriteDependencies struct {
	generateAndAssembleTSContent                        func(*vormaruntime.Vorma, *vormaruntime.LockedVorma) ([]byte, error)
	generateAndAssembleTSContentForRuntimeStateSnapshot func(*vormaruntime.Vorma, routeBuildRuntimeStateSnapshot) ([]byte, error)
	writeGeneratedTSContentIfChanged                    func(*vormaruntime.Vorma, string, []byte) error
}

type generatedTSWriteFileDependencies struct {
	generatedTSUnchanged     func(string, []byte) (bool, error)
	makeGeneratedTSDirectory func(string, fs.FileMode) error
	writeGeneratedTSFile     func(string, []byte, fs.FileMode) error
}

type generatedTSAssemblyExecutor struct {
	dependencies generatedTSAssemblyDependencies
}

type generatedTSWriteExecutor struct {
	dependencies generatedTSWriteDependencies
}

type generatedTSWriteFileExecutor struct {
	dependencies generatedTSWriteFileDependencies
}

func defaultGeneratedTSAssemblyDependencies() generatedTSAssemblyDependencies {
	return generatedTSAssemblyDependencies{
		generateTypeScript:                generateTypeScript,
		generateRollupInput:               generateRollupOptions,
		generateRollupInputForEntrypoints: generateRollupOptionsForEntrypoints,
		getEntrypoints:                    getEntrypoints,
		getEntrypointsForPaths:            getEntrypointsForPaths,
	}
}

func normalizeGeneratedTSAssemblyDependencies(
	dependencies generatedTSAssemblyDependencies,
) generatedTSAssemblyDependencies {
	defaultDependencies := defaultGeneratedTSAssemblyDependencies()
	if dependencies.generateTypeScript == nil {
		dependencies.generateTypeScript = defaultDependencies.generateTypeScript
	}
	if dependencies.generateRollupInput == nil {
		dependencies.generateRollupInput = defaultDependencies.generateRollupInput
	}
	if dependencies.generateRollupInputForEntrypoints == nil {
		dependencies.generateRollupInputForEntrypoints = defaultDependencies.generateRollupInputForEntrypoints
	}
	if dependencies.getEntrypoints == nil {
		dependencies.getEntrypoints = defaultDependencies.getEntrypoints
	}
	if dependencies.getEntrypointsForPaths == nil {
		dependencies.getEntrypointsForPaths = defaultDependencies.getEntrypointsForPaths
	}
	return dependencies
}

func newGeneratedTSAssemblyExecutor(
	dependencies generatedTSAssemblyDependencies,
) generatedTSAssemblyExecutor {
	return generatedTSAssemblyExecutor{
		dependencies: normalizeGeneratedTSAssemblyDependencies(dependencies),
	}
}

func defaultGeneratedTSWriteFileDependencies() generatedTSWriteFileDependencies {
	return generatedTSWriteFileDependencies{
		generatedTSUnchanged:     generatedTSUnchanged,
		makeGeneratedTSDirectory: os.MkdirAll,
		writeGeneratedTSFile:     writeFileAtomically,
	}
}

func normalizeGeneratedTSWriteFileDependencies(
	dependencies generatedTSWriteFileDependencies,
) generatedTSWriteFileDependencies {
	defaultDependencies := defaultGeneratedTSWriteFileDependencies()
	if dependencies.generatedTSUnchanged == nil {
		dependencies.generatedTSUnchanged = defaultDependencies.generatedTSUnchanged
	}
	if dependencies.makeGeneratedTSDirectory == nil {
		dependencies.makeGeneratedTSDirectory = defaultDependencies.makeGeneratedTSDirectory
	}
	if dependencies.writeGeneratedTSFile == nil {
		dependencies.writeGeneratedTSFile = defaultDependencies.writeGeneratedTSFile
	}
	return dependencies
}

func newGeneratedTSWriteFileExecutor(
	dependencies generatedTSWriteFileDependencies,
) generatedTSWriteFileExecutor {
	return generatedTSWriteFileExecutor{
		dependencies: normalizeGeneratedTSWriteFileDependencies(dependencies),
	}
}

func defaultGeneratedTSWriteDependencies() generatedTSWriteDependencies {
	return generatedTSWriteDependencies{
		generateAndAssembleTSContent: func(
			v *vormaruntime.Vorma,
			l *vormaruntime.LockedVorma,
		) ([]byte, error) {
			return defaultGeneratedTSAssemblyExecutor.generateAndAssembleTSContent(
				v,
				l,
			)
		},
		generateAndAssembleTSContentForRuntimeStateSnapshot: func(
			v *vormaruntime.Vorma,
			runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
		) ([]byte, error) {
			return defaultGeneratedTSAssemblyExecutor.generateAndAssembleTSContentForRouteBuildRuntimeState(
				v,
				runtimeStateSnapshot,
			)
		},
		writeGeneratedTSContentIfChanged: func(
			v *vormaruntime.Vorma,
			targetPath string,
			contentBytes []byte,
		) error {
			return defaultGeneratedTSWriteFileExecutor.writeGeneratedTSContentIfChanged(
				v,
				targetPath,
				contentBytes,
			)
		},
	}
}

func normalizeGeneratedTSWriteDependencies(
	dependencies generatedTSWriteDependencies,
) generatedTSWriteDependencies {
	defaultDependencies := defaultGeneratedTSWriteDependencies()
	if dependencies.generateAndAssembleTSContent == nil {
		dependencies.generateAndAssembleTSContent = defaultDependencies.generateAndAssembleTSContent
	}
	if dependencies.generateAndAssembleTSContentForRuntimeStateSnapshot == nil {
		dependencies.generateAndAssembleTSContentForRuntimeStateSnapshot = defaultDependencies.generateAndAssembleTSContentForRuntimeStateSnapshot
	}
	if dependencies.writeGeneratedTSContentIfChanged == nil {
		dependencies.writeGeneratedTSContentIfChanged = defaultDependencies.writeGeneratedTSContentIfChanged
	}
	return dependencies
}

func newGeneratedTSWriteExecutor(
	dependencies generatedTSWriteDependencies,
) generatedTSWriteExecutor {
	return generatedTSWriteExecutor{
		dependencies: normalizeGeneratedTSWriteDependencies(dependencies),
	}
}

var defaultGeneratedTSAssemblyExecutor = newGeneratedTSAssemblyExecutor(
	generatedTSAssemblyDependencies{},
)

var defaultGeneratedTSWriteFileExecutor = newGeneratedTSWriteFileExecutor(
	generatedTSWriteFileDependencies{},
)

var (
	reactDedupeList  = []string{"react", "react-dom"}
	preactDedupeList = []string{
		"preact",
		"preact/hooks",
		"@preact/signals",
		"preact/jsx-runtime",
		"preact/compat",
		"preact/test-utils",
	}
	solidDedupeList = []string{"solid-js", "solid-js/web"}
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
	const hashedPublicURL = staticPublicAssetMap[originalPublicURL];
	if (!hashedPublicURL) {
		throw new Error("Wave public URL lookup miss: " + originalPublicURL);
	}
	return publicPathPrefix + hashedPublicURL;
}

export const vormaViteConfig = {
	rollupInput: [{{range $i, $e := .Entrypoints}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
	publicPathPrefix,
	buildtimePublicURLFuncName: "{{.FuncName}}",
	distDir: "{{.DistDir}}",
	ignoredPatterns: [{{range $i, $e := .IgnoredPatterns}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
	dedupeList: [{{range $i, $e := .DedupeList}}{{if $i}},{{end}}
		"{{$e}}"{{end}}
	],
} as const;
`

var vitePluginTemplate = template.Must(
	template.New("vitePlugin").Parse(vitePluginTemplateStr),
)

type vitePluginTemplateData struct {
	Entrypoints      []string
	PublicPathPrefix string
	FuncName         string
	DistDir          string
	IgnoredPatterns  []string
	DedupeList       []string
}

func generateRollupOptions(
	l *vormaruntime.LockedVorma,
	entrypoints []string,
) (string, error) {
	return generateRollupOptionsForEntrypoints(l.Vorma(), entrypoints)
}

func generateRollupOptionsForEntrypoints(
	v *vormaruntime.Vorma,
	entrypoints []string,
) (string, error) {
	var sb stringsutil.Builder
	sb.Return()
	sb.Write(tsgen.Comment("Vorma Vite Config:"))
	sb.Return()

	vitePluginTemplateData, vitePluginTemplateDataError := buildVitePluginTemplateData(
		v,
		entrypoints,
	)
	if vitePluginTemplateDataError != nil {
		return "", fmt.Errorf(
			"build vite plugin template data: %w",
			vitePluginTemplateDataError,
		)
	}

	renderedViteConfig, err := renderVitePluginConfig(
		vitePluginTemplateData,
	)
	if err != nil {
		return "", fmt.Errorf("render vite plugin config: %w", err)
	}
	sb.Write(renderedViteConfig)
	return sb.String(), nil
}

func buildVitePluginTemplateData(
	v *vormaruntime.Vorma,
	entrypoints []string,
) (vitePluginTemplateData, error) {
	currentWorkingDirectory, currentWorkingDirectoryError := os.Getwd()
	if currentWorkingDirectoryError != nil {
		return vitePluginTemplateData{}, fmt.Errorf(
			"resolve current working directory: %w",
			currentWorkingDirectoryError,
		)
	}

	currentWorkingDirectoryRelativeDistDir, distDirError := pathRelativeToCurrentWorkingDirectoryForGeneratedTypeScript(
		currentWorkingDirectory,
		distRootPathForGeneratedTypeScript(v),
	)
	if distDirError != nil {
		return vitePluginTemplateData{}, fmt.Errorf(
			"normalize DistDir for generated TypeScript: %w",
			distDirError,
		)
	}

	ignoredPatterns, ignoredPatternsError := buildViteIgnoredPatterns(
		v,
		currentWorkingDirectory,
	)
	if ignoredPatternsError != nil {
		return vitePluginTemplateData{}, fmt.Errorf(
			"build Vite ignored patterns: %w",
			ignoredPatternsError,
		)
	}

	return vitePluginTemplateData{
		Entrypoints:      entrypoints,
		PublicPathPrefix: v.Wave.PublicPathPrefix(),
		FuncName:         v.Config.BuildtimePublicURLFuncName(),
		DistDir:          currentWorkingDirectoryRelativeDistDir,
		IgnoredPatterns:  ignoredPatterns,
		DedupeList:       dedupeListForUIVariant(v.Config.UIVariant()),
	}, nil
}

func dedupeListForUIVariant(uiVariant string) []string {
	switch vormaruntime.UIVariant(uiVariant) {
	case vormaruntime.UIVariantReact:
		return reactDedupeList
	case vormaruntime.UIVariantPreact:
		return preactDedupeList
	case vormaruntime.UIVariantSolid:
		return solidDedupeList
	default:
		return nil
	}
}

func buildViteIgnoredPatterns(
	v *vormaruntime.Vorma,
	currentWorkingDirectory string,
) ([]string, error) {
	currentWorkingDirectoryRelativeDistDir, distDirError := pathRelativeToCurrentWorkingDirectoryForGeneratedTypeScript(
		currentWorkingDirectory,
		distRootPathForGeneratedTypeScript(v),
	)
	if distDirError != nil {
		return nil, fmt.Errorf(
			"normalize DistDir for ignored patterns: %w",
			distDirError,
		)
	}

	currentWorkingDirectoryRelativePrivateStaticDir, privateStaticDirError := pathRelativeToCurrentWorkingDirectoryForGeneratedTypeScript(
		currentWorkingDirectory,
		v.Wave.PrivateStaticDir(),
	)
	if privateStaticDirError != nil {
		return nil, fmt.Errorf(
			"normalize private static dir for ignored patterns: %w",
			privateStaticDirError,
		)
	}

	currentWorkingDirectoryRelativeTSGenOutDir, tsGenOutDirError := pathRelativeToCurrentWorkingDirectoryForGeneratedTypeScript(
		currentWorkingDirectory,
		v.Config.TSGenOutDir(),
	)
	if tsGenOutDirError != nil {
		return nil, fmt.Errorf(
			"normalize TSGenOutDir for ignored patterns: %w",
			tsGenOutDirError,
		)
	}

	ignoredPatterns := []string{
		"**/*.go",
		path.Join("**", currentWorkingDirectoryRelativeDistDir, "**/*"),
		path.Join(
			"**",
			currentWorkingDirectoryRelativePrivateStaticDir,
			"**/*",
		),
		path.Join("**", currentWorkingDirectoryRelativeTSGenOutDir, "**/*"),
	}

	configFileIgnoredPattern, configFileIgnoredPatternError := formatConfigFilePatternForViteIgnore(
		currentWorkingDirectory,
		v.Wave.ConfigFile(),
	)
	if configFileIgnoredPatternError != nil {
		return nil, configFileIgnoredPatternError
	}
	if configFileIgnoredPattern != "" {
		ignoredPatterns = append(ignoredPatterns, configFileIgnoredPattern)
	}

	normalizedRouteDefinitionPatterns, err := routeparse.NormalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ClientRouteDefinitionPatterns(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"normalize client route definition patterns: %w",
			err,
		)
	}
	for _, routeDefinitionPattern := range normalizedRouteDefinitionPatterns {
		resolveRootRelativeRouteDefinitionPattern, routeDefinitionPatternError := normalizePathOrPatternToResolveRootRelativeForGeneratedTypeScript(
			v,
			routeDefinitionPattern,
		)
		if routeDefinitionPatternError != nil {
			return nil, fmt.Errorf(
				"normalize client route definition pattern %q: %w",
				routeDefinitionPattern,
				routeDefinitionPatternError,
			)
		}
		ignoredPatterns = append(
			ignoredPatterns,
			path.Join("**", resolveRootRelativeRouteDefinitionPattern),
		)
	}
	return ignoredPatterns, nil
}

func pathRelativeToCurrentWorkingDirectoryForGeneratedTypeScript(
	currentWorkingDirectory string,
	pathToNormalize string,
) (string, error) {
	trimmedPathToNormalize := strings.TrimSpace(pathToNormalize)
	if trimmedPathToNormalize == "" {
		return "", nil
	}

	if !filepath.IsAbs(trimmedPathToNormalize) {
		return filepath.ToSlash(filepath.Clean(trimmedPathToNormalize)), nil
	}

	currentWorkingDirectoryRelativePath, relativePathError := filepath.Rel(
		currentWorkingDirectory,
		trimmedPathToNormalize,
	)
	if relativePathError != nil {
		return "", fmt.Errorf(
			"make %q relative to current working directory %q: %w",
			trimmedPathToNormalize,
			currentWorkingDirectory,
			relativePathError,
		)
	}

	normalizedCurrentWorkingDirectoryRelativePath := filepath.Clean(
		currentWorkingDirectoryRelativePath,
	)
	if normalizedCurrentWorkingDirectoryRelativePath == ".." ||
		strings.HasPrefix(
			normalizedCurrentWorkingDirectoryRelativePath,
			".."+string(filepath.Separator),
		) {
		return "", fmt.Errorf(
			"path %q is outside current working directory %q",
			trimmedPathToNormalize,
			currentWorkingDirectory,
		)
	}

	return filepath.ToSlash(normalizedCurrentWorkingDirectoryRelativePath), nil
}

func distRootPathForGeneratedTypeScript(v *vormaruntime.Vorma) string {
	if v == nil || v.Wave == nil {
		return ""
	}

	if parsedWaveConfig := v.Wave.ParsedConfig(); parsedWaveConfig != nil &&
		parsedWaveConfig.Dist() != nil {
		if trimmedDistRootPath := strings.TrimSpace(
			parsedWaveConfig.Dist().Root(),
		); trimmedDistRootPath != "" {
			return trimmedDistRootPath
		}
	}

	return strings.TrimSpace(v.Wave.DistDir())
}

func formatConfigFilePatternForViteIgnore(
	currentWorkingDirectory string,
	configFilePattern string,
) (string, error) {
	trimmedPattern := strings.TrimSpace(configFilePattern)
	if trimmedPattern == "" {
		return "", nil
	}

	currentWorkingDirectoryRelativeConfigFilePattern, configFilePatternError := pathRelativeToCurrentWorkingDirectoryForGeneratedTypeScript(
		currentWorkingDirectory,
		trimmedPattern,
	)
	if configFilePatternError != nil {
		return "", fmt.Errorf(
			"normalize config file pattern for Vite ignore: %w",
			configFilePatternError,
		)
	}

	return filepath.ToSlash(
		path.Join("**", currentWorkingDirectoryRelativeConfigFilePattern),
	), nil
}

func renderVitePluginConfig(
	templateData vitePluginTemplateData,
) (string, error) {
	var buffer bytes.Buffer
	if err := vitePluginTemplate.Execute(&buffer, templateData); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}
	return buffer.String(), nil
}

func getEntrypoints(l *vormaruntime.LockedVorma) ([]string, error) {
	return getEntrypointsForPaths(l.Vorma(), l.Paths())
}

func getEntrypointsForPaths(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
) ([]string, error) {
	if v == nil || v.Wave == nil || v.Wave.ParsedConfig() == nil {
		return nil, errors.New("vorma app with parsed wave config is required")
	}

	entryPoints := make(map[string]struct{}, len(paths)+1)
	clientEntryRelativeToResolveRoot, clientEntryError := normalizePathOrPatternToResolveRootRelativeForGeneratedTypeScript(
		v,
		v.Config.ClientEntry(),
	)
	if clientEntryError != nil {
		return nil, fmt.Errorf("normalize client entry: %w", clientEntryError)
	}
	entryPoints[clientEntryRelativeToResolveRoot] = struct{}{}
	for _, currentPath := range paths {
		if currentPath.SrcPath != "" {
			sourcePathRelativeToResolveRoot, sourcePathError := normalizePathOrPatternToResolveRootRelativeForGeneratedTypeScript(
				v,
				currentPath.SrcPath,
			)
			if sourcePathError != nil {
				return nil, fmt.Errorf(
					"normalize route source path %q: %w",
					currentPath.SrcPath,
					sourcePathError,
				)
			}
			entryPoints[sourcePathRelativeToResolveRoot] = struct{}{}
		}
	}
	keys := make([]string, 0, len(entryPoints))
	for key := range entryPoints {
		keys = append(keys, key)
	}
	slices.SortStableFunc(keys, strings.Compare)
	return keys, nil
}

func normalizePathOrPatternToResolveRootRelativeForGeneratedTypeScript(
	v *vormaruntime.Vorma,
	pathOrPattern string,
) (string, error) {
	if v == nil || v.Wave == nil || v.Wave.ParsedConfig() == nil {
		return "", errors.New("vorma app with parsed wave config is required")
	}

	resolveRootRelativePathOrPattern, relativePathOrPatternError := runtimeconfig.NormalizePathOrPatternToResolveRootRelative(
		v.Wave.ParsedConfig().ResolveRoot(),
		pathOrPattern,
	)
	if relativePathOrPatternError != nil {
		return "", relativePathOrPatternError
	}
	return filepath.ToSlash(filepath.Clean(resolveRootRelativePathOrPattern)), nil
}

func writeGeneratedTSWithDependencies(
	l *vormaruntime.LockedVorma,
	dependencies generatedTSWriteDependencies,
) error {
	return newGeneratedTSWriteExecutor(dependencies).writeGeneratedTS(l)
}

func (generatedTSWriteExecutor generatedTSWriteExecutor) writeGeneratedTS(
	l *vormaruntime.LockedVorma,
) error {
	v := l.Vorma()

	contentBytes, err := generatedTSWriteExecutor.dependencies.generateAndAssembleTSContent(
		v,
		l,
	)
	if err != nil {
		return err
	}

	targetPath := filepath.Join(
		v.Config.TSGenOutDir(),
		runtimepaths.GeneratedTypeScriptIndexFileName,
	)
	return generatedTSWriteExecutor.dependencies.writeGeneratedTSContentIfChanged(
		v,
		targetPath,
		contentBytes,
	)
}

func (generatedTSWriteExecutor generatedTSWriteExecutor) writeGeneratedTSForRouteBuildRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
) error {
	contentBytes, err := generatedTSWriteExecutor.dependencies.generateAndAssembleTSContentForRuntimeStateSnapshot(
		v,
		runtimeStateSnapshot,
	)
	if err != nil {
		return err
	}

	targetPath := filepath.Join(
		v.Config.TSGenOutDir(),
		runtimepaths.GeneratedTypeScriptIndexFileName,
	)
	return generatedTSWriteExecutor.dependencies.writeGeneratedTSContentIfChanged(
		v,
		targetPath,
		contentBytes,
	)
}

func generateAndAssembleTSContent(
	v *vormaruntime.Vorma,
	l *vormaruntime.LockedVorma,
) ([]byte, error) {
	return generateAndAssembleTSContentWithDependencies(
		v,
		l,
		generatedTSAssemblyDependencies{},
	)
}

func generateAndAssembleTSContentWithDependencies(
	v *vormaruntime.Vorma,
	l *vormaruntime.LockedVorma,
	dependencies generatedTSAssemblyDependencies,
) ([]byte, error) {
	return newGeneratedTSAssemblyExecutor(
		dependencies,
	).generateAndAssembleTSContent(v, l)
}

func generateAndAssembleTSContentForRouteBuildRuntimeStateSnapshotWithDependencies(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
	dependencies generatedTSAssemblyDependencies,
) ([]byte, error) {
	return newGeneratedTSAssemblyExecutor(
		dependencies,
	).generateAndAssembleTSContentForRouteBuildRuntimeState(
		v,
		runtimeStateSnapshot,
	)
}

func (generatedTSAssemblyExecutor generatedTSAssemblyExecutor) generateAndAssembleTSContent(
	v *vormaruntime.Vorma,
	l *vormaruntime.LockedVorma,
) ([]byte, error) {
	tsOutput, err := generatedTSAssemblyExecutor.dependencies.generateTypeScript(
		tsGenInputForLockedVorma(v, l),
	)
	if err != nil {
		return nil, fmt.Errorf("generate TypeScript: %w", err)
	}

	entrypoints, entrypointsError := generatedTSAssemblyExecutor.dependencies.getEntrypoints(
		l,
	)
	if entrypointsError != nil {
		return nil, fmt.Errorf("resolve entrypoints: %w", entrypointsError)
	}
	rollupOptions, err := generatedTSAssemblyExecutor.dependencies.generateRollupInput(
		l,
		entrypoints,
	)
	if err != nil {
		return nil, fmt.Errorf("generate rollup options: %w", err)
	}

	return assembleGeneratedTSContent(tsOutput, rollupOptions), nil
}

func (generatedTSAssemblyExecutor generatedTSAssemblyExecutor) generateAndAssembleTSContentForRouteBuildRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
) ([]byte, error) {
	tsOutput, err := generatedTSAssemblyExecutor.dependencies.generateTypeScript(
		tsGenInputForRouteBuildRuntimeState(v, runtimeStateSnapshot),
	)
	if err != nil {
		return nil, fmt.Errorf("generate TypeScript: %w", err)
	}

	entrypoints, entrypointsError := generatedTSAssemblyExecutor.dependencies.getEntrypointsForPaths(
		v,
		runtimeStateSnapshot.paths,
	)
	if entrypointsError != nil {
		return nil, fmt.Errorf("resolve entrypoints: %w", entrypointsError)
	}
	rollupOptions, err := generatedTSAssemblyExecutor.dependencies.generateRollupInputForEntrypoints(
		v,
		entrypoints,
	)
	if err != nil {
		return nil, fmt.Errorf("generate rollup options: %w", err)
	}

	return assembleGeneratedTSContent(tsOutput, rollupOptions), nil
}

func assembleGeneratedTSContent(tsOutput string, rollupOptions string) []byte {
	var contentBuilder strings.Builder
	contentBuilder.Grow(len(tsOutput) + len(rollupOptions))
	contentBuilder.WriteString(tsOutput)
	contentBuilder.WriteString(rollupOptions)
	return []byte(contentBuilder.String())
}

func writeGeneratedTSContentIfChanged(
	v *vormaruntime.Vorma,
	targetPath string,
	contentBytes []byte,
) error {
	return writeGeneratedTSContentIfChangedWithDependencies(
		v,
		targetPath,
		contentBytes,
		generatedTSWriteFileDependencies{},
	)
}

func writeGeneratedTSContentIfChangedWithDependencies(
	v *vormaruntime.Vorma,
	targetPath string,
	contentBytes []byte,
	dependencies generatedTSWriteFileDependencies,
) error {
	return newGeneratedTSWriteFileExecutor(
		dependencies,
	).writeGeneratedTSContentIfChanged(
		v,
		targetPath,
		contentBytes,
	)
}

func (generatedTSWriteFileExecutor generatedTSWriteFileExecutor) writeGeneratedTSContentIfChanged(
	v *vormaruntime.Vorma,
	targetPath string,
	contentBytes []byte,
) error {
	unchanged, err := generatedTSWriteFileExecutor.dependencies.generatedTSUnchanged(
		targetPath,
		contentBytes,
	)
	if err != nil {
		return fmt.Errorf("check existing generated file: %w", err)
	}
	if unchanged {
		v.Log.Debug("Generated config unchanged, skipping write")
		return nil
	}

	if err := generatedTSWriteFileExecutor.dependencies.makeGeneratedTSDirectory(
		filepath.Dir(targetPath),
		buildArtifactDirectoryMode,
	); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := generatedTSWriteFileExecutor.dependencies.writeGeneratedTSFile(
		targetPath,
		contentBytes,
		buildArtifactFileMode,
	); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func tsGenInputForLockedVorma(
	v *vormaruntime.Vorma,
	l *vormaruntime.LockedVorma,
) tsGenInput {
	return tsGenInput{
		LoadersRouter: v.LoadersRouter().NestedRouter,
		ActionsRouter: v.ActionsRouter().Router,
		Paths:         l.Paths(),
		Config:        v.Config,
		AdHocTypes:    v.AdHocTypes(),
		ExtraTSCode:   v.ExtraTSCode(),
	}
}

func tsGenInputForRouteBuildRuntimeState(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
) tsGenInput {
	return tsGenInput{
		LoadersRouter: v.LoadersRouter().NestedRouter,
		ActionsRouter: v.ActionsRouter().Router,
		Paths:         runtimeStateSnapshot.paths,
		Config:        v.Config,
		AdHocTypes:    v.AdHocTypes(),
		ExtraTSCode:   v.ExtraTSCode(),
	}
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
