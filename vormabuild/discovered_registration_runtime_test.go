package vormabuild

import (
	"testing"

	"github.com/vormadev/vorma"
)

func TestDiscoveredRegistrationRuntimeHelpers(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	if got := len(app.ActionsRouter().AllRoutes()); got != 0 {
		t.Fatalf("expected no pre-registered actions, got %d", got)
	}

	loaderTask := vorma.NewLoader(
		app,
		"/no-op",
		func(rd *vorma.LoaderReqData) (string, error) {
			return "ok", nil
		},
		func(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
			return rd
		},
	)
	if loaderTask == nil {
		t.Fatal("expected NewLoader to return loader task")
	}
	if app.LoadersRouter().NestedRouter.HasTaskHandler("/no-op") {
		t.Fatal("expected NewLoader to avoid direct router registration")
	}

	_ = vorma.Internal__RegisterDiscoveredLoader(
		app,
		"/registered",
		func(rd *vorma.LoaderReqData) (string, error) {
			return "ok", nil
		},
		func(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
			return rd
		},
	)
	if !app.LoadersRouter().NestedRouter.HasTaskHandler("/registered") {
		t.Fatal("expected Internal__RegisterDiscoveredLoader to register nested handler")
	}

	actionTask := vorma.NewAction(
		app,
		"POST",
		"/no-op-action",
		func(rd *vorma.ActionReqData[vorma.None]) (string, error) {
			return "ok", nil
		},
		func(rd *vorma.ActionReqData[vorma.None]) *vorma.ActionReqData[vorma.None] {
			return rd
		},
	)
	if actionTask == nil {
		t.Fatal("expected NewAction to return action task")
	}
	if got := len(app.ActionsRouter().AllRoutes()); got != 0 {
		t.Fatalf("expected NewAction to avoid direct action registration, got %d routes", got)
	}

	_ = vorma.Internal__RegisterDiscoveredAction(
		app,
		"POST",
		"/registered-action",
		func(rd *vorma.ActionReqData[vorma.None]) (string, error) {
			return "ok", nil
		},
		func(rd *vorma.ActionReqData[vorma.None]) *vorma.ActionReqData[vorma.None] {
			return rd
		},
	)
	registeredActionRoutes := app.ActionsRouter().AllRoutes()
	if got := len(registeredActionRoutes); got != 1 {
		t.Fatalf("expected one discovered action registration, got %d", got)
	}
	if registeredActionRoutes[0].Method() != "POST" || registeredActionRoutes[0].OriginalPattern() != "/registered-action" {
		t.Fatalf(
			"unexpected registered action route: method=%q pattern=%q",
			registeredActionRoutes[0].Method(),
			registeredActionRoutes[0].OriginalPattern(),
		)
	}
}
