package wavebuild

import (
	"errors"

	"github.com/vormadev/vorma/kit/tasks"
)

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p3_Effects struct {
	applyDevServerRestart    *tasks.Task[p3_BatchInput, struct{}]
	queueRetryWaitRestart    *tasks.Task[p3_BatchInput, struct{}]
	restartAppProcess        *tasks.Task[p3_BatchInput, struct{}]
	restartViteProcess       *tasks.Task[p3_BatchInput, struct{}]
	executeFWMutationEffects *tasks.Task[p3_BatchInput, struct{}]
	planP4_RequestedEffects  *tasks.Task[p3_BatchInput, p3_Output]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p3_EffectsDef = p3_Effects{
	applyDevServerRestart:    p3_ApplyDevServerRestartTask,
	queueRetryWaitRestart:    p3_QueueRetryWaitRestartTask,
	restartAppProcess:        p3_RestartAppProcessTask,
	restartViteProcess:       p3_RestartViteProcessTask,
	executeFWMutationEffects: p3_ExecuteFWMutationEffectsTask,
	planP4_RequestedEffects:  p3_PlanP4_RequestedEffectsTask,
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

var p3_ExecuteFWMutationEffectsTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p3_BatchInput,
	) (struct{}, error) {
		fwRequestedEffects := fwRequestedEffectsFromPointer(
			input.p2_RequestedEffects.fwRequestedEffects,
		)
		if !fwRequestedEffects.hasBackendMutationEffects() {
			return struct{}{}, nil
		}
		if isTestEnv() {
			for _, effectKey := range fwRequestedEffects.backendMutationEffectKeys {
				recordTestEffect(
					tasksCtx,
					_LABEL_P3_EXECUTE_FW_MUTATION_EFFECT+
						"["+
						string(effectKey)+
						"]",
				)
			}
			return struct{}{}, nil
		}
		registrations := input.p2_RequestedEffects.fwExecutionRegistrations
		if registrations == nil {
			return struct{}{}, errors.New(
				"wavebuild: fw execution registrations are required for backend mutation effects",
			)
		}
		if len(registrations.backendMutationEffectsByKey) == 0 {
			return struct{}{}, errors.New(
				"wavebuild: backend mutation fw effect registry is empty",
			)
		}
		var ignoredResult struct{}
		boundFWMutationTasks := make(
			[]tasks.BoundTask,
			0,
			len(fwRequestedEffects.backendMutationEffectKeys),
		)
		for _, effectKey := range fwRequestedEffects.backendMutationEffectKeys {
			fwMutationTask, hasFWMutationTask := registrations.backendMutationEffectsByKey[effectKey]
			if !hasFWMutationTask || fwMutationTask == nil {
				return struct{}{}, errors.New(
					"wavebuild: backend mutation fw effect task is not registered for key " +
						string(
							effectKey,
						),
				)
			}
			boundFWMutationTasks = append(
				boundFWMutationTasks,
				fwMutationTask.Bind(input, &ignoredResult),
			)
		}
		if runParallelError := tasksCtx.RunParallel(
			boundFWMutationTasks...,
		); runParallelError != nil {
			return struct{}{}, runParallelError
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
		backendMutationTasks := make([]tasks.BoundTask, 0, 4)
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
		if fwRequestedEffectsFromPointer(
			input.p2_RequestedEffects.fwRequestedEffects,
		).hasBackendMutationEffects() {
			backendMutationTasks = append(
				backendMutationTasks,
				p3_ExecuteFWMutationEffectsTask.Bind(
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
