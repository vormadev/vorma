package routeexecution

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime/rendering"
	"github.com/vormadev/vorma/internal/vormaruntime/routepipeline"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/nestedmux"
	"github.com/vormadev/vorma/kit/tasks"
)

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
				RuntimeSnapshot: runtimeSnapshotForRouteExecutionTests(),
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
				RuntimeSnapshot:          runtimeSnapshotForRouteExecutionTests(),
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

	request := createRequestWithTasksCtxForRouteExecutionTests(
		http.MethodGet,
		"/items/42",
	)
	executionInputs, found := PrepareExecutionInputs(
		PrepareExecutionInputsInput{
			Request:         request,
			NestedRouter:    router,
			RuntimeSnapshot: runtimeSnapshotForRouteExecutionTests(),
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

func runtimeSnapshotForRouteExecutionTests() routepipeline.RuntimeSnapshot {
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

func createRequestWithTasksCtxForRouteExecutionTests(
	method string,
	url string,
) *http.Request {
	request := httptest.NewRequest(method, url, nil)
	tasksCtx := tasks.NewCtx(request.Context())
	return mux.RequestWithTasksCtx(request, tasksCtx)
}
