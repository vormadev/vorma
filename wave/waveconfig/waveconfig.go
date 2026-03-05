// Package waveconfig provides JSON parsing with filesystem-path invariants
// that can be applied to typed configuration structs.
package waveconfig

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
	"github.com/vormadev/vorma/wave/waveartifacts"
	"github.com/vormadev/vorma/wave/waveenv"
	"github.com/vormadev/vorma/wave/wavewatch"
)

// filesystemPathField identifies one config field containing a filesystem path.
type filesystemPathField struct {
	FieldPath      string
	ConfiguredPath string
}

type untypedOptions struct {
	ValidateRequiredSections func(reflect.Value) error
	FilesystemPathSelectors  []string
	PanicMachineAbsolutePath func(filesystemPathField)
}

var waveFilesystemPathSelectors = []string{
	"Core.ResolveRoot",
	"Core.MainAppEntry",
	"Core.StaticAssetDirs.Private",
	"Core.StaticAssetDirs.Public",
	"Core.CSSEntryFiles.Critical",
	"Core.CSSEntryFiles.NonCritical",
	"Vite.JSPackageManagerCmdDir",
	"Vite.ViteConfigFile",
	"Watch.Exclude.Dirs.*",
	"Watch.Exclude.Files.*",
	"Watch.Include.*.Pattern",
	"Watch.Include.*.OnChangeHooks.*.Exclude.*",
}

var waveConfigDirectoryRelativeFilesystemPathSelectors = []string{
	"Core.MainAppEntry",
	"Core.StaticAssetDirs.Private",
	"Core.StaticAssetDirs.Public",
	"Core.CSSEntryFiles.Critical",
	"Core.CSSEntryFiles.NonCritical",
	"Vite.JSPackageManagerCmdDir",
	"Vite.ViteConfigFile",
	"Watch.Exclude.Dirs.*",
	"Watch.Exclude.Files.*",
	"Watch.Include.*.Pattern",
	"Watch.Include.*.OnChangeHooks.*.Exclude.*",
}

// parseWaveConfigJSON unmarshals one Wave config JSON payload and enforces
// Wave's machine-absolute filesystem-path invariants.
func parseWaveConfigJSON(
	data []byte,
	configPath string,
) (ParsedConfig, error) {
	trimmedConfigPath := strings.TrimSpace(configPath)
	if trimmedConfigPath == "" {
		return nil, fmt.Errorf("config path is required")
	}
	// configPathRelativeToPathBasis is always relative to the caller-selected
	// config-path basis passed to ParseConfigJSONWithConfigPath. In wave.New,
	// that basis is the deterministic CWD discovery result for ConfigPath +
	// Core.ProjectID.
	configPathRelativeToPathBasis := filepath.Clean(trimmedConfigPath)
	configFileDirectoryRelativeToPathBasis := filepath.Dir(
		configPathRelativeToPathBasis,
	)

	var parsedConfigModel parsedConfigWireModel
	parseError := parseJSONWithFilesystemPathRulesIntoTarget(
		data,
		&parsedConfigModel,
		untypedOptions{
			ValidateRequiredSections: validateWaveConfigRequiredSections,
			FilesystemPathSelectors:  waveFilesystemPathSelectors,
			PanicMachineAbsolutePath: panicMachineAbsoluteWaveConfigPath,
		},
	)
	if parseError != nil {
		return nil, parseError
	}
	normalizedSemanticConfigJSON, normalizeError := normalizeConfigJSONForSemanticComparison(
		data,
	)
	if normalizeError != nil {
		return nil, normalizeError
	}
	resolveRoot := "."
	if parsedConfigModel.Core != nil &&
		strings.TrimSpace(parsedConfigModel.Core.ResolveRoot) != "" {
		resolveRoot = strings.TrimSpace(parsedConfigModel.Core.ResolveRoot)
	}
	effectiveResolveRootRelativeToPathBasis := filepath.Clean(
		filepath.Join(configFileDirectoryRelativeToPathBasis, resolveRoot),
	)

	if pathResolutionError := resolveFilesystemPathFieldsRelativeToBasePath(
		reflect.ValueOf(&parsedConfigModel).Elem(),
		waveConfigDirectoryRelativeFilesystemPathSelectors,
		effectiveResolveRootRelativeToPathBasis,
	); pathResolutionError != nil {
		return nil, pathResolutionError
	}
	if finalizeError := finalizeWaveDistLayout(
		reflect.ValueOf(&parsedConfigModel).Elem(),
		configFileDirectoryRelativeToPathBasis,
	); finalizeError != nil {
		return nil, finalizeError
	}

	return &parsedConfig{
		core:                newCoreConfigFromWire(parsedConfigModel.Core),
		vite:                newViteConfigFromWire(parsedConfigModel.Vite),
		watch:               newWatchConfigFromWire(parsedConfigModel.Watch),
		dist:                newDistLayoutFromWire(parsedConfigModel.Dist),
		configFileDirectory: effectiveResolveRootRelativeToPathBasis,
		semanticConfigJSON:  normalizedSemanticConfigJSON,
	}, nil
}

func normalizeConfigJSONForSemanticComparison(data []byte) ([]byte, error) {
	var parsedJSONValue any
	if unmarshalError := json.Unmarshal(data, &parsedJSONValue); unmarshalError != nil {
		return nil, fmt.Errorf("parse config: %w", unmarshalError)
	}
	normalizedConfigJSON, marshalError := json.Marshal(parsedJSONValue)
	if marshalError != nil {
		return nil, fmt.Errorf("normalize config for semantic comparison: %w", marshalError)
	}
	return normalizedConfigJSON, nil
}

func parseJSONWithFilesystemPathRulesIntoTarget(
	data []byte,
	target any,
	options untypedOptions,
) error {
	targetValue, targetValueError := parseTargetValue(target)
	if targetValueError != nil {
		return targetValueError
	}
	if unmarshalError := json.Unmarshal(data, target); unmarshalError != nil {
		return fmt.Errorf("parse config: %w", unmarshalError)
	}

	if options.ValidateRequiredSections != nil {
		if validateError := options.ValidateRequiredSections(targetValue); validateError != nil {
			return validateError
		}
	}

	filesystemPathFields := collectFilesystemPathFieldsFromSelectors(
		targetValue,
		options.FilesystemPathSelectors,
	)
	for _, configuredFilesystemPathField := range filesystemPathFields {
		trimmedConfiguredPath := strings.TrimSpace(
			configuredFilesystemPathField.ConfiguredPath,
		)
		if trimmedConfiguredPath == "" {
			continue
		}
		if !waveenv.IsMachineAbsoluteFilesystemPath(trimmedConfiguredPath) {
			continue
		}

		machineAbsolutePathField := filesystemPathField{
			FieldPath:      configuredFilesystemPathField.FieldPath,
			ConfiguredPath: trimmedConfiguredPath,
		}
		if options.PanicMachineAbsolutePath == nil {
			panic(
				fmt.Sprintf(
					"config: %s must not be a machine-absolute filesystem path: %q",
					machineAbsolutePathField.FieldPath,
					machineAbsolutePathField.ConfiguredPath,
				),
			)
		}
		options.PanicMachineAbsolutePath(machineAbsolutePathField)
	}
	return nil
}

func parseTargetValue(target any) (reflect.Value, error) {
	if target == nil {
		return reflect.Value{}, fmt.Errorf("config parse target is nil")
	}
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return reflect.Value{}, fmt.Errorf(
			"config parse target must be a non-nil pointer",
		)
	}
	resolvedTargetValue := dereferenceValueForSelectorTraversal(targetValue)
	if !resolvedTargetValue.IsValid() {
		return reflect.Value{}, fmt.Errorf("config parse target is invalid")
	}
	if resolvedTargetValue.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf(
			"config parse target must point to a struct",
		)
	}
	return resolvedTargetValue, nil
}

func validateWaveConfigRequiredSections(parsedValue reflect.Value) error {
	coreField := parsedValue.FieldByName("Core")
	if !coreField.IsValid() {
		return fmt.Errorf("config: Core section is required")
	}
	resolvedCoreField := dereferenceValueForSelectorTraversal(coreField)
	if !resolvedCoreField.IsValid() {
		return fmt.Errorf("config: Core section is required")
	}
	return nil
}

func panicMachineAbsoluteWaveConfigPath(
	filesystemPathField filesystemPathField,
) {
	fieldSemanticsMessage := waveConfigFieldSemanticsMessage(
		filesystemPathField.FieldPath,
	)
	panic(fmt.Sprintf(
		"config: %s must not be a machine-absolute filesystem path: %q. "+
			"%s "+
			"A leading slash path like \"/dist\" is machine-absolute.",
		filesystemPathField.FieldPath,
		filesystemPathField.ConfiguredPath,
		fieldSemanticsMessage,
	))
}

func waveConfigFieldSemanticsMessage(fieldPath string) string {
	switch {
	case fieldPath == "Core.ProjectID":
		return "Core.ProjectID must be a non-empty identifier string."
	case fieldPath == "Core.MainAppEntry":
		return "Core.MainAppEntry must be relative to your JSON config file."
	case fieldPath == "Core.StaticAssetDirs.Private":
		return "Core.StaticAssetDirs.Private must be relative to your JSON config file."
	case fieldPath == "Core.StaticAssetDirs.Public":
		return "Core.StaticAssetDirs.Public must be relative to your JSON config file."
	case fieldPath == "Core.CSSEntryFiles.Critical":
		return "Core.CSSEntryFiles.Critical must be relative to your JSON config file."
	case fieldPath == "Core.CSSEntryFiles.NonCritical":
		return "Core.CSSEntryFiles.NonCritical must be relative to your JSON config file."
	case fieldPath == "Vite.JSPackageManagerCmdDir":
		return "Vite.JSPackageManagerCmdDir must be relative to your JSON config file."
	case fieldPath == "Vite.ViteConfigFile":
		return "Vite.ViteConfigFile must be relative to your JSON config file."
	case strings.HasPrefix(fieldPath, "Watch.Exclude.Dirs["):
		return "Watch.Exclude.Dirs entries must be relative to your JSON config file."
	case strings.HasPrefix(fieldPath, "Watch.Exclude.Files["):
		return "Watch.Exclude.Files entries must be relative to your JSON config file."
	case strings.HasPrefix(fieldPath, "Watch.Include[") &&
		strings.Contains(fieldPath, "].Pattern"):
		return "Watch.Include[].Pattern entries must be relative to your JSON config file."
	case strings.HasPrefix(fieldPath, "Watch.Include[") &&
		strings.Contains(fieldPath, ".OnChangeHooks[") &&
		strings.Contains(fieldPath, "].Exclude["):
		return "Watch.Include[].OnChangeHooks[].Exclude entries must be relative to your JSON config file."
	default:
		return "Use a non-absolute path according to this field's semantics (relative to your JSON config file, relative to your parser config-path basis, or to the owning path field)."
	}
}

func resolveFilesystemPathFieldsRelativeToBasePath(
	rootValue reflect.Value,
	filesystemPathSelectors []string,
	basePath string,
) error {
	trimmedBasePath := strings.TrimSpace(basePath)
	if trimmedBasePath == "" {
		return fmt.Errorf(
			"base path is required for path resolution relative to your JSON config file",
		)
	}

	for _, filesystemPathSelector := range filesystemPathSelectors {
		selectorSegments := parseFilesystemPathSelectorSegments(
			filesystemPathSelector,
		)
		rewriteError := rewriteFilesystemPathFieldsForSelector(
			rootValue,
			selectorSegments,
			"",
			func(fieldPath string, configuredPath string) (string, error) {
				return resolvePathRelativeToBaseDirectory(
					trimmedBasePath,
					configuredPath,
				)
			},
		)
		if rewriteError != nil {
			return fmt.Errorf(
				"resolve %s relative to base path %q: %w",
				filesystemPathSelector,
				trimmedBasePath,
				rewriteError,
			)
		}
	}

	return nil
}

func rewriteFilesystemPathFieldsForSelector(
	currentValue reflect.Value,
	remainingSelectorSegments []string,
	fieldPathPrefix string,
	rewritePath func(fieldPath string, configuredPath string) (string, error),
) error {
	resolvedCurrentValue := dereferenceValueForSelectorTraversal(currentValue)
	if !resolvedCurrentValue.IsValid() {
		return nil
	}

	if len(remainingSelectorSegments) == 0 {
		if resolvedCurrentValue.Kind() != reflect.String {
			return nil
		}
		if !resolvedCurrentValue.CanSet() {
			return nil
		}
		rewrittenPath, rewriteError := rewritePath(
			fieldPathPrefix,
			resolvedCurrentValue.String(),
		)
		if rewriteError != nil {
			return rewriteError
		}
		resolvedCurrentValue.SetString(rewrittenPath)
		return nil
	}

	currentSelectorSegment := remainingSelectorSegments[0]
	nextSelectorSegments := remainingSelectorSegments[1:]
	if currentSelectorSegment == "*" {
		if resolvedCurrentValue.Kind() != reflect.Slice &&
			resolvedCurrentValue.Kind() != reflect.Array {
			return nil
		}
		for elementIndex := 0; elementIndex < resolvedCurrentValue.Len(); elementIndex++ {
			nextFieldPathPrefix := fmt.Sprintf(
				"%s[%d]",
				fieldPathPrefix,
				elementIndex,
			)
			rewriteError := rewriteFilesystemPathFieldsForSelector(
				resolvedCurrentValue.Index(elementIndex),
				nextSelectorSegments,
				nextFieldPathPrefix,
				rewritePath,
			)
			if rewriteError != nil {
				return rewriteError
			}
		}
		return nil
	}

	if resolvedCurrentValue.Kind() != reflect.Struct {
		return nil
	}
	nextValue := resolvedCurrentValue.FieldByName(currentSelectorSegment)
	if !nextValue.IsValid() {
		return nil
	}

	nextFieldPathPrefix := currentSelectorSegment
	if fieldPathPrefix != "" {
		nextFieldPathPrefix = fieldPathPrefix + "." + currentSelectorSegment
	}
	return rewriteFilesystemPathFieldsForSelector(
		nextValue,
		nextSelectorSegments,
		nextFieldPathPrefix,
		rewritePath,
	)
}

func resolvePathRelativeToBaseDirectory(
	basePath string,
	configuredPath string,
) (string, error) {
	trimmedConfiguredPath := strings.TrimSpace(configuredPath)
	if trimmedConfiguredPath == "" {
		return "", nil
	}
	if waveenv.IsMachineAbsoluteFilesystemPath(trimmedConfiguredPath) {
		return "", fmt.Errorf(
			"machine-absolute filesystem paths are invalid: %q",
			trimmedConfiguredPath,
		)
	}
	return filepath.Clean(
		filepath.Join(basePath, trimmedConfiguredPath),
	), nil
}

func finalizeWaveDistLayout(
	parsedValue reflect.Value,
	configFileDirectoryRelativeToPathBasis string,
) error {
	coreField := parsedValue.FieldByName("Core")
	if !coreField.IsValid() {
		return fmt.Errorf("config: Core section is required")
	}
	resolvedCoreField := dereferenceValueForSelectorTraversal(coreField)
	if !resolvedCoreField.IsValid() ||
		resolvedCoreField.Kind() != reflect.Struct {
		return fmt.Errorf("config: Core section is required")
	}
	projectIDField := resolvedCoreField.FieldByName("ProjectID")
	if !projectIDField.IsValid() || projectIDField.Kind() != reflect.String {
		return fmt.Errorf("config: Core.ProjectID is required")
	}
	projectID := strings.TrimSpace(projectIDField.String())
	if projectID == "" {
		return fmt.Errorf("config: Core.ProjectID is required")
	}
	if projectIDField.CanSet() {
		projectIDField.SetString(projectID)
	}
	mainAppEntryField := resolvedCoreField.FieldByName("MainAppEntry")
	if !mainAppEntryField.IsValid() || mainAppEntryField.Kind() != reflect.String {
		return fmt.Errorf("config: Core.MainAppEntry is required")
	}
	mainAppEntry := strings.TrimSpace(mainAppEntryField.String())
	if mainAppEntry == "" {
		return fmt.Errorf("config: Core.MainAppEntry is required")
	}
	if mainAppEntryField.CanSet() {
		mainAppEntryField.SetString(mainAppEntry)
	}

	distField := parsedValue.FieldByName("Dist")
	if !distField.IsValid() {
		return fmt.Errorf("config parser target is missing Dist field")
	}
	resolvedDistField := dereferenceValueForSelectorTraversal(distField)
	if !resolvedDistField.IsValid() ||
		resolvedDistField.Kind() != reflect.Struct {
		return fmt.Errorf("config parser target Dist field is invalid")
	}
	rootField := resolvedDistField.FieldByName("Root")
	if !rootField.IsValid() || rootField.Kind() != reflect.String {
		return fmt.Errorf("config parser target is missing Dist.Root field")
	}
	if !rootField.CanSet() {
		return fmt.Errorf(
			"config parser target Dist.Root field is not settable",
		)
	}
	distRootRelativeToPathBasis := filepath.Clean(
		filepath.Join(
			configFileDirectoryRelativeToPathBasis,
			reservedWaveDistDirectoryName,
		),
	)
	if strings.TrimSpace(distRootRelativeToPathBasis) == "" {
		return fmt.Errorf("config: derived dist root is empty")
	}
	normalizedDistRootRelativeToPathBasis := filepath.ToSlash(
		distRootRelativeToPathBasis,
	)
	if strings.HasPrefix(normalizedDistRootRelativeToPathBasis, "/") {
		return fmt.Errorf(
			"config: derived dist root must stay relative to parser config-path basis: %q",
			distRootRelativeToPathBasis,
		)
	}
	rootField.SetString(filepath.Clean(filepath.FromSlash(normalizedDistRootRelativeToPathBasis)))
	return nil
}

func collectFilesystemPathFieldsFromSelectors(
	rootValue reflect.Value,
	filesystemPathSelectors []string,
) []filesystemPathField {
	filesystemPathFields := make(
		[]filesystemPathField,
		0,
		len(filesystemPathSelectors),
	)
	for _, filesystemPathSelector := range filesystemPathSelectors {
		selectorSegments := parseFilesystemPathSelectorSegments(
			filesystemPathSelector,
		)
		selectedFilesystemPathFields := collectFilesystemPathFieldsForSelector(
			rootValue,
			selectorSegments,
			"",
		)
		filesystemPathFields = append(
			filesystemPathFields,
			selectedFilesystemPathFields...,
		)
	}
	return filesystemPathFields
}

func parseFilesystemPathSelectorSegments(
	filesystemPathSelector string,
) []string {
	trimmedSelector := strings.TrimSpace(filesystemPathSelector)
	if trimmedSelector == "" {
		panic("filesystem path selector is empty")
	}
	selectorSegments := strings.Split(trimmedSelector, ".")
	for selectorSegmentIndex, selectorSegment := range selectorSegments {
		if strings.TrimSpace(selectorSegment) == "" {
			panic(
				fmt.Sprintf(
					"filesystem path selector %q contains empty segment at index %d",
					filesystemPathSelector,
					selectorSegmentIndex,
				),
			)
		}
	}
	return selectorSegments
}

func collectFilesystemPathFieldsForSelector(
	currentValue reflect.Value,
	remainingSelectorSegments []string,
	fieldPathPrefix string,
) []filesystemPathField {
	resolvedCurrentValue := dereferenceValueForSelectorTraversal(currentValue)
	if !resolvedCurrentValue.IsValid() {
		return nil
	}

	if len(remainingSelectorSegments) == 0 {
		if resolvedCurrentValue.Kind() != reflect.String {
			return nil
		}
		return []filesystemPathField{
			{
				FieldPath:      fieldPathPrefix,
				ConfiguredPath: resolvedCurrentValue.String(),
			},
		}
	}

	currentSelectorSegment := remainingSelectorSegments[0]
	nextSelectorSegments := remainingSelectorSegments[1:]
	if currentSelectorSegment == "*" {
		if resolvedCurrentValue.Kind() != reflect.Slice &&
			resolvedCurrentValue.Kind() != reflect.Array {
			return nil
		}
		filesystemPathFields := make(
			[]filesystemPathField,
			0,
			resolvedCurrentValue.Len(),
		)
		for elementIndex := 0; elementIndex < resolvedCurrentValue.Len(); elementIndex++ {
			nextFieldPathPrefix := fmt.Sprintf(
				"%s[%d]",
				fieldPathPrefix,
				elementIndex,
			)
			selectedFilesystemPathFields := collectFilesystemPathFieldsForSelector(
				resolvedCurrentValue.Index(elementIndex),
				nextSelectorSegments,
				nextFieldPathPrefix,
			)
			filesystemPathFields = append(
				filesystemPathFields,
				selectedFilesystemPathFields...,
			)
		}
		return filesystemPathFields
	}

	if resolvedCurrentValue.Kind() != reflect.Struct {
		return nil
	}
	nextValue := resolvedCurrentValue.FieldByName(currentSelectorSegment)
	if !nextValue.IsValid() {
		return nil
	}

	nextFieldPathPrefix := currentSelectorSegment
	if fieldPathPrefix != "" {
		nextFieldPathPrefix = fieldPathPrefix + "." + currentSelectorSegment
	}
	return collectFilesystemPathFieldsForSelector(
		nextValue,
		nextSelectorSegments,
		nextFieldPathPrefix,
	)
}

func dereferenceValueForSelectorTraversal(
	currentValue reflect.Value,
) reflect.Value {
	resolvedValue := currentValue
	for resolvedValue.IsValid() &&
		(resolvedValue.Kind() == reflect.Pointer ||
			resolvedValue.Kind() == reflect.Interface) {
		if resolvedValue.IsNil() {
			return reflect.Value{}
		}
		resolvedValue = resolvedValue.Elem()
	}
	return resolvedValue
}

const (
	reservedWaveDistDirectoryName = ".wavedist"
	segStatic                     = "static"
	segAssets                     = waveartifacts.AssetsDirname
	segPublic                     = waveartifacts.PublicDirname
	segPrivate                    = waveartifacts.PrivateDirname
	segInternal                   = waveartifacts.InternalDirname
)

const (
	fileBinary            = "main"
	fileBinaryWindows     = "main.exe"
	fileKeep              = ".keep"
	fileCriticalCSS       = waveartifacts.CriticalCSSFileName
	fileNormalCSSRef      = waveartifacts.NormalCSSRefFileName
	filePublicMapRef      = waveartifacts.PublicFileMapRefFileName
	filePublicMapGob      = waveartifacts.PublicFileMapGobFileName
	filePrivateMapGob     = waveartifacts.PrivateFileMapGobFileName
	fileBuildOutputLedger = waveartifacts.BuildOutputLedgerFileName
)

// DistLayout provides parsed dist layout paths normalized to the parser
// config-path basis passed to ParseConfigJSONWithConfigPath.
type DistLayout interface {
	// Root returns the normalized dist root path relative to parser
	// config-path basis.
	Root() string
	// Binary returns the dist-relative path to the compiled app binary.
	Binary() string
	// Static returns the dist-relative static output root.
	Static() string
	// StaticPublic returns the dist-relative public static asset directory.
	StaticPublic() string
	// StaticPrivate returns the dist-relative private static asset directory.
	StaticPrivate() string
	// Internal returns the dist-relative internal artifact directory.
	Internal() string
	// CriticalCSS returns the dist-relative critical CSS artifact path.
	CriticalCSS() string
	// NormalCSSRef returns the dist-relative normal CSS reference artifact path.
	NormalCSSRef() string
	// PublicFileMapRef returns the dist-relative public file-map reference path.
	PublicFileMapRef() string
	// PublicFileMapGob returns the dist-relative public file-map gob path.
	PublicFileMapGob() string
	// PrivateFileMapGob returns the dist-relative private file-map gob path.
	PrivateFileMapGob() string
	// BuildOutputLedger returns the dist-relative build-output ledger path.
	BuildOutputLedger() string
	// KeepFile returns the dist-relative static keep-file path.
	KeepFile() string
	// Clone returns a defensive copy of this parsed dist layout.
	Clone() DistLayout

	// waveDistLayoutSeal prevents external implementations of DistLayout.
	waveDistLayoutSeal()
}

type distLayout struct {
	// root is always derived by parser as a config-sibling `.wavedist`
	// directory path relative to parser config-path basis. It is never
	// machine-absolute and never rebased under ResolveRoot.
	root string
}

func (layout *distLayout) waveDistLayoutSeal() {}

func (layout *distLayout) Root() string {
	if layout == nil {
		return ""
	}
	return layout.root
}

func (layout *distLayout) Binary() string {
	name := fileBinary
	if runtime.GOOS == "windows" {
		name = fileBinaryWindows
	}
	return filepath.Join(layout.Root(), name)
}

func (layout *distLayout) Static() string {
	return filepath.Join(layout.Root(), segStatic)
}

func (layout *distLayout) StaticPublic() string {
	return filepath.Join(layout.Static(), segAssets, segPublic)
}

func (layout *distLayout) StaticPrivate() string {
	return filepath.Join(layout.Static(), segAssets, segPrivate)
}

func (layout *distLayout) Internal() string {
	return filepath.Join(layout.Static(), segInternal)
}

func (layout *distLayout) CriticalCSS() string {
	return filepath.Join(layout.Internal(), fileCriticalCSS)
}

func (layout *distLayout) NormalCSSRef() string {
	return filepath.Join(layout.Internal(), fileNormalCSSRef)
}

func (layout *distLayout) PublicFileMapRef() string {
	return filepath.Join(layout.Internal(), filePublicMapRef)
}

func (layout *distLayout) PublicFileMapGob() string {
	return filepath.Join(layout.Internal(), filePublicMapGob)
}

func (layout *distLayout) PrivateFileMapGob() string {
	return filepath.Join(layout.Internal(), filePrivateMapGob)
}

func (layout *distLayout) BuildOutputLedger() string {
	return filepath.Join(layout.Internal(), fileBuildOutputLedger)
}

func (layout *distLayout) KeepFile() string {
	return filepath.Join(layout.Static(), fileKeep)
}

func (layout *distLayout) Clone() DistLayout {
	if layout == nil {
		return nil
	}
	return &distLayout{
		root: layout.root,
	}
}

// CoreConfig is the parsed, normalized Core config section.
type CoreConfig interface {
	// ProjectID returns Core.ProjectID exactly as configured (trimmed).
	ProjectID() string
	// ConfiguredResolveRoot returns Core.ResolveRoot exactly as configured in
	// JSON (unresolved), or empty when not set.
	ConfiguredResolveRoot() string
	// DevBuildHook returns Core.DevBuildHook exactly as configured (command
	// string; not path-resolved).
	DevBuildHook() string
	// DevBuildHookTimeoutMilliseconds returns the parsed Core
	// DevBuildHookTimeoutMilliseconds value.
	DevBuildHookTimeoutMilliseconds() int
	// ProdBuildHook returns Core.ProdBuildHook exactly as configured (command
	// string; not path-resolved).
	ProdBuildHook() string
	// ProdBuildHookTimeoutMilliseconds returns the parsed Core
	// ProdBuildHookTimeoutMilliseconds value.
	ProdBuildHookTimeoutMilliseconds() int
	// MainAppEntry is normalized relative to effective ResolveRoot (which itself
	// is relative to parser config-path basis).
	MainAppEntry() string
	// StaticAssetDirsPrivate is normalized relative to effective ResolveRoot.
	StaticAssetDirsPrivate() string
	// StaticAssetDirsPublic is normalized relative to effective ResolveRoot.
	StaticAssetDirsPublic() string
	// CriticalCSSEntryFile is normalized relative to effective ResolveRoot.
	CriticalCSSEntryFile() string
	// NonCriticalCSSEntryFile is normalized relative to effective ResolveRoot.
	NonCriticalCSSEntryFile() string
	// PublicPathPrefix returns Core.PublicPathPrefix exactly as configured.
	PublicPathPrefix() string
	// ServerOnlyMode returns the parsed Core.ServerOnlyMode value.
	ServerOnlyMode() bool
	// SequentialGoBuild returns the parsed Core.SequentialGoBuild value.
	SequentialGoBuild() bool
	// Clone returns a defensive copy of this parsed Core config.
	Clone() CoreConfig

	// waveCoreConfigSeal prevents external implementations of CoreConfig.
	waveCoreConfigSeal()
}

type coreConfig struct {
	// projectID is Core.ProjectID exactly as configured (trimmed), not path-resolved.
	projectID string
	// configuredResolveRoot is Core.ResolveRoot exactly as configured (trimmed),
	// before defaulting or path resolution.
	configuredResolveRoot string
	// devBuildHook is Core.DevBuildHook exactly as configured.
	devBuildHook string
	// devBuildHookTimeoutMilliseconds is the parsed Core.DevBuildHookTimeoutMilliseconds value.
	devBuildHookTimeoutMilliseconds int
	// prodBuildHook is Core.ProdBuildHook exactly as configured.
	prodBuildHook string
	// prodBuildHookTimeoutMilliseconds is the parsed Core.ProdBuildHookTimeoutMilliseconds value.
	prodBuildHookTimeoutMilliseconds int
	// mainAppEntry is Core.MainAppEntry resolved relative to effective ResolveRoot.
	mainAppEntry string
	// staticAssetDirsPrivate is Core.StaticAssetDirs.Private resolved relative to effective ResolveRoot.
	staticAssetDirsPrivate string
	// staticAssetDirsPublic is Core.StaticAssetDirs.Public resolved relative to effective ResolveRoot.
	staticAssetDirsPublic string
	// criticalCSSEntryFile is Core.CSSEntryFiles.Critical resolved relative to effective ResolveRoot.
	criticalCSSEntryFile string
	// nonCriticalCSSEntryFile is Core.CSSEntryFiles.NonCritical resolved relative to effective ResolveRoot.
	nonCriticalCSSEntryFile string
	// publicPathPrefix is Core.PublicPathPrefix exactly as configured.
	publicPathPrefix string
	// serverOnlyMode is the parsed Core.ServerOnlyMode value.
	serverOnlyMode bool
	// sequentialGoBuild is the parsed Core.SequentialGoBuild value.
	sequentialGoBuild bool
}

func (coreConfig *coreConfig) waveCoreConfigSeal() {}

func (coreConfig *coreConfig) ProjectID() string {
	if coreConfig == nil {
		return ""
	}
	return coreConfig.projectID
}

func (coreConfig *coreConfig) ConfiguredResolveRoot() string {
	if coreConfig == nil {
		return ""
	}
	return coreConfig.configuredResolveRoot
}

func (coreConfig *coreConfig) DevBuildHook() string {
	if coreConfig == nil {
		return ""
	}
	return coreConfig.devBuildHook
}

func (coreConfig *coreConfig) DevBuildHookTimeoutMilliseconds() int {
	if coreConfig == nil {
		return 0
	}
	return coreConfig.devBuildHookTimeoutMilliseconds
}

func (coreConfig *coreConfig) ProdBuildHook() string {
	if coreConfig == nil {
		return ""
	}
	return coreConfig.prodBuildHook
}

func (coreConfig *coreConfig) ProdBuildHookTimeoutMilliseconds() int {
	if coreConfig == nil {
		return 0
	}
	return coreConfig.prodBuildHookTimeoutMilliseconds
}

func (coreConfig *coreConfig) MainAppEntry() string {
	if coreConfig == nil {
		return ""
	}
	return coreConfig.mainAppEntry
}

func (coreConfig *coreConfig) StaticAssetDirsPrivate() string {
	if coreConfig == nil {
		return ""
	}
	return coreConfig.staticAssetDirsPrivate
}

func (coreConfig *coreConfig) StaticAssetDirsPublic() string {
	if coreConfig == nil {
		return ""
	}
	return coreConfig.staticAssetDirsPublic
}

func (coreConfig *coreConfig) CriticalCSSEntryFile() string {
	if coreConfig == nil {
		return ""
	}
	return coreConfig.criticalCSSEntryFile
}

func (coreConfig *coreConfig) NonCriticalCSSEntryFile() string {
	if coreConfig == nil {
		return ""
	}
	return coreConfig.nonCriticalCSSEntryFile
}

func (coreConfig *coreConfig) PublicPathPrefix() string {
	if coreConfig == nil {
		return ""
	}
	return coreConfig.publicPathPrefix
}

func (coreConfig *coreConfig) ServerOnlyMode() bool {
	return coreConfig != nil && coreConfig.serverOnlyMode
}

func (coreConfig *coreConfig) SequentialGoBuild() bool {
	return coreConfig != nil && coreConfig.sequentialGoBuild
}

func (coreConfig *coreConfig) Clone() CoreConfig {
	if coreConfig == nil {
		return nil
	}
	clonedCoreConfig := *coreConfig
	return &clonedCoreConfig
}

// ViteConfig is the parsed, normalized Vite config section.
type ViteConfig interface {
	// JSPackageManagerBaseCmd returns Vite.JSPackageManagerBaseCmd exactly as
	// configured.
	JSPackageManagerBaseCmd() string
	// JSPackageManagerCmdDir is normalized relative to effective ResolveRoot.
	JSPackageManagerCmdDir() string
	// DefaultPort returns Vite.DefaultPort after parse normalization.
	DefaultPort() int
	// ViteConfigFile is normalized relative to effective ResolveRoot.
	ViteConfigFile() string
	// Clone returns a defensive copy of this parsed Vite config.
	Clone() ViteConfig

	// waveViteConfigSeal prevents external implementations of ViteConfig.
	waveViteConfigSeal()
}

type viteConfig struct {
	// jsPackageManagerBaseCmd is Vite.JSPackageManagerBaseCmd exactly as configured.
	jsPackageManagerBaseCmd string
	// jsPackageManagerCmdDir is Vite.JSPackageManagerCmdDir resolved relative to effective ResolveRoot.
	jsPackageManagerCmdDir string
	// defaultPort is the parsed/defaulted Vite.DefaultPort value.
	defaultPort int
	// viteConfigFile is Vite.ViteConfigFile resolved relative to effective ResolveRoot.
	viteConfigFile string
}

func (viteConfig *viteConfig) waveViteConfigSeal() {}

func (viteConfig *viteConfig) JSPackageManagerBaseCmd() string {
	if viteConfig == nil {
		return ""
	}
	return viteConfig.jsPackageManagerBaseCmd
}

func (viteConfig *viteConfig) JSPackageManagerCmdDir() string {
	if viteConfig == nil {
		return ""
	}
	return viteConfig.jsPackageManagerCmdDir
}

func (viteConfig *viteConfig) DefaultPort() int {
	if viteConfig == nil {
		return 0
	}
	return viteConfig.defaultPort
}

func (viteConfig *viteConfig) ViteConfigFile() string {
	if viteConfig == nil {
		return ""
	}
	return viteConfig.viteConfigFile
}

func (viteConfig *viteConfig) Clone() ViteConfig {
	if viteConfig == nil {
		return nil
	}
	clonedViteConfig := *viteConfig
	return &clonedViteConfig
}

// WatchConfig is the parsed, normalized Watch config section.
type WatchConfig interface {
	// HealthcheckEndpoint returns Watch.HealthcheckEndpoint after parse
	// defaulting/normalization.
	HealthcheckEndpoint() string
	// HookStageFailurePolicy returns Watch.HookStageFailurePolicy after parse
	// defaulting/normalization.
	HookStageFailurePolicy() string
	// PreCommandTimeoutMilliseconds returns the parsed pre-stage command timeout.
	PreCommandTimeoutMilliseconds() int
	// ConcurrentCommandTimeoutMilliseconds returns the parsed concurrent-stage
	// command timeout.
	ConcurrentCommandTimeoutMilliseconds() int
	// ConcurrentNoWaitCommandTimeoutMilliseconds returns the parsed
	// concurrent-no-wait-stage command timeout.
	ConcurrentNoWaitCommandTimeoutMilliseconds() int
	// PostCommandTimeoutMilliseconds returns the parsed post-stage command
	// timeout.
	PostCommandTimeoutMilliseconds() int
	// PreCallbackTimeoutMilliseconds returns the parsed pre-stage callback
	// timeout.
	PreCallbackTimeoutMilliseconds() int
	// ConcurrentCallbackTimeoutMilliseconds returns the parsed concurrent-stage
	// callback timeout.
	ConcurrentCallbackTimeoutMilliseconds() int
	// ConcurrentNoWaitCallbackTimeoutMilliseconds returns the parsed
	// concurrent-no-wait-stage callback timeout.
	ConcurrentNoWaitCallbackTimeoutMilliseconds() int
	// PostCallbackTimeoutMilliseconds returns the parsed post-stage callback
	// timeout.
	PostCallbackTimeoutMilliseconds() int
	// Include returns patterns normalized relative to effective ResolveRoot.
	Include() []wavewatch.WatchedFile
	// ExcludeDirs returns patterns normalized relative to effective ResolveRoot.
	ExcludeDirs() []string
	// ExcludeFiles returns patterns normalized relative to effective ResolveRoot.
	ExcludeFiles() []string
	// Clone returns a defensive copy of this parsed Watch config.
	Clone() WatchConfig

	// waveWatchConfigSeal prevents external implementations of WatchConfig.
	waveWatchConfigSeal()
}

type watchConfig struct {
	// healthcheckEndpoint is the parsed/defaulted Watch.HealthcheckEndpoint value.
	healthcheckEndpoint string
	// hookStageFailurePolicy is the parsed/defaulted Watch.HookStageFailurePolicy value.
	hookStageFailurePolicy string
	// preCommandTimeoutMilliseconds is the parsed/defaulted pre-stage hook command timeout.
	preCommandTimeoutMilliseconds int
	// concurrentCommandTimeoutMilliseconds is the parsed/defaulted concurrent-stage hook command timeout.
	concurrentCommandTimeoutMilliseconds int
	// concurrentNoWaitCommandTimeoutMilliseconds is the parsed/defaulted concurrent-no-wait-stage hook command timeout.
	concurrentNoWaitCommandTimeoutMilliseconds int
	// postCommandTimeoutMilliseconds is the parsed/defaulted post-stage hook command timeout.
	postCommandTimeoutMilliseconds int
	// preCallbackTimeoutMilliseconds is the parsed/defaulted pre-stage hook callback timeout.
	preCallbackTimeoutMilliseconds int
	// concurrentCallbackTimeoutMilliseconds is the parsed/defaulted concurrent-stage hook callback timeout.
	concurrentCallbackTimeoutMilliseconds int
	// concurrentNoWaitCallbackTimeoutMilliseconds is the parsed/defaulted concurrent-no-wait-stage hook callback timeout.
	concurrentNoWaitCallbackTimeoutMilliseconds int
	// postCallbackTimeoutMilliseconds is the parsed/defaulted post-stage hook callback timeout.
	postCallbackTimeoutMilliseconds int
	// include holds Watch.Include entries resolved relative to effective ResolveRoot.
	include []wavewatch.WatchedFile
	// excludeDirs holds Watch.Exclude.Dirs entries resolved relative to effective ResolveRoot.
	excludeDirs []string
	// excludeFiles holds Watch.Exclude.Files entries resolved relative to effective ResolveRoot.
	excludeFiles []string
}

func (watchConfig *watchConfig) waveWatchConfigSeal() {}

func (watchConfig *watchConfig) HealthcheckEndpoint() string {
	if watchConfig == nil {
		return ""
	}
	return watchConfig.healthcheckEndpoint
}

func (watchConfig *watchConfig) HookStageFailurePolicy() string {
	if watchConfig == nil {
		return ""
	}
	return watchConfig.hookStageFailurePolicy
}

func (watchConfig *watchConfig) PreCommandTimeoutMilliseconds() int {
	if watchConfig == nil {
		return 0
	}
	return watchConfig.preCommandTimeoutMilliseconds
}

func (watchConfig *watchConfig) ConcurrentCommandTimeoutMilliseconds() int {
	if watchConfig == nil {
		return 0
	}
	return watchConfig.concurrentCommandTimeoutMilliseconds
}

func (watchConfig *watchConfig) ConcurrentNoWaitCommandTimeoutMilliseconds() int {
	if watchConfig == nil {
		return 0
	}
	return watchConfig.concurrentNoWaitCommandTimeoutMilliseconds
}

func (watchConfig *watchConfig) PostCommandTimeoutMilliseconds() int {
	if watchConfig == nil {
		return 0
	}
	return watchConfig.postCommandTimeoutMilliseconds
}

func (watchConfig *watchConfig) PreCallbackTimeoutMilliseconds() int {
	if watchConfig == nil {
		return 0
	}
	return watchConfig.preCallbackTimeoutMilliseconds
}

func (watchConfig *watchConfig) ConcurrentCallbackTimeoutMilliseconds() int {
	if watchConfig == nil {
		return 0
	}
	return watchConfig.concurrentCallbackTimeoutMilliseconds
}

func (watchConfig *watchConfig) ConcurrentNoWaitCallbackTimeoutMilliseconds() int {
	if watchConfig == nil {
		return 0
	}
	return watchConfig.concurrentNoWaitCallbackTimeoutMilliseconds
}

func (watchConfig *watchConfig) PostCallbackTimeoutMilliseconds() int {
	if watchConfig == nil {
		return 0
	}
	return watchConfig.postCallbackTimeoutMilliseconds
}

func (watchConfig *watchConfig) Include() []wavewatch.WatchedFile {
	if watchConfig == nil {
		return nil
	}
	return cloneWatchedFiles(watchConfig.include)
}

func (watchConfig *watchConfig) ExcludeDirs() []string {
	if watchConfig == nil {
		return nil
	}
	return append([]string(nil), watchConfig.excludeDirs...)
}

func (watchConfig *watchConfig) ExcludeFiles() []string {
	if watchConfig == nil {
		return nil
	}
	return append([]string(nil), watchConfig.excludeFiles...)
}

func (watchConfig *watchConfig) Clone() WatchConfig {
	if watchConfig == nil {
		return nil
	}
	clonedWatchConfig := *watchConfig
	clonedWatchConfig.include = cloneWatchedFiles(watchConfig.include)
	clonedWatchConfig.excludeDirs = append([]string(nil), watchConfig.excludeDirs...)
	clonedWatchConfig.excludeFiles = append([]string(nil), watchConfig.excludeFiles...)
	return &clonedWatchConfig
}

// ParsedConfig is an opaque parsed Wave config handle. It is intentionally an
// interface so callers cannot construct config instances directly; parsing is
// the only supported construction path.
type ParsedConfig interface {
	// Core returns the parsed Core section.
	Core() CoreConfig
	// Vite returns the parsed Vite section, or nil when omitted.
	Vite() ViteConfig
	// Watch returns the parsed Watch section, or nil when omitted.
	Watch() WatchConfig
	// Dist returns parsed dist layout paths derived by parser from the
	// config-sibling `.wavedist` directory.
	Dist() DistLayout
	// ConfigFileDirectory returns effective ResolveRoot in parser config-path
	// basis.
	ConfigFileDirectory() string
	// SemanticConfigJSON returns canonical JSON bytes for semantic config
	// equality checks.
	SemanticConfigJSON() []byte
	// PublicPathPrefix returns the normalized effective public path prefix.
	PublicPathPrefix() string
	// ViteManifestPath returns the expected private static path of Vite's
	// manifest file.
	ViteManifestPath() string
	// ResolveRoot returns effective resolve root in parser config-path basis.
	ResolveRoot() string
	// HealthcheckEndpoint returns Watch.HealthcheckEndpoint with parser defaults
	// applied.
	HealthcheckEndpoint() string
	// UsingBrowser reports whether browser-mode runtime behavior is enabled.
	UsingBrowser() bool
	// UsingVite reports whether a parsed Vite section is present.
	UsingVite() bool
	// CriticalCSSEntry returns Core.CSSEntryFiles.Critical with parser
	// normalization applied.
	CriticalCSSEntry() string
	// NonCriticalCSSEntry returns Core.CSSEntryFiles.NonCritical with parser
	// normalization applied.
	NonCriticalCSSEntry() string
	// Clone returns a defensive copy of this parsed config snapshot.
	Clone() ParsedConfig

	// waveParsedConfigSeal prevents external implementations of ParsedConfig.
	waveParsedConfigSeal()
}

// parsedConfig is the internal parsed configuration implementation.
type parsedConfig struct {
	// core is the parsed Core section; fields are parser-normalized only.
	core CoreConfig
	// vite is the parsed Vite section; fields are parser-normalized only.
	vite ViteConfig
	// watch is the parsed Watch section; fields are parser-normalized only.
	watch WatchConfig
	// dist is parser-derived dist layout rooted at config-sibling `.wavedist`.
	dist DistLayout

	// configFileDirectory stores effective ResolveRoot in parser config-path
	// basis.
	configFileDirectory string
	// semanticConfigJSON stores canonical JSON used for semantic config change
	// detection.
	semanticConfigJSON []byte
}

type distLayoutWireModel struct {
	Root string `json:"-"`
}

type coreConfigWireModel struct {
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

type viteConfigWireModel struct {
	JSPackageManagerBaseCmd string `json:"JSPackageManagerBaseCmd"`
	JSPackageManagerCmdDir  string `json:"JSPackageManagerCmdDir,omitempty"`
	DefaultPort             int    `json:"DefaultPort,omitempty"`
	ViteConfigFile          string `json:"ViteConfigFile,omitempty"`
}

type hookCommandTimeoutConfigWireModel struct {
	PreCommandTimeoutMilliseconds              int `json:"PreCommandTimeoutMilliseconds,omitempty"`
	ConcurrentCommandTimeoutMilliseconds       int `json:"ConcurrentCommandTimeoutMilliseconds,omitempty"`
	ConcurrentNoWaitCommandTimeoutMilliseconds int `json:"ConcurrentNoWaitCommandTimeoutMilliseconds,omitempty"`
	PostCommandTimeoutMilliseconds             int `json:"PostCommandTimeoutMilliseconds,omitempty"`
}

type hookCallbackTimeoutConfigWireModel struct {
	PreCallbackTimeoutMilliseconds              int `json:"PreCallbackTimeoutMilliseconds,omitempty"`
	ConcurrentCallbackTimeoutMilliseconds       int `json:"ConcurrentCallbackTimeoutMilliseconds,omitempty"`
	ConcurrentNoWaitCallbackTimeoutMilliseconds int `json:"ConcurrentNoWaitCallbackTimeoutMilliseconds,omitempty"`
	PostCallbackTimeoutMilliseconds             int `json:"PostCallbackTimeoutMilliseconds,omitempty"`
}

type watchConfigWireModel struct {
	HealthcheckEndpoint    string                             `json:"HealthcheckEndpoint,omitempty"`
	HookStageFailurePolicy string                             `json:"HookStageFailurePolicy,omitempty"`
	HookCommandTimeouts    hookCommandTimeoutConfigWireModel  `json:"HookCommandTimeouts"`
	HookCallbackTimeouts   hookCallbackTimeoutConfigWireModel `json:"HookCallbackTimeouts"`
	Include                []wavewatch.WatchedFile            `json:"Include,omitempty"`
	Exclude                struct {
		Dirs  []string `json:"Dirs,omitempty"`
		Files []string `json:"Files,omitempty"`
	} `json:"Exclude,omitempty"`
}

// parsedConfigWireModel is used only while unmarshalling and path-resolving raw
// JSON. Parsed runtime values are copied into parsedConfig afterward.
type parsedConfigWireModel struct {
	Core  *coreConfigWireModel  `json:"Core"`
	Vite  *viteConfigWireModel  `json:"Vite,omitempty"`
	Watch *watchConfigWireModel `json:"Watch,omitempty"`
	Dist  distLayoutWireModel   `json:"-"`
}

func (parsedConfig *parsedConfig) waveParsedConfigSeal() {}

func (parsedConfig *parsedConfig) Core() CoreConfig {
	if parsedConfig == nil {
		return nil
	}
	return parsedConfig.core
}

func (parsedConfig *parsedConfig) Vite() ViteConfig {
	if parsedConfig == nil {
		return nil
	}
	return parsedConfig.vite
}

func (parsedConfig *parsedConfig) Watch() WatchConfig {
	if parsedConfig == nil {
		return nil
	}
	return parsedConfig.watch
}

func (parsedConfig *parsedConfig) Dist() DistLayout {
	if parsedConfig == nil {
		return nil
	}
	return parsedConfig.dist
}

func (parsedConfig *parsedConfig) ConfigFileDirectory() string {
	if parsedConfig == nil {
		return ""
	}
	return parsedConfig.configFileDirectory
}

func (parsedConfig *parsedConfig) SemanticConfigJSON() []byte {
	if parsedConfig == nil {
		return nil
	}
	return append([]byte(nil), parsedConfig.semanticConfigJSON...)
}

// PublicPathPrefix returns the normalized configured public path prefix.
// The root prefix is represented as "/".
func (parsedConfig *parsedConfig) PublicPathPrefix() string {
	if parsedConfig == nil || parsedConfig.core == nil {
		return "/"
	}
	prefix := parsedConfig.core.PublicPathPrefix()
	if prefix == "" || prefix == "/" {
		return "/"
	}
	return matcher.EnsureLeadingAndTrailingSlash(prefix)
}

// ViteManifestPath returns the expected private output path for the Vite
// manifest produced during builds.
func (parsedConfig *parsedConfig) ViteManifestPath() string {
	if parsedConfig == nil || parsedConfig.dist == nil {
		return ""
	}
	return filepath.Join(
		parsedConfig.dist.StaticPrivate(),
		waveartifacts.ViteManifestFilePath(),
	)
}

func (parsedConfig *parsedConfig) ResolveRoot() string {
	if parsedConfig != nil &&
		strings.TrimSpace(parsedConfig.configFileDirectory) != "" {
		return filepath.Clean(parsedConfig.configFileDirectory)
	}
	return "."
}

func (parsedConfig *parsedConfig) HealthcheckEndpoint() string {
	if parsedConfig != nil &&
		parsedConfig.watch != nil &&
		parsedConfig.watch.HealthcheckEndpoint() != "" {
		return parsedConfig.watch.HealthcheckEndpoint()
	}
	return "/"
}

func (parsedConfig *parsedConfig) UsingBrowser() bool {
	if parsedConfig == nil || parsedConfig.core == nil {
		return true
	}
	return !parsedConfig.core.ServerOnlyMode()
}

func (parsedConfig *parsedConfig) UsingVite() bool {
	return parsedConfig != nil && parsedConfig.vite != nil
}

func (parsedConfig *parsedConfig) CriticalCSSEntry() string {
	if parsedConfig == nil ||
		parsedConfig.core == nil ||
		parsedConfig.core.CriticalCSSEntryFile() == "" {
		return ""
	}
	return filepath.Clean(parsedConfig.core.CriticalCSSEntryFile())
}

func (parsedConfig *parsedConfig) NonCriticalCSSEntry() string {
	if parsedConfig == nil ||
		parsedConfig.core == nil ||
		parsedConfig.core.NonCriticalCSSEntryFile() == "" {
		return ""
	}
	return filepath.Clean(parsedConfig.core.NonCriticalCSSEntryFile())
}

func newDistLayoutFromWire(wireLayout distLayoutWireModel) DistLayout {
	return &distLayout{
		root: wireLayout.Root,
	}
}

func newCoreConfigFromWire(wireCoreConfig *coreConfigWireModel) CoreConfig {
	if wireCoreConfig == nil {
		return nil
	}
	return &coreConfig{
		projectID:                        wireCoreConfig.ProjectID,
		configuredResolveRoot:            wireCoreConfig.ResolveRoot,
		devBuildHook:                     wireCoreConfig.DevBuildHook,
		devBuildHookTimeoutMilliseconds:  wireCoreConfig.DevBuildHookTimeoutMilliseconds,
		prodBuildHook:                    wireCoreConfig.ProdBuildHook,
		prodBuildHookTimeoutMilliseconds: wireCoreConfig.ProdBuildHookTimeoutMilliseconds,
		mainAppEntry:                     wireCoreConfig.MainAppEntry,
		staticAssetDirsPrivate:           wireCoreConfig.StaticAssetDirs.Private,
		staticAssetDirsPublic:            wireCoreConfig.StaticAssetDirs.Public,
		criticalCSSEntryFile:             wireCoreConfig.CSSEntryFiles.Critical,
		nonCriticalCSSEntryFile:          wireCoreConfig.CSSEntryFiles.NonCritical,
		publicPathPrefix:                 wireCoreConfig.PublicPathPrefix,
		serverOnlyMode:                   wireCoreConfig.ServerOnlyMode,
		sequentialGoBuild:                wireCoreConfig.SequentialGoBuild,
	}
}

func newViteConfigFromWire(wireViteConfig *viteConfigWireModel) ViteConfig {
	if wireViteConfig == nil {
		return nil
	}
	return &viteConfig{
		jsPackageManagerBaseCmd: wireViteConfig.JSPackageManagerBaseCmd,
		jsPackageManagerCmdDir:  wireViteConfig.JSPackageManagerCmdDir,
		defaultPort:             wireViteConfig.DefaultPort,
		viteConfigFile:          wireViteConfig.ViteConfigFile,
	}
}

func newWatchConfigFromWire(wireWatchConfig *watchConfigWireModel) WatchConfig {
	if wireWatchConfig == nil {
		return nil
	}
	return &watchConfig{
		healthcheckEndpoint:                         wireWatchConfig.HealthcheckEndpoint,
		hookStageFailurePolicy:                      wireWatchConfig.HookStageFailurePolicy,
		preCommandTimeoutMilliseconds:               wireWatchConfig.HookCommandTimeouts.PreCommandTimeoutMilliseconds,
		concurrentCommandTimeoutMilliseconds:        wireWatchConfig.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds,
		concurrentNoWaitCommandTimeoutMilliseconds:  wireWatchConfig.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds,
		postCommandTimeoutMilliseconds:              wireWatchConfig.HookCommandTimeouts.PostCommandTimeoutMilliseconds,
		preCallbackTimeoutMilliseconds:              wireWatchConfig.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds,
		concurrentCallbackTimeoutMilliseconds:       wireWatchConfig.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds,
		concurrentNoWaitCallbackTimeoutMilliseconds: wireWatchConfig.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds,
		postCallbackTimeoutMilliseconds:             wireWatchConfig.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds,
		include:                                     cloneWatchedFiles(wireWatchConfig.Include),
		excludeDirs:                                 append([]string(nil), wireWatchConfig.Exclude.Dirs...),
		excludeFiles:                                append([]string(nil), wireWatchConfig.Exclude.Files...),
	}
}

func ParseConfigJSON(data []byte) (ParsedConfig, error) {
	return nil, fmt.Errorf(
		"config path is required; use ParseConfigJSONWithConfigPath",
	)
}

func ParseConfigJSONWithConfigPath(
	data []byte,
	configPath string,
) (ParsedConfig, error) {
	// configPath may be relative (preferred) or machine-absolute. Machine-
	// absolute paths are normalized to the caller's current working directory so
	// parser output remains non-absolute and basis-consistent.
	configPathRelativeToParserPathBasis, normalizeConfigPathError := normalizeConfigPathRelativeToParserPathBasis(
		configPath,
	)
	if normalizeConfigPathError != nil {
		return nil, normalizeConfigPathError
	}
	parsedConfig, parseError := parseWaveConfigJSON(
		data,
		configPathRelativeToParserPathBasis,
	)
	if parseError != nil {
		return nil, parseError
	}
	return parsedConfig, nil
}

func ParseConfigFile(path string) (ParsedConfig, error) {
	configFilePath := strings.TrimSpace(path)
	if configFilePath == "" {
		return nil, fmt.Errorf("config file path is required")
	}
	configFilePathForRead := filepath.Clean(configFilePath)

	data, err := os.ReadFile(configFilePathForRead)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg, err := ParseConfigJSONWithConfigPath(data, configFilePathForRead)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func normalizeConfigPathRelativeToParserPathBasis(
	configPath string,
) (string, error) {
	trimmedConfigPath := strings.TrimSpace(configPath)
	if trimmedConfigPath == "" {
		return "", fmt.Errorf("config path is required")
	}
	cleanedConfigPath := filepath.Clean(trimmedConfigPath)
	if !waveenv.IsMachineAbsoluteFilesystemPath(cleanedConfigPath) {
		return cleanedConfigPath, nil
	}
	configPathMachineAbsolute := cleanedConfigPath

	currentWorkingDirectory, currentWorkingDirectoryError := os.Getwd()
	if currentWorkingDirectoryError != nil {
		return "", fmt.Errorf(
			"resolve working directory for config path normalization: %w",
			currentWorkingDirectoryError,
		)
	}
	configPathRelativeToCurrentWorkingDirectory, relativePathError := filepath.Rel(
		filepath.Clean(currentWorkingDirectory),
		configPathMachineAbsolute,
	)
	if relativePathError != nil {
		return "", fmt.Errorf(
			"relativize machine-absolute config path %q from working directory %q: %w",
			configPathMachineAbsolute,
			currentWorkingDirectory,
			relativePathError,
		)
	}
	return filepath.Clean(configPathRelativeToCurrentWorkingDirectory), nil
}

func (currentParsedConfig *parsedConfig) Clone() ParsedConfig {
	if currentParsedConfig == nil {
		return nil
	}

	return &parsedConfig{
		core:                cloneCoreConfig(currentParsedConfig.core),
		vite:                cloneViteConfig(currentParsedConfig.vite),
		watch:               cloneWatchConfig(currentParsedConfig.watch),
		dist:                cloneDistLayout(currentParsedConfig.dist),
		configFileDirectory: currentParsedConfig.configFileDirectory,
		semanticConfigJSON: append(
			[]byte(nil),
			currentParsedConfig.semanticConfigJSON...,
		),
	}
}

func cloneDistLayout(currentDistLayout DistLayout) DistLayout {
	if currentDistLayout == nil {
		return nil
	}
	return currentDistLayout.Clone()
}

func cloneCoreConfig(currentCoreConfig CoreConfig) CoreConfig {
	if currentCoreConfig == nil {
		return nil
	}
	return currentCoreConfig.Clone()
}

func cloneViteConfig(currentViteConfig ViteConfig) ViteConfig {
	if currentViteConfig == nil {
		return nil
	}
	return currentViteConfig.Clone()
}

func cloneWatchConfig(currentWatchConfig WatchConfig) WatchConfig {
	if currentWatchConfig == nil {
		return nil
	}
	return currentWatchConfig.Clone()
}

func cloneWatchedFiles(
	watchedFiles []wavewatch.WatchedFile,
) []wavewatch.WatchedFile {
	if len(watchedFiles) == 0 {
		return nil
	}
	clonedWatchedFiles := make(
		[]wavewatch.WatchedFile,
		0,
		len(watchedFiles),
	)
	for _, watchedFile := range watchedFiles {
		clonedWatchedFiles = append(
			clonedWatchedFiles,
			cloneWatchedFile(watchedFile),
		)
	}
	return clonedWatchedFiles
}

func cloneWatchedFile(
	watchedFile wavewatch.WatchedFile,
) wavewatch.WatchedFile {
	clonedWatchedFile := watchedFile
	clonedWatchedFile.OnChangeHooks = cloneOnChangeHooks(
		watchedFile.OnChangeHooks,
	)
	clonedWatchedFile.SortedHooks = cloneSortedHooks(
		watchedFile.SortedHooks,
	)
	return clonedWatchedFile
}

func cloneSortedHooks(
	sortedHooks *wavewatch.SortedHooks,
) *wavewatch.SortedHooks {
	if sortedHooks == nil {
		return nil
	}
	return &wavewatch.SortedHooks{
		Pre: cloneOnChangeHooks(sortedHooks.Pre),
		Concurrent: cloneOnChangeHooks(
			sortedHooks.Concurrent,
		),
		ConcurrentNoWait: cloneOnChangeHooks(
			sortedHooks.ConcurrentNoWait,
		),
		Post: cloneOnChangeHooks(sortedHooks.Post),
	}
}

func cloneOnChangeHooks(
	onChangeHooks []wavewatch.OnChangeHook,
) []wavewatch.OnChangeHook {
	if len(onChangeHooks) == 0 {
		return nil
	}
	clonedOnChangeHooks := make(
		[]wavewatch.OnChangeHook,
		0,
		len(onChangeHooks),
	)
	for _, onChangeHook := range onChangeHooks {
		clonedOnChangeHook := onChangeHook
		clonedOnChangeHook.Exclude = append(
			[]string(nil),
			onChangeHook.Exclude...)
		clonedOnChangeHooks = append(clonedOnChangeHooks, clonedOnChangeHook)
	}
	return clonedOnChangeHooks
}

// MustReadFile wraps fs.ReadFile and panics on error.
func MustReadFile(fileSystem fs.FS, filePath string) []byte {
	data, err := fs.ReadFile(fileSystem, filePath)
	if err != nil {
		panic(err)
	}
	return data
}
