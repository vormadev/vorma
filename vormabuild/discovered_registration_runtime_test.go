package vormabuild

import (
	"testing"

	"github.com/vormadev/vorma"
	"github.com/vormadev/vorma/vormagogen"
)

func TestDiscoveredRegistrationRuntimeHelpers(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := vorma.NewVormaApp(
		vorma.VormaAppConfig{
			Wave:   fixture.app.Wave,
			Logger: testLogger(),
		},
	)

	if got := len(app.RegisteredActionRoutes()); got != 0 {
		t.Fatalf("expected no pre-registered actions, got %d", got)
	}

	loaderTask := vorma.DefineLoaderForRegistration(
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
		t.Fatal("expected DefineLoaderForRegistration to return loader task")
	}
	if app.HasRegisteredLoaderTask("/no-op") {
		t.Fatal(
			"expected DefineLoaderForRegistration to avoid direct router registration",
		)
	}

	_ = vormagogen.RegisterLoaderDiscoveredByBuild(
		app,
		"/registered",
		func(rd *vorma.LoaderReqData) (string, error) {
			return "ok", nil
		},
		func(rd *vorma.LoaderReqData) *vorma.LoaderReqData {
			return rd
		},
	)
	if !app.HasRegisteredLoaderTask("/registered") {
		t.Fatal(
			"expected RegisterLoaderDiscoveredByBuild to register nested handler",
		)
	}

	actionTask := vorma.DefineActionForRegistration(
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
		t.Fatal("expected DefineActionForRegistration to return action task")
	}
	if got := len(app.RegisteredActionRoutes()); got != 0 {
		t.Fatalf(
			"expected DefineActionForRegistration to avoid direct action registration, got %d routes",
			got,
		)
	}

	_ = vormagogen.RegisterActionDiscoveredByBuild(
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
	registeredActionRoutes := app.RegisteredActionRoutes()
	if got := len(registeredActionRoutes); got != 1 {
		t.Fatalf("expected one discovered action registration, got %d", got)
	}
	if registeredActionRoutes[0].Method != "POST" ||
		registeredActionRoutes[0].Pattern != "/registered-action" {
		t.Fatalf(
			"unexpected registered action route: method=%q pattern=%q",
			registeredActionRoutes[0].Method,
			registeredActionRoutes[0].Pattern,
		)
	}
}
