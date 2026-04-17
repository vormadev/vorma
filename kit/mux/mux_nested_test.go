package mux

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/tasks"
	"github.com/vormadev/vorma/kit/validate"
)

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func create_request_with_tasks_ctx(method, url string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	return RequestWithTasksCtx(req, tasks.NewCtx(req.Context()))
}

func find_nested_result(
	results *NestedTasksResults,
	pattern string,
) *NestedTasksResult {
	for _, r := range results.Results {
		if r.Pattern() == pattern {
			return r
		}
	}
	return nil
}

type test_error struct{ msg string }

func (e *test_error) Error() string { return e.msg }

/////////////////////////////////////////////////////////////////////
/////// REGISTRATION
/////////////////////////////////////////////////////////////////////

func TestNestedRouterBasics(t *testing.T) {
	t.Run("Defaults", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		if nr.DynamicParamPrefix() != ':' {
			t.Error("Default dynamic param prefix should be ':'")
		}
		if nr.SplatSegmentIdentifier() != '*' {
			t.Error("Default splat segment should be '*'")
		}
		if nr.ExplicitIndexSegmentIdentifier() != "" {
			t.Error("Default explicit index segment should be empty")
		}
	})

	t.Run("WithOptions", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{
			DynamicParamPrefix:             '@',
			SplatSegmentIdentifier:         '#',
			ExplicitIndexSegmentIdentifier: "_index",
		})
		if nr.DynamicParamPrefix() != '@' {
			t.Error("DynamicParamPrefix not set correctly")
		}
		if nr.SplatSegmentIdentifier() != '#' {
			t.Error("SplatSegmentIdentifier not set correctly")
		}
		if nr.ExplicitIndexSegmentIdentifier() != "_index" {
			t.Error("ExplicitIndexSegmentIdentifier not set correctly")
		}
	})
}

func TestNestedRouteRegistration(t *testing.T) {
	t.Run("AddTaskHandler", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		handler := TaskHandlerFromFunc(
			func(rd *RequestCtx[None]) (string, error) { return "test", nil },
		)
		route := AddNestedTaskHandler(nr, "/test", handler)

		if route.OriginalPattern() != "/test" {
			t.Errorf("Expected '/test', got %q", route.OriginalPattern())
		}
		if !nr.IsRegistered("/test") {
			t.Error("Route should be registered")
		}
		if len(nr.AllRoutes()) != 1 {
			t.Errorf("Expected 1 route, got %d", len(nr.AllRoutes()))
		}
	})

	t.Run("AddPatternWithoutHandler", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "/static")
		if !nr.IsRegistered("/static") {
			t.Error("Pattern should be registered")
		}
	})

	t.Run("Duplicate_Panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("Expected panic on duplicate registration")
			}
		}()
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "/test")
		AddNestedPatternWithoutHandler(nr, "/test")
	})

	t.Run("AllRoutes_DefensiveCopy", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "/immutable")

		all := nr.AllRoutes()
		delete(all, "/immutable")
		all["/injected"] = nil

		if !nr.IsRegistered("/immutable") {
			t.Fatal("mutating AllRoutes() should not remove routes")
		}
		if nr.IsRegistered("/injected") {
			t.Fatal("mutating AllRoutes() should not inject routes")
		}
	})

	t.Run("Matcher_DefensiveCopy", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "/registered")

		matcher_copy := nr.Matcher()
		matcher_copy.RegisterPattern("/injected")

		req := create_request_with_tasks_ctx(http.MethodGet, "/injected")
		_, found := FindNestedMatches(nr, req)
		if found {
			t.Fatal("mutating Matcher() should not affect router")
		}

		req2 := create_request_with_tasks_ctx(http.MethodGet, "/registered")
		_, found2 := FindNestedMatches(nr, req2)
		if !found2 {
			t.Fatal("registered route should still match")
		}
	})

	t.Run("AddPatternWithoutHandlerIfMissing_Idempotent", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		if !nr.AddPatternWithoutHandlerIfMissing("/safe") {
			t.Fatal("expected first registration to return true")
		}
		if nr.AddPatternWithoutHandlerIfMissing("/safe") {
			t.Fatal("expected duplicate to return false")
		}
		if !nr.IsRegistered("/safe") {
			t.Fatal("pattern should remain registered")
		}
	})

	t.Run(
		"AddPatternWithoutHandlerIfMissing_ConcurrentSafe",
		func(t *testing.T) {
			nr := NewNestedRouter(NestedOptions{})
			start := make(chan struct{})
			const goroutines = 16
			var completed atomic.Int32
			var successes atomic.Int32
			done := make(chan struct{})

			for i := 0; i < goroutines; i++ {
				go func() {
					<-start
					if nr.AddPatternWithoutHandlerIfMissing("/concurrent") {
						successes.Add(1)
					}
					if completed.Add(1) == goroutines {
						close(done)
					}
				}()
			}
			close(start)
			<-done

			if successes.Load() != 1 {
				t.Fatalf("expected exactly 1 success, got %d", successes.Load())
			}
		},
	)
}

/////////////////////////////////////////////////////////////////////
/////// MATCHING
/////////////////////////////////////////////////////////////////////

func TestFindNestedMatchesMux(t *testing.T) {
	t.Run("Single_Match", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "/users")

		req := create_request_with_tasks_ctx(http.MethodGet, "/users")
		results, found := FindNestedMatches(nr, req)
		if !found {
			t.Error("Should find matches")
		}
		if len(results.Matches) != 1 {
			t.Errorf("Expected 1 match, got %d", len(results.Matches))
		}
	})

	t.Run("Nested_Matches", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "")
		AddNestedPatternWithoutHandler(nr, "/users")
		AddNestedPatternWithoutHandler(nr, "/users/:id")

		req := create_request_with_tasks_ctx(http.MethodGet, "/users/123")
		results, found := FindNestedMatches(nr, req)
		if !found {
			t.Error("Should find matches")
		}
		if len(results.Matches) != 3 {
			t.Errorf("Expected 3 matches, got %d", len(results.Matches))
		}
		if results.Params["id"] != "123" {
			t.Errorf("Expected param id='123', got %q", results.Params["id"])
		}
	})

	t.Run("No_Match", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "/users")

		req := create_request_with_tasks_ctx(http.MethodGet, "/posts")
		_, found := FindNestedMatches(nr, req)
		if found {
			t.Error("Should not find matches")
		}
	})

	t.Run("Splat_Pattern", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "/files/*")

		req := create_request_with_tasks_ctx(
			http.MethodGet,
			"/files/docs/readme.txt",
		)
		results, found := FindNestedMatches(nr, req)
		if !found {
			t.Error("Should find matches")
		}
		expected := []string{"docs", "readme.txt"}
		if !slice_equal(results.SplatValues, expected) {
			t.Errorf("Expected splat %v, got %v", expected, results.SplatValues)
		}
	})

	t.Run("ConcurrentRouteRegistration", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "/seed")

		const total = 512
		start := make(chan struct{})
		done := make(chan struct{})
		go func() {
			<-start
			for i := range total {
				nr.AddPatternWithoutHandlerIfMissing(
					fmt.Sprintf("/concurrent/%d", i),
				)
			}
			close(done)
		}()
		close(start)
		for i := 0; i < total; i++ {
			req := create_request_with_tasks_ctx(
				http.MethodGet,
				fmt.Sprintf("/concurrent/%d", i),
			)
			FindNestedMatches(nr, req)
		}
		<-done

		req := create_request_with_tasks_ctx(
			http.MethodGet,
			fmt.Sprintf("/concurrent/%d", total-1),
		)
		_, found := FindNestedMatches(nr, req)
		if !found {
			t.Fatal("expected concurrently registered route to be matchable")
		}
	})
}

/////////////////////////////////////////////////////////////////////
/////// TASK EXECUTION
/////////////////////////////////////////////////////////////////////

func TestRunNestedTasks(t *testing.T) {
	t.Run("Multiple_Tasks", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedTaskHandler(
			nr,
			"",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[None]) (map[string]string, error) {
					return map[string]string{"layout": "main"}, nil
				},
			),
		)
		AddNestedTaskHandler(
			nr,
			"/users",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[None]) (map[string]string, error) {
					return map[string]string{"page": "users"}, nil
				},
			),
		)
		AddNestedTaskHandler(
			nr,
			"/users/:id",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[None]) (map[string]string, error) {
					return map[string]string{"user": rd.Params()["id"]}, nil
				},
			),
		)

		req := create_request_with_tasks_ctx(http.MethodGet, "/users/456")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Error("Should find matches")
		}
		if len(results.Results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results.Results))
		}
		for i, r := range results.Results {
			if !r.OK() {
				t.Errorf("Task %d failed: %v", i, r.Err())
			}
			if !r.RanTask() {
				t.Errorf("Task %d should have run", i)
			}
		}

		user_result := find_nested_result(results, "/users/:id")
		if user_result == nil || user_result.Data() == nil {
			t.Error("User data missing")
		} else {
			data := user_result.Data().(map[string]string)
			if data["user"] != "456" {
				t.Errorf("Expected user='456', got %q", data["user"])
			}
		}
		if results.Params["id"] != "456" {
			t.Errorf("Expected param id='456', got %q", results.Params["id"])
		}
	})

	t.Run("Typed_Input", func(t *testing.T) {
		type search_input struct {
			Query string `json:"q"`
			Page  int    `json:"page"`
		}

		nr := NewNestedRouter(NestedOptions{
			ParseInput: func(r *http.Request, input_ptr any) error {
				return validate.URLSearchParamsInto(r, input_ptr)
			},
		})
		AddNestedTaskHandler(
			nr,
			"/users",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[search_input]) (map[string]any, error) {
					input := rd.Input()
					return map[string]any{
						"query": input.Query,
						"page":  input.Page,
					}, nil
				},
			),
		)

		req := create_request_with_tasks_ctx(http.MethodGet, "/users?q=ada&page=2")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("should find matches")
		}

		users_result := find_nested_result(results, "/users")
		if users_result == nil {
			t.Fatal("users result missing")
		}
		if !users_result.OK() {
			t.Fatalf("users task failed: %v", users_result.Err())
		}
		data := users_result.Data().(map[string]any)
		if data["query"] != "ada" {
			t.Fatalf("expected query ada, got %v", data["query"])
		}
		if data["page"] != 2 {
			t.Fatalf("expected page 2, got %v", data["page"])
		}
	})

	t.Run("Mixed_Handlers_And_No_Handlers", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		handler := TaskHandlerFromFunc(
			func(rd *RequestCtx[None]) (string, error) { return "with handler", nil },
		)
		AddNestedPatternWithoutHandler(nr, "")
		AddNestedTaskHandler(nr, "/page", handler)

		req := create_request_with_tasks_ctx(http.MethodGet, "/page")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Error("Should find matches")
		}

		root := find_nested_result(results, "")
		if root == nil {
			t.Error("Root result should exist")
		} else {
			if root.RanTask() {
				t.Error("Root should not have run a task")
			}
			if root.Data() != nil {
				t.Error("Root should have nil data")
			}
		}

		page := find_nested_result(results, "/page")
		if page == nil {
			t.Error("Page result should exist")
		} else {
			if !page.RanTask() {
				t.Error("Page should have run a task")
			}
			if page.Data() != "with handler" {
				t.Errorf("Expected 'with handler', got %v", page.Data())
			}
		}
	})

	t.Run("Task_Error_Handling", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedTaskHandler(
			nr,
			"/error",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				return "", &test_error{msg: "task failed"}
			}),
		)

		req := create_request_with_tasks_ctx(http.MethodGet, "/error")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Error("Should find matches")
		}
		r := find_nested_result(results, "/error")
		if r.OK() {
			t.Error("Result should not be OK")
		}
		if r.Err() == nil || r.Err().Error() != "task failed" {
			t.Errorf("Expected 'task failed', got %v", r.Err())
		}
	})

	t.Run("Error_Does_Not_Cancel_Siblings", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		var outer_ran atomic.Bool
		AddNestedTaskHandler(
			nr,
			"/items",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				outer_ran.Store(true)
				time.Sleep(25 * time.Millisecond)
				return "outer-ok", nil
			}),
		)
		AddNestedTaskHandler(
			nr,
			"/items/:id",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				return "", errors.New("inner failed")
			}),
		)

		req := create_request_with_tasks_ctx(http.MethodGet, "/items/123")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("should find matches")
		}

		outer := find_nested_result(results, "/items")
		if outer == nil {
			t.Fatal("missing outer result")
		}
		if !outer_ran.Load() {
			t.Fatal("outer handler did not run")
		}
		if outer.Err() != nil {
			t.Fatalf("outer err = %v, want nil", outer.Err())
		}
		if outer.Data() != "outer-ok" {
			t.Fatalf("outer data = %#v, want %#v", outer.Data(), "outer-ok")
		}

		inner := find_nested_result(results, "/items/:id")
		if inner == nil {
			t.Fatal("missing inner result")
		}
		if inner.Err() == nil || inner.Err().Error() != "inner failed" {
			t.Fatalf("inner err = %v, want %q", inner.Err(), "inner failed")
		}
	})

	t.Run("Parent_Error_Cancels_Descendants", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		var child_started atomic.Bool
		AddNestedTaskHandler(
			nr,
			"/items",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				return "", errors.New("parent failed")
			}),
		)
		AddNestedTaskHandler(
			nr,
			"/items/:id",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				child_started.Store(true)
				select {
				case <-rd.TasksCtx().NativeContext().Done():
					return "", rd.TasksCtx().NativeContext().Err()
				case <-time.After(150 * time.Millisecond):
					return "child-finished", nil
				}
			}),
		)

		req := create_request_with_tasks_ctx(http.MethodGet, "/items/123")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("should find matches")
		}

		parent := find_nested_result(results, "/items")
		if parent == nil || parent.Err() == nil ||
			parent.Err().Error() != "parent failed" {
			t.Fatalf("parent err = %v, want %q", parent.Err(), "parent failed")
		}

		child := find_nested_result(results, "/items/:id")
		if child == nil || child.Err() == nil {
			t.Fatal("child should have been canceled")
		}
		if !errors.Is(child.Err(), context.Canceled) {
			t.Fatalf("child err = %v, want context.Canceled", child.Err())
		}
	})

	t.Run("Parallel_Execution", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		sleep := 60 * time.Millisecond
		AddNestedTaskHandler(
			nr,
			"/parallel",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				time.Sleep(sleep)
				return "parent-ok", nil
			}),
		)
		AddNestedTaskHandler(
			nr,
			"/parallel/:id",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				time.Sleep(sleep)
				return "child-ok", nil
			}),
		)

		req := create_request_with_tasks_ctx(http.MethodGet, "/parallel/123")
		start := time.Now()
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		elapsed := time.Since(start)
		if !found {
			t.Fatal("should find matches")
		}
		parent := find_nested_result(results, "/parallel")
		child := find_nested_result(results, "/parallel/:id")
		if parent.Err() != nil || child.Err() != nil {
			t.Fatalf(
				"expected both to succeed, parent=%v child=%v",
				parent.Err(),
				child.Err(),
			)
		}
		if elapsed >= (sleep + 40*time.Millisecond) {
			t.Fatalf(
				"expected parallel execution, elapsed=%v (sleep=%v)",
				elapsed,
				sleep,
			)
		}
	})

	t.Run("Middle_Error_Cancels_Only_Descendants", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedTaskHandler(
			nr,
			"/items",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				time.Sleep(25 * time.Millisecond)
				return "parent-ok", nil
			}),
		)
		AddNestedTaskHandler(
			nr,
			"/items/:id",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				return "", errors.New("middle failed")
			}),
		)
		AddNestedTaskHandler(
			nr,
			"/items/:id/details",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				select {
				case <-rd.TasksCtx().NativeContext().Done():
					return "", rd.TasksCtx().NativeContext().Err()
				case <-time.After(150 * time.Millisecond):
					return "leaf-ok", nil
				}
			}),
		)

		req := create_request_with_tasks_ctx(
			http.MethodGet,
			"/items/123/details",
		)
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("should find matches")
		}

		parent := find_nested_result(results, "/items")
		if parent == nil || parent.Err() != nil {
			t.Fatalf("parent err = %v, want nil", parent.Err())
		}
		if parent.Data() != "parent-ok" {
			t.Fatalf("parent data = %#v, want %#v", parent.Data(), "parent-ok")
		}

		middle := find_nested_result(results, "/items/:id")
		if middle == nil || middle.Err() == nil ||
			middle.Err().Error() != "middle failed" {
			t.Fatalf("middle err = %v, want %q", middle.Err(), "middle failed")
		}

		leaf := find_nested_result(results, "/items/:id/details")
		if leaf == nil || !errors.Is(leaf.Err(), context.Canceled) {
			t.Fatalf("leaf err = %v, want context.Canceled", leaf.Err())
		}
	})

	t.Run("Request_Cancellation", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		wait_fn := func(rd *RequestCtx[None]) (string, error) {
			select {
			case <-rd.TasksCtx().NativeContext().Done():
				return "", rd.TasksCtx().NativeContext().Err()
			case <-time.After(250 * time.Millisecond):
				return "unexpected", nil
			}
		}
		AddNestedTaskHandler(nr, "/items", TaskHandlerFromFunc(wait_fn))
		AddNestedTaskHandler(nr, "/items/:id", TaskHandlerFromFunc(wait_fn))

		native_ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		req := httptest.NewRequest(http.MethodGet, "/items/123", nil).
			WithContext(native_ctx)
		req = RequestWithTasksCtx(req, tasks.NewCtx(req.Context()))

		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel()
		}()

		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("should find matches")
		}
		parent := find_nested_result(results, "/items")
		child := find_nested_result(results, "/items/:id")
		if parent == nil || child == nil {
			t.Fatal("missing results")
		}
		if !errors.Is(parent.Err(), context.Canceled) {
			t.Fatalf("parent err = %v, want context.Canceled", parent.Err())
		}
		if !errors.Is(child.Err(), context.Canceled) {
			t.Fatalf("child err = %v, want context.Canceled", child.Err())
		}
	})

	t.Run("SharedTaskDependency_RunsOnce", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		var shared_runs atomic.Int32
		shared := tasks.NewTask(func(c *tasks.Ctx, _ None) (string, error) {
			shared_runs.Add(1)
			time.Sleep(20 * time.Millisecond)
			return "shared-result", nil
		})

		AddNestedTaskHandler(
			nr,
			"/items",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				return shared.Run(rd.TasksCtx(), None{})
			}),
		)
		AddNestedTaskHandler(
			nr,
			"/items/:id",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				return shared.Run(rd.TasksCtx(), None{})
			}),
		)

		req := create_request_with_tasks_ctx(http.MethodGet, "/items/123")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("should find matches")
		}
		parent := find_nested_result(results, "/items")
		child := find_nested_result(results, "/items/:id")
		if parent == nil || child == nil {
			t.Fatal("missing results")
		}
		if parent.Err() != nil || child.Err() != nil {
			t.Fatalf(
				"expected success, parent=%v child=%v",
				parent.Err(),
				child.Err(),
			)
		}
		if parent.Data() != "shared-result" || child.Data() != "shared-result" {
			t.Fatalf(
				"unexpected data: parent=%#v child=%#v",
				parent.Data(),
				child.Data(),
			)
		}
		if shared_runs.Load() != 1 {
			t.Fatalf("shared runs = %d, want 1", shared_runs.Load())
		}
	})

	t.Run("HasTaskHandlerAt", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "/no-handler")
		AddNestedTaskHandler(
			nr,
			"/with-handler",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[None]) (string, error) { return "test", nil },
			),
		)

		req := create_request_with_tasks_ctx(http.MethodGet, "/with-handler")
		matches, _ := FindNestedMatches(nr, req)
		results := RunNestedTasks(nr, req, matches)

		for i, r := range results.Results {
			has := results.HasTaskHandlerAt(i)
			if r.RanTask() && !has {
				t.Errorf(
					"Index %d ran task but HasTaskHandlerAt returned false",
					i,
				)
			}
			if !r.RanTask() && has {
				t.Errorf(
					"Index %d didn't run task but HasTaskHandlerAt returned true",
					i,
				)
			}
		}
	})
}

/////////////////////////////////////////////////////////////////////
/////// EXPLICIT INDEX
/////////////////////////////////////////////////////////////////////

func TestNestedRouterWithExplicitIndex(t *testing.T) {
	nr := NewNestedRouter(
		NestedOptions{ExplicitIndexSegmentIdentifier: "_index"},
	)
	AddNestedTaskHandler(
		nr,
		"/users/_index",
		TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
			return "index page", nil
		}),
	)

	req := create_request_with_tasks_ctx(http.MethodGet, "/users/")
	results, found := FindNestedMatchesAndRunTasks(nr, req)
	if !found {
		t.Error("Should find matches with explicit index")
	}
	r := find_nested_result(results, "/users/_index")
	if r == nil {
		t.Error("Should have result for index pattern")
	} else if r.Data() != "index page" {
		t.Errorf("Expected 'index page', got %v", r.Data())
	}
}

/////////////////////////////////////////////////////////////////////
/////// RESPONSE PROXIES
/////////////////////////////////////////////////////////////////////

func TestNestedResponseProxies(t *testing.T) {
	nr := NewNestedRouter(NestedOptions{})
	AddNestedTaskHandler(
		nr,
		"/",
		TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
			rd.ResponseProxy().SetHeader("X-Handler-1", "value1")
			return "handler1", nil
		}),
	)
	AddNestedPatternWithoutHandler(nr, "/page")
	AddNestedTaskHandler(
		nr,
		"/page/details",
		TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
			rd.ResponseProxy().SetHeader("X-Handler-2", "value2")
			return "handler2", nil
		}),
	)

	req := create_request_with_tasks_ctx(http.MethodGet, "/page/details")
	results, _ := FindNestedMatchesAndRunTasks(nr, req)

	if len(results.ResponseProxies) != len(results.Results) {
		t.Errorf(
			"Expected %d proxies, got %d",
			len(results.Results),
			len(results.ResponseProxies),
		)
	}
	for i, proxy := range results.ResponseProxies {
		if results.Results[i].RanTask() {
			if proxy == nil {
				t.Errorf("proxy at %d should exist for task handler", i)
			}
		} else {
			if proxy != nil {
				t.Errorf("proxy at %d should be nil for no-handler route", i)
			}
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// ROUTE REPLACEMENT
/////////////////////////////////////////////////////////////////////

func TestNestedRouterRouteReplacement(t *testing.T) {
	t.Run("ReplaceRoutes", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedTaskHandler(
			nr,
			"/old",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[None]) (string, error) { return "old", nil },
			),
		)
		AddNestedPatternWithoutHandler(nr, "/legacy")

		replacement := NewNestedRouter(NestedOptions{})
		AddNestedTaskHandler(
			replacement,
			"/fresh",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[None]) (string, error) { return "new", nil },
			),
		)
		AddNestedPatternWithoutHandler(replacement, "/static")
		nr.ReplaceRoutes(replacement.AllRoutes())

		if nr.IsRegistered("/old") || nr.IsRegistered("/legacy") {
			t.Fatal("old routes should be removed")
		}
		if !nr.IsRegistered("/fresh") || !nr.IsRegistered("/static") {
			t.Fatal("new routes should be registered")
		}

		req := create_request_with_tasks_ctx(http.MethodGet, "/old")
		if _, found := FindNestedMatchesAndRunTasks(nr, req); found {
			t.Fatal("/old should not match")
		}

		req2 := create_request_with_tasks_ctx(http.MethodGet, "/fresh")
		results, found := FindNestedMatchesAndRunTasks(nr, req2)
		if !found {
			t.Fatal("/fresh should match")
		}
		r := find_nested_result(results, "/fresh")
		if r == nil || r.Err() != nil || r.Data() != "new" {
			t.Fatalf("/fresh result: data=%#v err=%v", r.Data(), r.Err())
		}
	})

	t.Run("ReplaceRoutes_DefensiveCopy", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		replacement := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(replacement, "/initial")
		routes := replacement.AllRoutes()
		nr.ReplaceRoutes(routes)

		delete(routes, "/initial")
		routes["/injected"] = nil

		if !nr.IsRegistered("/initial") {
			t.Fatal("mutating input should not remove route")
		}
		if nr.IsRegistered("/injected") {
			t.Fatal("mutating input should not inject route")
		}
	})

	t.Run("RebuildPreservingHandlers", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedTaskHandler(
			nr,
			"/keep",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[None]) (string, error) { return "keep", nil },
			),
		)
		AddNestedPatternWithoutHandler(nr, "/drop")
		AddNestedPatternWithoutHandler(nr, "/will-be-replaced")

		nr.RebuildPreservingHandlers([]string{"/keep", "/new-no-handler"})

		if !nr.IsRegistered("/keep") || !nr.IsRegistered("/new-no-handler") {
			t.Fatal("expected /keep and /new-no-handler to be registered")
		}
		if nr.IsRegistered("/drop") || nr.IsRegistered("/will-be-replaced") {
			t.Fatal("old no-handler routes should be removed")
		}

		req := create_request_with_tasks_ctx(http.MethodGet, "/keep")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("/keep should match")
		}
		r := find_nested_result(results, "/keep")
		if r == nil || r.Err() != nil || r.Data() != "keep" {
			t.Fatalf("/keep result: data=%#v err=%v", r.Data(), r.Err())
		}

		req2 := create_request_with_tasks_ctx(http.MethodGet, "/new-no-handler")
		results2, found := FindNestedMatchesAndRunTasks(nr, req2)
		if !found {
			t.Fatal("/new-no-handler should match")
		}
		r2 := find_nested_result(results2, "/new-no-handler")
		if r2 == nil {
			t.Fatal("missing /new-no-handler result")
		}
		if r2.RanTask() {
			t.Fatal("/new-no-handler should not run a task")
		}
	})

	t.Run("ConcurrentReplaceRoutes_ConsistentResults", func(t *testing.T) {
		nr := NewNestedRouter(NestedOptions{})

		build_routes := func() map[string]AnyNestedRoute {
			build_handler := func(pat string) *TaskHandler[None, string] {
				return TaskHandlerFromFunc(
					func(*RequestCtx[None]) (string, error) { return pat, nil },
				)
			}
			tmp := NewNestedRouter(NestedOptions{})
			AddNestedTaskHandler(tmp, "", build_handler(""))
			AddNestedTaskHandler(tmp, "/items", build_handler("/items"))
			AddNestedTaskHandler(tmp, "/items/:id", build_handler("/items/:id"))
			return tmp.AllRoutes()
		}

		nr.ReplaceRoutes(build_routes())

		stop := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				select {
				case <-stop:
					return
				default:
					nr.ReplaceRoutes(build_routes())
				}
			}
		}()

		expected_patterns := []string{"", "/items", "/items/:id"}
		for i := 0; i < 500; i++ {
			req := create_request_with_tasks_ctx(http.MethodGet, "/items/123")
			results, found := FindNestedMatchesAndRunTasks(nr, req)
			if !found {
				t.Fatal("expected /items/123 to match during replacement")
			}
			for _, pat := range expected_patterns {
				r := find_nested_result(results, pat)
				if r == nil {
					t.Fatalf("missing result for %q", pat)
				}
				if r.Err() != nil {
					t.Fatalf("pattern %q error: %v", pat, r.Err())
				}
				data, ok := r.Data().(string)
				if !ok {
					t.Fatalf(
						"pattern %q data type = %T, want string",
						pat,
						r.Data(),
					)
				}
				if data != pat {
					t.Fatalf("pattern %q returned data %q", pat, data)
				}
			}
		}

		close(stop)
		<-done
	})
}

/////////////////////////////////////////////////////////////////////
/////// BENCHMARKS
/////////////////////////////////////////////////////////////////////

func BenchmarkNestedRouter(b *testing.B) {
	b.Run("Simple_Nested_Match", func(b *testing.B) {
		nr := NewNestedRouter(NestedOptions{})
		AddNestedPatternWithoutHandler(nr, "/")
		AddNestedPatternWithoutHandler(nr, "/users")
		AddNestedPatternWithoutHandler(nr, "/users/:id")
		req := create_request_with_tasks_ctx(http.MethodGet, "/users/123")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			FindNestedMatches(nr, req)
		}
	})

	b.Run("Nested_Tasks_Execution", func(b *testing.B) {
		nr := NewNestedRouter(NestedOptions{})
		handler := TaskHandlerFromFunc(
			func(rd *RequestCtx[None]) (map[string]string, error) {
				return map[string]string{"id": rd.Params()["id"]}, nil
			},
		)
		AddNestedTaskHandler(nr, "/", handler)
		AddNestedTaskHandler(nr, "/users", handler)
		AddNestedTaskHandler(nr, "/users/:id", handler)
		req := create_request_with_tasks_ctx(http.MethodGet, "/users/123")
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			FindNestedMatchesAndRunTasks(nr, req)
		}
	})

	b.Run("Deep_Nesting", func(b *testing.B) {
		nr := NewNestedRouter(NestedOptions{})
		for _, p := range []string{"/", "/app", "/app/dashboard", "/app/dashboard/users", "/app/dashboard/users/:id", "/app/dashboard/users/:id/profile", "/app/dashboard/users/:id/profile/settings"} {
			AddNestedPatternWithoutHandler(nr, p)
		}
		req := create_request_with_tasks_ctx(
			http.MethodGet,
			"/app/dashboard/users/123/profile/settings",
		)
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			FindNestedMatches(nr, req)
		}
	})
}
