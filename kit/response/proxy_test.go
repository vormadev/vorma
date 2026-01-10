package response

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxy_Status(t *testing.T) {
	t.Run("SetStatus_Basic", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(200)

		status, text := p.GetStatus()
		if status != 200 {
			t.Errorf("Expected status 200, got %d", status)
		}
		if text != "" {
			t.Errorf("Expected empty status text, got %q", text)
		}
	})

	t.Run("SetStatus_WithText", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(400, "Bad Request Custom")

		status, text := p.GetStatus()
		if status != 400 {
			t.Errorf("Expected status 400, got %d", status)
		}
		if text != "Bad Request Custom" {
			t.Errorf("Expected 'Bad Request Custom', got %q", text)
		}
	})

	t.Run("Status_Helpers", func(t *testing.T) {
		testCases := []struct {
			status     int
			isError    bool
			isRedirect bool
			isSuccess  bool
		}{
			{200, false, false, true},
			{201, false, false, true},
			{299, false, false, true},
			{301, false, true, false},
			{302, false, true, false},
			{399, false, true, false},
			{400, true, false, false},
			{404, true, false, false},
			{500, true, false, false},
		}

		for _, tc := range testCases {
			p := NewProxy()
			p.SetStatus(tc.status)
			p._location = "/somewhere" // For redirect test

			if p.IsError() != tc.isError {
				t.Errorf("Status %d: IsError() = %v, want %v", tc.status, p.IsError(), tc.isError)
			}
			if p.isServerRedirect() != tc.isRedirect {
				t.Errorf("Status %d: isServerRedirect() = %v, want %v", tc.status, p.isServerRedirect(), tc.isRedirect)
			}
			if p.IsSuccess() != tc.isSuccess {
				t.Errorf("Status %d: IsSuccess() = %v, want %v", tc.status, p.IsSuccess(), tc.isSuccess)
			}
		}
	})
}

func TestProxy_Headers(t *testing.T) {
	t.Run("SetHeader_Overwrites", func(t *testing.T) {
		p := NewProxy()
		p.SetHeader("X-Test", "value1")
		p.SetHeader("X-Test", "value2")

		if v := p.GetHeader("X-Test"); v != "value2" {
			t.Errorf("Expected 'value2', got %q", v)
		}

		if vals := p.GetHeaders("X-Test"); len(vals) != 1 || vals[0] != "value2" {
			t.Errorf("Expected ['value2'], got %v", vals)
		}
	})

	t.Run("AddHeader_Appends", func(t *testing.T) {
		p := NewProxy()
		p.AddHeader("X-Test", "value1")
		p.AddHeader("X-Test", "value2")
		p.AddHeader("X-Test", "value3")

		if v := p.GetHeader("X-Test"); v != "value1" {
			t.Errorf("GetHeader should return first value, got %q", v)
		}

		vals := p.GetHeaders("X-Test")
		if len(vals) != 3 {
			t.Errorf("Expected 3 values, got %d", len(vals))
		}
		expected := []string{"value1", "value2", "value3"}
		for i, v := range vals {
			if v != expected[i] {
				t.Errorf("Expected %q at index %d, got %q", expected[i], i, v)
			}
		}
	})

	t.Run("GetHeader_NonExistent", func(t *testing.T) {
		p := NewProxy()
		if v := p.GetHeader("X-Missing"); v != "" {
			t.Errorf("Expected empty string for missing header, got %q", v)
		}
		if vals := p.GetHeaders("X-Missing"); vals != nil {
			t.Errorf("Expected nil for missing headers, got %v", vals)
		}
	})
}

func TestProxy_Cookies(t *testing.T) {
	t.Run("SetCookie", func(t *testing.T) {
		p := NewProxy()

		cookie1 := &http.Cookie{Name: "session", Value: "abc123"}
		cookie2 := &http.Cookie{Name: "user", Value: "john"}

		p.SetCookie(cookie1)
		p.SetCookie(cookie2)

		cookies := p.GetCookies()
		if len(cookies) != 2 {
			t.Errorf("Expected 2 cookies, got %d", len(cookies))
		}

		// Cookies should be in order
		if cookies[0].Name != "session" {
			t.Errorf("Expected first cookie to be 'session', got %q", cookies[0].Name)
		}
		if cookies[1].Name != "user" {
			t.Errorf("Expected second cookie to be 'user', got %q", cookies[1].Name)
		}
	})
}

func TestProxy_Redirects(t *testing.T) {
	t.Run("ServerRedirect", func(t *testing.T) {
		p := NewProxy()
		p.serverRedirect("/login", 302)

		if !p.isServerRedirect() {
			t.Error("Expected isServerRedirect to be true")
		}
		if p.isClientRedirect() {
			t.Error("Expected isClientRedirect to be false")
		}
		if !p.IsRedirect() {
			t.Error("Expected IsRedirect to be true")
		}
		if p.GetLocation() != "/login" {
			t.Errorf("Expected location '/login', got %q", p.GetLocation())
		}
		status, _ := p.GetStatus()
		if status != 302 {
			t.Errorf("Expected status 302, got %d", status)
		}
	})

	t.Run("ClientRedirect", func(t *testing.T) {
		p := NewProxy()
		err := p.clientRedirect("https://example.com")

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !p.isClientRedirect() {
			t.Error("Expected isClientRedirect to be true")
		}
		if p.isServerRedirect() {
			t.Error("Expected isServerRedirect to be false")
		}
		if !p.IsRedirect() {
			t.Error("Expected IsRedirect to be true")
		}

		// Should set status to 200 if not already set
		status, _ := p.GetStatus()
		if status != 200 {
			t.Errorf("Expected status 200, got %d", status)
		}

		// Should have client redirect header
		if h := p.GetHeader(ClientRedirectHeader); h != "https://example.com" {
			t.Errorf("Expected client redirect header, got %q", h)
		}
	})

	t.Run("Redirect_Method", func(t *testing.T) {
		// Test without client redirect acceptance
		req := httptest.NewRequest("GET", "/", nil)
		p := NewProxy()

		upgraded, err := p.Redirect(req, "/login", 302)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if upgraded {
			t.Error("Should not upgrade to client redirect without proper header")
		}
		if !p.isServerRedirect() {
			t.Error("Should be server redirect")
		}

		// Test with client redirect acceptance (would need to mock doesAcceptClientRedirect)
		// This would require knowing what doesAcceptClientRedirect checks for
	})
}

func TestProxy_ApplyToResponseWriter(t *testing.T) {
	t.Run("Apply_Headers_And_Status", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(201)
		p.SetHeader("X-Custom", "value")
		p.AddHeader("X-Multi", "val1")
		p.AddHeader("X-Multi", "val2")

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		p.ApplyToResponseWriter(w, req)

		if w.Code != 201 {
			t.Errorf("Expected status 201, got %d", w.Code)
		}
		if v := w.Header().Get("X-Custom"); v != "value" {
			t.Errorf("Expected header 'value', got %q", v)
		}
		vals := w.Header().Values("X-Multi")
		if len(vals) != 2 {
			t.Errorf("Expected 2 values for X-Multi, got %d", len(vals))
		}
	})

	t.Run("Apply_Error_With_Text", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(404, "Page not found custom")

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		p.ApplyToResponseWriter(w, req)

		if w.Code != 404 {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
		if body := w.Body.String(); body != "Page not found custom\n" {
			t.Errorf("Expected custom error text, got %q", body)
		}
	})

	t.Run("Apply_Error_Without_Text", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(500)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		p.ApplyToResponseWriter(w, req)

		if w.Code != 500 {
			t.Errorf("Expected status 500, got %d", w.Code)
		}
		if body := w.Body.String(); body != "Internal Server Error\n" {
			t.Errorf("Expected default error text, got %q", body)
		}
	})

	t.Run("Apply_Cookies", func(t *testing.T) {
		p := NewProxy()
		p.SetCookie(&http.Cookie{Name: "session", Value: "abc"})
		p.SetCookie(&http.Cookie{Name: "user", Value: "john"})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		p.ApplyToResponseWriter(w, req)

		cookies := w.Result().Cookies()
		if len(cookies) != 2 {
			t.Errorf("Expected 2 cookies, got %d", len(cookies))
		}
	})

	t.Run("Apply_Server_Redirect", func(t *testing.T) {
		p := NewProxy()
		p.serverRedirect("/login", 302)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		p.ApplyToResponseWriter(w, req)

		if w.Code != 302 {
			t.Errorf("Expected status 302, got %d", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/login" {
			t.Errorf("Expected Location header '/login', got %q", loc)
		}
	})
}

func TestMergeProxyResponses(t *testing.T) {
	t.Run("Merge_First_Error_Wins", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetStatus(200)

		p2 := NewProxy()
		p2.SetStatus(403, "Forbidden")

		p3 := NewProxy()
		p3.SetStatus(401, "Unauthorized")

		merged := MergeProxyResponses(p1, p2, p3)

		status, text := merged.GetStatus()
		if status != 403 {
			t.Errorf("Expected first error (403) to win, got %d", status)
		}
		if text != "Forbidden" {
			t.Errorf("Expected 'Forbidden', got %q", text)
		}
	})

	t.Run("Merge_Last_Success_Wins", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetStatus(200)

		p2 := NewProxy()
		p2.SetStatus(201)

		p3 := NewProxy()
		p3.SetStatus(202)

		merged := MergeProxyResponses(p1, p2, p3)

		status, _ := merged.GetStatus()
		if status != 202 {
			t.Errorf("Expected last success (202) to win, got %d", status)
		}
	})

	t.Run("Merge_Headers_Combined", func(t *testing.T) {
		p1 := NewProxy()
		p1.AddHeader("X-Test", "val1")
		p1.SetHeader("X-Only-P1", "p1")

		p2 := NewProxy()
		p2.AddHeader("X-Test", "val2")
		p2.SetHeader("X-Only-P2", "p2")

		merged := MergeProxyResponses(p1, p2)

		// Headers should be merged in order
		vals := merged.GetHeaders("X-Test")
		if len(vals) != 2 || vals[0] != "val1" || vals[1] != "val2" {
			t.Errorf("Expected merged headers [val1, val2], got %v", vals)
		}

		if v := merged.GetHeader("X-Only-P1"); v != "p1" {
			t.Errorf("Expected 'p1', got %q", v)
		}
		if v := merged.GetHeader("X-Only-P2"); v != "p2" {
			t.Errorf("Expected 'p2', got %q", v)
		}
	})

	t.Run("Merge_Cookies_Later_Overwrites", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetCookie(&http.Cookie{Name: "session", Value: "old"})
		p1.SetCookie(&http.Cookie{Name: "user", Value: "john"})

		p2 := NewProxy()
		p2.SetCookie(&http.Cookie{Name: "session", Value: "new"})
		p2.SetCookie(&http.Cookie{Name: "theme", Value: "dark"})

		merged := MergeProxyResponses(p1, p2)

		cookies := merged.GetCookies()
		// Should have 3 unique cookies
		cookieMap := make(map[string]string)
		for _, c := range cookies {
			cookieMap[c.Name] = c.Value
		}

		if cookieMap["session"] != "new" {
			t.Errorf("Expected session cookie to be 'new', got %q", cookieMap["session"])
		}
		if cookieMap["user"] != "john" {
			t.Errorf("Expected user cookie to be 'john', got %q", cookieMap["user"])
		}
		if cookieMap["theme"] != "dark" {
			t.Errorf("Expected theme cookie to be 'dark', got %q", cookieMap["theme"])
		}
	})

	t.Run("Merge_First_Redirect_Wins", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetStatus(200)

		p2 := NewProxy()
		p2.serverRedirect("/login", 302)

		p3 := NewProxy()
		p3.serverRedirect("/home", 301)

		merged := MergeProxyResponses(p1, p2, p3)

		status, _ := merged.GetStatus()
		if status != 302 {
			t.Errorf("Expected status 302 (first redirect), got %d", status)
		}
		if loc := merged.GetLocation(); loc != "/login" {
			t.Errorf("Expected location '/login' (first redirect), got %q", loc)
		}
	})

	t.Run("Merge_Error_Beats_Redirect", func(t *testing.T) {
		// This test depends on the fix we discussed
		p1 := NewProxy()
		p1.SetStatus(403, "Forbidden")

		p2 := NewProxy()
		p2.serverRedirect("/login", 302)

		merged := MergeProxyResponses(p1, p2)

		status, _ := merged.GetStatus()
		if status != 403 {
			t.Errorf("Expected error (403) to beat redirect, got %d", status)
		}
		if merged.IsRedirect() {
			t.Error("Should not be a redirect when error is present")
		}
	})

	t.Run("Merge_Success_Then_Redirect", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetStatus(200)

		p2 := NewProxy()
		p2.serverRedirect("/dashboard", 302)

		merged := MergeProxyResponses(p1, p2)

		// Redirect should override success status
		status, _ := merged.GetStatus()
		if status != 302 {
			t.Errorf("Expected redirect to override success, got %d", status)
		}
		if !merged.IsRedirect() {
			t.Error("Should be a redirect")
		}
	})
}

// Test that client redirects should override, not accumulate
func TestProxy_ClientRedirect_Override(t *testing.T) {
	t.Run("Multiple_ClientRedirects_Override", func(t *testing.T) {
		p := NewProxy()

		// First redirect
		err := p.clientRedirect("/first")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Second redirect should override
		err = p.clientRedirect("/second")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should only have one redirect header value - the last one
		vals := p.GetHeaders(ClientRedirectHeader)
		if len(vals) != 1 {
			t.Errorf("Expected 1 redirect value, got %d", len(vals))
		}
		if len(vals) > 0 && vals[0] != "/second" {
			t.Errorf("Expected '/second', got %v", vals[0])
		}
	})

	t.Run("Redirect_Called_Multiple_Times", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(ClientAcceptsRedirectHeader, "true")

		p := NewProxy()

		// Multiple redirect calls
		p.Redirect(req, "/first")
		p.Redirect(req, "/second")
		p.Redirect(req, "/third")

		// Should have only the last redirect
		vals := p.GetHeaders(ClientRedirectHeader)
		if len(vals) != 1 {
			t.Errorf("Expected 1 redirect value, got %d", len(vals))
		}
		if len(vals) > 0 && vals[0] != "/third" {
			t.Errorf("Expected '/third', got %v", vals[0])
		}
	})
}

// Test that SetHeader clears previous values
func TestProxy_SetHeader_Clears(t *testing.T) {
	t.Run("Set_Clears_Previous_Values", func(t *testing.T) {
		p := NewProxy()

		// This sequence should result in only ["value3", "value4"]
		p.SetHeader("X-Custom", "value1")
		p.AddHeader("X-Custom", "value2")
		p.SetHeader("X-Custom", "value3") // Should clear previous
		p.AddHeader("X-Custom", "value4")

		vals := p.GetHeaders("X-Custom")

		if len(vals) != 2 {
			t.Errorf("Expected 2 values, got %d: %v", len(vals), vals)
		}
		if len(vals) >= 2 && (vals[0] != "value3" || vals[1] != "value4") {
			t.Errorf("Expected ['value3', 'value4'], got %v", vals)
		}
	})

	t.Run("Multiple_Sets_Last_Wins", func(t *testing.T) {
		p := NewProxy()

		p.SetHeader("X-Test", "first")
		p.SetHeader("X-Test", "second")
		p.SetHeader("X-Test", "third")

		vals := p.GetHeaders("X-Test")
		if len(vals) != 1 || vals[0] != "third" {
			t.Errorf("Expected ['third'], got %v", vals)
		}
	})
}

// Test ApplyToResponseWriter handles redirects correctly
func TestProxy_ApplyToResponseWriter_RedirectOrder(t *testing.T) {
	t.Run("Redirect_Works_With_Status", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(200)
		p.serverRedirect("/login", 302)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		p.ApplyToResponseWriter(w, req)

		// Redirect should work properly
		if w.Code != 302 {
			t.Errorf("Expected redirect status 302, got %d", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/login" {
			t.Errorf("Expected Location header '/login', got %q", loc)
		}
	})

	t.Run("Redirect_Ignored_On_Error", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(404, "Not Found")
		p.serverRedirect("/404-page", 302)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		p.ApplyToResponseWriter(w, req)

		// Error status should win, redirect ignored
		if w.Code != 404 {
			t.Errorf("Expected error status 404, got %d", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "" {
			t.Errorf("Should not have Location header on error, got %q", loc)
		}
	})

	t.Run("Redirect_Overrides_Success_Status", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(200)
		p.SetHeader("Content-Type", "application/json")
		p.serverRedirect("/login", 303)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		p.ApplyToResponseWriter(w, req)

		// Redirect should override the 200 status
		if w.Code != 303 {
			t.Errorf("Expected redirect status 303, got %d", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/login" {
			t.Errorf("Expected Location header '/login', got %q", loc)
		}
	})
}

// Test ApplyToResponseWriter respects Set vs Add
func TestProxy_ApplyToResponseWriter_HeaderSemantics(t *testing.T) {
	t.Run("SetHeader_Replaces_Existing", func(t *testing.T) {
		// Simulate middleware setting headers
		w := httptest.NewRecorder()
		w.Header().Set("X-Request-ID", "original")
		w.Header().Set("X-Custom", "middleware")

		p := NewProxy()
		p.SetHeader("X-Request-ID", "proxy-id")
		p.SetHeader("X-Custom", "proxy-value")

		req := httptest.NewRequest("GET", "/", nil)
		p.ApplyToResponseWriter(w, req)

		// Should have replaced the values
		if v := w.Header().Get("X-Request-ID"); v != "proxy-id" {
			t.Errorf("Expected 'proxy-id', got %q", v)
		}
		if v := w.Header().Get("X-Custom"); v != "proxy-value" {
			t.Errorf("Expected 'proxy-value', got %q", v)
		}

		// Should not have multiple values
		if vals := w.Header().Values("X-Request-ID"); len(vals) != 1 {
			t.Errorf("Expected 1 value, got %d: %v", len(vals), vals)
		}
	})

	t.Run("AddHeader_Appends_To_Existing", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Add("X-Forward", "10.0.0.1")

		p := NewProxy()
		p.AddHeader("X-Forward", "10.0.0.2")
		p.AddHeader("X-Forward", "10.0.0.3")

		req := httptest.NewRequest("GET", "/", nil)
		p.ApplyToResponseWriter(w, req)

		vals := w.Header().Values("X-Forward")
		if len(vals) != 3 {
			t.Errorf("Expected 3 values, got %d: %v", len(vals), vals)
		}
		expected := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
		for i, v := range vals {
			if v != expected[i] {
				t.Errorf("Expected %q at position %d, got %q", expected[i], i, v)
			}
		}
	})

	t.Run("Client_Redirect_Single_Value", func(t *testing.T) {
		w := httptest.NewRecorder()

		p := NewProxy()
		p.clientRedirect("/first")
		p.clientRedirect("/second")

		req := httptest.NewRequest("GET", "/", nil)
		p.ApplyToResponseWriter(w, req)

		vals := w.Header().Values(ClientRedirectHeader)
		if len(vals) != 1 {
			t.Errorf("Expected 1 redirect value, got %d: %v", len(vals), vals)
		}
		if len(vals) > 0 && vals[0] != "/second" {
			t.Errorf("Expected '/second', got %q", vals[0])
		}
	})
}

// Test MergeProxyResponses respects header operations
func TestMergeProxyResponses_HeaderOperations(t *testing.T) {
	t.Run("Merge_SetHeader_Clears_Previous", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetHeader("X-Test", "p1-value1")
		p1.AddHeader("X-Test", "p1-value2")

		p2 := NewProxy()
		p2.SetHeader("X-Test", "p2-value1") // Should clear p1's values
		p2.AddHeader("X-Test", "p2-value2")

		merged := MergeProxyResponses(p1, p2)

		vals := merged.GetHeaders("X-Test")
		if len(vals) != 2 {
			t.Errorf("Expected 2 values, got %d: %v", len(vals), vals)
		}
		if len(vals) >= 2 && (vals[0] != "p2-value1" || vals[1] != "p2-value2") {
			t.Errorf("Expected ['p2-value1', 'p2-value2'], got %v", vals)
		}
	})

	t.Run("Merge_Complex_Operations", func(t *testing.T) {
		p1 := NewProxy()
		p1.AddHeader("Cache-Control", "no-cache")
		p1.AddHeader("Cache-Control", "no-store")

		p2 := NewProxy()
		p2.SetHeader("Cache-Control", "max-age=3600") // Should replace all

		p3 := NewProxy()
		p3.AddHeader("Cache-Control", "must-revalidate")

		merged := MergeProxyResponses(p1, p2, p3)

		vals := merged.GetHeaders("Cache-Control")
		// Should be ["max-age=3600", "must-revalidate"]
		if len(vals) != 2 {
			t.Errorf("Expected 2 values, got %d: %v", len(vals), vals)
		}
		if len(vals) >= 2 && (vals[0] != "max-age=3600" || vals[1] != "must-revalidate") {
			t.Errorf("Expected ['max-age=3600', 'must-revalidate'], got %v", vals)
		}
	})

	t.Run("Merge_First_Client_Redirect_Wins", func(t *testing.T) {
		p1 := NewProxy()
		p1.clientRedirect("/page1")

		p2 := NewProxy()
		p2.clientRedirect("/page2")

		p3 := NewProxy()
		p3.clientRedirect("/page3")

		merged := MergeProxyResponses(p1, p2, p3)

		vals := merged.GetHeaders(ClientRedirectHeader)
		if len(vals) != 1 {
			t.Errorf("Expected 1 redirect value, got %d", len(vals))
		}
		if len(vals) > 0 && vals[0] != "/page1" {
			t.Errorf("Expected first redirect '/page1', got %q", vals[0])
		}
	})
}

// Test complex scenarios
func TestProxy_ComplexScenarios(t *testing.T) {
	t.Run("Middleware_Chain_Simulation", func(t *testing.T) {
		// Proxy 1: Initial middleware
		p1 := NewProxy()
		p1.SetHeader("X-Request-ID", "req-123")
		p1.AddHeader("X-Forwarded-For", "10.0.0.1")
		p1.SetStatus(200)

		// Proxy 2: Auth middleware - overrides request ID
		p2 := NewProxy()
		p2.SetHeader("X-Request-ID", "auth-456")
		p2.AddHeader("X-Auth-User", "john")

		// Proxy 3: Business logic - final request ID
		p3 := NewProxy()
		p3.AddHeader("X-Forwarded-For", "10.0.0.2")
		p3.SetHeader("X-Request-ID", "final-789")

		// Merge all
		merged := MergeProxyResponses(p1, p2, p3)

		// Apply to response writer
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		merged.ApplyToResponseWriter(w, req)

		// Verify final state
		reqID := w.Header().Get("X-Request-ID")
		if reqID != "final-789" {
			t.Errorf("Expected X-Request-ID 'final-789', got %q", reqID)
		}

		forwards := w.Header().Values("X-Forwarded-For")
		if len(forwards) != 2 {
			t.Errorf("Expected 2 X-Forwarded-For values, got %d: %v", len(forwards), forwards)
		}

		authUser := w.Header().Get("X-Auth-User")
		if authUser != "john" {
			t.Errorf("Expected X-Auth-User 'john', got %q", authUser)
		}
	})

	t.Run("Error_Redirect_Priority", func(t *testing.T) {
		// Test that errors take precedence over redirects
		p1 := NewProxy()
		p1.serverRedirect("/login", 302)

		p2 := NewProxy()
		p2.SetStatus(403, "Forbidden")

		merged := MergeProxyResponses(p1, p2)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		merged.ApplyToResponseWriter(w, req)

		if w.Code != 403 {
			t.Errorf("Expected error 403 to override redirect, got %d", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "" {
			t.Errorf("Should not have Location header with error, got %q", loc)
		}
		if !strings.Contains(w.Body.String(), "Forbidden") {
			t.Errorf("Expected error body to contain 'Forbidden'")
		}
	})
}

func TestProxy_Redirect_InvalidURL(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(ClientAcceptsRedirectHeader, "true")
	p := NewProxy()
	_, err := p.Redirect(req, "javascript:alert(1)")
	if err == nil {
		t.Error("Expected error for invalid URL scheme")
	}
}
