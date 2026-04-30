package main

import (
	"testing"

	"github.com/vormadev/vorma/internal/cmd/internal/tooling"
)

func TestEnforcerDryRunActions(t *testing.T) {
	tests := [][]string{
		{"install"},
		{"install", "--lang", "go"},
		{"install", "--lang", "ts", "--scope", "fw"},
		{"fmt"},
		{"fmt", "--lang", "go"},
		{"fmt", "--lang", "ts", "--scope", "other"},
		{"lint"},
		{"lint", "--lang", "go", "--scope", "fw"},
		{"lint", "--lang", "ts", "--scope", "other"},
		{"fix"},
		{"typecheck"},
		{"typecheck", "--lang", "go"},
		{"typecheck", "--scope", "fw"},
		{"test"},
		{"test", "--lang", "ts", "--scope", "fw"},
		{"build"},
		{"stress", "--intensity", "2"},
		{"gate"},
		{"gate", "--scope", "fw"},
		{"gate", "--scope", "other"},
		{"fmt", "lint", "typecheck", "--lang", "go", "--scope", "fw"},
	}

	for _, args := range tests {
		t.Run(args[0], func(t *testing.T) {
			if err := run_args_for_test(args...); err != nil {
				t.Fatalf("%v dry run failed: %v", args, err)
			}
		})
	}
}

func TestEnforcerStressRequiresIntensity(t *testing.T) {
	if err := run_args_for_test("stress"); err == nil {
		t.Fatalf("stress should require intensity")
	}
}

func TestGateActionsIncludeFix(t *testing.T) {
	got := map[action_name]bool{}
	for _, action := range gate_actions {
		got[action] = true
	}
	for _, action := range []action_name{
		action_vals.Install,
		action_vals.Fix,
		action_vals.Format,
		action_vals.Lint,
		action_vals.Typecheck,
		action_vals.Build,
		action_vals.Test,
	} {
		if !got[action] {
			t.Fatalf("gate_actions missing %s", action)
		}
	}
}

func TestEnforcerRejectsOldLeafCommands(t *testing.T) {
	commands := []string{
		"test-kit",
		"test-docs",
		"test-internal-go",
		"typecheck-docs",
		"test-framework-prod-react",
	}

	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			if err := run_args_for_test(command); err == nil {
				t.Fatalf("%s should not be public", command)
			}
		})
	}
}

func run_args_for_test(args ...string) error {
	app := enforcer_app{App: tooling.App{Root: "/repo", DryRun: true}}
	req, err := app.parse_request(args)
	if err != nil {
		return err
	}
	return app.run_request(req)
}
