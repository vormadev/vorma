package vormabuild

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

type reloadEndpointDependencies struct {
	reloadEndpointURLForApp  func(*vormaruntime.Vorma, string) string
	newReloadEndpointRequest func(context.Context, string) (*http.Request, error)
	doReloadEndpointRequest  func(*http.Request) (*http.Response, error)
}

type reloadActionDependencies struct {
	callReloadEndpoint  func(*vormaruntime.Vorma, callReloadEndpointOptions) error
	nextReloadAttemptID func() string
}

type reloadEndpointRequestExecutor struct {
	dependencies reloadEndpointDependencies
}

type reloadActionExecutor struct {
	dependencies reloadActionDependencies
}

type callReloadEndpointOptions struct {
	endpoint        string
	reloadAttemptID string
	expectedBuildID string
	reloadTrigger   string
}

var reloadAttemptSequence atomic.Uint64

const (
	reloadAttemptIDHeaderName       = "X-Vorma-Reload-Attempt-Id"
	reloadExpectedBuildIDHeaderName = "X-Vorma-Reload-Expected-Build-Id"
	reloadTriggerHeaderName         = "X-Vorma-Reload-Trigger"
)

var defaultReloadEndpointRequestExecutor = newReloadEndpointRequestExecutor(
	reloadEndpointDependencies{},
)

var defaultReloadActionExecutor = newReloadActionExecutor(
	reloadActionDependencies{
		callReloadEndpoint: func(
			v *vormaruntime.Vorma,
			options callReloadEndpointOptions,
		) error {
			return defaultReloadEndpointRequestExecutor.callReloadEndpoint(v, options)
		},
		nextReloadAttemptID: nextReloadAttemptID,
	},
)

func nextReloadAttemptID() string {
	attemptSequence := reloadAttemptSequence.Add(1)
	return fmt.Sprintf("reload-%d", attemptSequence)
}

func defaultReloadEndpointDependencies() reloadEndpointDependencies {
	return reloadEndpointDependencies{
		reloadEndpointURLForApp: func(v *vormaruntime.Vorma, endpoint string) string {
			return reloadEndpointURL(v.MustGetPort(), endpoint)
		},
		newReloadEndpointRequest: newReloadEndpointRequest,
		doReloadEndpointRequest: func(req *http.Request) (*http.Response, error) {
			client := &http.Client{Timeout: 10 * time.Second}
			return client.Do(req)
		},
	}
}

func normalizeReloadEndpointDependencies(
	dependencies reloadEndpointDependencies,
) reloadEndpointDependencies {
	defaultDependencies := defaultReloadEndpointDependencies()
	if dependencies.reloadEndpointURLForApp == nil {
		dependencies.reloadEndpointURLForApp = defaultDependencies.reloadEndpointURLForApp
	}
	if dependencies.newReloadEndpointRequest == nil {
		dependencies.newReloadEndpointRequest = defaultDependencies.newReloadEndpointRequest
	}
	if dependencies.doReloadEndpointRequest == nil {
		dependencies.doReloadEndpointRequest = defaultDependencies.doReloadEndpointRequest
	}
	return dependencies
}

func newReloadEndpointRequestExecutor(
	dependencies reloadEndpointDependencies,
) reloadEndpointRequestExecutor {
	return reloadEndpointRequestExecutor{
		dependencies: normalizeReloadEndpointDependencies(dependencies),
	}
}

func normalizeReloadActionDependencies(
	dependencies reloadActionDependencies,
) reloadActionDependencies {
	if dependencies.callReloadEndpoint == nil {
		dependencies.callReloadEndpoint = func(
			v *vormaruntime.Vorma,
			options callReloadEndpointOptions,
		) error {
			return defaultReloadEndpointRequestExecutor.callReloadEndpoint(v, options)
		}
	}
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

func getReloadActionForEndpointWithFallback(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
	reloadTrigger string,
) *wave.RefreshAction {
	return defaultReloadActionExecutor.getReloadActionForEndpointWithFallback(
		v,
		endpoint,
		warnMessage,
		reloadTrigger,
	)
}

func (executor reloadActionExecutor) getReloadActionForEndpointWithFallback(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
	reloadTrigger string,
) *wave.RefreshAction {
	reloadOptions := callReloadEndpointOptions{
		endpoint:        endpoint,
		reloadAttemptID: executor.dependencies.nextReloadAttemptID(),
		expectedBuildID: strings.TrimSpace(v.GetBuildID()),
		reloadTrigger:   reloadTrigger,
	}

	if err := executor.dependencies.callReloadEndpoint(v, reloadOptions); err != nil {
		v.Log.Warn(
			warnMessage,
			"error",
			err,
			"reload_endpoint",
			reloadOptions.endpoint,
			"reload_attempt_id",
			reloadOptions.reloadAttemptID,
			"expected_build_id",
			reloadOptions.expectedBuildID,
			"reload_trigger",
			reloadOptions.reloadTrigger,
			"fallback_action",
			"restart-without-recompile",
			"fallback_reason",
			"reload-endpoint-call-failed",
		)
		return newRestartWithoutRecompileAction()
	}

	v.Log.Debug(
		"reload endpoint succeeded",
		"reload_endpoint",
		reloadOptions.endpoint,
		"reload_attempt_id",
		reloadOptions.reloadAttemptID,
		"expected_build_id",
		reloadOptions.expectedBuildID,
		"reload_trigger",
		reloadOptions.reloadTrigger,
		"result_action",
		"reload-browser-and-wait",
		"decision_reason",
		"reload-endpoint-call-succeeded",
	)
	return newReloadBrowserAndWaitAction()
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

func (executor reloadEndpointRequestExecutor) callReloadEndpoint(
	v *vormaruntime.Vorma,
	options callReloadEndpointOptions,
) error {
	trimmedEndpoint := strings.TrimSpace(options.endpoint)
	if trimmedEndpoint == "" {
		return errors.New("reload endpoint path is required")
	}

	url := executor.dependencies.reloadEndpointURLForApp(v, trimmedEndpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := executor.dependencies.newReloadEndpointRequest(ctx, url)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	applyReloadEndpointRequestHeaders(req, options)

	resp, err := executor.dependencies.doReloadEndpointRequest(req)
	if err != nil {
		return fmt.Errorf(
			"request failed (endpoint=%q attempt=%q): %w",
			trimmedEndpoint,
			options.reloadAttemptID,
			err,
		)
	}
	defer resp.Body.Close()

	if err := validateReloadEndpointStatus(resp.StatusCode); err != nil {
		return fmt.Errorf(
			"validate response status (endpoint=%q attempt=%q): %w",
			trimmedEndpoint,
			options.reloadAttemptID,
			err,
		)
	}

	return nil
}

func applyReloadEndpointRequestHeaders(
	request *http.Request,
	options callReloadEndpointOptions,
) {
	if request == nil {
		return
	}

	trimmedReloadAttemptID := strings.TrimSpace(options.reloadAttemptID)
	if trimmedReloadAttemptID != "" {
		request.Header.Set(reloadAttemptIDHeaderName, trimmedReloadAttemptID)
	}

	trimmedExpectedBuildID := strings.TrimSpace(options.expectedBuildID)
	if trimmedExpectedBuildID != "" {
		request.Header.Set(reloadExpectedBuildIDHeaderName, trimmedExpectedBuildID)
	}

	trimmedReloadTrigger := strings.TrimSpace(options.reloadTrigger)
	if trimmedReloadTrigger != "" {
		request.Header.Set(reloadTriggerHeaderName, trimmedReloadTrigger)
	}
}

func reloadEndpointURL(port int, endpoint string) string {
	return fmt.Sprintf("http://localhost:%d%s", port, endpoint)
}

func newReloadEndpointRequest(ctx context.Context, url string) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
}

func validateReloadEndpointStatus(statusCode int) error {
	if statusCode != http.StatusOK {
		return fmt.Errorf("endpoint returned %d", statusCode)
	}
	return nil
}
