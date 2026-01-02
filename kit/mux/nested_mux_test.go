package mux

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
