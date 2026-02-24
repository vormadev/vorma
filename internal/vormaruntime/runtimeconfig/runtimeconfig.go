// Package runtimeconfig centralizes Vorma runtime configuration defaults and
// validation rules.
//
// Keeping normalization and validation here makes it explicit which config
// invariants the runtime depends on and allows other packages to reuse the same
// config semantics without duplicating logic.
package runtimeconfig

import (
	"fmt"
	"strings"
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
	for _, pattern := range *config.ClientRouteDefinitionPatterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf(
				"config: Vorma.ClientRouteDefinitionPatterns cannot contain empty entries",
			)
		}
	}
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
