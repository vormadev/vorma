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
		r := NewRouter()
		var task_mw_ran bool
		var main_handler_ran bool

		task_mw := TaskMiddlewareFromFunc(
			func(rc *RequestCtx[None]) (None, error) {
				task_mw_ran = true
				rc.ResponseProxy().
					SetStatus(http.StatusForbidden, "Forbidden by Task MW")
				return None{}, errors.New("task middleware intentional error")
			},
		)
		AddGlobalTaskMiddleware(r, task_mw)

		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, r *http.Request) {
				main_handler_ran = true
				t.Error(
					"Main handler should not be called if task middleware errors",
				)
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !task_mw_ran {
			t.Error("Task middleware should have run")
		}
		if main_handler_ran {
			t.Error(
				"Main handler ran but should have been short-circuited by task middleware error",
			)
		}
		if w.Code != http.StatusInternalServerError {
			t.Errorf(
				"Expected status 500 when middleware returns error, got %d",
				w.Code,
			)
		}
		if !strings.Contains(w.Body.String(), "Internal Server Error") {
			t.Errorf(
				"Expected body to contain 'Internal Server Error', got %q",
				w.Body.String(),
			)
		}
	})

	t.Run("TaskMiddlewareSetsClientErrorAndHalts", func(t *testing.T) {
		r := NewRouter()
		task_mw := TaskMiddlewareFromFunc(
			func(rd *RequestCtx[None]) (None, error) {
				rd.ResponseProxy().SetStatus(http.StatusTeapot)
				rd.ResponseProxy().SetHeader("X-Tea-Type", "Earl Grey")
				return None{}, nil
			},
		)
		AddGlobalTaskMiddleware(r, task_mw)

		main_handler_ran := false
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/tea",
			func(w http.ResponseWriter, r *http.Request) {
				main_handler_ran = true
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/tea", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if main_handler_ran {
			t.Error(
				"Main handler ran but should have been short-circuited by task middleware 418 error status",
			)
		}
		if w.Code != http.StatusTeapot {
			t.Errorf("Expected status %d, got %d", http.StatusTeapot, w.Code)
		}
		if w.Header().Get("X-Tea-Type") != "Earl Grey" {
			t.Errorf(
				"Expected header 'X-Tea-Type: Earl Grey', got %q",
				w.Header().Get("X-Tea-Type"),
			)
		}
	})

	t.Run("MultipleTaskMiddlewaresMergeProxiesAndCanHalt", func(t *testing.T) {
		r := NewRouter()
		var mw1_ran, mw2_ran, mw3_ran bool
		var main_handler_ran bool
		var wg sync.WaitGroup
		wg.Add(3)

		tmw1 := TaskMiddlewareFromFunc(
			func(rd *RequestCtx[None]) (None, error) {
				defer wg.Done()
				mw1_ran = true
				rd.ResponseProxy().AddHeader("X-Multi-Trace", "MW1")
				rd.ResponseProxy().SetStatus(http.StatusAccepted)
				return None{}, nil
			},
		)
		tmw2 := TaskMiddlewareFromFunc(
			func(rd *RequestCtx[None]) (None, error) {
				defer wg.Done()
				mw2_ran = true
				rd.ResponseProxy().AddHeader("X-Multi-Trace", "MW2")
				rd.ResponseProxy().SetStatus(http.StatusConflict)
				return None{}, nil
			},
		)
		tmw3 := TaskMiddlewareFromFunc(
			func(rd *RequestCtx[None]) (None, error) {
				defer wg.Done()
				mw3_ran = true
				rd.ResponseProxy().AddHeader("X-Multi-Trace", "MW3")
				return None{}, nil
			},
		)

		AddGlobalTaskMiddleware(r, tmw1)
		AddMethodLevelTaskMiddleware(r, http.MethodGet, tmw2)
		route := AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/multi",
			func(w http.ResponseWriter, r *http.Request) {
				main_handler_ran = true
			},
		)
		AddPatternLevelTaskMiddleware(route, tmw3)

		req := httptest.NewRequest(http.MethodGet, "/multi", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		wg.Wait()

		if !(mw1_ran && mw2_ran && mw3_ran) {
			t.Errorf(
				"Expected all task middlewares to run, got: mw1=%v, mw2=%v, mw3=%v",
				mw1_ran,
				mw2_ran,
				mw3_ran,
			)
		}
		if main_handler_ran {
			t.Error(
				"Main handler ran but should have been short-circuited by task middleware 409 error status",
			)
		}
		if w.Code != http.StatusConflict {
			t.Errorf("Expected status %d, got %d", http.StatusConflict, w.Code)
		}
		traces := w.Header().Values("X-Multi-Trace")
		if len(traces) != 3 {
			t.Errorf(
				"Expected 3 X-Multi-Trace headers, got %d: %v",
				len(traces),
				traces,
			)
		}
	})

	t.Run(
		"TaskMiddlewareChainCacheInvalidatesWhenNewMiddlewaresAreRegistered",
		func(t *testing.T) {
			r := NewRouter()
			route := AddTaskHandler(
				r,
				http.MethodGet,
				"/cache-chain",
				TaskHandlerFromFunc(
					func(rd *RequestCtx[None]) (map[string]string, error) {
						return map[string]string{"ok": "true"}, nil
					},
				),
			)

			AddGlobalTaskMiddleware(
				r,
				TaskMiddlewareFromFunc(
					func(rd *RequestCtx[None]) (None, error) {
						rd.ResponseProxy().AddHeader("X-Task-MW", "global-1")
						return None{}, nil
					},
				),
			)

			first_req := httptest.NewRequest(
				http.MethodGet,
				"/cache-chain",
				nil,
			)
			first_res := httptest.NewRecorder()
			r.ServeHTTP(first_res, first_req)
			if first_res.Code != http.StatusOK {
				t.Fatalf(
					"first request status = %d, want %d",
					first_res.Code,
					http.StatusOK,
				)
			}
			if got := first_res.Header().Values("X-Task-MW"); len(got) != 1 ||
				got[0] != "global-1" {
				t.Fatalf("first request X-Task-MW = %v, want [global-1]", got)
			}

			AddGlobalTaskMiddleware(
				r,
				TaskMiddlewareFromFunc(
					func(rd *RequestCtx[None]) (None, error) {
						rd.ResponseProxy().AddHeader("X-Task-MW", "global-2")
						return None{}, nil
					},
				),
			)
			AddMethodLevelTaskMiddleware(
				r,
				http.MethodGet,
				TaskMiddlewareFromFunc(
					func(rd *RequestCtx[None]) (None, error) {
						rd.ResponseProxy().AddHeader("X-Task-MW", "method")
						return None{}, nil
					},
				),
			)
			AddPatternLevelTaskMiddleware(
				route,
				TaskMiddlewareFromFunc(
					func(rd *RequestCtx[None]) (None, error) {
						rd.ResponseProxy().AddHeader("X-Task-MW", "pattern")
						return None{}, nil
					},
				),
			)

			second_req := httptest.NewRequest(
				http.MethodGet,
				"/cache-chain",
				nil,
			)
			second_res := httptest.NewRecorder()
			r.ServeHTTP(second_res, second_req)
			if second_res.Code != http.StatusOK {
				t.Fatalf(
					"second request status = %d, want %d",
					second_res.Code,
					http.StatusOK,
				)
			}

			got_values := second_res.Header().Values("X-Task-MW")
			if len(got_values) != 4 {
				t.Fatalf(
					"second request X-Task-MW len = %d, want 4 (%v)",
					len(got_values),
					got_values,
				)
			}
			got_set := map[string]bool{}
			for _, v := range got_values {
				got_set[v] = true
			}
			for _, want := range []string{"global-1", "global-2", "method", "pattern"} {
				if !got_set[want] {
					t.Fatalf(
						"second request X-Task-MW missing value %q (got %v)",
						want,
						got_values,
					)
				}
			}
		},
	)
}

func TestComplexMiddlewareScenarios(t *testing.T) {
	t.Run("MixedStackOrderAndExecution", func(t *testing.T) {
		r := NewRouter()
		var execution_order []string
		var mu sync.Mutex

		append_order := func(id string) {
			mu.Lock()
			defer mu.Unlock()
			execution_order = append(execution_order, id)
		}

		AddGlobalHTTPMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					append_order("GlobalHTTP-Pre")
					next.ServeHTTP(w, r)
					append_order("GlobalHTTP-Post")
				},
			)
		})
		AddMethodLevelHTTPMiddleware(
			r,
			http.MethodGet,
			func(next http.Handler) http.Handler {
				return http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						append_order("MethodHTTP-Pre")
						next.ServeHTTP(w, r)
						append_order("MethodHTTP-Post")
					},
				)
			},
		)

		var global_task_done, method_task_done, pattern_task_done bool
		var wg sync.WaitGroup
		wg.Add(3)

		AddGlobalTaskMiddleware(
			r,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				defer wg.Done()
				append_order("GlobalTask")
				rd.ResponseProxy().SetHeader("X-Global-Task", "Done")
				global_task_done = true
				return None{}, nil
			}),
		)
		AddMethodLevelTaskMiddleware(
			r,
			http.MethodGet,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				defer wg.Done()
				append_order("MethodTask")
				rd.ResponseProxy().SetHeader("X-Method-Task", "Done")
				method_task_done = true
				return None{}, nil
			}),
		)

		route := AddTaskHandler(
			r,
			http.MethodGet,
			"/complex",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				append_order("TaskHandler")
				rd.ResponseProxy().SetHeader("X-Handler-Task", "Done")
				return "handler_done", nil
			}),
		)
		AddPatternLevelHTTPMiddleware(
			route,
			func(next http.Handler) http.Handler {
				return http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						append_order("PatternHTTP-Pre")
						next.ServeHTTP(w, r)
						append_order("PatternHTTP-Post")
					},
				)
			},
		)
		AddPatternLevelTaskMiddleware(
			route,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				defer wg.Done()
				append_order("PatternTask")
				rd.ResponseProxy().SetHeader("X-Pattern-Task", "Done")
				pattern_task_done = true
				return None{}, nil
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/complex", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		wg.Wait()

		expected_http_order := []string{
			"GlobalHTTP-Pre", "MethodHTTP-Pre", "PatternHTTP-Pre",
			"TaskHandler",
			"PatternHTTP-Post", "MethodHTTP-Post", "GlobalHTTP-Post",
		}
		var http_order []string
		mu.Lock()
		for _, entry := range execution_order {
			if strings.Contains(entry, "HTTP") ||
				strings.Contains(entry, "Handler") {
				http_order = append(http_order, entry)
			}
		}
		mu.Unlock()

		if !slice_equal(http_order, expected_http_order) {
			t.Errorf(
				"HTTP execution order incorrect.\nExpected: %v\nGot:      %v",
				expected_http_order,
				http_order,
			)
		}
		if !(global_task_done && method_task_done && pattern_task_done) {
			t.Errorf(
				"Not all task middlewares ran: G=%v, M=%v, P=%v",
				global_task_done,
				method_task_done,
				pattern_task_done,
			)
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

func TestRequestCtxAccess(t *testing.T) {
	t.Run("InTaskHandler", func(t *testing.T) {
		r := NewRouter()
		var params_ok, splat_ok, ctx_ok, req_ok, proxy_ok bool

		AddTaskHandler(
			r,
			http.MethodGet,
			"/task/:id/path/*",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (string, error) {
				if len(rd.Params()) > 0 && rd.Params()["id"] == "123" {
					params_ok = true
				}
				if len(rd.SplatValues()) > 0 && rd.SplatValues()[0] == "foo" {
					splat_ok = true
				}
				if rd.TasksCache() != nil {
					ctx_ok = true
				}
				if rd.Request() != nil {
					req_ok = true
				}
				if rd.ResponseProxy() != nil {
					proxy_ok = true
					rd.ResponseProxy().SetHeader("X-From-Task", "OK")
				}
				return "ok", nil
			}),
		)

		req := httptest.NewRequest(
			http.MethodGet,
			"/task/123/path/foo/bar",
			nil,
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !params_ok {
			t.Error("Params not checked or incorrect")
		}
		if !splat_ok {
			t.Error("SplatValues not checked or incorrect")
		}
		if !ctx_ok {
			t.Error("TasksCache was nil")
		}
		if !req_ok {
			t.Error("Request was nil")
		}
		if !proxy_ok {
			t.Error("ResponseProxy was nil")
		}
		if w.Header().Get("X-From-Task") != "OK" {
			t.Error("Header from task via proxy not set")
		}
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", w.Code)
		}
	})

	t.Run("InHTTPHandler_StandardPath", func(t *testing.T) {
		r := NewRouter()
		var params_ok, splat_ok bool
		AddGlobalTaskMiddleware(
			r,
			TaskMiddlewareFromFunc(
				func(rd *RequestCtx[None]) (None, error) { return None{}, nil },
			),
		)

		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/http/:id/path/*",
			func(w http.ResponseWriter, req *http.Request) {
				params := GetParams(req)
				splats := GetSplatValues(req)
				if len(params) > 0 && params["id"] == "456" {
					params_ok = true
				}
				if len(splats) > 0 && splats[0] == "baz" {
					splat_ok = true
				}
				w.WriteHeader(http.StatusOK)
			},
		)

		req := httptest.NewRequest(
			http.MethodGet,
			"/http/456/path/baz/qux",
			nil,
		)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if !params_ok {
			t.Error("Params not checked or incorrect in HTTP handler")
		}
		if !splat_ok {
			t.Error("SplatValues not checked or incorrect in HTTP handler")
		}
	})
}

func TestRoutingEdgeCases(t *testing.T) {
	t.Run("StaticVsParam", func(t *testing.T) {
		r := NewRouter()
		var static_called, param_called bool
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/users/new",
			func(w http.ResponseWriter, r *http.Request) { static_called = true },
		)
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/users/:id",
			func(w http.ResponseWriter, r *http.Request) { param_called = true },
		)

		req := httptest.NewRequest(http.MethodGet, "/users/new", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if !static_called || param_called {
			t.Errorf(
				"Expected static route called, static=%v param=%v",
				static_called,
				param_called,
			)
		}

		static_called, param_called = false, false
		req = httptest.NewRequest(http.MethodGet, "/users/123", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if static_called || !param_called {
			t.Errorf(
				"Expected param route called, static=%v param=%v",
				static_called,
				param_called,
			)
		}
	})

	t.Run("SplatURLDecoding", func(t *testing.T) {
		r := NewRouter()
		var captured_splat []string

		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/test/*",
			func(w http.ResponseWriter, req *http.Request) {
				captured_splat = GetSplatValues(req)
				w.WriteHeader(http.StatusOK)
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/test/foo%20bar%2Fbaz", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected status OK, got %d", rec.Code)
		}
		expected := []string{"foo bar", "baz"}
		if !slice_equal(captured_splat, expected) {
			t.Errorf(
				"Expected splat values %v, got %v",
				expected,
				captured_splat,
			)
		}
		reconstructed := strings.Join(captured_splat, "/")
		if reconstructed != "foo bar/baz" {
			t.Errorf(
				"Expected reconstructed 'foo bar/baz', got %q",
				reconstructed,
			)
		}
	})

	t.Run("EmptySplat", func(t *testing.T) {
		r := NewRouter()
		var splat_values []string
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/files/*",
			func(w http.ResponseWriter, req *http.Request) {
				splat_values = GetSplatValues(req)
			},
		)
		req := httptest.NewRequest(http.MethodGet, "/files/", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if !(len(splat_values) == 1 && splat_values[0] == "") {
			t.Errorf(
				"Expected splat for /files/ to be [\"\"], got %v",
				splat_values,
			)
		}
	})
}

func TestServeHTTP_ErrorHandling(t *testing.T) {
	t.Run("PanicRecoveryMiddleware", func(t *testing.T) {
		r := NewRouter()
		AddGlobalHTTPMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					defer func() {
						if err := recover(); err != nil {
							fmt.Println("Recovered from panic:", err)
							http.Error(
								w,
								"Recovered Internal Server Error",
								http.StatusInternalServerError,
							)
						}
					}()
					next.ServeHTTP(w, r)
				},
			)
		})
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/panic",
			func(w http.ResponseWriter, r *http.Request) {
				panic("intentional panic in handler")
			},
		)
		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 after panic, got %d", w.Code)
		}
		if !strings.Contains(
			w.Body.String(),
			"Recovered Internal Server Error",
		) {
			t.Errorf(
				"Expected recovery message in body, got %q",
				w.Body.String(),
			)
		}
	})

	t.Run("NilTaskHandlerLeadsToError", func(t *testing.T) {
		r := NewRouter()
		var nil_task *TaskHandler[None, None]
		_ = AddTaskHandler(r, http.MethodGet, "/nil-task", nil_task)
		req := httptest.NewRequest(http.MethodGet, "/nil-task", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500 for nil task handler, got %d", w.Code)
		}
	})
}

func TestParseInputEdgeCases(t *testing.T) {
	type my_input struct {
		Field string `json:"field"`
	}
	type my_output struct {
		OutputField string `json:"outputField"`
	}

	t.Run("NilParseInputWithTaskExpectingInput", func(t *testing.T) {
		r := NewRouter(Options{ParseInput: nil})
		var received_input my_input
		AddTaskHandler(
			r,
			http.MethodPost,
			"/test",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[my_input]) (my_output, error) {
					received_input = rd.Input()
					return my_output{
						OutputField: "got: " + rd.Input().Field,
					}, nil
				},
			),
		)
		body := strings.NewReader(`{"field":"hello"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", w.Code)
		}
		if received_input.Field != "" {
			t.Errorf(
				"Expected zero value for input field, got %q",
				received_input.Field,
			)
		}
	})

	t.Run("ParseInputMutatesInputPtr", func(t *testing.T) {
		r := NewRouter(Options{
			ParseInput: func(req *http.Request, input_ptr any) error {
				if err := json.NewDecoder(req.Body).Decode(input_ptr); err != nil {
					return err
				}
				if mi, ok := input_ptr.(*my_input); ok {
					mi.Field += "_mutated"
				}
				return nil
			},
		})
		var received_input my_input
		AddTaskHandler(
			r,
			http.MethodPost,
			"/test",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[my_input]) (my_output, error) {
					received_input = rd.Input()
					return my_output{
						OutputField: "final: " + rd.Input().Field,
					}, nil
				},
			),
		)
		body := strings.NewReader(`{"field":"original"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %d", w.Code)
		}
		if received_input.Field != "original_mutated" {
			t.Errorf(
				"Expected 'original_mutated', got %q",
				received_input.Field,
			)
		}
	})
}

func TestParseInputSkipsHTTPRoutes(t *testing.T) {
	t.Run("TaskMiddleware_SlowPath", func(t *testing.T) {
		parse_calls := 0
		router := NewRouter(Options{
			ParseInput: func(req *http.Request, input_ptr any) error {
				parse_calls++
				return errors.New(
					"parse input should not run for HTTP handlers",
				)
			},
		})
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				return None{}, nil
			}),
		)
		AddHTTPHandlerFunc(
			router,
			http.MethodGet,
			"/http",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/http", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", rec.Code)
		}
		if parse_calls != 0 {
			t.Fatalf("ParseInput called %d times, expected 0", parse_calls)
		}
	})

	t.Run("TasksCacheRequirer_SlowPath", func(t *testing.T) {
		parse_calls := 0
		router := NewRouter(Options{
			ParseInput: func(req *http.Request, input_ptr any) error {
				parse_calls++
				return errors.New(
					"parse input should not run for HTTP handlers",
				)
			},
		})
		AddHTTPHandler(
			router,
			http.MethodGet,
			"/http",
			TasksCacheRequirerFunc(func(w http.ResponseWriter, r *http.Request) {
				if GetTasksCache(r) == nil {
					t.Fatal("TasksCache should be available for TasksCacheRequirer")
				}
				w.WriteHeader(http.StatusNoContent)
			}),
		)

		req := httptest.NewRequest(http.MethodGet, "/http", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("Expected status 204, got %d", rec.Code)
		}
		if parse_calls != 0 {
			t.Fatalf("ParseInput called %d times, expected 0", parse_calls)
		}
	})
}

func TestTasksCacheRequirer(t *testing.T) {
	t.Run("Gets_TasksCache_Even_Without_Middleware", func(t *testing.T) {
		router := NewRouter()
		handler := TasksCacheRequirerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if GetTasksCache(r) == nil {
					t.Error("TasksCache should be available for TasksCacheRequirer")
					http.Error(w, "No TasksCache", http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
		)
		AddHTTPHandler(router, http.MethodGet, "/test", handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("With_Custom_Type", func(t *testing.T) {
		router := NewRouter()
		h := custom_handler{t: t}
		AddHTTPHandler(router, http.MethodGet, "/test", h)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("Regular_Handler_Without_TasksCacheRequirer", func(t *testing.T) {
		router := NewRouter()
		AddHTTPHandlerFunc(
			router,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, r *http.Request) {
				if GetTasksCache(r) != nil {
					t.Error(
						"TasksCache should not be available for regular handlers without middleware",
					)
				}
				w.WriteHeader(http.StatusOK)
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})
}

type custom_handler struct{ t *testing.T }

func (h custom_handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if GetTasksCache(r) == nil {
		h.t.Error("TasksCache should be available for custom TasksCacheRequirer")
		http.Error(w, "No TasksCache", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
func (h custom_handler) NeedsTasksCache() {}

func TestResponseProxy(t *testing.T) {
	t.Run("Task_Middleware_Sets_Response", func(t *testing.T) {
		router := NewRouter()
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				rd.ResponseProxy().
					SetStatus(http.StatusForbidden, "Forbidden by Task MW")
				return None{}, nil
			}),
		)
		AddHTTPHandlerFunc(
			router,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, r *http.Request) {
				t.Error(
					"Handler should not be called when response proxy has error",
				)
				w.WriteHeader(http.StatusOK)
			},
		)

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
		router := NewRouter()
		AddTaskHandler(router, http.MethodGet, "/test",
			TaskHandlerFromFunc(func(rd *RequestCtx[None]) (None, error) {
				return None{}, errors.New("handler error")
			}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})
}

func TestTaskMiddlewareErrors(t *testing.T) {
	t.Run("Error_Returns_500", func(t *testing.T) {
		router := NewRouter()
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				return None{}, errors.New("database connection failed")
			}),
		)
		var handler_called bool
		AddHTTPHandlerFunc(
			router,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, r *http.Request) {
				handler_called = true
				w.WriteHeader(http.StatusOK)
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if handler_called {
			t.Error(
				"Handler should not be called when task middleware returns error",
			)
		}
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})

	t.Run("Sets_Error_Status_Returns_Nil", func(t *testing.T) {
		router := NewRouter()
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				if rd.Request().Header.Get("Authorization") == "" {
					rd.ResponseProxy().SetStatus(401, "Authentication required")
					return None{}, nil
				}
				return None{}, nil
			}),
		)
		var handler_called bool
		AddHTTPHandlerFunc(
			router,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, r *http.Request) {
				handler_called = true
				w.WriteHeader(http.StatusOK)
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if handler_called {
			t.Error(
				"Handler should not be called when task middleware sets error status",
			)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Authentication required") {
			t.Errorf(
				"Expected 'Authentication required' in body, got %q",
				rec.Body.String(),
			)
		}
	})

	t.Run("Sets_Redirect_Returns_Nil", func(t *testing.T) {
		router := NewRouter()
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				if rd.Request().Header.Get("Authorization") == "" {
					rd.ResponseProxy().Redirect(rd.Request(), "/login", 302)
					return None{}, nil
				}
				return None{}, nil
			}),
		)
		var handler_called bool
		AddHTTPHandlerFunc(
			router,
			http.MethodGet,
			"/protected",
			func(w http.ResponseWriter, r *http.Request) {
				handler_called = true
				w.WriteHeader(http.StatusOK)
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if handler_called {
			t.Error(
				"Handler should not be called when task middleware sets redirect",
			)
		}
		if rec.Code != http.StatusFound {
			t.Errorf("Expected status 302, got %d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/login" {
			t.Errorf("Expected Location '/login', got %q", loc)
		}
	})

	t.Run("Multiple_Any_Error_Returns_500", func(t *testing.T) {
		router := NewRouter()
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(
				func(rd *RequestCtx[None]) (None, error) { return None{}, nil },
			),
		)
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				return None{}, errors.New("service unavailable")
			}),
		)
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(
				func(rd *RequestCtx[None]) (None, error) { return None{}, nil },
			),
		)

		var handler_called bool
		AddHTTPHandlerFunc(
			router,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, r *http.Request) {
				handler_called = true
				w.WriteHeader(http.StatusOK)
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if handler_called {
			t.Error(
				"Handler should not be called when any task middleware returns error",
			)
		}
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rec.Code)
		}
	})

	t.Run("First_Error_Status_Wins", func(t *testing.T) {
		router := NewRouter()
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(
				func(rd *RequestCtx[None]) (None, error) { return None{}, nil },
			),
		)
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				rd.ResponseProxy().SetStatus(403, "Forbidden")
				return None{}, nil
			}),
		)
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				rd.ResponseProxy().SetStatus(401, "Unauthorized")
				return None{}, nil
			}),
		)

		var handler_called bool
		AddHTTPHandlerFunc(
			router,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, r *http.Request) {
				handler_called = true
				w.WriteHeader(http.StatusOK)
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if handler_called {
			t.Error(
				"Handler should not be called when any middleware sets error status",
			)
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("Expected status 403 (first error), got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Forbidden") {
			t.Errorf("Expected 'Forbidden' in body, got %q", rec.Body.String())
		}
	})

	t.Run("Success_Allows_Handler", func(t *testing.T) {
		router := NewRouter()
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(
				func(rd *RequestCtx[None]) (None, error) { return None{}, nil },
			),
		)
		AddGlobalTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(
				func(rd *RequestCtx[None]) (None, error) { return None{}, nil },
			),
		)

		var handler_called bool
		AddHTTPHandlerFunc(
			router,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, r *http.Request) {
				handler_called = true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Success"))
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if !handler_called {
			t.Error(
				"Handler should be called when all task middlewares succeed",
			)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
		if rec.Body.String() != "Success" {
			t.Errorf("Expected body 'Success', got %q", rec.Body.String())
		}
	})
}
