package tooling

import (
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
