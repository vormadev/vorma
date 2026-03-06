package wavebuild

import "github.com/vormadev/vorma/kit/tasks"

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p4_Effects struct {
	awaitBackendReadiness   *tasks.Task[p4_BatchInput, struct{}]
	planP5_RequestedEffects *tasks.Task[p4_BatchInput, p4_Output]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p4_EffectsDef = p4_Effects{
	awaitBackendReadiness:   p4_AwaitBackendReadinessTask,
	planP5_RequestedEffects: p4_PlanP5_RequestedEffectsTask,
}

/////////////////////////////////////////////////////////////////////
/////// Effect Tasks
/////////////////////////////////////////////////////////////////////

var p4_AwaitBackendReadinessTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p4_BatchInput,
	) (struct{}, error) {
		if !input.p4_RequestedEffects.awaitBackendReadiness {
			return struct{}{}, nil
		}
		if recordTestEffect(tasksCtx, _LABEL_P4_AWAIT_BACKEND_READINESS) {
			return struct{}{}, nil
		}
		return struct{}{}, nil
	},
)

var p4_PlanP5_RequestedEffectsTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p4_BatchInput,
	) (p4_Output, error) {
		if _, awaitBackendReadinessError := p4_AwaitBackendReadinessTask.Run(
			tasksCtx,
			input,
		); awaitBackendReadinessError != nil {
			return p4_Output{}, awaitBackendReadinessError
		}
		return p4_Output{p5_RequestedEffects: input.p4_RequestedEffects.p5_RequestedEffects}, nil
	},
)
