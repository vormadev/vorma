package vormabuild

import (
	"errors"
	"strings"
	"testing"
)

func TestGenerateBuildIDWithPrefix(t *testing.T) {
	t.Run("prefixes generated suffix", func(t *testing.T) {
		buildID, err := generateBuildIDWithPrefix("dev_", func() (string, error) {
			return "abc123", nil
		})
		if err != nil {
			t.Fatalf("generateBuildIDWithPrefix returned error: %v", err)
		}
		if buildID != "dev_abc123" {
			t.Fatalf("build ID = %q, want %q", buildID, "dev_abc123")
		}
	})

	t.Run("wraps suffix generation error", func(t *testing.T) {
		expectedErr := errors.New("id generation failed")
		_, err := generateBuildIDWithPrefix("dev_", func() (string, error) {
			return "", expectedErr
		})
		if err == nil {
			t.Fatal("expected generateBuildIDWithPrefix to return error")
		}
		if !strings.Contains(err.Error(), "generate build ID") {
			t.Fatalf("error = %q, expected generate-build-id context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped suffix generation error", err)
		}
	})
}
