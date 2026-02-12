package tooling

import (
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestResolveCmd_DevBuildHook(t *testing.T) {
	tests := []struct {
		name string
		cfg  *wave.ParsedConfig
		want string
	}{
		{
			name: "runs user then framework hook",
			cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{DevBuildHook: "go generate ./..."},
				FrameworkDevBuildHook: "go run ./backend/cmd/build --dev --hook",
			},
			want: "go generate ./... && go run ./backend/cmd/build --dev --hook",
		},
		{
			name: "framework hook fallback when user hook empty",
			cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{},
				FrameworkDevBuildHook: "go run ./backend/cmd/build --dev --hook",
			},
			want: "go run ./backend/cmd/build --dev --hook",
		},
		{
			name: "user hook only",
			cfg: &wave.ParsedConfig{
				Core: &wave.CoreConfig{DevBuildHook: "go generate ./..."},
			},
			want: "go generate ./...",
		},
		{
			name: "returns empty when no hooks configured",
			cfg: &wave.ParsedConfig{
				Core: &wave.CoreConfig{},
			},
			want: "",
		},
		{
			name: "trims whitespace around hooks",
			cfg: &wave.ParsedConfig{
				Core:                  &wave.CoreConfig{DevBuildHook: "   go generate ./...   "},
				FrameworkDevBuildHook: "\tgo run ./backend/cmd/build --dev --hook\t",
			},
			want: "go generate ./... && go run ./backend/cmd/build --dev --hook",
		},
		{
			name: "handles nil config safely",
			cfg:  nil,
			want: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			s := &server{cfg: testCase.cfg}
			got := s.resolveCmd("DevBuildHook")
			if got != testCase.want {
				t.Fatalf("resolveCmd(DevBuildHook) = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestResolveCmd_NonDevBuildHookPassThrough(t *testing.T) {
	s := &server{}
	got := s.resolveCmd("echo hi")
	if got != "echo hi" {
		t.Fatalf("resolveCmd(echo hi) = %q, want %q", got, "echo hi")
	}
}

func TestResolveSequentialShellCommands(t *testing.T) {
	got := resolveSequentialShellCommands("", " echo a ", "\t", "echo b")
	want := "echo a && echo b"
	if got != want {
		t.Fatalf("resolveSequentialShellCommands(...) = %q, want %q", got, want)
	}
}
