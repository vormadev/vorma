package tooling

import (
	"strings"

	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/wave"
	"golang.org/x/sync/errgroup"
)

func (s *server) runSequentialHookStageForEligibleEvents(
	eventsWithHooks []eventWithHooks,
	watcher *Watcher,
	runHooksForEvent func(eventWithHooks, *Watcher) ([]wave.RefreshAction, error),
	hookExecutionFailureLogMessage string,
) []wave.RefreshAction {
	if runHooksForEvent == nil {
		return nil
	}

	descriptors := deriveHookStageExecutionDescriptors(eventsWithHooks)
	allStageActions := make([]wave.RefreshAction, 0)
	for _, descriptor := range descriptors {
		stageActions, err := runHooksForEvent(descriptor.eventWithHooks, watcher)
		if err != nil {
			s.log.Error(hookExecutionFailureLogMessage, "error", err)
		}
		allStageActions = append(allStageActions, stageActions...)
	}
	return allStageActions
}

func (s *server) fireNoWaitHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *Watcher,
) {
	descriptors := deriveHookStageExecutionDescriptors(eventsWithHooks)
	for _, descriptor := range descriptors {
		s.fireNoWaitHooks(descriptor.eventWithHooks, watcher)
	}
}

func (s *server) runPreHooksForEvents(
	eventsWithHooks []eventWithHooks,
	work *workSet,
	watcher *Watcher,
) []wave.RefreshAction {
	for _, eventWithHooksForPre := range eventsWithHooks {
		work.addImplicitWork(eventWithHooksForPre.classified)
	}
	return s.runSequentialHookStageForEligibleEvents(
		eventsWithHooks,
		watcher,
		s.runPreHooks,
		"Pre-hook execution failed",
	)
}

func (s *server) runConcurrentHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *Watcher,
) []wave.RefreshAction {
	descriptors := deriveHookStageExecutionDescriptors(eventsWithHooks)
	actionsByDescriptorIndex := make([][]wave.RefreshAction, len(descriptors))
	var concurrentHooksGroup errgroup.Group

	for descriptorIndex := range descriptors {
		descriptorForExecution := descriptors[descriptorIndex]
		descriptorIndexForResult := descriptorIndex
		concurrentHooksGroup.Go(func() error {
			concurrentActions, err := s.runConcurrentHooks(
				descriptorForExecution.eventWithHooks,
				watcher,
			)
			if err != nil {
				s.log.Error("Concurrent hook execution failed", "error", err)
			}
			actionsByDescriptorIndex[descriptorIndexForResult] = concurrentActions
			return nil
		})
	}

	_ = concurrentHooksGroup.Wait()

	allConcurrentActions := make([]wave.RefreshAction, 0)
	for _, concurrentActionsForEvent := range actionsByDescriptorIndex {
		allConcurrentActions = append(allConcurrentActions, concurrentActionsForEvent...)
	}

	return allConcurrentActions
}

func (s *server) runPostHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *Watcher,
) []wave.RefreshAction {
	return s.runSequentialHookStageForEligibleEvents(
		eventsWithHooks,
		watcher,
		s.runPostHooks,
		"Post-hook execution failed",
	)
}

func (s *server) fireNoWaitHooks(ewh eventWithHooks, watcher *Watcher) {
	plans := deriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hookStageTypeConcurrentNoWait,
		s.resolveHookCommand,
	)
	for _, plan := range plans {
		if plan.callback != nil {
			go func(cb func(*wave.HookContext) (*wave.RefreshAction, error), hookContext *wave.HookContext) {
				if _, err := cb(hookContext); err != nil {
					s.log.Warn("concurrent-no-wait callback failed", "error", err)
				}
			}(plan.callback, ewh.hookCtx)
		}
		if strings.TrimSpace(plan.command) != "" {
			go func(command string) {
				if err := executil.RunShell(command); err != nil {
					s.log.Warn("concurrent-no-wait hook failed", "cmd", command, "error", err)
				}
			}(plan.command)
		}
	}
}

func (s *server) runPreHooks(ewh eventWithHooks, watcher *Watcher) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	plans := deriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hookStageTypePre,
		s.resolveHookCommand,
	)
	for _, plan := range plans {
		action, err := s.executeHookExecutionPlan(plan, ewh.hookCtx)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, err
		}
	}

	return actions, nil
}

func (s *server) runConcurrentHooks(ewh eventWithHooks, watcher *Watcher) ([]wave.RefreshAction, error) {
	plans := deriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hookStageTypeConcurrent,
		s.resolveHookCommand,
	)
	if len(plans) == 0 {
		return nil, nil
	}

	actionsByHookIndex := make([]*wave.RefreshAction, len(plans))
	var executionGroup errgroup.Group

	for hookIndex := range plans {
		planForExecution := plans[hookIndex]
		hookIndexForResult := hookIndex
		executionGroup.Go(func() error {
			action, err := s.executeHookExecutionPlan(planForExecution, ewh.hookCtx)
			actionsByHookIndex[hookIndexForResult] = action
			return err
		})
	}

	err := executionGroup.Wait()
	actions := make([]wave.RefreshAction, 0, len(actionsByHookIndex))
	for _, action := range actionsByHookIndex {
		if action != nil {
			actions = append(actions, *action)
		}
	}

	return actions, err
}

func (s *server) runPostHooks(ewh eventWithHooks, watcher *Watcher) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	plans := deriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hookStageTypePost,
		s.resolveHookCommand,
	)
	for _, plan := range plans {
		action, err := s.executeHookExecutionPlan(plan, ewh.hookCtx)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, err
		}
	}

	return actions, nil
}

func (s *server) executeHook(
	hook wave.OnChangeHook,
	hookContext *wave.HookContext,
) (*wave.RefreshAction, error) {
	return s.executeHookExecutionPlan(
		deriveHookExecutionPlanFromHook(hook, s.resolveHookCommand),
		hookContext,
	)
}

func (s *server) executeHookExecutionPlan(
	plan hookExecutionPlan,
	hookContext *wave.HookContext,
) (*wave.RefreshAction, error) {
	var action *wave.RefreshAction
	if plan.callback != nil {
		callbackAction, err := plan.callback(hookContext)
		if err != nil {
			return nil, err
		}
		action = callbackAction
	}

	if strings.TrimSpace(plan.command) != "" {
		if err := executil.RunShell(plan.command); err != nil {
			return action, err
		}
	}

	return action, nil
}
