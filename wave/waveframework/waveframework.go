// Package waveframework provides framework/tooling-facing helpers that operate
// on Wave runtime instances without expanding the end-user `wave` API surface.
package waveframework

import (
	"context"
	"strings"
	"sync"

	"github.com/vormadev/vorma/lab/jsonschema"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/wavewatch"
)

// ViteFileMapChangedNotifyEndpointPath is the Vite devserver endpoint path
// used by Wave to notify that the public filemap has changed.
const ViteFileMapChangedNotifyEndpointPath = "/__wave_notify_filemap_changed"

// FrameworkRuntimeReloadAttemptIDHeaderName is the request header carrying
// framework runtime reload attempt identifier.
const FrameworkRuntimeReloadAttemptIDHeaderName = "X-Wave-Framework-Reload-Attempt-Id"

// FrameworkRuntimeReloadExpectedBuildIDHeaderName is the request header
// carrying expected framework runtime build identifier.
const FrameworkRuntimeReloadExpectedBuildIDHeaderName = "X-Wave-Framework-Reload-Expected-Build-Id"

// FrameworkRuntimeReloadTriggerHeaderName is the request header carrying
// framework runtime reload trigger identifier.
const FrameworkRuntimeReloadTriggerHeaderName = "X-Wave-Framework-Reload-Trigger"

const (
	defaultBrowserRuntimeNamespace              = "__wave"
	defaultBrowserPublicURLResolverFunctionName = "getPublicURL"
	defaultBrowserRevalidateFunctionName        = "__waveRevalidate"
	defaultRefreshRebuildingOverlayElementID    = "wave-refreshscript-rebuilding"
	defaultCriticalCSSStyleElementID            = "wave-critical-css"
	defaultNonCriticalCSSLinkElementID          = "wave-normal-css"
)

// GoBuildOverlay describes a temporary overlay used for Go build execution.
type GoBuildOverlay struct {
	OverlayConfigPath string
	Cleanup           func() error
}

// ConfigState stores framework-owned runtime/buildtime mutable state for one
// parsed Wave config.
type ConfigState struct {
	WatchPatterns                        []wavewatch.WatchedFile
	IgnoredPatterns                      []string
	SchemaExtensions                     map[string]jsonschema.Entry
	DevBuildHook                         string
	ProdBuildHook                        string
	RunBuildHook                         func(context.Context, bool) error
	PrepareGoBuildOverlay                func() (*GoBuildOverlay, error)
	ConfigureForToolingReload            func(waveconfig.ParsedConfig, []byte) error
	PublicFileMapReloadEndpointPath      string
	BrowserRuntimeNamespace              string
	BrowserPublicURLResolverFunctionName string
	BrowserRevalidateFunctionName        string
	RefreshRebuildingOverlayElementID    string
	CriticalCSSStyleElementID            string
	NonCriticalCSSLinkElementID          string
}

var configStateByParsedConfig sync.Map

// StateForConfig returns mutable framework state for one parsed config.
func StateForConfig(parsedConfig waveconfig.ParsedConfig) *ConfigState {
	if parsedConfig == nil {
		return nil
	}
	existingState, hasExistingState := configStateByParsedConfig.Load(
		parsedConfig,
	)
	if hasExistingState {
		statePointer, _ := existingState.(*ConfigState)
		if statePointer != nil {
			return statePointer
		}
	}
	newState := &ConfigState{}
	storedState, _ := configStateByParsedConfig.LoadOrStore(
		parsedConfig,
		newState,
	)
	statePointer, _ := storedState.(*ConfigState)
	if statePointer == nil {
		return newState
	}
	return statePointer
}

// CopyRuntimeStateForToolingReload transfers framework state from source to
// target when config is reparsed from disk.
func CopyRuntimeStateForToolingReload(
	target waveconfig.ParsedConfig,
	source waveconfig.ParsedConfig,
) {
	if target == nil || source == nil || target == source {
		return
	}
	storedState, hasState := configStateByParsedConfig.Load(source)
	if !hasState {
		return
	}
	statePointer, _ := storedState.(*ConfigState)
	if statePointer == nil {
		configStateByParsedConfig.Delete(target)
		return
	}
	configStateByParsedConfig.Store(target, cloneConfigState(statePointer))
}

// ConfigureReloadedConfigForTooling applies framework-specific config reload
// wiring for one parsed config reload transition when configured by the
// framework.
func ConfigureReloadedConfigForTooling(
	source waveconfig.ParsedConfig,
	target waveconfig.ParsedConfig,
	rawConfigJSON []byte,
) error {
	if source == nil || target == nil {
		return nil
	}
	configState := StateForConfig(source)
	if configState == nil || configState.ConfigureForToolingReload == nil {
		return nil
	}
	return configState.ConfigureForToolingReload(target, rawConfigJSON)
}

func cloneConfigState(source *ConfigState) *ConfigState {
	if source == nil {
		return nil
	}
	clonedState := *source
	clonedState.WatchPatterns = cloneWatchedFiles(source.WatchPatterns)
	clonedState.IgnoredPatterns = append([]string(nil), source.IgnoredPatterns...)
	if source.SchemaExtensions != nil {
		clonedState.SchemaExtensions = make(
			map[string]jsonschema.Entry,
			len(source.SchemaExtensions),
		)
		for schemaExtensionKey, schemaExtensionEntry := range source.SchemaExtensions {
			clonedState.SchemaExtensions[schemaExtensionKey] = schemaExtensionEntry
		}
	}
	return &clonedState
}

func cloneWatchedFiles(source []wavewatch.WatchedFile) []wavewatch.WatchedFile {
	if len(source) == 0 {
		return nil
	}
	clonedWatchPatterns := make([]wavewatch.WatchedFile, 0, len(source))
	for _, watchedFile := range source {
		clonedWatchPatterns = append(clonedWatchPatterns, cloneWatchedFile(watchedFile))
	}
	return clonedWatchPatterns
}

func cloneWatchedFile(source wavewatch.WatchedFile) wavewatch.WatchedFile {
	clonedWatchPattern := source
	clonedWatchPattern.OnChangeHooks = cloneOnChangeHooks(source.OnChangeHooks)
	clonedWatchPattern.SortedHooks = cloneSortedHooks(source.SortedHooks)
	return clonedWatchPattern
}

func cloneOnChangeHooks(source []wavewatch.OnChangeHook) []wavewatch.OnChangeHook {
	if len(source) == 0 {
		return nil
	}
	clonedHooks := make([]wavewatch.OnChangeHook, 0, len(source))
	for _, sourceHook := range source {
		clonedHook := sourceHook
		clonedHook.Exclude = append([]string(nil), sourceHook.Exclude...)
		clonedHooks = append(clonedHooks, clonedHook)
	}
	return clonedHooks
}

func cloneSortedHooks(source *wavewatch.SortedHooks) *wavewatch.SortedHooks {
	if source == nil {
		return nil
	}
	return &wavewatch.SortedHooks{
		Pre:              cloneOnChangeHooks(source.Pre),
		Concurrent:       cloneOnChangeHooks(source.Concurrent),
		ConcurrentNoWait: cloneOnChangeHooks(source.ConcurrentNoWait),
		Post:             cloneOnChangeHooks(source.Post),
	}
}

// BrowserRuntimeNamespace returns the browser global namespace used by runtime
// integration scripts.
func BrowserRuntimeNamespace(parsedConfig waveconfig.ParsedConfig) string {
	state := StateForConfig(parsedConfig)
	if state == nil || strings.TrimSpace(state.BrowserRuntimeNamespace) == "" {
		return defaultBrowserRuntimeNamespace
	}
	return state.BrowserRuntimeNamespace
}

// BrowserPublicURLResolverFunctionName returns the browser helper function name
// used for resolving public asset URLs.
func BrowserPublicURLResolverFunctionName(
	parsedConfig waveconfig.ParsedConfig,
) string {
	state := StateForConfig(parsedConfig)
	if state == nil ||
		strings.TrimSpace(state.BrowserPublicURLResolverFunctionName) == "" {
		return defaultBrowserPublicURLResolverFunctionName
	}
	return state.BrowserPublicURLResolverFunctionName
}

// BrowserRevalidateFunctionName returns the browser helper function name used
// for route revalidation.
func BrowserRevalidateFunctionName(
	parsedConfig waveconfig.ParsedConfig,
) string {
	state := StateForConfig(parsedConfig)
	if state == nil ||
		strings.TrimSpace(state.BrowserRevalidateFunctionName) == "" {
		return defaultBrowserRevalidateFunctionName
	}
	return state.BrowserRevalidateFunctionName
}

// RefreshRebuildingOverlayElementID returns the DOM element id used for the
// rebuild overlay during dev refresh cycles.
func RefreshRebuildingOverlayElementID(
	parsedConfig waveconfig.ParsedConfig,
) string {
	state := StateForConfig(parsedConfig)
	if state == nil ||
		strings.TrimSpace(state.RefreshRebuildingOverlayElementID) == "" {
		return defaultRefreshRebuildingOverlayElementID
	}
	return state.RefreshRebuildingOverlayElementID
}

// CriticalCSSStyleElementID returns the DOM element id used for injected
// critical CSS.
func CriticalCSSStyleElementID(parsedConfig waveconfig.ParsedConfig) string {
	state := StateForConfig(parsedConfig)
	if state == nil ||
		strings.TrimSpace(state.CriticalCSSStyleElementID) == "" {
		return defaultCriticalCSSStyleElementID
	}
	return state.CriticalCSSStyleElementID
}

// NonCriticalCSSLinkElementID returns the DOM element id used for injected
// non-critical stylesheet links.
func NonCriticalCSSLinkElementID(parsedConfig waveconfig.ParsedConfig) string {
	state := StateForConfig(parsedConfig)
	if state == nil ||
		strings.TrimSpace(state.NonCriticalCSSLinkElementID) == "" {
		return defaultNonCriticalCSSLinkElementID
	}
	return state.NonCriticalCSSLinkElementID
}
