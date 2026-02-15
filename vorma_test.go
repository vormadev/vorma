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

func TestNewLoaderUsesDecoratedContext(t *testing.T) {
	requestData := &LoaderReqData{}

	loaderTask := NewLoader(
		nil,
		"/loader",
		func(ctx *testLoaderContext) (string, error) {
			if ctx == nil {
				t.Fatal("expected decorated loader context")
			}
			if ctx.req != requestData {
				t.Fatal("expected loader context to contain original request data")
			}
			if ctx.marker != "loader-decorated" {
				t.Fatalf("loader context marker = %q, want %q", ctx.marker, "loader-decorated")
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
		t.Fatal("expected NewLoader to return non-nil task")
	}

	got, err := loaderTask.Run(tasks.NewCtx(context.Background()), requestData)
	if err != nil {
		t.Fatalf("loader task returned error: %v", err)
	}
	if got != "loader-ok" {
		t.Fatalf("loader task output = %q, want %q", got, "loader-ok")
	}
}

func TestNewActionUsesDecoratedContext(t *testing.T) {
	requestData := &ActionReqData[None]{}

	actionTask := NewAction(
		nil,
		"POST",
		"/action",
		func(ctx *testActionContext[None]) (string, error) {
			if ctx == nil {
				t.Fatal("expected decorated action context")
			}
			if ctx.req != requestData {
				t.Fatal("expected action context to contain original request data")
			}
			if ctx.marker != "action-decorated" {
				t.Fatalf("action context marker = %q, want %q", ctx.marker, "action-decorated")
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
		t.Fatal("expected NewAction to return non-nil task")
	}

	got, err := actionTask.Run(tasks.NewCtx(context.Background()), requestData)
	if err != nil {
		t.Fatalf("action task returned error: %v", err)
	}
	if got != "action-ok" {
		t.Fatalf("action task output = %q, want %q", got, "action-ok")
	}
}

func TestNewLoaderPanicsWhenLoaderFunctionIsNil(t *testing.T) {
	var loaderFunc func(*LoaderReqData) (string, error)

	expectPanicContaining(
		t,
		"vorma.NewLoader: loader function cannot be nil",
		func() {
			_ = NewLoader(
				nil,
				"/loader",
				loaderFunc,
				func(rd *LoaderReqData) *LoaderReqData { return rd },
			)
		},
	)
}

func TestNewLoaderPanicsWhenDecorateContextIsNil(t *testing.T) {
	var decorateLoaderContext func(*LoaderReqData) *LoaderReqData

	expectPanicContaining(
		t,
		"vorma.NewLoader: decorateCtx cannot be nil",
		func() {
			_ = NewLoader(
				nil,
				"/loader",
				func(rd *LoaderReqData) (string, error) { return "ok", nil },
				decorateLoaderContext,
			)
		},
	)
}

func TestNewActionPanicsWhenActionFunctionIsNil(t *testing.T) {
	var actionFunc func(*ActionReqData[None]) (string, error)

	expectPanicContaining(
		t,
		"vorma.NewAction: action function cannot be nil",
		func() {
			_ = NewAction(
				nil,
				"POST",
				"/action",
				actionFunc,
				func(rd *ActionReqData[None]) *ActionReqData[None] { return rd },
			)
		},
	)
}

func TestNewActionPanicsWhenDecorateContextIsNil(t *testing.T) {
	var decorateActionContext func(*ActionReqData[None]) *ActionReqData[None]

	expectPanicContaining(
		t,
		"vorma.NewAction: decorateCtx cannot be nil",
		func() {
			_ = NewAction(
				nil,
				"POST",
				"/action",
				func(rd *ActionReqData[None]) (string, error) { return "ok", nil },
				decorateActionContext,
			)
		},
	)
}

func TestInternalRegisterDiscoveredLoaderPanicsWhenAppIsNil(t *testing.T) {
	expectPanicContaining(
		t,
		"vorma.Internal__RegisterDiscoveredLoader: app cannot be nil",
		func() {
			_ = Internal__RegisterDiscoveredLoader(
				(*Vorma)(nil),
				"/loader",
				func(rd *LoaderReqData) (string, error) { return "ok", nil },
				func(rd *LoaderReqData) *LoaderReqData { return rd },
			)
		},
	)
}

func TestInternalRegisterDiscoveredActionPanicsWhenAppIsNil(t *testing.T) {
	expectPanicContaining(
		t,
		"vorma.Internal__RegisterDiscoveredAction: app cannot be nil",
		func() {
			_ = Internal__RegisterDiscoveredAction(
				(*Vorma)(nil),
				"POST",
				"/action",
				func(rd *ActionReqData[None]) (string, error) { return "ok", nil },
				func(rd *ActionReqData[None]) *ActionReqData[None] { return rd },
			)
		},
	)
}

func TestInternalGetCurrentNPMVersionMatchesPackageJSON(t *testing.T) {
	currentVersion := strings.TrimSpace(Internal__GetCurrentNPMVersion())
	if currentVersion == "" {
		t.Fatal("expected Internal__GetCurrentNPMVersion to return non-empty version")
	}

	packageJSONContents, err := os.ReadFile("package.json")
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}

	var parsedPackageJSON struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(packageJSONContents, &parsedPackageJSON); err != nil {
		t.Fatalf("parse package.json: %v", err)
	}
	if parsedPackageJSON.Version == "" {
		t.Fatal("expected package.json version to be non-empty")
	}
	if currentVersion != parsedPackageJSON.Version {
		t.Fatalf(
			"Internal__GetCurrentNPMVersion() = %q, want %q",
			currentVersion,
			parsedPackageJSON.Version,
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
			t.Fatalf("panic = %q, expected to contain %q", panicText, expectedSubstring)
		}
	}()

	fn()
}
