package vormagogen

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma"
)

func TestRegisterLoaderDiscoveredByBuildPanicsWhenAppIsNil(t *testing.T) {
	expectPanicContaining(
		t,
		"vormagogen.RegisterLoaderDiscoveredByBuild: app cannot be nil",
		func() {
			_ = RegisterLoaderDiscoveredByBuild(
				(*vorma.Vorma)(nil),
				"/loader",
				func(rd *vorma.LoaderReqData) (string, error) { return "ok", nil },
				func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
			)
		},
	)
}

func TestRegisterActionDiscoveredByBuildPanicsWhenAppIsNil(t *testing.T) {
	expectPanicContaining(
		t,
		"vormagogen.RegisterActionDiscoveredByBuild: app cannot be nil",
		func() {
			_ = RegisterActionDiscoveredByBuild(
				(*vorma.Vorma)(nil),
				"POST",
				"/action",
				func(rd *vorma.ActionReqData[vorma.None]) (string, error) { return "ok", nil },
				func(rd *vorma.ActionReqData[vorma.None]) *vorma.ActionReqData[vorma.None] { return rd },
			)
		},
	)
}

func expectPanicContaining(
	t *testing.T,
	expectedSubstring string,
	fn func(),
) {
	t.Helper()

	defer func() {
		recoveredValue := recover()
		if recoveredValue == nil {
			t.Fatalf("expected panic containing %q but function did not panic", expectedSubstring)
		}
		panicMessage := recoveredValueToString(recoveredValue)
		if !strings.Contains(panicMessage, expectedSubstring) {
			t.Fatalf(
				"panic message = %q, expected to contain %q",
				panicMessage,
				expectedSubstring,
			)
		}
	}()

	fn()
}

func recoveredValueToString(recoveredValue any) string {
	switch value := recoveredValue.(type) {
	case string:
		return value
	case error:
		return value.Error()
	default:
		return "non-string panic value"
	}
}
