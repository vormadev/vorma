// Package wavewatch defines Wave's file-watch and hook execution contract used
// by framework tooling, devservers, and build pipelines.
//
// This package exists so watch-hook runtime/buildtime behavior can evolve
// independently from end-user JSON parsing concerns in waveconfig.
package wavewatch

import "context"

// OnChangeTiming controls when one hook runs relative to rebuild work.
type OnChangeTiming string

const (
	// OnChangeStrategyPre runs before rebuild.
	OnChangeStrategyPre OnChangeTiming = "pre"
	// OnChangeStrategyPost runs after rebuild.
	OnChangeStrategyPost OnChangeTiming = "post"
	// OnChangeStrategyConcurrent runs concurrently with rebuild.
	OnChangeStrategyConcurrent OnChangeTiming = "concurrent"
	// OnChangeStrategyConcurrentNoWait runs fire-and-forget during rebuild.
	OnChangeStrategyConcurrentNoWait OnChangeTiming = "concurrent-no-wait"
)

// HookContext provides callback context during file-change handling.
type HookContext struct {
	// ExecutionContext is canceled when the surrounding execution pipeline is
	// canceled (for example, concurrent stage cancellation after build failure).
	ExecutionContext context.Context
	// FilePath is the absolute path of the changed file.
	FilePath string
	// ChangedFilePaths contains all changed file paths associated with the hook
	// execution context. For deduped pattern hook execution, this includes all
	// files in the matched batch.
	ChangedFilePaths []string
	// AppStoppedForBatch is true when app process shutdown already happened for
	// this batch, so callback logic must not depend on live app endpoints.
	AppStoppedForBatch bool
}

// FrameworkRuntimeReloadRequest describes one framework runtime reload request
// that must run before browser reload payload broadcast.
type FrameworkRuntimeReloadRequest struct {
	// EndpointPath is the framework runtime endpoint path to call.
	EndpointPath string
	// ReloadAttemptID is the attempt identifier propagated as request header.
	ReloadAttemptID string
	// ExpectedBuildID is the expected framework build identifier propagated as
	// request header.
	ExpectedBuildID string
	// ReloadTrigger is the reload trigger identifier propagated as request
	// header.
	ReloadTrigger string
}

// RefreshAction describes follow-up actions after one callback/hook completes.
type RefreshAction struct {
	ReloadBrowser                 bool
	WaitForApp                    bool
	WaitForVite                   bool
	TriggerRestart                bool
	RecompileGo                   bool
	FrameworkRuntimeReloadRequest *FrameworkRuntimeReloadRequest
}

// Merge combines two RefreshActions with OR semantics.
//
// TriggerRestart is evaluated by callers, and framework runtime reload request
// uses first-non-nil precedence.
func (refreshAction RefreshAction) Merge(other RefreshAction) RefreshAction {
	mergedFrameworkRuntimeReloadRequest := refreshAction.FrameworkRuntimeReloadRequest
	if mergedFrameworkRuntimeReloadRequest == nil {
		mergedFrameworkRuntimeReloadRequest = other.FrameworkRuntimeReloadRequest
	}
	return RefreshAction{
		ReloadBrowser: refreshAction.ReloadBrowser ||
			other.ReloadBrowser,
		WaitForApp: refreshAction.WaitForApp ||
			other.WaitForApp,
		WaitForVite: refreshAction.WaitForVite ||
			other.WaitForVite,
		TriggerRestart: refreshAction.TriggerRestart ||
			other.TriggerRestart,
		RecompileGo: refreshAction.RecompileGo ||
			other.RecompileGo,
		FrameworkRuntimeReloadRequest: mergedFrameworkRuntimeReloadRequest,
	}
}

// IsZero reports whether this RefreshAction requests no follow-up work.
func (refreshAction RefreshAction) IsZero() bool {
	return !refreshAction.ReloadBrowser &&
		!refreshAction.WaitForApp &&
		!refreshAction.WaitForVite &&
		!refreshAction.TriggerRestart &&
		!refreshAction.RecompileGo &&
		refreshAction.FrameworkRuntimeReloadRequest == nil
}

// OnChangeHook defines one command/callback action triggered by a watched-file
// match.
type OnChangeHook struct {
	// Cmd is a shell command to run.
	Cmd string `json:"Cmd,omitempty"`
	// CommandTimeoutMilliseconds overrides stage-level command timeout for this
	// hook when > 0.
	CommandTimeoutMilliseconds int `json:"CommandTimeoutMilliseconds,omitempty"`
	// DisableStageCommandTimeout disables stage-level command timeout for this
	// hook.
	DisableStageCommandTimeout bool `json:"DisableStageCommandTimeout,omitempty"`
	// CallbackTimeoutMilliseconds overrides stage-level callback timeout for this
	// hook when > 0.
	CallbackTimeoutMilliseconds int `json:"CallbackTimeoutMilliseconds,omitempty"`
	// DisableStageCallbackTimeout disables stage-level callback timeout for this
	// hook.
	DisableStageCallbackTimeout bool `json:"DisableStageCallbackTimeout,omitempty"`
	// RunCombinedDevBuildHookCommands executes configured development build hooks
	// in order (Core.DevBuildHook then framework dev build hook).
	RunCombinedDevBuildHookCommands bool `json:"RunCombinedDevBuildHookCommands,omitempty"`
	// Timing controls when this hook runs relative to Wave's rebuild process.
	Timing OnChangeTiming `json:"Timing,omitempty"`
	// Exclude contains glob patterns for files to exclude from triggering this
	// hook.
	Exclude []string `json:"Exclude,omitempty"`
	// Callback is a Go function to run. It is framework-facing and not
	// JSON-configurable.
	Callback func(*HookContext) (*RefreshAction, error) `json:"-"`
}

// SortedHooks stores hooks partitioned by execution stage.
type SortedHooks struct {
	Pre              []OnChangeHook
	Concurrent       []OnChangeHook
	ConcurrentNoWait []OnChangeHook
	Post             []OnChangeHook
}

// WatchedFile defines one watched glob pattern and its hook behavior.
type WatchedFile struct {
	Pattern                            string         `json:"Pattern"`
	OnChangeHooks                      []OnChangeHook `json:"OnChangeHooks,omitempty"`
	RecompileGoBinary                  bool           `json:"RecompileGoBinary,omitempty"`
	RestartApp                         bool           `json:"RestartApp,omitempty"`
	OnlyRunClientDefinedRevalidateFunc bool           `json:"OnlyRunClientDefinedRevalidateFunc,omitempty"`
	RunOnChangeOnly                    bool           `json:"RunOnChangeOnly,omitempty"`
	SkipRebuildingNotification         bool           `json:"SkipRebuildingNotification,omitempty"`
	TreatAsNonGo                       bool           `json:"TreatAsNonGo,omitempty"`
	SortedHooks                        *SortedHooks   `json:"-"`
}

// Sort populates SortedHooks by grouping OnChangeHooks according to Timing.
func (watchedFile *WatchedFile) Sort() {
	if watchedFile.SortedHooks != nil {
		return
	}

	watchedFile.SortedHooks = &SortedHooks{}
	for _, onChangeHook := range watchedFile.OnChangeHooks {
		switch onChangeHook.Timing {
		case OnChangeStrategyPost:
			watchedFile.SortedHooks.Post = append(
				watchedFile.SortedHooks.Post,
				onChangeHook,
			)
		case OnChangeStrategyConcurrent:
			watchedFile.SortedHooks.Concurrent = append(
				watchedFile.SortedHooks.Concurrent,
				onChangeHook,
			)
		case OnChangeStrategyConcurrentNoWait:
			watchedFile.SortedHooks.ConcurrentNoWait = append(
				watchedFile.SortedHooks.ConcurrentNoWait,
				onChangeHook,
			)
		default:
			watchedFile.SortedHooks.Pre = append(
				watchedFile.SortedHooks.Pre,
				onChangeHook,
			)
		}
	}
}
