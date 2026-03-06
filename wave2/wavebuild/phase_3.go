package wavebuild

import (
	"github.com/vormadev/vorma/kit/tasks"
)

// phase3BatchInput is the backend-settling input produced by phase 2.
type phase3BatchInput struct {
	batch        phaseBatchInput
	backendGoals phase2BackendSettlingGoals
}

// phase3FrontendSettlingGoals are frontend-settling goals produced by phase 3.
type phase3FrontendSettlingGoals struct {
	terminalBrowserAction frontendTerminalBrowserAction
}

func newPhase3EffectTask(
	effectID phaseEffectID,
) *tasks.Task[phase3BatchInput, struct{}] {
	return tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			input phase3BatchInput,
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

// phase3ApplyDevServerRestartTask executes dev-server restart.
var phase3ApplyDevServerRestartTask = newPhase3EffectTask(
	phaseEffectIDBackendApplyDevServerRestart,
)

// phase3QueueRetryWaitRestartTask executes queued retry-wait restart handling.
var phase3QueueRetryWaitRestartTask = newPhase3EffectTask(
	phaseEffectIDBackendQueueRetryWaitRestart,
)

// phase3RestartAppProcessTask executes app process restart.
var phase3RestartAppProcessTask = newPhase3EffectTask(
	phaseEffectIDBackendRestartAppProcess,
)

// phase3RestartViteProcessTask executes Vite process restart.
var phase3RestartViteProcessTask = newPhase3EffectTask(
	phaseEffectIDBackendRestartViteProcess,
)

// phase3RefreshFrameworkRouteTask executes framework route refresh.
var phase3RefreshFrameworkRouteTask = newPhase3EffectTask(
	phaseEffectIDBackendRefreshFrameworkRoute,
)

// phase3RefreshFrameworkTemplateTask executes framework template refresh.
var phase3RefreshFrameworkTemplateTask = newPhase3EffectTask(
	phaseEffectIDBackendRefreshFrameworkTemplate,
)

// phase3RefreshFrameworkPublicFileMapTask executes framework public-file-map refresh.
var phase3RefreshFrameworkPublicFileMapTask = newPhase3EffectTask(
	phaseEffectIDBackendRefreshFrameworkPublicFileMap,
)

// phase3AwaitBackendReadinessTask executes backend readiness wait.
var phase3AwaitBackendReadinessTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase3BatchInput,
	) (struct{}, error) {
		if input.backendGoals.queueRetryWaitRestart {
			if _, queuedRestartError := phase3QueueRetryWaitRestartTask.Run(
				taskContext,
				input,
			); queuedRestartError != nil {
				return struct{}{}, queuedRestartError
			}
			return struct{}{}, nil
		}
		if !input.backendGoals.awaitBackendReadiness {
			return struct{}{}, nil
		}

		var ignoredResult struct{}
		backendSettlingTasks := make([]tasks.BoundTask, 0, 6)
		if input.backendGoals.restartDevServerCycle {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3ApplyDevServerRestartTask.Bind(input, &ignoredResult),
			)
		}
		if input.backendGoals.restartAppProcess {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3RestartAppProcessTask.Bind(input, &ignoredResult),
			)
		}
		if input.backendGoals.restartViteProcess {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3RestartViteProcessTask.Bind(input, &ignoredResult),
			)
		}
		if input.backendGoals.refreshFrameworkRoute {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3RefreshFrameworkRouteTask.Bind(input, &ignoredResult),
			)
		}
		if input.backendGoals.refreshFrameworkTemplate {
			backendSettlingTasks = append(
				backendSettlingTasks,
				phase3RefreshFrameworkTemplateTask.Bind(input, &ignoredResult),
			)
		}
		if input.backendGoals.refreshFrameworkPublicFileMap {
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
		if _, effectError := input.batch.runPhaseEffect(
			taskContext,
			phaseEffectIDBackendAwaitReadiness,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// phase3PlanFrontendSettlingGoalsTask maps backend goals to frontend settling goals.
var phase3PlanFrontendSettlingGoalsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input phase3BatchInput,
	) (phase3FrontendSettlingGoals, error) {
		if _, awaitReadyError := phase3AwaitBackendReadinessTask.Run(
			taskContext,
			input,
		); awaitReadyError != nil {
			return phase3FrontendSettlingGoals{}, awaitReadyError
		}
		return input.backendGoals.derivePhase3FrontendSettlingGoals(), nil
	},
)

func (backendGoals phase2BackendSettlingGoals) derivePhase3FrontendSettlingGoals() phase3FrontendSettlingGoals {
	if backendGoals.queueRetryWaitRestart {
		return phase3FrontendSettlingGoals{
			terminalBrowserAction: frontendTerminalBrowserActionNone,
		}
	}

	terminalBrowserAction := backendGoals.requestedTerminalBrowserAction
	if backendGoals.restartViteProcess &&
		terminalBrowserAction == frontendTerminalBrowserActionNotifyVitePublicFileMapChanged {
		terminalBrowserAction = frontendTerminalBrowserActionNone
	}

	// Restarting Vite currently requires terminal hard reload so browser clients
	// reconnect against the active Vite endpoint.
	// Potential policy refinement: require this only when restart changes the
	// effective browser-facing Vite endpoint (for example, port change).
	if backendGoals.restartDevServerCycle ||
		backendGoals.restartAppProcess ||
		backendGoals.restartViteProcess ||
		backendGoals.refreshFrameworkRoute ||
		backendGoals.refreshFrameworkTemplate ||
		backendGoals.refreshFrameworkPublicFileMap {
		terminalBrowserAction = terminalBrowserAction.dominantWith(
			frontendTerminalBrowserActionHardReload,
		)
	}
	return phase3FrontendSettlingGoals{
		terminalBrowserAction: terminalBrowserAction,
	}
}
