package buildentry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
	"github.com/vormadev/vorma/wave/waveframework"
)

func TestParseBuildCommandOptions(t *testing.T) {
	t.Run("defaults when no flags are provided", func(t *testing.T) {
		options, err := parseBuildCommandOptions(nil)
		if err != nil {
			t.Fatalf("parseBuildCommandOptions returned error: %v", err)
		}
		if options.runInDevelopmentMode {
			t.Fatal("expected runInDevelopmentMode to default to false")
		}
		if options.runHookOnly {
			t.Fatal("expected runHookOnly to default to false")
		}
		if options.runHookExecutionOnly {
			t.Fatal("expected runHookExecutionOnly to default to false")
		}
		if options.skipGoBinaryBuildStep {
			t.Fatal("expected skipGoBinaryBuildStep to default to false")
		}
	})

	t.Run("parses hook flags", func(t *testing.T) {
		options, err := parseBuildCommandOptions([]string{
			"--dev",
			"--hook",
			"--no-binary",
		})
		if err != nil {
			t.Fatalf("parseBuildCommandOptions returned error: %v", err)
		}
		if !options.runInDevelopmentMode {
			t.Fatal("expected runInDevelopmentMode to be true")
		}
		if !options.runHookOnly {
			t.Fatal("expected runHookOnly to be true")
		}
		if options.runHookExecutionOnly {
			t.Fatal("expected runHookExecutionOnly to be false")
		}
		if !options.skipGoBinaryBuildStep {
			t.Fatal("expected skipGoBinaryBuildStep to be true")
		}
	})

	t.Run("parses hook-inner flag", func(t *testing.T) {
		options, err := parseBuildCommandOptions([]string{"--dev", "--hook-inner"})
		if err != nil {
			t.Fatalf("parseBuildCommandOptions returned error: %v", err)
		}
		if !options.runInDevelopmentMode {
			t.Fatal("expected runInDevelopmentMode to be true")
		}
		if options.runHookOnly {
			t.Fatal("expected runHookOnly to be false")
		}
		if !options.runHookExecutionOnly {
			t.Fatal("expected runHookExecutionOnly to be true")
		}
	})

	t.Run("returns parse error for unknown flag", func(t *testing.T) {
		_, err := parseBuildCommandOptions([]string{"--definitely-unknown"})
		if err == nil {
			t.Fatal(
				"expected parseBuildCommandOptions to return an error for unknown flag",
			)
		}
		if !strings.Contains(err.Error(), "flag provided but not defined") {
			t.Fatalf("error = %q, expected unknown-flag parse message", err)
		}
	})

	t.Run("returns parse error for positional arguments", func(t *testing.T) {
		_, err := parseBuildCommandOptions(
			[]string{"--dev", "extra-positional-arg"},
		)
		if err == nil {
			t.Fatal(
				"expected parseBuildCommandOptions to return an error for positional arguments",
			)
		}
		if !strings.Contains(err.Error(), "unexpected positional arguments") {
			t.Fatalf(
				"error = %q, expected positional-argument parse message",
				err,
			)
		}
	})

	t.Run("returns parse error when hook flags are combined", func(t *testing.T) {
		_, err := parseBuildCommandOptions(
			[]string{"--hook", "--hook-inner"},
		)
		if err == nil {
			t.Fatal("expected parse error for combined --hook and --hook-inner")
		}
		if !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatalf("error = %q, expected combined-hook flag error", err)
		}
	})
}

func TestParseCommandOptions(t *testing.T) {
	t.Run("maps --hook-inner to public options", func(t *testing.T) {
		options, err := ParseCommandOptions([]string{"--dev", "--hook-inner"})
		if err != nil {
			t.Fatalf("ParseCommandOptions returned error: %v", err)
		}
		if !options.Dev {
			t.Fatal("expected Dev=true")
		}
		if options.HookOnly {
			t.Fatal("expected HookOnly=false")
		}
		if !options.HookExecutionOnly {
			t.Fatal("expected HookExecutionOnly=true")
		}
	})
}

func TestValidateBuildCommandHooks(t *testing.T) {
	validHooks := defaultBuildCommandHooks()

	t.Run("valid hooks return nil", func(t *testing.T) {
		if err := validateBuildCommandHooks(validHooks); err != nil {
			t.Fatalf("validateBuildCommandHooks returned error: %v", err)
		}
	})

	t.Run("configure hook is required", func(t *testing.T) {
		invalidHooks := validHooks
		invalidHooks.configureBuildEnvironment = nil
		err := validateBuildCommandHooks(invalidHooks)
		if err == nil {
			t.Fatal("expected error for missing configureBuildEnvironment hook")
		}
		if !strings.Contains(
			err.Error(),
			"configureBuildEnvironment is required",
		) {
			t.Fatalf("error = %q, expected missing-configure-hook message", err)
		}
	})

	t.Run("build hook is required", func(t *testing.T) {
		invalidHooks := validHooks
		invalidHooks.runBuildHook = nil
		err := validateBuildCommandHooks(invalidHooks)
		if err == nil {
			t.Fatal("expected error for missing runBuildHook hook")
		}
		if !strings.Contains(err.Error(), "runBuildHook is required") {
			t.Fatalf("error = %q, expected missing-build-hook message", err)
		}
	})

	t.Run("execution-only hook is required", func(t *testing.T) {
		invalidHooks := validHooks
		invalidHooks.runHookExecutionOnly = nil
		err := validateBuildCommandHooks(invalidHooks)
		if err == nil {
			t.Fatal("expected error for missing runHookExecutionOnly hook")
		}
		if !strings.Contains(err.Error(), "runHookExecutionOnly is required") {
			t.Fatalf("error = %q, expected missing-execution-hook message", err)
		}
	})

	t.Run("post processing hook is required", func(t *testing.T) {
		invalidHooks := validHooks
		invalidHooks.runProdHookPostProcessing = nil
		err := validateBuildCommandHooks(invalidHooks)
		if err == nil {
			t.Fatal("expected error for missing runProdHookPostProcessing hook")
		}
		if !strings.Contains(
			err.Error(),
			"runProdHookPostProcessing is required",
		) {
			t.Fatalf("error = %q, expected missing-post-hook message", err)
		}
	})

	t.Run("full build hook is required", func(t *testing.T) {
		invalidHooks := validHooks
		invalidHooks.runFullBuild = nil
		err := validateBuildCommandHooks(invalidHooks)
		if err == nil {
			t.Fatal("expected error for missing runFullBuild hook")
		}
		if !strings.Contains(err.Error(), "runFullBuild is required") {
			t.Fatalf(
				"error = %q, expected missing-full-build-hook message",
				err,
			)
		}
	})
}

func TestRunBuildCommand(t *testing.T) {
	t.Run("returns error when runtime is nil", func(t *testing.T) {
		err := runBuildCommand(
			nil,
			nil,
			defaultBuildCommandHooks(),
		)
		if err == nil {
			t.Fatal("expected runBuildCommand to return error for nil runtime")
		}
		if !strings.Contains(err.Error(), "vorma runtime is required") {
			t.Fatalf("error = %q, expected nil-runtime context", err)
		}
	})

	t.Run("returns error when required hooks are missing", func(t *testing.T) {
		err := runBuildCommand(
			&vormaruntime.Vorma{},
			nil,
			buildCommandHooks{},
		)
		if err == nil {
			t.Fatal(
				"expected runBuildCommand to return error for missing hooks",
			)
		}
		if !strings.Contains(
			err.Error(),
			"build command hook configureBuildEnvironment is required",
		) {
			t.Fatalf("error = %q, expected missing-hook context", err)
		}
	})

	t.Run(
		"parses flags and executes full build through provided hooks",
		func(t *testing.T) {
			var runFullBuildCalled bool
			err := runBuildCommand(
				&vormaruntime.Vorma{},
				[]string{"--dev", "--no-binary"},
				buildCommandHooks{
					configureBuildEnvironment: func(*vormaruntime.Vorma) {
						t.Fatal(
							"did not expect configureBuildEnvironment in non-hook mode",
						)
					},
					runBuildHook: func(*vormaruntime.Vorma, bool) error {
						t.Fatal("did not expect runBuildHook in non-hook mode")
						return nil
					},
					runHookExecutionOnly: func(*vormaruntime.Vorma, bool) error {
						t.Fatal("did not expect runHookExecutionOnly in non-hook mode")
						return nil
					},
					runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
						t.Fatal(
							"did not expect runProdHookPostProcessing in non-hook mode",
						)
						return nil
					},
					runFullBuild: func(_ *vormaruntime.Vorma, isDev bool, skipBinary bool) error {
						runFullBuildCalled = true
						if !isDev {
							t.Fatal(
								"expected parsed --dev flag to set isDev=true",
							)
						}
						if !skipBinary {
							t.Fatal(
								"expected parsed --no-binary flag to set skipBinary=true",
							)
						}
						return nil
					},
				},
			)
			if err != nil {
				t.Fatalf("runBuildCommand returned error: %v", err)
			}
			if !runFullBuildCalled {
				t.Fatal("expected runFullBuild to be called")
			}
		},
	)

	t.Run("returns parse error with context", func(t *testing.T) {
		err := runBuildCommand(
			&vormaruntime.Vorma{},
			[]string{"--not-a-real-flag"},
			defaultBuildCommandHooks(),
		)
		if err == nil {
			t.Fatal("expected runBuildCommand to return parse error")
		}
		if !strings.Contains(err.Error(), "parse build flags") {
			t.Fatalf("error = %q, expected parse context", err)
		}
	})
}

func TestDefaultBuildCommandHooks(t *testing.T) {
	t.Run("runBuildHook uses framework runner", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		hooks := defaultBuildCommandHooks()
		hooks.configureBuildEnvironment(app)

		frameworkRunnerCalled := false
		waveframework.StateForConfig(app.Wave.ParsedConfig()).RunBuildHook = func(
			commandExecutionContext context.Context,
			runInDevelopmentMode bool,
		) error {
			frameworkRunnerCalled = true
			if commandExecutionContext == nil {
				t.Fatal("expected non-nil command execution context")
			}
			if !runInDevelopmentMode {
				t.Fatal("expected runInDevelopmentMode=true")
			}
			return nil
		}

		if err := hooks.runBuildHook(app, true); err != nil {
			t.Fatalf("default runBuildHook returned error: %v", err)
		}
		if !frameworkRunnerCalled {
			t.Fatal("expected default runBuildHook to call framework runner")
		}
	})

	t.Run("runHookExecutionOnly uses buildinner", func(t *testing.T) {
		fixture := testkit.NewBuildTestFixture(t, nil)
		app := fixture.App

		t.Chdir(fixture.RootDir)
		testkit.WriteBootstrapStyleRoutesFixtureFiles(t)

		hooks := defaultBuildCommandHooks()
		if err := hooks.runHookExecutionOnly(app, true); err != nil {
			t.Fatalf("default runHookExecutionOnly returned error: %v", err)
		}

		if !app.IsDevMode() {
			t.Fatal("expected default runHookExecutionOnly to set app to dev mode")
		}
		if app.BuildID() == "" {
			t.Fatal("expected default runHookExecutionOnly to assign a build ID")
		}
	})
}

func TestBuildCommandExecutorRun(t *testing.T) {
	type observedCalls struct {
		configureCalled                     bool
		runBuildHookCalled                  bool
		runBuildHookDevelopmentMode         bool
		runHookExecutionOnlyCalled          bool
		runHookExecutionOnlyDevelopmentMode bool
		runProdHookPostProcessingCalled     bool
		runFullBuildCalled                  bool
		runFullBuildDevelopmentMode         bool
		runFullBuildSkipGoBinaryBuild       bool
	}

	newHooks := func(calls *observedCalls) buildCommandHooks {
		return buildCommandHooks{
			configureBuildEnvironment: func(*vormaruntime.Vorma) {
				calls.configureCalled = true
			},
			runBuildHook: func(_ *vormaruntime.Vorma, isDev bool) error {
				calls.runBuildHookCalled = true
				calls.runBuildHookDevelopmentMode = isDev
				return nil
			},
			runHookExecutionOnly: func(_ *vormaruntime.Vorma, isDev bool) error {
				calls.runHookExecutionOnlyCalled = true
				calls.runHookExecutionOnlyDevelopmentMode = isDev
				return nil
			},
			runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
				calls.runProdHookPostProcessingCalled = true
				return nil
			},
			runFullBuild: func(_ *vormaruntime.Vorma, isDev bool, skipBinary bool) error {
				calls.runFullBuildCalled = true
				calls.runFullBuildDevelopmentMode = isDev
				calls.runFullBuildSkipGoBinaryBuild = skipBinary
				return nil
			},
		}
	}

	runWithOptions := func(options buildCommandOptions, hooks buildCommandHooks) error {
		commandExecutor := newBuildCommandExecutor(&vormaruntime.Vorma{}, hooks)
		return commandExecutor.run(options)
	}

	t.Run("hook mode runs configure and build hook only", func(t *testing.T) {
		var calls observedCalls
		err := runWithOptions(
			buildCommandOptions{
				runInDevelopmentMode: true,
				runHookOnly:          true,
			},
			newHooks(&calls),
		)
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
		if !calls.configureCalled {
			t.Fatal("expected configureBuildEnvironment to be called")
		}
		if !calls.runBuildHookCalled {
			t.Fatal("expected runBuildHook to be called")
		}
		if !calls.runBuildHookDevelopmentMode {
			t.Fatal("expected runBuildHook to receive development mode=true")
		}
		if calls.runHookExecutionOnlyCalled {
			t.Fatal("did not expect runHookExecutionOnly in outer hook mode")
		}
		if calls.runProdHookPostProcessingCalled {
			t.Fatal("did not expect post processing in outer hook mode")
		}
		if calls.runFullBuildCalled {
			t.Fatal("did not expect runFullBuild in hook mode")
		}
	})

	t.Run("hook-inner dev mode skips post processing", func(t *testing.T) {
		var calls observedCalls
		err := runWithOptions(
			buildCommandOptions{
				runInDevelopmentMode: true,
				runHookExecutionOnly: true,
			},
			newHooks(&calls),
		)
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
		if !calls.configureCalled {
			t.Fatal("expected configureBuildEnvironment to be called")
		}
		if calls.runBuildHookCalled {
			t.Fatal("did not expect runBuildHook in hook-inner mode")
		}
		if !calls.runHookExecutionOnlyCalled {
			t.Fatal("expected runHookExecutionOnly to be called")
		}
		if !calls.runHookExecutionOnlyDevelopmentMode {
			t.Fatal(
				"expected runHookExecutionOnly to receive development mode=true",
			)
		}
		if calls.runProdHookPostProcessingCalled {
			t.Fatal("did not expect post processing in hook-inner dev mode")
		}
		if calls.runFullBuildCalled {
			t.Fatal("did not expect runFullBuild in hook-inner mode")
		}
	})

	t.Run("hook-inner prod mode runs post processing", func(t *testing.T) {
		var calls observedCalls
		err := runWithOptions(
			buildCommandOptions{
				runHookExecutionOnly: true,
			},
			newHooks(&calls),
		)
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
		if !calls.runHookExecutionOnlyCalled ||
			!calls.runProdHookPostProcessingCalled {
			t.Fatalf("unexpected call sequence: %#v", calls)
		}
	})

	t.Run("returns error when both hook flags are enabled", func(t *testing.T) {
		var calls observedCalls
		err := runWithOptions(
			buildCommandOptions{
				runHookOnly:          true,
				runHookExecutionOnly: true,
			},
			newHooks(&calls),
		)
		if err == nil {
			t.Fatal("expected conflict error")
		}
		if !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatalf("error=%q, expected combined hook-flag error", err)
		}
	})

	t.Run("hook mode wraps build hook errors", func(t *testing.T) {
		expectedErr := errors.New("hook failed")
		hooks := buildCommandHooks{
			configureBuildEnvironment: func(*vormaruntime.Vorma) {},
			runBuildHook: func(*vormaruntime.Vorma, bool) error {
				return expectedErr
			},
			runHookExecutionOnly: func(*vormaruntime.Vorma, bool) error {
				t.Fatal("did not expect runHookExecutionOnly")
				return nil
			},
			runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
				t.Fatal("did not expect post processing when build hook fails")
				return nil
			},
			runFullBuild: func(*vormaruntime.Vorma, bool, bool) error {
				t.Fatal("did not expect full build in hook mode")
				return nil
			},
		}
		err := runWithOptions(buildCommandOptions{runHookOnly: true}, hooks)
		if err == nil {
			t.Fatal("expected hook error")
		}
		if !strings.Contains(err.Error(), "build hook failed") {
			t.Fatalf("error = %q, expected build-hook context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped hook error", err)
		}
	})

	t.Run("hook-inner mode wraps execution errors", func(t *testing.T) {
		expectedErr := errors.New("hook execution failed")
		hooks := buildCommandHooks{
			configureBuildEnvironment: func(*vormaruntime.Vorma) {},
			runBuildHook: func(*vormaruntime.Vorma, bool) error {
				t.Fatal("did not expect runBuildHook")
				return nil
			},
			runHookExecutionOnly: func(*vormaruntime.Vorma, bool) error {
				return expectedErr
			},
			runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
				t.Fatal("did not expect post processing when execution hook fails")
				return nil
			},
			runFullBuild: func(*vormaruntime.Vorma, bool, bool) error {
				t.Fatal("did not expect full build in hook-inner mode")
				return nil
			},
		}
		err := runWithOptions(buildCommandOptions{runHookExecutionOnly: true}, hooks)
		if err == nil {
			t.Fatal("expected hook execution error")
		}
		if !strings.Contains(err.Error(), "build hook failed") {
			t.Fatalf("error = %q, expected build-hook context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped execution hook error", err)
		}
	})

	t.Run("hook-inner prod mode wraps post-processing errors", func(t *testing.T) {
		expectedErr := errors.New("post-processing failed")
		hooks := buildCommandHooks{
			configureBuildEnvironment: func(*vormaruntime.Vorma) {},
			runBuildHook: func(*vormaruntime.Vorma, bool) error {
				t.Fatal("did not expect runBuildHook")
				return nil
			},
			runHookExecutionOnly: func(*vormaruntime.Vorma, bool) error {
				return nil
			},
			runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
				return expectedErr
			},
			runFullBuild: func(*vormaruntime.Vorma, bool, bool) error {
				t.Fatal("did not expect full build in hook-inner mode")
				return nil
			},
		}
		err := runWithOptions(buildCommandOptions{runHookExecutionOnly: true}, hooks)
		if err == nil {
			t.Fatal("expected post-processing error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped post-processing error", err)
		}
	})

	t.Run("full build mode passes options to runFullBuild", func(t *testing.T) {
		var calls observedCalls
		err := runWithOptions(
			buildCommandOptions{
				runInDevelopmentMode:  true,
				skipGoBinaryBuildStep: true,
			},
			newHooks(&calls),
		)
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
		if calls.configureCalled {
			t.Fatal("did not expect configureBuildEnvironment in full build mode")
		}
		if calls.runBuildHookCalled || calls.runHookExecutionOnlyCalled || calls.runProdHookPostProcessingCalled {
			t.Fatal("did not expect hook callbacks in full build mode")
		}
		if !calls.runFullBuildCalled {
			t.Fatal("expected runFullBuild to be called")
		}
		if !calls.runFullBuildDevelopmentMode {
			t.Fatal("expected runFullBuild to receive development mode=true")
		}
		if !calls.runFullBuildSkipGoBinaryBuild {
			t.Fatal("expected runFullBuild to receive skipGoBinaryBuild=true")
		}
	})

	t.Run("full build mode wraps build errors", func(t *testing.T) {
		expectedErr := errors.New("full build failed")
		hooks := buildCommandHooks{
			configureBuildEnvironment: func(*vormaruntime.Vorma) {
				t.Fatal("did not expect configureBuildEnvironment in full build mode")
			},
			runBuildHook: func(*vormaruntime.Vorma, bool) error {
				t.Fatal("did not expect runBuildHook in full build mode")
				return nil
			},
			runHookExecutionOnly: func(*vormaruntime.Vorma, bool) error {
				t.Fatal("did not expect runHookExecutionOnly in full build mode")
				return nil
			},
			runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
				t.Fatal("did not expect runProdHookPostProcessing in full build mode")
				return nil
			},
			runFullBuild: func(*vormaruntime.Vorma, bool, bool) error {
				return expectedErr
			},
		}
		err := runWithOptions(buildCommandOptions{}, hooks)
		if err == nil {
			t.Fatal("expected full-build error")
		}
		if !strings.Contains(err.Error(), "build failed") {
			t.Fatalf("error = %q, expected build failure context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped full-build error", err)
		}
	})
}

func TestBuildEntrypoint(t *testing.T) {
	t.Run("passes CLI args to runBuildCommandFromCLI", func(t *testing.T) {
		var capturedArgs []string
		var runCalled bool
		var fatalCalled bool
		runBuildEntrypointWithDependencies(
			&vormaruntime.Vorma{},
			[]string{"--dev", "--hook"},
			defaultBuildCommandHooks(),
			buildEntrypointDependencies{
				runBuildCommandFromCLI: func(
					_ *vormaruntime.Vorma,
					commandLineArgs []string,
					_ buildCommandHooks,
				) error {
					runCalled = true
					capturedArgs = append([]string{}, commandLineArgs...)
					return nil
				},
				fatalfForBuildCommand: func(string, ...any) {
					fatalCalled = true
				},
			},
		)

		if !runCalled {
			t.Fatal("expected runBuildCommandFromCLI to be called")
		}
		if fatalCalled {
			t.Fatal("did not expect fatalfForBuildCommand on successful build command")
		}
		if len(capturedArgs) != 2 || capturedArgs[0] != "--dev" || capturedArgs[1] != "--hook" {
			t.Fatalf("captured args = %#v, want [--dev --hook]", capturedArgs)
		}
	})

	t.Run(
		"calls fatalfForBuildCommand when runBuildCommandFromCLI fails",
		func(t *testing.T) {
			expectedErr := errors.New("run failed")
			var capturedFatalMessage string
			runBuildEntrypointWithDependencies(
				&vormaruntime.Vorma{},
				[]string{},
				defaultBuildCommandHooks(),
				buildEntrypointDependencies{
					runBuildCommandFromCLI: func(
						_ *vormaruntime.Vorma,
						_ []string,
						_ buildCommandHooks,
					) error {
						return expectedErr
					},
					fatalfForBuildCommand: func(format string, args ...any) {
						capturedFatalMessage = fmt.Sprintf(format, args...)
					},
				},
			)

			if !strings.Contains(capturedFatalMessage, expectedErr.Error()) {
				t.Fatalf(
					"fatal message = %q, expected wrapped run error",
					capturedFatalMessage,
				)
			}
		},
	)
}
