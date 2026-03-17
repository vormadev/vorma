package wavebuild

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func TestPollHTTPReadyEndpoint_WaitsForRequestedEndpoint(
	t *testing.T,
) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/@vite/client" {
				t.Fatalf("got path %q, want %q", r.URL.Path, "/@vite/client")
			}
			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	server_url, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	port, err := strconv.Atoi(server_url.Port())
	if err != nil {
		t.Fatalf("strconv.Atoi: %v", err)
	}

	var exit_err error
	done := make(chan struct{})

	if err := poll_http_ready_endpoint(
		done,
		&exit_err,
		"http://127.0.0.1:"+strconv.Itoa(port)+"/@vite/client",
		"vite dev server",
		vite_ready_timeout,
	); err != nil {
		t.Fatalf("poll_http_ready_endpoint: %v", err)
	}
}

func TestPollHTTPReadyEndpoint_ReturnsEarlyProcessExit(
	t *testing.T,
) {
	done := make(chan struct{})
	close(done)

	exit_err := errors.New("boom")

	err := poll_http_ready_endpoint(
		done,
		&exit_err,
		"http://127.0.0.1:5173/@vite/client",
		"vite dev server",
		vite_ready_timeout,
	)
	if err == nil {
		t.Fatal("poll_http_ready_endpoint unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "vite dev server exited during startup") {
		t.Fatalf("poll_http_ready_endpoint error = %q", err)
	}
}
