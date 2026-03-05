package devreload

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/vormabuild/internal/testkit"
)

const (
	reloadTriggerRouteDefinitionsWatch = "route-definitions-watch"
	reloadTriggerHTMLTemplateWatch     = "html-template-watch"
)

func TestNextReloadAttemptID_UsesReloadPrefixAndMonotonicSequence(
	t *testing.T,
) {
	firstAttemptID := nextReloadAttemptID()
	secondAttemptID := nextReloadAttemptID()

	if !strings.HasPrefix(firstAttemptID, "reload-") {
		t.Fatalf("first attempt ID=%q, expected reload- prefix", firstAttemptID)
	}
	if !strings.HasPrefix(secondAttemptID, "reload-") {
		t.Fatalf(
			"second attempt ID=%q, expected reload- prefix",
			secondAttemptID,
		)
	}
	if firstAttemptID == secondAttemptID {
		t.Fatalf(
			"expected attempt IDs to be unique, got first=%q second=%q",
			firstAttemptID,
			secondAttemptID,
		)
	}
}

func TestGetDeferredFrameworkRuntimeReloadAction(t *testing.T) {
	fixture := testkit.NewBuildTestFixture(t, nil)
	app := fixture.App
	app.WithLock(func(lockedVorma *vormaruntime.LockedVorma) {
		lockedVorma.SetBuildID("build-for-reload-action")
	})

	t.Run(
		"returns browser reload action with normalized deferred request metadata",
		func(t *testing.T) {
			executor := newReloadActionExecutor(reloadActionDependencies{
				nextReloadAttemptID: func() string {
					return "reload-test-attempt"
				},
			})

			action := executor.getDeferredFrameworkRuntimeReloadAction(
				app,
				" reload-routes ",
				"reload warning",
				" route-definitions-watch ",
				nil,
				"",
			)
			if action == nil {
				t.Fatal("expected non-nil reload action on success")
			}
			if !action.ReloadBrowser || !action.WaitForApp ||
				!action.WaitForVite {
				t.Fatalf(
					"action=%#v, expected browser reload + wait action",
					action,
				)
			}
			if action.TriggerRestart || action.RecompileGo {
				t.Fatalf(
					"action=%#v, expected no restart/recompile on success",
					action,
				)
			}
			if action.FrameworkRuntimeReloadRequest == nil {
				t.Fatal("expected deferred framework runtime reload request")
			}
			request := action.FrameworkRuntimeReloadRequest
			if request.EndpointPath != "/reload-routes" {
				t.Fatalf(
					"endpoint=%q, want %q",
					request.EndpointPath,
					"/reload-routes",
				)
			}
			if request.ReloadAttemptID != "reload-test-attempt" {
				t.Fatalf(
					"reloadAttemptID=%q, want %q",
					request.ReloadAttemptID,
					"reload-test-attempt",
				)
			}
			if request.ExpectedBuildID != "" {
				t.Fatalf(
					"expectedBuildID=%q, want empty string",
					request.ExpectedBuildID,
				)
			}
			if request.ReloadTrigger != reloadTriggerRouteDefinitionsWatch {
				t.Fatalf(
					"reloadTrigger=%q, want %q",
					request.ReloadTrigger,
					reloadTriggerRouteDefinitionsWatch,
				)
			}
		},
	)

	t.Run(
		"uses explicit expected build ID when provided",
		func(t *testing.T) {
			executor := newReloadActionExecutor(reloadActionDependencies{
				nextReloadAttemptID: func() string {
					return "reload-test-attempt"
				},
			})

			action := executor.getDeferredFrameworkRuntimeReloadAction(
				app,
				" reload-routes ",
				"reload warning",
				" route-definitions-watch ",
				nil,
				"build-for-reload-action",
			)
			if action == nil || action.FrameworkRuntimeReloadRequest == nil {
				t.Fatalf("expected deferred request, got %#v", action)
			}
			if got, want := action.FrameworkRuntimeReloadRequest.ExpectedBuildID, "build-for-reload-action"; got != want {
				t.Fatalf("expectedBuildID=%q, want %q", got, want)
			}
		},
	)

	t.Run(
		"missing endpoint path falls back to restart without recompilation",
		func(t *testing.T) {
			executor := newReloadActionExecutor(reloadActionDependencies{
				nextReloadAttemptID: func() string {
					return "reload-test-attempt"
				},
			})

			action := executor.getDeferredFrameworkRuntimeReloadAction(
				app,
				" ",
				"reload warning",
				reloadTriggerRouteDefinitionsWatch,
				nil,
				"",
			)
			if action == nil {
				t.Fatal(
					"expected fallback action when endpoint path is missing",
				)
			}
			if !action.TriggerRestart || action.RecompileGo {
				t.Fatalf(
					"action=%#v, expected restart without recompilation",
					action,
				)
			}
			if action.ReloadBrowser || action.WaitForApp || action.WaitForVite {
				t.Fatalf(
					"action=%#v, expected no browser reload/wait on fallback",
					action,
				)
			}
			if action.FrameworkRuntimeReloadRequest != nil {
				t.Fatalf(
					"action=%#v, expected nil deferred request on fallback",
					action,
				)
			}
		},
	)

	t.Run(
		"nil app still returns deferred request with empty expected build ID",
		func(t *testing.T) {
			executor := newReloadActionExecutor(reloadActionDependencies{
				nextReloadAttemptID: func() string {
					return "reload-test-attempt"
				},
			})

			action := executor.getDeferredFrameworkRuntimeReloadAction(
				nil,
				"/reload-template",
				"reload warning",
				reloadTriggerHTMLTemplateWatch,
				nil,
				"",
			)
			if action == nil || action.FrameworkRuntimeReloadRequest == nil {
				t.Fatalf(
					"expected deferred request for nil app, got %#v",
					action,
				)
			}
			if got := action.FrameworkRuntimeReloadRequest.ExpectedBuildID; got != "" {
				t.Fatalf(
					"expected empty expectedBuildID for nil app, got %q",
					got,
				)
			}
		},
	)

	t.Run(
		"explicit expected build ID overrides runtime build ID",
		func(t *testing.T) {
			executor := newReloadActionExecutor(reloadActionDependencies{
				nextReloadAttemptID: func() string {
					return "reload-test-attempt"
				},
			})

			action := executor.getDeferredFrameworkRuntimeReloadAction(
				app,
				"/reload-routes",
				"reload warning",
				reloadTriggerRouteDefinitionsWatch,
				nil,
				"build-before-fast-reload",
			)
			if action == nil || action.FrameworkRuntimeReloadRequest == nil {
				t.Fatalf("expected deferred request, got %#v", action)
			}
			if got, want := action.FrameworkRuntimeReloadRequest.ExpectedBuildID, "build-before-fast-reload"; got != want {
				t.Fatalf("expectedBuildID=%q, want %q", got, want)
			}
		},
	)
}
