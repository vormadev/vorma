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
		{"test", "--scope", "matcher"},
		{"test", "--lang", "ts", "--scope", "fw"},
		{"test", "--lang", "ts", "--scope", "matcher"},
		{"build"},
		{"stress", "--intensity", "2"},
		{"gate"},
		{"gate", "--scope", "fw"},
		{"gate", "--scope", "matcher"},
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

func TestEnforcerReusesSharedTasks(t *testing.T) {
	for action, matrix := range action_matrices {
		assert_matrix_has_all_tasks(t, action, matrix)
	}
	assert_matrix_has_all_tasks(t, action_vals.Stress, stress_action_matrix)

	typecheck_matrix := action_matrices[action_vals.Typecheck]
	build_matrix := action_matrices[action_vals.Build]
	if typecheck_matrix.go_fw != build_matrix.go_fw {
		t.Fatalf("Go fw compile check should be shared by typecheck and build")
	}
	if typecheck_matrix.go_matcher != build_matrix.go_matcher {
		t.Fatalf("Go matcher compile check should be shared by typecheck and build")
	}
	if typecheck_matrix.go_other != build_matrix.go_other {
		t.Fatalf("Go other compile check should be shared by typecheck and build")
	}
	if build_matrix.ts_fw != build_matrix.ts_matcher {
		t.Fatalf("TypeScript build should be shared by fw and matcher")
	}
	if build_matrix.ts_fw != build_matrix.ts_other {
		t.Fatalf("TypeScript build should be shared by fw and other")
	}

	test_matrix := action_matrices[action_vals.Test]
	if test_matrix.go_matcher != test_matrix.ts_matcher {
		t.Fatalf("matcher tests should be shared by Go and TypeScript test actions")
	}

	if stress_action_matrix.go_matcher != stress_action_matrix.ts_matcher {
		t.Fatalf("matcher stress should be shared by Go and TypeScript stress actions")
	}

	low_stress_tasks, err := enforcer_app{}.action_tasks(action_vals.Stress, enforcer_request{
		actions:   []action_name{action_vals.Stress},
		lang:      lang_scope_vals.All,
		scope:     target_scope_vals.All,
		intensity: 1,
	})
	if err != nil {
		t.Fatalf("low stress task resolution failed: %v", err)
	}
	high_stress_tasks, err := enforcer_app{}.action_tasks(action_vals.Stress, enforcer_request{
		actions:   []action_name{action_vals.Stress},
		lang:      lang_scope_vals.All,
		scope:     target_scope_vals.All,
		intensity: 2,
	})
	if err != nil {
		t.Fatalf("high stress task resolution failed: %v", err)
	}
	if len(low_stress_tasks) != len(high_stress_tasks) {
		t.Fatalf("stress task count should not depend on intensity")
	}
	for i := range low_stress_tasks {
		if low_stress_tasks[i] != high_stress_tasks[i] {
			t.Fatalf("stress task identity should not depend on intensity")
		}
	}
}

func assert_matrix_has_all_tasks(
	t *testing.T,
	action action_name,
	matrix action_matrix,
) {
	t.Helper()
	if matrix.go_fw == nil {
		t.Fatalf("%s matrix missing Go fw task", action)
	}
	if matrix.go_matcher == nil {
		t.Fatalf("%s matrix missing Go matcher task", action)
	}
	if matrix.go_other == nil {
		t.Fatalf("%s matrix missing Go other task", action)
	}
	if matrix.ts_fw == nil {
		t.Fatalf("%s matrix missing TypeScript fw task", action)
	}
	if matrix.ts_matcher == nil {
		t.Fatalf("%s matrix missing TypeScript matcher task", action)
	}
	if matrix.ts_other == nil {
		t.Fatalf("%s matrix missing TypeScript other task", action)
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
