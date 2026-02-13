package tooling

import (
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

func withTemporaryCommandLineState(
	t *testing.T,
	args []string,
	commandLine *flag.FlagSet,
	fn func(),
) {
	t.Helper()

	originalArgs := os.Args
	originalCommandLine := flag.CommandLine

	os.Args = args
	flag.CommandLine = commandLine

	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalCommandLine
	}()

	fn()
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer

	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	outputBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return string(outputBytes)
}

func TestParseCLIOptions(t *testing.T) {
	parsedCLIOptions, err := ParseCLIOptions(
		[]string{"-dev", "-hook", "-no-binary", "-explain", "-doctor"},
	)
	if err != nil {
		t.Fatalf("ParseCLIOptions returned error: %v", err)
	}

	if !parsedCLIOptions.IsDev {
		t.Fatal("expected IsDev=true")
	}
	if !parsedCLIOptions.HookOnly {
		t.Fatal("expected HookOnly=true")
	}
	if !parsedCLIOptions.NoBinary {
		t.Fatal("expected NoBinary=true")
	}
	if !parsedCLIOptions.Explain {
		t.Fatal("expected Explain=true")
	}
	if !parsedCLIOptions.Doctor {
		t.Fatal("expected Doctor=true")
	}
}

func TestParseCLIOptions_UnknownFlagReturnsError(t *testing.T) {
	_, err := ParseCLIOptions([]string{"-not-a-real-flag"})
	if err == nil {
		t.Fatal("expected ParseCLIOptions to fail for unknown flag")
	}
	if !strings.Contains(err.Error(), "parse CLI options") {
		t.Fatalf("unexpected parse error: %v", err)
	}
}

func TestBuildWaveWithHookOptions_HookModeInvokesHookWithDevFlag(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())

	called := false
	err := BuildWaveWithHookOptions(
		cfg,
		newDiscardLogger(),
		CLIOptions{HookOnly: true, IsDev: true},
		func(isDev bool) error {
			called = true
			if !isDev {
				t.Fatal("expected hook to receive IsDev=true")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("BuildWaveWithHookOptions returned error: %v", err)
	}
	if !called {
		t.Fatal("expected hook mode to invoke provided hook")
	}
}

func TestBuildWaveWithHookFromArgs_HookErrorIsReturned(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())

	err := BuildWaveWithHookFromArgs(
		cfg,
		newDiscardLogger(),
		[]string{"-hook"},
		func(bool) error {
			return errors.New("hook failure")
		},
	)
	if err == nil {
		t.Fatal("expected hook error to be returned")
	}
	if !strings.Contains(err.Error(), "run hook") {
		t.Fatalf("unexpected hook error: %v", err)
	}
}

func TestBuildWaveWithHookFromArgs_NoBinaryFlagRunsBuildWithoutCompile(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	err := BuildWaveWithHookFromArgs(
		cfg,
		newDiscardLogger(),
		[]string{"-no-binary"},
		nil,
	)
	if err != nil {
		t.Fatalf("BuildWaveWithHookFromArgs returned error: %v", err)
	}
}

func TestBuildWaveWithHookOptions_ExplainModePrintsReportAndSkipsBuild(t *testing.T) {
	output := captureStdout(t, func() {
		err := BuildWaveWithHookOptions(
			nil,
			newDiscardLogger(),
			CLIOptions{Explain: true},
			nil,
		)
		if err != nil {
			t.Fatalf("BuildWaveWithHookOptions returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Wave Explain") {
		t.Fatalf("expected explain output, got %q", output)
	}
}

func TestBuildWaveWithHookOptions_DoctorModeReturnsErrorWhenIssuesExist(t *testing.T) {
	var returnedError error
	output := captureStdout(t, func() {
		returnedError = BuildWaveWithHookOptions(
			nil,
			newDiscardLogger(),
			CLIOptions{Doctor: true},
			nil,
		)
	})

	if returnedError == nil {
		t.Fatal("expected doctor mode to return an error for invalid config")
	}
	if !strings.Contains(returnedError.Error(), "wave doctor found issues") {
		t.Fatalf("unexpected doctor error: %v", returnedError)
	}
	if !strings.Contains(output, "Wave Doctor") {
		t.Fatalf("expected doctor output, got %q", output)
	}
}

func TestBuildWaveWithHookOptions_DoctorModeReturnsNilWhenNoIssuesExist(t *testing.T) {
	root := t.TempDir()
	cfg := newParsedConfigForToolingTestsAtRoot(root)
	cfg.Core.ServerOnlyMode = true

	output := captureStdout(t, func() {
		err := BuildWaveWithHookOptions(
			cfg,
			newDiscardLogger(),
			CLIOptions{Doctor: true},
			nil,
		)
		if err != nil {
			t.Fatalf("BuildWaveWithHookOptions returned error: %v", err)
		}
	})

	if !strings.Contains(output, "issues: none") {
		t.Fatalf("expected doctor no-issues output, got %q", output)
	}
}

func TestBuildWave_DoesNotUseGlobalFlagCommandLineParser(t *testing.T) {
	cfg := newParsedConfigForToolingTestsAtRoot(t.TempDir())
	cfg.Core.ServerOnlyMode = true

	customGlobalFlagSet := flag.NewFlagSet("global", flag.ContinueOnError)
	customGlobalFlagSet.Bool("sentinel", false, "sentinel")

	withTemporaryCommandLineState(
		t,
		[]string{"wave-tooling-test", "-no-binary"},
		customGlobalFlagSet,
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("did not expect BuildWave to panic: %v", recovered)
				}
			}()

			BuildWave(cfg, newDiscardLogger())

			if flag.CommandLine != customGlobalFlagSet {
				t.Fatal("expected global flag.CommandLine pointer to remain unchanged")
			}
			if customGlobalFlagSet.Parsed() {
				t.Fatal("expected global flag.CommandLine parser to remain unparsed")
			}
		},
	)
}
