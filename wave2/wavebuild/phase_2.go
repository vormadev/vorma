package wavebuild

import (
	"github.com/vormadev/vorma/kit/tasks"
)

// phase2BatchInput is the build-phase input produced by phase 1.
type phase2BatchInput struct {
	batch      phaseBatchInput
	buildGoals phase1BuildGoals
}

// phase2BuildOutcomeFacts are observable build outcome facts produced by phase 2.
//
// These facts are phase-2 outputs consumed by later phase planners; they are
// not direct watcher-event classifications.
type phase2BuildOutcomeFacts struct {
	publicFileMapArtifactsChanged  bool
	publicFileMapArtifactsRepaired bool
}

// phase2BackendSettlingGoals are backend-settling goals produced by phase 2.
type phase2BackendSettlingGoals struct {
	restartDevServerCycle          bool
	restartAppProcess              bool
	restartViteProcess             bool
	refreshFrameworkRoute          bool
	refreshFrameworkTemplate       bool
	refreshFrameworkPublicFileMap  bool
	awaitBackendReadiness          bool
	queueRetryWaitRestart          bool
	requestedTerminalBrowserAction frontendTerminalBrowserAction
}

// phase2Output is the full phase-2 planner output consumed by phase 3.
type phase2Output struct {
	backendSettlingGoals phase2BackendSettlingGoals
	buildOutcomeFacts    phase2BuildOutcomeFacts
}

func newPhase2EffectTask(
	effectID phaseEffectID,
) *tasks.Task[phase2BatchInput, struct{}] {
	return tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			input phase2BatchInput,
		) (struct{}, error) {
			if _, effectError := input.batch.runPhaseEffect(
				taskContext,
				effectID,
			); effectError != nil {
				return struct{}{}, effectError
			}
			return struct{}{}, nil
		},
	)
}

// phase2BuildGoBinaryTask executes Go compilation.
var phase2BuildGoBinaryTask = newPhase2EffectTask(
	phaseEffectIDBuildCompileGoBinary,
)

// phase2BuildCriticalCSSTask executes critical CSS compilation.
var phase2BuildCriticalCSSTask = newPhase2EffectTask(
	phaseEffectIDBuildCriticalCSS,
)

// phase2BuildNormalCSSTask executes normal CSS compilation.
var phase2BuildNormalCSSTask = newPhase2EffectTask(
	phaseEffectIDBuildNormalCSS,
)

// phase2ProcessPublicStaticAssetsTask executes public static processing.
var phase2ProcessPublicStaticAssetsTask = newPhase2EffectTask(
	phaseEffectIDBuildProcessPublicStaticAssets,
)

// phase2CleanupStalePublicStaticOutputsTask executes stale public-static cleanup.
var phase2CleanupStalePublicStaticOutputsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase2BatchInput,
	) (phase2BuildOutcomeFacts, error) {
		effectResult, effectError := input.batch.runPhaseEffect(
			taskContext,
			phaseEffectIDBuildCleanupStalePublicStaticOutputs,
		)
		if effectError != nil {
			return phase2BuildOutcomeFacts{}, effectError
		}
		return phase2BuildOutcomeFacts{
			publicFileMapArtifactsRepaired: effectResult.publicFileMapArtifactsRepaired,
		}, nil
	},
)

// phase2ProcessPrivateStaticAssetsTask executes private static processing.
var phase2ProcessPrivateStaticAssetsTask = newPhase2EffectTask(
	phaseEffectIDBuildProcessPrivateStaticAssets,
)

// phase2GeneratePublicFileMapArtifactsTask executes public file-map generation.
var phase2GeneratePublicFileMapArtifactsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase2BatchInput,
	) (phase2BuildOutcomeFacts, error) {
		if _, publicStaticProcessingError := phase2ProcessPublicStaticAssetsTask.Run(
			taskContext,
			input,
		); publicStaticProcessingError != nil {
			return phase2BuildOutcomeFacts{}, publicStaticProcessingError
		}
		cleanupFacts, staleCleanupError := phase2CleanupStalePublicStaticOutputsTask.Run(
			taskContext,
			input,
		)
		if staleCleanupError != nil {
			return phase2BuildOutcomeFacts{}, staleCleanupError
		}
		effectResult, effectError := input.batch.runPhaseEffect(
			taskContext,
			phaseEffectIDBuildGeneratePublicFileMapArtifacts,
		)
		if effectError != nil {
			return phase2BuildOutcomeFacts{}, effectError
		}
		return cleanupFacts.merge(
			phase2BuildOutcomeFacts{
				publicFileMapArtifactsChanged: effectResult.publicFileMapArtifactsChanged,
			},
		), nil
	},
)

// phase2ValidateBuildOutputsTask validates synthetic build outputs.
var phase2ValidateBuildOutputsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase2BatchInput,
	) (phase2BuildOutcomeFacts, error) {
		if !input.buildGoals.validateBuildOutputs {
			return phase2BuildOutcomeFacts{}, nil
		}

		var ignoredResult struct{}
		var cleanupFacts phase2BuildOutcomeFacts
		var generatePublicFileMapFacts phase2BuildOutcomeFacts
		boundBuildTasks := make([]tasks.BoundTask, 0, 8)
		if input.buildGoals.compileGoBinary {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2BuildGoBinaryTask.Bind(input, &ignoredResult),
			)
		}
		if input.buildGoals.buildCriticalCSS {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2BuildCriticalCSSTask.Bind(input, &ignoredResult),
			)
		}
		if input.buildGoals.buildNormalCSS {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2BuildNormalCSSTask.Bind(input, &ignoredResult),
			)
		}
		if input.buildGoals.processPublicStaticAssets {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2ProcessPublicStaticAssetsTask.Bind(input, &ignoredResult),
			)
		}
		if input.buildGoals.cleanupStalePublicStaticOutputs {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2CleanupStalePublicStaticOutputsTask.Bind(
					input,
					&cleanupFacts,
				),
			)
		}
		if input.buildGoals.processPrivateStaticAssets {
			boundBuildTasks = append(
				boundBuildTasks,
				phase2ProcessPrivateStaticAssetsTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.buildGoals.generatePublicFileMap {
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
		if _, effectError := input.batch.runPhaseEffect(
			taskContext,
			phaseEffectIDBuildValidateOutputs,
		); effectError != nil {
			return phase2BuildOutcomeFacts{}, effectError
		}
		return cleanupFacts.merge(generatePublicFileMapFacts), nil
	},
)

func (leftFacts phase2BuildOutcomeFacts) merge(
	rightFacts phase2BuildOutcomeFacts,
) phase2BuildOutcomeFacts {
	return phase2BuildOutcomeFacts{
		publicFileMapArtifactsChanged: leftFacts.publicFileMapArtifactsChanged ||
			rightFacts.publicFileMapArtifactsChanged,
		publicFileMapArtifactsRepaired: leftFacts.publicFileMapArtifactsRepaired ||
			rightFacts.publicFileMapArtifactsRepaired,
	}
}

func (buildGoals phase1BuildGoals) derivePhase2BackendSettlingGoals(
	buildOutcomeFacts phase2BuildOutcomeFacts,
) phase2BackendSettlingGoals {
	if buildGoals.queueRetryWaitRestart {
		return phase2BackendSettlingGoals{
			queueRetryWaitRestart: true,
		}
	}

	requestedTerminalBrowserAction := buildGoals.requestedTerminalBrowserAction
	if buildOutcomeFacts.publicFileMapArtifactsChanged ||
		buildOutcomeFacts.publicFileMapArtifactsRepaired {
		requestedTerminalBrowserAction = requestedTerminalBrowserAction.dominantWith(
			frontendTerminalBrowserActionNotifyVitePublicFileMapChanged,
		)
	}

	phase2BackendSettlingGoals := phase2BackendSettlingGoals{
		restartDevServerCycle: buildGoals.restartDevServerCycle,
		restartAppProcess: buildGoals.requestBackendRestart ||
			buildGoals.compileGoBinary,
		restartViteProcess:       buildGoals.requestViteRestart,
		refreshFrameworkRoute:    buildGoals.requestFrameworkRouteRefresh,
		refreshFrameworkTemplate: buildGoals.requestFrameworkTemplateRefresh,
		refreshFrameworkPublicFileMap: buildGoals.requestFrameworkPublicFileMapRefresh ||
			buildOutcomeFacts.publicFileMapArtifactsChanged ||
			buildOutcomeFacts.publicFileMapArtifactsRepaired,
		requestedTerminalBrowserAction: requestedTerminalBrowserAction,
	}
	phase2BackendSettlingGoals.awaitBackendReadiness =
		phase2BackendSettlingGoals.restartDevServerCycle ||
			phase2BackendSettlingGoals.restartAppProcess ||
			phase2BackendSettlingGoals.restartViteProcess ||
			phase2BackendSettlingGoals.refreshFrameworkRoute ||
			phase2BackendSettlingGoals.refreshFrameworkTemplate ||
			phase2BackendSettlingGoals.refreshFrameworkPublicFileMap
	return phase2BackendSettlingGoals
}

func (leftGoals phase2BackendSettlingGoals) merge(
	rightGoals phase2BackendSettlingGoals,
) phase2BackendSettlingGoals {
	return phase2BackendSettlingGoals{
		restartDevServerCycle: leftGoals.restartDevServerCycle ||
			rightGoals.restartDevServerCycle,
		restartAppProcess: leftGoals.restartAppProcess ||
			rightGoals.restartAppProcess,
		restartViteProcess: leftGoals.restartViteProcess ||
			rightGoals.restartViteProcess,
		refreshFrameworkRoute: leftGoals.refreshFrameworkRoute ||
			rightGoals.refreshFrameworkRoute,
		refreshFrameworkTemplate: leftGoals.refreshFrameworkTemplate ||
			rightGoals.refreshFrameworkTemplate,
		refreshFrameworkPublicFileMap: leftGoals.refreshFrameworkPublicFileMap ||
			rightGoals.refreshFrameworkPublicFileMap,
		awaitBackendReadiness: leftGoals.awaitBackendReadiness ||
			rightGoals.awaitBackendReadiness,
		queueRetryWaitRestart: leftGoals.queueRetryWaitRestart ||
			rightGoals.queueRetryWaitRestart,
		requestedTerminalBrowserAction: leftGoals.requestedTerminalBrowserAction.dominantWith(
			rightGoals.requestedTerminalBrowserAction,
		),
	}
}

// phase2PlanOutputTask maps build results to backend-settling goals and build outcome facts.
var phase2PlanOutputTask = tasks.NewTask(
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
			backendSettlingGoals: input.buildGoals.derivePhase2BackendSettlingGoals(
				buildOutcomeFacts,
			),
			buildOutcomeFacts: buildOutcomeFacts,
		}, nil
	},
)
