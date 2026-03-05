package hooks_test

import (
	"github.com/vormadev/vorma/internal/wavetest"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveframework"
	"github.com/vormadev/vorma/wave/wavewatch"
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave/buildtime/internal/devserver/hooks"
)

func TestResolveHookCommand(t *testing.T) {
	testCases := []struct {
		Name string
		Cfg  waveconfig.ParsedConfig
		Hook wavewatch.OnChangeHook
		Want string
	}{
		{
			Name: "runs user then framework hook",
			Cfg: newParsedConfigForHooksTests(
				t,
				"go generate ./...",
				"go run ./backend/cmd/build --dev --hook",
			),
			Hook: wavewatch.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "go generate ./... && go run ./backend/cmd/build --dev --hook",
		},
		{
			Name: "framework hook fallback when user hook empty",
			Cfg: newParsedConfigForHooksTests(
				t,
				"",
				"go run ./backend/cmd/build --dev --hook",
			),
			Hook: wavewatch.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "go run ./backend/cmd/build --dev --hook",
		},
		{
			Name: "user hook only",
			Cfg:  newParsedConfigForHooksTests(t, "go generate ./...", ""),
			Hook: wavewatch.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "go generate ./...",
		},
		{
			Name: "returns empty when no hook commands configured",
			Cfg:  newParsedConfigForHooksTests(t, "", ""),
			Hook: wavewatch.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "",
		},
		{
			Name: "trims whitespace around hooks",
			Cfg: newParsedConfigForHooksTests(
				t,
				"   go generate ./...   ",
				"\tgo run ./backend/cmd/build --dev --hook\t",
			),
			Hook: wavewatch.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "go generate ./... && go run ./backend/cmd/build --dev --hook",
		},
		{
			Name: "handles nil config safely",
			Cfg:  nil,
			Hook: wavewatch.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			Want: "",
		},
		{
			Name: "resolves explicit cmd when hook does not use dev build hooks",
			Cfg:  newParsedConfigForHooksTests(t, "", ""),
			Hook: wavewatch.OnChangeHook{
				Cmd: "echo hi",
			},
			Want: "echo hi",
		},
		{
			Name: "runs explicit cmd before combined dev build hooks when both are set",
			Cfg: newParsedConfigForHooksTests(
				t,
				"go generate ./...",
				"go run ./backend/cmd/build --dev --hook",
			),
			Hook: wavewatch.OnChangeHook{
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
				t.Fatalf(
					"resolveHookCommand(...) = %q, want %q",
					got,
					testCase.Want,
				)
			}
		})
	}
}

func TestResolveSequentialShellCommands(t *testing.T) {
	got := hooks.ResolveSequentialShellCommands("", " echo a ", "\t", "echo b")
	want := "echo a && echo b"
	if got != want {
		t.Fatalf(
			"hooks.ResolveSequentialShellCommands(...) = %q, want %q",
			got,
			want,
		)
	}
}

func resolveHookCommandForHooksTests(
	parsedConfig waveconfig.ParsedConfig,
	hook wavewatch.OnChangeHook,
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

func getUserDevBuildHookForHooksTests(parsedConfig waveconfig.ParsedConfig) string {
	if parsedConfig == nil || parsedConfig.Core() == nil {
		return ""
	}
	return parsedConfig.Core().DevBuildHook()
}

func getFrameworkDevBuildHookForHooksTests(
	parsedConfig waveconfig.ParsedConfig,
) string {
	if parsedConfig == nil {
		return ""
	}
	return waveframework.StateForConfig(parsedConfig).DevBuildHook
}

func newParsedConfigForHooksTests(
	t *testing.T,
	userDevBuildHook string,
	frameworkDevBuildHook string,
) waveconfig.ParsedConfig {
	t.Helper()
	config := wavetest.NewParsedConfigAtRoot(t, t.TempDir())
	wavetest.SetCoreDevBuildHook(config, userDevBuildHook)
	waveframework.StateForConfig(config).DevBuildHook = frameworkDevBuildHook
	return config
}
