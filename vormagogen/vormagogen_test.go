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

func TestRegisterLoaderDiscoveredByBuildPanicsWhenLoaderFunctionIsNil(
	t *testing.T,
) {
	expectPanicContaining(
		t,
		"vormagogen.RegisterLoaderDiscoveredByBuild: loader function cannot be nil",
		func() {
			_ = RegisterLoaderDiscoveredByBuild[
				string,
				*vorma.LoaderReqData,
				vorma.LoaderReqData,
			](
				&vorma.Vorma{},
				"/loader",
				nil,
				func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
			)
		},
	)
}

func TestRegisterLoaderDiscoveredByBuildPanicsWhenDecorateContextIsNil(
	t *testing.T,
) {
	expectPanicContaining(
		t,
		"vormagogen.RegisterLoaderDiscoveredByBuild: decorateCtx cannot be nil",
		func() {
			_ = RegisterLoaderDiscoveredByBuild(
				&vorma.Vorma{},
				"/loader",
				func(rd *vorma.LoaderReqData) (string, error) { return "ok", nil },
				nil,
			)
		},
	)
}

func TestRegisterActionDiscoveredByBuildPanicsWhenActionFunctionIsNil(
	t *testing.T,
) {
	expectPanicContaining(
		t,
		"vormagogen.RegisterActionDiscoveredByBuild: action function cannot be nil",
		func() {
			_ = RegisterActionDiscoveredByBuild[
				vorma.None,
				string,
				*vorma.ActionReqData[vorma.None],
				vorma.ActionReqData[vorma.None],
			](
				&vorma.Vorma{},
				"POST",
				"/action",
				nil,
				func(rd *vorma.ActionReqData[vorma.None]) *vorma.ActionReqData[vorma.None] {
					return rd
				},
			)
		},
	)
}

func TestRegisterActionDiscoveredByBuildPanicsWhenDecorateContextIsNil(
	t *testing.T,
) {
	expectPanicContaining(
		t,
		"vormagogen.RegisterActionDiscoveredByBuild: decorateCtx cannot be nil",
		func() {
			_ = RegisterActionDiscoveredByBuild[
				vorma.None,
				string,
				*vorma.ActionReqData[vorma.None],
				vorma.ActionReqData[vorma.None],
			](
				&vorma.Vorma{},
				"POST",
				"/action",
				func(rd *vorma.ActionReqData[vorma.None]) (string, error) {
					return "ok", nil
				},
				nil,
			)
		},
	)
}

func TestPanicHelpersDoNotPanicForValidArguments(t *testing.T) {
	panicIfNilDiscoveredRegistrationApp("test", &vorma.Vorma{})
	panicIfNilLoaderRegistrationArguments(
		"test",
		func(rd *vorma.LoaderReqData) (string, error) { return "ok", nil },
		func(rd *vorma.LoaderReqData) *vorma.LoaderReqData { return rd },
	)
	panicIfNilActionRegistrationArguments(
		"test",
		func(rd *vorma.ActionReqData[vorma.None]) (string, error) {
			return "ok", nil
		},
		func(rd *vorma.ActionReqData[vorma.None]) *vorma.ActionReqData[vorma.None] {
			return rd
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
			t.Fatalf(
				"expected panic containing %q but function did not panic",
				expectedSubstring,
			)
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
