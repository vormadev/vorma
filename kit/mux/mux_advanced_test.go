package mux

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestTaskMiddleware_Interactions(t *testing.T) {
	t.Run("ErrorFromTaskMiddlewareReturns500", func(t *testing.T) {
		r := NewRouter(nil)
		var taskMwRan bool
		var mainHandlerRan bool

		taskMw := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			taskMwRan = true
			rd.ResponseProxy().SetStatus(http.StatusForbidden, "Forbidden by Task MW")
			return None{}, errors.New("task middleware intentional error") // Task also returns an error
		})
		AddGlobalTaskMiddleware(r, taskMw)

		AddHTTPHandlerFunc(r, http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
			mainHandlerRan = true
			t.Error("Main handler should not be called if task middleware errors")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !taskMwRan {
			t.Error("Task middleware should have run")
		}
		if mainHandlerRan {
			t.Error("Main handler ran but should have been short-circuited by task middleware error")
		}
		// When middleware returns an error, we get 500 regardless of proxy status
		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 when middleware returns error, got %d", w.Code)
		}
		// The body should be the generic 500 error, not the custom message
		if !strings.Contains(w.Body.String(), "Internal Server Error") {
			t.Errorf("Expected body to contain 'Internal Server Error', got %q", w.Body.String())
		}
	})

	t.Run("TaskMiddlewareSetsClientErrorAndHalts", func(t *testing.T) {
		r := NewRouter(nil)
		taskMw := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			rd.ResponseProxy().SetStatus(http.StatusTeapot) // 418 is an error (>=400)
			rd.ResponseProxy().SetHeader("X-Tea-Type", "Earl Grey")
			return None{}, nil // Task itself doesn't return an error, but proxy is set to an error status
		})
		AddGlobalTaskMiddleware(r, taskMw)

		mainHandlerRan := false
		AddHTTPHandlerFunc(r, http.MethodGet, "/tea", func(w http.ResponseWriter, r *http.Request) {
			mainHandlerRan = true
		})

		req := httptest.NewRequest(http.MethodGet, "/tea", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Main handler SHOULD NOT run if task middleware sets a 4xx status
		if mainHandlerRan {
			t.Error("Main handler ran but should have been short-circuited by task middleware 418 error status")
		}
		if w.Code != http.StatusTeapot {
			t.Errorf("Expected status %d, got %d", http.StatusTeapot, w.Code)
		}
		if w.Header().Get("X-Tea-Type") != "Earl Grey" {
			t.Errorf("Expected header 'X-Tea-Type: Earl Grey', got %q", w.Header().Get("X-Tea-Type"))
		}
	})

	t.Run("MultipleTaskMiddlewaresMergeProxiesAndCanHalt", func(t *testing.T) {
		r := NewRouter(nil)
		var mw1Ran, mw2Ran, mw3Ran bool
		var mainHandlerRan bool
		var wg sync.WaitGroup
		wg.Add(3) // For the three task middlewares

		tmw1 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			defer wg.Done()
			mw1Ran = true
			rd.ResponseProxy().AddHeader("X-Multi-Trace", "MW1")
			rd.ResponseProxy().SetStatus(http.StatusAccepted)
			return None{}, nil
		})
		// This middleware will cause the halt
		tmw2 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			defer wg.Done()
			mw2Ran = true
			rd.ResponseProxy().AddHeader("X-Multi-Trace", "MW2")
			rd.ResponseProxy().SetStatus(http.StatusConflict) // 409 is an error (>=400)
			return None{}, nil
		})
		tmw3 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			defer wg.Done()
			mw3Ran = true
			rd.ResponseProxy().AddHeader("X-Multi-Trace", "MW3")
			return None{}, nil
		})

		AddGlobalTaskMiddleware(r, tmw1)
		AddMethodLevelTaskMiddleware(r, http.MethodGet, tmw2)
		route := AddHTTPHandlerFunc(r, http.MethodGet, "/multi", func(w http.ResponseWriter, r *http.Request) {
			mainHandlerRan = true
		})
		AddPatternLevelTaskMiddleware(route, tmw3)

		req := httptest.NewRequest(http.MethodGet, "/multi", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		wg.Wait()

		if !(mw1Ran && mw2Ran && mw3Ran) {
			t.Errorf("Expected all task middlewares to run, got: mw1=%v, mw2=%v, mw3=%v", mw1Ran, mw2Ran, mw3Ran)
		}
		if mainHandlerRan {
			t.Error("Main handler ran but should have been short-circuited by task middleware 409 error status")
		}
		if w.Code != http.StatusConflict {
			t.Errorf("Expected status %d, got %d", http.StatusConflict, w.Code)
		}
		traces := w.Header().Values("X-Multi-Trace")
		// Order of AddHeader can be non-deterministic for parallel tasks if underlying map write is not guarded
		// For testing, we just check presence and count. response.MergeProxyResponses handles combining them.
		if len(traces) != 3 {
			t.Errorf("Expected 3 X-Multi-Trace headers, got %d: %v", len(traces), traces)
		}
	})

	t.Run(
		"TaskMiddlewareChainCacheInvalidatesWhenNewMiddlewaresAreRegistered",
		func(t *testing.T) {
			r := NewRouter(nil)
			route := AddTaskHandler(
				r,
				http.MethodGet,
				"/cache-chain",
				TaskHandlerFromFunc(func(rd *ReqData[None]) (map[string]string, error) {
					return map[string]string{"ok": "true"}, nil
				}),
			)

			AddGlobalTaskMiddleware(
				r,
				TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
					rd.ResponseProxy().AddHeader("X-Task-MW", "global-1")
					return None{}, nil
				}),
			)

			firstReq := httptest.NewRequest(http.MethodGet, "/cache-chain", nil)
			firstRes := httptest.NewRecorder()
			r.ServeHTTP(firstRes, firstReq)
			if firstRes.Code != http.StatusOK {
				t.Fatalf("first request status = %d, want %d", firstRes.Code, http.StatusOK)
			}
			if got := firstRes.Header().Values("X-Task-MW"); len(got) != 1 || got[0] != "global-1" {
				t.Fatalf(
					"first request X-Task-MW = %v, want [global-1]",
					got,
				)
			}

			AddGlobalTaskMiddleware(
				r,
				TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
					rd.ResponseProxy().AddHeader("X-Task-MW", "global-2")
					return None{}, nil
				}),
			)
			AddMethodLevelTaskMiddleware(
				r,
				http.MethodGet,
				TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
					rd.ResponseProxy().AddHeader("X-Task-MW", "method")
					return None{}, nil
				}),
			)
			AddPatternLevelTaskMiddleware(
				route,
				TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
					rd.ResponseProxy().AddHeader("X-Task-MW", "pattern")
					return None{}, nil
				}),
			)

			secondReq := httptest.NewRequest(http.MethodGet, "/cache-chain", nil)
			secondRes := httptest.NewRecorder()
			r.ServeHTTP(secondRes, secondReq)
			if secondRes.Code != http.StatusOK {
				t.Fatalf("second request status = %d, want %d", secondRes.Code, http.StatusOK)
			}

			gotValues := secondRes.Header().Values("X-Task-MW")
			if len(gotValues) != 4 {
				t.Fatalf("second request X-Task-MW len = %d, want 4 (%v)", len(gotValues), gotValues)
			}

			gotSet := map[string]bool{}
			for _, value := range gotValues {
				gotSet[value] = true
			}
			for _, wantValue := range []string{"global-1", "global-2", "method", "pattern"} {
				if !gotSet[wantValue] {
					t.Fatalf(
						"second request X-Task-MW missing value %q (got %v)",
						wantValue,
						gotValues,
					)
				}
			}
		},
	)
}

// --- TestComplexMiddlewareScenarios ---
// This test should still pass as is, because no task middleware sets an error/redirect status.
func TestComplexMiddlewareScenarios(t *testing.T) {
	t.Run("MixedStackOrderAndExecution", func(t *testing.T) {
		r := NewRouter(nil)
		var executionOrder []string
		var mu sync.Mutex

		appendOrder := func(id string) {
			mu.Lock()
			defer mu.Unlock()
			executionOrder = append(executionOrder, id)
		}

		AddGlobalHTTPMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				appendOrder("GlobalHTTP-Pre")
				next.ServeHTTP(w, r)
				appendOrder("GlobalHTTP-Post")
			})
		})
		AddMethodLevelHTTPMiddleware(r, http.MethodGet, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				appendOrder("MethodHTTP-Pre")
				next.ServeHTTP(w, r)
				appendOrder("MethodHTTP-Post")
			})
		})

		var globalTaskDone, methodTaskDone, patternTaskDone bool
		var wg sync.WaitGroup
		wg.Add(3)

		AddGlobalTaskMiddleware(r, TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			defer wg.Done()
			appendOrder("GlobalTask")
			rd.ResponseProxy().SetHeader("X-Global-Task", "Done")
			globalTaskDone = true
			return None{}, nil
		}))
		AddMethodLevelTaskMiddleware(r, http.MethodGet, TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			defer wg.Done()
			appendOrder("MethodTask")
			rd.ResponseProxy().SetHeader("X-Method-Task", "Done")
			methodTaskDone = true
			return None{}, nil
		}))

		route := AddTaskHandler(r, http.MethodGet, "/complex", TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			appendOrder("TaskHandler")
			rd.ResponseProxy().SetHeader("X-Handler-Task", "Done")
			return "handler_done", nil
		}))
		AddPatternLevelHTTPMiddleware(route, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				appendOrder("PatternHTTP-Pre")
				next.ServeHTTP(w, r)
				appendOrder("PatternHTTP-Post")
			})
		})
		AddPatternLevelTaskMiddleware(route, TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			defer wg.Done()
			appendOrder("PatternTask")
			rd.ResponseProxy().SetHeader("X-Pattern-Task", "Done")
			patternTaskDone = true
			return None{}, nil
		}))

		req := httptest.NewRequest(http.MethodGet, "/complex", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		wg.Wait()

		expectedHTTPOrder := []string{
			"GlobalHTTP-Pre", "MethodHTTP-Pre", "PatternHTTP-Pre",
			"TaskHandler",
			"PatternHTTP-Post", "MethodHTTP-Post", "GlobalHTTP-Post",
		}
		httpOrder := []string{}
		mu.Lock()
		for _, entry := range executionOrder {
			if strings.Contains(entry, "HTTP") || strings.Contains(entry, "Handler") {
				httpOrder = append(httpOrder, entry)
			}
		}
		mu.Unlock()

		if !sliceEqual(httpOrder, expectedHTTPOrder) {
			t.Errorf("HTTP execution order incorrect.\nExpected: %v\nGot:      %v", expectedHTTPOrder, httpOrder)
		}

		if !(globalTaskDone && methodTaskDone && patternTaskDone) {
			t.Errorf("Not all task middlewares ran: G=%v, M=%v, P=%v", globalTaskDone, methodTaskDone, patternTaskDone)
		}

		if w.Header().Get("X-Global-Task") != "Done" {
			t.Error("Missing X-Global-Task header")
		}
		if w.Header().Get("X-Method-Task") != "Done" {
			t.Error("Missing X-Method-Task header")
		}
		if w.Header().Get("X-Pattern-Task") != "Done" {
			t.Error("Missing X-Pattern-Task header")
		}
		if w.Header().Get("X-Handler-Task") != "Done" {
			t.Error("Missing X-Handler-Task header")
		}
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", w.Code)
		}
	})
}

// --- TestReqDataAccess ---
// These tests should remain valid as they test fundamental data access.
func TestReqDataAccess(t *testing.T) {
	t.Run("InTaskHandler", func(t *testing.T) {
		r := NewRouter(nil)
		var (
			paramsChecked, splatChecked, tasksCtxChecked, requestChecked, proxyChecked bool
		)
		AddTaskHandler(r, http.MethodGet, "/task/:id/path/*", TaskHandlerFromFunc(func(rd *ReqData[None]) (string, error) {
			if len(rd.Params()) > 0 && rd.Params()["id"] == "123" {
				paramsChecked = true
			}
			if len(rd.SplatValues()) > 0 && rd.SplatValues()[0] == "foo" {
				splatChecked = true
			}
			if rd.TasksCtx() != nil {
				tasksCtxChecked = true
			}
			if rd.Request() != nil {
				requestChecked = true
			}
			if rd.ResponseProxy() != nil {
				proxyChecked = true
				rd.ResponseProxy().SetHeader("X-From-Task", "OK")
			}
			return "ok", nil
		}))

		req := httptest.NewRequest(http.MethodGet, "/task/123/path/foo/bar", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !paramsChecked {
			t.Error("Params not checked or incorrect")
		}
		if !splatChecked {
			t.Error("SplatValues not checked or incorrect")
		}
		if !tasksCtxChecked {
			t.Error("TasksCtx was nil")
		}
		if !requestChecked {
			t.Error("Request was nil")
		}
		if !proxyChecked {
			t.Error("ResponseProxy was nil")
		}
		if w.Header().Get("X-From-Task") != "OK" {
			t.Error("Header from task via proxy not set")
		}
		if w.Code != http.StatusOK { // Added check for OK status
			t.Errorf("Expected status OK for InTaskHandler, got %d", w.Code)
		}
	})

	t.Run("InHTTPHandler_StandardPath", func(t *testing.T) {
		r := NewRouter(nil)
		var paramsChecked, splatChecked bool
		AddGlobalTaskMiddleware(r, TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) { return None{}, nil }))

		AddHTTPHandlerFunc(r, http.MethodGet, "/http/:id/path/*", func(w http.ResponseWriter, req *http.Request) {
			params := GetParams(req)
			splats := GetSplatValues(req)
			if len(params) > 0 && params["id"] == "456" {
				paramsChecked = true
			}
			if len(splats) > 0 && splats[0] == "baz" {
				splatChecked = true
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/http/456/path/baz/qux", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if !paramsChecked {
			t.Error("Params not checked or incorrect in HTTP handler (standard path)")
		}
		if !splatChecked {
			t.Error("SplatValues not checked or incorrect in HTTP handler (standard path)")
		}
	})
}

// --- TestRoutingEdgeCases ---
// These tests should remain valid.
func TestRoutingEdgeCases(t *testing.T) {
	t.Run("StaticVsParam", func(t *testing.T) {
		r := NewRouter(nil)
		var staticCalled, paramCalled bool
		AddHTTPHandlerFunc(r, http.MethodGet, "/users/new", func(w http.ResponseWriter, r *http.Request) { staticCalled = true })
		AddHTTPHandlerFunc(r, http.MethodGet, "/users/:id", func(w http.ResponseWriter, r *http.Request) { paramCalled = true })

		req := httptest.NewRequest(http.MethodGet, "/users/new", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if !staticCalled || paramCalled {
			t.Errorf("Expected static route /users/new to be called, staticCalled=%v, paramCalled=%v", staticCalled, paramCalled)
		}

		staticCalled, paramCalled = false, false
		req = httptest.NewRequest(http.MethodGet, "/users/123", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if staticCalled || !paramCalled {
			t.Errorf("Expected param route /users/:id to be called, staticCalled=%v, paramCalled=%v", staticCalled, paramCalled)
		}
	})

	t.Run("SplatURLDecoding", func(t *testing.T) {
		r := NewRouter(nil)
		var capturedSplatValues []string

		// Use a splat pattern
		AddHTTPHandlerFunc(r, http.MethodGet, "/test/*", func(w http.ResponseWriter, req *http.Request) {
			capturedSplatValues = GetSplatValues(req)
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test/foo%20bar%2Fbaz", nil)
		// r.URL.Path becomes "/test/foo bar/baz"
		// ParseSegments of "test/foo bar/baz" yields ["test", "foo bar", "baz"]
		// The splat for "/test/*" should capture ["foo bar", "baz"]

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected status OK, got %d", rec.Code)
		}

		expectedSplat := []string{"foo bar", "baz"}
		if !sliceEqual(capturedSplatValues, expectedSplat) {
			t.Errorf("Expected splat values %v, got %v", expectedSplat, capturedSplatValues)
		}

		// To get the string "foo bar/baz", the handler/test would join:
		reconstructedValue := strings.Join(capturedSplatValues, "/")
		if reconstructedValue != "foo bar/baz" {
			t.Errorf("Expected reconstructed value to be 'foo bar/baz', got %q", reconstructedValue)
		}
	})

	t.Run("EmptySplat", func(t *testing.T) {
		r := NewRouter(nil)
		var splatValues []string
		AddHTTPHandlerFunc(r, http.MethodGet, "/files/*", func(w http.ResponseWriter, req *http.Request) {
			splatValues = GetSplatValues(req)
		})
		req := httptest.NewRequest(http.MethodGet, "/files/", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if !(len(splatValues) == 1 && splatValues[0] == "") {
			t.Errorf("Expected splat for /files/ to be [\"\"], got %v", splatValues)
		}
	})
}

// --- TestServeHTTP_ErrorHandling ---
func TestServeHTTP_ErrorHandling(t *testing.T) {
	t.Run("PanicRecoveryMiddleware", func(t *testing.T) {
		r := NewRouter(nil)
		AddGlobalHTTPMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func() {
					if err := recover(); err != nil {
						fmt.Println("Recovered from panic:", err)
						http.Error(w, "Recovered Internal Server Error", http.StatusInternalServerError)
					}
				}()
				next.ServeHTTP(w, r)
			})
		})
		AddHTTPHandlerFunc(r, http.MethodGet, "/panic", func(w http.ResponseWriter, r *http.Request) {
			panic("intentional panic in handler")
		})
		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 after panic, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Recovered Internal Server Error") {
			t.Errorf("Expected recovery message in body, got %q", w.Body.String())
		}
	})

	t.Run("NilTaskHandlerLeadsToError", func(t *testing.T) {
		r := NewRouter(nil)
		var nilTask *TaskHandler[None, None]
		_ = AddTaskHandler(r, http.MethodGet, "/nil-task", nilTask)
		req := httptest.NewRequest(http.MethodGet, "/nil-task", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError { // Expecting 500 due to robust check in createTaskFinalHandler
			t.Errorf("Expected non-OK status (specifically 500) for nil task handler, got %d", w.Code)
		}
	})
}

// --- TestParseInputEdgeCases ---
// These tests should remain valid.
func TestParseInputEdgeCases(t *testing.T) {
	type MyInput struct {
		Field string `json:"field"`
	}
	type MyOutput struct {
		OutputField string `json:"outputField"`
	}

	t.Run("NilParseInputWithTaskExpectingInput", func(t *testing.T) {
		r := NewRouter(&Options{ParseInput: nil})
		var receivedInput MyInput
		AddTaskHandler(r, http.MethodPost, "/test", TaskHandlerFromFunc(func(rd *ReqData[MyInput]) (MyOutput, error) {
			receivedInput = rd.Input()
			return MyOutput{OutputField: "got: " + rd.Input().Field}, nil
		}))
		body := strings.NewReader(`{"field":"hello"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", w.Code)
		}
		if receivedInput.Field != "" {
			t.Errorf("Expected zero value for input field, got %q", receivedInput.Field)
		}
	})

	t.Run("ParseInputMutatesInputPtr", func(t *testing.T) {
		r := NewRouter(&Options{
			ParseInput: func(req *http.Request, inputPtr any) error {
				if err := json.NewDecoder(req.Body).Decode(inputPtr); err != nil {
					return err
				}
				if mi, ok := inputPtr.(*MyInput); ok {
					mi.Field += "_mutated"
				}
				return nil
			},
		})
		var receivedInput MyInput
		AddTaskHandler(r, http.MethodPost, "/test", TaskHandlerFromFunc(func(rd *ReqData[MyInput]) (MyOutput, error) {
			receivedInput = rd.Input()
			return MyOutput{OutputField: "final: " + rd.Input().Field}, nil
		}))
		body := strings.NewReader(`{"field":"original"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", w.Code)
		}
		if receivedInput.Field != "original_mutated" {
			t.Errorf("Expected mutated input 'original_mutated', got %q", receivedInput.Field)
		}
	})
}

func TestParseInputSkipsHTTPRoutes(t *testing.T) {
	t.Run("TaskMiddleware_SlowPath", func(t *testing.T) {
		parseInputCalls := 0
		router := NewRouter(&Options{
			ParseInput: func(req *http.Request, inputPtr any) error {
				parseInputCalls++
				return errors.New("parse input should not run for HTTP handlers")
			},
		})

		AddGlobalTaskMiddleware(router, TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			return None{}, nil
		}))

		AddHTTPHandlerFunc(router, http.MethodGet, "/http", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/http", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", rec.Code)
		}
		if parseInputCalls != 0 {
			t.Fatalf("ParseInput called %d times, expected 0 for HTTP routes", parseInputCalls)
		}
	})

	t.Run("TasksCtxRequirer_SlowPath", func(t *testing.T) {
		parseInputCalls := 0
		router := NewRouter(&Options{
			ParseInput: func(req *http.Request, inputPtr any) error {
				parseInputCalls++
				return errors.New("parse input should not run for HTTP handlers")
			},
		})

		AddHTTPHandler(router, http.MethodGet, "/http", TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
			if GetTasksCtx(r) == nil {
				t.Fatal("TasksCtx should be available for TasksCtxRequirer")
			}
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/http", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", rec.Code)
		}
		if parseInputCalls != 0 {
			t.Fatalf("ParseInput called %d times, expected 0 for HTTP routes", parseInputCalls)
		}
	})
}

func TestTasksCtxRequirer(t *testing.T) {
	t.Run("TasksCtxRequirer_Gets_TasksCtx_Even_Without_Middleware", func(t *testing.T) {
		router := NewRouter()

		// Create a handler that implements TasksCtxRequirer
		handler := TasksCtxRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
			tasksCtx := GetTasksCtx(r)
			if tasksCtx == nil {
				t.Error("TasksCtx should be available for TasksCtxRequirer")
				http.Error(w, "No TasksCtx", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		AddHTTPHandler(router, http.MethodGet, "/test", handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("TasksCtxRequirer_With_Custom_Type", func(t *testing.T) {
		router := NewRouter(nil)

		h := customHandler{t: t}
		AddHTTPHandler(router, http.MethodGet, "/test", h)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("Regular_Handler_Without_TasksCtxRequirer", func(t *testing.T) {
		router := NewRouter(nil)

		// Regular handler that doesn't implement TasksCtxRequirer
		AddHTTPHandlerFunc(router, http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
			tasksCtx := GetTasksCtx(r)
			if tasksCtx != nil {
				t.Error("TasksCtx should not be available for regular handlers without middleware")
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})
}

// Define the custom handler type outside the test function
type customHandler struct {
	t *testing.T
}

func (h customHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tasksCtx := GetTasksCtx(r)
	if tasksCtx == nil {
		h.t.Error("TasksCtx should be available for custom TasksCtxRequirer")
		http.Error(w, "No TasksCtx", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h customHandler) NeedsTasksCtx() {}

func TestResponseProxy(t *testing.T) {
	t.Run("Task_Middleware_Sets_Response", func(t *testing.T) {
		router := NewRouter(nil)

		taskMw := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			rd.ResponseProxy().SetStatus(http.StatusForbidden, "Forbidden by Task MW")
			return None{}, nil
		})

		AddGlobalTaskMiddleware(router, taskMw)

		AddHTTPHandlerFunc(router, http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
			// This should not be called
			t.Error("Handler should not be called when response proxy has error")
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", rec.Code)
		}
	})
}

func TestTaskHandlerErrors(t *testing.T) {
	t.Run("Task_Handler_Returns_Error", func(t *testing.T) {
		router := NewRouter(nil)

		handler := TaskHandlerFromFunc(func(rd *ReqData[None]) (None, error) {
			return None{}, errors.New("handler error")
		})

		AddTaskHandler(router, http.MethodGet, "/test", handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})
}

func TestTaskMiddlewareErrors(t *testing.T) {
	t.Run("Task_Middleware_Error_Returns_500", func(t *testing.T) {
		router := NewRouter(nil)

		// Task middleware that returns an actual error (unexpected failure)
		taskMw := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			return None{}, errors.New("database connection failed")
		})

		AddGlobalTaskMiddleware(router, taskMw)

		var handlerCalled bool
		AddHTTPHandlerFunc(router, http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Handler should NOT be called when middleware errors
		if handlerCalled {
			t.Error("Handler should not be called when task middleware returns error")
		}

		// Should return 500 Internal Server Error for unexpected errors
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})

	t.Run("Task_Middleware_Sets_Error_Status_Returns_Nil", func(t *testing.T) {
		router := NewRouter(nil)

		// Task middleware that handles an expected case (not authenticated)
		taskMw := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			// Check auth header
			if rd.Request().Header.Get("Authorization") == "" {
				rd.ResponseProxy().SetStatus(401, "Authentication required")
				return None{}, nil // No error - this is expected
			}
			return None{}, nil
		})

		AddGlobalTaskMiddleware(router, taskMw)

		var handlerCalled bool
		AddHTTPHandlerFunc(router, http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		// Request without auth header
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Handler should NOT be called when proxy has error status
		if handlerCalled {
			t.Error("Handler should not be called when task middleware sets error status")
		}

		// Should return the custom error status
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}

		// Should have the custom error message
		if !strings.Contains(rec.Body.String(), "Authentication required") {
			t.Errorf("Expected body to contain 'Authentication required', got %q", rec.Body.String())
		}
	})

	t.Run("Task_Middleware_Sets_Redirect_Returns_Nil", func(t *testing.T) {
		router := NewRouter(nil)

		// Task middleware that redirects (expected case)
		taskMw := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			// Redirect unauthenticated users to login
			if rd.Request().Header.Get("Authorization") == "" {
				rd.ResponseProxy().Redirect(rd.Request(), "/login", 302)
				return None{}, nil // No error - this is expected behavior
			}
			return None{}, nil
		})

		AddGlobalTaskMiddleware(router, taskMw)

		var handlerCalled bool
		AddHTTPHandlerFunc(router, http.MethodGet, "/protected", func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Handler should NOT be called when proxy has redirect
		if handlerCalled {
			t.Error("Handler should not be called when task middleware sets redirect")
		}

		// Should return redirect status
		if rec.Code != http.StatusFound {
			t.Errorf("Expected status 302, got %d", rec.Code)
		}

		// Should have Location header
		if loc := rec.Header().Get("Location"); loc != "/login" {
			t.Errorf("Expected Location header '/login', got %q", loc)
		}
	})

	t.Run("Multiple_Task_Middlewares_Any_Error_Returns_500", func(t *testing.T) {
		router := NewRouter(nil)

		// Mix of success and error middlewares
		taskMw1 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			return None{}, nil // success
		})

		taskMw2 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			return None{}, errors.New("service unavailable") // unexpected error
		})

		taskMw3 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			return None{}, nil // success
		})

		AddGlobalTaskMiddleware(router, taskMw1)
		AddGlobalTaskMiddleware(router, taskMw2)
		AddGlobalTaskMiddleware(router, taskMw3)

		var handlerCalled bool
		AddHTTPHandlerFunc(router, http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Handler should not run when any middleware errors
		if handlerCalled {
			t.Error("Handler should not be called when any task middleware returns error")
		}

		// Unexpected errors always result in 500
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})

	t.Run("Multiple_Task_Middlewares_First_Error_Status_Wins", func(t *testing.T) {
		router := NewRouter(nil)

		// Multiple middlewares that set error statuses
		// According to MergeProxyResponses: "FIRST ERROR ... will win"
		taskMw1 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			// This runs but doesn't set error
			return None{}, nil
		})

		taskMw2 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			// This sets a 403 (first error)
			rd.ResponseProxy().SetStatus(403, "Forbidden")
			return None{}, nil // No Go error
		})

		taskMw3 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			// This tries to set 401 but 403 should win (first error)
			rd.ResponseProxy().SetStatus(401, "Unauthorized")
			return None{}, nil // No Go error
		})

		AddGlobalTaskMiddleware(router, taskMw1)
		AddGlobalTaskMiddleware(router, taskMw2)
		AddGlobalTaskMiddleware(router, taskMw3)

		var handlerCalled bool
		AddHTTPHandlerFunc(router, http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Handler should not run when proxy has error
		if handlerCalled {
			t.Error("Handler should not be called when any middleware sets error status")
		}

		// Should get the first error set
		if rec.Code != http.StatusForbidden {
			t.Errorf("Expected status 403 (first error), got %d", rec.Code)
		}

		if !strings.Contains(rec.Body.String(), "Forbidden") {
			t.Errorf("Expected body to contain 'Forbidden', got %q", rec.Body.String())
		}
	})

	t.Run("Task_Middleware_Success_Allows_Handler", func(t *testing.T) {
		router := NewRouter(nil)

		// All middlewares succeed with no errors or proxy responses
		taskMw1 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			// Do some setup work
			return None{}, nil
		})

		taskMw2 := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			// Do some validation that passes
			return None{}, nil
		})

		AddGlobalTaskMiddleware(router, taskMw1)
		AddGlobalTaskMiddleware(router, taskMw2)

		var handlerCalled bool
		AddHTTPHandlerFunc(router, http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Success"))
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Handler SHOULD run when all middlewares succeed
		if !handlerCalled {
			t.Error("Handler should be called when all task middlewares succeed")
		}

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		if rec.Body.String() != "Success" {
			t.Errorf("Expected body 'Success', got %q", rec.Body.String())
		}
	})
}
