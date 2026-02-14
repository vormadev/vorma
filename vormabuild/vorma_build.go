package vormabuild

import (
	"fmt"
	"log"
	"os"

	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
	wavebuild "github.com/vormadev/vorma/wave/tooling"
)

type runtimeBuildDependencies struct {
	runWaveViteProductionBuild func(*vormaruntime.Vorma) error
	runPostViteProductionBuild func(*vormaruntime.Vorma) error
	setWaveModeToDev           func()
	runWaveDevelopmentServer   func(*vormaruntime.Vorma) error
	runWaveProductionBuild     func(*vormaruntime.Vorma, wavebuild.BuildOpts) error
}

type runtimeBuildToolingDependencies struct {
	newWaveBuilder         func(*vormaruntime.Vorma) runtimeWaveBuilder
	runWaveDevelopmentMode func(*vormaruntime.Vorma) error
}

type runtimeWaveBuilder interface {
	ViteProdBuild() error
	Build(wavebuild.BuildOpts) error
	Close() error
}

type buildEntrypointDependencies struct {
	runBuildCommandFromCLI func(*vormaruntime.Vorma, []string, buildCommandHooks) error
	fatalfForBuildCommand  func(string, ...any)
}

var runtimeBuildDeps = runtimeBuildDependencies{
	runWaveViteProductionBuild: runWaveViteProductionBuild,
	runPostViteProductionBuild: postViteProdBuild,
	setWaveModeToDev:           wave.SetModeToDev,
	runWaveDevelopmentServer:   runWaveDevelopmentServer,
	runWaveProductionBuild:     runWaveProductionBuild,
}

var runtimeBuildToolingDeps = runtimeBuildToolingDependencies{
	newWaveBuilder: func(v *vormaruntime.Vorma) runtimeWaveBuilder {
		return wavebuild.NewBuilder(
			v.Wave.Internal__GetParsedConfigMutableReference(),
			v.Wave.Logger(),
		)
	},
	runWaveDevelopmentMode: func(v *vormaruntime.Vorma) error {
		return wavebuild.RunDev(
			v.Wave.Internal__GetParsedConfigMutableReference(),
			v.Wave.Logger(),
		)
	},
}

var buildEntrypointDeps = buildEntrypointDependencies{
	runBuildCommandFromCLI: runBuildCommand,
	fatalfForBuildCommand:  log.Fatalf,
}

type runtimeBuildExecutor struct {
	vorma *vormaruntime.Vorma
}

// Build parses flags and runs the build or dev server.
func Build(v *vormaruntime.Vorma) {
	if err := buildEntrypointDeps.runBuildCommandFromCLI(v, os.Args[1:], defaultBuildCommandHooks()); err != nil {
		buildEntrypointDeps.fatalfForBuildCommand("%v", err)
	}
}

// build performs a full Vorma build.
func build(v *vormaruntime.Vorma, isDev bool, noBinary bool) error {
	configureBuildEnvironment(v)

	return newRuntimeBuildExecutor(v).run(isDev, noBinary)
}

func runWaveViteProductionBuild(v *vormaruntime.Vorma) error {
	return runWithRuntimeWaveBuilder(v, func(builder runtimeWaveBuilder) error {
		return builder.ViteProdBuild()
	})
}

func runWaveDevelopmentServer(v *vormaruntime.Vorma) error {
	return runtimeBuildToolingDeps.runWaveDevelopmentMode(v)
}

func runWaveProductionBuild(
	v *vormaruntime.Vorma,
	options wavebuild.BuildOpts,
) error {
	return runWithRuntimeWaveBuilder(v, func(builder runtimeWaveBuilder) error {
		return builder.Build(options)
	})
}

func runWithRuntimeWaveBuilder(
	v *vormaruntime.Vorma,
	runWithBuilder func(runtimeWaveBuilder) error,
) (operationErr error) {
	builder := runtimeBuildToolingDeps.newWaveBuilder(v)
	return runWithClosableResource(
		builder,
		"close wave builder",
		runWithBuilder,
	)
}

func runProdHookPostProcessing(v *vormaruntime.Vorma) error {
	if err := runtimeBuildDeps.runWaveViteProductionBuild(v); err != nil {
		return fmt.Errorf("Vite production build failed: %w", err)
	}

	if err := runtimeBuildDeps.runPostViteProductionBuild(v); err != nil {
		return fmt.Errorf("post Vite production build failed: %w", err)
	}

	return nil
}

func prepareDevBuildRuntime(v *vormaruntime.Vorma) {
	runtimeBuildDeps.setWaveModeToDev()
	// Set isDev on this process's Vorma instance so callbacks
	// (like rebuildRoutesOnly) can run in the dev server process.
	v.SetIsDev(true)
}

func productionBuildOptions(noBinary bool) wavebuild.BuildOpts {
	return wavebuild.BuildOpts{
		CompileGo: !noBinary,
		IsDev:     false,
		IsRebuild: false,
	}
}

func newRuntimeBuildExecutor(v *vormaruntime.Vorma) runtimeBuildExecutor {
	return runtimeBuildExecutor{
		vorma: v,
	}
}

func (buildExecutor runtimeBuildExecutor) run(
	isDev bool,
	noBinary bool,
) error {
	if isDev {
		return buildExecutor.runDevelopmentMode()
	}
	return buildExecutor.runProductionMode(noBinary)
}

func (buildExecutor runtimeBuildExecutor) runDevelopmentMode() error {
	prepareDevBuildRuntime(buildExecutor.vorma)
	return runtimeBuildDeps.runWaveDevelopmentServer(buildExecutor.vorma)
}

func (buildExecutor runtimeBuildExecutor) runProductionMode(noBinary bool) error {
	// Production Build
	//
	// The build flow is:
	// 1. wb.Build() sets up dist directory, then runs ProdBuildHook
	// 2. ProdBuildHook (e.g., "go run ./cmd/build --hook") runs in subprocess:
	//    a. buildInner() - parses routes, writes artifacts
	//    b. ViteProdBuild() - runs Vite
	//    c. postViteProdBuild() - processes Vite output
	// 3. wb.Build() compiles the Go binary
	//
	// All Vorma logic runs in the subprocess (via --hook) so state is shared.
	return runtimeBuildDeps.runWaveProductionBuild(buildExecutor.vorma, productionBuildOptions(noBinary))
}
