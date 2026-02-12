package vormabuild

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

func getReloadActionForEndpointWithFallback(
	v *vormaruntime.Vorma,
	endpoint string,
	warnMessage string,
) *wave.RefreshAction {
	if err := callReloadEndpoint(v, endpoint); err != nil {
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
	port := v.MustGetPort()
	url := reloadEndpointURL(port, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := newReloadEndpointRequest(ctx, url)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
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
