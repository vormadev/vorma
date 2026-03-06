package wavebuild

import (
	"github.com/vormadev/vorma/kit/tasks"
)

// Phase2BatchInput is the build-phase input produced by phase 1.
type Phase2BatchInput struct {
	Batch      PhaseBatchInput
	BuildGoals Phase1BuildGoals
}

// Phase2BackendSettlingGoals are backend-settling goals produced by phase 2.
type Phase2BackendSettlingGoals struct {
	RestartDevServerCycle          bool
	RestartAppProcess              bool
	RestartViteProcess             bool
	RefreshFrameworkRoute          bool
	RefreshFrameworkTemplate       bool
	RefreshFrameworkPublicFileMap  bool
	AwaitBackendReadiness          bool
	QueueRetryWaitRestart          bool
	RequestBrowserCSSHotReload     bool
	RequestBrowserInvalidateAssets bool
	RequestBrowserRevalidate       bool
	RequestBrowserHardReload       bool
	NoBrowserAction                bool
}

// Phase2EnsureBuildPhaseEnvelopeTask captures build-phase batch envelope.
var Phase2EnsureBuildPhaseEnvelopeTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (Phase2BatchInput, error) {
		return input, nil
	},
)

// Phase2EnsureWorkspaceReadyTask validates workspace readiness.
var Phase2EnsureWorkspaceReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, envelopeError := Phase2EnsureBuildPhaseEnvelopeTask.Run(
			taskContext,
			input,
		)
		if envelopeError != nil {
			return struct{}{}, envelopeError
		}
		return struct{}{}, nil
	},
)

// Phase2EnsureBuilderContextReadyTask validates builder context readiness.
var Phase2EnsureBuilderContextReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, workspaceError := Phase2EnsureWorkspaceReadyTask.Run(
			taskContext,
			input,
		)
		if workspaceError != nil {
			return struct{}{}, workspaceError
		}
		return struct{}{}, nil
	},
)

// Phase2EnsureBuildOutputDirectoriesReadyTask validates output directory readiness.
var Phase2EnsureBuildOutputDirectoriesReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, workspaceError := Phase2EnsureWorkspaceReadyTask.Run(
			taskContext,
			input,
		)
		if workspaceError != nil {
			return struct{}{}, workspaceError
		}
		return struct{}{}, nil
	},
)

// Phase2EnsureGoToolchainReadyTask validates Go toolchain readiness.
var Phase2EnsureGoToolchainReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, builderContextError := Phase2EnsureBuilderContextReadyTask.Run(
			taskContext,
			input,
		)
		if builderContextError != nil {
			return struct{}{}, builderContextError
		}
		return struct{}{}, nil
	},
)

// Phase2EnsureCSSToolchainReadyTask validates CSS toolchain readiness.
var Phase2EnsureCSSToolchainReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, builderContextError := Phase2EnsureBuilderContextReadyTask.Run(
			taskContext,
			input,
		)
		if builderContextError != nil {
			return struct{}{}, builderContextError
		}
		return struct{}{}, nil
	},
)

// Phase2EnsureStaticPipelineReadyTask validates static pipeline readiness.
var Phase2EnsureStaticPipelineReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, builderContextError := Phase2EnsureBuilderContextReadyTask.Run(
			taskContext,
			input,
		)
		if builderContextError != nil {
			return struct{}{}, builderContextError
		}
		return struct{}{}, nil
	},
)

// Phase2EnsurePublicFileMapPipelineReadyTask validates public-file-map pipeline readiness.
var Phase2EnsurePublicFileMapPipelineReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, staticPipelineError := Phase2EnsureStaticPipelineReadyTask.Run(
			taskContext,
			input,
		)
		if staticPipelineError != nil {
			return struct{}{}, staticPipelineError
		}
		return struct{}{}, nil
	},
)

// Phase2BuildGoBinaryTask executes Go compilation.
var Phase2BuildGoBinaryTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, goToolchainError := Phase2EnsureGoToolchainReadyTask.Run(
			taskContext,
			input,
		)
		if goToolchainError != nil {
			return struct{}{}, goToolchainError
		}
		_, outputDirectoryError := Phase2EnsureBuildOutputDirectoriesReadyTask.Run(
			taskContext,
			input,
		)
		if outputDirectoryError != nil {
			return struct{}{}, outputDirectoryError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBuildCompileGoBinary,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase2BuildCriticalCSSTask executes critical CSS compilation.
var Phase2BuildCriticalCSSTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, cssToolchainError := Phase2EnsureCSSToolchainReadyTask.Run(
			taskContext,
			input,
		)
		if cssToolchainError != nil {
			return struct{}{}, cssToolchainError
		}
		_, outputDirectoryError := Phase2EnsureBuildOutputDirectoriesReadyTask.Run(
			taskContext,
			input,
		)
		if outputDirectoryError != nil {
			return struct{}{}, outputDirectoryError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBuildCriticalCSS,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase2BuildNormalCSSTask executes normal CSS compilation.
var Phase2BuildNormalCSSTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, cssToolchainError := Phase2EnsureCSSToolchainReadyTask.Run(
			taskContext,
			input,
		)
		if cssToolchainError != nil {
			return struct{}{}, cssToolchainError
		}
		_, outputDirectoryError := Phase2EnsureBuildOutputDirectoriesReadyTask.Run(
			taskContext,
			input,
		)
		if outputDirectoryError != nil {
			return struct{}{}, outputDirectoryError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBuildNormalCSS,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase2ProcessPublicStaticAssetsTask executes public static processing.
var Phase2ProcessPublicStaticAssetsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, staticPipelineError := Phase2EnsureStaticPipelineReadyTask.Run(
			taskContext,
			input,
		)
		if staticPipelineError != nil {
			return struct{}{}, staticPipelineError
		}
		_, outputDirectoryError := Phase2EnsureBuildOutputDirectoriesReadyTask.Run(
			taskContext,
			input,
		)
		if outputDirectoryError != nil {
			return struct{}{}, outputDirectoryError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBuildProcessPublicStaticAssets,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase2CleanupStalePublicStaticOutputsTask executes stale public-static cleanup.
var Phase2CleanupStalePublicStaticOutputsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, staticPipelineError := Phase2EnsureStaticPipelineReadyTask.Run(
			taskContext,
			input,
		)
		if staticPipelineError != nil {
			return struct{}{}, staticPipelineError
		}
		_, outputDirectoryError := Phase2EnsureBuildOutputDirectoriesReadyTask.Run(
			taskContext,
			input,
		)
		if outputDirectoryError != nil {
			return struct{}{}, outputDirectoryError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBuildCleanupStalePublicStaticOutputs,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase2ProcessPrivateStaticAssetsTask executes private static processing.
var Phase2ProcessPrivateStaticAssetsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, staticPipelineError := Phase2EnsureStaticPipelineReadyTask.Run(
			taskContext,
			input,
		)
		if staticPipelineError != nil {
			return struct{}{}, staticPipelineError
		}
		_, outputDirectoryError := Phase2EnsureBuildOutputDirectoriesReadyTask.Run(
			taskContext,
			input,
		)
		if outputDirectoryError != nil {
			return struct{}{}, outputDirectoryError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBuildProcessPrivateStaticAssets,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase2GeneratePublicFileMapArtifactsTask executes public file-map generation.
var Phase2GeneratePublicFileMapArtifactsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, pipelineError := Phase2EnsurePublicFileMapPipelineReadyTask.Run(
			taskContext,
			input,
		)
		if pipelineError != nil {
			return struct{}{}, pipelineError
		}
		_, publicStaticProcessingError := Phase2ProcessPublicStaticAssetsTask.Run(
			taskContext,
			input,
		)
		if publicStaticProcessingError != nil {
			return struct{}{}, publicStaticProcessingError
		}
		_, staleCleanupError := Phase2CleanupStalePublicStaticOutputsTask.Run(
			taskContext,
			input,
		)
		if staleCleanupError != nil {
			return struct{}{}, staleCleanupError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBuildGeneratePublicFileMapArtifacts,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase2ValidateBuildOutputsTask validates synthetic build outputs.
var Phase2ValidateBuildOutputsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		_, envelopeError := Phase2EnsureBuildPhaseEnvelopeTask.Run(
			taskContext,
			input,
		)
		if envelopeError != nil {
			return struct{}{}, envelopeError
		}

		var ignoredResult struct{}
		boundBuildTasks := make([]tasks.BoundTask, 0, 8)
		if input.BuildGoals.CompileGoBinary {
			boundBuildTasks = append(
				boundBuildTasks,
				Phase2BuildGoBinaryTask.Bind(input, &ignoredResult),
			)
		}
		if input.BuildGoals.BuildCriticalCSS {
			boundBuildTasks = append(
				boundBuildTasks,
				Phase2BuildCriticalCSSTask.Bind(input, &ignoredResult),
			)
		}
		if input.BuildGoals.BuildNormalCSS {
			boundBuildTasks = append(
				boundBuildTasks,
				Phase2BuildNormalCSSTask.Bind(input, &ignoredResult),
			)
		}
		if input.BuildGoals.ProcessPublicStaticAssets {
			boundBuildTasks = append(
				boundBuildTasks,
				Phase2ProcessPublicStaticAssetsTask.Bind(input, &ignoredResult),
			)
		}
		if input.BuildGoals.CleanupStalePublicStaticOutputs {
			boundBuildTasks = append(
				boundBuildTasks,
				Phase2CleanupStalePublicStaticOutputsTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.BuildGoals.ProcessPrivateStaticAssets {
			boundBuildTasks = append(
				boundBuildTasks,
				Phase2ProcessPrivateStaticAssetsTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.BuildGoals.GeneratePublicFileMap {
			boundBuildTasks = append(
				boundBuildTasks,
				Phase2GeneratePublicFileMapArtifactsTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if runParallelError := taskContext.RunParallel(boundBuildTasks...); runParallelError != nil {
			return struct{}{}, runParallelError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBuildValidateOutputs,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

func reducePhase2BackendSettlingGoals(
	buildGoals Phase1BuildGoals,
) Phase2BackendSettlingGoals {
	if buildGoals.QueueRetryWaitRestart {
		return Phase2BackendSettlingGoals{
			QueueRetryWaitRestart: true,
			NoBrowserAction:       true,
		}
	}

	phase2BackendSettlingGoals := Phase2BackendSettlingGoals{
		RestartDevServerCycle: buildGoals.RestartDevServerCycle,
		RestartAppProcess: buildGoals.RequestBackendRestart ||
			buildGoals.CompileGoBinary,
		RestartViteProcess:       buildGoals.RequestViteRestart,
		RefreshFrameworkRoute:    buildGoals.RequestFrameworkRouteRefresh,
		RefreshFrameworkTemplate: buildGoals.RequestFrameworkTemplateRefresh,
		RefreshFrameworkPublicFileMap: buildGoals.RequestFrameworkPublicFileMapRefresh ||
			buildGoals.GeneratePublicFileMap ||
			buildGoals.CleanupStalePublicStaticOutputs,
		RequestBrowserCSSHotReload:     buildGoals.RequestBrowserCSSHotReload,
		RequestBrowserInvalidateAssets: buildGoals.RequestBrowserInvalidatePublicAssets,
		RequestBrowserRevalidate:       buildGoals.RequestBrowserRevalidate,
		RequestBrowserHardReload:       buildGoals.RequestBrowserHardReload,
	}
	phase2BackendSettlingGoals.AwaitBackendReadiness =
		phase2BackendSettlingGoals.RestartDevServerCycle ||
			phase2BackendSettlingGoals.RestartAppProcess ||
			phase2BackendSettlingGoals.RestartViteProcess ||
			phase2BackendSettlingGoals.RefreshFrameworkRoute ||
			phase2BackendSettlingGoals.RefreshFrameworkTemplate ||
			phase2BackendSettlingGoals.RefreshFrameworkPublicFileMap
	phase2BackendSettlingGoals.NoBrowserAction =
		!phase2BackendSettlingGoals.RequestBrowserCSSHotReload &&
			!phase2BackendSettlingGoals.RequestBrowserInvalidateAssets &&
			!phase2BackendSettlingGoals.RequestBrowserRevalidate &&
			!phase2BackendSettlingGoals.RequestBrowserHardReload
	return phase2BackendSettlingGoals
}

// Phase2PlanBackendSettlingGoalsTask maps build results to backend-settling goals.
var Phase2PlanBackendSettlingGoalsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (Phase2BackendSettlingGoals, error) {
		_, validateError := Phase2ValidateBuildOutputsTask.Run(
			taskContext,
			input,
		)
		if validateError != nil {
			return Phase2BackendSettlingGoals{}, validateError
		}
		return reducePhase2BackendSettlingGoals(input.BuildGoals), nil
	},
)

// Phase2RootCompileGoBinaryTask is a terminal build-goal root.
var Phase2RootCompileGoBinaryTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		return Phase2BuildGoBinaryTask.Run(taskContext, input)
	},
)

// Phase2RootBuildCriticalCSSTask is a terminal critical-css build root.
var Phase2RootBuildCriticalCSSTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		return Phase2BuildCriticalCSSTask.Run(taskContext, input)
	},
)

// Phase2RootBuildNormalCSSTask is a terminal normal-css build root.
var Phase2RootBuildNormalCSSTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		return Phase2BuildNormalCSSTask.Run(taskContext, input)
	},
)

// Phase2RootProcessPublicStaticAssetsTask is a terminal public-static root.
var Phase2RootProcessPublicStaticAssetsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		return Phase2ProcessPublicStaticAssetsTask.Run(taskContext, input)
	},
)

// Phase2RootCleanupStalePublicStaticOutputsTask is a terminal stale-public-static cleanup root.
var Phase2RootCleanupStalePublicStaticOutputsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		return Phase2CleanupStalePublicStaticOutputsTask.Run(taskContext, input)
	},
)

// Phase2RootProcessPrivateStaticAssetsTask is a terminal private-static root.
var Phase2RootProcessPrivateStaticAssetsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		return Phase2ProcessPrivateStaticAssetsTask.Run(taskContext, input)
	},
)

// Phase2RootGeneratePublicFileMapArtifactsTask is a terminal file-map root.
var Phase2RootGeneratePublicFileMapArtifactsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		return Phase2GeneratePublicFileMapArtifactsTask.Run(taskContext, input)
	},
)

// Phase2RootValidateBuildOutputsTask is a terminal validation root.
var Phase2RootValidateBuildOutputsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		return Phase2ValidateBuildOutputsTask.Run(taskContext, input)
	},
)

// RunPhase2TaskGraph executes phase-2 task roots then plans phase 3 goals.
func RunPhase2TaskGraph(
	taskContext *tasks.Ctx,
	input Phase2BatchInput,
) (Phase2BackendSettlingGoals, error) {
	var ignoredResult struct{}
	terminalBuildRoots := make([]tasks.BoundTask, 0, 9)

	if input.BuildGoals.CompileGoBinary {
		terminalBuildRoots = append(
			terminalBuildRoots,
			Phase2RootCompileGoBinaryTask.Bind(input, &ignoredResult),
		)
	}
	if input.BuildGoals.BuildCriticalCSS {
		terminalBuildRoots = append(
			terminalBuildRoots,
			Phase2RootBuildCriticalCSSTask.Bind(input, &ignoredResult),
		)
	}
	if input.BuildGoals.BuildNormalCSS {
		terminalBuildRoots = append(
			terminalBuildRoots,
			Phase2RootBuildNormalCSSTask.Bind(input, &ignoredResult),
		)
	}
	if input.BuildGoals.ProcessPublicStaticAssets {
		terminalBuildRoots = append(
			terminalBuildRoots,
			Phase2RootProcessPublicStaticAssetsTask.Bind(input, &ignoredResult),
		)
	}
	if input.BuildGoals.CleanupStalePublicStaticOutputs {
		terminalBuildRoots = append(
			terminalBuildRoots,
			Phase2RootCleanupStalePublicStaticOutputsTask.Bind(
				input,
				&ignoredResult,
			),
		)
	}
	if input.BuildGoals.ProcessPrivateStaticAssets {
		terminalBuildRoots = append(
			terminalBuildRoots,
			Phase2RootProcessPrivateStaticAssetsTask.Bind(
				input,
				&ignoredResult,
			),
		)
	}
	if input.BuildGoals.GeneratePublicFileMap {
		terminalBuildRoots = append(
			terminalBuildRoots,
			Phase2RootGeneratePublicFileMapArtifactsTask.Bind(
				input,
				&ignoredResult,
			),
		)
	}
	if input.BuildGoals.ValidateBuildOutputs {
		terminalBuildRoots = append(
			terminalBuildRoots,
			Phase2RootValidateBuildOutputsTask.Bind(input, &ignoredResult),
		)
	}
	if runParallelError := taskContext.RunParallel(terminalBuildRoots...); runParallelError != nil {
		return Phase2BackendSettlingGoals{}, runParallelError
	}
	return Phase2PlanBackendSettlingGoalsTask.Run(taskContext, input)
}
