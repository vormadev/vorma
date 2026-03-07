package wavebuild

import (
	"errors"

	"github.com/vormadev/vorma/kit/tasks"
)

/////////////////////////////////////////////////////////////////////
/////// Effect Catalog
/////////////////////////////////////////////////////////////////////

type p4_Effects struct {
	awaitBackendReadiness   *tasks.Task[p4_BatchInput, struct{}]
	executeFWNotifications  *tasks.Task[p4_BatchInput, p4_Output]
	planP5_RequestedEffects *tasks.Task[p4_BatchInput, p4_Output]
}

/////////////////////////////////////////////////////////////////////
/////// Effect Definitions
/////////////////////////////////////////////////////////////////////

var p4_EffectsDef = p4_Effects{
	awaitBackendReadiness:   p4_AwaitBackendReadinessTask,
	executeFWNotifications:  p4_ExecuteFWNotificationsTask,
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

var p4_ExecuteFWNotificationsTask = tasks.NewTask(
	func(
		tasksCtx *tasks.Ctx,
		input p4_BatchInput,
	) (p4_Output, error) {
		fwRequestedEffects := fwRequestedEffectsFromPointer(
			input.p4_RequestedEffects.fwRequestedEffects,
		)
		if !fwRequestedEffects.hasBackendConvergenceNotifications() {
			return p4_Output{}, nil
		}
		if isTestEnv() {
			for _, notification := range fwRequestedEffects.backendConvergenceNotificationQueue {
				normalizedNotification := notification.normalize()
				recordTestEffect(
					tasksCtx,
					_LABEL_P4_EXECUTE_FW_NOTIFICATION+
						"["+
						string(normalizedNotification.destinationKey)+
						"]",
				)
			}
			return p4_Output{}, nil
		}
		registrations := input.p4_RequestedEffects.fwExecutionRegistrations
		if registrations == nil {
			return p4_Output{}, errors.New(
				"wavebuild: fw execution registrations are required for backend convergence notifications",
			)
		}
		if len(
			registrations.backendConvergenceNotificationsByDestination,
		) == 0 {
			return p4_Output{}, errors.New(
				"wavebuild: backend convergence notification registry is empty",
			)
		}
		for _, notification := range fwRequestedEffects.backendConvergenceNotificationQueue {
			normalizedNotification := notification.normalize()
			if normalizedNotification.destinationKey == "" {
				continue
			}
			notificationTask, hasNotificationTask := registrations.backendConvergenceNotificationsByDestination[normalizedNotification.destinationKey]
			if !hasNotificationTask || notificationTask == nil {
				return p4_Output{}, errors.New(
					"wavebuild: backend convergence notification task is not registered for destination " +
						string(
							normalizedNotification.destinationKey,
						),
				)
			}
			notificationForTask := normalizedNotification
			_, notificationTaskError := notificationTask.Run(
				tasksCtx,
				p4_FWNotificationTaskInput{
					batch:        input.batch,
					notification: &notificationForTask,
				},
			)
			if notificationTaskError == nil {
				continue
			}
			if normalizedNotification.failurePolicy ==
				FrameworkNotificationFailurePolicyRestartBackendWithoutGoCompile {
				return p4_Output{
					requiresBackendRestartWithoutGoCompile: true,
					skipFrontendSettling:                   true,
				}, nil
			}
			return p4_Output{}, notificationTaskError
		}
		return p4_Output{}, nil
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
		fwNotificationExecutionOutput, notificationExecutionError := p4_ExecuteFWNotificationsTask.Run(
			tasksCtx,
			input,
		)
		if notificationExecutionError != nil {
			return p4_Output{}, notificationExecutionError
		}
		return p4_Output{
			p5_RequestedEffects:                    input.p4_RequestedEffects.p5_RequestedEffects,
			requiresBackendRestartWithoutGoCompile: fwNotificationExecutionOutput.requiresBackendRestartWithoutGoCompile,
			skipFrontendSettling:                   fwNotificationExecutionOutput.skipFrontendSettling,
		}, nil
	},
)
