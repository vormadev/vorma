package vormabuild

import (
	"testing"

	"github.com/vormadev/vorma"
)

func TestDiscoveredRegistrationRuntimeHelpers(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

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
}
