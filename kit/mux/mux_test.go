package mux

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

func TestRouterBasics(t *testing.T) {
	t.Run("NewRouter_Defaults", func(t *testing.T) {
		r := NewRouter()
		if r.DynamicParamPrefix() != ':' {
			t.Error("Default dynamic param prefix should be ':'")
		}
		if r.SplatSegmentIdentifier() != '*' {
			t.Error("Default splat segment should be '*'")
		}
	})

	t.Run("NewRouter_WithOptions", func(t *testing.T) {
		r := NewRouter(Options{
			DynamicParamPrefix:     '@',
			SplatSegmentIdentifier: '#',
			MountRoot:              "/api",
		})
		if r.DynamicParamPrefix() != '@' {
			t.Error("DynamicParamPrefix not set correctly")
		}
		if r.SplatSegmentIdentifier() != '#' {
			t.Error("SplatSegmentIdentifier not set correctly")
		}
		if r.MountRoot() != "/api/" {
			t.Errorf(
				"MountRoot not normalized correctly, got %q",
				r.MountRoot(),
			)
		}
	})

	t.Run("MountRoot_Normalization", func(t *testing.T) {
		cases := []struct {
			input    string
			expected string
		}{
			{"", ""},
			{"/", ""},
			{"api", "/api/"},
			{"/api", "/api/"},
			{"/api/", "/api/"},
			{"api/v1", "/api/v1/"},
		}
		for _, tc := range cases {
			r := NewRouter(Options{MountRoot: tc.input})
			if got := r.MountRoot(); got != tc.expected {
				t.Errorf(
					"MountRoot(%q) = %q, want %q",
					tc.input,
					got,
					tc.expected,
				)
			}
		}
	})
}

func TestHTTPHandlers(t *testing.T) {
	methods := []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace,
	}

	for _, method := range methods {
		t.Run("Method_"+method, func(t *testing.T) {
			r := NewRouter()
			called := false
			AddHTTPHandlerFunc(
				r,
				method,
				"/test",
				func(w http.ResponseWriter, r *http.Request) {
					called = true
					w.WriteHeader(http.StatusOK)
				},
			)

			req := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if !called {
				t.Errorf("Handler not called for method %s", method)
			}
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}

	t.Run("HEAD_Fallback_To_GET", func(t *testing.T) {
		r := NewRouter()
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Custom", "value")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("body content"))
			},
		)

		req := httptest.NewRequest(http.MethodHead, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if w.Header().Get("X-Custom") != "value" {
			t.Error("Headers not preserved in HEAD request")
		}
		if w.Body.Len() > 0 {
			t.Error("HEAD request should not have body")
		}
	})

	t.Run("HEAD_Fallback_PreservesNoBodyOnValidationError", func(t *testing.T) {
		type parse_input struct {
			Name string `json:"name"`
		}
		r := NewRouter(Options{
			ParseInput: func(req *http.Request, input_ptr any) error {
				return &schema.ValidationError{
					Err: errors.New("invalid input"),
				}
			},
		})
		AddTaskHandler(
			r,
			http.MethodGet,
			"/validate",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[parse_input]) (map[string]string, error) {
					return map[string]string{"ok": "true"}, nil
				},
			),
		)

		req := httptest.NewRequest(http.MethodHead, "/validate", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
		if w.Body.Len() > 0 {
			t.Fatalf(
				"expected empty body for HEAD fallback validation error, got %q",
				w.Body.String(),
			)
		}
	})

	t.Run("HEAD_Fallback_PreservesInferredHeaders", func(t *testing.T) {
		r := NewRouter()
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/inferred-headers",
			func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("<html><body>head parity</body></html>"))
			},
		)

		get_req := httptest.NewRequest(http.MethodGet, "/inferred-headers", nil)
		get_resp := httptest.NewRecorder()
		r.ServeHTTP(get_resp, get_req)

		head_req := httptest.NewRequest(
			http.MethodHead,
			"/inferred-headers",
			nil,
		)
		head_resp := httptest.NewRecorder()
		r.ServeHTTP(head_resp, head_req)

		if head_resp.Code != get_resp.Code {
			t.Fatalf(
				"HEAD status %d != GET status %d",
				head_resp.Code,
				get_resp.Code,
			)
		}
		if head_resp.Header().
			Get("Content-Type") !=
			get_resp.Header().
				Get("Content-Type") {
			t.Fatalf(
				"HEAD Content-Type %q != GET Content-Type %q",
				head_resp.Header().
					Get("Content-Type"),
				get_resp.Header().Get("Content-Type"),
			)
		}
		expected_cl := strconv.Itoa(get_resp.Body.Len())
		if head_resp.Header().Get("Content-Length") != expected_cl {
			t.Fatalf(
				"HEAD Content-Length %q, want %q",
				head_resp.Header().Get("Content-Length"),
				expected_cl,
			)
		}
		if head_resp.Body.Len() > 0 {
			t.Fatalf(
				"expected empty body for HEAD, got %q",
				head_resp.Body.String(),
			)
		}
	})
}

func TestTaskHandlers(t *testing.T) {
	type test_input struct {
		Name string `json:"name"`
	}
	type test_output struct {
		Message string `json:"message"`
	}

	t.Run("Basic_TaskHandler", func(t *testing.T) {
		r := NewRouter(Options{
			ParseInput: func(req *http.Request, input_ptr any) error {
				return json.NewDecoder(req.Body).Decode(input_ptr)
			},
		})
		handler := TaskHandlerFromFunc(
			func(rd *RequestCtx[test_input]) (test_output, error) {
				return test_output{Message: "Hello " + rd.Input().Name}, nil
			},
		)
		AddTaskHandler(r, http.MethodPost, "/greet", handler)

		body := strings.NewReader(`{"name":"World"}`)
		req := httptest.NewRequest(http.MethodPost, "/greet", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		var resp test_output
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		if resp.Message != "Hello World" {
			t.Errorf("Expected 'Hello World', got %q", resp.Message)
		}
	})

	t.Run("TaskHandler_With_None_Input", func(t *testing.T) {
		r := NewRouter()
		handler := TaskHandlerFromFunc(
			func(rd *RequestCtx[None]) (test_output, error) {
				return test_output{Message: "No input needed"}, nil
			},
		)
		AddTaskHandler(r, http.MethodGet, "/status", handler)

		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		var resp test_output
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Message != "No input needed" {
			t.Errorf("Expected 'No input needed', got %q", resp.Message)
		}
	})
}

func TestGetParams(t *testing.T) {
	t.Run("Dynamic_Params", func(t *testing.T) {
		r := NewRouter()
		var captured Params
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/users/:id",
			func(w http.ResponseWriter, req *http.Request) {
				captured = GetParams(req)
				w.WriteHeader(http.StatusOK)
			},
		)
		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if captured["id"] != "123" {
			t.Errorf("Expected param id='123', got %q", captured["id"])
		}
	})

	t.Run("Multiple_Params", func(t *testing.T) {
		r := NewRouter()
		var captured Params
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/users/:userId/posts/:postId",
			func(w http.ResponseWriter, req *http.Request) {
				captured = GetParams(req)
				w.WriteHeader(http.StatusOK)
			},
		)
		req := httptest.NewRequest(http.MethodGet, "/users/456/posts/789", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if captured["userId"] != "456" {
			t.Errorf("Expected userId='456', got %q", captured["userId"])
		}
		if captured["postId"] != "789" {
			t.Errorf("Expected postId='789', got %q", captured["postId"])
		}
	})

	t.Run("Splat_Values", func(t *testing.T) {
		r := NewRouter()
		var captured []string
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/files/*",
			func(w http.ResponseWriter, req *http.Request) {
				captured = GetSplatValues(req)
				w.WriteHeader(http.StatusOK)
			},
		)
		req := httptest.NewRequest(
			http.MethodGet,
			"/files/path/to/file.txt",
			nil,
		)
		r.ServeHTTP(httptest.NewRecorder(), req)
		expected := []string{"path", "to", "file.txt"}
		if !slice_equal(captured, expected) {
			t.Errorf("Expected splat %v, got %v", expected, captured)
		}
	})
}

func TestParams_NoParamsReturnsNilMap(t *testing.T) {
	router := NewRouter()
	request_count := 0
	var first_params, second_params Params

	AddHTTPHandlerFunc(
		router,
		http.MethodGet,
		"/no-params",
		func(w http.ResponseWriter, req *http.Request) {
			request_count++
			params := GetParams(req)
			if request_count == 1 {
				first_params = params
			}
			if request_count == 2 {
				second_params = params
			}
			w.WriteHeader(http.StatusOK)
		},
	)

	router.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/no-params", nil),
	)
	router.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/no-params", nil),
	)

	if first_params != nil {
		t.Fatalf(
			"expected first no-params map to be nil, got %#v",
			first_params,
		)
	}
	if second_params != nil {
		t.Fatalf(
			"expected second no-params map to be nil, got %#v",
			second_params,
		)
	}
}

func TestInjectTasksCacheMiddleware_NoParamsReturnsNilMap(t *testing.T) {
	request_count := 0
	var first_params, second_params Params
	var first_proxy_nil, second_proxy_nil bool

	handler := InjectTasksCacheMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			request_count++
			params := GetParams(req)
			if request_count == 1 {
				first_params = params
			}
			if request_count == 2 {
				second_params = params
			}
			// Access internal rd_transport to check proxy is nil.
			rd := request_store.Value(req.Context())
			if rd == nil {
				t.Fatal("expected request data to be present")
			}
			if request_count == 1 {
				first_proxy_nil = rd.response_proxy == nil
			}
			if request_count == 2 {
				second_proxy_nil = rd.response_proxy == nil
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	handler.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/middleware-no-params", nil),
	)
	handler.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/middleware-no-params", nil),
	)

	if first_params != nil {
		t.Fatalf("expected first params nil, got %#v", first_params)
	}
	if second_params != nil {
		t.Fatalf("expected second params nil, got %#v", second_params)
	}
	if !first_proxy_nil || !second_proxy_nil {
		t.Fatalf(
			"expected proxy nil; first=%v second=%v",
			first_proxy_nil,
			second_proxy_nil,
		)
	}
}

func TestHTTPMiddleware(t *testing.T) {
	t.Run("Global_Middleware_Order", func(t *testing.T) {
		r := NewRouter()
		var order []string
		UseMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(
				func(w http.ResponseWriter, req *http.Request) {
					order = append(order, "global1")
					next.ServeHTTP(w, req)
				},
			)
		})
		UseMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(
				func(w http.ResponseWriter, req *http.Request) {
					order = append(order, "global2")
					next.ServeHTTP(w, req)
				},
			)
		})
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, req *http.Request) {
				order = append(order, "handler")
			},
		)

		r.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/test", nil),
		)
		expected := []string{"global1", "global2", "handler"}
		if !slice_equal(order, expected) {
			t.Errorf("Expected order %v, got %v", expected, order)
		}
	})

	t.Run("Method_Level_Middleware", func(t *testing.T) {
		r := NewRouter()
		var order []string
		UseMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(
				func(w http.ResponseWriter, req *http.Request) {
					order = append(order, "global")
					next.ServeHTTP(w, req)
				},
			)
		})
		UseMiddlewareByMethod(
			r,
			http.MethodGet,
			func(next http.Handler) http.Handler {
				return http.HandlerFunc(
					func(w http.ResponseWriter, req *http.Request) {
						order = append(order, "method")
						next.ServeHTTP(w, req)
					},
				)
			},
		)
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, req *http.Request) {
				order = append(order, "handler")
			},
		)

		r.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/test", nil),
		)
		expected := []string{"global", "method", "handler"}
		if !slice_equal(order, expected) {
			t.Errorf("Expected order %v, got %v", expected, order)
		}
	})

	t.Run("Pattern_Level_Middleware", func(t *testing.T) {
		r := NewRouter()
		var order []string
		route := AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, req *http.Request) {
				order = append(order, "handler")
			},
		)
		UseMiddlewareByPattern(
			route,
			func(next http.Handler) http.Handler {
				return http.HandlerFunc(
					func(w http.ResponseWriter, req *http.Request) {
						order = append(order, "pattern")
						next.ServeHTTP(w, req)
					},
				)
			},
		)

		r.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/test", nil),
		)
		if order[0] != "pattern" || order[1] != "handler" {
			t.Errorf("Expected [pattern handler], got %v", order)
		}
	})

	t.Run("Middleware_With_If", func(t *testing.T) {
		r := NewRouter()
		var mw_called bool
		UseMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(
				func(w http.ResponseWriter, req *http.Request) {
					mw_called = true
					next.ServeHTTP(w, req)
				},
			)
		}, &MiddlewareOptions{
			If: func(req *http.Request) bool { return !strings.HasPrefix(req.URL.Path, "/public/") },
		})
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/public/assets",
			func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(http.StatusOK) },
		)
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/api/data",
			func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(http.StatusOK) },
		)

		mw_called = false
		r.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/public/assets", nil),
		)
		if mw_called {
			t.Error("Middleware should not run for /public/ paths")
		}

		mw_called = false
		r.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/api/data", nil),
		)
		if !mw_called {
			t.Error("Middleware should run for non-public paths")
		}
	})

	t.Run("Middleware_Short_Circuit", func(t *testing.T) {
		r := NewRouter()
		var handler_called bool
		UseMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(
				func(w http.ResponseWriter, req *http.Request) {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
				},
			)
		})
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, req *http.Request) { handler_called = true },
		)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
		if handler_called {
			t.Error(
				"Handler should not be called when middleware short-circuits",
			)
		}
	})
}

func TestTaskMiddleware(t *testing.T) {
	type auth_info struct{ UserID string }

	t.Run("Basic", func(t *testing.T) {
		r := NewRouter()
		var mw_called bool
		UseTaskMiddleware(
			r,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (auth_info, error) {
				mw_called = true
				return auth_info{UserID: "123"}, nil
			}),
		)
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(http.StatusOK) },
		)

		r.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/test", nil),
		)
		if !mw_called {
			t.Error("Task middleware was not called")
		}
	})

	t.Run("With_If", func(t *testing.T) {
		r := NewRouter()
		var mw_called bool
		UseTaskMiddleware(
			r,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				mw_called = true
				return None{}, nil
			}),
			&MiddlewareOptions{
				If: func(req *http.Request) bool { return strings.HasPrefix(req.URL.Path, "/api/") },
			},
		)
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/api/test",
			func(w http.ResponseWriter, req *http.Request) {},
		)
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/public/test",
			func(w http.ResponseWriter, req *http.Request) {},
		)

		mw_called = false
		r.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/api/test", nil),
		)
		if !mw_called {
			t.Error("Task middleware should run for /api/ paths")
		}

		mw_called = false
		r.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/public/test", nil),
		)
		if mw_called {
			t.Error("Task middleware should not run for /public/ paths")
		}
	})
}

func TestNotFound(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		r := NewRouter()
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/exists",
			func(w http.ResponseWriter, req *http.Request) {},
		)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/notfound", nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Custom", func(t *testing.T) {
		r := NewRouter()
		r.SetGlobalNotFoundHTTPHandler(
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("Custom 404"))
			}),
		)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/notfound", nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
		if body := w.Body.String(); body != "Custom 404" {
			t.Errorf("Expected 'Custom 404', got %q", body)
		}
	})
}

func TestMountRoot(t *testing.T) {
	t.Run("Strip_MountRoot", func(t *testing.T) {
		r := NewRouter(Options{MountRoot: "/api"})
		var called bool
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/users",
			func(w http.ResponseWriter, req *http.Request) { called = true },
		)

		r.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/api/users", nil),
		)
		if !called {
			t.Error("Handler should be called when mount root is stripped")
		}
	})

	t.Run("MountRoot_Method", func(t *testing.T) {
		r := NewRouter(Options{MountRoot: "/api"})
		if r.MountRoot() != "/api/" {
			t.Errorf("Expected '/api/', got %q", r.MountRoot())
		}
		if r.MountRoot("users") != "/api/users" {
			t.Errorf("Expected '/api/users', got %q", r.MountRoot("users"))
		}
	})

	t.Run("MountRoot_Panics_On_More_Than_One_Arg", func(t *testing.T) {
		r := NewRouter(Options{MountRoot: "/api"})
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic")
			}
		}()
		_ = r.MountRoot("users", "extra")
	})
}

func TestValidation(t *testing.T) {
	type validated_input struct {
		Email string `json:"email"`
	}

	t.Run("Validation_Error", func(t *testing.T) {
		r := NewRouter(Options{
			ParseInput: func(req *http.Request, input_ptr any) error {
				return &schema.ValidationError{
					Err: errors.New("Invalid email format"),
				}
			},
		})
		AddTaskHandler(
			r,
			http.MethodPost,
			"/validate",
			TaskHandlerFromFunc(
				func(rd *RequestCtx[validated_input]) (None, error) { return None{}, nil },
			),
		)

		body := strings.NewReader(`{"email":"invalid"}`)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/validate", body))

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Invalid email format") {
			t.Error("Expected validation error message in response")
		}
	})
}

func TestAllRoutes(t *testing.T) {
	r := NewRouter()
	AddHTTPHandlerFunc(
		r,
		http.MethodGet,
		"/users",
		func(w http.ResponseWriter, req *http.Request) {},
	)
	AddHTTPHandlerFunc(
		r,
		http.MethodPost,
		"/users",
		func(w http.ResponseWriter, req *http.Request) {},
	)
	AddHTTPHandlerFunc(
		r,
		http.MethodGet,
		"/posts",
		func(w http.ResponseWriter, req *http.Request) {},
	)

	routes := r.AllRoutes()
	if len(routes) != 3 {
		t.Errorf("Expected 3 routes, got %d", len(routes))
	}

	patterns := make(map[string]map[string]bool)
	for _, route := range routes {
		p := route.OriginalPattern()
		m := route.Method()
		if patterns[p] == nil {
			patterns[p] = make(map[string]bool)
		}
		patterns[p][m] = true
	}
	if !patterns["/users"][http.MethodGet] {
		t.Error("Missing GET /users")
	}
	if !patterns["/users"][http.MethodPost] {
		t.Error("Missing POST /users")
	}
	if !patterns["/posts"][http.MethodGet] {
		t.Error("Missing GET /posts")
	}

	routes[0] = nil
	after := r.AllRoutes()
	if len(after) == 0 || after[0] == nil {
		t.Fatal("AllRoutes should return a defensive copy")
	}
}

func TestTasksCacheAvailability(t *testing.T) {
	t.Run("InHTTPHandler_WithTaskMiddleware", func(t *testing.T) {
		router := NewRouter()
		UseTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(
				func(rd *RequestCtx[None]) (None, error) { return None{}, nil },
			),
		)
		AddHTTPHandlerFunc(
			router,
			"GET",
			"/test",
			func(w http.ResponseWriter, r *http.Request) {
				if GetTasksCache(r) == nil {
					t.Error("TasksCache is nil in HTTP handler")
					http.Error(w, "nil", http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
		)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("GET", "/test", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})

	t.Run("InTaskMiddleware", func(t *testing.T) {
		router := NewRouter()
		UseTaskMiddleware(
			router,
			TaskMiddlewareFromFunc(func(rd *RequestCtx[None]) (None, error) {
				if rd.TasksCache() == nil {
					t.Error("TasksCache is nil in task middleware")
				}
				return None{}, nil
			}),
		)
		AddHTTPHandlerFunc(
			router,
			"GET",
			"/test",
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) },
		)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("GET", "/test", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func slice_equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

/////////////////////////////////////////////////////////////////////
/////// BENCHMARKS
/////////////////////////////////////////////////////////////////////

func setup_api_router() *Router {
	r := NewRouter()
	logging := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.Method + " " + r.URL.Path
			next.ServeHTTP(w, r)
		})
	}
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				next.ServeHTTP(w, r)
			}
			next.ServeHTTP(w, r)
		})
	}
	UseMiddleware(r, logging)
	UseMiddleware(r, auth)

	ok := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	AddHTTPHandlerFunc(r, http.MethodGet, "/api/users", ok)
	AddHTTPHandlerFunc(r, http.MethodGet, "/api/users/:id", ok)
	AddHTTPHandlerFunc(
		r,
		http.MethodPost,
		"/api/users",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) },
	)
	AddHTTPHandlerFunc(r, http.MethodPut, "/api/users/:id", ok)
	AddHTTPHandlerFunc(
		r,
		http.MethodDelete,
		"/api/users/:id",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
	)
	AddHTTPHandlerFunc(r, http.MethodGet, "/api/users/:userId/posts", ok)
	AddHTTPHandlerFunc(
		r,
		http.MethodGet,
		"/api/users/:userId/posts/:postId",
		ok,
	)
	return r
}

func setup_large_router(n int) *Router {
	r := NewRouter()
	ok := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	for i := range n {
		path := fmt.Sprintf("/route%d", i)
		if i%2 == 0 {
			AddHTTPHandlerFunc(r, http.MethodGet, "/static/path/"+path, ok)
		} else {
			AddHTTPHandlerFunc(r, http.MethodGet, "/dynamic/:param/"+path, ok)
		}
	}
	return r
}

func BenchmarkRouter(b *testing.B) {
	b.Run("SimpleStaticRoute", func(b *testing.B) {
		r := NewRouter()
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/ping",
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) },
		)
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("DynamicRoute", func(b *testing.B) {
		r := NewRouter()
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/users/:id",
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) },
		)
		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		w := httptest.NewRecorder()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("WithMiddleware", func(b *testing.B) {
		r := NewRouter()
		UseMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r) },
			)
		})
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/test",
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) },
		)
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("RESTfulAPI", func(b *testing.B) {
		r := setup_api_router()
		paths := []string{
			"/api/users",
			"/api/users/123",
			"/api/users/456/posts",
			"/api/users/789/posts/999",
		}
		reqs := make([]*http.Request, len(paths))
		for i, p := range paths {
			reqs[i] = httptest.NewRequest(http.MethodGet, p, nil)
		}
		w := httptest.NewRecorder()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, reqs[i%len(reqs)])
		}
	})

	b.Run("LargeRouterMatch", func(b *testing.B) {
		r := setup_large_router(100)
		req := httptest.NewRequest(http.MethodGet, "/dynamic/param/99", nil)
		w := httptest.NewRecorder()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("WorstCaseMatch", func(b *testing.B) {
		r := setup_large_router(100)
		req := httptest.NewRequest(http.MethodGet, "/nomatch", nil)
		w := httptest.NewRecorder()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("NestedDynamicRoute", func(b *testing.B) {
		r := NewRouter()
		AddHTTPHandlerFunc(
			r,
			http.MethodGet,
			"/api/:version/users/:userId/posts/:postId/comments/:commentId",
			func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) },
		)
		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v1/users/123/posts/456/comments/789",
			nil,
		)
		w := httptest.NewRecorder()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("TaskHandler", func(b *testing.B) {
		r := NewRouter(Options{
			ParseInput: func(req *http.Request, input_ptr any) error {
				if req.Body != nil {
					defer req.Body.Close()
					return json.NewDecoder(req.Body).Decode(input_ptr)
				}
				return nil
			},
		})
		type input struct {
			Value int `json:"value"`
		}
		type output struct {
			Result int `json:"result"`
		}
		AddTaskHandler(r, http.MethodPost, "/double",
			TaskHandlerFromFunc(func(rd *RequestCtx[input]) (output, error) {
				return output{Result: rd.Input().Value * 2}, nil
			}))
		req := httptest.NewRequest(
			http.MethodPost,
			"/double",
			strings.NewReader(`{"value":42}`),
		)
		w := httptest.NewRecorder()
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req.Body = io.NopCloser(strings.NewReader(`{"value":42}`))
			r.ServeHTTP(w, req)
		}
	})
}
