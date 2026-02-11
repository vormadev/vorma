package mux

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/tasks"
)

func TestNestedRouterBasics(t *testing.T) {
	t.Run("NewNestedRouter_WithDefaults", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		if nr.GetDynamicParamPrefixRune() != ':' {
			t.Error("Default dynamic param prefix should be ':'")
		}
		if nr.GetSplatSegmentRune() != '*' {
			t.Error("Default splat segment should be '*'")
		}
		if nr.GetExplicitIndexSegment() != "" {
			t.Error("Default explicit index segment should be empty")
		}
	})

	t.Run("NewNestedRouter_WithOptions", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{
			DynamicParamPrefixRune: '@',
			SplatSegmentRune:       '#',
			ExplicitIndexSegment:   "_index",
		})

		if nr.GetDynamicParamPrefixRune() != '@' {
			t.Error("DynamicParamPrefixRune not set correctly")
		}
		if nr.GetSplatSegmentRune() != '#' {
			t.Error("SplatSegmentRune not set correctly")
		}
		if nr.GetExplicitIndexSegment() != "_index" {
			t.Error("ExplicitIndexSegment not set correctly")
		}
	})
}

func TestNestedRouteRegistration(t *testing.T) {
	t.Run("RegisterNestedTaskHandler", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		handler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "test result", nil
		})

		route := RegisterNestedTaskHandler(nr, "/test", handler)

		if route.OriginalPattern() != "/test" {
			t.Errorf("Expected pattern '/test', got %q", route.OriginalPattern())
		}
		if !nr.IsRegistered("/test") {
			t.Error("Route should be registered")
		}

		allRoutes := nr.AllRoutes()
		if len(allRoutes) != 1 {
			t.Errorf("Expected 1 route, got %d", len(allRoutes))
		}
	})

	t.Run("RegisterNestedPatternWithoutHandler", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		RegisterNestedPatternWithoutHandler(nr, "/static")

		if !nr.IsRegistered("/static") {
			t.Error("Pattern should be registered")
		}

		// Verify the route exists but has no handler
		route := nr.AllRoutes()["/static"]
		if route == nil {
			t.Error("Route should exist")
		}
	})

	t.Run("Duplicate_Registration_Panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic on duplicate registration")
			}
		}()

		nr := NewNestedRouter(&NestedOptions{})

		RegisterNestedPatternWithoutHandler(nr, "/test")
		RegisterNestedPatternWithoutHandler(nr, "/test") // Should panic
	})
}

func TestFindNestedMatches(t *testing.T) {
	t.Run("Single_Match", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		RegisterNestedPatternWithoutHandler(nr, "/users")

		req := createRequestWithTasksCtx(http.MethodGet, "/users")
		results, found := FindNestedMatches(nr, req)

		if !found {
			t.Error("Should find matches")
		}
		if len(results.Matches) != 1 {
			t.Errorf("Expected 1 match, got %d", len(results.Matches))
		}
		if results.Matches[0].OriginalPattern() != "/users" {
			t.Errorf("Expected pattern '/users', got %q", results.Matches[0].OriginalPattern())
		}
	})

	t.Run("Nested_Matches", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		// Register nested patterns like a UI router would have
		RegisterNestedPatternWithoutHandler(nr, "") // empty because we have no explicit index
		RegisterNestedPatternWithoutHandler(nr, "/users")
		RegisterNestedPatternWithoutHandler(nr, "/users/:id")

		req := createRequestWithTasksCtx(http.MethodGet, "/users/123")
		results, found := FindNestedMatches(nr, req)

		if !found {
			t.Error("Should find matches")
		}
		if len(results.Matches) != 3 {
			t.Errorf("Expected 3 matches, got %d", len(results.Matches))
		}

		// Verify all patterns matched
		patterns := make(map[string]bool)
		for _, match := range results.Matches {
			patterns[match.OriginalPattern()] = true
		}

		expectedPatterns := []string{"", "/users", "/users/:id"}
		for _, expected := range expectedPatterns {
			if !patterns[expected] {
				t.Errorf("Expected pattern %q to match", expected)
			}
		}

		// Check params
		if results.Params["id"] != "123" {
			t.Errorf("Expected param id='123', got %q", results.Params["id"])
		}
	})

	t.Run("No_Match", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		RegisterNestedPatternWithoutHandler(nr, "/users")

		req := createRequestWithTasksCtx(http.MethodGet, "/posts")
		_, found := FindNestedMatches(nr, req)

		if found {
			t.Error("Should not find matches for unregistered path")
		}
	})

	t.Run("Splat_Pattern", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		RegisterNestedPatternWithoutHandler(nr, "/files/*")

		req := createRequestWithTasksCtx(http.MethodGet, "/files/docs/readme.txt")
		results, found := FindNestedMatches(nr, req)

		if !found {
			t.Error("Should find matches")
		}
		if len(results.SplatValues) != 2 {
			t.Errorf("Expected 2 splat values, got %d", len(results.SplatValues))
		}
		expectedSplat := []string{"docs", "readme.txt"}
		if !sliceEqual(results.SplatValues, expectedSplat) {
			t.Errorf("Expected splat values %v, got %v", expectedSplat, results.SplatValues)
		}
	})
}

func TestRunNestedTasks(t *testing.T) {
	t.Run("Run_Multiple_Tasks", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		// Register handlers that return different data
		layoutHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (map[string]string, error) {
			return map[string]string{"layout": "main"}, nil
		})
		pageHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (map[string]string, error) {
			return map[string]string{"page": "users"}, nil
		})
		userHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (map[string]string, error) {
			return map[string]string{"user": rd.Params()["id"]}, nil
		})

		RegisterNestedTaskHandler(nr, "", layoutHandler) // empty because we have no explicit index
		RegisterNestedTaskHandler(nr, "/users", pageHandler)
		RegisterNestedTaskHandler(nr, "/users/:id", userHandler)

		req := createRequestWithTasksCtx(http.MethodGet, "/users/456")

		results, found := FindNestedMatchesAndRunTasks(nr, req)

		if !found {
			t.Error("Should find matches")
		}
		if len(results.Slice) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results.Slice))
		}

		// Verify all tasks ran successfully
		for i, result := range results.Slice {
			if !result.OK() {
				t.Errorf("Task %d failed: %v", i, result.Err())
			}
			if !result.RanTask() {
				t.Errorf("Task %d should have run", i)
			}
		}

		// Verify data from specific patterns
		layoutData := results.Map[""]
		if layoutData == nil || layoutData.Data() == nil {
			t.Error("Layout data missing")
		}

		userData := results.Map["/users/:id"]
		if userData == nil || userData.Data() == nil {
			t.Error("User data missing")
		} else {
			data := userData.Data().(map[string]string)
			if data["user"] != "456" {
				t.Errorf("Expected user='456', got %q", data["user"])
			}
		}

		// Verify params are accessible
		if results.Params["id"] != "456" {
			t.Errorf("Expected param id='456', got %q", results.Params["id"])
		}
	})

	t.Run("Mixed_Handlers_And_No_Handlers", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		handler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "with handler", nil
		})

		RegisterNestedPatternWithoutHandler(nr, "/static")
		RegisterNestedTaskHandler(nr, "/dynamic", handler)

		// Should match both patterns since /dynamic matches both /static and /dynamic patterns
		// Wait, actually looking at the matcher, it would only match /dynamic
		// Let me adjust the test to have patterns that would both match

		// Reset and use patterns that would both match a single request
		nr = NewNestedRouter(&NestedOptions{})
		RegisterNestedPatternWithoutHandler(nr, "") // empty because we have no explicit index
		RegisterNestedTaskHandler(nr, "/page", handler)

		req := createRequestWithTasksCtx(http.MethodGet, "/page")
		results, found := FindNestedMatchesAndRunTasks(nr, req)

		if !found {
			t.Error("Should find matches")
		}

		// Check that pattern without handler exists but didn't run a task
		rootResult := results.Map[""]
		if rootResult == nil {
			t.Error("Root result should exist")
		}
		if rootResult != nil && rootResult.RanTask() {
			t.Error("Root should not have run a task")
		}
		if rootResult != nil && rootResult.Data() != nil {
			t.Error("Root should have nil data")
		}

		// Check that pattern with handler ran its task
		pageResult := results.Map["/page"]
		if pageResult == nil {
			t.Error("Page result should exist")
		}
		if !pageResult.RanTask() {
			t.Error("Page should have run a task")
		}
		if pageResult.Data() != "with handler" {
			t.Errorf("Expected 'with handler', got %v", pageResult.Data())
		}
	})

	t.Run("Task_Error_Handling", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		errorHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "", &testError{msg: "task failed"}
		})

		RegisterNestedTaskHandler(nr, "/error", errorHandler)

		req := createRequestWithTasksCtx(http.MethodGet, "/error")

		results, found := FindNestedMatchesAndRunTasks(nr, req)

		if !found {
			t.Error("Should find matches")
		}

		errorResult := results.Map["/error"]
		if errorResult.OK() {
			t.Error("Result should not be OK when task errors")
		}
		if errorResult.Err() == nil {
			t.Error("Should have error")
		}
		if errorResult.Err().Error() != "task failed" {
			t.Errorf("Expected 'task failed', got %q", errorResult.Err().Error())
		}
	})

	t.Run("Task_Error_Does_Not_Cancel_Siblings", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		var outerRan atomic.Bool
		outerHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			outerRan.Store(true)
			// Give the sibling error path time to complete first.
			time.Sleep(25 * time.Millisecond)
			return "outer-ok", nil
		})
		innerHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "", errors.New("inner failed")
		})

		RegisterNestedTaskHandler(nr, "/items", outerHandler)
		RegisterNestedTaskHandler(nr, "/items/:id", innerHandler)

		req := createRequestWithTasksCtx(http.MethodGet, "/items/123")
		results, found := FindNestedMatchesAndRunTasks(nr, req)

		if !found {
			t.Fatal("should find matches")
		}

		outer := results.Map["/items"]
		if outer == nil {
			t.Fatal("missing outer result")
		}
		if !outerRan.Load() {
			t.Fatal("outer handler did not run")
		}
		if err := outer.Err(); err != nil {
			t.Fatalf("outer error = %v, want nil", err)
		}
		if got := outer.Data(); got != "outer-ok" {
			t.Fatalf("outer data = %#v, want %#v", got, "outer-ok")
		}

		inner := results.Map["/items/:id"]
		if inner == nil {
			t.Fatal("missing inner result")
		}
		if err := inner.Err(); err == nil || err.Error() != "inner failed" {
			t.Fatalf("inner err = %v, want %q", err, "inner failed")
		}
	})

	t.Run("Parent_Error_Cancels_Descendants", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		var childStarted atomic.Bool
		parentHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "", errors.New("parent failed")
		})
		childHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			childStarted.Store(true)
			select {
			case <-rd.TasksCtx().NativeContext().Done():
				return "", rd.TasksCtx().NativeContext().Err()
			case <-time.After(150 * time.Millisecond):
				return "child-finished", nil
			}
		})

		RegisterNestedTaskHandler(nr, "/items", parentHandler)
		RegisterNestedTaskHandler(nr, "/items/:id", childHandler)

		req := createRequestWithTasksCtx(http.MethodGet, "/items/123")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("should find matches")
		}

		parent := results.Map["/items"]
		if parent == nil {
			t.Fatal("missing parent result")
		}
		if err := parent.Err(); err == nil || err.Error() != "parent failed" {
			t.Fatalf("parent err = %v, want %q", err, "parent failed")
		}

		child := results.Map["/items/:id"]
		if child == nil {
			t.Fatal("missing child result")
		}
		if err := child.Err(); err == nil {
			t.Fatal("child should have been canceled after parent failure")
		}
		if err := child.Err(); !errors.Is(err, context.Canceled) {
			t.Fatalf("child err = %v, want context.Canceled", err)
		}

		// Child may or may not have started before cancellation won the race.
		_ = childStarted.Load()
	})

	t.Run("Matched_Tasks_Still_Run_In_Parallel", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		sleep := 60 * time.Millisecond
		parentHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			time.Sleep(sleep)
			return "parent-ok", nil
		})
		childHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			time.Sleep(sleep)
			return "child-ok", nil
		})

		RegisterNestedTaskHandler(nr, "/parallel", parentHandler)
		RegisterNestedTaskHandler(nr, "/parallel/:id", childHandler)

		req := createRequestWithTasksCtx(http.MethodGet, "/parallel/123")
		start := time.Now()
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		elapsed := time.Since(start)
		if !found {
			t.Fatal("should find matches")
		}
		if results.Map["/parallel"].Err() != nil || results.Map["/parallel/:id"].Err() != nil {
			t.Fatalf("expected both tasks to succeed, got parent=%v child=%v",
				results.Map["/parallel"].Err(), results.Map["/parallel/:id"].Err())
		}

		// Sequential would be around 2*sleep; parallel should be close to sleep.
		if elapsed >= (sleep + 40*time.Millisecond) {
			t.Fatalf("expected parallel execution, elapsed=%v (sleep=%v)", elapsed, sleep)
		}
	})

	t.Run("Middle_Error_Cancels_Only_Descendants", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		parentHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			time.Sleep(25 * time.Millisecond)
			return "parent-ok", nil
		})
		middleHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "", errors.New("middle failed")
		})
		leafHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			select {
			case <-rd.TasksCtx().NativeContext().Done():
				return "", rd.TasksCtx().NativeContext().Err()
			case <-time.After(150 * time.Millisecond):
				return "leaf-ok", nil
			}
		})

		RegisterNestedTaskHandler(nr, "/items", parentHandler)
		RegisterNestedTaskHandler(nr, "/items/:id", middleHandler)
		RegisterNestedTaskHandler(nr, "/items/:id/details", leafHandler)

		req := createRequestWithTasksCtx(http.MethodGet, "/items/123/details")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("should find matches")
		}

		parent := results.Map["/items"]
		if parent == nil {
			t.Fatal("missing parent result")
		}
		if err := parent.Err(); err != nil {
			t.Fatalf("parent err = %v, want nil", err)
		}
		if got := parent.Data(); got != "parent-ok" {
			t.Fatalf("parent data = %#v, want %#v", got, "parent-ok")
		}

		middle := results.Map["/items/:id"]
		if middle == nil {
			t.Fatal("missing middle result")
		}
		if err := middle.Err(); err == nil || err.Error() != "middle failed" {
			t.Fatalf("middle err = %v, want %q", err, "middle failed")
		}

		leaf := results.Map["/items/:id/details"]
		if leaf == nil {
			t.Fatal("missing leaf result")
		}
		if err := leaf.Err(); !errors.Is(err, context.Canceled) {
			t.Fatalf("leaf err = %v, want context.Canceled", err)
		}
	})

	t.Run("SharedTaskDependencyRunsOnceAcrossNestedHandlers", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		var sharedRuns atomic.Int32
		sharedTask := tasks.NewTask(func(c *tasks.Ctx, _ None) (string, error) {
			sharedRuns.Add(1)
			time.Sleep(20 * time.Millisecond)
			return "shared-result", nil
		})

		parentHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return sharedTask.Run(rd.TasksCtx(), None{})
		})
		childHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return sharedTask.Run(rd.TasksCtx(), None{})
		})

		RegisterNestedTaskHandler(nr, "/items", parentHandler)
		RegisterNestedTaskHandler(nr, "/items/:id", childHandler)

		req := createRequestWithTasksCtx(http.MethodGet, "/items/123")
		results, found := FindNestedMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("should find matches")
		}

		parent := results.Map["/items"]
		child := results.Map["/items/:id"]
		if parent == nil || child == nil {
			t.Fatal("missing nested results")
		}
		if parent.Err() != nil || child.Err() != nil {
			t.Fatalf("expected both handlers to succeed, got parent=%v child=%v", parent.Err(), child.Err())
		}
		if parent.Data() != "shared-result" || child.Data() != "shared-result" {
			t.Fatalf("unexpected shared data: parent=%#v child=%#v", parent.Data(), child.Data())
		}
		if sharedRuns.Load() != 1 {
			t.Fatalf("shared dependency runs = %d, want 1", sharedRuns.Load())
		}
	})

	t.Run("GetHasTaskHandler", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		handler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "test", nil
		})

		RegisterNestedPatternWithoutHandler(nr, "/no-handler")
		RegisterNestedTaskHandler(nr, "/with-handler", handler)

		req := createRequestWithTasksCtx(http.MethodGet, "/with-handler")
		matches, _ := FindNestedMatches(nr, req)

		results := RunNestedTasks(nr, req, matches)

		// The results should track which indices had task handlers
		// Based on the order of matches, we need to check the right indices
		for i, result := range results.Slice {
			hasHandler := results.GetHasTaskHandler(i)
			if result.RanTask() && !hasHandler {
				t.Errorf("Index %d ran task but GetHasTaskHandler returned false", i)
			}
			if !result.RanTask() && hasHandler {
				t.Errorf("Index %d didn't run task but GetHasTaskHandler returned true", i)
			}
		}
	})
}

func TestNestedRouterWithExplicitIndex(t *testing.T) {
	t.Run("Explicit_Index_Segment", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{
			ExplicitIndexSegment: "_index",
		})

		handler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "index page", nil
		})

		// With explicit index, you'd register like this instead of trailing slash
		RegisterNestedTaskHandler(nr, "/users/_index", handler)

		// This would match /users/ or /users
		req := createRequestWithTasksCtx(http.MethodGet, "/users/")
		results, found := FindNestedMatchesAndRunTasks(nr, req)

		if !found {
			t.Error("Should find matches with explicit index")
		}

		indexResult := results.Map["/users/_index"]
		if indexResult == nil {
			t.Error("Should have result for index pattern")
		}
		if indexResult.Data() != "index page" {
			t.Errorf("Expected 'index page', got %v", indexResult.Data())
		}
	})
}

func TestNestedRouterRouteReplacement(t *testing.T) {
	t.Run("ReplaceRoutes_ReplacesMatcherAndCompiledRoutes", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		oldHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "old", nil
		})
		newHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "new", nil
		})

		RegisterNestedTaskHandler(nr, "/old", oldHandler)
		RegisterNestedPatternWithoutHandler(nr, "/legacy")

		nr.ReplaceRoutes(map[string]AnyNestedRoute{
			"/fresh": &NestedRoute[string]{
				router:          nr,
				originalPattern: "/fresh",
				taskHandler:     newHandler,
			},
			"/static": &NestedRoute[None]{
				router:          nr,
				originalPattern: "/static",
				taskHandler:     nil,
			},
		})

		if nr.IsRegistered("/old") {
			t.Fatal("expected /old to be removed after ReplaceRoutes")
		}
		if nr.IsRegistered("/legacy") {
			t.Fatal("expected /legacy to be removed after ReplaceRoutes")
		}
		if !nr.IsRegistered("/fresh") || !nr.IsRegistered("/static") {
			t.Fatal("expected replacement routes to be registered")
		}

		reqOld := createRequestWithTasksCtx(http.MethodGet, "/old")
		if _, found := FindNestedMatchesAndRunTasks(nr, reqOld); found {
			t.Fatal("expected /old not to match after ReplaceRoutes")
		}

		reqFresh := createRequestWithTasksCtx(http.MethodGet, "/fresh")
		results, found := FindNestedMatchesAndRunTasks(nr, reqFresh)
		if !found {
			t.Fatal("expected /fresh to match after ReplaceRoutes")
		}
		fresh := results.Map["/fresh"]
		if fresh == nil {
			t.Fatal("missing /fresh result")
		}
		if err := fresh.Err(); err != nil {
			t.Fatalf("/fresh err = %v, want nil", err)
		}
		if got := fresh.Data(); got != "new" {
			t.Fatalf("/fresh data = %#v, want %#v", got, "new")
		}
	})

	t.Run("RebuildPreservingHandlers_PreservesOnlyHandlerRoutes", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		keepHandler := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			return "keep", nil
		})

		RegisterNestedTaskHandler(nr, "/keep", keepHandler)
		RegisterNestedPatternWithoutHandler(nr, "/drop-no-handler")
		RegisterNestedPatternWithoutHandler(nr, "/will-be-replaced")

		nr.RebuildPreservingHandlers([]string{"/keep", "/new-no-handler"})

		if !nr.IsRegistered("/keep") {
			t.Fatal("expected /keep to remain registered")
		}
		if !nr.IsRegistered("/new-no-handler") {
			t.Fatal("expected /new-no-handler to be registered")
		}
		if nr.IsRegistered("/drop-no-handler") || nr.IsRegistered("/will-be-replaced") {
			t.Fatal("expected old no-handler routes to be removed")
		}

		reqKeep := createRequestWithTasksCtx(http.MethodGet, "/keep")
		keepResults, found := FindNestedMatchesAndRunTasks(nr, reqKeep)
		if !found {
			t.Fatal("expected /keep to match after rebuild")
		}
		keep := keepResults.Map["/keep"]
		if keep == nil || keep.Err() != nil || keep.Data() != "keep" {
			t.Fatalf("/keep result unexpected: data=%#v err=%v", keep.Data(), keep.Err())
		}

		reqNew := createRequestWithTasksCtx(http.MethodGet, "/new-no-handler")
		newResults, found := FindNestedMatchesAndRunTasks(nr, reqNew)
		if !found {
			t.Fatal("expected /new-no-handler to match after rebuild")
		}
		newRouteResult := newResults.Map["/new-no-handler"]
		if newRouteResult == nil {
			t.Fatal("missing /new-no-handler result")
		}
		if newRouteResult.RanTask() {
			t.Fatal("/new-no-handler should not run a task")
		}
	})
}

func TestResponseProxies(t *testing.T) {
	t.Run("Response_Proxies_Created", func(t *testing.T) {
		nr := NewNestedRouter(&NestedOptions{})

		handler1 := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			rd.ResponseProxy().SetHeader("X-Handler-1", "value1")
			return "handler1", nil
		})
		handler2 := TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			rd.ResponseProxy().SetHeader("X-Handler-2", "value2")
			return "handler2", nil
		})

		RegisterNestedTaskHandler(nr, "/", handler1)
		RegisterNestedTaskHandler(nr, "/page", handler2)

		req := createRequestWithTasksCtx(http.MethodGet, "/page")

		results, _ := FindNestedMatchesAndRunTasks(nr, req)

		// Verify we have response proxies for each match
		if len(results.ResponseProxies) != len(results.Slice) {
			t.Errorf("Expected %d response proxies, got %d",
				len(results.Slice), len(results.ResponseProxies))
		}

		// In a real scenario, these would be merged and applied to the response
		// Here we just verify they exist
		for i, proxy := range results.ResponseProxies {
			if proxy == nil {
				t.Errorf("Response proxy at index %d is nil", i)
			}
		}
	})
}

// Test error type
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// Benchmarks
func BenchmarkNestedRouter(b *testing.B) {
	b.Run("Simple_Nested_Match", func(b *testing.B) {
		nr := NewNestedRouter(&NestedOptions{})

		RegisterNestedPatternWithoutHandler(nr, "/")
		RegisterNestedPatternWithoutHandler(nr, "/users")
		RegisterNestedPatternWithoutHandler(nr, "/users/:id")

		req := createRequestWithTasksCtx(http.MethodGet, "/users/123")

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			FindNestedMatches(nr, req)
		}
	})

	b.Run("Nested_Tasks_Execution", func(b *testing.B) {
		nr := NewNestedRouter(&NestedOptions{})

		handler := TaskHandlerFromFunc(func(rd *ReqData[None]) (map[string]string, error) {
			return map[string]string{"id": rd.Params()["id"]}, nil
		})

		RegisterNestedTaskHandler(nr, "/", handler)
		RegisterNestedTaskHandler(nr, "/users", handler)
		RegisterNestedTaskHandler(nr, "/users/:id", handler)

		req := createRequestWithTasksCtx(http.MethodGet, "/users/123")

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			FindNestedMatchesAndRunTasks(nr, req)
		}
	})

	b.Run("Deep_Nesting", func(b *testing.B) {
		nr := NewNestedRouter(&NestedOptions{})

		// Create a deeply nested route structure
		patterns := []string{
			"/",
			"/app",
			"/app/dashboard",
			"/app/dashboard/users",
			"/app/dashboard/users/:id",
			"/app/dashboard/users/:id/profile",
			"/app/dashboard/users/:id/profile/settings",
		}

		for _, pattern := range patterns {
			RegisterNestedPatternWithoutHandler(nr, pattern)
		}

		req := createRequestWithTasksCtx(http.MethodGet, "/app/dashboard/users/123/profile/settings")

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			FindNestedMatches(nr, req)
		}
	})
}

func createRequestWithTasksCtx(method, url string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	tasksCtx := tasks.NewCtx(req.Context())
	rd := &rdTransport{tasksCtx: tasksCtx, req: req}
	return requestStore.GetRequestWithContext(req, rd)
}
