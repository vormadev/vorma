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

	"github.com/vormadev/vorma/lab/stringsutil"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

type generatedTSAssemblyDependencies struct {
	generateTypeScript  func(TSGenInput) (string, error)
	generateRollupInput func(*vormaruntime.LockedVorma, []string) (string, error)
	getEntrypoints      func(*vormaruntime.LockedVorma) []string
}

type generatedTSWriteDependencies struct {
	generateAndAssembleTSContent     func(*vormaruntime.Vorma, *vormaruntime.LockedVorma) ([]byte, error)
	writeGeneratedTSContentIfChanged func(*vormaruntime.Vorma, string, []byte) error
}

type generatedTSWriteFileDependencies struct {
	generatedTSUnchanged     func(string, []byte) (bool, error)
	makeGeneratedTSDirectory func(string, fs.FileMode) error
	writeGeneratedTSFile     func(string, []byte, fs.FileMode) error
}

var generatedTSAssemblyDeps = generatedTSAssemblyDependencies{
	generateTypeScript:  generateTypeScript,
	generateRollupInput: generateRollupOptions,
	getEntrypoints:      getEntrypoints,
}

var generatedTSWriteDeps = generatedTSWriteDependencies{
	generateAndAssembleTSContent:     generateAndAssembleTSContent,
	writeGeneratedTSContentIfChanged: writeGeneratedTSContentIfChanged,
}

var generatedTSWriteFileDeps = generatedTSWriteFileDependencies{
	generatedTSUnchanged:     generatedTSUnchanged,
	makeGeneratedTSDirectory: os.MkdirAll,
	writeGeneratedTSFile:     writeFileAtomically,
}

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
	ignoredPatterns := []string{
		"**/*.go",
		path.Join("**", v.Wave.GetDistDir()+"/**/*"),
		path.Join("**", v.Wave.GetPrivateStaticDir()+"/**/*"),
		path.Join("**", v.Config.TSGenOutDir+"/**/*"),
	}

	if configFileIgnoredPattern := formatConfigFilePatternForViteIgnore(v.Wave.GetConfigFilePath()); configFileIgnoredPattern != "" {
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
	v := l.Vorma()
	paths := l.GetPaths()
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

// WriteGeneratedTS generates and writes the complete TypeScript output file.
func WriteGeneratedTS(l *vormaruntime.LockedVorma) error {
	v := l.Vorma()

	contentBytes, err := generatedTSWriteDeps.generateAndAssembleTSContent(v, l)
	if err != nil {
		return err
	}

	targetPath := filepath.Join(".", v.Config.TSGenOutDir, wave.GeneratedTSFileName)
	return generatedTSWriteDeps.writeGeneratedTSContentIfChanged(v, targetPath, contentBytes)
}

func generateAndAssembleTSContent(v *vormaruntime.Vorma, l *vormaruntime.LockedVorma) ([]byte, error) {
	tsOutput, err := generatedTSAssemblyDeps.generateTypeScript(tsGenInputForLockedVorma(v, l))
	if err != nil {
		return nil, fmt.Errorf("generate TypeScript: %w", err)
	}

	rollupOptions, err := generatedTSAssemblyDeps.generateRollupInput(
		l,
		generatedTSAssemblyDeps.getEntrypoints(l),
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
	unchanged, err := generatedTSWriteFileDeps.generatedTSUnchanged(targetPath, contentBytes)
	if err != nil {
		return fmt.Errorf("check existing generated file: %w", err)
	}
	if unchanged {
		v.Log.Info("Generated config unchanged, skipping write")
		return nil
	}

	if err := generatedTSWriteFileDeps.makeGeneratedTSDirectory(filepath.Dir(targetPath), os.ModePerm); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := generatedTSWriteFileDeps.writeGeneratedTSFile(targetPath, contentBytes, buildArtifactFileMode); err != nil {
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
