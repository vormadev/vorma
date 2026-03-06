package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p2_Effects struct {
	buildGoBinary                  *tasks.Task[p2_BatchInput, struct{}]
	buildCriticalCSS               *tasks.Task[p2_BatchInput, struct{}]
	buildNormalCSS                 *tasks.Task[p2_BatchInput, struct{}]
	processPublicStaticAssets      *tasks.Task[p2_BatchInput, struct{}]
	cleanupStalePublicStatic       *tasks.Task[p2_BatchInput, p2_BuildOutcomeFacts]
	processPrivateStaticAssets     *tasks.Task[p2_BatchInput, struct{}]
	generatePublicFileMapArtifacts *tasks.Task[p2_BatchInput, p2_BuildOutcomeFacts]
	validateBuildOutputs           *tasks.Task[p2_BatchInput, p2_BuildOutcomeFacts]
	planP2_Output                  *tasks.Task[p2_BatchInput, p2_Output]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p2_EffectsDef = p2_Effects{
	buildGoBinary:                  p2_BuildGoBinaryTask,
	buildCriticalCSS:               p2_BuildCriticalCSSTask,
	buildNormalCSS:                 p2_BuildNormalCSSTask,
	processPublicStaticAssets:      p2_ProcessPublicStaticAssetsTask,
	cleanupStalePublicStatic:       p2_CleanupStalePublicStaticTask,
	processPrivateStaticAssets:     p2_ProcessPrivateStaticAssetsTask,
	generatePublicFileMapArtifacts: p2_GeneratePublicFileMapArtifactsTask,
	validateBuildOutputs:           p2_ValidateBuildOutputsTask,
	planP2_Output:                  p2_PlanP2_OutputTask,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var p2_BuildGoBinaryTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p2_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P2_BUILD_GO_BINARY) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p2_BuildCriticalCSSTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p2_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P2_BUILD_CRITICAL_CSS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p2_BuildNormalCSSTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p2_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P2_BUILD_NORMAL_CSS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p2_ProcessPublicStaticAssetsTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p2_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P2_PROCESS_PUBLIC_STATIC_ASSETS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p2_CleanupStalePublicStaticTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p2_BatchInput,
	) (p2_BuildOutcomeFacts, error) {
		if recordTestEffect(tasksCtx, _LABEL_P2_CLEANUP_STALE_PUBLIC_STATIC) {
			return p2_BuildOutcomeFacts{}, nil
		}
		return p2_BuildOutcomeFacts{}, nil
	},
)

var p2_ProcessPrivateStaticAssetsTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p2_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P2_PROCESS_PRIVATE_STATIC_ASSETS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p2_GeneratePublicFileMapArtifactsTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p2_BatchInput,
	) (p2_BuildOutcomeFacts, error) {
		if recordTestEffect(
			tasksCtx,
			_LABEL_P2_GENERATE_PUBLIC_FILEMAP_ARTIFACTS,
		) {
			return p2_BuildOutcomeFacts{}, nil
		}
		if _, publicStaticProcessingError := p2_ProcessPublicStaticAssetsTask.Run(
			tasksCtx,
			input,
		); publicStaticProcessingError != nil {
			return p2_BuildOutcomeFacts{}, publicStaticProcessingError
		}
		cleanupFacts, staleCleanupError := p2_CleanupStalePublicStaticTask.Run(
			tasksCtx,
			input,
		)
		if staleCleanupError != nil {
			return p2_BuildOutcomeFacts{}, staleCleanupError
		}
		return cleanupFacts.merge(p2_BuildOutcomeFacts{}), nil
	},
)

var p2_ValidateBuildOutputsTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p2_BatchInput,
	) (p2_BuildOutcomeFacts, error) {
		if recordTestEffect(tasksCtx, _LABEL_P2_VALIDATE_BUILD_OUTPUTS) {
			return p2_BuildOutcomeFacts{}, nil
		}
		if !input.p1_RequestedEffects.validateBuildOutputs {
			return p2_BuildOutcomeFacts{}, nil
		}

		var ignoredResult struct{}
		var cleanupFacts p2_BuildOutcomeFacts
		var generatePublicFileMapFacts p2_BuildOutcomeFacts
		boundBuildTasks := make([]tasks.BoundTask, 0, 8)
		if input.p1_RequestedEffects.compileGoBinary {
			boundBuildTasks = append(
				boundBuildTasks,
				p2_BuildGoBinaryTask.Bind(input, &ignoredResult),
			)
		}
		if input.p1_RequestedEffects.buildCriticalCSS {
			boundBuildTasks = append(
				boundBuildTasks,
				p2_BuildCriticalCSSTask.Bind(input, &ignoredResult),
			)
		}
		if input.p1_RequestedEffects.buildNormalCSS {
			boundBuildTasks = append(
				boundBuildTasks,
				p2_BuildNormalCSSTask.Bind(input, &ignoredResult),
			)
		}
		if input.p1_RequestedEffects.processPublicStaticAssets {
			boundBuildTasks = append(
				boundBuildTasks,
				p2_ProcessPublicStaticAssetsTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.p1_RequestedEffects.cleanupStalePublicStaticOutputs {
			boundBuildTasks = append(
				boundBuildTasks,
				p2_CleanupStalePublicStaticTask.Bind(
					input,
					&cleanupFacts,
				),
			)
		}
		if input.p1_RequestedEffects.processPrivateStaticAssets {
			boundBuildTasks = append(
				boundBuildTasks,
				p2_ProcessPrivateStaticAssetsTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.p1_RequestedEffects.generatePublicFileMap {
			boundBuildTasks = append(
				boundBuildTasks,
				p2_GeneratePublicFileMapArtifactsTask.Bind(
					input,
					&generatePublicFileMapFacts,
				),
			)
		}
		if runParallelError := tasksCtx.RunParallel(boundBuildTasks...); runParallelError != nil {
			return p2_BuildOutcomeFacts{}, runParallelError
		}
		return cleanupFacts.merge(generatePublicFileMapFacts), nil
	},
)

var p2_PlanP2_OutputTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p2_BatchInput,
	) (p2_Output, error) {
		if recordTestEffect(tasksCtx, _LABEL_P2_PLAN_PHASE_2_OUTPUT) {
			return p2_Output{}, nil
		}
		buildOutcomeFacts, validateError := p2_ValidateBuildOutputsTask.Run(
			tasksCtx,
			input,
		)
		if validateError != nil {
			return p2_Output{}, validateError
		}
		if input.batch.mode == modeProd {
			return p2_Output{
				buildOutcomeFacts: buildOutcomeFacts,
			}, nil
		}
		return p2_Output{
			p2_RequestedEffects: input.p1_RequestedEffects.deriveP2_RequestedEffects(
				buildOutcomeFacts,
			),
			buildOutcomeFacts: buildOutcomeFacts,
		}, nil
	},
)
