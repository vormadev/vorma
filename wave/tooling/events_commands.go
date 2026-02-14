package tooling

import (
	"context"
	"strings"

	"github.com/vormadev/vorma/wave"
)

func needsHardReload(watchedFile *wave.WatchedFile) bool {
	if watchedFile == nil {
		return false
	}
	return watchedFile.RecompileGoBinary || watchedFile.RestartApp
}

func (s *server) resolveHookCommand(hook wave.OnChangeHook) string {
	if hook.RunCombinedDevBuildHookCommands {
		if strings.TrimSpace(hook.Cmd) != "" {
			return resolveSequentialShellCommands(
				hook.Cmd,
				getUserDevBuildHook(s.cfg),
				getFrameworkDevBuildHook(s.cfg),
			)
		}
		return resolveSequentialShellCommands(
			getUserDevBuildHook(s.cfg),
			getFrameworkDevBuildHook(s.cfg),
		)
	}
	return hook.Cmd
}

func (s *server) resolveHookExecutionPlan(
	hook wave.OnChangeHook,
) hookExecutionPlan {
	if s == nil || s.cfg == nil {
		return deriveHookExecutionPlanFromHook(hook, nil)
	}
	if !hook.RunCombinedDevBuildHookCommands || s.cfg.FrameworkRunBuildHook == nil {
		return deriveHookExecutionPlanFromHook(hook, s.resolveHookCommand)
	}

	frameworkBuildHookRunner := s.cfg.FrameworkRunBuildHook
	userAndExplicitCommand := resolveSequentialShellCommands(
		hook.Cmd,
		getUserDevBuildHook(s.cfg),
	)
	originalCallback := hook.Callback

	return hookExecutionPlan{
		callback: func(hookContext *wave.HookContext) (*wave.RefreshAction, error) {
			var callbackAction *wave.RefreshAction
			var callbackErr error
			if originalCallback != nil {
				callbackAction, callbackErr = originalCallback(hookContext)
				if callbackErr != nil {
					return callbackAction, callbackErr
				}
			}

			hookExecutionContext := context.Background()
			if hookContext != nil && hookContext.ExecutionContext != nil {
				hookExecutionContext = hookContext.ExecutionContext
			}

			if strings.TrimSpace(userAndExplicitCommand) != "" {
				if err := executeHookCommandWithContext(
					hookExecutionContext,
					userAndExplicitCommand,
				); err != nil {
					return callbackAction, err
				}
			}

			if err := frameworkBuildHookRunner(
				hookExecutionContext,
				true,
			); err != nil {
				return callbackAction, err
			}
			return callbackAction, nil
		},
		command:                     "",
		commandTimeoutMilliseconds:  hook.CommandTimeoutMilliseconds,
		disableStageCommandTimeout:  hook.DisableStageCommandTimeout,
		callbackTimeoutMilliseconds: hook.CallbackTimeoutMilliseconds,
		disableStageCallbackTimeout: hook.DisableStageCallbackTimeout,
	}
}

func getUserDevBuildHook(parsedConfig *wave.ParsedConfig) string {
	if parsedConfig == nil || parsedConfig.Core == nil {
		return ""
	}
	return parsedConfig.Core.DevBuildHook
}

func getFrameworkDevBuildHook(parsedConfig *wave.ParsedConfig) string {
	if parsedConfig == nil {
		return ""
	}
	return parsedConfig.FrameworkDevBuildHook
}

// resolveSequentialShellCommands combines non-empty shell commands in order.
// The resulting command preserves "fail fast" behavior by chaining with &&.
func resolveSequentialShellCommands(commands ...string) string {
	nonEmptyCommands := make([]string, 0, len(commands))
	for _, command := range commands {
		trimmedCommand := strings.TrimSpace(command)
		if trimmedCommand != "" {
			nonEmptyCommands = append(nonEmptyCommands, trimmedCommand)
		}
	}
	return strings.Join(nonEmptyCommands, " && ")
}
