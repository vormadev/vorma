package vormabuild

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
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
		if options.printDiagnosticsOnly {
			t.Fatal("expected printDiagnosticsOnly to default to false")
		}
		if options.skipGoBinaryBuildStep {
			t.Fatal("expected skipGoBinaryBuildStep to default to false")
		}
	})

	t.Run("parses all supported flags", func(t *testing.T) {
		options, err := parseBuildCommandOptions([]string{
			"--dev",
			"--hook",
			"--diagnostics",
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
		if !options.printDiagnosticsOnly {
			t.Fatal("expected printDiagnosticsOnly to be true")
		}
		if !options.skipGoBinaryBuildStep {
			t.Fatal("expected skipGoBinaryBuildStep to be true")
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
		"parses flags and executes through provided hooks",
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

	t.Run(
		"parses diagnostics flag and executes diagnostics hook only",
		func(t *testing.T) {
			var diagnosticsHookCalled bool
			err := runBuildCommand(
				&vormaruntime.Vorma{},
				[]string{"--diagnostics"},
				buildCommandHooks{
					configureBuildEnvironment: func(*vormaruntime.Vorma) {
						t.Fatal(
							"did not expect configureBuildEnvironment in diagnostics mode",
						)
					},
					runBuildHook: func(*vormaruntime.Vorma, bool) error {
						t.Fatal(
							"did not expect runBuildHook in diagnostics mode",
						)
						return nil
					},
					runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
						t.Fatal(
							"did not expect runProdHookPostProcessing in diagnostics mode",
						)
						return nil
					},
					printDiagnostics: func(*vormaruntime.Vorma) error {
						diagnosticsHookCalled = true
						return nil
					},
					runFullBuild: func(*vormaruntime.Vorma, bool, bool) error {
						t.Fatal(
							"did not expect runFullBuild in diagnostics mode",
						)
						return nil
					},
				},
			)
			if err != nil {
				t.Fatalf("runBuildCommand returned error: %v", err)
			}
			if !diagnosticsHookCalled {
				t.Fatal("expected printDiagnostics hook to be called")
			}
		},
	)

	t.Run(
		"diagnostics mode returns error when diagnostics hook is missing",
		func(t *testing.T) {
			err := runBuildCommand(
				&vormaruntime.Vorma{},
				[]string{"--diagnostics"},
				buildCommandHooks{
					configureBuildEnvironment: func(*vormaruntime.Vorma) {},
					runBuildHook: func(*vormaruntime.Vorma, bool) error {
						return nil
					},
					runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
						return nil
					},
					runFullBuild: func(*vormaruntime.Vorma, bool, bool) error {
						return nil
					},
				},
			)
			if err == nil {
				t.Fatal("expected diagnostics-mode hook validation error")
			}
			if !strings.Contains(
				err.Error(),
				"printDiagnostics is required in diagnostics mode",
			) {
				t.Fatalf(
					"error = %q, expected missing diagnostics-hook context",
					err,
				)
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

func TestDefaultBuildCommandHooks_RunBuildHookUsesBuildInner(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	t.Chdir(fixture.rootDir)
	writeBootstrapStyleRoutesFixtureFiles(t)

	hooks := defaultBuildCommandHooks()
	if err := hooks.runBuildHook(app, true); err != nil {
		t.Fatalf("default runBuildHook returned error: %v", err)
	}

	if !app.IsDevMode() {
		t.Fatal("expected default runBuildHook to set app to dev mode")
	}
	if app.BuildID() == "" {
		t.Fatal("expected default runBuildHook to assign a build ID")
	}
}

func TestBuildCommandExecutorRun(t *testing.T) {
	type observedCalls struct {
		configureCalled               bool
		runBuildHookCalled            bool
		runBuildHookDevelopmentMode   bool
		runProdHookPostProcessingCall bool
		printDiagnosticsCalled        bool
		runFullBuildCalled            bool
		runFullBuildDevelopmentMode   bool
		runFullBuildSkipGoBinaryBuild bool
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
			runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
				calls.runProdHookPostProcessingCall = true
				return nil
			},
			printDiagnostics: func(*vormaruntime.Vorma) error {
				calls.printDiagnosticsCalled = true
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

	t.Run(
		"hook dev mode runs configure and build hook only",
		func(t *testing.T) {
			var calls observedCalls
			err := runWithOptions(
				buildCommandOptions{
					runInDevelopmentMode: true,
					runHookOnly:          true,
				},
				newHooks(&calls),
			)
			if err != nil {
				t.Fatalf("runBuildCommandWithOptions returned error: %v", err)
			}
			if !calls.configureCalled {
				t.Fatal("expected configureBuildEnvironment to be called")
			}
			if !calls.runBuildHookCalled {
				t.Fatal("expected runBuildHook to be called")
			}
			if !calls.runBuildHookDevelopmentMode {
				t.Fatal(
					"expected runBuildHook to receive development mode = true",
				)
			}
			if calls.runProdHookPostProcessingCall {
				t.Fatal(
					"did not expect runProdHookPostProcessing in dev hook mode",
				)
			}
			if calls.printDiagnosticsCalled {
				t.Fatal("did not expect printDiagnostics in hook mode")
			}
			if calls.runFullBuildCalled {
				t.Fatal("did not expect runFullBuild in hook mode")
			}
		},
	)

	t.Run(
		"hook prod mode runs configure, build hook, and post processing",
		func(t *testing.T) {
			var calls observedCalls
			err := runWithOptions(
				buildCommandOptions{
					runInDevelopmentMode: false,
					runHookOnly:          true,
				},
				newHooks(&calls),
			)
			if err != nil {
				t.Fatalf("runBuildCommandWithOptions returned error: %v", err)
			}
			if !calls.configureCalled || !calls.runBuildHookCalled ||
				!calls.runProdHookPostProcessingCall {
				t.Fatalf("unexpected hook call sequence: %#v", calls)
			}
			if calls.runBuildHookDevelopmentMode {
				t.Fatal(
					"expected runBuildHook to receive development mode = false",
				)
			}
			if calls.printDiagnosticsCalled {
				t.Fatal("did not expect printDiagnostics in hook mode")
			}
			if calls.runFullBuildCalled {
				t.Fatal("did not expect runFullBuild in hook mode")
			}
		},
	)

	t.Run("diagnostics mode runs diagnostics hook only", func(t *testing.T) {
		var calls observedCalls
		err := runWithOptions(
			buildCommandOptions{
				printDiagnosticsOnly: true,
			},
			newHooks(&calls),
		)
		if err != nil {
			t.Fatalf("runBuildCommandWithOptions returned error: %v", err)
		}
		if !calls.printDiagnosticsCalled {
			t.Fatal("expected printDiagnostics to be called")
		}
		if calls.configureCalled || calls.runBuildHookCalled ||
			calls.runProdHookPostProcessingCall {
			t.Fatalf(
				"did not expect hook/build callbacks in diagnostics mode: %#v",
				calls,
			)
		}
		if calls.runFullBuildCalled {
			t.Fatal("did not expect runFullBuild in diagnostics mode")
		}
	})

	t.Run(
		"diagnostics mode takes precedence over hook and full-build options",
		func(t *testing.T) {
			var calls observedCalls
			err := runWithOptions(
				buildCommandOptions{
					runInDevelopmentMode:  true,
					runHookOnly:           true,
					printDiagnosticsOnly:  true,
					skipGoBinaryBuildStep: true,
				},
				newHooks(&calls),
			)
			if err != nil {
				t.Fatalf("runBuildCommandWithOptions returned error: %v", err)
			}
			if !calls.printDiagnosticsCalled {
				t.Fatal("expected printDiagnostics to be called")
			}
			if calls.configureCalled || calls.runBuildHookCalled ||
				calls.runProdHookPostProcessingCall {
				t.Fatalf(
					"did not expect hook-mode callbacks in diagnostics precedence mode: %#v",
					calls,
				)
			}
			if calls.runFullBuildCalled {
				t.Fatal(
					"did not expect runFullBuild when diagnostics is requested",
				)
			}
		},
	)

	t.Run(
		"diagnostics mode returns error when diagnostics hook is missing",
		func(t *testing.T) {
			var calls observedCalls
			hooks := newHooks(&calls)
			hooks.printDiagnostics = nil

			err := runWithOptions(
				buildCommandOptions{
					printDiagnosticsOnly: true,
				},
				hooks,
			)
			if err == nil {
				t.Fatal("expected missing diagnostics hook error")
			}
			if !strings.Contains(
				err.Error(),
				"printDiagnostics is required in diagnostics mode",
			) {
				t.Fatalf(
					"error = %q, expected missing diagnostics hook context",
					err,
				)
			}
		},
	)

	t.Run("diagnostics mode wraps diagnostics hook errors", func(t *testing.T) {
		expectedErr := errors.New("diagnostics failed")
		var calls observedCalls
		hooks := newHooks(&calls)
		hooks.printDiagnostics = func(*vormaruntime.Vorma) error {
			calls.printDiagnosticsCalled = true
			return expectedErr
		}

		err := runWithOptions(
			buildCommandOptions{
				printDiagnosticsOnly: true,
			},
			hooks,
		)
		if err == nil {
			t.Fatal("expected diagnostics hook error")
		}
		if !strings.Contains(err.Error(), "print diagnostics failed") {
			t.Fatalf("error = %q, expected diagnostics error context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped diagnostics hook error", err)
		}
	})

	t.Run("hook prod mode returns post-processing error", func(t *testing.T) {
		expectedErr := errors.New("post-processing failed")
		hooks := buildCommandHooks{
			configureBuildEnvironment: func(*vormaruntime.Vorma) {},
			runBuildHook: func(*vormaruntime.Vorma, bool) error {
				return nil
			},
			runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
				return expectedErr
			},
			runFullBuild: func(*vormaruntime.Vorma, bool, bool) error {
				t.Fatal("did not expect runFullBuild in hook mode")
				return nil
			},
		}
		err := runWithOptions(
			buildCommandOptions{
				runHookOnly: true,
			},
			hooks,
		)
		if err == nil {
			t.Fatal("expected post-processing error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped post-processing error", err)
		}
	})

	t.Run("hook mode wraps build hook errors", func(t *testing.T) {
		expectedErr := errors.New("hook failed")
		hooks := buildCommandHooks{
			configureBuildEnvironment: func(*vormaruntime.Vorma) {},
			runBuildHook: func(*vormaruntime.Vorma, bool) error {
				return expectedErr
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
		err := runWithOptions(
			buildCommandOptions{
				runHookOnly: true,
			},
			hooks,
		)
		if err == nil {
			t.Fatal("expected hook error")
		}
		if !strings.Contains(err.Error(), "build hook failed") {
			t.Fatalf("error = %q, expected build-hook context", err)
		}
		if !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Fatalf("error = %q, expected wrapped hook error", err)
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
			t.Fatalf("runBuildCommandWithOptions returned error: %v", err)
		}
		if calls.configureCalled {
			t.Fatal(
				"did not expect configureBuildEnvironment in full build mode",
			)
		}
		if calls.runBuildHookCalled || calls.runProdHookPostProcessingCall {
			t.Fatal("did not expect hook callbacks in full build mode")
		}
		if calls.printDiagnosticsCalled {
			t.Fatal("did not expect printDiagnostics in full build mode")
		}
		if !calls.runFullBuildCalled {
			t.Fatal("expected runFullBuild to be called")
		}
		if !calls.runFullBuildDevelopmentMode {
			t.Fatal("expected runFullBuild to receive development mode = true")
		}
		if !calls.runFullBuildSkipGoBinaryBuild {
			t.Fatal("expected runFullBuild to receive skipGoBinaryBuild = true")
		}
	})

	t.Run("full build mode wraps build errors", func(t *testing.T) {
		expectedErr := errors.New("full build failed")
		hooks := buildCommandHooks{
			configureBuildEnvironment: func(*vormaruntime.Vorma) {
				t.Fatal(
					"did not expect configureBuildEnvironment in full build mode",
				)
			},
			runBuildHook: func(*vormaruntime.Vorma, bool) error {
				t.Fatal("did not expect runBuildHook in full build mode")
				return nil
			},
			runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
				t.Fatal(
					"did not expect runProdHookPostProcessing in full build mode",
				)
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
		if !strings.Contains(err.Error(), expectedErr.Error()) {
			t.Fatalf("error = %q, expected wrapped full-build error", err)
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
			t.Fatal(
				"did not expect fatalfForBuildCommand on successful build command",
			)
		}
		if len(capturedArgs) != 2 || capturedArgs[0] != "--dev" ||
			capturedArgs[1] != "--hook" {
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

func TestRunHookOnlyBuildCommand(t *testing.T) {
	t.Run("development mode skips post processing", func(t *testing.T) {
		var configureCalled bool
		var runBuildHookCalled bool
		var runProdHookPostProcessingCalled bool

		commandExecutor := newBuildCommandExecutor(
			&vormaruntime.Vorma{},
			buildCommandHooks{
				configureBuildEnvironment: func(*vormaruntime.Vorma) {
					configureCalled = true
				},
				runBuildHook: func(*vormaruntime.Vorma, bool) error {
					runBuildHookCalled = true
					return nil
				},
				runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
					runProdHookPostProcessingCalled = true
					return nil
				},
				runFullBuild: func(*vormaruntime.Vorma, bool, bool) error {
					t.Fatal("did not expect runFullBuild in hook-only mode")
					return nil
				},
			},
		)
		err := commandExecutor.runHookOnly(true)
		if err != nil {
			t.Fatalf("runHookOnly returned error: %v", err)
		}
		if !configureCalled {
			t.Fatal("expected configureBuildEnvironment to be called")
		}
		if !runBuildHookCalled {
			t.Fatal("expected runBuildHook to be called")
		}
		if runProdHookPostProcessingCalled {
			t.Fatal(
				"did not expect runProdHookPostProcessing in hook-only dev mode",
			)
		}
	})

	t.Run("production mode executes post processing", func(t *testing.T) {
		var runProdHookPostProcessingCalled bool
		commandExecutor := newBuildCommandExecutor(
			&vormaruntime.Vorma{},
			buildCommandHooks{
				configureBuildEnvironment: func(*vormaruntime.Vorma) {},
				runBuildHook: func(*vormaruntime.Vorma, bool) error {
					return nil
				},
				runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
					runProdHookPostProcessingCalled = true
					return nil
				},
				runFullBuild: func(*vormaruntime.Vorma, bool, bool) error {
					t.Fatal("did not expect runFullBuild in hook-only mode")
					return nil
				},
			},
		)
		err := commandExecutor.runHookOnly(false)
		if err != nil {
			t.Fatalf("runHookOnly returned error: %v", err)
		}
		if !runProdHookPostProcessingCalled {
			t.Fatal(
				"expected runProdHookPostProcessing in hook-only production mode",
			)
		}
	})
}

func TestRunFullBuildCommand(t *testing.T) {
	t.Run("passes options to full build hook", func(t *testing.T) {
		var observedDevMode bool
		var observedSkipBinary bool
		commandExecutor := newBuildCommandExecutor(
			&vormaruntime.Vorma{},
			buildCommandHooks{
				configureBuildEnvironment: func(*vormaruntime.Vorma) {
					t.Fatal(
						"did not expect configureBuildEnvironment in full build mode",
					)
				},
				runBuildHook: func(*vormaruntime.Vorma, bool) error {
					t.Fatal("did not expect runBuildHook in full build mode")
					return nil
				},
				runProdHookPostProcessing: func(*vormaruntime.Vorma) error {
					t.Fatal(
						"did not expect runProdHookPostProcessing in full build mode",
					)
					return nil
				},
				runFullBuild: func(_ *vormaruntime.Vorma, isDev bool, skipBinary bool) error {
					observedDevMode = isDev
					observedSkipBinary = skipBinary
					return nil
				},
			},
		)
		err := commandExecutor.runFullBuild(true, true)
		if err != nil {
			t.Fatalf("runFullBuild returned error: %v", err)
		}
		if !observedDevMode {
			t.Fatal("expected runFullBuild to receive development mode=true")
		}
		if !observedSkipBinary {
			t.Fatal(
				"expected runFullBuild to receive skipGoBinaryBuildStep=true",
			)
		}
	})

	t.Run("wraps full build errors", func(t *testing.T) {
		expectedErr := errors.New("full build failed")
		commandExecutor := newBuildCommandExecutor(
			&vormaruntime.Vorma{},
			buildCommandHooks{
				configureBuildEnvironment: func(*vormaruntime.Vorma) {},
				runBuildHook:              func(*vormaruntime.Vorma, bool) error { return nil },
				runProdHookPostProcessing: func(*vormaruntime.Vorma) error { return nil },
				runFullBuild: func(*vormaruntime.Vorma, bool, bool) error {
					return expectedErr
				},
			},
		)
		err := commandExecutor.runFullBuild(false, false)
		if err == nil {
			t.Fatal("expected runFullBuild to return error")
		}
		if !strings.Contains(err.Error(), "build failed") {
			t.Fatalf("error = %q, expected build-failed context", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped full-build error", err)
		}
	})
}
