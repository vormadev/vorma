package vormabuild

import (
	"fmt"
	"log"
	"os"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/internal/vormaruntime"
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

type runtimeBuildToolingExecutor struct {
	dependencies runtimeBuildToolingDependencies
}

type runtimeBuildOperationExecutor struct {
	dependencies runtimeBuildDependencies
}

type buildEntrypointExecutor struct {
	dependencies buildEntrypointDependencies
}

type runtimeBuildExecutor struct {
	vorma                  *vormaruntime.Vorma
	runtimeBuildOperations runtimeBuildOperationExecutor
}

func defaultRuntimeBuildToolingDependencies() runtimeBuildToolingDependencies {
	return runtimeBuildToolingDependencies{
		newWaveBuilder: func(v *vormaruntime.Vorma) runtimeWaveBuilder {
			return wavebuild.NewBuilder(
				configureBuildEnvironment(v),
				v.Wave.Logger(),
			)
		},
		runWaveDevelopmentMode: func(v *vormaruntime.Vorma) error {
			return wavebuild.RunDev(
				configureBuildEnvironment(v),
				v.Wave.Logger(),
			)
		},
	}
}

func normalizeRuntimeBuildToolingDependencies(
	dependencies runtimeBuildToolingDependencies,
) runtimeBuildToolingDependencies {
	defaultDependencies := defaultRuntimeBuildToolingDependencies()
	if dependencies.newWaveBuilder == nil {
		dependencies.newWaveBuilder = defaultDependencies.newWaveBuilder
	}
	if dependencies.runWaveDevelopmentMode == nil {
		dependencies.runWaveDevelopmentMode = defaultDependencies.runWaveDevelopmentMode
	}
	return dependencies
}

func newRuntimeBuildToolingExecutor(
	dependencies runtimeBuildToolingDependencies,
) runtimeBuildToolingExecutor {
	return runtimeBuildToolingExecutor{
		dependencies: normalizeRuntimeBuildToolingDependencies(dependencies),
	}
}

func defaultRuntimeBuildDependencies() runtimeBuildDependencies {
	return runtimeBuildDependencies{
		runWaveViteProductionBuild: func(v *vormaruntime.Vorma) error {
			return defaultRuntimeBuildToolingExecutor.runWaveViteProductionBuild(
				v,
			)
		},
		runPostViteProductionBuild: postViteProdBuild,
		setWaveModeToDev:           wave.SetModeToDev,
		runWaveDevelopmentServer: func(v *vormaruntime.Vorma) error {
			return defaultRuntimeBuildToolingExecutor.runWaveDevelopmentServer(
				v,
			)
		},
		runWaveProductionBuild: func(
			v *vormaruntime.Vorma,
			options wavebuild.BuildOpts,
		) error {
			return defaultRuntimeBuildToolingExecutor.runWaveProductionBuild(
				v,
				options,
			)
		},
	}
}

func normalizeRuntimeBuildDependencies(
	dependencies runtimeBuildDependencies,
) runtimeBuildDependencies {
	defaultDependencies := defaultRuntimeBuildDependencies()
	if dependencies.runWaveViteProductionBuild == nil {
		dependencies.runWaveViteProductionBuild = defaultDependencies.runWaveViteProductionBuild
	}
	if dependencies.runPostViteProductionBuild == nil {
		dependencies.runPostViteProductionBuild = defaultDependencies.runPostViteProductionBuild
	}
	if dependencies.setWaveModeToDev == nil {
		dependencies.setWaveModeToDev = defaultDependencies.setWaveModeToDev
	}
	if dependencies.runWaveDevelopmentServer == nil {
		dependencies.runWaveDevelopmentServer = defaultDependencies.runWaveDevelopmentServer
	}
	if dependencies.runWaveProductionBuild == nil {
		dependencies.runWaveProductionBuild = defaultDependencies.runWaveProductionBuild
	}
	return dependencies
}

func newRuntimeBuildOperationExecutor(
	dependencies runtimeBuildDependencies,
) runtimeBuildOperationExecutor {
	return runtimeBuildOperationExecutor{
		dependencies: normalizeRuntimeBuildDependencies(dependencies),
	}
}

func defaultBuildEntrypointDependencies() buildEntrypointDependencies {
	return buildEntrypointDependencies{
		runBuildCommandFromCLI: runBuildCommand,
		fatalfForBuildCommand:  log.Fatalf,
	}
}

func normalizeBuildEntrypointDependencies(
	dependencies buildEntrypointDependencies,
) buildEntrypointDependencies {
	defaultDependencies := defaultBuildEntrypointDependencies()
	if dependencies.runBuildCommandFromCLI == nil {
		dependencies.runBuildCommandFromCLI = defaultDependencies.runBuildCommandFromCLI
	}
	if dependencies.fatalfForBuildCommand == nil {
		dependencies.fatalfForBuildCommand = defaultDependencies.fatalfForBuildCommand
	}
	return dependencies
}

func newBuildEntrypointExecutor(
	dependencies buildEntrypointDependencies,
) buildEntrypointExecutor {
	return buildEntrypointExecutor{
		dependencies: normalizeBuildEntrypointDependencies(dependencies),
	}
}

var defaultRuntimeBuildToolingExecutor = newRuntimeBuildToolingExecutor(
	runtimeBuildToolingDependencies{},
)

var defaultRuntimeBuildOperationExecutor = newRuntimeBuildOperationExecutor(
	runtimeBuildDependencies{},
)

var defaultBuildEntrypointExecutor = newBuildEntrypointExecutor(
	buildEntrypointDependencies{},
)

// Build parses flags and runs the build or dev server.
func Build(v *vorma.Vorma) {
	defaultBuildEntrypointExecutor.runBuildCommand(
		v.UnsafeRuntimeForFrameworkInternals(),
		os.Args[1:],
		defaultBuildCommandHooks(),
	)
}

func runBuildEntrypointWithDependencies(
	v *vormaruntime.Vorma,
	commandLineArgs []string,
	hooks buildCommandHooks,
	dependencies buildEntrypointDependencies,
) {
	newBuildEntrypointExecutor(
		dependencies,
	).runBuildCommand(v, commandLineArgs, hooks)
}

// build performs a full Vorma build.
func build(v *vormaruntime.Vorma, isDev bool, noBinary bool) error {
	return newRuntimeBuildExecutor(v).run(isDev, noBinary)
}

func buildWithRuntimeBuildDependencies(
	v *vormaruntime.Vorma,
	isDev bool,
	noBinary bool,
	dependencies runtimeBuildDependencies,
) error {
	return newRuntimeBuildExecutorWithDependencies(
		v,
		dependencies,
	).run(isDev, noBinary)
}

func (entrypointExecutor buildEntrypointExecutor) runBuildCommand(
	v *vormaruntime.Vorma,
	commandLineArgs []string,
	hooks buildCommandHooks,
) {
	if err := entrypointExecutor.dependencies.runBuildCommandFromCLI(v, commandLineArgs, hooks); err != nil {
		entrypointExecutor.dependencies.fatalfForBuildCommand("%v", err)
	}
}

func runWaveViteProductionBuildWithToolingDependencies(
	v *vormaruntime.Vorma,
	dependencies runtimeBuildToolingDependencies,
) error {
	return newRuntimeBuildToolingExecutor(
		dependencies,
	).runWaveViteProductionBuild(v)
}

func runWaveDevelopmentServerWithToolingDependencies(
	v *vormaruntime.Vorma,
	dependencies runtimeBuildToolingDependencies,
) error {
	return newRuntimeBuildToolingExecutor(
		dependencies,
	).runWaveDevelopmentServer(v)
}

func runWaveProductionBuildWithToolingDependencies(
	v *vormaruntime.Vorma,
	options wavebuild.BuildOpts,
	dependencies runtimeBuildToolingDependencies,
) error {
	return newRuntimeBuildToolingExecutor(
		dependencies,
	).runWaveProductionBuild(v, options)
}

func (toolingExecutor runtimeBuildToolingExecutor) runWaveViteProductionBuild(
	v *vormaruntime.Vorma,
) error {
	return toolingExecutor.runWithRuntimeWaveBuilder(
		v,
		func(builder runtimeWaveBuilder) error {
			return builder.ViteProdBuild()
		},
	)
}

func (toolingExecutor runtimeBuildToolingExecutor) runWaveDevelopmentServer(
	v *vormaruntime.Vorma,
) error {
	return toolingExecutor.dependencies.runWaveDevelopmentMode(v)
}

func (toolingExecutor runtimeBuildToolingExecutor) runWaveProductionBuild(
	v *vormaruntime.Vorma,
	options wavebuild.BuildOpts,
) error {
	return toolingExecutor.runWithRuntimeWaveBuilder(
		v,
		func(builder runtimeWaveBuilder) error {
			return builder.Build(options)
		},
	)
}

func (toolingExecutor runtimeBuildToolingExecutor) runWithRuntimeWaveBuilder(
	v *vormaruntime.Vorma,
	runWithBuilder func(runtimeWaveBuilder) error,
) (operationErr error) {
	builder := toolingExecutor.dependencies.newWaveBuilder(v)
	return runWithClosableResource(
		builder,
		"close wave builder",
		runWithBuilder,
	)
}

func runProdHookPostProcessing(v *vormaruntime.Vorma) error {
	return defaultRuntimeBuildOperationExecutor.runProdHookPostProcessing(v)
}

func runProdHookPostProcessingWithRuntimeBuildDependencies(
	v *vormaruntime.Vorma,
	dependencies runtimeBuildDependencies,
) error {
	return newRuntimeBuildOperationExecutor(
		dependencies,
	).runProdHookPostProcessing(v)
}

func (runtimeBuildOperations runtimeBuildOperationExecutor) runProdHookPostProcessing(
	v *vormaruntime.Vorma,
) error {
	if err := runtimeBuildOperations.dependencies.runWaveViteProductionBuild(v); err != nil {
		return fmt.Errorf("Vite production build failed: %w", err)
	}

	if err := runtimeBuildOperations.dependencies.runPostViteProductionBuild(v); err != nil {
		return fmt.Errorf("post Vite production build failed: %w", err)
	}

	return nil
}

func (runtimeBuildOperations runtimeBuildOperationExecutor) prepareDevBuildRuntime(
	v *vormaruntime.Vorma,
) {
	runtimeBuildOperations.dependencies.setWaveModeToDev()
	// Set isDev on this process's Vorma instance so callbacks
	// (like rebuildRoutesOnly) can run in the dev server process.
	commitRuntimeState(
		v,
		runtimeStateCommitInput{
			shouldCommitIsDev: true,
			isDev:             true,
		},
	)
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
		vorma:                  v,
		runtimeBuildOperations: defaultRuntimeBuildOperationExecutor,
	}
}

func newRuntimeBuildExecutorWithDependencies(
	v *vormaruntime.Vorma,
	dependencies runtimeBuildDependencies,
) runtimeBuildExecutor {
	return runtimeBuildExecutor{
		vorma:                  v,
		runtimeBuildOperations: newRuntimeBuildOperationExecutor(dependencies),
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
	buildExecutor.runtimeBuildOperations.prepareDevBuildRuntime(
		buildExecutor.vorma,
	)
	return buildExecutor.runtimeBuildOperations.dependencies.runWaveDevelopmentServer(
		buildExecutor.vorma,
	)
}

func (buildExecutor runtimeBuildExecutor) runProductionMode(
	noBinary bool,
) error {
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
	return buildExecutor.runtimeBuildOperations.dependencies.runWaveProductionBuild(
		buildExecutor.vorma,
		productionBuildOptions(noBinary),
	)
}
