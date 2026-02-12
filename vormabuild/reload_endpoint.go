package vormabuild

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

type reloadEndpointDependencies struct {
	reloadEndpointURLForApp  func(*vormaruntime.Vorma, string) string
	newReloadEndpointRequest func(context.Context, string) (*http.Request, error)
	doReloadEndpointRequest  func(*http.Request) (*http.Response, error)
}

type reloadActionDependencies struct {
	callReloadEndpoint func(*vormaruntime.Vorma, string) error
}

var reloadActionDeps = reloadActionDependencies{
	callReloadEndpoint: callReloadEndpoint,
}

var reloadEndpointDeps = reloadEndpointDependencies{
	reloadEndpointURLForApp: func(v *vormaruntime.Vorma, endpoint string) string {
		return reloadEndpointURL(v.MustGetPort(), endpoint)
	},
	newReloadEndpointRequest: newReloadEndpointRequest,
	doReloadEndpointRequest: func(req *http.Request) (*http.Response, error) {
		client := &http.Client{Timeout: 10 * time.Second}
		return client.Do(req)
	},
}

func getReloadActionForEndpointWithFallback(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
) *wave.RefreshAction {
	if err := reloadActionDeps.callReloadEndpoint(v, endpoint); err != nil {
		v.Log.Warn(warnMessage, "error", err)
		return newRestartWithoutRecompileAction()
	}
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

// callReloadEndpoint makes an HTTP GET request to the running app's reload endpoint.
func callReloadEndpoint(v *vormaruntime.Vorma, endpoint string) error {
	url := reloadEndpointDeps.reloadEndpointURLForApp(v, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := reloadEndpointDeps.newReloadEndpointRequest(ctx, url)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := reloadEndpointDeps.doReloadEndpointRequest(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := validateReloadEndpointStatus(resp.StatusCode); err != nil {
		return err
	}

	return nil
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
