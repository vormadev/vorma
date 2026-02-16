package vormabuild

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/lab/stringsutil"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/wave"
)

type generatedTSAssemblyDependencies struct {
	generateTypeScript                func(tsGenInput) (string, error)
	generateRollupInput               func(*vormaruntime.LockedVorma, []string) (string, error)
	generateRollupInputForEntrypoints func(*vormaruntime.Vorma, []string) (string, error)
	getEntrypoints                    func(*vormaruntime.LockedVorma) []string
	getEntrypointsForPaths            func(*vormaruntime.Vorma, map[string]*vormaruntime.Path) []string
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
			return defaultGeneratedTSAssemblyExecutor.generateAndAssembleTSContent(v, l)
		},
		generateAndAssembleTSContentForRuntimeStateSnapshot: func(
			v *vormaruntime.Vorma,
			runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
		) ([]byte, error) {
			return defaultGeneratedTSAssemblyExecutor.generateAndAssembleTSContentForRouteBuildRuntimeStateSnapshot(
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

var defaultGeneratedTSWriteExecutor = newGeneratedTSWriteExecutor(
	generatedTSWriteDependencies{},
)

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
	return generateRollupOptionsForEntrypoints(l.Vorma(), entrypoints)
}

func generateRollupOptionsForEntrypoints(v *vormaruntime.Vorma, entrypoints []string) (string, error) {
	var sb stringsutil.Builder
	sb.Return()
	sb.Write(tsgen.Comment("Vorma Vite Config:"))
	sb.Return()

	renderedViteConfig, err := renderVitePluginConfig(buildVitePluginTemplateData(v, entrypoints))
	if err != nil {
		return "", fmt.Errorf("render vite plugin config: %w", err)
	}
	sb.Write(renderedViteConfig)
	return sb.String(), nil
}

func buildVitePluginTemplateData(v *vormaruntime.Vorma, entrypoints []string) vitePluginTemplateData {
	return vitePluginTemplateData{
		Entrypoints:      entrypoints,
		PublicPathPrefix: v.Wave.GetPublicPathPrefix(),
		FuncName:         v.Config.BuildtimePublicURLFuncName,
		FilemapJSONPath:  path.Join(v.Config.TSGenOutDir, wave.RelPaths.PublicFileMapJSONName()),
		IgnoredPatterns:  buildViteIgnoredPatterns(v),
		DedupeList:       dedupeListForUIVariant(v.Config.UIVariant),
	}
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

func buildViteIgnoredPatterns(v *vormaruntime.Vorma) []string {
	ignoredPatterns := []string{
		"**/*.go",
		path.Join("**", v.Wave.GetDistDir()+"/**/*"),
		path.Join("**", v.Wave.GetPrivateStaticDir()+"/**/*"),
		path.Join("**", v.Config.TSGenOutDir+"/**/*"),
	}

	if configFileIgnoredPattern := formatConfigFilePatternForViteIgnore(v.Wave.GetConfigFile()); configFileIgnoredPattern != "" {
		ignoredPatterns = append(ignoredPatterns, configFileIgnoredPattern)
	}

	for _, routeDefinitionPattern := range normalizeRouteDefinitionPatternsInInputOrder(
		v.Config.ClientRouteDefinitionPatterns,
	) {
		ignoredPatterns = append(
			ignoredPatterns,
			path.Join("**", routeDefinitionPattern),
		)
	}
	return ignoredPatterns
}

func formatConfigFilePatternForViteIgnore(configFilePattern string) string {
	trimmedPattern := strings.TrimSpace(configFilePattern)
	if trimmedPattern == "" {
		return ""
	}

	if filepath.IsAbs(trimmedPattern) {
		return filepath.ToSlash(trimmedPattern)
	}

	return filepath.ToSlash(path.Join("**", trimmedPattern))
}

func renderVitePluginConfig(templateData vitePluginTemplateData) (string, error) {
	var buffer bytes.Buffer
	if err := vitePluginTemplate.Execute(&buffer, templateData); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}
	return buffer.String(), nil
}

func getEntrypoints(l *vormaruntime.LockedVorma) []string {
	return getEntrypointsForPaths(l.Vorma(), l.GetPaths())
}

func getEntrypointsForPaths(
	v *vormaruntime.Vorma,
	paths map[string]*vormaruntime.Path,
) []string {
	entryPoints := make(map[string]struct{}, len(paths)+1)
	entryPoints[path.Clean(v.Config.ClientEntry)] = struct{}{}
	for _, currentPath := range paths {
		if currentPath.SrcPath != "" {
			entryPoints[currentPath.SrcPath] = struct{}{}
		}
	}
	keys := make([]string, 0, len(entryPoints))
	for key := range entryPoints {
		keys = append(keys, key)
	}
	slices.SortStableFunc(keys, strings.Compare)
	return keys
}

// writeGeneratedTS generates and writes the complete TypeScript output file.
func writeGeneratedTS(l *vormaruntime.LockedVorma) error {
	return defaultGeneratedTSWriteExecutor.writeGeneratedTS(l)
}

func writeGeneratedTSForRouteBuildRuntimeStateSnapshot(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
) error {
	return defaultGeneratedTSWriteExecutor.writeGeneratedTSForRouteBuildRuntimeStateSnapshot(
		v,
		runtimeStateSnapshot,
	)
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

	contentBytes, err := generatedTSWriteExecutor.dependencies.generateAndAssembleTSContent(v, l)
	if err != nil {
		return err
	}

	targetPath := filepath.Join(".", v.Config.TSGenOutDir, wave.GeneratedTSFileName)
	return generatedTSWriteExecutor.dependencies.writeGeneratedTSContentIfChanged(v, targetPath, contentBytes)
}

func (generatedTSWriteExecutor generatedTSWriteExecutor) writeGeneratedTSForRouteBuildRuntimeStateSnapshot(
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

	targetPath := filepath.Join(".", v.Config.TSGenOutDir, wave.GeneratedTSFileName)
	return generatedTSWriteExecutor.dependencies.writeGeneratedTSContentIfChanged(v, targetPath, contentBytes)
}

func generateAndAssembleTSContent(v *vormaruntime.Vorma, l *vormaruntime.LockedVorma) ([]byte, error) {
	return generateAndAssembleTSContentWithDependencies(v, l, generatedTSAssemblyDependencies{})
}

func generateAndAssembleTSContentWithDependencies(
	v *vormaruntime.Vorma,
	l *vormaruntime.LockedVorma,
	dependencies generatedTSAssemblyDependencies,
) ([]byte, error) {
	return newGeneratedTSAssemblyExecutor(dependencies).generateAndAssembleTSContent(v, l)
}

func generateAndAssembleTSContentForRouteBuildRuntimeStateSnapshotWithDependencies(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
	dependencies generatedTSAssemblyDependencies,
) ([]byte, error) {
	return newGeneratedTSAssemblyExecutor(
		dependencies,
	).generateAndAssembleTSContentForRouteBuildRuntimeStateSnapshot(
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

	rollupOptions, err := generatedTSAssemblyExecutor.dependencies.generateRollupInput(
		l,
		generatedTSAssemblyExecutor.dependencies.getEntrypoints(l),
	)
	if err != nil {
		return nil, fmt.Errorf("generate rollup options: %w", err)
	}

	return assembleGeneratedTSContent(tsOutput, rollupOptions), nil
}

func (generatedTSAssemblyExecutor generatedTSAssemblyExecutor) generateAndAssembleTSContentForRouteBuildRuntimeStateSnapshot(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
) ([]byte, error) {
	tsOutput, err := generatedTSAssemblyExecutor.dependencies.generateTypeScript(
		tsGenInputForRouteBuildRuntimeStateSnapshot(v, runtimeStateSnapshot),
	)
	if err != nil {
		return nil, fmt.Errorf("generate TypeScript: %w", err)
	}

	rollupOptions, err := generatedTSAssemblyExecutor.dependencies.generateRollupInputForEntrypoints(
		v,
		generatedTSAssemblyExecutor.dependencies.getEntrypointsForPaths(
			v,
			runtimeStateSnapshot.paths,
		),
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
	return newGeneratedTSWriteFileExecutor(dependencies).writeGeneratedTSContentIfChanged(
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
		v.Log.Info("Generated config unchanged, skipping write")
		return nil
	}

	if err := generatedTSWriteFileExecutor.dependencies.makeGeneratedTSDirectory(
		filepath.Dir(targetPath),
		os.ModePerm,
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

func tsGenInputForLockedVorma(v *vormaruntime.Vorma, l *vormaruntime.LockedVorma) tsGenInput {
	return tsGenInput{
		LoadersRouter: v.LoadersRouter().NestedRouter,
		ActionsRouter: v.ActionsRouter().Router,
		Paths:         l.GetPaths(),
		Config:        v.Config,
		AdHocTypes:    v.GetAdHocTypes(),
		ExtraTSCode:   v.GetExtraTSCode(),
	}
}

func tsGenInputForRouteBuildRuntimeStateSnapshot(
	v *vormaruntime.Vorma,
	runtimeStateSnapshot routeBuildRuntimeStateSnapshot,
) tsGenInput {
	return tsGenInput{
		LoadersRouter: v.LoadersRouter().NestedRouter,
		ActionsRouter: v.ActionsRouter().Router,
		Paths:         runtimeStateSnapshot.paths,
		Config:        v.Config,
		AdHocTypes:    v.GetAdHocTypes(),
		ExtraTSCode:   v.GetExtraTSCode(),
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
