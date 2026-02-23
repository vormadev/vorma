package mux

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/validate"
)

func TestRouterBasics(t *testing.T) {
	t.Run("NewRouter_Defaults", func(t *testing.T) {
		r := NewRouter(nil)
		if r.DynamicParamPrefix() != ':' {
			t.Error("Default dynamic param prefix should be ':'")
		}
		if r.SplatSegmentIdentifier() != '*' {
			t.Error("Default splat segment should be '*'")
		}
	})

	t.Run("NewRouter_WithOptions", func(t *testing.T) {
		opts := &Options{
			DynamicParamPrefix:     '@',
			SplatSegmentIdentifier: '#',
			MountRoot:              "/api",
		}
		r := NewRouter(opts)

		if r.DynamicParamPrefix() != '@' {
			t.Error("DynamicParamPrefix not set correctly")
		}
		if r.SplatSegmentIdentifier() != '#' {
			t.Error("SplatSegmentIdentifier not set correctly")
		}
		if r.MountRoot() != "/api/" {
			t.Errorf("MountRoot not normalized correctly, got %q", r.MountRoot())
		}
	})

	t.Run("MountRoot_Normalization", func(t *testing.T) {
		testCases := []struct {
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

		for _, tc := range testCases {
			r := NewRouter(&Options{MountRoot: tc.input})
			if got := r.MountRoot(); got != tc.expected {
				t.Errorf("MountRoot(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		}
	})
}

func TestHTTPHandlers(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace,
	}

	for _, method := range methods {
		t.Run("Method_"+method, func(t *testing.T) {
			r := NewRouter(nil)
			called := false

			AddHTTPHandlerFunc(r, method, "/test", func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

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
		r := NewRouter(nil)

		// Only register GET handler
		AddHTTPHandlerFunc(r, http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom", "value")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("body content"))
		})

		// Make HEAD request
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

	t.Run("HEAD_Fallback_To_GET_PreservesNoBodyOnValidationError", func(t *testing.T) {
		type parseInput struct {
			Name string `json:"name"`
		}

		r := NewRouter(&Options{
			ParseInput: func(req *http.Request, inputPtr any) error {
				return &validate.ValidationError{
					Err: errors.New("invalid input"),
				}
			},
		})
		AddTaskHandler(
			r,
			http.MethodGet,
			"/validate",
			TaskHandlerFromFunc(func(rd *ReqData[parseInput]) (map[string]string, error) {
				return map[string]string{"ok": "true"}, nil
			}),
		)

		req := httptest.NewRequest(http.MethodHead, "/validate", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
		if w.Body.Len() > 0 {
			t.Fatalf("expected empty body for HEAD fallback validation error, got %q", w.Body.String())
		}
	})
}

func TestTaskHandlers(t *testing.T) {
	type TestInput struct {
		Name string `json:"name"`
	}
	type TestOutput struct {
		Message string `json:"message"`
	}

	t.Run("Basic_TaskHandler", func(t *testing.T) {
		r := NewRouter(&Options{
			ParseInput: func(req *http.Request, inputPtr any) error {
				return json.NewDecoder(req.Body).Decode(inputPtr)
			},
		})

		handler := TaskHandlerFromFunc(func(rd *ReqData[TestInput]) (TestOutput, error) {
			return TestOutput{Message: "Hello " + rd.Input().Name}, nil
		})

		AddTaskHandler(r, http.MethodPost, "/greet", handler)

		body := strings.NewReader(`{"name":"World"}`)
		req := httptest.NewRequest(http.MethodPost, "/greet", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp TestOutput
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		if resp.Message != "Hello World" {
			t.Errorf("Expected 'Hello World', got %q", resp.Message)
		}
	})

	t.Run("TaskHandler_With_None_Input", func(t *testing.T) {
		r := NewRouter(nil)

		handler := TaskHandlerFromFunc(func(rd *ReqData[None]) (TestOutput, error) {
			return TestOutput{Message: "No input needed"}, nil
		})

		AddTaskHandler(r, http.MethodGet, "/status", handler)

		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp TestOutput
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Message != "No input needed" {
			t.Errorf("Expected 'No input needed', got %q", resp.Message)
		}
	})
}

func TestGetParams(t *testing.T) {
	t.Run("Dynamic_Params", func(t *testing.T) {
		r := NewRouter(nil)
		var capturedParams Params

		AddHTTPHandlerFunc(r, http.MethodGet, "/users/:id", func(w http.ResponseWriter, req *http.Request) {
			capturedParams = GetParams(req)
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if id := capturedParams["id"]; id != "123" {
			t.Errorf("Expected param id='123', got %q", id)
		}
	})

	t.Run("Multiple_Params", func(t *testing.T) {
		r := NewRouter(nil)
		var capturedParams Params

		AddHTTPHandlerFunc(r, http.MethodGet, "/users/:userId/posts/:postId",
			func(w http.ResponseWriter, req *http.Request) {
				capturedParams = GetParams(req)
				w.WriteHeader(http.StatusOK)
			})

		req := httptest.NewRequest(http.MethodGet, "/users/456/posts/789", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if userId := capturedParams["userId"]; userId != "456" {
			t.Errorf("Expected userId='456', got %q", userId)
		}
		if postId := capturedParams["postId"]; postId != "789" {
			t.Errorf("Expected postId='789', got %q", postId)
		}
	})

	t.Run("Splat_Values", func(t *testing.T) {
		r := NewRouter(nil)
		var capturedSplat []string

		AddHTTPHandlerFunc(r, http.MethodGet, "/files/*", func(w http.ResponseWriter, req *http.Request) {
			capturedSplat = GetSplatValues(req)
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/files/path/to/file.txt", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		expected := []string{"path", "to", "file.txt"}
		if !sliceEqual(capturedSplat, expected) {
			t.Errorf("Expected splat values %v, got %v", expected, capturedSplat)
		}
	})
}

func TestParams_NoParamsReturnsNilMap(t *testing.T) {
	router := NewRouter(nil)

	requestCount := 0
	var firstRequestParams Params
	var secondRequestParams Params

	AddHTTPHandlerFunc(
		router,
		http.MethodGet,
		"/no-params",
		func(w http.ResponseWriter, req *http.Request) {
			requestCount++

			params := GetParams(req)
			if requestCount == 1 {
				firstRequestParams = params
			}
			if requestCount == 2 {
				secondRequestParams = params
			}

			w.WriteHeader(http.StatusOK)
		},
	)

	firstRequest := httptest.NewRequest(http.MethodGet, "/no-params", nil)
	firstResponseRecorder := httptest.NewRecorder()
	router.ServeHTTP(firstResponseRecorder, firstRequest)

	secondRequest := httptest.NewRequest(http.MethodGet, "/no-params", nil)
	secondResponseRecorder := httptest.NewRecorder()
	router.ServeHTTP(secondResponseRecorder, secondRequest)

	if firstRequestParams != nil {
		t.Fatalf(
			"expected first request no-params map to be nil, got %#v",
			firstRequestParams,
		)
	}
	if secondRequestParams != nil {
		t.Fatalf(
			"expected second request no-params map to be nil, got %#v",
			secondRequestParams,
		)
	}
}

func TestInjectTasksCtxMiddleware_NoParamsReturnsNilMap(t *testing.T) {
	requestCount := 0
	var firstRequestParams Params
	var secondRequestParams Params

	handler := InjectTasksCtxMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			requestCount++

			params := GetParams(req)
			if requestCount == 1 {
				firstRequestParams = params
			}
			if requestCount == 2 {
				secondRequestParams = params
			}

			w.WriteHeader(http.StatusNoContent)
		}),
	)

	firstRequest := httptest.NewRequest(http.MethodGet, "/middleware-no-params", nil)
	firstResponseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(firstResponseRecorder, firstRequest)

	secondRequest := httptest.NewRequest(http.MethodGet, "/middleware-no-params", nil)
	secondResponseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondResponseRecorder, secondRequest)

	if firstRequestParams != nil {
		t.Fatalf(
			"expected first middleware request no-params map to be nil, got %#v",
			firstRequestParams,
		)
	}
	if secondRequestParams != nil {
		t.Fatalf(
			"expected second middleware request no-params map to be nil, got %#v",
			secondRequestParams,
		)
	}
}

func TestHTTPMiddleware(t *testing.T) {
	t.Run("Global_Middleware_Order", func(t *testing.T) {
		r := NewRouter(nil)
		var order []string

		AddGlobalHTTPMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				order = append(order, "global1")
				next.ServeHTTP(w, req)
			})
		})

		AddGlobalHTTPMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				order = append(order, "global2")
				next.ServeHTTP(w, req)
			})
		})

		AddHTTPHandlerFunc(r, http.MethodGet, "/test", func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "handler")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		expected := []string{"global1", "global2", "handler"}
		if !sliceEqual(order, expected) {
			t.Errorf("Wrong execution order. Expected %v, got %v", expected, order)
		}
	})

	t.Run("Method_Level_Middleware", func(t *testing.T) {
		r := NewRouter(nil)
		var order []string

		AddGlobalHTTPMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				order = append(order, "global")
				next.ServeHTTP(w, req)
			})
		})

		AddMethodLevelHTTPMiddleware(r, http.MethodGet, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				order = append(order, "method")
				next.ServeHTTP(w, req)
			})
		})

		AddHTTPHandlerFunc(r, http.MethodGet, "/test", func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "handler")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		expected := []string{"global", "method", "handler"}
		if !sliceEqual(order, expected) {
			t.Errorf("Wrong execution order. Expected %v, got %v", expected, order)
		}
	})

	t.Run("Pattern_Level_Middleware", func(t *testing.T) {
		r := NewRouter(nil)
		var order []string

		route := AddHTTPHandlerFunc(r, http.MethodGet, "/test", func(w http.ResponseWriter, req *http.Request) {
			order = append(order, "handler")
		})

		AddPatternLevelHTTPMiddleware(route, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				order = append(order, "pattern")
				next.ServeHTTP(w, req)
			})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if order[0] != "pattern" || order[1] != "handler" {
			t.Errorf("Wrong execution order. Expected [pattern handler], got %v", order)
		}
	})

	t.Run("Middleware_With_If", func(t *testing.T) {
		r := NewRouter(nil)
		var middlewareCalled bool

		AddGlobalHTTPMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				middlewareCalled = true
				next.ServeHTTP(w, req)
			})
		}, &MiddlewareOptions{
			If: func(req *http.Request) bool {
				return !strings.HasPrefix(req.URL.Path, "/public/")
			},
		})

		AddHTTPHandlerFunc(r, http.MethodGet, "/public/assets", func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		AddHTTPHandlerFunc(r, http.MethodGet, "/api/data", func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Test public path (middleware should be skipped)
		middlewareCalled = false
		req := httptest.NewRequest(http.MethodGet, "/public/assets", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if middlewareCalled {
			t.Error("Middleware should not run for /public/ paths")
		}

		// Test API path (middleware should run)
		middlewareCalled = false
		req = httptest.NewRequest(http.MethodGet, "/api/data", nil)
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !middlewareCalled {
			t.Error("Middleware should run for non-public paths")
		}
	})

	t.Run("Middleware_Short_Circuit", func(t *testing.T) {
		r := NewRouter(nil)
		var handlerCalled bool

		AddGlobalHTTPMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				// Don't call next
			})
		})

		AddHTTPHandlerFunc(r, http.MethodGet, "/test", func(w http.ResponseWriter, req *http.Request) {
			handlerCalled = true
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
		if handlerCalled {
			t.Error("Handler should not be called when middleware short-circuits")
		}
	})
}

func TestTaskMiddleware(t *testing.T) {
	type AuthInfo struct {
		UserID string
	}

	t.Run("Task_Middleware_Basic", func(t *testing.T) {
		r := NewRouter(nil)
		var middlewareCalled bool

		taskMw := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (AuthInfo, error) {
			middlewareCalled = true
			return AuthInfo{UserID: "123"}, nil
		})

		AddGlobalTaskMiddleware(r, taskMw)

		AddHTTPHandlerFunc(r, http.MethodGet, "/test", func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !middlewareCalled {
			t.Error("Task middleware was not called")
		}
	})

	t.Run("Task_Middleware_With_If", func(t *testing.T) {
		r := NewRouter(nil)
		var middlewareCalled bool

		taskMw := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
			middlewareCalled = true
			return None{}, nil
		})

		AddGlobalTaskMiddleware(r, taskMw, &MiddlewareOptions{
			If: func(req *http.Request) bool {
				return strings.HasPrefix(req.URL.Path, "/api/")
			},
		})

		AddHTTPHandlerFunc(r, http.MethodGet, "/api/test", func(w http.ResponseWriter, req *http.Request) {})
		AddHTTPHandlerFunc(r, http.MethodGet, "/public/test", func(w http.ResponseWriter, req *http.Request) {})

		// Test API path
		middlewareCalled = false
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if !middlewareCalled {
			t.Error("Task middleware should run for /api/ paths")
		}

		// Test public path
		middlewareCalled = false
		req = httptest.NewRequest(http.MethodGet, "/public/test", nil)
		r.ServeHTTP(httptest.NewRecorder(), req)
		if middlewareCalled {
			t.Error("Task middleware should not run for /public/ paths")
		}
	})
}

func TestNotFound(t *testing.T) {
	t.Run("Default_NotFound", func(t *testing.T) {
		r := NewRouter(nil)
		AddHTTPHandlerFunc(r, http.MethodGet, "/exists", func(w http.ResponseWriter, req *http.Request) {})

		req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Custom_NotFound", func(t *testing.T) {
		r := NewRouter(nil)
		SetGlobalNotFoundHTTPHandler(r, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Custom 404"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

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
		r := NewRouter(&Options{MountRoot: "/api"})
		var called bool

		AddHTTPHandlerFunc(r, http.MethodGet, "/users", func(w http.ResponseWriter, req *http.Request) {
			called = true
		})

		// Request with mount root prefix
		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if !called {
			t.Error("Handler should be called when mount root is stripped")
		}
	})

	t.Run("MountRoot_Method", func(t *testing.T) {
		r := NewRouter(&Options{MountRoot: "/api"})

		if r.MountRoot() != "/api/" {
			t.Errorf("Expected '/api/', got %q", r.MountRoot())
		}
		if r.MountRoot("users") != "/api/users" {
			t.Errorf("Expected '/api/users', got %q", r.MountRoot("users"))
		}
	})

	t.Run("MountRoot_Panics_On_More_Than_One_Arg", func(t *testing.T) {
		r := NewRouter(&Options{MountRoot: "/api"})
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic when MountRoot receives more than one argument")
			}
		}()
		_ = r.MountRoot("users", "extra")
	})
}

func TestValidation(t *testing.T) {
	type ValidatedInput struct {
		Email string `json:"email"`
	}

	t.Run("Validation_Error", func(t *testing.T) {
		r := NewRouter(&Options{
			ParseInput: func(req *http.Request, inputPtr any) error {
				// Simulate validation error
				return &validate.ValidationError{Err: errors.New("Invalid email format")}
			},
		})

		handler := TaskHandlerFromFunc(func(rd *ReqData[ValidatedInput]) (None, error) {
			return None{}, nil
		})

		AddTaskHandler(r, http.MethodPost, "/validate", handler)

		body := strings.NewReader(`{"email":"invalid"}`)
		req := httptest.NewRequest(http.MethodPost, "/validate", body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), "Invalid email format") {
			t.Error("Expected validation error message in response")
		}
	})
}

func TestAllRoutes(t *testing.T) {
	r := NewRouter(nil)

	AddHTTPHandlerFunc(r, http.MethodGet, "/users", func(w http.ResponseWriter, req *http.Request) {})
	AddHTTPHandlerFunc(r, http.MethodPost, "/users", func(w http.ResponseWriter, req *http.Request) {})
	AddHTTPHandlerFunc(r, http.MethodGet, "/posts", func(w http.ResponseWriter, req *http.Request) {})

	routes := r.AllRoutes()
	if len(routes) != 3 {
		t.Errorf("Expected 3 routes, got %d", len(routes))
	}

	// Verify routes have correct patterns and methods
	patterns := make(map[string]map[string]bool)
	for _, route := range routes {
		pattern := route.OriginalPattern()
		method := route.Method()
		if patterns[pattern] == nil {
			patterns[pattern] = make(map[string]bool)
		}
		patterns[pattern][method] = true
	}

	if !patterns["/users"][http.MethodGet] {
		t.Error("Missing GET /users route")
	}
	if !patterns["/users"][http.MethodPost] {
		t.Error("Missing POST /users route")
	}
	if !patterns["/posts"][http.MethodGet] {
		t.Error("Missing GET /posts route")
	}

	// Mutating the returned slice must not mutate router-owned route storage.
	routes[0] = nil
	routesAfterMutation := r.AllRoutes()
	if len(routesAfterMutation) == 0 {
		t.Fatal("expected non-empty routes after mutation attempt")
	}
	if routesAfterMutation[0] == nil {
		t.Fatal("AllRoutes should return a defensive copy")
	}
}

// Helper functions
func sliceEqual(a, b []string) bool {
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

// Benchmarks
func BenchmarkRouter(b *testing.B) {
	b.Run("SimpleStaticRoute", func(b *testing.B) {
		r := NewRouter(nil)
		AddHTTPHandlerFunc(r, http.MethodGet, "/ping", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("DynamicRoute", func(b *testing.B) {
		r := NewRouter(nil)
		AddHTTPHandlerFunc(r, http.MethodGet, "/users/:id", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("WithMiddleware", func(b *testing.B) {
		r := NewRouter(nil)
		AddGlobalHTTPMiddleware(r, func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		})
		AddHTTPHandlerFunc(r, http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("RESTfulAPI", func(b *testing.B) {
		r := setupAPIRouterForBenchmarks()
		paths := []string{
			"/api/users",
			"/api/users/123",
			"/api/users/456/posts",
			"/api/users/789/posts/999",
		}

		reqs := make([]*http.Request, len(paths))
		for i, path := range paths {
			reqs[i] = httptest.NewRequest(http.MethodGet, path, nil)
		}
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, reqs[i%len(reqs)])
		}
	})

	b.Run("LargeRouterMatch", func(b *testing.B) {
		r := setupLargeRouterForBenchmarks(100)
		req := httptest.NewRequest(http.MethodGet, "/dynamic/param/99", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("WorstCaseMatch", func(b *testing.B) {
		r := setupLargeRouterForBenchmarks(100)
		req := httptest.NewRequest(http.MethodGet, "/nomatch", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("NestedDynamicRoute", func(b *testing.B) {
		r := NewRouter(nil)
		AddHTTPHandlerFunc(r, http.MethodGet, "/api/:version/users/:userId/posts/:postId/comments/:commentId",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123/posts/456/comments/789", nil)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			r.ServeHTTP(w, req)
		}
	})

	b.Run("TaskHandler", func(b *testing.B) {
		r := NewRouter(&Options{
			ParseInput: func(req *http.Request, inputPtr any) error {
				if req.Body != nil {
					defer req.Body.Close()
					return json.NewDecoder(req.Body).Decode(inputPtr)
				}
				return nil
			},
		})

		type Input struct {
			Value int `json:"value"`
		}
		type Output struct {
			Result int `json:"result"`
		}

		handler := TaskHandlerFromFunc(func(rd *ReqData[Input]) (Output, error) {
			return Output{Result: rd.Input().Value * 2}, nil
		})

		AddTaskHandler(r, http.MethodPost, "/double", handler)

		body := strings.NewReader(`{"value":42}`)
		req := httptest.NewRequest(http.MethodPost, "/double", body)
		w := httptest.NewRecorder()

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			req.Body = io.NopCloser(strings.NewReader(`{"value":42}`))
			r.ServeHTTP(w, req)
		}
	})
}

// Helper to create a typical API router setup
func setupAPIRouterForBenchmarks() *Router {
	r := NewRouter(nil)

	// Common middleware
	loggingMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate basic logging overhead
			_ = r.Method + " " + r.URL.Path
			next.ServeHTTP(w, r)
		})
	}

	authMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Simulate auth check overhead
			if r.Header.Get("Authorization") == "" {
				next.ServeHTTP(w, r)
			}
			next.ServeHTTP(w, r)
		})
	}

	// Global middleware
	AddGlobalHTTPMiddleware(r, loggingMW)
	AddGlobalHTTPMiddleware(r, authMW)

	// REST-style routes
	AddHTTPHandlerFunc(r, http.MethodGet, "/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	AddHTTPHandlerFunc(r, http.MethodGet, "/api/users/:id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	AddHTTPHandlerFunc(r, http.MethodPost, "/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	AddHTTPHandlerFunc(r, http.MethodPut, "/api/users/:id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	AddHTTPHandlerFunc(r, http.MethodDelete, "/api/users/:id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Nested resources
	AddHTTPHandlerFunc(r, http.MethodGet, "/api/users/:userId/posts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	AddHTTPHandlerFunc(r, http.MethodGet, "/api/users/:userId/posts/:postId", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return r
}

// Helper to create a router with many routes
func setupLargeRouterForBenchmarks(numRoutes int) *Router {
	r := NewRouter(nil)

	// Add a mix of static and dynamic routes
	for i := 0; i < numRoutes; i++ {
		path := fmt.Sprintf("/route%d", i)
		if i%2 == 0 {
			AddHTTPHandlerFunc(r, http.MethodGet, "/static/path/"+path, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		} else {
			AddHTTPHandlerFunc(r, http.MethodGet, "/dynamic/:param/"+path, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		}
	}

	return r
}

func TestTasksCtxIsAvailableInHTTPHandler(t *testing.T) {
	router := NewRouter(nil)

	// Add a task middleware to force TasksCtx creation
	taskMw := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
		return None{}, nil
	})
	AddGlobalTaskMiddleware(router, taskMw)

	// Register an HTTP handler that tries to access TasksCtx
	AddHTTPHandlerFunc(router, "GET", "/test", func(w http.ResponseWriter, r *http.Request) {
		// This should work because we have task middleware
		tasksCtx := GetTasksCtx(r)
		if tasksCtx == nil {
			t.Error("TasksCtx is nil in HTTP handler")
			http.Error(w, "TasksCtx is nil", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Make a test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestTasksCtxIsAvailableInTaskMiddleware(t *testing.T) {
	router := NewRouter(nil)

	// Create a task middleware that verifies TasksCtx is available
	taskMw := TaskMiddlewareFromFunc(func(rd *ReqData[None]) (None, error) {
		if rd.TasksCtx() == nil {
			t.Error("TasksCtx is nil in task middleware")
		}
		return None{}, nil
	})

	AddGlobalTaskMiddleware(router, taskMw)

	// Register a simple HTTP handler
	AddHTTPHandlerFunc(router, "GET", "/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Make a test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}
