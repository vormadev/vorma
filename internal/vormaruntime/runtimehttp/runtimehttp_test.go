package runtimehttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/rendering"
	"github.com/vormadev/vorma/internal/vormaruntime/routepipeline"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/kit/tasks"
	"github.com/vormadev/vorma/kit/validate"
)

func TestBuildLoadersNestedRouter_DefaultsAndOverrides(t *testing.T) {
	defaultRouter := BuildLoadersNestedRouter(LoadersRouterSpec{})
	if got, want := defaultRouter.ExplicitIndexSegmentIdentifier(), "_index"; got != want {
		t.Fatalf("default explicit index segment = %q, want %q", got, want)
	}

	customRouter := BuildLoadersNestedRouter(LoadersRouterSpec{
		DynamicParamPrefix:             '$',
		SplatSegmentIdentifier:         '~',
		ExplicitIndexSegmentIdentifier: "index",
	})
	if got, want := customRouter.DynamicParamPrefix(), '$'; got != want {
		t.Fatalf("dynamic param prefix = %q, want %q", got, want)
	}
	if got, want := customRouter.SplatSegmentIdentifier(), '~'; got != want {
		t.Fatalf("splat segment identifier = %q, want %q", got, want)
	}
	if got, want := customRouter.ExplicitIndexSegmentIdentifier(), "index"; got != want {
		t.Fatalf("explicit index segment = %q, want %q", got, want)
	}
}

func TestBuildSupportedMethodsMap(t *testing.T) {
	defaultMethods := BuildSupportedMethodsMap(nil)
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		if !defaultMethods[method] {
			t.Fatalf("default supported methods should include %q", method)
		}
	}

	customMethods := BuildSupportedMethodsMap([]string{
		"get",
		"  post ",
		"",
		"PATCH",
	})
	if len(customMethods) != 3 {
		t.Fatalf("len(custom methods) = %d, want 3", len(customMethods))
	}
	for _, method := range []string{"GET", "POST", "PATCH"} {
		if !customMethods[method] {
			t.Fatalf("custom supported methods should include %q", method)
		}
	}
}

func TestParseActionInput(t *testing.T) {
	type queryInput struct {
		Name string `json:"name"`
	}
	type jsonInput struct {
		Count int `json:"count"`
	}

	supportedMethods := BuildSupportedMethodsMap([]string{"POST"})

	t.Run("GET_uses_url_search_params_parser", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodGet,
			"/api/items?name=chris",
			nil,
		)
		var input queryInput
		err := ParseActionInput(request, &input, supportedMethods, nil)
		if err != nil {
			t.Fatalf("ParseActionInput(GET): %v", err)
		}
		if got, want := input.Name, "chris"; got != want {
			t.Fatalf("parsed query name = %q, want %q", got, want)
		}
	})

	t.Run("unsupported_method_returns_validation_error", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPut, "/api/items", nil)
		var input queryInput
		err := ParseActionInput(request, &input, supportedMethods, nil)
		if err == nil {
			t.Fatal("expected unsupported method error")
		}
		if !validate.IsValidationError(err) {
			t.Fatalf("expected validate.ValidationError, got %T", err)
		}
		if !strings.Contains(err.Error(), "unsupported method") {
			t.Fatalf("error = %q, want unsupported method", err)
		}
	})

	t.Run("form_content_type_requires_form_data_input", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/items",
			strings.NewReader("name=ok"),
		)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var input queryInput
		err := ParseActionInput(request, &input, supportedMethods, nil)
		if err == nil {
			t.Fatal("expected form-data input type validation error")
		}
		if !strings.Contains(err.Error(), "form content type requires") {
			t.Fatalf("error = %q, want form data input error", err)
		}
	})

	t.Run("form_content_type_allows_form_data_input", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/items",
			strings.NewReader("name=ok"),
		)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var input any
		err := ParseActionInput(
			request,
			&input,
			supportedMethods,
			func(any) bool { return true },
		)
		if err != nil {
			t.Fatalf("ParseActionInput(form data): %v", err)
		}
	})

	t.Run("json_body_is_parsed_for_supported_methods", func(t *testing.T) {
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/items",
			strings.NewReader(`{"count":42}`),
		)
		request.Header.Set("Content-Type", "application/json")
		var input jsonInput
		err := ParseActionInput(request, &input, supportedMethods, nil)
		if err != nil {
			t.Fatalf("ParseActionInput(JSON): %v", err)
		}
		if got, want := input.Count, 42; got != want {
			t.Fatalf("parsed count = %d, want %d", got, want)
		}
	})
}

func TestBuildActionsRouter(t *testing.T) {
	router, supportedMethods := BuildActionsRouter(ActionsRouterSpec{
		MountRoot:        "/actions/",
		SupportedMethods: []string{"post"},
		IsFormDataInput: func(input any) bool {
			_, ok := input.(*struct{})
			return ok
		},
	})
	if router == nil {
		t.Fatal("expected non-nil actions router")
	}
	if got, want := router.MountRoot(), "/actions/"; got != want {
		t.Fatalf("MountRoot = %q, want %q", got, want)
	}
	if !supportedMethods["POST"] || len(supportedMethods) != 1 {
		t.Fatalf("supported methods = %#v, want POST only", supportedMethods)
	}
}

func TestPrepareExecutionInputs(t *testing.T) {
	router := nestedmux.NewRouter(nil)
	nestedmux.AddPatternWithoutHandler(router, "")
	nestedmux.AddPatternWithoutHandler(router, "/items/:id")

	t.Run("matched_path_builds_execution_inputs", func(t *testing.T) {
		inputs, found := PrepareExecutionInputs(
			PrepareExecutionInputsInput{
				Request: httptest.NewRequest(
					http.MethodGet,
					"/items/42",
					nil,
				),
				NestedRouter:    router,
				RuntimeSnapshot: runtimeSnapshotForRuntimeHTTPTests(),
				IsSnapshotVersionCurrent: func(
					expectedSnapshotVersion uint64,
				) bool {
					return expectedSnapshotVersion == 5
				},
			},
		)
		if !found {
			t.Fatal("expected nested match for /items/42")
		}
		if got, want := inputs.RuntimeSnapshot.BuildID, "build-route-execution"; got != want {
			t.Fatalf("RuntimeSnapshot.BuildID = %q, want %q", got, want)
		}
		if got, want := inputs.MatchedPatterns, []string{"", "/items/:id"}; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("MatchedPatterns = %#v, want %#v", got, want)
		}
		if inputs.Cached == nil {
			t.Fatal("expected cached subset to be populated")
		}
		if got, want := inputs.Cached.ImportURLs, []string{
			"/frontend/src/routes/root.tsx",
			"/frontend/src/routes/items.$id.tsx",
		}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Cached.ImportURLs = %#v, want %#v", got, want)
		}
		if got, want := inputs.Cached.Deps, []string{
			"vorma_out/chunk-client.js",
			"vorma_out/chunk-root.js",
			"vorma_out/chunk-items.js",
		}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Cached.Deps = %#v, want %#v", got, want)
		}
	})

	t.Run("non_matching_path_returns_false_with_snapshot", func(t *testing.T) {
		inputs, found := PrepareExecutionInputs(
			PrepareExecutionInputsInput{
				Request: httptest.NewRequest(
					http.MethodGet,
					"/missing",
					nil,
				),
				NestedRouter:             router,
				RuntimeSnapshot:          runtimeSnapshotForRuntimeHTTPTests(),
				IsSnapshotVersionCurrent: nil,
			},
		)
		if found {
			t.Fatal("expected no nested match for /missing")
		}
		if got, want := inputs.RuntimeSnapshot.BuildID, "build-route-execution"; got != want {
			t.Fatalf("RuntimeSnapshot.BuildID = %q, want %q", got, want)
		}
		if inputs.MatchResults != nil {
			t.Fatalf("MatchResults = %#v, want nil", inputs.MatchResults)
		}
	})
}

func TestPlanRouteResultFromTaskResults(t *testing.T) {
	router := nestedmux.NewRouter(nil)
	nestedmux.AddTaskHandler(
		router,
		"",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (*int, error) {
				return nil, nil
			},
		),
	)
	nestedmux.AddTaskHandler(
		router,
		"/items/:id",
		mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "", errors.New("loader failed")
			},
		),
	)

	request := createRequestWithTasksCtxForRuntimeHTTPTests(
		http.MethodGet,
		"/items/42",
	)
	executionInputs, found := PrepareExecutionInputs(
		PrepareExecutionInputsInput{
			Request:         request,
			NestedRouter:    router,
			RuntimeSnapshot: runtimeSnapshotForRuntimeHTTPTests(),
			IsSnapshotVersionCurrent: func(
				expectedSnapshotVersion uint64,
			) bool {
				return expectedSnapshotVersion == 5
			},
		},
	)
	if !found {
		t.Fatal("expected nested match for /items/42")
	}
	tasksResults, found := nestedmux.FindMatchesAndRunTasks(router, request)
	if !found {
		t.Fatal("expected task results for /items/42")
	}

	warnedPatterns := make([]string, 0, 1)
	resolvedPattern := ""
	resolvedErrText := ""
	result := PlanRouteResultFromTaskResults(
		PlanRouteResultFromTaskResultsInput{
			ExecutionInputs: executionInputs,
			TasksResults:    tasksResults,
			WarnNilLoaderData: func(pattern string) {
				warnedPatterns = append(warnedPatterns, pattern)
			},
			ResolveClientLoaderErrorMessage: func(err error, pattern string) string {
				resolvedPattern = pattern
				if err != nil {
					resolvedErrText = err.Error()
				}
				return "safe-client-error"
			},
		},
	)

	if got, want := result.TerminalState, routepipeline.RouteTerminalStateNone; got != want {
		t.Fatalf("TerminalState = %v, want %v", got, want)
	}
	if result.Core == nil {
		t.Fatal("expected route core for non-terminal stage-one result")
	}
	if got, want := warnedPatterns, []string{""}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("warned patterns = %#v, want %#v", got, want)
	}
	if got, want := resolvedPattern, "/items/:id"; got != want {
		t.Fatalf("resolved pattern = %q, want %q", got, want)
	}
	if got, want := resolvedErrText, "loader failed"; got != want {
		t.Fatalf("resolved error text = %q, want %q", got, want)
	}
	if got, want := result.Core.OutermostServerError, "safe-client-error"; got != want {
		t.Fatalf("OutermostServerError = %q, want %q", got, want)
	}
	if result.Core.OutermostServerErrorIdx == nil ||
		*result.Core.OutermostServerErrorIdx != 1 {
		t.Fatalf(
			"OutermostServerErrorIdx = %#v, want 1",
			result.Core.OutermostServerErrorIdx,
		)
	}
	if !result.Core.HasRootData {
		t.Fatal("expected root data when root matched route ran a task")
	}
}

func runtimeSnapshotForRuntimeHTTPTests() routepipeline.RuntimeSnapshot {
	return routepipeline.RuntimeSnapshot{
		BuildID: "build-route-execution",
		IsDev:   true,
		Paths: map[string]*routepipeline.PathData{
			"": {
				OriginalPattern: "",
				SrcPath:         "frontend/src/routes/root.tsx",
				OutPath:         "vorma_out/routes/root.js",
				ExportKey:       "Root",
				ErrorExportKey:  "RootErrorBoundary",
				Deps:            []string{"vorma_out/chunk-root.js"},
			},
			"/items/:id": {
				OriginalPattern: "/items/:id",
				SrcPath:         "frontend/src/routes/items.$id.tsx",
				OutPath:         "vorma_out/routes/items.$id.js",
				ExportKey:       "ItemRoute",
				ErrorExportKey:  "ItemErrorBoundary",
				Deps:            []string{"vorma_out/chunk-items.js"},
			},
		},
		ClientEntryDeps: []string{"vorma_out/chunk-client.js"},
		ClientEntryOut:  "vorma_out/client-entry.js",
		DepToCSSBundleMap: map[string][]string{
			"vorma_out/client-entry.js": {"vorma_out/client.css"},
			"vorma_out/chunk-client.js": {"vorma_out/chunk-client.css"},
			"vorma_out/chunk-root.js":   {"vorma_out/chunk-root.css"},
			"vorma_out/chunk-items.js":  {"vorma_out/chunk-items.css"},
		},
		HTMLRenderSnapshot: rendering.LoadersHTMLRenderSnapshot{
			IsDevMode:      true,
			ClientEntryOut: "vorma_out/client-entry.js",
		},
		RouteManifestFile:        "vorma_out/route-manifest.js",
		RouteDataSnapshotVersion: 5,
	}
}

func createRequestWithTasksCtxForRuntimeHTTPTests(
	method string,
	url string,
) *http.Request {
	request := httptest.NewRequest(method, url, nil)
	tasksCtx := tasks.NewCtx(request.Context())
	return mux.RequestWithTasksCtx(request, tasksCtx)
}
