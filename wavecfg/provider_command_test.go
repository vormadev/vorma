package wavecfg

import (
	"path/filepath"
	"testing"
)

func TestGoRunProviderInvocation(t *testing.T) {
	invocation := GoRunProviderInvocation("backend/cmd/config")
	if invocation.Command != "go" {
		t.Fatalf("command = %q, want go", invocation.Command)
	}
	if len(invocation.Args) != 2 {
		t.Fatalf("len(args) = %d, want 2", len(invocation.Args))
	}
	if invocation.Args[0] != "run" {
		t.Fatalf("args[0] = %q, want run", invocation.Args[0])
	}
	if invocation.Args[1] != "./backend/cmd/config" {
		t.Fatalf("args[1] = %q, want ./backend/cmd/config", invocation.Args[1])
	}
}

func TestGoRunProviderInvocationAbsolutePathPanics(t *testing.T) {
	absoluteProviderCommandPath := filepath.Join(t.TempDir(), "backend", "cmd", "config")

	defer func() {
		if recover() == nil {
			t.Fatal("expected absolute provider command package path to panic")
		}
	}()

	_ = GoRunProviderInvocation(absoluteProviderCommandPath)
}
