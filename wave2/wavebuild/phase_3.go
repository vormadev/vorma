package wavebuild

import (
	"github.com/vormadev/vorma/kit/tasks"
)

// Phase3BatchInput is the backend-settling input produced by phase 2.
type Phase3BatchInput struct {
	Batch        PhaseBatchInput
	BackendGoals Phase2BackendSettlingGoals
}

// Phase3FrontendSettlingGoals are frontend-settling goals produced by phase 3.
type Phase3FrontendSettlingGoals struct {
	PerformCSSHotReload     bool
	PerformInvalidateAssets bool
	PerformRevalidate       bool
	PerformHardReload       bool
}

func newPhase3EffectTask(
	effectID PhaseEffectID,
) *tasks.Task[Phase3BatchInput, struct{}] {
	return tasks.NewTask(
		func(
			taskContext *tasks.Ctx,
			input Phase3BatchInput,
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

// Phase3ApplyDevServerRestartTask executes dev-server restart.
var Phase3ApplyDevServerRestartTask = newPhase3EffectTask(
	PhaseEffectIDBackendApplyDevServerRestart,
)

// Phase3QueueRetryWaitRestartTask executes queued retry-wait restart handling.
var Phase3QueueRetryWaitRestartTask = newPhase3EffectTask(
	PhaseEffectIDBackendQueueRetryWaitRestart,
)

// Phase3RestartAppProcessTask executes app process restart.
var Phase3RestartAppProcessTask = newPhase3EffectTask(
	PhaseEffectIDBackendRestartAppProcess,
)

// Phase3RestartViteProcessTask executes Vite process restart.
var Phase3RestartViteProcessTask = newPhase3EffectTask(
	PhaseEffectIDBackendRestartViteProcess,
)

// Phase3RefreshFrameworkRouteTask executes framework route refresh.
var Phase3RefreshFrameworkRouteTask = newPhase3EffectTask(
	PhaseEffectIDBackendRefreshFrameworkRoute,
)

// Phase3RefreshFrameworkTemplateTask executes framework template refresh.
var Phase3RefreshFrameworkTemplateTask = newPhase3EffectTask(
	PhaseEffectIDBackendRefreshFrameworkTemplate,
)

// Phase3RefreshFrameworkPublicFileMapTask executes framework public-file-map refresh.
var Phase3RefreshFrameworkPublicFileMapTask = newPhase3EffectTask(
	PhaseEffectIDBackendRefreshFrameworkPublicFileMap,
)

// Phase3AwaitBackendReadinessTask executes backend readiness wait.
var Phase3AwaitBackendReadinessTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		if input.BackendGoals.QueueRetryWaitRestart {
			if _, queuedRestartError := Phase3QueueRetryWaitRestartTask.Run(
				taskContext,
				input,
			); queuedRestartError != nil {
				return struct{}{}, queuedRestartError
			}
			return struct{}{}, nil
		}
		if !input.BackendGoals.AwaitBackendReadiness {
			return struct{}{}, nil
		}

		var ignoredResult struct{}
		backendSettlingTasks := make([]tasks.BoundTask, 0, 6)
		if input.BackendGoals.RestartDevServerCycle {
			backendSettlingTasks = append(
				backendSettlingTasks,
				Phase3ApplyDevServerRestartTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.RestartAppProcess {
			backendSettlingTasks = append(
				backendSettlingTasks,
				Phase3RestartAppProcessTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.RestartViteProcess {
			backendSettlingTasks = append(
				backendSettlingTasks,
				Phase3RestartViteProcessTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.RefreshFrameworkRoute {
			backendSettlingTasks = append(
				backendSettlingTasks,
				Phase3RefreshFrameworkRouteTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.RefreshFrameworkTemplate {
			backendSettlingTasks = append(
				backendSettlingTasks,
				Phase3RefreshFrameworkTemplateTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.RefreshFrameworkPublicFileMap {
			backendSettlingTasks = append(
				backendSettlingTasks,
				Phase3RefreshFrameworkPublicFileMapTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if runParallelError := taskContext.RunParallel(backendSettlingTasks...); runParallelError != nil {
			return struct{}{}, runParallelError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBackendAwaitReadiness,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase3PlanFrontendSettlingGoalsTask maps backend goals to frontend settling goals.
var Phase3PlanFrontendSettlingGoalsTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (Phase3FrontendSettlingGoals, error) {
		if _, awaitReadyError := Phase3AwaitBackendReadinessTask.Run(
			taskContext,
			input,
		); awaitReadyError != nil {
			return Phase3FrontendSettlingGoals{}, awaitReadyError
		}
		return reducePhase3FrontendSettlingGoals(input.BackendGoals), nil
	},
)

func reducePhase3FrontendSettlingGoals(
	backendGoals Phase2BackendSettlingGoals,
) Phase3FrontendSettlingGoals {
	if backendGoals.QueueRetryWaitRestart {
		return Phase3FrontendSettlingGoals{}
	}

	phase3FrontendSettlingGoals := Phase3FrontendSettlingGoals{
		PerformCSSHotReload:     backendGoals.RequestBrowserCSSHotReload,
		PerformInvalidateAssets: backendGoals.RequestBrowserInvalidateAssets,
		PerformRevalidate:       backendGoals.RequestBrowserRevalidate,
		PerformHardReload:       backendGoals.RequestBrowserHardReload,
	}

	if backendGoals.RestartDevServerCycle ||
		backendGoals.RestartAppProcess ||
		backendGoals.RestartViteProcess ||
		backendGoals.RefreshFrameworkRoute ||
		backendGoals.RefreshFrameworkTemplate ||
		backendGoals.RefreshFrameworkPublicFileMap {
		phase3FrontendSettlingGoals.PerformHardReload = true
	}
	return phase3FrontendSettlingGoals
}

// RunPhase3TaskGraph executes phase 3 and returns phase 4 goals.
func RunPhase3TaskGraph(
	taskContext *tasks.Ctx,
	input Phase3BatchInput,
) (Phase3FrontendSettlingGoals, error) {
	return Phase3PlanFrontendSettlingGoalsTask.Run(taskContext, input)
}
