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
	"Core.MainAppEntry",
	"Core.DistDir",
	"Core.StaticAssetDirs.Private",
	"Core.StaticAssetDirs.Public",
	"Core.CSSEntryFiles.Critical",
	"Core.CSSEntryFiles.NonCritical",
	"Vite.JSPackageManagerCmdDir",
	"Vite.ViteConfigFile",
	"Watch.WatchRoot",
	"Watch.Exclude.Dirs.*",
	"Watch.Exclude.Files.*",
	"Watch.Include.*.Pattern",
	"Watch.Include.*.OnChangeHooks.*.Exclude.*",
}

// parseWaveConfigJSON unmarshals one Wave config JSON payload and enforces
// Wave's machine-absolute filesystem-path invariants.
func parseWaveConfigJSON[T any](data []byte) (*T, error) {
	var parsedConfig T
	parseError := parseJSONWithFilesystemPathRulesIntoTarget(
		data,
		&parsedConfig,
		untypedOptions{
			ValidateRequiredSections: validateWaveConfigRequiredSections,
			FilesystemPathSelectors:  waveFilesystemPathSelectors,
			PanicMachineAbsolutePath: panicMachineAbsoluteWaveConfigPath,
		},
	)
	if parseError != nil {
		return nil, parseError
	}
	if finalizeError := finalizeWaveDistLayout(
		reflect.ValueOf(&parsedConfig).Elem(),
	); finalizeError != nil {
		return nil, finalizeError
	}
	if finalizeFilesystemPathsError := finalizeWaveFilesystemConfigPaths(
		reflect.ValueOf(&parsedConfig).Elem(),
	); finalizeFilesystemPathsError != nil {
		return nil, finalizeFilesystemPathsError
	}
	return &parsedConfig, nil
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
	case fieldPath == "Core.MainAppEntry":
		return "Core.MainAppEntry must be a CWD-relative app entry path (for example \"backend/cmd/serve\")."
	case fieldPath == "Core.DistDir":
		return "Core.DistDir must be a CWD-relative build output directory (for example \"dist\" or \"tmp/dist\")."
	case fieldPath == "Core.StaticAssetDirs.Private":
		return "Core.StaticAssetDirs.Private must be a CWD-relative source directory path."
	case fieldPath == "Core.StaticAssetDirs.Public":
		return "Core.StaticAssetDirs.Public must be a CWD-relative source directory path."
	case fieldPath == "Core.CSSEntryFiles.Critical":
		return "Core.CSSEntryFiles.Critical must be a CWD-relative CSS entry file path."
	case fieldPath == "Core.CSSEntryFiles.NonCritical":
		return "Core.CSSEntryFiles.NonCritical must be a CWD-relative CSS entry file path."
	case fieldPath == "Vite.JSPackageManagerCmdDir":
		return "Vite.JSPackageManagerCmdDir must be a CWD-relative working directory path."
	case fieldPath == "Vite.ViteConfigFile":
		return "Vite.ViteConfigFile must be a CWD-relative Vite config file path."
	case fieldPath == "Watch.WatchRoot":
		return "Watch.WatchRoot must be a CWD-relative watch-root directory path."
	case strings.HasPrefix(fieldPath, "Watch.Exclude.Dirs["):
		return "Watch.Exclude.Dirs entries must be watch-root-relative glob patterns."
	case strings.HasPrefix(fieldPath, "Watch.Exclude.Files["):
		return "Watch.Exclude.Files entries must be watch-root-relative glob patterns."
	case strings.HasPrefix(fieldPath, "Watch.Include[") &&
		strings.Contains(fieldPath, "].Pattern"):
		return "Watch.Include[].Pattern entries must be watch-root-relative glob patterns."
	case strings.HasPrefix(fieldPath, "Watch.Include[") &&
		strings.Contains(fieldPath, ".OnChangeHooks[") &&
		strings.Contains(fieldPath, "].Exclude["):
		return "Watch.Include[].OnChangeHooks[].Exclude entries must be watch-root-relative glob patterns."
	default:
		return "Use a non-absolute path according to this field's semantics (CWD-relative or relative to its owning path field)."
	}
}

func finalizeWaveDistLayout(parsedValue reflect.Value) error {
	coreField := parsedValue.FieldByName("Core")
	if !coreField.IsValid() {
		return fmt.Errorf("config: Core section is required")
	}
	resolvedCoreField := dereferenceValueForSelectorTraversal(coreField)
	if !resolvedCoreField.IsValid() ||
		resolvedCoreField.Kind() != reflect.Struct {
		return fmt.Errorf("config: Core section is required")
	}
	distDirField := resolvedCoreField.FieldByName("DistDir")
	if !distDirField.IsValid() || distDirField.Kind() != reflect.String {
		return fmt.Errorf("config: Core.DistDir is required")
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
	rootField.SetString(filepath.Clean(distDirField.String()))
	return nil
}

func finalizeWaveFilesystemConfigPaths(parsedValue reflect.Value) error {
	coreField := parsedValue.FieldByName("Core")
	if !coreField.IsValid() {
		return fmt.Errorf("config: Core section is required")
	}
	resolvedCoreField := dereferenceValueForSelectorTraversal(coreField)
	if !resolvedCoreField.IsValid() ||
		resolvedCoreField.Kind() != reflect.Struct {
		return fmt.Errorf("config: Core section is required")
	}
	if finalizeError := absolutizeFilesystemPathStructField(
		resolvedCoreField,
		"DistDir",
	); finalizeError != nil {
		return finalizeError
	}

	staticAssetDirsField := resolvedCoreField.FieldByName("StaticAssetDirs")
	resolvedStaticAssetDirsField := dereferenceValueForSelectorTraversal(
		staticAssetDirsField,
	)
	if resolvedStaticAssetDirsField.IsValid() &&
		resolvedStaticAssetDirsField.Kind() == reflect.Struct {
		if finalizeError := absolutizeFilesystemPathStructField(
			resolvedStaticAssetDirsField,
			"Private",
		); finalizeError != nil {
			return finalizeError
		}
		if finalizeError := absolutizeFilesystemPathStructField(
			resolvedStaticAssetDirsField,
			"Public",
		); finalizeError != nil {
			return finalizeError
		}
	}

	cssEntryFilesField := resolvedCoreField.FieldByName("CSSEntryFiles")
	resolvedCSSEntryFilesField := dereferenceValueForSelectorTraversal(
		cssEntryFilesField,
	)
	if resolvedCSSEntryFilesField.IsValid() &&
		resolvedCSSEntryFilesField.Kind() == reflect.Struct {
		if finalizeError := absolutizeFilesystemPathStructField(
			resolvedCSSEntryFilesField,
			"Critical",
		); finalizeError != nil {
			return finalizeError
		}
		if finalizeError := absolutizeFilesystemPathStructField(
			resolvedCSSEntryFilesField,
			"NonCritical",
		); finalizeError != nil {
			return finalizeError
		}
	}

	viteField := parsedValue.FieldByName("Vite")
	resolvedViteField := dereferenceValueForSelectorTraversal(viteField)
	if resolvedViteField.IsValid() &&
		resolvedViteField.Kind() == reflect.Struct {
		if finalizeError := absolutizeFilesystemPathStructField(
			resolvedViteField,
			"JSPackageManagerCmdDir",
		); finalizeError != nil {
			return finalizeError
		}
		if finalizeError := absolutizeFilesystemPathStructField(
			resolvedViteField,
			"ViteConfigFile",
		); finalizeError != nil {
			return finalizeError
		}
	}

	watchField := parsedValue.FieldByName("Watch")
	resolvedWatchField := dereferenceValueForSelectorTraversal(watchField)
	if resolvedWatchField.IsValid() &&
		resolvedWatchField.Kind() == reflect.Struct {
		if finalizeError := absolutizeFilesystemPathStructField(
			resolvedWatchField,
			"WatchRoot",
		); finalizeError != nil {
			return finalizeError
		}
	}

	distField := parsedValue.FieldByName("Dist")
	resolvedDistField := dereferenceValueForSelectorTraversal(distField)
	if resolvedDistField.IsValid() &&
		resolvedDistField.Kind() == reflect.Struct {
		if finalizeError := absolutizeFilesystemPathStructField(
			resolvedDistField,
			"Root",
		); finalizeError != nil {
			return finalizeError
		}
	}

	return nil
}

func absolutizeFilesystemPathStructField(
	structFieldValue reflect.Value,
	fieldName string,
) error {
	field := structFieldValue.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		return nil
	}
	if !field.CanSet() {
		return fmt.Errorf(
			"config parser target field %s is not settable",
			fieldName,
		)
	}

	trimmedValue := strings.TrimSpace(field.String())
	if trimmedValue == "" {
		return nil
	}
	field.SetString(waveenv.Absolute(trimmedValue))
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
	segStatic   = "static"
	segAssets   = waveartifacts.AssetsDirname
	segPublic   = waveartifacts.PublicDirname
	segPrivate  = waveartifacts.PrivateDirname
	segInternal = waveartifacts.InternalDirname
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

type distLayout struct {
	Root string
}

func (d distLayout) Binary() string {
	name := fileBinary
	if runtime.GOOS == "windows" {
		name = fileBinaryWindows
	}
	return filepath.Join(d.Root, name)
}

func (d distLayout) Static() string { return filepath.Join(d.Root, segStatic) }

func (d distLayout) StaticPublic() string {
	return filepath.Join(d.Static(), segAssets, segPublic)
}

func (d distLayout) StaticPrivate() string {
	return filepath.Join(d.Static(), segAssets, segPrivate)
}

func (d distLayout) Internal() string { return filepath.Join(d.Static(), segInternal) }

func (d distLayout) CriticalCSS() string { return filepath.Join(d.Internal(), fileCriticalCSS) }

func (d distLayout) NormalCSSRef() string {
	return filepath.Join(d.Internal(), fileNormalCSSRef)
}

func (d distLayout) PublicFileMapRef() string {
	return filepath.Join(d.Internal(), filePublicMapRef)
}

func (d distLayout) PublicFileMapGob() string {
	return filepath.Join(d.Internal(), filePublicMapGob)
}

func (d distLayout) PrivateFileMapGob() string {
	return filepath.Join(d.Internal(), filePrivateMapGob)
}

func (d distLayout) BuildOutputLedger() string {
	return filepath.Join(d.Internal(), fileBuildOutputLedger)
}

func (d distLayout) KeepFile() string {
	return filepath.Join(d.Static(), fileKeep)
}

// CoreConfig contains framework/runtime core configuration.
type CoreConfig struct {
	ConfigLocation                   string          `json:"ConfigLocation,omitempty"`
	DevBuildHook                     string          `json:"DevBuildHook,omitempty"`
	DevBuildHookTimeoutMilliseconds  int             `json:"DevBuildHookTimeoutMilliseconds,omitempty"`
	ProdBuildHook                    string          `json:"ProdBuildHook,omitempty"`
	ProdBuildHookTimeoutMilliseconds int             `json:"ProdBuildHookTimeoutMilliseconds,omitempty"`
	MainAppEntry                     string          `json:"MainAppEntry"`
	DistDir                          string          `json:"DistDir"`
	StaticAssetDirs                  StaticAssetDirs `json:"StaticAssetDirs"`
	CSSEntryFiles                    CSSEntryFiles   `json:"CSSEntryFiles,omitempty"`
	PublicPathPrefix                 string          `json:"PublicPathPrefix,omitempty"`
	ServerOnlyMode                   bool            `json:"ServerOnlyMode,omitempty"`
	SequentialGoBuild                bool            `json:"SequentialGoBuild,omitempty"`
}

type StaticAssetDirs struct {
	Private string `json:"Private"`
	Public  string `json:"Public"`
}

type CSSEntryFiles struct {
	Critical    string `json:"Critical,omitempty"`
	NonCritical string `json:"NonCritical,omitempty"`
}

type ViteConfig struct {
	JSPackageManagerBaseCmd string `json:"JSPackageManagerBaseCmd"`
	JSPackageManagerCmdDir  string `json:"JSPackageManagerCmdDir,omitempty"`
	DefaultPort             int    `json:"DefaultPort,omitempty"`
	ViteConfigFile          string `json:"ViteConfigFile,omitempty"`
}

// WatchConfig configures dev watch behavior and hooks.
type WatchConfig struct {
	WatchRoot              string                    `json:"WatchRoot,omitempty"`
	HealthcheckEndpoint    string                    `json:"HealthcheckEndpoint,omitempty"`
	HookStageFailurePolicy string                    `json:"HookStageFailurePolicy,omitempty"`
	HookCommandTimeouts    HookCommandTimeoutConfig  `json:"HookCommandTimeouts"`
	HookCallbackTimeouts   HookCallbackTimeoutConfig `json:"HookCallbackTimeouts"`
	Include                []wavewatch.WatchedFile   `json:"Include,omitempty"`
	Exclude                struct {
		Dirs  []string `json:"Dirs,omitempty"`
		Files []string `json:"Files,omitempty"`
	} `json:"Exclude,omitempty"`
}

// HookCommandTimeoutConfig configures command timeout overrides per hook stage.
type HookCommandTimeoutConfig struct {
	PreCommandTimeoutMilliseconds              int `json:"PreCommandTimeoutMilliseconds,omitempty"`
	ConcurrentCommandTimeoutMilliseconds       int `json:"ConcurrentCommandTimeoutMilliseconds,omitempty"`
	ConcurrentNoWaitCommandTimeoutMilliseconds int `json:"ConcurrentNoWaitCommandTimeoutMilliseconds,omitempty"`
	PostCommandTimeoutMilliseconds             int `json:"PostCommandTimeoutMilliseconds,omitempty"`
}

// HookCallbackTimeoutConfig configures callback timeout overrides per hook stage.
type HookCallbackTimeoutConfig struct {
	PreCallbackTimeoutMilliseconds              int `json:"PreCallbackTimeoutMilliseconds,omitempty"`
	ConcurrentCallbackTimeoutMilliseconds       int `json:"ConcurrentCallbackTimeoutMilliseconds,omitempty"`
	ConcurrentNoWaitCallbackTimeoutMilliseconds int `json:"ConcurrentNoWaitCallbackTimeoutMilliseconds,omitempty"`
	PostCallbackTimeoutMilliseconds             int `json:"PostCallbackTimeoutMilliseconds,omitempty"`
}

// ParsedConfig is the parsed and validated Wave configuration payload.
type ParsedConfig struct {
	Core  *CoreConfig  `json:"Core"`
	Vite  *ViteConfig  `json:"Vite,omitempty"`
	Watch *WatchConfig `json:"Watch,omitempty"`

	Dist distLayout `json:"-"`
}

// PublicPathPrefix returns the normalized configured public path prefix.
// The root prefix is represented as "/".
func (parsedConfig *ParsedConfig) PublicPathPrefix() string {
	prefix := parsedConfig.Core.PublicPathPrefix
	if prefix == "" || prefix == "/" {
		return "/"
	}
	return matcher.EnsureLeadingAndTrailingSlash(prefix)
}

// ViteManifestPath returns the expected private output path for the Vite
// manifest produced during builds.
func (parsedConfig *ParsedConfig) ViteManifestPath() string {
	return filepath.Join(
		parsedConfig.Dist.StaticPrivate(),
		waveartifacts.ViteManifestFilePath(),
	)
}

func (parsedConfig *ParsedConfig) WatchRoot() string {
	if parsedConfig.Watch != nil && parsedConfig.Watch.WatchRoot != "" {
		return filepath.Clean(parsedConfig.Watch.WatchRoot)
	}
	return "."
}

func (parsedConfig *ParsedConfig) HealthcheckEndpoint() string {
	if parsedConfig.Watch != nil &&
		parsedConfig.Watch.HealthcheckEndpoint != "" {
		return parsedConfig.Watch.HealthcheckEndpoint
	}
	return "/"
}

func (parsedConfig *ParsedConfig) UsingBrowser() bool {
	return !parsedConfig.Core.ServerOnlyMode
}

func (parsedConfig *ParsedConfig) UsingVite() bool {
	return parsedConfig.Vite != nil
}

func (parsedConfig *ParsedConfig) CriticalCSSEntry() string {
	if parsedConfig.Core.CSSEntryFiles.Critical == "" {
		return ""
	}
	return filepath.Clean(parsedConfig.Core.CSSEntryFiles.Critical)
}

func (parsedConfig *ParsedConfig) NonCriticalCSSEntry() string {
	if parsedConfig.Core.CSSEntryFiles.NonCritical == "" {
		return ""
	}
	return filepath.Clean(parsedConfig.Core.CSSEntryFiles.NonCritical)
}

func ParseConfigJSON(data []byte) (*ParsedConfig, error) {
	return parseWaveConfigJSON[ParsedConfig](data)
}

func ParseConfigFile(path string) (*ParsedConfig, error) {
	configFilePath := strings.TrimSpace(path)
	if configFilePath == "" {
		return nil, fmt.Errorf("config file path is required")
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg, err := ParseConfigJSON(data)
	if err != nil {
		return nil, err
	}

	cfg.Core.ConfigLocation = waveenv.Absolute(configFilePath)
	return cfg, nil
}

func (parsedConfig *ParsedConfig) Clone() *ParsedConfig {
	if parsedConfig == nil {
		return nil
	}

	return &ParsedConfig{
		Core:  cloneCoreConfig(parsedConfig.Core),
		Vite:  cloneViteConfig(parsedConfig.Vite),
		Watch: cloneWatchConfig(parsedConfig.Watch),
		Dist:  parsedConfig.Dist,
	}
}

func cloneCoreConfig(coreConfig *CoreConfig) *CoreConfig {
	if coreConfig == nil {
		return nil
	}
	clonedCoreConfig := *coreConfig
	return &clonedCoreConfig
}

func cloneViteConfig(ViteConfig *ViteConfig) *ViteConfig {
	if ViteConfig == nil {
		return nil
	}
	clonedViteConfig := *ViteConfig
	return &clonedViteConfig
}

func cloneWatchConfig(watchConfig *WatchConfig) *WatchConfig {
	if watchConfig == nil {
		return nil
	}
	clonedWatchConfig := *watchConfig
	clonedWatchConfig.Include = cloneWatchedFiles(watchConfig.Include)
	clonedWatchConfig.Exclude.Dirs = append(
		[]string(nil),
		watchConfig.Exclude.Dirs...)
	clonedWatchConfig.Exclude.Files = append(
		[]string(nil),
		watchConfig.Exclude.Files...,
	)
	return &clonedWatchConfig
}

func cloneWatchedFiles(watchedFiles []wavewatch.WatchedFile) []wavewatch.WatchedFile {
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

func cloneSortedHooks(sortedHooks *wavewatch.SortedHooks) *wavewatch.SortedHooks {
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
