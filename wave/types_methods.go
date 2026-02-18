package wave

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
)

// Sort partitions hook entries by execution timing for efficient watch
// pipeline planning.
func (wf *WatchedFile) Sort() {
	if wf.SortedHooks != nil {
		return
	}

	wf.SortedHooks = &SortedHooks{}
	for _, onChangeHook := range wf.OnChangeHooks {
		switch onChangeHook.Timing {
		case OnChangeStrategyPost:
			wf.SortedHooks.Post = append(wf.SortedHooks.Post, onChangeHook)
		case OnChangeStrategyConcurrent:
			wf.SortedHooks.Concurrent = append(
				wf.SortedHooks.Concurrent,
				onChangeHook,
			)
		case OnChangeStrategyConcurrentNoWait:
			wf.SortedHooks.ConcurrentNoWait = append(
				wf.SortedHooks.ConcurrentNoWait,
				onChangeHook,
			)
		default:
			wf.SortedHooks.Pre = append(wf.SortedHooks.Pre, onChangeHook)
		}
	}
}

// Merge combines two RefreshActions with OR semantics.
// TriggerRestart takes precedence over browser reload.
func (refreshAction RefreshAction) Merge(other RefreshAction) RefreshAction {
	return RefreshAction{
		ReloadBrowser:  refreshAction.ReloadBrowser || other.ReloadBrowser,
		WaitForApp:     refreshAction.WaitForApp || other.WaitForApp,
		WaitForVite:    refreshAction.WaitForVite || other.WaitForVite,
		TriggerRestart: refreshAction.TriggerRestart || other.TriggerRestart,
		RecompileGo:    refreshAction.RecompileGo || other.RecompileGo,
	}
}

// IsZero returns true if this RefreshAction specifies no action.
func (refreshAction RefreshAction) IsZero() bool {
	return !refreshAction.ReloadBrowser &&
		!refreshAction.WaitForApp &&
		!refreshAction.WaitForVite &&
		!refreshAction.TriggerRestart &&
		!refreshAction.RecompileGo
}

// Lookup resolves a source public asset path to its built public URL.
//
// It supports both raw lookup keys and keys that already include the configured
// public path prefix.
func (fileMap FileMap) Lookup(
	original string,
	prefix string,
) (url string, found bool) {
	normalizedOriginal := normalizePublicAssetPathForLookup(original)
	if entry, ok := fileMap[normalizedOriginal]; ok {
		return joinPublicURLPrefixAndPath(prefix, entry.DistName), true
	}

	deprefixedOriginal := trimConfiguredPublicPathPrefixFromLookupPath(
		normalizedOriginal,
		prefix,
	)
	if deprefixedOriginal != normalizedOriginal {
		if entry, ok := fileMap[deprefixedOriginal]; ok {
			return joinPublicURLPrefixAndPath(prefix, entry.DistName), true
		}
	}

	return "", false
}

func normalizePublicAssetPathForLookup(original string) string {
	return strings.TrimPrefix(path.Clean("/"+original), "/")
}

func trimConfiguredPublicPathPrefixFromLookupPath(
	normalizedLookupPath string,
	publicPathPrefix string,
) string {
	normalizedPublicPathPrefix := normalizeConfiguredPublicPathPrefixForLookup(
		publicPathPrefix,
	)
	if normalizedPublicPathPrefix == "" {
		return normalizedLookupPath
	}

	if normalizedLookupPath == normalizedPublicPathPrefix {
		return ""
	}

	normalizedPublicPathPrefixWithTrailingSlash := normalizedPublicPathPrefix + "/"
	if strings.HasPrefix(
		normalizedLookupPath,
		normalizedPublicPathPrefixWithTrailingSlash,
	) {
		return strings.TrimPrefix(
			normalizedLookupPath,
			normalizedPublicPathPrefixWithTrailingSlash,
		)
	}

	return normalizedLookupPath
}

func normalizeConfiguredPublicPathPrefixForLookup(
	publicPathPrefix string,
) string {
	return strings.Trim(path.Clean("/"+publicPathPrefix), "/")
}

func joinPublicURLPrefixAndPath(
	publicPathPrefix string,
	publicPath string,
) string {
	return matcher.EnsureLeadingSlash(path.Join(publicPathPrefix, publicPath))
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
		HashedOutputPrefixNoTrailing,
		"vorma_vite_manifest.json",
	)
}

// WatchRoot returns the normalized watch root path used by dev tooling.
func (parsedConfig *ParsedConfig) WatchRoot() string {
	if parsedConfig.Watch != nil && parsedConfig.Watch.WatchRoot != "" {
		return filepath.Clean(parsedConfig.Watch.WatchRoot)
	}
	return "."
}

// HealthcheckEndpoint returns the app healthcheck endpoint used by dev
// readiness probes.
func (parsedConfig *ParsedConfig) HealthcheckEndpoint() string {
	if parsedConfig.Watch != nil &&
		parsedConfig.Watch.HealthcheckEndpoint != "" {
		return parsedConfig.Watch.HealthcheckEndpoint
	}
	return "/"
}

// UsingBrowser reports whether browser runtime integration is enabled.
func (parsedConfig *ParsedConfig) UsingBrowser() bool {
	return !parsedConfig.Core.ServerOnlyMode
}

// UsingVite reports whether Vite integration is configured.
func (parsedConfig *ParsedConfig) UsingVite() bool {
	return parsedConfig.Vite != nil
}

// CriticalCSSEntry returns the normalized critical CSS entry path, if set.
func (parsedConfig *ParsedConfig) CriticalCSSEntry() string {
	if parsedConfig.Core.CSSEntryFiles.Critical == "" {
		return ""
	}
	return filepath.Clean(parsedConfig.Core.CSSEntryFiles.Critical)
}

// NonCriticalCSSEntry returns the normalized non-critical CSS entry path, if
// set.
func (parsedConfig *ParsedConfig) NonCriticalCSSEntry() string {
	if parsedConfig.Core.CSSEntryFiles.NonCritical == "" {
		return ""
	}
	return filepath.Clean(parsedConfig.Core.CSSEntryFiles.NonCritical)
}

// BrowserRuntimeNamespace returns the browser global namespace used by runtime
// integration scripts.
func (parsedConfig *ParsedConfig) BrowserRuntimeNamespace() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkBrowserRuntimeNamespace == "" {
		return DefaultBrowserRuntimeNamespace
	}
	return parsedConfig.FrameworkBrowserRuntimeNamespace
}

// BrowserPublicURLResolverFunctionName returns the browser helper function name
// used for resolving public asset URLs.
func (parsedConfig *ParsedConfig) BrowserPublicURLResolverFunctionName() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkBrowserPublicURLResolverFunctionName == "" {
		return DefaultBrowserPublicURLResolverFunctionName
	}
	return parsedConfig.FrameworkBrowserPublicURLResolverFunctionName
}

// BrowserRevalidateFunctionName returns the browser helper function name used
// for route revalidation.
func (parsedConfig *ParsedConfig) BrowserRevalidateFunctionName() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkBrowserRevalidateFunctionName == "" {
		return DefaultBrowserRevalidateFunctionName
	}
	return parsedConfig.FrameworkBrowserRevalidateFunctionName
}

// RefreshRebuildingOverlayElementID returns the DOM element id used for the
// rebuild overlay during dev refresh cycles.
func (parsedConfig *ParsedConfig) RefreshRebuildingOverlayElementID() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkRefreshRebuildingOverlayElementID == "" {
		return DefaultRefreshRebuildingOverlayElementID
	}
	return parsedConfig.FrameworkRefreshRebuildingOverlayElementID
}

// CriticalCSSStyleElementID returns the DOM element id used for injected
// critical CSS.
func (parsedConfig *ParsedConfig) CriticalCSSStyleElementID() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkCriticalCSSStyleElementID == "" {
		return DefaultCriticalCSSStyleElementID
	}
	return parsedConfig.FrameworkCriticalCSSStyleElementID
}

// NonCriticalCSSLinkElementID returns the DOM element id used for injected
// non-critical stylesheet links.
func (parsedConfig *ParsedConfig) NonCriticalCSSLinkElementID() string {
	if parsedConfig == nil ||
		parsedConfig.FrameworkNonCriticalCSSLinkElementID == "" {
		return DefaultNonCriticalCSSLinkElementID
	}
	return parsedConfig.FrameworkNonCriticalCSSLinkElementID
}
