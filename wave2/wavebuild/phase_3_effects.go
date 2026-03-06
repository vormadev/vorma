package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type phase3Effects struct {
	applyDevServerRestart         *tasks.Task[phase3BatchInput, struct{}]
	queueRetryWaitRestart         *tasks.Task[phase3BatchInput, struct{}]
	restartAppProcess             *tasks.Task[phase3BatchInput, struct{}]
	restartViteProcess            *tasks.Task[phase3BatchInput, struct{}]
	refreshFrameworkRoute         *tasks.Task[phase3BatchInput, struct{}]
	refreshFrameworkTemplate      *tasks.Task[phase3BatchInput, struct{}]
	refreshFrameworkPublicFileMap *tasks.Task[phase3BatchInput, struct{}]
	awaitBackendReadiness         *tasks.Task[phase3BatchInput, struct{}]
	planPhase3RequestedEffects    *tasks.Task[phase3BatchInput, phase3RequestedEffects]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var phase3EffectsDef = phase3Effects{
	applyDevServerRestart:         phase3ApplyDevServerRestartTask,
	queueRetryWaitRestart:         phase3QueueRetryWaitRestartTask,
	restartAppProcess:             phase3RestartAppProcessTask,
	restartViteProcess:            phase3RestartViteProcessTask,
	refreshFrameworkRoute:         phase3RefreshFrameworkRouteTask,
	refreshFrameworkTemplate:      phase3RefreshFrameworkTemplateTask,
	refreshFrameworkPublicFileMap: phase3RefreshFrameworkPublicFileMapTask,
	awaitBackendReadiness:         phase3AwaitBackendReadinessTask,
	planPhase3RequestedEffects:    phase3PlanPhase3RequestedEffectsTask,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var noopPhase3EffectTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase3BatchInput,
	) (struct{}, error) {
		return struct{}{}, nil
	},
)

var phase3ApplyDevServerRestartTask = noopPhase3EffectTask
var phase3QueueRetryWaitRestartTask = noopPhase3EffectTask
var phase3RestartAppProcessTask = noopPhase3EffectTask
var phase3RestartViteProcessTask = noopPhase3EffectTask
var phase3RefreshFrameworkRouteTask = noopPhase3EffectTask
var phase3RefreshFrameworkTemplateTask = noopPhase3EffectTask
var phase3RefreshFrameworkPublicFileMapTask = noopPhase3EffectTask

var phase3AwaitBackendReadinessTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase3BatchInput,
	) (struct{}, error) {
		if input.phase2RequestedEffects.queueRetryWaitRestart {
			if _, queuedRestartError := phase3QueueRetryWaitRestartTask.Run(
				taskContext,
				input,
			); queuedRestartError != nil {
				return struct{}{}, queuedRestartError
			}
			return struct{}{}, nil
		}
		if !input.phase2RequestedEffects.awaitBackendReadiness {
			return struct{}{}, nil
		}

		var ignoredResult struct{}
		backendSettlingTasks := make([]tasks.BoundTask, 0, 6)
		if input.phase2RequestedEffects.restartDevServerCycle {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3ApplyDevServerRestartTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.phase2RequestedEffects.restartAppProcess {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3RestartAppProcessTask.Bind(input, &ignoredResult),
			)
		}
		if input.phase2RequestedEffects.restartViteProcess {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3RestartViteProcessTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.phase2RequestedEffects.refreshFrameworkRoute {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3RefreshFrameworkRouteTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.phase2RequestedEffects.refreshFrameworkTemplate {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3RefreshFrameworkTemplateTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.phase2RequestedEffects.refreshFrameworkPublicFileMap {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3RefreshFrameworkPublicFileMapTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if runParallelError := taskContext.RunParallel(backendSettlingTasks...); runParallelError != nil {
			return struct{}{}, runParallelError
		}
		return struct{}{}, nil
	},
)

var phase3PlanPhase3RequestedEffectsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase3BatchInput,
	) (phase3RequestedEffects, error) {
		if _, awaitReadyError := phase3AwaitBackendReadinessTask.Run(
			taskContext,
			input,
		); awaitReadyError != nil {
			return phase3RequestedEffects{}, awaitReadyError
		}
		return input.phase2RequestedEffects.derivePhase3RequestedEffects(), nil
	},
)
