package vormabuild

import (
	"errors"
	"strings"
	"testing"

	"github.com/vormadev/vorma/vormaruntime"
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
	originalNewWaveBuilderStep := runtimeBuildToolingDeps.newWaveBuilder
	t.Cleanup(func() {
		runtimeBuildToolingDeps.newWaveBuilder = originalNewWaveBuilderStep
	})

	t.Run("runs Vite build and closes builder", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		builder := &fakeRuntimeWaveBuilder{}
		runtimeBuildToolingDeps.newWaveBuilder = func(v *vormaruntime.Vorma) runtimeWaveBuilder {
			if v != app {
				t.Fatalf("builder received app %p, want %p", v, app)
			}
			return builder
		}

		if err := runWaveViteProductionBuild(app); err != nil {
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
		runtimeBuildToolingDeps.newWaveBuilder = func(*vormaruntime.Vorma) runtimeWaveBuilder {
			return builder
		}

		err := runWaveViteProductionBuild(app)
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
		runtimeBuildToolingDeps.newWaveBuilder = func(*vormaruntime.Vorma) runtimeWaveBuilder {
			return builder
		}

		err := runWaveViteProductionBuild(app)
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
		runtimeBuildToolingDeps.newWaveBuilder = func(*vormaruntime.Vorma) runtimeWaveBuilder {
			return builder
		}

		err := runWaveViteProductionBuild(app)
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
	originalRunWaveDevelopmentModeStep := runtimeBuildToolingDeps.runWaveDevelopmentMode
	t.Cleanup(func() {
		runtimeBuildToolingDeps.runWaveDevelopmentMode = originalRunWaveDevelopmentModeStep
	})

	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	var developmentModeCalled bool
	runtimeBuildToolingDeps.runWaveDevelopmentMode = func(v *vormaruntime.Vorma) error {
		developmentModeCalled = true
		if v != app {
			t.Fatalf("runWaveDevelopmentMode received app %p, want %p", v, app)
		}
		return nil
	}

	if err := runWaveDevelopmentServer(app); err != nil {
		t.Fatalf("runWaveDevelopmentServer returned error: %v", err)
	}
	if !developmentModeCalled {
		t.Fatal("expected runtimeBuildToolingDeps.runWaveDevelopmentMode to be called")
	}
}

func TestRunWaveProductionBuild(t *testing.T) {
	originalNewWaveBuilderStep := runtimeBuildToolingDeps.newWaveBuilder
	t.Cleanup(func() {
		runtimeBuildToolingDeps.newWaveBuilder = originalNewWaveBuilderStep
	})

	t.Run("runs production build with options and closes builder", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		builder := &fakeRuntimeWaveBuilder{}
		runtimeBuildToolingDeps.newWaveBuilder = func(*vormaruntime.Vorma) runtimeWaveBuilder {
			return builder
		}

		buildOptions := wavebuild.BuildOpts{
			CompileGo: true,
			IsDev:     false,
			IsRebuild: false,
		}
		if err := runWaveProductionBuild(app, buildOptions); err != nil {
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
		runtimeBuildToolingDeps.newWaveBuilder = func(*vormaruntime.Vorma) runtimeWaveBuilder {
			return builder
		}

		err := runWaveProductionBuild(app, productionBuildOptions(false))
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
		runtimeBuildToolingDeps.newWaveBuilder = func(*vormaruntime.Vorma) runtimeWaveBuilder {
			return builder
		}

		err := runWaveProductionBuild(app, productionBuildOptions(false))
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
		runtimeBuildToolingDeps.newWaveBuilder = func(*vormaruntime.Vorma) runtimeWaveBuilder {
			return builder
		}

		err := runWaveProductionBuild(app, productionBuildOptions(false))
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
	originalNewWaveBuilderStep := runtimeBuildToolingDeps.newWaveBuilder
	originalRunWaveDevelopmentModeStep := runtimeBuildToolingDeps.runWaveDevelopmentMode
	t.Cleanup(func() {
		runtimeBuildToolingDeps.newWaveBuilder = originalNewWaveBuilderStep
		runtimeBuildToolingDeps.runWaveDevelopmentMode = originalRunWaveDevelopmentModeStep
	})

	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	runtimeBuildToolingDeps.newWaveBuilder = originalNewWaveBuilderStep
	runtimeBuildToolingDeps.runWaveDevelopmentMode = originalRunWaveDevelopmentModeStep

	builder := runtimeBuildToolingDeps.newWaveBuilder(app)
	if builder == nil {
		t.Fatal("expected runtimeBuildToolingDeps.newWaveBuilder to return a builder")
	}
	if err := builder.Close(); err != nil {
		t.Fatalf("builder.Close returned error: %v", err)
	}

	parsedConfig := app.Wave.Internal__GetParsedConfigMutableReference()
	parsedConfig.Core.MainAppEntry = ""

	err := runtimeBuildToolingDeps.runWaveDevelopmentMode(app)
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

		originalRunWaveViteProductionBuildStep := runtimeBuildDeps.runWaveViteProductionBuild
		originalRunPostViteProductionBuildStep := runtimeBuildDeps.runPostViteProductionBuild
		t.Cleanup(func() {
			runtimeBuildDeps.runWaveViteProductionBuild = originalRunWaveViteProductionBuildStep
			runtimeBuildDeps.runPostViteProductionBuild = originalRunPostViteProductionBuildStep
		})

		var viteBuildCalled bool
		var postProcessingCalled bool
		runtimeBuildDeps.runWaveViteProductionBuild = func(_ *vormaruntime.Vorma) error {
			viteBuildCalled = true
			return nil
		}
		runtimeBuildDeps.runPostViteProductionBuild = func(_ *vormaruntime.Vorma) error {
			postProcessingCalled = true
			return nil
		}

		err := runProdHookPostProcessing(app)
		if err != nil {
			t.Fatalf("runProdHookPostProcessing returned error: %v", err)
		}
		if !viteBuildCalled {
			t.Fatal("expected runtimeBuildDeps.runWaveViteProductionBuild to be called")
		}
		if !postProcessingCalled {
			t.Fatal("expected runtimeBuildDeps.runPostViteProductionBuild to be called")
		}
	})

	t.Run("wraps vite build error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		originalRunWaveViteProductionBuildStep := runtimeBuildDeps.runWaveViteProductionBuild
		originalRunPostViteProductionBuildStep := runtimeBuildDeps.runPostViteProductionBuild
		t.Cleanup(func() {
			runtimeBuildDeps.runWaveViteProductionBuild = originalRunWaveViteProductionBuildStep
			runtimeBuildDeps.runPostViteProductionBuild = originalRunPostViteProductionBuildStep
		})

		expectedErr := errors.New("vite step failed")
		runtimeBuildDeps.runWaveViteProductionBuild = func(_ *vormaruntime.Vorma) error {
			return expectedErr
		}
		runtimeBuildDeps.runPostViteProductionBuild = func(_ *vormaruntime.Vorma) error {
			t.Fatal("did not expect post-processing after vite failure")
			return nil
		}

		err := runProdHookPostProcessing(app)
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

		originalRunWaveViteProductionBuildStep := runtimeBuildDeps.runWaveViteProductionBuild
		originalRunPostViteProductionBuildStep := runtimeBuildDeps.runPostViteProductionBuild
		t.Cleanup(func() {
			runtimeBuildDeps.runWaveViteProductionBuild = originalRunWaveViteProductionBuildStep
			runtimeBuildDeps.runPostViteProductionBuild = originalRunPostViteProductionBuildStep
		})

		expectedErr := errors.New("post-processing failed")
		runtimeBuildDeps.runWaveViteProductionBuild = func(_ *vormaruntime.Vorma) error {
			return nil
		}
		runtimeBuildDeps.runPostViteProductionBuild = func(_ *vormaruntime.Vorma) error {
			return expectedErr
		}

		err := runProdHookPostProcessing(app)
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

	originalSetWaveModeToDevStep := runtimeBuildDeps.setWaveModeToDev
	originalRunWaveDevelopmentServer := runtimeBuildDeps.runWaveDevelopmentServer
	t.Cleanup(func() {
		runtimeBuildDeps.setWaveModeToDev = originalSetWaveModeToDevStep
		runtimeBuildDeps.runWaveDevelopmentServer = originalRunWaveDevelopmentServer
	})

	var setModeToDevCalled bool
	runtimeBuildDeps.setWaveModeToDev = func() {
		setModeToDevCalled = true
	}

	var developmentServerCalled bool
	runtimeBuildDeps.runWaveDevelopmentServer = func(_ *vormaruntime.Vorma) error {
		developmentServerCalled = true
		return nil
	}

	if err := newRuntimeBuildExecutor(app).runDevelopmentMode(); err != nil {
		t.Fatalf("runtimeBuildExecutor.runDevelopmentMode returned error: %v", err)
	}
	if !developmentServerCalled {
		t.Fatal("expected runtimeBuildDeps.runWaveDevelopmentServer to be called")
	}
	if !setModeToDevCalled {
		t.Fatal("expected runtimeBuildDeps.setWaveModeToDev to be called")
	}
	if !app.GetIsDevMode() {
		t.Fatal("expected app to be marked as dev mode")
	}
}

func TestRuntimeBuildExecutorRunProductionMode(t *testing.T) {
	t.Run("passes expected build options", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		originalRunWaveProductionBuildStep := runtimeBuildDeps.runWaveProductionBuild
		t.Cleanup(func() {
			runtimeBuildDeps.runWaveProductionBuild = originalRunWaveProductionBuildStep
		})

		var capturedOptions wavebuild.BuildOpts
		runtimeBuildDeps.runWaveProductionBuild = func(_ *vormaruntime.Vorma, options wavebuild.BuildOpts) error {
			capturedOptions = options
			return nil
		}

		if err := newRuntimeBuildExecutor(app).runProductionMode(true); err != nil {
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

		originalRunWaveProductionBuildStep := runtimeBuildDeps.runWaveProductionBuild
		t.Cleanup(func() {
			runtimeBuildDeps.runWaveProductionBuild = originalRunWaveProductionBuildStep
		})

		expectedErr := errors.New("wave build failed")
		runtimeBuildDeps.runWaveProductionBuild = func(_ *vormaruntime.Vorma, _ wavebuild.BuildOpts) error {
			return expectedErr
		}

		err := newRuntimeBuildExecutor(app).runProductionMode(false)
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

		originalRunWaveDevelopmentServer := runtimeBuildDeps.runWaveDevelopmentServer
		t.Cleanup(func() {
			runtimeBuildDeps.runWaveDevelopmentServer = originalRunWaveDevelopmentServer
		})

		var developmentServerCalled bool
		runtimeBuildDeps.runWaveDevelopmentServer = func(_ *vormaruntime.Vorma) error {
			developmentServerCalled = true
			return nil
		}

		if err := build(app, true, false); err != nil {
			t.Fatalf("build returned error: %v", err)
		}
		if !developmentServerCalled {
			t.Fatal("expected development build path to run")
		}
		parsedCfg := app.Wave.Internal__GetParsedConfigMutableReference()
		if len(parsedCfg.FrameworkWatchPatterns) == 0 {
			t.Fatal("expected configureBuildEnvironment to inject default watch patterns")
		}
	})

	t.Run("production mode returns production build error", func(t *testing.T) {
		fixture := newBuildTestFixture(t, nil)
		app := fixture.app

		originalRunWaveProductionBuildStep := runtimeBuildDeps.runWaveProductionBuild
		t.Cleanup(func() {
			runtimeBuildDeps.runWaveProductionBuild = originalRunWaveProductionBuildStep
		})

		expectedErr := errors.New("production build failed")
		runtimeBuildDeps.runWaveProductionBuild = func(_ *vormaruntime.Vorma, _ wavebuild.BuildOpts) error {
			return expectedErr
		}

		err := build(app, false, false)
		if err == nil {
			t.Fatal("expected build to return production error")
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("error = %v, expected wrapped production build error", err)
		}
	})
}
