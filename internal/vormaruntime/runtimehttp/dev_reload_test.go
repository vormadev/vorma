package runtimehttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateExpectedBuildIDOrWriteConflict(t *testing.T) {
	t.Run("nil_request_is_allowed", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ok := ValidateExpectedBuildIDOrWriteConflict(
			ValidateExpectedBuildIDOrWriteConflictInput{
				ResponseWriter:            recorder,
				Request:                   nil,
				CurrentBuildID:            "build-a",
				ExpectedBuildIDHeaderName: "X-Expected-Build-Id",
			},
		)
		if !ok {
			t.Fatal("expected nil request to be allowed")
		}
	})

	t.Run("missing_expected_header_is_allowed", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/reload", nil)
		ok := ValidateExpectedBuildIDOrWriteConflict(
			ValidateExpectedBuildIDOrWriteConflictInput{
				ResponseWriter:            recorder,
				Request:                   request,
				CurrentBuildID:            "build-a",
				ExpectedBuildIDHeaderName: "X-Expected-Build-Id",
			},
		)
		if !ok {
			t.Fatal("expected missing expected-build-id header to be allowed")
		}
		if got, want := recorder.Code, http.StatusOK; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
	})

	t.Run("matching_expected_header_is_allowed", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/reload", nil)
		request.Header.Set("X-Expected-Build-Id", "  build-a  ")
		ok := ValidateExpectedBuildIDOrWriteConflict(
			ValidateExpectedBuildIDOrWriteConflictInput{
				ResponseWriter:            recorder,
				Request:                   request,
				CurrentBuildID:            "build-a",
				ExpectedBuildIDHeaderName: "X-Expected-Build-Id",
			},
		)
		if !ok {
			t.Fatal("expected matching expected-build-id header to be allowed")
		}
		if got, want := recorder.Code, http.StatusOK; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
	})

	t.Run("mismatch_writes_conflict", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/reload", nil)
		request.Header.Set("X-Expected-Build-Id", "build-a")
		ok := ValidateExpectedBuildIDOrWriteConflict(
			ValidateExpectedBuildIDOrWriteConflictInput{
				ResponseWriter:            recorder,
				Request:                   request,
				CurrentBuildID:            "build-b",
				ExpectedBuildIDHeaderName: "X-Expected-Build-Id",
			},
		)
		if ok {
			t.Fatal(
				"expected mismatched expected-build-id header to be rejected",
			)
		}
		if got, want := recorder.Code, http.StatusConflict; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
		if got := recorder.Body.String(); !strings.Contains(
			got,
			"expected build id does not match current build id",
		) {
			t.Fatalf("body = %q, want conflict message", got)
		}
	})
}

func TestHandleDevReloadActionEndpoints(t *testing.T) {
	baseInput := DevReloadActionEndpointsInput{
		IsDevMode:            true,
		RoutesEndpointPath:   "/__reload-routes",
		TemplateEndpointPath: "/__reload-template",
	}

	t.Run("returns_false_when_not_dev_mode", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"http://example.com/__reload-routes",
			nil,
		)
		recorder := httptest.NewRecorder()
		handled := HandleDevReloadActionEndpoints(
			DevReloadActionEndpointsInput{
				ResponseWriter:       recorder,
				Request:              request,
				IsDevMode:            false,
				RoutesEndpointPath:   baseInput.RoutesEndpointPath,
				TemplateEndpointPath: baseInput.TemplateEndpointPath,
			},
		)
		if handled {
			t.Fatal("expected non-dev requests to be ignored")
		}
	})

	t.Run("returns_false_for_unknown_path", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"http://example.com/unknown",
			nil,
		)
		recorder := httptest.NewRecorder()
		handled := HandleDevReloadActionEndpoints(
			DevReloadActionEndpointsInput{
				ResponseWriter:       recorder,
				Request:              request,
				IsDevMode:            baseInput.IsDevMode,
				RoutesEndpointPath:   baseInput.RoutesEndpointPath,
				TemplateEndpointPath: baseInput.TemplateEndpointPath,
			},
		)
		if handled {
			t.Fatal("expected unknown path to be ignored")
		}
	})

	t.Run("rejects_non_post_methods", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodGet,
			"http://example.com/__reload-routes",
			nil,
		)
		recorder := httptest.NewRecorder()
		handled := HandleDevReloadActionEndpoints(
			DevReloadActionEndpointsInput{
				ResponseWriter:       recorder,
				Request:              request,
				IsDevMode:            baseInput.IsDevMode,
				RoutesEndpointPath:   baseInput.RoutesEndpointPath,
				TemplateEndpointPath: baseInput.TemplateEndpointPath,
			},
		)
		if !handled {
			t.Fatal("expected non-post route-reload request to be handled")
		}
		if got, want := recorder.Code, http.StatusMethodNotAllowed; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
		if got, want := recorder.Header().Get("Allow"), http.MethodPost; got != want {
			t.Fatalf("Allow header = %q, want %q", got, want)
		}
	})

	t.Run("validation_rejection_short_circuits_reload", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"http://example.com/__reload-routes",
			nil,
		)
		recorder := httptest.NewRecorder()
		reloadCalled := false
		handled := HandleDevReloadActionEndpoints(
			DevReloadActionEndpointsInput{
				ResponseWriter:       recorder,
				Request:              request,
				IsDevMode:            baseInput.IsDevMode,
				RoutesEndpointPath:   baseInput.RoutesEndpointPath,
				TemplateEndpointPath: baseInput.TemplateEndpointPath,
				ValidateExpectedBuildIDOrWriteConflict: func(
					responseWriter http.ResponseWriter,
					_ *http.Request,
				) bool {
					http.Error(
						responseWriter,
						"build id mismatch",
						http.StatusConflict,
					)
					return false
				},
				ReloadRoutesFromDisk: func() error {
					reloadCalled = true
					return nil
				},
			},
		)
		if !handled {
			t.Fatal("expected validation-rejected request to be handled")
		}
		if reloadCalled {
			t.Fatal(
				"expected reload not to run when validation rejects request",
			)
		}
		if got, want := recorder.Code, http.StatusConflict; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
	})

	t.Run("reload_failure_returns_500", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"http://example.com/__reload-routes",
			nil,
		)
		recorder := httptest.NewRecorder()
		reloadErr := errors.New("reload failed")
		handled := HandleDevReloadActionEndpoints(
			DevReloadActionEndpointsInput{
				ResponseWriter:       recorder,
				Request:              request,
				IsDevMode:            baseInput.IsDevMode,
				RoutesEndpointPath:   baseInput.RoutesEndpointPath,
				TemplateEndpointPath: baseInput.TemplateEndpointPath,
				ReloadRoutesFromDisk: func() error {
					return reloadErr
				},
			},
		)
		if !handled {
			t.Fatal("expected reload failure request to be handled")
		}
		if got, want := recorder.Code, http.StatusInternalServerError; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
		if got := recorder.Body.String(); !strings.Contains(
			got,
			"reload failed",
		) {
			t.Fatalf("body = %q, want reload error", got)
		}
	})

	t.Run("route_reload_success_writes_ok", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"http://example.com/__reload-routes",
			nil,
		)
		recorder := httptest.NewRecorder()
		reloadCalled := false
		handled := HandleDevReloadActionEndpoints(
			DevReloadActionEndpointsInput{
				ResponseWriter:       recorder,
				Request:              request,
				IsDevMode:            baseInput.IsDevMode,
				RoutesEndpointPath:   baseInput.RoutesEndpointPath,
				TemplateEndpointPath: baseInput.TemplateEndpointPath,
				ReloadRoutesFromDisk: func() error {
					reloadCalled = true
					return nil
				},
			},
		)
		if !handled {
			t.Fatal("expected route-reload request to be handled")
		}
		if !reloadCalled {
			t.Fatal("expected route-reload operation to run")
		}
		if got, want := recorder.Code, http.StatusOK; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
		if got, want := recorder.Body.String(), "ok"; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})

	t.Run("template_reload_success_writes_ok", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"http://example.com/__reload-template",
			nil,
		)
		recorder := httptest.NewRecorder()
		reloadCalled := false
		handled := HandleDevReloadActionEndpoints(
			DevReloadActionEndpointsInput{
				ResponseWriter:       recorder,
				Request:              request,
				IsDevMode:            baseInput.IsDevMode,
				RoutesEndpointPath:   baseInput.RoutesEndpointPath,
				TemplateEndpointPath: baseInput.TemplateEndpointPath,
				ReloadTemplateFromDisk: func() error {
					reloadCalled = true
					return nil
				},
			},
		)
		if !handled {
			t.Fatal("expected template-reload request to be handled")
		}
		if !reloadCalled {
			t.Fatal("expected template-reload operation to run")
		}
		if got, want := recorder.Code, http.StatusOK; got != want {
			t.Fatalf("status = %d, want %d", got, want)
		}
		if got, want := recorder.Body.String(), "ok"; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})
}
