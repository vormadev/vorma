package nestedmux_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/mux/nestedmux"
	"github.com/vormadev/vorma/kit/tasks"
)

func TestNestedRouterBasics(t *testing.T) {
	t.Run("NewNestedRouter_WithDefaults", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

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

	t.Run("NewNestedRouter_WithOptions", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{
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
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		handler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "test result", nil
			},
		)

		route := nestedmux.AddTaskHandler(nr, "/test", handler)

		if route.OriginalPattern() != "/test" {
			t.Errorf(
				"Expected pattern '/test', got %q",
				route.OriginalPattern(),
			)
		}
		if !nr.IsRegistered("/test") {
			t.Error("Route should be registered")
		}

		allRoutes := nr.AllRoutes()
		if len(allRoutes) != 1 {
			t.Errorf("Expected 1 route, got %d", len(allRoutes))
		}
	})

	t.Run("AddNestedPatternWithoutHandler", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		nestedmux.AddPatternWithoutHandler(nr, "/static")

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

		nr := nestedmux.NewRouter(&nestedmux.Options{})

		nestedmux.AddPatternWithoutHandler(nr, "/test")
		nestedmux.AddPatternWithoutHandler(nr, "/test") // Should panic
	})

	t.Run("AllRoutes_ReturnsDefensiveCopy", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})
		nestedmux.AddPatternWithoutHandler(nr, "/immutable")

		allRoutes := nr.AllRoutes()
		delete(allRoutes, "/immutable")
		allRoutes["/injected"] = nil

		if !nr.IsRegistered("/immutable") {
			t.Fatal(
				"mutating AllRoutes() result should not remove registered routes",
			)
		}
		if nr.IsRegistered("/injected") {
			t.Fatal(
				"mutating AllRoutes() result should not inject routes into router state",
			)
		}
	})

	t.Run("GetMatcher_ReturnsDefensiveCopy", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})
		nestedmux.AddPatternWithoutHandler(nr, "/registered")

		matcherCopy := nr.Matcher()
		matcherCopy.RegisterPattern("/injected")

		injectedRequest := createRequestWithGetTasksCtx(
			http.MethodGet,
			"/injected",
		)
		_, injectedFound := nestedmux.FindMatches(nr, injectedRequest)
		if injectedFound {
			t.Fatal(
				"mutating Matcher() result should not affect router matching state",
			)
		}

		registeredRequest := createRequestWithGetTasksCtx(
			http.MethodGet,
			"/registered",
		)
		_, registeredFound := nestedmux.FindMatches(nr, registeredRequest)
		if !registeredFound {
			t.Fatal(
				"registered route should still match after mutating matcher copy",
			)
		}
	})

	t.Run(
		"AddPatternWithoutHandlerIfMissing_IsIdempotent",
		func(t *testing.T) {
			nr := nestedmux.NewRouter(&nestedmux.Options{})

			if registered := nr.AddPatternWithoutHandlerIfMissing("/safe"); !registered {
				t.Fatal("expected first registration to report registered=true")
			}
			if registered := nr.AddPatternWithoutHandlerIfMissing("/safe"); registered {
				t.Fatal(
					"expected duplicate registration to report registered=false",
				)
			}
			if !nr.IsRegistered("/safe") {
				t.Fatal("expected pattern to remain registered")
			}
		},
	)

	t.Run(
		"AddPatternWithoutHandlerIfMissing_IsConcurrentSafe",
		func(t *testing.T) {
			nr := nestedmux.NewRouter(&nestedmux.Options{})
			startGate := make(chan struct{})
			const goroutineCount = 16
			var completedCount atomic.Int32
			var successfulRegistrationCount atomic.Int32

			done := make(chan struct{})
			for i := 0; i < goroutineCount; i++ {
				go func() {
					<-startGate
					if nr.AddPatternWithoutHandlerIfMissing(
						"/concurrent-safe",
					) {
						successfulRegistrationCount.Add(1)
					}
					if completedCount.Add(1) == goroutineCount {
						close(done)
					}
				}()
			}

			close(startGate)
			<-done

			if successfulRegistrationCount.Load() != 1 {
				t.Fatalf(
					"expected exactly one successful concurrent registration, got %d",
					successfulRegistrationCount.Load(),
				)
			}
			if !nr.IsRegistered("/concurrent-safe") {
				t.Fatal("expected concurrently registered pattern to exist")
			}
		},
	)
}

func TestFindNestedMatches(t *testing.T) {
	t.Run("Single_Match", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		nestedmux.AddPatternWithoutHandler(nr, "/users")

		req := createRequestWithGetTasksCtx(http.MethodGet, "/users")
		results, found := nestedmux.FindMatches(nr, req)

		if !found {
			t.Error("Should find matches")
		}
		if len(results.Matches) != 1 {
			t.Errorf("Expected 1 match, got %d", len(results.Matches))
		}
		if results.Matches[0].OriginalPattern() != "/users" {
			t.Errorf(
				"Expected pattern '/users', got %q",
				results.Matches[0].OriginalPattern(),
			)
		}
	})

	t.Run("Nested_Matches", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		// Register nested patterns like a UI router would have
		nestedmux.AddPatternWithoutHandler(
			nr,
			"",
		) // empty because we have no explicit index
		nestedmux.AddPatternWithoutHandler(nr, "/users")
		nestedmux.AddPatternWithoutHandler(nr, "/users/:id")

		req := createRequestWithGetTasksCtx(http.MethodGet, "/users/123")
		results, found := nestedmux.FindMatches(nr, req)

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
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		nestedmux.AddPatternWithoutHandler(nr, "/users")

		req := createRequestWithGetTasksCtx(http.MethodGet, "/posts")
		_, found := nestedmux.FindMatches(nr, req)

		if found {
			t.Error("Should not find matches for unregistered path")
		}
	})

	t.Run("Splat_Pattern", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		nestedmux.AddPatternWithoutHandler(nr, "/files/*")

		req := createRequestWithGetTasksCtx(
			http.MethodGet,
			"/files/docs/readme.txt",
		)
		results, found := nestedmux.FindMatches(nr, req)

		if !found {
			t.Error("Should find matches")
		}
		if len(results.SplatValues) != 2 {
			t.Errorf(
				"Expected 2 splat values, got %d",
				len(results.SplatValues),
			)
		}
		expectedSplat := []string{"docs", "readme.txt"}
		if !sliceEqual(results.SplatValues, expectedSplat) {
			t.Errorf(
				"Expected splat values %v, got %v",
				expectedSplat,
				results.SplatValues,
			)
		}
	})

	t.Run("IsSafeDuringConcurrentRouteRegistration", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})
		nestedmux.AddPatternWithoutHandler(nr, "/seed")

		const totalRoutes = 512
		start := make(chan struct{})
		done := make(chan struct{})

		go func() {
			<-start
			for i := range totalRoutes {
				nr.AddPatternWithoutHandlerIfMissing(
					fmt.Sprintf("/concurrent/%d", i),
				)
			}
			close(done)
		}()

		close(start)
		for i := 0; i < totalRoutes; i++ {
			req := createRequestWithGetTasksCtx(
				http.MethodGet,
				fmt.Sprintf("/concurrent/%d", i),
			)
			nestedmux.FindMatches(nr, req)
		}
		<-done

		finalRequest := createRequestWithGetTasksCtx(
			http.MethodGet,
			fmt.Sprintf("/concurrent/%d", totalRoutes-1),
		)
		_, found := nestedmux.FindMatches(nr, finalRequest)
		if !found {
			t.Fatal("expected concurrently registered route to be matchable")
		}
	})
}

func TestRunNestedTasks(t *testing.T) {
	t.Run("Run_Multiple_Tasks", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		// Register handlers that return different data
		layoutHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
				return map[string]string{"layout": "main"}, nil
			},
		)
		pageHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
				return map[string]string{"page": "users"}, nil
			},
		)
		userHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
				return map[string]string{"user": rd.Params()["id"]}, nil
			},
		)

		nestedmux.AddTaskHandler(
			nr,
			"",
			layoutHandler,
		) // empty because we have no explicit index
		nestedmux.AddTaskHandler(nr, "/users", pageHandler)
		nestedmux.AddTaskHandler(nr, "/users/:id", userHandler)

		req := createRequestWithGetTasksCtx(http.MethodGet, "/users/456")

		results, found := nestedmux.FindMatchesAndRunTasks(nr, req)

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
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		handler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "with handler", nil
			},
		)

		nestedmux.AddPatternWithoutHandler(nr, "/static")
		nestedmux.AddTaskHandler(nr, "/dynamic", handler)

		// Should match both patterns since /dynamic matches both /static and /dynamic patterns
		// Wait, actually looking at the matcher, it would only match /dynamic
		// Let me adjust the test to have patterns that would both match

		// Reset and use patterns that would both match a single request
		nr = nestedmux.NewRouter(&nestedmux.Options{})
		nestedmux.AddPatternWithoutHandler(
			nr,
			"",
		) // empty because we have no explicit index
		nestedmux.AddTaskHandler(nr, "/page", handler)

		req := createRequestWithGetTasksCtx(http.MethodGet, "/page")
		results, found := nestedmux.FindMatchesAndRunTasks(nr, req)

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
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		errorHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "", &testError{msg: "task failed"}
			},
		)

		nestedmux.AddTaskHandler(nr, "/error", errorHandler)

		req := createRequestWithGetTasksCtx(http.MethodGet, "/error")

		results, found := nestedmux.FindMatchesAndRunTasks(nr, req)

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
			t.Errorf(
				"Expected 'task failed', got %q",
				errorResult.Err().Error(),
			)
		}
	})

	t.Run("Task_Error_Does_Not_Cancel_Siblings", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		var outerRan atomic.Bool
		outerHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				outerRan.Store(true)
				// Give the sibling error path time to complete first.
				time.Sleep(25 * time.Millisecond)
				return "outer-ok", nil
			},
		)
		innerHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "", errors.New("inner failed")
			},
		)

		nestedmux.AddTaskHandler(nr, "/items", outerHandler)
		nestedmux.AddTaskHandler(nr, "/items/:id", innerHandler)

		req := createRequestWithGetTasksCtx(http.MethodGet, "/items/123")
		results, found := nestedmux.FindMatchesAndRunTasks(nr, req)

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
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		var childStarted atomic.Bool
		parentHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "", errors.New("parent failed")
			},
		)
		childHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				childStarted.Store(true)
				select {
				case <-rd.TasksCtx().NativeContext().Done():
					return "", rd.TasksCtx().NativeContext().Err()
				case <-time.After(150 * time.Millisecond):
					return "child-finished", nil
				}
			},
		)

		nestedmux.AddTaskHandler(nr, "/items", parentHandler)
		nestedmux.AddTaskHandler(nr, "/items/:id", childHandler)

		req := createRequestWithGetTasksCtx(http.MethodGet, "/items/123")
		results, found := nestedmux.FindMatchesAndRunTasks(nr, req)
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
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		sleep := 60 * time.Millisecond
		parentHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				time.Sleep(sleep)
				return "parent-ok", nil
			},
		)
		childHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				time.Sleep(sleep)
				return "child-ok", nil
			},
		)

		nestedmux.AddTaskHandler(nr, "/parallel", parentHandler)
		nestedmux.AddTaskHandler(nr, "/parallel/:id", childHandler)

		req := createRequestWithGetTasksCtx(http.MethodGet, "/parallel/123")
		start := time.Now()
		results, found := nestedmux.FindMatchesAndRunTasks(nr, req)
		elapsed := time.Since(start)
		if !found {
			t.Fatal("should find matches")
		}
		if results.Map["/parallel"].Err() != nil ||
			results.Map["/parallel/:id"].Err() != nil {
			t.Fatalf(
				"expected both tasks to succeed, got parent=%v child=%v",
				results.Map["/parallel"].Err(),
				results.Map["/parallel/:id"].Err(),
			)
		}

		// Sequential would be around 2*sleep; parallel should be close to sleep.
		if elapsed >= (sleep + 40*time.Millisecond) {
			t.Fatalf(
				"expected parallel execution, elapsed=%v (sleep=%v)",
				elapsed,
				sleep,
			)
		}
	})

	t.Run("Middle_Error_Cancels_Only_Descendants", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		parentHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				time.Sleep(25 * time.Millisecond)
				return "parent-ok", nil
			},
		)
		middleHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "", errors.New("middle failed")
			},
		)
		leafHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				select {
				case <-rd.TasksCtx().NativeContext().Done():
					return "", rd.TasksCtx().NativeContext().Err()
				case <-time.After(150 * time.Millisecond):
					return "leaf-ok", nil
				}
			},
		)

		nestedmux.AddTaskHandler(nr, "/items", parentHandler)
		nestedmux.AddTaskHandler(nr, "/items/:id", middleHandler)
		nestedmux.AddTaskHandler(nr, "/items/:id/details", leafHandler)

		req := createRequestWithGetTasksCtx(
			http.MethodGet,
			"/items/123/details",
		)
		results, found := nestedmux.FindMatchesAndRunTasks(nr, req)
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

	t.Run("Request_Cancellation_Cancels_All_Matched_Tasks", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		waitForContextCancellation := func(
			rd *mux.ReqData[mux.None],
		) (string, error) {
			select {
			case <-rd.TasksCtx().NativeContext().Done():
				return "", rd.TasksCtx().NativeContext().Err()
			case <-time.After(250 * time.Millisecond):
				return "unexpected-success", nil
			}
		}

		parentHandler := mux.TaskHandlerFromFunc(waitForContextCancellation)
		childHandler := mux.TaskHandlerFromFunc(waitForContextCancellation)

		nestedmux.AddTaskHandler(nr, "/items", parentHandler)
		nestedmux.AddTaskHandler(nr, "/items/:id", childHandler)

		nativeRequestCtx, cancelRequest := context.WithCancel(
			context.Background(),
		)
		t.Cleanup(cancelRequest)

		req := httptest.NewRequest(
			http.MethodGet,
			"/items/123",
			nil,
		).WithContext(nativeRequestCtx)
		req = mux.RequestWithTasksCtx(req, tasks.NewCtx(req.Context()))

		go func() {
			time.Sleep(20 * time.Millisecond)
			cancelRequest()
		}()

		results, found := nestedmux.FindMatchesAndRunTasks(nr, req)
		if !found {
			t.Fatal("should find matches")
		}

		parent := results.Map["/items"]
		child := results.Map["/items/:id"]
		if parent == nil || child == nil {
			t.Fatal("missing parent or child result")
		}
		if !errors.Is(parent.Err(), context.Canceled) {
			t.Fatalf("parent err = %v, want context.Canceled", parent.Err())
		}
		if !errors.Is(child.Err(), context.Canceled) {
			t.Fatalf("child err = %v, want context.Canceled", child.Err())
		}
	})

	t.Run(
		"SharedTaskDependencyRunsOnceAcrossNestedHandlers",
		func(t *testing.T) {
			nr := nestedmux.NewRouter(&nestedmux.Options{})

			var sharedRuns atomic.Int32
			sharedTask := tasks.NewTask(
				func(c *tasks.Ctx, _ mux.None) (string, error) {
					sharedRuns.Add(1)
					time.Sleep(20 * time.Millisecond)
					return "shared-result", nil
				},
			)

			parentHandler := mux.TaskHandlerFromFunc(
				func(rd *mux.ReqData[mux.None]) (string, error) {
					return sharedTask.Run(rd.TasksCtx(), mux.None{})
				},
			)
			childHandler := mux.TaskHandlerFromFunc(
				func(rd *mux.ReqData[mux.None]) (string, error) {
					return sharedTask.Run(rd.TasksCtx(), mux.None{})
				},
			)

			nestedmux.AddTaskHandler(nr, "/items", parentHandler)
			nestedmux.AddTaskHandler(nr, "/items/:id", childHandler)

			req := createRequestWithGetTasksCtx(http.MethodGet, "/items/123")
			results, found := nestedmux.FindMatchesAndRunTasks(nr, req)
			if !found {
				t.Fatal("should find matches")
			}

			parent := results.Map["/items"]
			child := results.Map["/items/:id"]
			if parent == nil || child == nil {
				t.Fatal("missing nested results")
			}
			if parent.Err() != nil || child.Err() != nil {
				t.Fatalf(
					"expected both handlers to succeed, got parent=%v child=%v",
					parent.Err(),
					child.Err(),
				)
			}
			if parent.Data() != "shared-result" ||
				child.Data() != "shared-result" {
				t.Fatalf(
					"unexpected shared data: parent=%#v child=%#v",
					parent.Data(),
					child.Data(),
				)
			}
			if sharedRuns.Load() != 1 {
				t.Fatalf(
					"shared dependency runs = %d, want 1",
					sharedRuns.Load(),
				)
			}
		},
	)

	t.Run("HasTaskHandlerAt", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		handler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "test", nil
			},
		)

		nestedmux.AddPatternWithoutHandler(nr, "/no-handler")
		nestedmux.AddTaskHandler(nr, "/with-handler", handler)

		req := createRequestWithGetTasksCtx(http.MethodGet, "/with-handler")
		matches, _ := nestedmux.FindMatches(nr, req)

		results := nestedmux.RunTasks(nr, req, matches)

		// The results should track which indices had task handlers
		// Based on the order of matches, we need to check the right indices
		for i, result := range results.Slice {
			hasHandler := results.HasTaskHandlerAt(i)
			if result.RanTask() && !hasHandler {
				t.Errorf(
					"Index %d ran task but HasTaskHandlerAt returned false",
					i,
				)
			}
			if !result.RanTask() && hasHandler {
				t.Errorf(
					"Index %d didn't run task but HasTaskHandlerAt returned true",
					i,
				)
			}
		}
	})
}

func TestNestedRouterWithExplicitIndex(t *testing.T) {
	t.Run("Explicit_Index_Segment", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{
			ExplicitIndexSegmentIdentifier: "_index",
		})

		handler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "index page", nil
			},
		)

		// With explicit index, you'd register like this instead of trailing slash
		nestedmux.AddTaskHandler(nr, "/users/_index", handler)

		// This would match /users/ or /users
		req := createRequestWithGetTasksCtx(http.MethodGet, "/users/")
		results, found := nestedmux.FindMatchesAndRunTasks(nr, req)

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
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		oldHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "old", nil
			},
		)
		newHandler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				return "new", nil
			},
		)

		nestedmux.AddTaskHandler(nr, "/old", oldHandler)
		nestedmux.AddPatternWithoutHandler(nr, "/legacy")

		replacementRouter := nestedmux.NewRouter(
			&nestedmux.Options{},
		)
		nestedmux.AddTaskHandler(replacementRouter, "/fresh", newHandler)
		nestedmux.AddPatternWithoutHandler(replacementRouter, "/static")
		nr.ReplaceRoutes(replacementRouter.AllRoutes())

		if nr.IsRegistered("/old") {
			t.Fatal("expected /old to be removed after ReplaceRoutes")
		}
		if nr.IsRegistered("/legacy") {
			t.Fatal("expected /legacy to be removed after ReplaceRoutes")
		}
		if !nr.IsRegistered("/fresh") || !nr.IsRegistered("/static") {
			t.Fatal("expected replacement routes to be registered")
		}

		reqOld := createRequestWithGetTasksCtx(http.MethodGet, "/old")
		if _, found := nestedmux.FindMatchesAndRunTasks(nr, reqOld); found {
			t.Fatal("expected /old not to match after ReplaceRoutes")
		}

		reqFresh := createRequestWithGetTasksCtx(http.MethodGet, "/fresh")
		results, found := nestedmux.FindMatchesAndRunTasks(nr, reqFresh)
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

	t.Run("ReplaceRoutes_DefensivelyCopiesInputMap", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		replacementRouter := nestedmux.NewRouter(
			&nestedmux.Options{},
		)
		nestedmux.AddPatternWithoutHandler(replacementRouter, "/initial")
		replacementRoutes := replacementRouter.AllRoutes()
		nr.ReplaceRoutes(replacementRoutes)

		delete(replacementRoutes, "/initial")
		replacementRoutes["/injected"] = nil

		if !nr.IsRegistered("/initial") {
			t.Fatal(
				"mutating ReplaceRoutes input map should not remove installed route",
			)
		}
		if nr.IsRegistered("/injected") {
			t.Fatal("mutating ReplaceRoutes input map should not inject route")
		}

		initialRequest := createRequestWithGetTasksCtx(
			http.MethodGet,
			"/initial",
		)
		_, initialFound := nestedmux.FindMatchesAndRunTasks(
			nr,
			initialRequest,
		)
		if !initialFound {
			t.Fatal(
				"expected /initial to remain matchable after caller map mutation",
			)
		}

		injectedRequest := createRequestWithGetTasksCtx(
			http.MethodGet,
			"/injected",
		)
		_, injectedFound := nestedmux.FindMatchesAndRunTasks(
			nr,
			injectedRequest,
		)
		if injectedFound {
			t.Fatal("expected /injected not to match after caller map mutation")
		}
	})

	t.Run(
		"RebuildPreservingHandlers_PreservesOnlyHandlerRoutes",
		func(t *testing.T) {
			nr := nestedmux.NewRouter(&nestedmux.Options{})

			keepHandler := mux.TaskHandlerFromFunc(
				func(rd *mux.ReqData[mux.None]) (string, error) {
					return "keep", nil
				},
			)

			nestedmux.AddTaskHandler(nr, "/keep", keepHandler)
			nestedmux.AddPatternWithoutHandler(nr, "/drop-no-handler")
			nestedmux.AddPatternWithoutHandler(nr, "/will-be-replaced")

			nr.RebuildPreservingHandlers([]string{"/keep", "/new-no-handler"})

			if !nr.IsRegistered("/keep") {
				t.Fatal("expected /keep to remain registered")
			}
			if !nr.IsRegistered("/new-no-handler") {
				t.Fatal("expected /new-no-handler to be registered")
			}
			if nr.IsRegistered("/drop-no-handler") ||
				nr.IsRegistered("/will-be-replaced") {
				t.Fatal("expected old no-handler routes to be removed")
			}

			reqKeep := createRequestWithGetTasksCtx(http.MethodGet, "/keep")
			keepResults, found := nestedmux.FindMatchesAndRunTasks(
				nr,
				reqKeep,
			)
			if !found {
				t.Fatal("expected /keep to match after rebuild")
			}
			keep := keepResults.Map["/keep"]
			if keep == nil || keep.Err() != nil || keep.Data() != "keep" {
				t.Fatalf(
					"/keep result unexpected: data=%#v err=%v",
					keep.Data(),
					keep.Err(),
				)
			}

			reqNew := createRequestWithGetTasksCtx(
				http.MethodGet,
				"/new-no-handler",
			)
			newResults, found := nestedmux.FindMatchesAndRunTasks(
				nr,
				reqNew,
			)
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
		},
	)

	t.Run(
		"RunNestedTasks_RemainsPatternConsistentDuringConcurrentReplaceRoutes",
		func(t *testing.T) {
			nr := nestedmux.NewRouter(&nestedmux.Options{})

			buildRouteMap := func() map[string]nestedmux.AnyRoute {
				buildHandler := func(pattern string) *mux.TaskHandler[mux.None, string] {
					return mux.TaskHandlerFromFunc(
						func(*mux.ReqData[mux.None]) (string, error) {
							return pattern, nil
						},
					)
				}
				replacementRouter := nestedmux.NewRouter(
					&nestedmux.Options{},
				)
				nestedmux.AddTaskHandler(
					replacementRouter,
					"",
					buildHandler(""),
				)
				nestedmux.AddTaskHandler(
					replacementRouter,
					"/items",
					buildHandler("/items"),
				)
				nestedmux.AddTaskHandler(
					replacementRouter,
					"/items/:id",
					buildHandler("/items/:id"),
				)
				return replacementRouter.AllRoutes()
			}

			nr.ReplaceRoutes(buildRouteMap())

			stop := make(chan struct{})
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					select {
					case <-stop:
						return
					default:
						nr.ReplaceRoutes(buildRouteMap())
					}
				}
			}()

			expectedPatterns := []string{"", "/items", "/items/:id"}
			for iteration := 0; iteration < 500; iteration++ {
				request := createRequestWithGetTasksCtx(
					http.MethodGet,
					"/items/123",
				)
				results, found := nestedmux.FindMatchesAndRunTasks(
					nr,
					request,
				)
				if !found {
					t.Fatal(
						"expected /items/123 to match during route replacement",
					)
				}
				for _, expectedPattern := range expectedPatterns {
					currentResult := results.Map[expectedPattern]
					if currentResult == nil {
						t.Fatalf(
							"missing result for pattern %q",
							expectedPattern,
						)
					}
					if err := currentResult.Err(); err != nil {
						t.Fatalf(
							"pattern %q returned error: %v",
							expectedPattern,
							err,
						)
					}
					currentData, dataOk := currentResult.Data().(string)
					if !dataOk {
						t.Fatalf(
							"pattern %q data type = %T, want string",
							expectedPattern,
							currentResult.Data(),
						)
					}
					if currentData != expectedPattern {
						t.Fatalf(
							"pattern %q returned data %q",
							expectedPattern,
							currentData,
						)
					}
				}
			}

			close(stop)
			<-done
		},
	)
}

func TestResponseProxies(t *testing.T) {
	t.Run("Response_Proxies_OnlyForMatchedTaskHandlers", func(t *testing.T) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		handler1 := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				rd.ResponseProxy().SetHeader("X-Handler-1", "value1")
				return "handler1", nil
			},
		)
		handler2 := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (string, error) {
				rd.ResponseProxy().SetHeader("X-Handler-2", "value2")
				return "handler2", nil
			},
		)

		nestedmux.AddTaskHandler(nr, "/", handler1)
		nestedmux.AddPatternWithoutHandler(nr, "/page")
		nestedmux.AddTaskHandler(nr, "/page/details", handler2)

		req := createRequestWithGetTasksCtx(http.MethodGet, "/page/details")

		results, _ := nestedmux.FindMatchesAndRunTasks(nr, req)

		// Verify proxy slots align with results.
		if len(results.ResponseProxies) != len(results.Slice) {
			t.Errorf("Expected %d response proxies, got %d",
				len(results.Slice), len(results.ResponseProxies))
		}

		// Verify only matched task-handler routes have proxies.
		for i, proxy := range results.ResponseProxies {
			if results.Slice[i].RanTask() {
				if proxy == nil {
					t.Errorf(
						"response proxy at index %d should exist for task handler",
						i,
					)
				}
				continue
			}
			if proxy != nil {
				t.Errorf(
					"response proxy at index %d should be nil for no-handler route",
					i,
				)
			}
		}
	})
}

func TestRunTasksWithoutPatternMap(t *testing.T) {
	nr := nestedmux.NewRouter(&nestedmux.Options{})

	leafHandler := mux.TaskHandlerFromFunc(
		func(rd *mux.ReqData[mux.None]) (string, error) {
			return "leaf-ok", nil
		},
	)

	nestedmux.AddPatternWithoutHandler(nr, "/docs")
	nestedmux.AddTaskHandler(nr, "/docs/:slug", leafHandler)

	req := createRequestWithGetTasksCtx(http.MethodGet, "/docs/runtime")
	matches, found := nestedmux.FindMatches(nr, req)
	if !found {
		t.Fatal("expected nested matches")
	}

	results := nestedmux.RunTasksWithoutPatternMap(nr, req, matches)
	if results == nil {
		t.Fatal("expected non-nil tasks results")
	}
	if results.Map != nil {
		t.Fatalf(
			"expected nil pattern map in no-map mode, got %#v",
			results.Map,
		)
	}
	if len(results.Slice) < 2 {
		t.Fatalf("slice length = %d, want at least 2", len(results.Slice))
	}

	var sawLeafResult bool
	var sawNoTaskRoute bool
	for _, routeResult := range results.Slice {
		if !routeResult.RanTask() {
			sawNoTaskRoute = true
			continue
		}
		switch routeResult.Data() {
		case "leaf-ok":
			sawLeafResult = true
		}
	}
	if !sawLeafResult {
		t.Fatal("expected leaf task result in no-map mode")
	}
	if !sawNoTaskRoute {
		t.Fatal("expected no-handler route result in no-map mode")
	}
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
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		nestedmux.AddPatternWithoutHandler(nr, "/")
		nestedmux.AddPatternWithoutHandler(nr, "/users")
		nestedmux.AddPatternWithoutHandler(nr, "/users/:id")

		req := createRequestWithGetTasksCtx(http.MethodGet, "/users/123")

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			nestedmux.FindMatches(nr, req)
		}
	})

	b.Run("Nested_Tasks_Execution", func(b *testing.B) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

		handler := mux.TaskHandlerFromFunc(
			func(rd *mux.ReqData[mux.None]) (map[string]string, error) {
				return map[string]string{"id": rd.Params()["id"]}, nil
			},
		)

		nestedmux.AddTaskHandler(nr, "/", handler)
		nestedmux.AddTaskHandler(nr, "/users", handler)
		nestedmux.AddTaskHandler(nr, "/users/:id", handler)

		req := createRequestWithGetTasksCtx(http.MethodGet, "/users/123")

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			nestedmux.FindMatchesAndRunTasks(nr, req)
		}
	})

	b.Run("Deep_Nesting", func(b *testing.B) {
		nr := nestedmux.NewRouter(&nestedmux.Options{})

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
			nestedmux.AddPatternWithoutHandler(nr, pattern)
		}

		req := createRequestWithGetTasksCtx(
			http.MethodGet,
			"/app/dashboard/users/123/profile/settings",
		)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			nestedmux.FindMatches(nr, req)
		}
	})
}

func createRequestWithGetTasksCtx(method, url string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	tasksCtx := tasks.NewCtx(req.Context())
	return mux.RequestWithTasksCtx(req, tasksCtx)
}

func sliceEqual(actual []string, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range actual {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}
