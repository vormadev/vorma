package vormabuild

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestReloadEndpointURL(t *testing.T) {
	got := reloadEndpointURL(8080, "/__vorma/reload-routes")
	want := "http://localhost:8080/__vorma/reload-routes"
	if got != want {
		t.Fatalf("reloadEndpointURL() = %q, want %q", got, want)
	}
}

func TestNewReloadEndpointRequest(t *testing.T) {
	request, err := newReloadEndpointRequest(context.Background(), "http://localhost:8080/path")
	if err != nil {
		t.Fatalf("newReloadEndpointRequest returned error: %v", err)
	}
	if request.Method != http.MethodGet {
		t.Fatalf("request method = %q, want %q", request.Method, http.MethodGet)
	}
	if request.URL.String() != "http://localhost:8080/path" {
		t.Fatalf("request URL = %q, want %q", request.URL.String(), "http://localhost:8080/path")
	}
}

func TestValidateReloadEndpointStatus(t *testing.T) {
	if err := validateReloadEndpointStatus(http.StatusOK); err != nil {
		t.Fatalf("validateReloadEndpointStatus(200) returned error: %v", err)
	}

	err := validateReloadEndpointStatus(http.StatusInternalServerError)
	if err == nil {
		t.Fatal("expected non-200 status to return error")
	}
	if !strings.Contains(err.Error(), "endpoint returned 500") {
		t.Fatalf("error = %q, expected status message", err)
	}
}
