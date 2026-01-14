package vorma

import (
	_ "embed"

	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/lab/parseutil"
	"github.com/vormadev/vorma/lab/tsgen"
	"github.com/vormadev/vorma/vormaruntime"
	"github.com/vormadev/vorma/wave"
)

// Type aliases for public API
type (
	Vorma                             = vormaruntime.Vorma
	HeadEls                           = headels.HeadEls
	AdHocType                         = tsgen.AdHocType
	VormaAppConfig                    = vormaruntime.VormaAppConfig
	LoadersRouter                     = vormaruntime.LoadersRouter
	LoaderReqData                     = vormaruntime.LoaderReqData
	ActionsRouter                     = vormaruntime.ActionsRouter
	ActionReqData[I any]              = vormaruntime.ActionReqData[I]
	None                              = mux.None
	Action[I any, O any]              = mux.TaskHandler[I, O]
	Loader[O any]                     = mux.TaskHandler[None, O]
	LoaderFunc[Ctx any, O any]        = func(*Ctx) (O, error)
	ActionFunc[Ctx any, I any, O any] = func(*Ctx) (O, error)
	LoadersRouterOptions              = vormaruntime.LoadersRouterOptions
	ActionsRouterOptions              = vormaruntime.ActionsRouterOptions
	FormData                          = vormaruntime.FormData
	LoaderError                       = vormaruntime.LoaderError
)

// Re-exported functions
var (
	MustGetPort            = wave.MustGetPort
	GetIsDev               = wave.GetIsDev
	SetModeToDev           = wave.SetModeToDev
	IsJSONRequest          = vormaruntime.IsJSONRequest
	VormaBuildIDHeaderKey  = vormaruntime.VormaBuildIDHeaderKey
	EnableThirdPartyRouter = mux.InjectTasksCtxMiddleware
)

func NewVormaApp(o VormaAppConfig) *Vorma {
	return vormaruntime.NewVormaApp(o)
}

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

func Internal__GetCurrentNPMVersion() string {
	_, _, currentVersion := parseutil.PackageJSONFromString(packageJSON)
	return currentVersion
}
