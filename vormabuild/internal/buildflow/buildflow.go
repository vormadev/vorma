// Package buildflow owns runtime build-mode orchestration for vormabuild.
//
// It coordinates the dev/prod execution paths, Vite/post-Vite production
// pipeline, and Wave tooling adapter lifecycle. Keeping this in one package
// keeps runtime-build semantics and tests cohesive without mixing them into
// CLI flag parsing or entrypoint wiring.
package buildflow

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vormadev/vorma/internal/artifactio"
	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/internal/vormaruntime/runtimepaths"
	"github.com/vormadev/vorma/lab/viteutil"
	"github.com/vormadev/vorma/vormabuild/internal/buildenv"
	"github.com/vormadev/vorma/vormabuild/internal/buildlifecycle"
	"github.com/vormadev/vorma/vormabuild/internal/routeartifacts"
	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/buildtime/builder"
	"github.com/vormadev/vorma/wave/buildtime/devserver"
)

type postViteProdBuildDependencies struct {
	toPathsFileStageTwo      func(*vormaruntime.Vorma) (*runtimepaths.PathsFile, error)
	writePathsToDiskStageTwo func(*vormaruntime.Vorma, *runtimepaths.PathsFile) error
	recordBuildOutputPaths   func(*vormaruntime.Vorma, *runtimepaths.PathsFile) error
	applyBuildIDToVorma      func(*vormaruntime.Vorma, string)
}

func defaultPostViteProdBuildDependencies() postViteProdBuildDependencies {
	return postViteProdBuildDependencies{
		toPathsFileStageTwo:      routeartifacts.ToPathsFileStageTwo,
		writePathsToDiskStageTwo: routeartifacts.WritePathsToDiskStageTwo,
		recordBuildOutputPaths:   recordFrameworkBuildOutputsToLedger,
		applyBuildIDToVorma:      applyBuildIDToVorma,
	}
}

func normalizePostViteProdBuildDependencies(
	dependencies postViteProdBuildDependencies,
) postViteProdBuildDependencies {
	defaultDependencies := defaultPostViteProdBuildDependencies()

	if dependencies.toPathsFileStageTwo == nil {
		dependencies.toPathsFileStageTwo = defaultDependencies.toPathsFileStageTwo
	}
	if dependencies.writePathsToDiskStageTwo == nil {
		dependencies.writePathsToDiskStageTwo = defaultDependencies.writePathsToDiskStageTwo
	}
	if dependencies.recordBuildOutputPaths == nil {
		dependencies.recordBuildOutputPaths = defaultDependencies.recordBuildOutputPaths
	}
	if dependencies.applyBuildIDToVorma == nil {
		dependencies.applyBuildIDToVorma = defaultDependencies.applyBuildIDToVorma
	}

	return dependencies
}

func postViteProdBuild(v *vormaruntime.Vorma) error {
	return postViteProdBuildWithDependencies(v, postViteProdBuildDependencies{})
}

func postViteProdBuildWithDependencies(
	v *vormaruntime.Vorma,
	dependencies postViteProdBuildDependencies,
) error {
	dependencies = normalizePostViteProdBuildDependencies(dependencies)

	pathsFile, err := dependencies.toPathsFileStageTwo(v)
	if err != nil {
		return fmt.Errorf("convert paths to stage two: %w", err)
	}

	if err := dependencies.writePathsToDiskStageTwo(v, pathsFile); err != nil {
		return fmt.Errorf("write stage-two paths: %w", err)
	}
	if err := dependencies.recordBuildOutputPaths(v, pathsFile); err != nil {
		return fmt.Errorf("record framework build outputs: %w", err)
	}

	dependencies.applyBuildIDToVorma(v, pathsFile.BuildID)
	return nil
}

func recordFrameworkBuildOutputsToLedger(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) error {
	buildConfig := v.Wave.ParsedConfig()
	if buildConfig == nil {
		return fmt.Errorf("wave build config is nil")
	}

	buildOutputPaths, collectError := collectFrameworkBuildOutputPaths(
		v,
		pathsFile,
	)
	if collectError != nil {
		return collectError
	}
	if recordError := builder.RecordBuildOutputPaths(
		buildConfig,
		buildOutputPaths,
	); recordError != nil {
		return recordError
	}
	return nil
}

func collectFrameworkBuildOutputPaths(
	v *vormaruntime.Vorma,
	pathsFile *runtimepaths.PathsFile,
) ([]string, error) {
	outputPaths := make([]string, 0, 64)
	outputPaths = append(
		outputPaths,
		filepath.Join(
			v.Wave.StaticPrivateOutDir(),
			filepath.FromSlash(runtimepaths.GetVormaPathsStageOneJSONPath()),
		),
		filepath.Join(
			v.Wave.StaticPrivateOutDir(),
			filepath.FromSlash(runtimepaths.GetVormaPathsStageTwoJSONPath()),
		),
		v.Wave.ViteManifestLocation(),
	)

	if pathsFile != nil {
		outputPaths = appendPathUnderStaticPublicOut(
			outputPaths,
			v,
			pathsFile.RouteManifestFile,
		)
		outputPaths = appendPathUnderStaticPublicOut(
			outputPaths,
			v,
			pathsFile.ClientEntryOut,
		)
		for _, dependencyPath := range pathsFile.ClientEntryDeps {
			outputPaths = appendPathUnderStaticPublicOut(
				outputPaths,
				v,
				dependencyPath,
			)
		}
		for _, cssBundlePaths := range pathsFile.DepToCSSBundleMap {
			for _, cssBundlePath := range cssBundlePaths {
				outputPaths = appendPathUnderStaticPublicOut(
					outputPaths,
					v,
					cssBundlePath,
				)
			}
		}
	}

	viteManifest, manifestReadError := viteutil.ReadManifest(
		v.Wave.ViteManifestLocation(),
	)
	if manifestReadError != nil {
		return nil, fmt.Errorf("read vite manifest for ledger: %w", manifestReadError)
	}
	for _, chunk := range viteManifest {
		outputPaths = appendPathUnderStaticPublicOut(
			outputPaths,
			v,
			chunk.File,
		)
		for _, cssPath := range chunk.CSS {
			outputPaths = appendPathUnderStaticPublicOut(
				outputPaths,
				v,
				cssPath,
			)
		}
		for _, assetPath := range chunk.Assets {
			outputPaths = appendPathUnderStaticPublicOut(
				outputPaths,
				v,
				assetPath,
			)
		}
	}

	return deduplicateAndSortBuildOutputPaths(outputPaths), nil
}

func appendPathUnderStaticPublicOut(
	outputPaths []string,
	v *vormaruntime.Vorma,
	relativePath string,
) []string {
	normalizedRelativePath := normalizeBuildOutputRelativePath(relativePath)
	if normalizedRelativePath == "" {
		return outputPaths
	}
	return append(
		outputPaths,
		filepath.Join(
			v.Wave.StaticPublicOutDir(),
			filepath.FromSlash(normalizedRelativePath),
		),
	)
}

func normalizeBuildOutputRelativePath(relativePath string) string {
	trimmedRelativePath := strings.TrimSpace(relativePath)
	if trimmedRelativePath == "" {
		return ""
	}
	normalizedRelativePath := filepath.ToSlash(filepath.Clean(trimmedRelativePath))
	if normalizedRelativePath == "." || normalizedRelativePath == ".." ||
		strings.HasPrefix(normalizedRelativePath, "../") {
		return ""
	}
	if filepath.IsAbs(trimmedRelativePath) {
		return ""
	}
	return normalizedRelativePath
}

func deduplicateAndSortBuildOutputPaths(paths []string) []string {
	uniquePathSet := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		trimmedPath := strings.TrimSpace(path)
		if trimmedPath == "" {
			continue
		}
		uniquePathSet[trimmedPath] = struct{}{}
	}
	uniquePaths := make([]string, 0, len(uniquePathSet))
	for path := range uniquePathSet {
		uniquePaths = append(uniquePaths, path)
	}
	sort.Strings(uniquePaths)
	return uniquePaths
}

func applyBuildIDToVorma(v *vormaruntime.Vorma, buildID string) {
	buildlifecycle.CommitRuntimeState(
		v,
		buildlifecycle.RuntimeStateCommitInput{
			ShouldCommitBuildID: true,
			BuildID:             buildID,
		},
	)
}

type toolingBuildOptions = builder.BuildOpts

type runtimeBuildDependencies struct {
	runWaveViteProductionBuild func(*vormaruntime.Vorma) error
	runPostViteProductionBuild func(*vormaruntime.Vorma) error
	setWaveModeToDev           func()
	runWaveDevelopmentServer   func(*vormaruntime.Vorma) error
	runWaveProductionBuild     func(*vormaruntime.Vorma, toolingBuildOptions) error
}

type runtimeBuildToolingDependencies struct {
	newWaveBuilder         func(*vormaruntime.Vorma) runtimeWaveBuilder
	runWaveDevelopmentMode func(*vormaruntime.Vorma) error
}

type runtimeWaveBuilder interface {
	ViteProdBuild() error
	Build(toolingBuildOptions) error
	Close() error
}

type runtimeBuildToolingExecutor struct {
	dependencies runtimeBuildToolingDependencies
}

type runtimeBuildOperationExecutor struct {
	dependencies runtimeBuildDependencies
}

type runtimeBuildExecutor struct {
	vorma                  *vormaruntime.Vorma
	runtimeBuildOperations runtimeBuildOperationExecutor
}

func defaultRuntimeBuildToolingDependencies() runtimeBuildToolingDependencies {
	return runtimeBuildToolingDependencies{
		newWaveBuilder: func(v *vormaruntime.Vorma) runtimeWaveBuilder {
			return builder.NewBuilder(
				buildenv.Configure(v),
				v.Wave.Logger(),
			)
		},
		runWaveDevelopmentMode: func(v *vormaruntime.Vorma) error {
			return devserver.RunDev(
				buildenv.Configure(v),
				v.Wave.ConfigFile(),
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
			options toolingBuildOptions,
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

var defaultRuntimeBuildToolingExecutor = newRuntimeBuildToolingExecutor(
	runtimeBuildToolingDependencies{},
)

var defaultRuntimeBuildOperationExecutor = newRuntimeBuildOperationExecutor(
	runtimeBuildDependencies{},
)

// runFullRuntimeBuild performs a full Vorma build.
func runFullRuntimeBuild(
	v *vormaruntime.Vorma,
	isDev bool,
	noBinary bool,
) error {
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
	options toolingBuildOptions,
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
	options toolingBuildOptions,
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
	return artifactio.RunWithClosableResource(
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
		return fmt.Errorf("vite production build failed: %w", err)
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
	buildlifecycle.CommitRuntimeState(
		v,
		buildlifecycle.RuntimeStateCommitInput{
			ShouldCommitIsDev: true,
			IsDev:             true,
		},
	)
}

func productionBuildOptions(noBinary bool) toolingBuildOptions {
	return toolingBuildOptions{
		CompileGo:                          !noBinary,
		IsDev:                              false,
		IsRebuild:                          false,
		SkipPostHookPublicStaticProcessing: true,
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

// RunFullRuntimeBuild executes the full runtime build path for the selected mode.
func RunFullRuntimeBuild(
	v *vormaruntime.Vorma,
	isDev bool,
	noBinary bool,
) error {
	return runFullRuntimeBuild(v, isDev, noBinary)
}

// RunProdHookPostProcessing executes the production hook post-processing steps.
func RunProdHookPostProcessing(v *vormaruntime.Vorma) error {
	return runProdHookPostProcessing(v)
}
