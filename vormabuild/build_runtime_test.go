package vormabuild

import (
	"errors"
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	wavebuild "github.com/vormadev/vorma/wave/tooling"
)

type fakeRuntimeWaveBuilder struct {
	viteErr        error
	buildErr       error
	closeErr       error
	viteCalled     bool
	buildCalled    bool
	closeCalled    bool
	receivedOption wavebuild.BuildOpts
}

func (builder *fakeRuntimeWaveBuilder) ViteProdBuild() error {
	builder.viteCalled = true
	return builder.viteErr
}

func (builder *fakeRuntimeWaveBuilder) Build(options wavebuild.BuildOpts) error {
	builder.buildCalled = true
	builder.receivedOption = options
	return builder.buildErr
}

func (builder *fakeRuntimeWaveBuilder) Close() error {
	builder.closeCalled = true
	return builder.closeErr
}

func TestRunWaveViteProductionBuild(t *testing.T) {
	t.Run("runs Vite build and closes builder", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		builder := &fakeRuntimeWaveBuilder{}
		dependencies := runtimeBuildToolingDependencies{
			newWaveBuilder: func(v *vormaruntime.Vorma) runtimeWaveBuilder {
				if v != app {
					t.Fatalf("builder received app %p, want %p", v, app)
				}
				return builder
			},
		}

		if err := runWaveViteProductionBuildWithToolingDependencies(app, dependencies); err != nil {
			t.Fatalf("runWaveViteProductionBuild returned error: %v", err)
		}
		if !builder.viteCalled {
			t.Fatal("expected builder.ViteProdBuild to be called")
		}
		if !builder.closeCalled {
			t.Fatal("expected builder.Close to be called")
		}
	})

	t.Run("propagates Vite build error and still closes builder", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("vite build failed")
		builder := &fakeRuntimeWaveBuilder{
			viteErr: expectedErr,
		}
		dependencies := runtimeBuildToolingDependencies{
			newWaveBuilder: func(*vormaruntime.Vorma) runtimeWaveBuilder {
				return builder
			},
		}

		err := runWaveViteProductionBuildWithToolingDependencies(app, dependencies)
		if err == nil {
			t.Fatal("expected runWaveViteProductionBuild to return error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped Vite build error", err)
		}
		if !builder.closeCalled {
			t.Fatal("expected builder.Close to be called after Vite build error")
		}
	})

	t.Run("returns close error when Vite build succeeds", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedCloseErr := errors.New("close failed")
		builder := &fakeRuntimeWaveBuilder{
			closeErr: expectedCloseErr,
		}
		dependencies := runtimeBuildToolingDependencies{
			newWaveBuilder: func(*vormaruntime.Vorma) runtimeWaveBuilder {
				return builder
			},
		}

		err := runWaveViteProductionBuildWithToolingDependencies(app, dependencies)
		if err == nil {
			t.Fatal("expected runWaveViteProductionBuild to return close error")
		}
		if !errors.Is(err, expectedCloseErr) {
			t.Fatalf("error = %v, expected wrapped close error", err)
		}
		if !strings.Contains(err.Error(), "close wave builder") {
			t.Fatalf("error = %q, expected close context", err)
		}
		if !builder.closeCalled {
			t.Fatal("expected builder.Close to be called")
		}
	})

	t.Run("joins close error when Vite build fails", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedBuildErr := errors.New("vite build failed")
		expectedCloseErr := errors.New("close failed")
		builder := &fakeRuntimeWaveBuilder{
			viteErr:  expectedBuildErr,
			closeErr: expectedCloseErr,
		}
		dependencies := runtimeBuildToolingDependencies{
			newWaveBuilder: func(*vormaruntime.Vorma) runtimeWaveBuilder {
				return builder
			},
		}

		err := runWaveViteProductionBuildWithToolingDependencies(app, dependencies)
		if err == nil {
			t.Fatal("expected runWaveViteProductionBuild to return joined build+close error")
		}
		if !errors.Is(err, expectedBuildErr) {
			t.Fatalf("error = %v, expected build error in joined chain", err)
		}
		if !errors.Is(err, expectedCloseErr) {
			t.Fatalf("error = %v, expected close error in joined chain", err)
		}
		if !strings.Contains(err.Error(), "close wave builder") {
			t.Fatalf("error = %q, expected close context", err)
		}
	})
}

func TestRunWaveDevelopmentServer(t *testing.T) {
	emptyMainAppEntry := ""
	fixture := newBuildTestFixture(t, &buildTestFixtureOptions{
		waveMainAppEntry: &emptyMainAppEntry,
	})
	app := fixture.app

	var developmentModeCalled bool
	dependencies := runtimeBuildToolingDependencies{
		runWaveDevelopmentMode: func(v *vormaruntime.Vorma) error {
			developmentModeCalled = true
			if v != app {
				t.Fatalf("runWaveDevelopmentMode received app %p, want %p", v, app)
			}
			return nil
		},
	}

	if err := runWaveDevelopmentServerWithToolingDependencies(app, dependencies); err != nil {
		t.Fatalf("runWaveDevelopmentServer returned error: %v", err)
	}
	if !developmentModeCalled {
		t.Fatal("expected runWaveDevelopmentMode to be called")
	}
}

func TestRunWaveProductionBuild(t *testing.T) {
	t.Run("runs production build with options and closes builder", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		builder := &fakeRuntimeWaveBuilder{}
		dependencies := runtimeBuildToolingDependencies{
			newWaveBuilder: func(*vormaruntime.Vorma) runtimeWaveBuilder {
				return builder
			},
		}

		buildOptions := wavebuild.BuildOpts{
			CompileGo: true,
			IsDev:     false,
			IsRebuild: false,
		}
		if err := runWaveProductionBuildWithToolingDependencies(app, buildOptions, dependencies); err != nil {
			t.Fatalf("runWaveProductionBuild returned error: %v", err)
		}
		if !builder.buildCalled {
			t.Fatal("expected builder.Build to be called")
		}
		if builder.receivedOption != buildOptions {
			t.Fatalf("builder.Build received options %#v, want %#v", builder.receivedOption, buildOptions)
		}
		if !builder.closeCalled {
			t.Fatal("expected builder.Close to be called")
		}
	})

	t.Run("propagates production build error and still closes builder", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("production build failed")
		builder := &fakeRuntimeWaveBuilder{
			buildErr: expectedErr,
		}
		dependencies := runtimeBuildToolingDependencies{
			newWaveBuilder: func(*vormaruntime.Vorma) runtimeWaveBuilder {
				return builder
			},
		}

		err := runWaveProductionBuildWithToolingDependencies(app, productionBuildOptions(false), dependencies)
		if err == nil {
			t.Fatal("expected runWaveProductionBuild to return error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped production build error", err)
		}
		if !builder.closeCalled {
			t.Fatal("expected builder.Close to be called after build error")
		}
	})

	t.Run("returns close error when production build succeeds", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedCloseErr := errors.New("close failed")
		builder := &fakeRuntimeWaveBuilder{
			closeErr: expectedCloseErr,
		}
		dependencies := runtimeBuildToolingDependencies{
			newWaveBuilder: func(*vormaruntime.Vorma) runtimeWaveBuilder {
				return builder
			},
		}

		err := runWaveProductionBuildWithToolingDependencies(app, productionBuildOptions(false), dependencies)
		if err == nil {
			t.Fatal("expected runWaveProductionBuild to return close error")
		}
		if !errors.Is(err, expectedCloseErr) {
			t.Fatalf("error = %v, expected wrapped close error", err)
		}
		if !strings.Contains(err.Error(), "close wave builder") {
			t.Fatalf("error = %q, expected close context", err)
		}
	})

	t.Run("joins close error when production build fails", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedBuildErr := errors.New("production build failed")
		expectedCloseErr := errors.New("close failed")
		builder := &fakeRuntimeWaveBuilder{
			buildErr: expectedBuildErr,
			closeErr: expectedCloseErr,
		}
		dependencies := runtimeBuildToolingDependencies{
			newWaveBuilder: func(*vormaruntime.Vorma) runtimeWaveBuilder {
				return builder
			},
		}

		err := runWaveProductionBuildWithToolingDependencies(app, productionBuildOptions(false), dependencies)
		if err == nil {
			t.Fatal("expected runWaveProductionBuild to return joined build+close error")
		}
		if !errors.Is(err, expectedBuildErr) {
			t.Fatalf("error = %v, expected build error in joined chain", err)
		}
		if !errors.Is(err, expectedCloseErr) {
			t.Fatalf("error = %v, expected close error in joined chain", err)
		}
		if !strings.Contains(err.Error(), "close wave builder") {
			t.Fatalf("error = %q, expected close context", err)
		}
	})
}

func TestRuntimeBuildToolingDefaultSteps(t *testing.T) {
	emptyMainAppEntry := ""
	fixture := newBuildTestFixture(t, &buildTestFixtureOptions{
		waveMainAppEntry: &emptyMainAppEntry,
	})
	app := fixture.app

	dependencies := defaultRuntimeBuildToolingDependencies()

	builder := dependencies.newWaveBuilder(app)
	if builder == nil {
		t.Fatal("expected default newWaveBuilder to return a builder")
	}
	if err := builder.Close(); err != nil {
		t.Fatalf("builder.Close returned error: %v", err)
	}

	err := dependencies.runWaveDevelopmentMode(app)
	if err == nil {
		t.Fatal("expected default runWaveDevelopmentMode to return config validation error")
	}
	if !strings.Contains(err.Error(), "config validation failed") {
		t.Fatalf("error = %q, expected config validation context", err)
	}
}

func TestRunProdHookPostProcessing(t *testing.T) {
	t.Run("runs vite build then post-processing", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		var viteBuildCalled bool
		var postProcessingCalled bool
		dependencies := runtimeBuildDependencies{
			runWaveViteProductionBuild: func(_ *vormaruntime.Vorma) error {
				viteBuildCalled = true
				return nil
			},
			runPostViteProductionBuild: func(_ *vormaruntime.Vorma) error {
				postProcessingCalled = true
				return nil
			},
		}

		err := runProdHookPostProcessingWithRuntimeBuildDependencies(app, dependencies)
		if err != nil {
			t.Fatalf("runProdHookPostProcessing returned error: %v", err)
		}
		if !viteBuildCalled {
			t.Fatal("expected runWaveViteProductionBuild to be called")
		}
		if !postProcessingCalled {
			t.Fatal("expected runPostViteProductionBuild to be called")
		}
	})

	t.Run("wraps vite build error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("vite step failed")
		dependencies := runtimeBuildDependencies{
			runWaveViteProductionBuild: func(_ *vormaruntime.Vorma) error {
				return expectedErr
			},
			runPostViteProductionBuild: func(_ *vormaruntime.Vorma) error {
				t.Fatal("did not expect post-processing after vite failure")
				return nil
			},
		}

		err := runProdHookPostProcessingWithRuntimeBuildDependencies(app, dependencies)
		if err == nil {
			t.Fatal("expected runProdHookPostProcessing to return error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped vite build error", err)
		}
	})

	t.Run("wraps post-processing error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("post-processing failed")
		dependencies := runtimeBuildDependencies{
			runWaveViteProductionBuild: func(_ *vormaruntime.Vorma) error {
				return nil
			},
			runPostViteProductionBuild: func(_ *vormaruntime.Vorma) error {
				return expectedErr
			},
		}

		err := runProdHookPostProcessingWithRuntimeBuildDependencies(app, dependencies)
		if err == nil {
			t.Fatal("expected runProdHookPostProcessing to return error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped post-processing error", err)
		}
	})
}

func TestRuntimeBuildExecutorRunDevelopmentMode(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	var setModeToDevCalled bool
	var developmentServerCalled bool
	dependencies := runtimeBuildDependencies{
		setWaveModeToDev: func() {
			setModeToDevCalled = true
		},
		runWaveDevelopmentServer: func(_ *vormaruntime.Vorma) error {
			developmentServerCalled = true
			return nil
		},
	}

	if err := newRuntimeBuildExecutorWithDependencies(app, dependencies).runDevelopmentMode(); err != nil {
		t.Fatalf("runtimeBuildExecutor.runDevelopmentMode returned error: %v", err)
	}
	if !developmentServerCalled {
		t.Fatal("expected runWaveDevelopmentServer to be called")
	}
	if !setModeToDevCalled {
		t.Fatal("expected setWaveModeToDev to be called")
	}
	if !app.GetIsDevMode() {
		t.Fatal("expected app to be marked as dev mode")
	}
}

func TestRuntimeBuildExecutorRunProductionMode(t *testing.T) {
	t.Run("passes expected build options", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		var capturedOptions wavebuild.BuildOpts
		dependencies := runtimeBuildDependencies{
			runWaveProductionBuild: func(
				_ *vormaruntime.Vorma,
				options wavebuild.BuildOpts,
			) error {
				capturedOptions = options
				return nil
			},
		}

		if err := newRuntimeBuildExecutorWithDependencies(app, dependencies).runProductionMode(true); err != nil {
			t.Fatalf("runtimeBuildExecutor.runProductionMode returned error: %v", err)
		}
		if capturedOptions.CompileGo {
			t.Fatalf("CompileGo = %v, want false when noBinary=true", capturedOptions.CompileGo)
		}
		if capturedOptions.IsDev {
			t.Fatalf("IsDev = %v, want false", capturedOptions.IsDev)
		}
		if capturedOptions.IsRebuild {
			t.Fatalf("IsRebuild = %v, want false", capturedOptions.IsRebuild)
		}
	})

	t.Run("propagates wave production build errors", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("wave build failed")
		dependencies := runtimeBuildDependencies{
			runWaveProductionBuild: func(
				_ *vormaruntime.Vorma,
				_ wavebuild.BuildOpts,
			) error {
				return expectedErr
			},
		}

		err := newRuntimeBuildExecutorWithDependencies(app, dependencies).runProductionMode(false)
		if err == nil {
			t.Fatal("expected runtimeBuildExecutor.runProductionMode to return error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped wave build error", err)
		}
	})
}

func TestProductionBuildOptions(t *testing.T) {
	optionsWithBinary := productionBuildOptions(false)
	if !optionsWithBinary.CompileGo {
		t.Fatalf("CompileGo = %v, want true when noBinary=false", optionsWithBinary.CompileGo)
	}
	if optionsWithBinary.IsDev {
		t.Fatalf("IsDev = %v, want false", optionsWithBinary.IsDev)
	}
	if optionsWithBinary.IsRebuild {
		t.Fatalf("IsRebuild = %v, want false", optionsWithBinary.IsRebuild)
	}

	optionsWithoutBinary := productionBuildOptions(true)
	if optionsWithoutBinary.CompileGo {
		t.Fatalf("CompileGo = %v, want false when noBinary=true", optionsWithoutBinary.CompileGo)
	}
	if optionsWithoutBinary.IsDev {
		t.Fatalf("IsDev = %v, want false", optionsWithoutBinary.IsDev)
	}
	if optionsWithoutBinary.IsRebuild {
		t.Fatalf("IsRebuild = %v, want false", optionsWithoutBinary.IsRebuild)
	}
}

func TestBuild(t *testing.T) {
	t.Run("dev mode configures environment and runs dev build path", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		var developmentServerCalled bool
		dependencies := runtimeBuildDependencies{
			runWaveDevelopmentServer: func(_ *vormaruntime.Vorma) error {
				developmentServerCalled = true
				return nil
			},
		}

		if err := buildWithRuntimeBuildDependencies(app, true, false, dependencies); err != nil {
			t.Fatalf("build returned error: %v", err)
		}
		if !developmentServerCalled {
			t.Fatal("expected development build path to run")
		}
		parsedCfg := configureBuildEnvironment(app)
		if len(parsedCfg.FrameworkWatchPatterns) == 0 {
			t.Fatal("expected configureBuildEnvironment to inject default watch patterns")
		}
	})

	t.Run("production mode returns production build error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		expectedErr := errors.New("production build failed")
		dependencies := runtimeBuildDependencies{
			runWaveProductionBuild: func(_ *vormaruntime.Vorma, _ wavebuild.BuildOpts) error {
				return expectedErr
			},
		}

		err := buildWithRuntimeBuildDependencies(app, false, false, dependencies)
		if err == nil {
			t.Fatal("expected build to return production error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped production build error", err)
		}
	})
}
