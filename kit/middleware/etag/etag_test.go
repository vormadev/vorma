package etag

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestETagMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           string
		headers        map[string]string
		config         *Config
		expectedStatus int
		expectedBody   string
		expectedETag   string
		checkETag      bool
	}{
		{
			name:           "GET request with no matching ETag",
			method:         http.MethodGet,
			body:           "Hello, World!",
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello, World!",
			checkETag:      true,
		},
		{
			name:           "HEAD request with no matching ETag",
			method:         http.MethodHead,
			body:           "Hello, World!",
			expectedStatus: http.StatusOK,
			expectedBody:   "",
			checkETag:      true,
		},
		{
			name:           "POST request should not get ETag",
			method:         http.MethodPost,
			body:           "Hello, World!",
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello, World!",
			checkETag:      false,
		},
		{
			name:   "GET request with matching ETag",
			method: http.MethodGet,
			body:   "Hello, World!",
			headers: map[string]string{
				"If-None-Match": `"0a0a9f2a6772942557ab5355d76af442f8f65e01"`,
			},
			expectedStatus: http.StatusNotModified,
			expectedBody:   "",
			checkETag:      true,
		},
		{
			name:   "GET request with matching weak ETag",
			method: http.MethodGet,
			body:   "Hello, World!",
			headers: map[string]string{
				"If-None-Match": `W/"0a0a9f2a6772942557ab5355d76af442f8f65e01"`,
			},
			config: &Config{
				Strong: false,
			},
			expectedStatus: http.StatusNotModified,
			expectedBody:   "",
			checkETag:      true,
		},
		{
			name:   "GET request with non-matching ETag",
			method: http.MethodGet,
			body:   "Hello, World!",
			headers: map[string]string{
				"If-None-Match": `"different-etag"`,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello, World!",
			checkETag:      true,
		},
		{
			name:   "GET request with Cache-Control: no-cache",
			method: http.MethodGet,
			body:   "Hello, World!",
			headers: map[string]string{
				"Cache-Control": "no-cache",
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello, World!",
			checkETag:      true,
		},
		{
			name:   "GET request exceeding max size",
			method: http.MethodGet,
			body:   strings.Repeat("X", 100),
			config: &Config{
				MaxBodySize: 50,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   strings.Repeat("X", 100),
			checkETag:      false,
		},
		{
			name:   "GET request with custom hash function",
			method: http.MethodGet,
			body:   "Hello, World!",
			config: &Config{
				Hash: md5.New,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "Hello, World!",
			checkETag:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.headers != nil {
					if setCookie, ok := tt.headers["Set-Cookie"]; ok {
						w.Header().Set("Set-Cookie", setCookie)
					}
					if cacheControl, ok := tt.headers["Cache-Control"]; ok {
						w.Header().Set("Cache-Control", cacheControl)
					}
				}
				w.Write([]byte(tt.body))
			})

			var middleware func(http.Handler) http.Handler
			if tt.config != nil {
				middleware = Auto(tt.config)
			} else {
				middleware = Auto()
			}

			server := httptest.NewServer(middleware(handler))
			defer server.Close()

			req, err := http.NewRequest(tt.method, server.URL, nil)
			if err != nil {
				t.Fatalf("Error creating request: %v", err)
			}

			if tt.headers != nil {
				for key, value := range tt.headers {
					req.Header.Set(key, value)
				}
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Error making request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			etag := resp.Header.Get("ETag")
			if tt.checkETag && etag == "" {
				t.Error("Expected ETag header to be set, but it was not")
			} else if !tt.checkETag && etag != "" {
				t.Errorf("Expected no ETag header, but got: %s", etag)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Error reading response body: %v", err)
			}

			if string(body) != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, string(body))
			}
		})
	}
}

func TestETagValue(t *testing.T) {
	tests := []struct {
		name        string
		etag        string
		ifNoneMatch string
		shouldMatch bool
	}{
		{
			name:        "Exact match",
			etag:        `"tag123"`,
			ifNoneMatch: `"tag123"`,
			shouldMatch: true,
		},
		{
			name:        "Weak vs strong match",
			etag:        `"tag123"`,
			ifNoneMatch: `W/"tag123"`,
			shouldMatch: true,
		},
		{
			name:        "Strong vs weak match",
			etag:        `W/"tag123"`,
			ifNoneMatch: `"tag123"`,
			shouldMatch: true,
		},
		{
			name:        "No match",
			etag:        `"tag123"`,
			ifNoneMatch: `"differenttag"`,
			shouldMatch: false,
		},
		{
			name:        "Wildcard match",
			etag:        `"anytag"`,
			ifNoneMatch: `*`,
			shouldMatch: true,
		},
		{
			name:        "Multiple tags with match",
			etag:        `"tag123"`,
			ifNoneMatch: `"nope", "tag123", "alsono"`,
			shouldMatch: true,
		},
		{
			name:        "Multiple tags without match",
			etag:        `"tag123"`,
			ifNoneMatch: `"nope", "nohope", "stillno"`,
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := etagMatches(tt.ifNoneMatch, tt.etag)
			if result != tt.shouldMatch {
				t.Errorf("etagMatches(%q, %q) = %v, want %v",
					tt.ifNoneMatch, tt.etag, result, tt.shouldMatch)
			}
		})
	}
}

func TestGenerateETag(t *testing.T) {
	content := []byte("Hello, World!")

	t.Run("Strong ETag", func(t *testing.T) {
		h := md5.New()
		h.Write(content)
		strong := true
		etag := generateETag(h, strong, http.Header{})

		if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
			t.Errorf("Strong ETag should be wrapped in double quotes, got: %s", etag)
		}

		if strings.HasPrefix(etag, `W/"`) {
			t.Errorf("Strong ETag should not have weak prefix, got: %s", etag)
		}
	})

	t.Run("Weak ETag", func(t *testing.T) {
		h := md5.New()
		h.Write(content)
		strong := false
		etag := generateETag(h, strong, http.Header{})

		if !strings.HasPrefix(etag, `W/"`) || !strings.HasSuffix(etag, `"`) {
			t.Errorf("Weak ETag should be wrapped in W/\"...\", got: %s", etag)
		}
	})
}

func TestCanUseETag(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *etagWriter
		expected bool
	}{
		{
			name: "Valid ETag scenario",
			setup: func() *etagWriter {
				w := httptest.NewRecorder()
				ew := newETagWriter(w, md5.New(), 1024)
				ew.Write([]byte("content"))
				return ew
			},
			expected: true,
		},
		{
			name: "Too big response",
			setup: func() *etagWriter {
				w := httptest.NewRecorder()
				ew := newETagWriter(w, md5.New(), 10)
				ew.Write([]byte("content exceeding max size"))
				return ew
			},
			expected: false,
		},
		{
			name: "Non-OK status",
			setup: func() *etagWriter {
				w := httptest.NewRecorder()
				ew := newETagWriter(w, md5.New(), 1024)
				ew.WriteHeader(http.StatusNotFound)
				ew.Write([]byte("content"))
				return ew
			},
			expected: false,
		},
		{
			name: "Empty response",
			setup: func() *etagWriter {
				w := httptest.NewRecorder()
				ew := newETagWriter(w, md5.New(), 1024)
				return ew
			},
			expected: false,
		},
		{
			name: "Cache-Control: no-cache",
			setup: func() *etagWriter {
				w := httptest.NewRecorder()
				ew := newETagWriter(w, md5.New(), 1024)
				ew.Header().Set("Cache-Control", "no-cache")
				ew.Write([]byte("content"))
				return ew
			},
			expected: true,
		},
		{
			name: "Cache-Control: no-store",
			setup: func() *etagWriter {
				w := httptest.NewRecorder()
				ew := newETagWriter(w, md5.New(), 1024)
				ew.Header().Set("Cache-Control", "no-store")
				ew.Write([]byte("content"))
				return ew
			},
			expected: false,
		},
		{
			name: "Has Set-Cookie",
			setup: func() *etagWriter {
				w := httptest.NewRecorder()
				ew := newETagWriter(w, md5.New(), 1024)
				ew.Header().Set("Set-Cookie", "session=123")
				ew.Write([]byte("content"))
				return ew
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ew := tt.setup()
			if canUseETag(ew) != tt.expected {
				t.Errorf("canUseETag() = %v, want %v", !tt.expected, tt.expected)
			}
		})
	}
}

func TestExtractETagValue(t *testing.T) {
	tests := []struct {
		name     string
		etag     string
		expected string
	}{
		{
			name:     "Strong ETag",
			etag:     `"abc123"`,
			expected: "abc123",
		},
		{
			name:     "Weak ETag",
			etag:     `W/"abc123"`,
			expected: "abc123",
		},
		{
			name:     "Unquoted value",
			etag:     "abc123",
			expected: "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractETagValue(tt.etag)
			if result != tt.expected {
				t.Errorf("extractETagValue(%q) = %q, want %q", tt.etag, result, tt.expected)
			}
		})
	}
}

func TestRespondNotModified(t *testing.T) {
	t.Run("Removes entity headers", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "100")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Language", "en-US")
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2022 07:28:00 GMT")
		w.Header().Set("X-Custom", "value")

		respondNotModified(w, `"etag123"`)

		if w.Code != http.StatusNotModified {
			t.Errorf("Expected status code %d, got %d", http.StatusNotModified, w.Code)
		}

		if w.Header().Get("Content-Type") != "" {
			t.Error("Content-Type header should be removed")
		}

		if w.Header().Get("Content-Length") != "" {
			t.Error("Content-Length header should be removed")
		}

		if w.Header().Get("X-Custom") == "" {
			t.Error("X-Custom header should be preserved")
		}

		if w.Header().Get("ETag") != `"etag123"` {
			t.Errorf("ETag header should be set to %q, got %q", `"etag123"`, w.Header().Get("ETag"))
		}
	})
}

func TestETagWriterImplementsResponseWriter(t *testing.T) {
	// Ensure etagWriter implements http.ResponseWriter interface
	var _ http.ResponseWriter = (*etagWriter)(nil)
}

func TestWriteOriginalResponse(t *testing.T) {
	t.Run("Normal scenario", func(t *testing.T) {
		w := httptest.NewRecorder()
		ew := newETagWriter(w, md5.New(), 1024)
		ew.Header().Set("X-Test", "value")
		ew.WriteHeader(http.StatusAccepted)
		ew.Write([]byte("test content"))

		ew.WriteOriginalResponse()

		if w.Code != http.StatusAccepted {
			t.Errorf("Expected status code %d, got %d", http.StatusAccepted, w.Code)
		}

		if w.Header().Get("X-Test") != "value" {
			t.Errorf("Header not copied correctly, expected %q, got %q", "value", w.Header().Get("X-Test"))
		}

		if w.Body.String() != "test content" {
			t.Errorf("Body not written correctly, expected %q, got %q", "test content", w.Body.String())
		}
	})

	t.Run("Too big scenario", func(t *testing.T) {
		w := httptest.NewRecorder()
		ew := newETagWriter(w, md5.New(), 5)
		ew.Write([]byte("content too big"))

		// The content should already be written to the original writer
		if w.Body.String() != "content too big" {
			t.Errorf("Body not written directly, expected %q, got %q", "content too big", w.Body.String())
		}

		// This should be a no-op
		ew.WriteOriginalResponse()
	})
}

func TestWriteResponseWithETag(t *testing.T) {
	t.Run("Normal scenario", func(t *testing.T) {
		w := httptest.NewRecorder()
		ew := newETagWriter(w, md5.New(), 1024)
		ew.Header().Set("X-Test", "value")
		ew.Write([]byte("test content"))

		ew.WriteResponseWithETag(`"etag123"`)

		if w.Header().Get("ETag") != `"etag123"` {
			t.Errorf("ETag header not set correctly, expected %q, got %q", `"etag123"`, w.Header().Get("ETag"))
		}

		if w.Header().Get("X-Test") != "value" {
			t.Errorf("Header not copied correctly, expected %q, got %q", "value", w.Header().Get("X-Test"))
		}

		if w.Header().Get("Content-Length") != "12" {
			t.Errorf("Content-Length not set correctly, expected %q, got %q", "12", w.Header().Get("Content-Length"))
		}

		if w.Body.String() != "test content" {
			t.Errorf("Body not written correctly, expected %q, got %q", "test content", w.Body.String())
		}
	})

	t.Run("Too big scenario", func(t *testing.T) {
		w := httptest.NewRecorder()
		ew := newETagWriter(w, md5.New(), 5)
		ew.Write([]byte("content too big"))

		ew.WriteResponseWithETag(`"etag123"`)

		if w.Header().Get("ETag") != `"etag123"` {
			t.Errorf("ETag header not set correctly, expected %q, got %q", `"etag123"`, w.Header().Get("ETag"))
		}

		// Content-Length should not be set for too big responses
		if w.Header().Get("Content-Length") != "" {
			t.Errorf("Content-Length should not be set, but got %q", w.Header().Get("Content-Length"))
		}
	})
}

func TestETagWriterClearsPooledBuffer(t *testing.T) {
	w := httptest.NewRecorder()
	ew := newETagWriter(w, md5.New(), 1024)
	ew.Write([]byte("test"))

	// Before closing, buf should exist
	if ew.buf == nil {
		t.Error("Buffer should exist before Close()")
	}

	ew.Close()

	// After closing, buf should be nil
	if ew.buf != nil {
		t.Error("Buffer should be nil after Close()")
	}
}

func TestETagWriterMultipleWrites(t *testing.T) {
	w := httptest.NewRecorder()
	ew := newETagWriter(w, md5.New(), 1024)

	ew.Write([]byte("first "))
	ew.Write([]byte("second "))
	ew.Write([]byte("third"))

	expectedLen := int64(len("first second third"))

	if ew.size != expectedLen {
		t.Errorf("Size tracking incorrect, expected %d, got %d", expectedLen, ew.size)
	}

	if ew.buf.String() != "first second third" {
		t.Errorf("Buffer content incorrect, expected %q, got %q", "first second third", ew.buf.String())
	}
}

func TestIsPayloadHeader(t *testing.T) {
	payloadHeaders := []string{
		"Content-Type", "content-type",
		"Content-Length", "content-length",
		"Content-Encoding", "content-encoding",
		"Content-Language", "content-language",
		"Content-MD5", "content-md5",
		"Content-Range", "content-range",
		"Content-Disposition", "content-disposition",
		"Last-Modified", "last-modified",
		"Digest", "digest",
	}

	nonPayloadHeaders := []string{
		"Content-Location", "content-location",
		"ETag", "etag",
		"Cache-Control", "cache-control",
		"Expires", "expires",
		"Date", "date",
		"Vary", "vary",
		"Transfer-Encoding",
		"X-Custom-Header",
		"Authorization",
		"Host",
	}

	for _, h := range payloadHeaders {
		if !isPayloadHeader(h) {
			t.Errorf("isPayloadHeader(%q) = false, want true", h)
		}
	}

	for _, h := range nonPayloadHeaders {
		if isPayloadHeader(h) {
			t.Errorf("isPayloadHeader(%q) = true, want false", h)
		}
	}
}

func BenchmarkETagMiddleware(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("X", 1000)))
	})

	middleware := Auto()
	server := httptest.NewServer(middleware(handler))
	defer server.Close()

	b.ResetTimer()

	for b.Loop() {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func BenchmarkETagWithMatch(b *testing.B) {
	var etag string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("X", 1000)))
	})

	middleware := Auto()
	server := httptest.NewServer(middleware(handler))
	defer server.Close()

	// First request to get the ETag
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, _ := http.DefaultClient.Do(req)
	io.Copy(io.Discard, resp.Body)
	etag = resp.Header.Get("ETag")
	resp.Body.Close()

	b.ResetTimer()

	for b.Loop() {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		req.Header.Set("If-None-Match", etag)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func TestBufferPooling(t *testing.T) {
	// Test that buffers work correctly after pooling
	w := httptest.NewRecorder()

	// Create first writer, write to it and close it
	ew1 := newETagWriter(w, md5.New(), 1024)
	ew1.Write([]byte("test content"))
	ew1.Close()

	// Create second writer
	ew2 := newETagWriter(w, md5.New(), 1024)

	// Test that the second writer works correctly
	// (buffer should be reset regardless of whether it's reused)
	if ew2.buf.Len() != 0 {
		t.Errorf("Buffer content not reset, expected empty buffer, got length %d", ew2.buf.Len())
	}

	// Verify it can write normally
	ew2.Write([]byte("new content"))
	if ew2.buf.String() != "new content" {
		t.Errorf("Buffer not working correctly after pool, got %q", ew2.buf.String())
	}

	ew2.Close()
}

func TestMultipleCalls(t *testing.T) {
	// Test whether the middleware works correctly with multiple responses
	middleware := Auto()
	server := httptest.NewServer(middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})))
	defer server.Close()

	// Make first request
	resp1, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Error making first request: %v", err)
	}
	defer resp1.Body.Close()

	body1, _ := io.ReadAll(resp1.Body)
	etag1 := resp1.Header.Get("ETag")

	if string(body1) != "Hello, World!" {
		t.Errorf("First response body incorrect, expected %q, got %q", "Hello, World!", string(body1))
	}

	if etag1 == "" {
		t.Error("First response should have an ETag")
	}

	// Make second request with matching ETag
	req2, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req2.Header.Set("If-None-Match", etag1)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Error making second request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("Second response should have status 304, got %d", resp2.StatusCode)
	}

	// Make third request with different ETag
	req3, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req3.Header.Set("If-None-Match", `"different-tag"`)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("Error making third request: %v", err)
	}
	defer resp3.Body.Close()

	body3, _ := io.ReadAll(resp3.Body)

	if resp3.StatusCode != http.StatusOK {
		t.Errorf("Third response should have status 200, got %d", resp3.StatusCode)
	}

	if string(body3) != "Hello, World!" {
		t.Errorf("Third response body incorrect, expected %q, got %q", "Hello, World!", string(body3))
	}
}

func TestWriteHeaderCalledTwice(t *testing.T) {
	w := httptest.NewRecorder()
	ew := newETagWriter(w, md5.New(), 1024)

	// First call should set the status
	ew.WriteHeader(http.StatusCreated)
	if ew.status != http.StatusCreated {
		t.Errorf("First WriteHeader call should set status to %d, got %d", http.StatusCreated, ew.status)
	}

	// Second call should be ignored
	ew.WriteHeader(http.StatusBadRequest)
	if ew.status != http.StatusCreated {
		t.Errorf("Second WriteHeader call should not change status, expected %d, got %d",
			http.StatusCreated, ew.status)
	}
}

func TestETagRoundTrip(t *testing.T) {
	// End-to-end test simulating a full request-response cycle
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Test content for ETag"))
	})

	middleware := Auto()
	server := httptest.NewServer(middleware(handler))
	defer server.Close()

	// First request - should get content and ETag
	resp1, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Error making first request: %v", err)
	}

	_, _ = io.ReadAll(resp1.Body)
	resp1.Body.Close()

	etag := resp1.Header.Get("ETag")
	if etag == "" {
		t.Fatal("No ETag in response")
	}

	// Second request with matching ETag - should get 304
	req2, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req2.Header.Set("If-None-Match", etag)

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Error making second request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("Expected status 304, got %d", resp2.StatusCode)
	}

	// Non-entity headers should still be present
	if resp2.Header.Get("Date") == "" {
		t.Error("Date header should be present in 304 response")
	}

	// Entity headers should be absent
	if resp2.Header.Get("Content-Type") != "" {
		t.Error("Content-Type header should not be present in 304 response")
	}

	// ETag should be present
	if resp2.Header.Get("ETag") != etag {
		t.Errorf("ETag header should be present in 304 response, expected %q, got %q",
			etag, resp2.Header.Get("ETag"))
	}

	// Body should be empty
	body2, _ := io.ReadAll(resp2.Body)
	if len(body2) > 0 {
		t.Errorf("Body should be empty in 304 response, got %q", string(body2))
	}
}

func TestWithVariousContentSizes(t *testing.T) {
	sizes := []int{0, 10, 100, 1000, 10000}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("Content size %d", size), func(t *testing.T) {
			content := strings.Repeat("X", size)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(content))
			})

			middleware := Auto()
			server := httptest.NewServer(middleware(handler))
			defer server.Close()

			resp, err := http.Get(server.URL)
			if err != nil {
				t.Fatalf("Error making request: %v", err)
			}
			defer resp.Body.Close()

			// Empty content (size 0) should not generate an ETag
			if size == 0 {
				if resp.Header.Get("ETag") != "" {
					t.Error("Empty response should not have an ETag")
				}
				return
			}

			if resp.Header.Get("ETag") == "" {
				t.Error("Response should have an ETag")
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Error reading response: %v", err)
			}

			if string(body) != content {
				t.Errorf("Response body mismatch, expected length %d, got %d",
					len(content), len(body))
			}
		})
	}
}

func TestConcurrentRequests(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	middleware := Auto()
	server := httptest.NewServer(middleware(handler))
	defer server.Close()

	var wg sync.WaitGroup
	concurrency := 10
	wg.Add(concurrency)

	for range concurrency {
		go func() {
			defer wg.Done()

			resp, err := http.Get(server.URL)
			if err != nil {
				t.Errorf("Error making request: %v", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			if resp.Header.Get("ETag") == "" {
				t.Error("Response should have an ETag")
			}

			_, err = io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("Error reading response: %v", err)
			}
		}()
	}

	wg.Wait()
}

// Test request methods other than GET/HEAD
func TestNonGetHeadMethods(t *testing.T) {
	methods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Response body"))
			})

			middleware := Auto()
			server := httptest.NewServer(middleware(handler))
			defer server.Close()

			req, err := http.NewRequest(method, server.URL, nil)
			if err != nil {
				t.Fatalf("Error creating request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Error making request: %v", err)
			}
			defer resp.Body.Close()

			// Non-GET/HEAD methods should not have ETag
			if resp.Header.Get("ETag") != "" {
				t.Errorf("%s request should not have ETag", method)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Error reading response: %v", err)
			}

			if string(body) != "Response body" {
				t.Errorf("Response body mismatch, expected %q, got %q",
					"Response body", string(body))
			}
		})
	}
}

// Test behavior with HTTP status codes
func TestStatusCodes(t *testing.T) {
	statusCodes := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNoContent,
		http.StatusBadRequest,
		http.StatusNotFound,
		http.StatusInternalServerError,
	}

	for _, code := range statusCodes {
		t.Run(fmt.Sprintf("Status %d", code), func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				if code != http.StatusNoContent {
					w.Write([]byte("Response body"))
				}
			})

			middleware := Auto()
			server := httptest.NewServer(middleware(handler))
			defer server.Close()

			resp, err := http.Get(server.URL)
			if err != nil {
				t.Fatalf("Error making request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != code {
				t.Errorf("Expected status %d, got %d", code, resp.StatusCode)
			}

			// Only 200 OK responses should get ETags
			if code == http.StatusOK && resp.Header.Get("ETag") == "" {
				t.Error("200 OK response should have an ETag")
			} else if code != http.StatusOK && resp.Header.Get("ETag") != "" {
				t.Errorf("Status %d response should not have an ETag", code)
			}
		})
	}
}

// Test ContentLength header
func TestContentLengthHeader(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Response with known length"))
	})

	middleware := Auto()
	server := httptest.NewServer(middleware(handler))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	contentLength := resp.Header.Get("Content-Length")
	expectedLen := fmt.Sprintf("%d", len("Response with known length"))
	if contentLength != expectedLen { //
		t.Errorf("Expected Content-Length: %s, got %q", expectedLen, contentLength)
	}
}

// Test multiple ETags in If-None-Match header
func TestMultipleETags(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	middleware := Auto()
	server := httptest.NewServer(middleware(handler))
	defer server.Close()

	// First request to get the actual ETag
	resp1, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Error making first request: %v", err)
	}
	defer resp1.Body.Close()

	etag := resp1.Header.Get("ETag")
	if etag == "" {
		t.Fatal("No ETag in response")
	}

	// Second request with multiple ETags including the actual one
	req2, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req2.Header.Set("If-None-Match", `"invalid-tag", `+etag+`, "another-invalid-tag"`)

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Error making second request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusNotModified {
		t.Errorf("Expected status 304, got %d", resp2.StatusCode)
	}
}

// Test with headers that should prevent ETags
func TestPreventETagHeaders(t *testing.T) {
	tests := []struct {
		header string
		value  string
	}{
		{
			header: "Cache-Control",
			value:  "no-store",
		},
		{
			header: "Cache-Control",
			value:  "private, no-store",
		},
		{
			header: "Set-Cookie",
			value:  "session=abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.header+" : "+tt.value, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set(tt.header, tt.value)
				w.Write([]byte("Response body"))
			})

			middleware := Auto()
			server := httptest.NewServer(middleware(handler))
			defer server.Close()

			resp, err := http.Get(server.URL)
			if err != nil {
				t.Fatalf("Error making request: %v", err)
			}
			defer resp.Body.Close()

			if resp.Header.Get("ETag") != "" {
				t.Errorf("Response with %s: %s should not have an ETag",
					tt.header, tt.value)
			}

			if resp.Header.Get(tt.header) != tt.value {
				t.Errorf("Expected %s: %s, got %s",
					tt.header, tt.value, resp.Header.Get(tt.header))
			}
		})
	}
}

// Test wildcard ETag matching
func TestWildcardETag(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	middleware := Auto()
	server := httptest.NewServer(middleware(handler))
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.Header.Set("If-None-Match", "*")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("Expected status 304 with wildcard ETag, got %d", resp.StatusCode)
	}
}

// Test Content-Length preservation for NotModified responses
func TestNotModifiedPreservesContentLength(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "123")
		w.Write([]byte("Hello, World!"))
	})

	middleware := Auto()
	server := httptest.NewServer(middleware(handler))
	defer server.Close()

	// First request to get the ETag
	resp1, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Error making first request: %v", err)
	}
	etag := resp1.Header.Get("ETag")
	resp1.Body.Close()

	if etag == "" {
		t.Fatal("No ETag in response")
	}

	// Second request with matching ETag
	req2, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req2.Header.Set("If-None-Match", etag)

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Error making second request: %v", err)
	}
	defer resp2.Body.Close()

	// 304 Not Modified should not have Content-Length
	if resp2.Header.Get("Content-Length") != "" {
		t.Errorf("304 response should not have Content-Length, got %s",
			resp2.Header.Get("Content-Length"))
	}
}

// Test ETag format compliance with RFC 7232
func TestETagFormat(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	// Test strong ETag
	t.Run("Strong ETag", func(t *testing.T) {
		middleware := Auto(&Config{
			Strong: true,
		})
		server := httptest.NewServer(middleware(handler))
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Error making request: %v", err)
		}
		defer resp.Body.Close()

		etag := resp.Header.Get("ETag")
		if etag == "" {
			t.Fatal("No ETag in response")
		}

		// Strong ETag should be "hash"
		if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
			t.Errorf("Strong ETag should be wrapped in quotes, got %q", etag)
		}

		if strings.HasPrefix(etag, `W/"`) {
			t.Errorf("Strong ETag should not have W/ prefix, got %q", etag)
		}
	})

	// Test weak ETag
	t.Run("Weak ETag", func(t *testing.T) {
		middleware := Auto(&Config{
			Strong: false,
		})
		server := httptest.NewServer(middleware(handler))
		defer server.Close()

		resp, err := http.Get(server.URL)
		if err != nil {
			t.Fatalf("Error making request: %v", err)
		}
		defer resp.Body.Close()

		etag := resp.Header.Get("ETag")
		if etag == "" {
			t.Fatal("No ETag in response")
		}

		// Weak ETag should be W/"hash"
		if !strings.HasPrefix(etag, `W/"`) || !strings.HasSuffix(etag, `"`) {
			t.Errorf("Weak ETag should be in format W/\"hash\", got %q", etag)
		}
	})
}

// Test custom hash function
func TestCustomHash(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	// Test with different hash functions
	hashFuncs := map[string]func() hash.Hash{
		"SHA-1":   sha1.New,
		"MD5":     md5.New,
		"SHA-256": sha256.New,
	}

	for name, hashFunc := range hashFuncs {
		t.Run(name, func(t *testing.T) {
			middleware := Auto(&Config{
				Hash: hashFunc,
			})
			server := httptest.NewServer(middleware(handler))
			defer server.Close()

			resp, err := http.Get(server.URL)
			if err != nil {
				t.Fatalf("Error making request with %s: %v", name, err)
			}
			defer resp.Body.Close()

			etag := resp.Header.Get("ETag")
			if etag == "" {
				t.Fatalf("No ETag in response with %s", name)
			}

			// Different hash functions should produce different ETags
			directHash := hashFunc()
			directHash.Write([]byte("Hello, World!"))
			expectedTag := `W/"` + hex.EncodeToString(directHash.Sum(nil)) + `"`

			if etag != expectedTag {
				t.Errorf("With %s, expected ETag %q, got %q", name, expectedTag, etag)
			}
		})
	}
}

func TestSkipFunc(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	middleware := Auto(&Config{
		SkipFunc: func(r *http.Request) bool {
			return r.URL.Path == "/skip"
		},
	})
	server := httptest.NewServer(middleware(handler))
	defer server.Close()

	// Request to /skip should not have ETag
	resp1, err := http.Get(server.URL + "/skip")
	if err != nil {
		t.Fatalf("Error making request to /skip: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.Header.Get("ETag") != "" {
		t.Error("/skip request should not have ETag")
	}

	// Request to / should have ETag
	resp2, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Error making request to /: %v", err)
	}

	defer resp2.Body.Close()

	if resp2.Header.Get("ETag") == "" {
		t.Error("/ request should have ETag")
	}
}

// __TODO
func TestXVormaBuildIdHeaderChangesEtag(t *testing.T) {}
