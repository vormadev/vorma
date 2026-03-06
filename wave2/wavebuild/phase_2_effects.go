package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type phase2Effects struct {
	buildGoBinary                  *tasks.Task[phase2BatchInput, struct{}]
	buildCriticalCSS               *tasks.Task[phase2BatchInput, struct{}]
	buildNormalCSS                 *tasks.Task[phase2BatchInput, struct{}]
	processPublicStaticAssets      *tasks.Task[phase2BatchInput, struct{}]
	cleanupStalePublicStatic       *tasks.Task[phase2BatchInput, phase2BuildOutcomeFacts]
	processPrivateStaticAssets     *tasks.Task[phase2BatchInput, struct{}]
	generatePublicFileMapArtifacts *tasks.Task[phase2BatchInput, phase2BuildOutcomeFacts]
	validateBuildOutputs           *tasks.Task[phase2BatchInput, phase2BuildOutcomeFacts]
	planPhase2Output               *tasks.Task[phase2BatchInput, phase2Output]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var phase2EffectsDef = phase2Effects{
	buildGoBinary:                  phase2BuildGoBinaryTask,
	buildCriticalCSS:               phase2BuildCriticalCSSTask,
	buildNormalCSS:                 phase2BuildNormalCSSTask,
	processPublicStaticAssets:      phase2ProcessPublicStaticAssetsTask,
	cleanupStalePublicStatic:       phase2CleanupStalePublicStaticTask,
	processPrivateStaticAssets:     phase2ProcessPrivateStaticAssetsTask,
	generatePublicFileMapArtifacts: phase2GeneratePublicFileMapArtifactsTask,
	validateBuildOutputs:           phase2ValidateBuildOutputsTask,
	planPhase2Output:               phase2PlanPhase2OutputTask,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var phase2BuildGoBinaryTask = noopPhase2EffectTask

var phase2BuildCriticalCSSTask = noopPhase2EffectTask

var phase2BuildNormalCSSTask = noopPhase2EffectTask

var phase2ProcessPublicStaticAssetsTask = noopPhase2EffectTask

var phase2CleanupStalePublicStaticTask = tasks.NewTask(
	func(taskContext *tasks.Ctx, input phase2BatchInput,
	) (phase2BuildOutcomeFacts, error) {
		return phase2BuildOutcomeFacts{}, nil
	},
)

var phase2ProcessPrivateStaticAssetsTask = noopPhase2EffectTask

var phase2GeneratePublicFileMapArtifactsTask = tasks.NewTask(
	func(taskContext *tasks.Ctx, input phase2BatchInput) (phase2BuildOutcomeFacts, error) {
		if _, publicStaticProcessingError := phase2ProcessPublicStaticAssetsTask.Run(
			taskContext,
			input,
		); publicStaticProcessingError != nil {
			return phase2BuildOutcomeFacts{}, publicStaticProcessingError
		}
		cleanupFacts, staleCleanupError := phase2CleanupStalePublicStaticTask.Run(
			taskContext,
			input,
		)
		if staleCleanupError != nil {
			return phase2BuildOutcomeFacts{}, staleCleanupError
		}
		return cleanupFacts.merge(phase2BuildOutcomeFacts{}), nil
	},
)

var phase2ValidateBuildOutputsTask = tasks.NewTask(
	func(taskContext *tasks.Ctx, input phase2BatchInput) (phase2BuildOutcomeFacts, error) {
		if !input.phase1RequestedEffects.validateBuildOutputs {
			return phase2BuildOutcomeFacts{}, nil
		}

		var ignoredResult struct{}
		var cleanupFacts phase2BuildOutcomeFacts
		var generatePublicFileMapFacts phase2BuildOutcomeFacts
		boundBuildTasks := make([]tasks.BoundTask, 0, 8)
		if input.phase1RequestedEffects.compileGoBinary {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2BuildGoBinaryTask.Bind(input, &ignoredResult),
			)
		}
		if input.phase1RequestedEffects.buildCriticalCSS {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2BuildCriticalCSSTask.Bind(input, &ignoredResult),
			)
		}
		if input.phase1RequestedEffects.buildNormalCSS {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2BuildNormalCSSTask.Bind(input, &ignoredResult),
			)
		}
		if input.phase1RequestedEffects.processPublicStaticAssets {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2ProcessPublicStaticAssetsTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.phase1RequestedEffects.cleanupStalePublicStaticOutputs {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2CleanupStalePublicStaticTask.Bind(
					input,
					&cleanupFacts,
				),
			)
		}
		if input.phase1RequestedEffects.processPrivateStaticAssets {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2ProcessPrivateStaticAssetsTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.phase1RequestedEffects.generatePublicFileMap {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2GeneratePublicFileMapArtifactsTask.Bind(
					input,
					&generatePublicFileMapFacts,
				),
			)
		}
		if runParallelError := taskContext.RunParallel(boundBuildTasks...); runParallelError != nil {
			return phase2BuildOutcomeFacts{}, runParallelError
		}
		return cleanupFacts.merge(generatePublicFileMapFacts), nil
	},
)

var phase2PlanPhase2OutputTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase2BatchInput,
	) (phase2Output, error) {
		buildOutcomeFacts, validateError := phase2ValidateBuildOutputsTask.Run(
			taskContext,
			input,
		)
		if validateError != nil {
			return phase2Output{}, validateError
		}
		if input.batch.mode == modeProd {
			return phase2Output{
				buildOutcomeFacts: buildOutcomeFacts,
			}, nil
		}
		return phase2Output{
			phase2RequestedEffects: input.phase1RequestedEffects.derivePhase2RequestedEffects(
				buildOutcomeFacts,
			),
			buildOutcomeFacts: buildOutcomeFacts,
		}, nil
	},
)
