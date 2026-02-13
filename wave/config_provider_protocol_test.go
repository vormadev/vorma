package wave

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseConfigProviderPayload(t *testing.T) {
	t.Run("parses valid payload and normalizes dependencies", func(t *testing.T) {
		payload, err := ParseConfigProviderPayload([]byte(`{
			"version": 1,
			"config": {"Core":{"DistDir":"dist"}},
			"dependencies": {
				"files": [" b.json ", "a.json", "a.json", ""],
				"globs": ["src/**/*.go", "src/**/*.go"],
				"env": [" WAVE_MODE ", "PORT", "PORT"]
			},
			"fingerprint": "abc123"
		}`))
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		if payload.Version != 1 {
			t.Fatalf("version = %d, want 1", payload.Version)
		}
		if payload.Fingerprint != "abc123" {
			t.Fatalf("fingerprint = %q, want abc123", payload.Fingerprint)
		}

		if got, want := payload.Dependencies.Files, []string{"a.json", "b.json"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("dependencies.files = %v, want %v", got, want)
		}
		if got, want := payload.Dependencies.Globs, []string{"src/**/*.go"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("dependencies.globs = %v, want %v", got, want)
		}
		if got, want := payload.Dependencies.Env, []string{"PORT", "WAVE_MODE"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("dependencies.env = %v, want %v", got, want)
		}
	})

	t.Run("fails when version is missing", func(t *testing.T) {
		_, err := ParseConfigProviderPayload([]byte(`{"config":{"Core":{}}}`))
		if err == nil {
			t.Fatal("expected error for missing version")
		}
		if !strings.Contains(err.Error(), "version must be > 0") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails when config is missing", func(t *testing.T) {
		_, err := ParseConfigProviderPayload([]byte(`{"version":1}`))
		if err == nil {
			t.Fatal("expected error for missing config")
		}
		if !strings.Contains(err.Error(), "config is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails when config is not a JSON object", func(t *testing.T) {
		_, err := ParseConfigProviderPayload([]byte(`{"version":1,"config":["x"]}`))
		if err == nil {
			t.Fatal("expected error for non-object config")
		}
		if !strings.Contains(err.Error(), "config must be a JSON object") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails on invalid JSON payload", func(t *testing.T) {
		_, err := ParseConfigProviderPayload([]byte(`{`))
		if err == nil {
			t.Fatal("expected parse error")
		}
		if !strings.Contains(err.Error(), "parse provider payload") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
