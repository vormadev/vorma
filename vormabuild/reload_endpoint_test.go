package vormabuild

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/wave"
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
		t.Setenv("__WAVE_PORT_HAS_BEEN_SET", "true")
		t.Setenv("PORT", "8081")

		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		defaultDependencies := defaultReloadEndpointDependencies()
		reloadURL := defaultDependencies.reloadEndpointURLForApp(
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

		defaultDependencies := defaultReloadEndpointDependencies()
		resp, err := defaultDependencies.doReloadEndpointRequest(req)
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
	v := &vormaruntime.Vorma{
		Log: testLogger(),
	}
	reloadOptions := callReloadEndpointOptions{
		endpoint:        vormaruntime.DefaultDevReloadRoutesEndpointPath,
		reloadAttemptID: "reload-123",
		expectedBuildID: "build-abc",
		reloadTrigger:   reloadTriggerRouteDefinitionsWatch,
	}

	t.Run("uses loaders reload endpoint request method", func(t *testing.T) {
		executor := newReloadEndpointRequestExecutor(reloadEndpointDependencies{
			reloadEndpointURLForApp: func(*vormaruntime.Vorma, string) string {
				return "http://localhost:1234" + vormaruntime.DefaultDevReloadRoutesEndpointPath
			},
			newReloadEndpointRequest: func(ctx context.Context, url string) (*http.Request, error) {
				return newReloadEndpointRequest(ctx, url)
			},
			doReloadEndpointRequest: func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodGet {
					return nil, errors.New("reload endpoint request method must be GET")
				}
				if got, want := request.Header.Get(reloadAttemptIDHeaderName), "reload-123"; got != want {
					return nil, errors.New("reload endpoint request missing reload attempt header")
				}
				if got, want := request.Header.Get(reloadExpectedBuildIDHeaderName), "build-abc"; got != want {
					return nil, errors.New("reload endpoint request missing expected build ID header")
				}
				if got, want := request.Header.Get(reloadTriggerHeaderName), reloadTriggerRouteDefinitionsWatch; got != want {
					return nil, errors.New("reload endpoint request missing reload trigger header")
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			},
		})

		err := executor.callReloadEndpoint(v, reloadOptions)
		if err != nil {
			t.Fatalf("callReloadEndpoint returned error: %v", err)
		}
	})

	t.Run("wraps request creation error", func(t *testing.T) {
		expectedErr := errors.New("request construction failed")
		executor := newReloadEndpointRequestExecutor(reloadEndpointDependencies{
			reloadEndpointURLForApp: func(_ *vormaruntime.Vorma, endpoint string) string {
				if endpoint != vormaruntime.DefaultDevReloadRoutesEndpointPath {
					t.Fatalf("endpoint = %q, want %q", endpoint, vormaruntime.DefaultDevReloadRoutesEndpointPath)
				}
				return "http://localhost:1234" + vormaruntime.DefaultDevReloadRoutesEndpointPath
			},
			newReloadEndpointRequest: func(context.Context, string) (*http.Request, error) {
				return nil, expectedErr
			},
			doReloadEndpointRequest: func(*http.Request) (*http.Response, error) {
				t.Fatal("did not expect request execution when request creation fails")
				return nil, nil
			},
		})

		err := executor.callReloadEndpoint(v, reloadOptions)
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
		executor := newReloadEndpointRequestExecutor(reloadEndpointDependencies{
			reloadEndpointURLForApp: func(*vormaruntime.Vorma, string) string {
				return "http://localhost:1234" + vormaruntime.DefaultDevReloadRoutesEndpointPath
			},
			newReloadEndpointRequest: func(ctx context.Context, url string) (*http.Request, error) {
				return newReloadEndpointRequest(ctx, url)
			},
			doReloadEndpointRequest: func(*http.Request) (*http.Response, error) {
				return nil, expectedErr
			},
		})

		err := executor.callReloadEndpoint(v, reloadOptions)
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
		executor := newReloadEndpointRequestExecutor(reloadEndpointDependencies{
			reloadEndpointURLForApp: func(*vormaruntime.Vorma, string) string {
				return "http://localhost:1234" + vormaruntime.DefaultDevReloadRoutesEndpointPath
			},
			newReloadEndpointRequest: func(ctx context.Context, url string) (*http.Request, error) {
				return newReloadEndpointRequest(ctx, url)
			},
			doReloadEndpointRequest: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
		})

		err := executor.callReloadEndpoint(v, reloadOptions)
		if err == nil {
			t.Fatal("expected callReloadEndpoint to return status validation error")
		}
		if !strings.Contains(err.Error(), "endpoint returned 500") {
			t.Fatalf("error = %q, expected status validation message", err)
		}
	})

	t.Run("returns nil on successful status", func(t *testing.T) {
		executor := newReloadEndpointRequestExecutor(reloadEndpointDependencies{
			reloadEndpointURLForApp: func(*vormaruntime.Vorma, string) string {
				return "http://localhost:1234" + vormaruntime.DefaultDevReloadRoutesEndpointPath
			},
			newReloadEndpointRequest: func(ctx context.Context, url string) (*http.Request, error) {
				return newReloadEndpointRequest(ctx, url)
			},
			doReloadEndpointRequest: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			},
		})

		err := executor.callReloadEndpoint(v, reloadOptions)
		if err != nil {
			t.Fatalf("callReloadEndpoint returned error: %v", err)
		}
	})

	t.Run("returns error when endpoint is empty", func(t *testing.T) {
		executor := newReloadEndpointRequestExecutor(reloadEndpointDependencies{})
		err := executor.callReloadEndpoint(v, callReloadEndpointOptions{})
		if err == nil {
			t.Fatal("expected callReloadEndpoint to return missing endpoint error")
		}
		if !strings.Contains(err.Error(), "reload endpoint path is required") {
			t.Fatalf("error = %q, expected missing-endpoint context", err)
		}
	})
}

func TestGetReloadActionForEndpointWithFallback(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	app.WithLock(func(l *vormaruntime.LockedVorma) {
		l.SetBuildID("build-for-reload-action")
	})

	t.Run("passes structured reload options and returns browser reload action on success", func(t *testing.T) {
		var capturedOptions callReloadEndpointOptions
		executor := newReloadActionExecutor(reloadActionDependencies{
			nextReloadAttemptID: func() string {
				return "reload-test-attempt"
			},
			callReloadEndpoint: func(_ *vormaruntime.Vorma, options callReloadEndpointOptions) error {
				capturedOptions = options
				return nil
			},
		})

		action := executor.getReloadActionForEndpointWithFallback(
			app,
			vormaruntime.DefaultDevReloadRoutesEndpointPath,
			"reload warning",
			reloadTriggerRouteDefinitionsWatch,
		)
		if action == nil {
			t.Fatal("expected non-nil reload action on success")
		}
		if !action.ReloadBrowser || !action.WaitForApp || !action.WaitForVite {
			t.Fatalf("action = %#v, expected browser reload + wait action", action)
		}
		if action.TriggerRestart || action.RecompileGo {
			t.Fatalf("action = %#v, expected no restart/recompile on success", action)
		}
		if capturedOptions.endpoint != vormaruntime.DefaultDevReloadRoutesEndpointPath {
			t.Fatalf("endpoint = %q, want %q", capturedOptions.endpoint, vormaruntime.DefaultDevReloadRoutesEndpointPath)
		}
		if capturedOptions.reloadAttemptID != "reload-test-attempt" {
			t.Fatalf("reloadAttemptID = %q, want %q", capturedOptions.reloadAttemptID, "reload-test-attempt")
		}
		if capturedOptions.expectedBuildID != "build-for-reload-action" {
			t.Fatalf("expectedBuildID = %q, want %q", capturedOptions.expectedBuildID, "build-for-reload-action")
		}
		if capturedOptions.reloadTrigger != reloadTriggerRouteDefinitionsWatch {
			t.Fatalf("reloadTrigger = %q, want %q", capturedOptions.reloadTrigger, reloadTriggerRouteDefinitionsWatch)
		}
	})

	t.Run("uses deterministic fallback action for distinct failure modes", func(t *testing.T) {
		failureModes := []error{
			errors.New("transport failure"),
			errors.New("status failure"),
		}

		var firstFallbackAction *wave.RefreshAction
		for _, failureMode := range failureModes {
			executor := newReloadActionExecutor(reloadActionDependencies{
				nextReloadAttemptID: func() string {
					return "reload-test-attempt"
				},
				callReloadEndpoint: func(*vormaruntime.Vorma, callReloadEndpointOptions) error {
					return failureMode
				},
			})

			action := executor.getReloadActionForEndpointWithFallback(
				app,
				vormaruntime.DefaultDevReloadRoutesEndpointPath,
				"reload warning",
				reloadTriggerRouteDefinitionsWatch,
			)
			if action == nil {
				t.Fatal("expected fallback action when reload endpoint call fails")
			}
			if !action.TriggerRestart || action.RecompileGo {
				t.Fatalf("action = %#v, expected restart without recompilation", action)
			}
			if action.ReloadBrowser || action.WaitForApp || action.WaitForVite {
				t.Fatalf("action = %#v, expected no browser reload/wait on fallback", action)
			}

			if firstFallbackAction == nil {
				clonedFirstAction := *action
				firstFallbackAction = &clonedFirstAction
				continue
			}
			if !reflect.DeepEqual(*action, *firstFallbackAction) {
				t.Fatalf("fallback action changed across failure modes: got %#v, want %#v", *action, *firstFallbackAction)
			}
		}
	})
}
