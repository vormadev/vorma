package tooling

import (
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestResolveHookCommand(t *testing.T) {
	tests := []struct {
		name string
		cfg  *wave.ParsedConfig
		hook wave.OnChangeHook
		want string
	}{
		{
			name: "runs user then framework hook",
			cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{DevBuildHook: "go generate ./..."},
				FrameworkDevBuildHook: "go run ./backend/cmd/build --dev --hook",
			},
			hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			want: "go generate ./... && go run ./backend/cmd/build --dev --hook",
		},
		{
			name: "framework hook fallback when user hook empty",
			cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{},
				FrameworkDevBuildHook: "go run ./backend/cmd/build --dev --hook",
			},
			hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			want: "go run ./backend/cmd/build --dev --hook",
		},
		{
			name: "user hook only",
			cfg: &wave.ParsedConfig{
				Core: &wave.CoreConfig{DevBuildHook: "go generate ./..."},
			},
			hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			want: "go generate ./...",
		},
		{
			name: "returns empty when no hook commands configured",
			cfg: &wave.ParsedConfig{
				Core: &wave.CoreConfig{},
			},
			hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			want: "",
		},
		{
			name: "trims whitespace around hooks",
			cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{DevBuildHook: "   go generate ./...   "},
				FrameworkDevBuildHook: "\tgo run ./backend/cmd/build --dev --hook\t",
			},
			hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			want: "go generate ./... && go run ./backend/cmd/build --dev --hook",
		},
		{
			name: "handles nil config safely",
			cfg:  nil,
			hook: wave.OnChangeHook{
				RunCombinedDevBuildHookCommands: true,
			},
			want: "",
		},
		{
			name: "resolves explicit cmd when hook does not use dev build hooks",
			cfg:  &wave.ParsedConfig{},
			hook: wave.OnChangeHook{
				Cmd: "echo hi",
			},
			want: "echo hi",
		},
		{
			name: "runs explicit cmd before combined dev build hooks when both are set",
			cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{DevBuildHook: "go generate ./..."},
				FrameworkDevBuildHook: "go run ./backend/cmd/build --dev --hook",
			},
			hook: wave.OnChangeHook{
				Cmd:                             "echo pre-step",
				RunCombinedDevBuildHookCommands: true,
			},
			want: "echo pre-step && go generate ./... && go run ./backend/cmd/build --dev --hook",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			s := &server{cfg: testCase.cfg}
			got := s.resolveHookCommand(testCase.hook)
			if got != testCase.want {
				t.Fatalf("resolveHookCommand(...) = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestResolveSequentialShellCommands(t *testing.T) {
	got := resolveSequentialShellCommands("", " echo a ", "\t", "echo b")
	want := "echo a && echo b"
	if got != want {
		t.Fatalf("resolveSequentialShellCommands(...) = %q, want %q", got, want)
	}
}
