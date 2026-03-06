package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p3_Effects struct {
	applyDevServerRestart         *tasks.Task[p3_BatchInput, struct{}]
	queueRetryWaitRestart         *tasks.Task[p3_BatchInput, struct{}]
	restartAppProcess             *tasks.Task[p3_BatchInput, struct{}]
	restartViteProcess            *tasks.Task[p3_BatchInput, struct{}]
	refreshFrameworkRoute         *tasks.Task[p3_BatchInput, struct{}]
	refreshFrameworkTemplate      *tasks.Task[p3_BatchInput, struct{}]
	refreshFrameworkPublicFileMap *tasks.Task[p3_BatchInput, struct{}]
	planP4_RequestedEffects       *tasks.Task[p3_BatchInput, p3_Output]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p3_EffectsDef = p3_Effects{
	applyDevServerRestart:         p3_ApplyDevServerRestartTask,
	queueRetryWaitRestart:         p3_QueueRetryWaitRestartTask,
	restartAppProcess:             p3_RestartAppProcessTask,
	restartViteProcess:            p3_RestartViteProcessTask,
	refreshFrameworkRoute:         p3_RefreshFrameworkRouteTask,
	refreshFrameworkTemplate:      p3_RefreshFrameworkTemplateTask,
	refreshFrameworkPublicFileMap: p3_RefreshFrameworkPublicFileMapTask,
	planP4_RequestedEffects:       p3_PlanP4_RequestedEffectsTask,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var p3_ApplyDevServerRestartTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p3_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P3_APPLY_DEV_SERVER_RESTART) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_QueueRetryWaitRestartTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p3_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P3_QUEUE_RETRY_WAIT_RESTART) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_RestartAppProcessTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p3_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P3_RESTART_APP_PROCESS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_RestartViteProcessTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p3_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P3_RESTART_VITE_PROCESS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_RefreshFrameworkRouteTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p3_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P3_REFRESH_FRAMEWORK_ROUTE) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_RefreshFrameworkTemplateTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p3_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(tasksCtx, _LABEL_P3_REFRESH_FRAMEWORK_TEMPLATE) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_RefreshFrameworkPublicFileMapTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p3_BatchInput,
	) (struct{}, error) {
		if recordTestEffect(
			tasksCtx,
			_LABEL_P3_REFRESH_FRAMEWORK_PUBLIC_FILEMAP,
		) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p3_PlanP4_RequestedEffectsTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p3_BatchInput,
	) (p3_Output, error) {
		if input.p2_RequestedEffects.queueRetryWaitRestart {
			if _, queuedRestartError := p3_QueueRetryWaitRestartTask.Run(
				tasksCtx,
				input,
			); queuedRestartError != nil {
				return p3_Output{}, queuedRestartError
			}
			return p3_Output{
				p4_RequestedEffects: input.p2_RequestedEffects.deriveP4_RequestedEffects(),
			}, nil
		}

		var ignoredResult struct{}
		backendMutationTasks := make([]tasks.BoundTask, 0, 6)
		if input.p2_RequestedEffects.restartDevServerCycle {
			backendMutationTasks = append(
				backendMutationTasks,
				p3_ApplyDevServerRestartTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.p2_RequestedEffects.restartAppProcess {
			backendMutationTasks = append(
				backendMutationTasks,
				p3_RestartAppProcessTask.Bind(input, &ignoredResult),
			)
		}
		if input.p2_RequestedEffects.restartViteProcess {
			backendMutationTasks = append(
				backendMutationTasks,
				p3_RestartViteProcessTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.p2_RequestedEffects.refreshFrameworkRoute {
			backendMutationTasks = append(
				backendMutationTasks,
				p3_RefreshFrameworkRouteTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.p2_RequestedEffects.refreshFrameworkTemplate {
			backendMutationTasks = append(
				backendMutationTasks,
				p3_RefreshFrameworkTemplateTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if input.p2_RequestedEffects.refreshFrameworkPublicFileMap {
			backendMutationTasks = append(
				backendMutationTasks,
				p3_RefreshFrameworkPublicFileMapTask.Bind(
					input,
					&ignoredResult,
				),
			)
		}
		if len(backendMutationTasks) > 0 {
			if runParallelError := tasksCtx.RunParallel(backendMutationTasks...); runParallelError != nil {
				return p3_Output{}, runParallelError
			}
		}
		return p3_Output{
			p4_RequestedEffects: input.p2_RequestedEffects.deriveP4_RequestedEffects(),
		}, nil
	},
)
