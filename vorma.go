package vorma

import (
	_ "embed"
	"net/http"

	"github.com/vormadev/vorma/internal/vormaruntime"
	"github.com/vormadev/vorma/kit/headels"
	"github.com/vormadev/vorma/kit/mux"
	"github.com/vormadev/vorma/lab/parseutil"
	"github.com/vormadev/vorma/lab/tsgen"
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

const VormaBuildIDHeaderKey = vormaruntime.VormaBuildIDHeaderKey

func MustGetPort() int { return wave.MustGetPort() }
func GetIsDev() bool   { return wave.GetIsDev() }
func SetModeToDev()    { wave.SetModeToDev() }

func IsJSONRequest(r *http.Request) bool {
	return vormaruntime.IsJSONRequest(r)
}
func EnableThirdPartyRouter(next http.Handler) http.Handler {
	return mux.InjectTasksCtxMiddleware(next)
}

func NewVormaApp(o VormaAppConfig) *Vorma {
	return vormaruntime.NewVormaApp(o)
}

func NewLoader[O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	p string,
	f func(CtxPtr) (O, error),
	decorateCtx func(*LoaderReqData) CtxPtr,
) *Loader[O] {
	panicIfNilLoaderRegistrationArguments("vorma.NewLoader", f, decorateCtx)
	_, _ = app, p
	return newLoaderTask(f, decorateCtx)
}

func NewAction[I any, O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	m string,
	p string,
	f func(CtxPtr) (O, error),
	decorateCtx func(*ActionReqData[I]) CtxPtr,
) *Action[I, O] {
	panicIfNilActionRegistrationArguments("vorma.NewAction", f, decorateCtx)
	_, _, _ = app, m, p
	return newActionTask(f, decorateCtx)
}

func Internal__RegisterDiscoveredLoader[O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	p string,
	f func(CtxPtr) (O, error),
	decorateCtx func(*LoaderReqData) CtxPtr,
) *Loader[O] {
	panicIfNilDiscoveredRegistrationApp("vorma.Internal__RegisterDiscoveredLoader", app)
	panicIfNilLoaderRegistrationArguments("vorma.Internal__RegisterDiscoveredLoader", f, decorateCtx)
	loaderTask := newLoaderTask(f, decorateCtx)
	mux.RegisterNestedTaskHandler(app.LoadersRouter().NestedRouter, p, loaderTask)
	return loaderTask
}

func Internal__RegisterDiscoveredAction[I any, O any, CtxPtr ~*Ctx, Ctx any](
	app *Vorma,
	m string,
	p string,
	f func(CtxPtr) (O, error),
	decorateCtx func(*ActionReqData[I]) CtxPtr,
) *Action[I, O] {
	panicIfNilDiscoveredRegistrationApp("vorma.Internal__RegisterDiscoveredAction", app)
	panicIfNilActionRegistrationArguments("vorma.Internal__RegisterDiscoveredAction", f, decorateCtx)
	actionTask := newActionTask(f, decorateCtx)
	mux.RegisterTaskHandler(app.ActionsRouter().Router, m, p, actionTask)
	return actionTask
}

func newLoaderTask[O any, CtxPtr ~*Ctx, Ctx any](
	loaderFunc func(CtxPtr) (O, error),
	decorateLoaderContext func(*LoaderReqData) CtxPtr,
) *Loader[O] {
	return mux.TaskHandlerFromFunc(func(loaderReqData *LoaderReqData) (O, error) {
		return loaderFunc(decorateLoaderContext(loaderReqData))
	})
}

func newActionTask[I any, O any, CtxPtr ~*Ctx, Ctx any](
	actionFunc func(CtxPtr) (O, error),
	decorateActionContext func(*ActionReqData[I]) CtxPtr,
) *Action[I, O] {
	return mux.TaskHandlerFromFunc(func(actionReqData *ActionReqData[I]) (O, error) {
		return actionFunc(decorateActionContext(actionReqData))
	})
}

func panicIfNilDiscoveredRegistrationApp(caller string, app *Vorma) {
	if app == nil {
		panic(caller + ": app cannot be nil")
	}
}

func panicIfNilLoaderRegistrationArguments[O any, CtxPtr ~*Ctx, Ctx any](
	caller string,
	loaderFunc func(CtxPtr) (O, error),
	decorateLoaderContext func(*LoaderReqData) CtxPtr,
) {
	if loaderFunc == nil {
		panic(caller + ": loader function cannot be nil")
	}
	if decorateLoaderContext == nil {
		panic(caller + ": decorateCtx cannot be nil")
	}
}

func panicIfNilActionRegistrationArguments[I any, O any, CtxPtr ~*Ctx, Ctx any](
	caller string,
	actionFunc func(CtxPtr) (O, error),
	decorateActionContext func(*ActionReqData[I]) CtxPtr,
) {
	if actionFunc == nil {
		panic(caller + ": action function cannot be nil")
	}
	if decorateActionContext == nil {
		panic(caller + ": decorateCtx cannot be nil")
	}
}

//go:embed package.json
var packageJSON string

func Internal__GetCurrentNPMVersion() string {
	_, _, currentVersion := parseutil.PackageJSONFromString(packageJSON)
	return currentVersion
}
