package hooks_test

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/devserver/internal/hooks"
)

func TestResolveHookCommand(t *testing.T) {
	testCases := []struct {
		Name string
		Cfg  *wave.ParsedConfig
		Hook wave.OnChangeHook
		Want string
	}{
		{
			Name: "runs user then framework hook",
			Cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{DevBuildHook: "go generate ./..."},
				FrameworkDevBuildHook: "go run ./backend/cmd/build --dev --hook",
			},
			Hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "go generate ./... && go run ./backend/cmd/build --dev --hook",
		},
		{
			Name: "framework hook fallback when user hook empty",
			Cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{},
				FrameworkDevBuildHook: "go run ./backend/cmd/build --dev --hook",
			},
			Hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "go run ./backend/cmd/build --dev --hook",
		},
		{
			Name: "user hook only",
			Cfg: &wave.ParsedConfig{
				Core: &wave.CoreConfig{DevBuildHook: "go generate ./..."},
			},
			Hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "go generate ./...",
		},
		{
			Name: "returns empty when no hook commands configured",
			Cfg: &wave.ParsedConfig{
				Core: &wave.CoreConfig{},
			},
			Hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "",
		},
		{
			Name: "trims whitespace around hooks",
			Cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{DevBuildHook: "   go generate ./...   "},
				FrameworkDevBuildHook: "\tgo run ./backend/cmd/build --dev --hook\t",
			},
			Hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "go generate ./... && go run ./backend/cmd/build --dev --hook",
		},
		{
			Name: "handles nil config safely",
			Cfg:  nil,
			Hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "",
		},
		{
			Name: "resolves explicit cmd when hook does not use dev build hooks",
			Cfg:  &wave.ParsedConfig{},
			Hook: wave.OnChangeHook{
				Cmd: "echo hi",
			},
			Want: "echo hi",
		},
		{
			Name: "runs explicit cmd before combined dev build hooks when both are set",
			Cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{DevBuildHook: "go generate ./..."},
				FrameworkDevBuildHook: "go run ./backend/cmd/build --dev --hook",
			},
			Hook: wave.OnChangeHook{
				Cmd:                             "echo pre-step",
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "echo pre-step && go generate ./... && go run ./backend/cmd/build --dev --hook",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			got := resolveHookCommandForHooksTests(testCase.Cfg, testCase.Hook)
			if got != testCase.Want {
				t.Fatalf("resolveHookCommand(...) = %q, want %q", got, testCase.Want)
			}
		})
	}
}

func TestResolveSequentialShellCommands(t *testing.T) {
	got := hooks.ResolveSequentialShellCommands("", " echo a ", "\t", "echo b")
	want := "echo a && echo b"
	if got != want {
		t.Fatalf("hooks.ResolveSequentialShellCommands(...) = %q, want %q", got, want)
	}
}

func resolveHookCommandForHooksTests(
	parsedConfig *wave.ParsedConfig,
	hook wave.OnChangeHook,
) string {
	if !hook.RunCombinedDevBuildHookCommands {
		return hook.Cmd
	}
	if strings.TrimSpace(hook.Cmd) != "" {
		return hooks.ResolveSequentialShellCommands(
			hook.Cmd,
			getUserDevBuildHookForHooksTests(parsedConfig),
			getFrameworkDevBuildHookForHooksTests(parsedConfig),
		)
	}
	return hooks.ResolveSequentialShellCommands(
		getUserDevBuildHookForHooksTests(parsedConfig),
		getFrameworkDevBuildHookForHooksTests(parsedConfig),
	)
}

func getUserDevBuildHookForHooksTests(parsedConfig *wave.ParsedConfig) string {
	if parsedConfig == nil || parsedConfig.Core == nil {
		return ""
	}
	return parsedConfig.Core.DevBuildHook
}

func getFrameworkDevBuildHookForHooksTests(
	parsedConfig *wave.ParsedConfig,
) string {
	if parsedConfig == nil {
		return ""
	}
	return parsedConfig.FrameworkDevBuildHook
}
