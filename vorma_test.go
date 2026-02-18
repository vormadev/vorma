package vorma

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/tasks"
)

type testLoaderContext struct {
	req    *LoaderReqData
	marker string
}

type testActionContext[I any] struct {
	req    *ActionReqData[I]
	marker string
}

func TestDefineLoaderForRegistrationUsesDecoratedContext(t *testing.T) {
	requestData := &LoaderReqData{}

	loaderTask := DefineLoaderForRegistration(
		nil,
		"/loader",
		func(ctx *testLoaderContext) (string, error) {
			if ctx == nil {
				t.Fatal("expected decorated loader context")
			}
			if ctx.req != requestData {
				t.Fatal(
					"expected loader context to contain original request data",
				)
			}
			if ctx.marker != "loader-decorated" {
				t.Fatalf(
					"loader context marker = %q, want %q",
					ctx.marker,
					"loader-decorated",
				)
			}
			return "loader-ok", nil
		},
		func(rd *LoaderReqData) *testLoaderContext {
			return &testLoaderContext{
				req:    rd,
				marker: "loader-decorated",
			}
		},
	)
	if loaderTask == nil {
		t.Fatal("expected DefineLoaderForRegistration to return non-nil task")
	}

	got, err := loaderTask.Run(tasks.NewCtx(context.Background()), requestData)
	if err != nil {
		t.Fatalf("loader task returned error: %v", err)
	}
	if got != "loader-ok" {
		t.Fatalf("loader task output = %q, want %q", got, "loader-ok")
	}
}

func TestDefineActionForRegistrationUsesDecoratedContext(t *testing.T) {
	requestData := &ActionReqData[None]{}

	actionTask := DefineActionForRegistration(
		nil,
		"POST",
		"/action",
		func(ctx *testActionContext[None]) (string, error) {
			if ctx == nil {
				t.Fatal("expected decorated action context")
			}
			if ctx.req != requestData {
				t.Fatal(
					"expected action context to contain original request data",
				)
			}
			if ctx.marker != "action-decorated" {
				t.Fatalf(
					"action context marker = %q, want %q",
					ctx.marker,
					"action-decorated",
				)
			}
			return "action-ok", nil
		},
		func(rd *ActionReqData[None]) *testActionContext[None] {
			return &testActionContext[None]{
				req:    rd,
				marker: "action-decorated",
			}
		},
	)
	if actionTask == nil {
		t.Fatal("expected DefineActionForRegistration to return non-nil task")
	}

	got, err := actionTask.Run(tasks.NewCtx(context.Background()), requestData)
	if err != nil {
		t.Fatalf("action task returned error: %v", err)
	}
	if got != "action-ok" {
		t.Fatalf("action task output = %q, want %q", got, "action-ok")
	}
}

func TestDefineLoaderForRegistrationPanicsWhenLoaderFunctionIsNil(
	t *testing.T,
) {
	var loaderFunc func(*LoaderReqData) (string, error)

	expectPanicContaining(
		t,
		"vorma.DefineLoaderForRegistration: loader function cannot be nil",
		func() {
			_ = DefineLoaderForRegistration(
				nil,
				"/loader",
				loaderFunc,
				func(rd *LoaderReqData) *LoaderReqData { return rd },
			)
		},
	)
}

func TestDefineLoaderForRegistrationPanicsWhenDecorateContextIsNil(
	t *testing.T,
) {
	var decorateLoaderContext func(*LoaderReqData) *LoaderReqData

	expectPanicContaining(
		t,
		"vorma.DefineLoaderForRegistration: decorateCtx cannot be nil",
		func() {
			_ = DefineLoaderForRegistration(
				nil,
				"/loader",
				func(rd *LoaderReqData) (string, error) { return "ok", nil },
				decorateLoaderContext,
			)
		},
	)
}

func TestDefineActionForRegistrationPanicsWhenActionFunctionIsNil(
	t *testing.T,
) {
	var actionFunc func(*ActionReqData[None]) (string, error)

	expectPanicContaining(
		t,
		"vorma.DefineActionForRegistration: action function cannot be nil",
		func() {
			_ = DefineActionForRegistration(
				nil,
				"POST",
				"/action",
				actionFunc,
				func(rd *ActionReqData[None]) *ActionReqData[None] { return rd },
			)
		},
	)
}

func TestDefineActionForRegistrationPanicsWhenDecorateContextIsNil(
	t *testing.T,
) {
	var decorateActionContext func(*ActionReqData[None]) *ActionReqData[None]

	expectPanicContaining(
		t,
		"vorma.DefineActionForRegistration: decorateCtx cannot be nil",
		func() {
			_ = DefineActionForRegistration(
				nil,
				"POST",
				"/action",
				func(rd *ActionReqData[None]) (string, error) { return "ok", nil },
				decorateActionContext,
			)
		},
	)
}

func TestCurrentReleaseVersionMatchesCanonicalVersionArtifacts(t *testing.T) {
	currentVersion := strings.TrimSpace(CurrentReleaseVersion())
	if currentVersion == "" {
		t.Fatal("expected CurrentReleaseVersion to return non-empty version")
	}

	versionFileContents, err := os.ReadFile("internal/__LAST_RELEASE.txt")
	if err != nil {
		t.Fatalf("read internal/__LAST_RELEASE.txt: %v", err)
	}
	canonicalVersion := strings.TrimSpace(string(versionFileContents))
	if canonicalVersion == "" {
		t.Fatal(
			"expected internal/__LAST_RELEASE.txt to contain non-empty version",
		)
	}
	if currentVersion != canonicalVersion {
		t.Fatalf(
			"CurrentReleaseVersion() = %q, want %q (from internal/__LAST_RELEASE.txt)",
			currentVersion,
			canonicalVersion,
		)
	}

	type packageJSON struct {
		Version string `json:"version"`
	}
	readPackageVersion := func(path string) string {
		t.Helper()
		packageJSONContents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}

		var parsedPackageJSON packageJSON
		if unmarshalErr := json.Unmarshal(packageJSONContents, &parsedPackageJSON); unmarshalErr != nil {
			t.Fatalf("parse %s: %v", path, unmarshalErr)
		}
		if parsedPackageJSON.Version == "" {
			t.Fatalf("expected %s version to be non-empty", path)
		}
		return parsedPackageJSON.Version
	}

	rootNPMVersion := readPackageVersion("package.json")
	if rootNPMVersion != canonicalVersion {
		t.Fatalf(
			"package.json version = %q, want %q (from internal/__LAST_RELEASE.txt)",
			rootNPMVersion,
			canonicalVersion,
		)
	}

	createNPMVersion := readPackageVersion(
		"typescript/vorma/create/package.json",
	)
	if createNPMVersion != canonicalVersion {
		t.Fatalf(
			"typescript/vorma/create/package.json version = %q, want %q (from internal/__LAST_RELEASE.txt)",
			createNPMVersion,
			canonicalVersion,
		)
	}
}

func expectPanicContaining(t *testing.T, expectedSubstring string, fn func()) {
	t.Helper()
	defer func() {
		recoveredValue := recover()
		if recoveredValue == nil {
			t.Fatalf("expected panic containing %q", expectedSubstring)
		}
		panicText := fmt.Sprint(recoveredValue)
		if !strings.Contains(panicText, expectedSubstring) {
			t.Fatalf(
				"panic = %q, expected to contain %q",
				panicText,
				expectedSubstring,
			)
		}
	}()

	fn()
}
