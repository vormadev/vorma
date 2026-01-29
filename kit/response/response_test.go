package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestResponse(t *testing.T) {
	tests := []struct {
		name           string
		method         func(r Response)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "OK JSON response",
			method:         func(r Response) { r.OK() },
			expectedStatus: http.StatusOK,
			expectedBody:   `{"ok":true}`,
		},
		{
			name:           "OK Text response",
			method:         func(r Response) { r.OKText() },
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "OK HTML response",
			method:         func(r Response) { r.HTML("<h1>Hello, World!</h1>") },
			expectedStatus: http.StatusOK,
			expectedBody:   "<h1>Hello, World!</h1>",
		},
		{
			name:           "Empty response",
			method:         func(r Response) {},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Text response",
			method:         func(r Response) { r.Text("Hello, World!") },
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello, World!",
		},
		{
			name:           "Not Modified response",
			method:         func(r Response) { r.NotModified() },
			expectedStatus: http.StatusNotModified,
		},
		{
			name:           "Not Found response",
			method:         func(r Response) { r.NotFound() },
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Unauthorized response with reason",
			method:         func(r Response) { r.Unauthorized("No token provided") },
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "No token provided\n",
		},
		{
			name:           "Unauthorized response without reason",
			method:         func(r Response) { r.Unauthorized() },
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized\n",
		},
		{
			name:           "Internal Server Error without reason",
			method:         func(r Response) { r.InternalServerError() },
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n",
		},
		{
			name:           "Internal Server Error with reason",
			method:         func(r Response) { r.InternalServerError("Database connection failed") },
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Database connection failed\n",
		},
		{
			name:           "BadRequest with one reason",
			method:         func(r Response) { r.BadRequest("Invalid input") },
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid input\n",
		},
		{
			name:           "BadRequest with multiple reasons",
			method:         func(r Response) { r.BadRequest("Invalid input", "Missing parameter") },
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid input Missing parameter\n",
		},
		{
			name:           "TooManyRequests with reason",
			method:         func(r Response) { r.TooManyRequests("Rate limit exceeded") },
			expectedStatus: http.StatusTooManyRequests,
			expectedBody:   "Rate limit exceeded\n",
		},
		{
			name:           "TooManyRequests without reason",
			method:         func(r Response) { r.TooManyRequests() },
			expectedStatus: http.StatusTooManyRequests,
			expectedBody:   "Too Many Requests\n",
		},
		{
			name:           "Forbidden with reason",
			method:         func(r Response) { r.Forbidden("Access denied") },
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Access denied\n",
		},
		{
			name:           "Forbidden without reason",
			method:         func(r Response) { r.Forbidden() },
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden\n",
		},
		{
			name:           "Empty JSON response",
			method:         func(r Response) { r.JSON(nil) },
			expectedStatus: http.StatusOK,
			expectedBody:   "null\n",
		},
		{
			name:           "SetStatus with body",
			method:         func(r Response) { r.SetStatus(http.StatusAccepted); r.Text("Request accepted") },
			expectedStatus: http.StatusAccepted,
			expectedBody:   "Request accepted",
		},
		{
			name:           "SetStatus with no body",
			method:         func(r Response) { r.SetStatus(http.StatusAccepted) },
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "Non-standard status code",
			method:         func(r Response) { r.SetStatus(499) },
			expectedStatus: 499,
		},
		{
			name:           "Error response with reason",
			method:         func(r Response) { r.Error(http.StatusNotFound, "Resource not found") },
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Resource not found\n",
		},
		{
			name:           "Error response without reason",
			method:         func(r Response) { r.Error(http.StatusNotFound) },
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Not Found\n",
		},
		{
			name:           "Error response with multiple reasons",
			method:         func(r Response) { r.Error(http.StatusNotFound, "Resource not found", "Invalid ID") },
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Resource not found Invalid ID\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			r := New(rr)
			test.method(r)

			if status := rr.Code; status != test.expectedStatus {
				t.Errorf("expected status %v, got %v", test.expectedStatus, status)
			}

			if test.expectedBody != "" {
				if rr.Header().Get("Content-Type") == "application/json" {
					compareJSON(t, test.expectedBody, rr.Body.String())
				} else {
					if rr.Body.String() != test.expectedBody {
						t.Errorf("expected body %v, got %v", test.expectedBody, rr.Body.String())
					}
				}
			}
		})
	}
}

func TestResponse_SetHeader(t *testing.T) {
	tests := []struct {
		name           string
		initialKey     string
		initialValue   string
		overwriteValue string
		expectedValue  string
	}{
		{
			name:          "Set initial header",
			initialKey:    "X-Custom-Header",
			initialValue:  "HeaderValue",
			expectedValue: "HeaderValue",
		},
		{
			name:           "Overwrite header",
			initialKey:     "X-Custom-Header",
			initialValue:   "HeaderValue",
			overwriteValue: "NewValue",
			expectedValue:  "NewValue",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rr := httptest.NewRecorder()

			r := New(rr)
			r.SetHeader(test.initialKey, test.initialValue)

			if test.overwriteValue != "" {
				r.SetHeader(test.initialKey, test.overwriteValue)
			}

			if header := rr.Header().Get(test.initialKey); header != test.expectedValue {
				t.Errorf("expected header '%s' to be '%s', got '%s'", test.initialKey, test.expectedValue, header)
			}
		})
	}
}

func compareJSON(t *testing.T, expected, actual string) {
	var expectedObj, actualObj map[string]any

	if err := json.Unmarshal([]byte(expected), &expectedObj); err != nil {
		t.Fatalf("failed to unmarshal expected JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(actual), &actualObj); err != nil {
		t.Fatalf("failed to unmarshal actual JSON: %v", err)
	}
	if !reflect.DeepEqual(expectedObj, actualObj) {
		t.Errorf("expected JSON %v, got %v", expectedObj, actualObj)
	}
}

func TestResponse_ContentType(t *testing.T) {
	t.Run("OK sets JSON content type", func(t *testing.T) {
		rr := httptest.NewRecorder()
		r := New(rr)
		r.OK()
		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %q", ct)
		}
	})

	t.Run("OKText sets text content type", func(t *testing.T) {
		rr := httptest.NewRecorder()
		r := New(rr)
		r.OKText()
		if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Errorf("Expected Content-Type 'text/plain', got %q", ct)
		}
	})
}

func TestResponse_ClientRedirect(t *testing.T) {
	t.Run("ClientRedirect sets header", func(t *testing.T) {
		rr := httptest.NewRecorder()
		r := New(rr)
		err := r.ClientRedirect("/somewhere")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if h := rr.Header().Get(ClientRedirectHeader); h != "/somewhere" {
			t.Errorf("Expected ClientRedirectHeader '/somewhere', got %q", h)
		}
	})
}
