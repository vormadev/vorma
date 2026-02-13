package modulegraph

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestResolveDeterministicModuleRegistrationOrder(t *testing.T) {
	t.Run("returns empty for empty input", func(t *testing.T) {
		orderedModules, err := ResolveDeterministicModuleRegistrationOrder(nil)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(orderedModules) != 0 {
			t.Fatalf("expected zero modules, got %d", len(orderedModules))
		}
	})

	t.Run("sorts by dependency then lexicographic module id", func(t *testing.T) {
		modules := []Module{
			{ModuleID: "c", DependencyModuleIDs: []string{"b"}},
			{ModuleID: "d"},
			{ModuleID: "a"},
			{ModuleID: "b", DependencyModuleIDs: []string{"a"}},
		}

		orderedModules, err := ResolveDeterministicModuleRegistrationOrder(modules)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		var orderedModuleIDs []string
		for _, module := range orderedModules {
			orderedModuleIDs = append(orderedModuleIDs, module.ModuleID)
		}

		expectedModuleIDs := []string{"a", "b", "c", "d"}
		if !reflect.DeepEqual(orderedModuleIDs, expectedModuleIDs) {
			t.Fatalf("module order = %v, want %v", orderedModuleIDs, expectedModuleIDs)
		}
	})

	t.Run("resolves independent modules in lexicographic order", func(t *testing.T) {
		modules := []Module{
			{ModuleID: "payments"},
			{ModuleID: "auth"},
			{ModuleID: "catalog"},
		}

		orderedModules, err := ResolveDeterministicModuleRegistrationOrder(modules)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		var orderedModuleIDs []string
		for _, module := range orderedModules {
			orderedModuleIDs = append(orderedModuleIDs, module.ModuleID)
		}

		expectedModuleIDs := []string{"auth", "catalog", "payments"}
		if !reflect.DeepEqual(orderedModuleIDs, expectedModuleIDs) {
			t.Fatalf("module order = %v, want %v", orderedModuleIDs, expectedModuleIDs)
		}
	})

	t.Run("fails when module id is empty", func(t *testing.T) {
		_, err := ResolveDeterministicModuleRegistrationOrder([]Module{
			{ModuleID: ""},
		})
		if err == nil {
			t.Fatal("expected error for empty module id")
		}
		if !strings.Contains(err.Error(), "empty module id") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails when module id has surrounding whitespace", func(t *testing.T) {
		_, err := ResolveDeterministicModuleRegistrationOrder([]Module{
			{ModuleID: " core "},
		})
		if err == nil {
			t.Fatal("expected error for module id whitespace")
		}
		if !strings.Contains(err.Error(), "leading or trailing whitespace") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails on duplicate module ids", func(t *testing.T) {
		_, err := ResolveDeterministicModuleRegistrationOrder([]Module{
			{ModuleID: "core"},
			{ModuleID: "core"},
		})
		if err == nil {
			t.Fatal("expected error for duplicate module ids")
		}
		if !strings.Contains(err.Error(), "duplicate module id") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails on missing dependency", func(t *testing.T) {
		_, err := ResolveDeterministicModuleRegistrationOrder([]Module{
			{
				ModuleID:            "core",
				DependencyModuleIDs: []string{"missing"},
			},
		})
		if err == nil {
			t.Fatal("expected error for missing dependency")
		}
		if !strings.Contains(err.Error(), "depends on missing module") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails on self dependency", func(t *testing.T) {
		_, err := ResolveDeterministicModuleRegistrationOrder([]Module{
			{
				ModuleID:            "core",
				DependencyModuleIDs: []string{"core"},
			},
		})
		if err == nil {
			t.Fatal("expected error for self dependency")
		}
		if !strings.Contains(err.Error(), "cannot depend on itself") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails on dependency cycle", func(t *testing.T) {
		_, err := ResolveDeterministicModuleRegistrationOrder([]Module{
			{
				ModuleID:            "a",
				DependencyModuleIDs: []string{"b"},
			},
			{
				ModuleID:            "b",
				DependencyModuleIDs: []string{"a"},
			},
		})
		if err == nil {
			t.Fatal("expected error for dependency cycle")
		}
		if !strings.Contains(err.Error(), "module dependency cycle detected") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRunDeterministicModuleRegistrationLifecycle(t *testing.T) {
	t.Run("runs each phase across full graph in deterministic order", func(t *testing.T) {
		var executionTrace []string

		recordHookExecution := func(phase RegistrationPhase, moduleID string) func() error {
			return func() error {
				executionTrace = append(executionTrace, string(phase)+":"+moduleID)
				return nil
			}
		}

		modules := []Module{
			{
				ModuleID:            "catalog",
				DependencyModuleIDs: []string{"auth"},
				RegisterLoaders:     recordHookExecution(RegistrationPhaseRegisterLoaders, "catalog"),
				RegisterActions:     recordHookExecution(RegistrationPhaseRegisterActions, "catalog"),
				RegisterRoutes:      recordHookExecution(RegistrationPhaseRegisterRoutes, "catalog"),
				Finalize:            recordHookExecution(RegistrationPhaseFinalize, "catalog"),
			},
			{
				ModuleID:        "auth",
				RegisterLoaders: recordHookExecution(RegistrationPhaseRegisterLoaders, "auth"),
				RegisterActions: recordHookExecution(RegistrationPhaseRegisterActions, "auth"),
				RegisterRoutes:  recordHookExecution(RegistrationPhaseRegisterRoutes, "auth"),
				Finalize:        recordHookExecution(RegistrationPhaseFinalize, "auth"),
			},
		}

		if err := RunDeterministicModuleRegistrationLifecycle(modules); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		expectedExecutionTrace := []string{
			"register-loaders:auth",
			"register-loaders:catalog",
			"register-actions:auth",
			"register-actions:catalog",
			"register-routes:auth",
			"register-routes:catalog",
			"finalize:auth",
			"finalize:catalog",
		}
		if !reflect.DeepEqual(executionTrace, expectedExecutionTrace) {
			t.Fatalf("execution trace = %v, want %v", executionTrace, expectedExecutionTrace)
		}
	})

	t.Run("returns module and phase when hook fails", func(t *testing.T) {
		expectedErr := errors.New("boom")

		err := RunDeterministicModuleRegistrationLifecycle([]Module{
			{
				ModuleID: "core",
				RegisterActions: func() error {
					return expectedErr
				},
			},
		})
		if err == nil {
			t.Fatal("expected error from failing hook")
		}
		if !strings.Contains(err.Error(), `module "core" phase "register-actions" failed`) {
			t.Fatalf("unexpected error: %v", err)
		}
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected wrapped error %v, got %v", expectedErr, err)
		}
	})
}
