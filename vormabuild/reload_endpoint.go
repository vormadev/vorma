package vormabuild

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

type reloadActionDependencies struct {
	nextReloadAttemptID func() string
}

type reloadActionExecutor struct {
	dependencies reloadActionDependencies
}

var reloadAttemptSequence atomic.Uint64

var defaultReloadActionExecutor = newReloadActionExecutor(
	reloadActionDependencies{
		nextReloadAttemptID: nextReloadAttemptID,
	},
)

func nextReloadAttemptID() string {
	attemptSequence := reloadAttemptSequence.Add(1)
	return fmt.Sprintf("reload-%d", attemptSequence)
}

func normalizeReloadActionDependencies(
	dependencies reloadActionDependencies,
) reloadActionDependencies {
	if dependencies.nextReloadAttemptID == nil {
		dependencies.nextReloadAttemptID = nextReloadAttemptID
	}
	return dependencies
}

func newReloadActionExecutor(
	dependencies reloadActionDependencies,
) reloadActionExecutor {
	return reloadActionExecutor{
		dependencies: normalizeReloadActionDependencies(dependencies),
	}
}

func getDeferredFrameworkRuntimeReloadAction(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
	reloadTrigger string,
	hookContext *wave.HookContext,
) *wave.RefreshAction {
	return defaultReloadActionExecutor.getDeferredFrameworkRuntimeReloadAction(
		v,
		endpoint,
		warnMessage,
		reloadTrigger,
		hookContext,
	)
}

func (executor reloadActionExecutor) getDeferredFrameworkRuntimeReloadAction(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
	reloadTrigger string,
	_ *wave.HookContext,
) *wave.RefreshAction {
	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		if v != nil && v.Log != nil {
			v.Log.Warn(
				warnMessage,
				"error",
				errors.New("reload endpoint path is required"),
				"fallback_action",
				"restart-without-recompile",
				"fallback_reason",
				"reload-endpoint-path-missing",
			)
		}
		return newRestartWithoutRecompileAction()
	}
	if !strings.HasPrefix(trimmedEndpoint, "/") {
		trimmedEndpoint = "/" + trimmedEndpoint
	}
	expectedBuildID := ""
	if v != nil {
		expectedBuildID = strings.TrimSpace(v.BuildID())
	}

	reloadAction := newReloadBrowserAndWaitAction()
	reloadAction.FrameworkRuntimeReloadRequest = &wave.FrameworkRuntimeReloadRequest{
		EndpointPath:    trimmedEndpoint,
		ReloadAttemptID: executor.dependencies.nextReloadAttemptID(),
		ExpectedBuildID: expectedBuildID,
		ReloadTrigger:   strings.TrimSpace(reloadTrigger),
	}
	return reloadAction
}

func newReloadBrowserAndWaitAction() *wave.RefreshAction {
	return &wave.RefreshAction{
		ReloadBrowser: true,
		WaitForApp:    true,
		WaitForVite:   true,
	}
}

func newRestartWithoutRecompileAction() *wave.RefreshAction {
	return &wave.RefreshAction{
		TriggerRestart: true,
		RecompileGo:    false,
	}
}
