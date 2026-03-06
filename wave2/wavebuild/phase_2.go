package wavebuild

import (
	"github.com/vormadev/vorma/kit/tasks"
)

// Phase2BatchInput is the build-phase input produced by phase 1.
type Phase2BatchInput struct {
	Batch      PhaseBatchInput
	BuildGoals Phase1BuildGoals
}

// Phase2BuildOutcomeFacts are observable build outcome facts produced by phase 2.
//
// These facts are phase-2 outputs consumed by later phase planners; they are
// not direct watcher-event classifications.
type Phase2BuildOutcomeFacts struct {
	PublicFileMapArtifactsChanged  bool
	PublicFileMapArtifactsRepaired bool
}

// Phase2BackendSettlingGoals are backend-settling goals produced by phase 2.
type Phase2BackendSettlingGoals struct {
	RestartDevServerCycle                        bool
	RestartAppProcess                            bool
	RestartViteProcess                           bool
	RefreshFrameworkRoute                        bool
	RefreshFrameworkTemplate                     bool
	RefreshFrameworkPublicFileMap                bool
	AwaitBackendReadiness                        bool
	QueueRetryWaitRestart                        bool
	RequestBrowserCSSHotReload                   bool
	RequestBrowserNotifyVitePublicFileMapChanged bool
	RequestBrowserRevalidate                     bool
	RequestBrowserHardReload                     bool
}

// Phase2Output is the full phase-2 planner output consumed by phase 3.
type Phase2Output struct {
	BackendSettlingGoals Phase2BackendSettlingGoals
	BuildOutcomeFacts    Phase2BuildOutcomeFacts
}

func newPhase2EffectTask(
	effectID PhaseEffectID,
) *tasks.Task[Phase2BatchInput, struct{}] {
	return tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			input Phase2BatchInput,
		) (struct{}, error) {
			if effectError := executePhaseEffect(
				taskContext,
				input.Batch,
				effectID,
			); effectError != nil {
				return struct{}{}, effectError
			}
			return struct{}{}, nil
		},
	)
}

// Phase2BuildGoBinaryTask executes Go compilation.
var Phase2BuildGoBinaryTask = newPhase2EffectTask(
	PhaseEffectIDBuildCompileGoBinary,
)

// Phase2BuildCriticalCSSTask executes critical CSS compilation.
var Phase2BuildCriticalCSSTask = newPhase2EffectTask(
	PhaseEffectIDBuildCriticalCSS,
)

// Phase2BuildNormalCSSTask executes normal CSS compilation.
var Phase2BuildNormalCSSTask = newPhase2EffectTask(
	PhaseEffectIDBuildNormalCSS,
)

// Phase2ProcessPublicStaticAssetsTask executes public static processing.
var Phase2ProcessPublicStaticAssetsTask = newPhase2EffectTask(
	PhaseEffectIDBuildProcessPublicStaticAssets,
)

// Phase2CleanupStalePublicStaticOutputsTask executes stale public-static cleanup.
var Phase2CleanupStalePublicStaticOutputsTask = newPhase2EffectTask(
	PhaseEffectIDBuildCleanupStalePublicStaticOutputs,
)

// Phase2ProcessPrivateStaticAssetsTask executes private static processing.
var Phase2ProcessPrivateStaticAssetsTask = newPhase2EffectTask(
	PhaseEffectIDBuildProcessPrivateStaticAssets,
)

// Phase2GeneratePublicFileMapArtifactsTask executes public file-map generation.
var Phase2GeneratePublicFileMapArtifactsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (struct{}, error) {
		if _, publicStaticProcessingError := Phase2ProcessPublicStaticAssetsTask.Run(
			taskContext,
			input,
		); publicStaticProcessingError != nil {
			return struct{}{}, publicStaticProcessingError
		}
		if _, staleCleanupError := Phase2CleanupStalePublicStaticOutputsTask.Run(
			taskContext,
			input,
		); staleCleanupError != nil {
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
		if !input.BuildGoals.ValidateBuildOutputs {
			return struct{}{}, nil
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
	buildOutcomeFacts Phase2BuildOutcomeFacts,
) Phase2BackendSettlingGoals {
	if buildGoals.QueueRetryWaitRestart {
		return Phase2BackendSettlingGoals{
			QueueRetryWaitRestart: true,
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
			buildOutcomeFacts.PublicFileMapArtifactsChanged ||
			buildOutcomeFacts.PublicFileMapArtifactsRepaired,
		RequestBrowserCSSHotReload: buildGoals.RequestBrowserCSSHotReload,
		RequestBrowserNotifyVitePublicFileMapChanged: buildGoals.RequestBrowserNotifyVitePublicFileMapChanged ||
			buildOutcomeFacts.PublicFileMapArtifactsChanged ||
			buildOutcomeFacts.PublicFileMapArtifactsRepaired,
		RequestBrowserRevalidate: buildGoals.RequestBrowserRevalidate,
		RequestBrowserHardReload: buildGoals.RequestBrowserHardReload,
	}
	phase2BackendSettlingGoals.AwaitBackendReadiness =
		phase2BackendSettlingGoals.RestartDevServerCycle ||
			phase2BackendSettlingGoals.RestartAppProcess ||
			phase2BackendSettlingGoals.RestartViteProcess ||
			phase2BackendSettlingGoals.RefreshFrameworkRoute ||
			phase2BackendSettlingGoals.RefreshFrameworkTemplate ||
			phase2BackendSettlingGoals.RefreshFrameworkPublicFileMap
	return phase2BackendSettlingGoals
}

func reducePhase2BuildOutcomeFacts(
	buildGoals Phase1BuildGoals,
) Phase2BuildOutcomeFacts {
	return Phase2BuildOutcomeFacts{
		PublicFileMapArtifactsChanged:  buildGoals.GeneratePublicFileMap,
		PublicFileMapArtifactsRepaired: buildGoals.CleanupStalePublicStaticOutputs,
	}
}

// Phase2PlanOutputTask maps build results to backend-settling goals and build outcome facts.
var Phase2PlanOutputTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase2BatchInput,
	) (Phase2Output, error) {
		if _, validateError := Phase2ValidateBuildOutputsTask.Run(
			taskContext,
			input,
		); validateError != nil {
			return Phase2Output{}, validateError
		}
		buildOutcomeFacts := reducePhase2BuildOutcomeFacts(input.BuildGoals)
		if input.Batch.Mode == ModeProd {
			return Phase2Output{
				BuildOutcomeFacts: buildOutcomeFacts,
			}, nil
		}
		return Phase2Output{
			BackendSettlingGoals: reducePhase2BackendSettlingGoals(
				input.BuildGoals,
				buildOutcomeFacts,
			),
			BuildOutcomeFacts: buildOutcomeFacts,
		}, nil
	},
)
