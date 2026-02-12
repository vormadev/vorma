package tooling

import (
	"errors"
	"flag"
	"os"
	"testing"
)

func withResettableCommandLine(t *testing.T, args []string, fn func()) {
	t.Helper()

	originalArgs := os.Args
	originalCommandLine := flag.CommandLine

	os.Args = args
	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)

	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalCommandLine
	}()

	fn()
}

func TestBuildWaveWithHook_HookModeInvokesHookWithDevFlag(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())

	withResettableCommandLine(t, []string{"wave-tooling-test", "-hook", "-dev"}, func() {
		called := false
		BuildWaveWithHook(cfg, newDiscardLogger(), func(isDev bool) error {
			called = true
			if !isDev {
				t.Fatal("expected hook to receive isDev=true when -dev is provided")
			}
			return nil
		})
		if !called {
			t.Fatal("expected hook mode to invoke provided hook")
		}
	})
}

func TestBuildWaveWithHook_HookErrorPanics(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())

	withResettableCommandLine(t, []string{"wave-tooling-test", "-hook"}, func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected hook error to panic")
			}
		}()

		BuildWaveWithHook(cfg, newDiscardLogger(), func(bool) error {
			return errors.New("hook failure")
		})
	})
}

func TestBuildWave_NoBinaryFlagRunsBuildWithoutCompile(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	withResettableCommandLine(t, []string{"wave-tooling-test", "-no-binary"}, func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("did not expect BuildWave to panic in no-binary mode: %v", recovered)
			}
		}()
		BuildWave(cfg, newDiscardLogger())
	})
}
