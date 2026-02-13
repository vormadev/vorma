package wave

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestConfigProviderRunnerRun(t *testing.T) {
	runner := NewConfigProviderRunner()

	t.Run("runs provider command and parses payload", func(t *testing.T) {
		payload, err := runner.Run(context.Background(), ConfigProviderInvocation{
			Command: "sh",
			Args: []string{
				"-c",
				`printf '%s' '{"version":1,"config":{"Core":{"DistDir":"dist"}},"dependencies":{"files":["b","a"],"env":["PORT"]}}'`,
			},
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if payload.Version != 1 {
			t.Fatalf("version = %d, want 1", payload.Version)
		}
		if got := string(payload.Config); got != `{"Core":{"DistDir":"dist"}}` {
			t.Fatalf("config = %q", got)
		}
		if got := strings.Join(payload.Dependencies.Files, ","); got != "a,b" {
			t.Fatalf("files = %q, want a,b", got)
		}
	})

	t.Run("fails when command is missing", func(t *testing.T) {
		_, err := runner.Run(context.Background(), ConfigProviderInvocation{})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "provider command is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails when stdout exceeds max", func(t *testing.T) {
		_, err := runner.Run(context.Background(), ConfigProviderInvocation{
			Command:      "sh",
			Args:         []string{"-c", `printf '%s' 'abcdefghijklmnopqrstuvwxyz'`},
			MaxStdoutBts: 10,
		})
		if err == nil {
			t.Fatal("expected overflow error")
		}
		if !strings.Contains(err.Error(), "provider stdout exceeded") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("times out long-running provider", func(t *testing.T) {
		_, err := runner.Run(context.Background(), ConfigProviderInvocation{
			Command: "sh",
			Args:    []string{"-c", "sleep 2"},
			Timeout: 50 * time.Millisecond,
		})
		if err == nil {
			t.Fatal("expected timeout error")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
