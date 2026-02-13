package vormaruntime

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNewAppRoot(t *testing.T) {
	t.Run("panics when app is nil", func(t *testing.T) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				t.Fatal("expected panic when app is nil")
			}
			if !strings.Contains(recovered.(string), "app cannot be nil") {
				t.Fatalf("unexpected panic: %v", recovered)
			}
		}()
		_ = NewAppRoot(nil, nil)
	})
}

func TestAppRootGetApp(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})

	t.Run("runs module registration once", func(t *testing.T) {
		registerLoadersCallCount := 0

		root := NewAppRoot(fixture.app, []AppModule{
			{
				ModuleID: "core",
				RegisterLoaders: func(app *Vorma) error {
					registerLoadersCallCount++
					return nil
				},
			},
		})

		firstCallApp := root.GetApp()
		secondCallApp := root.GetApp()
		if firstCallApp != fixture.app || secondCallApp != fixture.app {
			t.Fatal("expected root to return original app")
		}
		if registerLoadersCallCount != 1 {
			t.Fatalf("registerLoadersCallCount = %d, want 1", registerLoadersCallCount)
		}
	})

	t.Run("panics with wrapped registration error", func(t *testing.T) {
		expectedErr := errors.New("bad registration")
		root := NewAppRoot(fixture.app, []AppModule{
			{
				ModuleID: "core",
				RegisterLoaders: func(app *Vorma) error {
					return expectedErr
				},
			},
		})

		defer func() {
			recovered := recover()
			if recovered == nil {
				t.Fatal("expected panic from registration failure")
			}

			panicText := recovered.(error).Error()
			if !strings.Contains(panicText, "app module registration failed") {
				t.Fatalf("unexpected panic: %v", recovered)
			}
			if !strings.Contains(panicText, `module "core" phase "register-loaders" failed`) {
				t.Fatalf("unexpected panic: %v", recovered)
			}
		}()

		_ = root.GetApp()
	})
}

func TestRunAppModuleRegistrationLifecycle(t *testing.T) {
	fixture := newTestFixture(t, testFixtureOptions{})

	t.Run("returns error when app is nil", func(t *testing.T) {
		err := RunAppModuleRegistrationLifecycle(nil, nil)
		if err == nil {
			t.Fatal("expected error when app is nil")
		}
		if !strings.Contains(err.Error(), "app cannot be nil") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("registers phases in deterministic order", func(t *testing.T) {
		var executionTrace []string

		record := func(phase string, moduleID string) AppModuleRegistrationHook {
			return func(app *Vorma) error {
				executionTrace = append(executionTrace, phase+":"+moduleID)
				return nil
			}
		}

		err := RunAppModuleRegistrationLifecycle(fixture.app, []AppModule{
			{
				ModuleID:            "catalog",
				DependencyModuleIDs: []string{"auth"},
				RegisterLoaders:     record("register-loaders", "catalog"),
				RegisterActions:     record("register-actions", "catalog"),
				RegisterRoutes:      record("register-routes", "catalog"),
				Finalize:            record("finalize", "catalog"),
			},
			{
				ModuleID:        "auth",
				RegisterLoaders: record("register-loaders", "auth"),
				RegisterActions: record("register-actions", "auth"),
				RegisterRoutes:  record("register-routes", "auth"),
				Finalize:        record("finalize", "auth"),
			},
		})
		if err != nil {
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
			t.Fatalf("executionTrace = %v, want %v", executionTrace, expectedExecutionTrace)
		}
	})

	t.Run("returns cycle errors from module graph", func(t *testing.T) {
		err := RunAppModuleRegistrationLifecycle(fixture.app, []AppModule{
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
			t.Fatal("expected cycle error")
		}
		if !strings.Contains(err.Error(), "module dependency cycle detected") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
