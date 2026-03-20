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

		status, text := p.Status()
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

		status, text := p.Status()
		if status != 400 {
			t.Errorf("Expected status 400, got %d", status)
		}
		if text != "Bad Request Custom" {
			t.Errorf("Expected 'Bad Request Custom', got %q", text)
		}
	})

	t.Run("Status_Helpers", func(t *testing.T) {
		cases := []struct {
			status      int
			is_err      bool
			is_redirect bool
			is_success  bool
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

		for _, tc := range cases {
			p := NewProxy()
			p.SetStatus(tc.status)
			p._location = "/somewhere"

			if p.IsError() != tc.is_err {
				t.Errorf(
					"Status %d: IsError() = %v, want %v",
					tc.status,
					p.IsError(),
					tc.is_err,
				)
			}
			if p.is_server_redirect() != tc.is_redirect {
				t.Errorf(
					"Status %d: is_server_redirect() = %v, want %v",
					tc.status,
					p.is_server_redirect(),
					tc.is_redirect,
				)
			}
			if p.IsSuccess() != tc.is_success {
				t.Errorf(
					"Status %d: IsSuccess() = %v, want %v",
					tc.status,
					p.IsSuccess(),
					tc.is_success,
				)
			}
		}
	})

	t.Run("SetStatus_Clears_Stale_ErrorText", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(400, "Bad Request Custom")
		p.SetStatus(500)

		status, text := p.Status()
		if status != 500 {
			t.Fatalf("expected status 500, got %d", status)
		}
		if text != "" {
			t.Fatalf("expected status text to be cleared, got %q", text)
		}
	})
}

func TestProxy_Headers(t *testing.T) {
	t.Run("SetHeader_Overwrites", func(t *testing.T) {
		p := NewProxy()
		p.SetHeader("X-Test", "value1")
		p.SetHeader("X-Test", "value2")

		if v := p.Header("X-Test"); v != "value2" {
			t.Errorf("Expected 'value2', got %q", v)
		}
		if vals := p.Headers("X-Test"); len(vals) != 1 || vals[0] != "value2" {
			t.Errorf("Expected ['value2'], got %v", vals)
		}
	})

	t.Run("AddHeader_Appends", func(t *testing.T) {
		p := NewProxy()
		p.AddHeader("X-Test", "value1")
		p.AddHeader("X-Test", "value2")
		p.AddHeader("X-Test", "value3")

		if v := p.Header("X-Test"); v != "value1" {
			t.Errorf("Header should return first value, got %q", v)
		}

		vals := p.Headers("X-Test")
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

	t.Run("Header_NonExistent", func(t *testing.T) {
		p := NewProxy()
		if v := p.Header("X-Missing"); v != "" {
			t.Errorf("Expected empty string for missing header, got %q", v)
		}
		if vals := p.Headers("X-Missing"); vals != nil {
			t.Errorf("Expected nil for missing headers, got %v", vals)
		}
	})

	t.Run("Header_Key_Canonicalized", func(t *testing.T) {
		p := NewProxy()
		p.SetHeader("content-type", "application/json")
		p.AddHeader("x-forwarded-for", "10.0.0.1")

		if got := p.Header("Content-Type"); got != "application/json" {
			t.Fatalf("expected canonical key lookup to work, got %q", got)
		}
		forwarded := p.Headers("X-Forwarded-For")
		if len(forwarded) != 1 || forwarded[0] != "10.0.0.1" {
			t.Fatalf("expected canonical header values, got %v", forwarded)
		}
	})
}

func TestProxy_Cookies(t *testing.T) {
	t.Run("SetCookie", func(t *testing.T) {
		p := NewProxy()
		p.SetCookie(&http.Cookie{Name: "session", Value: "abc123"})
		p.SetCookie(&http.Cookie{Name: "user", Value: "john"})

		cookies := p.Cookies()
		if len(cookies) != 2 {
			t.Errorf("Expected 2 cookies, got %d", len(cookies))
		}
		if cookies[0].Name != "session" {
			t.Errorf("Expected first cookie 'session', got %q", cookies[0].Name)
		}
		if cookies[1].Name != "user" {
			t.Errorf("Expected second cookie 'user', got %q", cookies[1].Name)
		}
	})

	t.Run("SetCookie_Nil_NoOp", func(t *testing.T) {
		p := NewProxy()
		p.SetCookie(nil)

		if len(p.Cookies()) != 0 {
			t.Fatalf("expected no cookies, got %d", len(p.Cookies()))
		}
	})
}

func TestProxy_Redirects(t *testing.T) {
	t.Run("ServerRedirect", func(t *testing.T) {
		p := NewProxy()
		p.server_redirect("/login", 302)

		if !p.is_server_redirect() {
			t.Error("Expected is_server_redirect to be true")
		}
		if p.is_client_redirect() {
			t.Error("Expected is_client_redirect to be false")
		}
		if !p.IsRedirect() {
			t.Error("Expected IsRedirect to be true")
		}
		if p.Location() != "/login" {
			t.Errorf("Expected location '/login', got %q", p.Location())
		}
		status, _ := p.Status()
		if status != 302 {
			t.Errorf("Expected status 302, got %d", status)
		}
	})

	t.Run("ClientRedirect", func(t *testing.T) {
		p := NewProxy()
		err := p.client_redirect("https://example.com")

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !p.is_client_redirect() {
			t.Error("Expected is_client_redirect to be true")
		}
		if p.is_server_redirect() {
			t.Error("Expected is_server_redirect to be false")
		}
		if !p.IsRedirect() {
			t.Error("Expected IsRedirect to be true")
		}

		status, _ := p.Status()
		if status != 200 {
			t.Errorf("Expected status 200, got %d", status)
		}
		if h := p.Header(ClientRedirectHeader); h != "https://example.com" {
			t.Errorf("Expected client redirect header, got %q", h)
		}
	})

	t.Run("Redirect_Method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		p := NewProxy()

		upgraded, err := p.Redirect(req, "/login", 302)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if upgraded {
			t.Error(
				"Should not upgrade to client redirect without proper header",
			)
		}
		if !p.is_server_redirect() {
			t.Error("Should be server redirect")
		}
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
		if vals := w.Header().Values("X-Multi"); len(vals) != 2 {
			t.Errorf("Expected 2 values for X-Multi, got %d", len(vals))
		}
	})

	t.Run("Apply_Error_With_Text", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(404, "Page not found custom")

		w := httptest.NewRecorder()
		p.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

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
		p.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

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
		p.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

		if len(w.Result().Cookies()) != 2 {
			t.Errorf("Expected 2 cookies, got %d", len(w.Result().Cookies()))
		}
	})

	t.Run("Apply_Server_Redirect", func(t *testing.T) {
		p := NewProxy()
		p.server_redirect("/login", 302)

		w := httptest.NewRecorder()
		p.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

		if w.Code != 302 {
			t.Errorf("Expected status 302, got %d", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/login" {
			t.Errorf("Expected Location '/login', got %q", loc)
		}
	})
}

func TestMergeProxyResponses(t *testing.T) {
	t.Run("First_Error_Wins", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetStatus(200)
		p2 := NewProxy()
		p2.SetStatus(403, "Forbidden")
		p3 := NewProxy()
		p3.SetStatus(401, "Unauthorized")

		merged := MergeProxyResponses(p1, p2, p3)

		status, text := merged.Status()
		if status != 403 {
			t.Errorf("Expected first error (403), got %d", status)
		}
		if text != "Forbidden" {
			t.Errorf("Expected 'Forbidden', got %q", text)
		}
	})

	t.Run("Last_Success_Wins", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetStatus(200)
		p2 := NewProxy()
		p2.SetStatus(201)
		p3 := NewProxy()
		p3.SetStatus(202)

		merged := MergeProxyResponses(p1, p2, p3)

		status, _ := merged.Status()
		if status != 202 {
			t.Errorf("Expected last success (202), got %d", status)
		}
	})

	t.Run("Headers_Combined", func(t *testing.T) {
		p1 := NewProxy()
		p1.AddHeader("X-Test", "val1")
		p1.SetHeader("X-Only-P1", "p1")
		p2 := NewProxy()
		p2.AddHeader("X-Test", "val2")
		p2.SetHeader("X-Only-P2", "p2")

		merged := MergeProxyResponses(p1, p2)

		vals := merged.Headers("X-Test")
		if len(vals) != 2 || vals[0] != "val1" || vals[1] != "val2" {
			t.Errorf("Expected merged headers [val1, val2], got %v", vals)
		}
		if v := merged.Header("X-Only-P1"); v != "p1" {
			t.Errorf("Expected 'p1', got %q", v)
		}
		if v := merged.Header("X-Only-P2"); v != "p2" {
			t.Errorf("Expected 'p2', got %q", v)
		}
	})

	t.Run("Cookies_Later_Overwrites", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetCookie(&http.Cookie{Name: "session", Value: "old"})
		p1.SetCookie(&http.Cookie{Name: "user", Value: "john"})
		p2 := NewProxy()
		p2.SetCookie(&http.Cookie{Name: "session", Value: "new"})
		p2.SetCookie(&http.Cookie{Name: "theme", Value: "dark"})

		merged := MergeProxyResponses(p1, p2)

		cookie_map := make(map[string]string)
		for _, c := range merged.Cookies() {
			cookie_map[c.Name] = c.Value
		}
		if cookie_map["session"] != "new" {
			t.Errorf("Expected session 'new', got %q", cookie_map["session"])
		}
		if cookie_map["user"] != "john" {
			t.Errorf("Expected user 'john', got %q", cookie_map["user"])
		}
		if cookie_map["theme"] != "dark" {
			t.Errorf("Expected theme 'dark', got %q", cookie_map["theme"])
		}
	})

	t.Run("First_Redirect_Wins", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetStatus(200)
		p2 := NewProxy()
		p2.server_redirect("/login", 302)
		p3 := NewProxy()
		p3.server_redirect("/home", 301)

		merged := MergeProxyResponses(p1, p2, p3)

		status, _ := merged.Status()
		if status != 302 {
			t.Errorf("Expected status 302, got %d", status)
		}
		if loc := merged.Location(); loc != "/login" {
			t.Errorf("Expected location '/login', got %q", loc)
		}
	})

	t.Run("Error_Beats_Redirect", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetStatus(403, "Forbidden")
		p2 := NewProxy()
		p2.server_redirect("/login", 302)

		merged := MergeProxyResponses(p1, p2)

		status, _ := merged.Status()
		if status != 403 {
			t.Errorf("Expected error (403) to beat redirect, got %d", status)
		}
		if merged.IsRedirect() {
			t.Error("Should not be a redirect when error is present")
		}
	})

	t.Run("Success_Then_Redirect", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetStatus(200)
		p2 := NewProxy()
		p2.server_redirect("/dashboard", 302)

		merged := MergeProxyResponses(p1, p2)

		status, _ := merged.Status()
		if status != 302 {
			t.Errorf("Expected redirect to override success, got %d", status)
		}
		if !merged.IsRedirect() {
			t.Error("Should be a redirect")
		}
	})

	t.Run("Ignores_Nil_Proxies_And_Cookies", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetStatus(200)
		p1.SetCookie(nil)
		p1.SetCookie(&http.Cookie{Name: "session", Value: "ok"})

		merged := MergeProxyResponses(nil, p1, nil)

		status, _ := merged.Status()
		if status != 200 {
			t.Fatalf("expected status 200, got %d", status)
		}
		cookies := merged.Cookies()
		if len(cookies) != 1 {
			t.Fatalf("expected one cookie, got %d", len(cookies))
		}
		if cookies[0].Name != "session" {
			t.Fatalf("expected session cookie, got %q", cookies[0].Name)
		}
	})
}

func TestProxy_ClientRedirect_Override(t *testing.T) {
	t.Run("Multiple_ClientRedirects_Override", func(t *testing.T) {
		p := NewProxy()
		if err := p.client_redirect("/first"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if err := p.client_redirect("/second"); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		vals := p.Headers(ClientRedirectHeader)
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
		p.Redirect(req, "/first")
		p.Redirect(req, "/second")
		p.Redirect(req, "/third")

		vals := p.Headers(ClientRedirectHeader)
		if len(vals) != 1 {
			t.Errorf("Expected 1 redirect value, got %d", len(vals))
		}
		if len(vals) > 0 && vals[0] != "/third" {
			t.Errorf("Expected '/third', got %v", vals[0])
		}
	})
}

func TestProxy_SetHeader_Clears(t *testing.T) {
	t.Run("Set_Clears_Previous_Values", func(t *testing.T) {
		p := NewProxy()
		p.SetHeader("X-Custom", "value1")
		p.AddHeader("X-Custom", "value2")
		p.SetHeader("X-Custom", "value3")
		p.AddHeader("X-Custom", "value4")

		vals := p.Headers("X-Custom")
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

		vals := p.Headers("X-Test")
		if len(vals) != 1 || vals[0] != "third" {
			t.Errorf("Expected ['third'], got %v", vals)
		}
	})
}

func TestProxy_ApplyToResponseWriter_RedirectOrder(t *testing.T) {
	t.Run("Redirect_Works_With_Status", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(200)
		p.server_redirect("/login", 302)

		w := httptest.NewRecorder()
		p.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

		if w.Code != 302 {
			t.Errorf("Expected redirect status 302, got %d", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/login" {
			t.Errorf("Expected Location '/login', got %q", loc)
		}
	})

	t.Run("Redirect_Ignored_On_Error", func(t *testing.T) {
		p := NewProxy()
		p.SetStatus(404, "Not Found")
		p.server_redirect("/404-page", 302)

		w := httptest.NewRecorder()
		p.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

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
		p.server_redirect("/login", 303)

		w := httptest.NewRecorder()
		p.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

		if w.Code != 303 {
			t.Errorf("Expected redirect status 303, got %d", w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/login" {
			t.Errorf("Expected Location '/login', got %q", loc)
		}
	})
}

func TestProxy_ApplyToResponseWriter_HeaderSemantics(t *testing.T) {
	t.Run("SetHeader_Replaces_Existing", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("X-Request-ID", "original")
		w.Header().Set("X-Custom", "middleware")

		p := NewProxy()
		p.SetHeader("X-Request-ID", "proxy-id")
		p.SetHeader("X-Custom", "proxy-value")

		p.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

		if v := w.Header().Get("X-Request-ID"); v != "proxy-id" {
			t.Errorf("Expected 'proxy-id', got %q", v)
		}
		if v := w.Header().Get("X-Custom"); v != "proxy-value" {
			t.Errorf("Expected 'proxy-value', got %q", v)
		}
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

		p.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

		vals := w.Header().Values("X-Forward")
		if len(vals) != 3 {
			t.Errorf("Expected 3 values, got %d: %v", len(vals), vals)
		}
		expected := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
		for i, v := range vals {
			if v != expected[i] {
				t.Errorf("Expected %q at %d, got %q", expected[i], i, v)
			}
		}
	})

	t.Run("Client_Redirect_Single_Value", func(t *testing.T) {
		w := httptest.NewRecorder()

		p := NewProxy()
		p.client_redirect("/first")
		p.client_redirect("/second")

		p.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

		vals := w.Header().Values(ClientRedirectHeader)
		if len(vals) != 1 {
			t.Errorf("Expected 1 redirect value, got %d: %v", len(vals), vals)
		}
		if len(vals) > 0 && vals[0] != "/second" {
			t.Errorf("Expected '/second', got %q", vals[0])
		}
	})
}

func TestMergeProxyResponses_HeaderOperations(t *testing.T) {
	t.Run("SetHeader_Clears_Previous", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetHeader("X-Test", "p1-value1")
		p1.AddHeader("X-Test", "p1-value2")
		p2 := NewProxy()
		p2.SetHeader("X-Test", "p2-value1")
		p2.AddHeader("X-Test", "p2-value2")

		merged := MergeProxyResponses(p1, p2)

		vals := merged.Headers("X-Test")
		if len(vals) != 2 {
			t.Errorf("Expected 2 values, got %d: %v", len(vals), vals)
		}
		if len(vals) >= 2 &&
			(vals[0] != "p2-value1" || vals[1] != "p2-value2") {
			t.Errorf("Expected ['p2-value1', 'p2-value2'], got %v", vals)
		}
	})

	t.Run("Complex_Operations", func(t *testing.T) {
		p1 := NewProxy()
		p1.AddHeader("Cache-Control", "no-cache")
		p1.AddHeader("Cache-Control", "no-store")
		p2 := NewProxy()
		p2.SetHeader("Cache-Control", "max-age=3600")
		p3 := NewProxy()
		p3.AddHeader("Cache-Control", "must-revalidate")

		merged := MergeProxyResponses(p1, p2, p3)

		vals := merged.Headers("Cache-Control")
		if len(vals) != 2 {
			t.Errorf("Expected 2 values, got %d: %v", len(vals), vals)
		}
		if len(vals) >= 2 &&
			(vals[0] != "max-age=3600" || vals[1] != "must-revalidate") {
			t.Errorf(
				"Expected ['max-age=3600', 'must-revalidate'], got %v",
				vals,
			)
		}
	})

	t.Run("First_Client_Redirect_Wins", func(t *testing.T) {
		p1 := NewProxy()
		p1.client_redirect("/page1")
		p2 := NewProxy()
		p2.client_redirect("/page2")
		p3 := NewProxy()
		p3.client_redirect("/page3")

		merged := MergeProxyResponses(p1, p2, p3)

		vals := merged.Headers(ClientRedirectHeader)
		if len(vals) != 1 {
			t.Errorf("Expected 1 redirect value, got %d", len(vals))
		}
		if len(vals) > 0 && vals[0] != "/page1" {
			t.Errorf("Expected first redirect '/page1', got %q", vals[0])
		}
	})
}

func TestProxy_ComplexScenarios(t *testing.T) {
	t.Run("Middleware_Chain_Simulation", func(t *testing.T) {
		p1 := NewProxy()
		p1.SetHeader("X-Request-ID", "req-123")
		p1.AddHeader("X-Forwarded-For", "10.0.0.1")
		p1.SetStatus(200)

		p2 := NewProxy()
		p2.SetHeader("X-Request-ID", "auth-456")
		p2.AddHeader("X-Auth-User", "john")

		p3 := NewProxy()
		p3.AddHeader("X-Forwarded-For", "10.0.0.2")
		p3.SetHeader("X-Request-ID", "final-789")

		merged := MergeProxyResponses(p1, p2, p3)

		w := httptest.NewRecorder()
		merged.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

		req_id := w.Header().Get("X-Request-ID")
		if req_id != "final-789" {
			t.Errorf("Expected X-Request-ID 'final-789', got %q", req_id)
		}
		forwards := w.Header().Values("X-Forwarded-For")
		if len(forwards) != 2 {
			t.Errorf(
				"Expected 2 X-Forwarded-For values, got %d: %v",
				len(forwards),
				forwards,
			)
		}
		auth_user := w.Header().Get("X-Auth-User")
		if auth_user != "john" {
			t.Errorf("Expected X-Auth-User 'john', got %q", auth_user)
		}
	})

	t.Run("Error_Redirect_Priority", func(t *testing.T) {
		p1 := NewProxy()
		p1.server_redirect("/login", 302)
		p2 := NewProxy()
		p2.SetStatus(403, "Forbidden")

		merged := MergeProxyResponses(p1, p2)

		w := httptest.NewRecorder()
		merged.ApplyToResponseWriter(w, httptest.NewRequest("GET", "/", nil))

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
	used, err := p.Redirect(req, "javascript:alert(1)")
	if err == nil {
		t.Error("Expected error for invalid URL scheme")
	}
	if used {
		t.Error("Expected usedClientRedirect to be false when validation fails")
	}
}

func TestProxy_ApplyToResponseWriter_NilRequest_ServerRedirect(t *testing.T) {
	p := NewProxy()
	p.server_redirect("/login", http.StatusFound)

	w := httptest.NewRecorder()
	p.ApplyToResponseWriter(w, nil)

	if w.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Fatalf("expected Location '/login', got %q", loc)
	}
}
