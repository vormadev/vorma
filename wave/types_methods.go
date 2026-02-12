package wave

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/kit/matcher"
)

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
			wf.SortedHooks.Concurrent = append(wf.SortedHooks.Concurrent, onChangeHook)
		case OnChangeStrategyConcurrentNoWait:
			wf.SortedHooks.ConcurrentNoWait = append(wf.SortedHooks.ConcurrentNoWait, onChangeHook)
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

func (fileMap FileMap) Lookup(original string, prefix string) (url string, found bool) {
	normalizedOriginal := strings.TrimPrefix(path.Clean("/"+original), "/")
	if entry, ok := fileMap[normalizedOriginal]; ok {
		return matcher.EnsureLeadingSlash(path.Join(prefix, entry.DistName)), true
	}
	return matcher.EnsureLeadingSlash(path.Join(prefix, normalizedOriginal)), false
}

func (parsedConfig *ParsedConfig) PublicPathPrefix() string {
	prefix := parsedConfig.Core.PublicPathPrefix
	if prefix == "" || prefix == "/" {
		return "/"
	}
	return matcher.EnsureLeadingAndTrailingSlash(prefix)
}

func (parsedConfig *ParsedConfig) ViteManifestPath() string {
	return filepath.Join(
		parsedConfig.Dist.StaticPrivate(),
		HashedOutputPrefixNoTrailing,
		"vorma_vite_manifest.json",
	)
}

func (parsedConfig *ParsedConfig) WatchRoot() string {
	if parsedConfig.Watch != nil && parsedConfig.Watch.WatchRoot != "" {
		return filepath.Clean(parsedConfig.Watch.WatchRoot)
	}
	return "."
}

func (parsedConfig *ParsedConfig) HealthcheckEndpoint() string {
	if parsedConfig.Watch != nil && parsedConfig.Watch.HealthcheckEndpoint != "" {
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
