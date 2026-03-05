// Package wavetest centralizes shared test fixtures for waveconfig.ParsedConfig and
// related helpers used across Go test packages in this repository.
package wavetest

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveenv"
	"github.com/vormadev/vorma/wave/wavewatch"
)

// NewDiscardLogger returns a logger that discards all output.
func NewDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// NewParsedConfigAtRoot builds a default ParsedConfig rooted at root.
func NewParsedConfigAtRoot(
	tb testing.TB,
	root string,
) waveconfig.ParsedConfig {
	tb.Helper()

	normalizedRoot := normalizeParsedConfigFixtureRoot(root)
	rawConfigJSON, marshalError := json.Marshal(map[string]any{
		"Core": map[string]any{
			"ProjectID":    "wavetest-project",
			"MainAppEntry": "cmd/app",
			"StaticAssetDirs": map[string]any{
				"Public":  filepath.ToSlash(filepath.Join("static", "public")),
				"Private": filepath.ToSlash(filepath.Join("static", "private")),
			},
		},
		"Watch": map[string]any{},
	})
	if marshalError != nil {
		panic(fmt.Sprintf(
			"wavetest.NewParsedConfigAtRoot: marshal default config: %v",
			marshalError,
		))
	}
	parsedConfig, parseError := waveconfig.ParseConfigJSONWithConfigPath(
		rawConfigJSON,
		"wave.config.json",
	)
	if parseError != nil {
		panic(fmt.Sprintf(
			"wavetest.NewParsedConfigAtRoot: parse default config: %v",
			parseError,
		))
	}
	reanchorParsedConfigPathsToRoot(
		parsedConfig,
		normalizedRoot,
	)
	if rootInfo, rootStatError := os.Stat(normalizedRoot); rootStatError == nil {
		if !rootInfo.IsDir() {
			panic(
				fmt.Sprintf(
					"wavetest.NewParsedConfigAtRoot: root %q must be a directory",
					normalizedRoot,
				),
			)
		}
		distRootPath := filepath.Join(normalizedRoot, ".wavedist")
		if createDistRootError := os.MkdirAll(
			distRootPath,
			0o755,
		); createDistRootError != nil {
			panic(
				fmt.Sprintf(
					"wavetest.NewParsedConfigAtRoot: create dist root %q: %v",
					distRootPath,
					createDistRootError,
				),
			)
		}
	} else if !os.IsNotExist(rootStatError) {
		panic(
			fmt.Sprintf(
				"wavetest.NewParsedConfigAtRoot: stat root %q: %v",
				normalizedRoot,
				rootStatError,
			),
		)
	}
	return parsedConfig
}

func normalizeParsedConfigFixtureRoot(root string) string {
	trimmedRoot := strings.TrimSpace(root)
	if trimmedRoot == "" {
		panic(
			"wavetest.NewParsedConfigAtRoot: root is required and must be absolute",
		)
	}
	normalizedRoot := filepath.Clean(trimmedRoot)
	if normalizedRoot == "." {
		panic(
			"wavetest.NewParsedConfigAtRoot: root must not be '.'; use t.TempDir() to avoid repo-local test artifacts",
		)
	}
	if !filepath.IsAbs(normalizedRoot) {
		panic(
			fmt.Sprintf(
				"wavetest.NewParsedConfigAtRoot: root must be absolute, got %q",
				root,
			),
		)
	}
	return normalizedRoot
}

// reanchorParsedConfigPathsToRoot rewrites parsed path fields to the provided
// absolute root so tests operate on OS-temp fixture paths without writing
// workspace artifacts.
func reanchorParsedConfigPathsToRoot(
	cfg waveconfig.ParsedConfig,
	root string,
) {
	if cfg == nil {
		return
	}
	normalizedRoot := filepath.Clean(strings.TrimSpace(root))
	if normalizedRoot == "" || !filepath.IsAbs(normalizedRoot) {
		panic(
			fmt.Sprintf(
				"wavetest.reanchorParsedConfigPathsToRoot: root must be absolute, got %q",
				root,
			),
		)
	}

	configStruct := parsedConfigStructForUnsafeMutation(cfg)
	forceSetStringFieldByNameInStruct(
		configStruct,
		"configFileDirectory",
		normalizedRoot,
	)

	reanchorCoreConfigPathsToRoot(configStruct, normalizedRoot)
	reanchorViteConfigPathsToRoot(configStruct, normalizedRoot)
	reanchorWatchConfigPathsToRoot(configStruct, normalizedRoot)
	reanchorDistLayoutPathsToRoot(configStruct, normalizedRoot)
}

func parsedConfigStructForUnsafeMutation(
	cfg waveconfig.ParsedConfig,
) reflect.Value {
	configValue := reflect.ValueOf(cfg)
	if !configValue.IsValid() ||
		configValue.Kind() != reflect.Pointer ||
		configValue.IsNil() {
		panic("wavetest parsed config mutation requires pointer-backed config")
	}
	return configValue.Elem()
}

func reanchorCoreConfigPathsToRoot(
	configStruct reflect.Value,
	root string,
) {
	coreStruct, ok := implementationStructField(configStruct, "core")
	if !ok {
		return
	}
	for _, pathFieldName := range []string{
		"mainAppEntry",
		"staticAssetDirsPrivate",
		"staticAssetDirsPublic",
		"criticalCSSEntryFile",
		"nonCriticalCSSEntryFile",
	} {
		reanchorStringPathFieldByNameToRoot(coreStruct, pathFieldName, root)
	}
}

func reanchorViteConfigPathsToRoot(
	configStruct reflect.Value,
	root string,
) {
	viteStruct, ok := implementationStructField(configStruct, "vite")
	if !ok {
		return
	}
	for _, pathFieldName := range []string{
		"jsPackageManagerCmdDir",
		"viteConfigFile",
	} {
		reanchorStringPathFieldByNameToRoot(viteStruct, pathFieldName, root)
	}
}

func reanchorWatchConfigPathsToRoot(
	configStruct reflect.Value,
	root string,
) {
	watchStruct, ok := implementationStructField(configStruct, "watch")
	if !ok {
		return
	}

	includeField := watchStruct.FieldByName("include")
	if includeField.IsValid() {
		includeSlice := forceFieldWritable(includeField).Interface().([]wavewatch.WatchedFile)
		mutatedIncludeSlice := append(
			[]wavewatch.WatchedFile(nil),
			includeSlice...,
		)
		for includeIndex := range mutatedIncludeSlice {
			mutatedIncludeSlice[includeIndex].Pattern = reanchorPathToRoot(
				root,
				mutatedIncludeSlice[includeIndex].Pattern,
			)
			for hookIndex := range mutatedIncludeSlice[includeIndex].OnChangeHooks {
				for excludeIndex := range mutatedIncludeSlice[includeIndex].OnChangeHooks[hookIndex].Exclude {
					mutatedIncludeSlice[includeIndex].OnChangeHooks[hookIndex].Exclude[excludeIndex] = reanchorPathToRoot(
						root,
						mutatedIncludeSlice[includeIndex].OnChangeHooks[hookIndex].Exclude[excludeIndex],
					)
				}
			}
		}
		forceFieldWritable(includeField).Set(reflect.ValueOf(mutatedIncludeSlice))
	}

	for _, pathSliceFieldName := range []string{"excludeDirs", "excludeFiles"} {
		pathSliceField := watchStruct.FieldByName(pathSliceFieldName)
		if !pathSliceField.IsValid() {
			continue
		}
		pathSlice := forceFieldWritable(pathSliceField).Interface().([]string)
		mutatedPathSlice := append([]string(nil), pathSlice...)
		for pathIndex := range mutatedPathSlice {
			mutatedPathSlice[pathIndex] = reanchorPathToRoot(
				root,
				mutatedPathSlice[pathIndex],
			)
		}
		forceFieldWritable(pathSliceField).Set(reflect.ValueOf(mutatedPathSlice))
	}
}

func reanchorDistLayoutPathsToRoot(
	configStruct reflect.Value,
	root string,
) {
	distStruct, ok := implementationStructField(configStruct, "dist")
	if !ok {
		return
	}
	forceSetStringFieldByNameInStruct(
		distStruct,
		"root",
		filepath.Join(root, ".wavedist"),
	)
}

func implementationStructField(
	parentStruct reflect.Value,
	fieldName string,
) (reflect.Value, bool) {
	implementationField := parentStruct.FieldByName(fieldName)
	if !implementationField.IsValid() {
		return reflect.Value{}, false
	}
	if implementationField.IsNil() {
		return reflect.Value{}, false
	}
	resolvedField := implementationField.Elem()
	if resolvedField.Kind() == reflect.Pointer {
		if resolvedField.IsNil() {
			return reflect.Value{}, false
		}
		resolvedField = resolvedField.Elem()
	}
	if !resolvedField.IsValid() || resolvedField.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}
	return resolvedField, true
}

func reanchorStringPathFieldByNameToRoot(
	parentStruct reflect.Value,
	fieldName string,
	root string,
) {
	pathField := parentStruct.FieldByName(fieldName)
	if !pathField.IsValid() {
		return
	}
	forceSetStringFieldByNameInStruct(
		parentStruct,
		fieldName,
		reanchorPathToRoot(root, pathField.String()),
	)
}

func reanchorPathToRoot(root string, pathValue string) string {
	trimmedPath := strings.TrimSpace(pathValue)
	if trimmedPath == "" {
		return ""
	}
	if filepath.IsAbs(trimmedPath) {
		return filepath.Clean(trimmedPath)
	}
	return filepath.Clean(filepath.Join(root, trimmedPath))
}

func forceSetStringFieldByNameInStruct(
	parentStruct reflect.Value,
	fieldName string,
	fieldValue string,
) {
	field := parentStruct.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		panic(
			fmt.Sprintf(
				"wavetest.forceSetStringFieldByNameInStruct: missing string field %q",
				fieldName,
			),
		)
	}
	trimmedFieldValue := strings.TrimSpace(fieldValue)
	if trimmedFieldValue == "" {
		forceFieldWritable(field).SetString("")
		return
	}
	forceFieldWritable(field).SetString(filepath.Clean(fieldValue))
}

func forceFieldWritable(field reflect.Value) reflect.Value {
	if !field.CanAddr() {
		panic("wavetest.forceFieldWritable: field is not addressable")
	}
	return reflect.NewAt(
		field.Type(),
		unsafe.Pointer(field.UnsafeAddr()),
	).Elem()
}

type rawWaveConfigJSONDocument struct {
	Core  rawWaveCoreConfigJSONSection   `json:"Core"`
	Vite  *rawWaveViteConfigJSONSection  `json:"Vite,omitempty"`
	Watch *rawWaveWatchConfigJSONSection `json:"Watch,omitempty"`
}

type rawWaveCoreConfigJSONSection struct {
	ProjectID                        string `json:"ProjectID"`
	ResolveRoot                      string `json:"ResolveRoot,omitempty"`
	DevBuildHook                     string `json:"DevBuildHook,omitempty"`
	DevBuildHookTimeoutMilliseconds  int    `json:"DevBuildHookTimeoutMilliseconds,omitempty"`
	ProdBuildHook                    string `json:"ProdBuildHook,omitempty"`
	ProdBuildHookTimeoutMilliseconds int    `json:"ProdBuildHookTimeoutMilliseconds,omitempty"`
	MainAppEntry                     string `json:"MainAppEntry"`
	StaticAssetDirs                  struct {
		Private string `json:"Private"`
		Public  string `json:"Public"`
	} `json:"StaticAssetDirs"`
	CSSEntryFiles struct {
		Critical    string `json:"Critical,omitempty"`
		NonCritical string `json:"NonCritical,omitempty"`
	} `json:"CSSEntryFiles,omitempty"`
	PublicPathPrefix  string `json:"PublicPathPrefix,omitempty"`
	ServerOnlyMode    bool   `json:"ServerOnlyMode,omitempty"`
	SequentialGoBuild bool   `json:"SequentialGoBuild,omitempty"`
}

type rawWaveViteConfigJSONSection struct {
	JSPackageManagerBaseCmd string `json:"JSPackageManagerBaseCmd"`
	JSPackageManagerCmdDir  string `json:"JSPackageManagerCmdDir,omitempty"`
	DefaultPort             int    `json:"DefaultPort,omitempty"`
	ViteConfigFile          string `json:"ViteConfigFile,omitempty"`
}

type rawWaveWatchConfigJSONSection struct {
	HealthcheckEndpoint    string `json:"HealthcheckEndpoint,omitempty"`
	HookStageFailurePolicy string `json:"HookStageFailurePolicy,omitempty"`
	HookCommandTimeouts    struct {
		PreCommandTimeoutMilliseconds              int `json:"PreCommandTimeoutMilliseconds,omitempty"`
		ConcurrentCommandTimeoutMilliseconds       int `json:"ConcurrentCommandTimeoutMilliseconds,omitempty"`
		ConcurrentNoWaitCommandTimeoutMilliseconds int `json:"ConcurrentNoWaitCommandTimeoutMilliseconds,omitempty"`
		PostCommandTimeoutMilliseconds             int `json:"PostCommandTimeoutMilliseconds,omitempty"`
	} `json:"HookCommandTimeouts"`
	HookCallbackTimeouts struct {
		PreCallbackTimeoutMilliseconds              int `json:"PreCallbackTimeoutMilliseconds,omitempty"`
		ConcurrentCallbackTimeoutMilliseconds       int `json:"ConcurrentCallbackTimeoutMilliseconds,omitempty"`
		ConcurrentNoWaitCallbackTimeoutMilliseconds int `json:"ConcurrentNoWaitCallbackTimeoutMilliseconds,omitempty"`
		PostCallbackTimeoutMilliseconds             int `json:"PostCallbackTimeoutMilliseconds,omitempty"`
	} `json:"HookCallbackTimeouts"`
	Include []wavewatch.WatchedFile `json:"Include,omitempty"`
	Exclude struct {
		Dirs  []string `json:"Dirs,omitempty"`
		Files []string `json:"Files,omitempty"`
	} `json:"Exclude,omitempty"`
}

// MarshalParsedConfigToRawJSON serializes one ParsedConfig into Wave config JSON
// shape using config-file-relative path fields.
func MarshalParsedConfigToRawJSON(
	parsedConfig waveconfig.ParsedConfig,
) ([]byte, error) {
	if parsedConfig == nil || parsedConfig.Core() == nil {
		return nil, fmt.Errorf("parsed config with Core section is required")
	}
	resolveRoot := parsedConfig.ResolveRoot()
	if strings.TrimSpace(resolveRoot) == "" {
		resolveRoot = "."
	}

	core := parsedConfig.Core()
	mainAppEntry, mainAppEntryError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
		resolveRoot,
		core.MainAppEntry(),
	)
	if mainAppEntryError != nil {
		return nil, fmt.Errorf("normalize Core.MainAppEntry: %w", mainAppEntryError)
	}
	staticAssetsPrivateDirectory, staticAssetsPrivateDirectoryError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
		resolveRoot,
		core.StaticAssetDirsPrivate(),
	)
	if staticAssetsPrivateDirectoryError != nil {
		return nil, fmt.Errorf(
			"normalize Core.StaticAssetDirs.Private: %w",
			staticAssetsPrivateDirectoryError,
		)
	}
	staticAssetsPublicDirectory, staticAssetsPublicDirectoryError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
		resolveRoot,
		core.StaticAssetDirsPublic(),
	)
	if staticAssetsPublicDirectoryError != nil {
		return nil, fmt.Errorf(
			"normalize Core.StaticAssetDirs.Public: %w",
			staticAssetsPublicDirectoryError,
		)
	}
	criticalCSSEntryFilePath, criticalCSSEntryFilePathError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
		resolveRoot,
		core.CriticalCSSEntryFile(),
	)
	if criticalCSSEntryFilePathError != nil {
		return nil, fmt.Errorf(
			"normalize Core.CSSEntryFiles.Critical: %w",
			criticalCSSEntryFilePathError,
		)
	}
	nonCriticalCSSEntryFilePath, nonCriticalCSSEntryFilePathError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
		resolveRoot,
		core.NonCriticalCSSEntryFile(),
	)
	if nonCriticalCSSEntryFilePathError != nil {
		return nil, fmt.Errorf(
			"normalize Core.CSSEntryFiles.NonCritical: %w",
			nonCriticalCSSEntryFilePathError,
		)
	}

	rawConfigDocument := rawWaveConfigJSONDocument{
		Core: rawWaveCoreConfigJSONSection{
			ProjectID:                        core.ProjectID(),
			ResolveRoot:                      core.ConfiguredResolveRoot(),
			DevBuildHook:                     core.DevBuildHook(),
			DevBuildHookTimeoutMilliseconds:  core.DevBuildHookTimeoutMilliseconds(),
			ProdBuildHook:                    core.ProdBuildHook(),
			ProdBuildHookTimeoutMilliseconds: core.ProdBuildHookTimeoutMilliseconds(),
			MainAppEntry:                     mainAppEntry,
			PublicPathPrefix:                 core.PublicPathPrefix(),
			ServerOnlyMode:                   core.ServerOnlyMode(),
			SequentialGoBuild:                core.SequentialGoBuild(),
		},
	}
	rawConfigDocument.Core.StaticAssetDirs.Private = staticAssetsPrivateDirectory
	rawConfigDocument.Core.StaticAssetDirs.Public = staticAssetsPublicDirectory
	rawConfigDocument.Core.CSSEntryFiles.Critical = criticalCSSEntryFilePath
	rawConfigDocument.Core.CSSEntryFiles.NonCritical = nonCriticalCSSEntryFilePath

	if parsedConfig.Vite() != nil {
		vite := parsedConfig.Vite()
		packageManagerCommandDirectory, packageManagerCommandDirectoryError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
			resolveRoot,
			vite.JSPackageManagerCmdDir(),
		)
		if packageManagerCommandDirectoryError != nil {
			return nil, fmt.Errorf(
				"normalize Vite.JSPackageManagerCmdDir: %w",
				packageManagerCommandDirectoryError,
			)
		}
		viteConfigFilePath, viteConfigFilePathError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
			resolveRoot,
			vite.ViteConfigFile(),
		)
		if viteConfigFilePathError != nil {
			return nil, fmt.Errorf(
				"normalize Vite.ViteConfigFile: %w",
				viteConfigFilePathError,
			)
		}
		rawConfigDocument.Vite = &rawWaveViteConfigJSONSection{
			JSPackageManagerBaseCmd: vite.JSPackageManagerBaseCmd(),
			JSPackageManagerCmdDir:  packageManagerCommandDirectory,
			DefaultPort:             vite.DefaultPort(),
			ViteConfigFile:          viteConfigFilePath,
		}
	}

	if parsedConfig.Watch() != nil {
		watch := parsedConfig.Watch()
		rawWatchConfig := &rawWaveWatchConfigJSONSection{
			HealthcheckEndpoint:    watch.HealthcheckEndpoint(),
			HookStageFailurePolicy: watch.HookStageFailurePolicy(),
			Include:                watch.Include(),
		}
		rawWatchConfig.HookCommandTimeouts.PreCommandTimeoutMilliseconds = watch.PreCommandTimeoutMilliseconds()
		rawWatchConfig.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds = watch.ConcurrentCommandTimeoutMilliseconds()
		rawWatchConfig.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds = watch.ConcurrentNoWaitCommandTimeoutMilliseconds()
		rawWatchConfig.HookCommandTimeouts.PostCommandTimeoutMilliseconds = watch.PostCommandTimeoutMilliseconds()
		rawWatchConfig.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds = watch.PreCallbackTimeoutMilliseconds()
		rawWatchConfig.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds = watch.ConcurrentCallbackTimeoutMilliseconds()
		rawWatchConfig.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds = watch.ConcurrentNoWaitCallbackTimeoutMilliseconds()
		rawWatchConfig.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds = watch.PostCallbackTimeoutMilliseconds()
		rawWatchConfig.Exclude.Dirs = watch.ExcludeDirs()
		rawWatchConfig.Exclude.Files = watch.ExcludeFiles()

		for watchIncludeIndex := range rawWatchConfig.Include {
			normalizedPattern, patternError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
				resolveRoot,
				rawWatchConfig.Include[watchIncludeIndex].Pattern,
			)
			if patternError != nil {
				return nil, fmt.Errorf(
					"normalize Watch.Include[%d].Pattern: %w",
					watchIncludeIndex,
					patternError,
				)
			}
			rawWatchConfig.Include[watchIncludeIndex].Pattern = normalizedPattern
			for hookIndex := range rawWatchConfig.Include[watchIncludeIndex].OnChangeHooks {
				for excludedPatternIndex := range rawWatchConfig.Include[watchIncludeIndex].OnChangeHooks[hookIndex].Exclude {
					normalizedExcludedPattern, excludedPatternError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
						resolveRoot,
						rawWatchConfig.Include[watchIncludeIndex].OnChangeHooks[hookIndex].Exclude[excludedPatternIndex],
					)
					if excludedPatternError != nil {
						return nil, fmt.Errorf(
							"normalize Watch.Include[%d].OnChangeHooks[%d].Exclude[%d]: %w",
							watchIncludeIndex,
							hookIndex,
							excludedPatternIndex,
							excludedPatternError,
						)
					}
					rawWatchConfig.Include[watchIncludeIndex].OnChangeHooks[hookIndex].Exclude[excludedPatternIndex] = normalizedExcludedPattern
				}
			}
		}

		for excludedDirectoryPatternIndex := range rawWatchConfig.Exclude.Dirs {
			normalizedExcludedDirectoryPattern, excludedDirectoryPatternError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
				resolveRoot,
				rawWatchConfig.Exclude.Dirs[excludedDirectoryPatternIndex],
			)
			if excludedDirectoryPatternError != nil {
				return nil, fmt.Errorf(
					"normalize Watch.Exclude.Dirs[%d]: %w",
					excludedDirectoryPatternIndex,
					excludedDirectoryPatternError,
				)
			}
			rawWatchConfig.Exclude.Dirs[excludedDirectoryPatternIndex] = normalizedExcludedDirectoryPattern
		}
		for excludedFilePatternIndex := range rawWatchConfig.Exclude.Files {
			normalizedExcludedFilePattern, excludedFilePatternError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
				resolveRoot,
				rawWatchConfig.Exclude.Files[excludedFilePatternIndex],
			)
			if excludedFilePatternError != nil {
				return nil, fmt.Errorf(
					"normalize Watch.Exclude.Files[%d]: %w",
					excludedFilePatternIndex,
					excludedFilePatternError,
				)
			}
			rawWatchConfig.Exclude.Files[excludedFilePatternIndex] = normalizedExcludedFilePattern
		}

		rawConfigDocument.Watch = rawWatchConfig
	}

	return json.Marshal(rawConfigDocument)
}

func pathOrPatternRelativeToResolveRootForRawConfigJSON(
	resolveRoot string,
	pathOrPattern string,
) (string, error) {
	trimmedPathOrPattern := strings.TrimSpace(pathOrPattern)
	if trimmedPathOrPattern == "" {
		return "", nil
	}
	if !filepath.IsAbs(trimmedPathOrPattern) {
		return trimmedPathOrPattern, nil
	}

	trimmedResolveRoot := strings.TrimSpace(resolveRoot)
	if trimmedResolveRoot == "" {
		trimmedResolveRoot = "."
	}
	relativePathOrPattern, relativePathOrPatternError := filepath.Rel(
		trimmedResolveRoot,
		trimmedPathOrPattern,
	)
	if relativePathOrPatternError == nil {
		return filepath.Clean(relativePathOrPattern), nil
	}

	canonicalResolveRoot := waveenv.CanonicalizePathForLocationComparison(
		trimmedResolveRoot,
	)
	canonicalPathOrPattern := waveenv.CanonicalizePathForLocationComparison(
		trimmedPathOrPattern,
	)
	if canonicalResolveRoot != "" && canonicalPathOrPattern != "" {
		relativeCanonicalPathOrPattern, relativeCanonicalPathOrPatternError := filepath.Rel(
			canonicalResolveRoot,
			canonicalPathOrPattern,
		)
		if relativeCanonicalPathOrPatternError == nil {
			return filepath.Clean(relativeCanonicalPathOrPattern), nil
		}
	}

	return "", fmt.Errorf(
		"cannot represent absolute path %q relative to resolve root %q",
		trimmedPathOrPattern,
		trimmedResolveRoot,
	)
}

func mutateParsedConfigInPlace(
	cfg waveconfig.ParsedConfig,
	mutateRawConfig func(*rawWaveConfigJSONDocument),
) {
	if cfg == nil || cfg.Core() == nil {
		panic("wavetest.mutateParsedConfigInPlace requires non-nil cfg and cfg.Core")
	}
	if mutateRawConfig == nil {
		panic("wavetest.mutateParsedConfigInPlace requires non-nil mutateRawConfig")
	}

	rawConfigJSON, marshalError := MarshalParsedConfigToRawJSON(cfg)
	if marshalError != nil {
		panic(
			fmt.Sprintf(
				"wavetest.mutateParsedConfigInPlace: marshal parsed config to raw JSON: %v",
				marshalError,
			),
		)
	}

	var rawConfigDocument rawWaveConfigJSONDocument
	if unmarshalError := json.Unmarshal(rawConfigJSON, &rawConfigDocument); unmarshalError != nil {
		panic(
			fmt.Sprintf(
				"wavetest.mutateParsedConfigInPlace: unmarshal raw config JSON: %v",
				unmarshalError,
			),
		)
	}

	mutateRawConfig(&rawConfigDocument)
	// Reparse from a stable synthetic config path, then re-anchor parsed path
	// fields back to the current fixture root.
	rawConfigDocument.Core.ResolveRoot = "."

	mutatedRawConfigJSON, remarshalError := json.Marshal(rawConfigDocument)
	if remarshalError != nil {
		panic(
			fmt.Sprintf(
				"wavetest.mutateParsedConfigInPlace: remarshal raw config JSON: %v",
				remarshalError,
			),
		)
	}

	reparsedConfig, reparseError := waveconfig.ParseConfigJSONWithConfigPath(
		mutatedRawConfigJSON,
		"wave.config.json",
	)
	if reparseError != nil {
		panic(
			fmt.Sprintf(
				"wavetest.mutateParsedConfigInPlace: reparse mutated config: %v",
				reparseError,
			),
		)
	}
	parsedConfigRoot := filepath.Clean(cfg.ResolveRoot())
	if parsedConfigRoot != "" && filepath.IsAbs(parsedConfigRoot) {
		reanchorParsedConfigPathsToRoot(
			reparsedConfig,
			parsedConfigRoot,
		)
	}

	overwriteParsedConfigInPlace(cfg, reparsedConfig)
}

func overwriteParsedConfigInPlace(
	currentConfig waveconfig.ParsedConfig,
	replacementConfig waveconfig.ParsedConfig,
) {
	currentValue := reflect.ValueOf(currentConfig)
	replacementValue := reflect.ValueOf(replacementConfig)
	if !currentValue.IsValid() ||
		currentValue.Kind() != reflect.Pointer ||
		currentValue.IsNil() {
		panic("wavetest.overwriteParsedConfigInPlace requires pointer-backed current config")
	}
	if !replacementValue.IsValid() ||
		replacementValue.Kind() != reflect.Pointer ||
		replacementValue.IsNil() {
		panic("wavetest.overwriteParsedConfigInPlace requires pointer-backed replacement config")
	}
	if currentValue.Type() != replacementValue.Type() {
		panic(
			fmt.Sprintf(
				"wavetest.overwriteParsedConfigInPlace: type mismatch current=%s replacement=%s",
				currentValue.Type(),
				replacementValue.Type(),
			),
		)
	}
	currentElement := currentValue.Elem()
	if !currentElement.CanSet() {
		panic("wavetest.overwriteParsedConfigInPlace cannot set current config value")
	}
	currentElement.Set(replacementValue.Elem())
}

func ensureRawWatchConfig(rawConfigDocument *rawWaveConfigJSONDocument) *rawWaveWatchConfigJSONSection {
	if rawConfigDocument.Watch == nil {
		rawConfigDocument.Watch = &rawWaveWatchConfigJSONSection{}
	}
	return rawConfigDocument.Watch
}

func ensureRawViteConfig(rawConfigDocument *rawWaveConfigJSONDocument) *rawWaveViteConfigJSONSection {
	if rawConfigDocument.Vite == nil {
		rawConfigDocument.Vite = &rawWaveViteConfigJSONSection{}
	}
	return rawConfigDocument.Vite
}

func mustPathOrPatternRelativeToResolveRootForRawConfigMutation(
	cfg waveconfig.ParsedConfig,
	fieldPath string,
	pathOrPattern string,
) string {
	normalizedPathOrPattern, normalizedPathOrPatternError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
		cfg.ResolveRoot(),
		pathOrPattern,
	)
	if normalizedPathOrPatternError != nil {
		panic(
			fmt.Sprintf(
				"wavetest: normalize %s relative to resolve root: %v",
				fieldPath,
				normalizedPathOrPatternError,
			),
		)
	}
	return normalizedPathOrPattern
}

// SetCoreDevBuildHook mutates Core.DevBuildHook through parse/reparse.
func SetCoreDevBuildHook(cfg waveconfig.ParsedConfig, devBuildHook string) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.DevBuildHook = devBuildHook
	})
}

// SetCoreDevBuildHookTimeoutMilliseconds mutates
// Core.DevBuildHookTimeoutMilliseconds through parse/reparse.
func SetCoreDevBuildHookTimeoutMilliseconds(
	cfg waveconfig.ParsedConfig,
	timeoutMilliseconds int,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.DevBuildHookTimeoutMilliseconds = timeoutMilliseconds
	})
}

// SetCoreProdBuildHook mutates Core.ProdBuildHook through parse/reparse.
func SetCoreProdBuildHook(cfg waveconfig.ParsedConfig, prodBuildHook string) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.ProdBuildHook = prodBuildHook
	})
}

// SetCoreProdBuildHookTimeoutMilliseconds mutates
// Core.ProdBuildHookTimeoutMilliseconds through parse/reparse.
func SetCoreProdBuildHookTimeoutMilliseconds(
	cfg waveconfig.ParsedConfig,
	timeoutMilliseconds int,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.ProdBuildHookTimeoutMilliseconds = timeoutMilliseconds
	})
}

// SetCoreMainAppEntry mutates Core.MainAppEntry through parse/reparse.
func SetCoreMainAppEntry(cfg waveconfig.ParsedConfig, mainAppEntry string) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.MainAppEntry = mustPathOrPatternRelativeToResolveRootForRawConfigMutation(
			cfg,
			"Core.MainAppEntry",
			mainAppEntry,
		)
	})
}

// SetCoreStaticAssetDirsPrivate mutates Core.StaticAssetDirs.Private through
// parse/reparse.
func SetCoreStaticAssetDirsPrivate(
	cfg waveconfig.ParsedConfig,
	staticAssetsPrivateDirectory string,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.StaticAssetDirs.Private = mustPathOrPatternRelativeToResolveRootForRawConfigMutation(
			cfg,
			"Core.StaticAssetDirs.Private",
			staticAssetsPrivateDirectory,
		)
	})
}

// SetCoreStaticAssetDirsPublic mutates Core.StaticAssetDirs.Public through
// parse/reparse.
func SetCoreStaticAssetDirsPublic(
	cfg waveconfig.ParsedConfig,
	staticAssetsPublicDirectory string,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.StaticAssetDirs.Public = mustPathOrPatternRelativeToResolveRootForRawConfigMutation(
			cfg,
			"Core.StaticAssetDirs.Public",
			staticAssetsPublicDirectory,
		)
	})
}

// SetCoreCriticalCSSEntryFile mutates Core.CSSEntryFiles.Critical through
// parse/reparse.
func SetCoreCriticalCSSEntryFile(
	cfg waveconfig.ParsedConfig,
	criticalCSSEntryFile string,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.CSSEntryFiles.Critical = mustPathOrPatternRelativeToResolveRootForRawConfigMutation(
			cfg,
			"Core.CSSEntryFiles.Critical",
			criticalCSSEntryFile,
		)
	})
}

// SetCoreNonCriticalCSSEntryFile mutates Core.CSSEntryFiles.NonCritical
// through parse/reparse.
func SetCoreNonCriticalCSSEntryFile(
	cfg waveconfig.ParsedConfig,
	nonCriticalCSSEntryFile string,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.CSSEntryFiles.NonCritical = mustPathOrPatternRelativeToResolveRootForRawConfigMutation(
			cfg,
			"Core.CSSEntryFiles.NonCritical",
			nonCriticalCSSEntryFile,
		)
	})
}

// SetCorePublicPathPrefix mutates Core.PublicPathPrefix through parse/reparse.
func SetCorePublicPathPrefix(
	cfg waveconfig.ParsedConfig,
	publicPathPrefix string,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.PublicPathPrefix = publicPathPrefix
	})
}

// SetCoreServerOnlyMode mutates Core.ServerOnlyMode through parse/reparse.
func SetCoreServerOnlyMode(cfg waveconfig.ParsedConfig, serverOnlyMode bool) {
	if cfg == nil || cfg.Core() == nil {
		return
	}
	if cfg.Core().ServerOnlyMode() == serverOnlyMode {
		return
	}
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.ServerOnlyMode = serverOnlyMode
	})
}

// SetCoreSequentialGoBuild mutates Core.SequentialGoBuild through parse/reparse.
func SetCoreSequentialGoBuild(
	cfg waveconfig.ParsedConfig,
	sequentialGoBuild bool,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawConfigDocument.Core.SequentialGoBuild = sequentialGoBuild
	})
}

// SetViteJSPackageManagerBaseCmd mutates Vite.JSPackageManagerBaseCmd through
// parse/reparse.
func SetViteJSPackageManagerBaseCmd(
	cfg waveconfig.ParsedConfig,
	packageManagerBaseCommand string,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawViteConfig := ensureRawViteConfig(rawConfigDocument)
		rawViteConfig.JSPackageManagerBaseCmd = packageManagerBaseCommand
	})
}

// SetViteJSPackageManagerCmdDir mutates Vite.JSPackageManagerCmdDir through
// parse/reparse.
func SetViteJSPackageManagerCmdDir(
	cfg waveconfig.ParsedConfig,
	packageManagerCommandDirectory string,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawViteConfig := ensureRawViteConfig(rawConfigDocument)
		rawViteConfig.JSPackageManagerCmdDir = mustPathOrPatternRelativeToResolveRootForRawConfigMutation(
			cfg,
			"Vite.JSPackageManagerCmdDir",
			packageManagerCommandDirectory,
		)
	})
}

// SetViteDefaultPort mutates Vite.DefaultPort through parse/reparse.
func SetViteDefaultPort(cfg waveconfig.ParsedConfig, defaultPort int) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawViteConfig := ensureRawViteConfig(rawConfigDocument)
		rawViteConfig.DefaultPort = defaultPort
	})
}

// SetViteConfigFile mutates Vite.ViteConfigFile through parse/reparse.
func SetViteConfigFile(
	cfg waveconfig.ParsedConfig,
	viteConfigFilePath string,
) {
	normalizedViteConfigFilePath := viteConfigFilePath
	if cfg != nil &&
		cfg.Vite() != nil &&
		strings.TrimSpace(cfg.Vite().JSPackageManagerCmdDir()) != "" &&
		strings.TrimSpace(normalizedViteConfigFilePath) != "" &&
		!filepath.IsAbs(strings.TrimSpace(normalizedViteConfigFilePath)) {
		// In config JSON, ViteConfigFile is interpreted relative to
		// JSPackageManagerCmdDir when present in these test helpers so devserver
		// command construction observes the same path shape applications configure.
		normalizedViteConfigFilePath = filepath.Join(
			cfg.Vite().JSPackageManagerCmdDir(),
			normalizedViteConfigFilePath,
		)
	}
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawViteConfig := ensureRawViteConfig(rawConfigDocument)
		rawViteConfig.ViteConfigFile = mustPathOrPatternRelativeToResolveRootForRawConfigMutation(
			cfg,
			"Vite.ViteConfigFile",
			normalizedViteConfigFilePath,
		)
	})
}

// SetWatchHealthcheckEndpoint mutates Watch.HealthcheckEndpoint through
// parse/reparse.
func SetWatchHealthcheckEndpoint(
	cfg waveconfig.ParsedConfig,
	healthcheckEndpoint string,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.HealthcheckEndpoint = healthcheckEndpoint
	})
}

// SetWatchHookStageFailurePolicy mutates Watch.HookStageFailurePolicy through
// parse/reparse.
func SetWatchHookStageFailurePolicy(
	cfg waveconfig.ParsedConfig,
	hookStageFailurePolicy string,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.HookStageFailurePolicy = hookStageFailurePolicy
	})
}

// SetWatchPreCommandTimeoutMilliseconds mutates
// Watch.HookCommandTimeouts.PreCommandTimeoutMilliseconds through parse/reparse.
func SetWatchPreCommandTimeoutMilliseconds(
	cfg waveconfig.ParsedConfig,
	timeoutMilliseconds int,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.HookCommandTimeouts.PreCommandTimeoutMilliseconds = timeoutMilliseconds
	})
}

// SetWatchConcurrentCommandTimeoutMilliseconds mutates
// Watch.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds through
// parse/reparse.
func SetWatchConcurrentCommandTimeoutMilliseconds(
	cfg waveconfig.ParsedConfig,
	timeoutMilliseconds int,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds = timeoutMilliseconds
	})
}

// SetWatchConcurrentNoWaitCommandTimeoutMilliseconds mutates
// Watch.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds through
// parse/reparse.
func SetWatchConcurrentNoWaitCommandTimeoutMilliseconds(
	cfg waveconfig.ParsedConfig,
	timeoutMilliseconds int,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds = timeoutMilliseconds
	})
}

// SetWatchPostCommandTimeoutMilliseconds mutates
// Watch.HookCommandTimeouts.PostCommandTimeoutMilliseconds through
// parse/reparse.
func SetWatchPostCommandTimeoutMilliseconds(
	cfg waveconfig.ParsedConfig,
	timeoutMilliseconds int,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.HookCommandTimeouts.PostCommandTimeoutMilliseconds = timeoutMilliseconds
	})
}

// SetWatchPreCallbackTimeoutMilliseconds mutates
// Watch.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds through
// parse/reparse.
func SetWatchPreCallbackTimeoutMilliseconds(
	cfg waveconfig.ParsedConfig,
	timeoutMilliseconds int,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds = timeoutMilliseconds
	})
}

// SetWatchConcurrentCallbackTimeoutMilliseconds mutates
// Watch.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds through
// parse/reparse.
func SetWatchConcurrentCallbackTimeoutMilliseconds(
	cfg waveconfig.ParsedConfig,
	timeoutMilliseconds int,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds = timeoutMilliseconds
	})
}

// SetWatchConcurrentNoWaitCallbackTimeoutMilliseconds mutates
// Watch.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds
// through parse/reparse.
func SetWatchConcurrentNoWaitCallbackTimeoutMilliseconds(
	cfg waveconfig.ParsedConfig,
	timeoutMilliseconds int,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds = timeoutMilliseconds
	})
}

// SetWatchPostCallbackTimeoutMilliseconds mutates
// Watch.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds through
// parse/reparse.
func SetWatchPostCallbackTimeoutMilliseconds(
	cfg waveconfig.ParsedConfig,
	timeoutMilliseconds int,
) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds = timeoutMilliseconds
	})
}

// SetWatchInclude mutates Watch.Include through parse/reparse.
func SetWatchInclude(
	cfg waveconfig.ParsedConfig,
	include []wavewatch.WatchedFile,
) {
	preparedIncludeForRawConfig := make(
		[]wavewatch.WatchedFile,
		0,
		len(include),
	)
	resolveRoot := cfg.ResolveRoot()
	for watchedFileIndex, watchedFile := range include {
		normalizedPattern, normalizedPatternError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
			resolveRoot,
			watchedFile.Pattern,
		)
		if normalizedPatternError != nil {
			panic(
				fmt.Sprintf(
					"wavetest.SetWatchInclude: normalize include pattern at index %d: %v",
					watchedFileIndex,
					normalizedPatternError,
				),
			)
		}
		preparedWatchedFile := watchedFile
		preparedWatchedFile.Pattern = normalizedPattern
		preparedWatchedFile.SortedHooks = nil
		preparedWatchedFile.OnChangeHooks = append(
			[]wavewatch.OnChangeHook(nil),
			watchedFile.OnChangeHooks...,
		)
		for hookIndex := range preparedWatchedFile.OnChangeHooks {
			preparedWatchedFile.OnChangeHooks[hookIndex].Callback = nil
			preparedWatchedFile.OnChangeHooks[hookIndex].Exclude = append(
				[]string(nil),
				preparedWatchedFile.OnChangeHooks[hookIndex].Exclude...,
			)
			for excludedPatternIndex, excludedPattern := range preparedWatchedFile.OnChangeHooks[hookIndex].Exclude {
				normalizedExcludedPattern, normalizedExcludedPatternError := pathOrPatternRelativeToResolveRootForRawConfigJSON(
					resolveRoot,
					excludedPattern,
				)
				if normalizedExcludedPatternError != nil {
					panic(
						fmt.Sprintf(
							"wavetest.SetWatchInclude: normalize include[%d] hook[%d] exclude[%d]: %v",
							watchedFileIndex,
							hookIndex,
							excludedPatternIndex,
							normalizedExcludedPatternError,
						),
					)
				}
				preparedWatchedFile.OnChangeHooks[hookIndex].Exclude[excludedPatternIndex] = normalizedExcludedPattern
			}
		}
		preparedIncludeForRawConfig = append(
			preparedIncludeForRawConfig,
			preparedWatchedFile,
		)
	}

	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.Include = preparedIncludeForRawConfig
	})
	restoreWatchIncludeRuntimeCallbacks(cfg, include)
}

func restoreWatchIncludeRuntimeCallbacks(
	cfg waveconfig.ParsedConfig,
	includeWithRuntimeCallbacks []wavewatch.WatchedFile,
) {
	if cfg == nil || cfg.Watch() == nil {
		return
	}
	configValue := reflect.ValueOf(cfg)
	if !configValue.IsValid() ||
		configValue.Kind() != reflect.Pointer ||
		configValue.IsNil() {
		panic("wavetest.restoreWatchIncludeRuntimeCallbacks requires pointer-backed config")
	}

	configStruct := configValue.Elem()
	watchField := configStruct.FieldByName("watch")
	if !watchField.IsValid() || watchField.IsNil() {
		return
	}
	watchImplementation := watchField.Elem()
	if !watchImplementation.IsValid() ||
		watchImplementation.Kind() != reflect.Pointer ||
		watchImplementation.IsNil() {
		return
	}

	watchStruct := watchImplementation.Elem()
	includeField := watchStruct.FieldByName("include")
	if !includeField.IsValid() {
		panic("wavetest.restoreWatchIncludeRuntimeCallbacks missing watch include field")
	}
	if !includeField.CanAddr() {
		panic("wavetest.restoreWatchIncludeRuntimeCallbacks include field is not addressable")
	}
	includeFieldWritable := reflect.NewAt(
		includeField.Type(),
		unsafe.Pointer(includeField.UnsafeAddr()),
	).Elem()

	parsedInclude := append(
		[]wavewatch.WatchedFile(nil),
		includeFieldWritable.Interface().([]wavewatch.WatchedFile)...,
	)
	for watchedFileIndex := range parsedInclude {
		if watchedFileIndex >= len(includeWithRuntimeCallbacks) {
			break
		}
		if includeWithRuntimeCallbacks[watchedFileIndex].SortedHooks != nil {
			parsedInclude[watchedFileIndex].SortedHooks = includeWithRuntimeCallbacks[watchedFileIndex].SortedHooks
		}
		for hookIndex := range parsedInclude[watchedFileIndex].OnChangeHooks {
			if hookIndex >= len(includeWithRuntimeCallbacks[watchedFileIndex].OnChangeHooks) {
				break
			}
			parsedInclude[watchedFileIndex].OnChangeHooks[hookIndex].Callback =
				includeWithRuntimeCallbacks[watchedFileIndex].OnChangeHooks[hookIndex].Callback
		}
	}
	includeFieldWritable.Set(reflect.ValueOf(parsedInclude))
}

// SetWatchExcludeDirs mutates Watch.Exclude.Dirs through parse/reparse.
func SetWatchExcludeDirs(cfg waveconfig.ParsedConfig, excludeDirs []string) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.Exclude.Dirs = make([]string, 0, len(excludeDirs))
		for excludePatternIndex, excludePattern := range excludeDirs {
			rawWatchConfig.Exclude.Dirs = append(
				rawWatchConfig.Exclude.Dirs,
				mustPathOrPatternRelativeToResolveRootForRawConfigMutation(
					cfg,
					fmt.Sprintf("Watch.Exclude.Dirs[%d]", excludePatternIndex),
					excludePattern,
				),
			)
		}
	})
}

// SetWatchExcludeFiles mutates Watch.Exclude.Files through parse/reparse.
func SetWatchExcludeFiles(cfg waveconfig.ParsedConfig, excludeFiles []string) {
	mutateParsedConfigInPlace(cfg, func(rawConfigDocument *rawWaveConfigJSONDocument) {
		rawWatchConfig := ensureRawWatchConfig(rawConfigDocument)
		rawWatchConfig.Exclude.Files = make([]string, 0, len(excludeFiles))
		for excludePatternIndex, excludePattern := range excludeFiles {
			rawWatchConfig.Exclude.Files = append(
				rawWatchConfig.Exclude.Files,
				mustPathOrPatternRelativeToResolveRootForRawConfigMutation(
					cfg,
					fmt.Sprintf("Watch.Exclude.Files[%d]", excludePatternIndex),
					excludePattern,
				),
			)
		}
	})
}

// SetStaticAssetDirectories sets Core.StaticAssetDirs using explicit directory
// paths through parse/reparse.
func SetStaticAssetDirectories(
	cfg waveconfig.ParsedConfig,
	publicDir string,
	privateDir string,
) {
	SetCoreStaticAssetDirsPublic(cfg, filepath.Clean(publicDir))
	SetCoreStaticAssetDirsPrivate(cfg, filepath.Clean(privateDir))
}

// SetCSSEntryFiles sets Core.CSSEntryFiles using explicit critical and
// non-critical CSS entry paths through parse/reparse.
func SetCSSEntryFiles(
	cfg waveconfig.ParsedConfig,
	criticalEntry string,
	nonCriticalEntry string,
) {
	SetCoreCriticalCSSEntryFile(cfg, filepath.Clean(criticalEntry))
	SetCoreNonCriticalCSSEntryFile(cfg, filepath.Clean(nonCriticalEntry))
}

// MustCWDRelativePath converts one absolute filesystem path into a path
// relative to the current working directory. Relative inputs are cleaned and
// returned unchanged.
func MustCWDRelativePath(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return ""
	}
	if !filepath.IsAbs(trimmedPath) {
		return filepath.Clean(trimmedPath)
	}

	currentWorkingDirectory, currentWorkingDirectoryError := os.Getwd()
	if currentWorkingDirectoryError != nil {
		panic(
			fmt.Sprintf(
				"wavetest.MustCWDRelativePath: resolve current working directory: %v",
				currentWorkingDirectoryError,
			),
		)
	}

	relativePath, relativePathError := filepath.Rel(
		currentWorkingDirectory,
		trimmedPath,
	)
	if relativePathError != nil {
		panic(
			fmt.Sprintf(
				"wavetest.MustCWDRelativePath: make %q relative to cwd %q: %v",
				trimmedPath,
				currentWorkingDirectory,
				relativePathError,
			),
		)
	}

	return filepath.Clean(relativePath)
}

// NewWorkspaceTempDir allocates one per-test temporary directory in the
// operating-system temp area and registers cleanup on the test.
func NewWorkspaceTempDir(tb testing.TB, prefix string) string {
	tb.Helper()

	temporaryDirectoryPrefix := workspaceTempPrefixWithLocalIgnoreMarker(prefix)
	temporaryDirectory, temporaryDirectoryError := os.MkdirTemp(
		"",
		temporaryDirectoryPrefix,
	)
	if temporaryDirectoryError != nil {
		tb.Fatalf(
			"wavetest.NewWorkspaceTempDir: create temporary directory: %v",
			temporaryDirectoryError,
		)
	}
	tb.Cleanup(func() {
		_ = os.RemoveAll(temporaryDirectory)
	})

	return temporaryDirectory
}

func workspaceTempPrefixWithLocalIgnoreMarker(prefix string) string {
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		return "workspace.local.temp-"
	}
	if strings.Contains(trimmedPrefix, ".local.") {
		return trimmedPrefix
	}
	trimmedPrefix = strings.TrimSuffix(trimmedPrefix, "-")
	return trimmedPrefix + ".local.temp-"
}

// EnsureViteConfig allocates cfg.Vite through parse/reparse when tests require
// a non-nil Vite section and the fixture omitted it.
func EnsureViteConfig(
	tb testing.TB,
	cfg waveconfig.ParsedConfig,
) {
	tb.Helper()
	if cfg == nil {
		tb.Fatal("expected non-nil config")
		return
	}
	if cfg.Vite() != nil {
		return
	}
	SetViteJSPackageManagerBaseCmd(cfg, "pnpm")
}
