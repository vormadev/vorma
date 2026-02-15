package vormabuild

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
)

func TestReloadEndpointURL(t *testing.T) {
	got := reloadEndpointURL(8080, vormaruntime.DefaultDevReloadRoutesEndpointPath)
	want := "http://localhost:8080" + vormaruntime.DefaultDevReloadRoutesEndpointPath
	if got != want {
		t.Fatalf("reloadEndpointURL() = %q, want %q", got, want)
	}
}

func TestReloadEndpointDefaultDependencySteps(t *testing.T) {
	t.Run("builds app reload URL with endpoint suffix", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		reloadURL := reloadEndpointDeps.reloadEndpointURLForApp(
			app,
			vormaruntime.DefaultDevReloadRoutesEndpointPath,
		)
		if !strings.HasPrefix(reloadURL, "http://localhost:") {
			t.Fatalf("reload URL = %q, expected localhost URL", reloadURL)
		}
		if !strings.HasSuffix(reloadURL, vormaruntime.DefaultDevReloadRoutesEndpointPath) {
			t.Fatalf("reload URL = %q, expected endpoint suffix", reloadURL)
		}
	})

	t.Run("runs default request step", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "bad-scheme://localhost", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}

		resp, err := reloadEndpointDeps.doReloadEndpointRequest(req)
		if err == nil {
			if resp != nil && resp.Body != nil {
				t.Cleanup(func() {
					_ = resp.Body.Close()
				})
			}
			t.Fatal("expected request step to return error for unsupported protocol scheme")
		}
	})
}

func TestNewReloadEndpointRequest(t *testing.T) {
	request, err := newReloadEndpointRequest(context.Background(), "http://localhost:8080/path")
	if err != nil {
		t.Fatalf("newReloadEndpointRequest returned error: %v", err)
	}
	if request.Method != http.MethodGet {
		t.Fatalf("request method = %q, want %q", request.Method, http.MethodGet)
	}
	if request.URL.String() != "http://localhost:8080/path" {
		t.Fatalf("request URL = %q, want %q", request.URL.String(), "http://localhost:8080/path")
	}
}

func TestValidateReloadEndpointStatus(t *testing.T) {
	if err := validateReloadEndpointStatus(http.StatusOK); err != nil {
		t.Fatalf("validateReloadEndpointStatus(200) returned error: %v", err)
	}

	err := validateReloadEndpointStatus(http.StatusInternalServerError)
	if err == nil {
		t.Fatal("expected non-200 status to return error")
	}
	if !strings.Contains(err.Error(), "endpoint returned 500") {
		t.Fatalf("error = %q, expected status message", err)
	}
}

func TestCallReloadEndpoint(t *testing.T) {
	originalReloadEndpointURLForAppStep := reloadEndpointDeps.reloadEndpointURLForApp
	originalNewReloadEndpointRequestStep := reloadEndpointDeps.newReloadEndpointRequest
	originalDoReloadEndpointRequestStep := reloadEndpointDeps.doReloadEndpointRequest
	t.Cleanup(func() {
		reloadEndpointDeps.reloadEndpointURLForApp = originalReloadEndpointURLForAppStep
		reloadEndpointDeps.newReloadEndpointRequest = originalNewReloadEndpointRequestStep
		reloadEndpointDeps.doReloadEndpointRequest = originalDoReloadEndpointRequestStep
	})

	v := &vormaruntime.Vorma{
		Log: testLogger(),
	}

	t.Run("wraps request creation error", func(t *testing.T) {
		expectedErr := errors.New("request construction failed")
		reloadEndpointDeps.reloadEndpointURLForApp = func(_ *vormaruntime.Vorma, endpoint string) string {
			if endpoint != vormaruntime.DefaultDevReloadRoutesEndpointPath {
				t.Fatalf("endpoint = %q, want %q", endpoint, vormaruntime.DefaultDevReloadRoutesEndpointPath)
			}
			return "http://localhost:1234" + vormaruntime.DefaultDevReloadRoutesEndpointPath
		}
		reloadEndpointDeps.newReloadEndpointRequest = func(context.Context, string) (*http.Request, error) {
			return nil, expectedErr
		}
		reloadEndpointDeps.doReloadEndpointRequest = func(*http.Request) (*http.Response, error) {
			t.Fatal("did not expect request execution when request creation fails")
			return nil, nil
		}

		err := callReloadEndpoint(v, vormaruntime.DefaultDevReloadRoutesEndpointPath)
		if err == nil {
			t.Fatal("expected callReloadEndpoint to return request creation error")
		}
		if !strings.Contains(err.Error(), "create request") {
			t.Fatalf("error = %q, expected create-request context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped request creation error", err)
		}
	})

	t.Run("wraps request execution error", func(t *testing.T) {
		expectedErr := errors.New("request execution failed")
		reloadEndpointDeps.reloadEndpointURLForApp = func(*vormaruntime.Vorma, string) string {
			return "http://localhost:1234" + vormaruntime.DefaultDevReloadRoutesEndpointPath
		}
		reloadEndpointDeps.newReloadEndpointRequest = func(ctx context.Context, url string) (*http.Request, error) {
			return newReloadEndpointRequest(ctx, url)
		}
		reloadEndpointDeps.doReloadEndpointRequest = func(*http.Request) (*http.Response, error) {
			return nil, expectedErr
		}

		err := callReloadEndpoint(v, vormaruntime.DefaultDevReloadRoutesEndpointPath)
		if err == nil {
			t.Fatal("expected callReloadEndpoint to return request execution error")
		}
		if !strings.Contains(err.Error(), "request failed") {
			t.Fatalf("error = %q, expected request-failed context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped request execution error", err)
		}
	})

	t.Run("returns non-200 status validation error", func(t *testing.T) {
		reloadEndpointDeps.reloadEndpointURLForApp = func(*vormaruntime.Vorma, string) string {
			return "http://localhost:1234" + vormaruntime.DefaultDevReloadRoutesEndpointPath
		}
		reloadEndpointDeps.newReloadEndpointRequest = func(ctx context.Context, url string) (*http.Request, error) {
			return newReloadEndpointRequest(ctx, url)
		}
		reloadEndpointDeps.doReloadEndpointRequest = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}

		err := callReloadEndpoint(v, vormaruntime.DefaultDevReloadRoutesEndpointPath)
		if err == nil {
			t.Fatal("expected callReloadEndpoint to return status validation error")
		}
		if !strings.Contains(err.Error(), "endpoint returned 500") {
			t.Fatalf("error = %q, expected status validation message", err)
		}
	})

	t.Run("returns nil on successful status", func(t *testing.T) {
		reloadEndpointDeps.reloadEndpointURLForApp = func(*vormaruntime.Vorma, string) string {
			return "http://localhost:1234" + vormaruntime.DefaultDevReloadRoutesEndpointPath
		}
		reloadEndpointDeps.newReloadEndpointRequest = func(ctx context.Context, url string) (*http.Request, error) {
			return newReloadEndpointRequest(ctx, url)
		}
		reloadEndpointDeps.doReloadEndpointRequest = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		}

		err := callReloadEndpoint(v, vormaruntime.DefaultDevReloadRoutesEndpointPath)
		if err != nil {
			t.Fatalf("callReloadEndpoint returned error: %v", err)
		}
	})
}
