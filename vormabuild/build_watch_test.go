package vormabuild

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestRouteDefinitionsWatchPattern_ReturnsFalseWhenRouteDefsFileMissing(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	app.Config.ClientRouteDefsFile = ""

	pattern, ok := routeDefinitionsWatchPattern(app)
	if ok {
		t.Fatalf("expected no watch pattern when route definitions file is empty, got %#v", pattern)
	}
}

func TestHTMLTemplateWatchPattern_ReturnsFalseWhenTemplateLocationMissing(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app
	app.Config.HTMLTemplateLocation = ""

	pattern, ok := htmlTemplateWatchPattern(app)
	if ok {
		t.Fatalf("expected no watch pattern when html template location is empty, got %#v", pattern)
	}
}

func TestRouteDefinitionsOnChangeCallback_ReturnsErrorWhenNotInDevMode(t *testing.T) {
	fixture := newBuildTestFixture(t, nil)
	app := fixture.app

	callback := routeDefinitionsOnChangeCallback(app)
	action, err := callback(&wave.HookContext{AppStoppedForBatch: false})
	if err == nil {
		t.Fatal("expected callback to return error when app is not in dev mode")
	}
	if !strings.Contains(err.Error(), "only be called in dev mode") {
		t.Fatalf("error = %q, expected non-dev-mode context", err)
	}
	if action != nil {
		t.Fatalf("expected nil action when callback errors, got %#v", action)
	}
}
