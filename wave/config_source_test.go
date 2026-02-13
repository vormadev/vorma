package wave

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStaticConfigSourceLoadConfig(t *testing.T) {
	t.Run("loads in-memory config and dependencies", func(t *testing.T) {
		source := NewStaticConfigSource([]byte(`{"Core":{"DistDir":"dist"}}`))
		source.Dependencies = ConfigProviderDependencies{
			Files: []string{"backend/config/config.go"},
		}
		source.Fingerprint = "static-fingerprint"

		loadedConfig, err := source.LoadConfig(context.Background())
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		if got := string(loadedConfig.ConfigJSON); got != `{"Core":{"DistDir":"dist"}}` {
			t.Fatalf("config JSON = %q", got)
		}
		if got, want := loadedConfig.Dependencies.Files, []string{"backend/config/config.go"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("dependencies.files = %v, want %v", got, want)
		}
		if loadedConfig.Fingerprint != "static-fingerprint" {
			t.Fatalf("fingerprint = %q, want static-fingerprint", loadedConfig.Fingerprint)
		}
	})

	t.Run("fails when config JSON is missing", func(t *testing.T) {
		source := NewStaticConfigSource(nil)
		_, err := source.LoadConfig(context.Background())
		if err == nil {
			t.Fatal("expected error for missing config JSON")
		}
		if !strings.Contains(err.Error(), "config JSON is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("supports late-bound config JSON loading", func(t *testing.T) {
		source := NewStaticConfigSource(nil)
		source.resolveSource = func() ([]byte, error) {
			return []byte(`{"Core":{"DistDir":"dist"}}`), nil
		}

		loadedConfig, err := source.LoadConfig(context.Background())
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got := string(loadedConfig.ConfigJSON); got != `{"Core":{"DistDir":"dist"}}` {
			t.Fatalf("config JSON = %q", got)
		}
	})

	t.Run("returns source load errors", func(t *testing.T) {
		expectedErr := errors.New("resolve fail")
		source := NewStaticConfigSource(nil)
		source.resolveSource = func() ([]byte, error) {
			return nil, expectedErr
		}

		_, err := source.LoadConfig(context.Background())
		if err == nil {
			t.Fatal("expected resolve error")
		}
		if !strings.Contains(err.Error(), "load static config source") {
			t.Fatalf("unexpected error: %v", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected wrapped error %v, got %v", expectedErr, err)
		}
	})
}

func TestProviderConfigSourceLoadConfig(t *testing.T) {
	t.Run("loads provider config payload", func(t *testing.T) {
		source := NewProviderConfigSource(ConfigProviderInvocation{
			Command: "sh",
			Args: []string{
				"-c",
				`printf '%s' '{"version":1,"config":{"Core":{"DistDir":"dist"}},"dependencies":{"files":["backend/wave.config.go"],"env":["WAVE_MODE"]},"fingerprint":"abc"}'`,
			},
		})

		loadedConfig, err := source.LoadConfig(context.Background())
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		if got := string(loadedConfig.ConfigJSON); got != `{"Core":{"DistDir":"dist"}}` {
			t.Fatalf("config JSON = %q", got)
		}
		if got, want := loadedConfig.Dependencies.Files, []string{"backend/wave.config.go"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("dependencies.files = %v, want %v", got, want)
		}
		if got, want := loadedConfig.Dependencies.Env, []string{"WAVE_MODE"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("dependencies.env = %v, want %v", got, want)
		}
		if loadedConfig.Fingerprint != "abc" {
			t.Fatalf("fingerprint = %q, want abc", loadedConfig.Fingerprint)
		}
	})

	t.Run("resolves dependency paths relative to provider working dir", func(t *testing.T) {
		workingDir := t.TempDir()
		source := NewProviderConfigSource(ConfigProviderInvocation{
			Command:    "sh",
			WorkingDir: workingDir,
			Args: []string{
				"-c",
				`printf '%s' '{"version":1,"config":{"Core":{"DistDir":"dist"}},"dependencies":{"files":["backend/wave.config.go"],"globs":["backend/**/*.go"]}}'`,
			},
		})

		loadedConfig, err := source.LoadConfig(context.Background())
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		expectedFileDependency := filepath.ToSlash(filepath.Join(workingDir, "backend/wave.config.go"))
		if got, want := loadedConfig.Dependencies.Files, []string{expectedFileDependency}; !reflect.DeepEqual(got, want) {
			t.Fatalf("dependencies.files = %v, want %v", got, want)
		}

		expectedGlobDependency := filepath.ToSlash(filepath.Join(workingDir, "backend/**/*.go"))
		if got, want := loadedConfig.Dependencies.Globs, []string{expectedGlobDependency}; !reflect.DeepEqual(got, want) {
			t.Fatalf("dependencies.globs = %v, want %v", got, want)
		}
	})
}
