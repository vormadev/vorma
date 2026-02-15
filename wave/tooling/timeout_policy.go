package tooling

import (
	"context"
	"time"

	"github.com/vormadev/vorma/wave"
)

func deriveTimeoutDurationFromMilliseconds(
	timeoutMilliseconds int,
) time.Duration {
	if timeoutMilliseconds <= 0 {
		return 0
	}
	return time.Duration(timeoutMilliseconds) * time.Millisecond
}

func deriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
	stageTimeoutMilliseconds int,
	executionTimeoutMilliseconds int,
	disableStageTimeout bool,
) time.Duration {
	if disableStageTimeout {
		return 0
	}
	if executionTimeoutMilliseconds > 0 {
		return deriveTimeoutDurationFromMilliseconds(
			executionTimeoutMilliseconds,
		)
	}
	return deriveTimeoutDurationFromMilliseconds(stageTimeoutMilliseconds)
}

func deriveHookCommandStageTimeoutMilliseconds(
	watchConfig *wave.WatchConfig,
	stageType hookStageType,
) int {
	if watchConfig == nil {
		return 0
	}

	return deriveHookStageTimeoutMilliseconds(
		stageType,
		hookStageTimeoutMilliseconds{
			pre:              watchConfig.HookCommandTimeouts.PreCommandTimeoutMilliseconds,
			concurrent:       watchConfig.HookCommandTimeouts.ConcurrentCommandTimeoutMilliseconds,
			post:             watchConfig.HookCommandTimeouts.PostCommandTimeoutMilliseconds,
			concurrentNoWait: watchConfig.HookCommandTimeouts.ConcurrentNoWaitCommandTimeoutMilliseconds,
		},
	)
}

func deriveHookCallbackStageTimeoutMilliseconds(
	watchConfig *wave.WatchConfig,
	stageType hookStageType,
) int {
	if watchConfig == nil {
		return 0
	}

	return deriveHookStageTimeoutMilliseconds(
		stageType,
		hookStageTimeoutMilliseconds{
			pre:              watchConfig.HookCallbackTimeouts.PreCallbackTimeoutMilliseconds,
			concurrent:       watchConfig.HookCallbackTimeouts.ConcurrentCallbackTimeoutMilliseconds,
			post:             watchConfig.HookCallbackTimeouts.PostCallbackTimeoutMilliseconds,
			concurrentNoWait: watchConfig.HookCallbackTimeouts.ConcurrentNoWaitCallbackTimeoutMilliseconds,
		},
	)
}

type hookStageTimeoutMilliseconds struct {
	pre              int
	concurrent       int
	post             int
	concurrentNoWait int
}

func deriveHookStageTimeoutMilliseconds(
	stageType hookStageType,
	timeoutMillisecondsByStage hookStageTimeoutMilliseconds,
) int {
	switch stageType {
	case hookStageTypePre:
		return timeoutMillisecondsByStage.pre
	case hookStageTypeConcurrent:
		return timeoutMillisecondsByStage.concurrent
	case hookStageTypePost:
		return timeoutMillisecondsByStage.post
	case hookStageTypeConcurrentNoWait:
		return timeoutMillisecondsByStage.concurrentNoWait
	default:
		return 0
	}
}

func deriveHookCommandTimeoutDurationForExecutionPlan(
	watchConfig *wave.WatchConfig,
	stageType hookStageType,
	executionPlan hookExecutionPlan,
) time.Duration {
	return deriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		deriveHookCommandStageTimeoutMilliseconds(watchConfig, stageType),
		executionPlan.commandTimeoutMilliseconds,
		executionPlan.disableStageCommandTimeout,
	)
}

func deriveHookCallbackTimeoutDurationForExecutionPlan(
	watchConfig *wave.WatchConfig,
	stageType hookStageType,
	executionPlan hookExecutionPlan,
) time.Duration {
	return deriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		deriveHookCallbackStageTimeoutMilliseconds(watchConfig, stageType),
		executionPlan.callbackTimeoutMilliseconds,
		executionPlan.disableStageCallbackTimeout,
	)
}

func deriveBuildHookCommandTimeoutMilliseconds(
	coreConfig *wave.CoreConfig,
	isDev bool,
) int {
	if coreConfig == nil {
		return 0
	}

	if isDev {
		return coreConfig.DevBuildHookTimeoutMilliseconds
	}
	return coreConfig.ProdBuildHookTimeoutMilliseconds
}

func deriveBuildHookCommandTimeoutDuration(
	coreConfig *wave.CoreConfig,
	isDev bool,
) time.Duration {
	return deriveResolvedTimeoutDurationFromStageAndExecutionPolicy(
		deriveBuildHookCommandTimeoutMilliseconds(coreConfig, isDev),
		0,
		false,
	)
}

func deriveExecutionContextWithOptionalTimeout(
	parentExecutionContext context.Context,
	executionTimeoutDuration time.Duration,
) (
	executionContext context.Context,
	cancelExecutionContext context.CancelFunc,
) {
	if executionTimeoutDuration <= 0 {
		if parentExecutionContext == nil {
			return context.Background(), nil
		}
		return parentExecutionContext, nil
	}

	if parentExecutionContext == nil {
		return context.WithTimeout(context.Background(), executionTimeoutDuration)
	}
	return context.WithTimeout(parentExecutionContext, executionTimeoutDuration)
}
