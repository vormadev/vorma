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
	WaitForApp              bool
	WaitForVite             bool
	NoBrowserAction         bool
}

// Phase3EnsureBackendSettlingEnvelopeTask captures backend-settling envelope.
var Phase3EnsureBackendSettlingEnvelopeTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (Phase3BatchInput, error) {
		return input, nil
	},
)

// Phase3EnsureLifecycleCoordinatorReadyTask validates lifecycle coordinator readiness.
var Phase3EnsureLifecycleCoordinatorReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, envelopeError := Phase3EnsureBackendSettlingEnvelopeTask.Run(
			taskContext,
			input,
		)
		if envelopeError != nil {
			return struct{}{}, envelopeError
		}
		return struct{}{}, nil
	},
)

// Phase3EnsureAppSupervisorReadyTask validates app supervisor readiness.
var Phase3EnsureAppSupervisorReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, lifecycleError := Phase3EnsureLifecycleCoordinatorReadyTask.Run(
			taskContext,
			input,
		)
		if lifecycleError != nil {
			return struct{}{}, lifecycleError
		}
		return struct{}{}, nil
	},
)

// Phase3EnsureViteSupervisorReadyTask validates Vite supervisor readiness.
var Phase3EnsureViteSupervisorReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, lifecycleError := Phase3EnsureLifecycleCoordinatorReadyTask.Run(
			taskContext,
			input,
		)
		if lifecycleError != nil {
			return struct{}{}, lifecycleError
		}
		return struct{}{}, nil
	},
)

// Phase3EnsureFrameworkBridgeReadyTask validates framework refresh bridge readiness.
var Phase3EnsureFrameworkBridgeReadyTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, lifecycleError := Phase3EnsureLifecycleCoordinatorReadyTask.Run(
			taskContext,
			input,
		)
		if lifecycleError != nil {
			return struct{}{}, lifecycleError
		}
		return struct{}{}, nil
	},
)

// Phase3ApplyDevServerRestartTask executes dev-server restart.
var Phase3ApplyDevServerRestartTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, lifecycleError := Phase3EnsureLifecycleCoordinatorReadyTask.Run(
			taskContext,
			input,
		)
		if lifecycleError != nil {
			return struct{}{}, lifecycleError
		}
		_, appSupervisorError := Phase3EnsureAppSupervisorReadyTask.Run(
			taskContext,
			input,
		)
		if appSupervisorError != nil {
			return struct{}{}, appSupervisorError
		}
		_, viteSupervisorError := Phase3EnsureViteSupervisorReadyTask.Run(
			taskContext,
			input,
		)
		if viteSupervisorError != nil {
			return struct{}{}, viteSupervisorError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBackendApplyDevServerRestart,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase3QueueRetryWaitRestartTask executes queued retry-wait restart handling.
var Phase3QueueRetryWaitRestartTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, lifecycleError := Phase3EnsureLifecycleCoordinatorReadyTask.Run(
			taskContext,
			input,
		)
		if lifecycleError != nil {
			return struct{}{}, lifecycleError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBackendQueueRetryWaitRestart,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase3RestartAppProcessTask executes app process restart.
var Phase3RestartAppProcessTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, appSupervisorError := Phase3EnsureAppSupervisorReadyTask.Run(
			taskContext,
			input,
		)
		if appSupervisorError != nil {
			return struct{}{}, appSupervisorError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBackendRestartAppProcess,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase3RestartViteProcessTask executes Vite process restart.
var Phase3RestartViteProcessTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, viteSupervisorError := Phase3EnsureViteSupervisorReadyTask.Run(
			taskContext,
			input,
		)
		if viteSupervisorError != nil {
			return struct{}{}, viteSupervisorError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBackendRestartViteProcess,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase3RefreshFrameworkRouteTask executes framework route refresh.
var Phase3RefreshFrameworkRouteTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, frameworkBridgeError := Phase3EnsureFrameworkBridgeReadyTask.Run(
			taskContext,
			input,
		)
		if frameworkBridgeError != nil {
			return struct{}{}, frameworkBridgeError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBackendRefreshFrameworkRoute,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase3RefreshFrameworkTemplateTask executes framework template refresh.
var Phase3RefreshFrameworkTemplateTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, frameworkBridgeError := Phase3EnsureFrameworkBridgeReadyTask.Run(
			taskContext,
			input,
		)
		if frameworkBridgeError != nil {
			return struct{}{}, frameworkBridgeError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBackendRefreshFrameworkTemplate,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase3RefreshFrameworkPublicFileMapTask executes framework public-file-map refresh.
var Phase3RefreshFrameworkPublicFileMapTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		_, frameworkBridgeError := Phase3EnsureFrameworkBridgeReadyTask.Run(
			taskContext,
			input,
		)
		if frameworkBridgeError != nil {
			return struct{}{}, frameworkBridgeError
		}
		if effectError := executePhaseEffect(
			taskContext,
			input.Batch,
			PhaseEffectIDBackendRefreshFrameworkPublicFileMap,
		); effectError != nil {
			return struct{}{}, effectError
		}
		return struct{}{}, nil
	},
)

// Phase3AwaitBackendReadinessTask executes backend readiness wait.
var Phase3AwaitBackendReadinessTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		var ignoredResult struct{}
		backendSettlingTasks := make([]tasks.BoundTask, 0, 7)
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
		_, awaitReadyError := Phase3AwaitBackendReadinessTask.Run(
			taskContext,
			input,
		)
		if awaitReadyError != nil {
			return Phase3FrontendSettlingGoals{}, awaitReadyError
		}
		return reducePhase3FrontendSettlingGoals(input.BackendGoals), nil
	},
)

func reducePhase3FrontendSettlingGoals(
	backendGoals Phase2BackendSettlingGoals,
) Phase3FrontendSettlingGoals {
	if backendGoals.QueueRetryWaitRestart {
		return Phase3FrontendSettlingGoals{
			NoBrowserAction: true,
		}
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
	if phase3FrontendSettlingGoals.PerformHardReload {
		phase3FrontendSettlingGoals.WaitForApp = true
		phase3FrontendSettlingGoals.WaitForVite = true
	}
	phase3FrontendSettlingGoals.NoBrowserAction =
		!phase3FrontendSettlingGoals.PerformCSSHotReload &&
			!phase3FrontendSettlingGoals.PerformInvalidateAssets &&
			!phase3FrontendSettlingGoals.PerformRevalidate &&
			!phase3FrontendSettlingGoals.PerformHardReload
	return phase3FrontendSettlingGoals
}

// Phase3RootApplyDevServerRestartTask is a terminal dev-server restart root.
var Phase3RootApplyDevServerRestartTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		return Phase3ApplyDevServerRestartTask.Run(taskContext, input)
	},
)

// Phase3RootQueueRetryWaitRestartTask is a terminal queued-retry root.
var Phase3RootQueueRetryWaitRestartTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		return Phase3QueueRetryWaitRestartTask.Run(taskContext, input)
	},
)

// Phase3RootRestartAppProcessTask is a terminal app-restart root.
var Phase3RootRestartAppProcessTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		return Phase3RestartAppProcessTask.Run(taskContext, input)
	},
)

// Phase3RootRestartViteProcessTask is a terminal Vite restart root.
var Phase3RootRestartViteProcessTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		return Phase3RestartViteProcessTask.Run(taskContext, input)
	},
)

// Phase3RootRefreshFrameworkRouteTask is a terminal framework-route refresh root.
var Phase3RootRefreshFrameworkRouteTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		return Phase3RefreshFrameworkRouteTask.Run(taskContext, input)
	},
)

// Phase3RootRefreshFrameworkTemplateTask is a terminal framework-template refresh root.
var Phase3RootRefreshFrameworkTemplateTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		return Phase3RefreshFrameworkTemplateTask.Run(taskContext, input)
	},
)

// Phase3RootRefreshFrameworkPublicFileMapTask is a terminal framework file-map refresh root.
var Phase3RootRefreshFrameworkPublicFileMapTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		return Phase3RefreshFrameworkPublicFileMapTask.Run(taskContext, input)
	},
)

// Phase3RootAwaitBackendReadinessTask is a terminal backend-readiness root.
var Phase3RootAwaitBackendReadinessTask = tasks.NewTask(
	func(
		taskContext *tasks.Ctx,
		input Phase3BatchInput,
	) (struct{}, error) {
		return Phase3AwaitBackendReadinessTask.Run(taskContext, input)
	},
)

// RunPhase3TaskGraph executes phase-3 task roots then plans phase 4 goals.
func RunPhase3TaskGraph(
	taskContext *tasks.Ctx,
	input Phase3BatchInput,
) (Phase3FrontendSettlingGoals, error) {
	var ignoredResult struct{}
	terminalBackendRoots := make([]tasks.BoundTask, 0, 8)

	if input.BackendGoals.QueueRetryWaitRestart {
		terminalBackendRoots = append(
			terminalBackendRoots,
			Phase3RootQueueRetryWaitRestartTask.Bind(input, &ignoredResult),
		)
	} else {
		if input.BackendGoals.RestartDevServerCycle {
			terminalBackendRoots = append(
				terminalBackendRoots,
				Phase3RootApplyDevServerRestartTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.RestartAppProcess {
			terminalBackendRoots = append(
				terminalBackendRoots,
				Phase3RootRestartAppProcessTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.RestartViteProcess {
			terminalBackendRoots = append(
				terminalBackendRoots,
				Phase3RootRestartViteProcessTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.RefreshFrameworkRoute {
			terminalBackendRoots = append(
				terminalBackendRoots,
				Phase3RootRefreshFrameworkRouteTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.RefreshFrameworkTemplate {
			terminalBackendRoots = append(
				terminalBackendRoots,
				Phase3RootRefreshFrameworkTemplateTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.RefreshFrameworkPublicFileMap {
			terminalBackendRoots = append(
				terminalBackendRoots,
				Phase3RootRefreshFrameworkPublicFileMapTask.Bind(input, &ignoredResult),
			)
		}
		if input.BackendGoals.AwaitBackendReadiness {
			terminalBackendRoots = append(
				terminalBackendRoots,
				Phase3RootAwaitBackendReadinessTask.Bind(input, &ignoredResult),
			)
		}
	}
	if runParallelError := taskContext.RunParallel(terminalBackendRoots...); runParallelError != nil {
		return Phase3FrontendSettlingGoals{}, runParallelError
	}
	return Phase3PlanFrontendSettlingGoalsTask.Run(taskContext, input)
}
