package tooling

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vormadev/vorma/kit/executil"
	"github.com/vormadev/vorma/wave"
	"golang.org/x/sync/errgroup"
)

const maxConcurrentNoWaitHookExecutions = 16

func formatHookCallbackPanicError(panicValue any) error {
	return fmt.Errorf("hook callback panicked: %v", panicValue)
}

func deriveHookStageLabel(
	stageType hookStageType,
) string {
	switch stageType {
	case hookStageTypePre:
		return "pre"
	case hookStageTypeConcurrent:
		return "concurrent"
	case hookStageTypePost:
		return "post"
	case hookStageTypeConcurrentNoWait:
		return "concurrent-no-wait"
	default:
		return "unknown"
	}
}

func deriveHookExecutionContext(
	parentHookExecutionContext context.Context,
) context.Context {
	if parentHookExecutionContext == nil {
		return context.Background()
	}
	return parentHookExecutionContext
}

func cloneHookContextForExecution(
	hookContext *wave.HookContext,
	hookExecutionContext context.Context,
) *wave.HookContext {
	if hookContext == nil {
		return &wave.HookContext{
			ExecutionContext: hookExecutionContext,
		}
	}

	clonedHookContext := *hookContext
	clonedHookContext.ChangedFilePaths = append(
		[]string(nil),
		hookContext.ChangedFilePaths...)
	clonedHookContext.ExecutionContext = hookExecutionContext
	return &clonedHookContext
}

func wrapHookExecutionErrorWithStageAndPath(
	stageType hookStageType,
	changedFilePath string,
	hookExecutionError error,
) error {
	if hookExecutionError == nil {
		return nil
	}

	if strings.TrimSpace(changedFilePath) == "" {
		return fmt.Errorf(
			"%s hook failed: %w",
			deriveHookStageLabel(stageType),
			hookExecutionError,
		)
	}

	return fmt.Errorf(
		"%s hook failed for %s: %w",
		deriveHookStageLabel(stageType),
		changedFilePath,
		hookExecutionError,
	)
}

func joinHookExecutionErrorsInOrder(
	hookExecutionErrorsByHookIndex []error,
) error {
	if len(hookExecutionErrorsByHookIndex) == 0 {
		return nil
	}

	orderedHookExecutionErrors := make(
		[]error,
		0,
		len(hookExecutionErrorsByHookIndex),
	)
	for _, hookExecutionError := range hookExecutionErrorsByHookIndex {
		if hookExecutionError == nil {
			continue
		}
		orderedHookExecutionErrors = append(
			orderedHookExecutionErrors,
			hookExecutionError,
		)
	}
	if len(orderedHookExecutionErrors) == 0 {
		return nil
	}
	return errors.Join(orderedHookExecutionErrors...)
}

func executeHookCallbackSafely(
	callback func(*wave.HookContext) (*wave.RefreshAction, error),
	hookContext *wave.HookContext,
) (
	callbackAction *wave.RefreshAction,
	callbackExecutionError error,
) {
	if callback == nil {
		return nil, nil
	}

	defer func() {
		if panicValue := recover(); panicValue != nil {
			callbackExecutionError = formatHookCallbackPanicError(panicValue)
			callbackAction = nil
		}
	}()

	return callback(hookContext)
}

func (s *server) runNoWaitHookWithConcurrencyLimit(
	runNoWaitHook func(),
) {
	if s == nil || runNoWaitHook == nil {
		return
	}

	concurrentNoWaitHookExecutionLimiter := s.ensureConcurrentNoWaitHookExecutionLimiter()
	if concurrentNoWaitHookExecutionLimiter == nil {
		return
	}
	lifecycleExecutionContext := s.getOrCreateConcurrentNoWaitHookLifecycleContext()

	s.launchRunCycleScopedAsyncWorkOrDetached(func(
		context.Context,
	) {
		if lifecycleExecutionContext == nil {
			lifecycleExecutionContext = context.Background()
		}

		select {
		case concurrentNoWaitHookExecutionLimiter <- struct{}{}:
		case <-lifecycleExecutionContext.Done():
			return
		}
		defer func() {
			<-concurrentNoWaitHookExecutionLimiter
		}()

		if !shouldContinueConcurrentHookExecution(lifecycleExecutionContext) {
			return
		}
		runNoWaitHook()
	})
}

func (s *server) ensureConcurrentNoWaitHookExecutionLimiter() chan struct{} {
	if s == nil {
		return nil
	}

	s.concurrentNoWaitHookExecutionLimiterInitOnce.Do(func() {
		if s.concurrentNoWaitHookExecutionLimiter == nil {
			s.concurrentNoWaitHookExecutionLimiter = make(
				chan struct{},
				maxConcurrentNoWaitHookExecutions,
			)
		}
	})
	return s.concurrentNoWaitHookExecutionLimiter
}

func (s *server) getOrCreateConcurrentNoWaitHookLifecycleContext() context.Context {
	if s == nil {
		return context.Background()
	}

	currentRunCycleScope := s.getCurrentRunCycleScope()
	if currentRunCycleScope != nil {
		return currentRunCycleScope.executionContext
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.concurrentNoWaitHookLifecycleCtx == nil ||
		!shouldContinueConcurrentHookExecution(
			s.concurrentNoWaitHookLifecycleCtx,
		) {
		s.concurrentNoWaitHookLifecycleCtx, s.concurrentNoWaitHookLifecycleCancel = context.WithCancel(
			context.Background(),
		)
	}

	return s.concurrentNoWaitHookLifecycleCtx
}

func (s *server) cancelConcurrentNoWaitHookLifecycleContext() {
	if s == nil {
		return
	}

	s.mu.Lock()
	cancelConcurrentNoWaitHookLifecycleContext := s.concurrentNoWaitHookLifecycleCancel
	s.concurrentNoWaitHookLifecycleCtx = nil
	s.concurrentNoWaitHookLifecycleCancel = nil
	s.mu.Unlock()

	if cancelConcurrentNoWaitHookLifecycleContext != nil {
		cancelConcurrentNoWaitHookLifecycleContext()
	}
}

func (s *server) runSequentialHookStageForEligibleEventsWithErrors(
	eventsWithHooks []eventWithHooks,
	watcher *watcher,
	runHooksForEvent func(eventWithHooks, *watcher) ([]wave.RefreshAction, error),
	hookExecutionFailureLogMessage string,
) ([]wave.RefreshAction, []error) {
	if runHooksForEvent == nil {
		return nil, nil
	}

	descriptors := deriveHookStageExecutionDescriptors(eventsWithHooks)
	allStageActions := make([]wave.RefreshAction, 0)
	stageExecutionErrors := make([]error, 0)
	traceContextForHookStage := s.getCurrentWatcherExecutionTraceContext()
	for _, descriptor := range descriptors {
		stageActions, err := runHooksForEvent(
			descriptor.eventWithHooks,
			watcher,
		)
		if err != nil {
			s.log.Error(
				hookExecutionFailureLogMessage,
				"error",
				err,
				"cycle_id",
				traceContextForHookStage.cycleID,
				"batch_id",
				traceContextForHookStage.batchID,
			)
			stageExecutionErrors = append(stageExecutionErrors, err)
		}
		allStageActions = append(allStageActions, stageActions...)
	}
	return allStageActions, stageExecutionErrors
}

func (s *server) fireNoWaitHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *watcher,
) {
	descriptors := deriveHookStageExecutionDescriptors(eventsWithHooks)
	for _, descriptor := range descriptors {
		s.fireNoWaitHooks(descriptor.eventWithHooks, watcher)
	}
}

func (s *server) runPreHooksForEvents(
	eventsWithHooks []eventWithHooks,
	work *workSet,
	watcher *watcher,
) []wave.RefreshAction {
	stageActions, _ := s.runPreHooksForEventsWithErrors(
		eventsWithHooks,
		work,
		watcher,
	)
	return stageActions
}

func (s *server) runPreHooksForEventsWithErrors(
	eventsWithHooks []eventWithHooks,
	work *workSet,
	watcher *watcher,
) ([]wave.RefreshAction, []error) {
	for _, eventWithHooksForPre := range eventsWithHooks {
		work.addImplicitWork(eventWithHooksForPre.classified)
	}
	return s.runSequentialHookStageForEligibleEventsWithErrors(
		eventsWithHooks,
		watcher,
		s.runPreHooks,
		"Pre-hook execution failed",
	)
}

func (s *server) runConcurrentHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *watcher,
) []wave.RefreshAction {
	concurrentActions, _ := s.runConcurrentHooksForEventsWithContextAndErrors(
		context.Background(),
		eventsWithHooks,
		watcher,
	)
	return concurrentActions
}

func shouldContinueConcurrentHookExecution(
	concurrentHookExecutionContext context.Context,
) bool {
	if concurrentHookExecutionContext == nil {
		return true
	}

	select {
	case <-concurrentHookExecutionContext.Done():
		return false
	default:
		return true
	}
}

func (s *server) runConcurrentHooksForEventsWithContext(
	concurrentHookExecutionContext context.Context,
	eventsWithHooks []eventWithHooks,
	watcher *watcher,
) []wave.RefreshAction {
	concurrentActions, _ := s.runConcurrentHooksForEventsWithContextAndErrors(
		concurrentHookExecutionContext,
		eventsWithHooks,
		watcher,
	)
	return concurrentActions
}

func (s *server) runConcurrentHooksForEventsWithContextAndErrors(
	concurrentHookExecutionContext context.Context,
	eventsWithHooks []eventWithHooks,
	watcher *watcher,
) ([]wave.RefreshAction, []error) {
	descriptors := deriveHookStageExecutionDescriptors(eventsWithHooks)
	traceContextForHookStage := s.getCurrentWatcherExecutionTraceContext()
	actionsByDescriptorIndex := make([][]wave.RefreshAction, len(descriptors))
	executionErrorsByDescriptorIndex := make([]error, len(descriptors))
	var concurrentHooksGroup errgroup.Group

	for descriptorIndex := range descriptors {
		if !shouldContinueConcurrentHookExecution(
			concurrentHookExecutionContext,
		) {
			break
		}

		descriptorForExecution := descriptors[descriptorIndex]
		descriptorIndexForResult := descriptorIndex
		concurrentHooksGroup.Go(func() error {
			if !shouldContinueConcurrentHookExecution(
				concurrentHookExecutionContext,
			) {
				return nil
			}

			concurrentActions, err := s.runConcurrentHooksWithContext(
				concurrentHookExecutionContext,
				descriptorForExecution.eventWithHooks,
				watcher,
			)
			if err != nil {
				if shouldContinueConcurrentHookExecution(
					concurrentHookExecutionContext,
				) {
					s.log.Error(
						"Concurrent hook execution failed",
						"error",
						err,
						"cycle_id",
						traceContextForHookStage.cycleID,
						"batch_id",
						traceContextForHookStage.batchID,
					)
					executionErrorsByDescriptorIndex[descriptorIndexForResult] = err
				}
			}
			actionsByDescriptorIndex[descriptorIndexForResult] = concurrentActions
			return nil
		})
	}

	_ = concurrentHooksGroup.Wait()

	allConcurrentActions := make([]wave.RefreshAction, 0)
	for _, concurrentActionsForEvent := range actionsByDescriptorIndex {
		allConcurrentActions = append(
			allConcurrentActions,
			concurrentActionsForEvent...)
	}
	concurrentHookExecutionErrors := make([]error, 0)
	for _, concurrentHookExecutionError := range executionErrorsByDescriptorIndex {
		if concurrentHookExecutionError == nil {
			continue
		}
		concurrentHookExecutionErrors = append(
			concurrentHookExecutionErrors,
			concurrentHookExecutionError,
		)
	}

	return allConcurrentActions, concurrentHookExecutionErrors
}

func (s *server) runPostHooksForEvents(
	eventsWithHooks []eventWithHooks,
	watcher *watcher,
) []wave.RefreshAction {
	stageActions, _ := s.runPostHooksForEventsWithErrors(
		eventsWithHooks,
		watcher,
	)
	return stageActions
}

func (s *server) runPostHooksForEventsWithErrors(
	eventsWithHooks []eventWithHooks,
	watcher *watcher,
) ([]wave.RefreshAction, []error) {
	return s.runSequentialHookStageForEligibleEventsWithErrors(
		eventsWithHooks,
		watcher,
		s.runPostHooks,
		"Post-hook execution failed",
	)
}

func (s *server) fireNoWaitHooks(ewh eventWithHooks, watcher *watcher) {
	concurrentNoWaitHookLifecycleContext := s.getOrCreateConcurrentNoWaitHookLifecycleContext()
	traceContextForHookExecution := s.getCurrentWatcherExecutionTraceContext()

	plans := deriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hookStageTypeConcurrentNoWait,
		s.resolveHookExecutionPlan,
	)
	for _, plan := range plans {
		if plan.callback != nil {
			callbackForExecution := plan.callback
			changedFilePathForExecution := ewh.classified.event.Name
			hookCallbackTimeoutForExecution := s.deriveHookCallbackTimeoutForExecutionPlan(
				hookStageTypeConcurrentNoWait,
				plan,
			)
			s.runNoWaitHookWithConcurrencyLimit(func() {
				hookCallbackExecutionContext, cancelHookCallbackExecutionContext := deriveExecutionContextWithOptionalTimeout(
					concurrentNoWaitHookLifecycleContext,
					hookCallbackTimeoutForExecution,
				)
				if cancelHookCallbackExecutionContext != nil {
					defer cancelHookCallbackExecutionContext()
				}

				hookContextForExecution := cloneHookContextForExecution(
					ewh.hookCtx,
					deriveHookExecutionContext(hookCallbackExecutionContext),
				)

				if _, err := executeHookCallbackSafely(
					callbackForExecution,
					hookContextForExecution,
				); err != nil {
					s.log.Warn(
						"concurrent-no-wait callback failed",
						"stage",
						deriveHookStageLabel(hookStageTypeConcurrentNoWait),
						"path",
						changedFilePathForExecution,
						"error",
						err,
						"cycle_id",
						traceContextForHookExecution.cycleID,
						"batch_id",
						traceContextForHookExecution.batchID,
					)
				}
			})
		}
		if strings.TrimSpace(plan.command) != "" {
			commandForExecution := plan.command
			changedFilePathForExecution := ewh.classified.event.Name
			hookCommandTimeoutForExecution := s.deriveHookCommandTimeoutForExecutionPlan(
				hookStageTypeConcurrentNoWait,
				plan,
			)
			s.runNoWaitHookWithConcurrencyLimit(func() {
				hookCommandExecutionContext, cancelHookCommandExecutionContext := deriveExecutionContextWithOptionalTimeout(
					concurrentNoWaitHookLifecycleContext,
					hookCommandTimeoutForExecution,
				)
				if cancelHookCommandExecutionContext != nil {
					defer cancelHookCommandExecutionContext()
				}

				if err := executeHookCommandWithContext(
					hookCommandExecutionContext,
					commandForExecution,
				); err != nil {
					s.log.Warn(
						"concurrent-no-wait hook failed",
						"stage",
						deriveHookStageLabel(hookStageTypeConcurrentNoWait),
						"path",
						changedFilePathForExecution,
						"cmd",
						commandForExecution,
						"error",
						err,
						"cycle_id",
						traceContextForHookExecution.cycleID,
						"batch_id",
						traceContextForHookExecution.batchID,
					)
				}
			})
		}
	}
}

func (s *server) runPreHooks(
	ewh eventWithHooks,
	watcher *watcher,
) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	plans := deriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hookStageTypePre,
		s.resolveHookExecutionPlan,
	)
	for _, plan := range plans {
		action, err := s.executeHookExecutionPlanWithContext(
			context.Background(),
			hookStageTypePre,
			plan,
			ewh.hookCtx,
		)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, wrapHookExecutionErrorWithStageAndPath(
				hookStageTypePre,
				ewh.classified.event.Name,
				err,
			)
		}
	}

	return actions, nil
}

func (s *server) runConcurrentHooks(
	ewh eventWithHooks,
	watcher *watcher,
) ([]wave.RefreshAction, error) {
	return s.runConcurrentHooksWithContext(context.Background(), ewh, watcher)
}

func (s *server) runConcurrentHooksWithContext(
	concurrentHookExecutionContext context.Context,
	ewh eventWithHooks,
	watcher *watcher,
) ([]wave.RefreshAction, error) {
	plans := deriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hookStageTypeConcurrent,
		s.resolveHookExecutionPlan,
	)
	if len(plans) == 0 {
		return nil, nil
	}

	actionsByHookIndex := make([]*wave.RefreshAction, len(plans))
	hookExecutionErrorsByHookIndex := make([]error, len(plans))
	var executionGroup errgroup.Group

	for hookIndex := range plans {
		if !shouldContinueConcurrentHookExecution(
			concurrentHookExecutionContext,
		) {
			break
		}

		planForExecution := plans[hookIndex]
		hookIndexForResult := hookIndex
		executionGroup.Go(func() error {
			if !shouldContinueConcurrentHookExecution(
				concurrentHookExecutionContext,
			) {
				return nil
			}

			action, err := s.executeHookExecutionPlanWithContext(
				concurrentHookExecutionContext,
				hookStageTypeConcurrent,
				planForExecution,
				ewh.hookCtx,
			)
			actionsByHookIndex[hookIndexForResult] = action
			if err != nil {
				hookExecutionErrorsByHookIndex[hookIndexForResult] = wrapHookExecutionErrorWithStageAndPath(
					hookStageTypeConcurrent,
					ewh.classified.event.Name,
					err,
				)
			}
			return nil
		})
	}

	_ = executionGroup.Wait()
	actions := make([]wave.RefreshAction, 0, len(actionsByHookIndex))
	for _, action := range actionsByHookIndex {
		if action != nil {
			actions = append(actions, *action)
		}
	}

	return actions, joinHookExecutionErrorsInOrder(
		hookExecutionErrorsByHookIndex,
	)
}

func (s *server) runPostHooks(
	ewh eventWithHooks,
	watcher *watcher,
) ([]wave.RefreshAction, error) {
	var actions []wave.RefreshAction

	plans := deriveHookExecutionPlansForEventStage(
		watcher,
		ewh,
		hookStageTypePost,
		s.resolveHookExecutionPlan,
	)
	for _, plan := range plans {
		action, err := s.executeHookExecutionPlanWithContext(
			context.Background(),
			hookStageTypePost,
			plan,
			ewh.hookCtx,
		)
		if action != nil {
			actions = append(actions, *action)
		}
		if err != nil {
			return actions, wrapHookExecutionErrorWithStageAndPath(
				hookStageTypePost,
				ewh.classified.event.Name,
				err,
			)
		}
	}

	return actions, nil
}

func (s *server) executeHookExecutionPlan(
	stageType hookStageType,
	plan hookExecutionPlan,
	hookContext *wave.HookContext,
) (*wave.RefreshAction, error) {
	return s.executeHookExecutionPlanWithContext(
		context.Background(),
		stageType,
		plan,
		hookContext,
	)
}

func deriveHookExecutionContextError(
	hookExecutionContext context.Context,
) error {
	if hookExecutionContext == nil {
		return nil
	}
	return hookExecutionContext.Err()
}

func executeHookCommandWithContext(
	hookCommandExecutionContext context.Context,
	command string,
) error {
	return executil.RunShellWithContext(hookCommandExecutionContext, command)
}

func (s *server) deriveHookCommandTimeoutForExecutionPlan(
	stageType hookStageType,
	executionPlan hookExecutionPlan,
) time.Duration {
	if s == nil || s.cfg == nil {
		return deriveHookCommandTimeoutDurationForExecutionPlan(
			nil,
			stageType,
			executionPlan,
		)
	}
	return deriveHookCommandTimeoutDurationForExecutionPlan(
		s.cfg.Watch,
		stageType,
		executionPlan,
	)
}

func (s *server) deriveHookCallbackTimeoutForExecutionPlan(
	stageType hookStageType,
	executionPlan hookExecutionPlan,
) time.Duration {
	if s == nil || s.cfg == nil {
		return deriveHookCallbackTimeoutDurationForExecutionPlan(
			nil,
			stageType,
			executionPlan,
		)
	}
	return deriveHookCallbackTimeoutDurationForExecutionPlan(
		s.cfg.Watch,
		stageType,
		executionPlan,
	)
}

func (s *server) executeHookExecutionPlanWithContext(
	parentHookExecutionContext context.Context,
	stageType hookStageType,
	plan hookExecutionPlan,
	hookContext *wave.HookContext,
) (*wave.RefreshAction, error) {
	var action *wave.RefreshAction
	if plan.callback != nil {
		hookCallbackExecutionContext, cancelHookCallbackExecutionContext := deriveExecutionContextWithOptionalTimeout(
			parentHookExecutionContext,
			s.deriveHookCallbackTimeoutForExecutionPlan(stageType, plan),
		)
		if cancelHookCallbackExecutionContext != nil {
			defer cancelHookCallbackExecutionContext()
		}

		hookExecutionContext := deriveHookExecutionContext(
			hookCallbackExecutionContext,
		)
		hookContextForExecution := cloneHookContextForExecution(
			hookContext,
			hookExecutionContext,
		)

		callbackAction, err := executeHookCallbackSafely(
			plan.callback,
			hookContextForExecution,
		)
		if err != nil {
			return nil, err
		}
		action = callbackAction
	}

	if strings.TrimSpace(plan.command) != "" {
		hookCommandExecutionContext, cancelHookCommandExecutionContext := deriveExecutionContextWithOptionalTimeout(
			parentHookExecutionContext,
			s.deriveHookCommandTimeoutForExecutionPlan(stageType, plan),
		)
		if cancelHookCommandExecutionContext != nil {
			defer cancelHookCommandExecutionContext()
		}

		if !shouldContinueConcurrentHookExecution(hookCommandExecutionContext) {
			return action, deriveHookExecutionContextError(
				hookCommandExecutionContext,
			)
		}

		if err := executeHookCommandWithContext(
			hookCommandExecutionContext,
			plan.command,
		); err != nil {
			return action, err
		}
	}

	return action, nil
}
