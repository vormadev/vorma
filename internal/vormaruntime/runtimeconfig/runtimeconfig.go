// Package runtimeconfig centralizes Vorma runtime configuration defaults and
// validation rules.
//
// Keeping normalization and validation here makes it explicit which config
// invariants the runtime depends on and allows other packages to reuse the same
// config semantics without duplicating logic.
package runtimeconfig

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveenv"
)

const (
	// DefaultDevReloadRoutesEndpointPath is the default dev endpoint for route
	// reloading.
	DefaultDevReloadRoutesEndpointPath = "/__vorma_internal/reload-routes"
	// DefaultDevReloadTemplateEndpointPath is the default dev endpoint for root
	// template reloading.
	DefaultDevReloadTemplateEndpointPath = "/__vorma_internal/reload-template"
	// DefaultTemplateDataKeyHeadElements is the default key for rendered head
	// elements in template data.
	DefaultTemplateDataKeyHeadElements = "VormaHeadEls"
	// DefaultTemplateDataKeyBodyScripts is the default key for body script tags
	// in template data.
	DefaultTemplateDataKeyBodyScripts = "VormaBodyScripts"
	// DefaultTemplateDataKeySSRScript is the default key for SSR inner HTML in
	// template data.
	DefaultTemplateDataKeySSRScript = "VormaSSRScript"
	// DefaultTemplateDataKeySSRScriptHash is the default key for the SSR script
	// SHA-256 hash in template data.
	DefaultTemplateDataKeySSRScriptHash = "VormaSSRScriptSha256Hash"
	// DefaultTemplateDataKeyRootElementID is the default key for the client root
	// element id in template data.
	DefaultTemplateDataKeyRootElementID = "VormaRootID"
	// DefaultClientRootElementID is the default DOM id expected by client mount
	// logic.
	DefaultClientRootElementID = "vorma-root"
	// DefaultBuildtimePublicURLFuncName is the default function name emitted by
	// build-time public URL generation.
	DefaultBuildtimePublicURLFuncName = "waveBuildtimeURL"
)

const (
	// UnresolvedRoutePolicyWarn logs unresolved route definitions.
	UnresolvedRoutePolicyWarn = "warn"
	// UnresolvedRoutePolicyError fails build/runtime flows on unresolved routes.
	UnresolvedRoutePolicyError = "error"
)

// UIVariant identifies the runtime UI adapter variant.
type UIVariant string

const (
	// UIVariantReact configures React runtime adapters.
	UIVariantReact UIVariant = "react"
	// UIVariantPreact configures Preact runtime adapters.
	UIVariantPreact UIVariant = "preact"
	// UIVariantSolid configures Solid runtime adapters.
	UIVariantSolid UIVariant = "solid"
)

// VormaConfigJSON is the raw JSON wire model for the `Vorma` section before
// parser normalization and filesystem-path resolution.
type VormaConfigJSON struct {
	// IncludeDefaults is raw Vorma.IncludeDefaults before parser defaulting.
	IncludeDefaults *bool `json:"IncludeDefaults,omitempty"`
	// MainBuildEntry is raw unresolved Vorma.MainBuildEntry from config JSON.
	MainBuildEntry string `json:"MainBuildEntry"`
	// UIVariant is raw Vorma.UIVariant before normalization.
	UIVariant string `json:"UIVariant"`
	// HTMLTemplateLocation is raw Vorma.HTMLTemplateLocation before parser checks.
	HTMLTemplateLocation string `json:"HTMLTemplateLocation"`
	// ClientEntry is raw unresolved Vorma.ClientEntry from config JSON.
	ClientEntry string `json:"ClientEntry"`
	// ClientRouteDefinitionPatterns are raw unresolved client route patterns.
	ClientRouteDefinitionPatterns []string `json:"ClientRouteDefinitionPatterns"`
	// ServerRouteDefinitionPatterns are raw unresolved server route patterns.
	ServerRouteDefinitionPatterns []string `json:"ServerRouteDefinitionPatterns,omitempty"`
	// TSGenOutDir is raw unresolved Vorma.TSGenOutDir from config JSON.
	TSGenOutDir string `json:"TSGenOutDir"`
	// BuildtimePublicURLFuncName is raw function name before parser defaulting.
	BuildtimePublicURLFuncName string `json:"BuildtimePublicURLFuncName,omitempty"`
	// UnresolvedRoutePolicy is raw policy text before normalization/validation.
	UnresolvedRoutePolicy string `json:"UnresolvedRoutePolicy,omitempty"`
	// DevReloadRoutesEndpointPath is raw endpoint path before defaulting/validation.
	DevReloadRoutesEndpointPath string `json:"DevReloadRoutesEndpointPath,omitempty"`
	// DevReloadTemplateEndpointPath is raw endpoint path before defaulting/validation.
	DevReloadTemplateEndpointPath string `json:"DevReloadTemplateEndpointPath,omitempty"`
	// TemplateDataKeyHeadElements is raw template key before defaulting/validation.
	TemplateDataKeyHeadElements string `json:"TemplateDataKeyHeadElements,omitempty"`
	// TemplateDataKeyBodyScripts is raw template key before defaulting/validation.
	TemplateDataKeyBodyScripts string `json:"TemplateDataKeyBodyScripts,omitempty"`
	// TemplateDataKeySSRScript is raw template key before defaulting/validation.
	TemplateDataKeySSRScript string `json:"TemplateDataKeySSRScript,omitempty"`
	// TemplateDataKeySSRScriptHash is raw template key before defaulting/validation.
	TemplateDataKeySSRScriptHash string `json:"TemplateDataKeySSRScriptHash,omitempty"`
	// TemplateDataKeyRootElementID is raw template key before defaulting/validation.
	TemplateDataKeyRootElementID string `json:"TemplateDataKeyRootElementID,omitempty"`
	// ClientRootElementID is raw DOM id before defaulting/validation.
	ClientRootElementID string `json:"ClientRootElementID,omitempty"`
}

// VormaConfig is an opaque parsed configuration handle returned by the parser.
// Callers cannot construct implementations directly.
type VormaConfig interface {
	// IncludeDefaults returns the configured IncludeDefaults value when present.
	// Nil means the field was omitted in config.
	IncludeDefaults() *bool
	// MainBuildEntry returns Vorma.MainBuildEntry normalized relative to effective
	// ResolveRoot in parsed-config path-basis semantics.
	MainBuildEntry() string
	// UIVariant returns the normalized adapter variant string.
	UIVariant() string
	// HTMLTemplateLocation returns a private-static-relative template path.
	HTMLTemplateLocation() string
	// ClientEntry returns Vorma.ClientEntry normalized relative to effective
	// ResolveRoot.
	ClientEntry() string
	// ClientRouteDefinitionPatterns returns glob patterns normalized relative to
	// effective ResolveRoot.
	ClientRouteDefinitionPatterns() []string
	// ServerRouteDefinitionPatterns returns glob patterns normalized relative to
	// effective ResolveRoot.
	ServerRouteDefinitionPatterns() []string
	// TSGenOutDir returns Vorma.TSGenOutDir normalized relative to effective
	// ResolveRoot.
	TSGenOutDir() string
	// BuildtimePublicURLFuncName returns the emitted build-time URL function
	// name after defaulting.
	BuildtimePublicURLFuncName() string
	// UnresolvedRoutePolicy returns normalized unresolved-route policy.
	UnresolvedRoutePolicy() string
	// DevReloadRoutesEndpointPath returns a normalized absolute path.
	DevReloadRoutesEndpointPath() string
	// DevReloadTemplateEndpointPath returns a normalized absolute path.
	DevReloadTemplateEndpointPath() string
	// TemplateDataKeyHeadElements returns a normalized template-data key.
	TemplateDataKeyHeadElements() string
	// TemplateDataKeyBodyScripts returns a normalized template-data key.
	TemplateDataKeyBodyScripts() string
	// TemplateDataKeySSRScript returns a normalized template-data key.
	TemplateDataKeySSRScript() string
	// TemplateDataKeySSRScriptHash returns a normalized template-data key.
	TemplateDataKeySSRScriptHash() string
	// TemplateDataKeyRootElementID returns a normalized template-data key.
	TemplateDataKeyRootElementID() string
	// ClientRootElementID returns a normalized client root element id.
	ClientRootElementID() string
	// Clone returns a defensive copy of this parsed Vorma config.
	Clone() VormaConfig

	// runtimeVormaConfigSeal prevents external implementations.
	runtimeVormaConfigSeal()
}

type vormaConfig struct {
	// includeDefaults is the raw optional pointer from Vorma.IncludeDefaults.
	includeDefaults *bool
	// mainBuildEntry is Vorma.MainBuildEntry normalized relative to effective
	// ResolveRoot.
	mainBuildEntry string
	// uiVariant is normalized from Vorma.UIVariant.
	uiVariant string
	// htmlTemplateLocation is relative under private static assets.
	htmlTemplateLocation string
	// clientEntry is Vorma.ClientEntry normalized relative to effective
	// ResolveRoot.
	clientEntry string
	// clientRouteDefinitionPatterns are normalized relative to effective
	// ResolveRoot.
	clientRouteDefinitionPatterns []string
	// serverRouteDefinitionPatterns are normalized relative to effective
	// ResolveRoot.
	serverRouteDefinitionPatterns []string
	// tsGenOutDir is Vorma.TSGenOutDir normalized relative to effective
	// ResolveRoot.
	tsGenOutDir string
	// buildtimePublicURLFuncName is defaulted/normalized parser output.
	buildtimePublicURLFuncName string
	// unresolvedRoutePolicy is lowercased parser output.
	unresolvedRoutePolicy string
	// devReloadRoutesEndpointPath is normalized absolute-path endpoint.
	devReloadRoutesEndpointPath string
	// devReloadTemplateEndpointPath is normalized absolute-path endpoint.
	devReloadTemplateEndpointPath string
	// templateDataKeyHeadElements is normalized parser output.
	templateDataKeyHeadElements string
	// templateDataKeyBodyScripts is normalized parser output.
	templateDataKeyBodyScripts string
	// templateDataKeySSRScript is normalized parser output.
	templateDataKeySSRScript string
	// templateDataKeySSRScriptHash is normalized parser output.
	templateDataKeySSRScriptHash string
	// templateDataKeyRootElementID is normalized parser output.
	templateDataKeyRootElementID string
	// clientRootElementID is normalized parser output.
	clientRootElementID string
}

func (config *vormaConfig) runtimeVormaConfigSeal() {}

func (config *vormaConfig) IncludeDefaults() *bool {
	if config == nil || config.includeDefaults == nil {
		return nil
	}
	includeDefaults := *config.includeDefaults
	return &includeDefaults
}

func (config *vormaConfig) MainBuildEntry() string {
	if config == nil {
		return ""
	}
	return config.mainBuildEntry
}

func (config *vormaConfig) UIVariant() string {
	if config == nil {
		return ""
	}
	return config.uiVariant
}

func (config *vormaConfig) HTMLTemplateLocation() string {
	if config == nil {
		return ""
	}
	return config.htmlTemplateLocation
}

func (config *vormaConfig) ClientEntry() string {
	if config == nil {
		return ""
	}
	return config.clientEntry
}

func (config *vormaConfig) ClientRouteDefinitionPatterns() []string {
	if config == nil {
		return nil
	}
	return append([]string(nil), config.clientRouteDefinitionPatterns...)
}

func (config *vormaConfig) ServerRouteDefinitionPatterns() []string {
	if config == nil {
		return nil
	}
	return append([]string(nil), config.serverRouteDefinitionPatterns...)
}

func (config *vormaConfig) TSGenOutDir() string {
	if config == nil {
		return ""
	}
	return config.tsGenOutDir
}

func (config *vormaConfig) BuildtimePublicURLFuncName() string {
	if config == nil {
		return ""
	}
	return config.buildtimePublicURLFuncName
}

func (config *vormaConfig) UnresolvedRoutePolicy() string {
	if config == nil {
		return ""
	}
	return config.unresolvedRoutePolicy
}

func (config *vormaConfig) DevReloadRoutesEndpointPath() string {
	if config == nil {
		return ""
	}
	return config.devReloadRoutesEndpointPath
}

func (config *vormaConfig) DevReloadTemplateEndpointPath() string {
	if config == nil {
		return ""
	}
	return config.devReloadTemplateEndpointPath
}

func (config *vormaConfig) TemplateDataKeyHeadElements() string {
	if config == nil {
		return ""
	}
	return config.templateDataKeyHeadElements
}

func (config *vormaConfig) TemplateDataKeyBodyScripts() string {
	if config == nil {
		return ""
	}
	return config.templateDataKeyBodyScripts
}

func (config *vormaConfig) TemplateDataKeySSRScript() string {
	if config == nil {
		return ""
	}
	return config.templateDataKeySSRScript
}

func (config *vormaConfig) TemplateDataKeySSRScriptHash() string {
	if config == nil {
		return ""
	}
	return config.templateDataKeySSRScriptHash
}

func (config *vormaConfig) TemplateDataKeyRootElementID() string {
	if config == nil {
		return ""
	}
	return config.templateDataKeyRootElementID
}

func (config *vormaConfig) ClientRootElementID() string {
	if config == nil {
		return ""
	}
	return config.clientRootElementID
}

func (config *vormaConfig) Clone() VormaConfig {
	if config == nil {
		return nil
	}
	clonedConfig := *config
	if config.includeDefaults != nil {
		includeDefaults := *config.includeDefaults
		clonedConfig.includeDefaults = &includeDefaults
	}
	clonedConfig.clientRouteDefinitionPatterns = append(
		[]string(nil),
		config.clientRouteDefinitionPatterns...,
	)
	clonedConfig.serverRouteDefinitionPatterns = append(
		[]string(nil),
		config.serverRouteDefinitionPatterns...,
	)
	return &clonedConfig
}

type configWrapper struct {
	Vorma *VormaConfigJSON `json:"Vorma,omitempty"`
}

// ParseVormaConfigJSON parses the Vorma config block, applies parser
// normalization/defaulting, and resolves filesystem paths relative to
// parsedWaveConfig.ResolveRoot() in parsed-config path-basis semantics.
func ParseVormaConfigJSON(
	rawWaveConfigJSON []byte,
	parsedWaveConfig waveconfig.ParsedConfig,
) (VormaConfig, error) {
	if parsedWaveConfig == nil || parsedWaveConfig.Core() == nil {
		return nil, fmt.Errorf("parsed wave config with Core section is required")
	}
	effectiveRootPath := parsedWaveConfig.ResolveRoot()

	var wrapper configWrapper
	if err := json.Unmarshal(rawWaveConfigJSON, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse Vorma config: %w", err)
	}
	if wrapper.Vorma == nil {
		wrapper.Vorma = &VormaConfigJSON{}
	}

	if err := NormalizeAndValidateMutableConfig(
		MutableValidationConfig{
			MainBuildEntry:                &wrapper.Vorma.MainBuildEntry,
			UIVariant:                     &wrapper.Vorma.UIVariant,
			HTMLTemplateLocation:          &wrapper.Vorma.HTMLTemplateLocation,
			ClientEntry:                   &wrapper.Vorma.ClientEntry,
			ClientRouteDefinitionPatterns: &wrapper.Vorma.ClientRouteDefinitionPatterns,
			TSGenOutDir:                   &wrapper.Vorma.TSGenOutDir,
			BuildtimePublicURLFuncName:    &wrapper.Vorma.BuildtimePublicURLFuncName,
			UnresolvedRoutePolicy:         &wrapper.Vorma.UnresolvedRoutePolicy,
			DevReloadRoutesEndpointPath:   &wrapper.Vorma.DevReloadRoutesEndpointPath,
			DevReloadTemplateEndpointPath: &wrapper.Vorma.DevReloadTemplateEndpointPath,
			TemplateDataKeyHeadElements:   &wrapper.Vorma.TemplateDataKeyHeadElements,
			TemplateDataKeyBodyScripts:    &wrapper.Vorma.TemplateDataKeyBodyScripts,
			TemplateDataKeySSRScript:      &wrapper.Vorma.TemplateDataKeySSRScript,
			TemplateDataKeySSRScriptHash:  &wrapper.Vorma.TemplateDataKeySSRScriptHash,
			TemplateDataKeyRootElementID:  &wrapper.Vorma.TemplateDataKeyRootElementID,
			ClientRootElementID:           &wrapper.Vorma.ClientRootElementID,
		},
	); err != nil {
		return nil, err
	}
	if resolvePathsError := resolveConfigRelativeFilesystemPaths(
		wrapper.Vorma,
		effectiveRootPath,
	); resolvePathsError != nil {
		return nil, resolvePathsError
	}

	return &vormaConfig{
		includeDefaults:               cloneBoolPointer(wrapper.Vorma.IncludeDefaults),
		mainBuildEntry:                wrapper.Vorma.MainBuildEntry,
		uiVariant:                     wrapper.Vorma.UIVariant,
		htmlTemplateLocation:          wrapper.Vorma.HTMLTemplateLocation,
		clientEntry:                   wrapper.Vorma.ClientEntry,
		clientRouteDefinitionPatterns: append([]string(nil), wrapper.Vorma.ClientRouteDefinitionPatterns...),
		serverRouteDefinitionPatterns: append([]string(nil), wrapper.Vorma.ServerRouteDefinitionPatterns...),
		tsGenOutDir:                   wrapper.Vorma.TSGenOutDir,
		buildtimePublicURLFuncName:    wrapper.Vorma.BuildtimePublicURLFuncName,
		unresolvedRoutePolicy:         wrapper.Vorma.UnresolvedRoutePolicy,
		devReloadRoutesEndpointPath:   wrapper.Vorma.DevReloadRoutesEndpointPath,
		devReloadTemplateEndpointPath: wrapper.Vorma.DevReloadTemplateEndpointPath,
		templateDataKeyHeadElements:   wrapper.Vorma.TemplateDataKeyHeadElements,
		templateDataKeyBodyScripts:    wrapper.Vorma.TemplateDataKeyBodyScripts,
		templateDataKeySSRScript:      wrapper.Vorma.TemplateDataKeySSRScript,
		templateDataKeySSRScriptHash:  wrapper.Vorma.TemplateDataKeySSRScriptHash,
		templateDataKeyRootElementID:  wrapper.Vorma.TemplateDataKeyRootElementID,
		clientRootElementID:           wrapper.Vorma.ClientRootElementID,
	}, nil
}

func resolveConfigRelativeFilesystemPaths(
	config *VormaConfigJSON,
	effectiveResolveRootPath string,
) error {
	if config == nil {
		return nil
	}
	trimmedEffectiveResolveRootPath := strings.TrimSpace(
		effectiveResolveRootPath,
	)
	if trimmedEffectiveResolveRootPath == "" {
		trimmedEffectiveResolveRootPath = "."
	}

	var resolveError error
	config.MainBuildEntry, resolveError = resolvePathRelativeToResolveRoot(
		trimmedEffectiveResolveRootPath,
		config.MainBuildEntry,
		"Vorma.MainBuildEntry",
	)
	if resolveError != nil {
		return resolveError
	}
	config.ClientEntry, resolveError = resolvePathRelativeToResolveRoot(
		trimmedEffectiveResolveRootPath,
		config.ClientEntry,
		"Vorma.ClientEntry",
	)
	if resolveError != nil {
		return resolveError
	}
	config.TSGenOutDir, resolveError = resolvePathRelativeToResolveRoot(
		trimmedEffectiveResolveRootPath,
		config.TSGenOutDir,
		"Vorma.TSGenOutDir",
	)
	if resolveError != nil {
		return resolveError
	}
	for serverPatternIndex, serverPattern := range config.ServerRouteDefinitionPatterns {
		config.ServerRouteDefinitionPatterns[serverPatternIndex], resolveError = resolvePathRelativeToResolveRoot(
			trimmedEffectiveResolveRootPath,
			serverPattern,
			fmt.Sprintf(
				"Vorma.ServerRouteDefinitionPatterns[%d]",
				serverPatternIndex,
			),
		)
		if resolveError != nil {
			return resolveError
		}
	}
	for clientPatternIndex, clientPattern := range config.ClientRouteDefinitionPatterns {
		config.ClientRouteDefinitionPatterns[clientPatternIndex], resolveError = resolvePathRelativeToResolveRoot(
			trimmedEffectiveResolveRootPath,
			clientPattern,
			fmt.Sprintf(
				"Vorma.ClientRouteDefinitionPatterns[%d]",
				clientPatternIndex,
			),
		)
		if resolveError != nil {
			return resolveError
		}
	}
	return nil
}

func resolvePathRelativeToResolveRoot(
	effectiveResolveRootPath string,
	configuredPath string,
	fieldPath string,
) (string, error) {
	trimmedConfiguredPath := strings.TrimSpace(configuredPath)
	if trimmedConfiguredPath == "" {
		return "", nil
	}
	if waveenv.IsMachineAbsoluteFilesystemPath(trimmedConfiguredPath) {
		return "", fmt.Errorf(
			"%s must not be machine-absolute: %q",
			fieldPath,
			trimmedConfiguredPath,
		)
	}
	return filepath.Clean(
		filepath.Join(effectiveResolveRootPath, trimmedConfiguredPath),
	), nil
}

// NormalizePathOrPatternToResolveRootRelative converts a parsed absolute (or
// already-relative) path/pattern into resolve-root-relative slash form suitable
// for Vite/route artifact contract surfaces.
func NormalizePathOrPatternToResolveRootRelative(
	resolveRootPath string,
	pathOrPattern string,
) (string, error) {
	trimmedPathOrPattern := strings.TrimSpace(pathOrPattern)
	if trimmedPathOrPattern == "" {
		return "", nil
	}

	cleanPathOrPattern := filepath.Clean(trimmedPathOrPattern)
	if !filepath.IsAbs(cleanPathOrPattern) {
		return filepath.ToSlash(cleanPathOrPattern), nil
	}

	trimmedResolveRootPath := strings.TrimSpace(resolveRootPath)
	if trimmedResolveRootPath == "" {
		return "", fmt.Errorf(
			"resolve root is required to relativize absolute path %q",
			cleanPathOrPattern,
		)
	}
	cleanResolveRootPath := filepath.Clean(trimmedResolveRootPath)

	resolveRootRelativePathOrPattern, relativeError := filepath.Rel(
		cleanResolveRootPath,
		cleanPathOrPattern,
	)
	if relativeError != nil {
		return "", fmt.Errorf(
			"make %q relative to resolve root %q: %w",
			cleanPathOrPattern,
			cleanResolveRootPath,
			relativeError,
		)
	}

	cleanResolveRootRelativePathOrPattern := filepath.Clean(
		resolveRootRelativePathOrPattern,
	)
	if cleanResolveRootRelativePathOrPattern == ".." ||
		strings.HasPrefix(
			cleanResolveRootRelativePathOrPattern,
			".."+string(filepath.Separator),
		) {
		return "", fmt.Errorf(
			"path %q escapes resolve root %q",
			cleanPathOrPattern,
			cleanResolveRootPath,
		)
	}

	return filepath.ToSlash(cleanResolveRootRelativePathOrPattern), nil
}

// MutableValidationConfig provides pointer access to mutable config fields that
// are normalized and validated together.
type MutableValidationConfig struct {
	MainBuildEntry                *string
	UIVariant                     *string
	HTMLTemplateLocation          *string
	ClientEntry                   *string
	ClientRouteDefinitionPatterns *[]string
	TSGenOutDir                   *string
	BuildtimePublicURLFuncName    *string
	UnresolvedRoutePolicy         *string
	DevReloadRoutesEndpointPath   *string
	DevReloadTemplateEndpointPath *string
	TemplateDataKeyHeadElements   *string
	TemplateDataKeyBodyScripts    *string
	TemplateDataKeySSRScript      *string
	TemplateDataKeySSRScriptHash  *string
	TemplateDataKeyRootElementID  *string
	ClientRootElementID           *string
}

// ResolveDevReloadRoutesEndpointPath resolves configured endpoint path fallback.
func ResolveDevReloadRoutesEndpointPath(configuredPath string) string {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" {
		return DefaultDevReloadRoutesEndpointPath
	}
	return configuredPath
}

// ResolveDevReloadTemplateEndpointPath resolves configured endpoint path fallback.
func ResolveDevReloadTemplateEndpointPath(configuredPath string) string {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" {
		return DefaultDevReloadTemplateEndpointPath
	}
	return configuredPath
}

// ResolveTemplateDataKeyHeadElements resolves configured template key fallback.
func ResolveTemplateDataKeyHeadElements(configuredKey string) string {
	configuredKey = strings.TrimSpace(configuredKey)
	if configuredKey == "" {
		return DefaultTemplateDataKeyHeadElements
	}
	return configuredKey
}

// ResolveTemplateDataKeyBodyScripts resolves configured template key fallback.
func ResolveTemplateDataKeyBodyScripts(configuredKey string) string {
	configuredKey = strings.TrimSpace(configuredKey)
	if configuredKey == "" {
		return DefaultTemplateDataKeyBodyScripts
	}
	return configuredKey
}

// ResolveTemplateDataKeySSRScript resolves configured template key fallback.
func ResolveTemplateDataKeySSRScript(configuredKey string) string {
	configuredKey = strings.TrimSpace(configuredKey)
	if configuredKey == "" {
		return DefaultTemplateDataKeySSRScript
	}
	return configuredKey
}

// ResolveTemplateDataKeySSRScriptHash resolves configured template key fallback.
func ResolveTemplateDataKeySSRScriptHash(configuredKey string) string {
	configuredKey = strings.TrimSpace(configuredKey)
	if configuredKey == "" {
		return DefaultTemplateDataKeySSRScriptHash
	}
	return configuredKey
}

// ResolveTemplateDataKeyRootElementID resolves configured template key fallback.
func ResolveTemplateDataKeyRootElementID(configuredKey string) string {
	configuredKey = strings.TrimSpace(configuredKey)
	if configuredKey == "" {
		return DefaultTemplateDataKeyRootElementID
	}
	return configuredKey
}

// ResolveClientRootElementID resolves configured client root element fallback.
func ResolveClientRootElementID(configuredRootElementID string) string {
	configuredRootElementID = strings.TrimSpace(configuredRootElementID)
	if configuredRootElementID == "" {
		return DefaultClientRootElementID
	}
	return configuredRootElementID
}

// NormalizeAndValidateClientRouteDefinitionPatternsInInputOrder validates and
// returns route definition patterns in stable input order with duplicates
// rejected.
func NormalizeAndValidateClientRouteDefinitionPatternsInInputOrder(
	routeDefinitionPatterns []string,
) ([]string, error) {
	normalizedPatterns := make([]string, 0, len(routeDefinitionPatterns))
	seenPatterns := make(map[string]struct{}, len(routeDefinitionPatterns))
	for index, routeDefinitionPattern := range routeDefinitionPatterns {
		trimmedRouteDefinitionPattern := strings.TrimSpace(
			routeDefinitionPattern,
		)
		if trimmedRouteDefinitionPattern == "" {
			return nil, fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d] cannot be empty or whitespace",
				index,
			)
		}
		if trimmedRouteDefinitionPattern != routeDefinitionPattern {
			return nil, fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d]=%q must not contain surrounding whitespace",
				index,
				routeDefinitionPattern,
			)
		}
		if _, hasSeenPattern := seenPatterns[trimmedRouteDefinitionPattern]; hasSeenPattern {
			return nil, fmt.Errorf(
				"Vorma.ClientRouteDefinitionPatterns[%d]=%q duplicates an earlier pattern",
				index,
				trimmedRouteDefinitionPattern,
			)
		}

		seenPatterns[trimmedRouteDefinitionPattern] = struct{}{}
		normalizedPatterns = append(
			normalizedPatterns,
			trimmedRouteDefinitionPattern,
		)
	}
	return normalizedPatterns, nil
}

// NormalizeAndValidateMutableConfig applies defaults, trims, and validates
// mutable config fields in-place.
func NormalizeAndValidateMutableConfig(config MutableValidationConfig) error {
	if err := validateMutableConfigPointers(config); err != nil {
		return err
	}

	if *config.MainBuildEntry == "" {
		return fmt.Errorf("config: Vorma.MainBuildEntry is required")
	}
	if *config.UIVariant == "" {
		return fmt.Errorf("config: Vorma.UIVariant is required")
	}
	if *config.HTMLTemplateLocation == "" {
		return fmt.Errorf("config: Vorma.HTMLTemplateLocation is required")
	}
	if *config.ClientEntry == "" {
		return fmt.Errorf("config: Vorma.ClientEntry is required")
	}
	if len(*config.ClientRouteDefinitionPatterns) == 0 {
		return fmt.Errorf(
			"config: Vorma.ClientRouteDefinitionPatterns is required",
		)
	}
	normalizedRouteDefinitionPatterns, normalizedRouteDefinitionPatternsErr := NormalizeAndValidateClientRouteDefinitionPatternsInInputOrder(
		*config.ClientRouteDefinitionPatterns,
	)
	if normalizedRouteDefinitionPatternsErr != nil {
		return fmt.Errorf("config: %w", normalizedRouteDefinitionPatternsErr)
	}
	*config.ClientRouteDefinitionPatterns = normalizedRouteDefinitionPatterns
	if *config.TSGenOutDir == "" {
		return fmt.Errorf("config: Vorma.TSGenOutDir is required")
	}

	applyDefaultConfigStringValue(
		config.BuildtimePublicURLFuncName,
		DefaultBuildtimePublicURLFuncName,
	)

	trimConfigStringValue(config.UnresolvedRoutePolicy)
	*config.UnresolvedRoutePolicy = strings.ToLower(
		*config.UnresolvedRoutePolicy,
	)
	if !isValidUnresolvedRoutePolicy(*config.UnresolvedRoutePolicy) {
		return fmt.Errorf(
			`config: Vorma.UnresolvedRoutePolicy must be "warn" or "error" when set`,
		)
	}

	applyDefaultConfigStringValue(
		config.DevReloadRoutesEndpointPath,
		DefaultDevReloadRoutesEndpointPath,
	)
	applyDefaultConfigStringValue(
		config.DevReloadTemplateEndpointPath,
		DefaultDevReloadTemplateEndpointPath,
	)

	trimConfigStringValue(config.DevReloadRoutesEndpointPath)
	trimConfigStringValue(config.DevReloadTemplateEndpointPath)
	if !strings.HasPrefix(*config.DevReloadRoutesEndpointPath, "/") {
		return fmt.Errorf(
			"config: Vorma.DevReloadRoutesEndpointPath must start with '/'",
		)
	}
	if !strings.HasPrefix(*config.DevReloadTemplateEndpointPath, "/") {
		return fmt.Errorf(
			"config: Vorma.DevReloadTemplateEndpointPath must start with '/'",
		)
	}
	if *config.DevReloadRoutesEndpointPath == *config.DevReloadTemplateEndpointPath {
		return fmt.Errorf(
			"config: Vorma.DevReloadRoutesEndpointPath and Vorma.DevReloadTemplateEndpointPath must differ",
		)
	}

	applyDefaultAndTrimConfigStringValue(
		config.TemplateDataKeyHeadElements,
		DefaultTemplateDataKeyHeadElements,
	)
	applyDefaultAndTrimConfigStringValue(
		config.TemplateDataKeyBodyScripts,
		DefaultTemplateDataKeyBodyScripts,
	)
	applyDefaultAndTrimConfigStringValue(
		config.TemplateDataKeySSRScript,
		DefaultTemplateDataKeySSRScript,
	)
	applyDefaultAndTrimConfigStringValue(
		config.TemplateDataKeySSRScriptHash,
		DefaultTemplateDataKeySSRScriptHash,
	)
	applyDefaultAndTrimConfigStringValue(
		config.TemplateDataKeyRootElementID,
		DefaultTemplateDataKeyRootElementID,
	)
	applyDefaultAndTrimConfigStringValue(
		config.ClientRootElementID,
		DefaultClientRootElementID,
	)

	templateDataKeys := []string{
		*config.TemplateDataKeyHeadElements,
		*config.TemplateDataKeyBodyScripts,
		*config.TemplateDataKeySSRScript,
		*config.TemplateDataKeySSRScriptHash,
		*config.TemplateDataKeyRootElementID,
	}
	for _, templateDataKey := range templateDataKeys {
		if templateDataKey == "" {
			return fmt.Errorf(
				"config: Vorma template data keys must be non-empty",
			)
		}
	}
	seenTemplateDataKeys := make(map[string]struct{}, len(templateDataKeys))
	for _, templateDataKey := range templateDataKeys {
		if _, found := seenTemplateDataKeys[templateDataKey]; found {
			return fmt.Errorf("config: Vorma template data keys must be unique")
		}
		seenTemplateDataKeys[templateDataKey] = struct{}{}
	}
	if *config.ClientRootElementID == "" {
		return fmt.Errorf("config: Vorma.ClientRootElementID is required")
	}

	return nil
}

func validateMutableConfigPointers(config MutableValidationConfig) error {
	if config.MainBuildEntry == nil {
		return fmt.Errorf("config pointer for MainBuildEntry is required")
	}
	if config.UIVariant == nil {
		return fmt.Errorf("config pointer for UIVariant is required")
	}
	if config.HTMLTemplateLocation == nil {
		return fmt.Errorf("config pointer for HTMLTemplateLocation is required")
	}
	if config.ClientEntry == nil {
		return fmt.Errorf("config pointer for ClientEntry is required")
	}
	if config.ClientRouteDefinitionPatterns == nil {
		return fmt.Errorf(
			"config pointer for ClientRouteDefinitionPatterns is required",
		)
	}
	if config.TSGenOutDir == nil {
		return fmt.Errorf("config pointer for TSGenOutDir is required")
	}
	if config.BuildtimePublicURLFuncName == nil {
		return fmt.Errorf(
			"config pointer for BuildtimePublicURLFuncName is required",
		)
	}
	if config.UnresolvedRoutePolicy == nil {
		return fmt.Errorf(
			"config pointer for UnresolvedRoutePolicy is required",
		)
	}
	if config.DevReloadRoutesEndpointPath == nil {
		return fmt.Errorf(
			"config pointer for DevReloadRoutesEndpointPath is required",
		)
	}
	if config.DevReloadTemplateEndpointPath == nil {
		return fmt.Errorf(
			"config pointer for DevReloadTemplateEndpointPath is required",
		)
	}
	if config.TemplateDataKeyHeadElements == nil {
		return fmt.Errorf(
			"config pointer for TemplateDataKeyHeadElements is required",
		)
	}
	if config.TemplateDataKeyBodyScripts == nil {
		return fmt.Errorf(
			"config pointer for TemplateDataKeyBodyScripts is required",
		)
	}
	if config.TemplateDataKeySSRScript == nil {
		return fmt.Errorf(
			"config pointer for TemplateDataKeySSRScript is required",
		)
	}
	if config.TemplateDataKeySSRScriptHash == nil {
		return fmt.Errorf(
			"config pointer for TemplateDataKeySSRScriptHash is required",
		)
	}
	if config.TemplateDataKeyRootElementID == nil {
		return fmt.Errorf(
			"config pointer for TemplateDataKeyRootElementID is required",
		)
	}
	if config.ClientRootElementID == nil {
		return fmt.Errorf("config pointer for ClientRootElementID is required")
	}
	return nil
}

func applyDefaultConfigStringValue(configField *string, defaultValue string) {
	if configField == nil {
		return
	}
	if *configField == "" {
		*configField = defaultValue
	}
}

func trimConfigStringValue(configField *string) {
	if configField == nil {
		return
	}
	*configField = strings.TrimSpace(*configField)
}

func applyDefaultAndTrimConfigStringValue(
	configField *string,
	defaultValue string,
) {
	applyDefaultConfigStringValue(configField, defaultValue)
	trimConfigStringValue(configField)
}

func isValidUnresolvedRoutePolicy(policy string) bool {
	if policy == "" {
		return true
	}

	return policy == UnresolvedRoutePolicyWarn ||
		policy == UnresolvedRoutePolicyError
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	clonedValue := *value
	return &clonedValue
}
