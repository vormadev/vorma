package tooling

import (
	"errors"
	"flag"
	"github.com/vormadev/vorma/wave/tooling/builder"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/cli"
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

func buildCLIExecutionDependenciesForTests(
	cfg *wave.ParsedConfig,
) cli.ExecutionDependencies {
	return cli.ExecutionDependencies{
		RunDev: func(log *slog.Logger) error {
			return RunDev(cfg, log)
		},
		RunBuild: func(log *slog.Logger, compileGo bool) error {
			builder := toolingbuilder.NewBuilder(cfg, log)
			defer builder.Close()
			return builder.Build(toolingbuilder.BuildOpts{CompileGo: compileGo})
		},
	}
}

func TestParseCLIOptions(t *testing.T) {
	parsedCLIOptions, err := cli.ParseCLIOptions(
		[]string{"-dev", "-hook", "-no-binary"},
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
}

func TestParseCLIOptions_UnknownFlagReturnsError(t *testing.T) {
	_, err := cli.ParseCLIOptions([]string{"-not-a-real-flag"})
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
	err := cli.BuildWaveWithHookOptions(
		newDiscardLogger(),
		cli.CLIOptions{HookOnly: true, IsDev: true},
		func(isDev bool) error {
			called = true
			if !isDev {
				t.Fatal("expected hook to receive IsDev=true")
			}
			return nil
		},
		buildCLIExecutionDependenciesForTests(cfg),
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

	err := cli.BuildWaveWithHookFromArgs(
		newDiscardLogger(),
		[]string{"-hook"},
		func(bool) error {
			return errors.New("hook failure")
		},
		buildCLIExecutionDependenciesForTests(cfg),
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

	err := cli.BuildWaveWithHookFromArgs(
		newDiscardLogger(),
		[]string{"-no-binary"},
		nil,
		buildCLIExecutionDependenciesForTests(cfg),
	)
	if err != nil {
		t.Fatalf("BuildWaveWithHookFromArgs returned error: %v", err)
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
			if err := cli.BuildWaveWithHookFromArgs(
				newDiscardLogger(),
				os.Args[1:],
				nil,
				buildCLIExecutionDependenciesForTests(cfg),
			); err != nil {
				t.Fatalf("did not expect BuildWave to return error: %v", err)
			}

			if flag.CommandLine != customGlobalFlagSet {
				t.Fatal("expected global flag.CommandLine pointer to remain unchanged")
			}
			if customGlobalFlagSet.Parsed() {
				t.Fatal("expected global flag.CommandLine parser to remain unparsed")
			}
		},
	)
}
