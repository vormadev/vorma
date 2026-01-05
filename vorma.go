package vorma

import (
	_ "embed"

	rf "github.com/vormadev/vorma/internal/framework"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/kit/parseutil"
	"github.com/vormadev/vorma/wave"
)

/////////////////////////////////////////////////////////////////////
/////// PUBLIC API
/////////////////////////////////////////////////////////////////////

type (
	Vorma                             = rf.Vorma
	HeadEls                           = headels.HeadEls
	AdHocType                         = rf.AdHocType
	VormaAppConfig                    = rf.VormaAppConfig
	LoadersRouter                     = rf.LoadersRouter
	LoaderReqData                     = rf.LoaderReqData
	ActionsRouter                     = rf.ActionsRouter
	ActionReqData[I any]              = rf.ActionReqData[I]
	None                              = mux.None
	Action[I any, O any]              = rf.TaskHandler[I, O]
	Loader[O any]                     = rf.TaskHandler[None, O]
	BuildOptions                      = rf.BuildOptions
	LoaderFunc[Ctx any, O any]        = func(*Ctx) (O, error)
	ActionFunc[Ctx any, I any, O any] = func(*Ctx) (O, error)
	LoadersRouterOptions              = rf.LoadersRouterOptions
	ActionsRouterOptions              = rf.ActionsRouterOptions
	// Set this as your input type when you want to work with standard
	// HTTP forms (whether "application/x-www-form-urlencoded" or
	// "multipart/form-data"). This is just an empty struct with a
	// marker method to ensure the API client knows to accept FormData
	// as the input type. To get the actual form values, use the underlying
	// *http.Request (by calling `Request()` on your action ctx).
	FormData = rf.FormData
)

var (
	// Wave convenience re-exports
	MustGetPort  = wave.MustGetPort
	GetIsDev     = wave.GetIsDev
	SetModeToDev = wave.SetModeToDev

	IsJSONRequest          = rf.IsJSONRequest
	VormaBuildIDHeaderKey  = rf.VormaBuildIDHeaderKey
	EnableThirdPartyRouter = mux.InjectTasksCtxMiddleware
)

func NewVormaApp(o VormaAppConfig) *Vorma { return rf.NewVormaApp(o) }

func NewLoader[O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	p string,
	f func(CtxPtr) (O, error),
	decorateCtx func(*LoaderReqData) CtxPtr,
) *Loader[O] {
	wrappedF := func(c *LoaderReqData) (O, error) { return f(decorateCtx(c)) }
	loaderTask := mux.TaskHandlerFromFunc(wrappedF)
	mux.RegisterNestedTaskHandler(app.LoadersRouter().NestedRouter, p, loaderTask)
	return loaderTask
}

func NewAction[I any, O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	m string,
	p string,
	f func(CtxPtr) (O, error),
	decorateCtx func(*mux.ReqData[I]) CtxPtr,
) *Action[I, O] {
	wrappedF := func(c *mux.ReqData[I]) (O, error) { return f(decorateCtx(c)) }
	actionTask := mux.TaskHandlerFromFunc(wrappedF)
	mux.RegisterTaskHandler(app.ActionsRouter().Router, m, p, actionTask)
	return actionTask
}

//go:embed package.json
var packageJSON string

// This utility exists primarily in service of the vorma.dev
// website. There is no guarantee that this utility will always
// be available or kept up to date.
func Internal__GetCurrentNPMVersion() string {
	_, _, currentVersion := parseutil.PackageJSONFromString(packageJSON)
	return currentVersion
}
